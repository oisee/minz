package mir2

import "testing"

// ReassociatePointerOffsets leaves the shapes later folds rely on alone: a
// constant term (ptrAddConstantFolds), a link-time base (symbol immediate),
// a sum with another use, and a base defined after the sum.
func TestReassociatePointerOffsetsRefusals(t *testing.T) {
	build := func(kind string) *Func {
		f := (&Module{}).AddFunc("r")
		b := f.NewBlock("entry")
		f.Contract.Params = []Param{{Reg: 1, Ty: TyPtr}, {Reg: 2, Ty: TyU16}, {Reg: 3, Ty: TyU16}}
		add := &Inst{Op: OpAdd, Dst: 4, Src: [2]Reg{2, 3}, Ty: TyU16}
		pa := &Inst{Op: OpPtrAdd, Dst: 5, Src: [2]Reg{1, 4}, Ty: TyPtr}
		load := &Inst{Op: OpLoad, Dst: 6, Src: [2]Reg{5}, Ty: TyU8}
		b.Insts = []*Inst{add, pa, load}
		b.Term = &TermRet{Vals: []Reg{6}}
		switch kind {
		case "constant":
			b.Insts = append([]*Inst{{Op: OpConst, Dst: 7, Imm: 3, Ty: TyU16}}, b.Insts...)
			add.Src[1] = 7
		case "addr_of":
			b.Insts = append([]*Inst{{Op: OpAddrOf, Dst: 7, Sym: "g", Ty: TyPtr}}, b.Insts...)
			pa.Src[0] = 7
		case "shared":
			b.Term = &TermRet{Vals: []Reg{6, 4}}
		// The add's flags may be observed (wordMaskFlagsUnobserved): an
		// error ABI returns in CY, and a carry compare reads the flags of a
		// preceding subtraction.  The rewrite drops the add's own flag
		// result, so it must refuse in such functions.
		// The base stays live after the ptr_add, so the first step could not
		// take its home.
		case "base_live":
			b.Term = &TermRet{Vals: []Reg{6, 1}}
		case "fallible":
			f.Attrs.FallibleCY = true
		case "sub_carry":
			b.Insts = append(b.Insts, &Inst{Op: OpCmp, Dst: 8, Src: [2]Reg{2, 3}, Cond: CmpSubCarry, Ty: TyBool})
		case "late_base":
			b.Insts = []*Inst{add, {Op: OpPtrAdd, Dst: 7, Src: [2]Reg{1, 2}, Ty: TyPtr}, pa, load}
			pa.Src[0] = 7
		}
		return f
	}
	if n := ReassociatePointerOffsets(build("plain")); n != 1 {
		t.Fatalf("plain sum not reassociated: %d", n)
	}
	for _, kind := range []string{"constant", "addr_of", "shared", "late_base", "fallible", "sub_carry", "base_live"} {
		if n := ReassociatePointerOffsets(build(kind)); n != 0 {
			t.Fatalf("%s reassociated", kind)
		}
	}
	f := build("plain")
	f.Blocks[0].Insts[1].InBounds = true
	ReassociatePointerOffsets(f)
	if err := verifyFunc(f); err != nil {
		t.Fatal(err)
	}
	in := f.Blocks[0].Insts
	if in[0].Op != OpPtrAdd || in[0].Src != [2]Reg{1, 2} || in[1].Src != [2]Reg{4, 3} {
		t.Fatalf("unexpected rewrite: %+v %+v", in[0], in[1])
	}
	if in[0].InBounds || in[1].InBounds {
		t.Fatal("InBounds kept on a rewritten ptr_add")
	}
}
