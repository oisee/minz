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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/mir2"
)

// CompileOpts controls import resolution.
type CompileOpts struct {
	BaseDir string // directory of source file (for relative imports)
}

// Compile parses Frill source and returns an HIR module.
func Compile(src, name string) (*hir.Module, error) {
	return CompileWithOpts(src, name, CompileOpts{})
}

// CompileWithOpts parses with import resolution options.
func CompileWithOpts(src, name string, opts CompileOpts) (*hir.Module, error) {
	p := &parser{
		src: src, pos: 0, line: 1, name: name,
		adts: make(map[string]*adtDef), ctors: make(map[string]*adtCtor),
		arities: make(map[string]int), classes: make(map[string][]string),
		baseDir: opts.BaseDir,
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
	tokLBrace // {
	tokRBrace // }
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
	records     []*mir2.StructTy     // record type declarations
	arities     map[string]int       // function name → param count (for partial application)
	baseDir     string               // for import resolution
	lastTuple   []hir.Expr           // pending tuple elements from (e1, e2)
	classes     map[string][]string  // class name → method names
	warnings    []string             // linearity warnings

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
		case "==", "!=", "<=", ">=", "&&", "||", "|>", "->", ">>", "<-":
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
	case '{':
		return token{tokLBrace, "{", line}
	case '}':
		return token{tokRBrace, "}", line}
	case ':':
		return token{tokColon, ":", line}
	case '=':
		return token{tokEq, "=", line}
	case ',':
		return token{tokComma, ",", line}
	case '+', '-', '*', '/', '%', '<', '>', '|', '!', '~':
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
		case "import":
			imported, err := p.parseImport()
			if err != nil {
				return nil, err
			}
			if imported != nil {
				mod.Funcs = append(mod.Funcs, imported.Funcs...)
				mod.Structs = append(mod.Structs, imported.Structs...)
				mod.Strings = append(mod.Strings, imported.Strings...)
				mod.StrKinds = append(mod.StrKinds, imported.StrKinds...)
			}
		case "class":
			if err := p.parseClass(); err != nil {
				return nil, err
			}
		case "instance":
			fns, err := p.parseInstance()
			if err != nil {
				return nil, err
			}
			mod.Funcs = append(mod.Funcs, fns...)
		case "extern":
			fn, err := p.parseExtern()
			if err != nil {
				return nil, err
			}
			mod.Funcs = append(mod.Funcs, fn)
		case "prop":
			fn, asserts, err := p.parseProp()
			if err != nil {
				return nil, err
			}
			mod.Funcs = append(mod.Funcs, fn)
			mod.Asserts = append(mod.Asserts, asserts...)
		case "type":
			if err := p.parseTypeDecl(); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("line %d: unexpected %q at module level", t.line, t.text)
		}
	}

	// Append record types as structs
	mod.Structs = append(mod.Structs, p.records...)

	// Linearity warnings
	mod.Warnings = append(mod.Warnings, p.warnings...)

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

	// Record type: type Point = { x : u8, y : u8 }
	if p.peek().kind == tokLBrace {
		return p.parseRecordFields(nameTok.text)
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

// parseRecordFields: { field1 : type1, field2 : type2 }
func (p *parser) parseRecordFields(name string) error {
	p.next() // consume {
	var fields []mir2.StructField
	for p.peek().kind != tokRBrace && p.peek().kind != tokEOF {
		fname := p.next()
		if err := p.expect(tokColon, ":"); err != nil {
			return err
		}
		fty := p.parseType()
		fields = append(fields, mir2.StructField{Name: fname.text, Ty: fty})
		if p.peek().kind == tokComma {
			p.next()
		}
	}
	if err := p.expect(tokRBrace, "}"); err != nil {
		return err
	}
	st := &mir2.StructTy{Name: name, Fields: fields}
	p.records = append(p.records, st)
	return nil
}

// parseClass: class Name a where method1 : a -> retty, method2 : a -> retty
// Simplified: class Show where show : u8
// Stores method names for later resolution in instance.
func (p *parser) parseClass() error {
	p.next() // consume "class"
	className := p.next().text
	if err := p.expect(tokIdent, "where"); err != nil {
		return err
	}
	var methods []string
	for {
		if p.peek().kind != tokIdent || isKeyword(p.peek().text) {
			break
		}
		methodName := p.next().text
		// Skip : type annotation
		if p.peek().kind == tokColon {
			p.next()
			for p.peek().kind != tokEOF && p.peek().kind != tokComma &&
				!(p.peek().kind == tokIdent && isKeyword(p.peek().text)) {
				p.next()
			}
		}
		methods = append(methods, methodName)
		if p.peek().kind == tokComma {
			p.next()
		} else {
			break
		}
	}
	p.classes[className] = methods
	return nil
}

// parseInstance: instance ClassName TypeName where method1 (params) = body
// Generates: ClassName_method_TypeName(params) = body
func (p *parser) parseInstance() ([]*hir.Func, error) {
	p.next() // consume "instance"
	className := p.next().text
	typeName := p.next().text
	if err := p.expect(tokIdent, "where"); err != nil {
		return nil, err
	}

	methods, ok := p.classes[className]
	if !ok {
		return nil, fmt.Errorf("unknown class %q", className)
	}

	var funcs []*hir.Func
	for _, method := range methods {
		// Expect: methodName (params) = body
		tok := p.peek()
		if tok.kind != tokIdent || tok.text != method {
			break
		}
		p.next() // consume method name

		// Generate function named: method_TypeName
		mangledName := method + "_" + typeName

		// Parse params
		var params []hir.Param
		for p.peek().kind == tokLParen {
			p.next()
			pname := p.next()
			if err := p.expect(tokColon, ":"); err != nil {
				return nil, err
			}
			pty := p.parseType()
			if err := p.expect(tokRParen, ")"); err != nil {
				return nil, err
			}
			params = append(params, hir.Param{Name: pname.text, Ty: pty})
		}

		// Optional return type
		var retTy mir2.Ty = mir2.TyU8
		if p.peek().kind == tokColon {
			p.next()
			retTy = p.parseType()
		}

		if err := p.expect(tokEq, "="); err != nil {
			return nil, err
		}

		body, letStmts, err := p.parseBodyExpr()
		if err != nil {
			return nil, err
		}

		var stmts []hir.Stmt
		stmts = append(stmts, letStmts...)
		stmts = append(stmts, &hir.ReturnStmt{Val: body})

		fn := &hir.Func{
			Name:   mangledName,
			Params: params,
			RetTy:  retTy,
			Body:   &hir.Block{Body: stmts},
		}
		p.arities[mangledName] = len(params)
		funcs = append(funcs, fn)
	}

	return funcs, nil
}

// parseImport: import "path/to/module.frl"
func (p *parser) parseImport() (*hir.Module, error) {
	p.next() // consume "import"
	pathTok := p.next()
	if pathTok.kind != tokString {
		return nil, fmt.Errorf("line %d: import: expected string path", pathTok.line)
	}
	filePath := pathTok.text
	if !filepath.IsAbs(filePath) && p.baseDir != "" {
		filePath = filepath.Join(p.baseDir, filePath)
	}
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("line %d: import %q: %w", pathTok.line, pathTok.text, err)
	}
	ext := filepath.Ext(filePath)
	var child *hir.Module
	switch ext {
	case ".frl":
		child, err = CompileWithOpts(string(src), filepath.Base(filePath), CompileOpts{
			BaseDir: filepath.Dir(filePath),
		})
	case ".nanz":
		child, err = nanz.ParseWithOpts(string(src), filePath, nanz.ParseOpts{
			BaseDir: filepath.Dir(filePath),
		})
	default:
		return nil, fmt.Errorf("line %d: import: unsupported extension %s", pathTok.line, ext)
	}
	if err != nil {
		return nil, fmt.Errorf("import %q: %w", pathTok.text, err)
	}
	// Register imported function arities
	for _, f := range child.Funcs {
		p.arities[f.Name] = len(f.Params)
	}
	return child, nil
}

