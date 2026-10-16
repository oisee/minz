package mir2

import "testing"

func absCFGFixture(cond CmpCond) (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("renamed_difference")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassAcc}, {Reg: 2, Ty: TyU8, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	then, els := "forward", "reverse"
	if cond == CmpUlt || cond == CmpUle {
		then, els = els, then
	}
	f.Blocks = []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpCmp, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyBool, SrcTy: TyU8, Cond: cond, Cls: ClassFlag}}, Term: &TermBrIf{Cond: 3, Then: then, Else: els}},
		{Label: "forward", Insts: []*Inst{{Op: OpSub, Dst: 4, Src: [2]Reg{1, 2}, Ty: TyU8, Cls: ClassGeneral, SrcFile: "sample.c", SrcLine: 5}}, Term: &TermRet{Vals: []Reg{4}}},
		{Label: "reverse", Insts: []*Inst{{Op: OpSub, Dst: 5, Src: [2]Reg{2, 1}, Ty: TyU8, Cls: ClassGeneral, SrcFile: "sample.c", SrcLine: 7}}, Term: &TermRet{Vals: []Reg{5}}},
	}
	return m, f
}

func TestByteAbsDiffCFGAllPairs(t *testing.T) {
	for _, cond := range []CmpCond{CmpUgt, CmpUge, CmpUlt, CmpUle} {
		before, _ := absCFGFixture(cond)
		m, f := absCFGFixture(cond)
		if !FuseByteAbsDiffCFG(f) || len(f.Blocks) != 2 || f.Blocks[1].Insts[0].Op != OpNeg || f.Blocks[1].Insts[0].SrcLine != 7 || f.Blocks[0].Insts[0].SrcLine != 5 {
			t.Fatal("rewrite or provenance", cond)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		v0, v1 := NewVM(before), NewVM(m)
		for x := 0; x < 256; x++ {
			for y := 0; y < 256; y++ {
				want := x - y
				if want < 0 {
					want = -want
				}
				for _, v := range []*VM{v0, v1} {
					got, err := v.Call("renamed_difference", []Value{{I: int64(x)}, {I: int64(y)}})
					if err != nil || len(got) != 1 || got[0].I != int64(want) {
						t.Fatal(cond, x, y, got, err)
					}
				}
			}
		}
	}
}

func TestByteAbsDiffCFGRefusals(t *testing.T) {
	for _, kind := range []string{"signed", "word", "hard-param", "hard-sub", "hard-cmp", "shared-branch", "shared-cond", "redefined-source", "redefined-result", "effects", "asm", "wrong-operands", "multi-return", "fallible", "flag-return", "origin-conflict", "edge-args", "block-params"} {
		t.Run(kind, func(t *testing.T) {
			m, f := absCFGFixture(CmpUgt)
			head, forward := f.Blocks[0], f.Blocks[1]
			switch kind {
			case "signed":
				head.Insts[0].Cond = CmpGt
				head.Insts[0].SrcTy = TyI8
			case "word":
				f.Contract.Params[0].Ty = TyU16
			case "hard-param":
				f.Contract.Params[0].ClassHard = true
			case "hard-sub":
				forward.Insts[0].ClsHard = true
			case "hard-cmp":
				head.Insts[0].ClsHard = true
			case "shared-branch":
				f.Blocks = append(f.Blocks, &Block{Label: "other", Term: &TermJmp{Target: "forward"}})
			case "shared-cond":
				f.Blocks = append(f.Blocks, &Block{Label: "other", Term: &TermRet{Vals: []Reg{3}}})
			case "redefined-source":
				head.Insts = append([]*Inst{{Op: OpConst, Dst: 1, Ty: TyU8}}, head.Insts...)
			case "redefined-result":
				f.Blocks = append(f.Blocks, &Block{Label: "other", Insts: []*Inst{{Op: OpConst, Dst: 4, Ty: TyU8}}, Term: &TermRet{}})
			case "effects":
				forward.Insts = append(forward.Insts, &Inst{Op: OpStore, Src: [2]Reg{2, 1}, Ty: TyU8})
			case "asm":
				head.Insts = append([]*Inst{{Op: OpAsm}}, head.Insts...)
			case "wrong-operands":
				forward.Insts[0].Src = [2]Reg{2, 1}
			case "multi-return":
				f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU8})
			case "fallible":
				f.Attrs.FallibleCY = true
			case "flag-return":
				f.Contract.Returns[0].Class = ClassFlag
			case "origin-conflict":
				f.LoopOrigins = map[string]LoopOrigin{"entry": {SiteID: "a"}, "forward": {SiteID: "b"}}
			case "edge-args":
				head.Term.(*TermBrIf).ThenArgs = []Reg{1}
			case "block-params":
				forward.Params = []BlockParam{{Dst: 7, Ty: TyU8}}
			}
			before := m.Dump()
			if FuseByteAbsDiffCFG(f) || m.Dump() != before {
				t.Fatal("unsafe or partial rewrite", kind)
			}
		})
	}
}
