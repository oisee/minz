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
	pos  int // byte offset in source where this token starts
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
	l.tokens = append(l.tokens, token{kind: k, val: v, line: line, pos: l.pos})
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
// peekN returns the n-th token ahead without consuming any (0 == peek()).
func (l *lexer) peekN(n int) token {
	idx := l.cur + n
	if idx < len(l.tokens) {
		return l.tokens[idx]
	}
	return token{kind: tokEOF}
}
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

// processStringEscapes converts escape sequences in a raw string literal value
// (the content between the quotes, as captured by the lexer) to their byte values.
func processStringEscapes(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '0':
				b.WriteByte(0)
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
		i++
	}
	return b.String()
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
	module      *hir.Module // current module being built (set in parseModule)
	structs     map[string]*mir2.StructTy
	interfaces  map[string]*hir.InterfaceDecl   // interface name → declaration
	lambdas     []*hir.Func // anonymous functions generated from |params| body syntax
	lambdaCount int
	// Week 1: UFCS + struct methods + operator overloading
	globalTypes          map[string]mir2.Ty              // module-level: global varname → type (persistent)
	globalInterfaceTypes map[string]string               // module-level: global varname → interface name
	varTypes             map[string]mir2.Ty              // current function scope: varname → type (reset per func)
	varInterfaceTypes    map[string]string               // current function scope: varname → interface name
	varPtrElem           map[string]*mir2.StructTy       // current function scope: varname → pointed-to struct (for ^Struct params)
	methodTable          map[string]map[string]methodInfo // structName → methodName → info
	opTable     map[string]opOverload             // op symbol ("+", "-", ...) → overload
	// use-before-init analysis
	uninitVars  map[string]int  // varname → declaration line; nil disables tracking (at branches)
	warnings    []string        // accumulated diagnostic warnings
	// function return type table — populated as functions are parsed so that
	// call expressions can get the correct Ty instead of TyVoid.
	funcSigs map[string]mir2.Ty
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
	p.module = m
	p.structs = make(map[string]*mir2.StructTy)
	p.interfaces = make(map[string]*hir.InterfaceDecl)
	p.methodTable = make(map[string]map[string]methodInfo)
	p.opTable = make(map[string]opOverload)
	p.funcSigs = make(map[string]mir2.Ty)
	p.globalTypes = make(map[string]mir2.Ty)
	p.globalInterfaceTypes = make(map[string]string)
	p.varTypes = make(map[string]mir2.Ty)
	p.varInterfaceTypes = make(map[string]string)
	p.varPtrElem = make(map[string]*mir2.StructTy)
	p.uninitVars = make(map[string]int)

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
			// @extern fun ...  or  @extern(0xNNNN) fun ...
			p.l.next()
			attr := p.l.peek()
			if attr.kind == tokIdent && attr.val == "extern" {
				p.l.next()
				// Optional address: @extern(0xNNNN) fun ...
				var externAddr uint16
				if p.l.is(tokLParen) {
					p.l.next()
					addrTok, err := p.l.eat(tokInt)
					if err != nil {
						return nil, fmt.Errorf("line %d: expected address in @extern(...)", attr.line)
					}
					addr64, err := strconv.ParseUint(addrTok.val, 0, 16)
					if err != nil {
						return nil, fmt.Errorf("line %d: invalid address %q: %v", addrTok.line, addrTok.val, err)
					}
					externAddr = uint16(addr64)
					if _, err := p.l.eat(tokRParen); err != nil {
						return nil, err
					}
				}
				if err := p.l.eatIdent("fun"); err != nil {
					return nil, err
				}
				f, err := p.parseFunDecl(true)
				if err != nil {
					return nil, err
				}
				f.ExternAddr = externAddr
				m.Funcs = append(m.Funcs, f)
			} else {
				return nil, fmt.Errorf("line %d: unexpected @%s", attr.line, attr.val)
			}

		case t.kind == tokIdent && t.val == "assert":
			a, err := p.parseAssert()
			if err != nil {
				return nil, err
			}
			m.Asserts = append(m.Asserts, a)

		default:
			return nil, fmt.Errorf("line %d: unexpected token %q at module level", t.line, t.val)
		}
	}
	// Append lambdas generated during parsing (non-capturing anonymous functions).
	m.Funcs = append(m.Funcs, p.lambdas...)
	m.Warnings = p.warnings
	return m, nil
}

