package grace

import (
	"sort"

	"github.com/minz/minzc/pkg/rewrite"
)

// BlockBindings maps variable names to block labels.
type BlockBindings map[string]string

// matchView is a per-query index over an IRGraph. It answers the questions
// the matcher asks repeatedly (block by label, layout position, predecessors,
// blocks that branch to a label) without the linear scans of a naive adapter.
//
// Validity: a view is built at the start of one findMatch / FindAllMatches
// call and dropped when it returns. Inside that window Grace performs no
// mutation (actions run only after findMatch has returned) and predicates are
// read-only queries, so every memoised answer equals what the underlying graph
// would return at the time of the probe. A view must never be kept across an
// action.
type matchView struct {
	graph  rewrite.IRGraph
	labels []string

	// block memoises graph.Block(label). With an IRBlockLister graph it is
	// filled eagerly from the first occurrence of each label, which is the
	// block a label lookup returns.
	block map[string]rewrite.IRBlock

	// lastIdx is the last layout index of each label (what the original
	// linear scans for "back" edges and dominates computed).
	lastIdx map[string]int
	// allIdx lists every layout index of each label in ascending order.
	allIdx map[string][]int

	// preds memoises graph.Predecessors(label).
	preds map[string][]string

	// branchers maps a label to the ascending layout indices i whose block
	// (blockOf(labels[i])) has that label among its TermTargets. Lazy.
	branchers map[string][]int
}

func newMatchView(graph rewrite.IRGraph) *matchView {
	v := &matchView{
		graph:  graph,
		labels: graph.Blocks(),
		block:  make(map[string]rewrite.IRBlock),
		preds:  make(map[string][]string),
	}
	if lister, ok := graph.(rewrite.IRBlockLister); ok {
		list := lister.BlockList()
		if len(list) == len(v.labels) {
			for i, l := range v.labels {
				if _, seen := v.block[l]; !seen {
					v.block[l] = list[i]
				}
			}
		}
	}
	return v
}

func (v *matchView) blockOf(label string) rewrite.IRBlock {
	if b, ok := v.block[label]; ok {
		return b
	}
	b := v.graph.Block(label)
	v.block[label] = b
	return b
}

func (v *matchView) buildIndex() {
	if v.lastIdx != nil {
		return
	}
	v.lastIdx = make(map[string]int, len(v.labels))
	v.allIdx = make(map[string][]int, len(v.labels))
	for i, l := range v.labels {
		v.lastIdx[l] = i
		v.allIdx[l] = append(v.allIdx[l], i)
	}
}

// layoutIndex returns the last layout index of label, or -1.
func (v *matchView) layoutIndex(label string) int {
	v.buildIndex()
	if i, ok := v.lastIdx[label]; ok {
		return i
	}
	return -1
}

func (v *matchView) predecessors(label string) []string {
	if p, ok := v.preds[label]; ok {
		return p
	}
	p := v.graph.Predecessors(label)
	v.preds[label] = p
	return p
}

func (v *matchView) brancherIndices(label string) []int {
	if v.branchers == nil {
		v.branchers = make(map[string][]int)
		for i, l := range v.labels {
			b := v.blockOf(l)
			if b == nil {
				continue
			}
			for _, t := range b.TermTargets() {
				list := v.branchers[t]
				if n := len(list); n > 0 && list[n-1] == i {
					continue
				}
				v.branchers[t] = append(list, i)
			}
		}
	}
	return v.branchers[label]
}

// findMatch finds the first valid binding for a rule's match clauses.
func findMatch(graph rewrite.IRGraph, rule *GraceRule, preds *PredicateRegistry) BlockBindings {
	// Collect block-level match clauses
	var blockMatches []MatchClause
	var edgeMatches []MatchClause
	for _, m := range rule.Match {
		switch m.Kind {
		case MatchBlock:
			blockMatches = append(blockMatches, m)
		case MatchEdge:
			edgeMatches = append(edgeMatches, m)
		}
	}

	if len(blockMatches) == 0 {
		return nil
	}

	v := newMatchView(graph)

	if len(blockMatches) == 1 {
		return findSingleBlockMatch(v, rule, blockMatches[0], edgeMatches, preds)
	}

	if len(blockMatches) == 2 {
		return findTwoBlockMatch(v, rule, blockMatches, edgeMatches, preds)
	}

	// N-way matching for 3+ blocks (rare but supported)
	return findNBlockMatch(v, rule, blockMatches, edgeMatches, preds)
}

