// Package objc implements an Objective-C frontend for MinZ.
//
// On Z80 there is no ObjC runtime — all dispatch is static:
//
//	@interface Foo { int x; }   → struct Foo { x: i16 }
//	-(int)getX                  → fun Foo_getX(self: *Foo) -> i16
//	[foo getX]                  → Foo_getX(foo)
//	[foo add:5 andY:3]          → Foo_add_andY(foo, 5, 3)
//
// Architecture:
//
//	.m source → cparse.Translate() → cc.AST → lower.go → *hir.Module
package objc

import (
	"fmt"
	"strings"

	cc "github.com/minz/minzc/pkg/cparse"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Compile parses ObjC source and produces an HIR module.
func Compile(src, name string) (*hir.Module, error) {
	cfg := &cc.Config{ABI: z80ABI()}
	sources := []cc.Source{
		{Name: "<predefined>", Value: z80Predefined},
		{Name: name, Value: src},
	}

	ast, err := cc.Translate(cfg, sources)
	if err != nil {
		return nil, fmt.Errorf("objc parse: %w", err)
	}

	l := &lowerer{
		ast:     ast,
		hm:      &hir.Module{Name: strings.TrimSuffix(name, ".m")},
		globals: make(map[string]mir2.Ty),
		structs: make(map[string]*mir2.StructTy),
		classes: make(map[string]*classInfo),
	}

	if err := l.lower(); err != nil {
		return nil, fmt.Errorf("objc lower: %w", err)
	}

	return l.hm, nil
}

// classInfo holds resolved metadata for an ObjC class.
type classInfo struct {
	name    string
	st      *mir2.StructTy
	methods map[string]*methodSig
}

type methodSig struct {
	hirName  string
	retTy    mir2.Ty
	paramTys []mir2.Ty
}

type lowerer struct {
	ast     *cc.AST
	hm      *hir.Module
	globals map[string]mir2.Ty
	structs map[string]*mir2.StructTy
	classes map[string]*classInfo
}

func (l *lowerer) lower() error {
	// Pass 1: collect @interface declarations.
	for tu := l.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil {
			continue
		}
		if ed.Case == cc.ExternalDeclarationObjCInterface {
			l.collectInterface(ed.ObjCInterface)
		}
	}

	// Pass 2: lower everything.
	for tu := l.ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil {
			continue
		}
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
			l.lowerTopDecl(ed.Declaration)
		case cc.ExternalDeclarationObjCImplementation:
			if err := l.lowerImplementation(ed.ObjCImplementation); err != nil {
				return err
			}
		}
	}
	return nil
}

// ── @interface → struct + method sigs ────────────────────────────────────────

func (l *lowerer) collectInterface(iface *cc.ObjCInterfaceDecl) {
	if iface == nil {
		return
	}
	className := iface.ClassName.SrcStr()

	st := &mir2.StructTy{Name: className}
	for _, iv := range iface.Ivars {
		fname := iv.Name.SrcStr()
		fty := mapObjCType(iv.TypeTokens)
		st.Fields = append(st.Fields, mir2.StructField{Name: fname, Ty: fty})
	}
	l.structs[className] = st
	l.hm.Structs = append(l.hm.Structs, st)

	ci := &classInfo{
		name:    className,
		st:      st,
		methods: make(map[string]*methodSig),
	}
	for _, md := range iface.Methods {
		sel := md.MethodName()
		hirName := mangleName(className, sel)
		retTy := mapObjCType(md.ReturnType)
		var paramTys []mir2.Ty
		for _, part := range md.Selector {
			if part.Colon != nil {
				paramTys = append(paramTys, mapObjCType(part.ParamType))
			}
		}
		ci.methods[sel] = &methodSig{hirName: hirName, retTy: retTy, paramTys: paramTys}
	}
	l.classes[className] = ci
}

// ── @implementation → HIR functions ──────────────────────────────────────────

