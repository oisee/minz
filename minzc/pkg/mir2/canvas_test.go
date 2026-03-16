package mir2_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestCanvas_HostFunctions(t *testing.T) {
	// Build a minimal module with a function that draws to the canvas.
	m := &mir2.Module{Name: "canvas_test"}

	f := m.AddFunc("draw_test")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")

	// canvas_init(256, 192, 0)
	w := b.Const(256, mir2.TyU16, mir2.ClassPointer)
	h := b.Const(192, mir2.TyU16, mir2.ClassPointer)
	mode := b.Const(0, mir2.TyU16, mir2.ClassPointer)
	b.Call("canvas_init", []mir2.Reg{w, h, mode}, mir2.TyVoid, 0, mir2.CallAttrs{})

	// canvas_clear(0)
	black := b.Const(0, mir2.TyU16, mir2.ClassPointer)
	b.Call("canvas_clear", []mir2.Reg{black}, mir2.TyVoid, 0, mir2.CallAttrs{})

	// canvas_pixel(10, 20, 7)
	x10 := b.Const(10, mir2.TyU16, mir2.ClassPointer)
	y20 := b.Const(20, mir2.TyU16, mir2.ClassPointer)
	c7 := b.Const(7, mir2.TyU16, mir2.ClassPointer)
	b.Call("canvas_pixel", []mir2.Reg{x10, y20, c7}, mir2.TyVoid, 0, mir2.CallAttrs{})

	// canvas_line(0, 0, 50, 50, 2)
	z := b.Const(0, mir2.TyU16, mir2.ClassPointer)
	v50 := b.Const(50, mir2.TyU16, mir2.ClassPointer)
	c2 := b.Const(2, mir2.TyU16, mir2.ClassPointer)
	b.Call("canvas_line", []mir2.Reg{z, z, v50, v50, c2}, mir2.TyVoid, 0, mir2.CallAttrs{})

	// canvas_rect(100, 50, 40, 30, 4)
	v100 := b.Const(100, mir2.TyU16, mir2.ClassPointer)
	v40 := b.Const(40, mir2.TyU16, mir2.ClassPointer)
	v30 := b.Const(30, mir2.TyU16, mir2.ClassPointer)
	c4 := b.Const(4, mir2.TyU16, mir2.ClassPointer)
	b.Call("canvas_rect", []mir2.Reg{v100, v50, v40, v30, c4}, mir2.TyVoid, 0, mir2.CallAttrs{})

	// canvas_circle(128, 96, 30, 1)
	v128 := b.Const(128, mir2.TyU16, mir2.ClassPointer)
	v96 := b.Const(96, mir2.TyU16, mir2.ClassPointer)
	c1 := b.Const(1, mir2.TyU16, mir2.ClassPointer)
	b.Call("canvas_circle", []mir2.Reg{v128, v96, v30, c1}, mir2.TyVoid, 0, mir2.CallAttrs{})

	// return canvas_get_pixel(10, 20)
	ret := b.Call("canvas_get_pixel", []mir2.Reg{x10, y20}, mir2.TyU16, mir2.ClassPointer, mir2.CallAttrs{})
	b.Ret(ret)

	m.Funcs = append(m.Funcs, f)

	// Run on VM with canvas hosts.
	vm := mir2.NewVM(m)
	ref := mir2.RegisterCanvasHosts(vm)

	rets, err := vm.Call("draw_test", nil)
	if err != nil {
		t.Fatalf("VM error: %v", err)
	}
	if len(rets) == 0 || rets[0].I != 7 {
		t.Fatalf("get_pixel(10,20) = %d, want 7", rets[0].I)
	}
	t.Logf("pixel(10,20) = %d (white), correct", rets[0].I)

	// Verify canvas was created.
	if ref.C == nil {
		t.Fatal("canvas not initialized")
	}
	t.Logf("canvas: %dx%d, %dbpp", ref.C.W, ref.C.H, ref.C.BPP)

	// Save PNG to temp dir.
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "shapes.png")
	if err := ref.C.SavePNG(pngPath); err != nil {
		t.Fatalf("SavePNG: %v", err)
	}
	info, _ := os.Stat(pngPath)
	t.Logf("saved %s (%d bytes)", pngPath, info.Size())

	// Verify some pixels.
	// (0,0) should be color 2 (red, from the line)
	if p := ref.C.GetPixelIdx(0, 0); p != 2 {
		t.Errorf("pixel(0,0) = %d, want 2 (line start)", p)
	}
	// (25,25) should be color 2 (red, midpoint of diagonal)
	if p := ref.C.GetPixelIdx(25, 25); p != 2 {
		t.Errorf("pixel(25,25) = %d, want 2 (line midpoint)", p)
	}
}
