package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func wordMulFixture(k int64, ty mir2.Ty) (*mir2.Func, *mir2.AllocResult) {
	m := &mir2.Module{}
	f := m.AddFunc("multiply")
	x, c, d := f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []mir2.Param{{Name: "x", Reg: x, Ty: ty, Class: mir2.ClassPointer}}
	f.Contract.Returns = []mir2.Return{{Ty: ty, Class: mir2.ClassPointer}}
	b := f.NewBlock("entry")
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: c, Imm: k, Ty: ty})
	b.Append(&mir2.Inst{Op: mir2.OpMul, Dst: d, Src: [2]mir2.Reg{x, c}, Ty: ty})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{d}})
	return f, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "HL"}}}
}
func TestLocalWordMultiplyBoundsAndBinding(t *testing.T) {
	for _, k := range []int64{0, 1, 256, -7} {
		f, a := wordMulFixture(k, mir2.TyU16)
		if _, e := PrepareLocalTableProblem(f, a); e == nil {
			t.Fatal("unsupported multiplier", k)
		}
	}
	f, a := wordMulFixture(7, mir2.TyU16)
	p, e := PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	for _, op := range p.Ops {
		if op.Op != OpAdd && op.Op != OpMove && op.Op != OpRet {
			t.Fatal("opaque lowering", op)
		}
	}
	if len(f.Blocks[0].Insts) != 2 || f.Blocks[0].Insts[1].Op != mir2.OpMul {
		t.Fatal("original MIR mutated")
	}
	keys := map[int]bool{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			if keys[op.Dst] {
				t.Fatal("temp redefined")
			}
			keys[op.Dst] = true
		}
	}
	f.Blocks[0].Insts[0].Imm = 10
	q, e := PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	old, _ := CheckedTableKey(p.Ops, Z80, p.Options)
	newKey, _ := CheckedTableKey(q.Ops, Z80, q.Options)
	if old == newKey {
		t.Fatal("multiplier unbound")
	}
	// A dynamic multiplier is refused; signed-word modulo uses the same bits.
	f.Blocks[0].Insts[1].Src[1] = f.Contract.Params[0].Reg
	if _, e := PrepareLocalTableProblem(f, a); e == nil {
		t.Fatal("dynamic multiply accepted")
	}
	f, a = wordMulFixture(7, mir2.TyI16)
	if _, e = PrepareLocalTableProblem(f, a); e != nil {
		t.Fatal(e)
	}
}
