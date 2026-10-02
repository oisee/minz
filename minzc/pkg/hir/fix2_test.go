package hir_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

func TestFix2SignedSameWidthCast(t *testing.T) {
	for _, tc := range []struct {
		op string
		k  int
	}{{"/", 2}, {"/", 4}, {"%", 4}, {"%", 8}} {
		t.Run(fmt.Sprintf("%s%d", tc.op, tc.k), func(t *testing.T) {
			src := fmt.Sprintf("fun cast_arith(x: u8, unused: u8) -> i8 { return ((x as i8) %s %d) }", tc.op, tc.k)
			hm, err := nanz.Parse(src, "cast.nanz")
			if err != nil {
				t.Fatal(err)
			}
			fix := compileProductionHIRFixture(t, hm)
			checked, bad := newU8Judge(t, fix, "cast_arith", false).sweep(func(a, b uint8) bool { return b == 0 }, func(a, b uint8) int64 {
				x := int16(int8(a))
				k := int16(tc.k)
				if tc.op == "/" {
					return int64(uint8(x / k))
				}
				return int64(uint8(x % k))
			})
			if checked != 256 || len(bad) > 0 {
				t.Fatalf("%d checked:%s\n%s", checked, describeMismatches(bad), fix.asm)
			}
		})
	}
}
