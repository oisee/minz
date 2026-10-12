package pipeline

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// Source-path migration must not silently replace the independently frozen
// family model used by the historical sequence experiment.
func tetrisVerifyHistoricalModel(t *testing.T) {
	t.Helper()
	declarations := func(path string) map[string]string {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, path, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, d := range f.Decls {
			name := ""
			switch n := d.(type) {
			case *ast.FuncDecl:
				name = n.Name.Name
			case *ast.GenDecl:
				if len(n.Specs) == 1 {
					if typ, ok := n.Specs[0].(*ast.TypeSpec); ok {
						name = typ.Name.Name
					}
				}
			}
			if name == "" || name == "TestOriginalTetrisFamilyMechanics" {
				continue
			}
			var b bytes.Buffer
			if err := format.Node(&b, fs, d); err != nil {
				t.Fatal(err)
			}
			out[name] = b.String()
		}
		return out
	}
	old := declarations("../../../reports/tetris-main-native/historical-family-test.go.txt")
	current := declarations("original_tetris_family_test.go")
	if len(old) != len(current) {
		t.Fatalf("historical model declaration count %d != %d", len(old), len(current))
	}
	for name, src := range old {
		if current[name] != src {
			t.Fatalf("historical independent model changed: %s", name)
		}
	}
}

func TestTetrisHistoricalModelIdentity(t *testing.T) { tetrisVerifyHistoricalModel(t) }