func findSingleBlockMatch(v *matchView, rule *GraceRule, bm MatchClause, edges []MatchClause, preds *PredicateRegistry) BlockBindings {
	for _, label := range v.labels {
		b := v.blockOf(label)
		if b == nil {
			continue
		}
		if !matchBlockClause(bm, b) {
			continue
		}
		bindings := BlockBindings{bm.Var: label}
		if checkEdges(v, edges, bindings) && checkPredicates(v, rule.Where, bindings, preds) {
			return bindings
		}
	}
	return nil
}

// secondCandidates returns, in ascending layout order, a superset of the
// layout indices i for which an edge clause between the two block variables
// can hold once bms[0] is bound to l0. Every edge kind requires the target to
// be among the source's TermTargets, so a binding outside this set fails
// checkEdges before any predicate runs; skipping it changes neither the
// result nor the sequence of predicate calls. ok is false when no edge
// clause links the two (distinct) variables; then every block is tried.
func secondCandidates(v *matchView, bms []MatchClause, edges []MatchClause, l0 string) (idx []int, ok bool) {
	if bms[0].Var == bms[1].Var {
		return nil, false
	}
	for _, e := range edges {
		switch {
		case e.Var == bms[0].Var && e.Var2 == bms[1].Var:
			// l0 -> l1: l1 is one of l0's targets.
			b0 := v.blockOf(l0)
			if b0 == nil {
				return nil, true
			}
			v.buildIndex()
			for _, t := range b0.TermTargets() {
				idx = append(idx, v.allIdx[t]...)
			}
			sort.Ints(idx)
			out := idx[:0]
			for i, x := range idx {
				if i > 0 && idx[i-1] == x {
					continue
				}
				out = append(out, x)
			}
			return out, true
		case e.Var == bms[1].Var && e.Var2 == bms[0].Var:
			// l1 -> l0: l1 branches to l0.
			return v.brancherIndices(l0), true
		}
	}
	return nil, false
}

func findTwoBlockMatch(v *matchView, rule *GraceRule, bms []MatchClause, edges []MatchClause, preds *PredicateRegistry) BlockBindings {
	for _, l0 := range v.labels {
		b0 := v.blockOf(l0)
		if b0 == nil || !matchBlockClause(bms[0], b0) {
			continue
		}
		try := func(l1 string) BlockBindings {
			if l0 == l1 {
				return nil
			}
			b1 := v.blockOf(l1)
			if b1 == nil || !matchBlockClause(bms[1], b1) {
				return nil
			}
			bindings := BlockBindings{
				bms[0].Var: l0,
				bms[1].Var: l1,
			}
			if checkEdges(v, edges, bindings) && checkPredicates(v, rule.Where, bindings, preds) {
				return bindings
			}
			return nil
		}
		if cand, ok := secondCandidates(v, bms, edges, l0); ok {
			for _, i := range cand {
				if b := try(v.labels[i]); b != nil {
					return b
				}
			}
			continue
		}
		for _, l1 := range v.labels {
			if b := try(l1); b != nil {
				return b
			}
		}
	}
	return nil
}

func findNBlockMatch(v *matchView, rule *GraceRule, bms []MatchClause, edges []MatchClause, preds *PredicateRegistry) BlockBindings {
	// Recursive backtracking for N block variables
	bindings := make(BlockBindings)
	if backtrack(v, rule, bms, edges, preds, bindings, 0) {
		return bindings
	}
	return nil
}

func backtrack(v *matchView, rule *GraceRule, bms []MatchClause, edges []MatchClause, preds *PredicateRegistry, bindings BlockBindings, depth int) bool {
	if depth == len(bms) {
		return checkEdges(v, edges, bindings) && checkPredicates(v, rule.Where, bindings, preds)
	}

	bm := bms[depth]
	for _, label := range v.labels {
		// Check uniqueness
		used := false
		for _, val := range bindings {
			if val == label {
				used = true
				break
			}
		}
		if used {
			continue
		}

		b := v.blockOf(label)
		if b == nil || !matchBlockClause(bm, b) {
			continue
		}

		bindings[bm.Var] = label
		if backtrack(v, rule, bms, edges, preds, bindings, depth+1) {
			return true
		}
		delete(bindings, bm.Var)
	}
	return false
}

