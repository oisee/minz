package mir2_test

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestZ80ConstantShiftDyingPairExchange(t *testing.T) {
	for _, mode := range []int{0, 1, 2, 3} {
		live := mode != 0
		for _, source := range []string{"DE", "HL"} {
			dest := "HL"
			if source == "HL" {
				dest = "DE"
			}
			m := &mir2.Module{Name: "exchange"}
			f := m.AddFunc("shift")
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			x := b.Param("x", mir2.TyU16, mir2.ClassPair)
			n := b.Const(4, mir2.TyU16, mir2.ClassGeneral)
			shift := b.BinOp(mir2.OpShl, x, n, mir2.TyU16, mir2.ClassPair)
			ptr, keep := mir2.NoReg, mir2.NoReg
			if live {
				ptr = b.Const(0x6000, mir2.TyPtr, mir2.ClassPointer)
				if mode == 1 {
					b.Store(ptr, x, mir2.TyU16)
				} else {
					keep = b.Param("keep", mir2.TyU8, mir2.ClassGeneral)
					b.Store(ptr, keep, mir2.TyU8)
				}
			}
			b.Ret(shift)
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
			a := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: source}, n: {Kind: mir2.LocReg, Name: "BC"}, shift: {Kind: mir2.LocReg, Name: dest}}}
			if live {
				a.Locs[ptr] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "IX"}
				if mode >= 2 {
					half := "D"
					if source == "HL" {
						half = "H"
					}
					if mode == 3 {
						half = "E"
						if source == "HL" {
							half = "L"
						}
					}
					a.Locs[keep] = mir2.PhysLoc{Kind: mir2.LocReg, Name: half}
				}
			}
			asm := mustZ80Asm(mir2.Z80Codegen(m, a))
			if !live && !strings.Contains(asm, "EX DE, HL") {
				t.Fatal("dying source not exchanged", asm)
			}
			binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
			if err != nil || len(binary.Errors) != 0 {
				t.Fatal(err, binary.Errors)
			}
			z := emulator.NewRemogattoZ80()
			if err = z.LoadMemory(0x8000, binary.Binary); err != nil {
				t.Fatal(err)
			}
			if err = z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
				t.Fatal(err)
			}
			for xval := 0; xval < 65536; xval++ {
				r := emulator.Registers{DE: 0x1234, HL: 0x5678, PC: uint16(binary.Symbols["shift"]), SP: 0xeffe}
				if source == "DE" {
					r.DE = uint16(xval)
				} else {
					r.HL = uint16(xval)
				}
				z.SetRegisters(r)
				for steps := 0; steps < 1024 && z.GetPC() != 0xf100; steps++ {
					z.Step()
				}
				got := z.GetRegisters()
				if got.HL != uint16(xval<<4) || got.PC != 0xf100 || got.SP != 0xf000 {
					t.Fatal(source, live, xval, got, asm)
				}
				if mode == 1 && (uint16(z.Memory()[0x6000])|uint16(z.Memory()[0x6001])<<8) != uint16(xval) {
					t.Fatal("live source lost", source, xval)
				}
				if mode >= 2 {
					want := byte(xval)
					if mode == 2 {
						want = byte(xval >> 8)
					}
					if z.Memory()[0x6000] != want {
						t.Fatal("live half lost", source, mode, xval)
					}
				}
			}
			t.Logf("%s→%s mode=%d all65536 PASS", source, dest, mode)
		}
	}
}
