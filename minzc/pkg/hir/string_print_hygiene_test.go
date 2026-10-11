package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// Recursive pointee descriptors have finite pointer width, and are not names.
func TestCountedPrintRecursiveTypeAndBindings(t *testing.T) {
	node := &mir2.StructTy{Name: "Node"}
	p := mir2.PointerTo(node)
	node.Fields = []mir2.StructField{{Name: "next", Ty: p}}
	f := &Func{Name: "show", Params: []Param{{Name: "root", Ty: p}}, RetTy: mir2.TyVoid, Body: &Block{Body: []Stmt{
		&ExprStmt{Expr: &CallExpr{Fn: "@mir.io.print.sstr", Args: []Expr{&AddrOfExpr{Sym: "@mir2.str.0"}}, Ty: mir2.TyVoid}},
		&TupleLetStmt{Names: []string{"__print_ptr_1"}},
		&ForRangeStmt{Var: "__print_count_3"},
	}}}
	l := &lowerer{hirFuncs: map[string]*Func{"show": f}, m: &mir2.Module{}, env: map[string]mir2.Reg{}, fnAliases: map[string]string{}}
	if n := l.freshStringPrintName("__print_ptr_"); n != "__print_ptr_2" {
		t.Fatal("future tuple binding captured", n)
	}
	if n := l.freshStringPrintName("__print_count_"); n != "__print_count_4" {
		t.Fatal("loop binder captured", n)
	}
	// Execute real lowering with the supported recursive parameter and print.
	f.Body = &Block{Body: []Stmt{&ExprStmt{Expr: &CallExpr{Fn: "@mir.io.print.sstr", Args: []Expr{&AddrOfExpr{Sym: "@mir2.str.0"}}, Ty: mir2.TyVoid}}}}
	m := LowerModule(&Module{Name: "recursive", Funcs: []*Func{f}, Strings: []string{"AB"}, StrKinds: []mir2.StringKind{mir2.StrSString}})
	if e := mir2.Verify(m); e != nil {
		t.Fatal(e)
	}
}
