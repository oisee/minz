// Copyright 2021 The CC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cparse

import (
	"bytes"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"unicode"
	"unicode/utf8"

	"modernc.org/token"
)

// Static type asserts.
var (
	_ Node = (*Token)(nil)
)

// Rune range used for IDENTIFIER and similar non-characters.
const (
	eof                     = -1
	unicodePrivateAreaFirst = 0xe000
	unicodePrivateAreaLast  = 0xf8ff
)

// errHandler is a function called on error.
type errHandler func(msg string, args ...interface{})

// tokCh enables using stringer.
type tokCh int

// Values of Token.Rune representing non-character categories.
const (
	_ tokCh = iota + unicodePrivateAreaFirst

	ADDASSIGN         // '+='
	ALIGNAS           // '_Alignas'
	ALIGNOF           // '_Alignof'
	ANDAND            // '&&'
	ANDASSIGN         // '&='
	ARROW             // '->'
	ASM               // '__asm__'
	ATOMIC            // '_Atomic'
	ATTRIBUTE         // '__attribute__'
	AUTO              // 'auto'
	AUTOTYPE          // '__auto_type'
	BOOL              // '_Bool'
	BREAK             // 'break'
	CASE              // 'case'
	CHAR              // 'char'
	CHARCONST         // character constant
	COMPLEX           // '_Complex'
	CONST             // 'const'
	CONTINUE          // 'continue'
	DDD               // '...'
	DEC               // '--'
	DECIMAL128        // '_Decimal128'
	DECIMAL32         // '_Decimal32'
	DECIMAL64         // '_Decimal64'
	DECLSPEC          // '__declspec'
	DEFAULT           // 'default'
	DIVASSIGN         // '/='
	DO                // 'do'
	DOUBLE            // 'double'
	ELSE              // 'else'
	ENUM              // 'enum'
	EQ                // '=='
	EXTERN            // 'extern'
	FLOAT             // 'float'
	FLOAT128          // '_Float128'
	FLOAT128X         // '_Float128x'
	FLOAT16           // '_Float16'
	FLOAT32           // '_Float32'
	FLOAT32X          // '_Float32x'
	FLOAT64           // '_Float64'
	FLOAT64X          // '_Float64x'
	FLOATCONST        // floating point constant
	FOR               // 'for'
	GENERIC           // '_Generic'
	GEQ               // '>='
	GOTO              // 'goto'
	HEADER_NAME       // <header-name>
	IDENTIFIER        // identifier
	IF                // 'if'
	IMAG              // '__imag__'
	IMAGINARY         // '_Imaginary'
	INC               // '++'
	INLINE            // 'inline'
	INT               // 'int'
	INT128            // '__int128'
	INTCONST          // integer constant
	LABEL             // '__label__'
	LEQ               // '<='
	LONG              // 'long'
	LONGCHARCONST     // long character constant
	LONGSTRINGLITERAL // long string literal
	LSH               // '<<'
	LSHASSIGN         // '<<='
	MODASSIGN         // '%='
	MULASSIGN         // '*='
	NEQ               // '!='
	NONNULL           // '_Nonnull'
	NORETURN          // '_Noreturn'
	ORASSIGN          // '|='
	OROR              // '||'
	PPNUMBER          // preprocessing number
	PPPASTE           // '##'
	REAL              // '__real__'
	REGISTER          // 'register'
	RESTRICT          // 'restrict'
	RETURN            // 'return'
	RSH               // '>>'
	RSHASSIGN         // '>>='
	SHORT             // 'short'
	SIGNED            // 'signed'
	SIZEOF            // 'sizeof'
	STATIC            // 'static'
	STATICASSERT      // _Static_assert
	STRINGLITERAL     // string literal
	STRUCT            // 'struct'
	SUBASSIGN         // '-='
	SWITCH            // 'switch'
	THREADLOCAL       // '_Thread_local'
	TYPEDEF           // 'typedef'
	TYPENAME          // type name
	TYPEOF            // 'typeof'
	UINT128           // '__uint128_t'
	UNION             // 'union'
	UNSIGNED          // 'unsigned'
	VOID              // 'void'
	VOLATILE          // 'volatile'
	WHILE             // 'while'
	XORASSIGN         // '^='

	// ── ObjC extensions (MinZ fork) ──────────────────────────────
	OBJC_INTERFACE      // '@interface'
	OBJC_IMPLEMENTATION // '@implementation'
	OBJC_PROTOCOL       // '@protocol'
	OBJC_END            // '@end'
	OBJC_PROPERTY       // '@property'
	OBJC_SYNTHESIZE     // '@synthesize'
	OBJC_DYNAMIC        // '@dynamic'
	OBJC_SELECTOR       // '@selector'
	OBJC_ENCODE         // '@encode'
	OBJC_CLASS          // '@class'
	OBJC_OPTIONAL       // '@optional'
	OBJC_REQUIRED       // '@required'
	OBJC_IMPORT         // '#import' (preprocessor)
	OBJC_YES            // 'YES'
	OBJC_NO             // 'NO'
	OBJC_NIL            // 'nil'
	OBJC_SELF           // 'self'
	OBJC_SUPER          // 'super'
)

