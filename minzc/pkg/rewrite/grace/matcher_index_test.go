package grace

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/rewrite"
)

// listerGraph adds the IRBlockLister fast path to mockGraph.
type listerGraph struct{ *mockGraph }

func (g listerGraph) BlockList() []rewrite.IRBlock {
	list := make([]rewrite.IRBlock, len(g.blocks))
	for i := range g.blocks {
		list[i] = &g.blocks[i]
	}
	return list
}

// ── Reference matcher: the pre-index algorithm, kept verbatim in spirit ──

func refEdgeExists(graph rewrite.IRGraph, from, to, kind string) bool {
	fromBlock := graph.Block(from)
	if fromBlock == nil {
		return false
	}
	targets := fromBlock.TermTargets()
	has := func() bool {
		for _, t := range targets {
			if t == to {
				return true
			}
		}
		return false
	}
	switch kind {
	case "then":
		return len(targets) > 0 && targets[0] == to
	case "else":
		return len(targets) > 1 && targets[1] == to
	case "back":
		fromIdx, toIdx := -1, -1
		for i, l := range graph.Blocks() {
			if l == from {
				fromIdx = i
			}
			if l == to {
				toIdx = i
			}
		}
		return fromIdx >= 0 && toIdx >= 0 && toIdx <= fromIdx && has()
	default:
		return has()
	}
}

func refCheckEdges(graph rewrite.IRGraph, edges []MatchClause, b BlockBindings) bool {
	for _, e := range edges {
		f, ok1 := b[e.Var]
		t, ok2 := b[e.Var2]
		if !ok1 || !ok2 || !refEdgeExists(graph, f, t, e.EdgeKind) {
			return false
		}
	}
	return true
}

func refCheckPreds(graph rewrite.IRGraph, preds []Predicate, b BlockBindings, reg *PredicateRegistry) bool {
	for _, p := range preds {
		ok := true
		switch p.Kind {
		case PredPredCount, PredUseCount:
			ok = cmpInt(len(graph.Predecessors(b[p.Var])), p.Op, p.Value)
		case PredFieldCmp:
			blk := graph.Block(b[p.Var])
			ok = blk != nil && cmpInt(blockField(blk, p.Field), p.Op, p.Value)
		case PredDominates:
			ia, ib := -1, -1
			for i, l := range graph.Blocks() {
				if l == b[p.Var] {
					ia = i
				}
				if l == b[p.Var2] {
					ib = i
				}
			}
			ok = ia >= 0 && ib >= 0 && ia < ib
		case PredCustom:
			ok = reg.predicates[p.CustomName](graph, b, p.CustomArgs)
		}
		if !ok {
			return false
		}
	}
	return true
}

func refFindMatch(graph rewrite.IRGraph, rule *GraceRule, reg *PredicateRegistry) BlockBindings {
	var bms, edges []MatchClause
	for _, m := range rule.Match {
		if m.Kind == MatchBlock {
			bms = append(bms, m)
		} else if m.Kind == MatchEdge {
			edges = append(edges, m)
		}
	}
	blocks := graph.Blocks()
	bindings := BlockBindings{}
	var rec func(depth int) bool
	rec = func(depth int) bool {
		if depth == len(bms) {
			return refCheckEdges(graph, edges, bindings) && refCheckPreds(graph, rule.Where, bindings, reg)
		}
		for _, l := range blocks {
			used := false
			for _, v := range bindings {
				if v == l {
					used = true
				}
			}
			if used {
				continue
			}
			b := graph.Block(l)
			if b == nil || !matchBlockClause(bms[depth], b) {
				continue
			}
			bindings[bms[depth].Var] = l
			if rec(depth + 1) {
				return true
			}
			delete(bindings, bms[depth].Var)
		}
		return false
	}
	if len(bms) == 0 || !rec(0) {
		return nil
	}
	return bindings
}

func randomGraph(r *rand.Rand) *mockGraph {
	n := 1 + r.Intn(12)
	label := func(i int) string { return fmt.Sprintf("b%d", i) }
	kinds := []string{"jump", "br_if", "ret"}
	g := &mockGraph{}
	for i := 0; i < n; i++ {
		l := label(i)
		if i > 0 && r.Intn(10) == 0 {
			l = label(r.Intn(i)) // duplicate label
		}
		k := kinds[r.Intn(len(kinds))]
		var targets []string
		switch k {
		case "jump":
			targets = []string{label(r.Intn(n + 2))} // may name a missing block
		case "br_if":
			targets = []string{label(r.Intn(n + 1)), label(r.Intn(n + 1))}
		}
		g.blocks = append(g.blocks, mockBlock{label: l, termKind: k, targets: targets, params: r.Intn(2), insts: make([]mockInst, r.Intn(3))})
	}
	return g
}

