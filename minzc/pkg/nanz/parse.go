package nanz

// parse.go — hand-written recursive-descent parser.
// Nanz source → hir.Module
//
// Grammar (informal):
//
//	module      = (struct_decl | global_decl | fun_decl)*
//	struct_decl = 'struct' IDENT '{' (IDENT ':' type '\n')* '}'
//	global_decl = 'global' IDENT ':' type ['at' '(' expr ')'] ['=' array_lit | '=' expr] '\n'
//	fun_decl    = ['@extern'] 'fun' IDENT '(' params ')' ['->' type] ('{' stmt* '}' | '\n')
//	params      = (IDENT ':' type (',' IDENT ':' type)*)?
//
//	stmt        = var_decl | assign | store | if_stmt | while_stmt | for_stmt
//	            | return_stmt | expr_stmt | break | continue | switch_stmt | block
//	var_decl    = 'var' IDENT ':' type ['at' '(' expr ')'] ['=' (array_lit | expr)]
//	assign      = expr '=' expr                (where lhs is lvalue)
//	store       = '^' expr '=' expr
//	if_stmt     = 'if' expr '{' stmt* '}' ['else' '{' stmt* '}']
//	while_stmt  = 'while' expr '{' stmt* '}'
//	for_stmt    = 'for' IDENT 'in' expr '..' expr '{' stmt* '}'
//	return_stmt = 'return' [expr]
//	switch_stmt = 'switch' expr '{' ('case' INT ':' stmt*)* ['default' ':' stmt*] '}'
//
//	type        = '^' type | '[' type ';' INT ']' | IDENT
//	expr        = ... (Pratt parser, standard binary precedence)

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Parse parses Nanz source and returns a HIR module.
func Parse(src, name string) (*hir.Module, error) {
	l := newLexer(src)
	p := &parser{l: l, name: name}
	return p.parseModule()
}

// ── Lexer ─────────────────────────────────────────────────────────────────────

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokInt
	tokString
	tokArrow   // ->
	tokDotDot  // ..
	tokLBrace  // {
	tokRBrace  // }
	tokLParen  // (
	tokRParen  // )
	tokLBrack  // [
	tokRBrack  // ]
	tokColon   // :
	tokSemi    // ;
	tokComma   // ,
	tokDot     // .
	tokEq      // =
	tokEqEq    // ==
	tokBang    // !
	tokBangEq  // !=
	tokLt      // <
	tokLtEq    // <=
	tokGt      // >
	tokGtEq    // >=
	tokPlus    // +
	tokMinus   // -
	tokStar    // *
	tokSlash   // /
	tokPercent // %
	tokAmp     // &
	tokPipe    // |
	tokCaret   // ^ (also used as pointer dereference)
	tokTilde   // ~
	tokLtLt   // <<
	tokGtGt   // >>
	tokAt      // @
)

type token struct {
	kind tokKind
	val  string
	line int
}

type lexer struct {
	src    []byte
	pos    int
	line   int
	tokens []token
	cur    int
}

func newLexer(src string) *lexer {
	l := &lexer{src: []byte(src), line: 1}
	l.tokenize()
	return l
}

func (l *lexer) tokenize() {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]

		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.pos++
			continue
		}
		if ch == '\n' {
			l.line++
			l.pos++
			continue
		}
		// Line comment //
		if ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		// Block comment /* ... */
		if ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '*' {
			l.pos += 2
			for l.pos < len(l.src) {
				if l.src[l.pos] == '\n' {
					l.line++
				}
				if l.src[l.pos] == '*' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
					l.pos += 2
					break
				}
				l.pos++
			}
			continue
		}

		line := l.line
		// Multi-char tokens
		switch {
		case ch == '-' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '>':
			l.emit(tokArrow, "->", line); l.pos += 2; continue
		case ch == '.' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '.':
			l.emit(tokDotDot, "..", line); l.pos += 2; continue
		case ch == '=' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
			l.emit(tokEqEq, "==", line); l.pos += 2; continue
		case ch == '!' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
			l.emit(tokBangEq, "!=", line); l.pos += 2; continue
		case ch == '<' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
			l.emit(tokLtEq, "<=", line); l.pos += 2; continue
		case ch == '>' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
			l.emit(tokGtEq, ">=", line); l.pos += 2; continue
		case ch == '<' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '<':
			l.emit(tokLtLt, "<<", line); l.pos += 2; continue
		case ch == '>' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '>':
			l.emit(tokGtGt, ">>", line); l.pos += 2; continue
		}

		// Single-char
		var k tokKind
		switch ch {
		case '{':
			k = tokLBrace
		case '}':
			k = tokRBrace
		case '(':
			k = tokLParen
		case ')':
			k = tokRParen
		case '[':
			k = tokLBrack
		case ']':
			k = tokRBrack
		case ':':
			k = tokColon
		case ';':
			k = tokSemi
		case ',':
			k = tokComma
		case '.':
			k = tokDot
		case '=':
			k = tokEq
		case '!':
			k = tokBang
		case '<':
			k = tokLt
		case '>':
			k = tokGt
		case '+':
			k = tokPlus
		case '-':
			k = tokMinus
		case '*':
			k = tokStar
		case '/':
			k = tokSlash
		case '%':
			k = tokPercent
		case '&':
			k = tokAmp
		case '|':
			k = tokPipe
		case '^':
			k = tokCaret
		case '~':
			k = tokTilde
		case '@':
			k = tokAt
		default:
			// String literal
			if ch == '"' {
				l.pos++
				start := l.pos
				for l.pos < len(l.src) && l.src[l.pos] != '"' {
					l.pos++
				}
				s := string(l.src[start:l.pos])
				if l.pos < len(l.src) {
					l.pos++
				}
				l.emit(tokString, s, line)
				continue
			}
			// Number
			if ch >= '0' && ch <= '9' {
				start := l.pos
				// hex 0x...
				if ch == '0' && l.pos+1 < len(l.src) && (l.src[l.pos+1] == 'x' || l.src[l.pos+1] == 'X') {
					l.pos += 2
					for l.pos < len(l.src) && isHexDigit(l.src[l.pos]) {
						l.pos++
					}
				} else {
					for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
						l.pos++
					}
				}
				l.emit(tokInt, string(l.src[start:l.pos]), line)
				continue
			}
			// Identifier
			if isIdentStart(ch) {
				start := l.pos
				for l.pos < len(l.src) && isIdentCont(l.src[l.pos]) {
					l.pos++
				}
				l.emit(tokIdent, string(l.src[start:l.pos]), line)
				continue
			}
			// Unknown — skip
			l.pos++
			continue
		}
		l.emit(k, string(ch), line)
		l.pos++
	}
	l.emit(tokEOF, "", l.line)
}