// Node is implemented by Token and AST nodes.
type Node interface {
	Position() token.Position
}

// Token is the lexical token produced by the scanner.
type Token struct { // 32 bytes on a 64 bit machine.
	s   *scannerSource
	Ch  rune // '*' or IDENTIFIER etc.
	off uint32
	seq int32  // Sequence number, determines scope boundaries.
	sep uint32 // Index into .ss.buf of the preceding white space, including comments. Length is .src-.sep.
	src uint32 // Index into .ss.buf, length is in .len.
	len uint32 // Length of the source representation (.src).
	m   *Macro
}

// newToken returns a newly created Token. The pos field is set equal to src.
func newToken(s *scannerSource, ch rune, sep, src, len uint32) (r Token) {
	return Token{
		s:   s,
		Ch:  ch,
		off: src + uint32(s.fsOff),
		sep: sep,
		src: src,
		len: len,
	}
}

// String implements fmt.Stringer.
func (t Token) String() string { return PrettyString(t) }

// Name returns a human readable representation of t.Ch.
func (t *Token) Name() string { return runeName(t.Ch) }

// Seq returns t's sequence number in the token stream. The information
// determines scope boundaries.
func (t *Token) Seq() int { return int(t.seq) }

// Sep returns any white space, including comments, that precede t. The result
// is R/O but it's okay to append to it.
func (t *Token) Sep() []byte {
	if t.s == nil {
		return nil
	}

	n := t.src - t.sep
	return t.s.buf[t.sep : t.sep+n : t.sep+n]
}

// Src returns the source representation of t. The result is R/O but it's okay
// to append to it.
func (t *Token) Src() []byte {
	if t.s == nil {
		return nil
	}

	n := t.len
	return t.s.buf[t.src : t.src+n : t.src+n]
}

// SrcStr returns the source representation of t as a string.
// To avoid allocations, consider using Src.
func (t *Token) SrcStr() string {
	return string(t.Src())
}

// Set sets the values Sep and Src will report.
func (t *Token) Set(sep, src []byte) error {
	if uint64(len(sep)+len(src)) > math.MaxUint32 {
		return errorf("Token.Set: argument values too long")
	}

	if uint64(len(t.s.buf)+len(sep)+len(src)) > math.MaxUint32 {
		return errorf("Token.Set: underlying scanner source buffer overflow: size is already %v", len(t.s.buf))
	}
	t.sep = uint32(len(t.s.buf))
	t.s.buf = append(t.s.buf, sep...)
	t.src = uint32(len(t.s.buf))
	t.s.buf = append(t.s.buf, src...)
	t.len = uint32(len(src))
	return nil
}

// Position implements Node.
func (t Token) Position() (r token.Position) {
	if t.s == nil {
		return r
	}

	return t.s.fset.Position(int32(t.off) + int32(t.s.pos0))
}

