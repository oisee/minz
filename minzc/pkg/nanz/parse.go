package nanz

// parse.go — hand-written recursive-descent parser.
// Nanz source → hir.Module
//
// Grammar (informal):
//
//	module      = (struct_decl | enum_decl | global_decl | const_decl |
//	               fun_decl | pipe_decl | assert_block | import_decl)*
//	struct_decl = 'struct' IDENT '{' (IDENT ':' type)* '}'
//	enum_decl   = 'enum' IDENT '{' (IDENT ['(' type ')'] ['=' INT] ',')* '}'
//	              — without payload: u8 tags (C-style)
//	              — with payload:    u16 encoding (tag<<8 | payload), generates __tag/__payload
//	global_decl = 'global' IDENT ':' type ['at' '(' expr ')'] ['=' array_lit | '=' expr]
//	const_decl  = 'const' IDENT ':' type '=' expr
//	fun_decl    = ['@extern'] ('fun'|'fn') IDENT '(' params ')' ['->' type] ('{' stmt* '}' | '\n')
//	pipe_decl   = ('pipe'|'trans') IDENT '{' (map_step | filter_step | use_step)* '}'
//	params      = (IDENT ':' type (',' IDENT ':' type)*)?
//
//	stmt        = var_decl | let_decl | assign | store | if_stmt | while_stmt
//	            | for_stmt | return_stmt | expr_stmt | break | continue
//	            | switch_stmt | block
//	var_decl    = 'var' IDENT ':' type ['at' '(' expr ')'] ['=' (array_lit | expr)]
//	let_decl    = 'let' (IDENT | '(' IDENT+ ')') [':' type] '=' expr
//	assign      = expr '=' expr                (where lhs is lvalue)
//	store       = '^' expr '=' expr
//	if_stmt     = 'if' expr '{' stmt* '}' ['else' '{' stmt* '}']
//	while_stmt  = 'while' expr '{' stmt* '}'
//	for_stmt    = 'for' IDENT 'in' expr '..' expr '{' stmt* '}'
//	return_stmt = 'return' [expr]
//	switch_stmt = 'switch' expr '{' ('case' (INT|IDENT) ':' stmt*)* ['default' ':' stmt*] '}'
//
//	type        = '^' type | '[' type ';' INT ']' | IDENT
//	expr        = ... (Pratt parser, standard binary precedence)
//	match_expr  = 'match' expr '{' (pattern '=>' expr ',')* '}'
//	              — pattern = '_' | INT | IDENT | IDENT '(' IDENT ')'
//	              — exhaustive check for enums/ADTs
//	if_expr     = 'if' expr '{' expr '}' 'else' '{' expr '}'
//	lambda      = '|' (IDENT [':' type])* '|' (expr | '{' stmt* '}')

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/plm"
)

// ParseOpts configures module-aware parsing (import resolution).
type ParseOpts struct {
	BaseDir    string            // directory of the source file (for relative imports)
	StdlibDir  string            // path to stdlib/ root (for stdlib imports)
	Loaded     map[string]bool   // already-loaded module paths (circular import detection)
}

// Parse parses Nanz source and returns a HIR module (no import resolution).
func Parse(src, name string) (*hir.Module, error) {
	return ParseWithOpts(src, name, ParseOpts{})
}

// ParseWithOpts parses Nanz source with import resolution enabled.
func ParseWithOpts(src, name string, opts ParseOpts) (*hir.Module, error) {
	l := newLexer(src)
	if opts.Loaded == nil {
		opts.Loaded = make(map[string]bool)
	}
	p := &parser{l: l, name: name, opts: opts}
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
	tokTilde      // ~
	tokLtLt       // <<
	tokGtGt       // >>
	tokPipeGt     // |>
	tokColonColon // ::
	tokAt         // @
	tokFatArrow   // =>
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
		case ch == ':' && l.pos+1 < len(l.src) && l.src[l.pos+1] == ':':
			l.emit(tokColonColon, "::", line); l.pos += 2; continue
		case ch == '|' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '>':
			l.emit(tokPipeGt, "|>", line); l.pos += 2; continue
		case ch == '=' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '>':
			l.emit(tokFatArrow, "=>", line); l.pos += 2; continue
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
			// String literal: "...", c"...", l"...", """..."""
			if ch == '"' {
				// Triple-quote: """..."""
				if l.pos+2 < len(l.src) && l.src[l.pos+1] == '"' && l.src[l.pos+2] == '"' {
					l.pos += 3 // skip opening """
					start := l.pos
					for l.pos+2 < len(l.src) {
						if l.src[l.pos] == '"' && l.src[l.pos+1] == '"' && l.src[l.pos+2] == '"' {
							break
						}
						l.pos++
					}
					s := string(l.src[start:l.pos])
					if l.pos+2 < len(l.src) {
						l.pos += 3 // skip closing """
					}
					l.emit(tokString, s, line)
					continue
				}
				// Regular string
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
			// Prefixed string literal: c"..." → CString, l"..." → LString
			// Also handles c"""...""" and l"""...""" triple-quote variants.
			if (ch == 'c' || ch == 'l') && l.pos+1 < len(l.src) && l.src[l.pos+1] == '"' {
				prefix := string(ch)
				l.pos++ // skip prefix char, now at opening quote
				// Check for triple-quote: c"""..."""
				if l.pos+2 < len(l.src) && l.src[l.pos+1] == '"' && l.src[l.pos+2] == '"' {
					l.pos += 3 // skip """
					start := l.pos
					for l.pos+2 < len(l.src) {
						if l.src[l.pos] == '"' && l.src[l.pos+1] == '"' && l.src[l.pos+2] == '"' {
							break
						}
						l.pos++
					}
					s := string(l.src[start:l.pos])
					if l.pos+2 < len(l.src) {
						l.pos += 3 // skip closing """
					}
					l.emit(tokString, prefix+"\x00"+s, line)
					continue
				}
				// Single-quote prefix string: c"..."
				l.pos++ // skip opening quote
				start := l.pos
				for l.pos < len(l.src) && l.src[l.pos] != '"' {
					l.pos++
				}
				s := string(l.src[start:l.pos])
				if l.pos < len(l.src) {
					l.pos++
				}
				l.emit(tokString, prefix+"\x00"+s, line)
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
func (l *lexer) save() int                  { return l.cur }
func (l *lexer) restore(pos int)            { l.cur = pos }
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

// nanzADT tracks an algebraic data type: enum variants with optional payload.
type nanzADT struct {
	name         string
	constructors []nanzADTCtor
	hasPayload   bool // true if any variant has a payload
}

// nanzADTCtor is one variant of an ADT.
type nanzADTCtor struct {
	name    string
	tag     int64
	payload mir2.Ty // nil = no payload
	adtName string  // back-reference to parent ADT name
}

type parser struct {
	l           *lexer
	name        string
	opts        ParseOpts
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
	funcSigs      map[string]mir2.Ty
	funcParamTys  map[string][]mir2.Ty // function name → param types (for partial application)
	varEnumType   map[string]string   // variable name → enum name (for exhaustive switch check)
	typeAliases     map[string]mir2.Ty              // type X = Y — structural alias
	enums           map[string]map[string]int64     // enumName → {variantName → value}
	enumBaseTy      map[string]mir2.Ty              // enumName → base type (default TyU8)
	importedModules map[string]string               // modPath → mangled prefix (for qualified access)
	funcAliases     map[string]string               // local name → mangled name (for unqualified imports)
	pipes           map[string][]pipeStep           // pipe/trans name → stages
	lambdaHintTy    mir2.Ty                         // type hint for untyped lambda params (set by chain context)
	metaFuncs       map[string]string               // @name → full Nanz source of metafunction
	// ADT (algebraic data types) — enums with payload variants
	adts            map[string]*nanzADT             // type name → ADT definition
	adtCtors        map[string]*nanzADTCtor         // constructor name → ctor (for match + expr)
	autoFuncs       []*hir.Func                     // auto-generated helpers (__tag, __payload, lambdas)
	localArrayID    int                              // counter for mangled local array globals
}

// pipeStep is one stage in a named pipe/trans declaration.
type pipeStep struct {
	kind string // "map" or "filter"
	fn   string // lifted lambda/function name
}

// isKnownFunc reports whether name refers to a known function (not a local variable).
func (p *parser) isKnownFunc(name string) bool {
	if _, ok := p.funcSigs[name]; ok {
		return true
	}
	if _, ok := p.funcAliases[name]; ok {
		return true
	}
	return false
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

// inferChainElemTy infers the element type flowing through an iterator chain.
// For map/filter calls, the element type is the return type of the previous
// stage's callback. For range sources and raw expressions, it's the expression type.
func (p *parser) inferChainElemTy(base hir.Expr) mir2.Ty {
	// If base is a call to map/filter, the element type is the callback's return type
	if call, ok := base.(*hir.CallExpr); ok {
		switch call.Fn {
		case "map":
			// map(source, cb) — element type is cb's return type
			if len(call.Args) >= 2 {
				if ref, ok2 := call.Args[1].(*hir.VarRefExpr); ok2 {
					if sig, hasSig := p.funcSigs[ref.Name]; hasSig {
						return sig
					}
				}
				return call.Args[1].ExprTy()
			}
		case "filter":
			// filter doesn't change element type — recurse into source
			if len(call.Args) >= 1 {
				return p.inferChainElemTy(call.Args[0])
			}
		}
	}
	// Default: base expression's type
	ty := p.exprTy(base)
	if ty != nil {
		return ty
	}
	return mir2.TyU8
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
	p.funcParamTys = make(map[string][]mir2.Ty)
	p.varEnumType = make(map[string]string)
	p.globalTypes = make(map[string]mir2.Ty)
	p.globalInterfaceTypes = make(map[string]string)
	p.varTypes = make(map[string]mir2.Ty)
	p.varInterfaceTypes = make(map[string]string)
	p.varPtrElem = make(map[string]*mir2.StructTy)
	p.uninitVars = make(map[string]int)
	p.typeAliases = make(map[string]mir2.Ty)
	p.enums = make(map[string]map[string]int64)
	p.enumBaseTy = make(map[string]mir2.Ty)
	p.funcAliases = make(map[string]string)
	p.pipes = make(map[string][]pipeStep)
	p.metaFuncs = make(map[string]string)
	p.adts = make(map[string]*nanzADT)
	p.adtCtors = make(map[string]*nanzADTCtor)

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

		case t.kind == tokIdent && t.val == "const":
			g, err := p.parseConstDecl()
			if err != nil {
				return nil, err
			}
			m.Globals = append(m.Globals, g)

		case t.kind == tokIdent && t.val == "fun":
			// Check for metafunction: fun @name(...)
			// Peek past "fun" to see if next is "@"
			saved := p.l.save()
			p.l.next() // consume "fun"
			if p.l.is(tokAt) {
				p.l.next() // consume "@"
				nameTok, err := p.l.eat(tokIdent)
				if err != nil {
					return nil, fmt.Errorf("line %d: expected metafunction name after fun @", t.line)
				}
				// Capture the entire function source from "fun @name" to closing "}"
				// We need to re-parse it later, so store the raw text
				src, err := p.captureMetaFuncSource(nameTok.val)
				if err != nil {
					return nil, err
				}
				p.metaFuncs[nameTok.val] = src
			} else {
				p.l.restore(saved)
				f, err := p.parseFunDecl(false)
				if err != nil {
					return nil, err
				}
				m.Funcs = append(m.Funcs, f)
			}

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
			} else if attr.kind == tokIdent && attr.val == "screen" && p.metaFuncs["screen"] == "" {
				// Built-in @screen metafunction (only if no user-defined @screen)
				p.l.next() // consume "screen"
				var scalarArgs []string
				if p.l.is(tokLParen) {
					p.l.next()
					for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
						argTok := p.l.next()
						val := argTok.val
						if argTok.kind == tokString {
							if idx := strings.IndexByte(val, 0); idx >= 0 {
								val = val[idx+1:]
							}
						}
						scalarArgs = append(scalarArgs, val)
						if p.l.is(tokComma) {
							p.l.next()
						}
					}
					if _, err := p.l.eat(tokRParen); err != nil {
						return nil, err
					}
				}
				title := "Screen"
				if len(scalarArgs) > 0 {
					title = scalarArgs[0]
				}
				var block []metaBlockNode
				if p.l.is(tokLBrace) {
					var err error
					block, err = parseMetaBlock(p.l)
					if err != nil {
						return nil, fmt.Errorf("line %d: @screen block: %w", attr.line, err)
					}
				}
				emitted, err := generateScreenSource(title, block)
				if err != nil {
					return nil, fmt.Errorf("line %d: @screen: %w", attr.line, err)
				}
				if emitted != "" {
					generated, err := ParseWithOpts(emitted, p.name+"@screen", p.opts)
					if err != nil {
						return nil, fmt.Errorf("line %d: @screen: generated code error: %w\n--- generated ---\n%s", attr.line, err, emitted)
					}
					strOffset := len(m.Strings)
					if len(generated.Strings) > 0 {
						m.Strings = append(m.Strings, generated.Strings...)
						m.StrKinds = append(m.StrKinds, generated.StrKinds...)
						if strOffset > 0 {
							for _, f := range generated.Funcs {
								remapStringRefs(f.Body, strOffset)
							}
						}
					}
					// Skip generated @extern funcs if they already exist (e.g. from import tui.render)
					existingFuncs := make(map[string]bool)
					for _, f := range m.Funcs {
						existingFuncs[f.Name] = true
					}
					for _, f := range generated.Funcs {
						if f.IsExtern && existingFuncs[f.Name] {
							continue // imported real implementation takes priority
						}
						m.Funcs = append(m.Funcs, f)
					}
					m.Globals = append(m.Globals, generated.Globals...)
					m.Structs = append(m.Structs, generated.Structs...)
					for _, s := range generated.Structs {
						p.structs[s.Name] = s
					}
					// Register generated symbols in parent parser
					for _, g := range generated.Globals {
						p.globalTypes[g.Name] = g.Ty
					}
					for _, f := range generated.Funcs {
						p.funcSigs[f.Name] = f.RetTy
					}
					// Remap generated code's CallExpr names via import aliases.
					// e.g. @screen generates calls to "tui_puts" but import tui.render
					// aliases it to "tui__render__tui_puts".
					if len(p.funcAliases) > 0 {
						for _, f := range m.Funcs {
							remapCallAliases(f.Body, p.funcAliases)
						}
					}
				}
			} else if metaSrc, ok := p.metaFuncs[attr.val]; ok {
				// Metafunction invocation: @name("args") { block }
				metaName := attr.val
				p.l.next() // consume name

				// Parse scalar arguments: @name("title", 42)
				var scalarArgs []string
				if p.l.is(tokLParen) {
					p.l.next()
					for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
						argTok := p.l.next()
						val := argTok.val
						// Extract string content from prefixed format (c\x00content or \x00content)
						if argTok.kind == tokString {
							if idx := strings.IndexByte(val, 0); idx >= 0 {
								val = val[idx+1:]
							}
						}
						scalarArgs = append(scalarArgs, val)
						if p.l.is(tokComma) {
							p.l.next()
						}
					}
					if _, err := p.l.eat(tokRParen); err != nil {
						return nil, err
					}
				}

				// Parse block: { field "X" length 10, ... }
				var block []metaBlockNode
				if p.l.is(tokLBrace) {
					var err error
					block, err = parseMetaBlock(p.l)
					if err != nil {
						return nil, fmt.Errorf("line %d: @%s block: %w", attr.line, metaName, err)
					}
				}

				// Execute metafunction on MIR2 VM
				emitted, err := p.executeMetaInvocation(metaSrc, metaName, scalarArgs, block)
				if err != nil {
					return nil, fmt.Errorf("line %d: @%s: %w", attr.line, metaName, err)
				}

				// Parse emitted Nanz text and merge into current module
				if emitted != "" {
					generated, err := ParseWithOpts(emitted, p.name+"@"+metaName, p.opts)
					if err != nil {
						return nil, fmt.Errorf("line %d: @%s: emitted code error: %w", attr.line, metaName, err)
					}
					// Merge strings: remap @mir2.str.N references in generated code
					// to account for strings already in the parent module.
					strOffset := len(m.Strings)
					if len(generated.Strings) > 0 {
						m.Strings = append(m.Strings, generated.Strings...)
						m.StrKinds = append(m.StrKinds, generated.StrKinds...)
						// Remap string references in generated functions
						if strOffset > 0 {
							for _, f := range generated.Funcs {
								remapStringRefs(f.Body, strOffset)
							}
						}
					}
					m.Funcs = append(m.Funcs, generated.Funcs...)
					m.Globals = append(m.Globals, generated.Globals...)
					m.Structs = append(m.Structs, generated.Structs...)
					for _, s := range generated.Structs {
						p.structs[s.Name] = s
					}
				}
			} else {
				return nil, fmt.Errorf("line %d: unexpected @%s", attr.line, attr.val)
			}

		case t.kind == tokIdent && t.val == "import":
			if err := p.parseImport(); err != nil {
				return nil, err
			}

		case t.kind == tokIdent && t.val == "type":
			if err := p.parseTypeAlias(); err != nil {
				return nil, err
			}

		case t.kind == tokIdent && t.val == "enum":
			if err := p.parseEnumDecl(); err != nil {
				return nil, err
			}

		case t.kind == tokIdent && (t.val == "pipe" || t.val == "trans"):
			if err := p.parsePipeDecl(); err != nil {
				return nil, err
			}

		case t.kind == tokIdent && t.val == "assert":
			a, err := p.parseAssert()
			if err != nil {
				return nil, err
			}
			m.Asserts = append(m.Asserts, a)

		case t.kind == tokIdent && t.val == "sandbox":
			sb, err := p.parseSandbox()
			if err != nil {
				return nil, err
			}
			m.Sandboxes = append(m.Sandboxes, sb)

			case t.kind == tokIdent && t.val == "impl":
			funcs, err := p.parseImplBlock()
			if err != nil {
				return nil, err
			}
			m.Funcs = append(m.Funcs, funcs...)

		default:
			return nil, fmt.Errorf("line %d: unexpected token %q at module level", t.line, t.val)
		}
	}
	// Append lambdas generated during parsing (non-capturing anonymous functions).
	m.Funcs = append(m.Funcs, p.lambdas...)
	// Append auto-generated helpers (__tag, __payload, match payload wrappers).
	m.Funcs = append(m.Funcs, p.autoFuncs...)

	// Fix forward-referenced call types: any CallExpr with Ty==TyVoid whose
	// target function actually returns a value needs its Ty patched.  This
	// happens when function A calls function B that is declared later in the
	// file — at parse time B's signature wasn't in funcSigs yet.
	p.fixForwardCallTypes(m)

	m.Warnings = p.warnings
	return m, nil
}

