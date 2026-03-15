package c89

import (
	"fmt"

	cc "modernc.org/cc/v4"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

type lowerer struct {
	ast      *cc.AST
	hm       *hir.Module
	globals  map[string]mir2.Ty
	typedefs map[string]mir2.Ty
	structs  map[string]*mir2.StructTy
}

func (l *lowerer) lower() error {
	for tu := l.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		switch ed.Case {
		case cc.ExternalDeclarationFuncDef:
			f, err := l.lowerFunc(ed.FunctionDefinition)
			if err != nil {
				return err
			}
			if f != nil {
				l.hm.Funcs = append(l.hm.Funcs, f)
			}
		case cc.ExternalDeclarationDecl:
			if err := l.lowerTopDecl(ed.Declaration); err != nil {
				return err
			}
		}
	}
	return nil
}

// ── Type mapping ────────────────────────────────────────────────────────────

func (l *lowerer) mapType(t cc.Type) mir2.Ty {
	if t == nil {
		return mir2.TyVoid
	}
	switch t.Kind() {
	case cc.Void:
		return mir2.TyVoid
	case cc.Bool, cc.Char, cc.SChar, cc.UChar:
		if t.Kind() == cc.SChar {
			return mir2.TyI8
		}
		return mir2.TyU8
	case cc.Short, cc.UShort, cc.Int, cc.UInt, cc.Enum:
		if t.Kind() == cc.Short || t.Kind() == cc.Int {
			return mir2.TyI16
		}
		return mir2.TyU16
	case cc.Long, cc.ULong, cc.LongLong, cc.ULongLong:
		// 32-bit not natively supported; use i16 for now
		if t.Kind() == cc.Long || t.Kind() == cc.LongLong {
			return mir2.TyI16
		}
		return mir2.TyU16
	case cc.Ptr, cc.Function:
		return mir2.TyPtr
	case cc.Array:
		return mir2.TyPtr // arrays decay to pointers
	case cc.Struct:
		return mir2.TyPtr // structs lowered to pointer (address) on Z80
	default:
		return mir2.TyU16 // conservative fallback
	}
}

func (l *lowerer) structTag(st *cc.StructType) string {
	t := st.Tag()
	return t.SrcStr()
}

// ── Top-level declarations ──────────────────────────────────────────────────

func (l *lowerer) lowerTopDecl(d *cc.Declaration) error {
	if d == nil {
		return nil
	}
	for idl := d.InitDeclaratorList; idl != nil; idl = idl.InitDeclaratorList {
		id := idl.InitDeclarator
		if id == nil || id.Declarator == nil {
			continue
		}
		decl := id.Declarator
		name := decl.Name()
		if name == "" || name == "__predefined_declarator" {
			continue
		}
		ty := decl.Type()
		if ty == nil {
			continue
		}

		// Skip typedef declarations.
		if decl.IsTypename() {
			continue
		}

		// Skip function declarations (prototypes).
		if ty.Kind() == cc.Function {
			continue
		}

		// Struct type declaration.
		if ty.Kind() == cc.Struct {
			st := ty.(*cc.StructType)
			tag := l.structTag(st)
			if tag != "" && l.structs[tag] == nil {
				l.lowerStructDecl(tag, st)
			}
		}

		// Global variable.
		mty := l.mapType(ty)
		g := mir2.Global{Name: name, Ty: mty}

		// Initial value.
		if id.Initializer != nil {
			if val, ok := l.evalConstInit(id.Initializer); ok {
				g.Init = []byte{byte(val)}
			}
		}

		l.hm.Globals = append(l.hm.Globals, g)
		l.globals[name] = mty
	}
	return nil
}

func (l *lowerer) lowerStructDecl(tag string, st *cc.StructType) {
	mst := &mir2.StructTy{Name: tag}
	nf := st.NumFields()
	for i := 0; i < nf; i++ {
		f := st.FieldByIndex(i)
		if f == nil {
			continue
		}
		fname := f.Name()
		if fname == "" {
			fname = fmt.Sprintf("_f%d", i)
		}
		mst.Fields = append(mst.Fields, mir2.StructField{
			Name: fname,
			Ty:   l.mapType(f.Type()),
		})
	}
	l.structs[tag] = mst
	l.hm.Structs = append(l.hm.Structs, mst)
}

