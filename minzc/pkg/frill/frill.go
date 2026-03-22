// Package frill implements an ML-style functional language frontend for MinZ.
//
// Hrill compiles to HIR, reusing the entire MIR2→Z80/eZ80 pipeline.
// It aims to be a "Linear ML for 8-bit" — strict, size-aware, with ADT and
// pattern matching, targeting retro hardware.
//
// Syntax overview (Hrill-0: OCaml/F# subset):
//
//   (* function definition *)
//   let add (a : u8) (b : u8) : u8 = a + b
//
//   (* with type inference for body *)
//   let double (x : u8) : u8 = x + x
//
//   (* if expression *)
//   let max (a : u8) (b : u8) : u8 = if a > b then a else b
//
//   (* let binding *)
//   let dist (x : u8) (y : u8) : u8 =
//     let dx = x - y in
//     let dy = y - x in
//     dx + dy
//
//   (* type alias *)
//   type byte = u8
//
//   (* ADT — future *)
//   type option = None | Some of u8
//
//   (* pipe — future *)
//   let result = x |> double |> add 1
//
//   (* assert for testing *)
//   assert add 3 5 == 8
//   assert max 10 20 == 20
//
// Pipeline: Hrill source → parse → *hir.Module → MIR2 → codegen
package frill

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Compile parses Hrill source and returns an HIR module.
func Compile(src, name string) (*hir.Module, error) {
	p := &parser{src: src, pos: 0, line: 1, name: name}
	return p.parseModule()
}

// ── Token types ─────────────────────────────────────────────────────────────

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokInt
	tokString
	tokOp     // + - * / % == != < <= > >= && || |>
	tokLParen // (
	tokRParen // )
	tokColon  // :
	tokEq     // =
	tokArrow  // ->
	tokComma  // ,
)

type token struct {
	kind tokKind
	text string
	line int
}

// ── Lexer ───────────────────────────────────────────────────────────────────

type parser struct {
	src    string
	pos    int
	line   int
	name   string
	peeked *token
}

func (p *parser) peek() token {
	if p.peeked != nil {
		return *p.peeked
	}
	t := p.lex()
	p.peeked = &t
	return t
}

func (p *parser) next() token {
	if p.peeked != nil {
		t := *p.peeked
		p.peeked = nil
		return t
	}
	return p.lex()
}

