package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// A three-way comparison fused by MIR Grace loads its lhs into A. A loop
// counter or result living in A must survive on every edge (Z80 only; the
// MIR VM has no accumulator).
func TestBrIf2PreservesLiveAccumulatorVMZ80(t *testing.T) {
	const src = `fun cnt(a:u8,b:u8)->u8 { var n:u8=0
 while a!=b { if a>b {a=a-1} else {a=a+1}
 n=n+1 }
 return n }
fun ch(a:u8,b:u8)->u8 { var r:u8=0
 while a!=b { if a>b {r=1} else {r=2}
 a=b }
 return r }
assert cnt(10,4)==6 via z80
assert cnt(4,10)==6 via z80
assert cnt(7,7)==0 via z80
assert cnt(255,0)==255 via z80
assert cnt(10,4)==6 via mir2
assert ch(0,255)==2 via z80
assert ch(255,0)==1 via z80
assert ch(5,5)==0 via z80
assert ch(0,255)==2 via mir2
`
	for _, opts := range []Options{DefaultOptions(), {ContractOpt: true, UseGrace: true}, {ContractOpt: true, OptSize: true, UseGrace: true}} {
		h, err := nanz.Parse(src, "brif2.nanz")
		if err != nil {
			t.Fatal(err)
		}
		h.AssertStats = &hir.AssertStats{}
		if _, err := CompileHIRSteps(h, opts); err != nil {
			t.Fatal(err)
		}
		if *h.AssertStats != (hir.AssertStats{Executed: 9, Passed: 9}) {
			t.Fatalf("assert stats %+v", h.AssertStats)
		}
	}
}