// scannerSource captures source code and the associated position information.
type scannerSource struct {
	buf  []byte
	file *token.File
	fset *fset

	fsOff int32
	len   uint32 // Len of source code in .buf.
	pos0  token.Pos
}

// newScannerSource returns a new scanner source.
func newScannerSource(fset *fset, src Source) (s *scannerSource, err error) {
	s = &scannerSource{fset: fset}
	switch x := src.Value.(type) {
	case io.ReadCloser:
		defer func() {
			if e := x.Close(); e != nil && err == nil {
				err = e
			}
		}()

		if s.buf, err = io.ReadAll(x); err != nil {
			return nil, errorf("", err)
		}
	case []byte:
		s.buf = x
	case io.Reader:
		if s.buf, err = io.ReadAll(x); err != nil {
			return nil, errorf("", err)
		}
	case nil:
		if fs := src.FS; fs != nil {
			if f, err := fs.Open(filepath.ToSlash(src.Name)); err == nil {
				return newScannerSource(fset, Source{src.Name, f, nil})
			}
		}

		f, err := os.Open(src.Name)
		if err != nil {
			return nil, errorf("", err)
		}

		return newScannerSource(fset, Source{src.Name, fs.File(f), nil})
	case string:
		s.buf = []byte(x)
	default:
		return nil, errorf("invalid value type: %T", x)
	}
	if uint64(len(s.buf)) >= math.MaxUint32 {
		return nil, errorf("source too big: %v bytes", len(s.buf))
	}

	// [0]5.1.1.2, 2: A source file that is not empty shall end in a new-line character, ...
	if len(s.buf) != 0 && s.buf[len(s.buf)-1] != '\n' {
		s.buf = append(s.buf, '\n')
	}
	s.len = uint32(len(s.buf))
	s.file = token.NewFile(src.Name, int(s.len))
	s.pos0 = s.file.Pos(0)
	s.fsOff, err = fset.add(s.file)
	return s, err
}

// pos returns a token.Position representing off.
func (s *scannerSource) pos(off uint32) token.Position {
	return s.file.PositionFor(token.Pos(off)+s.pos0, true)
}

// scanner tokenizes a scannerSource.
type scanner struct {
	s  *scannerSource
	eh errHandler

	chSize int
	state  int // cppAtLineStart, ...

	ch  rune // eof at EOF or when consumed.
	off uint32
	sep uint32 // .off on .cppScan invocation.
	src uint32 // .off on .cppScan finding the start of a token after skipping the separator, if any.

	closed      bool
	joinedLines bool // Translation phase 2 skipped "\\\n".
}

// newScanner returns a new scanner. The errHandler function is invoked on
// scanner errors.
func newScanner(fset *fset, src Source, eh errHandler) (*scanner, error) {
	s, err := newScannerSource(fset, src)
	if err != nil {
		return nil, err
	}

	r := &scanner{
		s:  s,
		ch: eof,
		eh: eh,
	}
	if r.peek(0) == '\xef' && r.peek(1) == '\xbb' && r.peek(2) == '\xbf' {
		r.off += 3
	}
	return r, nil
}

// close causes all subsequent calls to .scan to return an EOF token.
func (s *scanner) close() { s.closed = true }

// rune returns the current rune s is positioned on. If s is at EOF or closed,
// rune returns eof.
func (s *scanner) rune() rune {
	if s.closed {
		return eof
	}

	if s.ch < 0 {
		if s.off >= s.s.len {
			s.closed = true
			return eof
		}
	}

	if s.peek(0) == '\\' {
		switch {
		case s.peek(1) == '\n':
			s.joinedLines = true
			s.off += 2
			s.s.file.AddLine(int(s.off))
		case s.peek(1) == '\r' && s.peek(2) == '\n':
			s.joinedLines = true
			s.off += 3
			s.s.file.AddLine(int(s.off))
		}
	}

	s.ch, s.chSize = utf8.DecodeRune(s.s.buf[s.off:])
	return s.ch
}