func (l *lowerer) evalConstInit(init *cc.Initializer) (int64, bool) {
	if init == nil || init.AssignmentExpression == nil {
		return 0, false
	}
	return l.evalConst(init.AssignmentExpression)
}

// ── Function lowering ───────────────────────────────────────────────────────

func (l *lowerer) lowerFunc(fd *cc.FunctionDefinition) (*hir.Func, error) {
	decl := fd.Declarator
	name := decl.Name()
	if name == "" {
		return nil, nil
	}

	ft, ok := decl.Type().(*cc.FunctionType)
	if !ok {
		return nil, fmt.Errorf("function %s: expected function type", name)
	}

	retTy := l.mapType(ft.Result())

	var params []hir.Param
	for _, p := range ft.Parameters() {
		pname := p.Name()
		if pname == "" {
			pname = fmt.Sprintf("_p%d", len(params))
		}
		params = append(params, hir.Param{
			Name: pname,
			Ty:   l.mapType(p.Type()),
		})
	}

	fl := &funcLow{
		low:    l,
		name:   name,
		retTy:  retTy,
		locals: make(map[string]mir2.Ty),
	}

	// Register params as locals.
	for _, p := range params {
		fl.locals[p.Name] = p.Ty
	}

	body, err := fl.lowerCompound(fd.CompoundStatement)
	if err != nil {
		return nil, fmt.Errorf("function %s: %w", name, err)
	}

	// Ensure function ends with a return.
	if len(body) == 0 || !isReturn(body[len(body)-1]) {
		if retTy == mir2.TyVoid {
			body = append(body, hir.RetVoid())
		}
	}

	return &hir.Func{
		Name:   name,
		Params: params,
		RetTy:  retTy,
		Body:   hir.Blk(body...),
	}, nil
}

func isReturn(s hir.Stmt) bool {
	_, ok := s.(*hir.ReturnStmt)
	return ok
}

// ── Function body lowering ──────────────────────────────────────────────────

type funcLow struct {
	low    *lowerer
	name   string
	retTy  mir2.Ty
	locals map[string]mir2.Ty
}

func (fl *funcLow) lowerCompound(cs *cc.CompoundStatement) ([]hir.Stmt, error) {
	if cs == nil {
		return nil, nil
	}
	var stmts []hir.Stmt
	for bl := cs.BlockItemList; bl != nil; bl = bl.BlockItemList {
		bi := bl.BlockItem
		switch bi.Case {
		case cc.BlockItemDecl:
			s, err := fl.lowerLocalDecl(bi.Declaration)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, s...)
		case cc.BlockItemStmt:
			s, err := fl.lowerStmt(bi.Statement)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, s...)
		}
	}
	return stmts, nil
}

func (fl *funcLow) lowerLocalDecl(d *cc.Declaration) ([]hir.Stmt, error) {
	if d == nil {
		return nil, nil
	}
	var stmts []hir.Stmt
	for idl := d.InitDeclaratorList; idl != nil; idl = idl.InitDeclaratorList {
		id := idl.InitDeclarator
		if id == nil || id.Declarator == nil {
			continue
		}
		decl := id.Declarator
		name := decl.Name()
		if name == "" {
			continue
		}
		ty := fl.low.mapType(decl.Type())
		fl.locals[name] = ty

		var init hir.Expr
		if id.Initializer != nil && id.Initializer.AssignmentExpression != nil {
			r, err := fl.lowerExpr(id.Initializer.AssignmentExpression)
			if err != nil {
				return nil, err
			}
			init = r.toExpr()
		}

		stmts = append(stmts, &hir.VarDeclStmt{Name: name, Ty: ty, Init: init})
	}
	return stmts, nil
}

// ── Statement lowering ──────────────────────────────────────────────────────

