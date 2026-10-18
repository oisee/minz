package abap

import "testing"

func TestSALVSemanticReference(t *testing.T) {
	src, want := itabReferenceExpectation(t, "testdata/salv-reference/csv-semantic-excerpt.abap")
	verifyItabReport(t, "salv-semantic-excerpt", src, want)
}
