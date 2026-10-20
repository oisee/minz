package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// Inlined constants used as both operands still need their left-operand load.
func TestZ80SelfConstantOperandsE2E(t *testing.T) {
	for name, source := range map[string]string{
		"seed864": `fun h1(p0: u16, p1: u16) -> u16 {
return (((p1 + p1) - (p0 + p0)))
}
fun fuzz_entry(a: u16, b: u16, c: u16) -> u16 {
var v0: u16 = a
v0 = h1(a, 1)
return ((v0 xor 112))
}
fun g() -> u16 { return fuzz_entry(43418, 47214, 60935) }
assert fuzz_entry(43418, 47214, 60935) == 44222 via z80
`,
		"seed1766": `fun h0(p0: u16, p1: u16) -> u16 {

return (((p1 | p1) + p0))
}
fun fuzz_entry(a: u16, b: u16, c: u16) -> u16 {
let t5: u16 = h0((a + a), 7)
return ((t5 xor 223))
}
fun g() -> u16 { return fuzz_entry(217, 15456, 227) }
assert fuzz_entry(217, 15456, 227) == 358 via z80
`,
	} {
		t.Run(name, func(t *testing.T) {
			hm, err := nanz.Parse(source, name+".nanz")
			if err != nil {
				t.Fatal(err)
			}
			hm.AssertStats = &hir.AssertStats{}
			if _, err := CompileHIRSteps(hm, DefaultOptions()); err != nil {
				t.Fatal(err)
			}
			if *hm.AssertStats != (hir.AssertStats{Executed: 1, Passed: 1}) {
				t.Fatalf("assert stats = %+v", hm.AssertStats)
			}
		})
	}
}
