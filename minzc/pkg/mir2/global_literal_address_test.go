package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestGlobalLiteralAddressPairsNative(t *testing.T) {
	for _, pair := range []string{"HL", "DE", "BC", "IX", "IY"} {
		for _, offset := range []int64{0, 1, 255, 256, 511, 65535} {
			t.Run(fmt.Sprintf("%s-offset%d", pair, offset), func(t *testing.T) {
				f := &mir2.Func{Name: "address", Contract: mir2.Contract{Returns: []mir2.Return{{Ty: mir2.TyPtr, Class: mir2.ClassPointer}}}, Blocks: []*mir2.Block{{Label: "entry", Insts: []*mir2.Inst{
					{Op: mir2.OpAddrOf, Dst: 1, Sym: "data", Ty: mir2.TyPtr},
					{Op: mir2.OpField, Dst: 2, Src: [2]mir2.Reg{1}, Imm: offset, Ty: mir2.TyPtr},
				}, Term: &mir2.TermRet{Vals: []mir2.Reg{2}}}}}
				m := &mir2.Module{Name: "literal-global", Funcs: []*mir2.Func{f}, Globals: []mir2.Global{{Name: "data", Ty: mir2.NewArray(mir2.TyU8, 512)}}}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{1: {Kind: mir2.LocReg, Name: "HL"}, 2: {Kind: mir2.LocReg, Name: pair}}}
				text, err := mir2.Z80Codegen(m, ar)
				if err != nil {
					t.Fatal(err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
				if err != nil || len(a.Errors) != 0 {
					t.Fatal(err, a.Errors)
				}
				z := emulator.NewRemogattoZ80()
				if err = z.LoadMemory(0x8000, a.Binary); err != nil {
					t.Fatal(err)
				}
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["address"]), SP: 0xeff0, A: 0x6c, F: 0xb5, HL: 0x60ff, DE: 0x7213, BC: 0x8456, IX: 0x9127, IY: 0xab38})
				before := z.GetRegisters()
				ram := append([]byte(nil), z.Memory()...)
				cycles := z.Step()
				after := z.GetRegisters()
				actual := map[string]uint16{"HL": after.HL, "DE": after.DE, "BC": after.BC, "IX": after.IX, "IY": after.IY}
				prior := map[string]uint16{"HL": before.HL, "DE": before.DE, "BC": before.BC, "IX": before.IX, "IY": before.IY}
				for name, value := range actual {
					want := prior[name]
					if name == pair {
						want = uint16(int64(a.Symbols["data"]) + offset)
					}
					if value != want {
						t.Fatalf("pair %s got%04x want%04x\n%s", name, value, want, text)
					}
				}
				width, expectedCycles := uint16(3), 10
				if pair == "IX" || pair == "IY" {
					width, expectedCycles = 4, 14
				}
				if after.A != before.A || after.F != before.F || after.SP != before.SP || after.PC != before.PC+width || cycles != expectedCycles {
					t.Fatal("address instruction clobbered state", before, after, cycles)
				}
				for i, b := range ram {
					if z.Memory()[i] != b {
						t.Fatal("address instruction wrote RAM", i)
					}
				}
			})
		}
	}
}

func TestGlobalLiteralAddressSharedSuccessorNative(t *testing.T) {
	f := &mir2.Func{Name: "read", Contract: mir2.Contract{Returns: []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}}, Blocks: []*mir2.Block{
		{Label: "entry", Insts: []*mir2.Inst{{Op: mir2.OpAddrOf, Dst: 1, Sym: "data", Ty: mir2.TyPtr}, {Op: mir2.OpField, Dst: 2, Src: [2]mir2.Reg{1}, Imm: 17, Ty: mir2.TyPtr}, {Op: mir2.OpLoad, Dst: 3, Src: [2]mir2.Reg{2}, Ty: mir2.TyU8}}, Term: &mir2.TermJmp{Target: "next"}},
		{Label: "next", Insts: []*mir2.Inst{{Op: mir2.OpLoad, Dst: 4, Src: [2]mir2.Reg{1}, Ty: mir2.TyU8}}, Term: &mir2.TermRet{Vals: []mir2.Reg{4}}},
	}}
	m := &mir2.Module{Name: "shared-global", Funcs: []*mir2.Func{f}, Globals: []mir2.Global{{Name: "data", Ty: mir2.NewArray(mir2.TyU8, 32), Init: []byte{0x73}}}}
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{1: {Kind: mir2.LocReg, Name: "HL"}, 2: {Kind: mir2.LocReg, Name: "DE"}, 3: {Kind: mir2.LocReg, Name: "C"}, 4: {Kind: mir2.LocReg, Name: "A"}}}
	text, err := mir2.Z80Codegen(m, ar)
	if err != nil || !strings.Contains(text, "LD HL, data") {
		t.Fatal("shared base was suppressed", err, text)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
	if err != nil || len(a.Errors) != 0 {
		t.Fatal(err, a.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err = z.LoadMemory(0x8000, a.Binary); err != nil {
		t.Fatal(err)
	}
	z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
	z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["read"]), SP: 0xeff0})
	for i := 0; i < 100; i++ {
		if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
			if z.GetRegisters().A != 0x73 {
				t.Fatal("successor lost global base", z.GetRegisters())
			}
			return
		}
		z.Step()
	}
	t.Fatal("native return", z.GetRegisters())
}
