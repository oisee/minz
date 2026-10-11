package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/z80spec"
	"strings"
)

// Table patterns may use fixed scratch registers not named in their operands.
// Inspect expanded instructions independently of cached assignment metadata.
// Unknown instruction effects reject the fast path rather than guessing.
func tableInstructionWrites(text string, desc *MachineDesc) (LocSet, error) {
	var writes LocSet
	add := func(name string) {
		if name == "AF" {
			writes = writes.Or(desc.LocSetByNames("A", "F"))
			return
		}
		if loc := desc.LocByName(name); loc >= 0 {
			writes = writes.Set(loc)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.ToUpper(strings.TrimSpace(strings.SplitN(line, ";", 2)[0]))
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		fields := strings.Fields(line)
		op := fields[0]
		operand := strings.TrimSpace(strings.TrimPrefix(line, op))
		dst := strings.TrimSpace(strings.SplitN(operand, ",", 2)[0])
		switch op {
		case "LD":
			if !strings.HasPrefix(dst, "(") {
				add(dst)
			}
		case "INC", "DEC":
			add(dst)
			if loc := desc.LocByName(dst); loc >= 0 && desc.Locs[loc].Width <= 8 {
				add("F")
			}
		case "ADD", "ADC", "SBC":
			add(dst)
			add("F")
		case "SUB", "AND", "OR", "XOR", "NEG", "CPL":
			add("A")
			add("F")
		case "CP", "BIT", "SCF", "CCF":
			add("F")
		case "SLA", "SRA", "SRL", "RL", "RR", "RLC", "RRC":
			add(dst)
			add("F")
		case "RLCA", "RRCA", "RLA", "RRA":
			add("A")
			add("F")
		case "SET", "RES":
			parts := strings.Split(operand, ",")
			if len(parts) != 2 {
				return 0, fmt.Errorf("invalid bit instruction %s", line)
			}
			add(strings.TrimSpace(parts[1]))
		case "POP":
			add(dst)
		case "PUSH", "RET", "NOP":
		case "EX":
			parts := strings.Split(operand, ",")
			if len(parts) != 2 {
				return 0, fmt.Errorf("invalid EX %s", line)
			}
			for _, part := range parts {
				add(strings.TrimSpace(part))
			}
		default:
			return 0, fmt.Errorf("unverified pattern instruction %s", line)
		}
	}
	return writes, nil
}

func validateTablePatternWrites(ops []VIROp, index int, assignment map[int]int, desc *MachineDesc, pir PIROp) error {
	writes, err := tableInstructionWrites(pir.Emit(desc), desc)
	if err != nil {
		return err
	}
	writes = writes.Or(pir.Pat.Clobbers)
	live := map[int]bool{}
	for i := len(ops) - 1; i > index; i-- {
		delete(live, ops[i].Dst)
		for _, v := range ops[i].AsmOuts {
			delete(live, v)
		}
		for _, v := range ops[i].Src {
			if v > 0 {
				live[v] = true
			}
		}
		for _, v := range ops[i].AsmIns {
			if v > 0 {
				live[v] = true
			}
		}
	}
	for v := range live {
		if v == ops[index].Dst {
			continue
		}
		loc := assignment[v]
		conflict := false
		writes.ForEach(func(w int) bool {
			if w >= len(desc.Locs) || w == loc || desc.Locs[w].Alias.Has(loc) || desc.Locs[loc].Alias.Has(w) || (desc.Name == "z80" && z80spec.Overlaps(desc.Locs[w].Name, desc.Locs[loc].Name)) {
				conflict = true
				return false
			}
			return true
		})
		if conflict {
			return fmt.Errorf("op %d pattern %s overwrites live vreg %d at %s", index, pir.Pat.Name, v, desc.Locs[loc].Name)
		}
	}
	return nil
}
