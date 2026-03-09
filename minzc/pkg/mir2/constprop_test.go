package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// ── PropagateConstants tests ─────────────────────────────────────────────────

// TestConstProp_SinglePredecessor: a block with one predecessor that passes a
// known constant should have its param folded.
//
//	entry:   x = Const(42); Jmp(succ, [x])
//	succ(p): r = Add(p, Const(1)); Ret(r)
//
// After propagation: p → OpConst(42); add uses constReg; codegen can fold CP.
func TestConstProp_SinglePredecessor(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("f")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Const(42, mir2.TyU8, mir2.ClassAcc)
	b.Jmp("succ", x)

	succ := b.SwitchToNewBlock("succ")
	p := b.BlockParam(succ, mir2.TyU8, mir2.ClassAcc)
	one := b.Const(1, mir2.TyU8, mir2.ClassAcc)
	r := b.Add(p, one, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r)

	// Two blocks before; block param p must receive Const(42) from entry.
	changed := mir2.PropagateConstants(f)
	if !changed {
		t.Fatal("PropagateConstants: expected change, got none")
	}

	// After propagation: succ should have a new OpConst(42) prepended.
	// The Add instruction should use that constant reg, not p directly.
	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// The Add instruction should resolve to "ADD A, A" (42+1 doesn't fold,
	// but the param is now a materialised constant → codegen sees Const(42)).
	// More importantly, there should be no spill needed for p.
	if !strings.Contains(asm, "LD A, 42") {
		t.Errorf("expected LD A, 42 in output (constant materialised); got:\n%s", asm)
	}
}

// TestConstProp_BothBranchesConst: a join block whose params come from both
// branches of a BrIf, both passing the same constant.
//
//	entry:  a = Const(7); cond = ...; BrIf(cond, then:[a], else_:[a])
//	then:   Jmp(join, [a])
//	else_:  Jmp(join, [a])
//	join(p): result = Add(p, p); Ret(result)
//
// p should be folded to 7; result = Add(7,7) → codegen emits LD A,7; ADD A,A.
func TestConstProp_BothBranchesConst(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("both")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Const(7, mir2.TyU8, mir2.ClassAcc)
	// Manufacture a cond using the constant itself (non-zero = true)
	zero := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	cond := b.Cmp(mir2.CmpNe, a, zero, mir2.ClassFlag, false)
	b.BrIf(cond, "then", []mir2.Reg{a}, "else_", []mir2.Reg{a})

	b.SwitchToNewBlock("then")
	b.Jmp("join", a)

	b.SwitchToNewBlock("else_")
	b.Jmp("join", a)

	join := b.SwitchToNewBlock("join")
	p := b.BlockParam(join, mir2.TyU8, mir2.ClassAcc)
	result := b.Add(p, p, mir2.TyU8, mir2.ClassAcc)
	b.Ret(result)

	changed := mir2.PropagateConstants(f)
	if !changed {
		t.Fatal("PropagateConstants: expected change, got none")
	}

	mir2.DeadStoreElim(f) // clean up dead uses of original p
	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// join block should now materialise 7 (not receive p from predecessor).
	if !strings.Contains(asm, "LD A, 7") {
		t.Errorf("expected LD A, 7 (constant folded into join); got:\n%s", asm)
	}
}

// TestConstProp_DifferentConstants: param from two branches with DIFFERENT
// constants must NOT be folded.
//
//	entry:  BrIf(cond, then:[Const(1)], else_:[Const(2)])
//	then/else_ → join(p)
//
// p must stay a block param; no OpConst should be prepended.
func TestConstProp_DifferentConstants(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("diff")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	c1 := b.Const(1, mir2.TyU8, mir2.ClassAcc)
	c2 := b.Const(2, mir2.TyU8, mir2.ClassAcc)
	zero := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	cond := b.Cmp(mir2.CmpNe, c1, zero, mir2.ClassFlag, false)
	b.BrIf(cond, "then", []mir2.Reg{c1}, "else_", []mir2.Reg{c2})

	b.SwitchToNewBlock("then")
	b.Jmp("join", c1)

	b.SwitchToNewBlock("else_")
	b.Jmp("join", c2)

	join := b.SwitchToNewBlock("join")
	p := b.BlockParam(join, mir2.TyU8, mir2.ClassAcc)
	b.Ret(p)

	changed := mir2.PropagateConstants(f)
	if changed {
		t.Error("PropagateConstants: must NOT fold param with different constant values on each edge")
	}
}

// TestConstProp_NoFoldForFuncParam: function parameters (entry-block params
// from Contract.Params) must NOT be folded — they have no predecessor edges.
func TestConstProp_NoFoldForFuncParam(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("nofold")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	b.Ret(x)

	before := len(f.Blocks[0].Insts)
	changed := mir2.PropagateConstants(f)
	after := len(f.Blocks[0].Insts)

	if changed {
		t.Error("PropagateConstants: must NOT fold function parameters (no predecessors)")
	}
	if before != after {
		t.Errorf("block insts changed unexpectedly: %d → %d", before, after)
	}
}

// TestConstProp_FibPreservesSemantics: fibonacci must produce correct results
// after constant propagation + reorder + DSE + allocate + codegen + emulate.
func TestConstProp_FibPreservesSemantics(t *testing.T) {
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for mir2.PropagateConstants(f) {
		}
		mir2.DeadStoreElim(f)
	}

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify after constprop: %v", err)
	}

	vm := mir2.NewVM(m)
	cases := [][2]int64{{1, 1}, {5, 5}, {10, 55}}
	for _, tc := range cases {
		got, err := vm.Call("fibonacci", []mir2.Value{{I: tc[0]}})
		if err != nil {
			t.Errorf("fib(%d): %v", tc[0], err)
			continue
		}
		if len(got) != 1 || got[0].I != tc[1] {
			t.Errorf("fib(%d) = %v, want %d", tc[0], got, tc[1])
		}
	}
}
