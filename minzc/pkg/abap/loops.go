package abap

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func indexGet() hir.Expr               { return &hir.CallExpr{Fn: "sy_get_index", Ty: mir2.TyI32} }
func indexSet(value hir.Expr) hir.Stmt { return charCall("sy_set_index", value) }
func loopValue(name string) hir.Expr   { return &hir.VarRefExpr{Name: name, Ty: mir2.TyI32} }

// DO's count is evaluated once. A counter starting at zero avoids forming
// N+1, including for INT_MAX. Increment before the body also makes CONTINUE
// correct without a synthetic increment at the end of the statement block.
func (l *lowerer) lowerIndexedLoop(body []Stmt_, times, condition hir.Expr) (hir.Stmt, error) {
	l.loopCounter++
	saved := fmt.Sprintf("_abap_loop_saved_%d", l.loopCounter)
	counter := fmt.Sprintf("_abap_loop_counter_%d", l.loopCounter)
	prefix := []hir.Stmt{
		&hir.VarDeclStmt{Name: saved, Ty: mir2.TyI32, Init: indexGet()},
		&hir.VarDeclStmt{Name: counter, Ty: mir2.TyI32, Init: &hir.IntLitExpr{Val: 0, Ty: mir2.TyI32}},
	}
	if times != nil {
		if times.ExprTy() != mir2.TyI32 {
			return nil, fmt.Errorf("DO TIMES: numeric conversion to TYPE i is not implemented for %s", times.ExprTy())
		}
		limit := fmt.Sprintf("_abap_loop_limit_%d", l.loopCounter)
		prefix = append(prefix, &hir.VarDeclStmt{Name: limit, Ty: mir2.TyI32, Init: times})
		condition = &hir.BinExpr{Op: "<", L: loopValue(counter), R: loopValue(limit), Ty: mir2.TyBool}
	}
	if condition == nil {
		condition = &hir.BoolLitExpr{Val: true}
	}
	l.indexLoops = append(l.indexLoops, saved)
	l.loopDepth++
	block, err := l.lowerStmts(body)
	l.loopDepth--
	l.indexLoops = l.indexLoops[:len(l.indexLoops)-1]
	if err != nil {
		return nil, err
	}
	block.Body = append([]hir.Stmt{
		&hir.AssignStmt{Target: loopValue(counter), Val: &hir.CallExpr{Fn: "_abap_add", Args: []hir.Expr{loopValue(counter), &hir.IntLitExpr{Val: 1, Ty: mir2.TyI32}}, Ty: mir2.TyI32}},
		indexSet(loopValue(counter)),
	}, block.Body...)
	prefix = append(prefix, &hir.WhileStmt{Cond: condition, Body: block}, indexSet(loopValue(saved)))
	return &hir.Block{Body: prefix}, nil
}

// RETURN bypasses loop epilogues. Restore every indexed loop in the current
// processing block, leaving any dynamically enclosing caller loop untouched.
func (l *lowerer) lowerLoopReturn() hir.Stmt {
	statements := []hir.Stmt{}
	for i := len(l.indexLoops) - 1; i >= 0; i-- {
		statements = append(statements, indexSet(loopValue(l.indexLoops[i])))
	}
	statements = append(statements, &hir.ReturnStmt{})
	if len(statements) == 1 {
		return statements[0]
	}
	return &hir.Block{Body: statements}
}
