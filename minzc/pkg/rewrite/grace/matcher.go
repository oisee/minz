package grace

import "github.com/minz/minzc/pkg/rewrite"

// BlockBindings maps variable names to block labels.
type BlockBindings map[string]string

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

	blocks := graph.Blocks()

	if len(blockMatches) == 1 {
		return findSingleBlockMatch(graph, rule, blockMatches[0], edgeMatches, preds, blocks)
	}

	if len(blockMatches) == 2 {
		return findTwoBlockMatch(graph, rule, blockMatches, edgeMatches, preds, blocks)
	}

	// N-way matching for 3+ blocks (rare but supported)
	return findNBlockMatch(graph, rule, blockMatches, edgeMatches, preds, blocks)
}

func findSingleBlockMatch(graph rewrite.IRGraph, rule *GraceRule, bm MatchClause, edges []MatchClause, preds *PredicateRegistry, blocks []string) BlockBindings {
	for _, label := range blocks {
		b := graph.Block(label)
		if b == nil {
			continue
		}
		if !matchBlockClause(bm, b) {
			continue
		}
		bindings := BlockBindings{bm.Var: label}
		if checkEdges(graph, edges, bindings) && checkPredicates(graph, rule.Where, bindings, preds) {
			return bindings
		}
	}
	return nil
}

func findTwoBlockMatch(graph rewrite.IRGraph, rule *GraceRule, bms []MatchClause, edges []MatchClause, preds *PredicateRegistry, blocks []string) BlockBindings {
	for _, l0 := range blocks {
		b0 := graph.Block(l0)
		if b0 == nil || !matchBlockClause(bms[0], b0) {
			continue
		}
		for _, l1 := range blocks {
			if l0 == l1 {
				continue
			}
			b1 := graph.Block(l1)
			if b1 == nil || !matchBlockClause(bms[1], b1) {
				continue
			}
			bindings := BlockBindings{
				bms[0].Var: l0,
				bms[1].Var: l1,
			}
			if checkEdges(graph, edges, bindings) && checkPredicates(graph, rule.Where, bindings, preds) {
				return bindings
			}
		}
	}
	return nil
}

func findNBlockMatch(graph rewrite.IRGraph, rule *GraceRule, bms []MatchClause, edges []MatchClause, preds *PredicateRegistry, blocks []string) BlockBindings {
	// Recursive backtracking for N block variables
	bindings := make(BlockBindings)
	if backtrack(graph, rule, bms, edges, preds, blocks, bindings, 0) {
		return bindings
	}
	return nil
}

func backtrack(graph rewrite.IRGraph, rule *GraceRule, bms []MatchClause, edges []MatchClause, preds *PredicateRegistry, blocks []string, bindings BlockBindings, depth int) bool {
	if depth == len(bms) {
		return checkEdges(graph, edges, bindings) && checkPredicates(graph, rule.Where, bindings, preds)
	}

	bm := bms[depth]
	for _, label := range blocks {
		// Check uniqueness
		used := false
		for _, v := range bindings {
			if v == label {
				used = true
				break
			}
		}
		if used {
			continue
		}

		b := graph.Block(label)
		if b == nil || !matchBlockClause(bm, b) {
			continue
		}

		bindings[bm.Var] = label
		if backtrack(graph, rule, bms, edges, preds, blocks, bindings, depth+1) {
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

func checkEdges(graph rewrite.IRGraph, edges []MatchClause, bindings BlockBindings) bool {
	for _, e := range edges {
		fromLabel, ok1 := bindings[e.Var]
		toLabel, ok2 := bindings[e.Var2]
		if !ok1 || !ok2 {
			return false
		}
		if !edgeExists(graph, fromLabel, toLabel, e.EdgeKind) {
			return false
		}
	}
	return true
}

func edgeExists(graph rewrite.IRGraph, from, to, kind string) bool {
	fromBlock := graph.Block(from)
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
		blocks := graph.Blocks()
		fromIdx, toIdx := -1, -1
		for i, l := range blocks {
			if l == from {
				fromIdx = i
			}
			if l == to {
				toIdx = i
			}
		}
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

	blocks := graph.Blocks()
	var results []BlockBindings

	if len(blockMatches) == 1 {
		bm := blockMatches[0]
		for _, label := range blocks {
			b := graph.Block(label)
			if b == nil || !matchBlockClause(bm, b) {
				continue
			}
			bindings := BlockBindings{bm.Var: label}
			if checkEdges(graph, edgeMatches, bindings) && checkPredicates(graph, rule.Where, bindings, preds) {
				result := make(BlockBindings)
				for k, v := range bindings {
					result[k] = v
				}
				results = append(results, result)
			}
		}
	} else if len(blockMatches) >= 2 {
		// Enumerate all combinations
		findAllBindings(graph, rule, blockMatches, edgeMatches, preds, blocks, make(BlockBindings), 0, &results)
	}

	return results
}

func findAllBindings(graph rewrite.IRGraph, rule *GraceRule, bms []MatchClause, edges []MatchClause, preds *PredicateRegistry, blocks []string, bindings BlockBindings, depth int, results *[]BlockBindings) {
	if depth == len(bms) {
		if checkEdges(graph, edges, bindings) && checkPredicates(graph, rule.Where, bindings, preds) {
			result := make(BlockBindings)
			for k, v := range bindings {
				result[k] = v
			}
			*results = append(*results, result)
		}
		return
	}

	bm := bms[depth]
	for _, label := range blocks {
		used := false
		for _, v := range bindings {
			if v == label {
				used = true
				break
			}
		}
		if used {
			continue
		}

		b := graph.Block(label)
		if b == nil || !matchBlockClause(bm, b) {
			continue
		}

		bindings[bm.Var] = label
		findAllBindings(graph, rule, bms, edges, preds, blocks, bindings, depth+1, results)
		delete(bindings, bm.Var)
	}
}
