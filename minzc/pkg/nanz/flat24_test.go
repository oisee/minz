package nanz

import (
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlat24ImportABI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "view.nanz"), []byte(`struct View{data:^u8 len:u16}
global shared:View
fun base()->^u8{return shared.data}`), 0600); err != nil {
		t.Fatal(err)
	}
	h, err := ParseWithOpts("import view { * }", "importer.nanz", ParseOpts{PointerBits: 24, BaseDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if h.PointerBits != 24 || len(h.Structs) != 1 || mir2.ByteWidth(h.Structs[0]) != 5 {
		t.Fatalf("import lost ABI: %+v", h)
	}
	for _, f := range h.Funcs {
		if mir2.IsPointer(f.RetTy) && f.RetTy.Width() != 24 {
			t.Fatal("imported return narrowed")
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "foreign.lanz"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseWithOpts("import foreign", "bad.nanz", ParseOpts{PointerBits: 24, BaseDir: dir}); err == nil || !strings.Contains(err.Error(), "24-bit pointer ABI") {
		t.Fatalf("foreign ABI accepted: %v", err)
	}
}

func TestFlat24UnsupportedFeaturesFailExplicitly(t *testing.T) {
	for _, source := range []string{
		`interface Animal { fun speak()->u8 }`,
		`gen values()->u8 { yield 1 }`,
		`fun @make() { }`,
		`@extern fun native()->u8`,
	} {
		if _, err := ParseWithOpts(source, "unsupported.nanz", ParseOpts{PointerBits: 24}); err == nil || !strings.Contains(err.Error(), "24-bit pointer ABI") {
			t.Fatalf("%s: %v", source, err)
		}
	}
	if _, err := ParseWithOpts("", "invalid.nanz", ParseOpts{PointerBits: 17}); err == nil {
		t.Fatal("invalid width accepted")
	}
}
