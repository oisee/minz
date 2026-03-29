// intrinsics.go — GPU-precomputed arithmetic intrinsics for VIR.
//
// Bridges between ISLE pattern matching and GPU-optimal instruction sequences.
// Pipeline: ISLE(isel) → IntrinsicTable(strength reduction) → Z3(regalloc)
//
// Tables loaded from z80-optimizer:
//   mul8:  mulopt8_clobber.json (254 entries, A×K→A)
//   mul16: mulopt16_complete.json (254 entries, HL×K→HL)
//   div8:  div8_optimal.json (254 entries, A÷K→A, includes carry_compare for K≥128)
//   u32:   u32_ops.json (13 operations)
//
// Each entry contains ready-to-emit Z80 asm ops[], cost, and clobber sets.
// The solver treats these as OpAsmBlock — opaque instruction blocks with
// declared inputs/outputs/clobbers. Z3 allocates registers around them.
package vir

import (
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// tryConstDiv checks if a MIR2 OpDiv has a constant divisor and returns
// an inline VIROp sequence from the precomputed div8 table.
// Returns nil if no constant divisor or no table entry.
// ABI: A ÷ K → A (quotient). Clobbers from table entry.
func tryConstDiv(inst *mir2.Inst, desc *MachineDesc, w int) []VIROp {
	if w > 8 {
		return nil // only 8-bit for now
	}

	table := GetDivOptTable()
	if table == nil || table.Size() == 0 {
		return nil
	}

	k := int(inst.Imm)
	if k <= 0 || k > 255 {
		return nil
	}

	entry := table.Lookup(k)
	if entry == nil {
		return nil
	}

	varSrc := int(inst.Src[0])
	dstVreg := int(inst.Dst)

	// Build asm template from precomputed ops
	var lines []string
	// Preamble (if any — e.g., LD B,K setup)
	for _, op := range entry.Preamble {
		lines = append(lines, "    "+op)
	}
	for _, op := range entry.Ops {
		lines = append(lines, "    "+op)
	}
	asmTemplate := strings.Join(lines, "\n")

	// Build clobber set from entry
	clobbers := desc.LocSetByNames("A", "F") // A and flags always
	for _, c := range entry.Clobber {
		switch c {
		case "B":
			clobbers = clobbers.Or(desc.LocSetByNames("B"))
		case "C":
			clobbers = clobbers.Or(desc.LocSetByNames("C"))
		case "D":
			clobbers = clobbers.Or(desc.LocSetByNames("D"))
		case "E":
			clobbers = clobbers.Or(desc.LocSetByNames("E"))
		case "H":
			clobbers = clobbers.Or(desc.LocSetByNames("H"))
		case "L":
			clobbers = clobbers.Or(desc.LocSetByNames("L"))
		}
	}

	return []VIROp{{
		Op:          OpAsmBlock,
		Dst:         dstVreg,
		Src:         [2]int{varSrc, -1},
		Width:       w,
		Clobbers:    clobbers,
		DstHint:     desc.LocSetByNames("A"),         // result in A
		SrcHint:     [2]LocSet{desc.LocSetByNames("A")}, // input in A
		AsmTemplate: asmTemplate,
	}}
}

// tryConstMod checks if a MIR2 OpMod has a constant divisor.
// Mod can be computed as: A - (A/K)*K, using the div table.
// For power-of-2 K: AND (K-1) is optimal (4T).
func tryConstMod(inst *mir2.Inst, desc *MachineDesc, w int) []VIROp {
	if w > 8 {
		return nil
	}

	k := int(inst.Imm)
	if k <= 0 || k > 255 {
		return nil
	}

	// Power of 2: AND (K-1)
	if k&(k-1) == 0 && k > 1 {
		mask := k - 1
		return []VIROp{{
			Op:          OpAndImm,
			Dst:         int(inst.Dst),
			Src:         [2]int{int(inst.Src[0]), -1},
			Imm:         int64(mask),
			Width:       w,
		}}
	}

	return nil // non-power-of-2 mod: fall through to runtime
}

// IntrinsicStats tracks how many intrinsic replacements were applied.
type IntrinsicStats struct {
	Mul8Inline  int // constant mul replaced with GPU-optimal sequence
	Div8Inline  int // constant div replaced with GPU-optimal sequence
	Mod8Inline  int // constant mod replaced (power-of-2 AND)
}
