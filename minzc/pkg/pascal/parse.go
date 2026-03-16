package pascal

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseProgram parses a Pascal source string into an AST.
func ParseProgram(src string) (*Program, error) {
	l, err := NewLexer(src)
	if err != nil {
		return nil, err
	}
	p := &parser{l: l}
	return p.parseProgram()
}

type parser struct {
	l *Lexer
}

func (p *parser) error(msg string) error {
	return fmt.Errorf("line %d: %s (got %q)", p.l.Peek().Line, msg, p.l.Peek().Val)
}

// ── Program ──────────────────────────────────────────────────────────────────

func (p *parser) parseProgram() (*Program, error) {
	prog := &Program{}

	// Optional: program Name;
	if p.l.Is("PROGRAM") {
		p.l.Next()
		tok := p.l.Next()
		if tok.Kind != TokIdent {
			return nil, p.error("expected program name")
		}
		prog.Name = tok.Val
		// Optional parameter list: program Name(Input, Output);
		if p.l.MatchKind(TokLParen) {
			for !p.l.IsKind(TokRParen) && !p.l.IsKind(TokEOF) {
				p.l.Next() // skip identifiers and commas
			}
			p.l.MatchKind(TokRParen)
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
	}

	// Compiler directives: {$R+} etc. are already skipped as comments.

	// Optional: uses clause (TP4+, we just skip it)
	if p.l.Is("USES") {
		p.l.Next()
		for !p.l.IsKind(TokSemicolon) && !p.l.IsKind(TokEOF) {
			p.l.Next()
		}
		p.l.MatchKind(TokSemicolon)
	}

	// Declarations
	decls, err := p.parseDecls()
	if err != nil {
		return nil, err
	}
	prog.Decls = decls

	// Main body: begin..end.
	if err := p.l.Expect("BEGIN"); err != nil {
		return nil, err
	}
	stmts, err := p.parseStmtList("END")
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("END"); err != nil {
		return nil, err
	}
	p.l.MatchKind(TokDot) // final period
	prog.Body = stmts
	return prog, nil
}

// ── Declarations ─────────────────────────────────────────────────────────────

func (p *parser) parseDecls() ([]Decl, error) {
	var decls []Decl
	for {
		switch {
		case p.l.Is("CONST"):
			p.l.Next()
			ds, err := p.parseConstBlock()
			if err != nil {
				return nil, err
			}
			decls = append(decls, ds...)

		case p.l.Is("TYPE"):
			p.l.Next()
			ds, err := p.parseTypeBlock()
			if err != nil {
				return nil, err
			}
			decls = append(decls, ds...)

		case p.l.Is("VAR"):
			p.l.Next()
			ds, err := p.parseVarBlock()
			if err != nil {
				return nil, err
			}
			decls = append(decls, ds...)

		case p.l.Is("PROCEDURE"), p.l.Is("FUNCTION"):
			d, err := p.parseProcDecl()
			if err != nil {
				return nil, err
			}
			decls = append(decls, d)

		case p.l.Is("LABEL"):
			// Skip label section
			p.l.Next()
			for !p.l.IsKind(TokSemicolon) && !p.l.IsKind(TokEOF) {
				p.l.Next()
			}
			p.l.MatchKind(TokSemicolon)

		default:
			return decls, nil
		}
	}
}

// parseConstBlock parses: Name = Expr; Name: Type = Expr; ...
func (p *parser) parseConstBlock() ([]Decl, error) {
	var decls []Decl
	for p.l.Peek().Kind == TokIdent && !isBlockKw(p.l.Peek().Val) {
		name := p.l.Next().Val

		// Typed constant: Name: Type = Expr;
		if p.l.MatchKind(TokColon) {
			ty, err := p.parseType()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.ExpectKind(TokEq); err != nil {
				return nil, err
			}
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			decls = append(decls, &TypedConstDecl{Name: name, Ty: ty, Val: val})
		} else {
			// Untyped constant: Name = Expr;
			if _, err := p.l.ExpectKind(TokEq); err != nil {
				return nil, err
			}
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			decls = append(decls, &ConstDecl{Name: name, Val: val})
		}
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
	}
	return decls, nil
}

// parseTypeBlock parses: Name = Type; ...
func (p *parser) parseTypeBlock() ([]Decl, error) {
	var decls []Decl
	for p.l.Peek().Kind == TokIdent && !isBlockKw(p.l.Peek().Val) {
		name := p.l.Next().Val
		if _, err := p.l.ExpectKind(TokEq); err != nil {
			return nil, err
		}
		ty, err := p.parseType()
		if err != nil {
			return nil, err
		}
		decls = append(decls, &TypeDecl{Name: name, Ty: ty})
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
	}
	return decls, nil
}

// parseVarBlock parses: Name1, Name2: Type; ...
func (p *parser) parseVarBlock() ([]Decl, error) {
	var decls []Decl
	for p.l.Peek().Kind == TokIdent && !isBlockKw(p.l.Peek().Val) {
		names, err := p.parseIdentList()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokColon); err != nil {
			return nil, err
		}
		ty, err := p.parseType()
		if err != nil {
			return nil, err
		}
		decls = append(decls, &VarDecl{Names: names, Ty: ty})
		if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
			return nil, err
		}
	}
	return decls, nil
}

