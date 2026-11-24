package nanz_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// A fused iterator chain lowers through lowerForEach, not through a while
// loop. Its loop must still carry a static-loop fact, or the variant search
// treats the function as loop-free: the loop is scored as 0 T and the
// allocation alternatives a C-style loop gets are never tried. Here the
// loop-invariant k then stayed in A and was saved and restored around every
// counter test and every element load.
func TestForEachChainLoopIsScoredByVariantSearch(t *testing.T) {
	src := `
global source: [u8; 256]
global n: u16
global result: u16
global k: u8
fun execute() -> void {
    var count: u16 = 0
    source.filter(|x: u8| x == k).forEach(|x: u8| { count = count + 1 }, n)
    result = count
}
fun run(len: u16, key: u8) -> u16 {
    for i in 0..16 { source[i] = i & 3 }
    n = len
    k = key
    execute()
    return result
}
assert run(16, 1) == 4
assert run(5, 0) == 2
assert run(0, 0) == 0
assert run(16, 9) == 0
`
	if err := g0RunAsserts(t, src); err != nil {
		t.Fatal(err)
	}
	m, err := nanz.Parse(src, "foreach_scoring")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	f := steps.MIR2Module.FuncByName("execute")
	if f == nil {
		t.Fatal("execute was not compiled")
	}
	found := false
	for label, fact := range f.StaticLoops {
		if strings.HasPrefix(label, "fe_head") {
			found = true
			if fact.Trips >= 0 || fact.Counted {
				t.Fatalf("forEach loop %s claims a proof it does not have: %+v", label, fact)
			}
		}
	}
	if !found {
		t.Fatalf("no static-loop fact for the forEach loop: %+v", f.StaticLoops)
	}
	start := strings.Index(steps.Assembly, "\nexecute:")
	if start < 0 {
		t.Fatal("execute not found in assembly")
	}
	body := steps.Assembly[start:]
	if end := strings.Index(body, "\n; fun "); end >= 0 {
		body = body[:end]
	}
	if strings.Contains(body, "preserve live accumulator before CMP") {
		t.Fatalf("loop-invariant operand is parked in A across the loop:\n%s", body)
	}
}
