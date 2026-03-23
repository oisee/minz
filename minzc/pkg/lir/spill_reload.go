// spill_reload.go — Insert reload instructions for spilled operands.
//
// When WFC/Z3 assigns a vreg to a non-GPR location (IXH/IXL, shadow,
// TSMC, mem), most ALU instructions can't use that location directly.
// This pass scans emitted Z80 assembly and inserts explicit
// load-from-spill / store-to-spill moves around such instructions.
//
// Spill tiers and reload strategies:
//
//	L2 (IXH/IXL/IYH/IYL): LD r, IXH (8T, DD prefix)
//	    → reload to GPR before ALU, store back after if modified
//	L3 (shadow B'..L'): EXX / LD r, r' / EXX (12T)
//	    → swap shadow bank, copy, swap back
//	L4 (TSMC): tsmc_label: LD A, 0 → patched imm8 (7T reload)
//	    → execute the patched instruction to reload
//	L5 (mem): LD A, (memN) / LD (memN), A (13T each way)
//	    → direct memory access via A
//
// The reload register is chosen to avoid conflicts with the instruction's
// other operands. For 16-bit ALU (SBC HL, rr), DE is used as scratch.
package lir

import (
	"fmt"
	"strings"
)

// spillLocNames lists all location names that can't be used directly in ALU ops.
var spillLocNames = map[string]string{
	// L2: IX/IY halves
	"IXH": "ix_half", "IXL": "ix_half", "IYH": "iy_half", "IYL": "iy_half",
	// L3: shadow (would need EXX bracket — complex, skip for now)
	// L4: TSMC
	"tsmc0": "tsmc", "tsmc1": "tsmc", "tsmc2": "tsmc", "tsmc3": "tsmc",
	"tsmc4": "tsmc", "tsmc5": "tsmc", "tsmc6": "tsmc", "tsmc7": "tsmc",
	// L5: memory
	"mem0": "mem", "mem1": "mem", "mem2": "mem", "mem3": "mem",
}

// InsertSpillReloads scans emitted instructions and inserts reload moves
// for any operand assigned to a non-GPR location.
func InsertSpillReloads(asm string, desc *MachineDesc) string {
	lines := strings.Split(asm, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if reloaded, ok := reloadSpillOperand(trimmed, desc); ok {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			for _, rl := range reloaded {
				result = append(result, indent+rl)
			}
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// reloadSpillOperand checks if an instruction uses a spill location
// as an operand and returns replacement instruction sequence.
func reloadSpillOperand(inst string, desc *MachineDesc) ([]string, bool) {
	// 16-bit ALU with spill: SBC/ADD HL, <spill>
	for _, prefix := range []string{"SBC HL, ", "ADD HL, "} {
		if strings.HasPrefix(inst, prefix) {
			operand := inst[len(prefix):]
			if kind, isSpill := spillLocNames[operand]; isSpill {
				opcode := inst[:len(prefix)-2] // "SBC HL" or "ADD HL"
				reload := reloadTo16("DE", operand, kind)
				return append(reload, fmt.Sprintf("%s, DE", opcode)), true
			}
		}
	}

	// 8-bit ALU with spill: ADD A,<spill> / SUB <spill> / AND/OR/XOR/CP <spill>
	for _, spec := range []struct{ prefix, op string }{
		{"ADD A, ", "ADD A, "},
		{"ADC A, ", "ADC A, "},
		{"SUB ", "SUB "},
		{"SBC A, ", "SBC A, "},
		{"AND ", "AND "},
		{"OR ", "OR "},
		{"XOR ", "XOR "},
		{"CP ", "CP "},
	} {
		if strings.HasPrefix(inst, spec.prefix) {
			operand := inst[len(spec.prefix):]
			if kind, isSpill := spillLocNames[operand]; isSpill {
				reload := reloadTo8("B", operand, kind)
				return append(reload, spec.op+"B"), true
			}
		}
	}

	// LD r, <spill> where r is not A (A can do LD A,(nn) directly)
	if strings.HasPrefix(inst, "LD ") && !strings.HasPrefix(inst, "LD A, ") &&
		!strings.HasPrefix(inst, "LD (") {
		parts := strings.SplitN(inst, ", ", 2)
		if len(parts) == 2 {
			operand := parts[1]
			if kind, isSpill := spillLocNames[operand]; isSpill {
				dst := strings.TrimPrefix(parts[0], "LD ")
				if dst != operand { // avoid self-load
					reload := reloadTo8("A", operand, kind)
					return append(reload, fmt.Sprintf("LD %s, A", dst)), true
				}
			}
		}
	}

	return nil, false
}

// reloadTo8 generates instructions to load an 8-bit spill value into targetReg.
func reloadTo8(targetReg, spillName, kind string) []string {
	switch kind {
	case "ix_half":
		// LD A, IXH (DD prefix, 8T) — works directly into any GPR
		return []string{fmt.Sprintf("LD %s, %s", targetReg, spillName)}
	case "tsmc":
		// TSMC reload: execute the patched instruction (puts value in A)
		// Then move to target if needed
		if targetReg == "A" {
			return []string{fmt.Sprintf("%s: LD A, 0", spillName)}
		}
		return []string{
			fmt.Sprintf("%s: LD A, 0", spillName),
			fmt.Sprintf("LD %s, A", targetReg),
		}
	case "mem":
		// LD A, (nn) then move to target
		if targetReg == "A" {
			return []string{fmt.Sprintf("LD A, (%s)", spillName)}
		}
		return []string{
			fmt.Sprintf("LD A, (%s)", spillName),
			fmt.Sprintf("LD %s, A", targetReg),
		}
	}
	return nil
}

// reloadTo16 generates instructions to load a 16-bit spill value into targetPair.
func reloadTo16(targetPair, spillName, kind string) []string {
	switch kind {
	case "mem":
		return []string{fmt.Sprintf("LD %s, (%s)", targetPair, spillName)}
	case "ix_half":
		// IX half is 8-bit — can't be 16-bit operand. Route through 8-bit.
		return []string{
			fmt.Sprintf("LD E, %s", spillName),
			"LD D, 0",
		}
	default:
		// Fallback: load via memory
		return []string{fmt.Sprintf("LD %s, (%s)", targetPair, spillName)}
	}
}
