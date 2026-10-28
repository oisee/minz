package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// runNanzAsserts compiles source with the production options and returns the
// recorded assert statistics. An assert without a `via` tag runs on both the
// MIR2 VM and the Z80, so Executed counts each backend separately.
func runNanzAsserts(t *testing.T, source, file string) hir.AssertStats {
	t.Helper()
	hm, err := nanz.Parse(source, file)
	if err != nil {
		t.Fatal(err)
	}
	hm.AssertStats = &hir.AssertStats{}
	if _, err := CompileHIRSteps(hm, DefaultOptions()); err != nil {
		t.Fatalf("compile: %v\nassert stats %+v", err, *hm.AssertStats)
	}
	return *hm.AssertStats
}

// TestAsmInputConstE2E: an inline-asm input operand reads the physical
// register the allocator picked for it, so the constant feeding it must be
// materialised. computeDeadConsts inventoried only Src/Args/terminator
// operands, the constant looked dead and `LD C,0x5A` was never emitted, so
// the template read whatever C happened to hold.
func TestAsmInputConstE2E(t *testing.T) {
	const source = `fun read_c_const() -> u8 {
    let k: u8 = 0x5A
    asm z80 (in k) { LD A, C }
}
assert read_c_const() == 0x5A via z80
`
	if stats := runNanzAsserts(t, source, "asm_input_const.nanz"); stats != (hir.AssertStats{Executed: 1, Passed: 1}) {
		t.Fatalf("assert stats = %+v", stats)
	}
}
