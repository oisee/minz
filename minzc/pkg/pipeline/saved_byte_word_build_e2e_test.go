package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// Direct-call fuzz seed 846 saves b9 in E before computing b10 in A.
// Widening b10 into DE for the divisor must preserve that still-live b9.
// Earlier control flow keeps the allocation that exposed this collision.
func TestZ80SavedByteWordBuildE2E(t *testing.T) {
	cases := []struct{ name, source string }{
		{"seed846", `fun h0(p0: u16) -> u16 {
let t0: u16 = p0
return ((t0 + (1 + t0)))
}
fun fuzz_entry(a: u16, b: u16, c: u16) -> u16 {
let d: u8 = ((a as u8) xor (c as u8))
var v0: u16 = a
var v1: u16 = (b xor c)
if b == c {
if c > v0 {

}
let t1: u16 = c
} else {
var lp2: u16 = 0
while lp2 < 0 {
let t3: u16 = a
lp2 = (lp2 + 1)
}
}
let t4: u16 = (v1 - a)
v1 = t4
if a >= c {
v1 = 1000
}
var lp5: u16 = 0
while lp5 < 3 {
if a < v1 {
let t6: u16 = (c + (d as u16))
} else {
v0 = ((v0 + lp5) + (v0 + (d as u16)))
let t7: u16 = ((v0 + 7) % ((v0 % 1) | 1))
}
let t8: u16 = ((a + t4) | (v0 - a))
lp5 = (lp5 + 1)
}
let b9: u8 = ((d | d) & (c as u8))
let b10: u8 = ((b9 & d) - d)
let t11: u16 = (((b10 as u16) + b) % ((b10 as u16) | 1))
return (((b10 as u16) xor 112) + ((b9 as u16) xor 260))
}
assert fuzz_entry(33915, 21387, 37935) == 448 via z80
`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			hm, err := nanz.Parse(tt.source, tt.name+".nanz")
			if err != nil {
				t.Fatal(err)
			}
			hm.AssertStats = &hir.AssertStats{}
			steps, err := CompileHIRSteps(hm, DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(steps.Assembly, "before ALU overwrite") {
				t.Fatal("regression no longer exercises a saved ALU operand")
			}
			if *hm.AssertStats != (hir.AssertStats{Executed: 1, Passed: 1}) {
				t.Fatalf("assert stats = %+v", hm.AssertStats)
			}
		})
	}
}