func (l *lexer) emit(k tokKind, v string, line int) {
	l.tokens = append(l.tokens, token{kind: k, val: v, line: line})
}

func (l *lexer) peek() token {
	if l.cur < len(l.tokens) {
		return l.tokens[l.cur]
	}
	return token{kind: tokEOF}
}
func (l *lexer) next() token {
	t := l.peek()
	if t.kind != tokEOF {
		l.cur++
	}
	return t
}
func (l *lexer) is(k tokKind) bool         { return l.peek().kind == k }
func (l *lexer) isIdent(v string) bool      { return l.peek().kind == tokIdent && l.peek().val == v }
func (l *lexer) eat(k tokKind) (token, error) {
	t := l.next()
	if t.kind != k {
		return t, fmt.Errorf("line %d: expected token kind %d, got %q", t.line, k, t.val)
	}
	return t, nil
}
func (l *lexer) eatIdent(v string) error {
	t := l.next()
	if t.kind != tokIdent || t.val != v {
		return fmt.Errorf("line %d: expected %q, got %q", t.line, v, t.val)
	}
	return nil
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}
func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// ── Parser ────────────────────────────────────────────────────────────────────

// methodInfo records a struct method's mangled function name and return type.
type methodInfo struct {
	funcName string
	retTy    mir2.Ty
}

// opOverload records an operator overload's mangled function name and return type.
type opOverload struct {
	funcName string
	retTy    mir2.Ty
}

type parser struct {
	l           *lexer
	name        string
	structs     map[string]*mir2.StructTy
	interfaces  map[string]*hir.InterfaceDecl   // interface name → declaration
	lambdas     []*hir.Func // anonymous functions generated from |params| body syntax
	lambdaCount int
	// Week 1: UFCS + struct methods + operator overloading
	globalTypes          map[string]mir2.Ty              // module-level: global varname → type (persistent)
	globalInterfaceTypes map[string]string               // module-level: global varname → interface name
	varTypes             map[string]mir2.Ty              // current function scope: varname → type (reset per func)
	varInterfaceTypes    map[string]string               // current function scope: varname → interface name
	methodTable          map[string]map[string]methodInfo // structName → methodName → info
	opTable     map[string]opOverload             // op symbol ("+", "-", ...) → overload
}

// exprTy returns the known type of an expression, consulting varTypes and
// globalTypes for VarRefExpr (since VarRefExpr.Ty defaults to TyU8 at parse time).
func (p *parser) exprTy(e hir.Expr) mir2.Ty {
	if vr, ok := e.(*hir.VarRefExpr); ok {
		if ty, found := p.varTypes[vr.Name]; found {
			return ty
		}
		if ty, found := p.globalTypes[vr.Name]; found {
			return ty
		}
	}
	return e.ExprTy()
}

// opToFuncName maps an operator token kind to a mangled function name.
func opToFuncName(k tokKind) string {
	switch k {
	case tokPlus:
		return "op_add"
	case tokMinus:
		return "op_sub"
	case tokStar:
		return "op_mul"
	case tokSlash:
		return "op_div"
	case tokPercent:
		return "op_rem"
	case tokEqEq:
		return "op_eq"
	case tokBangEq:
		return "op_ne"
	case tokLt:
		return "op_lt"
	case tokLtEq:
		return "op_le"
	case tokGt:
		return "op_gt"
	case tokGtEq:
		return "op_ge"
	case tokAmp:
		return "op_and"
	case tokPipe:
		return "op_or"
	case tokCaret:
		return "op_xor"
	}
	return "op_unknown"
}

// isOpToken returns true if k can be used as an operator function name.
func isOpToken(k tokKind) bool {
	switch k {
	case tokPlus, tokMinus, tokStar, tokSlash, tokPercent,
		tokEqEq, tokBangEq, tokLt, tokLtEq, tokGt, tokGtEq,
		tokAmp, tokPipe, tokCaret:
		return true
	}
	return false
}

func (p *parser) parseModule() (*hir.Module, error) {
	m := &hir.Module{Name: p.name}
	p.structs = make(map[string]*mir2.StructTy)
	p.interfaces = make(map[string]*hir.InterfaceDecl)
	p.methodTable = make(map[string]map[string]methodInfo)
	p.opTable = make(map[string]opOverload)
	p.globalTypes = make(map[string]mir2.Ty)
	p.globalInterfaceTypes = make(map[string]string)
	p.varTypes = make(map[string]mir2.Ty)
	p.varInterfaceTypes = make(map[string]string)

	for !p.l.is(tokEOF) {
		t := p.l.peek()
		switch {
		case t.kind == tokIdent && t.val == "struct":
			st, err := p.parseStructDecl()
			if err != nil {
				return nil, err
			}
			m.Structs = append(m.Structs, st)
			p.structs[st.Name] = st

		case t.kind == tokIdent && t.val == "interface":
			decl, err := p.parseInterfaceDecl()
			if err != nil {
				return nil, err
			}
			p.interfaces[decl.Name] = decl
			m.Interfaces = append(m.Interfaces, decl)

		case t.kind == tokIdent && t.val == "global":
			g, err := p.parseGlobalDecl()
			if err != nil {
				return nil, err
			}
			m.Globals = append(m.Globals, g)

		case t.kind == tokIdent && t.val == "fun":
			f, err := p.parseFunDecl(false)
			if err != nil {
				return nil, err
			}
			m.Funcs = append(m.Funcs, f)

		case t.kind == tokAt:
			// @extern fun ...
			p.l.next()
			attr := p.l.peek()
			if attr.kind == tokIdent && attr.val == "extern" {
				p.l.next()
				if err := p.l.eatIdent("fun"); err != nil {
					return nil, err
				}
				f, err := p.parseFunDecl(true)
				if err != nil {
					return nil, err
				}
				m.Funcs = append(m.Funcs, f)
			} else {
				return nil, fmt.Errorf("line %d: unexpected @%s", attr.line, attr.val)
			}

		default:
			return nil, fmt.Errorf("line %d: unexpected token %q at module level", t.line, t.val)
		}
	}
	// Append lambdas generated during parsing (non-capturing anonymous functions).
	m.Funcs = append(m.Funcs, p.lambdas...)
	return m, nil
}

