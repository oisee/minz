package hir

import "github.com/minz/minzc/pkg/mir2"

// lowerCountedStringPrint uses ordinary loads, loop SSA and print.char calls.
// The representation was selected only for a direct literal by the frontend.
func (l *lowerer) lowerCountedStringPrint(ex *CallExpr) mir2.Reg {
	if len(ex.Args) != 1 {
		panic("hir/lower: counted string print requires one argument")
	}
	ptr := l.fresh("__print_ptr_")
	count := l.fresh("__print_count_")
	ref := func(n string, ty mir2.Ty) Expr { return &VarRefExpr{Name: n, Ty: ty} }
	add := func(x Expr, n int64) Expr {
		return &BinExpr{Op: "+", L: x, R: &IntLitExpr{Val: n, Ty: mir2.TyU16}, Ty: mir2.TyPtr}
	}
	width := int64(1)
	ty := mir2.TyU8
	if ex.Fn == "@mir.io.print.lstr" {
		width = 2
		ty = mir2.TyU16
	}
	block := &Block{Body: []Stmt{
		&VarDeclStmt{Name: ptr, Ty: mir2.TyPtr, Init: ex.Args[0]},
		&VarDeclStmt{Name: count, Ty: ty, Init: &LoadExpr{Ptr: ref(ptr, mir2.TyPtr), Ty: ty}},
		&AssignStmt{Target: ref(ptr, mir2.TyPtr), Val: add(ref(ptr, mir2.TyPtr), width)},
		&WhileStmt{Cond: &BinExpr{Op: "!=", L: ref(count, ty), R: &IntLitExpr{Val: 0, Ty: ty}, Ty: mir2.TyBool}, Body: &Block{Body: []Stmt{
			&ExprStmt{Expr: &CallExpr{Fn: "@mir.io.print.char", Args: []Expr{&LoadExpr{Ptr: ref(ptr, mir2.TyPtr), Ty: mir2.TyU8}}, Ty: mir2.TyVoid}},
			&AssignStmt{Target: ref(ptr, mir2.TyPtr), Val: add(ref(ptr, mir2.TyPtr), 1)},
			&AssignStmt{Target: ref(count, ty), Val: &BinExpr{Op: "-", L: ref(count, ty), R: &IntLitExpr{Val: 1, Ty: ty}, Ty: ty}},
		}}},
	}}
	l.lowerBlock(block)
	return mir2.NoReg
}
