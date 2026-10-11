package mir2

import "testing"

func TestVerifyRejectsNilContainersBeforeSemanticScan(t *testing.T) {
	for _, m := range []*Module{nil, {Funcs: []*Func{nil}}, {Funcs: []*Func{{Name: "nil-block", Blocks: []*Block{nil}}}}, {Funcs: []*Func{{Name: "nil-inst", Blocks: []*Block{{Label: "entry", Insts: []*Inst{nil}, Term: &TermRet{}}}}}}} {
		if err := Verify(m); err == nil {
			t.Fatal("accepted nil MIR container")
		}
	}
}
