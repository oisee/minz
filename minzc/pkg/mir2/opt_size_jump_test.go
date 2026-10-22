package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestOptSizeUnconditionalJumpRange(t *testing.T) {
	for _, padding := range []int{1, 127, 128, 256} {
		source := "probe:\n    JP .done\n" + strings.Repeat("    NOP\n", padding) + ".done:\n    LD A, 7\n    RET\n"
		var lengths [2]int
		for i, size := range []bool{false, true} {
			code := asmPeepholePassWithSize(source, size)
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + code)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors, code)
			}
			lengths[i] = len(a.Binary)
			expected := byte(0xc3)
			if size && padding <= 127 {
				expected = 0x18
			}
			if a.Binary[0] != expected {
				t.Errorf("padding%d size%v opcode%02x want%02x", padding, size, a.Binary[0], expected)
			}
			z := emulator.NewRemogattoZ80()
			copy(z.Memory()[0x8000:], a.Binary)
			z.Memory()[0xeffe], z.Memory()[0xefff] = 0, 0xf1
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xeffe})
			for steps := 0; steps < 10 && z.GetPC() != 0xf100; steps++ {
				z.Step()
			}
			if z.GetPC() != 0xf100 || z.GetRegisters().A != 7 || z.GetRegisters().SP != 0xf000 {
				t.Fatal("jump changed execution")
			}
		}
		saving := 0
		if padding <= 127 {
			saving = 1
		}
		if lengths[0]-lengths[1] != saving {
			t.Errorf("padding%d bytes%v", padding, lengths)
		}
	}
}