func (fl *funcLow) lowerStmt(s *cc.Statement) ([]hir.Stmt, error) {
	if s == nil {
		return nil, nil
	}
	switch s.Case {
	case cc.StatementCompound:
		return fl.lowerCompound(s.CompoundStatement)

	case cc.StatementExpr:
		return fl.lowerExprStmt(s.ExpressionStatement)

	case cc.StatementSelection:
		return fl.lowerSelection(s.SelectionStatement)

	case cc.StatementIteration:
		return fl.lowerIteration(s.IterationStatement)

	case cc.StatementJump:
		return fl.lowerJump(s.JumpStatement)

	case cc.StatementLabeled:
		if s.LabeledStatement != nil && s.LabeledStatement.Statement != nil {
			return fl.lowerStmt(s.LabeledStatement.Statement)
		}
		return nil, nil

	default:
		return nil, nil
	}
}

func (fl *funcLow) lowerExprStmt(es *cc.ExpressionStatement) ([]hir.Stmt, error) {
	if es == nil || es.ExpressionList == nil {
		return nil, nil
	}
	r, err := fl.lowerExpr(es.ExpressionList)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	s := r.toStmt()
	if s == nil {
		return nil, nil
	}
	return []hir.Stmt{s}, nil
}

func (fl *funcLow) lowerSelection(ss *cc.SelectionStatement) ([]hir.Stmt, error) {
	if ss == nil {
		return nil, nil
	}
	switch ss.Case {
	case cc.SelectionStatementIf:
		cond, err := fl.lowerExprAsExpr(ss.ExpressionList)
		if err != nil {
			return nil, err
		}
		then, err := fl.lowerStmt(ss.Statement)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{hir.If(cond, hir.Blk(then...), nil)}, nil

	case cc.SelectionStatementIfElse:
		cond, err := fl.lowerExprAsExpr(ss.ExpressionList)
		if err != nil {
			return nil, err
		}
		then, err := fl.lowerStmt(ss.Statement)
		if err != nil {
			return nil, err
		}
		els, err := fl.lowerStmt(ss.Statement2)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{hir.If(cond, hir.Blk(then...), hir.Blk(els...))}, nil

	default:
		return nil, nil
	}
}

func (fl *funcLow) lowerIteration(is *cc.IterationStatement) ([]hir.Stmt, error) {
	if is == nil {
		return nil, nil
	}
	switch is.Case {
	case cc.IterationStatementWhile:
		cond, err := fl.lowerExprAsExpr(is.ExpressionList)
		if err != nil {
			return nil, err
		}
		body, err := fl.lowerStmt(is.Statement)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{hir.While(cond, hir.Blk(body...))}, nil

	case cc.IterationStatementDo:
		body, err := fl.lowerStmt(is.Statement)
		if err != nil {
			return nil, err
		}
		cond, err := fl.lowerExprAsExpr(is.ExpressionList)
		if err != nil {
			return nil, err
		}
		var stmts []hir.Stmt
		stmts = append(stmts, body...)
		stmts = append(stmts, hir.While(cond, hir.Blk(body...)))
		return stmts, nil

	case cc.IterationStatementFor:
		return fl.lowerForLoop(is, false)

	case cc.IterationStatementForDecl:
		return fl.lowerForLoop(is, true)

	default:
		return nil, nil
	}
}

