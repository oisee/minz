package grace

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/rewrite/sexpr"
)

// ParseRules parses one or more Grace rules from S-expression source.
func ParseRules(src string) ([]GraceRule, error) {
	nodes, err := sexpr.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("grace parse: %w", err)
	}

	var rules []GraceRule
	for _, n := range nodes {
		if !n.IsList() || len(n.List) == 0 {
			continue
		}
		if n.List[0].Atom != "grace" {
			continue
		}
		r, err := parseGraceRule(n)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func parseGraceRule(n sexpr.Node) (GraceRule, error) {
	// (grace name priority (match ...) [(where ...)] (action ...))
	r := GraceRule{}
	if len(n.List) < 4 {
		return r, fmt.Errorf("line %d: grace rule needs at least name + match + action", n.Line)
	}

	idx := 1
	// Name
	r.Name = n.List[idx].Atom
	idx++

	// Optional priority
	if n.List[idx].IsAtom() && isInt(n.List[idx].Atom) {
		r.Priority, _ = strconv.Atoi(n.List[idx].Atom)
		idx++
	}

	// Parse remaining sections
	for idx < len(n.List) {
		section := n.List[idx]
		if !section.IsList() || len(section.List) == 0 {
			idx++
			continue
		}
		head := section.List[0].Atom
		switch head {
		case "match":
			clauses, err := parseMatchClauses(section)
			if err != nil {
				return r, err
			}
			r.Match = clauses
		case "where":
			preds, err := parsePredicates(section)
			if err != nil {
				return r, err
			}
			r.Where = preds
		case "action":
			actions, err := parseActions(section)
			if err != nil {
				return r, err
			}
			r.Actions = actions
		}
		idx++
	}

	return r, nil
}

func parseMatchClauses(n sexpr.Node) ([]MatchClause, error) {
	var clauses []MatchClause
	for _, child := range n.List[1:] {
		if !child.IsList() || len(child.List) == 0 {
			continue
		}
		head := child.List[0].Atom
		switch head {
		case "block":
			c, err := parseBlockMatch(child)
			if err != nil {
				return nil, err
			}
			clauses = append(clauses, c)
		case "edge":
			c, err := parseEdgeMatch(child)
			if err != nil {
				return nil, err
			}
			clauses = append(clauses, c)
		case "node":
			c, err := parseNodeMatch(child)
			if err != nil {
				return nil, err
			}
			clauses = append(clauses, c)
		}
	}
	return clauses, nil
}

// parseBlockMatch: (block ?var [(term-kind "kind")])
func parseBlockMatch(n sexpr.Node) (MatchClause, error) {
	c := MatchClause{Kind: MatchBlock}
	if len(n.List) < 2 {
		return c, fmt.Errorf("line %d: block match needs variable", n.Line)
	}
	c.Var = stripQ(n.List[1].Atom)

	// Optional sub-clauses
	for _, sub := range n.List[2:] {
		if sub.IsList() && len(sub.List) >= 2 && sub.List[0].Atom == "term-kind" {
			c.TermKind = stripQuotes(sub.List[1].Atom)
		}
	}
	return c, nil
}

// parseEdgeMatch: (edge ?from ?to "kind")
func parseEdgeMatch(n sexpr.Node) (MatchClause, error) {
	c := MatchClause{Kind: MatchEdge}
	if len(n.List) < 4 {
		return c, fmt.Errorf("line %d: edge match needs from, to, kind", n.Line)
	}
	c.Var = stripQ(n.List[1].Atom)
	c.Var2 = stripQ(n.List[2].Atom)
	c.EdgeKind = stripQuotes(n.List[3].Atom)
	return c, nil
}

// parseNodeMatch: (node ?var ?block "op")
func parseNodeMatch(n sexpr.Node) (MatchClause, error) {
	c := MatchClause{Kind: MatchNode}
	if len(n.List) < 3 {
		return c, fmt.Errorf("line %d: node match needs variable and block", n.Line)
	}
	c.Var = stripQ(n.List[1].Atom)
	c.Var2 = stripQ(n.List[2].Atom) // parent block var
	if len(n.List) >= 4 {
		c.NodeOp = stripQuotes(n.List[3].Atom)
	}
	return c, nil
}

func parsePredicates(n sexpr.Node) ([]Predicate, error) {
	var preds []Predicate
	for _, child := range n.List[1:] {
		if !child.IsList() || len(child.List) == 0 {
			continue
		}
		p, err := parsePredicate(child)
		if err != nil {
			return nil, err
		}
		preds = append(preds, p)
	}
	return preds, nil
}

func parsePredicate(n sexpr.Node) (Predicate, error) {
	head := n.List[0].Atom
	p := Predicate{}

	switch head {
	case "pred-count":
		// (pred-count ?var op value)
		if len(n.List) < 4 {
			return p, fmt.Errorf("line %d: pred-count needs var, op, value", n.Line)
		}
		p.Kind = PredPredCount
		p.Var = stripQ(n.List[1].Atom)
		p.Op = n.List[2].Atom
		p.Value, _ = strconv.Atoi(n.List[3].Atom)

	case "no-params":
		// (no-params ?var)
		if len(n.List) < 2 {
			return p, fmt.Errorf("line %d: no-params needs var", n.Line)
		}
		p.Kind = PredNoParms
		p.Var = stripQ(n.List[1].Atom)

	case "all-pure":
		// (all-pure ?var)
		if len(n.List) < 2 {
			return p, fmt.Errorf("line %d: all-pure needs var", n.Line)
		}
		p.Kind = PredAllPure
		p.Var = stripQ(n.List[1].Atom)

	case "inst-count", "param-count":
		// (inst-count ?var op value)
		if len(n.List) < 4 {
			return p, fmt.Errorf("line %d: %s needs var, op, value", n.Line, head)
		}
		p.Kind = PredFieldCmp
		p.Var = stripQ(n.List[1].Atom)
		if head == "inst-count" {
			p.Field = "inst_count"
		} else {
			p.Field = "param_count"
		}
		p.Op = n.List[2].Atom
		p.Value, _ = strconv.Atoi(n.List[3].Atom)

	case "use-count":
		// (use-count ?var op value)
		if len(n.List) < 4 {
			return p, fmt.Errorf("line %d: use-count needs var, op, value", n.Line)
		}
		p.Kind = PredUseCount
		p.Var = stripQ(n.List[1].Atom)
		p.Op = n.List[2].Atom
		p.Value, _ = strconv.Atoi(n.List[3].Atom)

	case "dominates":
		// (dominates ?var1 ?var2)
		if len(n.List) < 3 {
			return p, fmt.Errorf("line %d: dominates needs two vars", n.Line)
		}
		p.Kind = PredDominates
		p.Var = stripQ(n.List[1].Atom)
		p.Var2 = stripQ(n.List[2].Atom)

	case "term-kind":
		// (term-kind ?var "kind")
		if len(n.List) < 3 {
			return p, fmt.Errorf("line %d: term-kind needs var and kind", n.Line)
		}
		p.Kind = PredTermKind
		p.Var = stripQ(n.List[1].Atom)
		p.Field = stripQuotes(n.List[2].Atom)

	default:
		// Custom predicate
		p.Kind = PredCustom
		p.CustomName = head
		for _, arg := range n.List[1:] {
			p.CustomArgs = append(p.CustomArgs, stripQ(arg.Atom))
		}
	}

	return p, nil
}

func parseActions(n sexpr.Node) ([]Action, error) {
	var actions []Action
	for _, child := range n.List[1:] {
		if !child.IsList() || len(child.List) == 0 {
			continue
		}
		a, err := parseAction(child)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, nil
}

func parseAction(n sexpr.Node) (Action, error) {
	head := n.List[0].Atom
	a := Action{}

	switch head {
	case "delete":
		a.Kind = ActDelete
		if len(n.List) >= 2 {
			a.Var = stripQ(n.List[1].Atom)
		}

	case "delete-block":
		a.Kind = ActDeleteBlock
		if len(n.List) >= 2 {
			a.Var = stripQ(n.List[1].Atom)
		}

	case "hoist-insts":
		// (hoist-insts ?src ?dst)
		a.Kind = ActHoistInsts
		if len(n.List) >= 3 {
			a.Var = stripQ(n.List[1].Atom)
			a.Var2 = stripQ(n.List[2].Atom)
		}

	case "replace-term":
		// (replace-term ?var "new-kind" target1 target2...)
		a.Kind = ActReplaceTerm
		if len(n.List) >= 3 {
			a.Var = stripQ(n.List[1].Atom)
			a.NewTermKind = stripQuotes(n.List[2].Atom)
			for _, t := range n.List[3:] {
				a.NewTargets = append(a.NewTargets, stripQ(t.Atom))
			}
		}

	case "remove-param":
		a.Kind = ActRemoveParam
		if len(n.List) >= 2 {
			a.Var = stripQ(n.List[1].Atom)
		}

	case "custom":
		a.Kind = ActCustom
		if len(n.List) >= 2 {
			a.CustomName = stripQuotes(n.List[1].Atom)
		}
		if len(n.List) >= 3 {
			a.Var = stripQ(n.List[2].Atom)
		}
		if len(n.List) >= 4 {
			a.Var2 = stripQ(n.List[3].Atom)
		}

	default:
		return a, fmt.Errorf("line %d: unknown action %q", n.Line, head)
	}

	return a, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func stripQ(s string) string {
	return strings.TrimPrefix(s, "?")
}

func stripQuotes(s string) string {
	return strings.Trim(s, `"`)
}

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
