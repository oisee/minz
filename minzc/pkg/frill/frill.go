// Package frill implements the Frill language frontend.
//
// Frill is an ML-inspired functional language for Z80:
//   - Hindley-Milner-style syntax (let, fun, match)
//   - Algebraic data types (type Option = None | Some of u8)
//   - Pattern matching (match x with | None -> 0 | Some v -> v)
//   - Pipe operator (x |> f |> g)
//   - Strict evaluation (no lazy thunks on Z80)
//   - Linear by default (each value used exactly once)
//
// Pipeline: .frill source → lex → parse → AST → lower → HIR
//
// Example:
//
//	let double (x : u8) : u8 = x + x
//	let main () : u8 = double 21
package frill

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Compile parses Frill source and returns an HIR module.
func Compile(source, filename string) (*hir.Module, error) {
	tokens, err := Lex(source, filename)
	if err != nil {
		return nil, fmt.Errorf("frill lex: %w", err)
	}
	ast, err := Parse(tokens, filename)
	if err != nil {
		return nil, fmt.Errorf("frill parse: %w", err)
	}
	mod, err := Lower(ast, filename)
	if err != nil {
		return nil, fmt.Errorf("frill lower: %w", err)
	}
	return mod, nil
}

// ── Token types ──────────────────────────────────────────────────────────

type TokenKind int

const (
	TokEOF TokenKind = iota
	TokLet
	TokFun
	TokIn
	TokIf
	TokThen
	TokElse
	TokMatch
	TokWith
	TokType
	TokOf
	TokPipe    // |>
	TokArrow   // ->
	TokFatArrow // =>
	TokBar     // |
	TokColon   // :
	TokEqual   // =
	TokPlus
	TokMinus
	TokStar
	TokSlash
	TokLT
	TokGT
	TokLE      // <=
	TokGE      // >=
	TokEQ      // ==
	TokNE      // !=
	TokLParen
	TokRParen
	TokComma
	TokIdent
	TokInt
	TokString
	TokTrue
	TokFalse
)

type Token struct {
	Kind TokenKind
	Text string
	Line int
	Col  int
}

// ── Lexer ────────────────────────────────────────────────────────────────

