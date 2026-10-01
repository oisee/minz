package nanz_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestHigherOrderRuntime(t *testing.T) {
	src := `
global numbers: [u8; 3] = [1, 2, 3]

fun add(a: u8, b: u8) -> u8 { return a + b }
fun sub(a: u8, b: u8) -> u8 { return a - b }
fun mul16(a: u16, b: u16) -> u16 { return a * b }

fun zero_arg() -> u8 {
    let answer = || 42
    return answer()
}

fun lambda_call() -> u8 {
    let inc = |x: u8| x + 1
    return inc(7)
}

fun partial_call() -> u8 {
    let add5 = add(5, _)
    return add5(7)
}

fun multi_placeholder() -> u8 {
    let swapped = sub(_, _)
    return swapped(9, 4)
}

fun wide_partial() -> u16 {
    let twice = mul16(2, _)
    return twice(1695)
}

fun captured_iterator() -> u8 {
    var sum: u8 = 0
    numbers.forEach(|x: u8| { sum = sum + x }, 3)
    return sum
}
`
	hm, err := nanz.Parse(src, "higher_order_runtime.nanz")
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(hm))
	for _, tc := range []struct {
		name string
		want int64
	}{
		{"zero_arg", 42},
		{"lambda_call", 8},
		{"partial_call", 12},
		{"multi_placeholder", 5},
		{"wide_partial", 3390},
		{"captured_iterator", 6},
	} {
		got, err := vm.Call(tc.name, nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(got) != 1 || got[0].I != tc.want {
			t.Fatalf("%s = %v, want %d", tc.name, got, tc.want)
		}
	}
}
