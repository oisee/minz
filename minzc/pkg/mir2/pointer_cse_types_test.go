package mir2

import "testing"

func TestPointerValueKeysPreserveConversionsAndWidths(t *testing.T) {
	f := &Func{Name: "keys", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8}}}, Blocks: []*Block{{Label: "entry", Insts: []*Inst{
		{Op: OpExt, Dst: 2, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyU16},
		{Op: OpSext, Dst: 3, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyU16},
		{Op: OpExt, Dst: 4, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyU32},
		{Op: OpConst, Dst: 5, Imm: 256, Ty: TyU8},
		{Op: OpConst, Dst: 6, Imm: 256, Ty: TyU16},
		{Op: OpConst, Dst: 7, Imm: 0, Ty: TyU8},
	}}}}
	for _, pair := range [][2]Reg{{2, 3}, {2, 4}, {5, 6}} {
		if valueKey(pair[0], f) == valueKey(pair[1], f) {
			t.Fatalf("merged distinct values %v", pair)
		}
	}
	if valueKey(5, f) != valueKey(7, f) {
		t.Fatal("same masked constant must have same key")
	}
}

func TestPointerDedupPreservesSignedOffset(t *testing.T) {
	C := func(r Reg, v int64, ty Ty) *Inst { return &Inst{Op: OpConst, Dst: r, Imm: v, Ty: ty} }
	I := func(op Op, r, a, b Reg, ty Ty) *Inst { return &Inst{Op: op, Dst: r, Src: [2]Reg{a, b}, Ty: ty} }
	mod := &Module{Globals: []Global{{Name: "buf", Ty: TyU8}}}
	zf := mod.AddFunc("ext_cse")
	for j := 0; j < 50; j++ {
		zf.AllocReg()
	}
	zf.Contract.Params = []Param{{Name: "idx", Reg: 1, Ty: TyU8}, {Name: "n", Reg: 2, Ty: TyU8}}
	ze := &Block{Label: "entry", Insts: []*Inst{C(3, 0, TyU8)}, Term: &TermJmp{Target: "head", Args: []Reg{2, 3}}}
	zh := &Block{Label: "head", Params: []BlockParam{{Dst: 10, Ty: TyU8}, {Dst: 15, Ty: TyU8}}, Insts: []*Inst{C(11, 0, TyU8), {Op: OpCmp, Dst: 12, Src: [2]Reg{10, 11}, Cond: CmpNe, Ty: TyBool}}, Term: &TermBrIf{Cond: 12, Then: "body", Else: "exit"}}
	zb := &Block{Label: "body", Insts: []*Inst{{Op: OpAddrOf, Dst: 20, Sym: "buf", Ty: TyPtr}, {Op: OpExt, Dst: 21, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyU16}, {Op: OpSext, Dst: 22, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyU16}, I(OpPtrAdd, 23, 20, 21, TyPtr), I(OpLoad, 24, 23, 0, TyU8), I(OpPtrAdd, 25, 20, 22, TyPtr), I(OpLoad, 26, 25, 0, TyU8), I(OpAdd, 27, 24, 26, TyU8), C(28, 1, TyU8), I(OpSub, 29, 10, 28, TyU8)}, Term: &TermJmp{Target: "head", Args: []Reg{29, 27}}}
	zx := &Block{Label: "exit", Term: &TermRet{Vals: []Reg{15}}}
	zf.Blocks = []*Block{ze, zh, zb, zx}
	check := func() {
		t.Helper()
		if err := Verify(mod); err != nil {
			t.Fatal(err)
		}
		vm := NewVM(mod)
		vm.EnsureHeap(65536)
		vm.WriteHeap(128, 5)
		vm.WriteHeap(65408, 17)
		got, err := vm.Call(zf.Name, []Value{{I: 128}, {I: 1}})
		if err != nil || len(got) != 1 || got[0].I != 22 {
			t.Fatalf("got %v %v, want22", got, err)
		}
	}
	check()
	ApplyPointerWalkDedup(zf, nil)
	check()
	RunGracePasses(zf, nil)
	check()
}
