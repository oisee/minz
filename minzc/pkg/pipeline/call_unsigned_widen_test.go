package pipeline

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

// Seed 21428 passes a u8 XOR expression to h0's u16 parameter. Before the
// fix raw/final MIR passed that byte unchanged; inlining substituted it into
// word subtraction. In the minimized fixture H=5 was overwritten by staging
// the word lhs in HL, then byte promotion emitted SBC HL,HL (zero). The
// original returns 64661 instead of 65207; minimized returns 709 instead of 703.
func TestCallUnsignedArgumentWideningVMZ80(t *testing.T) {
	for _, name := range []string{"arguments", "minimized", "original"} {
		t.Run(name, func(t *testing.T) {
			path := "testdata/p23/" + name + ".nanz"
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, backend := range []string{"mir2", "z80"} {
				h, err := nanz.Parse(string(src), path)
				if err != nil {
					t.Fatal(err)
				}
				for i := range h.Asserts {
					h.Asserts[i].Via = backend
				}
				if _, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: backend}); err != nil {
					t.Fatalf("%s: %v", backend, err)
				}
			}
		})
	}
}
