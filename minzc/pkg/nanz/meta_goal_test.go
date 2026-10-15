package nanz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Goal hints must survive imports, caller-scope and nested expansions without
// leaking between consecutive compilations or becoming target runtime calls.
func TestMetafunctionSizeGoalContext(t *testing.T) {
	definitions := `fun @declare_value() -> void {
 if meta_opt_size() {emit(c"fun chosen()->u8 {return 13}")}
 else {emit(c"fun chosen()->u8 {return 11}")}
}
fun @outer() -> void {emit(c"@assign_value()")}
fun @outer_declaration() -> void {emit(c"import hints") emit(c"@declare_value()")}
fun @assign_value() -> void {
 if meta_opt_size() {emit(c"value=19")}
 else {emit(c"value=17")}
}`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hints.nanz"), []byte(definitions), 0600); err != nil {
		t.Fatal(err)
	}
	for _, imported := range []bool{false, true} {
		source := definitions
		if imported {
			source = "import hints"
		}
		source += `
@outer_declaration()
fun main()->u16 {
 var value:u8=0
 @outer()
 return u16(chosen())*256+u16(value)
}`
		for _, size := range []bool{false, true, false, true} {
			h, err := nanz.ParseWithOpts(source, "goal-main.nanz", nanz.ParseOpts{BaseDir: dir, OptSize: size})
			if err != nil {
				t.Fatalf("import=%v size=%v: %v", imported, size, err)
			}
			module := hir.LowerModule(h)
			got, err := mir2.NewVM(module).Call("main", nil)
			want := int64(11*256 + 17)
			if size {
				want = 13*256 + 19
			}
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatalf("import=%v size=%v: %v err=%v want=%d", imported, size, got, err, want)
			}
			for _, function := range module.Funcs {
				if strings.Contains(function.Name, "meta_opt_size") {
					t.Fatal("compile-time goal leaked into target module", function.Name)
				}
			}
		}
	}
}
