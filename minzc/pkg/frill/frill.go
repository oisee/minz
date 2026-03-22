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
	p := &parser{
		src: src, pos: 0, line: 1, name: name,
		adts: make(map[string]*adtDef), ctors: make(map[string]*adtCtor),
	}
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

// adtDef tracks an algebraic data type: constructors with optional payload type.
type adtDef struct {
	name         string
	constructors []adtCtor
}

type adtCtor struct {
	name    string
	tag     int64
	payload mir2.Ty // nil = no payload (like None)
}

type parser struct {
	src    string
	pos    int
	line   int
	name   string
	peeked *token
	adts        map[string]*adtDef  // type name → definition
	ctors       map[string]*adtCtor // constructor name → ctor (for match + expr)
	autoFuncs   []*hir.Func         // auto-generated helpers (__tag, __payload, lambdas)
	lambdaCount int                  // counter for unique lambda names
	strings     []string             // interned string literals
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

	// Number: decimal or 0x hex
	if ch >= '0' && ch <= '9' {
		start := p.pos
		if ch == '0' && p.pos+1 < len(p.src) && (p.src[p.pos+1] == 'x' || p.src[p.pos+1] == 'X') {
			p.pos += 2 // skip 0x
			for p.pos < len(p.src) && ((p.src[p.pos] >= '0' && p.src[p.pos] <= '9') ||
				(p.src[p.pos] >= 'a' && p.src[p.pos] <= 'f') ||
				(p.src[p.pos] >= 'A' && p.src[p.pos] <= 'F')) {
				p.pos++
			}
		} else {
			for p.pos < len(p.src) && (p.src[p.pos] >= '0' && p.src[p.pos] <= '9') {
				p.pos++
			}
		}
		return token{tokInt, p.src[start:p.pos], line}
	}

	// String literal
	if ch == '"' {
		p.pos++ // skip opening "
		start := p.pos
		for p.pos < len(p.src) && p.src[p.pos] != '"' {
			if p.src[p.pos] == '\\' {
				p.pos++ // skip escape
			}
			if p.pos < len(p.src) && p.src[p.pos] == '\n' {
				p.line++
			}
			p.pos++
		}
		text := p.src[start:p.pos]
		if p.pos < len(p.src) {
			p.pos++ // skip closing "
		}
		return token{tokString, text, line}
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
	case '+', '-', '*', '/', '%', '<', '>', '|':
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
			if err := p.parseTypeDecl(); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("line %d: unexpected %q at module level", t.line, t.text)
		}
	}

	// Append auto-generated helper functions (__tag, __payload)
	mod.Funcs = append(mod.Funcs, p.autoFuncs...)

	// Intern string literals as CStrings (null-terminated)
	for i, s := range p.strings {
		mod.Strings = append(mod.Strings, s)
		mod.StrKinds = append(mod.StrKinds, mir2.StrCString)
		_ = i // sym __str_N used by AddrOfExpr
	}

	return mod, nil
}

