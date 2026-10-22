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

func TestPruneUnreachableFuncsBoundIntrinsicAndConditionalCall(t *testing.T) {
	for _, op := range []Op{OpCall, OpCallCond} {
		m := &Module{Funcs: []*Func{
			{Name: "main", Blocks: []*Block{{Insts: []*Inst{{Op: op, Sym: "@bound"}}}}},
			{Name: "@bound"}, {Name: "dead"},
		}}
		if !PruneUnreachableFuncs(m, map[string]bool{"main": true}) || m.FuncByName("@bound") == nil || m.FuncByName("dead") != nil {
			t.Fatalf("call %s lost bound callee", op)
		}
	}
}

// MIR-only producers must not need a parallel HIR root to protect an entry.
func TestPruneUnreachableFuncsNamedEntry(t *testing.T) {
	m := &Module{Funcs: []*Func{
		{Name: "provider", Attrs: FuncAttrs{Private: true, ReferencedByName: true}, Blocks: []*Block{{Insts: []*Inst{{Op: OpCall, Sym: "dependency"}}}}},
		{Name: "dependency"}, {Name: "dead"},
	}}
	if !PruneUnreachableFuncs(m, nil) || len(m.Funcs) != 2 || m.FuncByName("provider") == nil || m.FuncByName("dependency") == nil {
		t.Fatal("name-based entry or its dependency pruned")
	}
}

func TestCPMRuntimeClosureNamedEntry(t *testing.T) {
	m := &Module{Funcs: []*Func{
		{Name: "main"},
		{Name: "provider", Attrs: FuncAttrs{Private: true, ReferencedByName: true}, Blocks: []*Block{{Insts: []*Inst{{Op: OpCall, Sym: "dependency"}}}}},
		{Name: "dependency"}, {Name: "dead"},
	}}
	linked, err := cpmRuntimeClosure(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Funcs) != 4 || len(linked.Funcs) != 3 || linked.FuncByName("provider") == nil || linked.FuncByName("dependency") == nil {
		t.Fatal("runtime closure discarded named entry or modified source")
	}
}
