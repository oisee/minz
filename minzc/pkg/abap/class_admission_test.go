package abap

import (
	"strings"
	"testing"
)

func TestClassDeclarationsDoNotLoseSemantics(t *testing.T) {
	cases := []struct{ name, source, diagnostic string }{
		{"attribute-initializer", "CLASS lcl DEFINITION. PUBLIC SECTION. DATA x TYPE i VALUE 7. ENDCLASS.", "VALUE initialization"},
		{"attribute-readonly", "CLASS lcl DEFINITION. PUBLIC SECTION. DATA x TYPE i READ-ONLY. ENDCLASS.", "unsupported declaration addition"},
		{"inheritance", "CLASS lcl DEFINITION INHERITING FROM parent. ENDCLASS.", "definition additions"},
		{"final", "CLASS lcl DEFINITION FINAL. ENDCLASS.", "definition additions"},
		{"class-data", "CLASS lcl DEFINITION. PUBLIC SECTION. CLASS-DATA x TYPE i. ENDCLASS.", "unsupported section member"},
		{"interface", "CLASS lcl DEFINITION. PUBLIC SECTION. INTERFACES lif. ENDCLASS.", "unsupported section member"},
		{"exporting", "CLASS lcl DEFINITION. PUBLIC SECTION. METHODS f EXPORTING x TYPE i. ENDCLASS.", "unsupported signature addition"},
		{"changing", "CLASS lcl DEFINITION. PUBLIC SECTION. METHODS f CHANGING x TYPE i. ENDCLASS.", "unsupported signature addition"},
		{"optional", "CLASS lcl DEFINITION. PUBLIC SECTION. METHODS f IMPORTING x TYPE i OPTIONAL. ENDCLASS.", "fully typed parameters"},
		{"raising", "CLASS lcl DEFINITION. PUBLIC SECTION. METHODS f RAISING cx_error. ENDCLASS.", "unsupported signature addition"},
		{"unknown-method", "CLASS lcl DEFINITION. ENDCLASS. CLASS lcl IMPLEMENTATION. METHOD f. ENDMETHOD. ENDCLASS.", "no matching declaration"},
		{"unknown-class", "CLASS lcl IMPLEMENTATION. METHOD f. ENDMETHOD. ENDCLASS.", "no matching definition"},
		{"duplicate-method", "CLASS lcl DEFINITION. PUBLIC SECTION. METHODS f. METHODS f. ENDCLASS.", "duplicate member"},
		{"missing-body", "CLASS lcl DEFINITION. PUBLIC SECTION. METHODS f. ENDCLASS.", "no implementation"},
		{"duplicate-body", "CLASS lcl DEFINITION. PUBLIC SECTION. METHODS f. ENDCLASS. CLASS lcl IMPLEMENTATION. METHOD f. ENDMETHOD. METHOD f. ENDMETHOD. ENDCLASS.", "duplicate implementation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile("REPORT zadmit. "+tc.source, "zadmit")
			if m != nil || err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("module=%t err=%v want %q", m != nil, err, tc.diagnostic)
			}
			t.Log(err)
		})
	}
}

func TestClassEmptyImplementationIsPresent(t *testing.T) {
	p, err := Parse("REPORT zadmit. CLASS lcl DEFINITION. PUBLIC SECTION. METHODS f. ENDCLASS. CLASS lcl IMPLEMENTATION. METHOD f. ENDMETHOD. ENDCLASS.", "zadmit")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range p.Decls {
		if c, ok := d.(*ClassDecl); ok {
			if len(c.Methods) != 1 || c.Methods[0].Body == nil {
				t.Fatal("empty implementation confused with missing implementation")
			}
			return
		}
	}
	t.Fatal("class disappeared")
}

func TestClassValueImportingVMAndZ80(t *testing.T) {
	src := `REPORT zref. DATA x TYPE i. DATA y TYPE i. DATA seen TYPE i.
 CLASS lcl DEFINITION. PUBLIC SECTION.
 CLASS-METHODS calc IMPORTING VALUE(a) TYPE i RETURNING VALUE(rv) TYPE i.
 ENDCLASS.
 CLASS lcl IMPLEMENTATION. METHOD calc. rv = a. RETURN. rv = 99. ENDMETHOD. ENDCLASS.`
	m, err := Compile(src, "zref")
	if err != nil {
		t.Fatal(err)
	}
	in := [3]int32{-2147483648, 65537, 2147483647}
	installScalarReturnHarness(m, "lcl_calc", in)
	verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(in[0]), "y": sqlI32(in[1]), "seen": sqlI32(in[2]), "_abap_index_value": sqlI32(23)})
}

func TestClassSemanticASTCannotBypassAdmission(t *testing.T) {
	for _, c := range []*ClassDecl{
		{Name: "c", Super: "parent"},
		{Name: "c", Interfaces: []string{"i"}},
		{Name: "c", Methods: []*MethodDecl{{Name: "f", Exporting: []FormParam{{Name: "x", AbapTy: "i"}}}}},
		{Name: "c", Methods: []*MethodDecl{{Name: "f", Changing: []FormParam{{Name: "x", AbapTy: "i"}}}}},
	} {
		if m, err := LowerProgram(&Program{Name: "zref", Decls: []Decl{c}}); m != nil || err == nil {
			t.Fatalf("incomplete class ABI admitted: %+v module=%t err=%v", c, m != nil, err)
		}
	}
}
