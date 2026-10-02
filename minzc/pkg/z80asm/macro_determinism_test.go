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
		// Preserve sequential parameter substitution in declaration order,
		// including when an earlier argument contains a later parameter name.
		if !reflect.DeepEqual(lines, []string{"DB 7, 7"}) {
			t.Fatalf("unstable macro expansion: %v", lines)
		}
	}
}
