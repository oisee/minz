// Package isle implements a term-rewriting DSL for instruction selection and
// algebraic simplification, inspired by Cranelift's ISLE.
//
// ISLE rules describe how IR operations rewrite to target instructions or
// simplified forms. Rules are compiled and matched by priority.
//
// Syntax (S-expression based):
//
//	(type Reg)
//	(decl lower (MIROp) Inst)
//	(rule [priority] (lhs-pattern) [(if guard...)] (rhs-expression))
//	(extern name)
package isle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/rewrite/sexpr"
)

// ── AST ─────────────────────────────────────────────────────────────────────

// RuleSet is a parsed ISLE program (set of rules + declarations).
type RuleSet struct {
	Types   []TypeDecl
	Decls   []TermDecl
	Rules   []Rule
	Externs []string // extern constructor names
}

// TypeDecl declares a type name.
type TypeDecl struct {
	Name string
}

// TermDecl declares a term (function/constructor) with typed signature.
type TermDecl struct {
	Name    string
	Args    []string // arg type names
	RetType string   // return type name
}

// Rule is one rewrite rule: LHS pattern → RHS expression, with optional guard.
type Rule struct {
	Priority int    // higher = tried first (0 = default)
	LHS      Term   // pattern to match (with variables ?x)
	Guard    *Guard // optional (if ...) guard; nil = unconditional
	RHS      Term   // expression to produce
}

// Guard is an (if ...) clause constraining when a rule can fire.
type Guard struct {
	Op   string // comparison operator: "<", ">", "<=", ">=", "==", "!="
	Args []Term // operands (typically 2: lhs, rhs)
}

// Term is a tree node in a pattern or expression.
type Term struct {
	Kind   TermKind
	Name   string // atom name, variable name, or constructor name
	Args   []Term // child terms (for constructor/call)
	IntVal int64  // for integer literal
}

// TermKind classifies a Term node.
type TermKind int

const (
	TermVar  TermKind = iota // ?x — pattern variable
	TermAtom                 // symbol/keyword
	TermInt                  // integer literal
	TermCall                 // (name args...) — constructor/function call
	TermWild                 // _ — wildcard (matches anything)
)

// ── Parser ──────────────────────────────────────────────────────────────────

// Parse parses ISLE rules from source text.
func Parse(src string) (*RuleSet, error) {
	nodes, err := sexpr.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("isle parse: %w", err)
	}

	rs := &RuleSet{}
	for _, n := range nodes {
		if !n.IsList() || len(n.List) == 0 {
			continue
		}
		head := n.List[0].Atom
		switch head {
		case "type":
			if len(n.List) >= 2 {
				rs.Types = append(rs.Types, TypeDecl{Name: n.List[1].Atom})
			}
		case "decl":
			d, err := parseDecl(n)
			if err != nil {
				return nil, err
			}
			rs.Decls = append(rs.Decls, d)
		case "rule":
			r, err := parseRule(n)
			if err != nil {
				return nil, err
			}
			rs.Rules = append(rs.Rules, r)
		case "extern":
			if len(n.List) >= 2 {
				rs.Externs = append(rs.Externs, n.List[1].Atom)
			}
		}
	}

	// Sort rules by priority (descending — highest first).
	sort.Slice(rs.Rules, func(i, j int) bool {
		return rs.Rules[i].Priority > rs.Rules[j].Priority
	})

	return rs, nil
}

func parseDecl(n sexpr.Node) (TermDecl, error) {
	if len(n.List) < 4 {
		return TermDecl{}, fmt.Errorf("line %d: decl needs (decl name (args) ret)", n.Line)
	}
	name := n.List[1].Atom
	var args []string
	if n.List[2].IsList() {
		for _, a := range n.List[2].List {
			args = append(args, a.Atom)
		}
	}
	ret := n.List[3].Atom
	return TermDecl{Name: name, Args: args, RetType: ret}, nil
}