// parseAssert parses: assert fn(arg, ...) == expected
// or: assert expected == fn(arg, ...)
// Only literal integer arguments and expected values are supported.
func (p *parser) parseAssert() (hir.Assert, error) {
	tok := p.l.peek()
	line := tok.line
	if err := p.l.eatIdent("assert"); err != nil {
		return hir.Assert{}, err
	}

	// Parse: ident(args...)
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return hir.Assert{}, fmt.Errorf("line %d: assert: expected function name", line)
	}
	if _, err2 := p.l.eat(tokLParen); err2 != nil {
		return hir.Assert{}, fmt.Errorf("line %d: assert: expected '(' after function name", line)
	}

	var args []int64
	for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
		if len(args) > 0 {
			if _, err2 := p.l.eat(tokComma); err2 != nil {
				return hir.Assert{}, fmt.Errorf("line %d: assert: expected ',' between arguments", line)
			}
		}
		// Optionally negative.
		neg := false
		if p.l.is(tokMinus) {
			p.l.next()
			neg = true
		}
		intTok, err2 := p.l.eat(tokInt)
		if err2 != nil {
			return hir.Assert{}, fmt.Errorf("line %d: assert: expected integer literal argument", line)
		}
		v, err3 := strconv.ParseInt(intTok.val, 0, 64)
		if err3 != nil {
			return hir.Assert{}, fmt.Errorf("line %d: assert: invalid integer %q", line, intTok.val)
		}
		if neg {
			v = -v
		}
		args = append(args, v)
	}
	if _, err2 := p.l.eat(tokRParen); err2 != nil {
		return hir.Assert{}, fmt.Errorf("line %d: assert: expected ')'", line)
	}

	// Parse: == expected  OR  == (v1, v2, ...)
	if _, err2 := p.l.eat(tokEqEq); err2 != nil {
		return hir.Assert{}, fmt.Errorf("line %d: assert: expected '==' after call", line)
	}

	// Multi-return tuple: == (v1, v2)
	if p.l.is(tokLParen) {
		p.l.next() // consume '('
		var multi []int64
		for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
			if len(multi) > 0 {
				if _, err2 := p.l.eat(tokComma); err2 != nil {
					return hir.Assert{}, fmt.Errorf("line %d: assert: expected ',' in tuple", line)
				}
			}
			neg2 := false
			if p.l.is(tokMinus) {
				p.l.next()
				neg2 = true
			}
			vTok, err2 := p.l.eat(tokInt)
			if err2 != nil {
				return hir.Assert{}, fmt.Errorf("line %d: assert: expected integer in tuple", line)
			}
			v, err3 := strconv.ParseInt(vTok.val, 0, 64)
			if err3 != nil {
				return hir.Assert{}, fmt.Errorf("line %d: assert: invalid integer %q", line, vTok.val)
			}
			if neg2 {
				v = -v
			}
			multi = append(multi, v)
		}
		if _, err2 := p.l.eat(tokRParen); err2 != nil {
			return hir.Assert{}, fmt.Errorf("line %d: assert: expected ')' after tuple", line)
		}
		via := p.parseAssertVia()
		src := fmt.Sprintf("assert %s(%s) == (%s)", nameTok.val, intSliceStr(args), intSliceStr(multi))
		if via != "" {
			src += " via " + via
		}
		return hir.Assert{
			FuncName:      nameTok.val,
			Args:          args,
			ExpectedMulti: multi,
			Source:        src,
			Line:          line,
			Via:           via,
		}, nil
	}

	// Single-value expected
	neg := false
	if p.l.is(tokMinus) {
		p.l.next()
		neg = true
	}
	expTok, err := p.l.eat(tokInt)
	if err != nil {
		return hir.Assert{}, fmt.Errorf("line %d: assert: expected integer literal after '=='", line)
	}
	expected, err := strconv.ParseInt(expTok.val, 0, 64)
	if err != nil {
		return hir.Assert{}, fmt.Errorf("line %d: assert: invalid integer %q", line, expTok.val)
	}
	if neg {
		expected = -expected
	}

	via := p.parseAssertVia()
	src := fmt.Sprintf("assert %s(%s) == %d", nameTok.val, intSliceStr(args), expected)
	if via != "" {
		src += " via " + via
	}
	return hir.Assert{
		FuncName: nameTok.val,
		Args:     args,
		Expected: expected,
		Source:   src,
		Line:     line,
		Via:      via,
	}, nil
}