func (p *parser) expect(kind tokKind, text string) error {
	t := p.next()
	if t.kind != kind || (text != "" && t.text != text) {
		return fmt.Errorf("line %d: expected %q, got %q", t.line, text, t.text)
	}
	return nil
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		if ch == '\n' {
			p.pos++
			p.line++
		} else if ch == ' ' || ch == '\t' || ch == '\r' {
			p.pos++
		} else if p.pos+1 < len(p.src) && ch == '(' && p.src[p.pos+1] == '*' {
			// Block comment (* ... *)
			p.pos += 2
			for p.pos+1 < len(p.src) {
				if p.src[p.pos] == '*' && p.src[p.pos+1] == ')' {
					p.pos += 2
					break
				}
				if p.src[p.pos] == '\n' {
					p.line++
				}
				p.pos++
			}
		} else if p.pos+1 < len(p.src) && ch == '-' && p.src[p.pos+1] == '-' {
			// Line comment -- ...
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *parser) lex() token {
	p.skipWhitespace()
	if p.pos >= len(p.src) {
		return token{kind: tokEOF, line: p.line}
	}

	line := p.line
	ch := p.src[p.pos]

	// Number
	if ch >= '0' && ch <= '9' {
		start := p.pos
		for p.pos < len(p.src) && (p.src[p.pos] >= '0' && p.src[p.pos] <= '9') {
			p.pos++
		}
		return token{tokInt, p.src[start:p.pos], line}
	}

	// Identifier or keyword
	if ch == '_' || unicode.IsLetter(rune(ch)) {
		start := p.pos
		for p.pos < len(p.src) {
			c := p.src[p.pos]
			if c == '_' || c == '\'' || unicode.IsLetter(rune(c)) || (c >= '0' && c <= '9') {
				p.pos++
			} else {
				break
			}
		}
		return token{tokIdent, p.src[start:p.pos], line}
	}

	// Two-char operators
	if p.pos+1 < len(p.src) {
		two := p.src[p.pos : p.pos+2]
		switch two {
		case "==", "!=", "<=", ">=", "&&", "||", "|>", "->":
			p.pos += 2
			if two == "->" {
				return token{tokArrow, two, line}
			}
			return token{tokOp, two, line}
		}
	}

	// Single-char
	p.pos++
	switch ch {
	case '(':
		return token{tokLParen, "(", line}
	case ')':
		return token{tokRParen, ")", line}
	case ':':
		return token{tokColon, ":", line}
	case '=':
		return token{tokEq, "=", line}
	case ',':
		return token{tokComma, ",", line}
	case '+', '-', '*', '/', '%', '<', '>':
		return token{tokOp, string(ch), line}
	}

	return token{tokIdent, string(ch), line}
}

// ── Parser ──────────────────────────────────────────────────────────────────

func (p *parser) parseModule() (*hir.Module, error) {
	mod := &hir.Module{Name: p.name}

	for p.peek().kind != tokEOF {
		t := p.peek()
		switch t.text {
		case "let":
			fn, err := p.parseLet()
			if err != nil {
				return nil, err
			}
			mod.Funcs = append(mod.Funcs, fn)
		case "assert":
			a, err := p.parseAssert()
			if err != nil {
				return nil, err
			}
			mod.Asserts = append(mod.Asserts, a)
		case "type":
			p.next() // skip "type" — TODO: type aliases/ADTs
			for p.peek().kind != tokEOF && p.peek().text != "let" && p.peek().text != "assert" && p.peek().text != "type" {
				p.next()
			}
		default:
			return nil, fmt.Errorf("line %d: unexpected %q at module level", t.line, t.text)
		}
	}

	return mod, nil
}

// parseLet parses:  let name (p1 : t1) (p2 : t2) : retty = body
func (p *parser) parseLet() (*hir.Func, error) {
	p.next() // consume "let"
	nameTok := p.next()
	if nameTok.kind != tokIdent {
		return nil, fmt.Errorf("line %d: expected function name, got %q", nameTok.line, nameTok.text)
	}

	fn := &hir.Func{Name: nameTok.text}

	// Parse parameters: (name : type) ...
	for p.peek().kind == tokLParen {
		p.next() // (
		pname := p.next()
		if err := p.expect(tokColon, ":"); err != nil {
			return nil, err
		}
		pty := p.parseType()
		if err := p.expect(tokRParen, ")"); err != nil {
			return nil, err
		}
		fn.Params = append(fn.Params, hir.Param{Name: pname.text, Ty: pty})
	}

	// Return type: : type
	if p.peek().kind == tokColon {
		p.next() // :
		fn.RetTy = p.parseType()
	} else {
		fn.RetTy = mir2.TyVoid
	}

	// = body (may contain let-in chains which desugar to VarDecl stmts)
	if err := p.expect(tokEq, "="); err != nil {
		return nil, err
	}

	var stmts []hir.Stmt
	body, letStmts, err := p.parseBodyExpr()
	if err != nil {
		return nil, err
	}
	stmts = append(stmts, letStmts...)
	stmts = append(stmts, &hir.ReturnStmt{Val: body})

	fn.Body = &hir.Block{Body: stmts}

	return fn, nil
}

// parseAssert: assert funcname arg1 arg2 == expected
func (p *parser) parseAssert() (hir.Assert, error) {
	p.next() // consume "assert"
	line := p.peek().line

	funcName := p.next()
	if funcName.kind != tokIdent {
		return hir.Assert{}, fmt.Errorf("line %d: assert: expected function name", line)
	}

	// Parse args (int literals) until ==
	var args []int64
	for p.peek().kind == tokInt {
		val, _ := strconv.ParseInt(p.next().text, 10, 64)
		args = append(args, val)
	}

	if err := p.expect(tokOp, "=="); err != nil {
		return hir.Assert{}, fmt.Errorf("line %d: assert: expected '=='", line)
	}

	expTok := p.next()
	expected, _ := strconv.ParseInt(expTok.text, 10, 64)

	return hir.Assert{
		FuncName: funcName.text,
		Args:     args,
		Expected: expected,
		Source:   fmt.Sprintf("assert %s %s == %s", funcName.text, fmtArgs(args), expTok.text),
		Line:     line,
		Via:      "mir2",
	}, nil
}

// ── Expression parser ───────────────────────────────────────────────────────

func (p *parser) parseExpr() (hir.Expr, error) {
	return p.parsePipe()
}

// parsePipe: expr |> fn |> fn2  →  fn2(fn(expr))
func (p *parser) parsePipe() (hir.Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOp && p.peek().text == "|>" {
		p.next() // consume |>
		// Right side: function name, optionally with extra args
		fnTok := p.next()
		if fnTok.kind != tokIdent {
			return nil, fmt.Errorf("line %d: expected function name after |>, got %q", fnTok.line, fnTok.text)
		}
		// Collect extra args (int literals or parens before next |> or operator)
		args := []hir.Expr{left}
		for p.peek().kind == tokInt || p.peek().kind == tokLParen {
			arg, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		left = &hir.CallExpr{Fn: fnTok.text, Args: args, Ty: left.ExprTy()}
	}
	return left, nil
}

func (p *parser) parseComparison() (hir.Expr, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOp {
		op := p.peek().text
		if op == "==" || op == "!=" || op == "<" || op == "<=" || op == ">" || op == ">=" {
			p.next()
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			left = &hir.BinExpr{Op: op, L: left, R: right, Ty: mir2.TyBool}
		} else {
			break
		}
	}
	return left, nil
}

func (p *parser) parseAdd() (hir.Expr, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOp && (p.peek().text == "+" || p.peek().text == "-") {
		op := p.next().text
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &hir.BinExpr{Op: op, L: left, R: right, Ty: left.ExprTy()}
	}
	return left, nil
}

func (p *parser) parseMul() (hir.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOp && (p.peek().text == "*" || p.peek().text == "/" || p.peek().text == "%") {
		op := p.next().text
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &hir.BinExpr{Op: op, L: left, R: right, Ty: left.ExprTy()}
	}
	return left, nil
}

func (p *parser) parseUnary() (hir.Expr, error) {
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (hir.Expr, error) {
	t := p.peek()

	// Parenthesized expression
	if t.kind == tokLParen {
		p.next()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(tokRParen, ")"); err != nil {
			return nil, err
		}
		return e, nil
	}

	// Integer literal
	if t.kind == tokInt {
		p.next()
		val, _ := strconv.ParseInt(t.text, 10, 64)
		ty := mir2.TyU8
		if val > 255 {
			ty = mir2.TyU16
		}
		return &hir.IntLitExpr{Val: val, Ty: ty}, nil
	}

	// if-then-else expression
	if t.kind == tokIdent && t.text == "if" {
		return p.parseIf()
	}

	// let-in expression
	if t.kind == tokIdent && t.text == "let" {
		return p.parseLetIn()
	}

	// Identifier (variable ref or function call)
	if t.kind == tokIdent {
		p.next()
		name := t.text

		// Check if followed by arguments (another primary expr that isn't an operator)
		// For now: only call if next token is int or ident (simple heuristic)
		var args []hir.Expr
		for p.peek().kind == tokInt || (p.peek().kind == tokIdent && !isKeyword(p.peek().text)) || p.peek().kind == tokLParen {
			arg, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}

		if len(args) > 0 {
			return &hir.CallExpr{Fn: name, Args: args, Ty: mir2.TyU8}, nil
		}

		// Variable reference — infer type from context
		return &hir.VarRefExpr{Name: name, Ty: mir2.TyU8}, nil
	}

	return nil, fmt.Errorf("line %d: unexpected token %q", t.line, t.text)
}

// parseIf: if cond then thenExpr else elseExpr
func (p *parser) parseIf() (hir.Expr, error) {
	p.next() // "if"
	cond, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	if err := p.expect(tokIdent, "then"); err != nil {
		return nil, err
	}
	thenE, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(tokIdent, "else"); err != nil {
		return nil, err
	}
	elseE, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &hir.CondExpr{Cond: cond, Then: thenE, Else: elseE, Ty: thenE.ExprTy()}, nil
}

// parseBodyExpr parses an expression that may start with let-in chains.
// Returns the final expression + any VarDeclStmt's extracted from let-in.
//
//   let x = 1 in let y = 2 in x + y
//   →  stmts: [VarDecl x=1, VarDecl y=2], expr: x+y
func (p *parser) parseBodyExpr() (hir.Expr, []hir.Stmt, error) {
	var stmts []hir.Stmt
	for p.peek().kind == tokIdent && p.peek().text == "let" {
		// Peek ahead: is this let-in or a function call to "let" (shouldn't happen)?
		// Save position to restore if this isn't a let-in
		p.next() // consume "let"
		nameTok := p.next()
		if p.peek().kind != tokEq {
			// Not a let-in binding — put tokens back conceptually
			// This shouldn't happen in well-formed Frill
			return nil, nil, fmt.Errorf("line %d: expected '=' after let %s", nameTok.line, nameTok.text)
		}
		p.next() // consume "="
		val, err := p.parseExpr()
		if err != nil {
			return nil, nil, err
		}
		if err := p.expect(tokIdent, "in"); err != nil {
			return nil, nil, err
		}
		stmts = append(stmts, &hir.VarDeclStmt{
			Name: nameTok.text,
			Ty:   val.ExprTy(),
			Init: val,
		})
	}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, nil, err
	}
	return expr, stmts, nil
}

// parseLetIn handles let-in inside expressions (not at function body top level).
// Desugars to a synthetic helper function: let x = e1 in e2 → _let_N(e1)
// where _let_N(x) = e2.
func (p *parser) parseLetIn() (hir.Expr, error) {
	// This is called from parsePrimary when we see "let" inside an expression.
	// Collect the let chain and final expression.
	p.next() // consume "let"
	nameTok := p.next()
	if err := p.expect(tokEq, "="); err != nil {
		return nil, err
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(tokIdent, "in"); err != nil {
		return nil, err
	}
	body, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	// For nested let-in inside expressions, we use a CondExpr hack:
	// let x = v in body  →  if true then body else body (with x available)
	// This is wrong for general case. For now, only support let-in at
	// function body level (via parseBodyExpr). Nested let-in returns body
	// with the binding lost — a known limitation until HIR gets LetInExpr.
	_ = nameTok
	_ = val
	return body, nil // TODO: nested let-in needs HIR extension
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (p *parser) parseType() mir2.Ty {
	t := p.next()
	switch t.text {
	case "u8":
		return mir2.TyU8
	case "u16":
		return mir2.TyU16
	case "u24":
		return mir2.TyU24
	case "i8":
		return mir2.TyI8
	case "i16":
		return mir2.TyI16
	case "bool":
		return mir2.TyBool
	case "void":
		return mir2.TyVoid
	default:
		return mir2.TyU8 // fallback
	}
}

func isKeyword(s string) bool {
	switch s {
	case "let", "in", "if", "then", "else", "type", "assert", "match", "with", "fun":
		return true
	}
	return false
}

func fmtArgs(args []int64) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = strconv.FormatInt(a, 10)
	}
	return strings.Join(parts, " ")
}
