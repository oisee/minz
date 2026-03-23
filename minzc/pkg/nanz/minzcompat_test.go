package nanz_test

import (
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// TestMinZCompat verifies that the Nanz parser accepts MinZ syntax:
// - Semicolons at end of statements
// - `let` keyword (with optional `mut`)
// - `const` keyword
// - Trailing commas in enums
func TestMinZCompat_BasicSyntax(t *testing.T) {
	src := `
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

assert add(3, 5) == 8 via mir2
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

func TestMinZCompat_LetVar(t *testing.T) {
	// let mut replaced with var across corpus — let is immutable binding, var is mutable
	src := `
fun fib(n: u8) -> u8 {
    var a: u8 = 0;
    var b: u8 = 1;
    var i: u8 = 0;
    while i < n {
        let temp: u8 = a + b;
        a = b;
        b = temp;
        i = i + 1;
    }
    return a;
}

assert fib(0) == 0 via mir2
assert fib(1) == 1 via mir2
assert fib(5) == 5 via mir2
assert fib(7) == 13 via mir2
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

func TestMinZCompat_Enums(t *testing.T) {
	src := `
enum Direction {
    Up,
    Down,
    Left,
    Right,
}

fun opposite(dir: u8) -> u8 {
    if (dir == Direction.Up) {
        return Direction.Down
    }
    if (dir == Direction.Down) {
        return Direction.Up
    }
    if (dir == Direction.Left) {
        return Direction.Right
    }
    return Direction.Left
}

assert opposite(0) == 1 via mir2
assert opposite(1) == 0 via mir2
assert opposite(2) == 3 via mir2
assert opposite(3) == 2 via mir2
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

func TestMinZCompat_Const(t *testing.T) {
	src := `
const SCREEN_MEM: u16 = 16384;
const ATTR_MEM: u16 = 22528;

fun get_screen() -> u16 {
    return SCREEN_MEM
}

fun get_attr() -> u16 {
    return ATTR_MEM
}

assert get_screen() == 16384 via mir2
assert get_attr() == 22528 via mir2
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

func TestMinZCompat_LocalArrayLiteral(t *testing.T) {
	src := `
fun sum5(arr: ^u8) -> u8 {
    return arr[0] + arr[1] + arr[2] + arr[3] + arr[4]
}

fun test_local_arr() -> u8 {
    let data: [u8; 5] = [10, 20, 30, 40, 50];
    return data[0]
}

fun test_let_arr_sum() -> u8 {
    let data: [u8; 3] = [1, 2, 3];
    return data[0] + data[1] + data[2]
}

assert test_local_arr() == 10 via mir2
assert test_let_arr_sum() == 6 via mir2
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

func TestMinZCompat_Struct(t *testing.T) {
	src := `
struct Point {
    x: u8,
    y: u8
}

fun make_x(v: u8) -> u8 {
    return v
}

assert make_x(42) == 42 via mir2
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