// ── Struct ────────────────────────────────────────────────────────────────────

func (p *parser) parseStructDecl() (*mir2.StructTy, error) {
	if err := p.l.eatIdent("struct"); err != nil {
		return nil, err
	}
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.l.eat(tokLBrace); err != nil {
		return nil, err
	}
	st := &mir2.StructTy{Name: nameTok.val}
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		fieldName, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokColon); err != nil {
			return nil, err
		}
		ty, err := p.parseType()
		if err != nil {
			return nil, err
		}
		st.Fields = append(st.Fields, mir2.StructField{Name: fieldName.val, Ty: ty})
		// optional comma or newline separator — just consume comma if present
		if p.l.is(tokComma) {
			p.l.next()
		}
	}
	if _, err := p.l.eat(tokRBrace); err != nil {
		return nil, err
	}
	return st, nil
}

// ── Interface declaration ─────────────────────────────────────────────────────

// parseInterfaceDecl parses: interface Name { methodName; ... }
// Method lines may optionally start with the keyword "fun"; return types and
// parameter lists are skipped (the parser only captures method names).
// Separator between entries is flexible: the Nanz lexer is whitespace-agnostic,
// so we just look for identifiers until we hit '}'.
func (p *parser) parseInterfaceDecl() (*hir.InterfaceDecl, error) {
	if err := p.l.eatIdent("interface"); err != nil {
		return nil, err
	}
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.l.eat(tokLBrace); err != nil {
		return nil, err
	}
	decl := &hir.InterfaceDecl{Name: nameTok.val}
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		// Optional "fun" keyword before the method name.
		if p.l.isIdent("fun") {
			p.l.next()
		}
		// Expect a method name identifier.
		methodTok, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, err
		}
		decl.Methods = append(decl.Methods, methodTok.val)
		// Skip the rest of the line until '}', ';', or next method.
		// Since the Nanz lexer is whitespace-insensitive, we skip tokens
		// until we see a tokRBrace or the next method entry (next tokIdent
		// that is not part of a type annotation).  The simplest rule:
		// skip until we see '}' or a bare identifier (which we'll re-read
		// at the top of the loop).  We handle optional comma separators and
		// any type signature tokens.
		for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
			// A comma is an explicit separator — consume and move on.
			if p.l.is(tokComma) {
				p.l.next()
				break
			}
			// If the next token is an identifier that is NOT a keyword we
			// recognise as part of a type signature ('fun' is handled at the
			// top of the outer loop), it likely starts the next method name.
			// Stop skipping so the outer loop can read it.
			next := p.l.peek()
			if next.kind == tokIdent {
				// Could be the next method name or "fun".  Stop here.
				break
			}
			p.l.next()
		}
	}
	if _, err := p.l.eat(tokRBrace); err != nil {
		return nil, err
	}
	return decl, nil
}

// ── Global declaration ────────────────────────────────────────────────────────

func (p *parser) parseGlobalDecl() (mir2.Global, error) {
	if err := p.l.eatIdent("global"); err != nil {
		return mir2.Global{}, err
	}
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return mir2.Global{}, err
	}
	if _, err := p.l.eat(tokColon); err != nil {
		return mir2.Global{}, err
	}
	ty, ifaceName, err := p.parseTypeWithIface()
	if err != nil {
		return mir2.Global{}, err
	}
	g := mir2.Global{Name: nameTok.val, Ty: ty}
	p.globalTypes[nameTok.val] = ty // register for UFCS/field-offset lookup
	if ifaceName != "" {
		p.globalInterfaceTypes[nameTok.val] = ifaceName
	}

	// at(addr)?
	if p.l.isIdent("at") {
		p.l.next()
		if _, err := p.l.eat(tokLParen); err != nil {
			return g, err
		}
		addrExpr, err := p.parseExpr()
		if err != nil {
			return g, err
		}
		if _, err := p.l.eat(tokRParen); err != nil {
			return g, err
		}
		if lit, ok := addrExpr.(*hir.IntLitExpr); ok {
			addr := uint16(lit.Val)
			g.At = &addr
		}
	}

	// = initializer?
	if p.l.is(tokEq) {
		p.l.next()
		init, err := p.parseInitializer(ty)
		if err != nil {
			return g, err
		}
		g.Init = init
	}

	return g, nil
}

func (p *parser) parseInitializer(ty mir2.Ty) ([]byte, error) {
	if p.l.is(tokLBrack) {
		// Array initializer [v1, v2, ...]
		p.l.next()
		var data []byte
		for !p.l.is(tokRBrack) && !p.l.is(tokEOF) {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if lit, ok := e.(*hir.IntLitExpr); ok {
				// Emit as 1 or 2 bytes depending on element type
				elemTy := ty
				if at, ok := ty.(*mir2.ArrayTy); ok {
					elemTy = at.Elem
				}
				switch elemTy.Width() {
				case 8:
					data = append(data, byte(lit.Val))
				case 16:
					data = append(data, byte(lit.Val), byte(lit.Val>>8))
				default:
					data = append(data, byte(lit.Val))
				}
			}
			if p.l.is(tokComma) {
				p.l.next()
			}
		}
		if _, err := p.l.eat(tokRBrack); err != nil {
			return nil, err
		}
		return data, nil
	}
	// Scalar initializer
	e, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if lit, ok := e.(*hir.IntLitExpr); ok {
		switch ty.Width() {
		case 8:
			return []byte{byte(lit.Val)}, nil
		case 16:
			return []byte{byte(lit.Val), byte(lit.Val >> 8)}, nil
		}
	}
	return nil, nil
}

// ── Function declaration ──────────────────────────────────────────────────────

