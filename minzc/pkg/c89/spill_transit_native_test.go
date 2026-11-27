package c89_test

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

// spillTransitCase is a C function checked on the emulator against a Go
// reference for every value of its first byte argument.
type spillTransitCase struct {
	name, fn, src string
	others        [][]int64
	ref           func(args []int64) int64
}

func runSpillTransitCases(t *testing.T, cases []spillTransitCase) {
	t.Helper()
	for _, tc := range cases {
		for _, size := range []bool{false, true} {
			t.Run(tc.name, func(t *testing.T) {
				m, err := c89.CompileWithOpts(tc.src, tc.name+".c", c89.CompileOpts{NoLibc: true})
				if err != nil {
					t.Fatal(err)
				}
				steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: true, OptSize: size, AssertMode: "none"})
				if err != nil {
					t.Fatal(err)
				}
				m.Asserts = nil
				for v := int64(0); v < 256; v++ {
					for _, rest := range tc.others {
						args := append([]int64{v}, rest...)
						m.Asserts = append(m.Asserts, hir.Assert{FuncName: tc.fn, Args: args, Expected: tc.ref(args)})
					}
				}
				if err := pipeline.RunAssertsZ80(m, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
					t.Fatalf("size=%v: %v", size, err)
				}
			})
		}
	}
}

// A spilled byte loaded into a register other than A goes through A, the only
// register LD r,(nn) can name. It used to overwrite a live A: the shifted
// spill in each loop below destroyed y (first case) or w2 (second case).
func TestC89SpillByteLoadPreservesAccumulatorNative(t *testing.T) {
	runSpillTransitCases(t, []spillTransitCase{
		{"popcount-pressure", "f3", `unsigned char f3(unsigned char a, unsigned char b) {
    unsigned char x = a, y = b, z = a ^ 0x5a, r, n = b & 3;
    unsigned short c = 0, h = 1;
    if (n) for (r = 0; r < n; r++) { z |= ((z + 102) & (z | y)); h += x & 1; c += z & 1; z = z >> 1; }
    return (unsigned char)(c + h + x + (y << 3) + (z << 1));
}`, [][]int64{{0}, {1}, {2}, {3}, {102}, {255}}, func(a []int64) int64 {
			x, y, z := uint8(a[0]), uint8(a[1]), uint8(a[0])^0x5a
			c, h := uint16(0), uint16(1)
			for r := uint8(0); r < y&3; r++ {
				z |= (z + 102) & (z | y)
				h += uint16(x & 1)
				c += uint16(z & 1)
				z >>= 1
			}
			return int64(uint8(c + h + uint16(x) + uint16(y)<<3 + uint16(z)<<1))
		}},
		{"counted-shift", "h", `unsigned short h(unsigned char v, unsigned char k, unsigned char n) {
    unsigned char r, bit, w, w2 = 0, t = 0, q2 = k + 1;
    unsigned short c = 0;
    for (r = 0; r < n; r++) { c = 0; w = v; for (bit = 0; bit < 8; bit++) { c += w & 1; t = t + k; w = w >> 1; } }
    return c + t + w2 + k;
}`, [][]int64{{200, 1}, {9, 3}, {60, 2}, {0, 1}, {255, 2}}, func(a []int64) int64 {
			v, k, n := uint8(a[0]), uint8(a[1]), uint8(a[2])
			var c uint16
			var t, w uint8
			for r := uint8(0); r < n; r++ {
				c, w = 0, v
				for bit := 0; bit < 8; bit++ {
					c += uint16(w & 1)
					t += k
					w >>= 1
				}
			}
			return int64(c + uint16(t) + uint16(k))
		}},
	})
}

// A byte copied into a word-typed block parameter homed in a spill slot (the
// C ternary below joins an unsigned char into an int-typed parameter) must
// write the zero extension into both bytes of the slot without touching A.
// It used to store the low byte through A, destroying the live y in A, and
// left the high byte stale.
func TestC89ByteToWordSpillCopyNative(t *testing.T) {
	runSpillTransitCases(t, []spillTransitCase{
		{"ternary-join", "f3", `unsigned short f3(unsigned char a, unsigned char b) {
    unsigned char x = a, y = b, z = a ^ 0x5a, r, i, j;
    unsigned short c = 0, h = 1;
    for (r = 0; r < 1; r++) { for (i = 250; i < 255; i++) { y += (i << 1); } for (j = 0; j < 7; j++) { z |= (x & ((y & 1) ? x : y)); h = (h << 1) ^ 655; } }
    return c + h + x + (y << 3) + (unsigned short)(z * 7);
}`, [][]int64{{0}, {1}, {103}, {178}, {255}}, func(a []int64) int64 {
			x, y, z := uint8(a[0]), uint8(a[1]), uint8(a[0])^0x5a
			h := uint16(1)
			for i := 250; i < 255; i++ {
				y += uint8(i << 1)
			}
			for j := 0; j < 7; j++ {
				sel := y
				if y&1 != 0 {
					sel = x
				}
				z |= x & sel
				h = h<<1 ^ 655
			}
			return int64(h + uint16(x) + uint16(y)<<3 + uint16(z)*7)
		}},
	})
}
