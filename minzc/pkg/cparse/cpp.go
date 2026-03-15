// Copyright 2021 The CC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cparse

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"modernc.org/token"
)

const (
	maxIncludeLevel = 200 // gcc, std is at least 15.
)

var (
	_ tokenSequence = (*cppTokens)(nil)
	_ tokenSequence = (*dequeue)(nil)
	_ tokenSequence = (*tokenizer)(nil)

	comma  = []byte{','}
	hash   = []byte{'#'}
	nl     = []byte{'\n'}
	one    = []byte{'1'}
	pragma = []byte("pragma")
	qq     = []byte(`""`)
	sp     = []byte{' '}
	zero   = []byte{'0'}

	protectedMacros = map[string]struct{}{
		"__DATE__":                 {},
		"__FILE__":                 {},
		"__LINE__":                 {},
		"__STDC_HOSTED__":          {},
		"__STDC_MB_MIGHT_NEQ_WC__": {},
		"__STDC_VERSION__":         {},
		"__STDC__":                 {},
		"__TIME__":                 {},
		"defined":                  {},
	}
)

// cppParser produces a preprocessingFile.
type cppParser struct {
	s    *scanner
	eh   errHandler
	line []Token // nil when consumed

	closed bool
}

// newCppParser returns a newly created cppParser. The errHandler function is invoked on
// parser errors.
func newCppParser(fset *fset, src Source, eh errHandler) (*cppParser, error) {
	s, err := newScanner(fset, src, eh)
	if err != nil {
		return nil, err
	}

	return &cppParser{s: s, eh: eh}, nil
}

// close causes all subsequent calls to .line to signal EOF.
func (p *cppParser) close() {
	if p.closed {
		return
	}

	if len(p.line) == 0 || p.line[0].Ch != eof {
		p.line = []Token{p.s.newToken(eof)}
	}
	p.closed = true
}

func (p *cppParser) pos() token.Position {
	p.rune()
	return p.line[0].Position()
}

// rune sets p.line to the line p is currently positioned on, up to and
// including its final '\n' token. After all lines are produced or p is closed,
// rune always returns line consisting of a single EOF token, with .Ch set to
// eof.  This EOF token still has proper position and separator information if
// the end of file was reached normally, while parsing.
//
// rune returns p.line[0].Ch.
func (p *cppParser) rune() rune {
	if p.line != nil || p.closed {
		return p.line[0].Ch
	}

	var tok Token
	for tok.Ch != '\n' {
		tok = p.s.cppScan()
		if tok.Ch == eof {
			p.line = []Token{tok}
			p.closed = true
			break
		}

		p.line = append(p.line, tok)
	}
	return p.line[0].Ch
}

// shift returns p.line and if p is not closed, sets p.line to nil
func (p *cppParser) shift() (r []Token) {
	r = p.line
	if !p.closed {
		p.line = nil
	}
	return r
}

// preprocessingFile produces the AST based on [0]6.10.
//
// preprocessing-file: group_opt
func (p *cppParser) preprocessingFile() group { return p.group(false) }

// group:
//
//	group-part
//	group group-part
type group []groupPart

// group:
//
//	group-part
//	group group-part
func (p *cppParser) group(inIfSection bool) (r group) {
	for {
		g := p.groupPart(inIfSection)
		if g == nil {
			break
		}

		r = append(r, g)
		if _, ok := g.(eofLine); ok {
			break
		}
	}
	return r
}

// group-part:
//
//	if-section
//	control-line
//	text-line
//	# non-directive
//	eof-line
type groupPart interface{}

// groupPart parses a group-part.
func (p *cppParser) groupPart(inIfSection bool) groupPart {
	switch p.rune() {
	case '#':
		switch verb := p.line[1].SrcStr(); verb {
		case "if", "ifdef", "ifndef":
			return p.ifSection()
		case "include", "include_next", "define", "undef", "line", "error", "pragma", "\n":
			gp := p.shift()
		out:
			switch {
			case verb == "line":
				// eg. ["#" "line" "1" "\"20010206-1.c\"" "\n"].5
				// or. ["#" "line" "2" "\n"].4
				if len(gp) < 4 {
					break
				}

				var ln uint64
				switch s := gp[2].SrcStr(); {
				case s == "__LINE__":
					ln = uint64(gp[2].Position().Line)
				default:
					var err error
					if ln, err = strconv.ParseUint(gp[2].SrcStr(), 10, 31); err != nil {
						break out
					}
				}

				fn := gp[0].Position().Filename
				if len(gp) >= 4 && gp[3].Ch == rune(STRINGLITERAL) {
					fn = gp[3].SrcStr()
					fn = fn[1 : len(fn)-1]
				}

				nl := gp[len(gp)-1]
				pos := nl.Position()
				p.s.s.file.AddLineInfo(int(pos.Offset+1), fn, int(ln))
			}
			return controlLine(gp)
		case "elif", "else", "endif":
			if inIfSection {
				return nil
			}

			p.eh("%v: unexpected #%s", p.pos(), p.line[1].Src())
			return nonDirective(p.shift())
		default:
			return nonDirective(p.shift())
		}
	case eof:
		return eofLine(p.shift()[0])
	default:
		return textLine(p.shift())
	}
}

// controlLine parses an control-line. At unexpected eof it returns ok == false.
//
// control-line:
//
//	# include pp-tokens new-line
//	# define identifier replacement-list new-line
//	# define identifier lparen identifier-list_opt ) replacement-list new-line
//	# define identifier lparen ... ) replacement-list new-line
//	# define identifier lparen identifier-list , ... ) replacement-list new-line
//	# undef identifier new-line
//	# line pp-tokens new-line
//	# error pp-tokens_opt new-line
//	# pragma pp-tokens_opt new-line
//	# new-line
type controlLine []Token

// textLine is a groupPart representing a source line not starting with '#'.
type textLine []Token

// eofLine is a groupPart representing the end of a file
type eofLine Token

// non-directive is a groupPart representing a source line starting with '#'
// but not followed by any recognized token.
type nonDirective []Token

// if-section:
//
//	if-group elif-groups_opt else-group_opt endif-line
type ifSection struct {
	ifGroup    *ifGroup
	elifGroups []elifGroup
	elseGroup  *elseGroup
	endifLine  []Token
}

// ifSection parses an if-section.
func (p *cppParser) ifSection() *ifSection {
	return &ifSection{
		ifGroup:    p.ifGroup(),
		elifGroups: p.elifGroups(),
		elseGroup:  p.elseGroup(),
		endifLine:  p.endifLine(),
	}
}

// endifLine parses:

// endif-line:
//
//	# endif new-line
func (p *cppParser) endifLine() []Token {
	if p.rune() != '#' || p.line[1].SrcStr() != "endif" {
		p.eh("%v: expected #endif", p.pos())
		return nil
	}

	return p.shift()
}

// else-group:
//
//	# else new-line group_opt
type elseGroup struct {
	line  []Token
	group group
}

// elseGroup parses else-group.
func (p *cppParser) elseGroup() (r *elseGroup) {
	if p.rune() == '#' && p.line[1].SrcStr() == "else" {
		return &elseGroup{p.shift(), p.group(true)}
	}

	return nil
}

// elif-group:
//
//	# elif constant-expression new-line group_opt
type elifGroup struct {
	line  []Token
	group group
}

// elifGroups parses:
//
// elif-groups:
//
//	elif-group
//	elif-groups elif-group
func (p *cppParser) elifGroups() (r []elifGroup) {
	for p.rune() == '#' && p.line[1].SrcStr() == "elif" {
		r = append(r, elifGroup{p.shift(), p.group(true)})
	}
	return r
}

// if-group:
//
//	# if constant-expression new-line group_opt
//	# ifdef identifier new-line group_opt
//	# ifndef identifier new-line group_opt
type ifGroup struct {
	line  []Token
	group group
}

// ifGroup parses an if-group.
func (p *cppParser) ifGroup() (r *ifGroup) { return &ifGroup{p.shift(), p.group(true)} }

type hideSet map[string]struct{}

func (h *hideSet) String() string {
	if h == nil {
		return ""
	}

	hs := *h
	var a []string
	for k := range hs {
		a = append(a, k)
	}
	return fmt.Sprintf("%s", a)
}

func (h hideSet) has(s string) bool { _, ok := h[s]; return ok }

func (h hideSet) add(s string) hideSet {
	r := hideSet{}
	for k := range h {
		r[k] = struct{}{}
	}
	r[s] = struct{}{}
	return r
}

// ∪ - union.
func (h hideSet) cup(src hideSet) hideSet {
	if h == nil {
		return src
	}

	if src == nil {
		return h
	}

	r := hideSet{}
	for k := range h {
		r[k] = struct{}{}
	}
	for k := range src {
		r[k] = struct{}{}
	}
	return r
}

// ∩ - intersection.
func (h hideSet) cap(src hideSet) hideSet {
	if h == nil {
		return nil
	}

	if src == nil {
		return nil
	}

	r := hideSet{}
	for k := range h {
		if _, ok := src[k]; ok {
			r[k] = struct{}{}
		}
	}
	return r
}

type cppToken struct {
	Token
	hs hideSet
}

// Macro represents a preprocessor #define.
type Macro struct {
	Name            Token
	Params          []Token
	params          []string
	replacementList cppTokens
	typer
	valuer

	MinArgs int // m x: 0, m() x: 0, m(...): 0, m(a) a: 1, m(a, ...): 1, m(a, b): 2, m(a, b, ...): 2.
	VarArg  int // m(a): -1, m(...): 0, m(a, ...): 1, m(a...): 0, m(a, b...): 1.

	IsConst  bool // Defined only once or all definitions are the same.
	IsFnLike bool // m: false, m(): true, m(x): true.
}