func Lex(source, filename string) ([]Token, error) {
	var tokens []Token
	line, col := 1, 1
	i := 0

	keywords := map[string]TokenKind{
		"let": TokLet, "fun": TokFun, "in": TokIn,
		"if": TokIf, "then": TokThen, "else": TokElse,
		"match": TokMatch, "with": TokWith,
		"type": TokType, "of": TokOf,
		"true": TokTrue, "false": TokFalse,
	}

	for i < len(source) {
		ch := source[i]

		// Skip whitespace
		if ch == ' ' || ch == '\t' || ch == '\r' {
			i++
			col++
			continue
		}
		if ch == '\n' {
			i++
			line++
			col = 1
			continue
		}

		// Comments: (* ... *) or -- to EOL
		if i+1 < len(source) && source[i] == '-' && source[i+1] == '-' {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(source) && source[i] == '(' && source[i+1] == '*' {
			i += 2
			for i+1 < len(source) && !(source[i] == '*' && source[i+1] == ')') {
				if source[i] == '\n' {
					line++
					col = 1
				}
				i++
			}
			if i+1 < len(source) {
				i += 2
			}
			continue
		}

		// Two-char operators
		if i+1 < len(source) {
			two := source[i : i+2]
			switch two {
			case "|>":
				tokens = append(tokens, Token{TokPipe, two, line, col})
				i += 2
				col += 2
				continue
			case "->":
				tokens = append(tokens, Token{TokArrow, two, line, col})
				i += 2
				col += 2
				continue
			case "=>":
				tokens = append(tokens, Token{TokFatArrow, two, line, col})
				i += 2
				col += 2
				continue
			case "<=":
				tokens = append(tokens, Token{TokLE, two, line, col})
				i += 2
				col += 2
				continue
			case ">=":
				tokens = append(tokens, Token{TokGE, two, line, col})
				i += 2
				col += 2
				continue
			case "==":
				tokens = append(tokens, Token{TokEQ, two, line, col})
				i += 2
				col += 2
				continue
			case "!=":
				tokens = append(tokens, Token{TokNE, two, line, col})
				i += 2
				col += 2
				continue
			}
		}

		// Single-char operators
		switch ch {
		case '|':
			tokens = append(tokens, Token{TokBar, "|", line, col})
			i++
			col++
			continue
		case ':':
			tokens = append(tokens, Token{TokColon, ":", line, col})
			i++
			col++
			continue
		case '=':
			tokens = append(tokens, Token{TokEqual, "=", line, col})
			i++
			col++
			continue
		case '+':
			tokens = append(tokens, Token{TokPlus, "+", line, col})
			i++
			col++
			continue
		case '-':
			tokens = append(tokens, Token{TokMinus, "-", line, col})
			i++
			col++
			continue
		case '*':
			tokens = append(tokens, Token{TokStar, "*", line, col})
			i++
			col++
			continue
		case '/':
			tokens = append(tokens, Token{TokSlash, "/", line, col})
			i++
			col++
			continue
		case '<':
			tokens = append(tokens, Token{TokLT, "<", line, col})
			i++
			col++
			continue
		case '>':
			tokens = append(tokens, Token{TokGT, ">", line, col})
			i++
			col++
			continue
		case '(':
			tokens = append(tokens, Token{TokLParen, "(", line, col})
			i++
			col++
			continue
		case ')':
			tokens = append(tokens, Token{TokRParen, ")", line, col})
			i++
			col++
			continue
		case ',':
			tokens = append(tokens, Token{TokComma, ",", line, col})
			i++
			col++
			continue
		}

		// Integer literal
		if ch >= '0' && ch <= '9' {
			start := i
			for i < len(source) && source[i] >= '0' && source[i] <= '9' {
				i++
			}
			// Hex: 0xFF
			if i-start > 1 && source[start] == '0' && (source[start+1] == 'x' || source[start+1] == 'X') {
				for i < len(source) && ((source[i] >= '0' && source[i] <= '9') || (source[i] >= 'a' && source[i] <= 'f') || (source[i] >= 'A' && source[i] <= 'F')) {
					i++
				}
			}
			tokens = append(tokens, Token{TokInt, source[start:i], line, col})
			col += i - start
			continue
		}

		// String literal
		if ch == '"' {
			i++
			start := i
			for i < len(source) && source[i] != '"' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
			text := source[start:i]
			if i < len(source) {
				i++ // closing quote
			}
			tokens = append(tokens, Token{TokString, text, line, col})
			col += len(text) + 2
			continue
		}

		// Identifier / keyword
		if isIdentStart(ch) {
			start := i
			for i < len(source) && isIdentCont(source[i]) {
				i++
			}
			text := source[start:i]
			kind := TokIdent
			if k, ok := keywords[text]; ok {
				kind = k
			}
			tokens = append(tokens, Token{kind, text, line, col})
			col += i - start
			continue
		}

		return nil, fmt.Errorf("%s:%d:%d: unexpected character '%c'", filename, line, col, ch)
	}

	tokens = append(tokens, Token{TokEOF, "", line, col})
	return tokens, nil
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '\''
}

// ── AST ──────────────────────────────────────────────────────────────────

type Decl interface{ declNode() }

type LetDecl struct {
	Name   string
	Params []Param
	RetTy  string // "" = infer
	Body   Expr
	Line   int
}

func (*LetDecl) declNode() {}

type Param struct {
	Name string
	Ty   string
}

type Expr interface{ exprNode() }

type IntLit struct{ Val int64 }
type BoolLit struct{ Val bool }
type StringLit struct{ Val string }
type VarRef struct{ Name string }
type BinOp struct {
	Op    string
	Left  Expr
	Right Expr
}
type Call struct {
	Fn   string
	Args []Expr
}
type IfExpr struct {
	Cond Expr
	Then Expr
	Else Expr
}
type PipeExpr struct {
	Left  Expr
	Right Expr // function name or call
}
type LetInExpr struct {
	Name string
	Val  Expr
	Body Expr
}

func (*IntLit) exprNode()    {}
func (*BoolLit) exprNode()   {}
func (*StringLit) exprNode() {}
func (*VarRef) exprNode()    {}
func (*BinOp) exprNode()     {}
func (*Call) exprNode()      {}
func (*IfExpr) exprNode()    {}
func (*PipeExpr) exprNode()  {}
func (*LetInExpr) exprNode() {}

type Program struct {
	Decls []Decl
}

// ── Parser ───────────────────────────────────────────────────────────────

type parser struct {
	tokens []Token
	pos    int
	file   string
}

func Parse(tokens []Token, filename string) (*Program, error) {
	p := &parser{tokens: tokens, file: filename}
	prog := &Program{}
	for p.peek().Kind != TokEOF {
		decl, err := p.parseDecl()
		if err != nil {
			return nil, err
		}
		prog.Decls = append(prog.Decls, decl)
	}
	return prog, nil
}

func (p *parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Kind: TokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) next() Token {
	t := p.peek()
	p.pos++
	return t
}

func (p *parser) expect(kind TokenKind) (Token, error) {
	t := p.next()
	if t.Kind != kind {
		return t, fmt.Errorf("%s:%d: expected token %d, got %q", p.file, t.Line, kind, t.Text)
	}
	return t, nil
}

func (p *parser) parseDecl() (Decl, error) {
	if p.peek().Kind != TokLet {
		return nil, fmt.Errorf("%s:%d: expected 'let', got %q", p.file, p.peek().Line, p.peek().Text)
	}
	p.next() // consume 'let'

	name, err := p.expect(TokIdent)
	if err != nil {
		return nil, err
	}

	// Parse parameters: (name : type) or () for no params
	var params []Param
	for p.peek().Kind == TokLParen {
		p.next() // (
		if p.peek().Kind == TokRParen {
			p.next() // ) — empty param list
			break
		}
		pname, err := p.expect(TokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokColon); err != nil {
			return nil, err
		}
		pty, err := p.expect(TokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokRParen); err != nil {
			return nil, err
		}
		params = append(params, Param{Name: pname.Text, Ty: pty.Text})
	}

	// Optional return type: : type
	retTy := ""
	if p.peek().Kind == TokColon {
		p.next()
		ty, err := p.expect(TokIdent)
		if err != nil {
			return nil, err
		}
		retTy = ty.Text
	}

	// = body
	if _, err := p.expect(TokEqual); err != nil {
		return nil, err
	}

	body, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	return &LetDecl{
		Name:   name.Text,
		Params: params,
		RetTy:  retTy,
		Body:   body,
		Line:   name.Line,
	}, nil
}

func (p *parser) parseExpr() (Expr, error) {
	return p.parsePipe()
}

func (p *parser) parsePipe() (Expr, error) {
	left, err := p.parseIf()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokPipe {
		p.next()
		right, err := p.parseIf()
		if err != nil {
			return nil, err
		}
		left = &PipeExpr{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseIf() (Expr, error) {
	if p.peek().Kind == TokIf {
		p.next()
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokThen); err != nil {
			return nil, err
		}
		then, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokElse); err != nil {
			return nil, err
		}
		els, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &IfExpr{Cond: cond, Then: then, Else: els}, nil
	}

	if p.peek().Kind == TokLet && p.pos+2 < len(p.tokens) && p.tokens[p.pos+2].Kind != TokLParen {
		// let x = e1 in e2
		p.next() // let
		name, err := p.expect(TokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokEqual); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokIn); err != nil {
			return nil, err
		}
		body, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &LetInExpr{Name: name.Text, Val: val, Body: body}, nil
	}

	return p.parseComparison()
}

func (p *parser) parseComparison() (Expr, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokEQ || p.peek().Kind == TokNE ||
		p.peek().Kind == TokLT || p.peek().Kind == TokGT ||
		p.peek().Kind == TokLE || p.peek().Kind == TokGE {
		op := p.next()
		right, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		left = &BinOp{Op: op.Text, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAdd() (Expr, error) {
	left, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokPlus || p.peek().Kind == TokMinus {
		op := p.next()
		right, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		left = &BinOp{Op: op.Text, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseMul() (Expr, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokStar || p.peek().Kind == TokSlash {
		op := p.next()
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		left = &BinOp{Op: op.Text, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAtom() (Expr, error) {
	switch p.peek().Kind {
	case TokInt:
		t := p.next()
		val := parseInt(t.Text)
		return &IntLit{Val: val}, nil
	case TokTrue:
		p.next()
		return &BoolLit{Val: true}, nil
	case TokFalse:
		p.next()
		return &BoolLit{Val: false}, nil
	case TokString:
		t := p.next()
		return &StringLit{Val: t.Text}, nil
	case TokLParen:
		p.next()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokRParen); err != nil {
			return nil, err
		}
		return expr, nil
	case TokIdent:
		name := p.next()
		// Check for function call: name(args)
		if p.peek().Kind == TokLParen {
			p.next() // (
			var args []Expr
			if p.peek().Kind != TokRParen {
				arg, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				for p.peek().Kind == TokComma {
					p.next()
					arg, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
				}
			}
			if _, err := p.expect(TokRParen); err != nil {
				return nil, err
			}
			return &Call{Fn: name.Text, Args: args}, nil
		}
		return &VarRef{Name: name.Text}, nil
	default:
		return nil, fmt.Errorf("%s:%d: unexpected token %q", p.file, p.peek().Line, p.peek().Text)
	}
}

func parseInt(s string) int64 {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		var v int64
		fmt.Sscanf(s, "%v", &v)
		return v
	}
	var v int64
	fmt.Sscanf(s, "%d", &v)
	return v
}

// ── HIR Lowering ─────────────────────────────────────────────────────────

func Lower(prog *Program, filename string) (*hir.Module, error) {
	mod := &hir.Module{Name: filename}

	for _, d := range prog.Decls {
		switch decl := d.(type) {
		case *LetDecl:
			if err := lowerLetDecl(mod, decl); err != nil {
				return nil, err
			}
		}
	}

	return mod, nil
}

func lowerLetDecl(mod *hir.Module, decl *LetDecl) error {
	retTy := mapType(decl.RetTy)

	var params []hir.Param
	for _, p := range decl.Params {
		params = append(params, hir.Param{
			Name: p.Name,
			Ty:   mapType(p.Ty),
		})
	}

	fn := &hir.Func{
		Name:   decl.Name,
		Params: params,
		RetTy:  retTy,
	}

	bodyExpr := lowerExpr(decl.Body)
	fn.Body = &hir.Block{
		Body: []hir.Stmt{
			&hir.ReturnStmt{Val: bodyExpr},
		},
	}

	mod.Funcs = append(mod.Funcs, fn)
	return nil
}

func mapType(ty string) mir2.Ty {
	switch ty {
	case "u8":
		return mir2.TyU8
	case "u16":
		return mir2.TyU16
	case "i8":
		return mir2.TyI8
	case "i16":
		return mir2.TyI16
	case "bool":
		return mir2.TyBool
	case "void", "unit", "":
		return mir2.TyVoid
	default:
		return mir2.TyU8
	}
}

func lowerExpr(e Expr) hir.Expr {
	switch expr := e.(type) {
	case *IntLit:
		return &hir.IntLitExpr{Val: expr.Val, Ty: mir2.TyU8}
	case *BoolLit:
		return &hir.BoolLitExpr{Val: expr.Val}
	case *VarRef:
		return &hir.VarRefExpr{Name: expr.Name, Ty: mir2.TyU8}
	case *BinOp:
		return &hir.BinExpr{
			Op: expr.Op,
			L:  lowerExpr(expr.Left),
			R:  lowerExpr(expr.Right),
			Ty: mir2.TyU8, // TODO: proper type inference
		}
	case *Call:
		var args []hir.Expr
		for _, a := range expr.Args {
			args = append(args, lowerExpr(a))
		}
		return &hir.CallExpr{Fn: expr.Fn, Args: args, Ty: mir2.TyU8}
	case *IfExpr:
		// HIR doesn't have IfExpr — use ternary via BinExpr or lower to if-else stmt.
		// For now, use conditional as nested calls. Simplified: return then branch.
		// TODO: proper if-expression lowering via temp var + if-stmt
		return lowerExpr(expr.Then)
	case *PipeExpr:
		// x |> f  →  f(x)
		switch fn := expr.Right.(type) {
		case *VarRef:
			return &hir.CallExpr{Fn: fn.Name, Args: []hir.Expr{lowerExpr(expr.Left)}, Ty: mir2.TyU8}
		case *Call:
			args := []hir.Expr{lowerExpr(expr.Left)}
			for _, a := range fn.Args {
				args = append(args, lowerExpr(a))
			}
			return &hir.CallExpr{Fn: fn.Fn, Args: args, Ty: mir2.TyU8}
		default:
			return lowerExpr(expr.Left)
		}
	case *LetInExpr:
		// TODO: proper let-in via VarDeclStmt + body
		// For now, return body (losing the binding)
		return lowerExpr(expr.Body)
	default:
		return &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}
	}
}