var randomRules = []string{
	`(grace r1 0 (match (block ?a (term-kind "jump"))) (where (pred-count ?a >= 1) (probe ?a)) (action))`,
	`(grace r2 0 (match (block ?p (term-kind "jump")) (block ?s) (edge ?p ?s "succ")) (where (pred-count ?s == 1) (probe ?p ?s)) (action))`,
	`(grace r3 0 (match (block ?h (term-kind "br_if")) (block ?b) (edge ?h ?b "then") (edge ?b ?h "succ")) (where (probe ?h ?b)) (action))`,
	`(grace r4 0 (match (block ?x) (block ?y) (edge ?y ?x "back")) (where (probe ?x ?y)) (action))`,
	`(grace r5 0 (match (block ?c (term-kind "br_if")) (block ?e) (edge ?c ?e "else")) (where (inst-count ?e < 2) (probe ?c ?e)) (action))`,
	`(grace r6 0 (match (block ?x) (block ?y)) (where (dominates ?x ?y) (probe ?x ?y)) (action))`,
	`(grace r7 0 (match (block ?a) (block ?b) (block ?c) (edge ?a ?b "succ") (edge ?b ?c "succ")) (where (probe ?a ?b ?c)) (action))`,
}

// TestIndexedMatcherAgreesWithReference checks, on random graphs (including
// duplicate labels and edges to missing blocks), that the indexed matcher
// returns the same first match as the original exhaustive enumeration and
// calls the custom predicate on exactly the same bindings in the same order.
func TestIndexedMatcherAgreesWithReference(t *testing.T) {
	r := rand.New(rand.NewSource(338))
	var rules []GraceRule
	for _, src := range randomRules {
		parsed, err := ParseRules(src)
		if err != nil {
			t.Fatal(err)
		}
		rules = append(rules, parsed...)
	}
	for iter := 0; iter < 3000; iter++ {
		g := randomGraph(r)
		seed := r.Int63()
		for ri := range rules {
			for _, graph := range []rewrite.IRGraph{g, listerGraph{g}} {
				run := func(find func(rewrite.IRGraph, *GraceRule, *PredicateRegistry) BlockBindings) (BlockBindings, []string) {
					var log []string
					reg := NewPredicateRegistry()
					reg.RegisterPredicate("probe", func(_ rewrite.IRGraph, b BlockBindings, args []string) bool {
						var parts []string
						for _, a := range args {
							parts = append(parts, b[a])
						}
						key := strings.Join(parts, ",")
						log = append(log, key)
						h := seed
						for _, c := range key {
							h = h*31 + int64(c)
						}
						return h%3 == 0
					})
					return find(graph, &rules[ri], reg), log
				}
				want, wantLog := run(refFindMatch)
				got, gotLog := run(findMatch)
				if !reflect.DeepEqual(want, got) || !reflect.DeepEqual(wantLog, gotLog) {
					t.Fatalf("iter %d rule %s: indexed matcher diverged\nwant %v %v\ngot  %v %v\ngraph %+v",
						iter, rules[ri].Name, want, wantLog, got, gotLog, g.blocks)
				}
			}
		}
	}
}

// TestGenerationInvalidatesPredicateCache: a predicate caching a whole-graph
// fact on Generation must see the mutation made by an action between two
// queries. A cache that ignored the generation would answer from the first
// query and stop after one rewrite.
func TestGenerationInvalidatesPredicateCache(t *testing.T) {
	g := &mockGraph{blocks: []mockBlock{
		{label: "a", termKind: "jump", targets: []string{"b"}, insts: make([]mockInst, 3)},
		{label: "b", termKind: "ret"},
	}}
	rules, err := ParseRules(`(grace shrink 0 (match (block ?b)) (where (has-insts ?b)) (action (custom "drop" ?b)))`)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewPredicateRegistry()
	cachedGen, computed := uint64(0), false
	var cachedTotal int
	total := func() int {
		if !computed || cachedGen != reg.Generation() {
			cachedTotal, cachedGen, computed = 0, reg.Generation(), true
			for _, b := range g.blocks {
				cachedTotal += len(b.insts)
			}
		}
		return cachedTotal
	}
	reg.RegisterPredicate("has-insts", func(_ rewrite.IRGraph, b BlockBindings, args []string) bool {
		return total() > 0 && b[args[0]] == "a"
	})
	reg.RegisterAction("drop", func(_ rewrite.IRGraphMut, _ BlockBindings) bool {
		if len(g.blocks[0].insts) > 0 {
			g.blocks[0].insts = g.blocks[0].insts[1:]
		}
		return true // a stale cache would keep this rule firing until maxRounds
	})
	before := reg.Generation()
	res := ApplyRules(mutMock{g}, rules, reg, 10)
	if res.Applied != 3 || len(g.blocks[0].insts) != 0 {
		t.Fatalf("applied %d, %d insts left; want 3 and 0", res.Applied, len(g.blocks[0].insts))
	}
	if reg.Generation() == before {
		t.Fatal("generation did not advance across actions")
	}
	// The cached total must now be stale-free: zero instructions remain.
	if total() != 0 {
		t.Fatalf("cached total %d after mutation, want 0", total())
	}
}

type mutMock struct{ *mockGraph }

func (mutMock) RemoveInst(string, int)                  {}
func (mutMock) ReplaceInst(string, int, rewrite.IRNode) {}
func (mutMock) AppendInst(string, rewrite.IRNode)       {}
func (mutMock) HoistInsts(string, string)               {}
func (mutMock) RemoveBlock(string)                      {}
func (mutMock) SetTerm(string, string, []string)        {}
func (mutMock) RemoveBlockParam(string, int)            {}