func newMacro(nm Token, params []Token, replList []cppToken, minArgs, varArg int, isFnLike bool) (*Macro, error) {
	var fp []string
	for _, v := range params {
		fp = append(fp, v.SrcStr())
	}
	if len(fp) > 1 {
		m := map[string]struct{}{}
		for i, v := range fp {
			if _, ok := m[v]; ok {
				return nil, errorf("%v: duplicate parameter name", params[i].Position())
			}
		}
	}
	return &Macro{
		IsConst:         true,
		IsFnLike:        isFnLike,
		MinArgs:         minArgs,
		Name:            nm,
		Params:          params,
		VarArg:          varArg,
		params:          fp,
		replacementList: replList,
	}, nil
}

// Position implements Node.
func (m *Macro) Position() token.Position { return m.Name.Position() }

func (m *Macro) isSame(n *Macro) bool {
	if !bytes.Equal(m.Name.Src(), n.Name.Src()) ||
		len(m.params) != len(n.params) ||
		len(m.replacementList) != len(n.replacementList) ||
		m.MinArgs != n.MinArgs ||
		m.VarArg != n.VarArg ||
		m.IsFnLike != n.IsFnLike {
		return false
	}

	for i, v := range m.params {
		if n.params[i] != v {
			return false
		}
	}
	for i, v := range m.replacementList {
		if !bytes.Equal(v.Src(), n.replacementList[i].Src()) {
			return false
		}
	}

	return true
}

func (m *Macro) is(s string) int {
	for i, v := range m.params {
		if s == v {
			return i
		}
	}

	if m.VarArg >= 0 && s == "__VA_ARGS__" {
		return m.VarArg
	}

	return -1
}

// ReplacementList returns the tokens m is substitued with.
func (m *Macro) ReplacementList() (r []Token) {
	for _, v := range m.replacementList {
		if v.Ch != ' ' {
			r = append(r, v.Token)
		}
	}
	return r
}

func (m *Macro) fp() []string {
	if !m.IsFnLike {
		return nil
	}

	return m.params
}

func (m *Macro) ts() cppTokens { return m.replacementList }

type tokenSequence interface {
	peek(int) cppToken
	peekNonBlank() (cppToken, int)
	prepend(tokenSequence) tokenSequence
	shift() cppToken
	skip(int)
}

type dequeue []tokenSequence

func (d *dequeue) prepend(ts tokenSequence) tokenSequence {
	switch x := ts.(type) {
	case *cppTokens:
		if len(*x) != 0 {
			*d = append(*d, ts)
		}
		return d
	default:
		panic(todo("%T", x))
	}
}

func (d *dequeue) peek(index int) (tok cppToken) {
	panic(todo(""))
}

func (d *dequeue) peekNonBlank() (cppToken, int) {
	q := *d
	if len(q) == 0 {
		panic(todo(""))
	}

	switch e := q[len(q)-1]; x := e.(type) {
	case *tokenizer:
		return x.peekNonBlank()
	case *cppTokens:
		for i, v := range *x {
			if v.Ch != ' ' && v.Ch != '\n' {
				return v, i
			}
		}

		q2 := q[:len(q)-1]
		return q2[len(q2)-1].peekNonBlank()
	default:
		panic(todo("%T", x))
	}
}

func (d *dequeue) shift() (t cppToken) {
	q := *d
	if len(q) == 0 {
		panic(todo(""))
	}

	switch e := q[len(q)-1]; x := e.(type) {
	case *tokenizer:
		if t = x.shift(); t.Ch != eof {
			return t
		}

		*d = q[:len(q)-1]
		return t
	case *cppTokens:
		if t = x.shift(); t.Ch != eof {
			if len(*x) != 0 {
				return t
			}
		}

		*d = q[:len(q)-1]
		return t
	default:
		panic(todo("%T", x))
	}
}

func (d *dequeue) skip(n int) {
	for ; n != 0; n-- {
		d.shift()
	}
}

type tokenizer struct {
	c   *cpp
	in  []cppToken
	out cppTokens
	q   *dequeue
}

func newTokenizer(c *cpp) *tokenizer {
	t := &tokenizer{c: c}
	t.q = &dequeue{t}
	return t
}

func (t *tokenizer) prepend(tokenSequence) tokenSequence {
	panic(todo(""))
}

func (t *tokenizer) peek(index int) (tok cppToken) {
	for {
		if index < len(t.in) {
			return t.in[index]
		}

		t.in = append(t.in, tokens2CppTokens(t.c.nextLine(), false)...)
		continue
	}
}

func (t *tokenizer) peekNonBlank() (cppToken, int) {
	for i := 0; ; i++ {
		if tok := t.peek(i); tok.Ch != ' ' && tok.Ch != '\n' {
			return tok, i
		}
	}
}

func (t *tokenizer) shift() (tok cppToken) {
	if len(t.in) == 0 {
		t.in = tokens2CppTokens(t.c.nextLine(), false)
	}

	tok = t.in[0]
	if tok.Ch != eof {
		t.in = t.in[1:]
	}
	return tok

}

func (t *tokenizer) skip(n int) {
	for ; n != 0; n-- {
		t.shift()
	}
}

func (t *tokenizer) token() (tok Token) {
	for len(t.out) == 0 {
		t.c.expand(true, false, t.q, &t.out)
	}
	tok = t.out[0].Token
	if tok.Ch != eof {
		t.out = t.out[1:]
	}
	return tok
}

type cppTokens []cppToken

func (p *cppTokens) prepend(ts tokenSequence) tokenSequence {
	switch x := ts.(type) {
	case *cppTokens:
		*p = append(*x, *p...)
		return p
	default:
		panic(todo("%T", x))
	}
}

func (p *cppTokens) peek(index int) cppToken {
	if index < len(*p) {
		return (*p)[index]
	}

	return p.eofCppToken()
}

func (p *cppTokens) eofCppToken() cppToken { return cppToken{Token: Token{Ch: eof}} }

func (p *cppTokens) shift() (tok cppToken) {
	s := *p
	if len(s) == 0 {
		return p.eofCppToken()
	}

	tok = s[0]
	if tok.Ch != eof {
		s = s[1:]
		*p = s
	}
	return tok
}

func (p *cppTokens) skipBlank() {
	s := *p
	for len(s) != 0 && s[0].Ch == ' ' {
		s = s[1:]
	}
	*p = s
}

func (p *cppTokens) peekNonBlank() (cppToken, int) {
	for i, v := range *p {
		if v.Ch != ' ' && v.Ch != '\n' {
			return v, i
		}
	}

	return p.eofCppToken(), -1
}

func (p *cppTokens) skip(n int) {
	*p = (*p)[n:]
}

func (p *cppTokens) token() Token {
	p.skipBlank()
	return p.peek(0).Token
}

func (p *cppTokens) rune() rune {
	p.skipBlank()
	return p.peek(0).Ch
}

// cpp is the C preprocessor.
type cpp struct {
	cfg         *Config
	eh          errHandler
	eof         eofLine
	fset        *fset
	groups      map[string]group
	included    *included
	indentLevel int // debug dumps
	macros      map[string]*Macro
	mstack      map[string][]*Macro
	sources     []Source
	stack       []interface{}
	tok         Token
	tokenizer   *tokenizer
	tos         interface{}
	undefs      map[string]struct{}

	counter      int // __COUNTER__
	includeLevel int

	closed bool
}

// newCPP returns a newly created cpp.
func newCPP(cfg *Config, fset *fset, sources []Source, eh errHandler) (*cpp, error) {
	m := map[string]struct{}{}
	for _, v := range sources {
		if _, ok := m[v.Name]; ok {
			return nil, errorf("duplicate source name: %q", v.Name)
		}

		if v.FS == nil {
			v.FS = cfg.FS
		}
		m[v.Name] = struct{}{}
	}
	c := &cpp{
		cfg:     cfg,
		eh:      eh,
		fset:    fset,
		groups:  map[string]group{},
		macros:  map[string]*Macro{},
		mstack:  map[string][]*Macro{},
		sources: sources,
		undefs:  map[string]struct{}{},
	}
	c.tokenizer = newTokenizer(c)
	c.tok.Ch = eof // Invalidate
	return c, nil
}

// rune returns the current token rune the preprocessor is positioned on.
func (c *cpp) rune() rune {
	if c.closed || c.tok.Ch != eof {
		return c.tok.Ch
	}

	if c.tok = c.tokenizer.token(); c.tok.Ch == eof {
		c.closed = true
	}
	return c.tok.Ch
}

// shift returns c.tok and invalidates c.tok.Ch.
func (c *cpp) shift() (r Token) {
	r = c.tok
	c.tok.Ch = eof
	return r
}

//lint:ignore U1000 debug helper
func (c *cpp) indent() string {
	c.indentLevel++
	return fmt.Sprintf("\t%s", strings.Repeat("· ", c.indentLevel-1))
}

//lint:ignore U1000 debug helper
func (c *cpp) undent() string {
	c.indentLevel--
	return fmt.Sprintf("\t%s", strings.Repeat("· ", c.indentLevel))
}

