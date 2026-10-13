package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

// Compare the pre-change generator's emitted code with direct HIR construction.
// Independent memory/PC/SP oracles remain in the HardwareFill matrix.
func TestStructuredFillMatchesTextBaseline(t *testing.T) {
	baseline, err := os.ReadFile("../../../reports/structured-hardware-emit/fill-text-baseline.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`layout "linear" 24576 8 2 11; region 1 0 4 2; value 91;`,
		`layout "linear" 24576 256 1 256; value 255;`,
		`layout "zx_bitmap_bytes" 16384; region 2 7 5 2; value 71;`,
	} {
		current := hardwareFillParse(t, body)
		text := string(baseline) + "\n" + strings.Replace(hardwareFillSource(body), "import hw.fill\n", "", 1)
		old, err := nanz.Parse(text, "baseline.nanz")
		if err != nil {
			t.Fatal(err)
		}
		for _, opts := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
			before, err := CompileHIRSteps(old, opts)
			if err != nil {
				t.Fatal(err)
			}
			after, err := CompileHIRSteps(current, opts)
			if err != nil {
				t.Fatal(err)
			}
			assemble := func(src string) int {
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + src)
				if err != nil || len(a.Errors) != 0 {
					t.Fatal(err, a.Errors)
				}
				return len(a.Binary)
			}
			oldSize, newSize := assemble(before.Assembly), assemble(after.Assembly)
			if newSize > oldSize {
				t.Fatalf("code grew %d -> %d for %s", oldSize, newSize, body)
			}
			t.Logf("%s grace=%v size=%v: %d -> %d bytes; identical ASM=%v", body, opts.UseGrace, opts.OptSize, oldSize, newSize, before.Assembly == after.Assembly)
		}
	}
}