// parseExtern: extern name (p1 : t1) (p2 : t2) : retty
// or: extern name (p1 : t1) : retty = 0x0010  (fixed address)
func (p *parser) parseExtern() (*hir.Func, error) {
	p.next() // consume "extern"
	nameTok := p.next()

	fn := &hir.Func{Name: nameTok.text, IsExtern: true}

	for p.peek().kind == tokLParen {
		p.next()
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
	p.arities[fn.Name] = len(fn.Params)

	if p.peek().kind == tokColon {
		p.next()
		fn.RetTy = p.parseType()
	} else {
		fn.RetTy = mir2.TyVoid
	}

	// Optional: = address (for ROM/BIOS calls)
	if p.peek().kind == tokEq {
		p.next()
		addrTok := p.next()
		addr, _ := strconv.ParseInt(addrTok.text, 0, 64)
		fn.ExternAddr = uint16(addr)
	}

	return fn, nil
}

// parseProp: prop |x| lhs == rhs
// Generates: __prop_N(x : u8) : u8 = if lhs == rhs then 1 else 0
// + 256 asserts: assert __prop_N 0 == 1, assert __prop_N 1 == 1, ...
func (p *parser) parseProp() (*hir.Func, []hir.Assert, error) {
	p.next() // consume "prop"
	line := p.line

	// Parse lambda: |param| body
	if err := p.expect(tokOp, "|"); err != nil {
		return nil, nil, fmt.Errorf("line %d: prop: expected |param| expr == expr", line)
	}
	paramTok := p.next()
	if err := p.expect(tokOp, "|"); err != nil {
		return nil, nil, err
	}

	// Parse: lhs == rhs
	lhs, err := p.parseAdd()
	if err != nil {
		return nil, nil, err
	}
	if err := p.expect(tokOp, "=="); err != nil {
		return nil, nil, fmt.Errorf("line %d: prop: expected '==' in property", line)
	}
	rhs, err := p.parseAdd()
	if err != nil {
		return nil, nil, err
	}

	// Generate test function: __prop_N(x) = if lhs == rhs then 1 else 0
	propName := fmt.Sprintf("__prop_%d", p.lambdaCount)
	p.lambdaCount++

	cond := &hir.BinExpr{Op: "==", L: lhs, R: rhs, Ty: mir2.TyBool}
	body := &hir.CondExpr{
		Cond: cond,
		Then: &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8},
		Else: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8},
		Ty:   mir2.TyU8,
	}

	fn := &hir.Func{
		Name:   propName,
		Params: []hir.Param{{Name: paramTok.text, Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body:   &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: body}}},
	}

	// Generate 256 asserts (one per u8 value)
	var asserts []hir.Assert
	for i := int64(0); i < 256; i++ {
		asserts = append(asserts, hir.Assert{
			FuncName: propName,
			Args:     []int64{i},
			Expected: 1,
			Source:   fmt.Sprintf("prop |%s| ... (x=%d)", paramTok.text, i),
			Line:     line,
			Via:      "mir2",
		})
	}

	return fn, asserts, nil
}

