//go:build mzx_headless

package main

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/spectrum"
)

// Final VRAM cannot reproduce either border timing or the pixels that the ULA
// fetched before a mid-frame write. Capture must use the executed frame.
func TestHeadlessScreenshotPreservesFetchedFrame(t *testing.T) {
	m, err := spectrum.New48K(embedded48ROM)
	if err != nil {
		t.Fatal(err)
	}
	fb := m.Framebuffer()
	border := color.RGBA{255, 0, 0, 255}
	screen := color.RGBA{0, 255, 255, 255}
	for i := 0; i < len(fb); i += 4 {
		copy(fb[i:i+4], []byte{border.R, border.G, border.B, border.A})
	}
	off := (m.Mode.BorderTop*m.ScreenWidth() + m.Mode.BorderLeft) * 4
	copy(fb[off:off+4], []byte{screen.R, screen.G, screen.B, screen.A})
	// These bytes would render a black pixel if the screenshot rebuilt VRAM.
	m.Memory.Write(0x4000, 0, false)
	m.Memory.Write(0x5800, 0, false)
	for _, crop := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "frame.png")
		saveScreenPNG(m, path, crop)
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		w, h := m.ScreenWidth(), m.ScreenHeight()
		x, y := m.Mode.BorderLeft, m.Mode.BorderTop
		if crop {
			w, h = 256, 192
			x, y = 0, 0
		} else if got := color.RGBAModel.Convert(img.At(0, 0)); got != border {
			t.Fatalf("border=%v", got)
		}
		if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
			t.Fatalf("bounds=%v", img.Bounds())
		}
		if got := color.RGBAModel.Convert(img.At(x, y)); got != screen {
			t.Fatalf("fetched pixel=%v want=%v", got, screen)
		}
	}
}
