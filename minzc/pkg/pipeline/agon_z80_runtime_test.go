package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestAgonZ80RejectsOpaqueASMAndRecursion(t *testing.T) {
	for _, src := range []string{
		"fun main() { asm z80 { RST.LIS $10 } }",
		"fun dead() { asm z80 { NOP } } fun main() { }",
		"global depth:u8=4 fun recurse() { if depth>0 { depth=depth-1 recurse() depth=depth+1 } } fun main() { recurse() }",
		"fun __agon_entry() {} fun main() { __agon_entry() }",
	} {
		m, err := nanz.Parse(src, "admission.nanz")
		if err != nil {
			t.Fatal(err)
		}
		m.Target = hir.TargetAgon
		if _, err := CompileHIRSteps(m, Options{Backend: "z80", Runtime: mir2.Z80RuntimeAgonZ80, AssertMode: "none"}); err == nil || (!strings.Contains(err.Error(), "agon-z80") && !strings.Contains(err.Error(), "ambiguous Z80 symbol")) {
			t.Fatalf("unadmitted source accepted: %s: %v", src, err)
		}
	}
}
