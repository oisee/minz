package pipeline

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"testing"
)

// Original source and final MIR remain intact; only valid DWord homes are pinned.
func TestWideShiftSourcePhysicalPlacement(t *testing.T) {
	const source = "fun shift(a:u32)->u32{return a<<1}"
	for _, pair := range []string{"HL", "DE", "BC"} {
		h, e := nanz.Parse(source, "wide_shift_placement.nanz")
		if e != nil {
			t.Fatal(e)
		}
		s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		f := s.MIR2Module.FuncByName("shift")
		var shift *mir2.Inst
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == mir2.OpShl {
					shift = i
				}
			}
		}
		if shift == nil {
			t.Fatal("shift absent")
		}
		s.Allocation.Locs[shift.Src[0]] = mir2.PhysLoc{Kind: mir2.LocDWord, Name: pair}
		s.Allocation.Locs[shift.Dst] = mir2.PhysLoc{Kind: mir2.LocDWord, Name: pair}
		asm, e := mir2.Z80Codegen(s.MIR2Module, s.Allocation)
		if e != nil {
			t.Fatal(e)
		}
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if e != nil {
			t.Fatal(e)
		}
		if len(bin.Errors) > 0 {
			t.Fatal(bin.Errors)
		}
		vm := mir2.NewVM(s.MIR2Module)
		for _, v := range []uint32{0, 1, 0xffff, 0x10000, 0x80000000, 0xffffffff, 0x12345678} {
			want := v << 1
			out, e := vm.Call("shift", []mir2.Value{{I: int64(v)}})
			if e != nil || uint32(out[0].I) != want {
				t.Fatal("VM", out, e)
			}
			boot, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0xf100\nLD HL,0x1357\nLD DE,0x2468\nLD BC,0x3579\nEXX\nLD HL,0xabcd\nLD DE,0xbcde\nLD BC,0xcdef\nEXX\nLD %s,%d\nEXX\nLD %s,%d\nEXX\nCALL %d\nLD (0x6100),HL\nEXX\nLD (0x6102),HL\nEXX\nDI\nHALT\n", pair, v&65535, pair, v>>16, bin.Symbols["shift"]))
			if e != nil {
				t.Fatal(e)
			}
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, bin.Binary)
			z.LoadMemory(0xf100, boot.Binary)
			z.SetRegisters(emulator.Registers{PC: 0xf100, SP: 0xeff0})
			for n := 0; !z.IsHalted() && n < 1000; n++ {
				z.Step()
			}
			got := binary.LittleEndian.Uint32(z.Memory()[0x6100:0x6104])
			if got != want || z.GetSP() != 0xeff0 || !z.IsHalted() {
				t.Fatalf("source placed %s a=%08x got=%08x want=%08x\n%s", pair, v, got, want, asm)
			}
		}
		if dir := os.Getenv("WIDE_SHIFT_EXPORT"); dir != "" {
			os.MkdirAll(dir, 0755)
			os.WriteFile(dir+"/"+pair+".nanz", []byte(source), 0644)
			os.WriteFile(dir+"/"+pair+".a80", []byte(asm), 0644)
			os.WriteFile(dir+"/"+pair+".bin", bin.Binary, 0644)
		}
	}
}
