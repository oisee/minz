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

func TestSelectionScreen(t *testing.T) {
	src := `REPORT zsel.
PARAMETERS: p_name TYPE c LENGTH 10 DEFAULT 'Test',
            p_num  TYPE i DEFAULT 5.
INITIALIZATION.
  WRITE 'init'.
START-OF-SELECTION.
  WRITE p_name.
  WRITE p_num.`

	prog, err := Parse(src, "zsel")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Should have PARAMETERS
	if len(prog.Params) != 2 {
		t.Errorf("params = %d, want 2", len(prog.Params))
	} else {
		if prog.Params[0].Name != "p_name" {
			t.Errorf("param[0].Name = %q, want p_name", prog.Params[0].Name)
		}
		if prog.Params[1].Name != "p_num" {
			t.Errorf("param[1].Name = %q, want p_num", prog.Params[1].Name)
		}
	}

	// Should have events
	if _, ok := prog.Events["INITIALIZATION"]; !ok {
		t.Error("missing INITIALIZATION event")
	}
	if _, ok := prog.Events["START-OF-SELECTION"]; !ok {
		t.Error("missing START-OF-SELECTION event")
	}

	// Compile to HIR
	hm, err := Compile(src, "zsel")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Should have main function with event flow
	hasMain := false
	hasSYGetter := false
	hasSelShow := false
	for _, f := range hm.Funcs {
		if f.Name == "main" {
			hasMain = true
		}
		if f.Name == "sy_get_index" {
			hasSYGetter = true
		}
		if f.Name == "sel_show" {
			hasSelShow = true
		}
	}
	if !hasMain {
		t.Error("missing main function")
	}
	if !hasSYGetter {
		t.Error("missing SY getter functions")
	}
	if !hasSelShow {
		t.Error("missing sel_show function")
	}

	t.Logf("HIR: %d globals, %d funcs, %d structs, params=%d, events=%d",
		len(hm.Globals), len(hm.Funcs), len(hm.Structs),
		len(prog.Params), len(prog.Events))
}
