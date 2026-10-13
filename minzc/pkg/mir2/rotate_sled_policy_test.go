package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestRotateSledGoalAndWholeModuleSize(t *testing.T) {
	run := "    RLCA\n    RLCA\n    RLCA\n    RLCA\n"
	src := strings.Repeat(run+"next:\n", 9)
	if strings.Contains(asmPeepholePass(src), "CALL __rotate_") {
		t.Fatal("Speed introduced slow calls")
	}
	if strings.Count(asmPeepholePassWithSize(src, true), "CALL __rotate_4") != 9 {
		t.Fatal("9 byte saving exceeds 8-byte runtime")
	}
	for _, n := range []int{1, 8} {
		if strings.Contains(asmPeepholePassWithSize(strings.Repeat(run+"next:\n", n), true), "CALL __rotate_") {
			t.Fatal("not strict total size win", n)
		}
	}
	if strings.Contains(asmPeepholePassWithSize(strings.Repeat(" RLCA\n", 8), true), "__rotate_8") {
		t.Fatal("unsupported rotate count")
	}
	if strings.Contains(asmPeepholePassWithSize(strings.Repeat(" RLCA\n", 3), true), "CALL __rotate_3") {
		t.Fatal("three bytes equal, slower")
	}
}

func TestRotateSledSizeIncludesRuntimeBinary(t *testing.T) {
	source := "entry:\n"
	for i := 0; i < 9; i++ {
		source += " RLCA\n RLCA\n RLCA\n RLCA\n" + fmt.Sprintf("site%d:\n", i)
	}
	source += " RET\n"
	inline, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + source)
	if e != nil || len(inline.Errors) > 0 {
		t.Fatal(e, inline.Errors)
	}
	selected := asmPeepholePassWithSize(source, true)
	runtime := "\n__rotate_7: RLCA\n__rotate_6: RLCA\n__rotate_5: RLCA\n__rotate_4: RLCA\n__rotate_3: RLCA\n__rotate_2: RLCA\n__rotate_1: RLCA\n__rotate_0: RET\n"
	folded, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + selected + runtime)
	if e != nil || len(folded.Errors) > 0 {
		t.Fatal(e, folded.Errors)
	}
	if len(inline.Binary) != 37 || len(folded.Binary) != 36 {
		t.Fatal("totalBIN", len(inline.Binary), len(folded.Binary))
	}
	for _, a := range []*z80asm.Result{inline, folded} {
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		z.LoadMemory(0x8000, a.Binary)
		z.LoadMemory(0x7000, []byte{0xcd, 0, 0x80, 0x76})
		for x := 0; x < 256; x++ {
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: byte(x), BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
			done := false
			for n := 0; n < 1000; n++ {
				if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
					done = true
					break
				}
				z.Step()
			}
			r := z.GetRegisters()
			want := byte(x)<<4 | byte(x)>>4
			if !done || r.A != want || r.BC != 0x1234 || r.DE != 0x5678 || r.HL != 0x9abc || r.IX != 0x1357 || r.IY != 0x2468 {
				t.Fatal("nine-sites", x, r)
			}
		}
	}
}
