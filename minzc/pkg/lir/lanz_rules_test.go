package lir

import (
	"testing"

	"github.com/minz/minzc/pkg/lanz"
)

// TestLanzAsRuleDSL demonstrates that Lanz S-expressions can represent
// optimization rules. The same parser that compiles Lanz programs can
// parse rule definitions — no new DSL needed.
func TestLanzAsRuleDSL(t *testing.T) {
	// Rule definitions as Lanz S-expressions.
	// These aren't compiled as programs — they're parsed as data structures
	// that the optimizer interprets.
	ruleSrc := `
;; Rule 1: add(x, 1) → inc(x) when dst == src
(rule inc_peephole
  (match (add ?x (const 1)))
  (emit  (inc ?x)))

;; Rule 2: sub(a,b) followed by cmp.lt(a,b) → fuse
(rule sub_cmp_fusion
  (match (seq (sub ?a ?b) (cmp lt ?a ?b)))
  (emit  (seq (sub ?a ?b) (cmp-sub-carry (result-of sub)))))

;; Rule 3: neg(sub(a,b)) → sub(b,a)
(rule neg_sub_swap
  (match (neg (sub ?a ?b)))
  (emit  (sub ?b ?a)))

;; Rule 4: cp 0 after flag-setting → delete cp
(rule redundant_cp
  (match (seq (?i (flag-setter)) (cp 0) (?br (branch))))
  (when  (uses-flag ?br (flags-of ?i)))
  (emit  (seq ?i ?br)))
`

	// Parse as S-expressions (Lanz parser handles this natively)
	nodes, err := lanz.ParseSExpr(ruleSrc)
	if err != nil {
		t.Fatalf("parse rules: %v", err)
	}

	// Verify we got 4 rule definitions
	ruleCount := 0
	for _, n := range nodes {
		if n.IsList() && len(n.List) > 0 && n.List[0].IsAtom() && n.List[0].Atom == "rule" {
			ruleCount++
			name := n.List[1].Atom
			t.Logf("parsed rule: %s (%d sub-forms)", name, len(n.List)-2)
		}
	}

	if ruleCount != 4 {
		t.Errorf("expected 4 rules, got %d", ruleCount)
	}
}

// TestLanzRoundTrip_IRPattern shows that a Lanz program and its
// optimization can both be expressed in the same language.
func TestLanzRoundTrip_IRPattern(t *testing.T) {
	// Original program in Lanz
	programSrc := `
(fun double ((x u8)) u8
  (return (+ x x)))
(assert double 5 == 10)
`
	// This compiles and runs
	mod, err := lanz.Compile(programSrc, "test")
	if err != nil {
		t.Fatalf("compile program: %v", err)
	}
	if len(mod.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(mod.Funcs))
	}

	// The SAME Lanz parser can express patterns over this program:
	patternSrc := `
;; Match: any function where body is (return (+ ?x ?x))
;; This is "double" pattern — same arg used twice
(pattern double-detect
  (fun ?name ((?x ?ty)) ?ty
    (return (+ ?x ?x))))
`
	patNodes, err := lanz.ParseSExpr(patternSrc)
	if err != nil {
		t.Fatalf("parse pattern: %v", err)
	}

	if len(patNodes) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patNodes))
	}
	t.Logf("pattern parsed: %s", patNodes[0].List[1].Atom)
}