func (l *lowerer) lowerImplementation(impl *cc.ObjCImplementationDecl) error {
	if impl == nil {
		return nil
	}
	className := impl.ClassName.SrcStr()
	ci := l.classes[className]

	for _, md := range impl.Methods {
		sel := md.MethodName()
		hirName := mangleName(className, sel)
		retTy := mapObjCType(md.ReturnType)

		if ci != nil {
			if sig, ok := ci.methods[sel]; ok {
				retTy = sig.retTy
				hirName = sig.hirName
			}
		}

		params := []hir.Param{{Name: "self", Ty: mir2.TyPtr}}
		for _, part := range md.Selector {
			if part.Colon != nil && part.ParamName != nil {
				pname := part.ParamName.SrcStr()
				pty := mapObjCType(part.ParamType)
				params = append(params, hir.Param{Name: pname, Ty: pty})
			}
		}

		fl := &funcLow{
			low:    l,
			name:   hirName,
			retTy:  retTy,
			class:  className,
			locals: make(map[string]mir2.Ty),
		}
		for _, p := range params {
			fl.locals[p.Name] = p.Ty
		}

		body, err := fl.lowerCompound(md.Body)
		if err != nil {
			return fmt.Errorf("@implementation %s method %s: %w", className, sel, err)
		}
		if retTy == mir2.TyVoid && (len(body) == 0 || !isReturn(body[len(body)-1])) {
			body = append(body, hir.RetVoid())
		}

		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
			Name:   hirName,
			Params: params,
			RetTy:  retTy,
			Body:   hir.Blk(body...),
		})
	}
	return nil
}

// ── C function lowering ──────────────────────────────────────────────────────

func (l *lowerer) lowerFunc(fd *cc.FunctionDefinition) (*hir.Func, error) {
	if fd == nil {
		return nil, nil
	}
	decl := fd.Declarator
	name := decl.Name()
	if name == "" {
		return nil, nil
	}
	ft, ok := decl.Type().(*cc.FunctionType)
	if !ok {
		return nil, fmt.Errorf("function %s: expected function type", name)
	}
	retTy := l.mapCType(ft.Result())

	var params []hir.Param
	for _, p := range ft.Parameters() {
		if p.Type() != nil && p.Type().Kind() == cc.Void {
			continue
		}
		pname := p.Name()
		if pname == "" {
			pname = fmt.Sprintf("_p%d", len(params))
		}
		params = append(params, hir.Param{Name: pname, Ty: l.mapCType(p.Type())})
	}

	fl := &funcLow{low: l, name: name, retTy: retTy, locals: make(map[string]mir2.Ty)}
	for _, p := range params {
		fl.locals[p.Name] = p.Ty
	}

	body, err := fl.lowerCompound(fd.CompoundStatement)
	if err != nil {
		return nil, fmt.Errorf("function %s: %w", name, err)
	}
	if retTy == mir2.TyVoid && (len(body) == 0 || !isReturn(body[len(body)-1])) {
		body = append(body, hir.RetVoid())
	}

	return &hir.Func{Name: name, Params: params, RetTy: retTy, Body: hir.Blk(body...)}, nil
}

func (l *lowerer) lowerTopDecl(d *cc.Declaration) {
	if d == nil {
		return
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
		if ty == nil || decl.IsTypename() || ty.Kind() == cc.Function {
			continue
		}
		mty := l.mapCType(ty)
		l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: name, Ty: mty})
		l.globals[name] = mty
	}
}

// ── Function body lowering ───────────────────────────────────────────────────

type funcLow struct {
	low    *lowerer
	name   string
	retTy  mir2.Ty
	class  string
	locals map[string]mir2.Ty
}

func (fl *funcLow) lowerCompound(cs *cc.CompoundStatement) ([]hir.Stmt, error) {
	if cs == nil {
		return nil, nil
	}
	var stmts []hir.Stmt
	for bi := cs.BlockItemList; bi != nil; bi = bi.BlockItemList {
		item := bi.BlockItem
		if item == nil {
			continue
		}
		switch item.Case {
		case cc.BlockItemDecl:
			ds, err := fl.lowerLocalDecl(item.Declaration)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, ds...)
		case cc.BlockItemStmt:
			ss, err := fl.lowerStmt(item.Statement)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, ss...)
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
		name := id.Declarator.Name()
		if name == "" {
			continue
		}
		ty := fl.low.mapCType(id.Declarator.Type())
		fl.locals[name] = ty
		var init hir.Expr
		if id.Initializer != nil && id.Initializer.AssignmentExpression != nil {
			var err error
			init, err = fl.lowerExpr(id.Initializer.AssignmentExpression)
			if err != nil {
				return nil, err
			}
		}
		stmts = append(stmts, &hir.VarDeclStmt{Name: name, Ty: ty, Init: init})
	}
	return stmts, nil
}

