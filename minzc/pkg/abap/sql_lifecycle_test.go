package abap

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestSQLLexicalCleanupVMAndZ80(t *testing.T) {
	for _, tc := range []struct {
		name, body               string
		finite                   bool
		rows                     int
		trace, indexTrace, steps int32
	}{
		{name: "form-return", body: "FORM probe. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDFORM. PERFORM probe.", trace: 1, indexTrace: 23},
		{name: "select-do-return", body: "FORM probe. SELECT n FROM t INTO x. DO 2 TIMES. RETURN. ENDDO. ENDSELECT. ENDFORM. PERFORM probe.", trace: 1, indexTrace: 23},
		{name: "do-select-return", body: "FORM probe. DO 2 TIMES. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDDO. ENDFORM. PERFORM probe.", trace: 1, indexTrace: 1},
		{name: "nested-select-return", body: "FORM probe. SELECT n FROM t INTO x. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDSELECT. ENDFORM. PERFORM probe.", trace: 21, indexTrace: 2323},
		{name: "select-do-select-return", body: "FORM probe. SELECT n FROM t INTO x. DO 2 TIMES. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDDO. ENDSELECT. ENDFORM. PERFORM probe.", trace: 21, indexTrace: 123},
		{name: "callee-does-not-close-caller", body: "FORM probe. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDFORM. SELECT n FROM t INTO x. PERFORM probe. EXIT. ENDSELECT.", trace: 21, indexTrace: 2323},
		{name: "callee-keeps-caller-index", body: "FORM probe. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDFORM. DO 2 TIMES. PERFORM probe. EXIT. ENDDO.", trace: 1, indexTrace: 1},
		{name: "continue", body: "SELECT n FROM t INTO x. CONTINUE. seen = 99. ENDSELECT.", finite: true, rows: 3, trace: 1, indexTrace: 23, steps: 4},
		{name: "normal-exhaustion", body: "SELECT n FROM t INTO x. ENDSELECT.", finite: true, rows: 3, trace: 1, indexTrace: 23, steps: 4},
		{name: "main-return", body: "SELECT n FROM t INTO x. RETURN. ENDSELECT. seen = 99.", trace: 1, indexTrace: 23},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile("REPORT zref. DATA x TYPE i VALUE 17. DATA y TYPE i. DATA seen TYPE i. "+tc.body, "zref")
			if err != nil {
				t.Fatal(err)
			}
			installSQLLifecycleModel(m, tc.finite, tc.rows)
			want := map[string][]byte{"x": sqlI32(65537), "y": sqlI32(tc.indexTrace), "seen": sqlI32(tc.trace), "_abap_index_value": sqlI32(23)}
			if tc.finite {
				want["_sql_test_steps"] = sqlI32(tc.steps)
			}
			verifySQLRowModel(t, m, want)
		})
	}
}

// A test-owned row model supplies distinct handles and logs close order and
// the index observed at each close. This is resource/control-flow evidence,
// not execution against a native database server or full-report SAP parity.
func installSQLLifecycleModel(m *hir.Module, finite bool, rows int) {
	installSQLRowModel(m, "65537", 1)
	m.Globals = append(m.Globals, mir2.Global{Name: "_sql_test_queries", Ty: mir2.TyI32}, mir2.Global{Name: "_sql_test_steps", Ty: mir2.TyI32})
	for n := range m.Globals {
		if m.Globals[n].Name == "_abap_index_value" {
			m.Globals[n].Init = sqlI32(23)
		}
	}
	value := func(name string) hir.Expr { return &hir.VarRefExpr{Name: name, Ty: mir2.TyI32} }
	number := func(n int64) hir.Expr { return &hir.IntLitExpr{Val: n, Ty: mir2.TyI32} }
	add := func(a, b hir.Expr) hir.Expr {
		return &hir.CallExpr{Fn: "_abap_add", Args: []hir.Expr{a, b}, Ty: mir2.TyI32}
	}
	mul := func(a, b hir.Expr) hir.Expr {
		return &hir.CallExpr{Fn: "_abap_mul", Args: []hir.Expr{a, b}, Ty: mir2.TyI32}
	}
	set := func(name string, e hir.Expr) hir.Stmt {
		return &hir.AssignStmt{Target: &hir.VarRefExpr{Name: name, Ty: mir2.TyI32}, Val: e}
	}
	for _, f := range m.Funcs {
		switch f.Name {
		case "sqlite_query":
			f.Body = &hir.Block{Body: []hir.Stmt{set("_sql_test_queries", add(value("_sql_test_queries"), number(1))), &hir.ReturnStmt{Val: &hir.CastExpr{X: value("_sql_test_queries"), Ty: mir2.TyU16}}}}
		case "sqlite_step":
			if finite {
				f.Body = &hir.Block{Body: []hir.Stmt{set("_sql_test_steps", add(value("_sql_test_steps"), number(1))), &hir.ReturnStmt{Val: &hir.CastExpr{X: &hir.BinExpr{Op: "<=", L: value("_sql_test_steps"), R: number(int64(rows)), Ty: mir2.TyBool}, Ty: mir2.TyU8}}}}
			}
		case "sqlite_finalize":
			handle := &hir.CastExpr{X: &hir.VarRefExpr{Name: f.Params[0].Name, Ty: f.Params[0].Ty}, Ty: mir2.TyI32}
			f.Body = &hir.Block{Body: []hir.Stmt{set("seen", add(mul(value("seen"), number(10)), handle)), set("y", add(mul(value("y"), number(100)), indexGet())), &hir.ReturnStmt{Val: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}}}}
		}
	}
}

func TestSQLPrototypeTablesAreNotAdmitted(t *testing.T) {
	for _, tc := range []struct{ source, refusal string }{
		{"REPORT zadmit. SELECT n FROM t INTO TABLE @DATA(items).", "prototype is not admitted"},
		{"REPORT zadmit. cl_salv_table=>factory( items ).", "CL_SALV_TABLE FACTORY parameter/section items is not admitted"},
	} {
		m, err := Compile(tc.source, "zadmit")
		if m != nil || err == nil || !strings.Contains(err.Error(), tc.refusal) {
			t.Fatalf("prototype silently admitted: module=%t err=%v", m != nil, err)
		}
	}
	for _, statement := range []Stmt_{
		&SelectStmt{Fields: []string{"n"}, Table: "t", IntoTable: "items"}, &ALVDisplayStmt{TableVar: "items"},
	} {
		p := &Program{Name: "zadmit", Decls: []Decl{&formBodyDecl{stmt: statement}}}
		if m, err := LowerProgram(p); m != nil || err == nil {
			t.Fatalf("AST bypassed prototype admission: module=%t err=%v", m != nil, err)
		}
	}
}