func parseRule(n sexpr.Node) (Rule, error) {
	// (rule [priority] lhs [(if op args...)] rhs)
	r := Rule{}
	idx := 1

	// Check for priority
	if len(n.List) >= 4 && n.List[1].IsAtom() && isInt(n.List[1].Atom) {
		r.Priority = parseInt(n.List[1].Atom)
		idx = 2
	}

	if idx >= len(n.List) {
		return r, fmt.Errorf("line %d: rule needs LHS and RHS", n.Line)
	}

	r.LHS = parseTerm(n.List[idx])
	idx++

	// Check for guard: (if ...)
	if idx < len(n.List) && n.List[idx].IsList() && len(n.List[idx].List) > 0 && n.List[idx].List[0].Atom == "if" {
		g, err := parseGuard(n.List[idx])
		if err != nil {
			return r, err
		}
		r.Guard = g
		idx++
	}

	if idx >= len(n.List) {
		return r, fmt.Errorf("line %d: rule needs RHS", n.Line)
	}

	r.RHS = parseTerm(n.List[idx])
	return r, nil
}

func parseGuard(n sexpr.Node) (*Guard, error) {
	// Two forms:
	//   (if op arg1 arg2)        — flat
	//   (if (op arg1 arg2))      — nested (single list child)
	if len(n.List) < 2 {
		return nil, fmt.Errorf("line %d: guard needs (if op args...) or (if (op args...))", n.Line)
	}

	// Nested form: (if (< ?n 256))
	if len(n.List) == 2 && n.List[1].IsList() && len(n.List[1].List) >= 2 {
		inner := n.List[1]
		g := &Guard{Op: inner.List[0].Atom}
		for _, a := range inner.List[1:] {
			g.Args = append(g.Args, parseTerm(a))
		}
		return g, nil
	}

	// Flat form: (if < ?n 256)
	if len(n.List) < 3 {
		return nil, fmt.Errorf("line %d: guard needs (if op args...) or (if (op args...))", n.Line)
	}
	g := &Guard{Op: n.List[1].Atom}
	for _, a := range n.List[2:] {
		g.Args = append(g.Args, parseTerm(a))
	}
	return g, nil
}

func parseTerm(n sexpr.Node) Term {
	if n.IsAtom() {
		s := n.Atom
		if s == "_" {
			return Term{Kind: TermWild}
		}
		if strings.HasPrefix(s, "?") {
			return Term{Kind: TermVar, Name: s[1:]} // strip ?
		}
		if isInt(s) {
			return Term{Kind: TermInt, IntVal: int64(parseInt(s))}
		}
		return Term{Kind: TermAtom, Name: s}
	}
	// List: (name args...)
	t := Term{Kind: TermCall}
	if len(n.List) > 0 {
		t.Name = n.List[0].Atom
		for _, a := range n.List[1:] {
			t.Args = append(t.Args, parseTerm(a))
		}
	}
	return t
}

func isInt(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 && c == '-' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func parseInt(s string) int {
	v := 0
	neg := false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		v = v*10 + int(c-'0')
	}
	if neg {
		v = -v
	}
	return v
}

// ── Bindings ────────────────────────────────────────────────────────────────

// Bindings holds variable bindings from a successful pattern match.
type Bindings map[string]Term

// ── Matcher ─────────────────────────────────────────────────────────────────

// Match attempts to match term against pattern, producing variable bindings.
// Returns nil bindings if match fails.
func Match(pattern, term Term) Bindings {
	b := Bindings{}
	if matchInto(pattern, term, b) {
		return b
	}
	return nil
}