func (fl *funcLow) lowerStmt(s *cc.Statement) ([]hir.Stmt, error) {
	if s == nil {
		return nil, nil
	}
	switch s.Case {
	case cc.StatementCompound:
		return fl.lowerCompound(s.CompoundStatement)
	case cc.StatementExpr:
		es := s.ExpressionStatement
		if es == nil || es.ExpressionList == nil {
			return nil, nil
		}
		e, err := fl.lowerExpr(es.ExpressionList)
		if err != nil {
			return nil, err
		}
		if e != nil {
			return []hir.Stmt{&hir.ExprStmt{Expr: e}}, nil
		}
		return nil, nil
	case cc.StatementJump:
		js := s.JumpStatement
		if js == nil {
			return nil, nil
		}
		if js.Case == cc.JumpStatementReturn {
			if js.ExpressionList == nil {
				return []hir.Stmt{hir.RetVoid()}, nil
			}
			val, err := fl.lowerExpr(js.ExpressionList)
			if err != nil {
				return nil, err
			}
			return []hir.Stmt{&hir.ReturnStmt{Val: val}}, nil
		}
	case cc.StatementSelection:
		ss := s.SelectionStatement
		if ss == nil {
			return nil, nil
		}
		if ss.Case == cc.SelectionStatementIf {
			cond, err := fl.lowerExpr(ss.ExpressionList)
			if err != nil {
				return nil, err
			}
			thenStmts, err := fl.lowerStmt(ss.Statement)
			if err != nil {
				return nil, err
			}
			var elseStmts []hir.Stmt
			if ss.Statement2 != nil {
				elseStmts, err = fl.lowerStmt(ss.Statement2)
				if err != nil {
					return nil, err
				}
			}
			ifSt := &hir.IfStmt{Cond: cond, Then: hir.Blk(thenStmts...)}
			if len(elseStmts) > 0 {
				ifSt.Else = hir.Blk(elseStmts...)
			}
			return []hir.Stmt{ifSt}, nil
		}
	case cc.StatementIteration:
		is := s.IterationStatement
		if is == nil {
			return nil, nil
		}
		if is.Case == cc.IterationStatementWhile {
			cond, err := fl.lowerExpr(is.ExpressionList)
			if err != nil {
				return nil, err
			}
			body, err := fl.lowerStmt(is.Statement)
			if err != nil {
				return nil, err
			}
			return []hir.Stmt{&hir.WhileStmt{Cond: cond, Body: hir.Blk(body...)}}, nil
		}
	}
	return nil, nil
}

// ── Expression lowering (follows cc/v4 precedence-based AST) ─────────────────

