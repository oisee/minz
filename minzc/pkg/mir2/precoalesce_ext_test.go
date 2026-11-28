package mir2

import "testing"

// join(%p: bool); %d = ext %p : bool -> u8; ret %d. The returned %d is
// A-class, so the single-use %p is too. A second use keeps %p general.
func TestPropagateClassHintsThroughByteExt(t *testing.T) {
	for _, extraUse := range []bool{false, true} {
		ext := &Inst{Op: OpExt, Dst: 2, Src: [2]Reg{1}, Ty: TyU8, SrcTy: TyBool, Cls: ClassGeneral}
		join := &Block{Label: "join", Params: []BlockParam{{Dst: 1, Ty: TyBool, Class: ClassGeneral}}, Insts: []*Inst{ext}, Term: &TermRet{Vals: []Reg{2}}}
		if extraUse {
			join.Insts = append(join.Insts, &Inst{Op: OpMove, Dst: 3, Src: [2]Reg{1}, Ty: TyBool})
		}
		entry := &Block{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 4, Ty: TyBool, Imm: 0}}, Term: &TermJmp{Target: "join", Args: []Reg{4}}}
		f := &Func{Name: "f", Contract: Contract{Returns: []Return{{Ty: TyU8}}}, Blocks: []*Block{entry, join}}
		info := collectRegInfo(f)
		propagateClassHints(f, info)
		want := ClassAcc
		if extraUse {
			want = ClassGeneral
		}
		if info[1].Cls != want || info[2].Cls != ClassAcc {
			t.Fatalf("extra use %t: param class %v (want %v), ext class %v", extraUse, info[1].Cls, want, info[2].Cls)
		}
		if !extraUse && info[4].Cls != ClassAcc {
			t.Fatalf("edge constant not A-class: %v", info[4].Cls)
		}
	}
}