func (s *scanner) peek(delta uint32) int {
	if s.off+delta < s.s.len {
		return int(s.s.buf[s.off+uint32(delta)])
	}

	return eof
}

// shift moves s to and returns the shift rune. If s is at EOF or closed, shift
// returns eof.
func (s *scanner) shift() rune {
	if s.closed || s.ch < 0 {
		return eof
	}

	s.off += uint32(s.chSize)
	prev := s.ch
	s.ch = eof
	r := s.rune()
	if prev == '\n' {
		s.s.file.AddLine(int(s.off))
	}
	return r
}

// pos returns a token.Position representing off.
func (s *scanner) pos(off uint32) token.Position { return s.s.pos(off) }

// cppScan0 returns the next preprocessing token, see [0]6.4. If s is at EOF or
// closed, cppScan0 returns eof.
func (s *scanner) cppScan0() (tok Token) {
	s.scanSep()
	s.src = s.off
	c := s.rune()
	switch c {
	case

		'(', ')', ',', ':', ';',
		'?', '[', '\n', ']', '{',
		'}', '~':

		s.shift()
		return s.newToken(c)
	case '*':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(MULASSIGN))
		default:
			return s.newToken(c)
		}
	case '=':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(EQ))
		default:
			return s.newToken(c)
		}
	case '^':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(XORASSIGN))
		default:
			return s.newToken(c)
		}
	case '+':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(ADDASSIGN))
		case '+':
			s.shift()
			return s.newToken(rune(INC))
		default:
			return s.newToken(c)
		}
	case '&':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(ANDASSIGN))
		case '&':
			s.shift()
			return s.newToken(rune(ANDAND))
		default:
			return s.newToken(c)
		}
	case '-':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(SUBASSIGN))
		case '-':
			s.shift()
			return s.newToken(rune(DEC))
		case '>':
			s.shift()
			return s.newToken(rune(ARROW))
		default:
			return s.newToken(c)
		}
	case '|':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(ORASSIGN))
		case '|':
			s.shift()
			return s.newToken(rune(OROR))
		default:
			return s.newToken(c)
		}
	case '%':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(MODASSIGN))
		default:
			return s.newToken(c)
		}
	case '/':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(DIVASSIGN))
		default:
			return s.newToken(c)
		}
	case '!':
		switch s.shift() {
		case '=':
			s.shift()
			return s.newToken(rune(NEQ))
		default:
			return s.newToken(c)
		}
	case '#':
		switch s.shift() {
		case '#':
			s.shift()
			return s.newToken(rune(PPPASTE))
		default:
			return s.newToken(c)
		}
	case '<':
		switch s.shift() {
		case '<':
			switch s.shift() {
			case '=':
				s.shift()
				return s.newToken(rune(LSHASSIGN))
			default:
				return s.newToken(rune(LSH))
			}
		case '=':
			s.shift()
			return s.newToken(rune(LEQ))
		default:
			return s.newToken(c)
		}
	case '>':
		switch s.shift() {
		case '>':
			switch s.shift() {
			case '=':
				s.shift()
				return s.newToken(rune(RSHASSIGN))
			default:
				return s.newToken(rune(RSH))
			}
		case '=':
			s.shift()
			return s.newToken(rune(GEQ))
		default:
			return s.newToken(c)
		}
	case '.':
		if s.peek(1) == '.' && s.peek(2) == '.' {
			s.shift()
			s.shift()
			s.shift()
			return s.newToken(rune(DDD))
		}

		s.shift()
		switch c2 := s.rune(); {
		case c2 >= '0' && c2 <= '9':
			return s.ppnumber()
		default:
			return s.newToken(c)
		}
	case '"':
		return s.stringLiteral(rune(STRINGLITERAL))
	case '\'':
		return s.characterConstant(rune(CHARCONST))
	case 'L':
		switch s.peek(1) {
		case '\'':
			s.shift()
			return s.characterConstant(rune(LONGCHARCONST))
		case '"':
			s.shift()
			return s.stringLiteral(rune(LONGSTRINGLITERAL))
		}
	case 'u':
		if s.peek(1) == '8' && s.peek(2) == '"' {
			s.shift()
			s.shift()
			return s.stringLiteral(rune(STRINGLITERAL))
		}
	case eof:
		s.closed = true
		return s.newToken(c)
	}

	switch {
	case c == '@':
		return s.objcAtKeyword()
	case unicode.IsLetter(c) || c == '_' || c == '$':
		return s.identifier()
	case c >= '0' && c <= '9':
		return s.ppnumber()
	default:
		s.shift()
		return s.newToken(c)
	}
}