// [1], pg 1.
func (c *cpp) expand(outer, eval bool, TS tokenSequence, w *cppTokens) {
	// trc("* %s%v outer %v (%v)", c.indent(), toksDump(TS), outer, origin(2))
	// defer func() { trc("->%s%v (%v)", c.undent(), toksDump(w), origin(2)) }()
more:
	t := TS.shift()
	// trc("@ %[2]s%v %v", c.undent(), c.indent(), &t, toksDump(TS))
	if t.Ch == eof {
		// if TS is {} then
		//	return {};
		if outer {
			*w = append(*w, t)
		}
		return
	}

	T := t.Token
	HS := t.hs
	src := T.SrcStr()
	if eval && src == "defined" {
		TS = c.parseDefined(TS)
		goto more
	}

	if src == "_Pragma" {
		c.parsePragma(TS)
		goto more
	}

	if HS.has(src) {
		// if TS is T^HS • TS’ and T is in HS then
		//	return T^HS • expand(TS’);
		goto ret
	}

	if m := c.macro(T, src); m != nil {
		var subst cppTokens
	out:
		switch {
		default:
			// trc("  %s<%s is a ()-less macro, replacing by %s>", c.indent(), T.Src(), toksDump(m.replacementList))
			// defer func(n int) { trc("  %s<%s expanded to> %s", c.undent(), T.Src(), toksDump((*w)[n:])) }(len(*w))
			if src == "__COUNTER__" {
				r := m.replacementList[0]
				r.s = t.s
				r.off = t.off
				r.Set(t.Sep(), []byte(fmt.Sprint(c.counter)))
				c.counter++
				m.replacementList[0] = r
			}
			// if TS is T^HS • TS’ and T is a "()-less macro" then
			//	return expand(subst(ts(T),{},{},HS∪{T},{}) • TS’);
			repl := m.ts()
			subst = c.subst(eval, m, repl, nil, nil, HS.add(src), nil)
			for i := range subst {
				subst[i].off = T.off
				subst[i].m = m
			}
			TS.prepend(&subst)
			goto more
		case m.IsFnLike:
			t2, skip := TS.peekNonBlank()
			if t2.Ch == '(' {
				TS.skip(skip + 1)
				args, rparen, ok := c.parseMacroArgs(TS)
				// trc("args=%s rparen=%#U %q ok=%v", toksDump(args), rparen.Ch, rparen.SrcStr(), ok)
				if !ok {
					panic(todo(""))
					// return r
				}

				if len(args) > m.MinArgs && m.VarArg < 0 {
					c.eh("%v: too many macro arguments", rparen.Position())
					break out
				}

				// trc("  %s<%s is a (%v)'d macro, replacing by %s> using args %v", c.indent(), T.Src(), m.fp(), toksDump(m.replacementList), toksDump(args))
				// defer func(n int) { trc("  %s<%s expanded to> %s", c.undent(), T.Src(), toksDump((*w)[n:])) }(len(*w))
				// if TS is T^HS • ( • TS’ and T is a "()’d macro" then
				//	check TS’ is actuals • )^HS’ • TS’’ and actuals are "correct for T"
				//	return expand(subst(ts(T),fp(T),actuals,(HS∩HS’)∪{T},{}) • TS’’);
				subst = c.subst(eval, m, m.ts(), m.fp(), args, HS.cap(rparen.hs).add(src), nil)
				for i := range subst {
					subst[i].off = T.off
				}
				TS.prepend(&subst)
				goto more
			}
		}

	}

ret:
	// note TS must be T HS • TS’
	// return T HS • expand(TS’);
	*w = append(*w, t)
	if !outer {
		goto more
	}
}

// [1], pg 2.
func (c *cpp) subst(eval bool, m *Macro, IS cppTokens, FP []string, AP []cppTokens, HS hideSet, OS cppTokens) (r cppTokens) {
	// trc("* %sIS %v, HS %v, FP %v, AP %v, OS %v (%v)", c.indent(), toksDump(IS), &HS, FP, toksDump(AP), toksDump(OS), origin(2))
	// defer func() { trc("->%s%v", c.undent(), toksDump(r)) }()
	var expandedArgs map[int]cppTokens
more:
	// trc("  %[2]s%v %v", c.undent(), c.indent(), &t, toksDump(IS))
	if len(IS) == 0 {
		// if IS is {} then
		//	return hsadd(HS,OS);
		r = c.hsAdd(HS, OS)
		return r
	}

	t := IS[0]
	IS = IS[1:]
	if t.Ch == '#' {
		if t2, skip := IS.peekNonBlank(); t2.Ch == rune(IDENTIFIER) {
			if i := m.is(t2.SrcStr()); i >= 0 {
				// if IS is # • T • IS’ and T is FP[i] then
				//	return subst(IS’,FP,AP,HS,OS • stringize(select(i,AP )));
				IS = IS[skip+1:]
				OS = append(OS, c.stringize(t, c.apSelect(m, t2, AP, i)))
				goto more
			}
		}
	}

	spaceDeleted := false
	if t.Ch == ' ' && len(IS) != 0 && IS[0].Ch == rune(PPPASTE) { // ' ' "##" -> "##"
		t = IS[0]
		IS = IS[1:]
		spaceDeleted = true
	}
	if t.Ch == rune(PPPASTE) {
		t2, skip := IS.peekNonBlank()
		if t2.Ch == rune(IDENTIFIER) {
			if i := m.is(t2.SrcStr()); i >= 0 {
				// if IS is ## • T • IS’ and T is FP[i] then
				if i >= len(AP) || len(AP[i]) == 0 {
					//	if select(i,AP ) is {} then /* only if actuals can be empty */
					//		return subst(IS’,FP,AP,HS,OS );
					if t2.SrcStr() == "__VA_ARGS__" {
						if len(OS) != 0 && OS[len(OS)-1].Ch == ',' {
							//
							// ... the variable argument is left out when the eprintf macro is used, then
							// the comma before the ‘##’ will be deleted.
							//
							// https://gcc.gnu.org/onlinedocs/gcc-4.9.4/cpp/Variadic-Macros.html
							OS = OS[:len(OS)-1]
						}
					}
					IS.skip(skip + 1)
					goto more
				}

				//	else
				//		return subst(IS’,FP,AP,HS,glue(OS,select(i,AP )));
				IS = IS[skip+1:]
				switch {
				case t2.SrcStr() == "__VA_ARGS__" && spaceDeleted:
					if len(OS) != 0 && OS[len(OS)-1].Ch == ',' {
						OS = append(OS, c.apSelect(m, t2, AP, i)...)
						break
					}

					fallthrough
				default:
					OS = c.glue(OS, c.apSelect(m, t2, AP, i))
				}
				goto more
			}
		}

		// else if IS is ## • T HS’ • IS’ then
		//	return subst(IS’,FP,AP,HS,glue(OS,T^HS’));
		IS = IS[skip+1:]
		OS = c.glue(OS, cppTokens{t2})
		goto more
	}

	if t.Ch == rune(IDENTIFIER) {
		if t2, skip := IS.peekNonBlank(); t2.Ch == rune(PPPASTE) {
			if i := m.is(t.SrcStr()); i >= 0 {
				// if IS is T • ##^HS’ • IS’ and T is FP[i] then
				//	if select(i,AP ) is {} then /* only if actuals can be empty */
				if i >= len(AP) || len(AP[i]) == 0 {
					IS = IS[skip+1:] // ##
					t2, skip := IS.peekNonBlank()
					if j := m.is(t2.SrcStr()); j >= 0 {
						//		if IS’ is T’ • IS’’ and T’ is FP[j] then
						//			return subst(IS’’,FP,AP,HS,OS • select(j,AP));
						IS = IS[skip+1:]
						OS = append(OS, c.apSelect(m, t, AP, j)...)
						goto more
					}

					//		else
					//			return subst(IS’,FP,AP,HS,OS);
					goto more
				} else {
					//	else
					//		return subst(##^HS’ • IS’,FP,AP,HS,OS • select(i,AP));
					OS = append(OS, c.apSelect(m, t, AP, i)...)
					goto more
				}
			}
		}
	}

	if len(FP) != 0 || m.VarArg >= 0 {
		if i := m.is(t.SrcStr()); i >= 0 {
			// if IS is T • IS’ and T is FP[i] then
			//	return subst(IS’,FP,AP,HS,OS • expand(select(i,AP )));
			switch arg, ok := expandedArgs[i]; {
			case ok:
				OS = append(OS, arg...)
			default:
				n := len(OS)
				c.expand(false, eval, c.apSelectP(m, t, AP, i), &OS)
				if expandedArgs == nil {
					expandedArgs = map[int]cppTokens{}
				}
				expandedArgs[i] = OS[n:]
			}
			goto more
		}
	}

	// note IS must be T HS’ • IS’
	// return subst(IS’,FP,AP,HS,OS • T HS’ );
	OS = append(OS, t)
	goto more
}

func (c *cpp) glue(LS, RS cppTokens) (r cppTokens) {
	// trc("  \t%sLS %v, RS %v (%v)", c.indent(), toksDump(LS), toksDump(RS), origin(2))
	// defer func() { trc("->\t%s%v", c.undent(), toksDump(r)) }()

	// if LS is L^HS and RS is R^HS’ • RS’ then
	//	return L&R^(HS∩HS’) • RS’; /* undefined if L&R is invalid */
	switch len(LS) {
	case 0:
		return RS
	case 1:
		t := LS[0]
		switch {
		case t.Ch == rune(IDENTIFIER) && t.SrcStr() == "L":
			switch RS[0].Ch {
			case rune(STRINGLITERAL):
				t.Ch = rune(LONGSTRINGLITERAL)
			case rune(CHARCONST):
				t.Ch = rune(LONGCHARCONST)
			}
		}
		t.Set(nil, append(t.Src(), RS[0].Src()...))
		t.hs = t.hs.cap(RS[0].hs)
		r = append(cppTokens{t}, RS[1:]...)
		return r
	}

	// note LS must be L^HS • LS’
	// return L^HS • glue(LS’,RS );
	r = append(c.glue(LS[1:], RS), cppToken{})
	copy(r[1:], r)
	r[0] = LS[0]
	return r
}

