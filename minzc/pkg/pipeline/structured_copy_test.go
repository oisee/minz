package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

// Compare the pre-change generator's emitted code with direct HIR construction.
// Independent memory/PC/SP oracles remain in the HardwareCopy matrix.
func TestStructuredCopyMatchesTextBaseline(t *testing.T) {
	baseline, err := os.ReadFile("../../../reports/structured-hardware-copy/copy-text-baseline.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`source "linear" 24576 8 2 11; destination "linear" 25088 8 2 13; region 1 0 2 0 4 2;`,
		`source "linear" 24576 256 1 256; destination "linear" 25088 256 1 256; region 0 0 0 0 256 1;`,
		`source "linear" 24576 8 2 8; destination "zx_bitmap_bytes" 16384; region 0 0 2 7 5 2;`,
	} {
		text := string(baseline) + "\n" + strings.Replace(hardwareCopySource(body), "import hw.copy\n", "", 1)

		for _, opts := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
			current := hardwareCopyParse(t, hardwareCopySource(body))
			old, err := nanz.Parse(text, "baseline.nanz")
			if err != nil {
				t.Fatal(err)
			}
			before, err := CompileHIRSteps(old, opts)
			if err != nil {
				t.Fatal(err)
			}
			after, err := CompileHIRSteps(current, opts)
			if err != nil {
				t.Fatal(err)
			}
			assemble := func(src string) (int, int) {
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + src)
				if err != nil || len(a.Errors) != 0 {
					t.Fatal(err, a.Errors)
				}
				z := emulator.NewRemogattoZ80()
				if err := z.LoadMemory(0x8000, a.Binary); err != nil {
					t.Fatal(err)
				}
				z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["copy_test"]), SP: 0xeff0})
				for count := 0; count < 2000000; count++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						return len(a.Binary), z.GetCycles()
					}
					z.Step()
				}
				t.Fatal("native did not restore PC/SP", z.GetRegisters())
				return 0, 0
			}
			oldSize, oldCycles := assemble(before.Assembly)
			newSize, newCycles := assemble(after.Assembly)
			if newSize > oldSize || newCycles > oldCycles {
				t.Fatalf("code regressed %d -> %d bytes, %d -> %d T-states for %s", oldSize, newSize, oldCycles, newCycles, body)
			}
			t.Logf("%s grace=%v size=%v: %d -> %d bytes; %d -> %d T-states; identical ASM=%v", body, opts.UseGrace, opts.OptSize, oldSize, newSize, oldCycles, newCycles, before.Assembly == after.Assembly)
		}
	}
}
