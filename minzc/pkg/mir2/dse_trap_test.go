package mir2

import (
	"strings"
	"testing"
)

// Both DSE implementations must preserve an unused operation that can trap.
// Folding a proven safe division beforehand remains a valid way to remove it.
func TestDSEPreservesDivisionTraps(t *testing.T) {
	passes := []struct {
		name string
		run  func(*Func)
	}{
		{"Go", DeadStoreElim},
		{"Grace", RunGraceDSE},
	}
	for _, pass := range passes {
		for _, op := range []Op{OpDiv, OpSDiv, OpMod, OpSMod} {
			t.Run(pass.name+"/"+op.String(), func(t *testing.T) {
				m, f := deadDivisionProbe(op, 0)
				_, before := NewVM(m).Call(f.Name, nil)
				if before == nil || !strings.Contains(before.Error(), "zero") {
					t.Fatalf("expected zero-divisor error before DSE, got %v", before)
				}
				pass.run(f)
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				_, after := NewVM(m).Call(f.Name, nil)
				if after == nil || after.Error() != before.Error() {
					t.Fatalf("DSE changed trap: before=%v after=%v", before, after)
				}
			})
		}
	}
}

func TestDSEDeletesFoldedSafeDivision(t *testing.T) {
	for _, op := range []Op{OpDiv, OpSDiv, OpMod, OpSMod} {
		t.Run(op.String(), func(t *testing.T) {
			m, f := deadDivisionProbe(op, 2)
			if !FoldConstants(f) {
				t.Fatal("expected safe constant division to fold")
			}
			DeadStoreElim(f)
			if len(f.Blocks[0].Insts) != 0 {
				t.Fatalf("dead folded division retained %d instructions", len(f.Blocks[0].Insts))
			}
			if _, err := NewVM(m).Call(f.Name, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func deadDivisionProbe(op Op, divisor int64) (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("dead_division")
	a, b, dst := f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Blocks = []*Block{{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: a, Imm: 7, Ty: TyU8},
			{Op: OpConst, Dst: b, Imm: divisor, Ty: TyU8},
			{Op: op, Dst: dst, Src: [2]Reg{a, b}, Ty: TyU8},
		},
		Term: &TermRet{},
	}}
	return m, f
}
