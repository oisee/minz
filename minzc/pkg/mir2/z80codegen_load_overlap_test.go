package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Execute machine code: overwriting the address low byte can look correct
// when the loaded low byte happens to match it. Include page crossings.
func TestZ80WordLoadAddressOverlap(t *testing.T) {
	pairs := []string{"IX", "IY", "HL", "DE", "BC"}
	for _, ptrLoc := range pairs {
		for _, dstLoc := range pairs {
			for _, tc := range []struct{ addr, value uint16 }{{0xC000, 0xB712}, {0xC03D, 0x4AE9}, {0xD0FF, 0x9031}} {
				t.Run(fmt.Sprintf("%s_to_%s/%04x_%04x", ptrLoc, dstLoc, tc.addr, tc.value), func(t *testing.T) {
					m := &mir2.Module{Name: "word_load_overlap"}
					f := m.AddFunc("load_word")
					f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
					b := mir2.NewBuilder(f)
					b.SwitchToNewBlock("entry")
					ptr := b.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
					acc := b.Param("acc", mir2.TyU8, mir2.ClassAcc)
					out := b.Load(ptr, mir2.TyU16, mir2.ClassPair)
					// Keep A live and make it observable in memory after the load.
					byteOut := b.AddrOf("acc_out", mir2.ClassPointer)
					b.Store(byteOut, acc, mir2.TyU8)
					b.Ret(out)
					m.AddGlobal(mir2.Global{Name: "acc_out", Ty: mir2.TyU8})
					outputLoc := "IY"
					if dstLoc == "IY" {
						outputLoc = "IX"
					}
					ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
						ptr: {Kind: mir2.LocReg, Name: ptrLoc}, acc: {Kind: mir2.LocReg, Name: "A"},
						out: {Kind: mir2.LocReg, Name: dstLoc}, byteOut: {Kind: mir2.LocReg, Name: outputLoc},
					}}
					asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD %s,%d\nLD A,0x65\nSCF\nCALL load_word\nLD (0xE000),HL\nDI\nHALT\n", ptrLoc, tc.addr) + mustZ80Asm(mir2.Z80Codegen(m, ar))
					res, err := z80asm.NewAssembler().AssembleString(asm)
					if err != nil || len(res.Errors) != 0 {
						t.Fatalf("assemble: %v %v\n%s", err, res.Errors, asm)
					}
					z := emulator.NewRemogattoZ80()
					z.Reset()
					if err := z.LoadMemory(0x8000, res.Binary); err != nil {
						t.Fatal(err)
					}
					// Poison distinguishes reads through a corrupted address.
					for a := 0xC000; a < 0xD200; a++ {
						z.SetMemory(uint16(a), 0xDA)
					}
					z.SetMemory(tc.addr, byte(tc.value))
					z.SetMemory(tc.addr+1, byte(tc.value>>8))
					z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
					for n := 0; !z.IsHalted(); n++ {
						if n >= 10000 {
							t.Fatalf("no HALT\n%s", asm)
						}
						z.Step()
					}
					r := z.GetRegisters()
					if r.HL != tc.value || r.A != 0x65 || r.F&1 == 0 || r.SP != 0xFF00 {
						t.Fatalf("return/scratch: HL=%04x A=%02x F=%02x SP=%04x want HL=%04x A=65 carry=1 SP=ff00\n%s", r.HL, r.A, r.F, r.SP, tc.value, asm)
					}
					if got := uint16(z.GetMemory(0xE000)) | uint16(z.GetMemory(0xE001))<<8; got != tc.value {
						t.Fatalf("stored return=%04x want %04x", got, tc.value)
					}
					if got := z.GetMemory(uint16(res.Symbols["acc_out"])); got != 0x65 {
						t.Fatalf("live accumulator store=%02x want 65\n%s", got, asm)
					}
					for a := 0xC000; a < 0xD200; a++ {
						want := byte(0xDA)
						if uint16(a) == tc.addr {
							want = byte(tc.value)
						}
						if uint16(a) == tc.addr+1 {
							want = byte(tc.value >> 8)
						}
						if got := z.GetMemory(uint16(a)); got != want {
							t.Fatalf("source memory[%04x]=%02x want %02x", a, got, want)
						}
					}
				})
			}
		}
	}
}