func (p *parser) parseFunDecl(isExtern bool) (*hir.Func, error) {
	// 'fun' already consumed by caller for @extern; consume here for plain fun
	if !isExtern {
		if err := p.l.eatIdent("fun"); err != nil {
			return nil, err
		}
	}

	// ── Parse function name ────────────────────────────────────────────────────
	// Three forms:
	//   fun foo(...)               → regular function, name = "foo"
	//   fun Vec2.add(...)          → struct method, name = "Vec2_add", registered in methodTable
	//   fun +(a: Vec2, b: Vec2)    → operator overload, name = "op_add", registered in opTable
	var funcName string
	var opSym string // non-empty if this is an operator overload

	if isOpToken(p.l.peek().kind) {
		// Operator overload: fun + / fun * / etc.
		opTok := p.l.next()
		opSym = opTok.val
		funcName = opToFuncName(opTok.kind)
	} else {
		nameTok, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, err
		}
		if p.l.is(tokDot) {
			// Struct method: fun TypeName.methodName(...)
			p.l.next()
			methodTok, err := p.l.eat(tokIdent)
			if err != nil {
				return nil, err
			}
			structName := nameTok.val
			methodName := methodTok.val
			funcName = structName + "_" + methodName
			// Pre-register method name (retTy filled in below after parsing)
			if p.methodTable[structName] == nil {
				p.methodTable[structName] = make(map[string]methodInfo)
			}
			// Placeholder — retTy updated after return type is parsed
			p.methodTable[structName][methodName] = methodInfo{funcName: funcName, retTy: mir2.TyVoid}
		} else {
			funcName = nameTok.val
		}
	}

	if _, err := p.l.eat(tokLParen); err != nil {
		return nil, err
	}
	var params []hir.Param
	var paramIfaceNames []string // parallel to params: interface name or ""
	for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
		pname, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokColon); err != nil {
			return nil, err
		}
		pty, ifaceName, err := p.parseTypeWithIface()
		if err != nil {
			return nil, err
		}
		params = append(params, hir.Param{Name: pname.val, Ty: pty})
		paramIfaceNames = append(paramIfaceNames, ifaceName)
		if p.l.is(tokComma) {
			p.l.next()
		}
	}
	if _, err := p.l.eat(tokRParen); err != nil {
		return nil, err
	}

	var err error
	retTy := mir2.Ty(mir2.TyVoid)
	if p.l.is(tokArrow) {
		p.l.next()
		retTy, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}

	// Update method/op tables now that we know the return type
	if opSym != "" {
		p.opTable[opSym] = opOverload{funcName: funcName, retTy: retTy}
	} else {
		// Update methodTable if this is a struct method (find by funcName)
		for structName, methods := range p.methodTable {
			for methodName, info := range methods {
				if info.funcName == funcName {
					p.methodTable[structName][methodName] = methodInfo{funcName: funcName, retTy: retTy}
				}
			}
		}
	}

	// Reset var scope and populate from params for the function body
	p.varTypes = make(map[string]mir2.Ty)
	p.varInterfaceTypes = make(map[string]string)
	for i, param := range params {
		p.varTypes[param.Name] = param.Ty
		if i < len(paramIfaceNames) && paramIfaceNames[i] != "" {
			p.varInterfaceTypes[param.Name] = paramIfaceNames[i]
		}
	}

	f := &hir.Func{
		Name:     funcName,
		Params:   params,
		RetTy:    retTy,
		IsExtern: isExtern,
	}

	if isExtern || !p.l.is(tokLBrace) {
		// No body
		return f, nil
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	f.Body = body
	return f, nil
}

// ── Types ─────────────────────────────────────────────────────────────────────

func (p *parser) parseType() (mir2.Ty, error) {
	t := p.l.peek()

	// ^T — pointer to T
	if t.kind == tokCaret {
		p.l.next()
		// Consume inner type if present (^u8, ^u16, ^Point…).
		// MIR2 TyPtr is untyped — we parse and discard the element type.
		if !p.l.is(tokEOF) && !p.l.is(tokComma) && !p.l.is(tokRParen) &&
			!p.l.is(tokLBrace) && !p.l.is(tokRBrace) && !p.l.is(tokSemi) &&
			!p.l.is(tokEq) && !p.l.is(tokArrow) {
			_, _ = p.parseType()
		}
		return mir2.TyPtr, nil
	}

	// [T; N] — array
	if t.kind == tokLBrack {
		p.l.next()
		elemTy, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokSemi); err != nil {
			return nil, err
		}
		lenTok, err := p.l.eat(tokInt)
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(lenTok.val)
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokRBrack); err != nil {
			return nil, err
		}
		return mir2.NewArray(elemTy, n), nil
	}

	// Named type
	if t.kind == tokIdent {
		p.l.next()
		var base mir2.Ty
		switch t.val {
		case "u8":
			base = mir2.TyU8
		case "u16":
			base = mir2.TyU16
		case "i8":
			base = mir2.TyI8
		case "i16":
			base = mir2.TyI16
		case "bool":
			base = mir2.TyBool
		case "void":
			return mir2.TyVoid, nil
		case "ptr":
			return mir2.TyPtr, nil
		default:
			// Named struct type
			if st, ok := p.structs[t.val]; ok {
				return st, nil
			}
			// Interface type: treated as an opaque pointer at the call site.
			// Monomorphisation happens at the call site; no vtable is emitted.
			if _, ok := p.interfaces[t.val]; ok {
				return mir2.TyPtr, nil
			}
			return nil, fmt.Errorf("line %d: unknown type %q", t.line, t.val)
		}
		// Optional range annotation: T<lo..hi>  (hi is inclusive in source)
		if base != nil && p.l.is(tokLt) {
			return p.parseRangedType(base)
		}
		return base, nil
	}

	return nil, fmt.Errorf("line %d: expected type, got %q", t.line, t.val)
}

// parseTypeWithIface is like parseType but also returns the interface name when
// the type is an interface (e.g. "Animal" → TyPtr, "Animal").  Returns ("", nil)
// for the interface name when the type is not an interface.
func (p *parser) parseTypeWithIface() (mir2.Ty, string, error) {
	t := p.l.peek()
	if t.kind == tokIdent {
		if _, ok := p.interfaces[t.val]; ok {
			p.l.next()
			return mir2.TyPtr, t.val, nil
		}
	}
	ty, err := p.parseType()
	return ty, "", err
}

// findImplementors returns all struct names that satisfy every method in the
// named interface AND have methodName in their methodTable.
func (p *parser) findImplementors(ifaceName, methodName string) []string {
	decl := p.interfaces[ifaceName]
	if decl == nil {
		return nil
	}
	var result []string
	for structName, methods := range p.methodTable {
		ok := true
		for _, m := range decl.Methods {
			if _, has := methods[m]; !has {
				ok = false
				break
			}
		}
		if ok {
			if _, has := methods[methodName]; has {
				result = append(result, structName)
			}
		}
	}
	return result
}

