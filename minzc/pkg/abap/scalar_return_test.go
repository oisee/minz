package abap

import (
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// These are function-boundary ABI gates. The harness calls the generated
// scalar entry; no claim is made about source-level method-call admission.
func TestScalarMethodEarlyReturnVMAndZ80(t *testing.T) {
	source, err := os.ReadFile("testdata/scalar-return/method.abap")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, src string
		in, want  [3]int32
	}{
		{"return", string(source), [3]int32{-2147483648, -65537, 65537}, [3]int32{-2147483648, -65537, 65538}},
		{"exit-processing-block", strings.Replace(string(source), "RETURN.", "EXIT.", 1), [3]int32{-65537, 0, 2147483647}, [3]int32{-65537, 65538, 65538}},
		{"initial-result", strings.Replace(string(source), "METHOD calc.", "METHOD calc. IF a = 0. RETURN. ENDIF.", 1), [3]int32{0, -65537, 1}, [3]int32{0, -65537, 65538}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile(tc.src, "zref")
			if err != nil {
				t.Fatal(err)
			}
			installScalarReturnHarness(m, "lcl_calc", tc.in)
			verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(tc.want[0]), "y": sqlI32(tc.want[1]), "seen": sqlI32(tc.want[2]), "_abap_index_value": sqlI32(23)})
		})
	}
}

func installScalarReturnHarness(m *hir.Module, fn string, input [3]int32) {
	body := []hir.Stmt{charCall("sy_set_index", &hir.IntLitExpr{Val: 23, Ty: mir2.TyI32})}
	for i, name := range []string{"x", "y", "seen"} {
		body = append(body, &hir.AssignStmt{Target: &hir.VarRefExpr{Name: name, Ty: mir2.TyI32}, Val: &hir.CallExpr{Fn: fn, Args: []hir.Expr{&hir.IntLitExpr{Val: int64(input[i]), Ty: mir2.TyI32}}, Ty: mir2.TyI32}})
	}
	body = append(body, &hir.ReturnStmt{})
	m.Funcs = append(m.Funcs, &hir.Func{Name: "main", RetTy: mir2.TyVoid, Body: &hir.Block{Body: body}})
}

func TestScalarFunctionEarlyReturnVMAndZ80(t *testing.T) {
	a := FormParam{Name: "a", AbapTy: "i"}
	rv := FormParam{Name: "rv", AbapTy: "i"}
	p := &Program{Name: "zref", Decls: []Decl{
		&DataDecl{Name: "x", AbapTy: "i"}, &DataDecl{Name: "y", AbapTy: "i"}, &DataDecl{Name: "seen", AbapTy: "i"},
		&FunctionDecl{Name: "f", Importing: []FormParam{a}, Exporting: []FormParam{rv}, Body: []Stmt_{
			&AssignStmt{Target: "rv", Val: &VarRef{Name: "a"}}, &ReturnStmt{},
			&AssignStmt{Target: "rv", Val: &IntLit{Val: 99}},
		}},
	}}
	m, err := LowerProgramWithTarget(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	emitRuntimeFuncs(m, 0)
	if err := emitInteger32Runtime(m, 0); err != nil {
		t.Fatal(err)
	}
	in := [3]int32{-2147483648, 65537, 2147483647}
	installScalarReturnHarness(m, "f", in)
	verifySQLRowModel(t, m, map[string][]byte{"x": sqlI32(in[0]), "y": sqlI32(in[1]), "seen": sqlI32(in[2]), "_abap_index_value": sqlI32(23)})
}
