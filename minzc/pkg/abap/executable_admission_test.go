package abap

import (
	"strings"
	"testing"
)

func TestExecutableAdmissionUnknownNames(t *testing.T) {
	for _, statement := range []string{
		"missing = 1.",
		"x = missing.",
		"WRITE missing.",
		"IF missing = 1. x = 2. ENDIF.",
	} {
		for _, scope := range []string{"report", "form"} {
			t.Run(scope+"/"+statement, func(t *testing.T) {
				body := statement
				if scope == "form" {
					body = "FORM probe. " + body + " ENDFORM."
				}
				module, err := Compile("REPORT zadmit. DATA x TYPE i. "+body, "zadmit")
				if module != nil || err == nil || !strings.Contains(err.Error(), "undeclared") {
					t.Fatalf("executable module returned=%t error=%v; expected undeclared-object refusal", module != nil, err)
				}
			})
		}
	}
}

func TestExecutableAdmissionExceptionBlocks(t *testing.T) {
	for _, body := range []string{
		"TRY. x = x / 0. CATCH cx_sy_zerodivide. x = 0. ENDTRY.",
		"FORM probe. TRY. x = x / 0. CATCH cx_sy_zerodivide. x = 0. ENDTRY. ENDFORM.",
	} {
		module, err := Compile("REPORT zadmit. DATA x TYPE i VALUE 1. "+body, "zadmit")
		if module != nil || err == nil || !strings.Contains(err.Error(), "TRY/CATCH") {
			t.Fatalf("exception control flow: executable module returned=%t error=%v", module != nil, err)
		}
		t.Logf("explicit refusal: %v", err)
	}
}

func TestExecutableAdmissionEventAdditions(t *testing.T) {
	for _, header := range []string{
		"AT SELECTION-SCREEN OUTPUT.",
		"AT SELECTION-SCREEN ON x.",
		"AT SELECTION-SCREEN ON EXIT-COMMAND.",
	} {
		t.Run(header, func(t *testing.T) {
			p, err := Parse("REPORT zadmit. PARAMETERS x TYPE i. "+header+" x = 1.", "zadmit")
			if p != nil || err == nil || !strings.Contains(err.Error(), "event addition") {
				t.Fatalf("program returned=%t error=%v; expected event addition refusal", p != nil, err)
			}
		})
	}
	if _, err := Parse("REPORT zadmit. PARAMETERS x TYPE i. AT SELECTION-SCREEN. x = 1.", "zadmit"); err != nil {
		t.Fatalf("unmodified validation event refused: %v", err)
	}
}

func TestExecutableAdmissionSelectionLayout(t *testing.T) {
	for _, layout := range []string{
		"SELECTION-SCREEN BEGIN OF BLOCK b1. PARAMETERS x TYPE i. SELECTION-SCREEN END OF BLOCK b1.",
		"SELECTION-SCREEN SKIP 2.",
		"SELECTION-SCREEN ULINE.",
	} {
		t.Run(layout, func(t *testing.T) {
			module, err := Compile("REPORT zadmit. "+layout, "zadmit")
			if module != nil || err == nil || !strings.Contains(err.Error(), "SELECTION-SCREEN") {
				t.Fatalf("layout silently admitted: module=%t err=%v", module != nil, err)
			}
		})
	}
	if _, err := Compile("REPORT zadmit. PARAMETERS x TYPE i DEFAULT 7. START-OF-SELECTION. x = x + 1.", "zadmit"); err != nil {
		t.Fatalf("plain PARAMETERS input refused: %v", err)
	}
}