func (fl *funcLow) lowerForLoop(is *cc.IterationStatement, isForDecl bool) ([]hir.Stmt, error) {
	var stmts []hir.Stmt

	// Init.
	if isForDecl {
		if is.Declaration != nil {
			s, err := fl.lowerLocalDecl(is.Declaration)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, s...)
		}
	} else if is.ExpressionList != nil {
		r, err := fl.lowerExpr(is.ExpressionList)
		if err != nil {
			return nil, err
		}
		if r != nil {
			if s := r.toStmt(); s != nil {
				stmts = append(stmts, s)
			}
		}
	}

	// Condition and post differ based on for variant.
	var condNode, postNode cc.ExpressionNode
	if isForDecl {
		condNode = is.ExpressionList
		postNode = is.ExpressionList2
	} else {
		condNode = is.ExpressionList2
		postNode = is.ExpressionList3
	}

	// Condition.
	var cond hir.Expr
	if condNode != nil {
		var err error
		cond, err = fl.lowerExprAsExpr(condNode)
		if err != nil {
			return nil, err
		}
	} else {
		cond = hir.Bool(true)
	}

	// Body.
	body, err := fl.lowerStmt(is.Statement)
	if err != nil {
		return nil, err
	}

	// Post.
	if postNode != nil {
		r, err := fl.lowerExpr(postNode)
		if err != nil {
			return nil, err
		}
		if r != nil {
			if s := r.toStmt(); s != nil {
				body = append(body, s)
			}
		}
	}

	stmts = append(stmts, hir.While(cond, hir.Blk(body...)))
	return stmts, nil
}

func (fl *funcLow) lowerJump(js *cc.JumpStatement) ([]hir.Stmt, error) {
	if js == nil {
		return nil, nil
	}
	switch js.Case {
	case cc.JumpStatementReturn:
		if js.ExpressionList != nil {
			val, err := fl.lowerExprAsExpr(js.ExpressionList)
			if err != nil {
				return nil, err
			}
			return []hir.Stmt{hir.Ret(val)}, nil
		}
		return []hir.Stmt{hir.RetVoid()}, nil

	case cc.JumpStatementBreak:
		return []hir.Stmt{hir.Break()}, nil

	case cc.JumpStatementContinue:
		return []hir.Stmt{hir.Continue()}, nil

	default:
		return nil, nil // goto — deferred
	}
}

// ── Expression lowering ─────────────────────────────────────────────────────

// exprResult wraps either a plain hir.Expr or an assignment (target + val).
type exprResult struct {
	expr   hir.Expr // non-nil for normal expressions
	target hir.Expr // non-nil for assignments
	val    hir.Expr // non-nil for assignments
}

func (r *exprResult) isAssign() bool { return r != nil && r.target != nil }

func (r *exprResult) toExpr() hir.Expr {
	if r == nil {
		return nil
	}
	if r.isAssign() {
		return r.val
	}
	return r.expr
}

func (r *exprResult) toStmt() hir.Stmt {
	if r == nil {
		return nil
	}
	if r.isAssign() {
		// *ptr = val → StoreStmt (not Assign to LoadExpr)
		if load, ok := r.target.(*hir.LoadExpr); ok {
			return &hir.StoreStmt{Ptr: load.Ptr, Val: r.val}
		}
		return hir.Assign(r.target, r.val)
	}
	if r.expr != nil {
		return &hir.ExprStmt{Expr: r.expr}
	}
	return nil
}

func wrapExpr(e hir.Expr) *exprResult       { return &exprResult{expr: e} }
func wrapAssign(t, v hir.Expr) *exprResult   { return &exprResult{target: t, val: v} }

// lowerExprAsExpr is a convenience that returns hir.Expr (discards assignment info).
func (fl *funcLow) lowerExprAsExpr(e cc.ExpressionNode) (hir.Expr, error) {
	r, err := fl.lowerExpr(e)
	if err != nil {
		return nil, err
	}
	return r.toExpr(), nil
}

