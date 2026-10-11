package mir2

import (
	"reflect"
	"testing"
)

func truncMaskFixture(ty Ty, reverse bool) (*Module, *Func, *Inst, *Inst) {
	m := &Module{}
	f := m.AddFunc("lanes")
	f.Contract.Params = []Param{{Reg: 1, Ty: ty, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}, {Ty: ty, Class: ClassGeneral}}
	andSrc := [2]Reg{1, 2}
	if reverse {
		andSrc = [2]Reg{2, 1}
	}
	mask := &Inst{Op: OpAnd, Dst: 3, Src: andSrc, Ty: ty}
	trunc := &Inst{Op: OpTrunc, Dst: 4, Src: [2]Reg{3}, SrcTy: ty, Ty: TyU8, Cls: ClassAcc, SrcLine: 17, SrcFile: "mask.nanz"}
	f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 2, Imm: 255, Ty: ty}, mask, trunc}, Term: &TermRet{Vals: []Reg{4, 3}}}}
	return m, f, mask, trunc
}
func TestTruncMaskIdentityAllWordValues(t *testing.T) {
	for _, ty := range []Ty{TyU16, TyI16} {
		for _, reverse := range []bool{false, true} {
			m, f, mask, trunc := truncMaskFixture(ty, reverse)
			beforeMask := *mask
			if !SimplifyIdentities(f) || trunc.Src[0] != 1 || trunc.SrcTy != ty || trunc.SrcLine != 17 || trunc.SrcFile != "mask.nanz" || !reflect.DeepEqual(*mask, beforeMask) {
				t.Fatal("pure trunc rewrite/metadata")
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			vm := NewVM(m)
			for input := int64(0); input < 65536; input++ {
				v, e := vm.Call("lanes", []Value{{I: input}})
				if e != nil || len(v) != 2 || v[0].I != input&255 || v[1].I != input&255 {
					t.Fatal("independent lowlane/fullmask oracle", ty, input, v, e)
				}
			}
		}
	}
}
func TestTruncMaskIdentityRefusals(t *testing.T) {
	for _, mode := range []string{"mask254", "mask256", "byte-source", "wide-source", "hard-mask", "hard-trunc", "not-and", "later-constant"} {
		t.Run(mode, func(t *testing.T) {
			m, f, mask, trunc := truncMaskFixture(TyU16, false)
			k := f.Entry().Insts[0]
			switch mode {
			case "mask254":
				k.Imm = 254
			case "mask256":
				k.Imm = 256
			case "byte-source":
				mask.Ty = TyU8
				trunc.SrcTy = TyU8
			case "wide-source":
				mask.Ty = TyU32
				trunc.SrcTy = TyU32
			case "hard-mask":
				mask.ClsHard = true
			case "hard-trunc":
				trunc.ClsHard = true
			case "not-and":
				mask.Op = OpOr
			case "later-constant":
				f.Entry().Insts = []*Inst{mask, trunc, k}
			}
			before := m.Dump()
			if SimplifyIdentities(f) || m.Dump() != before {
				t.Fatal("refused pattern changed")
			}
		})
	}
}

func TestTruncMaskRetainsPartialHeapStorePrefix(t *testing.T) {
	m, f, _, trunc := truncMaskFixture(TyU16, false)
	f.Contract.Params = append(f.Contract.Params, Param{Reg: 5, Ty: TyPtr, Class: ClassPointer})
	f.Contract.Returns = nil
	store := &Inst{Op: OpStore, Src: [2]Reg{5, trunc.Dst}, Ty: TyU8}
	f.Entry().Insts = append(f.Entry().Insts, store, &Inst{Op: OpConst, Dst: 6, Imm: 1, Ty: TyU16}, &Inst{Op: OpPtrAdd, Dst: 7, Src: [2]Reg{5, 6}, Ty: TyPtr}, &Inst{Op: OpStore, Src: [2]Reg{7, trunc.Dst}, Ty: TyU8})
	f.Entry().Term = &TermRet{}
	run := func() []VMDataAccess {
		vm := NewVM(m)
		vm.heap = make([]byte, 4)
		var events []VMDataAccess
		vm.ObserveDataAccess = func(a VMDataAccess) { events = append(events, a) }
		_, e := vm.Call("lanes", []Value{{I: 0x1234}, {I: 3}})
		if e == nil || vm.heap[3] != 0x34 || len(events) != 1 || events[0].Addr != 3 {
			t.Fatal("low-store/high-bounds failure prefix", events, e)
		}
		return events
	}
	before := run()
	if !SimplifyIdentities(f) {
		t.Fatal("rewrite absent")
	}
	DeadStoreElim(f)
	after := run()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("fault prefix changed")
	}
}
