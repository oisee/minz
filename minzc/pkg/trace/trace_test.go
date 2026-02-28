package trace

import (
	"bytes"
	"strings"
	"testing"
)

func TestNilTracerIsNoOp(t *testing.T) {
	var tr *Tracer
	tr.Log("semantic", "should not panic") // must not panic
}

func TestNewNilWriter(t *testing.T) {
	tr := New(nil)
	if tr != nil {
		t.Fatal("New(nil) should return nil")
	}
	tr.Log("test", "still safe") // must not panic
}

func TestLogFormat(t *testing.T) {
	var buf bytes.Buffer
	tr := New(&buf)
	tr.Log("semantic", "DJNZ loop: array[%d]", 5)
	tr.Log("optimizer", "ConstantFolding: %d ops", 3)
	tr.Log("codegen", "Peephole: %d patterns applied", 12)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}

	expected := []string{
		"[semantic ] DJNZ loop: array[5]",
		"[optimizer] ConstantFolding: 3 ops",
		"[codegen  ] Peephole: 12 patterns applied",
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d:\n  got:  %q\n  want: %q", i, lines[i], want)
		}
	}
}