// characterConstant scans a character-constant.
//
// [0]A.1.5
func (s *scanner) characterConstant(c rune) Token {
	s.shift() // '\''
	for s.cChar() {
	}
	switch s.rune() {
	case '\'':
		s.shift()
		return s.newToken(c)
	default:
		s.eh("%v: character constant not terminated", s.pos(s.src))
		return s.fail()
	}
}

// cChar scans c-char.
//
// [0]A.1.5
func (s *scanner) cChar() bool {
	switch s.rune() {
	case '\'', '\n':
		return false
	case '\\':
		s.shift()
		return s.escapeSequence()
	default:
		s.shift()
		return true
	}
}

// stringLiteral scans a string-literal.
//
// [0]A.1.6
func (s *scanner) stringLiteral(c rune) Token {
	s.shift() // '"'
	for s.sChar() {
	}
	switch s.rune() {
	case '"':
		s.shift()
		return s.newToken(c)
	default:
		s.eh("%v: string literal not terminated", s.pos(s.src))
		return s.fail()
	}
}

// sChar scans s-char.
//
// [0]A.1.6
func (s *scanner) sChar() bool {
	switch s.rune() {
	case '"', '\n':
		return false
	case '\\':
		s.shift()
		return s.escapeSequence()
	default:
		s.shift()
		return true
	}
}

// escapeSequence scans escape-sequence.
//
// [0]A.1.5
func (s *scanner) escapeSequence() bool {
	// '\\' already consumed
	return s.simpleEscapeSequence() ||
		s.octalEscapeSequence() ||
		s.hexadecimalEscapeSequence() ||
		s.universalCharacterName() ||
		s.shift() != eof
}

// universalCharacterName scans universal-character-name.
//
// [0]A.1.4
func (s *scanner) universalCharacterName() bool {
	// '\\' already consumed
	switch s.rune() {
	case 'u':
		s.shift()
		s.hexQuad()
		return true
	case 'U':
		s.shift()
		if s.hexQuad() {
			s.hexQuad()
		}
		return true
	}
	return false
}

// hexQuad scans hex-quad.
//
// [0]A.1.4
func (s *scanner) hexQuad() (r bool) {
	for i := 0; i < 4; i++ {
		if !s.hexadecimalDigit() {
			s.eh("%v: expected hexadecimal digit", s.pos(s.off))
			return r
		}

		r = true
	}
	return r
}

// hexadecimalEscapeSequence scans hexadecimal-escape-sequence.
//
// [0]A.1.5
func (s *scanner) hexadecimalEscapeSequence() bool {
	// '\\' already consumed
	switch s.rune() {
	case 'x', 'X':
		s.shift()
		ok := false
		for s.hexadecimalDigit() {
			ok = true
		}
		if !ok {
			s.eh("%v: expected hexadecimal digit", s.pos(s.off))
		}
		return true
	}
	return false
}

// hexadecimalDigit scans hexadecimal-digit.
//
// [0]A.1.5
func (s *scanner) hexadecimalDigit() bool {
	switch c := s.rune(); {
	case
		c >= '0' && c <= '9',
		c >= 'a' && c <= 'f',
		c >= 'A' && c <= 'F':

		s.shift()
		return true
	}
	return false
}

