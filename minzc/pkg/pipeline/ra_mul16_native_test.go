package pipeline

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestMul16AccumulatorNative(t *testing.T) {
	const src = `unsigned short acc(unsigned char cnt) {
 unsigned short s=0; unsigned char k;
 for(k=0;k<cnt;k++) s+=k*5; return s; }`
	for _, mode := range []string{"default", "size", "grace"} {
		t.Run(mode, func(t *testing.T) {
			h, err := c89.CompileWithOpts(src, "acc.c", c89.CompileOpts{NoLibc: true})
			if err != nil {
				t.Fatal(err)
			}
			s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseHIRGrace: mode == "grace", UseGrace: mode == "grace", OptSize: mode == "size", AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("assembly:\n%s", s.Assembly)
			vm := mir2.NewVM(s.MIR2Module)
			for _, n := range []int64{0, 1, 2, 10, 200, 255} {
				want := int64(uint16(5 * n * (n - 1) / 2))
				got, err := vm.Call("acc", []mir2.Value{{I: n}})
				if err != nil || got[0].I != want {
					t.Fatal(n, got, err, want)
				}
				cycles, size := measureGraceZ80Compiled(t, h, s, "acc", []int64{n}, nil, want, 100000)
				t.Logf("acc(%d) T=%d bytes=%d", n, cycles, size)
			}
		})
	}
}
