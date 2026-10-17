package abap

import (
	"strings"
	"testing"
)

func TestRuntimeNamespaceAdmission(t *testing.T) {
	for _, src := range []string{
		"DATA _abap_error_code TYPE i.", "FORM sel_show. ENDFORM.",
		"FORM f. DATA _abap_index_value TYPE i. ENDFORM.",
		"PARAMETERS sqlite_query TYPE i.",
	} {
		m, err := Compile("REPORT zref. "+src, "zref")
		if err == nil || m != nil || !strings.Contains(err.Error(), "reserved compiler runtime namespace") {
			t.Fatalf("source=%s module=%v error=%v", src, m, err)
		}
	}
}
