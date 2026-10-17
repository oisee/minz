package abap

import (
	"strings"
	"testing"
)

func TestReportAndFormAdditionsDoNotDisappear(t *testing.T) {
	for _, tc := range []struct{ name, source, diagnostic string }{
		{"line-size", "REPORT zadmit LINE-SIZE 5.", "REPORT additions"},
		{"page-heading", "REPORT zadmit NO STANDARD PAGE HEADING.", "REPORT additions"},
		{"message-id", "REPORT zadmit MESSAGE-ID sabapdocu.", "REPORT additions"},
		{"tables", "REPORT zadmit. FORM probe TABLES items. ENDFORM.", "FORM header section FormTables"},
		{"raising", "REPORT zadmit. FORM probe RAISING cx_error. ENDFORM.", "FORM header section FormRaising"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile(tc.source, "zadmit")
			if m != nil || err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("module=%t err=%v want %q", m != nil, err, tc.diagnostic)
			}
			t.Log(err)
		})
	}
}

func TestFunctionSemanticASTCannotLoseOutputABI(t *testing.T) {
	for _, tc := range []struct {
		name, diagnostic string
		decl             *FunctionDecl
	}{
		{"changing", "CHANGING reference ABI", &FunctionDecl{Name: "f", Changing: []FormParam{{Name: "x", AbapTy: "i"}}}},
		{"multiple-results", "multiple EXPORTING results", &FunctionDecl{Name: "f", Exporting: []FormParam{{Name: "x", AbapTy: "i"}, {Name: "y", AbapTy: "i"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := LowerProgram(&Program{Name: "zadmit", Decls: []Decl{tc.decl}})
			if m != nil || err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("module=%t err=%v want %q", m != nil, err, tc.diagnostic)
			}
		})
	}
}
