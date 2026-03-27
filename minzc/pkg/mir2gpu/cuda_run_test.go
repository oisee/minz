package mir2gpu_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2gpu"
	"github.com/minz/minzc/pkg/nanz"
)

func TestCUDA_RealGPU_Double(t *testing.T) {
	if _, err := exec.LookPath("nvcc"); err != nil {
		t.Skip("nvcc not found")
	}

	src := `fun double(x: u8) -> u8 { return x + x }`

	hm, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}

	// Generate standalone CUDA
	code, err := mir2gpu.StandaloneCUDA(m, "double", 256)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Generated CUDA:\n%s", code)

	// Write to temp file
	cuFile := "/tmp/minz_mir2gpu_test.cu"
	binFile := "/tmp/minz_mir2gpu_test"
	if err := os.WriteFile(cuFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Compile with nvcc
	cmd := exec.Command("nvcc", "-o", binFile, cuFile, "-arch=sm_50")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nvcc failed: %s\n%s", err, string(output))
	}
	t.Logf("nvcc: compiled OK")

	// Run on GPU
	cmd = exec.Command(binFile)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("GPU run failed: %s\n%s", err, string(output))
	}

	result := string(output)
	t.Logf("GPU output:\n%s", result)

	// Verify: double(i) == i*2 for all 256 values
	if !strings.Contains(result, "Correct: 256/256") {
		t.Errorf("Expected 256/256 correct, got:\n%s", result)
	}
}