func (p *parser) parseIdentList() ([]string, error) {
	tok := p.l.Next()
	if tok.Kind != TokIdent {
		return nil, fmt.Errorf("line %d: expected identifier, got %q", tok.Line, tok.Val)
	}
	names := []string{tok.Val}
	for p.l.MatchKind(TokComma) {
		tok = p.l.Next()
		if tok.Kind != TokIdent {
			return nil, fmt.Errorf("line %d: expected identifier after comma, got %q", tok.Line, tok.Val)
		}
		names = append(names, tok.Val)
	}
	return names, nil
}

// ── Types ────────────────────────────────────────────────────────────────────

func (p *parser) parseType() (PasType, error) {
	// ^Type (pointer)
	if p.l.MatchKind(TokCaret) {
		base, err := p.parseType()
		if err != nil {
			return nil, err
		}
		return &PointerType{Base: base}, nil
	}

	// array[Lo..Hi] of ElemTy
	if p.l.Is("ARRAY") {
		return p.parseArrayType()
	}

	// record..end
	if p.l.Is("RECORD") {
		return p.parseRecordType()
	}

	// string[N]
	if p.l.Is("STRING") {
		p.l.Next()
		maxLen := 255 // default
		if p.l.MatchKind(TokLBrack) {
			n, err := p.parseIntLiteral()
			if err != nil {
				return nil, err
			}
			maxLen = int(n)
			if _, err := p.l.ExpectKind(TokRBrack); err != nil {
				return nil, err
			}
		}
		return &StringType{MaxLen: maxLen}, nil
	}

	// function/procedure pointer type: function(x: byte): byte
	if p.l.Is("FUNCTION") || p.l.Is("PROCEDURE") {
		isFunc := p.l.Is("FUNCTION")
		p.l.Next()
		var paramTypes []PasType
		if p.l.MatchKind(TokLParen) {
			for !p.l.IsKind(TokRParen) {
				// Skip parameter names — just grab types.
				// Format: name: type  or  name1, name2: type
				for p.l.IsKind(TokIdent) {
					p.l.Next() // consume name
					if p.l.MatchKind(TokComma) {
						continue
					}
					break
				}
				if _, err := p.l.ExpectKind(TokColon); err != nil {
					return nil, err
				}
				pt, err := p.parseType()
				if err != nil {
					return nil, err
				}
				paramTypes = append(paramTypes, pt)
				p.l.MatchKind(TokSemicolon) // params separated by ;
			}
			if _, err := p.l.ExpectKind(TokRParen); err != nil {
				return nil, err
			}
		}
		var retTy PasType
		if isFunc && p.l.MatchKind(TokColon) {
			rt, err := p.parseType()
			if err != nil {
				return nil, err
			}
			retTy = rt
		}
		return &ProcPtrType{Params: paramTypes, RetTy: retTy}, nil
	}

	// Named type or basic type
	tok := p.l.Next()
	if tok.Kind != TokIdent {
		return nil, fmt.Errorf("line %d: expected type name, got %q", tok.Line, tok.Val)
	}

	switch tok.Val {
	case "INTEGER", "BYTE", "WORD", "BOOLEAN", "CHAR",
		"SHORTINT", "LONGINT", "REAL":
		return &BasicType{Name: tok.Val}, nil
	default:
		return &TypeRef{Name: tok.Val}, nil
	}
}

