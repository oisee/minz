package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestTypedIndirectWordNative(t *testing.T) {
	src := `type WordTransform=fun(u16)->u16 fun plus37(x:u16)->u16{return x+37} fun apply_word(f:WordTransform,x:u16)->u16{return f(x)} fun keep_address()->u16{return apply_word(plus37,9)}`
	s, e := CompileHIRSteps(graceMeasureParse(t, src), Options{ContractOpt: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	bin, e := z80asm.NewAssembler().AssembleString(s.Assembly)
	if e != nil || len(bin.Errors) > 0 {
		t.Fatal(e, bin.Errors)
	}
	symbol := func(n string) int {
		v := bin.Symbols[n]
		if v == 0 {
			v = bin.Symbols[strings.ToUpper(n)]
		}
		return v
	}
	f := s.MIR2Module.FuncByName("apply_word")
	p0 := s.Allocation.Locs[f.Contract.Params[0].Reg].Name
	p1 := s.Allocation.Locs[f.Contract.Params[1].Reg].Name
	for _, x := range []uint16{0, 1, 255, 256, 32767, 32768, 65498, 65500, 65535} {
		boot, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("LD %s,%d\nLD %s,%d\nCALL %d\nDI\nHALT\n", p0, symbol("plus37"), p1, x, symbol("apply_word")))
		if e != nil || len(boot.Errors) > 0 {
			t.Fatal(e, boot.Errors)
		}
		z := emulator.NewRemogattoZ80()
		z.MaxCycles = 10000
		z.LoadMemory(uint16(bin.Origin), bin.Binary)
		z.LoadMemory(0xf100, boot.Binary)
		z.SetPC(0xf100)
		z.SetSP(0xf000)
		if e = z.Run(); e != nil {
			t.Fatal(e)
		}
		r := z.GetRegisters()
		if r.HL != x+37 || r.SP != 0xf000 {
			t.Fatalf("x%d got%04x SP%04x ASM%s", x, r.HL, r.SP, s.Assembly)
		}
	}
}
