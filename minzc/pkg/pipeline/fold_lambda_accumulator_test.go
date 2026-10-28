package pipeline

import (
	"encoding/binary"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// The prescribed fold/filter source: a u16 accumulator folded over a filtered
// byte array with a *lambda* callback. The frontend's `let`/fold type inference
// only resolved the accumulator type for a *named* callback, so `let s` fell
// back to the u8 default and the u16 sum was truncated at the binding.
const foldLambdaAccumulatorSource = `global a: [u8; 4]
global n: u16
global r: u16
fun execute() {
    let s = a.filter(|x: u8| x >= 128).fold(u16(0), |acc: u16, x: u8| acc + u16(x), n)
    r = s
}
`

// runFoldLambdaGlobalsVM executes() on the MIR2 VM with the a/n globals seeded
// and returns the u16 written to r.
func runFoldLambdaGlobalsVM(t *testing.T, m *mir2.Module, data []byte, n int) uint16 {
	t.Helper()
	vm := mir2.NewVM(m)
	vm.WriteHeapSlice(vm.GlobalAddr("a"), data)
	vm.WriteHeapSlice(vm.GlobalAddr("n"), []byte{byte(n), byte(n >> 8)})
	if _, err := vm.Call("execute", nil); err != nil {
		t.Fatal(err)
	}
	return binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("r"), 2))
}

// The lambda and named-callback forms must agree: both fold a u16 accumulator.
func TestFoldLambdaAccumulatorTypeVMZ80(t *testing.T) {
	mixed := []byte{200, 5, 255, 128} // 5 is filtered out
	cases := []struct {
		name string
		data []byte
		n    int
		sum  uint16
	}{
		{"all_pass_n4", []byte{128, 200, 255, 129}, 4, 128 + 200 + 255 + 129}, // 712: > 0xFF
		{"none_pass_n4", []byte{0, 127, 5, 100}, 4, 0},
		{"mixed_pass_n4", mixed, 4, 200 + 255 + 128},
		{"mixed_pass_n2", mixed, 2, 200}, // 5 filtered out
		{"mixed_pass_n0", mixed, 0, 0},
	}
	for _, src := range []struct {
		name string
		text string
	}{
		{"lambda_callback", foldLambdaAccumulatorSource},
		{"named_callback", foldFilterStageNamedSource},
	} {
		t.Run(src.name, func(t *testing.T) {
			h := graceMeasureParse(t, src.text)
			steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			// Both the plainly lowered and the fully compiled module: the
			// accumulator type is a frontend decision, so both must agree.
			vms := []*mir2.Module{hir.LowerModule(h), steps.MIR2Module}
			for _, c := range cases {
				for mi, m := range vms {
					if got := runFoldLambdaGlobalsVM(t, m, c.data, c.n); got != c.sum {
						t.Errorf("%s: VM(%d) r = %d, want %d (data=%v n=%d)", c.name, mi, got, c.sum, c.data, c.n)
					}
				}
				if got := runExecuteGlobalsZ80(t, steps, c.data, c.n); got != c.sum {
					t.Errorf("%s: Z80 r = %d, want %d (data=%v n=%d)", c.name, got, c.sum, c.data, c.n)
				}
			}
		})
	}
}
