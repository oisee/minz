package interop

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

func TestExtractImports(t *testing.T) {
	src := `#import "math.pas"
#import "utils.c"
int foo(void) { return 42; }
#import "core.lizp"
`
	imports, cleaned := ExtractImports(src)

	if len(imports) != 3 {
		t.Fatalf("expected 3 imports, got %d", len(imports))
	}
	if imports[0] != "math.pas" {
		t.Errorf("import[0] = %q, want math.pas", imports[0])
	}
	if imports[1] != "utils.c" {
		t.Errorf("import[1] = %q, want utils.c", imports[1])
	}
	if imports[2] != "core.lizp" {
		t.Errorf("import[2] = %q, want core.lizp", imports[2])
	}

	// Cleaned source should not contain #import lines.
	if got := len(ExtractImports_imports(cleaned)); got != 0 {
		t.Errorf("cleaned source still has %d imports", got)
	}

	// Original code should survive.
	if !contains(cleaned, "int foo(void)") {
		t.Errorf("cleaned source lost original code")
	}
}

func ExtractImports_imports(src string) []string {
	imports, _ := ExtractImports(src)
	return imports
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestExtractImports_NoImports(t *testing.T) {
	src := `int main(void) { return 0; }`
	imports, cleaned := ExtractImports(src)
	if len(imports) != 0 {
		t.Errorf("expected 0 imports, got %d", len(imports))
	}
	if cleaned != src {
		t.Errorf("cleaned should equal original when no imports")
	}
}

func TestMergeInto(t *testing.T) {
	target := &hir.Module{
		Name:  "main",
		Funcs: []*hir.Func{{Name: "main"}},
	}
	imported := &hir.Module{
		Funcs: []*hir.Func{
			{Name: "helper"},
			{Name: "main"}, // duplicate — should be skipped
		},
	}

	MergeInto(target, []*hir.Module{imported})

	if len(target.Funcs) != 2 {
		t.Fatalf("expected 2 funcs, got %d", len(target.Funcs))
	}
	if target.Funcs[0].Name != "main" || target.Funcs[1].Name != "helper" {
		t.Errorf("unexpected func order: %s, %s", target.Funcs[0].Name, target.Funcs[1].Name)
	}
}
