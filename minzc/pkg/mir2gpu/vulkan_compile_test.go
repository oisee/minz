package mir2gpu_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2gpu"
	"github.com/minz/minzc/pkg/nanz"
)

func TestVulkan_CompileShader(t *testing.T) {
	if _, err := exec.LookPath("glslangValidator"); err != nil {
		t.Skip("glslangValidator not found")
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

	code, err := mir2gpu.Compile(m, mir2gpu.CompileOptions{Backend: mir2gpu.Vulkan})
	if err != nil {
		t.Fatal(err)
	}

	compFile := "/tmp/minz_vulkan_test.comp"
	spvFile := "/tmp/minz_vulkan_test.spv"
	if err := os.WriteFile(compFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Compile GLSL → SPIR-V
	cmd := exec.Command("glslangValidator", "-V", compFile, "-o", spvFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("GLSL source:\n%s", code)
		t.Fatalf("glslangValidator failed: %s\n%s", err, string(output))
	}

	info, _ := os.Stat(spvFile)
	t.Logf("Vulkan SPIR-V compiled: %d bytes", info.Size())
}
