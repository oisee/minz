package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestOptSizeCostTieBreak(t *testing.T) {
	image := func(body string) *variantImage {
		im, e := assembleVariant(Steps{Assembly: "; fun probe() -> u8 = A ; clobbers: A, F\nprobe:\n" + body + "    RET\n"})
		if e != nil {
			t.Fatal(e)
		}
		return im
	}
	slow := image("    XOR A\n    NOP\n")
	fast := image("    LD A, 0\n")
	module := &mir2.Module{Funcs: []*mir2.Func{{Name: "probe"}}}
	if len(slow.binary) != len(fast.binary) {
		t.Fatal("fixture must tie in encoded bytes")
	}
	if !sizeVariantFaster(slow, fast, module, module) || sizeVariantFaster(fast, slow, module, module) || sizeVariantFaster(fast, fast, module, module) {
		t.Fatal("equal bytes must use strict static T tie-break")
	}
	unknown := image("    CPIR\n")
	if sizeVariantFaster(unknown, fast, module, module) {
		t.Fatal("unknown timing must retain incumbent on a byte tie")
	}
}