// fixForwardCallTypes walks all functions and patches CallExpr nodes whose Ty
// is TyVoid but whose target function actually returns a value.  This fixes
// forward references: when fun A (line 100) calls fun B (line 200), B's
// signature isn't in funcSigs during the first parse of A.
func (p *parser) fixForwardCallTypes(m *hir.Module) {
	for _, f := range m.Funcs {
		if f.Body != nil {
			p.fixBlockCallTypes(f.Body)
		}
	}
}

func (p *parser) fixBlockCallTypes(b *hir.Block) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		p.fixStmtCallTypes(s)
	}
}

func (p *parser) fixStmtCallTypes(s hir.Stmt) {
	switch st := s.(type) {
	case *hir.VarDeclStmt:
		p.fixExprCallTypes(st.Init)
	case *hir.AssignStmt:
		p.fixExprCallTypes(st.Val)
	case *hir.ReturnStmt:
		p.fixExprCallTypes(st.Val)
		for _, v := range st.Vals {
			p.fixExprCallTypes(v)
		}
	case *hir.ExprStmt:
		p.fixExprCallTypes(st.Expr)
	case *hir.IfStmt:
		p.fixExprCallTypes(st.Cond)
		p.fixBlockCallTypes(st.Then)
		p.fixBlockCallTypes(st.Else)
	case *hir.WhileStmt:
		p.fixExprCallTypes(st.Cond)
		p.fixBlockCallTypes(st.Body)
	case *hir.ForRangeStmt:
		p.fixBlockCallTypes(st.Body)
	case *hir.StoreStmt:
		p.fixExprCallTypes(st.Ptr)
		p.fixExprCallTypes(st.Val)
	case *hir.TupleLetStmt:
		p.fixExprCallTypes(st.Call)
	case *hir.ForEachStmt:
		p.fixBlockCallTypes(st.Body)
	}
}

func (p *parser) fixExprCallTypes(e hir.Expr) {
	if e == nil {
		return
	}
	switch ex := e.(type) {
	case *hir.CallExpr:
		if ex.Ty == mir2.TyVoid {
			if sig, ok := p.funcSigs[ex.Fn]; ok && sig != mir2.TyVoid {
				ex.Ty = sig
			}
		}
		for _, a := range ex.Args {
			p.fixExprCallTypes(a)
		}
	case *hir.BinExpr:
		p.fixExprCallTypes(ex.L)
		p.fixExprCallTypes(ex.R)
	case *hir.UnaryExpr:
		p.fixExprCallTypes(ex.X)
	case *hir.CastExpr:
		p.fixExprCallTypes(ex.X)
	case *hir.FieldExpr:
		p.fixExprCallTypes(ex.X)
	case *hir.LoadExpr:
		p.fixExprCallTypes(ex.Ptr)
	case *hir.DerefExpr:
		p.fixExprCallTypes(ex.Ptr)
	case *hir.IndexExpr:
		p.fixExprCallTypes(ex.Base)
		p.fixExprCallTypes(ex.Idx)
	case *hir.CondExpr:
		p.fixExprCallTypes(ex.Cond)
		p.fixExprCallTypes(ex.Then)
		p.fixExprCallTypes(ex.Else)
	}
}

// parseTypeAlias parses: type Name = ExistingType
// Structural alias — Name and ExistingType are fully interchangeable.
func (p *parser) parseTypeAlias() error {
	p.l.next() // consume "type"
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return fmt.Errorf("line %d: type alias: expected name", p.l.line)
	}
	if _, err := p.l.eat(tokEq); err != nil {
		return fmt.Errorf("line %d: type alias: expected '='", nameTok.line)
	}
	ty, err := p.parseType()
	if err != nil {
		return fmt.Errorf("line %d: type alias %s: %v", nameTok.line, nameTok.val, err)
	}
	p.typeAliases[nameTok.val] = ty
	return nil
}

// parseEnumDecl parses two forms:
//
//   enum Dir { UP, DOWN, LEFT, RIGHT }              — C-style (u8 tags)
//   enum Option { None, Some(u8) }                  — ADT with payload (u16 encoded)
//
// If any variant has a payload, the enum becomes an ADT: u16 encoding where
// high byte = tag, low byte = payload.  __tag/__payload helpers are generated.
func (p *parser) parseEnumDecl() error {
	p.l.next() // consume "enum"
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return fmt.Errorf("line %d: enum: expected name", p.l.line)
	}
	if _, err := p.l.eat(tokLBrace); err != nil {
		return fmt.Errorf("line %d: enum %s: expected '{'", nameTok.line, nameTok.val)
	}

	variants := make(map[string]int64)
	var nextVal int64
	var adtCtors []nanzADTCtor
	hasPayload := false

	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		vTok, err := p.l.eat(tokIdent)
		if err != nil {
			return fmt.Errorf("line %d: enum %s: expected variant name", p.l.line, nameTok.val)
		}

		var payloadTy mir2.Ty

		if p.l.is(tokLParen) {
			// Payload variant: Some(u8)
			p.l.next() // consume '('
			payloadTy, err = p.parseType()
			if err != nil {
				return fmt.Errorf("line %d: enum %s::%s: %v", p.l.line, nameTok.val, vTok.val, err)
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return fmt.Errorf("line %d: enum %s::%s: expected ')' after payload type", p.l.line, nameTok.val, vTok.val)
			}
			hasPayload = true
		} else if p.l.is(tokEq) {
			p.l.next() // consume '='
			valTok, err := p.l.eat(tokInt)
			if err != nil {
				return fmt.Errorf("line %d: enum %s::%s: expected integer value", p.l.line, nameTok.val, vTok.val)
			}
			v, err := strconv.ParseInt(valTok.val, 0, 64)
			if err != nil {
				return fmt.Errorf("line %d: enum %s::%s: invalid value %q", valTok.line, nameTok.val, vTok.val, valTok.val)
			}
			nextVal = v
		}

		if !hasPayload && nextVal > 255 {
			return fmt.Errorf("line %d: enum %s::%s: value %d exceeds u8 (0-255)", vTok.line, nameTok.val, vTok.val, nextVal)
		}
		variants[vTok.val] = nextVal
		adtCtors = append(adtCtors, nanzADTCtor{
			name:    vTok.val,
			tag:     nextVal,
			payload: payloadTy,
			adtName: nameTok.val,
		})
		nextVal++

		// Optional comma between variants
		if p.l.is(tokComma) {
			p.l.next()
		}
	}

	if _, err := p.l.eat(tokRBrace); err != nil {
		return fmt.Errorf("line %d: enum %s: expected '}'", p.l.line, nameTok.val)
	}

	p.enums[nameTok.val] = variants

	if hasPayload {
		// ADT mode: u16 encoding (tag << 8 | payload)
		p.enumBaseTy[nameTok.val] = mir2.TyU16
		def := &nanzADT{name: nameTok.val, constructors: adtCtors, hasPayload: true}
		p.adts[nameTok.val] = def
		for i := range def.constructors {
			p.adtCtors[def.constructors[i].name] = &def.constructors[i]
		}
		// Generate __tag and __payload helpers (idempotent — only once)
		if p.funcSigs["__tag"] == nil {
			p.autoFuncs = append(p.autoFuncs,
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
			p.funcSigs["__tag"] = mir2.TyU8
			p.funcSigs["__payload"] = mir2.TyU8
		}
	} else {
		// Simple C-style enum: u8 tags
		p.enumBaseTy[nameTok.val] = mir2.TyU8
		// Also register as ADT (without payload) for match expression support
		def := &nanzADT{name: nameTok.val, constructors: adtCtors, hasPayload: false}
		p.adts[nameTok.val] = def
		for i := range def.constructors {
			p.adtCtors[def.constructors[i].name] = &def.constructors[i]
		}
	}

	return nil
}