// parseRangedType parses the <lo..hi> suffix that follows an integer base type.
// The cursor must be positioned at the '<' token.
// hi in source syntax is INCLUSIVE; we store it as exclusive (hi+1) in RangedTy.
//
//	u8<0..63>   → RangedTy{Base: TyU8, Lo: 0, Hi: 64}
//	u16<100..200> → RangedTy{Base: TyU16, Lo: 100, Hi: 201}
func (p *parser) parseRangedType(base mir2.Ty) (mir2.Ty, error) {
	// Consume '<'
	ltTok, err := p.l.eat(tokLt)
	if err != nil {
		return nil, err
	}

	loTok, err := p.l.eat(tokInt)
	if err != nil {
		return nil, fmt.Errorf("line %d: expected integer lo in range type", ltTok.line)
	}
	lo, err := strconv.ParseInt(loTok.val, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("line %d: invalid range lo %q: %v", loTok.line, loTok.val, err)
	}

	if _, err := p.l.eat(tokDotDot); err != nil {
		return nil, fmt.Errorf("line %d: expected '..' in range type", loTok.line)
	}

	hiTok, err := p.l.eat(tokInt)
	if err != nil {
		return nil, fmt.Errorf("line %d: expected integer hi in range type", loTok.line)
	}
	hi, err := strconv.ParseInt(hiTok.val, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("line %d: invalid range hi %q: %v", hiTok.line, hiTok.val, err)
	}

	if _, err := p.l.eat(tokGt); err != nil {
		return nil, fmt.Errorf("line %d: expected '>' to close range type", hiTok.line)
	}

	if lo > hi {
		return nil, fmt.Errorf("line %d: range lo (%d) > hi (%d)", ltTok.line, lo, hi)
	}

	// hi in source is inclusive → store exclusive (hi+1) to match Go convention
	return mir2.NewRanged(base, lo, hi+1), nil
}

// ── Block & statements ────────────────────────────────────────────────────────

func (p *parser) parseBlock() (*hir.Block, error) {
	if _, err := p.l.eat(tokLBrace); err != nil {
		return nil, err
	}
	var stmts []hir.Stmt
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if s != nil {
			stmts = append(stmts, s)
		}
	}
	if _, err := p.l.eat(tokRBrace); err != nil {
		return nil, err
	}
	return &hir.Block{Body: stmts}, nil
}

func (p *parser) parseStmt() (hir.Stmt, error) {
	t := p.l.peek()

	switch {
	case t.kind == tokIdent && t.val == "var":
		return p.parseVarDecl()
	case t.kind == tokIdent && t.val == "let":
		return p.parseLetDecl()
	case t.kind == tokIdent && t.val == "if":
		return p.parseIf()
	case t.kind == tokIdent && t.val == "while":
		return p.parseWhile()
	case t.kind == tokIdent && t.val == "for":
		return p.parseFor()
	case t.kind == tokIdent && t.val == "return":
		return p.parseReturn()
	case t.kind == tokIdent && t.val == "break":
		p.l.next()
		return &hir.BreakStmt{}, nil
	case t.kind == tokIdent && t.val == "continue":
		p.l.next()
		return &hir.ContinueStmt{}, nil
	case t.kind == tokIdent && t.val == "switch":
		return p.parseSwitch()
	case t.kind == tokLBrace:
		return p.parseBlock()
	default:
		// expr (= expr)? — assignment or bare call
		return p.parseExprStmt()
	}
}

// parseLetDecl parses: let name [: T] = expr
// Type is inferred from the RHS if not specified.
func (p *parser) parseLetDecl() (hir.Stmt, error) {
	if err := p.l.eatIdent("let"); err != nil {
		return nil, err
	}
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return nil, err
	}

	var ty mir2.Ty

	// Optional explicit type: let x: T = ...
	if p.l.is(tokColon) {
		p.l.next()
		ty, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}

	if _, err := p.l.eat(tokEq); err != nil {
		return nil, err
	}

	init, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	// Infer type from RHS if not given.
	if ty == nil {
		ty = init.ExprTy()
	}
	// If the explicit type is non-void and the init is a call with unknown (void) return type,
	// patch the call's type so the lowerer emits a value call (not CallVoid).
	if ty != nil && ty != mir2.TyVoid {
		if call, ok := init.(*hir.CallExpr); ok && call.Ty == mir2.TyVoid {
			call.Ty = ty
		}
	}

	// Unwrap array type
	d := &hir.VarDeclStmt{Name: nameTok.val}
	if at, ok := ty.(*mir2.ArrayTy); ok {
		d.Ty = at.Elem
		d.ArrayLen = at.Len
		p.varTypes[nameTok.val] = at.Elem
	} else {
		d.Ty = ty
		d.Init = init
		p.varTypes[nameTok.val] = ty
	}
	return d, nil
}

func (p *parser) parseVarDecl() (hir.Stmt, error) {
	if err := p.l.eatIdent("var"); err != nil {
		return nil, err
	}
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.l.eat(tokColon); err != nil {
		return nil, err
	}
	ty, err := p.parseType()
	if err != nil {
		return nil, err
	}

	d := &hir.VarDeclStmt{Name: nameTok.val}

	// Unwrap array type
	if at, ok := ty.(*mir2.ArrayTy); ok {
		d.Ty = at.Elem
		d.ArrayLen = at.Len
		p.varTypes[nameTok.val] = at.Elem
	} else {
		d.Ty = ty
		p.varTypes[nameTok.val] = ty
	}

	// at(addr)?
	if p.l.isIdent("at") {
		p.l.next()
		if _, err := p.l.eat(tokLParen); err != nil {
			return nil, err
		}
		addrExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokRParen); err != nil {
			return nil, err
		}
		if lit, ok := addrExpr.(*hir.IntLitExpr); ok {
			addr := uint16(lit.Val)
			d.At = &addr
		}
	}

	// = initializer?
	if p.l.is(tokEq) {
		p.l.next()
		if p.l.is(tokLBrack) {
			p.l.next()
			for !p.l.is(tokRBrack) && !p.l.is(tokEOF) {
				e, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				d.Initial = append(d.Initial, e)
				if p.l.is(tokComma) {
					p.l.next()
				}
			}
			if _, err := p.l.eat(tokRBrack); err != nil {
				return nil, err
			}
		} else {
			init, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			d.Init = init
		}
	}

	return d, nil
}

