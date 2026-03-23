package nanz_test

import (
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestImplBlock_TraitForType(t *testing.T) {
	src := `
struct Circle {
    x: u8,
    y: u8,
    radius: u8
}

interface Drawable {
    draw
}

impl Drawable for Circle {
    fun draw(self) -> u8 {
        return self.radius * 6
    }
}

fun test_draw() -> u8 {
    var c: Circle
    c.radius = 5
    return c.draw()
}

assert test_draw() == 30 via mir2
`
	m, err := nanz.Parse(src, "test.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Check desugared function exists
	found := false
	for _, f := range m.Funcs {
		if f.Name == "Circle_draw" {
			found = true
			if len(f.Params) != 1 || f.Params[0].Name != "self" {
				t.Errorf("Circle_draw params: got %v", f.Params)
			}
		}
	}
	if !found {
		t.Error("Circle_draw not found — impl desugaring failed")
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestImplBlock_MultiMethod(t *testing.T) {
	src := `
struct Vec2 {
    x: u8,
    y: u8
}

impl Math for Vec2 {
    fun length_sq(self) -> u8 {
        return self.x * self.x + self.y * self.y
    }

    fun manhattan(self) -> u8 {
        return self.x + self.y
    }
}

fun test_length() -> u8 {
    var v: Vec2
    v.x = 3
    v.y = 4
    return v.length_sq()
}

fun test_manhattan() -> u8 {
    var v: Vec2
    v.x = 10
    v.y = 20
    return v.manhattan()
}

assert test_length() == 25 via mir2
assert test_manhattan() == 30 via mir2
`
	m, err := nanz.Parse(src, "test.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestImplBlock_WithParams(t *testing.T) {
	src := `
struct Counter {
    val: u8
}

impl Ops for Counter {
    fun add(self, n: u8) -> u8 {
        return self.val + n
    }
}

fun test_add() -> u8 {
    var c: Counter
    c.val = 10
    return c.add(5)
}

assert test_add() == 15 via mir2
`
	m, err := nanz.Parse(src, "test.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
}
