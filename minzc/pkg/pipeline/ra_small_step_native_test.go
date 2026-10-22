package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func TestSmallStepFillNative(t *testing.T) {
	const src = `fun fill(p: ^u8, count: u8, value: u8) -> u8 {
 for i: u8 in 0..count { p[i] = value }
 return p[0] + p[count - 1]
 }`
	for _, mode := range []string{"default", "size", "grace"} {
		t.Run(mode, func(t *testing.T) {
			h, err := nanz.Parse(src, "fill.nanz")
			if err != nil {
				t.Fatal(err)
			}
			s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseHIRGrace: mode == "grace", UseGrace: mode == "grace", OptSize: mode == "size", AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(s.Assembly, "INC ") {
				t.Fatal("no increment", s.Assembly)
			}
			if strings.Contains(s.Assembly, "LD A, 1\n") {
				t.Fatal("small constant materialized", s.Assembly)
			}
			for _, n := range []int64{1, 2, 10, 255} {
				for _, v := range []int64{0, 1, 127, 255} {
					vm := mir2.NewVM(s.MIR2Module)
					vm.EnsureHeap(0x4100)
					vm.WriteHeapSlice(0x4000, make([]byte, 256))
					args := []int64{0x4000, n, v}
					got, err := vm.Call("fill", []mir2.Value{{I: args[0]}, {I: n}, {I: v}})
					want := int64(byte(2 * v))
					if err != nil || got[0].I != want {
						t.Fatal(n, v, got, err)
					}
					for _, b := range vm.ReadHeap(0x4000, int(n)) {
						if b != byte(v) {
							t.Fatal("VM store", b, v)
						}
					}
					cycles, size := measureGraceZ80Compiled(t, h, s, "fill", args, nil, want, 100000)
					t.Logf("fill(%d,%d) T=%d bytes=%d", n, v, cycles, size)
				}
			}
		})
	}
}
