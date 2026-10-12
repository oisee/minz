package mir2

import "testing"

func cpmNarrowFixture() (*Module, *AllocResult) {
	m := &Module{}
	f := m.AddFunc("main")
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpAlloca, Dst: 1, Ty: TyPtr, Imm: 2, Cls: ClassPointer}, {Op: OpMove, Dst: 2, Src: [2]Reg{1}, Ty: TyU8, Cls: ClassGeneral}, {Op: OpConst, Dst: 3, Ty: TyU8, Imm: 42, Cls: ClassAcc}, {Op: OpStore, Src: [2]Reg{2, 3}, Ty: TyU8}}
	b.Term = &TermRet{}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "HL"}, 2: {Kind: LocMem, Name: "_spill_main_r2"}, 3: {Kind: LocReg, Name: "A"}}}
	return m, ar
}
func TestCPMRejectsNarrowedLocalPointerNativePlacement(t *testing.T) {
	m, ar := cpmNarrowFixture()
	if e := Verify(m); e != nil {
		t.Fatal(e)
	}
	text, e := Z80Codegen(m, ar, Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: 1024})
	if e == nil || text != "" {
		t.Fatal("narrowed local address admitted", e)
	}
}
func TestCPMLocalAddressTypedAuthority(t *testing.T) {
	for _, kind := range []string{"narrow-move", "field-type", "address-metadata", "constant-metadata", "bool-offset"} {
		t.Run(kind, func(t *testing.T) {
			m, _ := cpmNarrowFixture()
			f := m.Funcs[0]
			f.Entry().Insts[1].Ty = TyPtr
			switch kind {
			case "narrow-move":
				f.Entry().Insts[1].Ty = TyU8
			case "field-type":
				f.Entry().Insts[1].Op = OpField
				f.Entry().Insts[1].Ty = TyU8
			case "address-metadata":
				f.Entry().Insts[1].SequentialBytes = true
			case "constant-metadata", "bool-offset":
				f.Entry().Insts[1].Op = OpPtrAdd
				f.Entry().Insts[1].Src[1] = 4
				c := &Inst{Op: OpConst, Dst: 4, Ty: TyU8, Imm: 1}
				if kind == "constant-metadata" {
					c.SequentialBytes = true
				} else {
					c.Ty = TyBool
				}
				f.Entry().Insts = append([]*Inst{f.Entry().Insts[0], c}, f.Entry().Insts[1:]...)
			}
			locals, e := cpmLocalAddresses(f)
			if e != nil {
				t.Fatal(e)
			}
			if _, ok := locals[2]; ok {
				t.Fatal("invalid authority")
			}
		})
	}
	m, _ := cpmNarrowFixture()
	f := m.Funcs[0]
	f.Entry().Insts[1].Ty = TyPtr
	locals, e := cpmLocalAddresses(f)
	if e != nil || locals[2].size != 2 {
		t.Fatal("valid move", locals, e)
	}
	c := &Inst{Op: OpConst, Dst: 4, Ty: TyU8, Imm: 257}
	f.Entry().Insts[1].Op = OpPtrAdd
	f.Entry().Insts[1].Src[1] = 4
	f.Entry().Insts = append([]*Inst{f.Entry().Insts[0], c}, f.Entry().Insts[1:]...)
	locals, e = cpmLocalAddresses(f)
	if e != nil || locals[2].offset != 1 {
		t.Fatal("typed stored offset", locals, e)
	}
}
func TestCPMReadOnlyPointerCannotNarrow(t *testing.T) {
	f := &Func{Contract: Contract{Params: []Param{{Reg: 1, Ty: TyPtr}}}, Blocks: []*Block{{Label: "entry", Insts: []*Inst{{Op: OpMove, Dst: 2, Src: [2]Reg{1}, Ty: TyU8}, {Op: OpLoad, Dst: 3, Src: [2]Reg{2}, Ty: TyU8}}, Term: &TermRet{Vals: []Reg{3}}}}}
	if cpmReadOnlyPointerParam(f, 0, 2) {
		t.Fatal("narrowed readonly pointer admitted")
	}
	f.Entry().Insts[0].Ty = TyPtr
	if !cpmReadOnlyPointerParam(f, 0, 2) {
		t.Fatal("valid readonly pointer rejected")
	}
}
