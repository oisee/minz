package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestSignedWordLiteralLeftKeepsArithmeticWidth(t *testing.T) {
	const text = `fun abs_word(x:i16)->u16 {if x<0{return 0-x}
return x}
fun add_word(x:i16)->u16{return 1+x}
fun subtract_word(x:i16)->u16{return 7-x}
fun same_bits(x:u16)->u16{return 0+x}`
	source, e := nanz.Parse(text, "literal-width.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, UseGrace: true, AssertMode: "none"}, {ContractOpt: true, UseGrace: true, OptSize: true, AssertMode: "none"}} {
		m, e := nanz.Parse(text, "literal-width.nanz")
		if e != nil {
			t.Fatal(e)
		}
		steps, e := CompileHIRSteps(m, mode)
		if e != nil {
			t.Fatal(e)
		}
		program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil || len(program.Errors) > 0 {
			t.Fatal(e, program.Errors)
		}
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(0x8000, program.Binary)
		for _, x := range []uint16{0, 1, 127, 128, 255, 256, 257, 32767, 32768, 32769, 65534, 65535} {
			a := int16(x)
			wantAbs := uint16(a)
			if a < 0 {
				wantAbs = uint16(-int32(a))
			}
			for _, c := range []struct {
				name string
				want uint16
			}{{"abs_word", wantAbs}, {"add_word", x + 1}, {"subtract_word", 7 - x}, {"same_bits", x}} {
				args := []mir2.Value{{I: int64(x)}}
				for _, mod := range []*mir2.Module{hir.LowerModule(source), steps.MIR2Module} {
					v, e := mir2.NewVM(mod).Call(c.name, args)
					if e != nil || len(v) != 1 || v[0].I != int64(c.want) {
						t.Fatal("literal width VM", c, x, v, e)
					}
				}
				f := steps.MIR2Module.FuncByName(c.name)
				regs := emulator.Registers{PC: uint16(program.Symbols[c.name]), SP: 0xeff0}
				loc := assertParamLocation(f.Contract.Params[0], steps.Allocation)
				switch loc.Name {
				case "HL":
					regs.HL = x
				case "DE":
					regs.DE = x
				case "BC":
					regs.BC = x
				case "IX":
					regs.IX = x
				default:
					t.Fatal(loc)
				}
				z.Memory()[0xeff0] = 0
				z.Memory()[0xeff1] = 0x70
				z.SetRegisters(regs)
				done := false
				for n := 0; n < 10000; n++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						done = true
						break
					}
					z.Step()
				}
				if !done {
					t.Fatal("native frame", c, x)
				}
				if e := checkAssertZ80Result(z, hir.Assert{FuncName: c.name, Expected: int64(c.want)}, f, map[string]*hir.Func{c.name: m.FuncByName(c.name)}); e != nil {
					t.Fatal("literal native", c, x, e)
				}
			}
		}
	}
}
