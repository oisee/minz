package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestLocalWordShiftBoundsAndBinding(t *testing.T) {
	for _, amount := range []int64{-1, 0, 16, 255} {
		f, a := wordMulFixture(amount, mir2.TyU16)
		f.Blocks[0].Insts[1].Op = mir2.OpShl
		if _, err := PrepareLocalTableProblem(f, a); err == nil {
			t.Fatal("invalid shift accepted", amount)
		}
	}
	f, a := wordMulFixture(4, mir2.TyU16)
	f.Blocks[0].Insts[1].Op = mir2.OpShl
	p, err := PrepareLocalTableProblem(f, a)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, op := range p.Ops {
		if op.Op == OpAdd {
			count++
		}
		if op.Op != OpAdd && op.Op != OpMove && op.Op != OpRet {
			t.Fatal("implicit operation", op)
		}
	}
	if count != 4 || f.Blocks[0].Insts[1].Op != mir2.OpShl {
		t.Fatal("shift not explicit or MIR mutated", count)
	}
	old, err := CheckedTableKey(p.Ops, Z80, p.Options)
	if err != nil {
		t.Fatal(err)
	}
	f.Blocks[0].Insts[0].Imm = 5
	q, err := PrepareLocalTableProblem(f, a)
	if err != nil {
		t.Fatal(err)
	}
	current, err := CheckedTableKey(q.Ops, Z80, q.Options)
	if err != nil || old == current {
		t.Fatal("count unbound", err)
	}
	f.Blocks[0].Insts[1].Src[1] = f.Contract.Params[0].Reg
	if _, err = PrepareLocalTableProblem(f, a); err == nil {
		t.Fatal("dynamic shift accepted")
	}
}