// parsePipeDecl parses a named pipe/trans declaration:
//
//	pipe alive { filter(|i: u8| particles[i].life > 0) }
//	trans on_screen { use alive; filter(|i: u8| particles[i].x < 255) }
//
// Each step is either a map/filter with a lambda, or "use other_pipe" which
// includes all steps from another pipe (snapshot at definition time).
// The lambdas are lifted to top-level functions just like inline lambdas.
func (p *parser) parsePipeDecl() error {
	p.l.next() // consume "pipe" or "trans"
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return fmt.Errorf("line %d: pipe: expected name", p.l.line)
	}
	if _, err := p.l.eat(tokLBrace); err != nil {
		return fmt.Errorf("line %d: pipe %s: expected '{'", nameTok.line, nameTok.val)
	}

	var steps []pipeStep
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		// Optional |> prefix before each step
		if p.l.is(tokPipeGt) {
			p.l.next()
		} else if p.l.is(tokPipe) {
			p.l.next()
			if p.l.is(tokGt) {
				p.l.next()
			}
		}
		if p.l.is(tokRBrace) {
			break
		}
		stepTok := p.l.peek()
		if stepTok.kind != tokIdent {
			return fmt.Errorf("line %d: pipe %s: expected step (map, filter, use)", stepTok.line, nameTok.val)
		}
		switch stepTok.val {
		case "use":
			p.l.next() // consume "use"
			refTok, err := p.l.eat(tokIdent)
			if err != nil {
				return fmt.Errorf("line %d: pipe %s: use: expected pipe name", p.l.line, nameTok.val)
			}
			ref, ok := p.pipes[refTok.val]
			if !ok {
				return fmt.Errorf("line %d: pipe %s: unknown pipe %q", refTok.line, nameTok.val, refTok.val)
			}
			steps = append(steps, ref...) // snapshot — copy steps
		case "map", "filter":
			kind := stepTok.val
			p.l.next() // consume "map" or "filter"
			if _, err := p.l.eat(tokLParen); err != nil {
				return fmt.Errorf("line %d: pipe %s: %s: expected '('", p.l.line, nameTok.val, kind)
			}
			lambdaExpr, err := p.parseLambda()
			if err != nil {
				return fmt.Errorf("line %d: pipe %s: %s: %w", p.l.line, nameTok.val, kind, err)
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return fmt.Errorf("line %d: pipe %s: %s: expected ')'", p.l.line, nameTok.val, kind)
			}
			fnName := ""
			if vr, ok := lambdaExpr.(*hir.VarRefExpr); ok {
				fnName = vr.Name
			}
			steps = append(steps, pipeStep{kind: kind, fn: fnName})
		default:
			return fmt.Errorf("line %d: pipe %s: unknown step %q (expected map, filter, use)", stepTok.line, nameTok.val, stepTok.val)
		}
		// Optional semicolon between steps
		if p.l.is(tokSemi) {
			p.l.next()
		}
	}

	if _, err := p.l.eat(tokRBrace); err != nil {
		return fmt.Errorf("line %d: pipe %s: expected '}'", p.l.line, nameTok.val)
	}

	p.pipes[nameTok.val] = steps
	return nil
}

// parseImport parses:
//
//	import math.gcd                  → qualified: math.gcd.funcname(...)
//	import math.gcd { gcd }          → unqualified: gcd(...)
//	import math.gcd { gcd as g }     → aliased: g(...)
//	import math.gcd { * }            → glob: all symbols unqualified
//
// The imported module is parsed recursively and merged into the current module.
// Function names are mangled with the module prefix (dots replaced with $).
func (p *parser) parseImport() error {
	p.l.next() // consume "import"
	line := p.l.peek().line

	// Parse dot-separated module path: math.gcd → ["math", "gcd"]
	var parts []string
	for {
		tok, err := p.l.eat(tokIdent)
		if err != nil {
			return fmt.Errorf("line %d: import: expected module name", line)
		}
		parts = append(parts, tok.val)
		if !p.l.is(tokDot) {
			break
		}
		p.l.next() // consume '.'
	}
	modPath := strings.Join(parts, ".")

	// Parse optional selective import: { name, name as alias, * }
	type importSym struct {
		name  string // original name in imported module
		alias string // local alias (empty = same as name)
	}
	var selected []importSym
	globImport := false
	qualified := true // default: qualified access (math.gcd.funcname)

	if p.l.is(tokLBrace) {
		p.l.next() // consume '{'
		qualified = false
		for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
			if len(selected) > 0 {
				if _, err := p.l.eat(tokComma); err != nil {
					return fmt.Errorf("line %d: import: expected ',' between names", line)
				}
			}
			// Glob: { * }
			if p.l.is(tokStar) {
				p.l.next()
				globImport = true
				break
			}
			nameTok, err := p.l.eat(tokIdent)
			if err != nil {
				return fmt.Errorf("line %d: import: expected symbol name", line)
			}
			sym := importSym{name: nameTok.val}
			// Optional: name as alias
			if p.l.is(tokIdent) && p.l.peek().val == "as" {
				p.l.next() // consume "as"
				aliasTok, err := p.l.eat(tokIdent)
				if err != nil {
					return fmt.Errorf("line %d: import: expected alias after 'as'", line)
				}
				sym.alias = aliasTok.val
			}
			selected = append(selected, sym)
		}
		if _, err := p.l.eat(tokRBrace); err != nil {
			return fmt.Errorf("line %d: import: expected '}'", line)
		}
	}

	// Plain `import tui.render` (no braces) → glob import: all symbols accessible
	if len(selected) == 0 && !globImport {
		globImport = true
	}

	// Resolve module path to filesystem
	filePath, err := p.resolveModulePath(modPath, line)
	if err != nil {
		return err
	}

	// Circular import detection
	absPath, _ := filepath.Abs(filePath)
	if p.opts.Loaded[absPath] {
		return fmt.Errorf("line %d: import %s: circular import detected", line, modPath)
	}

	// Load and parse imported module
	src, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("line %d: import %s: %w", line, modPath, err)
	}

	// Dispatch to the right parser based on file extension.
	var imported *hir.Module
	ext := filepath.Ext(filePath)
	switch ext {
	case ".lanz":
		imported, err = lanz.Compile(string(src), modPath)
	case ".lizp":
		imported, err = lizp.Compile(string(src), modPath)
	case ".plm":
		imported, err = plm.Compile(string(src))
	case ".pas":
		imported, err = pascal.Compile(string(src), modPath)
	default: // .nanz
		childOpts := ParseOpts{
			BaseDir:   filepath.Dir(filePath),
			StdlibDir: p.opts.StdlibDir,
			Loaded:    make(map[string]bool),
		}
		for k, v := range p.opts.Loaded {
			childOpts.Loaded[k] = v
		}
		childOpts.Loaded[absPath] = true
		imported, err = ParseWithOpts(string(src), modPath, childOpts)
	}
	if err != nil {
		return fmt.Errorf("line %d: import %s: %w", line, modPath, err)
	}

	// Merge imported module into current module.
	// Module prefix for name mangling: "tui.render" → "tui__render__"
	// Uses __ instead of $ for Z80 assembler label compatibility.
	modPrefix := strings.ReplaceAll(modPath, ".", "__") + "__"

	// Build name mapping: original → mangled (for all symbols in imported module)
	nameMap := make(map[string]string)

	// Merge functions
	for _, f := range imported.Funcs {
		origName := f.Name
		// Do not mangle @extern functions — they map to host functions by name
		if f.IsExtern {
			p.funcSigs[origName] = f.RetTy
			p.funcParamTys[origName] = paramTysFromFunc(f)
			p.module.Funcs = append(p.module.Funcs, f)
			continue
		}
		mangledName := modPrefix + f.Name
		nameMap[origName] = mangledName
		f.Name = mangledName

		// Register mangled function signature
		pty := paramTysFromFunc(f)
		p.funcSigs[mangledName] = f.RetTy
		p.funcParamTys[mangledName] = pty

		if globImport {
			// Glob: also register under original name → mangled name
			p.funcSigs[origName] = f.RetTy
			p.funcParamTys[origName] = pty
			p.funcAliases[origName] = mangledName
		}
		for _, sym := range selected {
			if sym.name == origName {
				localName := sym.name
				if sym.alias != "" {
					localName = sym.alias
				}
				p.funcSigs[localName] = f.RetTy
				p.funcParamTys[localName] = pty
				p.funcAliases[localName] = mangledName
			}
		}

		p.module.Funcs = append(p.module.Funcs, f)
	}

	// Merge structs
	for _, st := range imported.Structs {
		mangledName := modPrefix + st.Name
		origName := st.Name
		nameMap[origName] = mangledName
		st.Name = mangledName
		p.structs[mangledName] = st

		if globImport {
			p.structs[origName] = st
		}
		for _, sym := range selected {
			if sym.name == origName {
				localName := sym.name
				if sym.alias != "" {
					localName = sym.alias
				}
				p.structs[localName] = st
			}
		}

		p.module.Structs = append(p.module.Structs, st)
	}

	// Merge globals
	for _, g := range imported.Globals {
		mangledName := modPrefix + g.Name
		origName := g.Name
		nameMap[origName] = mangledName
		g.Name = mangledName
		p.globalTypes[mangledName] = g.Ty

		if globImport {
			p.globalTypes[origName] = g.Ty
		}
		for _, sym := range selected {
			if sym.name == origName {
				localName := sym.name
				if sym.alias != "" {
					localName = sym.alias
				}
				p.globalTypes[localName] = g.Ty
			}
		}

		p.module.Globals = append(p.module.Globals, g)
	}

	// Rewrite internal references in imported functions:
	// CallExpr.Fn and VarRefExpr.Name (for globals) must use mangled names.
	for _, f := range imported.Funcs {
		if f.Body != nil {
			for _, stmt := range f.Body.Body {
				rewriteImportedSymbols(stmt, nameMap)
			}
		}
	}

	// Merge string literals
	p.module.Strings = append(p.module.Strings, imported.Strings...)

	// Merge enums
	for eName, variants := range p.childEnums(imported) {
		mangledEnum := modPrefix + eName
		p.enums[mangledEnum] = variants
		p.enumBaseTy[mangledEnum] = mir2.TyU8

		if globImport {
			p.enums[eName] = variants
			p.enumBaseTy[eName] = mir2.TyU8
		}
		for _, sym := range selected {
			if sym.name == eName {
				localName := sym.name
				if sym.alias != "" {
					localName = sym.alias
				}
				p.enums[localName] = variants
				p.enumBaseTy[localName] = mir2.TyU8
			}
		}
	}

	// Merge type aliases
	for aName, aTy := range p.childTypeAliases(imported) {
		mangledAlias := modPrefix + aName
		p.typeAliases[mangledAlias] = aTy

		if globImport {
			p.typeAliases[aName] = aTy
		}
		for _, sym := range selected {
			if sym.name == aName {
				localName := sym.name
				if sym.alias != "" {
					localName = sym.alias
				}
				p.typeAliases[localName] = aTy
			}
		}
	}

	// For qualified import, register module prefix so that
	// "math.gcd.funcname()" resolves during expression parsing.
	if qualified {
		if p.importedModules == nil {
			p.importedModules = make(map[string]string)
		}
		p.importedModules[modPath] = modPrefix
	}

	return nil
}

