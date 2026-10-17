package mir2

import (
	"strings"
	"testing"
)

func TestVerifyRejectsOversizedGlobalInitializer(t *testing.T) {
	m := &Module{Name: "test-producer", Globals: []Global{{Name: "too_long", Ty: TyU8, Init: []byte{1, 2}}}}
	err := Verify(m)
	if err == nil {
		t.Fatal("accepted 2-byte initializer for u8 global")
	}
	for _, s := range []string{"too_long", "2", "1", "test-producer"} {
		if !strings.Contains(err.Error(), s) {
			t.Fatalf("diagnostic %q missing %q", err, s)
		}
	}
}

func TestGlobalInitializerLayoutBoundaries(t *testing.T) {
	for _, g := range []Global{
		{Name: "array", Ty: NewArray(TyU8, 2), Init: []byte{1, 2, 3}},
		{Name: "relocated", Ty: TyU8, Init: []byte{0, 0}, Relocs: []GlobalRelocation{{Symbol: "relocated"}}},
		{Name: "vtable", Ty: TyPtr, VtableSyms: []string{"", ""}},
	} {
		t.Run(g.Name, func(t *testing.T) {
			g.Producer = "fixture-producer"
			m := &Module{Globals: []Global{g}}
			if err := Verify(m); err == nil || !strings.Contains(err.Error(), "fixture-producer") {
				t.Fatalf("verify: %v", err)
			}
			if asm, err := Z80Codegen(m, &AllocResult{}); err == nil || asm != "" {
				t.Fatalf("codegen: %q, %v", asm, err)
			}
			if vm := NewVM(m); vm.initErr == nil {
				t.Fatal("VM accepted invalid layout")
			}
		})
	}
	m := &Module{Globals: []Global{
		{Name: "buffer", Ty: NewArray(TyU8, 4), Init: []byte{7, 8}},
		{Name: "next", Ty: TyU8, Init: []byte{9}},
	}}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	vm := NewVM(m)
	if vm.initErr != nil {
		t.Fatal(vm.initErr)
	}
	if vm.GlobalAddr("next")-vm.GlobalAddr("buffer") != 4 {
		t.Fatal("VM allocation disagrees with type layout")
	}
	if got := vm.heap[:5]; string(got) != string([]byte{7, 8, 0, 0, 9}) {
		t.Fatalf("VM data: %v", got)
	}
}
