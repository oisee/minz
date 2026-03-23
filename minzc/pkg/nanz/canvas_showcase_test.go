package nanz_test

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// TestCanvasImplShowcase renders shapes using impl blocks and exports PNG.
func TestCanvasImplShowcase(t *testing.T) {
	src := `
// Canvas host function declarations
@extern fun canvas_init(w: u16, h: u16, mode: u16) -> u16
@extern fun canvas_clear(color: u16) -> u16
@extern fun canvas_pixel(x: u16, y: u16, color: u16) -> u16
@extern fun canvas_line(x0: u16, y0: u16, x1: u16, y1: u16, color: u16) -> u16
@extern fun canvas_rect(x: u16, y: u16, w: u16, h: u16, color: u16) -> u16
@extern fun canvas_fill_rect(x: u16, y: u16, w: u16, h: u16, color: u16) -> u16
@extern fun canvas_circle(cx: u16, cy: u16, r: u16, color: u16) -> u16

// -- Shapes with impl --

struct Circle {
    cx: u16,
    cy: u16,
    r: u16,
    color: u16
}

struct Box {
    x: u16,
    y: u16,
    w: u16,
    h: u16,
    color: u16
}

impl Renderable for Circle {
    fun draw(self) -> u16 {
        return canvas_circle(self.cx, self.cy, self.r, self.color)
    }
}

impl Renderable for Box {
    fun draw(self) -> u16 {
        return canvas_fill_rect(self.x, self.y, self.w, self.h, self.color)
    }

    fun outline(self) -> u16 {
        return canvas_rect(self.x, self.y, self.w, self.h, 15)
    }
}

// -- Scene --

fun draw_scene() -> u16 {
    canvas_init(256, 192, 0)
    canvas_clear(0)

    // Background boxes
    var sky: Box
    sky.x = 0
    sky.y = 0
    sky.w = 256
    sky.h = 96
    sky.color = 1
    sky.draw()

    var ground: Box
    ground.x = 0
    ground.y = 96
    ground.w = 256
    ground.h = 96
    ground.color = 4
    ground.draw()

    // Sun
    var sun: Circle
    sun.cx = 200
    sun.cy = 40
    sun.r = 25
    sun.color = 14
    sun.draw()

    // House body
    var house: Box
    house.x = 60
    house.y = 70
    house.w = 60
    house.h = 50
    house.color = 2
    house.draw()
    house.outline()

    // Door
    var door: Box
    door.x = 80
    door.y = 95
    door.w = 15
    door.h = 25
    door.color = 6
    door.draw()

    // Roof lines
    canvas_line(55, 70, 90, 40, 15)
    canvas_line(90, 40, 125, 70, 15)

    // Window
    var win: Box
    win.x = 65
    win.y = 78
    win.w = 10
    win.h = 10
    win.color = 13
    win.draw()

    // Tree trunk
    var trunk: Box
    trunk.x = 170
    trunk.y = 85
    trunk.w = 6
    trunk.h = 35
    trunk.color = 6
    trunk.draw()

    // Tree crown
    var crown: Circle
    crown.cx = 173
    crown.cy = 75
    crown.r = 20
    crown.color = 4
    crown.draw()

    return 0
}

sandbox "canvas_scene" {
    assert draw_scene() == 0 via mir2
}
`
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

	outPath := "/tmp/nanz_impl_showcase.png"
	if canvasRef.C != nil {
		if err := canvasRef.C.SavePNG(outPath); err != nil {
			t.Fatalf("save PNG: %v", err)
		}
		info, _ := os.Stat(outPath)
		t.Logf("PNG saved: %s (%d bytes)", outPath, info.Size())
	} else {
		t.Log("No canvas — draw_scene may not have initialized it")
	}
}