// parseLet parses:  let name (p1 : t1) (p2 : t2) : retty = body
func (p *parser) parseLet() (*hir.Func, error) {
	p.next() // consume "let"
	nameTok := p.next()
	if nameTok.kind != tokIdent {
		return nil, fmt.Errorf("line %d: expected function name, got %q", nameTok.line, nameTok.text)
	}

	fn := &hir.Func{Name: nameTok.text}

	// Parse parameters: (name : type) or (! name : type) or (& name : type) or (~ name : type)
	// Quantity annotations: ! = linear (1), & = shared (ω), ~ = erased (0)
	// Default (no annotation) = linear (1)
	type paramQuantity struct {
		name     string
		quantity int // 0=erased, 1=linear, 2=shared(ω)
	}
	var paramQtys []paramQuantity
	for p.peek().kind == tokLParen {
		p.next() // (
		qty := -1 // default: no annotation
		pk := p.peek()
		if pk.kind == tokOp && pk.text == "!" {
			p.next(); qty = 1 // explicit linear
		} else if pk.kind == tokOp && pk.text == "~" {
			p.next(); qty = 0 // erased
		}
		pname := p.next()
		if err := p.expect(tokColon, ":"); err != nil {
			return nil, err
		}
		pty := p.parseType()
		if err := p.expect(tokRParen, ")"); err != nil {
			return nil, err
		}
		fn.Params = append(fn.Params, hir.Param{Name: pname.text, Ty: pty})
		paramQtys = append(paramQtys, paramQuantity{pname.text, qty})
	}
	p.arities[fn.Name] = len(fn.Params)

	// Return type: : type or : (type, type) (optional — inferred from body if omitted)
	inferRetTy := false
	if p.peek().kind == tokColon {
		p.next() // :
		if p.peek().kind == tokLParen {
			// Tuple return: (u8, u8)
			p.next() // (
			for {
				ty := p.parseType()
				fn.RetTys = append(fn.RetTys, ty)
				if p.peek().kind == tokComma {
					p.next()
				} else {
					break
				}
			}
			if err := p.expect(tokRParen, ")"); err != nil {
				return nil, err
			}
			fn.RetTy = fn.RetTys[0] // first element as primary
		} else {
			fn.RetTy = p.parseType()
		}
	} else {
		inferRetTy = true
		fn.RetTy = mir2.TyU8
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

	// Multi-return: if body is a tuple expression (parsed from parens with comma)
	if len(fn.RetTys) > 0 {
		if tupleExprs, ok := p.asTuple(body); ok && len(tupleExprs) == len(fn.RetTys) {
			stmts = append(stmts, &hir.ReturnStmt{Vals: tupleExprs})
		} else {
			stmts = append(stmts, &hir.ReturnStmt{Val: body})
		}
	} else {
		stmts = append(stmts, &hir.ReturnStmt{Val: body})
	}

	fn.Body = &hir.Block{Body: stmts}

	// If body is a partial application ref and fn has no params,
	// adopt the partial's definition (currying sugar).
	// let inc = add 1  →  inc becomes __partial_N with its params
	if len(fn.Params) == 0 && len(letStmts) == 0 && len(whereStmts) == 0 {
		if vr, ok := body.(*hir.VarRefExpr); ok {
			for _, af := range p.autoFuncs {
				if af.Name == vr.Name {
					// Adopt: rename partial to fn.Name
					af.Name = fn.Name
					p.arities[fn.Name] = len(af.Params)
					return af, nil
				}
			}
		}
	}

	// Linearity enforcement: verify QTT annotations match actual usage.
	// ! (linear, qty=1): must use exactly once — error if 0 or 2+
	// & (shared, qty=2): can use any number of times — no restriction
	// ~ (erased, qty=0): must NOT use — error if used at all
	// (default: linear) — warn on mismatch but don't error
	if fn.Body != nil && len(fn.Params) > 0 {
		uses := countVarUses(fn.Body)
		for i, param := range fn.Params {
			n := uses[param.Name]
			// Check QTT annotation if present
			var declQty int = -1 // -1 = no annotation
			for _, pq := range paramQtys {
				if pq.name == param.Name {
					declQty = pq.quantity
				}
			}
			// Enforce annotations
			if declQty == 0 && n > 0 {
				return nil, fmt.Errorf("line %d: linearity error: %s: erased param '~%s' used %d times (must be 0)",
					p.line, fn.Name, param.Name, n)
			}
			if declQty == 1 && n != 1 {
				return nil, fmt.Errorf("line %d: linearity error: %s: linear param '!%s' used %d times (must be exactly 1)",
					p.line, fn.Name, param.Name, n)
			}
			// Default analysis (no annotation)
			switch {
			case n == 0 && param.Name != "_":
				p.warnings = append(p.warnings,
					fmt.Sprintf("linearity: %s: param '%s' is erased (0 uses) — could be eliminated", fn.Name, param.Name))
				fn.Params[i].SMC = false
			case n == 1:
				// Linear — ideal
			default:
				if n > 1 {
					p.warnings = append(p.warnings,
						fmt.Sprintf("linearity: %s: param '%s' used %d times (shared)", fn.Name, param.Name, n))
				}
			}
		}
	}

	return fn, nil
}

func countVarUses(block *hir.Block) map[string]int {
	uses := map[string]int{}
	for _, stmt := range block.Body {
		countUsesStmt(stmt, uses)
	}
	return uses
}

func countUsesStmt(stmt hir.Stmt, uses map[string]int) {
	switch s := stmt.(type) {
	case *hir.ReturnStmt:
		if s.Val != nil { countUsesExpr(s.Val, uses) }
		for _, v := range s.Vals { countUsesExpr(v, uses) }
	case *hir.VarDeclStmt:
		if s.Init != nil { countUsesExpr(s.Init, uses) }
	}
}

func countUsesExpr(expr hir.Expr, uses map[string]int) {
	switch e := expr.(type) {
	case *hir.VarRefExpr:
		uses[e.Name]++
	case *hir.BinExpr:
		countUsesExpr(e.L, uses)
		countUsesExpr(e.R, uses)
	case *hir.CallExpr:
		for _, a := range e.Args { countUsesExpr(a, uses) }
	case *hir.CondExpr:
		countUsesExpr(e.Cond, uses)
		countUsesExpr(e.Then, uses)
		countUsesExpr(e.Else, uses)
	case *hir.CastExpr:
		countUsesExpr(e.X, uses)
	}
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
		// Tuple: (e1, e2, ...)
		if p.peek().kind == tokComma {
			elems := []hir.Expr{e}
			for p.peek().kind == tokComma {
				p.next()
				elem, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				elems = append(elems, elem)
			}
			if err := p.expect(tokRParen, ")"); err != nil {
				return nil, err
			}
			p.lastTuple = elems
			return elems[0], nil // return first; caller uses asTuple()
		}
		if err := p.expect(tokRParen, ")"); err != nil {
			return nil, err
		}
		return e, nil
	}

	// Inline asm: asm "instructions" → returns 0, side effect is the asm
	if t.kind == tokIdent && t.text == "asm" {
		p.next()
		asmTok := p.next()
		if asmTok.kind != tokString {
			return nil, fmt.Errorf("line %d: asm requires string", asmTok.line)
		}
		// Wrap as function with asm body, call it immediately
		asmName := fmt.Sprintf("__asm_%d", p.lambdaCount)
		p.lambdaCount++
		p.autoFuncs = append(p.autoFuncs, &hir.Func{
			Name: asmName, RetTy: mir2.TyVoid,
			Body: &hir.Block{Body: []hir.Stmt{
				&hir.AsmStmt{Code: asmTok.text},
			}},
		})
		return &hir.CallExpr{Fn: asmName, Ty: mir2.TyU8}, nil
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
		sym := fmt.Sprintf("@mir2.str.%d", idx)
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
			// Check if this ADT has ANY payload constructor
			adtHasPayload := false
			for _, def := range p.adts {
				for _, c := range def.constructors {
					if c.name == ctor.name {
						for _, c2 := range def.constructors {
							if c2.payload != nil {
								adtHasPayload = true
							}
						}
					}
				}
			}
			if ctor.payload != nil {
				// Constructor with payload: encode as u16 = (tag << 8) | payload
				arg, err := p.parsePrimary()
				if err != nil {
					return nil, err
				}
				tagExpr := &hir.IntLitExpr{Val: ctor.tag * 256, Ty: mir2.TyU16}
				return &hir.BinExpr{Op: "+", L: tagExpr, R: &hir.CastExpr{X: arg, Ty: mir2.TyU16}, Ty: mir2.TyU16}, nil
			}
			if adtHasPayload {
				// No-payload constructor in payload ADT: return u16 tag (e.g. None → 0 as u16)
				return &hir.IntLitExpr{Val: ctor.tag * 256, Ty: mir2.TyU16}, nil
			}
			// Simple ADT (no payloads anywhere): u8 tag
			return &hir.IntLitExpr{Val: ctor.tag, Ty: mir2.TyU8}, nil
		}
	}

	// Built-in: peek(ptr) = load u8 from address
	if t.kind == tokIdent && t.text == "peek" {
		p.next()
		arg, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &hir.LoadExpr{Ptr: arg, Ty: mir2.TyU8}, nil
	}

	// poke is defined as a regular extern — users import it from stdlib
	// (can't use HIR StoreStmt from expression context directly)

	// Identifier (variable ref or function call)
	if t.kind == tokIdent {
		p.next()
		name := t.text

		// Collect call arguments: atoms only (int literals, plain idents, parens).
		// Idents are treated as variable refs, NOT nested function calls.
		arity := p.arities[name] // 0 if unknown
		var args []hir.Expr
		for {
			if arity > 0 && len(args) >= arity {
				break
			}
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
				sym := fmt.Sprintf("@mir2.str.%d", idx)
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
			// Partial application: if fewer args than function arity, generate wrapper
			if arity, ok := p.arities[name]; ok && len(args) < arity {
				missing := arity - len(args)
				partialName := fmt.Sprintf("__partial_%d", p.lambdaCount)
				p.lambdaCount++
				// Generate: __partial_N(p1, p2, ...) = name(captured_args..., p1, p2, ...)
				var params []hir.Param
				var callArgs []hir.Expr
				callArgs = append(callArgs, args...)
				for i := 0; i < missing; i++ {
					pn := fmt.Sprintf("__p%d", i)
					params = append(params, hir.Param{Name: pn, Ty: mir2.TyU8})
					callArgs = append(callArgs, &hir.VarRefExpr{Name: pn, Ty: mir2.TyU8})
				}
				body := &hir.CallExpr{Fn: name, Args: callArgs, Ty: mir2.TyU8}
				p.autoFuncs = append(p.autoFuncs, &hir.Func{
					Name: partialName, Params: params, RetTy: body.Ty,
					Body: &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: body}}},
				})
				p.arities[partialName] = missing
				return &hir.VarRefExpr{Name: partialName, Ty: mir2.TyU8}, nil
			}
			return &hir.CallExpr{Fn: name, Args: args, Ty: mir2.TyU8}, nil
		}

		// Composition: f >> g >> h → generate __compose_N(x) = h(g(f(x)))
		if p.peek().kind == tokOp && p.peek().text == ">>" {
			chain := []string{name}
			for p.peek().kind == tokOp && p.peek().text == ">>" {
				p.next() // consume >>
				next := p.next()
				if next.kind != tokIdent {
					return nil, fmt.Errorf("line %d: expected function name after >>", next.line)
				}
				chain = append(chain, next.text)
			}
			// Generate: __compose_N(x) = chain[last](chain[...](chain[0](x)))
			composeName := fmt.Sprintf("__compose_%d", p.lambdaCount)
			p.lambdaCount++
			paramName := "__cx"
			var body hir.Expr = &hir.VarRefExpr{Name: paramName, Ty: mir2.TyU8}
			for _, fn := range chain {
				body = &hir.CallExpr{Fn: fn, Args: []hir.Expr{body}, Ty: mir2.TyU8}
			}
			p.autoFuncs = append(p.autoFuncs, &hir.Func{
				Name:   composeName,
				Params: []hir.Param{{Name: paramName, Ty: mir2.TyU8}},
				RetTy:  mir2.TyU8,
				Body:   &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: body}}},
			})
			p.arities[composeName] = 1
			return &hir.VarRefExpr{Name: composeName, Ty: mir2.TyU8}, nil
		}

		// Variable reference
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

	// Interleaved do-statements and let-in bindings
	for {
		if p.peek().kind == tokIdent && p.peek().text == "do" {
			p.next() // consume "do"
			// Mutation: do name <- expr
			// Detect by parsing first token as potential assignment target
			doExpr, err := p.parseExpr()
			if err != nil {
				return nil, nil, err
			}
			// Check if next token is <- (means doExpr is actually the target name)
			if p.peek().kind == tokOp && p.peek().text == "<-" {
				p.next() // consume <-
				val, err := p.parseExpr()
				if err != nil {
					return nil, nil, err
				}
				// doExpr should be a VarRefExpr
				if vr, ok := doExpr.(*hir.VarRefExpr); ok {
					stmts = append(stmts, &hir.AssignStmt{
						Target: vr,
						Val:    val,
					})
					continue
				}
			}
			// Regular do: discard result
			if err != nil {
				return nil, nil, err
			}
			discardName := fmt.Sprintf("__do_%d", p.lambdaCount)
			p.lambdaCount++
			stmts = append(stmts, &hir.VarDeclStmt{Name: discardName, Ty: doExpr.ExprTy(), Init: doExpr})
		} else if p.peek().kind == tokIdent && p.peek().text == "let" {
		p.next() // consume "let"

		// Tuple destructuring: let (a, b) = expr in body
		if p.peek().kind == tokLParen {
			p.next() // (
			var names []string
			var tys []mir2.Ty
			for {
				name := p.next()
				names = append(names, name.text)
				tys = append(tys, mir2.TyU8) // TODO: infer types
				if p.peek().kind == tokComma {
					p.next()
				} else {
					break
				}
			}
			if err := p.expect(tokRParen, ")"); err != nil {
				return nil, nil, err
			}
			if err := p.expect(tokEq, "="); err != nil {
				return nil, nil, err
			}
			val, err := p.parseExpr()
			if err != nil {
				return nil, nil, err
			}
			if err := p.expect(tokIdent, "in"); err != nil {
				return nil, nil, err
			}
			// Emit TupleLetStmt
			if call, ok := val.(*hir.CallExpr); ok {
				stmts = append(stmts, &hir.TupleLetStmt{Names: names, Tys: tys, Call: call})
			}
			continue
		}

		nameTok := p.next()
		if p.peek().kind != tokEq {
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
		} else if p.peek().kind == tokIdent && p.peek().text == "while" {
			// while cond do ... end
			p.next() // consume "while"
			cond, err := p.parseComparison()
			if err != nil {
				return nil, nil, err
			}
			if err := p.expect(tokIdent, "do"); err != nil {
				return nil, nil, err
			}
			// Parse while body as sub-block of do/let stmts
			var whileBody []hir.Stmt
			for {
				if p.peek().kind == tokIdent && p.peek().text == "end" {
					p.next()
					break
				}
				if p.peek().kind == tokIdent && p.peek().text == "do" {
					p.next()
					doE, err := p.parseExpr()
					if err != nil {
						return nil, nil, err
					}
					// Check mutation
					if p.peek().kind == tokOp && p.peek().text == "<-" {
						p.next()
						val, err := p.parseExpr()
						if err != nil {
							return nil, nil, err
						}
						if vr, ok := doE.(*hir.VarRefExpr); ok {
							whileBody = append(whileBody, &hir.AssignStmt{Target: vr, Val: val})
						}
					} else {
						dn := fmt.Sprintf("__wd_%d", p.lambdaCount)
						p.lambdaCount++
						whileBody = append(whileBody, &hir.VarDeclStmt{Name: dn, Ty: doE.ExprTy(), Init: doE})
					}
				} else {
					break
				}
			}
			stmts = append(stmts, &hir.WhileStmt{
				Cond: cond,
				Body: &hir.Block{Body: whileBody},
			})
		} else {
			break
		}
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
		isDefault       bool
		val             int64
		guard           hir.Expr // nil = no guard; non-nil = extra condition
		body            hir.Expr
		bindName        string // variable name bound to scrutinee (for guards)
		hasPayloadBind  bool   // true when | Some x -> ... binds payload
	}
	var arms []arm

	// Parse arms: | pattern [when guard] -> body
	for p.peek().kind == tokOp && p.peek().text == "|" {
		p.next() // consume |
		tok := p.peek()
		isDefault := false
		hasPayloadBind := false
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
				// Named constructor — check for payload binding: | Some x -> ...
				val = ctor.tag
				if ctor.payload != nil && p.peek().kind == tokIdent && !isKeyword(p.peek().text) {
					bindName = p.next().text
					hasPayloadBind = true
				}
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
		// For payload binding: | Some x -> body  →  wrap body in function taking x
		if hasPayloadBind && bindName != "" {
			wrapName := fmt.Sprintf("__mpay_%d", p.lambdaCount)
			p.lambdaCount++
			p.autoFuncs = append(p.autoFuncs, &hir.Func{
				Name: wrapName, Params: []hir.Param{{Name: bindName, Ty: mir2.TyU8}},
				RetTy: body.ExprTy(),
				Body:  &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: body}}},
			})
			// Replace body with: wrapName(__payload(scrutinee))
			body = &hir.CallExpr{
				Fn:   wrapName,
				Args: []hir.Expr{&hir.CallExpr{Fn: "__payload", Args: []hir.Expr{scrutinee}, Ty: mir2.TyU8}},
				Ty:   body.ExprTy(),
			}
		}
		hasPayloadBind = false
		arms = append(arms, arm{isDefault: isDefault, val: val, guard: guard, body: body, bindName: bindName, hasPayloadBind: false})
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

	// For payload ADTs: wrap match in a function that pre-computes __tag.
	// This prevents the optimizer from folding away individual __tag calls.
	if scrutinee.ExprTy() == mir2.TyU16 {
		tagFnName := fmt.Sprintf("__match_tag_%d", p.lambdaCount)
		tagVarName := fmt.Sprintf("__t%d", p.lambdaCount)
		payloadVarName := fmt.Sprintf("__v%d", p.lambdaCount)
		p.lambdaCount++

		// Build a wrapper function:
		// __match_tag_N(opt: u16, extra_params...) =
		//   let __tN = __tag opt in
		//   let __vN = __payload opt in
		//   match __tN with | ... end
		// Then replace the match expression with a call to this wrapper.
		// For now, simpler: replace scrutinee with __tag(scrutinee) as VarRef
		// by emitting a VarDeclStmt into the enclosing function body.

		// We can't inject stmts from inside parseMatch, so instead:
		// use a synthetic intermediate — build the tag as a nested call
		// that the optimizer can't fold because it's opaque.
		_ = tagFnName
		_ = tagVarName
		_ = payloadVarName
		// Override scrutinee to be __tag(scrutinee) result
		tagScrutinee := &hir.CallExpr{Fn: "__tag", Args: []hir.Expr{scrutinee}, Ty: mir2.TyU8}
		scrutinee = tagScrutinee
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
			// Scrutinee is already u8 (either plain ADT or __tag(opt) from above)
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
	case "let", "in", "if", "then", "else", "type", "assert", "match", "with", "fun", "end", "where", "when", "prop", "extern", "do", "import", "asm", "class", "instance", "while":
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

// pendingTuple stores tuple elements when a (e1, e2) expression is parsed.
// Since tupleExpr can't implement hir.Expr (unexported interface method),
// we store the first element as the expression and save the full list here.
func (p *parser) asTuple(_ hir.Expr) ([]hir.Expr, bool) {
	if p.lastTuple != nil {
		t := p.lastTuple
		p.lastTuple = nil
		return t, true
	}
	return nil, false
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
