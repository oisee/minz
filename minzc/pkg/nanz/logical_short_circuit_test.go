package nanz_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestLogicalShortCircuit(t *testing.T) {
	src := `
global calls: u8

fun touch() -> u8 {
    calls = calls + 1
    return 1
}

fun in_range(x: u8) -> u8 {
    if x >= 32 && x < 127 { return 1 }
    return 0
}

fun and_skips_rhs() -> u8 {
    calls = 0
    if 0 == 1 && touch() == 1 { return 99 }
    return calls
}

fun or_skips_rhs() -> u8 {
    calls = 0
    if 1 == 1 || touch() == 1 { return calls }
    return 99
}

fun and_evaluates_rhs() -> u8 {
    calls = 0
    if 1 == 1 && touch() == 1 { return calls }
    return 99
}

fun or_evaluates_rhs() -> u8 {
    calls = 0
    if 0 == 1 || touch() == 1 { return calls }
    return 99
}

fun precedence(x: u8) -> u8 {
    if x == 1 || x == 2 && x > 1 { return 1 }
    return 0
}

fun as_value(x: u8) -> u8 {
    let ok: bool = x > 1 && x < 4
    if ok { return 1 }
    return 0
}

fun bitwise_still_works(x: u8) -> u8 {
    return x & 3 | 4
}

fun loop_guard() -> u8 {
    var x: u8 = 0
    while x < 3 && x < 10 {
        x = x + 1
    }
    return x
}
`
	hm, err := nanz.Parse(src, "logical_short_circuit.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIR(hm); err != nil {
		t.Fatalf("Z80 compile: %v", err)
	}
	vm := mir2.NewVM(hir.LowerModule(hm))
	for _, tc := range []struct {
		fn   string
		arg  int64
		want int64
	}{
		{"in_range", 31, 0}, {"in_range", 32, 1},
		{"in_range", 126, 1}, {"in_range", 127, 0},
		{"and_skips_rhs", 0, 0}, {"or_skips_rhs", 0, 0},
		{"and_evaluates_rhs", 0, 1}, {"or_evaluates_rhs", 0, 1},
		{"precedence", 1, 1}, {"precedence", 2, 1}, {"precedence", 3, 0},
		{"as_value", 1, 0}, {"as_value", 2, 1}, {"as_value", 4, 0},
		{"bitwise_still_works", 7, 7},
		{"loop_guard", 0, 3},
	} {
		args := []mir2.Value{}
		if tc.fn == "in_range" || tc.fn == "precedence" || tc.fn == "as_value" || tc.fn == "bitwise_still_works" {
			args = append(args, mir2.Value{I: tc.arg})
		}
		got, err := vm.Call(tc.fn, args)
		if err != nil {
			t.Fatalf("%s(%d): %v", tc.fn, tc.arg, err)
		}
		if len(got) != 1 || got[0].I != tc.want {
			t.Fatalf("%s(%d) = %v, want %d", tc.fn, tc.arg, got, tc.want)
		}
	}
}
