package hir_test

import (
	"os/exec"
	"testing"
)

func TestJudgeScripts(t *testing.T) {
	t.Parallel()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	cmd := exec.Command(python, "../../../scripts/test_judges.py")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("judge script self-tests: %v\n%s", err, out)
	}
}