func (fl *funcLow) lowerExpr(e cc.ExpressionNode) (hir.Expr, error) {
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
		if x.Case == cc.CastExpressionUnary {
			return fl.lowerExpr(x.UnaryExpression)
		}
		return fl.lowerExpr(x.CastExpression)

	// Binary operators — each precedence level.
	case *cc.MultiplicativeExpression:
		return fl.lowerBinOp(x.MultiplicativeExpression, x.CastExpression, x.Case, x.Type())
	case *cc.AdditiveExpression:
		return fl.lowerBinOp(x.AdditiveExpression, x.MultiplicativeExpression, x.Case, x.Type())
	case *cc.ShiftExpression:
		return fl.lowerBinOp(x.ShiftExpression, x.AdditiveExpression, x.Case, x.Type())
	case *cc.RelationalExpression:
		return fl.lowerBinOp(x.RelationalExpression, x.ShiftExpression, x.Case, x.Type())
	case *cc.EqualityExpression:
		return fl.lowerBinOp(x.EqualityExpression, x.RelationalExpression, x.Case, x.Type())
	case *cc.AndExpression:
		return fl.lowerBinOp(x.AndExpression, x.EqualityExpression, x.Case, x.Type())
	case *cc.ExclusiveOrExpression:
		return fl.lowerBinOp(x.ExclusiveOrExpression, x.AndExpression, x.Case, x.Type())
	case *cc.InclusiveOrExpression:
		return fl.lowerBinOp(x.InclusiveOrExpression, x.ExclusiveOrExpression, x.Case, x.Type())

	case *cc.LogicalAndExpression:
		if x.LogicalAndExpression != nil && x.InclusiveOrExpression != nil {
			l, err := fl.lowerExpr(x.LogicalAndExpression)
			if err != nil {
				return nil, err
			}
			r, err := fl.lowerExpr(x.InclusiveOrExpression)
			if err != nil {
				return nil, err
			}
			return &hir.BinExpr{Op: "&&", L: l, R: r, Ty: mir2.TyBool}, nil
		}
		return fl.lowerExpr(x.InclusiveOrExpression)

	case *cc.LogicalOrExpression:
		if x.LogicalOrExpression != nil && x.LogicalAndExpression != nil {
			l, err := fl.lowerExpr(x.LogicalOrExpression)
			if err != nil {
				return nil, err
			}
			r, err := fl.lowerExpr(x.LogicalAndExpression)
			if err != nil {
				return nil, err
			}
			return &hir.BinExpr{Op: "||", L: l, R: r, Ty: mir2.TyBool}, nil
		}
		return fl.lowerExpr(x.LogicalAndExpression)

	case *cc.ConditionalExpression:
		if x.LogicalOrExpression != nil && x.ExpressionList == nil {
			return fl.lowerExpr(x.LogicalOrExpression)
		}
		return fl.lowerExpr(x.LogicalOrExpression) // simplified

	case *cc.ConstantExpression:
		return fl.lowerExpr(x.ConditionalExpression)

	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionCond {
			return fl.lowerExpr(x.ConditionalExpression)
		}
		if x.Case == cc.AssignmentExpressionAssign {
			tgt, err := fl.lowerExpr(x.UnaryExpression)
			if err != nil {
				return nil, err
			}
			val, err := fl.lowerExpr(x.AssignmentExpression)
			if err != nil {
				return nil, err
			}
			_ = tgt // assignment as expression — return value for now
			return val, nil
		}
		return nil, nil

	case *cc.ExpressionList:
		var last hir.Expr
		for el := x; el != nil; el = el.ExpressionList {
			var err error
			last, err = fl.lowerExpr(el.AssignmentExpression)
			if err != nil {
				return nil, err
			}
		}
		return last, nil
	}
	return nil, nil
}

func (fl *funcLow) lowerPrimary(pe *cc.PrimaryExpression) (hir.Expr, error) {
	switch pe.Case {
	case cc.PrimaryExpressionInt:
		v := pe.Value()
		if v == nil {
			return hir.U8(0), nil
		}
		return &hir.IntLitExpr{Val: constToInt64(v), Ty: fl.low.mapCType(pe.Type())}, nil

	case cc.PrimaryExpressionIdent:
		name := pe.Token.SrcStr()
		if _, isLocal := fl.locals[name]; !isLocal {
			if _, isGlobal := fl.low.globals[name]; !isGlobal {
				t := pe.Type()
				isFunc := t != nil && (t.Kind() == cc.Function || t.Kind() == cc.Ptr)
				if !isFunc {
					if v := pe.Value(); v != nil {
						return &hir.IntLitExpr{Val: constToInt64(v), Ty: fl.low.mapCType(t)}, nil
					}
				}
			}
		}
		return hir.Var(name, fl.typeOf(name)), nil

	case cc.PrimaryExpressionExpr:
		return fl.lowerExpr(pe.ExpressionList)

	case cc.PrimaryExpressionObjCMessage:
		return fl.lowerMessage(pe.ObjCMessage)
	}
	return nil, nil
}

