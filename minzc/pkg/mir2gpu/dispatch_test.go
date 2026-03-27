package mir2gpu_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2gpu"
)

func TestDispatch_ReduceWithCall(t *testing.T) {
	src := `
fun double(x: u8) -> u8 { return x + x }

fun apply_all() -> u8 {
	var sum: u8 = 0
	for i in 0..10 {
		sum = sum + double(i)
	}
	return sum
}
`
	m := parseMIR2(t, src)

	for _, backend := range []mir2gpu.Backend{mir2gpu.CUDA, mir2gpu.OpenCL, mir2gpu.Metal} {
		t.Run(backend.String(), func(t *testing.T) {
			results := mir2gpu.DispatchAll(m, mir2gpu.DispatchOptions{Backend: backend})
			t.Logf("Dispatches: %d", len(results))
			t.Logf("%s", mir2gpu.FormatDispatchResults(results))

			if len(results) == 0 {
				t.Fatal("expected at least 1 dispatch")
			}

			r := results[0]
			if r.Error != nil {
				t.Fatalf("dispatch error: %v", r.Error)
			}

			t.Logf("Kernel (%d bytes):\n%s", len(r.KernelCode), r.KernelCode)

			// Verify kernel contains the reduce function
			if !strings.Contains(r.KernelCode, "reduce_") {
				t.Error("kernel should contain reduce_ function")
			}
			if !strings.Contains(r.KernelCode, "fn_double") {
				t.Error("kernel should contain fn_double device function")
			}
		})
	}
}

func TestDispatch_InlineReduce(t *testing.T) {
	src := `
fun sum_to(n: u8) -> u8 {
	var result: u8 = 0
	var i: u8 = 0
	while i < n {
		result = result + i
		i = i + 1
	}
	return result
}
`
	m := parseMIR2(t, src)

	results := mir2gpu.DispatchAll(m, mir2gpu.DispatchOptions{Backend: mir2gpu.CUDA})
	if len(results) == 0 {
		t.Fatal("expected dispatch")
	}

	r := results[0]
	if r.Error != nil {
		t.Fatalf("dispatch error: %v", r.Error)
	}

	t.Logf("Kernel:\n%s", r.KernelCode)

	if r.Candidate.Kind != mir2gpu.GPUReduce {
		t.Errorf("expected reduce, got %s", r.Candidate.Kind)
	}
}
