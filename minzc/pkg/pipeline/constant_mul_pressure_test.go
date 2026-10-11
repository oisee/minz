package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestConstantByteMulOrdinaryLiveWordsNative(t *testing.T) {
	for _, coefficient := range []int{3, 5, 6, 7, 9, 10, 12, 15} {
		src := fmt.Sprintf(`global left:u16=4660
 global middle:u16=35243
 global right:u16=17767
 fun multiply_live(x:u8)->u16 { let a:u16=left let b:u16=middle let c:u16=right let scaled:u8=x*%d return a+b+c+(scaled as u16) }`, coefficient)
		s, err := CompileHIRSteps(graceMeasureParse(t, src), Options{ContractOpt: true, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		bin, err := z80asm.NewAssembler().AssembleString(s.Assembly)
		if err != nil || len(bin.Errors) > 0 {
			t.Fatal(err, bin.Errors)
		}
		f := s.MIR2Module.FuncByName("multiply_live")
		address := bin.Symbols[f.Name]
		if address == 0 {
			address = bin.Symbols[strings.ToUpper(f.Name)]
		}
		loc := s.Allocation.Locs[f.Contract.Params[0].Reg]
		if loc.Kind != mir2.LocReg {
			t.Fatal("byte ABI", loc)
		}
		vm := mir2.NewVM(s.MIR2Module)
		for x := 0; x < 256; x++ {
			want := (4660 + 35243 + 17767 + int(byte(x*coefficient))) & 65535
			v, err := vm.Call(f.Name, []mir2.Value{{I: int64(x)}})
			if err != nil || v[0].I != int64(want) {
				t.Fatal("VM", x, want, v, err)
			}
			boot, err := z80asm.NewAssembler().AssembleString(fmt.Sprintf("LD %s,%d\nCALL %d\nDI\nHALT\n", loc.Name, x, address))
			if err != nil || len(boot.Errors) > 0 {
				t.Fatal(err, boot.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 10000
			z.LoadMemory(uint16(bin.Origin), bin.Binary)
			z.LoadMemory(0xf100, boot.Binary)
			z.SetPC(0xf100)
			z.SetSP(0xf000)
			if err := z.Run(); err != nil {
				t.Fatal(err)
			}
			r := z.GetRegisters()
			if int(r.HL) != want || r.SP != 0xf000 {
				t.Fatalf("coeff%d x%d HL%04x want%04x SP%04x ASM%s", coefficient, x, r.HL, want, r.SP, s.Assembly)
			}
		}
	}
	t.Log("eight constants ×256 inputs ordinary-source live-word VM/native PASS")
}
