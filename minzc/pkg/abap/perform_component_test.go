package abap

import (
	"strings"
	"testing"
)

func TestPerformIntegerComponentReference(t *testing.T) {
	src, want := itabReferenceExpectation(t, "testdata/itab-reference/perform-component.abap")
	verifyItabReport(t, "perform-component", src, want)
}

func TestPerformComponentsAliasAndForwardReferences(t *testing.T) {
	src, want := itabReferenceExpectation(t, "testdata/itab-reference/perform-alias.abap")
	verifyItabReport(t, "perform-alias", src, want)
}

func TestPerformComponentsRejectIncompatibleActuals(t *testing.T) {
	const prefix = `REPORT zbad.
TYPES: BEGIN OF ty, n TYPE i, tag TYPE c LENGTH 3, END OF ty.
DATA ls TYPE ty.
DATA tab TYPE STANDARD TABLE OF ty WITH EMPTY KEY.
`
	for _, tc := range []struct{ name, call, param, want string }{
		{"character-to-integer", "ls-tag", "i", "incompatible type"},
		{"integer-to-character", "ls-n", "c", "needs a character object"},
		{"character-to-string", "ls-tag", "string", "needs an owned string object"},
		{"missing-component", "ls-missing", "i", "has no component"},
		{"undeclared-structure", "missing-n", "i", "not a declared structure"},
		{"table-component", "tab-n", "i", "internal table body"},
		{"nested-component", "ls-n-sub", "i", "nested structures"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, section := range []string{"USING", "CHANGING"} {
				src := prefix + "PERFORM f " + section + " " + tc.call + ".\nFORM f " + section + " p TYPE " + tc.param + ".\nENDFORM."
				m, err := Compile(src, "bad")
				if m != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("%s: executable=%t error=%v, want %q", section, m != nil, err, tc.want)
				}
			}
		})
	}
}