func (p *parser) parseIf() (hir.Stmt, error) {
	if err := p.l.eatIdent("if"); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	then, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	s := &hir.IfStmt{Cond: cond, Then: then}
	if p.l.isIdent("else") {
		p.l.next()
		els, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		s.Else = els
	}
	return s, nil
}

func (p *parser) parseWhile() (hir.Stmt, error) {
	if err := p.l.eatIdent("while"); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &hir.WhileStmt{Cond: cond, Body: body}, nil
}

func (p *parser) parseFor() (hir.Stmt, error) {
	if err := p.l.eatIdent("for"); err != nil {
		return nil, err
	}
	varTok, err := p.l.eat(tokIdent)
	if err != nil {
		return nil, err
	}

	// Optional explicit element type: for x: T in ...
	var elemTy mir2.Ty = mir2.TyU8
	if p.l.is(tokColon) {
		p.l.next()
		elemTy, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}

	if err := p.l.eatIdent("in"); err != nil {
		return nil, err
	}

	// Parse the base expression (primary + field/call but NOT index — we handle
	// index specially to detect the [start..end] slice form).
	// Use parsePrimary directly to avoid parsePostfix consuming '[' as an index.
	base, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	// Consume dot/call/deref postfixes but stop before [
	base, err = p.parsePostfixNoBrack(base)
	if err != nil {
		return nil, err
	}

	// ── for x: T in ptr[start..end] → ForEachStmt ──
	if p.l.is(tokLBrack) {
		p.l.next()
		// Optional start (default 0)
		var startExpr hir.Expr = &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}
		if !p.l.is(tokDotDot) {
			startExpr, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
		if _, err := p.l.eat(tokDotDot); err != nil {
			return nil, err
		}
		endExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokRBrack); err != nil {
			return nil, err
		}
		// Compute len = end - start; if start == literal 0, len = end directly.
		var lenExpr hir.Expr
		if lit, ok := startExpr.(*hir.IntLitExpr); ok && lit.Val == 0 {
			lenExpr = endExpr
		} else {
			lenExpr = &hir.BinExpr{Op: "-", L: endExpr, R: startExpr, Ty: endExpr.ExprTy()}
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &hir.ForEachStmt{
			Var: varTok.val, ElemTy: elemTy,
			Ptr: base, Start: startExpr, Len: lenExpr,
			Body: body,
		}, nil
	}

	// ── for i in start..end → ForRangeStmt ──
	// base is the start expression here.
	if p.l.is(tokDotDot) {
		p.l.next()
		end, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &hir.ForRangeStmt{Var: varTok.val, Start: base, End: end, Body: body}, nil
	}

	return nil, fmt.Errorf("line %d: for: expected ptr[start..end] or start..end after 'in'", p.l.peek().line)
}

// parsePostfixNoBrack is like parsePostfix but stops before '['.
// Used by parseFor to detect whether the next token is a range slice.
func (p *parser) parsePostfixNoBrack(base hir.Expr) (hir.Expr, error) {
	for {
		t := p.l.peek()
		switch t.kind {
		case tokCaret:
			p.l.next()
			base = &hir.LoadExpr{Ptr: base, Ty: mir2.TyU8}
		case tokDot:
			p.l.next()
			fieldTok, err := p.l.eat(tokIdent)
			if err != nil {
				return nil, err
			}
			base = p.makeFieldExpr(base, fieldTok.val)
		case tokLParen:
			p.l.next()
			var args []hir.Expr
			for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
				a, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, a)
				if p.l.is(tokComma) {
					p.l.next()
				}
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			name := ""
			if vr, ok := base.(*hir.VarRefExpr); ok {
				name = vr.Name
			}
			base = &hir.CallExpr{Fn: name, Args: args, Ty: mir2.TyVoid}
		default:
			return base, nil
		}
	}
}

func (p *parser) parseReturn() (hir.Stmt, error) {
	if err := p.l.eatIdent("return"); err != nil {
		return nil, err
	}
	// No value if next is } or EOF
	if p.l.is(tokRBrace) || p.l.is(tokEOF) {
		return &hir.ReturnStmt{}, nil
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &hir.ReturnStmt{Val: val}, nil
}

func (p *parser) parseSwitch() (hir.Stmt, error) {
	if err := p.l.eatIdent("switch"); err != nil {
		return nil, err
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.l.eat(tokLBrace); err != nil {
		return nil, err
	}
	s := &hir.SwitchStmt{Val: val}
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		if p.l.isIdent("case") {
			p.l.next()
			intTok, err := p.l.eat(tokInt)
			if err != nil {
				return nil, err
			}
			v, _ := strconv.ParseInt(intTok.val, 0, 64)
			if _, err := p.l.eat(tokColon); err != nil {
				return nil, err
			}
			var body []hir.Stmt
			for !p.l.isIdent("case") && !p.l.isIdent("default") && !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
				st, err := p.parseStmt()
				if err != nil {
					return nil, err
				}
				body = append(body, st)
			}
			s.Cases = append(s.Cases, &hir.SwitchCase{Val: v, Body: &hir.Block{Body: body}})
		} else if p.l.isIdent("default") {
			p.l.next()
			if _, err := p.l.eat(tokColon); err != nil {
				return nil, err
			}
			var body []hir.Stmt
			for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
				st, err := p.parseStmt()
				if err != nil {
					return nil, err
				}
				body = append(body, st)
			}
			s.Default = &hir.Block{Body: body}
		} else {
			break
		}
	}
	if _, err := p.l.eat(tokRBrace); err != nil {
		return nil, err
	}
	return s, nil
}

func (p *parser) parseExprStmt() (hir.Stmt, error) {
	lhs, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	// Assignment?
	if p.l.is(tokEq) {
		p.l.next()
		rhs, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		switch lv := lhs.(type) {
		case *hir.LoadExpr:
			// ptr^ = val → StoreStmt
			return &hir.StoreStmt{Ptr: lv.Ptr, Val: rhs}, nil
		case *hir.IndexExpr:
			// arr[i] = val → AssignStmt (HIR lowerer handles ptr arithmetic)
			return &hir.AssignStmt{Target: lv, Val: rhs}, nil
		default:
			return &hir.AssignStmt{Target: lhs, Val: rhs}, nil
		}
	}
	return &hir.ExprStmt{Expr: lhs}, nil
}

// ── Expression parser (Pratt) ─────────────────────────────────────────────────

func (p *parser) parseExpr() (hir.Expr, error) { return p.parseBinary(0) }