func (fl *funcLow) lowerExpr(e cc.ExpressionNode) (*exprResult, error) {
	if e == nil {
		return nil, nil
	}

	switch x := e.(type) {
	case *cc.PrimaryExpression:
		return fl.lowerPrimary(x)

	case *cc.PostfixExpression:
		return fl.lowerPostfix(x)

	case *cc.UnaryExpression:
		return fl.lowerUnary(x)

	case *cc.CastExpression:
		switch x.Case {
		case cc.CastExpressionUnary:
			return fl.lowerExpr(x.UnaryExpression)
		case cc.CastExpressionCast:
			inner, err := fl.lowerExprAsExpr(x.CastExpression)
			if err != nil {
				return nil, err
			}
			return wrapExpr(hir.Cast(inner, fl.low.mapType(x.Type()))), nil
		}
		return nil, nil

	case *cc.MultiplicativeExpression:
		return fl.lowerBinaryOp(x.MultiplicativeExpression, x.CastExpression, x.Case, x.Type())

	case *cc.AdditiveExpression:
		return fl.lowerBinaryOp(x.AdditiveExpression, x.MultiplicativeExpression, x.Case, x.Type())

	case *cc.ShiftExpression:
		return fl.lowerBinaryOp(x.ShiftExpression, x.AdditiveExpression, x.Case, x.Type())

	case *cc.RelationalExpression:
		return fl.lowerBinaryOp(x.RelationalExpression, x.ShiftExpression, x.Case, x.Type())

	case *cc.EqualityExpression:
		return fl.lowerBinaryOp(x.EqualityExpression, x.RelationalExpression, x.Case, x.Type())

	case *cc.AndExpression:
		return fl.lowerBinaryOp(x.AndExpression, x.EqualityExpression, x.Case, x.Type())

	case *cc.ExclusiveOrExpression:
		return fl.lowerBinaryOp(x.ExclusiveOrExpression, x.AndExpression, x.Case, x.Type())

	case *cc.InclusiveOrExpression:
		return fl.lowerBinaryOp(x.InclusiveOrExpression, x.ExclusiveOrExpression, x.Case, x.Type())

	case *cc.LogicalAndExpression:
		if x.LogicalAndExpression != nil && x.InclusiveOrExpression != nil {
			// a && b → short-circuit: if a then (b != 0) else 0
			l, err := fl.lowerExprAsExpr(x.LogicalAndExpression)
			if err != nil {
				return nil, err
			}
			r, err := fl.lowerExprAsExpr(x.InclusiveOrExpression)
			if err != nil {
				return nil, err
			}
			return wrapExpr(&hir.CondExpr{
				Cond: l, Then: r,
				Else: &hir.IntLitExpr{Val: 0, Ty: mir2.TyBool},
				Ty:   mir2.TyBool,
			}), nil
		}
		return fl.lowerExpr(x.InclusiveOrExpression)

	case *cc.LogicalOrExpression:
		if x.LogicalOrExpression != nil && x.LogicalAndExpression != nil {
			// a || b → short-circuit: if a then 1 else (b != 0)
			l, err := fl.lowerExprAsExpr(x.LogicalOrExpression)
			if err != nil {
				return nil, err
			}
			r, err := fl.lowerExprAsExpr(x.LogicalAndExpression)
			if err != nil {
				return nil, err
			}
			return wrapExpr(&hir.CondExpr{
				Cond: l,
				Then: &hir.IntLitExpr{Val: 1, Ty: mir2.TyBool},
				Else: r, Ty: mir2.TyBool,
			}), nil
		}
		return fl.lowerExpr(x.LogicalAndExpression)

	case *cc.ConditionalExpression:
		if x.LogicalOrExpression != nil && x.ExpressionList == nil {
			return fl.lowerExpr(x.LogicalOrExpression)
		}
		cond, err := fl.lowerExprAsExpr(x.LogicalOrExpression)
		if err != nil {
			return nil, err
		}
		then, err := fl.lowerExprAsExpr(x.ExpressionList)
		if err != nil {
			return nil, err
		}
		els, err := fl.lowerExprAsExpr(x.ConditionalExpression)
		if err != nil {
			return nil, err
		}
		return wrapExpr(&hir.CondExpr{Cond: cond, Then: then, Else: els, Ty: fl.low.mapType(x.Type())}), nil

	case *cc.AssignmentExpression:
		return fl.lowerAssignment(x)

	case *cc.ExpressionList:
		// Comma expression — evaluate all, return last.
		var last *exprResult
		for el := x; el != nil; el = el.ExpressionList {
			var err error
			last, err = fl.lowerExpr(el.AssignmentExpression)
			if err != nil {
				return nil, err
			}
		}
		return last, nil

	default:
		return nil, fmt.Errorf("unsupported expression type %T", e)
	}
}

