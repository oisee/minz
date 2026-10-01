package nanz_test

import (
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// TestCanvasImplShowcase renders shapes using impl blocks and exports PNG.
func TestCanvasImplShowcase(t *testing.T) {
	srcBytes, err := os.ReadFile("../../../examples/nanz/canvas_house.nanz")
	if err != nil {
		t.Fatalf("read showcase: %v", err)
	}
	src := string(srcBytes)
	hirMod, err := nanz.Parse(src, "canvas_showcase.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Lower HIR → MIR2
	mir2mod := hir.LowerModule(hirMod)

	// Run on VM with canvas hosts
	vm := mir2.NewVM(mir2mod)
	canvasRef := mir2.RegisterCanvasHosts(vm)
	_, err = vm.Call("draw_scene", nil)
	if err != nil {
		t.Fatalf("draw_scene: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "canvas_house.png")
	if canvasRef.C != nil {
		if err := canvasRef.C.SavePNG(outPath); err != nil {
			t.Fatalf("save PNG: %v", err)
		}
		gotFile, err := os.Open(outPath)
		if err != nil {
			t.Fatal(err)
		}
		got, err := png.Decode(gotFile)
		gotFile.Close()
		if err != nil {
			t.Fatal(err)
		}
		wantFile, err := os.Open("../../../media/canvas_house.png")
		if err != nil {
			t.Fatal(err)
		}
		want, err := png.Decode(wantFile)
		wantFile.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("Nanz canvas render differs from committed PNG")
		}
		info, _ := os.Stat(outPath)
		t.Logf("canvas matches committed PNG (%d bytes)", info.Size())
	} else {
		t.Fatal("draw_scene did not initialize a canvas")
	}
}