// parseAssertVia consumes an optional "via mir2" or "via z80" suffix.
// Returns "" if no via clause is present.
func (p *parser) parseAssertVia() string {
	if !p.l.is(tokIdent) || p.l.peek().val != "via" {
		return ""
	}
	p.l.next() // consume "via"
	if !p.l.is(tokIdent) {
		return ""
	}
	target := p.l.peek().val
	if target == "mir2" || target == "z80" {
		p.l.next()
		return target
	}
	return ""
}

func intSliceStr(vs []int64) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = strconv.FormatInt(v, 10)
	}
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += ", "
		}
		s += p
	}
	return s
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
	var paramIfaceNames []string        // parallel to params: interface name or ""
	var paramPtrElems []*mir2.StructTy  // parallel to params: ^Struct elem type or nil
	for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
		// Check for @z80_X / @smc annotation BEFORE param name
		var regClass mir2.RegClass
		var smcParam bool
		if p.l.is(tokAt) {
			p.l.next()
			annot := p.l.peek()
			if annot.kind == tokIdent {
				switch annot.val {
				case "smc":
					smcParam = true
					p.l.next()
				case "z80_a":
					regClass = mir2.ClassAcc
					p.l.next()
				case "z80_hl":
					regClass = mir2.ClassPointer
					p.l.next()
				case "z80_de":
					regClass = mir2.ClassIndex
					p.l.next()
				case "z80_b":
					regClass = mir2.ClassCounter
					p.l.next()
				case "z80_c":
					regClass = mir2.ClassGeneral
					p.l.next()
				default:
					return nil, fmt.Errorf("line %d: unknown register annotation @%s", annot.line, annot.val)
				}
			}
		}
		pname, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.l.eat(tokColon); err != nil {
			return nil, err
		}
		pty, ifaceName, ptrElem, err := p.parseParamType()
		if err != nil {
			return nil, err
		}
		params = append(params, hir.Param{Name: pname.val, Ty: pty, RegClass: regClass, SMC: smcParam})
		paramIfaceNames = append(paramIfaceNames, ifaceName)
		paramPtrElems = append(paramPtrElems, ptrElem)
		if p.l.is(tokComma) {
			p.l.next()
		}
	}
	if _, err := p.l.eat(tokRParen); err != nil {
		return nil, err
	}

	var err error
	retTy := mir2.Ty(mir2.TyVoid)
	var retTys []mir2.Ty // non-nil → multi-return
	if p.l.is(tokArrow) {
		p.l.next()
		// -> (T1, T2, …) — multi-return tuple
		if p.l.is(tokLParen) {
			p.l.next()
			for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
				ty, err2 := p.parseType()
				if err2 != nil {
					return nil, err2
				}
				retTys = append(retTys, ty)
				if p.l.is(tokComma) {
					p.l.next()
				}
			}
			if _, err = p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			if len(retTys) > 0 {
				retTy = retTys[0] // keep RetTy as primary for compatibility
			}
		} else {
			retTy, err = p.parseType()
			if err != nil {
				return nil, err
			}
		}
	}

	// Register function signature for call-site typing.
	p.funcSigs[funcName] = retTy

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
	p.varPtrElem = make(map[string]*mir2.StructTy)
	p.uninitVars = make(map[string]int) // fresh tracking per function
	for i, param := range params {
		p.varTypes[param.Name] = param.Ty
		if i < len(paramIfaceNames) && paramIfaceNames[i] != "" {
			p.varInterfaceTypes[param.Name] = paramIfaceNames[i]
		}
		if i < len(paramPtrElems) && paramPtrElems[i] != nil {
			p.varPtrElem[param.Name] = paramPtrElems[i]
		}
	}

	f := &hir.Func{
		Name:     funcName,
		Params:   params,
		RetTy:    retTy,
		RetTys:   retTys,
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
		case "u24":
			base = mir2.TyU24
		case "u32":
			base = mir2.TyU32
		case "i8":
			base = mir2.TyI8
		case "i16":
			base = mir2.TyI16
		case "i24":
			base = mir2.TyI24
		case "i32":
			base = mir2.TyI32
		case "bool":
			base = mir2.TyBool
		case "void":
			return mir2.TyVoid, nil
		case "ptr":
			return mir2.TyPtr, nil
		case "f", "f8", "f16":
			// Fixed-point types reserved; arithmetic semantics (>>fracBits after mul) not yet codegen'd.
			return nil, fmt.Errorf("line %d: fixed-point type %q not yet available in Nanz (coming soon)", t.line, t.val)
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

// parseParamType parses a parameter type and returns (ty, ifaceName, ptrElemSt, error).
// Handles three cases:
//   - Interface name (e.g. "Animal") → (TyPtr, "Animal", nil, nil)
//   - ^StructName (e.g. ^Acc)        → (TyPtr, "", *StructTy, nil)
//   - Any other type                 → (ty, "", nil, err)
func (p *parser) parseParamType() (mir2.Ty, string, *mir2.StructTy, error) {
	t := p.l.peek()
	// Interface name?
	if t.kind == tokIdent {
		if _, ok := p.interfaces[t.val]; ok {
			p.l.next()
			return mir2.TyPtr, t.val, nil, nil
		}
	}
	// ^T — pointer type
	if t.kind == tokCaret {
		p.l.next()
		// Peek at inner type: is it a known struct?
		var elemSt *mir2.StructTy
		if next := p.l.peek(); next.kind == tokIdent {
			if st, ok := p.structs[next.val]; ok {
				elemSt = st
			}
		}
		// Consume inner type
		if !p.l.is(tokEOF) && !p.l.is(tokComma) && !p.l.is(tokRParen) &&
			!p.l.is(tokLBrace) && !p.l.is(tokRBrace) && !p.l.is(tokSemi) &&
			!p.l.is(tokEq) && !p.l.is(tokArrow) {
			_, _ = p.parseType()
		}
		return mir2.TyPtr, "", elemSt, nil
	}
	ty, err := p.parseType()
	return ty, "", nil, err
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
	case t.kind == tokIdent && t.val == "asm":
		return p.parseAsm()
	case t.kind == tokLBrace:
		return p.parseBlock()
	default:
		// expr (= expr)? — assignment or bare call
		return p.parseExprStmt()
	}
}

