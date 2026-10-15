package nanz_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetafunctionVariantImportAndNestedContext(t *testing.T) {
	dir := t.TempDir()
	definitions := `fun @pick()->void {
  let v:u8=meta_variant(c"test.row",2,0)
  if v==0 {emit(c"value=11")} else {emit(c"value=13")}
 }
 fun @outer()->void {emit(c"@pick()")}`
	if err := os.WriteFile(filepath.Join(dir, "hints.nanz"), []byte(definitions), 0600); err != nil {
		t.Fatal(err)
	}
	src := `import hints
 fun main()->u8 {var value:u8=0 @outer() return value}`
	for _, variant := range []uint8{0, 1, 0, 1} {
		selection := map[string]uint8{"test.row": variant}
		ctx := nanz.NewMetaVariantContext(selection)
		selection["test.row"] = 1 - variant // constructor owns the selection
		m, err := nanz.ParseWithOpts(src, "main.nanz", nanz.ParseOpts{BaseDir: dir, MetaVariants: ctx})
		if err != nil {
			t.Fatal(err)
		}
		if err := ctx.Validate(); err != nil {
			t.Fatal(err)
		}
		vs := ctx.Variants()
		if len(vs) != 1 || vs[0].Key != "test.row" {
			t.Fatal(vs)
		}
		vs[0].Default = 1 // returned declarations cannot mutate context
		if ctx.Variants()[0].Default != 0 {
			t.Fatal("declaration alias")
		}
		values, err := mir2.NewVM(hir.LowerModule(m)).Call("main", nil)
		if err != nil || len(values) != 1 || values[0].I != 11+int64(variant)*2 {
			t.Fatal(values, err)
		}
		for _, f := range m.Funcs {
			if strings.Contains(f.Name, "meta_variant") {
				t.Fatal("host leaked", f.Name)
			}
		}
	}
	// A normal parse uses fallback, with no variant machinery in the output.
	m, err := nanz.ParseWithOpts(src, "main.nanz", nanz.ParseOpts{BaseDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	values, err := mir2.NewVM(hir.LowerModule(m)).Call("main", nil)
	if err != nil || values[0].I != 11 {
		t.Fatal(values, err)
	}
}

func TestMetafunctionVariantRefusals(t *testing.T) {
	for _, tc := range []struct {
		body string
		sel  map[string]uint8
		want string
	}{
		{`meta_variant(c"x",1,0)`, nil, "2..32"},
		{`meta_variant(c"x",2,2)`, nil, "in-range"},
		{`meta_variant(c"",2,0)`, nil, "invalid key"},
		{`meta_variant(c"x",2,0)`, map[string]uint8{"x": 2}, "out of range"},
		{`meta_variant(c"x",2,0) meta_variant(c"x",3,0)`, nil, "conflicting"},
	} {
		ctx := nanz.NewMetaVariantContext(tc.sel)
		_, err := nanz.ParseWithOpts("fun @bad()->void {"+tc.body+"}\n@bad()", "bad.nanz", nanz.ParseOpts{MetaVariants: ctx})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("want %s: %v", tc.want, err)
		}
	}
	ctx := nanz.NewMetaVariantContext(map[string]uint8{"missing": 0})
	if _, err := nanz.ParseWithOpts("fun main()->u8 {return 1}", "plain.nanz", nanz.ParseOpts{MetaVariants: ctx}); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Validate(); err == nil {
		t.Fatal("unused variant selection accepted")
	}
}
