package mir2gpu_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2gpu"
)

func TestDispatch_CUDA_RealGPU(t *testing.T) {
	if _, err := exec.LookPath("nvcc"); err != nil {
		t.Skip("nvcc not found")
	}

	// sum = reduce(double, 0..256) — each thread computes double(tid), host sums
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

	results := mir2gpu.DispatchAll(m, mir2gpu.DispatchOptions{Backend: mir2gpu.CUDA})
	if len(results) == 0 {
		t.Fatal("no dispatches")
	}
	if results[0].Error != nil {
		t.Fatal(results[0].Error)
	}

	kernelCode := results[0].KernelCode
	numThreads := results[0].Candidate.NumThreads

	// Wrap kernel in standalone CUDA program
	hostCode := fmt.Sprintf(`%s

#include <stdio.h>
#include <stdlib.h>

int main() {
    const int N = %d;
    uint32_t *d_results, *h_results;
    h_results = (uint32_t*)malloc(N * sizeof(uint32_t));
    cudaMalloc(&d_results, N * sizeof(uint32_t));
    cudaMemset(d_results, 0, N * sizeof(uint32_t));

    reduce_apply_all<<<(N+255)/256, 256>>>(d_results, N);
    cudaDeviceSynchronize();
    cudaMemcpy(h_results, d_results, N * sizeof(uint32_t), cudaMemcpyDeviceToHost);

    // Print first 16
    for (int i = 0; i < 16 && i < N; i++) {
        printf("double(%%d) = %%u\n", i, h_results[i]);
    }

    // Verify: double(i) = (i*2) & 0xFF
    int correct = 0;
    for (int i = 0; i < N; i++) {
        if (h_results[i] == (uint32_t)((i * 2) & 0xFF)) correct++;
    }
    printf("Map phase correct: %%d/%%d\n", correct, N);

    // Reduce: sum all results (on host)
    uint32_t sum = 0;
    for (int i = 0; i < N; i++) {
        sum += h_results[i];
    }
    printf("Reduce sum(double(0..%%d)) = %%u\n", N, sum);

    cudaFree(d_results);
    free(h_results);
    return correct == N ? 0 : 1;
}
`, kernelCode, numThreads)

	// Write, compile, run
	cuFile := "/tmp/minz_reduce_test.cu"
	binFile := "/tmp/minz_reduce_test"
	os.WriteFile(cuFile, []byte(hostCode), 0644)

	cmd := exec.Command("nvcc", "-o", binFile, cuFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Source:\n%s", hostCode)
		t.Fatalf("nvcc failed: %s\n%s", err, output)
	}

	cmd = exec.Command(binFile)
	output, err = cmd.CombinedOutput()
	t.Logf("GPU output:\n%s", string(output))
	if err != nil {
		t.Fatalf("execution failed: %s", err)
	}

	if !strings.Contains(string(output), "Map phase correct: 256/256") {
		t.Error("map phase failed")
	}
}
