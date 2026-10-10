package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

func TestHIRGraceMIR2Assertions(t *testing.T) {
	for _, useHIR := range []bool{false, true} {
		for _, useMIR := range []bool{false, true} {
			for _, want := range []string{"0", "1"} {
				src := `fun wrap(x: u8) -> u8 {
    if true { return x + 1 } else { return 99 }
}
assert wrap(255) == ` + want + ` via mir2
`
				h, err := nanz.Parse(src, "hir_grace.nanz")
				if err != nil {
					t.Fatal(err)
				}
				before := h.Dump()
				steps, err := CompileHIRSteps(h, Options{UseHIRGrace: useHIR, UseGrace: useMIR, AssertMode: "mir2", SkipZ80Emission: true})
				if want == "0" && err != nil {
					t.Fatalf("HIR=%v MIR=%v: %v", useHIR, useMIR, err)
				}
				if want == "1" && (err == nil || !strings.Contains(err.Error(), "[mir2]")) {
					t.Fatalf("failing assert lost: HIR=%v MIR=%v err=%v", useHIR, useMIR, err)
				}
				if want == "0" {
					if (steps.HIR != before) != useHIR {
						t.Fatalf("HIR flag not reflected in dump: HIR=%v", useHIR)
					}
					if steps.Assembly != "" {
						t.Fatal("unexpected machine emission")
					}
				}
			}
		}
	}
}