// Given a token sequence, stringize returns a single string literal token
// containing the concatenated spellings of the tokens.
//
// [1] pg. 3
func (c *cpp) stringize(t cppToken, s0 cppTokens) (r cppToken) {
	// trc("  %s%v (%v)", c.indent(), toksDump(s0), origin(2))
	// defer func() { trc("->%s%v: %s", c.undent(), toksDump(cppTokens{r}), r.Src()) }()
	if len(s0) == 0 {
		t.hs = nil
		t.Ch = rune(STRINGLITERAL)
		t.Token.Set(nil, qq)
		return t
	}

	r = s0[0]

	// [0]6.10.3.2
	//
	// Each occurrence of white space between the argument’s preprocessing
	// tokens becomes a single space character in the character string
	// literal.
	s := make([]Token, 0, len(s0))
	var last rune
	for _, v := range s0 {
		if v.Ch == '\n' {
			v.Ch = ' '
			v.Set(nil, sp)
		}
		if v.Ch == ' ' {
			if last == ' ' {
				continue
			}
		}

		last = v.Ch
		s = append(s, v.Token)
	}

	// White space before the first preprocessing token and after the last
	// preprocessing token composing the argument is deleted.
	s = c.toksTrim(s)

	// The character string literal corresponding to an empty argument is
	// ""
	if len(s) == 0 {
		r.Ch = rune(STRINGLITERAL)
		r.Set(nil, []byte(`""`))
		return r
	}

	var a []string
	// Otherwise, the original spelling of each preprocessing token in the
	// argument is retained in the character string literal, except for
	// special handling for producing the spelling of string literals and
	// character constants: a \ character is inserted before each " and \
	// character of a character constant or string literal (including the
	// delimiting " characters), except that it is implementation-defined
	// whether a \ character is inserted before the \ character beginning a
	// universal character name.
	for _, v := range s {
		s := v.SrcStr()
		switch v.Ch {
		case rune(CHARCONST), rune(STRINGLITERAL), rune(LONGCHARCONST), rune(LONGSTRINGLITERAL):
			s = strings.ReplaceAll(s, `\`, `\\`)
			s = strings.ReplaceAll(s, `"`, `\"`)
		}
		a = append(a, s)
	}
	r.Ch = rune(STRINGLITERAL)
	r.Set(nil, []byte(`"`+strings.Join(a, "")+`"`))
	return r
}

func (c *cpp) parsePragma(ts tokenSequence) {
	t, skip := ts.peekNonBlank()
	ts.skip(skip + 1)
	if t.Ch != '(' {
		c.eh("%v: expected '('", t.Position())
		return
	}

	t2, skip := ts.peekNonBlank()
	ts.skip(skip + 1)
	s := t2.SrcStr()
	switch t2.Ch {
	case rune(STRINGLITERAL):
		// ok
	case rune(LONGSTRINGLITERAL):
		s = s[1:] // Remove leading 'L'
	default:
		c.eh("%v: expected string-literal", t2.Position())
		return
	}

	t, skip = ts.peekNonBlank()
	ts.skip(skip + 1)
	if t.Ch != ')' {
		c.eh("%v: expected '('", t.Position())
	}

	// [0]6.10.9 The string literal is destringized by deleting the L prefix, if
	// present, deleting the leading and trailing double-quotes, replacing each
	// escape sequence \" by a double-quote, and replacing each escape sequence \\
	// by a single backslash. The resulting sequence of characters is processed
	// through translation phase 3 to produce preprocessing tokens that are
	// executed as if they were the pp-tokens in a pragma directive.
	s = s[1 : len(s)-1]
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	sc, err := newScanner(c.fset, Source{"_Pragma_", s, nil}, c.eh)
	if err != nil {
		c.eh("%v: %v", t2.Position(), err)
		return
	}

	hashTok := Token{s: t.s, Ch: '#'}
	hashTok.Set(nil, hash)
	pragmaTok := Token{s: t.s, Ch: rune(IDENTIFIER)}
	pragmaTok.Set(nil, pragma)
	a := controlLine{hashTok, pragmaTok}
	for {
		t := sc.cppScan0()
		a = append(a, t)
		if t.Ch == '\n' {
			break
		}
	}
	c.push(a)
}

func (c *cpp) macro(t Token, nm string) *Macro {
	if m := c.macros[nm]; m != nil {
		switch nm {
		case "__FILE__":
			r := m.replacementList[0]
			r.Ch = rune(STRINGLITERAL)
			r.s = t.s
			r.off = t.off
			s := fmt.Sprintf("%q", t.Position().Filename)
			r.Set(t.Sep(), []byte(s))
			m.replacementList[0] = r
		case "__LINE__":
			r := m.replacementList[0]
			r.Ch = rune(PPNUMBER)
			r.s = t.s
			r.off = t.off
			s := fmt.Sprintf(`%d`, t.Position().Line)
			r.Set(t.Sep(), []byte(s))
			m.replacementList[0] = r
		}
		return m
	}

	switch nm {
	case "__COUNTER__":
		nmt := t
		t.Ch = rune(PPNUMBER)
		m, err := newMacro(nmt, nil, []cppToken{{Token: t}}, 0, -1, false)
		m.IsConst = false
		m.val = nil
		if err != nil {
			c.eh("%v", errorf("", err))
			return nil
		}

		c.macros[nm] = m
		return m
	case "defined":
		return nil
	}

	if _, ok := protectedMacros[nm]; !ok {
		return nil
	}

	switch nm {
	case "__FILE__":
		nmt := t
		t.Ch = rune(STRINGLITERAL)
		t.Set(t.Sep(), []byte(fmt.Sprintf("%q", t.Position().Filename)))
		m, err := newMacro(nmt, nil, []cppToken{{Token: t}}, 0, -1, false)
		m.IsConst = false
		m.val = nil
		if err != nil {
			c.eh("%v", errorf("", err))
			return nil
		}

		c.macros[nm] = m
		return m
	case "__LINE__":
		nmt := t
		t.Ch = rune(PPNUMBER)
		t.Set(t.Sep(), []byte(fmt.Sprintf(`%d`, t.Position().Line)))
		m, err := newMacro(nmt, nil, []cppToken{{Token: t}}, 0, -1, false)
		m.IsConst = false
		m.val = nil
		if err != nil {
			c.eh("%v", errorf("", err))
			return nil
		}

		c.macros[nm] = m
		return m
	case "__DATE__":
		nmt := t
		t.Ch = rune(STRINGLITERAL)
		t.Set(t.Sep(), []byte(time.Now().Format("\"Jan _2 2006\"")))
		m, err := newMacro(nmt, nil, []cppToken{{Token: t}}, 0, -1, false)
		if err != nil {
			c.eh("%v", errorf("", err))
			return nil
		}

		c.macros[nm] = m
		return m
	case "__TIME__":
		nmt := t
		t.Ch = rune(STRINGLITERAL)
		t.Set(t.Sep(), []byte(time.Now().Format("\"15:04:05\"")))
		m, err := newMacro(nmt, nil, []cppToken{{Token: t}}, 0, -1, false)
		m.IsConst = false
		m.val = nil
		if err != nil {
			c.eh("%v", errorf("", err))
			return nil
		}

		c.macros[nm] = m
		return m
	case "__STDC__":
		return nil
	case "__STDC_VERSION__":
		return nil
	default:
		panic(todo("%v: %q", t.Position(), nm))
	}
}

func (c *cpp) parseDefined(ts tokenSequence) (r tokenSequence) {
	c.skipBlank(ts)
	var t cppToken
	switch p := ts.peek(0); p.Ch {
	case '(':
		ts.shift()
		c.skipBlank(ts)
		switch p = ts.peek(0); p.Ch {
		case rune(IDENTIFIER):
			t = ts.shift()
			c.skipBlank(ts)
			if paren := ts.shift(); paren.Ch != ')' {
				c.eh("%v: expected ')'", paren.Position())
			}
		default:
			c.eh("%v: operator \"defined\" requires an identifier", p.Position())
			ts.shift()
			return ts
		}
	case rune(IDENTIFIER):
		t = ts.shift()
	default:
		c.eh("%v: operator \"defined\" requires an identifier", p.Position())
		ts.shift()
		return ts
	}

	nm := t.SrcStr()
	oneTok := Token{s: t.s, Ch: rune(PPNUMBER)}
	oneTok.Set(nil, one)
	zeroTok := Token{s: t.s, Ch: rune(PPNUMBER)}
	zeroTok.Set(nil, zero)
	switch c.macro(t.Token, nm) {
	case nil:
		switch nm {
		case "__has_include", "__has_include__":
			t.Token = oneTok
		default:
			t.Token = zeroTok
		}
	default:
		t.Token = oneTok
	}

	s := cppTokens(append([]cppToken{t}, *ts.(*cppTokens)...))
	return &s
}

func (c *cpp) skipBlank(ts tokenSequence) {
	for ts.peek(0).Ch == ' ' {
		ts.shift()
	}
}

func (c *cpp) apSelectP(m *Macro, t cppToken, AP []cppTokens, i int) *cppTokens {
	r := c.apSelect(m, t, AP, i)
	return &r
}

func (c *cpp) apSelect(m *Macro, t cppToken, AP []cppTokens, i int) (r cppTokens) {
	// trc("  %s%v %v i=%v (%v: %v:)", c.indent(), &t, toksDump(AP), i, origin(3), origin(2))
	// defer func() { trc("->%sout %v", c.undent(), toksDump(r)) }()
	if m.VarArg < 0 || m.VarArg != i {
		if i < len(AP) {
			return AP[i]
		}

		return nil
	}

	return c.varArgs(t, AP[i:])
}

func (c *cpp) varArgs(t cppToken, AP []cppTokens) (r cppTokens) {
	// trc("  %s%v %v (%v)", c.indent(), &t, toksDump(AP), origin(2))
	// defer func() { trc("->%sout %v", c.undent(), toksDump(r)) }()
	commaTok := Token{s: t.s, Ch: ','}
	commaTok.Set(nil, comma)
	spTok := Token{s: t.s, Ch: ' '}
	spTok.Set(nil, sp)
	for i, v := range AP {
		if i != 0 {
			r = append(r, cppToken{commaTok, nil})
			if len(v) != 0 && len(v[0].Sep()) != 0 {
				r = append(r, cppToken{spTok, nil})
			}
		}
		r = append(r, v...)
	}
	return r
}

func (c *cpp) parseMacroArgs(TS tokenSequence) (args []cppTokens, rparen cppToken, ok bool) {
	// trc("  %sin  %v (%v)", c.indent(), toksDump(TS), origin(2))
	// defer func() { trc("->%sout %v", c.undent(), toksDump(args)) }()
	var arg cppTokens
	level := 0
	var t cppToken
out:
	for {
		t = TS.shift()
		switch t.Ch {
		case ',':
			if level != 0 {
				arg = append(arg, t)
				break
			}

			args = append(args, arg)
			arg = nil
		case '(':
			arg = append(arg, t)
			level++
		case ')':
			if level == 0 {
				args = append(args, arg)
				break out
			}

			arg = append(arg, t)
			level--
		case '\n':
			continue
		case eof:
			panic(todo("", &t))
		default:
			arg = append(arg, t)
		}
	}
	for i, arg := range args {
		args[i] = toksTrim(arg)
	}
	if len(args) == 1 && len(args[0]) == 0 {
		args = nil
	}
	return args, t, true
}

func (c *cpp) hsAdd(HS hideSet, TS cppTokens) (r cppTokens) {
	if len(TS) == 0 {
		// if TS is {} then
		//	return {};
		return r
	}

	// note TS must be T^HS’ • TS’
	// return T^(HS∪HS’) • hsadd(HS,TS’);
	r = make(cppTokens, len(TS))
	for i, v := range TS {
		v.hs = v.hs.cup(HS)
		r[i] = v
	}
	return r
}

// nextLine returns the next input textLine.
func (c *cpp) nextLine() (r textLine) {
	for {
		// a := []string{fmt.Sprintf("%T", c.tos)}
		// for _, v := range c.stack {
		// 	a = append(a, fmt.Sprintf("%T", v))
		// }
		// trc("STACK %v", a)
		switch x := c.tos.(type) {
		case nil:
			// trc("<nil>, len(c.sources): %v", len(c.sources))
			if len(c.sources) == 0 {
				return textLine{Token(c.eof)}
			}

			src := c.sources[0]
			c.sources = c.sources[1:]
			g, err := c.group(src)
			if err != nil {
				c.eh("%v", err)
				break
			}

			c.push(g)
		case group:
			// trc("group len %v", len(x))
			if len(x) == 0 {
				c.pop()
				break
			}

			c.tos = x[1:]
			c.push(x[0])
		case *included:
			c.included = x
			c.pop()
		case controlLine:
			// trc("%v: controlLine %v", x[0].Position(), toksDump(x))
			c.pop()
			if len(x) < 3 {
				// eg. ["#" "\n"].2
				break
			}

			switch x[1].SrcStr() {
			case "define":
				c.define(x)
			case "undef":
				c.undef(x)
			case "include":
				c.include(x)
			case "include_next":
				c.includeNext(x)
			case "pragma":
				// eg.  ["#" "pragma" "STDC" "FP_CONTRACT" "ON" "\n"]
				switch x[2].SrcStr() {
				case "push_macro":
					// ["#" "pragma" "push_macro" '(' "foo" ')' "\n"]
					if len(x) >= 6 && x[3].Ch == '(' && x[4].Ch == rune(STRINGLITERAL) && x[5].Ch == ')' {
						nm := x[4].SrcStr()
						nm = nm[1 : len(nm)-1]
						m := c.macros[nm]
						c.mstack[nm] = append(c.mstack[nm], m)
					}
				case "pop_macro":
					// ["#" "pragma" "pop_macro" '(' "foo" ')' "\n"]
					if len(x) >= 6 && x[3].Ch == '(' && x[4].Ch == rune(STRINGLITERAL) && x[5].Ch == ')' {
						nm := x[4].SrcStr()
						nm = nm[1 : len(nm)-1]
						s := c.mstack[nm]
						if len(s) == 0 {
							break
						}

						m := s[len(s)-1]
						s = s[:len(s)-1]
						switch m {
						case nil:
							delete(c.macros, nm)
						default:
							c.macros[nm] = m
						}
						c.mstack[nm] = s
					}
				default:
					if h := c.cfg.PragmaHandler; h != nil {
						if err := h(x[2 : len(x)-1]); err != nil {
							c.eh("%v:", err)
						}
					}
				}
			case "line":
				// Handled in cppParser.
			case "error":
				x = x[:len(x)-1] // '\n'
				var b []byte
				for i, v := range x[2:] {
					if i != 0 && len(v.Sep()) != 0 {
						b = append(b, ' ')
					}
					b = append(b, v.Src()...)
				}
				c.eh("%v: %s", x[1].Position(), b)
			default:
				panic(todo("%v: %q", x[0].Position(), x[1].Src()))
			}
		case textLine:
			// trc("%v: textLine %v", x[0].Position(), toksDump(x))
			c.pop()
			return x
		case eofLine:
			c.eof = x
			// trc("%v: eofLine, len(c.stack): %v", (*Token)(&x).Position(), len(c.stack))
			if len(c.stack) == 0 {
				trc("EOF")
				return textLine{Token(x)}
			}

			c.pop()
		case *ifSection:
			switch {
			case x.ifGroup != nil:
				// trc("%v: ifSection/if %v", x.ifGroup.line[0].Position, toksDump(x.ifGroup.line))
				switch {
				case c.ifGroup(x.ifGroup):
					c.tos = x.ifGroup.group
				case len(x.elifGroups) != 0:
					y := *x
					y.ifGroup = nil
					c.tos = &y
				case x.elseGroup != nil:
					c.tos = x.elseGroup.group
				default:
					c.pop()
				}
			case len(x.elifGroups) != 0:
				// trc("%v: ifSection/elif %v", x.elifGroups[0].line[0].Position, toksDump(x.elifGroups[0].line))
				if eg := x.elifGroups[0]; c.elifGroup(eg) {
					c.tos = eg.group
					break
				}

				x.elifGroups = x.elifGroups[1:]
			case x.elseGroup != nil:
				//	# else new-line group_opt
				c.tos = x.elseGroup.group
			default:
				c.pop()
			}
		case nonDirective:
			c.pop()
		default:
			panic(todo("internal error: %T", x))
		}
	}
}

func (c *cpp) elifGroup(eg elifGroup) bool {
	//	# elif constant-expression new-line group_opt
	ln := eg.line[:len(eg.line)-1] // Remove new-line
	if len(ln) < 3 {
		c.eh("%v: expected expression", ln[1].Position())
		return false
	}

	return c.isNonZero(c.eval(ln[2:]))
}

type included struct {
	using string
}

// includeNext executes an #include_next control-line: https://gcc.gnu.org/onlinedocs/cpp/Wrapper-Headers.html
func (c *cpp) includeNext(ln controlLine) {
	// eg. ["#" "include_next" "<stdio.h>" "\n"]
	if len(ln) < 3 {
		c.eh("%v: missing argument", ln[1].Position())
		return
	}

	if c.includeLevel == maxIncludeLevel {
		c.eh("%v: too many include levels", ln[1].Position())
		c.closed = true
		return
	}

	c.includeLevel++
	defer func() { c.includeLevel-- }()

	s0 := cppTokens(tokens2CppTokens(ln[2:], true))
	var s cppTokens
	c.expand(false, true, &s0, &s)
	if c.cfg.fakeIncludes {
		t := s[0].Token
		t.Set(nil, t.Src())
		c.push(textLine([]Token{
			t,
			ln[len(ln)-1],
		}))
		return
	}

	var nm string
	raw := c.includeArg(s)
	if Dmesgs {
		Dmesg("#include_next raw argument in 'raw' %s", raw)
	}
	switch {
	case strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`):
		nm = raw[1 : len(raw)-1]
	case strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">"):
		nm = raw[1 : len(raw)-1]
	default:
		c.eh("%v: invalid argument", s[0].Position())
		c.closed = true
		return
	}

	searchPaths := c.cfg.SysIncludePaths
	var using string
	if c.included != nil {
		for i, v := range searchPaths {
			if c.included.using == v {
				searchPaths = searchPaths[i+1:]
				using = c.included.using
				break
			}
		}
	}
	fn := ln[2].Position().Filename
	if Dmesgs {
		Dmesg("#include_next processed argument in 'nm' %s, current file 'fn' %s, included 'using' %s", nm, fn, using)
		if c.included != nil {
			Dmesg("c.included.using: %s", c.included.using)
		}
	}
	for _, v := range searchPaths {
		v = filepath.Clean(v)
		if Dmesgs {
			Dmesg("#include_next cleaned sysinclude path to try 'v' %s", v)
		}
		pth := filepath.Join(v, nm)
		if Dmesgs {
			Dmesg("#include_next joined 'pth' %s", pth)
		}
		if g, err := c.group(Source{pth, nil, c.cfg.FS}); err == nil {
			if Dmesgs {
				Dmesg("#include_next %s OK, using: %s", nm, v)
			}
			c.push(c.included)
			c.included = &included{using: v}
			c.push(g)
			return
		}
	}

	if Dmesgs {
		Dmesg("#include_next FAIL")
	}
	c.eh("%v: include file not found: %s", s[0].Position(), raw)
	c.closed = true
}

// include executes an #include control-line, [0]6.10.
func (c *cpp) include(ln controlLine) {
	// eg. ["#" "include" "<stdio.h>" "\n"]
	if len(ln) < 3 {
		c.eh("%v: missing argument", ln[1].Position())
		return
	}

	if c.includeLevel == maxIncludeLevel {
		c.eh("%v: too many include levels", ln[1].Position())
		c.closed = true
		return
	}

	c.includeLevel++ //TODO ineffective
	defer func() { c.includeLevel-- }()

	s0 := cppTokens(tokens2CppTokens(ln[2:], true))
	var s cppTokens
	c.expand(false, true, &s0, &s)
	if c.cfg.fakeIncludes {
		t := s[0].Token
		t.Set(nil, t.Src())
		c.push(textLine([]Token{
			t,
			ln[len(ln)-1],
		}))
		return
	}

	switch raw := c.includeArg(s); {
	case strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`):
		nm := raw[1 : len(raw)-1]
		for _, v := range c.cfg.IncludePaths {
			if v == "" || v == "@" {
				v, _ = filepath.Split(ln[2].Position().Filename)
				v = filepath.Clean(v)
			}
			pth := filepath.Join(v, nm)
			if g, err := c.group(Source{pth, nil, c.cfg.FS}); err == nil {
				if Dmesgs {
					Dmesg("#include %s OK, using: %s", nm, v)
				}
				c.push(c.included)
				c.included = &included{using: v}
				c.push(g)
				return
			}
		}

		c.eh("%v: include file not found: %s", s[0].Position(), raw)
		c.closed = true
	case strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">"):
		nm := raw[1 : len(raw)-1]
		for _, v := range c.cfg.SysIncludePaths {
			pth := filepath.Join(v, nm)
			if g, err := c.group(Source{pth, nil, c.cfg.FS}); err == nil {
				if Dmesgs {
					Dmesg("#include %s OK, using: %s", nm, v)
				}
				c.push(c.included)
				c.included = &included{using: v}
				c.push(g)
				return
			}
		}

		c.eh("%v: include file not found: %s", s[0].Position(), raw)
		c.closed = true
	default:
		c.eh("%v: invalid argument", s[0].Position())
		c.closed = true
	}
}

