package mir2

// CallGraph records the inter-function call structure of a Module.
//
// It is the pre-requisite for interprocedural optimisations such as
// contract optimisation (Phase 5b), SoA256 page assignment (Phase 5c),
// and bank-switch layout (Phase 5d).
type CallGraph struct {
	// Edges maps callerName → list of (callee name, call count).
	// Call count is the static number of call sites in the caller body
	// (not dynamic frequency; use profiler data for hot-path weighting).
	Edges map[string][]CallEdge

	// Callers maps calleeName → list of callers.
	Callers map[string][]string

	// order is the topological order (leaves first), or nil if cyclic.
	order []string
	// acyclic is true when a full topo sort was possible.
	acyclic bool
}

// CallEdge is one directed call relationship.
type CallEdge struct {
	Callee string
	Count  int // number of call sites in the caller body
}

// BuildCallGraph scans every function in m and builds the call graph.
// Intrinsic calls (@mir.io.*) and indirect calls are not included.
func BuildCallGraph(m *Module) *CallGraph {
	cg := &CallGraph{
		Edges:   make(map[string][]CallEdge),
		Callers: make(map[string][]string),
	}

	// Ensure every function has an entry in Edges (even leaves).
	for _, f := range m.Funcs {
		if _, ok := cg.Edges[f.Name]; !ok {
			cg.Edges[f.Name] = nil
		}
	}

	// Scan all OpCall instructions.
	for _, f := range m.Funcs {
		counts := make(map[string]int)
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op != OpCall {
					continue
				}
				// Skip intrinsics and indirect targets.
				if inst.Sym == "" || inst.Sym[0] == '@' {
					continue
				}
				counts[inst.Sym]++
			}
		}
		for callee, n := range counts {
			cg.Edges[f.Name] = append(cg.Edges[f.Name], CallEdge{Callee: callee, Count: n})
			cg.Callers[callee] = append(cg.Callers[callee], f.Name)
		}
	}

	// Compute topological order (Kahn's algorithm).
	cg.order, cg.acyclic = topoSort(m, cg)
	return cg
}

// IsLeaf reports whether f has no outgoing call edges to other module functions.
func (cg *CallGraph) IsLeaf(name string) bool {
	return len(cg.Edges[name]) == 0
}

// Order returns a topologically sorted list of function names (leaves first).
// If the call graph is cyclic, Order returns a partial order (cycle members at end).
func (cg *CallGraph) Order() []string { return cg.order }

// Acyclic reports whether the call graph has no cycles (i.e. no recursion).
func (cg *CallGraph) Acyclic() bool { return cg.acyclic }

// CallCount returns the static number of call sites from caller to callee.
func (cg *CallGraph) CallCount(caller, callee string) int {
	for _, e := range cg.Edges[caller] {
		if e.Callee == callee {
			return e.Count
		}
	}
	return 0
}

// topoSort performs Kahn's topological sort on the REVERSE call graph,
// producing a "leaves first" order suitable for bottom-up DP.
//
// We want to process callees before their callers so that, when we optimise
// a caller's contract, callee contracts are already decided.
//
// In the REVERSE graph (callee→caller), in-degree of a node = number of its
// callees in the original graph (i.e. functions it calls).  Kahn's on this
// reverse graph gives leaves (no callees) first.
//
// Returns (order, true) if acyclic; (partial-order + cycle members, false) if cyclic.
func topoSort(m *Module, cg *CallGraph) ([]string, bool) {
	// inDeg[f] = number of distinct callees f has (= out-degree in call graph
	// = in-degree in reverse call graph).
	inDeg := make(map[string]int, len(m.Funcs))
	for _, f := range m.Funcs {
		inDeg[f.Name] = len(cg.Edges[f.Name]) // number of callee edges
	}

	// Start with leaves: functions that call nothing.
	var queue []string
	for _, f := range m.Funcs {
		if inDeg[f.Name] == 0 {
			queue = append(queue, f.Name)
		}
	}

	var order []string
	for len(queue) > 0 {
		// Stable: pick alphabetically smallest for deterministic output.
		best := 0
		for i, n := range queue {
			if n < queue[best] {
				best = i
			}
		}
		name := queue[best]
		queue = append(queue[:best], queue[best+1:]...)
		order = append(order, name)

		// name is now processed; reduce the in-degree of all its CALLERS
		// (in the reverse graph, processing a callee "releases" its callers).
		for _, caller := range cg.Callers[name] {
			inDeg[caller]--
			if inDeg[caller] == 0 {
				queue = append(queue, caller)
			}
		}
	}

	acyclic := len(order) == len(m.Funcs)
	if !acyclic {
		// Append remaining cycle members in declaration order.
		inOrder := make(map[string]bool, len(order))
		for _, n := range order {
			inOrder[n] = true
		}
		for _, f := range m.Funcs {
			if !inOrder[f.Name] {
				order = append(order, f.Name)
			}
		}
	}
	return order, acyclic
}
