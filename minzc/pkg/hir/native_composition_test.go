package hir_test

import (
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

type nativeCompositionFixture struct {
	name, src string
	arity     int
	model     func([]int64) int64
}

func nativeCompositionFixtures() []nativeCompositionFixture {
	return []nativeCompositionFixture{
		{"a1", `fun a1(a: u8, b: u8, c: u8) -> u8 { let p: u8 = a + b
 return p - (c + a) }`, 3, func(a []int64) int64 { return (a[1] - a[2]) & 255 }},
		{"a2", `fun a2(a: u8, b: u8, c: u8, d: u8) -> u8 { let p: u8 = a - b
 return p + (c - d) }`, 4, func(a []int64) int64 { return (a[0] - a[1] + a[2] - a[3]) & 255 }},
		{"a4", `fun a4(a: u8, b: u8, c: u8) -> u8 { return c - ((a >> 1) - (b << 1)) }`, 3, func(a []int64) int64 { return (a[2] - (a[0] >> 1) + (a[1] << 1)) & 255 }},
		{"a5", `fun a5(a: u8) -> u8 { return 5 - a + 3 }`, 1, func(a []int64) int64 { return (8 - a[0]) & 255 }},
		{"a8", `fun a8(a: u8, b: u8, c: u8) -> u8 { return b - a + (c << 1) }`, 3, func(a []int64) int64 { return (a[1] - a[0] + (a[2] << 1)) & 255 }},
		{"a9", `fun a9(a: u8, b: u8) -> u8 { let p: u8 = a >> 1
 return p + (a << 1) + b }`, 2, func(a []int64) int64 { return ((a[0] >> 1) + (a[0] << 1) + a[1]) & 255 }},
	}
}

// Two byte arguments are exhausted; additional arguments use 3 and 4.
// Complete Cartesian research sweeps remain in the immutable archive.
func TestNativeCompositionExecution(t *testing.T) {
	for _, tc := range nativeCompositionFixtures() {
		t.Run(tc.name, func(t *testing.T) {
			hm, err := nanz.Parse(tc.src, "critic.nanz")
			if err != nil {
				t.Fatal(err)
			}
			count := 65536
			if tc.arity == 1 {
				count = 256
			}
			inputs := func(i int) []int64 {
				a := []int64{int64(i & 255), int64((i >> 8) & 255), 3, 4}
				return a[:tc.arity]
			}
			bad, first := sweepCodegenJudge(t, hm.Funcs, tc.name, count, inputs, tc.model)
			if bad != 0 {
				t.Fatalf("native %d/%d mismatches (%s)", bad, count, first)
			}
		})
	}
}