func (p *parser) parseArrayType() (PasType, error) {
	if err := p.l.Expect("ARRAY"); err != nil {
		return nil, err
	}
	if _, err := p.l.ExpectKind(TokLBrack); err != nil {
		return nil, err
	}

	lo, err := p.parseIntLiteral()
	if err != nil {
		return nil, err
	}
	if _, err := p.l.ExpectKind(TokDotDot); err != nil {
		return nil, err
	}
	hi, err := p.parseIntLiteral()
	if err != nil {
		return nil, err
	}

	// Multi-dim: array[0..2, 0..2] of ...
	// For now just handle one dimension; multi-dim flattens
	if _, err := p.l.ExpectKind(TokRBrack); err != nil {
		return nil, err
	}

	if err := p.l.Expect("OF"); err != nil {
		return nil, err
	}
	elem, err := p.parseType()
	if err != nil {
		return nil, err
	}
	return &ArrayType{Lo: int(lo), Hi: int(hi), Elem: elem}, nil
}

func (p *parser) parseRecordType() (PasType, error) {
	if err := p.l.Expect("RECORD"); err != nil {
		return nil, err
	}
	var fields []RecordField
	for !p.l.Is("END") && !p.l.IsKind(TokEOF) {
		if p.l.MatchKind(TokSemicolon) {
			continue
		}
		names, err := p.parseIdentList()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokColon); err != nil {
			return nil, err
		}
		ty, err := p.parseType()
		if err != nil {
			return nil, err
		}
		fields = append(fields, RecordField{Names: names, Ty: ty})
		p.l.MatchKind(TokSemicolon) // optional trailing semicolon
	}
	if err := p.l.Expect("END"); err != nil {
		return nil, err
	}
	return &RecordType{Fields: fields}, nil
}

func (p *parser) parseIntLiteral() (int64, error) {
	tok := p.l.Peek()
	neg := false
	if tok.Kind == TokMinus {
		neg = true
		p.l.Next()
		tok = p.l.Peek()
	}
	if tok.Kind == TokNumber {
		p.l.Next()
		n, err := parseNumber(tok.Val)
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", tok.Line, err)
		}
		if neg {
			n = -n
		}
		return n, nil
	}
	// Could be a named constant — return 0 as placeholder
	if tok.Kind == TokIdent {
		p.l.Next()
		// Hardcode common TP constants
		switch tok.Val {
		case "MAXINT":
			return 32767, nil
		default:
			return 0, nil // unresolved constant
		}
	}
	return 0, fmt.Errorf("line %d: expected integer, got %q", tok.Line, tok.Val)
}

// ── Procedure/Function ───────────────────────────────────────────────────────

func (p *parser) parseProcDecl() (*ProcDecl, error) {
	isFunc := p.l.Is("FUNCTION")
	p.l.Next() // consume PROCEDURE/FUNCTION

	tok := p.l.Next()
	if tok.Kind != TokIdent {
		return nil, p.error("expected procedure/function name")
	}
	pd := &ProcDecl{Name: tok.Val}

	// Parameter list
	if p.l.MatchKind(TokLParen) {
		params, err := p.parseParamList()
		if err != nil {
			return nil, err
		}
		pd.Params = params
		if _, err := p.l.ExpectKind(TokRParen); err != nil {
			return nil, err
		}
	}

	// Return type for functions
	if isFunc {
		if _, err := p.l.ExpectKind(TokColon); err != nil {
			return nil, err
		}
		ty, err := p.parseType()
		if err != nil {
			return nil, err
		}
		pd.RetTy = ty
	}

	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}

	// Forward declaration
	if p.l.Is("FORWARD") {
		p.l.Next()
		p.l.MatchKind(TokSemicolon)
		pd.Body = nil
		return pd, nil
	}

	// Local declarations (including nested procedures)
	locals, err := p.parseDecls()
	if err != nil {
		return nil, err
	}
	// Separate nested procs from other locals
	for _, d := range locals {
		if sub, ok := d.(*ProcDecl); ok {
			pd.SubProc = append(pd.SubProc, sub)
		} else {
			pd.Locals = append(pd.Locals, d)
		}
	}

	// Body: begin..end;
	if err := p.l.Expect("BEGIN"); err != nil {
		return nil, err
	}
	stmts, err := p.parseStmtList("END")
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("END"); err != nil {
		return nil, err
	}
	if _, err := p.l.ExpectKind(TokSemicolon); err != nil {
		return nil, err
	}
	pd.Body = stmts
	return pd, nil
}

