package nanz_test

import (
	"encoding/json"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// The current-source VM and README animation must match an independent signed16 model.
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

	frozen, err := os.ReadFile("../../../reports/rotozoomer-pure/freeze.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		A []int `json:"axis_a"`
		B []int `json:"axis_b"`
	}
	if err = json.Unmarshal(frozen, &spec); err != nil {
		t.Fatal(err)
	}
	palette := []color.RGBA{{8, 9, 30, 255}, {27, 41, 83, 255}, {27, 123, 137, 255}, {229, 64, 117, 255}, {255, 166, 95, 255}, {255, 240, 178, 255}}
	abs := func(x int16) int16 {
		if x < 0 {
			return -x
		}
		return x
	}
	tex := func(x, y int16) int {
		d := abs(x-8) + abs(y-8)
		if d < 3 {
			return 5
		}
		if d < 6 {
			return 4
		}
		if (x/4+y/4)%2 == 0 {
			return 3
		}
		if (x^y)&2 == 0 {
			return 2
		}
		return 1
	}
	// Frame 32 must equal frame 0: the source wraps its phase instead of bouncing.
	for frame := int64(0); frame <= 32; frame++ {
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
		a, b := int16(spec.A[frame%32]), int16(spec.B[frame%32])
		urow, vrow := int16(18432)-a*128-b*96, int16(18432)+b*128-a*96
		for y := 0; y < 192; y++ {
			u, v := urow, vrow
			for x := 0; x < 256; x++ {
				expected := palette[tex((u/256)%16, (v/256)%16)]
				u, v = u+a, v-b
				er, eg, eb, _ := expected.RGBA()
				gr, gg, gb, _ := got.At(x, y).RGBA()
				wr, wg, wb, _ := want.At(x, y).RGBA()
				if gr != er || gg != eg || gb != eb || wr != er || wg != eg || wb != eb {
					t.Fatalf("frame%d (%d,%d) VM/GIF differ from independent signed16 model", frame, x, y)
				}
				if gr != wr || gg != wg || gb != wb {
					t.Fatalf("frame %d differs from GIF at (%d,%d): got %d,%d,%d want %d,%d,%d", frame, x, y, gr, gg, gb, wr, wg, wb)
				}
			}
			urow, vrow = urow+b, vrow+a
		}
	}
}