// octalEscapeSequence scans octal-escape-sequence.
//
// [0]A.1.5
func (s *scanner) octalEscapeSequence() bool {
	// '\\' already consumed
	switch c := s.rune(); {
	case c >= '0' && c <= '7':
		for s.octalDigit() {
		}
		return true
	}
	return false
}

// octalDigit scans octal-digit.
//
// [0]A.1.5
func (s *scanner) octalDigit() bool {
	switch c := s.rune(); {
	case c >= '0' && c <= '7':
		s.shift()
		return true
	}
	return false
}

// simpleEscapeSequence scans simple-escape-sequence.
//
// [0]A.1.5
func (s *scanner) simpleEscapeSequence() bool {
	// '\\' already consumed
	switch s.rune() {
	case '\'', '"', '?', '\\', 'a', 'b', 'e', 'f', 'n', 'r', 't', 'v':
		s.shift()
		return true
	}
	return false
}

// newToken returns a newly created Token, conveniently filling in the usual defaults.
func (s *scanner) newToken(c rune) Token {
	switch {
	case s.joinedLines:
		s.joinedLines = false
		tok := newToken(s.s, c, s.sep, s.src, s.off-s.src)
		sep := bytes.ReplaceAll(tok.Sep(), []byte("\\\n"), nil)
		src := bytes.ReplaceAll(tok.Src(), []byte("\\\n"), nil)
		tok.Set(
			bytes.ReplaceAll(sep, []byte("\\\r\n"), nil),
			bytes.ReplaceAll(src, []byte("\\\r\n"), nil),
		)
		return tok
	default:
		return newToken(s.s, c, s.sep, s.src, s.off-s.src)

	}
}

// ppnumber scans preprocessor numbers.
//
// [0]A.1.9
func (s *scanner) ppnumber() Token {
	s.shift()
	for {
		switch c := s.rune(); c {
		case '.':
			s.shift()
		case 'e', 'E', 'p', 'P':
			s.shift()
			s.sign(false)
		default:
			switch {
			case c >= '0' && c <= '9':
				s.shift()
			case unicode.IsLetter(c) || c == '_':
				s.shift()
			default:
				return s.newToken(rune(PPNUMBER))
			}
		}
	}
}

// sign scans sign.
//
// [0]A.1.5
func (s *scanner) sign(must bool) {
	switch s.rune() {
	case '+', '-':
		s.shift()
	default:
		if must {
			s.shift()
			s.eh("%v: expected sign", s.pos(s.off))
		}
	}
}

// scanSep will set s.sep to s.off and scan until end the of a separator, if
// any.
func (s *scanner) scanSep() {
	s.sep = s.off
	for {
		switch s.rune() {
		case ' ', '\t', '\f', '\v', '\r':
			s.shift()
		case '/':
			off := s.off
			switch s.shift() {
			case '*':
				s.comment(off)
			case '/':
				s.lineComment(off)
				return
			default:
				s.off = off
				s.ch = eof
				return
			}
		default:
			return
		}
	}
}

// lineComment scans until the end of a //-style comment. The leading '/' is
// already consumed.
func (s *scanner) lineComment(start uint32) {
	s.shift() // '/'
	for {
		switch s.rune() {
		case eof, '\n':
			return
		}
		s.shift()
	}
}

// identifier scans an identifier
func (s *scanner) identifier() Token {
	s.shift()
	for {
		switch c := s.rune(); {
		case unicode.IsLetter(c) || c == '_' || unicode.IsDigit(c) || c == '$':
			s.shift()
		default:
			return s.newToken(rune(IDENTIFIER))
		}
	}
}

// fail reports an error at current position, closes s and returns an EOF
// Token.
func (s *scanner) fail() Token {
	s.eh("%v: unexpected rune: %s (%s)", s.pos(s.off), runeName(s.rune()), origin(2))
	s.closed = true
	return newToken(s.s, eof, s.off, s.off, 0)
}