func (fl *funcLow) lowerPrimary(pe *cc.PrimaryExpression) (*exprResult, error) {
	switch pe.Case {
	case cc.PrimaryExpressionIdent:
		tok := pe.Token
		name := tok.SrcStr()
		ty := fl.typeOf(name)
		return wrapExpr(hir.Var(name, ty)), nil

	case cc.PrimaryExpressionInt:
		v := pe.Value()
		if v == nil {
			return wrapExpr(hir.U8(0)), nil
		}
		ty := fl.low.mapType(pe.Type())
		return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: ty}), nil

	case cc.PrimaryExpressionChar:
		v := pe.Value()
		if v == nil {
			return wrapExpr(hir.U8(0)), nil
		}
		return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: mir2.TyU8}), nil

	case cc.PrimaryExpressionString:
		tok := pe.Token
		s := tok.SrcStr()
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
		}
		idx := len(fl.low.hm.Strings)
		fl.low.hm.Strings = append(fl.low.hm.Strings, s)
		return wrapExpr(&hir.IntLitExpr{Val: int64(idx), Ty: mir2.TyPtr}), nil

	case cc.PrimaryExpressionExpr:
		return fl.lowerExpr(pe.ExpressionList)

	default:
		return wrapExpr(hir.U8(0)), nil
	}
}

func (fl *funcLow) lowerPostfix(pf *cc.PostfixExpression) (*exprResult, error) {
	switch pf.Case {
	case cc.PostfixExpressionPrimary:
		return fl.lowerExpr(pf.PrimaryExpression)

	case cc.PostfixExpressionCall:
		return fl.lowerCall(pf)

	case cc.PostfixExpressionIndex:
		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		idx, err := fl.lowerExprAsExpr(pf.ExpressionList)
		if err != nil {
			return nil, err
		}
		elemTy := fl.low.mapType(pf.Type())
		return wrapExpr(hir.Index(base, idx, elemTy)), nil

	case cc.PostfixExpressionSelect: // struct.field
		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		tok := pf.Token2
		field := tok.SrcStr()
		ty := fl.low.mapType(pf.Type())
		return wrapExpr(&hir.FieldExpr{X: base, Field: field, Ty: ty}), nil

	case cc.PostfixExpressionPSelect: // struct->field
		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		tok := pf.Token2
		field := tok.SrcStr()
		ty := fl.low.mapType(pf.Type())
		return wrapExpr(&hir.FieldExpr{X: base, Field: field, Ty: ty}), nil

	case cc.PostfixExpressionInc: // x++
		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		ty := base.ExprTy()
		return wrapAssign(base, hir.Add(base, &hir.IntLitExpr{Val: 1, Ty: ty}, ty)), nil

	case cc.PostfixExpressionDec: // x--
		base, err := fl.lowerExprAsExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		ty := base.ExprTy()
		return wrapAssign(base, hir.Sub(base, &hir.IntLitExpr{Val: 1, Ty: ty}, ty)), nil

	default:
		return fl.lowerExpr(pf.PrimaryExpression)
	}
}

func (fl *funcLow) lowerCall(pf *cc.PostfixExpression) (*exprResult, error) {
	fnExpr, err := fl.lowerExprAsExpr(pf.PostfixExpression)
	if err != nil {
		return nil, err
	}

	fnName := ""
	if v, ok := fnExpr.(*hir.VarRefExpr); ok {
		fnName = v.Name
	}

	var args []hir.Expr
	for al := pf.ArgumentExpressionList; al != nil; al = al.ArgumentExpressionList {
		arg, err := fl.lowerExprAsExpr(al.AssignmentExpression)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}

	retTy := fl.low.mapType(pf.Type())
	return wrapExpr(hir.Call(fnName, retTy, args...)), nil
}

