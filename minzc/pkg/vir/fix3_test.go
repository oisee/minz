package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"path/filepath"
	"strings"
	"testing"
)

func TestPBQPFallbackReportsSymbolCollision(t *testing.T) {
	m := &mir2.Module{}
	for _, name := range []string{"f", "v_f"} {
		f := m.AddFunc(name)
		b := f.NewBlock("entry")
		b.Seal(&mir2.TermRet{})
	}
	// A missing solver forces the production PBQP fallback without requiring Z3.
	_, results := CodegenModule(m, SolverOptions{Z3Path: filepath.Join(t.TempDir(), "missing-z3"), PBQPAlloc: &mir2.AllocResult{}})
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	for _, r := range results {
		if r.OK || !strings.Contains(r.Error, "ambiguous Z80 symbol") {
			t.Fatalf("result = %+v", r)
		}
	}
}