// comment scans until the end of a /*-style comment. The leading '/' is
// already consumed.
func (s *scanner) comment(start uint32) {
	s.shift() // '*'
	for {
		switch s.rune() {
		case eof:
			s.eh("%v: comment not terminated", s.pos(start))
			return
		case '*':
			switch s.shift() {
			case '/':
				s.shift()
				return
			}
		default:
			s.shift()
		}
	}
}

const (
	cppScanAtLineStart = iota
	cppScanHash        // Returned a first token off a line and it was '#'.
	cppScanOther
	cppScanHeaderName
)

// cppScan returns the next preprocessing token, see [0]6.4. If s is at EOF or
// closed, cppScan returns eof.
func (s *scanner) cppScan() (tok Token) {
	switch s.state {
	case cppScanAtLineStart:
		switch tok = s.cppScan0(); tok.Ch {
		case '\n':
			s.state = cppScanAtLineStart
		case '#':
			s.state = cppScanHash
		default:
			s.state = cppScanOther
		}
		return tok
	case cppScanHash:
		switch tok = s.cppScan0(); {
		case tok.Ch == rune(IDENTIFIER) && (bytes.Equal(tok.Src(), []byte("include")) || bytes.Equal(tok.Src(), []byte("include_next"))):
			s.state = cppScanHeaderName
		default:
			s.state = cppScanOther
		}
		return tok
	case cppScanHeaderName:
		s.state = cppScanOther
		return s.headerName()
	case cppScanOther:
		switch tok = s.cppScan0(); tok.Ch {
		case '\n':
			s.state = cppScanAtLineStart
		}
		return tok
	default:
		s.eh("%v: internal error, scanner state: %v", s.pos(s.off), s.state)
		s.state = cppScanOther
		return s.cppScan0()
	}
}

// headerName scans preprocessor header-name.
//
// [0]A.1.8
func (s *scanner) headerName() Token {
	s.scanSep()
	s.src = s.off
	switch s.rune() {
	case '<':
		for {
			switch s.shift() {
			case '>':
				s.shift()
				s.state = cppScanOther
				return s.newToken(rune(HEADER_NAME))
			case '\n':
				s.state = cppScanAtLineStart
				return s.fail()
			}
		}
	case '"':
		for {
			switch s.shift() {
			case '"':
				s.shift()
				s.state = cppScanOther
				return s.newToken(rune(HEADER_NAME))
			case '\n':
				s.state = cppScanAtLineStart
				return s.fail()
			}
		}
	default:
		return s.cppScan0()
	}
}

// ── ObjC @ keyword scanner (MinZ fork) ──────────────────────────────────────

var objcAtKeywords = map[string]tokCh{
	"interface":      OBJC_INTERFACE,
	"implementation": OBJC_IMPLEMENTATION,
	"protocol":       OBJC_PROTOCOL,
	"end":            OBJC_END,
	"property":       OBJC_PROPERTY,
	"synthesize":     OBJC_SYNTHESIZE,
	"dynamic":        OBJC_DYNAMIC,
	"selector":       OBJC_SELECTOR,
	"encode":         OBJC_ENCODE,
	"class":          OBJC_CLASS,
	"optional":       OBJC_OPTIONAL,
	"required":       OBJC_REQUIRED,
}

// objcAtKeyword scans an ObjC @keyword token.
// Called when the scanner sees '@'. If followed by a known keyword,
// returns the appropriate OBJC_* token. Otherwise returns '@' as-is.
func (s *scanner) objcAtKeyword() Token {
	s.shift() // consume '@'
	c := s.rune()
	if !unicode.IsLetter(c) {
		return s.newToken('@')
	}

	// Scan the identifier after '@'.
	start := s.off
	for unicode.IsLetter(s.rune()) || unicode.IsDigit(s.rune()) || s.rune() == '_' {
		s.shift()
	}
	word := string(s.s.buf[start:s.off])

	if ch, ok := objcAtKeywords[word]; ok {
		return s.newToken(rune(ch))
	}

	// Unknown @word — return '@' followed by the identifier.
	// For now, just return '@' and let the parser handle it.
	return s.newToken('@')
}
