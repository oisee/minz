// Package lanz implements a Lisp-like S-expression frontend for HIR.
//
// Lanz is a compile-time meta-language where S-expressions map 1:1 to HIR
// nodes.  It serves as a readable, parseable, round-trippable representation
// of the typed AST.
//
// Syntax:
//
//	atom    = INT | IDENT | STRING | TYPE
//	sexpr   = '(' atom sexpr* ')'
//	module  = sexpr*
//
// HIR mapping:
//
//	(fun name ((p1 ty1) ...) retty body...)       → hir.Func
//	(global name ty)                               → mir2.Global
//	(global name ty init)                          → mir2.Global with init
//	(struct name ((f1 ty1) (f2 ty2) ...))          → mir2.StructTy
//	(var name ty)                                  → hir.VarDeclStmt
//	(var name ty init)                             → hir.VarDeclStmt with init
//	(set target val)                               → hir.AssignStmt
//	(return expr)                                  → hir.ReturnStmt
//	(if cond then)                                 → hir.IfStmt
//	(if cond then else)                            → hir.IfStmt with else
//	(while cond body...)                           → hir.WhileStmt
//	(for var lo hi body...)                        → hir.ForRangeStmt
//	(call fn args...)                              → hir.CallExpr
//	(+ a b), (- a b), (* a b), (/ a b), ...       → hir.BinExpr
//	(neg x), (not x), (bitnot x)                  → hir.UnaryExpr
//	(load ptr)                                     → hir.LoadExpr
//	(store ptr val)                                → hir.StoreStmt
//	(addr sym)                                     → hir.AddrOfExpr
//	(cast expr ty)                                 → hir.CastExpr
//	(index base idx)                               → hir.IndexExpr
//	(field base name offset)                       → hir.FieldExpr
//	(block stmts...)                               → hir.Block
//	(break)                                        → hir.BreakStmt
//	(continue)                                     → hir.ContinueStmt
//	42, 0xFF                                       → hir.IntLitExpr
//	true, false                                    → hir.BoolLitExpr
//	name                                           → hir.VarRefExpr
package lanz

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ── S-expression types ───────────────────────────────────────────────────────

// Node is an S-expression node: either an Atom (string) or a List of nodes.
type Node struct {
	Atom string // non-empty for atoms
	List []Node // non-empty for lists
	Line int    // source line (1-based)
}

func (n Node) IsAtom() bool { return len(n.List) == 0 }
func (n Node) IsList() bool { return len(n.List) > 0 }

func (n Node) String() string {
	if n.IsAtom() {
		return n.Atom
	}
	parts := make([]string, len(n.List))
	for i, c := range n.List {
		parts[i] = c.String()
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// ── Tokenizer ────────────────────────────────────────────────────────────────

// ParseSExpr parses a string of S-expressions into a list of top-level nodes.
func ParseSExpr(src string) ([]Node, error) {
	p := &sexprParser{src: src, pos: 0, line: 1}
	var nodes []Node
	for p.skipWS(); p.pos < len(src); p.skipWS() {
		n, err := p.parseNode()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

type sexprParser struct {
	src  string
	pos  int
	line int
}

func (p *sexprParser) skipWS() {
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		if ch == '\n' {
			p.line++
			p.pos++
		} else if ch == ';' {
			// Line comment
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		} else if ch <= ' ' {
			p.pos++
		} else {
			break
		}
	}
}

func (p *sexprParser) parseNode() (Node, error) {
	p.skipWS()
	if p.pos >= len(p.src) {
		return Node{}, fmt.Errorf("line %d: unexpected EOF", p.line)
	}
	if p.src[p.pos] == '(' {
		return p.parseList()
	}
	if p.src[p.pos] == '"' {
		return p.parseString()
	}
	return p.parseAtom()
}

func (p *sexprParser) parseList() (Node, error) {
	line := p.line
	p.pos++ // consume '('
	var children []Node
	for {
		p.skipWS()
		if p.pos >= len(p.src) {
			return Node{}, fmt.Errorf("line %d: unterminated list", line)
		}
		if p.src[p.pos] == ')' {
			p.pos++
			return Node{List: children, Line: line}, nil
		}
		child, err := p.parseNode()
		if err != nil {
			return Node{}, err
		}
		children = append(children, child)
	}
}

func (p *sexprParser) parseAtom() (Node, error) {
	line := p.line
	start := p.pos
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		if ch <= ' ' || ch == '(' || ch == ')' || ch == ';' {
			break
		}
		p.pos++
	}
	if p.pos == start {
		return Node{}, fmt.Errorf("line %d: unexpected character %q", line, p.src[p.pos])
	}
	return Node{Atom: p.src[start:p.pos], Line: line}, nil
}

func (p *sexprParser) parseString() (Node, error) {
	line := p.line
	p.pos++ // consume opening "
	var sb strings.Builder
	for p.pos < len(p.src) {
		ch := p.src[p.pos]
		if ch == '"' {
			p.pos++
			return Node{Atom: `"` + sb.String() + `"`, Line: line}, nil
		}
		if ch == '\\' && p.pos+1 < len(p.src) {
			p.pos++
			switch p.src[p.pos] {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			default:
				sb.WriteByte(p.src[p.pos])
			}
			p.pos++
			continue
		}
		if ch == '\n' {
			p.line++
		}
		sb.WriteByte(ch)
		p.pos++
	}
	return Node{}, fmt.Errorf("line %d: unterminated string", line)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// IsInt returns true if the atom looks like an integer literal.
func IsInt(s string) bool {
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '-' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	// Hex
	if len(s) > start+2 && s[start:start+2] == "0x" {
		for _, ch := range s[start+2:] {
			if !unicode.In(ch, unicode.Hex_Digit) {
				return false
			}
		}
		return len(s) > start+2
	}
	for _, ch := range s[start:] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// ParseInt parses an integer atom (decimal or 0x hex).
func ParseInt(s string) (int64, error) {
	return strconv.ParseInt(s, 0, 64)
}