// childEnums is a no-op placeholder — enums from imported modules are already
// in the parser's enums map from the child parse.  For now returns empty map;
// a proper implementation would pass enum data through hir.Module.
func (p *parser) childEnums(_ *hir.Module) map[string]map[string]int64 {
	return nil
}

// childTypeAliases — same placeholder as childEnums.
func (p *parser) childTypeAliases(_ *hir.Module) map[string]mir2.Ty {
	return nil
}

// resolveModulePath converts a dot-separated module path to a filesystem path.
// Search order: baseDir (local), then stdlibDir.
// Tries extensions in priority order: .nanz, .lanz, .plm.
func (p *parser) resolveModulePath(modPath string, line int) (string, error) {
	basePath := strings.ReplaceAll(modPath, ".", string(filepath.Separator))
	exts := []string{".nanz", ".minz", ".lanz", ".lizp", ".plm", ".pas"}
	dirs := []string{}
	if p.opts.BaseDir != "" {
		dirs = append(dirs, p.opts.BaseDir)
	}
	if p.opts.StdlibDir != "" {
		dirs = append(dirs, p.opts.StdlibDir)
	}

	for _, dir := range dirs {
		for _, ext := range exts {
			candidate := filepath.Join(dir, basePath+ext)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("line %d: import %s: module not found (searched %s, %s)",
		line, modPath, p.opts.BaseDir, p.opts.StdlibDir)
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
		v, err2 := p.parseAssertValue()
		if err2 != nil {
			return hir.Assert{}, fmt.Errorf("line %d: assert: %v", line, err2)
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
			v, err2 := p.parseAssertValue()
			if err2 != nil {
				return hir.Assert{}, fmt.Errorf("line %d: assert: %v", line, err2)
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
	expected, err := p.parseAssertValue()
	if err != nil {
		return hir.Assert{}, fmt.Errorf("line %d: assert: %v", line, err)
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

// parseAssertValue parses a compile-time integer value in an assert context.
// Accepts: integer literal (optionally negative), enum value (State.IDLE), or
// bare identifier that resolves to an enum variant.
func (p *parser) parseAssertValue() (int64, error) {
	// Enum qualified: State.IDLE
	if p.l.is(tokIdent) {
		t := p.l.peek()
		if variants, ok := p.enums[t.val]; ok && p.l.peekN(1).kind == tokDot {
			p.l.next() // consume enum name
			p.l.next() // consume '.'
			vTok, err := p.l.eat(tokIdent)
			if err != nil {
				return 0, fmt.Errorf("%s. expected variant name", t.val)
			}
			val, ok := variants[vTok.val]
			if !ok {
				return 0, fmt.Errorf("%s.%s: unknown variant", t.val, vTok.val)
			}
			return val, nil
		}
	}
	// Optionally negative integer
	neg := false
	if p.l.is(tokMinus) {
		p.l.next()
		neg = true
	}
	intTok, err := p.l.eat(tokInt)
	if err != nil {
		return 0, fmt.Errorf("expected integer literal or enum value")
	}
	v, err := strconv.ParseInt(intTok.val, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", intTok.val)
	}
	if neg {
		v = -v
	}
	return v, nil
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

// parseSandbox parses:
//
//	sandbox "name" {
//	    assert fn() == expected [via mir2|z80]
//	    assert fn2() == expected2 [via mir2|z80]
//	}
func (p *parser) parseSandbox() (hir.Sandbox, error) {
	line := p.l.peek().line
	if err := p.l.eatIdent("sandbox"); err != nil {
		return hir.Sandbox{}, err
	}

	// Expect string literal for the sandbox name.
	if !p.l.is(tokString) {
		return hir.Sandbox{}, fmt.Errorf("line %d: expected sandbox name (string), got %q", p.l.peek().line, p.l.peek().val)
	}
	name := p.l.peek().val
	p.l.next()

	// Expect opening brace.
	if !p.l.is(tokLBrace) {
		return hir.Sandbox{}, fmt.Errorf("line %d: expected '{' after sandbox name", p.l.peek().line)
	}
	p.l.next()

	var asserts []hir.Assert
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		if p.l.is(tokIdent) && p.l.peek().val == "assert" {
			a, err := p.parseAssert()
			if err != nil {
				return hir.Sandbox{}, err
			}
			asserts = append(asserts, a)
		} else {
			return hir.Sandbox{}, fmt.Errorf("line %d: expected 'assert' inside sandbox, got %q", p.l.peek().line, p.l.peek().val)
		}
	}

	if !p.l.is(tokRBrace) {
		return hir.Sandbox{}, fmt.Errorf("line %d: expected '}' to close sandbox", p.l.peek().line)
	}
	p.l.next()

	return hir.Sandbox{
		Name:    name,
		Asserts: asserts,
		Line:    line,
	}, nil
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

// ── Impl block ───────────────────────────────────────────────────────────────

// parseImplBlock parses:
//
//	impl TraitName for TypeName { fun method(self, ...) -> T { ... } ... }
//
// Desugars each method to: fun TypeName_method(self: ^TypeName, ...) -> T { ... }
// and registers it in the methodTable for UFCS dispatch.
func (p *parser) parseImplBlock() ([]*hir.Func, error) {
	if err := p.l.eatIdent("impl"); err != nil {
		return nil, err
	}
	// impl Trait for Type { ... }
	traitTok, err := p.l.eat(tokIdent)
	if err != nil {
		return nil, err
	}
	typeName := traitTok.val // could be trait or type

	// Check for "for Type" (trait impl) vs bare "impl Type" (plain impl)
	if p.l.isIdent("for") {
		p.l.next() // consume "for"
		typeTok, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, err
		}
		typeName = typeTok.val
	}

	if _, err := p.l.eat(tokLBrace); err != nil {
		return nil, err
	}

	var funcs []*hir.Func
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		if !p.l.isIdent("fun") && !p.l.isIdent("fn") {
			return nil, fmt.Errorf("line %d: impl block: expected 'fun', got %q", p.l.peek().line, p.l.peek().val)
		}
		p.l.next() // consume "fun"/"fn"

		methodTok, err := p.l.eat(tokIdent)
		if err != nil {
			return nil, err
		}
		methodName := methodTok.val
		funcName := typeName + "_" + methodName

		if _, err := p.l.eat(tokLParen); err != nil {
			return nil, err
		}

		// Parse params — first param "self" gets type ^TypeName
		var params []hir.Param
		for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
			paramTok, err := p.l.eat(tokIdent)
			if err != nil {
				return nil, err
			}
			if paramTok.val == "self" {
				params = append(params, hir.Param{Name: "self", Ty: mir2.TyPtr})
			} else {
				// name: type
				if _, err := p.l.eat(tokColon); err != nil {
					return nil, err
				}
				ty, err := p.parseType()
				if err != nil {
					return nil, err
				}
				params = append(params, hir.Param{Name: paramTok.val, Ty: ty})
			}
			if p.l.is(tokComma) {
				p.l.next()
			}
		}
		if _, err := p.l.eat(tokRParen); err != nil {
			return nil, err
		}

		// Return type
		retTy := mir2.Ty(mir2.TyVoid)
		if p.l.is(tokArrow) {
			p.l.next()
			retTy, err = p.parseType()
			if err != nil {
				return nil, err
			}
		}

		// Register method for UFCS
		if p.methodTable[typeName] == nil {
			p.methodTable[typeName] = make(map[string]methodInfo)
		}
		p.methodTable[typeName][methodName] = methodInfo{funcName: funcName, retTy: retTy}
		p.funcSigs[funcName] = retTy

		// Set up per-function scope for field access on self
		p.varTypes = make(map[string]mir2.Ty)
		p.varPtrElem = make(map[string]*mir2.StructTy)
		p.varInterfaceTypes = make(map[string]string)
		if st, ok := p.structs[typeName]; ok {
			p.varPtrElem["self"] = st
		}
		p.varTypes["self"] = mir2.TyPtr
		for _, param := range params {
			p.varTypes[param.Name] = param.Ty
		}

		// Parse body
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}

		f := &hir.Func{
			Name:   funcName,
			Params: params,
			RetTy:  retTy,
			Body:   body,
		}
		funcs = append(funcs, f)
	}

	if _, err := p.l.eat(tokRBrace); err != nil {
		return nil, err
	}
	return funcs, nil
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

func (p *parser) parseConstDecl() (mir2.Global, error) {
	if err := p.l.eatIdent("const"); err != nil {
		return mir2.Global{}, err
	}
	nameTok, err := p.l.eat(tokIdent)
	if err != nil {
		return mir2.Global{}, err
	}
	if _, err := p.l.eat(tokColon); err != nil {
		return mir2.Global{}, err
	}
	ty, _, err := p.parseTypeWithIface()
	if err != nil {
		return mir2.Global{}, err
	}
	g := mir2.Global{Name: nameTok.val, Ty: ty, IsConst: true}
	p.globalTypes[nameTok.val] = ty

	if _, err := p.l.eat(tokEq); err != nil {
		return g, fmt.Errorf("line %d: const %s requires an initializer", nameTok.line, nameTok.val)
	}
	init, err := p.parseInitializer(ty)
	if err != nil {
		return g, err
	}
	g.Init = init
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
					regClass = mir2.ClassRegC
					p.l.next()
				case "z80_d":
					regClass = mir2.ClassRegD
					p.l.next()
				case "z80_e":
					regClass = mir2.ClassRegE
					p.l.next()
				case "z80_h":
					regClass = mir2.ClassRegH
					p.l.next()
				case "z80_l":
					regClass = mir2.ClassRegL
					p.l.next()
				case "z80_ix":
					regClass = mir2.ClassIX
					p.l.next()
				case "z80_iy":
					regClass = mir2.ClassIY
					p.l.next()
				case "z80_ixh", "z80_ixl", "z80_iyh", "z80_iyl":
					regClass = mir2.ClassIXY8
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
		// Track enum type name for exhaustive switch check
		if tname := p.l.peek(); tname.kind == tokIdent {
			if _, isEnum := p.enums[tname.val]; isEnum {
				p.varEnumType[pname.val] = tname.val
			}
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
	{
		tys := make([]mir2.Ty, len(params))
		for i, pm := range params {
			tys[i] = pm.Ty
		}
		p.funcParamTys[funcName] = tys
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

	// Reset var scope and populate from params for the function body.
	// Note: varEnumType is populated during param parsing (above), so we
	// save and restore only the param entries.
	savedEnumTypes := p.varEnumType
	p.varTypes = make(map[string]mir2.Ty)
	p.varInterfaceTypes = make(map[string]string)
	p.varPtrElem = make(map[string]*mir2.StructTy)
	p.varEnumType = make(map[string]string)
	p.uninitVars = make(map[string]int) // fresh tracking per function
	// Re-populate param enum types from the saved map
	for _, param := range params {
		if eName, ok := savedEnumTypes[param.Name]; ok {
			p.varEnumType[param.Name] = eName
		}
	}
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
		case "fun", "fn":
			// Function type: fun(T1, T2) -> R
			// On Z80, function pointers are just addresses (TyPtr).
			// Parse and discard param/return types for now.
			if p.l.is(tokLParen) {
				p.l.next() // consume (
				for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
					if _, err := p.parseType(); err != nil {
						return nil, fmt.Errorf("line %d: function type param: %v", t.line, err)
					}
					if p.l.is(tokComma) {
						p.l.next()
					}
				}
				if _, err := p.l.eat(tokRParen); err != nil {
					return nil, fmt.Errorf("line %d: function type: expected ')'", t.line)
				}
				// Optional -> ReturnType
				if p.l.is(tokArrow) {
					p.l.next()
					if _, err := p.parseType(); err != nil {
						return nil, fmt.Errorf("line %d: function type return: %v", t.line, err)
					}
				}
			}
			return mir2.TyPtr, nil
		case "String", "SString", "LString", "CString":
			// All string types are pointers at the MIR2 level.
			// The encoding difference is tracked in the string pool, not the type system.
			return mir2.TyPtr, nil
		case "f", "f8", "f16":
			// Fixed-point types reserved; arithmetic semantics (>>fracBits after mul) not yet codegen'd.
			return nil, fmt.Errorf("line %d: fixed-point type %q not yet available in Nanz (coming soon)", t.line, t.val)
		default:
			// Type alias (structural)
			if aliased, ok := p.typeAliases[t.val]; ok {
				return aliased, nil
			}
			// Enum type → base type (u8)
			if ty, ok := p.enumBaseTy[t.val]; ok {
				return ty, nil
			}
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

// resolveTypeSize returns the byte size of a named type (for sizeof()).
func (p *parser) resolveTypeSize(name string, line int) (int, error) {
	switch name {
	case "u8", "i8", "bool":
		return 1, nil
	case "u16", "i16", "ptr":
		return 2, nil
	case "u24", "i24":
		return 3, nil
	case "u32", "i32":
		return 4, nil
	default:
		// Type alias → resolve and recurse
		if aliased, ok := p.typeAliases[name]; ok {
			return mir2.ByteWidth(aliased), nil
		}
		// Enum → always u8 (1 byte)
		if _, ok := p.enums[name]; ok {
			return 1, nil
		}
		if st, ok := p.structs[name]; ok {
			return mir2.ByteWidth(st), nil
		}
		return 0, fmt.Errorf("line %d: sizeof: unknown type %q", line, name)
	}
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
		// Track enum type name for exhaustive switch check
		if tname := p.l.peek(); tname.kind == tokIdent {
			if _, isEnum := p.enums[tname.val]; isEnum {
				p.varEnumType[nameTok.val] = tname.val
			}
		}
		ty, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}

	if _, err := p.l.eat(tokEq); err != nil {
		return nil, err
	}

	// Array literal initializer: let arr: [u8; 5] = [1, 2, 3, 4, 5]
	// → generate mangled global, return VarDeclStmt pointing to it.
	if p.l.is(tokLBrack) && ty != nil {
		if at, isArr := ty.(*mir2.ArrayTy); isArr {
			initData, err := p.parseInitializer(ty)
			if err != nil {
				return nil, err
			}
			mangledName := fmt.Sprintf("__arr_%d", p.localArrayID)
			p.localArrayID++
			p.module.Globals = append(p.module.Globals, mir2.Global{
				Name: mangledName,
				Ty:   ty,
				Init: initData,
			})
			// Return var decl as array with pointer to the mangled global.
			d := &hir.VarDeclStmt{
				Name:     nameTok.val,
				Ty:       at.Elem,
				ArrayLen: at.Len,
				Init:     &hir.AddrOfExpr{Sym: mangledName},
			}
			p.varTypes[nameTok.val] = at.Elem
			if p.uninitVars != nil {
				delete(p.uninitVars, nameTok.val)
			}
			return d, nil
		}
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
	// fold/reduce returns the accumulator type (inferred from callback return).
	if ty == nil || ty == mir2.TyVoid {
		if call, ok := init.(*hir.CallExpr); ok && (call.Fn == "fold" || call.Fn == "reduce") && len(call.Args) >= 3 {
			if cbRef, ok2 := call.Args[2].(*hir.AddrOfExpr); ok2 {
				if cbSig, hasSig := p.funcSigs[cbRef.Sym]; hasSig {
					ty = cbSig
				}
			} else if cbRef, ok2 := call.Args[2].(*hir.VarRefExpr); ok2 {
				if cbSig, hasSig := p.funcSigs[cbRef.Name]; hasSig {
					ty = cbSig
				}
			}
			if ty == nil || ty == mir2.TyVoid {
				ty = mir2.TyU8 // default fold accumulator type
			}
		}
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
	// Track enum type name for exhaustive switch check
	if tname := p.l.peek(); tname.kind == tokIdent {
		if _, isEnum := p.enums[tname.val]; isEnum {
			p.varEnumType[nameTok.val] = tname.val
		}
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
		if p.l.isIdent("if") {
			// else if → wrap in block containing a single if stmt
			inner, err := p.parseIf()
			if err != nil {
				return nil, err
			}
			s.Else = &hir.Block{Body: []hir.Stmt{inner}}
		} else {
			els, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			s.Else = els
		}
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
	// If T is ^StructName, we set elemStride = sizeof(StructName) for
	// pointer-to-struct iteration (e.g. for node: ^BlockNode in nodes[0..n]).
	var elemTy mir2.Ty = mir2.TyU8
	var elemStride int
	var ptrStructName string // for tracking struct pointer receiver
	if p.l.is(tokColon) {
		p.l.next()
		// Check for ^StructName specifically
		if p.l.is(tokCaret) {
			saved := p.l.save()
			p.l.next() // consume ^
			if p.l.peek().kind == tokIdent {
				name := p.l.peek().val
				if st, ok := p.structs[name]; ok {
					// Pointer to known struct — compute stride
					p.l.next() // consume struct name
					elemTy = mir2.TyPtr
					ptrStructName = name
					// sizeof(struct) = sum of field widths
					stride := 0
					for _, f := range st.Fields {
						w := mir2.ByteWidth(f.Ty)
						if w < 1 {
							w = 1
						}
						stride += w
					}
					elemStride = stride
				} else {
					p.l.restore(saved)
					elemTy, err = p.parseType()
					if err != nil {
						return nil, err
					}
				}
			} else {
				p.l.restore(saved)
				elemTy, err = p.parseType()
				if err != nil {
					return nil, err
				}
			}
		} else {
			elemTy, err = p.parseType()
			if err != nil {
				return nil, err
			}
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
		// Register the loop variable BEFORE parsing the body so field access works
		if ptrStructName != "" {
			p.varTypes[varTok.val] = mir2.TyPtr
			p.varPtrElem[varTok.val] = p.structs[ptrStructName]
		} else {
			p.varTypes[varTok.val] = elemTy
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return &hir.ForEachStmt{
			Var: varTok.val, ElemTy: elemTy, ElemStride: elemStride,
			PtrIter: ptrStructName != "",
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
			// .apply(pipeName) — expand pipe stages into nested map/filter calls
			if fieldTok.val == "apply" && p.l.is(tokLParen) {
				p.l.next() // consume '('
				pipeTok, err := p.l.eat(tokIdent)
				if err != nil {
					return nil, fmt.Errorf("line %d: apply: expected pipe name", p.l.line)
				}
				steps, ok := p.pipes[pipeTok.val]
				if !ok {
					return nil, fmt.Errorf("line %d: apply: unknown pipe %q", pipeTok.line, pipeTok.val)
				}
				if _, err := p.l.eat(tokRParen); err != nil {
					return nil, err
				}
				// Wrap base in nested map/filter calls (innermost first)
				for _, step := range steps {
					base = &hir.CallExpr{
						Fn:   step.kind,
						Args: []hir.Expr{base, &hir.VarRefExpr{Name: step.fn, Ty: mir2.TyU8}},
						Ty:   mir2.TyVoid,
					}
				}
				continue
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
			var err error
			base, err = p.resolveCall(base, args)
			if err != nil {
				return nil, err
			}
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

	// Optional clauses before the asm body '{':
	//   (in x, y)                — input operands
	//   (ret REG) or (out REG)   — return register
	//   (clob A, F) or (clob auto) or (clob all) — clobber specification
	// If no clob clause → auto (compiler infers from asm text).
	var ins []hir.AsmOperand
	retReg := ""
	var clobberRegs []string
	clobberAll := false
	clobberAuto := true // default: auto-detect from asm text
	for p.l.is(tokLParen) {
		p.l.next() // consume '('
		if p.l.peek().kind != tokIdent {
			return nil, fmt.Errorf("line %d: asm clause: expected 'in', 'ret', 'out', or 'clob'", p.l.peek().line)
		}
		clause := p.l.peek().val
		p.l.next() // consume clause keyword
		switch clause {
		case "in":
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
		case "ret", "out":
			regTok, err := p.l.eat(tokIdent)
			if err != nil {
				return nil, fmt.Errorf("line %d: asm (%s): expected register name", regTok.line, clause)
			}
			retReg = strings.ToUpper(regTok.val)
		case "clob":
			clobberAuto = false
			if p.l.peek().kind == tokIdent && p.l.peek().val == "auto" {
				p.l.next()
				clobberAuto = true
			} else if p.l.peek().kind == tokIdent && p.l.peek().val == "all" {
				p.l.next()
				clobberAll = true
			} else {
				// Explicit register list: (clob A, F, HL)
				for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
					regTok, err := p.l.eat(tokIdent)
					if err != nil {
						return nil, fmt.Errorf("line %d: asm (clob): expected register name", regTok.line)
					}
					clobberRegs = append(clobberRegs, strings.ToUpper(regTok.val))
					if p.l.is(tokComma) {
						p.l.next()
					}
				}
			}
		default:
			return nil, fmt.Errorf("line %d: asm: unknown clause %q (expected 'in', 'ret', 'out', or 'clob')", p.l.peek().line, clause)
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
	return &hir.AsmStmt{
		Target:      target,
		Code:        code,
		ClobberAll:  clobberAll,
		Ins:         ins,
		RetReg:      retReg,
		ClobberRegs: clobberRegs,
		ClobberAuto: clobberAuto,
	}, nil
}

func (p *parser) parseSwitch() (hir.Stmt, error) {
	switchLine := p.l.line
	if err := p.l.eatIdent("switch"); err != nil {
		return nil, err
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	p.warnUninitInExpr(val)
	p.uninitVars = nil // stop tracking inside switch (conservative)

	// Determine if val is an enum-typed variable (for exhaustive check + variant names in case)
	var switchEnumName string
	var switchEnumVariants map[string]int64
	if vr, ok := val.(*hir.VarRefExpr); ok {
		if eName, ok := p.varEnumType[vr.Name]; ok {
			switchEnumName = eName
			switchEnumVariants = p.enums[eName]
		}
	}

	if _, err := p.l.eat(tokLBrace); err != nil {
		return nil, err
	}
	s := &hir.SwitchStmt{Val: val}
	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		if p.l.isIdent("case") {
			p.l.next()
			var v int64
			tok := p.l.peek()
			if tok.kind == tokInt {
				// Integer literal case
				p.l.next()
				v, _ = strconv.ParseInt(tok.val, 0, 64)
			} else if tok.kind == tokIdent {
				// Try enum variant name: case IDLE: or case EnumName.VARIANT:
				p.l.next()
				resolved := false
				// Direct variant name (when switching on an enum-typed var)
				if switchEnumVariants != nil {
					if val, ok := switchEnumVariants[tok.val]; ok {
						v = val
						resolved = true
					}
				}
				// Qualified: EnumName.VARIANT
				if !resolved && p.l.is(tokDot) {
					if variants, ok := p.enums[tok.val]; ok {
						p.l.next() // consume '.'
						vTok, err := p.l.eat(tokIdent)
						if err != nil {
							return nil, fmt.Errorf("line %d: case %s. expected variant name", tok.line, tok.val)
						}
						val, ok := variants[vTok.val]
						if !ok {
							return nil, fmt.Errorf("line %d: case %s.%s: unknown variant", vTok.line, tok.val, vTok.val)
						}
						v = val
						resolved = true
					}
				}
				if !resolved {
					return nil, fmt.Errorf("line %d: case: expected integer or enum variant, got %q", tok.line, tok.val)
				}
			} else {
				return nil, fmt.Errorf("line %d: case: expected integer or enum variant", tok.line)
			}
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

	// Exhaustive check: if switching on an enum-typed variable without default,
	// verify all variants are covered.
	if switchEnumName != "" && s.Default == nil {
		covered := make(map[int64]bool)
		for _, c := range s.Cases {
			covered[c.Val] = true
		}
		var missing []string
		for vName, vVal := range switchEnumVariants {
			if !covered[vVal] {
				missing = append(missing, vName)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("line %d: switch on %s is not exhaustive, missing variants: %s",
				switchLine, switchEnumName, strings.Join(missing, ", "))
		}
	}

	return s, nil
}

// parseMatchExpr parses a Rust-style match expression:
//
//	match expr {
//	    Some(v) => v + 1,
//	    None    => 0,
//	    _       => default_val,
//	}
//
// Returns an HIR CondExpr chain (nested if-then-else).
// For payload ADTs, the scrutinee is compared by tag (__tag(x)),
// and payload bindings use __payload(x) wrapped in a helper function.
func (p *parser) parseMatchExpr() (hir.Expr, error) {
	matchLine := p.l.peek().line
	p.l.next() // consume "match"

	scrutinee, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.l.eat(tokLBrace); err != nil {
		return nil, fmt.Errorf("line %d: match: expected '{'", matchLine)
	}

	// Determine if scrutinee is an ADT with payload
	var scrutADT *nanzADT
	if vr, ok := scrutinee.(*hir.VarRefExpr); ok {
		if eName, ok := p.varEnumType[vr.Name]; ok {
			if adt, ok := p.adts[eName]; ok {
				scrutADT = adt
			}
		}
	}

	type matchArm struct {
		isDefault      bool
		tagVal         int64
		body           hir.Expr
		payloadBind    string  // variable name bound to payload
		hasPayloadBind bool
	}
	var arms []matchArm

	for !p.l.is(tokRBrace) && !p.l.is(tokEOF) {
		tok := p.l.peek()
		a := matchArm{}

		if tok.kind == tokIdent && tok.val == "_" {
			// Wildcard: _ => expr
			p.l.next()
			a.isDefault = true
		} else if tok.kind == tokInt {
			// Integer pattern: 42 => expr
			p.l.next()
			a.tagVal, _ = strconv.ParseInt(tok.val, 0, 64)
		} else if tok.kind == tokIdent {
			if ctor, ok := p.adtCtors[tok.val]; ok {
				// ADT constructor: Some(v) => ... or None => ...
				p.l.next()
				a.tagVal = ctor.tag
				if ctor.payload != nil && p.l.is(tokLParen) {
					// Payload binding: Some(v) => ...
					p.l.next() // consume '('
					bindTok, err := p.l.eat(tokIdent)
					if err != nil {
						return nil, fmt.Errorf("line %d: match: expected binding name", tok.line)
					}
					if _, err := p.l.eat(tokRParen); err != nil {
						return nil, fmt.Errorf("line %d: match: expected ')' after binding", tok.line)
					}
					a.payloadBind = bindTok.val
					a.hasPayloadBind = true
				}
			} else if variants, ok := p.enums[tok.val]; ok && p.l.peekN(1).kind == tokDot {
				// Qualified: EnumName.VARIANT => ...
				p.l.next() // consume enum name
				p.l.next() // consume '.'
				vTok, err := p.l.eat(tokIdent)
				if err != nil {
					return nil, fmt.Errorf("line %d: match: expected variant after '.'", tok.line)
				}
				val, ok := variants[vTok.val]
				if !ok {
					return nil, fmt.Errorf("line %d: match: unknown variant %s.%s", vTok.line, tok.val, vTok.val)
				}
				a.tagVal = val
			} else {
				// Try as bare variant of the scrutinee's enum
				resolved := false
				if scrutADT != nil {
					for _, c := range scrutADT.constructors {
						if c.name == tok.val {
							p.l.next()
							a.tagVal = c.tag
							if c.payload != nil && p.l.is(tokLParen) {
								p.l.next()
								bindTok, err := p.l.eat(tokIdent)
								if err != nil {
									return nil, fmt.Errorf("line %d: match: expected binding name", tok.line)
								}
								if _, err := p.l.eat(tokRParen); err != nil {
									return nil, fmt.Errorf("line %d: match: expected ')'", tok.line)
								}
								a.payloadBind = bindTok.val
								a.hasPayloadBind = true
							}
							resolved = true
							break
						}
					}
				}
				if !resolved {
					// Check the enums map for bare variant names (simple enums)
					if vr, ok2 := scrutinee.(*hir.VarRefExpr); ok2 {
						if eName, ok3 := p.varEnumType[vr.Name]; ok3 {
							if variants, ok4 := p.enums[eName]; ok4 {
								if val, ok5 := variants[tok.val]; ok5 {
									p.l.next()
									a.tagVal = val
									resolved = true
								}
							}
						}
					}
				}
				if !resolved {
					return nil, fmt.Errorf("line %d: match: unexpected pattern %q", tok.line, tok.val)
				}
			}
		} else {
			return nil, fmt.Errorf("line %d: match: expected pattern, got %q", tok.line, tok.val)
		}

		// Expect =>
		if _, err := p.l.eat(tokFatArrow); err != nil {
			return nil, fmt.Errorf("line %d: match: expected '=>'", p.l.line)
		}

		// Parse body expression
		body, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		// Wrap payload binding: create helper function taking payload as arg
		if a.hasPayloadBind && a.payloadBind != "" {
			wrapName := fmt.Sprintf("__mpay_%d", p.lambdaCount)
			p.lambdaCount++
			p.autoFuncs = append(p.autoFuncs, &hir.Func{
				Name:   wrapName,
				Params: []hir.Param{{Name: a.payloadBind, Ty: mir2.TyU8}},
				RetTy:  body.ExprTy(),
				Body:   &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: body}}},
			})
			p.funcSigs[wrapName] = body.ExprTy()
			body = &hir.CallExpr{
				Fn:   wrapName,
				Args: []hir.Expr{&hir.CallExpr{Fn: "__payload", Args: []hir.Expr{scrutinee}, Ty: mir2.TyU8}},
				Ty:   body.ExprTy(),
			}
		}
		a.body = body
		arms = append(arms, a)

		// Optional comma between arms
		if p.l.is(tokComma) {
			p.l.next()
		}
	}

	if _, err := p.l.eat(tokRBrace); err != nil {
		return nil, fmt.Errorf("line %d: match: expected '}'", p.l.line)
	}

	if len(arms) == 0 {
		return nil, fmt.Errorf("line %d: match: no arms", matchLine)
	}

	// Exhaustiveness check for enums/ADTs
	hasDefault := false
	for _, a := range arms {
		if a.isDefault {
			hasDefault = true
		}
	}
	if !hasDefault && scrutADT != nil {
		covered := make(map[int64]bool)
		for _, a := range arms {
			if !a.isDefault {
				covered[a.tagVal] = true
			}
		}
		var missing []string
		for _, c := range scrutADT.constructors {
			if !covered[c.tag] {
				missing = append(missing, c.name)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("line %d: match is not exhaustive, missing: %s",
				matchLine, strings.Join(missing, ", "))
		}
	}

	// Build CondExpr chain from arms (right-fold: last arm is innermost else)
	// For payload ADTs, compare __tag(scrutinee) == tagVal
	// For simple enums, compare scrutinee == tagVal directly
	useTag := scrutADT != nil && scrutADT.hasPayload
	var taggedScrutinee hir.Expr
	if useTag {
		taggedScrutinee = &hir.CallExpr{Fn: "__tag", Args: []hir.Expr{scrutinee}, Ty: mir2.TyU8}
	}

	// Find default arm body (or use 0 as fallback)
	var defaultBody hir.Expr
	var nonDefaultArms []matchArm
	for _, a := range arms {
		if a.isDefault {
			defaultBody = a.body
		} else {
			nonDefaultArms = append(nonDefaultArms, a)
		}
	}
	if defaultBody == nil {
		defaultBody = &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}
	}

	// Build chain from right to left
	result := defaultBody
	for i := len(nonDefaultArms) - 1; i >= 0; i-- {
		a := nonDefaultArms[i]
		var cmp hir.Expr
		if useTag {
			cmp = &hir.BinExpr{
				Op: "==",
				L:  taggedScrutinee,
				R:  &hir.IntLitExpr{Val: a.tagVal, Ty: mir2.TyU8},
				Ty: mir2.TyBool,
			}
		} else {
			cmp = &hir.BinExpr{
				Op: "==",
				L:  scrutinee,
				R:  &hir.IntLitExpr{Val: a.tagVal, Ty: p.exprTy(scrutinee)},
				Ty: mir2.TyBool,
			}
		}
		result = &hir.CondExpr{Cond: cmp, Then: a.body, Else: result, Ty: a.body.ExprTy()}
	}

	return result, nil
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
		// Also record struct type in varTypes so subsequent field accesses resolve correctly.
		if vr, ok := lhs.(*hir.VarRefExpr); ok {
			if p.uninitVars != nil {
				delete(p.uninitVars, vr.Name)
			}
			if lit, ok := rhs.(*hir.StructLitExpr); ok {
				p.varTypes[vr.Name] = lit.St
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
	tokPipe: {"|", 1},
	// XOR uses the `xor` keyword operator (see parseBinary), not ^.
	// ^ is reserved exclusively for postfix pointer dereference.
	tokAmp: {"&", 3},
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
		// "expr as type" cast — high precedence postfix.
		if t.kind == tokIdent && t.val == "as" {
			p.l.next() // consume "as"
			tyTok, err2 := p.l.eat(tokIdent)
			if err2 != nil {
				return nil, err2
			}
			ty, ok2 := map[string]mir2.Ty{
				"u8": mir2.TyU8, "u16": mir2.TyU16, "u24": mir2.TyU24, "u32": mir2.TyU32,
				"i8": mir2.TyI8, "i16": mir2.TyI16, "i24": mir2.TyI24, "i32": mir2.TyI32,
				"bool": mir2.TyBool, "ptr": mir2.TyPtr,
			}[tyTok.val]
			if !ok2 {
				return nil, fmt.Errorf("line %d: unknown cast target type %q", tyTok.line, tyTok.val)
			}
			lhs = &hir.CastExpr{X: lhs, Ty: ty}
			continue
		}
		// |> value pipe: expr |> f → f(expr), expr |> f(a) → f(expr, a)
		if t.kind == tokPipeGt && minPrec < 1 {
			p.l.next() // consume |>
			rhs, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			// If rhs is a call f(args...), insert lhs as first arg
			if call, ok := rhs.(*hir.CallExpr); ok {
				call.Args = append([]hir.Expr{lhs}, call.Args...)
				lhs = call
			} else if ref, ok := rhs.(*hir.VarRefExpr); ok {
				// Bare function name: f → f(lhs)
				retTy := mir2.Ty(mir2.TyU8)
				if sig, hasSig := p.funcSigs[ref.Name]; hasSig {
					retTy = sig
				}
				lhs = &hir.CallExpr{Fn: ref.Name, Args: []hir.Expr{lhs}, Ty: retTy}
			} else if addr, ok := rhs.(*hir.AddrOfExpr); ok {
				// Function-as-value via AddrOfExpr: f → f(lhs)
				retTy := mir2.Ty(mir2.TyU8)
				if sig, hasSig := p.funcSigs[addr.Sym]; hasSig {
					retTy = sig
				}
				lhs = &hir.CallExpr{Fn: addr.Sym, Args: []hir.Expr{lhs}, Ty: retTy}
			} else {
				return nil, fmt.Errorf("line %d: |> pipe: right side must be a function name or call", t.line)
			}
			continue
		}
		// `xor`/`XOR` keyword operator — bitwise XOR (precedence 2, between | and &).
		if t.kind == tokIdent && (t.val == "xor" || t.val == "XOR") && minPrec < 2 {
			p.l.next()
			rhs, err := p.parseBinary(2)
			if err != nil {
				return nil, err
			}
			ty := resultTy(lhs.ExprTy(), rhs.ExprTy(), "^")
			lhs = &hir.BinExpr{Op: "^", L: lhs, R: rhs, Ty: ty}
			continue
		}
		bo, ok := binops[t.kind]
		if !ok || bo.prec <= minPrec {
			break
		}
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
			// expr^ — postfix pointer dereference (always).
			// XOR uses the `xor` keyword operator, not ^.
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
			// Qualified module access: mod.func(args) or mod.sub.func(args)
			// base.field    — struct field access
			// base.method() — UFCS method call: rewritten to method(base, args...)
			p.l.next()
			fieldTok, err := p.l.eat(tokIdent)
			if err != nil {
				return nil, err
			}

			// Check for qualified module call: mod.func(...) or mod.sub.func(...)
			if vr, ok := base.(*hir.VarRefExpr); ok && p.importedModules != nil {
				resolved := false
				// Try progressively longer module paths: "mod", "mod.sub", "mod.sub.pkg"
				modKey := vr.Name
				member := fieldTok.val
				for !resolved {
					if prefix, found := p.importedModules[modKey]; found {
						if p.l.is(tokLParen) {
							// Module-qualified function call: mod.func(args)
							mangledName := prefix + member
							p.l.next() // consume '('
							var args []hir.Expr
							for !p.l.is(tokRParen) && !p.l.is(tokEOF) {
								a, err2 := p.parseExpr()
								if err2 != nil {
									return nil, err2
								}
								args = append(args, a)
								if p.l.is(tokComma) {
									p.l.next()
								}
							}
							if _, err2 := p.l.eat(tokRParen); err2 != nil {
								return nil, err2
							}
							callTy := mir2.Ty(mir2.TyVoid)
							if ty, ok2 := p.funcSigs[mangledName]; ok2 {
								callTy = ty
							}
							base = &hir.CallExpr{Fn: mangledName, Args: args, Ty: callTy}
							resolved = true
							break
						}
						// Module-qualified enum: mod.EnumName.VARIANT
						if variants, eOk := p.enums[prefix+member]; eOk && p.l.is(tokDot) {
							p.l.next() // consume '.'
							vTok, err2 := p.l.eat(tokIdent)
							if err2 != nil {
								return nil, fmt.Errorf("line %d: %s.%s. expected variant", fieldTok.line, modKey, member)
							}
							val, ok2 := variants[vTok.val]
							if !ok2 {
								return nil, fmt.Errorf("line %d: %s.%s.%s: unknown variant", vTok.line, modKey, member, vTok.val)
							}
							base = &hir.IntLitExpr{Val: val, Ty: mir2.TyU8}
							resolved = true
							break
						}
						// Module-qualified struct literal: mod.StructName { ... }
						if st, sOk := p.structs[prefix+member]; sOk && p.l.is(tokLBrace) {
							lit, err2 := p.parseStructLit(st)
							if err2 != nil {
								return nil, err2
							}
							base = lit
							resolved = true
							break
						}
						// Module-qualified global: mod.varname
						if ty, gOk := p.globalTypes[prefix+member]; gOk {
							base = &hir.VarRefExpr{Name: prefix + member, Ty: ty}
							resolved = true
							break
						}
						break
					}
					// Not a module yet — maybe "mod.sub" is a longer module path
					// Check if next token continues the path: mod.sub.pkg.func(...)
					if !p.l.is(tokDot) {
						break
					}
					modKey = modKey + "." + member
					p.l.next() // consume '.'
					nextTok, err2 := p.l.eat(tokIdent)
					if err2 != nil {
						return nil, err2
					}
					member = nextTok.val
				}
				if resolved {
					continue // continue outer parsePostfix loop
				}
			}

			// .apply(pipeName) — expand pipe stages into nested map/filter calls
			if fieldTok.val == "apply" && p.l.is(tokLParen) {
				p.l.next() // consume '('
				pipeTok, err := p.l.eat(tokIdent)
				if err != nil {
					return nil, fmt.Errorf("line %d: apply: expected pipe name", p.l.line)
				}
				steps, ok := p.pipes[pipeTok.val]
				if !ok {
					return nil, fmt.Errorf("line %d: apply: unknown pipe %q", pipeTok.line, pipeTok.val)
				}
				if _, err := p.l.eat(tokRParen); err != nil {
					return nil, err
				}
				for _, step := range steps {
					base = &hir.CallExpr{
						Fn:   step.kind,
						Args: []hir.Expr{base, &hir.VarRefExpr{Name: step.fn, Ty: mir2.TyU8}},
						Ty:   mir2.TyVoid,
					}
				}
				continue
			}

			if p.l.is(tokLParen) {
				// Method call: base.method(a, b) → method(base, a, b)
				p.l.next()
				args := []hir.Expr{base}
				// Set lambda type hint for iterator chain methods
				savedHint := p.lambdaHintTy
				switch fieldTok.val {
				case "map", "filter", "forEach", "fold", "reduce":
					p.lambdaHintTy = p.inferChainElemTy(base)
				}
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
				p.lambdaHintTy = savedHint
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
			var err error
			base, err = p.resolveCall(base, args)
			if err != nil {
				return nil, err
			}
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
		case "match":
			return p.parseMatchExpr()
		case "u8", "u16", "u24", "u32", "i8", "i16", "i24", "i32":
			// cast: u8(expr)
			p.l.next()
			ty := map[string]mir2.Ty{
				"u8": mir2.TyU8, "u16": mir2.TyU16, "u24": mir2.TyU24, "u32": mir2.TyU32,
				"i8": mir2.TyI8, "i16": mir2.TyI16, "i24": mir2.TyI24, "i32": mir2.TyI32,
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
		// ptr(expr) — cast u16 to pointer for direct memory access.
		// ptr(0x5800)^ reads byte; ptr(0x5800)^ = val writes byte.
		if t.val == "ptr" && p.l.peekN(1).kind == tokLParen {
			p.l.next() // consume "ptr"
			p.l.next() // consume "("
			x, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			return &hir.CastExpr{X: x, Ty: mir2.TyPtr}, nil
		}
		// sizeof(TypeName) — compile-time constant, resolves to byte size of type.
		if t.val == "sizeof" && p.l.peekN(1).kind == tokLParen {
			p.l.next() // consume "sizeof"
			p.l.next() // consume "("
			nameTok, err := p.l.eat(tokIdent)
			if err != nil {
				return nil, fmt.Errorf("line %d: sizeof: expected type name", t.line)
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, fmt.Errorf("line %d: sizeof: expected ')'", t.line)
			}
			size, err := p.resolveTypeSize(nameTok.val, t.line)
			if err != nil {
				return nil, err
			}
			ty := mir2.Ty(mir2.TyU8)
			if size > 255 {
				ty = mir2.TyU16
			}
			return &hir.IntLitExpr{Val: int64(size), Ty: ty}, nil
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
		// ADT constructor: Some(42) or None
		if ctor, ok := p.adtCtors[t.val]; ok {
			p.l.next() // consume constructor name
			adt := p.adts[ctor.adtName]
			if ctor.payload != nil && p.l.is(tokLParen) {
				// Constructor with payload: Some(expr) → (tag * 256) + u16(expr)
				p.l.next() // consume '('
				arg, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				if _, err := p.l.eat(tokRParen); err != nil {
					return nil, err
				}
				tagExpr := &hir.IntLitExpr{Val: ctor.tag * 256, Ty: mir2.TyU16}
				return &hir.BinExpr{Op: "+", L: tagExpr, R: &hir.CastExpr{X: arg, Ty: mir2.TyU16}, Ty: mir2.TyU16}, nil
			}
			if adt.hasPayload {
				// No-payload constructor in payload ADT: None → tag * 256 as u16
				return &hir.IntLitExpr{Val: ctor.tag * 256, Ty: mir2.TyU16}, nil
			}
			// Simple enum (no payloads): tag as u8
			return &hir.IntLitExpr{Val: ctor.tag, Ty: mir2.TyU8}, nil
		}
		// Enum qualified access: State.IDLE → IntLitExpr
		if variants, ok := p.enums[t.val]; ok && p.l.peekN(1).kind == tokDot {
			p.l.next() // consume enum name
			p.l.next() // consume '.'
			vTok, err := p.l.eat(tokIdent)
			if err != nil {
				return nil, fmt.Errorf("line %d: %s. expected variant name", t.line, t.val)
			}
			val, ok := variants[vTok.val]
			if !ok {
				return nil, fmt.Errorf("line %d: %s.%s: unknown variant", vTok.line, t.val, vTok.val)
			}
			return &hir.IntLitExpr{Val: val, Ty: p.enumBaseTy[t.val]}, nil
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
		// Bare function name used as a value (not followed by '(' call) →
		// function pointer.  Emit AddrOfExpr so MIR2 generates LD HL, label.
		if p.isKnownFunc(t.val) && !p.l.is(tokLParen) {
			return &hir.AddrOfExpr{Sym: t.val}, nil
		}
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
		raw := t.val
		kind := mir2.StrSString // default: SString (u8-prefix)

		// Detect prefixed strings: c\x00... → CString, l\x00... → LString
		if len(raw) >= 2 && raw[1] == '\x00' {
			switch raw[0] {
			case 'c':
				kind = mir2.StrCString
				raw = raw[2:]
			case 'l':
				kind = mir2.StrLString
				raw = raw[2:]
			}
		}

		s := processStringEscapes(raw)

		// Deduplicate and intern with kind
		idx := -1
		for i, existing := range p.module.Strings {
			if existing == s && i < len(p.module.StrKinds) && p.module.StrKinds[i] == kind {
				idx = i
				break
			}
		}
		if idx == -1 {
			idx = len(p.module.Strings)
			p.module.Strings = append(p.module.Strings, s)
			p.module.StrKinds = append(p.module.StrKinds, kind)
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
		if p.l.isIdent("target") {
			p.l.next()
			if _, err := p.l.eat(tokLParen); err != nil {
				return nil, err
			}
			if _, err := p.l.eat(tokRParen); err != nil {
				return nil, err
			}
			return &hir.CallExpr{Fn: "@target", Args: nil, Ty: mir2.TyU8}, nil
		}
		if !p.l.isIdent("ptr") {
			return nil, fmt.Errorf("line %d: expected @ptr/@target(...), got @%s", t.line, p.l.peek().val)
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
// resolveCall resolves a function call expression, including partial application.
// If args contain _ placeholders, it desugars into a synthetic lambda.
func (p *parser) resolveCall(base hir.Expr, args []hir.Expr) (hir.Expr, error) {
	name := ""
	if vr, ok := base.(*hir.VarRefExpr); ok {
		name = vr.Name
	}
	// Resolve import alias: "tui_puts" → "tui__render__tui_puts"
	if resolved, ok := p.funcAliases[name]; ok {
		name = resolved
	}

	// Partial application: detect _ placeholders
	// add(5, _) desugars to: fun lambda_N(p0: u8) -> retTy { return add(5, p0) }
	if name != "" {
		var placeholders []int
		for i, arg := range args {
			if vr, ok := arg.(*hir.VarRefExpr); ok && vr.Name == "_" {
				placeholders = append(placeholders, i)
			}
		}
		if len(placeholders) > 0 {
			paramTys := p.funcParamTys[name]
			var lambdaParams []hir.Param
			for j, pos := range placeholders {
				paramName := fmt.Sprintf("_p%d", j)
				pty := mir2.Ty(mir2.TyU8) // default
				if paramTys != nil && pos < len(paramTys) {
					pty = paramTys[pos]
				}
				lambdaParams = append(lambdaParams, hir.Param{Name: paramName, Ty: pty})
				args[pos] = &hir.VarRefExpr{Name: paramName, Ty: pty}
			}
			retTy := mir2.Ty(mir2.TyU8)
			if ty, ok := p.funcSigs[name]; ok {
				retTy = ty
			}
			lambdaName := fmt.Sprintf("lambda_%d", p.lambdaCount)
			p.lambdaCount++
			p.lambdas = append(p.lambdas, &hir.Func{
				Name:   lambdaName,
				Params: lambdaParams,
				RetTy:  retTy,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.ReturnStmt{Val: &hir.CallExpr{Fn: name, Args: args, Ty: retTy}},
				}},
			})
			return &hir.VarRefExpr{Name: lambdaName, Ty: retTy}, nil
		}
	}

	callTy := mir2.Ty(mir2.TyVoid)
	if name != "" {
		if ty, ok := p.funcSigs[name]; ok {
			callTy = ty
		}
	}
	// Indirect call: local variable → function pointer.
	if name != "" && callTy == mir2.TyVoid && !p.isKnownFunc(name) {
		if _, isLocal := p.varTypes[name]; isLocal {
			return &hir.CallIndirectExpr{
				FnPtr: &hir.VarRefExpr{Name: name, Ty: mir2.TyPtr},
				Args:  args,
				Ty:    mir2.TyU8,
			}, nil
		}
		return &hir.CallExpr{Fn: name, Args: args, Ty: callTy}, nil
	}
	return &hir.CallExpr{Fn: name, Args: args, Ty: callTy}, nil
}

// paramTysFromFunc extracts parameter types from an HIR function.
func paramTysFromFunc(f *hir.Func) []mir2.Ty {
	tys := make([]mir2.Ty, len(f.Params))
	for i, pm := range f.Params {
		tys[i] = pm.Ty
	}
	return tys
}

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
		if p.lambdaHintTy != nil {
			pty = p.lambdaHintTy
		}
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

// ── String reference remapping ────────────────────────────────────────────────

// remapStringRefs adjusts @mir2.str.N references in a block by adding offset
// to each string index. This is needed when merging generated code (from
// metafunctions) into a parent module that already has strings.
func remapStringRefs(block *hir.Block, offset int) {
	if block == nil {
		return
	}
	for _, s := range block.Body {
		remapStmtStringRefs(s, offset)
	}
}

func remapStmtStringRefs(s hir.Stmt, offset int) {
	switch s := s.(type) {
	case *hir.ExprStmt:
		remapExprStringRefs(s.Expr, offset)
	case *hir.VarDeclStmt:
		remapExprStringRefs(s.Init, offset)
	case *hir.AssignStmt:
		remapExprStringRefs(s.Val, offset)
	case *hir.ReturnStmt:
		remapExprStringRefs(s.Val, offset)
	case *hir.IfStmt:
		remapExprStringRefs(s.Cond, offset)
		remapStringRefs(s.Then, offset)
		remapStringRefs(s.Else, offset)
	case *hir.WhileStmt:
		remapExprStringRefs(s.Cond, offset)
		remapStringRefs(s.Body, offset)
	case *hir.ForRangeStmt:
		remapExprStringRefs(s.Start, offset)
		remapExprStringRefs(s.End, offset)
		remapStringRefs(s.Body, offset)
	}
}

func remapExprStringRefs(e hir.Expr, offset int) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *hir.AddrOfExpr:
		// @mir2.str.N → @mir2.str.(N+offset)
		var idx int
		if n, _ := fmt.Sscanf(e.Sym, "@mir2.str.%d", &idx); n == 1 {
			e.Sym = fmt.Sprintf("@mir2.str.%d", idx+offset)
		}
	case *hir.CallExpr:
		for _, a := range e.Args {
			remapExprStringRefs(a, offset)
		}
	case *hir.BinExpr:
		remapExprStringRefs(e.L, offset)
		remapExprStringRefs(e.R, offset)
	case *hir.UnaryExpr:
		remapExprStringRefs(e.X, offset)
	case *hir.CastExpr:
		remapExprStringRefs(e.X, offset)
	case *hir.CondExpr:
		remapExprStringRefs(e.Cond, offset)
		remapExprStringRefs(e.Then, offset)
		remapExprStringRefs(e.Else, offset)
	}
}

// remapCallAliases walks HIR and replaces CallExpr.Fn names via alias map.
// Used to resolve @screen-generated calls ("tui_puts") to imported names
// ("tui__render__tui_puts") when import tui.render is present.
func remapCallAliases(block *hir.Block, aliases map[string]string) {
	if block == nil {
		return
	}
	for _, s := range block.Body {
		remapStmtCallAliases(s, aliases)
	}
}

func remapStmtCallAliases(s hir.Stmt, aliases map[string]string) {
	switch s := s.(type) {
	case *hir.ExprStmt:
		remapExprCallAliases(s.Expr, aliases)
	case *hir.VarDeclStmt:
		remapExprCallAliases(s.Init, aliases)
	case *hir.AssignStmt:
		remapExprCallAliases(s.Val, aliases)
	case *hir.ReturnStmt:
		remapExprCallAliases(s.Val, aliases)
	case *hir.IfStmt:
		remapExprCallAliases(s.Cond, aliases)
		remapCallAliases(s.Then, aliases)
		remapCallAliases(s.Else, aliases)
	case *hir.WhileStmt:
		remapExprCallAliases(s.Cond, aliases)
		remapCallAliases(s.Body, aliases)
	case *hir.Block:
		remapCallAliases(s, aliases)
	}
}

func remapExprCallAliases(e hir.Expr, aliases map[string]string) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *hir.CallExpr:
		if resolved, ok := aliases[e.Fn]; ok {
			e.Fn = resolved
		}
		for _, arg := range e.Args {
			remapExprCallAliases(arg, aliases)
		}
	case *hir.BinExpr:
		remapExprCallAliases(e.L, aliases)
		remapExprCallAliases(e.R, aliases)
	}
}

// ── Metafunction support ─────────────────────────────────────────────────────

// captureMetaFuncSource captures the full source text of a metafunction body.
// The caller has already consumed "fun", "@", and the name token.
// We extract the raw source text from the original source bytes using token positions.
func (p *parser) captureMetaFuncSource(name string) (string, error) {
	var sb strings.Builder

	// Build extern declarations for host functions the metafunction can call
	sb.WriteString("@extern fun emit(s: ^u8) -> void\n")
	sb.WriteString("@extern fun block_len() -> u8\n")
	sb.WriteString("@extern fun node_keyword(i: u8) -> ^u8\n")
	sb.WriteString("@extern fun node_arg_count(i: u8) -> u8\n")
	sb.WriteString("@extern fun node_arg_str(i: u8, j: u8) -> ^u8\n")
	sb.WriteString("@extern fun node_kwarg(i: u8, key: ^u8) -> ^u8\n")
	sb.WriteString("@extern fun node_has_kwarg(i: u8, key: ^u8) -> u8\n")
	sb.WriteString("@extern fun str_concat(a: ^u8, b: ^u8) -> ^u8\n")
	sb.WriteString("@extern fun str_from_int(n: u16) -> ^u8\n")
	sb.WriteString("@extern fun str_eq(a: ^u8, b: ^u8) -> u8\n")
	sb.WriteString("@extern fun str_chr(code: u8) -> ^u8\n")
	sb.WriteString("@extern fun block_nodes() -> ^u8\n")
	sb.WriteString("@extern fun emit_tui_puts(s: ^u8) -> void\n")
	sb.WriteString("@extern fun emit_tui_goto(x: u8, y: u8) -> void\n")
	sb.WriteString("@extern fun emit_tui_color(fg: u8, bg: u8, bright: u8) -> void\n")
	sb.WriteString("\n")

	// Include any structs already declared in the module so the metafunction
	// can reference them (e.g. for node: ^BlockNode in nodes[0..n]).
	for _, st := range p.module.Structs {
		sb.WriteString(fmt.Sprintf("struct %s {\n", st.Name))
		for _, f := range st.Fields {
			tyName := "u8"
			switch f.Ty {
			case mir2.TyU16:
				tyName = "u16"
			case mir2.TyI8:
				tyName = "i8"
			case mir2.TyI16:
				tyName = "i16"
			case mir2.TyBool:
				tyName = "bool"
			case mir2.TyPtr:
				tyName = "^u8"
			}
			sb.WriteString(fmt.Sprintf("    %s: %s\n", f.Name, tyName))
		}
		sb.WriteString("}\n\n")
	}

	// Record the start position (current token) in the raw source.
	// We scan forward to find the balanced closing brace, then extract
	// the raw source text between start and end.
	startTok := p.l.peek()
	startPos := startTok.pos

	// Skip forward through tokens to find balanced closing brace
	depth := 0
	started := false
	endPos := startPos
	for !p.l.is(tokEOF) {
		t := p.l.next()
		if t.kind == tokLBrace {
			depth++
			started = true
		}
		if t.kind == tokRBrace {
			depth--
			if started && depth == 0 {
				endPos = t.pos + 1
				break
			}
		}
	}

	// Extract raw source text
	src := p.l.src
	if endPos > len(src) {
		endPos = len(src)
	}
	rawBody := string(src[startPos:endPos])

	sb.WriteString("fun ")
	sb.WriteString(name)
	sb.WriteString(rawBody)
	sb.WriteString("\n")
	return sb.String(), nil
}

// executeMetaInvocation compiles and executes a metafunction, returning emitted Nanz source.
func (p *parser) executeMetaInvocation(metaSrc, funcName string, scalarArgs []string, block []metaBlockNode) (string, error) {
	return executeMetaFuncWithStringArgs(metaSrc, funcName, scalarArgs, block, p.module)
}

// executeMetaFuncWithStringArgs is like executeMetaFunc but pre-allocates
// string arguments on the VM heap before calling the function.
func executeMetaFuncWithStringArgs(
	metaSrc string,
	funcName string,
	stringArgs []string,
	block []metaBlockNode,
	callerMod *hir.Module,
) (string, error) {
	// 1. Parse metafunction source → HIR
	metaHIR, err := Parse(metaSrc, "meta_"+funcName+".nanz")
	if err != nil {
		return "", fmt.Errorf("metafunc @%s: parse error: %w", funcName, err)
	}

	// 2. HIR → MIR2
	mirMod := hir.LowerModule(metaHIR)
	for _, f := range mirMod.Funcs {
		mir2.EliminateDeadBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			if !p && !c {
				break
			}
		}
	}

	// 3. Create VM
	vm := mir2.NewVM(mirMod)
	vm.MaxSteps = 1_000_000
	vm.MaxMemory = 1 << 20

	// 4. Register host functions
	mr := newMetaRuntime(callerMod)
	mr.registerHosts(vm, block)

	// 5. Allocate string args on heap
	var vmArgs []mir2.Value
	for _, s := range stringArgs {
		vmArgs = append(vmArgs, mr.allocString(s))
	}

	// 6. Call metafunction
	_, err = vm.Call(funcName, vmArgs)
	if err != nil {
		return "", fmt.Errorf("metafunc @%s: VM error: %w", funcName, err)
	}

	return mr.emitted.String(), nil
}

// rewriteImportedSymbols walks an HIR statement tree and rewrites function
// calls and global variable references to use mangled names from nameMap.
func rewriteImportedSymbols(stmt hir.Stmt, nameMap map[string]string) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *hir.VarDeclStmt:
		rewriteExpr(s.Init, nameMap)
		for _, e := range s.Initial {
			rewriteExpr(e, nameMap)
		}
	case *hir.AssignStmt:
		rewriteExpr(s.Target, nameMap)
		rewriteExpr(s.Val, nameMap)
	case *hir.ReturnStmt:
		rewriteExpr(s.Val, nameMap)
		for _, e := range s.Vals {
			rewriteExpr(e, nameMap)
		}
	case *hir.ExprStmt:
		rewriteExpr(s.Expr, nameMap)
	case *hir.Block:
		for _, st := range s.Body {
			rewriteImportedSymbols(st, nameMap)
		}
	case *hir.IfStmt:
		rewriteExpr(s.Cond, nameMap)
		if s.Then != nil {
			for _, st := range s.Then.Body {
				rewriteImportedSymbols(st, nameMap)
			}
		}
		if s.Else != nil {
			for _, st := range s.Else.Body {
				rewriteImportedSymbols(st, nameMap)
			}
		}
	case *hir.WhileStmt:
		rewriteExpr(s.Cond, nameMap)
		if s.Body != nil {
			for _, st := range s.Body.Body {
				rewriteImportedSymbols(st, nameMap)
			}
		}
	case *hir.ForRangeStmt:
		rewriteExpr(s.Start, nameMap)
		rewriteExpr(s.End, nameMap)
		if s.Body != nil {
			for _, st := range s.Body.Body {
				rewriteImportedSymbols(st, nameMap)
			}
		}
	case *hir.ForEachStmt:
		rewriteExpr(s.Ptr, nameMap)
		rewriteExpr(s.Start, nameMap)
		rewriteExpr(s.Len, nameMap)
		if s.Body != nil {
			for _, st := range s.Body.Body {
				rewriteImportedSymbols(st, nameMap)
			}
		}
	case *hir.TupleLetStmt:
		rewriteExpr(s.Call, nameMap)
	case *hir.StoreStmt:
		rewriteExpr(s.Ptr, nameMap)
		rewriteExpr(s.Val, nameMap)
	case *hir.SwitchStmt:
		rewriteExpr(s.Val, nameMap)
		for _, c := range s.Cases {
			if c.Body != nil {
				for _, st := range c.Body.Body {
					rewriteImportedSymbols(st, nameMap)
				}
			}
		}
	}
}

func rewriteExpr(expr hir.Expr, nameMap map[string]string) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *hir.CallExpr:
		if mangled, ok := nameMap[e.Fn]; ok {
			e.Fn = mangled
		}
		for _, a := range e.Args {
			rewriteExpr(a, nameMap)
		}
	case *hir.CallIndirectExpr:
		rewriteExpr(e.FnPtr, nameMap)
		for _, a := range e.Args {
			rewriteExpr(a, nameMap)
		}
	case *hir.VarRefExpr:
		if mangled, ok := nameMap[e.Name]; ok {
			e.Name = mangled
		}
	case *hir.BinExpr:
		rewriteExpr(e.L, nameMap)
		rewriteExpr(e.R, nameMap)
	case *hir.UnaryExpr:
		rewriteExpr(e.X, nameMap)
	case *hir.FieldExpr:
		rewriteExpr(e.X, nameMap)
	case *hir.AddrOfExpr:
		if mangled, ok := nameMap[e.Sym]; ok {
			e.Sym = mangled
		}
	case *hir.LoadExpr:
		rewriteExpr(e.Ptr, nameMap)
	case *hir.DerefExpr:
		rewriteExpr(e.Ptr, nameMap)
	case *hir.CastExpr:
		rewriteExpr(e.X, nameMap)
	case *hir.IndexExpr:
		rewriteExpr(e.Base, nameMap)
		rewriteExpr(e.Idx, nameMap)
	case *hir.StructLitExpr:
		for _, f := range e.Fields {
			rewriteExpr(f.Val, nameMap)
		}
	case *hir.CondExpr:
		rewriteExpr(e.Cond, nameMap)
		rewriteExpr(e.Then, nameMap)
		rewriteExpr(e.Else, nameMap)
	}
}
