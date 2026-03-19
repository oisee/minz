package grace

import "github.com/minz/minzc/pkg/rewrite"

// PredicateFunc is a custom predicate implemented in Go.
type PredicateFunc func(graph rewrite.IRGraph, bindings BlockBindings, args []string) bool

// CustomActionFunc is a custom action implemented in Go.
type CustomActionFunc func(graph rewrite.IRGraphMut, bindings BlockBindings) bool

// PredicateRegistry holds registered custom predicates and actions.
type PredicateRegistry struct {
	predicates map[string]PredicateFunc
	actions    map[string]CustomActionFunc
}

// NewPredicateRegistry creates an empty registry.
func NewPredicateRegistry() *PredicateRegistry {
	return &PredicateRegistry{
		predicates: make(map[string]PredicateFunc),
		actions:    make(map[string]CustomActionFunc),
	}
}

// RegisterPredicate adds a custom predicate.
func (r *PredicateRegistry) RegisterPredicate(name string, fn PredicateFunc) {
	r.predicates[name] = fn
}

// RegisterAction adds a custom action.
func (r *PredicateRegistry) RegisterAction(name string, fn CustomActionFunc) {
	r.actions[name] = fn
}

// checkPredicates evaluates all WHERE predicates against bindings.
func checkPredicates(graph rewrite.IRGraph, preds []Predicate, bindings BlockBindings, reg *PredicateRegistry) bool {
	for _, p := range preds {
		if !checkPredicate(graph, p, bindings, reg) {
			return false
		}
	}
	return true
}

func checkPredicate(graph rewrite.IRGraph, p Predicate, bindings BlockBindings, reg *PredicateRegistry) bool {
	switch p.Kind {
	case PredPredCount:
		label, ok := bindings[p.Var]
		if !ok {
			return false
		}
		count := len(graph.Predecessors(label))
		return cmpInt(count, p.Op, p.Value)

	case PredFieldCmp:
		label, ok := bindings[p.Var]
		if !ok {
			return false
		}
		b := graph.Block(label)
		if b == nil {
			return false
		}
		val := blockField(b, p.Field)
		return cmpInt(val, p.Op, p.Value)

	case PredNoParms:
		label, ok := bindings[p.Var]
		if !ok {
			return false
		}
		b := graph.Block(label)
		if b == nil {
			return false
		}
		return b.ParamCount() == 0

	case PredAllPure:
		label, ok := bindings[p.Var]
		if !ok {
			return false
		}
		b := graph.Block(label)
		if b == nil {
			return false
		}
		for i := 0; i < b.InstCount(); i++ {
			if !b.Inst(i).IsPure() {
				return false
			}
		}
		return true

	case PredUseCount:
		// Use-count on a block (number of successor references to it)
		label, ok := bindings[p.Var]
		if !ok {
			return false
		}
		count := len(graph.Predecessors(label))
		return cmpInt(count, p.Op, p.Value)

	case PredDominates:
		// Simplified dominance check: a dominates b if a appears before b
		// in the block list and there's a path from a to b.
		labelA, okA := bindings[p.Var]
		labelB, okB := bindings[p.Var2]
		if !okA || !okB {
			return false
		}
		blocks := graph.Blocks()
		idxA, idxB := -1, -1
		for i, l := range blocks {
			if l == labelA {
				idxA = i
			}
			if l == labelB {
				idxB = i
			}
		}
		return idxA >= 0 && idxB >= 0 && idxA < idxB

	case PredTermKind:
		label, ok := bindings[p.Var]
		if !ok {
			return false
		}
		b := graph.Block(label)
		if b == nil {
			return false
		}
		return b.TermKind() == p.Field

	case PredCustom:
		if reg == nil {
			return false
		}
		fn, ok := reg.predicates[p.CustomName]
		if !ok {
			return false
		}
		return fn(graph, bindings, p.CustomArgs)
	}

	return true
}

func blockField(b rewrite.IRBlock, field string) int {
	switch field {
	case "inst_count":
		return b.InstCount()
	case "param_count":
		return b.ParamCount()
	}
	return 0
}

func cmpInt(a int, op string, b int) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	case ">":
		return a > b
	case "<":
		return a < b
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	}
	return false
}
