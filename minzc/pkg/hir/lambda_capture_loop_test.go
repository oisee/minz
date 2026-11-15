package hir_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// A lambda inlined by an iterator call assigns its captures in the
// surrounding loop or branch. Those captures are loop-carried / join state:
// before the fix every outer iteration restarted from the initial value and
// the outer loop returned it unchanged.
func TestIteratorLambdaCaptureMutationCrossesOuterControl(t *testing.T) {
	m, err := nanz.Parse(`
fun nested_count(n: u8) -> u16 {
    var total: u16 = 0
    for i in 0..n {
        range(0..3).forEach(|j: u8| { total = total + 1 })
    }
    return total
}
fun nested_bits(a: u8, b: u8) -> u16 {
    var total: u16 = 0
    for i in 0..2 {
        var v: u8 = a
        if i == 1 { v = b }
        range(0..8).forEach(|bit: u8| {
            total = total + u16(v & 1)
            v = v >> 1
        })
    }
    return total
}
fun branch_count(n: u8) -> u16 {
    var t: u16 = 0
    if n > 0 {
        range(0..n).forEach(|i: u8| { t = t + 2 })
    }
    return t
}
`, "lambda-capture-loop.nanz")
	if err != nil {
		t.Fatal(err)
	}
	m.Asserts = append(m.Asserts,
		hir.Assert{FuncName: "nested_count", Args: []int64{0}, Expected: 0},
		hir.Assert{FuncName: "nested_count", Args: []int64{4}, Expected: 12},
		hir.Assert{FuncName: "nested_bits", Args: []int64{255, 1}, Expected: 9},
		hir.Assert{FuncName: "nested_bits", Args: []int64{0xA5, 0x0F}, Expected: 8},
		hir.Assert{FuncName: "branch_count", Args: []int64{0}, Expected: 0},
		hir.Assert{FuncName: "branch_count", Args: []int64{5}, Expected: 10},
	)
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
