package nanz_test

// E2E emulator tests for pipe/trans declarations — verifies that
// pipe-expanded iterator chains produce correct Z80 code.

import (
	"testing"
)

func TestPipe_E2E_RangeFold(t *testing.T) {
	// pipe with a map stage: range(0..5).apply(doubled).fold(0, add_acc)
	// doubled maps each element x → x+x
	// fold accumulates: sum of 2*5 + 2*4 + 2*3 + 2*2 + 2*1 = 10+8+6+4+2 = 30
	// (range(0..5) counts 5,4,3,2,1 — DJNZ semantics)
	src := `
fun add_acc(acc: u8, x: u8) -> u8 { return (acc + x) }

pipe doubled { map(|x: u8| x + x) }

fun sum_doubled() -> u8 {
    return range(0..5).apply(doubled).fold(0, add_acc)
}
`
	got, err := compileAndRunEnum(t, src, "sum_doubled")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 30 {
		t.Errorf("sum_doubled: want 30, got %d", got)
	}
}

func TestPipe_E2E_UseComposition(t *testing.T) {
	// trans composed = { use base; map(|x| x + 1) }
	// base doubles, then composed adds 1
	// range(0..3): elements 3,2,1 → double → 6,4,2 → +1 → 7,5,3 → sum = 15
	src := `
fun add_acc(acc: u8, x: u8) -> u8 { return (acc + x) }

pipe base { map(|x: u8| x + x) }
trans composed { use base; map(|x: u8| x + 1) }

fun sum_composed() -> u8 {
    return range(0..3).apply(composed).fold(0, add_acc)
}
`
	got, err := compileAndRunEnum(t, src, "sum_composed")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 15 {
		t.Errorf("sum_composed: want 15, got %d", got)
	}
}

func TestValuePipe_E2E(t *testing.T) {
	// 5 |> double |> inc → inc(double(5)) = inc(10) = 11
	src := `
fun double(x: u8) -> u8 { return (x + x) }
fun inc(x: u8) -> u8 { return (x + 1) }

fun piped() -> u8 {
    return 5 |> double |> inc
}
`
	got, err := compileAndRunEnum(t, src, "piped")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 11 {
		t.Errorf("piped: want 11, got %d", got)
	}
}

func TestValuePipeWithArgs_E2E(t *testing.T) {
	// 5 |> add(3) |> mul(2) → mul(add(5,3), 2) = mul(8,2) = 16
	src := `
fun add(a: u8, b: u8) -> u8 { return (a + b) }
fun mul(a: u8, b: u8) -> u8 { return (a * b) }

fun piped() -> u8 {
    return 5 |> add(3) |> mul(2)
}
`
	got, err := compileAndRunEnum(t, src, "piped")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 16 {
		t.Errorf("piped: want 16, got %d", got)
	}
}

func TestLambdaInference_E2E(t *testing.T) {
	// range(0..5).map(|x| x + x).fold(0, add_acc) — no type annotation on lambda
	// Same as TestPipe_E2E_RangeFold but without explicit u8
	src := `
fun add_acc(acc: u8, x: u8) -> u8 { return (acc + x) }

fun sum_doubled() -> u8 {
    return range(0..5).map(|x| x + x).fold(0, add_acc)
}
`
	got, err := compileAndRunEnum(t, src, "sum_doubled")
	if err != nil {
		t.Fatalf("compile/run: %v", err)
	}
	if got != 30 {
		t.Errorf("sum_doubled: want 30, got %d", got)
	}
}
