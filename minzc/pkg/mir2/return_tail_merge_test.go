package mir2

import "testing"

func returnTailFixture() *Func {
	f := &Func{Name: "tails", Contract: Contract{Returns: []Return{{Ty: TyU8, Class: ClassAcc}}}}
	f.Blocks = []*Block{
		{Label: "entry", Term: &TermBrIf{Cond: 1, Then: "yes", Else: "again"}},
		{Label: "yes", Insts: []*Inst{{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 1}}, Term: &TermRet{Vals: []Reg{2}}},
		{Label: "again", Term: &TermCondRet{Cond: 1, Then: "other", Vals: []Reg{1}}},
		{Label: "other", Insts: []*Inst{{Op: OpConst, Dst: 3, Ty: TyU8, Imm: 1}}, Term: &TermRet{Vals: []Reg{3}}},
	}
	return f
}
func TestMergeConstantReturnTailsContracts(t *testing.T) {
	f := returnTailFixture()
	if !MergeConstantReturnTails(f) || len(f.Blocks) != 3 || f.Blocks[2].Term.(*TermCondRet).Then != "yes" {
		t.Fatal("tail not redirected")
	}
	if MergeConstantReturnTails(f) {
		t.Fatal("merge not idempotent")
	}
	for _, kind := range []string{"effect", "value", "type", "params", "flag-return", "external-use", "edge-args", "implicit-counter", "hard-class"} {
		t.Run(kind, func(t *testing.T) {
			f := returnTailFixture()
			tail := f.Blocks[3]
			switch kind {
			case "effect":
				tail.Insts = append([]*Inst{{Op: OpStore}}, tail.Insts...)
			case "value":
				tail.Insts[0].Imm = 2
			case "type":
				tail.Insts[0].Ty = TyU16
			case "params":
				tail.Params = []BlockParam{{Dst: 4, Ty: TyU8}}
			case "flag-return":
				f.Contract.Returns[0].Class = ClassFlag
			case "external-use":
				f.Blocks[2].Term.(*TermCondRet).Vals = []Reg{3}
			case "edge-args":
				f.Blocks[2].Term.(*TermCondRet).ThenArgs = []Reg{1}
			case "implicit-counter":
				f.Blocks[2].Term = &TermDJNZ{Counter: 1, Body: "other", Exit: "yes"}
			case "hard-class":
				tail.Insts[0].ClsHard = true
			}
			if MergeConstantReturnTails(f) {
				t.Fatal("invalid tail sharing")
			}
		})
	}
}
