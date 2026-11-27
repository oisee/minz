package c89_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

// A fused bit-accumulate step (SRL v / JR NC / INC acc) produces the shift's
// result at the add, while the MIR defines it at the later shr. An instruction
// between the two may relocate v's register (here x = q ^ 5 runs through A and
// saves A to a scratch byte). The shifted byte must follow that relocation and
// reach the shr's home; it used to stay in the scratch byte, which a later
// extension then reused, so h(5, 9, 3) returned 42 instead of 31.
func TestC89BitAccumShiftSurvivesRelocationNative(t *testing.T) {
	cases := []struct {
		name, src string
		ref       func(v, x, y uint16) uint16
	}{
		{"xor-between", `unsigned short h(unsigned char v, unsigned char x, unsigned char y) { unsigned short c = y;
  unsigned char q = x << 1, q2 = 3, w; c += v & 1; x = q ^ 5; w = v >> 1; c += w & 1; w = w >> 1;
  return c + w + x + y; }`, func(v, x, y uint16) uint16 {
			c := y
			q := uint8(x << 1)
			c += v & 1
			xx := q ^ 5
			w := uint8(v) >> 1
			c += uint16(w & 1)
			w >>= 1
			return c + uint16(w) + uint16(xx) + y
		}},
		{"add-between", `unsigned short h(unsigned char v, unsigned char x, unsigned char y) { unsigned short c = y;
  unsigned char q = y, w; c += v & 1; x = (q << 1) + y; w = v >> 1; c += w & 1; w = w >> 1;
  return c + w + x + y; }`, func(v, x, y uint16) uint16 {
			c := y
			q := uint8(y)
			c += v & 1
			xx := q<<1 + uint8(y)
			w := uint8(v) >> 1
			c += uint16(w & 1)
			w >>= 1
			return c + uint16(w) + uint16(xx) + y
		}},
	}
	for _, tc := range cases {
		for _, size := range []bool{false, true} {
			t.Run(tc.name, func(t *testing.T) {
				m, err := c89.CompileWithOpts(tc.src, "bit-accum.c", c89.CompileOpts{NoLibc: true})
				if err != nil {
					t.Fatal(err)
				}
				steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: true, OptSize: size, AssertMode: "none"})
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(steps.Assembly, ".bit_accum_") {
					t.Fatalf("size=%v: bit-accum fusion did not fire; the test no longer covers it", size)
				}
				m.Asserts = nil
				for v := 0; v < 256; v++ {
					for _, xy := range [][2]int{{9, 3}, {200, 17}, {60, 255}, {0, 0}, {255, 255}, {129, 16}} {
						m.Asserts = append(m.Asserts, hir.Assert{FuncName: "h",
							Args:     []int64{int64(v), int64(xy[0]), int64(xy[1])},
							Expected: int64(tc.ref(uint16(v), uint16(xy[0]), uint16(xy[1])))})
					}
				}
				if err := pipeline.RunAssertsZ80(m, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
					t.Fatalf("size=%v: %v", size, err)
				}
			})
		}
	}
}
