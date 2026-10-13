package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestMul16SmallConstantSourceVMZ80(t *testing.T) {
	for _, constant := range []int{10, 12} {
		src := fmt.Sprintf("fun multiply(x: u16) -> u16 { return x * %d }", constant)
		for _, input := range []int64{0, 1, 255, 256, 5461, 6553, 32767, 32768, 65534, 65535} {
			for _, grace := range []bool{false, true} {
				t.Run(fmt.Sprintf("k%d/x%d/grace%v", constant, input, grace), func(t *testing.T) {
					want := input * int64(constant) & 65535
					h := graceMeasureParse(t, src)
					steps, err := CompileHIRSteps(h, Options{AssertMode: "none", UseHIRGrace: grace})
					if err != nil {
						t.Fatal(err)
					}
					got, err := mir2.NewVM(steps.MIR2Module).Call("multiply", []mir2.Value{{I: input}})
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("VM got%v err%v want%d", got, err, want)
					}
					cycles, bytes := measureGraceZ80HIR(t, h, "multiply", Options{AssertMode: "none", UseHIRGrace: grace}, []int64{input}, nil, want)
					t.Logf("T=%d bytes=%d want=%d", cycles, bytes, want)
				})
			}
		}
	}
}
