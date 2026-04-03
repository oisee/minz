package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

func TestGraceVerify_PointerThreadingSourceShape(t *testing.T) {
const src = `
global data_g: [u8; 9] = [1, 2, 3, 4, 5, 6, 7, 8, 9]

fun sum_prefix(n: u8) -> u8 {
    var i: u8 = 0
    var acc: u8 = 0
    while i < n {
        acc = acc + data_g[i]
        i = i + 1
    }
    return acc
}

fun sum_neighbors(n: u8) -> u8 {
    var i: u8 = 0
    var acc: u8 = 0
    while i < n {
        let a: u8 = data_g[i]
        let b: u8 = data_g[i + 1]
        acc = acc + a + b
        i = i + 1
    }
    return acc
}

assert sum_prefix(4) == 10 via mir2
assert sum_neighbors(4) == 24 via mir2
`

	hm, err := nanz.ParseWithOpts(src, "pointer_threading_regression.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = CompileHIRSteps(hm, Options{
		ContractOpt: true,
		UseGrace:    true,
	})
	if err != nil {
		t.Fatalf("CompileHIRSteps with Grace: %v", err)
	}
}