// parseTypeDecl: type Name = Ctor1 | Ctor2 of ty | Ctor3
// Registers constructors in p.ctors for use in match arms and expressions.
func (p *parser) parseTypeDecl() error {
	p.next() // consume "type"
	nameTok := p.next()
	if nameTok.kind != tokIdent {
		return fmt.Errorf("line %d: expected type name", nameTok.line)
	}
	if err := p.expect(tokEq, "="); err != nil {
		return err
	}

	def := &adtDef{name: nameTok.text}
	var tag int64

	for {
		ctorTok := p.next()
		if ctorTok.kind != tokIdent {
			return fmt.Errorf("line %d: expected constructor name, got %q", ctorTok.line, ctorTok.text)
		}

		ctor := adtCtor{name: ctorTok.text, tag: tag}

		// Optional "of type"
		if p.peek().kind == tokIdent && p.peek().text == "of" {
			p.next() // consume "of"
			ctor.payload = p.parseType()
		}

		def.constructors = append(def.constructors, ctor)
		p.ctors[ctor.name] = &def.constructors[len(def.constructors)-1]
		tag++

		// | separates constructors
		if p.peek().kind == tokOp && p.peek().text == "|" {
			p.next()
			continue
		}
		break
	}

	p.adts[def.name] = def

	// If any constructor has payload, generate __tag and __payload helpers.
	hasPayload := false
	for _, c := range def.constructors {
		if c.payload != nil {
			hasPayload = true
			break
		}
	}
	if hasPayload {
		p.autoFuncs = append(p.autoFuncs,
			// __tag(x : u16) : u8 = x / 256
			&hir.Func{
				Name:   "__tag",
				Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}},
				RetTy:  mir2.TyU8,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.ReturnStmt{Val: &hir.CastExpr{
						X:  &hir.BinExpr{Op: "/", L: &hir.VarRefExpr{Name: "x", Ty: mir2.TyU16}, R: &hir.IntLitExpr{Val: 256, Ty: mir2.TyU16}, Ty: mir2.TyU16},
						Ty: mir2.TyU8,
					}},
				}},
			},
			// __payload(x : u16) : u8 = x % 256
			&hir.Func{
				Name:   "__payload",
				Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}},
				RetTy:  mir2.TyU8,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.ReturnStmt{Val: &hir.CastExpr{
						X:  &hir.BinExpr{Op: "%", L: &hir.VarRefExpr{Name: "x", Ty: mir2.TyU16}, R: &hir.IntLitExpr{Val: 256, Ty: mir2.TyU16}, Ty: mir2.TyU16},
						Ty: mir2.TyU8,
					}},
				}},
			},
		)
	}

	return nil
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

	// Return type: : type (optional — inferred from body if omitted)
	inferRetTy := false
	if p.peek().kind == tokColon {
		p.next() // :
		fn.RetTy = p.parseType()
	} else {
		inferRetTy = true
		fn.RetTy = mir2.TyU8 // placeholder, updated after parsing body
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

	// where-clauses: where name = expr (Haskell-style, definitions after body)
	var whereStmts []hir.Stmt
	for p.peek().kind == tokIdent && p.peek().text == "where" {
		p.next() // consume "where"
		wname := p.next()
		if err := p.expect(tokEq, "="); err != nil {
			return nil, err
		}
		wval, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		whereStmts = append(whereStmts, &hir.VarDeclStmt{
			Name: wname.text, Ty: wval.ExprTy(), Init: wval,
		})
	}

	// Infer return type from body expression
	if inferRetTy && body != nil {
		fn.RetTy = body.ExprTy()
	}

	// where-bindings come BEFORE the let-in bindings and body
	stmts = append(stmts, whereStmts...)
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

	// Parse args (int literals or constructor names) until ==
	var args []int64
	for p.peek().kind == tokInt || (p.peek().kind == tokIdent && !isKeyword(p.peek().text)) {
		tok := p.next()
		if tok.kind == tokInt {
			val, _ := strconv.ParseInt(tok.text, 0, 64)
			args = append(args, val)
		} else if ctor, ok := p.ctors[tok.text]; ok {
			args = append(args, ctor.tag)
		} else {
			return hir.Assert{}, fmt.Errorf("line %d: assert: unknown arg %q", line, tok.text)
		}
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

		// Lambda: |param| body
		if p.peek().kind == tokOp && p.peek().text == "|" {
			p.next() // consume |
			paramTok := p.next()
			if err := p.expect(tokOp, "|"); err != nil {
				return nil, fmt.Errorf("line %d: expected | after lambda param", paramTok.line)
			}
			body, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			// Desugar: generate anonymous function, call it with left
			lambdaName := fmt.Sprintf("__lambda_%d", p.lambdaCount)
			p.lambdaCount++
			p.autoFuncs = append(p.autoFuncs, &hir.Func{
				Name:   lambdaName,
				Params: []hir.Param{{Name: paramTok.text, Ty: left.ExprTy()}},
				RetTy:  body.ExprTy(),
				Body:   &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: body}}},
			})
			left = &hir.CallExpr{Fn: lambdaName, Args: []hir.Expr{left}, Ty: body.ExprTy()}
			continue
		}

		// Operator section in pipe: |> (+1) or |> (*2)
		if p.peek().kind == tokLParen {
			section, err := p.parsePrimary() // parsePrimary handles (op arg) → __section_N
			if err != nil {
				return nil, err
			}
			// section is VarRefExpr pointing to __section_N
			if vr, ok := section.(*hir.VarRefExpr); ok {
				left = &hir.CallExpr{Fn: vr.Name, Args: []hir.Expr{left}, Ty: left.ExprTy()}
			}
			continue
		}

		// Named function: fn arg1 arg2 ...
		fnTok := p.next()
		if fnTok.kind != tokIdent {
			return nil, fmt.Errorf("line %d: expected function name after |>, got %q", fnTok.line, fnTok.text)
		}
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
	if p.peek().kind == tokOp && p.peek().text == "-" {
		p.next()
		x, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &hir.BinExpr{Op: "-", L: &hir.IntLitExpr{Val: 0, Ty: x.ExprTy()}, R: x, Ty: x.ExprTy()}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (hir.Expr, error) {
	t := p.peek()

	// Parenthesized expression or operator section
	if t.kind == tokLParen {
		p.next()
		// Operator section: (+1) → lambda |__x| __x + 1
		if p.peek().kind == tokOp {
			op := p.next().text
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expect(tokRParen, ")"); err != nil {
				return nil, err
			}
			// Generate lambda: __section_N(x) = x op arg
			paramName := fmt.Sprintf("__s%d", p.lambdaCount)
			lambdaName := fmt.Sprintf("__section_%d", p.lambdaCount)
			p.lambdaCount++
			body := &hir.BinExpr{Op: op, L: &hir.VarRefExpr{Name: paramName, Ty: arg.ExprTy()}, R: arg, Ty: arg.ExprTy()}
			p.autoFuncs = append(p.autoFuncs, &hir.Func{
				Name:   lambdaName,
				Params: []hir.Param{{Name: paramName, Ty: arg.ExprTy()}},
				RetTy:  arg.ExprTy(),
				Body:   &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: body}}},
			})
			return &hir.VarRefExpr{Name: lambdaName, Ty: mir2.TyU8}, nil
		}
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(tokRParen, ")"); err != nil {
			return nil, err
		}
		return e, nil
	}

	// Bool literals
	if t.kind == tokIdent && t.text == "true" {
		p.next()
		return &hir.BoolLitExpr{Val: true}, nil
	}
	if t.kind == tokIdent && t.text == "false" {
		p.next()
		return &hir.BoolLitExpr{Val: false}, nil
	}

	// String literal — intern and return pointer
	if t.kind == tokString {
		p.next()
		// Process escape sequences
		s := unescapeString(t.text)
		// Store string index for later — module will be populated in parseModule
		idx := len(p.strings)
		p.strings = append(p.strings, s)
		sym := fmt.Sprintf("__str_%d", idx)
		return &hir.AddrOfExpr{Sym: sym}, nil
	}

	// Integer literal
	if t.kind == tokInt {
		p.next()
		val, _ := strconv.ParseInt(t.text, 0, 64)
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

	// match expression
	if t.kind == tokIdent && t.text == "match" {
		return p.parseMatch()
	}

	// let-in expression
	if t.kind == tokIdent && t.text == "let" {
		return p.parseLetIn()
	}

	// ADT constructor (e.g. None, Some 42)
	if t.kind == tokIdent {
		if ctor, ok := p.ctors[t.text]; ok {
			p.next()
			if ctor.payload != nil {
				// Constructor with payload: encode as u16 = (tag << 8) | payload
				// e.g. Some 42 → 0x012A = 298
				arg, err := p.parsePrimary()
				if err != nil {
					return nil, err
				}
				tagExpr := &hir.IntLitExpr{Val: ctor.tag * 256, Ty: mir2.TyU16}
				return &hir.BinExpr{Op: "+", L: tagExpr, R: &hir.CastExpr{X: arg, Ty: mir2.TyU16}, Ty: mir2.TyU16}, nil
			}
			// No payload: just the tag
			return &hir.IntLitExpr{Val: ctor.tag, Ty: mir2.TyU8}, nil
		}
	}

	// Identifier (variable ref or function call)
	if t.kind == tokIdent {
		p.next()
		name := t.text

		// Collect call arguments: atoms only (int literals, plain idents, parens).
		// Idents are treated as variable refs, NOT nested function calls.
		var args []hir.Expr
		for {
			pk := p.peek()
			if pk.kind == tokInt {
				p.next()
				val, _ := strconv.ParseInt(pk.text, 10, 64)
				ty := mir2.TyU8
				if val > 255 { ty = mir2.TyU16 }
				args = append(args, &hir.IntLitExpr{Val: val, Ty: ty})
			} else if pk.kind == tokString {
				p.next()
				s := unescapeString(pk.text)
				idx := len(p.strings)
				p.strings = append(p.strings, s)
				sym := fmt.Sprintf("__str_%d", idx)
				args = append(args, &hir.AddrOfExpr{Sym: sym})
			} else if pk.kind == tokLParen {
				p.next()
				e, err := p.parseExpr()
				if err != nil { return nil, err }
				if err := p.expect(tokRParen, ")"); err != nil { return nil, err }
				args = append(args, e)
			} else if pk.kind == tokIdent && !isKeyword(pk.text) {
				// Check if it's a constructor
				if ctor, ok := p.ctors[pk.text]; ok {
					p.next()
					args = append(args, &hir.IntLitExpr{Val: ctor.tag, Ty: mir2.TyU8})
				} else {
					// Plain variable ref — don't recurse into function call
					p.next()
					args = append(args, &hir.VarRefExpr{Name: pk.text, Ty: mir2.TyU8})
				}
			} else {
				break
			}
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

// parseMatch: match expr with | pat -> expr | pat -> expr | _ -> expr end
// Desugars to nested CondExpr (if-then-else chain).
//
// Syntax:
//   match x with
//   | 0 -> expr0
//   | 1 -> expr1
//   | _ -> default
//   end
func (p *parser) parseMatch() (hir.Expr, error) {
	p.next() // consume "match"
	scrutinee, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(tokIdent, "with"); err != nil {
		return nil, err
	}

	type arm struct {
		isDefault bool
		val       int64
		guard     hir.Expr // nil = no guard; non-nil = extra condition
		body      hir.Expr
		bindName  string // variable name bound to scrutinee (for guards)
	}
	var arms []arm

	// Parse arms: | pattern [when guard] -> body
	for p.peek().kind == tokOp && p.peek().text == "|" {
		p.next() // consume |
		tok := p.peek()
		isDefault := false
		var val int64
		var bindName string
		if tok.kind == tokIdent && tok.text == "_" {
			p.next()
			isDefault = true
		} else if tok.kind == tokInt {
			p.next()
			val, _ = strconv.ParseInt(tok.text, 10, 64)
		} else if tok.kind == tokIdent {
			p.next()
			if ctor, ok := p.ctors[tok.text]; ok {
				// Named constructor
				val = ctor.tag
			} else {
				// Variable binding: | n when n > 10 -> ...
				bindName = tok.text
				isDefault = true
			}
		} else {
			return nil, fmt.Errorf("line %d: expected pattern, got %q", tok.line, tok.text)
		}

		// Optional guard: when <expr>
		var guard hir.Expr
		if p.peek().kind == tokIdent && p.peek().text == "when" {
			p.next() // consume "when"
			guard, err = p.parseComparison()
			if err != nil {
				return nil, err
			}
		}

		if err := p.expect(tokArrow, "->"); err != nil {
			return nil, err
		}

		body, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		arms = append(arms, arm{isDefault: isDefault, val: val, guard: guard, body: body, bindName: bindName})
	}

	// Optional "end" keyword
	if p.peek().kind == tokIdent && p.peek().text == "end" {
		p.next()
	}

	if len(arms) == 0 {
		return nil, fmt.Errorf("match with no arms")
	}

	// Exhaustiveness check: if arms reference ADT constructors, verify all covered.
	hasDefault := false
	ctorNames := map[string]bool{}
	for _, a := range arms {
		if a.isDefault {
			hasDefault = true
		}
	}
	if !hasDefault {
		// Collect referenced constructor names from arms
		for _, a := range arms {
			if !a.isDefault {
				for name, ctor := range p.ctors {
					if ctor.tag == a.val {
						ctorNames[name] = true
					}
				}
			}
		}
		// Find which ADT these belong to
		for _, def := range p.adts {
			matchesThisADT := false
			for _, c := range def.constructors {
				if ctorNames[c.name] {
					matchesThisADT = true
					break
				}
			}
			if matchesThisADT {
				var missing []string
				for _, c := range def.constructors {
					if !ctorNames[c.name] {
						missing = append(missing, c.name)
					}
				}
				if len(missing) > 0 {
					return nil, fmt.Errorf("line %d: non-exhaustive match on %s: missing %s",
						p.line, def.name, strings.Join(missing, ", "))
				}
			}
		}
	}

	// Build nested CondExpr from bottom up.
	// Default arm (if any) becomes the innermost else.
	// Guards add an extra condition: (val == pat) && guard
	var result hir.Expr
	for i := len(arms) - 1; i >= 0; i-- {
		a := arms[i]
		if a.isDefault && a.guard == nil {
			result = a.body
		} else if a.isDefault && a.guard != nil {
			// Variable binding with guard: | n when n > 10 -> body
			// Condition is just the guard (pattern matches everything)
			elseE := result
			if elseE == nil {
				elseE = &hir.IntLitExpr{Val: 0, Ty: scrutinee.ExprTy()}
			}
			result = &hir.CondExpr{Cond: a.guard, Then: a.body, Else: elseE, Ty: a.body.ExprTy()}
		} else {
			cond := &hir.BinExpr{
				Op: "==",
				L:  scrutinee,
				R:  &hir.IntLitExpr{Val: a.val, Ty: scrutinee.ExprTy()},
				Ty: mir2.TyBool,
			}
			if a.guard != nil {
				// Combine: (scrutinee == val) && guard
				cond = &hir.BinExpr{Op: "&&", L: cond, R: a.guard, Ty: mir2.TyBool}
			}
			elseE := result
			if elseE == nil {
				elseE = &hir.IntLitExpr{Val: 0, Ty: scrutinee.ExprTy()}
			}
			result = &hir.CondExpr{Cond: cond, Then: a.body, Else: elseE, Ty: a.body.ExprTy()}
		}
	}

	return result, nil
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
	case "let", "in", "if", "then", "else", "type", "assert", "match", "with", "fun", "end", "where", "when":
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

func unescapeString(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case '0':
				b.WriteByte(0)
			default:
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
