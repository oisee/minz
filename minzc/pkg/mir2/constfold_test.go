package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// ── FoldConstants tests ──────────────────────────────────────────────────────

func TestFoldConst_BinaryArith(t *testing.T) {
	// entry: x=Const(10); y=Const(3); z=Add(x,y); Ret(z)
	// After fold: z=Const(13); Ret(Const(13)) → codegen: LD A,13; RET
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("add_consts")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Const(10, mir2.TyU8, mir2.ClassAcc)
	y := b.Const(3, mir2.TyU8, mir2.ClassAcc)
	z := b.Add(x, y, mir2.TyU8, mir2.ClassAcc)
	b.Ret(z)

	changed := mir2.FoldConstants(f)
	if !changed {
		t.Fatal("FoldConstants: expected change for Add(Const,Const)")
	}
	mir2.DeadStoreElim(f)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	if !strings.Contains(asm, "LD A, 13") {
		t.Errorf("expected LD A, 13 (10+3 folded); got:\n%s", asm)
	}
}

func TestFoldConst_Chain(t *testing.T) {
	// x=Const(3); y=Mul(x,Const(2)); z=Add(y,Const(1))
	// → y=Const(6); z=Const(7)
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("chain")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Const(3, mir2.TyU8, mir2.ClassAcc)
	two := b.Const(2, mir2.TyU8, mir2.ClassAcc)
	y := b.Mul(x, two, mir2.TyU8, mir2.ClassAcc)
	one := b.Const(1, mir2.TyU8, mir2.ClassAcc)
	z := b.Add(y, one, mir2.TyU8, mir2.ClassAcc)
	b.Ret(z)

	mir2.FoldConstants(f)
	mir2.DeadStoreElim(f)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	if !strings.Contains(asm, "LD A, 7") {
		t.Errorf("expected LD A, 7 (chain 3*2+1 folded); got:\n%s", asm)
	}
}

func TestFoldConst_Bitwise(t *testing.T) {
	// AND(0b11001100, 0b10101010) = 0b10001000 = 136
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("bits")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Const(0b11001100, mir2.TyU8, mir2.ClassAcc)
	bb := b.Const(0b10101010, mir2.TyU8, mir2.ClassAcc)
	r := b.And(a, bb, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r)

	mir2.FoldConstants(f)
	mir2.DeadStoreElim(f)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)

	if !strings.Contains(asm, "LD A, 136") {
		t.Errorf("expected LD A, 136 (0xCC & 0xAA folded); got:\n%s", asm)
	}
}

func TestFoldConst_Cmp(t *testing.T) {
	// Cmp(Eq, Const(5), Const(5)) → Const(1) (true)
	// BrIf(1, then, else_) → branch is constant, DCE would remove one side
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("cmp")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	five := b.Const(5, mir2.TyU8, mir2.ClassAcc)
	cond := b.Cmp(mir2.CmpEq, five, five, mir2.ClassFlag, false)
	b.BrIf(cond, "yes", nil, "no", nil)

	b.SwitchToNewBlock("yes")
	b.Ret(b.Const(1, mir2.TyU8, mir2.ClassAcc))

	b.SwitchToNewBlock("no")
	b.Ret(b.Const(0, mir2.TyU8, mir2.ClassAcc))

	changed := mir2.FoldConstants(f)
	if !changed {
		t.Fatal("FoldConstants: Cmp(Eq,5,5) should fold to Const(1)")
	}
	// The folded cond (Const(1)) is now a known constant.
	// Further: BrIf(Const(1)) → codegen always takes the "yes" branch.
	// (Branch-on-constant elimination is a future pass; for now just verify folding happened.)
}

func TestFoldConst_NoFoldNonConst(t *testing.T) {
	// Add(param, Const(1)) — param is not a constant; must not fold.
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("nofold")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	one := b.Const(1, mir2.TyU8, mir2.ClassAcc)
	r := b.Add(x, one, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r)

	changed := mir2.FoldConstants(f)
	if changed {
		t.Error("FoldConstants: must NOT fold Add(param, Const) when param is not constant")
	}
}

func TestFoldConst_WithPropagation(t *testing.T) {
	// PropagateConstants + FoldConstants together:
	//   entry: x=Const(20); Jmp(succ, [x])
	//   succ(p): r=Add(p, Const(2)); Ret(r)
	// After PropagateConstants: p→Const(20)
	// After FoldConstants: r=Add(Const(20),Const(2))→Const(22)
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("combined")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Const(20, mir2.TyU8, mir2.ClassAcc)
	b.Jmp("succ", x)

	succ := b.SwitchToNewBlock("succ")
	p := b.BlockParam(succ, mir2.TyU8, mir2.ClassAcc)
	two := b.Const(2, mir2.TyU8, mir2.ClassAcc)
	r := b.Add(p, two, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r)

	mir2.PropagateConstants(f)
	mir2.FoldConstants(f)
	mir2.DeadStoreElim(f)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	if !strings.Contains(asm, "LD A, 22") {
		t.Errorf("expected LD A, 22 (20+2 propagated+folded); got:\n%s", asm)
	}
}
