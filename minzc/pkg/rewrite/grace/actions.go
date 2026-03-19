package grace

import "github.com/minz/minzc/pkg/rewrite"

// applyActions executes a sequence of rewrite actions.
func applyActions(graph rewrite.IRGraphMut, actions []Action, bindings BlockBindings, reg *PredicateRegistry) bool {
	applied := false
	for _, a := range actions {
		if applyAction(graph, a, bindings, reg) {
			applied = true
		}
	}
	return applied
}

func applyAction(graph rewrite.IRGraphMut, action Action, bindings BlockBindings, reg *PredicateRegistry) bool {
	switch action.Kind {
	case ActDelete:
		label, ok := bindings[action.Var]
		if !ok {
			return false
		}
		// Delete all instructions from the block
		b := graph.Block(label)
		if b == nil {
			return false
		}
		for b.InstCount() > 0 {
			graph.RemoveInst(label, 0)
		}
		return true

	case ActDeleteBlock:
		label, ok := bindings[action.Var]
		if !ok {
			return false
		}
		graph.RemoveBlock(label)
		return true

	case ActHoistInsts:
		srcLabel, ok1 := bindings[action.Var]
		dstLabel, ok2 := bindings[action.Var2]
		if !ok1 || !ok2 {
			return false
		}
		graph.HoistInsts(srcLabel, dstLabel)
		return true

	case ActReplaceTerm:
		label, ok := bindings[action.Var]
		if !ok {
			return false
		}
		// Resolve target variable references
		targets := make([]string, len(action.NewTargets))
		for i, t := range action.NewTargets {
			if resolved, ok := bindings[t]; ok {
				targets[i] = resolved
			} else {
				targets[i] = t
			}
		}
		graph.SetTerm(label, action.NewTermKind, targets)
		return true

	case ActRemoveParam:
		label, ok := bindings[action.Var]
		if !ok {
			return false
		}
		b := graph.Block(label)
		if b == nil || b.ParamCount() == 0 {
			return false
		}
		// Remove all dead params (params with 0 uses)
		// For now, remove all params — adapter can be smarter
		for b.ParamCount() > 0 {
			graph.RemoveBlockParam(label, 0)
		}
		return true

	case ActCustom:
		if reg == nil {
			return false
		}
		fn, ok := reg.actions[action.CustomName]
		if !ok {
			return false
		}
		return fn(graph, bindings)
	}

	return false
}