func (p *parser) parseParamList() ([]ParamGroup, error) {
	var groups []ParamGroup
	for {
		if p.l.IsKind(TokRParen) || p.l.IsKind(TokEOF) {
			break
		}
		isVar := p.l.Match("VAR")
		names, err := p.parseIdentList()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokColon); err != nil {
			return nil, err
		}
		ty, err := p.parseType()
		if err != nil {
			return nil, err
		}
		groups = append(groups, ParamGroup{Names: names, Ty: ty, IsVar: isVar})
		if !p.l.MatchKind(TokSemicolon) {
			break
		}
	}
	return groups, nil
}

// ── Statements ───────────────────────────────────────────────────────────────

func (p *parser) parseStmtList(terminator string) ([]Stmt, error) {
	var stmts []Stmt
	for !p.l.Is(terminator) && !p.l.IsKind(TokEOF) {
		// Skip empty statements (consecutive semicolons)
		if p.l.MatchKind(TokSemicolon) {
			continue
		}
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if s != nil {
			stmts = append(stmts, s)
		}
		// Semicolons between statements
		p.l.MatchKind(TokSemicolon)
	}
	return stmts, nil
}

func (p *parser) parseStmt() (Stmt, error) {
	tok := p.l.Peek()

	switch {
	case tok.Kind == TokIdent && tok.Val == "BEGIN":
		return p.parseBlock()
	case tok.Kind == TokIdent && tok.Val == "IF":
		return p.parseIf()
	case tok.Kind == TokIdent && tok.Val == "WHILE":
		return p.parseWhile()
	case tok.Kind == TokIdent && tok.Val == "REPEAT":
		return p.parseRepeat()
	case tok.Kind == TokIdent && tok.Val == "FOR":
		return p.parseFor()
	case tok.Kind == TokIdent && tok.Val == "CASE":
		return p.parseCase()
	case tok.Kind == TokIdent && tok.Val == "EXIT":
		p.l.Next()
		return &ExitStmt{}, nil
	case tok.Kind == TokIdent && tok.Val == "ASSERT":
		return p.parseAssert()
	case tok.Kind == TokIdent:
		return p.parseAssignOrCall()
	default:
		return nil, nil // empty statement
	}
}

func (p *parser) parseBlock() (Stmt, error) {
	if err := p.l.Expect("BEGIN"); err != nil {
		return nil, err
	}
	stmts, err := p.parseStmtList("END")
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("END"); err != nil {
		return nil, err
	}
	return &Block{Stmts: stmts}, nil
}

func (p *parser) parseIf() (Stmt, error) {
	p.l.Next() // consume IF
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("THEN"); err != nil {
		return nil, err
	}
	then, err := p.parseStmt()
	if err != nil {
		return nil, err
	}
	var els Stmt
	if p.l.Match("ELSE") {
		els, err = p.parseStmt()
		if err != nil {
			return nil, err
		}
	}
	return &IfStmt{Cond: cond, Then: then, Else: els}, nil
}

func (p *parser) parseWhile() (Stmt, error) {
	p.l.Next() // consume WHILE
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("DO"); err != nil {
		return nil, err
	}
	body, err := p.parseStmt()
	if err != nil {
		return nil, err
	}
	return &WhileStmt{Cond: cond, Body: body}, nil
}

func (p *parser) parseRepeat() (Stmt, error) {
	p.l.Next() // consume REPEAT
	stmts, err := p.parseStmtList("UNTIL")
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("UNTIL"); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &RepeatStmt{Body: stmts, Cond: cond}, nil
}

func (p *parser) parseFor() (Stmt, error) {
	p.l.Next() // consume FOR
	varTok := p.l.Next()
	if varTok.Kind != TokIdent {
		return nil, p.error("expected loop variable")
	}
	if _, err := p.l.ExpectKind(TokAssign); err != nil {
		return nil, err
	}
	start, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	downto := false
	if p.l.Is("DOWNTO") {
		downto = true
		p.l.Next()
	} else if err := p.l.Expect("TO"); err != nil {
		return nil, err
	}
	end, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("DO"); err != nil {
		return nil, err
	}
	body, err := p.parseStmt()
	if err != nil {
		return nil, err
	}
	return &ForStmt{Var: varTok.Val, Start: start, End: end, Downto: downto, Body: body}, nil
}