func (fl *funcLow) lowerPostfix(pf *cc.PostfixExpression) (hir.Expr, error) {
	switch pf.Case {
	case cc.PostfixExpressionPrimary:
		return fl.lowerExpr(pf.PrimaryExpression)
	case cc.PostfixExpressionCall:
		fn, err := fl.lowerExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		ref, ok := fn.(*hir.VarRefExpr)
		if !ok {
			return nil, fmt.Errorf("non-identifier function call")
		}
		var args []hir.Expr
		for al := pf.ArgumentExpressionList; al != nil; al = al.ArgumentExpressionList {
			a, err := fl.lowerExpr(al.AssignmentExpression)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
		}
		return hir.Call(ref.Name, fl.low.mapCType(pf.Type()), args...), nil

	case cc.PostfixExpressionSelect: // struct.field
		base, err := fl.lowerExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		return &hir.FieldExpr{X: base, Field: pf.Token2.SrcStr(), Ty: fl.low.mapCType(pf.Type())}, nil

	case cc.PostfixExpressionPSelect: // struct->field
		base, err := fl.lowerExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		return &hir.FieldExpr{X: base, Field: pf.Token2.SrcStr(), Ty: fl.low.mapCType(pf.Type())}, nil

	case cc.PostfixExpressionInc:
		base, err := fl.lowerExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		ty := base.ExprTy()
		return hir.Add(base, &hir.IntLitExpr{Val: 1, Ty: ty}, ty), nil

	case cc.PostfixExpressionDec:
		base, err := fl.lowerExpr(pf.PostfixExpression)
		if err != nil {
			return nil, err
		}
		ty := base.ExprTy()
		return hir.Sub(base, &hir.IntLitExpr{Val: 1, Ty: ty}, ty), nil
	}
	return nil, nil
}

func (fl *funcLow) lowerUnary(ue *cc.UnaryExpression) (hir.Expr, error) {
	switch ue.Case {
	case cc.UnaryExpressionPostfix:
		return fl.lowerExpr(ue.PostfixExpression)
	case cc.UnaryExpressionMinus:
		inner, err := fl.lowerExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "-", X: inner, Ty: fl.low.mapCType(ue.Type())}, nil
	case cc.UnaryExpressionNot:
		inner, err := fl.lowerExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "!", X: inner, Ty: mir2.TyBool}, nil
	case cc.UnaryExpressionCpl:
		inner, err := fl.lowerExpr(ue.CastExpression)
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "~", X: inner, Ty: fl.low.mapCType(ue.Type())}, nil
	}
	return nil, nil
}

// lowerBinOp handles any binary expression node generically.
func (fl *funcLow) lowerBinOp(left, right cc.ExpressionNode, caseVal interface{}, ty cc.Type) (hir.Expr, error) {
	if left == nil {
		return fl.lowerExpr(right)
	}
	l, err := fl.lowerExpr(left)
	if err != nil {
		return nil, err
	}
	r, err := fl.lowerExpr(right)
	if err != nil {
		return nil, err
	}
	mty := fl.low.mapCType(ty)
	op := binOpStr(caseVal)
	if op == "" {
		return l, nil
	}
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		mty = mir2.TyBool
	}
	return &hir.BinExpr{Op: op, L: l, R: r, Ty: mty}, nil
}

// ── ObjC message expression lowering ─────────────────────────────────────────