// parseLetDecl parses:
//   - let name [: T] = expr           — single binding
//   - let (a, b [, …]) = fn(args)     — tuple destructuring of multi-return call
//
// Type is inferred from the RHS if not specified.
func (p *parser) parseLetDecl() (hir.Stmt, error) {
	if err := p.l.eatIdent("let"); err != nil {
		return nil, err
	}

	// Tuple destructuring: let (a, b) = fn(...)
	if p.l.is(tokLParen) {
		return p.parseTupleLet()
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
	// Warn if any uninit var is used in the init expression.
	p.warnUninitInExpr(init)
	// 'let' always provides an initializer — the declared name is init'd from the start.
	if p.uninitVars != nil {
		delete(p.uninitVars, nameTok.val)
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

// parseTupleLet parses: let (a, b [, c]) = fn(args)
// The RHS must be a call expression.  Types are inferred from the callee's
// RetTys if available, otherwise left as TyVoid (lowerer will use ExtraRetTys).
func (p *parser) parseTupleLet() (hir.Stmt, error) {
	if _, err := p.l.eat(tokLParen); err != nil {
		return nil, err
	}
	var names []string
	for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
		tok := p.l.peek()
		if tok.kind == tokIdent {
			p.l.next()
			names = append(names, tok.val) // includes "_" as a valid identifier
		} else {
			return nil, fmt.Errorf("expected identifier in tuple binding, got %v", tok)
		}
		if p.l.is(tokComma) {
			p.l.next()
		}
	}
	if _, err := p.l.eat(tokRParen); err != nil {
		return nil, err
	}
	if _, err := p.l.eat(tokEq); err != nil {
		return nil, err
	}
	// RHS must be a call expression.
	callExpr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	ce, ok := callExpr.(*hir.CallExpr)
	if !ok {
		return nil, fmt.Errorf("right-hand side of tuple let must be a function call, got %T", callExpr)
	}

	// Determine types from known function declaration (if available).
	tys := make([]mir2.Ty, len(names))
	if f := p.module.FuncByName(ce.Fn); f != nil && len(f.RetTys) > 0 {
		for i := range tys {
			if i < len(f.RetTys) {
				tys[i] = f.RetTys[i]
			} else {
				tys[i] = mir2.TyU16 // default
			}
		}
	} else {
		// Fallback: guess u16 for all positions.
		for i := range tys {
			tys[i] = mir2.TyU16
		}
	}

	// Register bound names in varTypes.
	for i, name := range names {
		if name != "_" {
			p.varTypes[name] = tys[i]
			if p.uninitVars != nil {
				delete(p.uninitVars, name)
			}
		}
	}

	return &hir.TupleLetStmt{Names: names, Tys: tys, Call: ce}, nil
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

	// Track this var as uninitialized until we see an init or at(addr).
	if p.uninitVars != nil {
		p.uninitVars[nameTok.val] = nameTok.line
	}

	// at(addr)?
	if p.l.isIdent("at") {
		// Memory-mapped variable — treated as initialized (address is the init).
		if p.uninitVars != nil {
			delete(p.uninitVars, nameTok.val)
		}
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
		// Has explicit initializer — no longer uninitialized.
		if p.uninitVars != nil {
			delete(p.uninitVars, nameTok.val)
		}
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

// parseCondExpr parses an if-as-expression:
//
//	if cond { val } else { val }
//
// The body of each branch must be a single return statement or a single expression.
// Returns a *hir.CondExpr suitable for use as an initializer in let/var declarations.
func (p *parser) parseCondExpr() (hir.Expr, error) {
	if err := p.l.eatIdent("if"); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	thenBlock, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	if !p.l.isIdent("else") {
		return nil, fmt.Errorf("line %d: if-expression requires an else branch", p.l.peek().line)
	}
	p.l.next() // consume "else"
	elseBlock, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	// Extract the single expression from each branch (must be a return or bare expr).
	thenExpr := extractBlockExpr(thenBlock)
	if thenExpr == nil {
		return nil, fmt.Errorf("if-expression then-branch must contain a single expression or return")
	}
	elseExpr := extractBlockExpr(elseBlock)
	if elseExpr == nil {
		return nil, fmt.Errorf("if-expression else-branch must contain a single expression or return")
	}
	ty := thenExpr.ExprTy()
	if ty == nil {
		ty = elseExpr.ExprTy()
	}
	return &hir.CondExpr{Cond: cond, Then: thenExpr, Else: elseExpr, Ty: ty}, nil
}

// extractBlockExpr returns the single expression from a block that contains
// either a ReturnStmt or an ExprStmt, or nil if the block doesn't match.
func extractBlockExpr(blk *hir.Block) hir.Expr {
	if len(blk.Body) != 1 {
		return nil
	}
	switch s := blk.Body[0].(type) {
	case *hir.ReturnStmt:
		return s.Val
	case *hir.ExprStmt:
		return s.Expr
	}
	return nil
}

func (p *parser) parseIf() (hir.Stmt, error) {
	if err := p.l.eatIdent("if"); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.warnUninitInExpr(cond)
	// Stop tracking after a branch — conservative, avoids false positives.
	p.uninitVars = nil
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
	p.warnUninitInExpr(cond)
	p.uninitVars = nil // stop tracking inside loops (conservative)
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
	p.warnUninitInExpr(base)
	p.uninitVars = nil // stop tracking inside loops (conservative)

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
			isStructPtr := false
			if vr, ok := base.(*hir.VarRefExpr); ok {
				isStructPtr = p.varPtrElem[vr.Name] != nil
			}
			if !isStructPtr {
				base = &hir.LoadExpr{Ptr: base, Ty: mir2.TyU8}
			}
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
			callTy := mir2.Ty(mir2.TyVoid)
			if name != "" {
				if ty, ok := p.funcSigs[name]; ok {
					callTy = ty
				}
			}
			base = &hir.CallExpr{Fn: name, Args: args, Ty: callTy}
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
	// Multi-return: return (e1, e2, …)
	// Disambiguate from a parenthesised single expression: if we see '(' followed
	// by an expression and then a comma, it's a tuple return.
	if p.l.is(tokLParen) {
		// Speculatively parse as tuple. We check for comma after first element.
		p.l.next() // consume '('
		first, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.l.is(tokComma) {
			// It's a multi-return tuple.
			vals := []hir.Expr{first}
			for p.l.is(tokComma) {
				p.l.next()
				v, err2 := p.parseExpr()
				if err2 != nil {
					return nil, err2
				}
				vals = append(vals, v)
			}
			if _, err2 := p.l.eat(tokRParen); err2 != nil {
				return nil, err2
			}
			for _, v := range vals {
				p.warnUninitInExpr(v)
			}
			return &hir.ReturnStmt{Vals: vals}, nil
		}
		// Single parenthesised expression — consume ')' and treat as single return.
		if _, err2 := p.l.eat(tokRParen); err2 != nil {
			return nil, err2
		}
		p.warnUninitInExpr(first)
		return &hir.ReturnStmt{Val: first}, nil
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.warnUninitInExpr(val)
	return &hir.ReturnStmt{Val: val}, nil
}

// warnUninitInExpr walks a HIR expression and emits use-before-init warnings
// for any VarRefExpr that refers to a variable still in uninitVars.
// Each variable is warned only once (then removed from uninitVars).
// No-op when uninitVars is nil (tracking disabled, e.g. inside branches).
func (p *parser) warnUninitInExpr(e hir.Expr) {
	if p.uninitVars == nil || e == nil {
		return
	}
	switch ex := e.(type) {
	case *hir.VarRefExpr:
		if line, ok := p.uninitVars[ex.Name]; ok {
			p.warnings = append(p.warnings,
				fmt.Sprintf("warning: '%s' used before initialization (declared at line %d)", ex.Name, line))
			delete(p.uninitVars, ex.Name) // warn only once per var
		}
	case *hir.BinExpr:
		p.warnUninitInExpr(ex.L)
		p.warnUninitInExpr(ex.R)
	case *hir.UnaryExpr:
		p.warnUninitInExpr(ex.X)
	case *hir.CallExpr:
		for _, arg := range ex.Args {
			p.warnUninitInExpr(arg)
		}
	case *hir.CastExpr:
		p.warnUninitInExpr(ex.X)
	case *hir.LoadExpr:
		p.warnUninitInExpr(ex.Ptr)
	case *hir.DerefExpr:
		p.warnUninitInExpr(ex.Ptr)
	case *hir.FieldExpr:
		p.warnUninitInExpr(ex.X)
	case *hir.IndexExpr:
		p.warnUninitInExpr(ex.Base)
		p.warnUninitInExpr(ex.Idx)
	// IntLitExpr, BoolLitExpr, AddrOfExpr, ConstPtrExpr: no variable refs
	}
}

// parseAsm parses an inline assembly block:
//
//	asm z80 { LD A, 0x42 / OUT (0x23), A }
//	asm { LD A, 0x42 }              — target = "" (matches any)
//	asm z80 (in n, m) { OUT (0x23), A } — explicit input operands
//
// The braces' byte positions in the source are used to extract the raw asm
// text verbatim (so Z80 syntax like "LD (HL), A" is preserved exactly).
// '/' inside the block acts as a line separator — each '/' becomes '\n    '
// in the output, allowing single-line multi-instruction asm blocks.
//
// Optional operand list (in varname, ...) before '{' tells the IR which
// variables the asm block reads.  This prevents the register allocator from
// moving those variables to a different physical register.
func (p *parser) parseAsm() (hir.Stmt, error) {
	if err := p.l.eatIdent("asm"); err != nil {
		return nil, err
	}
	// Optional target tag (e.g. "z80", "ez80", "6502").
	// Identifiers: "z80", "ez80"; integers: "6502" (tokenized as int).
	target := ""
	if p.l.peek().kind == tokIdent || p.l.peek().kind == tokInt {
		target = p.l.peek().val
		p.l.next()
	}

	// Optional operand list: (in x, y) before the asm body '{'.
	var ins []hir.AsmOperand
	if p.l.is(tokLParen) {
		p.l.next() // consume '('
		if p.l.peek().kind == tokIdent && p.l.peek().val == "in" {
			p.l.next() // consume 'in'
			for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
				name, err := p.l.eat(tokIdent)
				if err != nil {
					return nil, fmt.Errorf("line %d: asm (in): expected variable name", name.line)
				}
				ins = append(ins, hir.AsmOperand{Name: name.val})
				if p.l.is(tokComma) {
					p.l.next()
				}
			}
		}
		if _, err := p.l.eat(tokRParen); err != nil {
			return nil, err
		}
	}

	// The '{' token carries its source byte offset in token.pos.
	lbrace := p.l.peek()
	if lbrace.kind != tokLBrace {
		return nil, fmt.Errorf("line %d: asm block: expected '{', got %q", lbrace.line, lbrace.val)
	}
	p.l.next() // consume '{'
	startPos := lbrace.pos + 1 // byte offset of first char inside the block

	// Walk the token stream to find the matching '}'.
	depth := 1
	var closeTok token
	for depth > 0 {
		t := p.l.peek()
		if t.kind == tokEOF {
			return nil, fmt.Errorf("line %d: asm block: unterminated '{'", lbrace.line)
		}
		if t.kind == tokLBrace {
			depth++
		} else if t.kind == tokRBrace {
			depth--
			if depth == 0 {
				closeTok = t
				break
			}
		}
		p.l.next()
	}
	p.l.next() // consume closing '}'

	// Extract raw source between the braces.
	code := strings.TrimSpace(string(p.l.src[startPos:closeTok.pos]))
	return &hir.AsmStmt{Target: target, Code: code, ClobberAll: true, Ins: ins}, nil
}

func (p *parser) parseSwitch() (hir.Stmt, error) {
	if err := p.l.eatIdent("switch"); err != nil {
		return nil, err
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.warnUninitInExpr(val)
	p.uninitVars = nil // stop tracking inside switch (conservative)
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
		// If lhs is a simple var being assigned, it is now initialized — remove from uninit set.
		if vr, ok := lhs.(*hir.VarRefExpr); ok {
			if p.uninitVars != nil {
				delete(p.uninitVars, vr.Name)
			}
		}
		// Warn for any uninit var used in the rhs.
		p.warnUninitInExpr(rhs)
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
	// Not an assignment: lhs is a value expression — check for uninit use.
	p.warnUninitInExpr(lhs)
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
			// expr^ — postfix dereference.
			// For ^Struct pointers, `self^` is transparent: .field access and method
			// calls via varPtrElem already handle field resolution correctly on the
			// pointer itself.  Only emit a LoadExpr for ^primitive (^u8, ^u16) vars.
			p.l.next()
			isStructPtr := false
			if vr, ok := base.(*hir.VarRefExpr); ok {
				isStructPtr = p.varPtrElem[vr.Name] != nil
			}
			if !isStructPtr {
				base = &hir.LoadExpr{Ptr: base, Ty: mir2.TyU8}
			}
			// else: `self^` on ^Struct — keep base as-is; subsequent .field handles it
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
				// Type-aware dispatch: if base is a struct (or ^Struct pointer) with
				// this method registered, use the mangled name and known return type.
				callName := fieldTok.val
				callRetTy := mir2.Ty(mir2.TyVoid)
				if st, ok := p.exprTy(base).(*mir2.StructTy); ok {
					if info, found := p.methodTable[st.Name][fieldTok.val]; found {
						callName = info.funcName
						callRetTy = info.retTy
					}
				}
				// ^Struct pointer receiver: look up method by the pointed-to struct.
				if callName == fieldTok.val {
					if vr, vrOk := base.(*hir.VarRefExpr); vrOk {
						if st2 := p.varPtrElem[vr.Name]; st2 != nil {
							if info, found := p.methodTable[st2.Name][fieldTok.val]; found {
								callName = info.funcName
								callRetTy = info.retTy
							}
						}
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
			callTy := mir2.Ty(mir2.TyVoid)
			if name != "" {
				if ty, ok := p.funcSigs[name]; ok {
					callTy = ty
				}
			}
			base = &hir.CallExpr{Fn: name, Args: args, Ty: callTy}
		default:
			return base, nil
		}
	}
}

// parseStructLit parses the body of a struct literal: { field: expr, ... }
// The struct name has already been consumed by the caller.
func (p *parser) parseStructLit(st *mir2.StructTy) (*hir.StructLitExpr, error) {
	if _, err := p.l.eat(tokLBrace); err != nil {
		return nil, err
	}
	var fields []hir.FieldInit
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		name, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, fmt.Errorf("line %d: struct literal: expected field name", name.line)
		}
		if _, err := p.l.eat(tokColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		fields = append(fields, hir.FieldInit{Name: name.val, Val: val})
		if p.l.is(tokComma) {
			p.l.next()
		}
	}
	if _, err := p.l.eat(tokRBrace); err != nil {
		return nil, err
	}
	return &hir.StructLitExpr{St: st, Fields: fields}, nil
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
		case "if":
			// if-as-expression: if cond { val } else { val }
			// Returns a CondExpr — used in: let x = if c { a } else { b }
			return p.parseCondExpr()
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
		// range(lo..hi) or range(hi..lo) — counter-based iterator source.
		// Produces a RangeSourceExpr; Rev=true when lo > hi (back-counting).
		if t.val == "range" && p.l.peekN(1).kind == tokLParen {
			p.l.next() // consume "range"
			p.l.next() // consume "("
			lo, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err2 := p.l.eat(tokDotDot); err2 != nil {
				return nil, fmt.Errorf("line %d: range: expected lo..hi", t.line)
			}
			hi, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err2 := p.l.eat(tokRParen); err2 != nil {
				return nil, err2
			}
			// Detect back-counting: if lo is an ident/var and hi is literal 0,
			// or hi < lo literally — set Rev=true.
			rev := false
			if lit, ok2 := hi.(*hir.IntLitExpr); ok2 && lit.Val == 0 {
				rev = true
			}
			return &hir.RangeSourceExpr{Lo: lo, Hi: hi, Rev: rev}, nil
		}
		// Struct literal: StructName { field: val, ... }
		if st, ok := p.structs[t.val]; ok && p.l.peekN(1).kind == tokLBrace {
			p.l.next() // consume struct name
			lit, err := p.parseStructLit(st)
			if err != nil {
				return nil, err
			}
			return lit, nil
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
		s := processStringEscapes(t.val)
		// Deduplicate and intern
		idx := -1
		for i, existing := range p.module.Strings {
			if existing == s {
				idx = i
				break
			}
		}
		if idx == -1 {
			idx = len(p.module.Strings)
			p.module.Strings = append(p.module.Strings, s)
		}
		return &hir.AddrOfExpr{Sym: fmt.Sprintf("@mir2.str.%d", idx)}, nil

	case tokPipe:
		// |params| expr  or  |params| { stmts }  — non-capturing lambda
		return p.parseLambda()

	case tokAt:
		// @print(expr), @print_nl(), @print_u8(expr), @ptr(T, addr)
		p.l.next()
		if p.l.isIdent("print") {
			p.l.next()
			if _, err := p.l.eat(tokLParen); err != nil {
				return nil, err
			}
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			return &hir.CallExpr{Fn: "@mir.io.print.str", Args: []hir.Expr{arg}, Ty: mir2.TyVoid}, nil
		}
		if p.l.isIdent("print_nl") {
			p.l.next()
			if p.l.is(tokLParen) {
				p.l.next()
				if _, err := p.l.eat(tokRParen); err != nil {
					return nil, err
				}
			}
			return &hir.CallExpr{Fn: "@mir.io.print.nl", Args: nil, Ty: mir2.TyVoid}, nil
		}
		if p.l.isIdent("print_u8") {
			p.l.next()
			if _, err := p.l.eat(tokLParen); err != nil {
				return nil, err
			}
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			return &hir.CallExpr{Fn: "@mir.io.print.u8", Args: []hir.Expr{arg}, Ty: mir2.TyVoid}, nil
		}
		// @print_dec(expr) — print u8 value as decimal digits via OUT ($23)
		if p.l.isIdent("print_dec") {
			p.l.next()
			if _, err := p.l.eat(tokLParen); err != nil {
				return nil, err
			}
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			return &hir.CallExpr{Fn: "@mir.io.print.dec", Args: []hir.Expr{arg}, Ty: mir2.TyVoid}, nil
		}
		// @console_log(expr) — OUT ($23), A  single raw byte to stdout
		if p.l.isIdent("console_log") {
			p.l.next()
			if _, err := p.l.eat(tokLParen); err != nil {
				return nil, err
			}
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			return &hir.CallExpr{Fn: "@mir.io.console.log", Args: []hir.Expr{arg}, Ty: mir2.TyVoid}, nil
		}
		// @console_err(expr) — OUT (0x02), A  single raw byte to stderr
		if p.l.isIdent("console_err") {
			p.l.next()
			if _, err := p.l.eat(tokLParen); err != nil {
				return nil, err
			}
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			return &hir.CallExpr{Fn: "@mir.io.console.err", Args: []hir.Expr{arg}, Ty: mir2.TyVoid}, nil
		}
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

	// Find the struct type: either the base is directly a struct, or a ^Struct pointer.
	var st *mir2.StructTy
	if s, ok := p.exprTy(base).(*mir2.StructTy); ok {
		st = s
	} else if vr, ok := base.(*hir.VarRefExpr); ok {
		st = p.varPtrElem[vr.Name] // nil if not a ^Struct pointer
	}

	if st != nil {
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