func (p *parser) parseCase() (Stmt, error) {
	p.l.Next() // consume CASE
	sel, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.l.Expect("OF"); err != nil {
		return nil, err
	}

	cs := &CaseStmt{Sel: sel}
	for !p.l.Is("END") && !p.l.Is("ELSE") && !p.l.IsKind(TokEOF) {
		if p.l.MatchKind(TokSemicolon) {
			continue
		}
		// Parse case label(s): val1, val2, ...: stmt
		var vals []int64
		for {
			v, err := p.parseCaseLabel()
			if err != nil {
				return nil, err
			}
			vals = append(vals, v)
			if !p.l.MatchKind(TokComma) {
				break
			}
		}
		if _, err := p.l.ExpectKind(TokColon); err != nil {
			return nil, err
		}
		body, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		cs.Arms = append(cs.Arms, CaseArm{Vals: vals, Body: body})
		p.l.MatchKind(TokSemicolon)
	}

	if p.l.Match("ELSE") {
		stmts, err := p.parseStmtList("END")
		if err != nil {
			return nil, err
		}
		cs.Default = stmts
	}

	if err := p.l.Expect("END"); err != nil {
		return nil, err
	}
	return cs, nil
}

func (p *parser) parseCaseLabel() (int64, error) {
	tok := p.l.Peek()
	if tok.Kind == TokString && len(tok.Val) == 1 {
		p.l.Next()
		return int64(tok.Val[0]), nil
	}
	return p.parseIntLiteral()
}

func (p *parser) parseAssert() (Stmt, error) {
	p.l.Next() // consume ASSERT
	// assert FuncName(args) = Expected
	// The whole thing parses as a comparison expression: FuncName(args) = Expected
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	// Must be a comparison: Call = Literal
	binop, ok := expr.(*BinOp)
	if !ok || (binop.Op != "=" && binop.Op != "<>") {
		return nil, p.error("assert requires form: assert Func(args) = expected")
	}

	ce, ok := binop.L.(*CallExpr)
	if !ok {
		return nil, p.error("assert requires a function call on the left side")
	}

	return &AssertStmt{
		FuncName: ce.Name,
		Args:     ce.Args,
		Op:       binop.Op,
		Expected: binop.R,
	}, nil
}

// parseAssignOrCall: identifier followed by := (assign), ( (call), or nothing.
func (p *parser) parseAssignOrCall() (Stmt, error) {
	// Parse left-hand side as an expression (handles a.b, a[i], a^, etc.)
	lhs, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	// Assignment: lhs := rhs
	if p.l.MatchKind(TokAssign) {
		rhs, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &AssignStmt{Target: lhs, Val: rhs}, nil
	}

	// It's a call statement (procedure call as expression was already parsed)
	if ce, ok := lhs.(*CallExpr); ok {
		return &CallStmt{Name: ce.Name, Args: ce.Args}, nil
	}
	if vr, ok := lhs.(*VarRef); ok {
		return &CallStmt{Name: vr.Name, Args: nil}, nil
	}
	return nil, fmt.Errorf("line %d: expected assignment or procedure call", p.l.Peek().Line)
}

// ── Expressions (precedence climbing) ────────────────────────────────────────

func (p *parser) parseExpr() (Expr, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (Expr, error) {
	l, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.l.Is("OR") || p.l.Is("XOR") {
		op := p.l.Next().Val
		r, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		l = &BinOp{Op: op, L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseAnd() (Expr, error) {
	l, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.l.Is("AND") {
		p.l.Next()
		r, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		l = &BinOp{Op: "AND", L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseNot() (Expr, error) {
	if p.l.Is("NOT") {
		p.l.Next()
		x, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		return &UnaryOp{Op: "NOT", X: x}, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (Expr, error) {
	l, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.l.Peek()
		switch tok.Kind {
		case TokEq:
			p.l.Next()
			r, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "=", L: l, R: r}
		case TokNotEq:
			p.l.Next()
			r, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "<>", L: l, R: r}
		case TokLt:
			p.l.Next()
			r, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "<", L: l, R: r}
		case TokGt:
			p.l.Next()
			r, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: ">", L: l, R: r}
		case TokLtEq:
			p.l.Next()
			r, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "<=", L: l, R: r}
		case TokGtEq:
			p.l.Next()
			r, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: ">=", L: l, R: r}
		case TokIdent:
			if tok.Val == "IN" {
				// set membership — not supported yet, skip
				return l, nil
			}
			return l, nil
		default:
			return l, nil
		}
	}
}

func (p *parser) parseAddSub() (Expr, error) {
	l, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.l.Peek()
		switch {
		case tok.Kind == TokPlus:
			p.l.Next()
			r, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "+", L: l, R: r}
		case tok.Kind == TokMinus:
			p.l.Next()
			r, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "-", L: l, R: r}
		case tok.Kind == TokIdent && tok.Val == "SHL":
			p.l.Next()
			r, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "SHL", L: l, R: r}
		case tok.Kind == TokIdent && tok.Val == "SHR":
			p.l.Next()
			r, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "SHR", L: l, R: r}
		default:
			return l, nil
		}
	}
}

