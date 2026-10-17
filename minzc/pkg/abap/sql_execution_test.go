package abap

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestSQLSeedAndNativeBlockUseOpenedHandle(t *testing.T) {
	source := `*!sql CREATE TABLE t(n);
REPORT zsql.
DATA x TYPE i VALUE 17. DATA y TYPE i. DATA seen TYPE i.
EXEC SQL.
INSERT INTO t VALUES(65537);
ENDEXEC.
SELECT SINGLE n FROM t INTO x.`
	m, err := Compile(source, "zsql")
	if err != nil {
		t.Fatal(err)
	}
	installSQLRowModel(m, "65537", 1)
	for _, f := range m.Funcs {
		switch f.Name {
		case "sqlite_open":
			f.Body = &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.IntLitExpr{Val: 17, Ty: mir2.TyU16}}}}
		case "sqlite_exec", "sqlite_query":
			check := &hir.IfStmt{
				Cond: &hir.BinExpr{Op: "!=", L: &hir.VarRefExpr{Name: f.Params[0].Name, Ty: mir2.TyU16}, R: &hir.IntLitExpr{Val: 17, Ty: mir2.TyU16}, Ty: mir2.TyBool},
				Then: &hir.Block{Body: []hir.Stmt{charCall("_abap_fault", &hir.IntLitExpr{Val: 6, Ty: mir2.TyU8})}},
			}
			f.Body.Body = append([]hir.Stmt{check}, f.Body.Body...)
			if f.Name == "sqlite_exec" {
				f.Body.Body = append([]hir.Stmt{&hir.AssignStmt{Target: &hir.VarRefExpr{Name: "y", Ty: mir2.TyI32}, Val: &hir.BinExpr{Op: "+", L: &hir.VarRefExpr{Name: "y", Ty: mir2.TyI32}, R: &hir.IntLitExpr{Val: 1, Ty: mir2.TyI32}, Ty: mir2.TyI32}}}, f.Body.Body...)
			}
		}
		if strings.HasPrefix(f.Name, "_abap_seed_") {
			t.Fatal("seed still uses an opaque ASM wrapper")
		}
	}
	verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(65537), "y": sqlI32(2), "seen": sqlI32(1), "_abap_db": {17, 0}})
}

func TestSQLFailuresDoNotLookLikeSuccess(t *testing.T) {
	for _, tc := range []struct {
		name, source, fn string
		result           int64
	}{
		{"open", "SELECT SINGLE n FROM t INTO x.", "sqlite_open", 0},
		{"step", "SELECT SINGLE n FROM t INTO x.", "sqlite_step", 2},
		{"exec", "EXEC SQL.\nINSERT INTO t VALUES(1);\nENDEXEC.", "sqlite_exec", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile("REPORT zsql. DATA x TYPE i VALUE 17. DATA y TYPE i. DATA seen TYPE i.\n"+tc.source+"\ny = 99.", "zsql")
			if err != nil {
				t.Fatal(err)
			}
			installSQLRowModel(m, "65537", 1)
			for _, f := range m.Funcs {
				if f.Name == tc.fn {
					f.Body = &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.IntLitExpr{Val: tc.result, Ty: f.RetTy}}}}
				}
			}
			verifySQLRowModelWithFault(t, m, map[string][]byte{"x": sqlI32(17), "y": sqlI32(0), "seen": sqlI32(0)}, 6)
		})
	}
}

func TestNativeSQLPreservesLiteralAndAdjacentTokens(t *testing.T) {
	p, err := Parse("REPORT zsql.\nEXEC SQL.\nINSERT INTO t VALUES(1.5,'SQL: EXEC.');\nENDEXEC.", "zsql")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range p.Decls {
		if b, ok := d.(*formBodyDecl); ok {
			if s, ok := b.stmt.(*ExecSQLStmt); ok {
				if s.SQL != "INSERT INTO t VALUES(1.5,'SQL: EXEC.');" {
					t.Fatalf("SQL=%q", s.SQL)
				}
				return
			}
		}
	}
	t.Fatal("SQL body disappeared")
}

func TestNativeSQLHostBindingsAreExplicitRefusal(t *testing.T) {
	m, err := Compile("REPORT zsql. DATA x TYPE i.\nEXEC SQL.\nINSERT INTO t VALUES(:x);\nENDEXEC.", "zsql")
	if m != nil || err == nil || !strings.Contains(err.Error(), "host bindings") {
		t.Fatalf("module=%t err=%v", m != nil, err)
	}
}

func TestSQLSeedTextTransportAdmission(t *testing.T) {
	for _, sql := range []string{"INSERT INTO t VALUES('a\x00b')", "INSERT INTO t VALUES('é')"} {
		m, err := LowerProgram(&Program{Name: "zsql", SeedSQL: []string{sql}})
		if m != nil || err == nil || !strings.Contains(err.Error(), "ASCII without NUL") {
			t.Fatalf("module=%t err=%v", m != nil, err)
		}
	}
	m, err := Compile("REPORT zsql. DATA x TYPE i. SELECT SINGLE n FROM t WHERE label = 'a\x00b' INTO x.", "zsql")
	if m != nil || err == nil || !strings.Contains(err.Error(), "ASCII without NUL") {
		t.Fatalf("query text truncated: module=%t err=%v", m != nil, err)
	}
}
