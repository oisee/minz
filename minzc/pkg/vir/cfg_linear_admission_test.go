package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// Both fallback paths used to accept schedules with no modeled CFG edges.
// Keep the obligation explicit until PHI and back-edge contracts are supplied.
func TestLinearFallbackRequiresCFGContracts(t *testing.T) {
	m := &mir2.Module{}
	f := m.AddFunc("branch")
	f.NewBlock("entry").Seal(&mir2.TermJmp{Target: "exit"})
	f.NewBlock("exit").Seal(&mir2.TermRet{})
	vf := &Func{}
	for _, codegen := range []struct {
		name string
		fn   func(*mir2.Func, *Func, *MachineDesc, SolverOptions) (string, error)
	}{
		{"whole", codegenFuncWhole}, {"islands", codegenFuncIslands},
	} {
		t.Run(codegen.name, func(t *testing.T) {
			if asm, err := codegen.fn(f, vf, Z80, SolverOptions{}); err == nil || asm != "" {
				t.Fatalf("unmodeled CFG admitted: %q %v", asm, err)
			}
		})
	}
}
