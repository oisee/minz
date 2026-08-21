package lir

import (
	"os/exec"
	"testing"
)

// TestZ3PathResolvesFromEnvironment guards the regression that made the Z3 path
// dead everywhere but one machine: Z3Path was hardcoded to a conda prefix, so
// hasZ3() was false and the solver silently never ran.
func TestZ3PathResolvesFromEnvironment(t *testing.T) {
	if Z3Path == "/home/alice/miniconda3/bin/z3" {
		t.Fatal("Z3Path is hardcoded to a developer-specific path")
	}
	t.Setenv("MINZ_Z3", "/custom/z3")
	if got := resolveZ3Path(); got != "/custom/z3" {
		t.Errorf("MINZ_Z3 should win: got %q", got)
	}
	t.Setenv("MINZ_Z3", "")
	want, err := exec.LookPath("z3")
	if err != nil {
		if got := resolveZ3Path(); got != "z3" {
			t.Errorf("with no z3 on PATH the fallback should be %q, got %q", "z3", got)
		}
		return
	}
	if got := resolveZ3Path(); got != want {
		t.Errorf("should resolve from PATH: got %q, want %q", got, want)
	}
}