func matchInto(pat, term Term, b Bindings) bool {
	switch pat.Kind {
	case TermWild:
		return true
	case TermVar:
		if existing, ok := b[pat.Name]; ok {
			return termsEqual(existing, term)
		}
		b[pat.Name] = term
		return true
	case TermInt:
		return term.Kind == TermInt && term.IntVal == pat.IntVal
	case TermAtom:
		return term.Kind == TermAtom && term.Name == pat.Name
	case TermCall:
		if term.Kind != TermCall || term.Name != pat.Name {
			return false
		}
		if len(pat.Args) != len(term.Args) {
			return false
		}
		for i := range pat.Args {
			if !matchInto(pat.Args[i], term.Args[i], b) {
				return false
			}
		}
		return true
	}
	return false
}

func termsEqual(a, b Term) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case TermInt:
		return a.IntVal == b.IntVal
	case TermAtom, TermVar:
		return a.Name == b.Name
	case TermCall:
		if a.Name != b.Name || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !termsEqual(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	}
	return true
}

// ── Rewriter ────────────────────────────────────────────────────────────────

// Apply substitutes bound variables in the RHS expression template.
func Apply(rhs Term, bindings Bindings) Term {
	switch rhs.Kind {
	case TermVar:
		if val, ok := bindings[rhs.Name]; ok {
			return val
		}
		return rhs
	case TermCall:
		result := Term{Kind: TermCall, Name: rhs.Name}
		for _, a := range rhs.Args {
			result.Args = append(result.Args, Apply(a, bindings))
		}
		return result
	default:
		return rhs
	}
}

// Rewrite tries all rules in order (highest priority first) and returns
// the first successful rewrite. Returns (result, true) or (input, false).
func (rs *RuleSet) Rewrite(input Term) (Term, bool) {
	return rs.RewriteWith(input, nil)
}

// RewriteWith tries all rules with optional extern registry.
func (rs *RuleSet) RewriteWith(input Term, externs *ExternRegistry) (Term, bool) {
	for _, r := range rs.Rules {
		bindings := Match(r.LHS, input)
		if bindings == nil {
			continue
		}
		// Check guard
		if r.Guard != nil && !evalGuard(r.Guard, bindings) {
			continue
		}
		// Apply extern constructors in the RHS
		result := Apply(r.RHS, bindings)
		if externs != nil {
			result = externs.Resolve(result, bindings)
		}
		return result, true
	}
	return input, false
}

// RewriteAll recursively rewrites all sub-terms bottom-up (single pass).
func (rs *RuleSet) RewriteAll(input Term) Term {
	return rs.RewriteAllWith(input, nil)
}

// RewriteAllWith recursively rewrites with extern support.
func (rs *RuleSet) RewriteAllWith(input Term, externs *ExternRegistry) Term {
	if input.Kind == TermCall {
		for i := range input.Args {
			input.Args[i] = rs.RewriteAllWith(input.Args[i], externs)
		}
	}
	if result, ok := rs.RewriteWith(input, externs); ok {
		return result
	}
	return input
}

// RewriteFixpoint applies RewriteAll repeatedly until no changes occur
// or fuel is exhausted.
func (rs *RuleSet) RewriteFixpoint(input Term, fuel int) (Term, int) {
	return rs.RewriteFixpointWith(input, fuel, nil)
}

// RewriteFixpointWith applies with extern support.
func (rs *RuleSet) RewriteFixpointWith(input Term, fuel int, externs *ExternRegistry) (Term, int) {
	for i := 0; i < fuel; i++ {
		result := rs.RewriteAllWith(input, externs)
		if result.String() == input.String() {
			return result, i + 1
		}
		input = result
	}
	return input, fuel
}

// ── String representation ───────────────────────────────────────────────────

func (t Term) String() string {
	switch t.Kind {
	case TermWild:
		return "_"
	case TermVar:
		return "?" + t.Name
	case TermInt:
		return fmt.Sprintf("%d", t.IntVal)
	case TermAtom:
		return t.Name
	case TermCall:
		parts := []string{t.Name}
		for _, a := range t.Args {
			parts = append(parts, a.String())
		}
		return "(" + strings.Join(parts, " ") + ")"
	}
	return "?"
}
