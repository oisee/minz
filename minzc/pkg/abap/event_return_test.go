package abap

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Oracle: ABAP 7.52 RETURN exits the current processing block normally, except
// START-OF-SELECTION stops further reporting events. The independent 2.14.8
// transpiler explicitly refuses INITIALIZATION, so this is specification-based
// event-driver evidence, not another independent full-report reference gate.
// https://help.sap.com/doc/abapdocu_752_index_htm/7.52/en-US/abapreturn.htm
func TestEventReturnBoundariesVMAndZ80(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		x, y, seen int32
	}{
		{"initialization", "INITIALIZATION. x = 1. RETURN. x = 99. START-OF-SELECTION. y = 2. END-OF-SELECTION. seen = 3.", 1, 2, 3},
		{"initialization-loop", "INITIALIZATION. DO 2 TIMES. x = 1. RETURN. ENDDO. START-OF-SELECTION. y = sy-index. END-OF-SELECTION. seen = 3.", 1, 0, 3},
		{"start-stops-reporting", "INITIALIZATION. x = 1. START-OF-SELECTION. y = 2. RETURN. y = 99. END-OF-SELECTION. seen = 3.", 1, 2, 0},
		{"end", "INITIALIZATION. x = 1. START-OF-SELECTION. y = 2. END-OF-SELECTION. seen = 3. RETURN. seen = 99.", 1, 2, 3},
		{"selection-validation", "PARAMETERS p TYPE i DEFAULT 7. INITIALIZATION. x = 1. AT SELECTION-SCREEN. x = 2. RETURN. x = 99. START-OF-SELECTION. y = 3. END-OF-SELECTION. seen = 4.", 2, 3, 4},
		{"implicit-start-prefix", "x = 1. START-OF-SELECTION. y = x + 2. END-OF-SELECTION. seen = y + 3.", 1, 3, 6},
		{"repeated-start", "START-OF-SELECTION. x = 1. START-OF-SELECTION. y = x + 2. END-OF-SELECTION. seen = y + 3.", 1, 3, 6},
		{"repeated-initialization-return", "INITIALIZATION. x = 1. RETURN. x = 99. INITIALIZATION. y = 2. START-OF-SELECTION. seen = x + y.", 1, 2, 3},
		{"repeated-end-return", "START-OF-SELECTION. x = 1. END-OF-SELECTION. y = 2. RETURN. y = 99. END-OF-SELECTION. seen = x + y.", 1, 2, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile("REPORT zref. DATA x TYPE i. DATA y TYPE i. DATA seen TYPE i. "+tc.body, "zref")
			if err != nil {
				t.Fatal(err)
			}
			// A fixed input adapter supplies one valid selection result. It models
			// input transport only; ABAP event/control-flow bodies are unchanged.
			for _, f := range m.Funcs {
				if strings.HasPrefix(f.Name, "sel_") {
					f.IsExtern = false
					var result hir.Expr
					if f.RetTy != mir2.TyVoid {
						n := int64(0)
						if f.Name == "sel_show" {
							n = 1
						}
						if f.Name == "sel_get_int" {
							n = 7
						}
						result = &hir.IntLitExpr{Val: n, Ty: f.RetTy}
					}
					f.Body = &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: result}}}
				}
			}
			verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(tc.x), "y": sqlI32(tc.y), "seen": sqlI32(tc.seen), "_abap_index_value": sqlI32(0)})
		})
	}
}

func TestSelectionEventDataIsLocal(t *testing.T) {
	src := `REPORT zref. DATA x TYPE i VALUE 7. DATA y TYPE i. DATA seen TYPE i.
PARAMETERS p TYPE i DEFAULT 7.
AT SELECTION-SCREEN.
DATA x TYPE i VALUE 3.
y = x.
START-OF-SELECTION.
seen = x.`
	m, err := Compile(src, "zref")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range m.Funcs {
		if strings.HasPrefix(f.Name, "sel_") {
			f.IsExtern = false
			var result hir.Expr
			if f.RetTy != mir2.TyVoid {
				value := int64(0)
				if f.Name == "sel_show" {
					value = 1
				}
				if f.Name == "sel_get_int" {
					value = 7
				}
				result = &hir.IntLitExpr{Val: value, Ty: f.RetTy}
			}
			f.Body = &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: result}}}
		}
	}
	verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(7), "y": sqlI32(3), "seen": sqlI32(7)})
}

func TestEventMetadataCannotHideChangedBody(t *testing.T) {
	p, err := Parse("REPORT zref. DATA x TYPE i. INITIALIZATION. x = 1.", "zref")
	if err != nil {
		t.Fatal(err)
	}
	p.Events["INITIALIZATION"] = append(p.Events["INITIALIZATION"], &AssignStmt{Target: "x", Val: &IntLit{Val: 2}})
	m, err := LowerProgram(p)
	if m != nil || err == nil || !strings.Contains(err.Error(), "event block metadata") {
		t.Fatalf("edited event body ignored: module=%t err=%v", m != nil, err)
	}
}
