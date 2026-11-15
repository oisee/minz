package hir_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// Nanz masks before widening — u16(v & 1) lowers to ext(and(v, 1)) — so the
// bit-accum fusion must unwrap the outer ext to fire. Plain while-loop form,
// no iterator lambda: this isolates the mir2 pattern match from capture
// handling. The value asserts pass either way; the .bit_accum_ label check
// is what fails without the ext(and(v,1)) match.
func TestBitAccumFusesNanzExtAndForm(t *testing.T) {
	m, err := nanz.Parse(`
fun popc(v0: u8) -> u16 {
    var v: u8 = v0
    var total: u16 = 0
    var i: u8 = 0
    while i < 8 {
        total = total + u16(v & 1)
        v = v >> 1
        i = i + 1
    }
    return total
}
`, "bit-accum-ext-and.nanz")
	if err != nil {
		t.Fatal(err)
	}
	m.Asserts = append(m.Asserts,
		hir.Assert{FuncName: "popc", Args: []int64{0}, Expected: 0},
		hir.Assert{FuncName: "popc", Args: []int64{255}, Expected: 8},
		hir.Assert{FuncName: "popc", Args: []int64{0xA5}, Expected: 4},
		hir.Assert{FuncName: "popc", Args: []int64{1}, Expected: 1},
		hir.Assert{FuncName: "popc", Args: []int64{0x80}, Expected: 1},
	)
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(steps.Assembly, ".bit_accum_") {
		t.Fatalf("expected fused SRL/JR NC/INC bit-accum step (.bit_accum_ label) in assembly for popc")
	}
}