func (fl *funcLow) lowerMessage(msg *cc.ObjCMessageExpr) (hir.Expr, error) {
	if msg == nil {
		return nil, nil
	}
	receiver, err := fl.lowerExpr(msg.Receiver)
	if err != nil {
		return nil, err
	}

	// Build selector string.
	if len(msg.Args) == 0 {
		return nil, fmt.Errorf("empty message expression")
	}
	var sel string
	if len(msg.Args) == 1 && msg.Args[0].Colon == nil {
		sel = msg.Args[0].Keyword.SrcStr()
	} else {
		for _, a := range msg.Args {
			sel += a.Keyword.SrcStr() + ":"
		}
	}

	className := fl.resolveReceiverClass(receiver)
	hirName := mangleName(className, sel)

	retTy := mir2.Ty(mir2.TyI16)
	if ci, ok := fl.low.classes[className]; ok {
		if sig, ok := ci.methods[sel]; ok {
			retTy = sig.retTy
			hirName = sig.hirName
		}
	}

	args := []hir.Expr{receiver}
	for _, a := range msg.Args {
		if a.Colon != nil && a.Value != nil {
			v, err := fl.lowerExpr(a.Value)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
	}

	return hir.Call(hirName, retTy, args...), nil
}

func (fl *funcLow) resolveReceiverClass(recv hir.Expr) string {
	if fl.class != "" {
		if ref, ok := recv.(*hir.VarRefExpr); ok && ref.Name == "self" {
			return fl.class
		}
	}
	return "_Unknown"
}

// ── Type mapping ─────────────────────────────────────────────────────────────

func (l *lowerer) mapCType(t cc.Type) mir2.Ty {
	if t == nil {
		return mir2.TyVoid
	}
	switch t.Kind() {
	case cc.Void:
		return mir2.TyVoid
	case cc.Bool, cc.Char, cc.SChar, cc.UChar:
		return mir2.TyU8
	case cc.Short, cc.UShort, cc.Int, cc.UInt, cc.Enum:
		if t.Kind() == cc.Short || t.Kind() == cc.Int {
			return mir2.TyI16
		}
		return mir2.TyU16
	case cc.Ptr, cc.Function, cc.Array, cc.Struct:
		return mir2.TyPtr
	default:
		return mir2.TyU16
	}
}

func mapObjCType(tokens []cc.Token) mir2.Ty {
	for _, t := range tokens {
		switch t.SrcStr() {
		case "void":
			return mir2.TyVoid
		case "int":
			return mir2.TyI16
		case "char", "uint8_t", "BOOL":
			return mir2.TyU8
		case "unsigned":
			return mir2.TyU16
		}
	}
	if len(tokens) == 0 {
		return mir2.TyI16
	}
	return mir2.TyI16
}

func (fl *funcLow) typeOf(name string) mir2.Ty {
	if ty, ok := fl.locals[name]; ok {
		return ty
	}
	if ty, ok := fl.low.globals[name]; ok {
		return ty
	}
	return mir2.TyI16
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func mangleName(className, selector string) string {
	sel := strings.TrimRight(selector, ":")
	sel = strings.ReplaceAll(sel, ":", "_")
	return className + "_" + sel
}

func binOpStr(caseVal interface{}) string {
	switch v := caseVal.(type) {
	case cc.MultiplicativeExpressionCase:
		switch v {
		case cc.MultiplicativeExpressionMul:
			return "*"
		case cc.MultiplicativeExpressionDiv:
			return "/"
		case cc.MultiplicativeExpressionMod:
			return "%"
		}
	case cc.AdditiveExpressionCase:
		switch v {
		case cc.AdditiveExpressionAdd:
			return "+"
		case cc.AdditiveExpressionSub:
			return "-"
		}
	case cc.ShiftExpressionCase:
		switch v {
		case cc.ShiftExpressionLsh:
			return "<<"
		case cc.ShiftExpressionRsh:
			return ">>"
		}
	case cc.RelationalExpressionCase:
		switch v {
		case cc.RelationalExpressionLt:
			return "<"
		case cc.RelationalExpressionGt:
			return ">"
		case cc.RelationalExpressionLeq:
			return "<="
		case cc.RelationalExpressionGeq:
			return ">="
		}
	case cc.EqualityExpressionCase:
		switch v {
		case cc.EqualityExpressionEq:
			return "=="
		case cc.EqualityExpressionNeq:
			return "!="
		}
	case cc.AndExpressionCase:
		return "&"
	case cc.ExclusiveOrExpressionCase:
		return "^"
	case cc.InclusiveOrExpressionCase:
		return "|"
	}
	return ""
}

func isReturn(s hir.Stmt) bool {
	_, ok := s.(*hir.ReturnStmt)
	return ok
}

func constToInt64(v cc.Value) int64 {
	switch vv := v.(type) {
	case cc.Int64Value:
		return int64(vv)
	case cc.UInt64Value:
		return int64(vv)
	}
	return 0
}
