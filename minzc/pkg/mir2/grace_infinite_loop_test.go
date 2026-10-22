package mir2

import (
	"fmt"
	"testing"
)

func TestGraceKeepsSelfLoopTargets(t *testing.T) {
	for _, params := range []bool{false, true} {
		t.Run(fmt.Sprint(params), func(t *testing.T) {
			f := &Func{Name: "spin", nextReg: 10}
			entry := &Block{Label: "entry", Term: &TermJmp{Target: "cycle"}}
			cycle := &Block{Label: "cycle", Term: &TermJmp{Target: "cycle"}}
			if params {
				entry.Insts = []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 7}}
				entry.Term.(*TermJmp).Args = []Reg{1}
				cycle.Params = []BlockParam{{Dst: 2, Ty: TyU8, Class: ClassGeneral}}
				cycle.Term.(*TermJmp).Args = []Reg{2}
			}
			f.Blocks = []*Block{entry, cycle}
			m := &Module{Funcs: []*Func{f}}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			RunGracePasses(f, NewGraceStats())
			EliminateDeadBlocks(f)
			if err := Verify(m); err != nil {
				t.Fatalf("params=%v: %v", params, err)
			}
			b := f.Blocks[0]
			seen := map[string]bool{}
			for !seen[b.Label] {
				seen[b.Label] = true
				j, ok := b.Term.(*TermJmp)
				if !ok {
					t.Fatalf("params=%v: cycle replaced with %T", params, b.Term)
				}
				b = f.BlockByLabel(j.Target)
				if b == nil {
					t.Fatal("lost cycle target")
				}
			}
		})
	}
}