func (fl *funcLow) lowerUnary(ue *cc.UnaryExpression) (*exprResult, error) {
	switch ue.Case {
	case cc.UnaryExpressionPostfix:
		return fl.lowerExpr(ue.PostfixExpression)

	case cc.UnaryExpressionSizeofExpr:
		if v := ue.Value(); v != nil {
			return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: mir2.TyU16}), nil
		}
		return wrapExpr(&hir.IntLitExpr{Val: 0, Ty: mir2.TyU16}), nil

	case cc.UnaryExpressionSizeofType:
		if v := ue.Value(); v != nil {
			return wrapExpr(&hir.IntLitExpr{Val: constToInt64(v), Ty: mir2.TyU16}), nil
		}
		return wrapExpr(&hir.IntLitExpr{Val: 0, Ty: mir2.TyU16}), nil

	case cc.UnaryExpressionAddrof: // &x
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		if v, ok := inner.(*hir.VarRefExpr); ok {
			return wrapExpr(hir.Addr(v.Name)), nil
		}
		return wrapExpr(inner), nil

	case cc.UnaryExpressionDeref: // *x
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		ty := fl.low.mapType(ue.Type())
		return wrapExpr(hir.Load(inner, ty)), nil

	case cc.UnaryExpressionMinus:
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		return wrapExpr(&hir.UnaryExpr{Op: "-", X: inner, Ty: inner.ExprTy()}), nil

	case cc.UnaryExpressionPlus:
		return fl.lowerExpr(ue.CastExpression)

	case cc.UnaryExpressionCpl:
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		return wrapExpr(&hir.UnaryExpr{Op: "~", X: inner, Ty: inner.ExprTy()}), nil

	case cc.UnaryExpressionNot:
		inner, err := fl.lowerExprAsExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		return wrapExpr(&hir.UnaryExpr{Op: "!", X: inner, Ty: mir2.TyBool}), nil

	case cc.UnaryExpressionInc: // ++x
		inner, err := fl.lowerExprAsExpr(ue.UnaryExpression)
		if err != nil {
			return nil, err
		}
		ty := inner.ExprTy()
		return wrapAssign(inner, hir.Add(inner, &hir.IntLitExpr{Val: 1, Ty: ty}, ty)), nil

	case cc.UnaryExpressionDec: // --x
		inner, err := fl.lowerExprAsExpr(ue.UnaryExpression)
		if err != nil {
			return nil, err
		}
		ty := inner.ExprTy()
		return wrapAssign(inner, hir.Sub(inner, &hir.IntLitExpr{Val: 1, Ty: ty}, ty)), nil

	default:
		return fl.lowerExpr(ue.CastExpression)
	}
}

func (fl *funcLow) lowerAssignment(ae *cc.AssignmentExpression) (*exprResult, error) {
	if ae.ConditionalExpression != nil && ae.AssignmentExpression == nil {
		return fl.lowerExpr(ae.ConditionalExpression)
	}

	target, err := fl.lowerExprAsExpr(ae.UnaryExpression)
	if err != nil {
		return nil, err
	}
	val, err := fl.lowerExprAsExpr(ae.AssignmentExpression)
	if err != nil {
		return nil, err
	}

	ty := fl.low.mapType(ae.Type())
	switch ae.Case {
	case cc.AssignmentExpressionAdd:
		val = hir.Add(target, val, ty)
	case cc.AssignmentExpressionSub:
		val = hir.Sub(target, val, ty)
	case cc.AssignmentExpressionMul:
		val = hir.Mul(target, val, ty)
	case cc.AssignmentExpressionAnd:
		val = &hir.BinExpr{Op: "&", L: target, R: val, Ty: ty}
	case cc.AssignmentExpressionOr:
		val = &hir.BinExpr{Op: "|", L: target, R: val, Ty: ty}
	case cc.AssignmentExpressionXor:
		val = &hir.BinExpr{Op: "^", L: target, R: val, Ty: ty}
	case cc.AssignmentExpressionLsh:
		val = &hir.BinExpr{Op: "<<", L: target, R: val, Ty: ty}
	case cc.AssignmentExpressionRsh:
		val = &hir.BinExpr{Op: ">>", L: target, R: val, Ty: ty}
	}

	return wrapAssign(target, val), nil
}

