package mir2

import (
	"strings"
	"testing"
)

func TestZ80RejectsGlobalDataOutsideAddressSpace(t *testing.T) {
	for _, globals := range [][]Global{
		{{Name: "story", Ty: &ArrayTy{Elem: TyU8, Len: 131072}}},
		{{Name: "first", Ty: &ArrayTy{Elem: TyU8, Len: 40000}}, {Name: "second", Ty: &ArrayTy{Elem: TyU8, Len: 40000}}},
		{{Name: "buffer", Ty: NewArray(TyU8, 65537), Init: make([]byte, 65537)}},
	} {
		asm, err := Z80Codegen(&Module{Globals: globals}, &AllocResult{})
		if err == nil || asm != "" || !strings.Contains(err.Error(), "16-bit address space") {
			t.Fatalf("oversize global data admitted: err=%v assembly length=%d", err, len(asm))
		}
	}
}

func TestZ80CapacityCheckDoesNotRejectSmallGlobals(t *testing.T) {
	asm, err := Z80Codegen(&Module{Globals: []Global{{Name: "bytes", Ty: &ArrayTy{Elem: TyU8, Len: 16}}}}, &AllocResult{})
	if err != nil || !strings.Contains(asm, "bytes:") {
		t.Fatalf("small global err=%v assembly=%q", err, asm)
	}
}