type binop struct {
	op   string
	prec int
}

var binops = map[tokKind]binop{
	tokPipe:    {"|", 1},
	tokCaret:   {"^", 2},  // bitwise XOR when used as infix
	tokAmp:     {"&", 3},
	tokEqEq:    {"==", 4},
	tokBangEq:  {"!=", 4},
	tokLt:      {"<", 5},
	tokLtEq:    {"<=", 5},
	tokGt:      {">", 5},
	tokGtEq:    {">=", 5},
	tokLtLt:    {"<<", 6},
	tokGtGt:    {">>", 6},
	tokPlus:    {"+", 7},
	tokMinus:   {"-", 7},
	tokStar:    {"*", 8},
	tokSlash:   {"/", 8},
	tokPercent: {"%", 8},
}

func (p *parser) parseBinary(minPrec int) (hir.Expr, error) {
	lhs, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.l.peek()
		bo, ok := binops[t.kind]
		if !ok || bo.prec <= minPrec {
			break
		}
		// ^ is ambiguous: as infix it's XOR; as prefix it's deref.
		// We only treat ^ as infix here (we're in parseBinary, so lhs is already parsed).
		p.l.next()
		rhs, err := p.parseBinary(bo.prec)
		if err != nil {
			return nil, err
		}
		// Operator overloading: if lhs is a struct type and this op has an overload,
		// emit a CallExpr instead of BinExpr.
		if ov, hasOv := p.opTable[bo.op]; hasOv {
			if _, isStruct := p.exprTy(lhs).(*mir2.StructTy); isStruct {
				lhs = &hir.CallExpr{Fn: ov.funcName, Args: []hir.Expr{lhs, rhs}, Ty: ov.retTy}
				continue
			}
		}
		ty := resultTy(lhs.ExprTy(), rhs.ExprTy(), bo.op)
		lhs = &hir.BinExpr{Op: bo.op, L: lhs, R: rhs, Ty: ty}
	}
	return lhs, nil
}

func (p *parser) parseUnary() (hir.Expr, error) {
	t := p.l.peek()
	switch t.kind {
	case tokMinus:
		p.l.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "-", X: x, Ty: x.ExprTy()}, nil
	case tokBang:
		p.l.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "!", X: x, Ty: mir2.TyBool}, nil
	case tokTilde:
		p.l.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "~", X: x, Ty: x.ExprTy()}, nil
	case tokAmp:
		// &name — address-of
		p.l.next()
		nameTok, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, err
		}
		return &hir.AddrOfExpr{Sym: nameTok.val}, nil
	}
	return p.parsePostfixFull()
}

func (p *parser) parsePostfixFull() (hir.Expr, error) {
	base, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	return p.parsePostfix(base)
}

func (p *parser) parsePostfix(base hir.Expr) (hir.Expr, error) {
	for {
		t := p.l.peek()
		switch t.kind {
		case tokCaret:
			// expr^ — postfix dereference (load)
			p.l.next()
			base = &hir.LoadExpr{Ptr: base, Ty: mir2.TyU8}
		case tokLBrack:
			// base[idx] — index (range slices are handled by parseFor directly)
			p.l.next()
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRBrack); err != nil {
				return nil, err
			}
			base = &hir.IndexExpr{Base: base, Idx: idx, ElemTy: mir2.TyU8}
		case tokDot:
			// base.field    — struct field access
			// base.method() — UFCS method call: rewritten to method(base, args...)
			//                 If base's type is a struct with a registered method, use
			//                 the mangled name (e.g. Vec2_add). Otherwise use fieldName.
			p.l.next()
			fieldTok, err := p.l.eat(tokIdent)
			if err != nil {
				return nil, err
			}
			if p.l.is(tokLParen) {
				// Method call: base.method(a, b) → method(base, a, b)
				p.l.next()
				args := []hir.Expr{base}
				for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
					a, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if p.l.is(tokComma) {
						p.l.next()
					}
				}
				if _, err := p.l.eat(tokRParen); err != nil {
					return nil, err
				}
				// Type-aware dispatch: if base is a struct with this method registered,
				// use the mangled name and known return type.
				callName := fieldTok.val
				callRetTy := mir2.Ty(mir2.TyVoid)
				if st, ok := p.exprTy(base).(*mir2.StructTy); ok {
					if info, found := p.methodTable[st.Name][fieldTok.val]; found {
						callName = info.funcName
						callRetTy = info.retTy
					}
				}
				// Interface dispatch: if base is a variable with a known interface type,
				// resolve to the unique implementing struct's method.
				if callName == fieldTok.val {
					if vr, ok2 := base.(*hir.VarRefExpr); ok2 {
						ifaceName := p.varInterfaceTypes[vr.Name]
						if ifaceName == "" {
							ifaceName = p.globalInterfaceTypes[vr.Name]
						}
						if ifaceName != "" {
							impls := p.findImplementors(ifaceName, fieldTok.val)
							switch len(impls) {
							case 1:
								if info, found := p.methodTable[impls[0]][fieldTok.val]; found {
									callName = info.funcName
									callRetTy = info.retTy
								}
							case 0:
								return nil, fmt.Errorf("line %d: no type in this module implements interface %s method %q",
									fieldTok.line, ifaceName, fieldTok.val)
							default:
								return nil, fmt.Errorf("line %d: ambiguous dispatch: %s.%s() — multiple types implement %s: %v; use concrete type",
									fieldTok.line, vr.Name, fieldTok.val, ifaceName, impls)
							}
						}
					}
				}
				base = &hir.CallExpr{Fn: callName, Args: args, Ty: callRetTy}
			} else {
				base = p.makeFieldExpr(base, fieldTok.val)
			}
		case tokLParen:
			// call: base must be VarRefExpr (function name)
			p.l.next()
			var args []hir.Expr
			for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
				a, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, a)
				if p.l.is(tokComma) {
					p.l.next()
				}
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			name := ""
			if vr, ok := base.(*hir.VarRefExpr); ok {
				name = vr.Name
			}
			base = &hir.CallExpr{Fn: name, Args: args, Ty: mir2.TyVoid}
		default:
			return base, nil
		}
	}
}

