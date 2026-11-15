package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// `if a || b { return p } return q` ends the join block with a comparison
// held in F and a conditional return that tests the joined byte and returns
// the comparison. Testing the byte must not replace the returned predicate.
func TestConditionalReturnOfComparisonAfterLogicalOrVMZ80(t *testing.T) {
	ops := []struct {
		name, op string
		holds    func(x, k int) bool
	}{
		{"gt", ">", func(x, k int) bool { return x > k }},
		{"ge", ">=", func(x, k int) bool { return x >= k }},
		{"lt", "<", func(x, k int) bool { return x < k }},
		{"le", "<=", func(x, k int) bool { return x <= k }},
		{"eq", "==", func(x, k int) bool { return x == k }},
		{"ne", "!=", func(x, k int) bool { return x != k }},
	}
	var src strings.Builder
	asserts := 0
	for _, o := range ops {
		fmt.Fprintf(&src, "fun p%s(x: u8, k: u8) -> bool {\n    if x > 181 || k == 7 { return x != 221 }\n    return x %s k\n}\n", o.name, o.op)
		for _, x := range []int{0, 7, 18, 132, 181, 182, 221, 255} {
			for _, k := range []int{0, 7, 18, 132, 200, 255} {
				want := 0
				if x > 181 || k == 7 {
					if x != 221 {
						want = 1
					}
				} else if o.holds(x, k) {
					want = 1
				}
				for _, via := range []string{"mir2", "z80"} {
					fmt.Fprintf(&src, "assert p%s(%d, %d) == %d via %s\n", o.name, x, k, want, via)
					asserts++
				}
			}
		}
	}
	for _, opts := range []Options{DefaultOptions(), {ContractOpt: true, UseGrace: true}, {ContractOpt: true, OptSize: true, UseGrace: true}} {
		h, err := nanz.Parse(src.String(), "condret.nanz")
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
