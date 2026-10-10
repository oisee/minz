package mir2_test

import (
	m "github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestGracePreservesNarrowIndexWrap(t *testing.T) {
	C := func(r m.Reg, v int64, ty m.Ty) *m.Inst { return &m.Inst{Op: m.OpConst, Dst: r, Imm: v, Ty: ty} }
	I := func(op m.Op, r, a, b m.Reg, ty m.Ty) *m.Inst {
		return &m.Inst{Op: op, Dst: r, Src: [2]m.Reg{a, b}, Ty: ty}
	}
	mod := &m.Module{Globals: []m.Global{{Name: "buf", Ty: m.TyU8}}}
	f := mod.AddFunc("narrow_wrap")
	for j := 0; j < 50; j++ {
		f.AllocReg()
	}
	entry := &m.Block{Label: "entry", Insts: []*m.Inst{C(1, 255, m.TyU8), C(2, 2, m.TyU8), C(3, 0, m.TyU16)}, Term: &m.TermJmp{Target: "head", Args: []m.Reg{1, 2, 3}}}
	head := &m.Block{Label: "head", Params: []m.BlockParam{{Dst: 10, Ty: m.TyU8}, {Dst: 11, Ty: m.TyU8}, {Dst: 12, Ty: m.TyU16}}, Insts: []*m.Inst{C(13, 0, m.TyU8), {Op: m.OpCmp, Dst: 14, Src: [2]m.Reg{11, 13}, Cond: m.CmpNe, Ty: m.TyBool, SrcTy: m.TyU8}}, Term: &m.TermBrIf{Cond: 14, Then: "body", Else: "exit"}}
	body := &m.Block{Label: "body", Insts: []*m.Inst{{Op: m.OpAddrOf, Dst: 20, Sym: "buf", Ty: m.TyPtr}, {Op: m.OpExt, Dst: 21, Src: [2]m.Reg{10}, SrcTy: m.TyU8, Ty: m.TyU16}, I(m.OpPtrAdd, 22, 20, 21, m.TyPtr), I(m.OpLoad, 23, 22, 0, m.TyU8), C(24, 1, m.TyU8), I(m.OpAdd, 25, 10, 24, m.TyU8), I(m.OpSub, 26, 11, 24, m.TyU8), I(m.OpAdd, 27, 12, 23, m.TyU16)}, Term: &m.TermJmp{Target: "head", Args: []m.Reg{25, 26, 27}}}
	exit := &m.Block{Label: "exit", Term: &m.TermRet{Vals: []m.Reg{12}}}
	f.Blocks = []*m.Block{entry, head, body, exit}

	check := func() {
		t.Helper()
		if err := m.Verify(mod); err != nil {
			t.Fatal(err)
		}
		vm := m.NewVM(mod)
		vm.EnsureHeap(1024)
		vm.WriteHeap(0, 3)
		vm.WriteHeap(255, 5)
		vm.WriteHeap(256, 17)
		got, err := vm.Call(f.Name, nil)
		if err != nil || len(got) != 1 || got[0].I != 8 {
			t.Fatalf("got %v, %v; want 8", got, err)
		}
	}
	check()
	stats := &m.GraceStats{ByRule: map[string]int{}}
	m.RunGracePasses(f, stats)
	check()
	if stats.ByRule["ptr-threading"] != 0 {
		t.Fatal("unsafe pointer threading ran")
	}
}
