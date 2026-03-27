package mir2gpu_test

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2gpu"
	"github.com/minz/minzc/pkg/nanz"
)

func TestMetal_GenerateFiles(t *testing.T) {
	src := `fun double(x: u8) -> u8 { return x + x }`

	hm, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}

	metalSrc, swiftHost, err := mir2gpu.StandaloneMetal(m, "double", 256)
	if err != nil {
		t.Fatal(err)
	}

	// Write files for Mac testing
	os.WriteFile("/tmp/minz_metal_test.metal", []byte(metalSrc), 0644)
	os.WriteFile("/tmp/minz_metal_host.swift", []byte(swiftHost), 0644)

	t.Logf("Metal shader: %d bytes → /tmp/minz_metal_test.metal", len(metalSrc))
	t.Logf("Swift host: %d bytes → /tmp/minz_metal_host.swift", len(swiftHost))
	t.Logf("\nTo run on Mac:\n  swiftc -framework Metal -framework Foundation /tmp/minz_metal_host.swift -o /tmp/metal_test && /tmp/metal_test")
}
