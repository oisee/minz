package c89_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"

	c89 "github.com/minz/minzc/pkg/c89"
)

// TestPlasmaRender compiles the ObjC plasma demo and renders PNGs.
func TestPlasmaRender(t *testing.T) {
	srcPath := filepath.Join("..", "..", "..", "examples", "objc", "plasma.m")
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Skipf("plasma.m not found: %v", err)
	}

	hm, err := c89.Compile(string(src), "plasma.m")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		mir2.PropagateConstants(f)
		mir2.FoldConstants(f)
	}

	vm := mir2.NewVM(m)
	vm.MaxSteps = 50_000_000 // plasma is compute-heavy
	ref := mir2.RegisterCanvasHosts(vm)

	// Init canvas via host function directly.
	_, err = vm.Hosts["canvas_init"]([]mir2.Value{{I: 0}, {I: 0}, {I: 0}})
	if err != nil {
		t.Fatalf("canvas_init: %v", err)
	}

	// Setup plasma palette — this IS a module function.
	_, err = vm.Call("setup_plasma_palette", nil)
	if err != nil {
		t.Fatalf("setup_plasma_palette: %v", err)
	}
	t.Logf("canvas: %dx%d", ref.C.W, ref.C.H)

	outDir := filepath.Join("..", "..", "..", "examples", "objc", "output")
	os.MkdirAll(outDir, 0755)

	// Render each effect by calling ClassName_render(self, t).
	// Struct layout: __vtable(2) + field1(2) + field2(2)
	effects := []struct {
		name   string
		fn     string
		field1 int16 // scale/spacing/shift
		field2 int16 // speed/dummy
		t      int64
	}{
		{"plasma", "Plasma_render", 8, 2, 42},
		{"diamond", "Diamond_render", 4, 0, 0},
		{"xor", "XorPattern_render", 0, 0, 10},
	}

	for _, eff := range effects {
		// Allocate object on heap: [vtable:u16][field1:u16][field2:u16]
		obj := make([]byte, 6)
		binary.LittleEndian.PutUint16(obj[0:], 0)                   // __vtable (unused for static)
		binary.LittleEndian.PutUint16(obj[2:], uint16(eff.field1))
		binary.LittleEndian.PutUint16(obj[4:], uint16(eff.field2))
		self := vm.AllocHeap(obj)

		_, err = vm.Call(eff.fn, []mir2.Value{self, {I: eff.t}})
		if err != nil {
			t.Fatalf("%s: %v", eff.name, err)
		}

		pngPath := filepath.Join(outDir, eff.name+".png")
		if err := ref.C.SavePNG(pngPath); err != nil {
			t.Fatalf("SavePNG %s: %v", eff.name, err)
		}
		info, _ := os.Stat(pngPath)
		t.Logf("  %s → %s (%d bytes)", eff.name, pngPath, info.Size())
	}
}
