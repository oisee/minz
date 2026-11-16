package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// Loop rotation repeats a small while-loop header test at the latch. sum16
// rotates a 16-bit counter test (LD A,H / OR L) whose exit edge falls into a
// conditional return; walk rotates a byte compare whose exit edge carries a
// block-argument copy through the header's trampoline. Both run on the Z80
// emulator against the VM, with rotation on, off, and in size mode.
func TestLoopRotationWhileLoopsVMZ80(t *testing.T) {
	var src strings.Builder
	src.WriteString(`fun sum16(n: u16) -> u16 {
    var s: u16 = 0
    var i: u16 = n
    while i != 0 {
        s = s + i
        i = i - 1
    }
    return s
}
fun walk(a: u8, b: u8) -> u16 {
    var k: u8 = 0
    var x: u8 = a
    while x < b {
        x = x + 3
        k = k + 1
    }
    let w: u16 = u16(k) * 256
    return w + u16(x)
}
`)
	asserts := 0
	for _, n := range []int{0, 1, 2, 10, 255, 256, 361} {
		for _, via := range []string{"mir2", "z80"} {
			fmt.Fprintf(&src, "assert sum16(%d) == %d via %s\n", n, (n*(n+1)/2)&0xFFFF, via)
			asserts++
		}
	}
	for _, ab := range [][2]int{{0, 0}, {1, 10}, {10, 1}, {0, 255}, {7, 8}, {250, 252}, {5, 5}} {
		k, x := 0, ab[0]
		for x < ab[1] {
			x = (x + 3) & 0xFF
			k = (k + 1) & 0xFF
			if k > 200 {
				t.Fatal("fixture loop does not terminate")
			}
		}
		for _, via := range []string{"mir2", "z80"} {
			fmt.Fprintf(&src, "assert walk(%d, %d) == %d via %s\n", ab[0], ab[1], k*256+x, via)
			asserts++
		}
	}
	off := DefaultOptions()
	off.DisableLoopRotation = true
	for i, opts := range []Options{DefaultOptions(), off, {ContractOpt: true, OptSize: true, UseGrace: true}} {
		h, err := nanz.Parse(src.String(), "loop_rotation.nanz")
		if err != nil {
			t.Fatal(err)
		}
		h.AssertStats = &hir.AssertStats{}
		steps, err := CompileHIRSteps(h, opts)
		if err != nil {
			t.Fatal(err)
		}
		if *h.AssertStats != (hir.AssertStats{Executed: asserts, Passed: asserts}) {
			t.Fatalf("options %d: assert stats %+v, want %d passed", i, h.AssertStats, asserts)
		}
		rotated := strings.Count(steps.Assembly, "; loop rotation: test of")
		if want := map[int]int{0: 2, 1: 0, 2: 0}[i]; rotated != want {
			t.Fatalf("options %d: %d rotated loops, want %d:\n%s", i, rotated, want, steps.Assembly)
		}
	}
}
