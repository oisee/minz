// Package isle implements a term-rewriting DSL for instruction selection,
// inspired by Cranelift's ISLE (Instruction Selection Lowering Expressions).
//
// ISLE rules describe how MIR2 operations lower to target instructions.
// Rules are compiled into a decision tree for efficient matching.
//
// Syntax (S-expression based, parsed by Lanz parser):
//
//	;; Type declarations
//	(type Reg)
//	(type Inst)
//	(type u8)
//
//	;; Term declarations: (decl name (arg-types...) result-type)
//	(decl lower (MIROp) Inst)
//	(decl put_in_a (Reg) Reg)
//	(decl gpr8 (Reg) Reg)
//
//	;; Rules: (rule [priority] (lhs-pattern) (rhs-expression))
//	(rule (lower (add u8 ?x ?y))
//	      (z80_add_a (put_in_a ?x) (gpr8 ?y)))
//
//	;; Prioritized rule (higher number = tried first):
//	(rule 10 (lower (add u8 ?x (const 1)))
//	         (z80_inc (gpr8 ?x)))
//
//	;; Extern constructors (implemented in Go):
//	(extern put_in_a)
//	(extern gpr8)
//
// The ISLE compiler:
// 1. Parses rules from S-expressions (reuses Lanz parser)
// 2. Builds a decision tree (trie) from all LHS patterns
// 3. At runtime, traverses the trie to find matching rules
// 4. Evaluates RHS to produce target instructions
package isle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/lanz"
)

// ── AST ─────────────────────────────────────────────────────────────────────

// RuleSet is a parsed ISLE program (set of rules + declarations).
type RuleSet struct {
	Types    []TypeDecl
	Decls    []TermDecl
	Rules    []Rule
	Externs  []string // extern constructor names
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

// Rule is one rewrite rule: LHS pattern → RHS expression.
type Rule struct {
	Priority int    // higher = tried first (0 = default)
	LHS      Term   // pattern to match (with variables ?x)
	RHS      Term   // expression to produce
}

// Term is a tree node in a pattern or expression.
type Term struct {
	Kind     TermKind
	Name     string  // atom name, variable name, or constructor name
	Args     []Term  // child terms (for constructor/call)
	IntVal   int64   // for integer literal
}

type TermKind int

const (
	TermVar    TermKind = iota // ?x — pattern variable
	TermAtom                   // symbol/keyword
	TermInt                    // integer literal
	TermCall                   // (name args...) — constructor/function call
	TermWild                   // _ — wildcard (matches anything)
)

// ── Parser ──────────────────────────────────────────────────────────────────

// Parse parses ISLE rules from source text.
func Parse(src string) (*RuleSet, error) {
	nodes, err := lanz.ParseSExpr(src)
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

func parseDecl(n lanz.Node) (TermDecl, error) {
	// (decl name (arg-types...) ret-type)
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

func parseRule(n lanz.Node) (Rule, error) {
	// (rule [priority] lhs rhs)
	// (rule lhs rhs) — priority 0
	// (rule 10 lhs rhs) — priority 10
	r := Rule{}
	idx := 1

	// Check for priority
	if len(n.List) >= 4 && n.List[1].IsAtom() && isInt(n.List[1].Atom) {
		r.Priority = parseInt(n.List[1].Atom)
		idx = 2
	}

	if idx+1 >= len(n.List) {
		return r, fmt.Errorf("line %d: rule needs LHS and RHS", n.Line)
	}

	r.LHS = parseTerm(n.List[idx])
	r.RHS = parseTerm(n.List[idx+1])
	return r, nil
}

func parseTerm(n lanz.Node) Term {
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

// ── Matcher ─────────────────────────────────────────────────────────────────

// Bindings holds variable bindings from a successful pattern match.
type Bindings map[string]Term

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
		return true // matches anything
	case TermVar:
		if existing, ok := b[pat.Name]; ok {
			return termsEqual(existing, term) // consistency check
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
		return rhs // unbound variable — leave as-is
	case TermCall:
		result := Term{Kind: TermCall, Name: rhs.Name}
		for _, a := range rhs.Args {
			result.Args = append(result.Args, Apply(a, bindings))
		}
		return result
	default:
		return rhs // atoms, ints, wildcards pass through
	}
}

// Rewrite tries all rules in order (highest priority first) and returns
// the first successful rewrite. Returns (result, true) or (input, false).
func (rs *RuleSet) Rewrite(input Term) (Term, bool) {
	for _, r := range rs.Rules {
		if bindings := Match(r.LHS, input); bindings != nil {
			return Apply(r.RHS, bindings), true
		}
	}
	return input, false
}

// RewriteAll recursively rewrites all sub-terms bottom-up.
func (rs *RuleSet) RewriteAll(input Term) Term {
	// First rewrite children
	if input.Kind == TermCall {
		for i := range input.Args {
			input.Args[i] = rs.RewriteAll(input.Args[i])
		}
	}
	// Then try to rewrite this node
	if result, ok := rs.Rewrite(input); ok {
		return result
	}
	return input
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
