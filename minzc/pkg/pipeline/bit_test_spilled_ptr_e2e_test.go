package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// An inlined `(x & 1) == 1` on an array element becomes BIT 0, (HL). When the
// element address lives in a spill slot it is reloaded into HL, which here
// holds the loop index: the index must survive the test.
func TestBitTestThroughSpilledPointerKeepsHLVMZ80(t *testing.T) {
	var src strings.Builder
	src.WriteString(`global ga: [u8; 600]
global gb: [u8; 600]
global gc: [u8; 600]

fun isodd(x: u8) -> bool {
    return (x & 1) == 1
}

fun f0(n: u16, q: u8) -> u16 {
    var s0: u16 = n
    var s1: u16 = 4097
    var s2: u16 = n
    var s3: u16 = 12291
    var b2: u8 = q + 62
    var i: u16 = 0
    while i < 2 {
        s3 = (u16(gb[i + 100]) << 8) | u16(gc[200 + i])
        if isodd(ga[7 + i]) { s3 = s3 + 1 } else { s3 = s3 + i }
        s2 = s2 + u16(gc[40 - i])
        s1 = s1 + u16(b2)
        i = i + 1
    }
    return s0 xor s1 xor s2 xor s3
}
`)
	asserts := 0
	for _, n := range []int{0, 1, 513, 65535} {
		for _, q := range []int{0, 136, 193, 255} {
			// The arrays hold zeros: s3 ends as the last index, s2 stays n.
			want := (4097 + 2*((q+62)&255)) ^ 1
			for _, via := range []string{"mir2", "z80"} {
				fmt.Fprintf(&src, "assert f0(%d, %d) == %d via %s\n", n, q, want, via)
				asserts++
			}
		}
	}
	for _, opts := range []Options{DefaultOptions(), {ContractOpt: true, UseGrace: true}, {ContractOpt: true, OptSize: true, UseGrace: true}} {
		h, err := nanz.Parse(src.String(), "bitspill.nanz")
		if err != nil {
			t.Fatal(err)
		}
		h.AssertStats = &hir.AssertStats{}
		if _, err := CompileHIRSteps(h, opts); err != nil {
			t.Fatal(err)
		}
		if *h.AssertStats != (hir.AssertStats{Executed: asserts, Passed: asserts}) {
			t.Fatalf("assert stats %+v, want %d passed", h.AssertStats, asserts)
		}
	}
}
