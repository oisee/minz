package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// The mutable global keeps the call dynamic. PBQP spills f's second parameter;
// the caller must write the named slot that f actually reads.
func TestZ80CallIntoSpilledParameter(t *testing.T) {
	const source = `global gb: u16 = 6683
fun f(a: u16, b: u16, c: u16) -> u16 {
let d: u8 = ((a as u8) xor (c as u8))
let t1: u16 = ((b + 1000) + (a & c))
let t2: u16 = ((t1 xor b) - (a | c))
let t3: u16 = ((t2 + a) xor (t1 - c))
let t4: u16 = ((t3 & b) + (t2 | a))
return ((a xor 1) + (b xor 38) + (c xor 75) + (t1 xor 112) + (t2 xor 9) + (t3 xor 7) + (t4 xor 5) + ((d as u16) xor 3))
}
fun g() -> u16 {
gb = gb + 1
return f(770, gb, 26064)
}
assert g() == 4430 via z80
`
	hm, err := nanz.Parse(source, "call_spilled_param.nanz")
	if err != nil {
		t.Fatal(err)
	}
	hm.AssertStats = &hir.AssertStats{}
	steps, err := CompileHIRSteps(hm, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if *hm.AssertStats != (hir.AssertStats{Executed: 1, Passed: 1}) {
		t.Fatalf("assert stats = %+v", hm.AssertStats)
	}
	gStart := strings.Index(steps.Assembly, "\ng:\n")
	if gStart < 0 || !strings.Contains(steps.Assembly[gStart:], "LD (_spill_v_f_r") || !strings.Contains(steps.Assembly[gStart:], "JP v_f") {
		t.Fatalf("expected dynamic tail call through the callee spill slot:\n%s", steps.Assembly)
	}
}
