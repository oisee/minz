// Canvas host functions for MIR2 VM — cross-language framebuffer with PNG export.
//
// Any frontend (C89, ObjC, Nanz, Lizp, Lanz, PL/M) can declare these as @extern
// and draw to the canvas. The VM-side handles pixel storage and PNG output.
//
// Host functions:
//   @canvas.init(w, h, mode)       → 0  (mode: 0=256-color, 1=RGB565, 2=RGB24)
//   @canvas.clear(color)           → 0
//   @canvas.pixel(x, y, color)     → 0
//   @canvas.line(x0, y0, x1, y1, color) → 0
//   @canvas.rect(x, y, w, h, color)     → 0
//   @canvas.fill_rect(x, y, w, h, color) → 0
//   @canvas.circle(cx, cy, r, color)     → 0
//   @canvas.get_pixel(x, y)        → color
//   @canvas.save(path_ptr)         → 0 on success, -1 on error
//   @canvas.width()                → width
//   @canvas.height()               → height
//   @canvas.palette(index, r, g, b) → 0  (set palette entry for 256-color mode)
//
// Video modes (matching retro targets):
//   Mode 0: 256×192, 8bpp indexed (ZX Spectrum-like, 15 colors)
//   Mode 1: 320×200, 8bpp indexed (CGA/VGA Mode 13h)
//   Mode 2: 320×240, 16bpp RGB565
//   Mode 3: 640×480, 8bpp indexed (VGA)
//   Mode 4: custom (use w,h params directly, 8bpp)
package mir2

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

// Canvas is a framebuffer managed by the VM host.
type Canvas struct {
	W, H    int
	BPP     int // 8, 16, or 24
	Pixels  []byte
	Palette [256]uint32 // for 8bpp indexed mode
}

// Predefined video modes.
var canvasModes = map[int64][3]int{
	0: {256, 192, 8},  // ZX Spectrum
	1: {320, 200, 8},  // VGA Mode 13h
	2: {320, 240, 16}, // QVGA RGB565
	3: {640, 480, 8},  // VGA
	// 4 = custom: use w,h args, 8bpp
}

func newCanvas(w, h, bpp int) *Canvas {
	c := &Canvas{W: w, H: h, BPP: bpp}
	bytesPerPixel := bpp / 8
	if bytesPerPixel < 1 {
		bytesPerPixel = 1
	}
	c.Pixels = make([]byte, w*h*bytesPerPixel)
	// Default palette: ZX Spectrum 16 colors + gray ramp
	zxColors := []uint32{
		0xFF000000, // 0: black
		0xFF0000D7, // 1: blue
		0xFFD70000, // 2: red
		0xFFD700D7, // 3: magenta
		0xFF00D700, // 4: green
		0xFF00D7D7, // 5: cyan
		0xFFD7D700, // 6: yellow
		0xFFD7D7D7, // 7: white
		0xFF000000, // 8: bright black
		0xFF0000FF, // 9: bright blue
		0xFFFF0000, // 10: bright red
		0xFFFF00FF, // 11: bright magenta
		0xFF00FF00, // 12: bright green
		0xFF00FFFF, // 13: bright cyan
		0xFFFFFF00, // 14: bright yellow
		0xFFFFFFFF, // 15: bright white
	}
	for i, c2 := range zxColors {
		c.Palette[i] = c2
	}
	// Gray ramp for 16-255
	for i := 16; i < 256; i++ {
		g := byte((i - 16) * 255 / 239)
		c.Palette[i] = 0xFF000000 | uint32(g)<<16 | uint32(g)<<8 | uint32(g)
	}
	return c
}

func (c *Canvas) setPixel(x, y int, col int64) {
	if x < 0 || x >= c.W || y < 0 || y >= c.H {
		return
	}
	switch c.BPP {
	case 8:
		c.Pixels[y*c.W+x] = byte(col)
	case 16:
		idx := (y*c.W + x) * 2
		c.Pixels[idx] = byte(col)
		c.Pixels[idx+1] = byte(col >> 8)
	case 24:
		idx := (y*c.W + x) * 3
		c.Pixels[idx] = byte(col)
		c.Pixels[idx+1] = byte(col >> 8)
		c.Pixels[idx+2] = byte(col >> 16)
	}
}

