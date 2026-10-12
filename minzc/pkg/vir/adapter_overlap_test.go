package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestAdapterRejectsOverlappingParameterABI(t *testing.T) {
	f := &mir2.Func{Name: "probe", Contract: mir2.Contract{Params: []mir2.Param{{Reg: 1, Ty: mir2.TyU16}, {Reg: 2, Ty: mir2.TyU8}}}}
	for _, tc := range []struct {
		name          string
		caller, stand map[int]int
	}{
		{"unchanged destination", map[int]int{1: Z80.LocByName("HL"), 2: Z80.LocByName("A")}, map[int]int{1: Z80.LocByName("HL"), 2: Z80.LocByName("H")}},
		{"caller alias", map[int]int{1: Z80.LocByName("IX"), 2: Z80.LocByName("IXH")}, map[int]int{1: Z80.LocByName("HL"), 2: Z80.LocByName("A")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asm, err := emitAdapterEntry("probe:\nRET\n", f, tc.caller, tc.stand, Z80)
			if err == nil || asm != "" {
				t.Fatalf("overlapping independent arguments admitted: %q %v", asm, err)
			}
		})
	}
}

func TestAdapterRejectsContradictoryMoves(t *testing.T) {
	moves := []adapterMove{{Z80.LocByName("HL"), Z80.LocByName("BC")}, {Z80.LocByName("A"), Z80.LocByName("B")}}
	for _, emit := range []func([]adapterMove, *MachineDesc) []string{resolveParallelMoves, emitStackAdapter} {
		lines := emit(moves, Z80)
		if len(lines) != 1 || !strings.HasPrefix(lines[0], "ERROR ") {
			t.Fatalf("conflicting destination admitted: %v", lines)
		}
	}
}
