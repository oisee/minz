package nanz_test

import (
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// The README animation must come from the current Nanz source, not an old render.
func TestRotozoomerGallery(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/rotozoomer.nanz")
	if err != nil {
		t.Fatal(err)
	}
	hm, err := nanz.Parse(string(source), "rotozoomer.nanz")
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(hm))
	canvas := mir2.RegisterCanvasHosts(vm)

	file, err := os.Open("../../../media/rotozoomer.gif")
	if err != nil {
		t.Fatal(err)
	}
	animation, err := gif.DecodeAll(file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(animation.Image) != 14 {
		t.Fatalf("GIF has %d frames, want 14", len(animation.Image))
	}

	for _, frame := range []int64{0, 3, 7} {
		if _, err := vm.Call("render_frame", []mir2.Value{{I: frame}}); err != nil {
			t.Fatalf("frame %d: %v", frame, err)
		}
		path := filepath.Join(t.TempDir(), "frame.png")
		if err := canvas.C.SavePNG(path); err != nil {
			t.Fatal(err)
		}
		pngFile, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := png.Decode(pngFile)
		pngFile.Close()
		if err != nil {
			t.Fatal(err)
		}
		want := animation.Image[frame]
		if got.Bounds() != want.Bounds() {
			t.Fatalf("frame %d bounds: got %v, want %v", frame, got.Bounds(), want.Bounds())
		}
		for y := 0; y < 192; y++ {
			for x := 0; x < 256; x++ {
				gr, gg, gb, _ := got.At(x, y).RGBA()
				wr, wg, wb, _ := want.At(x, y).RGBA()
				if gr != wr || gg != wg || gb != wb {
					t.Fatalf("frame %d differs from GIF at (%d,%d): got %d,%d,%d want %d,%d,%d", frame, x, y, gr, gg, gb, wr, wg, wb)
				}
			}
		}
	}
}
