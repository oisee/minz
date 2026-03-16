package abap

import (
	"testing"
)

func TestParseSimple(t *testing.T) {
	src := `REPORT zhello.
DATA lv_x TYPE i VALUE 42.
WRITE lv_x.`

	prog, err := Parse(src, "zhello")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if prog.Name != "zhello" {
		t.Errorf("name = %q, want zhello", prog.Name)
	}
	if len(prog.Decls) == 0 {
		t.Fatal("no declarations parsed")
	}

	// Should have at least a DATA decl and a WRITE stmt
	var hasData, hasWrite bool
	for _, d := range prog.Decls {
		switch d.(type) {
		case *DataDecl:
			hasData = true
		case *formBodyDecl:
			hasWrite = true
		}
	}
	if !hasData {
		t.Error("missing DATA declaration")
	}
	if !hasWrite {
		t.Error("missing WRITE statement")
	}
}

func TestCompileToHIR(t *testing.T) {
	src := `REPORT zhello.
DATA lv_x TYPE i VALUE 42.
WRITE lv_x.`

	hm, err := Compile(src, "zhello")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if hm.Name != "zhello" {
		t.Errorf("module name = %q, want zhello", hm.Name)
	}

	// Should have globals (lv_x) and functions (main, abap_write)
	if len(hm.Globals) == 0 {
		t.Error("no globals")
	}
	if len(hm.Funcs) == 0 {
		t.Error("no functions")
	}

	// Check that abap_write runtime was emitted
	hasWriteRT := false
	for _, f := range hm.Funcs {
		if f.Name == "abap_write" {
			hasWriteRT = true
		}
	}
	if !hasWriteRT {
		t.Error("missing abap_write runtime function")
	}

	t.Logf("HIR module: %d globals, %d funcs, %d strings",
		len(hm.Globals), len(hm.Funcs), len(hm.Strings))
}
