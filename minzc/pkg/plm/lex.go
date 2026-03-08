package plm

import (
	"fmt"
	"strings"
)

// ── Token types ───────────────────────────────────────────────────────────────

type TokKind int

const (
	// Literals
	TokIdent  TokKind = iota // identifier or keyword
	TokNumber                // decimal: 42  or hex: 0FFH
	TokString                // 'text' inside LITERALLY

	// Punctuation
	TokColon     // :
	TokSemicolon // ;
	TokComma     // ,
	TokLParen    // (
	TokRParen    // )
	TokDot       // .

	// Operators
	TokEq    // =
	TokNotEq // <>
	TokLt    // <
	TokGt    // >
	TokLtEq  // <=
	TokGtEq  // >=
	TokPlus  // +
	TokMinus // -
	TokStar  // *
	TokSlash // /

	TokEOF
)

type Token struct {
	Kind TokKind
	Val  string // upper-cased for identifiers/keywords
	Line int
}

// ── Lexer ─────────────────────────────────────────────────────────────────────

type Lexer struct {
	src    []byte
	pos    int
	line   int
	tokens []Token
	cur    int
}

func NewLexer(src string) (*Lexer, error) {
	l := &Lexer{src: []byte(strings.ToUpper(src)), line: 1}
	if err := l.tokenize(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Lexer) tokenize() error {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]

		// Whitespace
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.pos++
			continue
		}
		if ch == '\n' {
			l.line++
			l.pos++
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

		// Multi-char operators
		if ch == '<' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '>' {
			l.emit(TokNotEq, "<>", line)
			l.pos += 2
			continue
		}
		if ch == '<' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=' {
			l.emit(TokLtEq, "<=", line)
			l.pos += 2
			continue
		}
		if ch == '>' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=' {
			l.emit(TokGtEq, ">=", line)
			l.pos += 2
			continue
		}

		// Single-char operators / punctuation
		switch ch {
		case ':':
			l.emit(TokColon, ":", line)
			l.pos++
			continue
		case ';':
			l.emit(TokSemicolon, ";", line)
			l.pos++
			continue
		case ',':
			l.emit(TokComma, ",", line)
			l.pos++
			continue
		case '(':
			l.emit(TokLParen, "(", line)
			l.pos++
			continue
		case ')':
			l.emit(TokRParen, ")", line)
			l.pos++
			continue
		case '.':
			l.emit(TokDot, ".", line)
			l.pos++
			continue
		case '=':
			l.emit(TokEq, "=", line)
			l.pos++
			continue
		case '<':
			l.emit(TokLt, "<", line)
			l.pos++
			continue
		case '>':
			l.emit(TokGt, ">", line)
			l.pos++
			continue
		case '+':
			l.emit(TokPlus, "+", line)
			l.pos++
			continue
		case '-':
			l.emit(TokMinus, "-", line)
			l.pos++
			continue
		case '*':
			l.emit(TokStar, "*", line)
			l.pos++
			continue
		case '/':
			l.emit(TokSlash, "/", line)
			l.pos++
			continue
		}

		// String literal (LITERALLY 'value')
		// PL/M-80: '' (two single quotes) inside a string = one literal quote.
		if ch == '\'' {
			l.pos++
			var val []byte
			for l.pos < len(l.src) {
				if l.src[l.pos] == '\'' {
					if l.pos+1 < len(l.src) && l.src[l.pos+1] == '\'' {
						val = append(val, '\'')
						l.pos += 2
						continue
					}
					break // closing quote
				}
				val = append(val, l.src[l.pos])
				l.pos++
			}
			l.emit(TokString, string(val), line)
			if l.pos < len(l.src) {
				l.pos++ // closing '
			}
			continue
		}

		// Number: decimal or hex (0FFH, 1AH, 255)
		if ch >= '0' && ch <= '9' {
			start := l.pos
			for l.pos < len(l.src) && isHexDigit(l.src[l.pos]) {
				l.pos++
			}
			// Hex suffix H?
			if l.pos < len(l.src) && l.src[l.pos] == 'H' {
				l.pos++
			}
			l.emit(TokNumber, string(l.src[start:l.pos]), line)
			continue
		}

		// Identifier or keyword
		if isIdentStart(ch) {
			start := l.pos
			for l.pos < len(l.src) && isIdentCont(l.src[l.pos]) {
				l.pos++
			}
			// PL/M-80: '$' is a separator in identifiers (ignored), so strip it.
			val := strings.ReplaceAll(string(l.src[start:l.pos]), "$", "")
			l.emit(TokIdent, val, line)
			continue
		}

		return fmt.Errorf("line %d: unexpected character %q", l.line, ch)
	}

	l.emit(TokEOF, "", l.line)
	return nil
}

func (l *Lexer) emit(kind TokKind, val string, line int) {
	l.tokens = append(l.tokens, Token{Kind: kind, Val: val, Line: line})
}

// ── Token stream helpers ──────────────────────────────────────────────────────

func (l *Lexer) Peek() Token {
	if l.cur < len(l.tokens) {
		return l.tokens[l.cur]
	}
	return Token{Kind: TokEOF}
}

func (l *Lexer) Next() Token {
	t := l.Peek()
	if t.Kind != TokEOF {
		l.cur++
	}
	return t
}

// Is returns true if the next token is an identifier with value kw.
func (l *Lexer) Is(kw string) bool {
	t := l.Peek()
	return t.Kind == TokIdent && t.Val == kw
}

// IsKind returns true if the next token has the given kind.
func (l *Lexer) IsKind(k TokKind) bool { return l.Peek().Kind == k }

// Expect consumes a keyword identifier or returns error.
func (l *Lexer) Expect(kw string) error {
	t := l.Next()
	if t.Kind != TokIdent || t.Val != kw {
		return fmt.Errorf("line %d: expected %q, got %q", t.Line, kw, t.Val)
	}
	return nil
}

// ExpectKind consumes a token of the given kind or returns error.
func (l *Lexer) ExpectKind(k TokKind) (Token, error) {
	t := l.Next()
	if t.Kind != k {
		return t, fmt.Errorf("line %d: unexpected token %q", t.Line, t.Val)
	}
	return t, nil
}

// ── Character helpers ─────────────────────────────────────────────────────────

func isIdentStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || c == '$'
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}
