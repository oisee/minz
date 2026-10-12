package hir_test

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

// Frozen before execution: unsigned zero/one, sign-bit transition and upper
// wrap boundaries. Exhaust both a/b for every selected c, and c/d pair for a2.
// This extends the retained LIR composition value oracles; it is not a complete
// 3/4-argument Cartesian or a full memory/stack-effect oracle.
func TestNativeCompositionAdditionalArgumentBoundaries(t *testing.T) {
	boundaries := []int64{0, 1, 127, 128, 254, 255}
	for _, fixture := range nativeCompositionFixtures() {
		if fixture.arity < 3 {
			continue
		}
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			hm, err := nanz.Parse(fixture.src, "critic-boundary.nanz")
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range boundaries {
				domainD := []int64{0}
				if fixture.arity == 4 {
					domainD = boundaries
				}
				for _, d := range domainD {
					t.Run(fmt.Sprintf("c%d_d%d", c, d), func(t *testing.T) {
						var completed atomic.Int64
						inputs := func(i int) []int64 {
							completed.Add(1)
							return []int64{int64(i & 255), int64(i >> 8), c, d}[:fixture.arity]
						}
						bad, first := sweepCodegenJudge(t, hm.Funcs, fixture.name, 65536, inputs, fixture.model)
						if completed.Load() != 65536 {
							t.Fatalf("executed %d/65536 cases", completed.Load())
						}
						if bad != 0 {
							t.Fatalf("native %d/65536 mismatches (%s)", bad, first)
						}
					})
				}
			}
		})
	}
}
