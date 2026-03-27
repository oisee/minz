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

func TestOpenCL_RealGPU_Double(t *testing.T) {
	// Check OpenCL available
	if _, err := os.Stat("/usr/lib/x86_64-linux-gnu/libOpenCL.so"); err != nil {
		t.Skip("OpenCL not available")
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

	code, err := mir2gpu.StandaloneOpenCL(m, "double", 256)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Generated OpenCL host: %d bytes", len(code))

	cFile := "/tmp/minz_opencl_test.c"
	binFile := "/tmp/minz_opencl_test"
	if err := os.WriteFile(cFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Compile with gcc + OpenCL
	cmd := exec.Command("gcc", "-o", binFile, cFile, "-lOpenCL", "-std=c99")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gcc failed: %s\n%s", err, string(output))
	}
	t.Logf("gcc: compiled OK")

	// Run
	cmd = exec.Command(binFile)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("OpenCL run failed: %s\n%s", err, string(output))
	}

	result := string(output)
	t.Logf("OpenCL output:\n%s", result)

	if !strings.Contains(result, "Correct: 256/256") {
		t.Errorf("Expected 256/256 correct, got:\n%s", result)
	}
}
