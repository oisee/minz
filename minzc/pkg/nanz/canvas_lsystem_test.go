package nanz_test

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestCanvasLSystem(t *testing.T) {
	src := `
@extern fun canvas_init(w: u16, h: u16, mode: u16) -> u16
@extern fun canvas_clear(color: u16) -> u16
@extern fun canvas_line(x0: u16, y0: u16, x1: u16, y1: u16, color: u16) -> u16

// ── Fixed-point trig (64-entry quarter-sine, scale=127) ───────────
// Full circle = 256 steps. Quarter = 64 entries.
// sin(0..63) stored; other quadrants derived by symmetry.

global qsin: [u8; 64] = [
    0,   3,   6,  9,  12,  16,  19,  22,
   25,  28,  31, 34,  37,  40,  43,  46,
   49,  51,  54, 57,  60,  62,  65,  67,
   70,  72,  75, 77,  79,  81,  83,  85,
   87,  89,  91, 93,  94,  96,  97,  99,
  100, 101, 103,104, 105, 106, 107, 108,
  108, 109, 110,110, 111, 111, 112, 112,
  112, 112, 112,112, 112, 112, 112, 112
]

// sin(a) for a in 0..255 (full circle), returns -112..+112
fun fsin(a: u8) -> i16 {
    var q: u8 = a / 64       // quadrant 0..3
    var idx: u8 = a & 63     // index within quadrant
    if q == 0 {
        return qsin[idx]
    }
    if q == 1 {
        return qsin[63 - idx]
    }
    if q == 2 {
        var v: u16 = qsin[idx]
        return 0 - v
    }
    // q == 3
    var v: u16 = qsin[63 - idx]
    return 0 - v
}

// cos(a) = sin(a + 64)
fun fcos(a: u8) -> i16 {
    return fsin(a + 64)
}

// ── Turtle state (globals for VM compatibility) ──────────────────

global tx: i16 = 128
global ty: i16 = 160
global tangle: u8 = 192    // pointing up (192 = 270 degrees in 256-step)
global tcolor: u16 = 14    // bright yellow

// ── Turtle operations ────────────────────────────────────────────

fun turtle_forward(dist: u8) -> u16 {
    var dx: i16 = fcos(tangle)
    var dy: i16 = fsin(tangle)
    // scale: dist * trig / 112
    var nx: i16 = tx + (dx * dist) / 112
    var ny: i16 = ty + (dy * dist) / 112
    canvas_line(tx, ty, nx, ny, tcolor)
    tx = nx
    ty = ny
    return 0
}

fun turtle_right(steps: u8) -> void {
    tangle = tangle + steps
}

fun turtle_left(steps: u8) -> void {
    tangle = tangle - steps
}

// ── Stack for push/pop (L-system branching) ──────────────────────

global stack_x: [i16; 32]
global stack_y: [i16; 32]
global stack_a: [u8; 32]
global sp: u8 = 0

fun turtle_push() -> void {
    stack_x[sp] = tx
    stack_y[sp] = ty
    stack_a[sp] = tangle
    sp = sp + 1
}

fun turtle_pop() -> void {
    sp = sp - 1
    tx = stack_x[sp]
    ty = stack_y[sp]
    tangle = stack_a[sp]
}

// ── L-system: simple fractal tree ────────────────────────────────
// Rules: F → F[+F]F[-F]F
// F = forward, + = right 25deg, - = left 25deg, [ = push, ] = pop

// Depth 0: just draw forward
fun tree_0(len: u8) -> void {
    turtle_forward(len)
}

// ── Tree variants ────────────────────────────────────────────────

// Binary symmetric tree
fun tree_sym(len: u8, depth: u8, angle: u8) -> void {
    if depth == 0 {
        turtle_forward(len)
        return
    }
    turtle_forward(len)
    var next: u8 = len * 2 / 3
    if next < 2 { next = 2 }
    var d1: u8 = depth - 1

    turtle_push()
    turtle_left(angle)
    tree_sym(next, d1, angle)
    turtle_pop()

    turtle_push()
    turtle_right(angle)
    tree_sym(next, d1, angle)
    turtle_pop()
}

// Ternary tree: center + left + right
fun tree_tri(len: u8, depth: u8, angle: u8) -> void {
    if depth == 0 {
        turtle_forward(len)
        return
    }
    turtle_forward(len)
    var next: u8 = len / 2
    if next < 2 { next = 2 }
    var d1: u8 = depth - 1

    turtle_push()
    tree_tri(next, d1, angle)
    turtle_pop()

    turtle_push()
    turtle_left(angle)
    tree_tri(next, d1, angle)
    turtle_pop()

    turtle_push()
    turtle_right(angle)
    tree_tri(next, d1, angle)
    turtle_pop()
}

// Asymmetric tree: left branch longer than right
fun tree_asym(len: u8, depth: u8) -> void {
    if depth == 0 {
        turtle_forward(len)
        return
    }
    turtle_forward(len)
    var d1: u8 = depth - 1
    var long: u8 = len * 3 / 4
    var short: u8 = len / 2
    if long < 2 { long = 2 }
    if short < 2 { short = 2 }

    turtle_push()
    turtle_left(25)
    tree_asym(long, d1)
    turtle_pop()

    turtle_push()
    turtle_right(35)
    tree_asym(short, d1)
    turtle_pop()
}

// Fern: one main stem with alternating side branches
fun fern(len: u8, depth: u8) -> void {
    if depth == 0 {
        turtle_forward(len)
        return
    }
    turtle_forward(len)
    var d1: u8 = depth - 1
    var side: u8 = len * 2 / 3
    if side < 2 { side = 2 }

    // Right branch
    turtle_push()
    turtle_right(40)
    fern(side, d1)
    turtle_pop()

    turtle_forward(len)

    // Left branch
    turtle_push()
    turtle_left(40)
    fern(side, d1)
    turtle_pop()

    turtle_forward(len)

    // Top
    fern(side, d1)
}

// ── Scene 1: Forest of symmetric trees ───────────────────────────

fun scene_forest() -> u16 {
    canvas_init(256, 192, 0)
    canvas_clear(0)

    // Ground
    canvas_fill_rect(0, 185, 256, 7, 4)

    // Five trees with varying angles
    tx = 25
    ty = 184
    tangle = 192
    tcolor = 4
    tree_sym(10, 6, 25)

    tx = 70
    ty = 184
    tangle = 192
    tcolor = 12
    tree_sym(12, 6, 30)

    tx = 128
    ty = 184
    tangle = 192
    tcolor = 4
    tree_sym(14, 7, 28)

    tx = 185
    ty = 184
    tangle = 192
    tcolor = 12
    tree_sym(11, 6, 35)

    tx = 235
    ty = 184
    tangle = 192
    tcolor = 4
    tree_sym(9, 6, 22)

    return 0
}

// ── Scene 2: Ternary tree (bushy) ────────────────────────────────

fun scene_ternary() -> u16 {
    canvas_init(256, 192, 0)
    canvas_clear(0)

    canvas_fill_rect(0, 185, 256, 7, 6)

    tx = 128
    ty = 184
    tangle = 192
    tcolor = 12
    tree_tri(16, 5, 28)

    return 0
}

// ── Scene 3: Asymmetric windblown trees ──────────────────────────

fun scene_windblown() -> u16 {
    canvas_init(256, 192, 0)
    canvas_clear(0)

    canvas_fill_rect(0, 185, 256, 7, 4)

    tx = 60
    ty = 184
    tangle = 192
    tcolor = 4
    tree_asym(14, 7)

    tx = 150
    ty = 184
    tangle = 192
    tcolor = 14
    tree_asym(12, 7)

    tx = 220
    ty = 184
    tangle = 192
    tcolor = 6
    tree_asym(10, 6)

    return 0
}

// ── Scene 4: Fern ────────────────────────────────────────────────

fun scene_fern() -> u16 {
    canvas_init(256, 192, 0)
    canvas_clear(0)

    canvas_fill_rect(0, 185, 256, 7, 4)

    tx = 80
    ty = 184
    tangle = 192
    tcolor = 4
    fern(8, 5)

    tx = 180
    ty = 184
    tangle = 192
    tcolor = 12
    fern(7, 5)

    return 0
}

sandbox "lsystem" {
    assert scene_forest() == 0 via mir2
    assert scene_ternary() == 0 via mir2
    assert scene_windblown() == 0 via mir2
    assert scene_fern() == 0 via mir2
}
`
	hirMod, err := nanz.Parse(src, "lsystem.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mir2mod := hir.LowerModule(hirMod)

	scenes := []struct {
		name string
		fn   string
	}{
		{"forest", "scene_forest"},
		{"ternary", "scene_ternary"},
		{"windblown", "scene_windblown"},
		{"fern", "scene_fern"},
	}

	for _, sc := range scenes {
		vm := mir2.NewVM(mir2mod)
		canvasRef := mir2.RegisterCanvasHosts(vm)
		_, err = vm.Call(sc.fn, nil)
		if err != nil {
			t.Fatalf("%s: %v", sc.name, err)
		}
		outPath := "/tmp/nanz_lsystem_" + sc.name + ".png"
		if canvasRef.C != nil {
			if err := canvasRef.C.SavePNG(outPath); err != nil {
				t.Fatalf("save %s: %v", sc.name, err)
			}
			info, _ := os.Stat(outPath)
			t.Logf("%s: %s (%d bytes)", sc.name, outPath, info.Size())
		}
	}
}
