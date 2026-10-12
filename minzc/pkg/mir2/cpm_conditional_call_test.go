package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestCPMDirectConditionalCallAdmissionNative(t *testing.T) {
	for _, taken := range []bool{false, true} {
		m := &Module{Globals: []Global{{Name: "value", Ty: TyU8}}}
		f := m.AddFunc("main")
		b := f.NewBlock("entry")
		v := int64(1)
		if taken {
			v = 0
		}
		b.Insts = []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: v, Cls: ClassAcc}, {Op: OpConst, Dst: 2, Ty: TyU8, Imm: 0, Cls: ClassGeneral}, {Op: OpCmp, Dst: 3, Ty: TyBool, Src: [2]Reg{1, 2}, Cond: CmpEq, Cls: ClassFlag}, {Op: OpCallCond, Sym: "helper", Cond: CmpEq, Src: [2]Reg{3}}}
		b.Term = &TermRet{}
		h := m.AddFunc("helper")
		hb := h.NewBlock("entry")
		hb.Insts = []*Inst{{Op: OpAddrOf, Dst: 4, Ty: TyPtr, Sym: "value", Cls: ClassPointer}, {Op: OpConst, Dst: 5, Ty: TyU8, Imm: 42, Cls: ClassAcc}, {Op: OpStore, Ty: TyU8, Src: [2]Reg{4, 5}}}
		hb.Term = &TermRet{}
		ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "A"}, 2: {Kind: LocReg, Name: "B"}, 3: {Kind: LocFlag, Name: "F"}, 4: {Kind: LocReg, Name: "HL"}, 5: {Kind: LocReg, Name: "A"}}}
		text, e := Z80Codegen(m, ar, Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: 128})
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString(text)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		z.LoadMemory(0x100, a.Binary)
		z.LoadMemory(0x7000, []byte{0xcd, 0, 1, 0})
		z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
		for n := 0; n < 10000 && z.GetPC() != 0x7003; n++ {
			z.Step()
		}
		want := byte(0)
		if taken {
			want = 42
		}
		if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 || z.Memory()[a.Symbols["value"]] != want {
			t.Fatal(taken, z.GetRegisters(), z.Memory()[a.Symbols["value"]])
		}
		b.Insts[3].Sym = "missing"
		if text, e := Z80Codegen(m, ar, Z80CodegenOptions{Runtime: Z80RuntimeCPM, RuntimeStackBytes: 128}); e == nil || text != "" {
			t.Fatal("unknown conditional callee admitted")
		}
	}
}