func (c *Canvas) getPixel(x, y int) int64 {
	if x < 0 || x >= c.W || y < 0 || y >= c.H {
		return 0
	}
	switch c.BPP {
	case 8:
		return int64(c.Pixels[y*c.W+x])
	case 16:
		idx := (y*c.W + x) * 2
		return int64(c.Pixels[idx]) | int64(c.Pixels[idx+1])<<8
	case 24:
		idx := (y*c.W + x) * 3
		return int64(c.Pixels[idx]) | int64(c.Pixels[idx+1])<<8 | int64(c.Pixels[idx+2])<<16
	}
	return 0
}

func (c *Canvas) clear(col int64) {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			c.setPixel(x, y, col)
		}
	}
}

func (c *Canvas) line(x0, y0, x1, y1 int, col int64) {
	// Bresenham's line algorithm.
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	for {
		c.setPixel(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := err * 2
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func (c *Canvas) rect(x, y, w, h int, col int64) {
	c.line(x, y, x+w-1, y, col)
	c.line(x+w-1, y, x+w-1, y+h-1, col)
	c.line(x+w-1, y+h-1, x, y+h-1, col)
	c.line(x, y+h-1, x, y, col)
}

func (c *Canvas) fillRect(x, y, w, h int, col int64) {
	for row := y; row < y+h; row++ {
		for col2 := x; col2 < x+w; col2++ {
			c.setPixel(col2, row, col)
		}
	}
}

func (c *Canvas) circle(cx, cy, r int, col int64) {
	// Midpoint circle algorithm.
	x, y := r, 0
	d := 1 - r
	for x >= y {
		c.setPixel(cx+x, cy+y, col)
		c.setPixel(cx-x, cy+y, col)
		c.setPixel(cx+x, cy-y, col)
		c.setPixel(cx-x, cy-y, col)
		c.setPixel(cx+y, cy+x, col)
		c.setPixel(cx-y, cy+x, col)
		c.setPixel(cx+y, cy-x, col)
		c.setPixel(cx-y, cy-x, col)
		y++
		if d < 0 {
			d += 2*y + 1
		} else {
			x--
			d += 2*(y-x) + 1
		}
	}
}

func (c *Canvas) toRGBA(px int64) (r, g, b uint8) {
	switch c.BPP {
	case 8:
		argb := c.Palette[byte(px)]
		return uint8(argb >> 16), uint8(argb >> 8), uint8(argb)
	case 16:
		// RGB565
		v := uint16(px)
		return uint8((v>>11)&0x1F) << 3, uint8((v>>5)&0x3F) << 2, uint8(v&0x1F) << 3
	case 24:
		return uint8(px), uint8(px >> 8), uint8(px >> 16)
	}
	return 0, 0, 0
}

// GetPixelIdx returns the raw pixel value at (x,y) — palette index for 8bpp.
func (c *Canvas) GetPixelIdx(x, y int) int {
	return int(c.getPixel(x, y))
}

// SavePNG exports the canvas to a PNG file.
func (c *Canvas) SavePNG(path string) error {
	return c.savePNG(path)
}

func (c *Canvas) savePNG(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			px := c.getPixel(x, y)
			r, g, b := c.toRGBA(px)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// CanvasRef holds a reference to the active canvas, which is created lazily
// when @canvas.init is called. Check ref.C after VM execution.
type CanvasRef struct {
	C *Canvas
}

// RegisterCanvasHosts installs @canvas.* host functions on a VM.
// Returns a CanvasRef whose .C field is populated after @canvas.init runs.
func RegisterCanvasHosts(vm *VM) *CanvasRef {
	ref := &CanvasRef{}

	vm.Hosts["@canvas.init"] = func(args []Value) ([]Value, error) {
		w, h, mode := int(args[0].I), int(args[1].I), args[2].I
		if preset, ok := canvasModes[mode]; ok {
			w, h = preset[0], preset[1]
			ref.C = newCanvas(w, h, preset[2])
		} else {
			// Mode 4+: custom size, 8bpp
			ref.C = newCanvas(w, h, 8)
		}
		return []Value{{I: 0}}, nil
	}

	vm.Hosts["@canvas.clear"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return nil, fmt.Errorf("canvas not initialized")
		}
		ref.C.clear(args[0].I)
		return []Value{{I: 0}}, nil
	}

	vm.Hosts["@canvas.pixel"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return nil, fmt.Errorf("canvas not initialized")
		}
		ref.C.setPixel(int(args[0].I), int(args[1].I), args[2].I)
		return []Value{{I: 0}}, nil
	}

	vm.Hosts["@canvas.get_pixel"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return nil, fmt.Errorf("canvas not initialized")
		}
		return []Value{{I: ref.C.getPixel(int(args[0].I), int(args[1].I))}}, nil
	}

	vm.Hosts["@canvas.line"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return nil, fmt.Errorf("canvas not initialized")
		}
		ref.C.line(int(args[0].I), int(args[1].I), int(args[2].I), int(args[3].I), args[4].I)
		return []Value{{I: 0}}, nil
	}

	vm.Hosts["@canvas.rect"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return nil, fmt.Errorf("canvas not initialized")
		}
		ref.C.rect(int(args[0].I), int(args[1].I), int(args[2].I), int(args[3].I), args[4].I)
		return []Value{{I: 0}}, nil
	}

	vm.Hosts["@canvas.fill_rect"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return nil, fmt.Errorf("canvas not initialized")
		}
		ref.C.fillRect(int(args[0].I), int(args[1].I), int(args[2].I), int(args[3].I), args[4].I)
		return []Value{{I: 0}}, nil
	}

	vm.Hosts["@canvas.circle"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return nil, fmt.Errorf("canvas not initialized")
		}
		ref.C.circle(int(args[0].I), int(args[1].I), int(args[2].I), args[3].I)
		return []Value{{I: 0}}, nil
	}

	vm.Hosts["@canvas.width"] = func(_ []Value) ([]Value, error) {
		if ref.C == nil {
			return []Value{{I: 0}}, nil
		}
		return []Value{{I: int64(ref.C.W)}}, nil
	}

	vm.Hosts["@canvas.height"] = func(_ []Value) ([]Value, error) {
		if ref.C == nil {
			return []Value{{I: 0}}, nil
		}
		return []Value{{I: int64(ref.C.H)}}, nil
	}

	vm.Hosts["@canvas.palette"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return nil, fmt.Errorf("canvas not initialized")
		}
		idx := byte(args[0].I)
		r, g, b := byte(args[1].I), byte(args[2].I), byte(args[3].I)
		ref.C.Palette[idx] = 0xFF000000 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
		return []Value{{I: 0}}, nil
	}

	// @canvas.save reads a NUL-terminated path from VM heap.
	vm.Hosts["@canvas.save"] = func(args []Value) ([]Value, error) {
		if ref.C == nil {
			return []Value{{I: -1}}, nil
		}
		// args[0] is a pointer to a NUL-terminated string in VM heap
		ptr := int(args[0].I)
		var path []byte
		for ptr < len(vm.heap) && vm.heap[ptr] != 0 {
			path = append(path, vm.heap[ptr])
			ptr++
		}
		if err := ref.C.savePNG(string(path)); err != nil {
			return []Value{{I: -1}}, nil
		}
		return []Value{{I: 0}}, nil
	}

	// C-friendly aliases (same functions, C-style names for C89/ObjC frontends).
	vm.Hosts["canvas_init"] = vm.Hosts["@canvas.init"]
	vm.Hosts["canvas_clear"] = vm.Hosts["@canvas.clear"]
	vm.Hosts["canvas_pixel"] = vm.Hosts["@canvas.pixel"]
	vm.Hosts["canvas_get_pixel"] = vm.Hosts["@canvas.get_pixel"]
	vm.Hosts["canvas_line"] = vm.Hosts["@canvas.line"]
	vm.Hosts["canvas_rect"] = vm.Hosts["@canvas.rect"]
	vm.Hosts["canvas_fill_rect"] = vm.Hosts["@canvas.fill_rect"]
	vm.Hosts["canvas_circle"] = vm.Hosts["@canvas.circle"]
	vm.Hosts["canvas_width"] = vm.Hosts["@canvas.width"]
	vm.Hosts["canvas_height"] = vm.Hosts["@canvas.height"]
	vm.Hosts["canvas_palette"] = vm.Hosts["@canvas.palette"]
	vm.Hosts["canvas_save"] = vm.Hosts["@canvas.save"]

	return ref
}