func (p *parser) parsePrimary() (hir.Expr, error) {
	t := p.l.peek()

	switch t.kind {
	case tokInt:
		p.l.next()
		v, err := strconv.ParseInt(t.val, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: bad integer %q: %v", t.line, t.val, err)
		}
		ty := mir2.Ty(mir2.TyU8)
		if v > 255 || v < 0 {
			ty = mir2.TyU16
		}
		return &hir.IntLitExpr{Val: v, Ty: ty}, nil

	case tokIdent:
		switch t.val {
		case "true":
			p.l.next()
			return &hir.BoolLitExpr{Val: true}, nil
		case "false":
			p.l.next()
			return &hir.BoolLitExpr{Val: false}, nil
		case "u8", "u16", "i8", "i16":
			// cast: u8(expr)
			p.l.next()
			ty := map[string]mir2.Ty{
				"u8": mir2.TyU8, "u16": mir2.TyU16,
				"i8": mir2.TyI8, "i16": mir2.TyI16,
			}[t.val]
			if _, err := p.l.eat(tokLParen); err != nil {
				return nil, err
			}
			x, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			return &hir.CastExpr{X: x, Ty: ty}, nil
		}
		p.l.next()
		ty := mir2.Ty(mir2.TyU8)
		if t2, ok := p.varTypes[t.val]; ok {
			ty = t2
		} else if t2, ok := p.globalTypes[t.val]; ok {
			ty = t2
		}
		return &hir.VarRefExpr{Name: t.val, Ty: ty}, nil

	case tokLParen:
		p.l.next()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokRParen); err != nil {
			return nil, err
		}
		return e, nil

	case tokString:
		p.l.next()
		return &hir.AddrOfExpr{Sym: "@str." + strings.ReplaceAll(t.val, " ", "_")}, nil

	case tokPipe:
		// |params| expr  or  |params| { stmts }  — non-capturing lambda
		return p.parseLambda()

	case tokAt:
		// @ptr(T, addr) — typed constant pointer to absolute address
		p.l.next()
		if !p.l.isIdent("ptr") {
			return nil, fmt.Errorf("line %d: expected @ptr(...), got @%s", t.line, p.l.peek().val)
		}
		p.l.next()
		if _, err := p.l.eat(tokLParen); err != nil {
			return nil, err
		}
		elemTy, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokComma); err != nil {
			return nil, err
		}
		addrExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokRParen); err != nil {
			return nil, err
		}
		addr := uint16(0)
		if lit, ok := addrExpr.(*hir.IntLitExpr); ok {
			addr = uint16(lit.Val)
		}
		return &hir.ConstPtrExpr{ElemTy: elemTy, Addr: addr}, nil
	}

	return nil, fmt.Errorf("line %d: unexpected token %q in expression", t.line, t.val)
}

// ── Lambda ────────────────────────────────────────────────────────────────────

// parseLambda parses a non-capturing lambda expression: |params| expr or |params| { stmts }.
//
// The lambda is desugared into an anonymous top-level function "lambda_N" and the
// expression returns a VarRefExpr naming that function. Zero-cost: no closure, no
// heap allocation. Params default to u8 if the type annotation is omitted.
//
//	lambda = '|' [param (',' param)*] '|' ('{' stmt* '}' | expr)
//	param  = IDENT [':' type]
func (p *parser) parseLambda() (hir.Expr, error) {
	if _, err := p.l.eat(tokPipe); err != nil {
		return nil, err
	}

	// Parse parameter list: |x: u8, y: u8|  or  |x, y|  or  ||
	var params []hir.Param
	for !p.l.is(tokPipe) && !p.l.is(tokEOF) {
		pname, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, fmt.Errorf("line %d: lambda: expected parameter name: %w", pname.line, err)
		}
		pty := mir2.Ty(mir2.TyU8) // default type
		if p.l.is(tokColon) {
			p.l.next()
			pty, err = p.parseType()
			if err != nil {
				return nil, err
			}
		}
		params = append(params, hir.Param{Name: pname.val, Ty: pty})
		if p.l.is(tokComma) {
			p.l.next()
		}
	}
	if _, err := p.l.eat(tokPipe); err != nil {
		return nil, fmt.Errorf("lambda: expected closing '|': %w", err)
	}

	// Parse body: block { ... } or single expression → implicit return
	var body *hir.Block
	if p.l.is(tokLBrace) {
		var err error
		body, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	} else {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		body = &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: expr}}}
	}

	// Infer return type from single-expression body.
	retTy := mir2.Ty(mir2.TyVoid)
	if len(body.Body) == 1 {
		if rs, ok := body.Body[0].(*hir.ReturnStmt); ok && rs.Val != nil {
			retTy = rs.Val.ExprTy()
		}
	}

	// Generate a unique name and register the anonymous function.
	name := fmt.Sprintf("lambda_%d", p.lambdaCount)
	p.lambdaCount++
	p.lambdas = append(p.lambdas, &hir.Func{
		Name:   name,
		Params: params,
		RetTy:  retTy,
		Body:   body,
	})

	return &hir.VarRefExpr{Name: name, Ty: retTy}, nil
}

// ── Type helpers ──────────────────────────────────────────────────────────────

// makeFieldExpr builds a FieldExpr for base.fieldName, computing the byte
// offset from the known struct layout when the base type is a known struct.
//
// For struct Vec2 { x: u8, y: u8 }:
//   - v.x → FieldExpr{Offset: 0, Ty: u8}
//   - v.y → FieldExpr{Offset: 1, Ty: u8}
//
// If the base type is not a known struct (unknown variable, or non-struct type),
// the offset defaults to 0 and the field type defaults to u8. This is safe:
// the lowerer only emits an extra OpPtrAdd when offset > 0.
func (p *parser) makeFieldExpr(base hir.Expr, fieldName string) *hir.FieldExpr {
	fieldTy := mir2.Ty(mir2.TyU8)
	fieldOffset := 0

	if st, ok := p.exprTy(base).(*mir2.StructTy); ok {
		byteOffset := 0
		for _, f := range st.Fields {
			if f.Name == fieldName {
				fieldTy = f.Ty
				fieldOffset = byteOffset
				break
			}
			byteOffset += f.Ty.Width() / 8
		}
	}

	return &hir.FieldExpr{X: base, Field: fieldName, Offset: fieldOffset, Ty: fieldTy}
}

func resultTy(l, r mir2.Ty, op string) mir2.Ty {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return mir2.TyBool
	}
	if l == mir2.TyU16 || r == mir2.TyU16 {
		return mir2.TyU16
	}
	if l == mir2.TyPtr || r == mir2.TyPtr {
		return mir2.TyPtr
	}
	return l
}
