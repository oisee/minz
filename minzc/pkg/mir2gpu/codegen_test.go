package mir2gpu_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2gpu"
	"github.com/minz/minzc/pkg/nanz"
)

func TestCompile_AllBackends(t *testing.T) {
	src := `
fun add(a: u8, b: u8) -> u8 { return a + b }
fun double(x: u8) -> u8 { return x + x }
fun max(a: u8, b: u8) -> u8 {
    if a > b { return a }
    return b
}
`
	hm, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}

	backends := []mir2gpu.Backend{
		mir2gpu.CUDA,
		mir2gpu.OpenCL,
		mir2gpu.Vulkan,
		mir2gpu.Metal,
	}

	for _, backend := range backends {
		t.Run(backend.String(), func(t *testing.T) {
			code, err := mir2gpu.Compile(m, mir2gpu.CompileOptions{Backend: backend})
			if err != nil {
				t.Fatal(err)
			}
			if len(code) == 0 {
				t.Fatal("empty output")
			}
			t.Logf("%s output: %d bytes", backend, len(code))
			fmt.Println(code[:min(len(code), 300)])
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