func (c *cpp) hasInclude(t Token, raw string) (r bool) {
	switch {
	case strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`):
		nm := raw[1 : len(raw)-1]
		for _, v := range c.cfg.IncludePaths {
			if v == "" {
				v, _ = filepath.Split(t.Position().Filename)
			}
			pth := filepath.Join(v, nm)
			if c.hasFile(t, pth) {
				return true
			}
		}
	case strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">"):
		nm := raw[1 : len(raw)-1]
		for _, v := range c.cfg.SysIncludePaths {
			pth := filepath.Join(v, nm)
			if c.hasFile(t, pth) {
				return true
			}
		}
	default:
		c.eh("%v: invalid argument", t.Position())
	}
	return false
}

func (c *cpp) hasFile(t Token, fn string) bool {
	var fi os.FileInfo
	var err error
	switch fs := c.cfg.FS; {
	case fs != nil:
		f, err := fs.Open(fn)
		if err != nil {
			break
		}

		defer f.Close()
		if fi, err = f.Stat(); err != nil {
			return false
		}
	}
	if fi == nil {
		if fi, err = os.Stat(fn); err != nil {
			return false
		}
	}

	return !fi.IsDir()
}

func (c *cpp) includeArg(s cppTokens) (r string) {
	switch t := s[0]; t.Ch {
	case rune(STRINGLITERAL), rune(HEADER_NAME):
		return t.SrcStr()
	case '<':
		b := t.Src()
		s = s[1:]
	out:
		for len(s) != 0 {
			t = s[0]
			s = s[1:]
			b = append(b, t.Src()...)
			switch t.Ch {
			case '>':
				break out
			}
		}
		return string(b)
	default:
		return ""
	}
}

func (c *cpp) ifGroup(ig *ifGroup) bool {
	ln := ig.line[:len(ig.line)-1] // Remove new-line
	switch ln[1].SrcStr() {
	case "ifdef":
		if len(ln) < 3 { // '#' "ifdef" IDENTIFIER
			c.eh("%v: expected identifier", ln[1].Position())
			return false
		}

		t := ln[2]
		return c.macro(t, t.SrcStr()) != nil
	case "ifndef":
		if len(ln) < 3 { // '#' "ifndef" IDENTIFIER
			c.eh("%v: expected identifier", ln[1].Position())
			return false
		}

		t := ln[2]
		return c.macro(t, t.SrcStr()) == nil
	case "if":
		if len(ln) < 3 { // '#' "if" <expr>
			c.eh("%v: expected expression", ln[1].Position())
			return false
		}

		return c.isNonZero(c.eval(ln[2:]))
	default:
		panic(todo("", toksDump(ln)))
	}
}

func (c *cpp) eval(s0 []Token) (r interface{}) {
	s1 := cppTokens(tokens2CppTokens(s0, false))
	p := &cppTokens{}
	c.expand(false, true, &s1, p)
	p.skipBlank()
	if len(*p) == 0 {
		c.eh("%v: expected expression", s0[0])
		return int64(0)
	}

	val := c.expression(p, true)
	switch t := p.token(); t.Ch {
	case eof, '#':
		// ok
	default:
		c.eh("%v: unexpected %s", t.Position(), runeName(t.Ch))
		return nil
	}

	return val
}

// [0], 6.5.17 Comma operator
//
//	 expression:
//		assignment-expression
//		expression , assignment-expression
func (c *cpp) expression(s *cppTokens, eval bool) interface{} {
	for {
		r := c.assignmentExpression(s, eval)
		if s.rune() != ',' {
			return r
		}

		s.shift()
	}
}

// [0], 6.5.16 Assignment operators
//
//	 assignment-expression:
//		conditional-expression
//		unary-expression assignment-operator assignment-expression
//
//	 assignment-operator: one of
//		= *= /= %= += -= <<= >>= &= ^= |=
func (c *cpp) assignmentExpression(s *cppTokens, eval bool) interface{} {
	return c.conditionalExpression(s, eval)
}

// [0], 6.5.15 Conditional operator
//
//	 conditional-expression:
//			logical-OR-expression
//			logical-OR-expression ? expression : conditional-expression
func (c *cpp) conditionalExpression(s *cppTokens, eval bool) interface{} {
	expr := c.logicalOrExpression(s, eval)
	if s.rune() == '?' {
		s.shift()
		exprIsNonZero := c.isNonZero(expr)
		expr2 := c.conditionalExpression(s, exprIsNonZero)
		if t := s.token(); t.Ch != ':' {
			c.eh("%v: expected ':'", t.Position())
			return expr
		}

		s.shift()
		expr3 := c.conditionalExpression(s, !exprIsNonZero)

		// [0] 6.5.15
		//
		// 5. If both the second and third operands have arithmetic type, the result
		// type that would be determined by the usual arithmetic conversions, were they
		// applied to those two operands, is the type of the result.
		switch x := expr2.(type) {
		case int64:
			switch expr3.(type) {
			case uint64:
				expr2 = uint64(x)
			}
		case uint64:
			switch y := expr3.(type) {
			case int64:
				expr3 = uint64(y)
			}
		}
		switch {
		case exprIsNonZero:
			expr = expr2
		default:
			expr = expr3
		}
	}
	return expr
}

// [0], 6.5.14 Logical OR operator
//
//	 logical-OR-expression:
//			logical-AND-expression
//			logical-OR-expression || logical-AND-expression
func (c *cpp) logicalOrExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.logicalAndExpression(s, eval)
	for s.rune() == rune(OROR) {
		s.shift()
		if c.isNonZero(lhs) {
			eval = false
		}
		rhs := c.logicalAndExpression(s, eval)
		if c.isNonZero(lhs) || c.isNonZero(rhs) {
			lhs = int64(1)
		}
	}
	return lhs
}

// [0], 6.5.13 Logical AND operator
//
//	 logical-AND-expression:
//			inclusive-OR-expression
//			logical-AND-expression && inclusive-OR-expression
func (c *cpp) logicalAndExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.inclusiveOrExpression(s, eval)
	for s.rune() == rune(ANDAND) {
		s.shift()
		if c.isZero(lhs) {
			eval = false
		}
		rhs := c.inclusiveOrExpression(s, eval)
		if c.isZero(lhs) || c.isZero(rhs) {
			lhs = int64(0)
		}
	}
	return lhs
}

// [0], 6.5.12 Bitwise inclusive OR operator
//
//	 inclusive-OR-expression:
//			exclusive-OR-expression
//			inclusive-OR-expression | exclusive-OR-expression
func (c *cpp) inclusiveOrExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.exclusiveOrExpression(s, eval)
	for s.rune() == '|' {
		s.shift()
		rhs := c.exclusiveOrExpression(s, eval)
		if eval {
			switch x := lhs.(type) {
			case int64:
				switch y := rhs.(type) {
				case int64:
					lhs = x | y
				case uint64:
					lhs = uint64(x) | y
				}
			case uint64:
				switch y := rhs.(type) {
				case int64:
					lhs = x | uint64(y)
				case uint64:
					lhs = x | y
				}
			}
		}
	}
	return lhs
}

// [0], 6.5.11 Bitwise exclusive OR operator
//
//	 exclusive-OR-expression:
//			AND-expression
//			exclusive-OR-expression ^ AND-expression
func (c *cpp) exclusiveOrExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.andExpression(s, eval)
	for s.rune() == '^' {
		s.shift()
		rhs := c.andExpression(s, eval)
		if eval {
			switch x := lhs.(type) {
			case int64:
				switch y := rhs.(type) {
				case int64:
					lhs = x ^ y
				case uint64:
					lhs = uint64(x) ^ y
				}
			case uint64:
				switch y := rhs.(type) {
				case int64:
					lhs = x ^ uint64(y)
				case uint64:
					lhs = x ^ y
				}
			}
		}
	}
	return lhs
}

// [0], 6.5.10 Bitwise AND operator
//
//	 AND-expression:
//			equality-expression
//			AND-expression & equality-expression
func (c *cpp) andExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.equalityExpression(s, eval)
	for s.rune() == '&' {
		s.shift()
		rhs := c.equalityExpression(s, eval)
		if eval {
			switch x := lhs.(type) {
			case int64:
				switch y := rhs.(type) {
				case int64:
					lhs = x & y
				case uint64:
					lhs = uint64(x) & y
				}
			case uint64:
				switch y := rhs.(type) {
				case int64:
					lhs = x & uint64(y)
				case uint64:
					lhs = x & y
				}
			}
		}
	}
	return lhs
}

// [0], 6.5.9 Equality operators
//
//	 equality-expression:
//			relational-expression
//			equality-expression == relational-expression
//			equality-expression != relational-expression
func (c *cpp) equalityExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.relationalExpression(s, eval)
	for {
		var v bool
		switch s.token().Ch {
		case rune(EQ):
			s.shift()
			rhs := c.relationalExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						v = x == y
					case uint64:
						v = uint64(x) == y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						v = x == uint64(y)
					case uint64:
						v = x == y
					}
				}
			}
		case rune(NEQ):
			s.shift()
			rhs := c.relationalExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						v = x != y
					case uint64:
						v = uint64(x) != y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						v = x != uint64(y)
					case uint64:
						v = x != y
					}
				}
			}
		default:
			return lhs
		}
		switch {
		case v:
			lhs = int64(1)
		default:
			lhs = int64(0)
		}
	}
}

// [0], 6.5.8 Relational operators
//
//	 relational-expression:
//			shift-expression
//			relational-expression <  shift-expression
//			relational-expression >  shift-expression
//			relational-expression <= shift-expression
//			relational-expression >= shift-expression
func (c *cpp) relationalExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.shiftExpression(s, eval)
	for {
		var v bool
		switch s.token().Ch {
		case '<':
			s.shift()
			rhs := c.shiftExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						v = x < y
					case uint64:
						v = uint64(x) < y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						v = x < uint64(y)
					case uint64:
						v = x < y
					}
				}
			}
		case '>':
			s.shift()
			rhs := c.shiftExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						v = x > y
					case uint64:
						v = uint64(x) > y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						v = x > uint64(y)
					case uint64:
						v = x > y
					}
				}
			}
		case rune(LEQ):
			s.shift()
			rhs := c.shiftExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						v = x <= y
					case uint64:
						v = uint64(x) <= y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						v = x <= uint64(y)
					case uint64:
						v = x <= y
					}
				}
			}
		case rune(GEQ):
			s.shift()
			rhs := c.shiftExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						v = x >= y
					case uint64:
						v = uint64(x) >= y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						v = x >= uint64(y)
					case uint64:
						v = x >= y
					}
				}
			}
		default:
			return lhs
		}
		switch {
		case v:
			lhs = int64(1)
		default:
			lhs = int64(0)
		}
	}
}

// [0], 6.5.7 Bitwise shift operators
//
//	 shift-expression:
//			additive-expression
//			shift-expression << additive-expression
//			shift-expression >> additive-expression
func (c *cpp) shiftExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.additiveExpression(s, eval)
	for {
		switch s.token().Ch {
		case rune(LSH):
			s.shift()
			rhs := c.additiveExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						lhs = x << uint(y)
					case uint64:
						lhs = uint64(x) << uint(y)
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						lhs = x << uint(y)
					case uint64:
						lhs = x << uint(y)
					}
				}
			}
		case rune(RSH):
			s.shift()
			rhs := c.additiveExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						lhs = x >> uint(y)
					case uint64:
						lhs = uint64(x) >> uint(y)
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						lhs = x >> uint(y)
					case uint64:
						lhs = x >> uint(y)
					}
				}
			}
		default:
			return lhs
		}
	}
}

// [0], 6.5.6 Additive operators
//
//	 additive-expression:
//			multiplicative-expression
//			additive-expression + multiplicative-expression
//			additive-expression - multiplicative-expression
func (c *cpp) additiveExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.multiplicativeExpression(s, eval)
	for {
		switch s.token().Ch {
		case '+':
			s.shift()
			rhs := c.multiplicativeExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						lhs = x + y
					case uint64:
						lhs = uint64(x) + y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						lhs = x + uint64(y)
					case uint64:
						lhs = x + y
					}
				}
			}
		case '-':
			s.shift()
			rhs := c.multiplicativeExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						lhs = x - y
					case uint64:
						lhs = uint64(x) - y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						lhs = x - uint64(y)
					case uint64:
						lhs = x - y
					}
				}
			}
		default:
			return lhs
		}
	}
}

// [0], 6.5.5 Multiplicative operators
//
//	 multiplicative-expression:
//			unary-expression // [0], 6.10.1, 1.
//			multiplicative-expression * unary-expression
//			multiplicative-expression / unary-expression
//			multiplicative-expression % unary-expression
func (c *cpp) multiplicativeExpression(s *cppTokens, eval bool) interface{} {
	lhs := c.unaryExpression(s, eval)
	for {
		switch s.token().Ch {
		case '*':
			s.shift()
			rhs := c.unaryExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						lhs = x * y
					case uint64:
						lhs = uint64(x) * y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						lhs = x * uint64(y)
					case uint64:
						lhs = x * y
					}
				}
			}
		case '/':
			t := s.shift()
			rhs := c.unaryExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						if y == 0 {
							c.eh("%v: division by zero", t.Position())
							break
						}

						lhs = x / y
					case uint64:
						if y == 0 {
							c.eh("%v: division by zero", t.Position())
							break
						}

						lhs = uint64(x) / y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						if y == 0 {
							c.eh("%v: division by zero", t.Position())
							break
						}

						lhs = x / uint64(y)
					case uint64:
						if y == 0 {
							c.eh("%v: division by zero", t.Position())
							break
						}

						lhs = x / y
					}
				}
			}
		case '%':
			t := s.shift()
			rhs := c.unaryExpression(s, eval)
			if eval {
				switch x := lhs.(type) {
				case int64:
					switch y := rhs.(type) {
					case int64:
						if y == 0 {
							c.eh("%v: division by zero", t.Position())
							break
						}

						lhs = x % y
					case uint64:
						if y == 0 {
							c.eh("%v: division by zero", t.Position())
							break
						}

						lhs = uint64(x) % y
					}
				case uint64:
					switch y := rhs.(type) {
					case int64:
						if y == 0 {
							c.eh("%v: division by zero", t.Position())
							break
						}

						lhs = x % uint64(y)
					case uint64:
						if y == 0 {
							c.eh("%v: division by zero", t.Position())
							break
						}

						lhs = x % y
					}
				}
			}
		default:
			return lhs
		}
	}
}

// [0], 6.5.3 Unary operators
//
//	 unary-expression:
//			primary-expression
//			unary-operator unary-expression
//
//	 unary-operator: one of
//			+ - ~ !
func (c *cpp) unaryExpression(s *cppTokens, eval bool) interface{} {
	switch s.token().Ch {
	case '+':
		s.shift()
		return c.unaryExpression(s, eval)
	case '-':
		s.shift()
		expr := c.unaryExpression(s, eval)
		if eval {
			switch x := expr.(type) {
			case int64:
				expr = -x
			case uint64:
				expr = -x
			}
		}
		return expr
	case '~':
		s.shift()
		expr := c.unaryExpression(s, eval)
		if eval {
			switch x := expr.(type) {
			case int64:
				expr = ^x
			case uint64:
				expr = ^x
			}
		}
		return expr
	case '!':
		s.shift()
		expr := c.unaryExpression(s, eval)
		if eval {
			var v bool
			switch x := expr.(type) {
			case int64:
				v = x == 0
			case uint64:
				v = x == 0
			}
			switch {
			case v:
				expr = int64(1)
			default:
				expr = int64(0)
			}
		}
		return expr
	default:
		return c.primaryExpression(s, eval)
	}
}

// [0], 6.5.1 Primary expressions
//
//	 primary-expression:
//			identifier
//			constant
//			( expression )
func (c *cpp) primaryExpression(s *cppTokens, eval bool) interface{} {
	switch t := s.token(); t.Ch {
	case rune(CHARCONST), rune(LONGCHARCONST):
		s.shift()
		r := charConst(c.eh, t)
		if r < 0 {
			r = -r
		}
		return int64(r)
	case rune(IDENTIFIER):
		s.shift()
		switch t.SrcStr() {
		case "__has_include", "__has_include__":
			t0 := t
			var arg string
			if t := s.token(); t.Ch != '(' {
				c.eh("%v: expected '('", t.Position())
				for {
					s.shift()
					switch s.token().Ch {
					case ')':
						s.shift()
						return int64(0)
					case eof:
						return int64(0)
					}
				}
				return int64(0)
			}
			s.shift()
			var b []byte
		out:
			switch t := s.token(); t.Ch {
			case rune(STRINGLITERAL):
				s.shift()
				arg = t.SrcStr()
			default:
				for {
					b = append(b, t.Src()...)
					s.shift()
					switch t = s.token(); t.Ch {
					case ')', eof:
						arg = string(b)
						break out
					}
				}
			}
			switch t = s.token(); t.Ch {
			case ')':
				s.shift()
				if c.hasInclude(t0, arg) {
					return int64(1)
				}

				return int64(0)
			default:
				c.eh("%v: expected ')'", t.Position())
				return int64(0)
			}
		}

		if s.rune() == '(' {
			for {
				s.shift()
				switch s.token().Ch {
				case ')', eof:
					s.shift()
					return int64(0)
				}
			}
		}
		return int64(0)
	case rune(PPNUMBER):
		s.shift()
		return c.intConst(t)
	case '(':
		s.shift()
		expr := c.expression(s, eval)
		if s.rune() == ')' {
			s.shift()
			return expr
		}

		c.eh("%v: unabalanced parenthesis", t.Position())
	case '#':
		c.eh("%v: assertions are a deprecated extension", t.Position())
	default:
		c.eh("%v: internal error", t.Position())
	}
	return int64(0)
}

// [0], 6.4.4.1 Integer constants
//
//	 integer-constant:
//			decimal-constant integer-suffix_opt
//			octal-constant integer-suffix_opt
//			hexadecimal-constant integer-suffix_opt
//
//	 decimal-constant:
//			nonzero-digit
//			decimal-constant digit
//
//	 octal-constant:
//			0
//			octal-constant octal-digit
//
//	 hexadecimal-prefix: one of
//			0x 0X
//
//	 integer-suffix_opt: one of
//			u ul ull l lu ll llu
func (c *cpp) intConst(t Token) (r interface{}) {
	var n uint64
	s0 := t.SrcStr()
	s := strings.TrimRight(s0, "uUlL")
	var err error
	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		if n, err = strconv.ParseUint(s[2:], 16, 64); err != nil {
			c.eh("%v: %v", t.Position(), err)
			return int64(0)
		}
	case strings.HasPrefix(s, "0"):
		if n, err = strconv.ParseUint(s, 8, 64); err != nil {
			c.eh("%v: %v", t.Position(), err)
			return int64(0)
		}
	default:
		if n, err = strconv.ParseUint(s, 10, 64); err != nil {
			c.eh("%v: %v", t.Position(), err)
			return int64(0)
		}
	}

	switch suffix := strings.ToLower(s0[len(s):]); suffix {
	case "", "l", "ll":
		if n > math.MaxInt64 {
			return n
		}

		return int64(n)
	case "llu", "lu", "u", "ul", "ull":
		return n
	default:
		panic(todo(""))
		// c.err(t, "invalid suffix: %v", s0)
		// fallthrough
	}
}

func (c *cpp) isZero(val interface{}) bool {
	switch x := val.(type) {
	case int64:
		return x == 0
	case uint64:
		return x == 0
	default:
		c.eh("", errorf("internal error: %T", x))
		return false
	}
}

func (c *cpp) isNonZero(val interface{}) bool {
	switch x := val.(type) {
	case nil:
		return false
	case int64:
		return x != 0
	case uint64:
		return x != 0
	default:
		c.eh("", errorf("internal error: %T", x))
		return false
	}
}

// undef executes an #undef control-line, [0]6.10.
func (c *cpp) undef(ln controlLine) {
	// eg. ["#" "undef" "  x" "      \n"]
	if len(ln) < 3 {
		return
	}

	nm := ln[2].SrcStr()
	if _, ok := protectedMacros[nm]; !ok {
		c.undefs[nm] = struct{}{}
		delete(c.macros, nm)
	}
}

// define executes a #define control-line, [0]6.10.
func (c *cpp) define(ln controlLine) {
	def := ln[1]
	ln = ln[2:] // Remove '#' and "define"
	if len(ln) == 0 {
		c.eh("%v: missing macro name", def.Position())
		return
	}

	nm := ln[0]
	ln = ln[1:]
	switch {
	case ln[0].Ch == '(' && len(ln[0].Sep()) == 0:
		// lparen: a ( character not immediately preceded by white-space
		// # define identifier ( args_opt ) replacement-list new-line
		//                     ^ln[0]
		c.defineFnMacro(nm, ln[:len(ln)-1]) // strip new-line
	default:
		// # define identifier replacement-list new-line
		//                     ^ln[0]
		c.defineObjectMacro(nm, ln[:len(ln)-1]) // strip new-line
	}
}

func (c *cpp) defineFnMacro(nm Token, ln []Token) {
	fp, rl0, minArgs, varArg := c.parseMacroParams(ln)
	rl := c.parseReplacementList(rl0)
	c.newMacro(nm, fp, rl, minArgs, varArg, true)
}

func (c *cpp) parseMacroParams(ln []Token) (fp, rl []Token, minArgs, vaArg int) {
	if len(ln) == 0 || ln[0].Ch != '(' {
		c.eh("internal error")
		return nil, nil, -1, -1
	}

	lpar := ln[0]
	ln = ln[1:] // remove '('
	vaArg = -1
	for {
		if len(ln) == 0 {
			// (A)
			c.eh("%v: macro paramater list is missing final ')'", lpar.Position())
			return nil, nil, -1, -1
		}

		switch ln[0].Ch {
		case ')':
			// (B)
			ln = ln[1:]
			return fp, ln, minArgs, vaArg
		case rune(IDENTIFIER):
			fp = append(fp, ln[0])
			minArgs++
			ln = ln[1:]
			if len(ln) == 0 {
				break // -> (A)
			}

			switch ln[0].Ch {
			case ')':
				// ok -> (B)
			case ',':
				ln = ln[1:]
			case rune(DDD):
				if vaArg >= 0 {
					c.eh("%v: multiple var arguments", ln[0].Position())
					return nil, nil, -1, -1
				}

				vaArg = len(fp) - 1
				ln = ln[1:]
			default:
				c.eh("%v: unexpected %v", ln[0].Position(), runeName(ln[0].Ch))
				return nil, nil, -1, -1
			}
		case rune(DDD):
			if vaArg >= 0 {
				c.eh("%v: multiple var arguments", ln[0].Position())
				return nil, nil, -1, -1
			}

			vaArg = len(fp)
			ln = ln[1:]
			if len(ln) == 0 {
				break // -> (A)
			}

			switch ln[0].Ch {
			case ')':
				// ok -> (B)
			default:
				ln = nil // -> (A)
			}
		default:
			c.eh("%v: unexpected %v", ln[0].Position(), runeName(ln[0].Ch))
			return nil, nil, -1, -1
		}
	}
}

func (c *cpp) defineObjectMacro(nm Token, ln []Token) {
	rl := c.parseReplacementList(ln)
	c.newMacro(nm, nil, rl, -1, -1, false)
}

func (c *cpp) newMacro(nm Token, params []Token, replList []cppToken, minArgs, varArg int, isFnLike bool) {
	// trc("nm %q, params %v, replList %v, minArgs %v, varArg %v, isFnLike %v", nm.Src(), toksDump(params), toksDump(replList), minArgs, varArg, isFnLike)
	s := nm.SrcStr()
	if _, ok := protectedMacros[s]; ok {
		if nm.Position().Filename != "<predefined>" {
			return
		}
	}

	m, err := newMacro(nm, params, replList, minArgs, varArg, isFnLike)
	if err != nil {
		c.eh("%v", err)
		return
	}

	_, undef := c.undefs[s]
	if ex := c.macros[s]; undef || ex != nil && (!ex.IsConst || !m.isSame(ex)) {
		m.IsConst = false
		m.val = nil
	}
	c.macros[s] = m
}

// parseReplacementList transforms s into preprocessing tokens that have separate
// tokens for white space.
func (c *cpp) parseReplacementList(s []Token) (r []cppToken) {
	return tokens2CppTokens(c.toksTrim(s), true)
}

func (c *cpp) toksTrim(s []Token) []Token {
	for len(s) != 0 && s[0].Ch == ' ' {
		s = s[1:]
	}
	for len(s) != 0 && s[len(s)-1].Ch == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func (c *cpp) pop() {
	if n := len(c.stack); n != 0 {
		c.tos = c.stack[n-1]
		c.stack = c.stack[:n-1]
		return
	}

	c.tos = nil
}

func (c *cpp) push(v interface{}) {
	if c.tos != nil {
		c.stack = append(c.stack, c.tos)
	}
	c.tos = v
}

func (c *cpp) group(src Source) (group, error) {
	if g, ok := c.groups[src.Name]; ok {
		return g, nil
	}

	if src.FS == nil {
		src.FS = c.cfg.FS
	}

	p, err := newCppParser(c.fset, src, c.eh)
	if err != nil {
		return nil, err
	}

	g := p.group(false)
	c.groups[src.Name] = g
	return g, nil
}
