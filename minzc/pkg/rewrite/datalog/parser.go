package datalog

import (
	"fmt"

	"github.com/minz/minzc/pkg/rewrite/sexpr"
)

// ParseFacts parses Datalog facts from S-expression source.
//
// Syntax:
//
//	(fact predicate arg1 arg2 ...)
//	(fact reg "A" "8" "acc")
func ParseFacts(src string) (*FactDB, error) {
	nodes, err := sexpr.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("datalog parse: %w", err)
	}

	db := NewFactDB()
	for _, n := range nodes {
		if !n.IsList() || len(n.List) < 2 {
			continue
		}
		if n.List[0].Atom != "fact" {
			continue
		}
		pred := stripQuotes(n.List[1].Atom)
		var args []string
		for _, a := range n.List[2:] {
			args = append(args, stripQuotes(a.Atom))
		}
		db.Add(pred, args...)
	}
	return db, nil
}

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
