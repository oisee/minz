package mir2

import "testing"

func TestPruneUnreachableFuncsOpaqueAssembly(t *testing.T) {
	for _, template := range []string{"CALL imported__helper", "JP NZ, imported__helper", "LD HL, imported__helper / JP (HL)", "DW imported__helper"} {
		t.Run(template, func(t *testing.T) {
			m := &Module{Funcs: []*Func{
				{Name: "main", Blocks: []*Block{{Insts: []*Inst{{Op: OpAsm, Asm: &AsmExtra{Template: template}}}}}},
				{Name: "imported__helper"}, {Name: "other"},
			}}
			if PruneUnreachableFuncs(m, map[string]bool{"main": true}) || len(m.Funcs) != 3 {
				t.Fatal("opaque reachable assembly must retain potential function targets")
			}
		})
	}
}

func TestPruneUnreachableFuncsUnreachableAssembly(t *testing.T) {
	m := &Module{Funcs: []*Func{
		{Name: "main", Blocks: []*Block{{Insts: []*Inst{{Op: OpCall, Sym: "helper"}}}}},
		{Name: "helper"},
		{Name: "dead", Blocks: []*Block{{Insts: []*Inst{{Op: OpAsm, Asm: &AsmExtra{Template: "CALL unrelated"}}}}}},
		{Name: "unrelated"},
	}}
	if !PruneUnreachableFuncs(m, map[string]bool{"main": true}) || len(m.Funcs) != 2 || m.FuncByName("helper") == nil {
		t.Fatal("unreachable assembly should not disable ordinary call-graph pruning")
	}
}
