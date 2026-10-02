package z80asm

import (
	"reflect"
	"testing"
)

func TestMacroSubstitutionDeterministic(t *testing.T) {
	mp := NewMacroProcessor()
	if err := mp.DefineMacro("values", []string{"x", "y"}, []string{"DB x, y"}); err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 100; run++ {
		lines, err := mp.ExpandMacro("values", []string{"y", "7"})
		if err != nil {
			t.Fatal(err)
		}
		// Argument text must not be substituted again.
		if !reflect.DeepEqual(lines, []string{"DB y, 7"}) {
			t.Fatalf("unstable macro expansion: %v", lines)
		}
	}
}

func TestMacroSubstitutionSimultaneous(t *testing.T) {
	mp := NewMacroProcessor()
	if err := mp.DefineMacro("PAIR", []string{"A", "B"}, []string{"LD A,B", "DB {A},%B,&A", "DB AB,B_suffix,(A)"}); err != nil {
		t.Fatal(err)
	}
	got, err := mp.ExpandMacro("PAIR", []string{"B+1", "7"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"LD B+1,7", "DB B+1,7,B+1", "DB AB,B_suffix,(B+1)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
