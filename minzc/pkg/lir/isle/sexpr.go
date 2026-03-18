// sexpr.go — Self-contained S-expression parser for ISLE rules.
// Copied from lanz/parse.go to break the import cycle lanz→pipeline→lir→isle→lanz.
package isle

import (
	"fmt"
	"strings"
)

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
	p.pos++
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
	p.pos++
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
