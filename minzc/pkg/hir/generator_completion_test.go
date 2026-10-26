package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"testing"
)

// Explicit completion must publish false, set sticky DONE and leave out intact,
// including returns in yield-free branches outside flattened producer loops.
func TestGeneratorExplicitCompletionDone(t *testing.T) {
	m, err := nanz.Parse(`
global gate: u8 = 0

gen one() -> u8 {
    yield 7
    return
}
fun trailing_return() -> u8 {
    var count: u8 = 0
    for value in one() {
        count = count + 1
        if count == 3 { break }
    }
    return count
}

gen maybe() -> u8 {
    if gate == 0 { return }
    yield 9
}
fun sticky_return() -> u8 {
    gate = 0
    var it = maybe()
    var out: u8 = 77
    let first = pull_next(&it, &out)
    gate = 1
    let second = pull_next(&it, &out)
    if first { return 1 }
    if second { return 2 }
    return out
}
`, "generator-completion.nanz")
	if err != nil {
		t.Fatal(err)
	}
	m.NonNullObjects = true
	m.Asserts = append(m.Asserts, hir.Assert{FuncName: "trailing_return", Expected: 1, Source: "trailing return completes"}, hir.Assert{FuncName: "sticky_return", Expected: 77, Source: "early return is sticky DONE"})
	if err = hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	if err = pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
