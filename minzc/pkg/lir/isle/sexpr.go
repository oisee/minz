// sexpr.go — S-expression parser for ISLE rules.
// Delegates to the shared parser in pkg/rewrite/sexpr.
package isle

import (
	"github.com/minz/minzc/pkg/rewrite/sexpr"
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
	// Convert back to shared node for String()
	return toShared(n).String()
}

// ParseSExpr parses a string of S-expressions into a list of top-level nodes.
func ParseSExpr(src string) ([]Node, error) {
	shared, err := sexpr.Parse(src)
	if err != nil {
		return nil, err
	}
	return fromSharedSlice(shared), nil
}

// ── Conversion helpers ──────────────────────────────────────────────────────

func fromShared(n sexpr.Node) Node {
	if n.IsAtom() {
		return Node{Atom: n.Atom, Line: n.Line}
	}
	return Node{List: fromSharedSlice(n.List), Line: n.Line}
}

func fromSharedSlice(nodes []sexpr.Node) []Node {
	out := make([]Node, len(nodes))
	for i, n := range nodes {
		out[i] = fromShared(n)
	}
	return out
}

func toShared(n Node) sexpr.Node {
	if n.IsAtom() {
		return sexpr.Node{Atom: n.Atom, Line: n.Line}
	}
	list := make([]sexpr.Node, len(n.List))
	for i, c := range n.List {
		list[i] = toShared(c)
	}
	return sexpr.Node{List: list, Line: n.Line}
}