func (p *parser) parseMulDiv() (Expr, error) {
	l, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.l.Peek()
		switch {
		case tok.Kind == TokStar:
			p.l.Next()
			r, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "*", L: l, R: r}
		case tok.Kind == TokSlash:
			p.l.Next()
			r, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "/", L: l, R: r}
		case tok.Kind == TokIdent && tok.Val == "MOD":
			p.l.Next()
			r, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "MOD", L: l, R: r}
		case tok.Kind == TokIdent && tok.Val == "DIV":
			p.l.Next()
			r, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			l = &BinOp{Op: "DIV", L: l, R: r}
		default:
			return l, nil
		}
	}
}

func (p *parser) parseUnary() (Expr, error) {
	tok := p.l.Peek()
	if tok.Kind == TokMinus {
		p.l.Next()
		x, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		return &UnaryOp{Op: "-", X: x}, nil
	}
	if tok.Kind == TokAt {
		p.l.Next()
		x, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		return &AddrExpr{X: x}, nil
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() (Expr, error) {
	base, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.l.IsKind(TokLBrack):
			// Array indexing: base[idx]
			p.l.Next()
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.ExpectKind(TokRBrack); err != nil {
				return nil, err
			}
			base = &IndexExpr{Base: base, Idx: idx}

		case p.l.IsKind(TokDot):
			// Field access: base.field
			p.l.Next()
			tok := p.l.Next()
			if tok.Kind != TokIdent {
				return nil, fmt.Errorf("line %d: expected field name after '.', got %q", tok.Line, tok.Val)
			}
			base = &FieldExpr{Base: base, Field: tok.Val}

		case p.l.IsKind(TokCaret):
			// Pointer dereference: base^
			p.l.Next()
			base = &DerefExpr{X: base}

		default:
			return base, nil
		}
	}
}

func (p *parser) parsePrimary() (Expr, error) {
	tok := p.l.Peek()

	switch tok.Kind {
	case TokNumber:
		p.l.Next()
		n, err := parseNumber(tok.Val)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", tok.Line, err)
		}
		return &IntLit{Val: n}, nil

	case TokString:
		p.l.Next()
		if len(tok.Val) == 1 {
			return &CharLit{Val: tok.Val[0]}, nil
		}
		return &StrLit{Val: tok.Val}, nil

	case TokLParen:
		p.l.Next()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.ExpectKind(TokRParen); err != nil {
			return nil, err
		}
		return e, nil

	case TokIdent:
		switch tok.Val {
		case "TRUE":
			p.l.Next()
			return &BoolLit{Val: true}, nil
		case "FALSE":
			p.l.Next()
			return &BoolLit{Val: false}, nil
		}

		p.l.Next()
		name := tok.Val

		// Function call: Name(args)
		if p.l.MatchKind(TokLParen) {
			var args []Expr
			if !p.l.IsKind(TokRParen) {
				for {
					a, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if !p.l.MatchKind(TokComma) {
						break
					}
				}
			}
			if _, err := p.l.ExpectKind(TokRParen); err != nil {
				return nil, err
			}
			return &CallExpr{Name: name, Args: args}, nil
		}

		return &VarRef{Name: name}, nil

	default:
		return nil, p.error("expected expression")
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func parseNumber(s string) (int64, error) {
	if strings.HasPrefix(s, "#") {
		// #65 = ord 65
		return strconv.ParseInt(s[1:], 10, 64)
	}
	if strings.HasPrefix(s, "$") {
		// $FF = hex
		return strconv.ParseInt(s[1:], 16, 64)
	}
	return strconv.ParseInt(s, 10, 64)
}

func isBlockKw(s string) bool {
	switch s {
	case "BEGIN", "END", "CONST", "TYPE", "VAR", "PROCEDURE", "FUNCTION",
		"LABEL", "USES", "PROGRAM":
		return true
	}
	return false
}
