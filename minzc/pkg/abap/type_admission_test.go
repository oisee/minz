package abap

import (
	"strings"
	"testing"
)

func TestDeclarationAdmissionRejectsLostSemantics(t *testing.T) {
	cases := []struct{ source, message string }{
		{"DATA x TYPE int8.", "TYPE int8"},
		{"DATA x TYPE f.", "TYPE f"},
		{"DATA x TYPE p DECIMALS 2.", "DECIMALS"},
		{"DATA x TYPE p.", "TYPE p"},
		{"DATA x TYPE n LENGTH 4.", "TYPE n"},
		{"DATA x TYPE x LENGTH 4.", "TYPE x"},
		{"DATA x TYPE d.", "TYPE d"},
		{"DATA x TYPE t.", "TYPE t"},
		{"DATA x TYPE zunknown.", "TYPE zunknown"},
		{"DATA x LIKE sy-index.", "LIKE"},
		{"PARAMETERS x TYPE i OBLIGATORY.", "OBLIGATORY"},
		{"PARAMETERS x TYPE i AS CHECKBOX.", "AS"},
		{"PARAMETERS x TYPE int8.", "TYPE int8"},
		{"FORM probe CHANGING x TYPE int8. ENDFORM.", "FORM probe parameter x"},
		{"FORM probe. DATA x TYPE f. ENDFORM.", "DATA x"},
		{"FORM probe. IF 1 = 1. DATA x TYPE int8. ENDIF. ENDFORM.", "DATA x"},
		{"START-OF-SELECTION. DO 1 TIMES. DATA x TYPE f. ENDDO.", "DATA x"},
	}
	for _, c := range cases {
		t.Run(c.source, func(t *testing.T) {
			m, err := Compile("REPORT ztypes.\n"+c.source, "ztypes")
			if err == nil || !strings.Contains(err.Error(), c.message) {
				t.Fatalf("module=%v error=%v; expected %s", m, err, c.message)
			}
			if m != nil {
				t.Fatal("refused source returned executable module")
			}
		})
	}
}

func TestDeclarationAdmissionPreservesDefaultsAndInitial(t *testing.T) {
	p, err := Parse("REPORT ztypes.\nDATA x.\nDATA y TYPE i VALUE IS INITIAL.\nPARAMETERS z TYPE i DEFAULT -7.", "ztypes")
	if err != nil {
		t.Fatal(err)
	}
	var x, y *DataDecl
	for _, d := range p.Decls {
		if v, ok := d.(*DataDecl); ok {
			if v.Name == "x" {
				x = v
			}
			if v.Name == "y" {
				y = v
			}
		}
	}
	if x == nil || x.AbapTy != "c" || x.Length != 0 {
		t.Fatalf("implicit character DATA: %+v", x)
	}
	if y == nil || y.AbapTy != "i" || y.Value != nil {
		t.Fatalf("VALUE IS INITIAL: %+v", y)
	}
	if len(p.Params) != 1 || p.Params[0].Length != 0 {
		t.Fatalf("numeric parameter got character default length: %+v", p.Params)
	}
	if _, err := LowerProgram(p); err != nil {
		t.Fatal(err)
	}
}

func TestDeclarationAdmissionNoIgnoredTokens(t *testing.T) {
	for _, toks := range [][]string{
		{"DATA", "x", "TYPE", "i", "LENGTH", "2", "."},
		{"DATA", "x", "TYPE", "i", "TYPE", "c", "."},
		{"DATA", "x", "TYPE", "c", "LENGTH", "0", "."},
		{"DATA", "x", "TYPE", "c", "LENGTH", "unknown", "."},
		{"DATA", "x", "TYPE", "."},
		{"DATA", "x", "LENGTH", "."},
		{"DATA", "x", "TYPE", "i", "VALUE", "7", "READ-ONLY", "."},
	} {
		if d, err := parseDataStmt(toks); err == nil {
			t.Fatalf("accepted unrepresented tokens %v as %+v", toks, d)
		}
	}
}
