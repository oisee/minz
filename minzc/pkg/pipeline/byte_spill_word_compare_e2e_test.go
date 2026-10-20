package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// Direct-call fuzz seed 985 (branch fix/P12-spill-slot-width): the u16 loop
// bound `3` lives in a byte-typed spill slot (DB 0) while the loop-head
// compare reads it with LD BC,(slot). The neighbouring slot's byte became the
// high byte of the bound, the while loop ran away and the Z80 assert hit the
// execution limit. The bound must be read at its declared width.
func TestZ80ByteSpillSlotWordCompareE2E(t *testing.T) {
	const source = `fun fuzz_entry(a: u16, b: u16, c: u16) -> u16 {
let d: u8 = ((a as u8) xor (c as u8))
var v0: u16 = a
var v1: u16 = (b xor c)
if b == v1 {
let t1: u16 = a
}
v0 = c
let t2: u16 = c
let t3: u16 = v0
var lp4: u16 = 0
while lp4 < 3 {
let t5: u16 = ((t2 + v0) / 16)
let t6: u16 = (v0 | (19079 - t3))
lp4 = (lp4 + 1)
}
var lp7: u16 = 0
while lp7 < 0 {
if v1 > c {
let t8: u16 = (((d as u16) xor b) | (0 + t3))
}
lp7 = (lp7 + 1)
}
let t9: u16 = a
return (((d as u16) xor 1) + (c xor 38) + (v0 xor 75) + (t3 xor 112) + (t2 xor 149) + (a xor 186) + (b xor 223) + (v1 xor 260))
}
fun g() -> u16 { return fuzz_entry(37858, 28825, 59858) }
assert fuzz_entry(37858, 28825, 59858) == 17044 via z80
`
	hm, err := nanz.Parse(source, "byte_spill_word_compare.nanz")
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
	if !containsByteSlot(steps.Assembly, "fuzz_entry") {
		t.Logf("note: allocator gave fuzz_entry no byte-wide spill slot")
	}
}

// containsByteSlot reports whether the function's data section declares a
// one-byte spill home, the shape this regression depends on.
func containsByteSlot(asm, fn string) bool {
	_, tail, ok := strings.Cut(asm, "; — spill slots for "+fn)
	if !ok {
		_, tail, ok = strings.Cut(asm, "; — extra spill slots for "+fn)
	}
	return ok && strings.Contains(tail, ": DB 0\n")
}
