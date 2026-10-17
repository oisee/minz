package abap

import (
	"fmt"
	"github.com/minz/minzc/pkg/abap/internal/abapparse"
	"path/filepath"
	"strings"
)

// Parse uses the embedded, pinned Go translation of abaplint. Syntax or
// admission failures are returned directly; no Node or Wasm fallback runs.
func Parse(src, name string) (*Program, error) {
	filename := filepath.Base(name)
	if !strings.Contains(filename, ".prog.") && !strings.Contains(filename, ".clas.") && !strings.Contains(filename, ".intf.") {
		base := strings.TrimSuffix(filename, ".abap")
		kind := "prog"
		upper := strings.ToUpper(strings.TrimSpace(src))
		if strings.HasPrefix(upper, "CLASS ") {
			kind = "clas"
		} else if strings.HasPrefix(upper, "INTERFACE ") {
			kind = "intf"
		}
		filename = base + "." + kind + ".abap"
	}
	r, err := abapparse.Parse(src, filename)
	if err != nil {
		return nil, err
	}
	if len(r.Diagnostics) > 0 {
		d := r.Diagnostics[0]
		return nil, fmt.Errorf("%s:%d:%d: %s: %s", name, d.Row, d.Col, d.Key, d.Message)
	}
	var child func(*abapparse.Node) *ASTChild
	child = func(n *abapparse.Node) *ASTChild {
		if n == nil {
			return nil
		}
		c := &ASTChild{Type: n.Type, Str: n.Str, Row: n.Row, Col: n.Col}
		for _, t := range n.Tokens {
			c.Tokens = append(c.Tokens, &Token{Str: t.Str, Row: t.Row, Col: t.Col})
		}
		for _, x := range n.Children {
			c.Children = append(c.Children, child(x))
		}
		return c
	}
	result := &ParseResult{}
	if n := child(r.Structure); n != nil {
		result.Structure = &StructNode{Type: n.Type, Children: n.Children}
	}
	for _, n := range r.Statements {
		c := child(n)
		result.Statements = append(result.Statements, &StmtNode{Type: c.Type, Tokens: c.Tokens, Children: c.Children})
	}
	return buildProgram(name, result)
}
