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

// Branch: draw trunk, then split left and right with shorter branches
fun branch(len: u8, depth: u8) -> void {
    if depth == 0 {
        turtle_forward(len)
        return
    }
    turtle_forward(len)
    var next: u8 = len * 2 / 3
    if next < 2 { next = 2 }
    var d1: u8 = depth - 1

    turtle_push()
    turtle_left(30)
    branch(next, d1)
    turtle_pop()

    turtle_push()
    turtle_right(30)
    branch(next, d1)
    turtle_pop()
}

// ── Draw scene ───────────────────────────────────────────────────

fun draw_lsystem() -> u16 {
    canvas_init(256, 192, 0)
    canvas_clear(0)

    // Draw tree from bottom center
    tx = 128
    ty = 180
    tangle = 192    // pointing up

    tcolor = 4      // green
    branch(12, 7)

    // Second tree
    tx = 55
    ty = 186
    tangle = 192
    tcolor = 12     // bright green
    branch(10, 6)

    // Third tree
    tx = 200
    ty = 183
    tangle = 192
    tcolor = 6      // yellow
    branch(9, 6)

    // Ground line
    canvas_line(0, 190, 255, 190, 4)

    return 0
}

sandbox "lsystem" {
    assert draw_lsystem() == 0 via mir2
}
`
	hirMod, err := nanz.Parse(src, "lsystem.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	mir2mod := hir.LowerModule(hirMod)
	vm := mir2.NewVM(mir2mod)
	canvasRef := mir2.RegisterCanvasHosts(vm)
	_, err = vm.Call("draw_lsystem", nil)
	if err != nil {
		t.Fatalf("draw_lsystem: %v", err)
	}

	outPath := "/tmp/nanz_lsystem.png"
	if canvasRef.C != nil {
		if err := canvasRef.C.SavePNG(outPath); err != nil {
			t.Fatalf("save PNG: %v", err)
		}
		info, _ := os.Stat(outPath)
		t.Logf("PNG saved: %s (%d bytes)", outPath, info.Size())
	} else {
		t.Fatal("No canvas")
	}
}
