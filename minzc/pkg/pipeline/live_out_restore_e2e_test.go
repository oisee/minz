package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

// Seed 10712 relocates d from B to memory and b0 from A to B.
// Both must be restored simultaneously before entering the successor.
func TestZ80LiveOutRestoreParallelE2E(t *testing.T) {
	const source = `fun h1(p0: u16) -> u16 {

return (((p0 & p0) - (p0 - p0)))
}
fun fuzz_entry(a: u16, b: u16, c: u16) -> u16 {
let d: u8 = ((a as u8) xor (c as u8))
var v0: u16 = a
var v1: u16 = (b xor c)
let b0: u8 = (((v0 as u8) + d) + 65)
let t1: u16 = h1((a - a))
let t2: u16 = (b - ((b0 as u16) + 256))
v1 = ((3 & 256) xor a)
if v0 > v1 {
let t3: u16 = (((b0 as u16) / 3) - a)
} else {
}
let t5: u16 = c
return ((b xor 1) + (t5 xor 38) + (a xor 75) + ((b0 as u16) xor 112) + (v0 xor 149) + (t1 xor 186) + (t2 xor 223) + ((d as u16) xor 260))
}
fun g() -> u16 { return fuzz_entry(42465, 87, 211) }
assert fuzz_entry(42465, 87, 211) == 20057 via z80
`
	for _, backend := range []string{"mir2", "z80"} {
		t.Run(backend, func(t *testing.T) {
			hm, err := nanz.Parse(strings.ReplaceAll(source, "via z80", "via "+backend), "live_out_restore.nanz")
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
