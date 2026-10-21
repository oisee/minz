package pipeline

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestDeadForwardedLoopArgNative(t *testing.T) {
	const src = `unsigned short fib(unsigned char k) {
 unsigned short a=0,b=1,t; unsigned char i;
 for(i=0;i<k;i++){t=a+b;a=b;b=t;} return a; }`
	for _, mode := range []string{"default", "size", "grace"} {
		t.Run(mode, func(t *testing.T) {
			h, err := c89.CompileWithOpts(src, "deadarg.c", c89.CompileOpts{NoLibc: true})
			if err != nil {
				t.Fatal(err)
			}
			s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseHIRGrace: mode == "grace", UseGrace: mode == "grace", OptSize: mode == "size", AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("assembly:\n%s", s.Assembly)
			vm := mir2.NewVM(s.MIR2Module)
			for _, n := range []int64{0, 1, 2, 10, 20, 255} {
				a, b := uint16(0), uint16(1)
				for i := int64(0); i < n; i++ {
					a, b = b, a+b
				}
				got, err := vm.Call("fib", []mir2.Value{{I: n}})
				if err != nil || len(got) != 1 || got[0].I != int64(a) {
					t.Fatal(n, got, err, a)
				}
				cycles, size := measureGraceZ80Compiled(t, h, s, "fib", []int64{n}, nil, int64(a), 100000)
				t.Logf("fib(%d) T=%d bytes=%d", n, cycles, size)
			}
		})
	}
}