// lowerBinaryOp handles all binary expression types via the cc.Case value.
func (fl *funcLow) lowerBinaryOp(left, right cc.ExpressionNode, caseVal interface{}, ty cc.Type) (*exprResult, error) {
	if left == nil {
		return fl.lowerExpr(right)
	}

	l, err := fl.lowerExprAsExpr(left)
	if err != nil {
		return nil, err
	}
	r, err := fl.lowerExprAsExpr(right)
	if err != nil {
		return nil, err
	}

	mty := fl.low.mapType(ty)

	// Selectively undo C integer promotion for 8-bit operations.
	// Only narrow for ops where u8 overflow doesn't lose information:
	// subtraction, division, bitwise ops. Do NOT narrow addition or
	// multiplication — those can legitimately overflow u8 (e.g. 200+200).
	lty, rty := l.ExprTy(), r.ExprTy()
	narrow8 := (lty == mir2.TyU8 || lty == mir2.TyI8) && lty == rty

	op := ""
	switch v := caseVal.(type) {
	case cc.MultiplicativeExpressionCase:
		switch v {
		case cc.MultiplicativeExpressionMul:
			op = "*"
		case cc.MultiplicativeExpressionDiv:
			op = "/"
		case cc.MultiplicativeExpressionMod:
			op = "%"
		}
	case cc.AdditiveExpressionCase:
		switch v {
		case cc.AdditiveExpressionAdd:
			op = "+"
		case cc.AdditiveExpressionSub:
			op = "-"
		}
	case cc.ShiftExpressionCase:
		switch v {
		case cc.ShiftExpressionLsh:
			op = "<<"
		case cc.ShiftExpressionRsh:
			op = ">>"
		}
	case cc.RelationalExpressionCase:
		mty = mir2.TyBool
		switch v {
		case cc.RelationalExpressionLt:
			op = "<"
		case cc.RelationalExpressionGt:
			op = ">"
		case cc.RelationalExpressionLeq:
			op = "<="
		case cc.RelationalExpressionGeq:
			op = ">="
		}
	case cc.EqualityExpressionCase:
		mty = mir2.TyBool
		switch v {
		case cc.EqualityExpressionEq:
			op = "=="
		case cc.EqualityExpressionNeq:
			op = "!="
		}
	case cc.AndExpressionCase:
		op = "&"
	case cc.ExclusiveOrExpressionCase:
		op = "^"
	case cc.InclusiveOrExpressionCase:
		op = "|"
	}

	if op == "" {
		return wrapExpr(l), nil
	}

	// Apply u8 narrowing only for safe operators.
	if narrow8 {
		switch op {
		case "-", "/", "%", "&", "|", "^": // safe: result fits in u8
			mty = lty
		// "+", "*", "<<" — do NOT narrow, overflow loses information
		}
	}

	// Rewrite modulo: a % b → a - (a / b) * b
	// HIR lowerer doesn't support "%" directly; this decomposition works
	// for both signed and unsigned types on Z80.
	if op == "%" {
		div := &hir.BinExpr{Op: "/", L: l, R: r, Ty: mty}
		mul := &hir.BinExpr{Op: "*", L: div, R: r, Ty: mty}
		return wrapExpr(&hir.BinExpr{Op: "-", L: l, R: mul, Ty: mty}), nil
	}

	return wrapExpr(&hir.BinExpr{Op: op, L: l, R: r, Ty: mty}), nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (fl *funcLow) typeOf(name string) mir2.Ty {
	if ty, ok := fl.locals[name]; ok {
		return ty
	}
	if ty, ok := fl.low.globals[name]; ok {
		return ty
	}
	return mir2.TyU16
}

func (l *lowerer) evalConst(e cc.ExpressionNode) (int64, bool) {
	if e == nil {
		return 0, false
	}
	v := e.Value()
	if v == nil {
		return 0, false
	}
	return constToInt64(v), true
}

func constToInt64(v cc.Value) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case cc.Int64Value:
		return int64(n)
	case cc.UInt64Value:
		return int64(n)
	default:
		s := fmt.Sprintf("%v", n)
		var i int64
		fmt.Sscanf(s, "%d", &i)
		return i
	}
}
