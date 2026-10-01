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
	module := hir.LowerModule(hm)

	file, err := os.Open("../../../media/rotozoomer.gif")
	if err != nil {
		t.Fatal(err)
	}
	animation, err := gif.DecodeAll(file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(animation.Image) != 32 {
		t.Fatalf("GIF has %d frames, want 32", len(animation.Image))
	}
	if animation.LoopCount != 0 {
		t.Fatalf("GIF loop count is %d, want infinite (0)", animation.LoopCount)
	}
	for frame, delay := range animation.Delay {
		if delay != 16 {
			t.Fatalf("frame %d delay is %d centiseconds, want 16", frame, delay)
		}
	}

	// Frame 32 must equal frame 0: the source wraps its phase instead of bouncing.
	for _, frame := range []int64{0, 7, 16, 24, 31, 32} {
		vm := mir2.NewVM(module)
		canvas := mir2.RegisterCanvasHosts(vm)
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
		want := animation.Image[frame%32]
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