func matchBlockClause(m MatchClause, b rewrite.IRBlock) bool {
	if m.TermKind != "" && b.TermKind() != m.TermKind {
		return false
	}
	return true
}

func checkEdges(v *matchView, edges []MatchClause, bindings BlockBindings) bool {
	for _, e := range edges {
		fromLabel, ok1 := bindings[e.Var]
		toLabel, ok2 := bindings[e.Var2]
		if !ok1 || !ok2 {
			return false
		}
		if !edgeExists(v, fromLabel, toLabel, e.EdgeKind) {
			return false
		}
	}
	return true
}

func edgeExists(v *matchView, from, to, kind string) bool {
	fromBlock := v.blockOf(from)
	if fromBlock == nil {
		return false
	}
	targets := fromBlock.TermTargets()

	switch kind {
	case "succ":
		for _, t := range targets {
			if t == to {
				return true
			}
		}
	case "then":
		if len(targets) > 0 && targets[0] == to {
			return true
		}
	case "else":
		if len(targets) > 1 && targets[1] == to {
			return true
		}
	case "back":
		// Back-edge: to appears before from in layout
		fromIdx, toIdx := v.layoutIndex(from), v.layoutIndex(to)
		if fromIdx >= 0 && toIdx >= 0 && toIdx <= fromIdx {
			for _, t := range targets {
				if t == to {
					return true
				}
			}
		}
	default:
		// Unknown edge kind — treat as successor
		for _, t := range targets {
			if t == to {
				return true
			}
		}
	}
	return false
}

// FindAllMatches returns all valid bindings for a rule (used by tests).
func FindAllMatches(graph rewrite.IRGraph, rule *GraceRule, preds *PredicateRegistry) []BlockBindings {
	var blockMatches []MatchClause
	var edgeMatches []MatchClause
	for _, m := range rule.Match {
		switch m.Kind {
		case MatchBlock:
			blockMatches = append(blockMatches, m)
		case MatchEdge:
			edgeMatches = append(edgeMatches, m)
		}
	}

	v := newMatchView(graph)
	var results []BlockBindings

	if len(blockMatches) == 1 {
		bm := blockMatches[0]
		for _, label := range v.labels {
			b := v.blockOf(label)
			if b == nil || !matchBlockClause(bm, b) {
				continue
			}
			bindings := BlockBindings{bm.Var: label}
			if checkEdges(v, edgeMatches, bindings) && checkPredicates(v, rule.Where, bindings, preds) {
				result := make(BlockBindings)
				for k, val := range bindings {
					result[k] = val
				}
				results = append(results, result)
			}
		}
	} else if len(blockMatches) >= 2 {
		// Enumerate all combinations
		findAllBindings(v, rule, blockMatches, edgeMatches, preds, make(BlockBindings), 0, &results)
	}

	return results
}

func findAllBindings(v *matchView, rule *GraceRule, bms []MatchClause, edges []MatchClause, preds *PredicateRegistry, bindings BlockBindings, depth int, results *[]BlockBindings) {
	if depth == len(bms) {
		if checkEdges(v, edges, bindings) && checkPredicates(v, rule.Where, bindings, preds) {
			result := make(BlockBindings)
			for k, val := range bindings {
				result[k] = val
			}
			*results = append(*results, result)
		}
		return
	}

	bm := bms[depth]
	for _, label := range v.labels {
		used := false
		for _, val := range bindings {
			if val == label {
				used = true
				break
			}
		}
		if used {
			continue
		}

		b := v.blockOf(label)
		if b == nil || !matchBlockClause(bm, b) {
			continue
		}

		bindings[bm.Var] = label
		findAllBindings(v, rule, bms, edges, preds, bindings, depth+1, results)
		delete(bindings, bm.Var)
	}
}
