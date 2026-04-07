package mir2

// PBQP-based interprocedural contract optimisation (PFCCO).
//
// This is an EXPERIMENTAL alternative to the greedy DP in OptimizeContracts.
// Each function is a PBQP node whose domain is the set of candidate
// ContractChoices (param/return class vectors).
//
// Node cost  = unary adapter cost inside the function body
//              (how much does the body pay if the param is in this class?)
//
// Edge cost  = adapter move cost at each call site between caller and callee
//              (how much does the caller pay if its convention is X and
//               the callee's convention is Y?)
//
// The PBQP solver (R0/R1/RN reduction) finds the globally cheapest assignment
// of conventions to all functions simultaneously, which the greedy DP can miss
// on diamond call graphs (A→C, B→C where A and B have conflicting preferences
// for C's convention).
//
// Pass-through chain fix: simple forwarders (single call, no ALU on params)
// get zero unary cost so edge costs drive the decision. This prevents
// unaryCostWithMod's bias toward original callee contracts from dominating.
//
// Remaining gap: multi-call functions (e.g. double_sum calling double twice)
// where unaryCostWithMod depends on original (pre-optimization) callee
// contracts via inferNaturalClass. PBQP picks a different convention than
// greedy in some cases. Fix: decouple unaryCost from original contracts.
//
// Status: NOT the default. Use OptimizeContracts() (greedy) for production.
//
// The solver reuses the existing candidateChoices / unaryCostWithMod / classMoveCost
// helpers from contracts.go.

// pfccoNode is a PBQP node representing one function's convention choice.
type pfccoNode struct {
	name    string
	choices []*ContractChoice // domain: candidate conventions
	costs   []int             // node cost per choice (unary body adapter cost)
}

// pfccoEdge is a PBQP edge between caller and callee.
type pfccoEdge struct {
	callerIdx int // index into pfccoNodes
	calleeIdx int // index into pfccoNodes
	// costMatrix[i][j] = adapter cost when caller picks choice i and callee picks choice j.
	// Dimensions: len(caller.choices) × len(callee.choices).
	costMatrix [][]int
	callCount  int // static number of call sites (weight multiplier)
}

// OptimizeContractsPBQP finds globally-optimal register classes for all
// function contracts using PBQP graph reduction on the call graph.
//
// Drop-in replacement for OptimizeContracts.
func OptimizeContractsPBQP(m *Module, ct CostTable) ContractSet {
	cg := BuildCallGraph(m)
	addrTaken := addrTakenFuncs(m)

	// Build PBQP nodes: one per function.
	nameToIdx := make(map[string]int, len(m.Funcs))
	nodes := make([]pfccoNode, 0, len(m.Funcs))

	for _, f := range m.Funcs {
		idx := len(nodes)
		nameToIdx[f.Name] = idx

		var choices []*ContractChoice
		var costs []int

		// Fixed-convention functions: extern, address-taken, asm — single choice.
		if f.Attrs.IsExtern || len(f.Blocks) == 0 || addrTaken[f.Name] || funcHasAsm(f) {
			ch := currentChoice(f)
			choices = []*ContractChoice{ch}
			costs = []int{0}
		} else {
			// Only vary param classes; keep return classes fixed.
			// ApplyContracts never overrides return classes, so varying
			// them in PBQP adds noise that skews param selection.
			choices = candidateParamOnlyChoices(f, ct)
			if len(choices) == 0 {
				choices = []*ContractChoice{currentChoice(f)}
			}
			costs = make([]int, len(choices))
			// For pass-through functions (param only used as call arg, no ALU),
			// unaryCostWithMod is biased toward the callee's *original* contract
			// class, not the optimized one. Zero out unary costs and let edge
			// costs drive the decision — they capture the real adapter cost.
			if isPassThrough(f) {
				for i := range costs {
					costs[i] = 0
				}
			} else {
				for i, ch := range choices {
					costs[i] = unaryCostWithMod(f, ch, ct, m)
				}
			}
		}

		nodes = append(nodes, pfccoNode{
			name:    f.Name,
			choices: choices,
			costs:   costs,
		})
	}

	// Build PBQP edges: one per call-graph edge.
	var edges []pfccoEdge
	for callerName, callEdges := range cg.Edges {
		callerIdx, ok := nameToIdx[callerName]
		if !ok {
			continue
		}
		caller := m.FuncByName(callerName)
		if caller == nil {
			continue
		}

		for _, ce := range callEdges {
			calleeIdx, ok := nameToIdx[ce.Callee]
			if !ok {
				continue
			}

			callerNode := &nodes[callerIdx]
			calleeNode := &nodes[calleeIdx]

			// Build cost matrix: callerChoice × calleeChoice → adapter cost.
			matrix := make([][]int, len(callerNode.choices))
			for ci, callerCh := range callerNode.choices {
				matrix[ci] = make([]int, len(calleeNode.choices))
				for cj, calleeCh := range calleeNode.choices {
					matrix[ci][cj] = pfccoEdgeCost(caller, callerCh, ce.Callee, calleeCh, ct)
				}
			}

			edges = append(edges, pfccoEdge{
				callerIdx:  callerIdx,
				calleeIdx:  calleeIdx,
				costMatrix: matrix,
				callCount:  ce.Count,
			})
		}
	}

	// PBQP solve: R0/R1/RN reduction.
	assignment := pfccoSolve(nodes, edges)

	// Build result.
	cs := make(ContractSet, len(m.Funcs))
	for i, node := range nodes {
		choiceIdx := assignment[i]
		if choiceIdx < 0 || choiceIdx >= len(node.choices) {
			cs[node.name] = currentChoice(m.FuncByName(node.name))
		} else {
			cs[node.name] = node.choices[choiceIdx]
		}
	}
	return cs
}

// isPassThrough reports whether f is a simple forwarder: single call,
// params only used as call args, no ALU/load/store on params.
// Functions with multiple calls or ALU ops on params are NOT pass-through
// because param class affects spill cost and register pressure.
func isPassThrough(f *Func) bool {
	paramRegs := make(map[Reg]bool, len(f.Contract.Params))
	for _, p := range f.Contract.Params {
		paramRegs[p.Reg] = true
	}
	callCount := 0
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpCall {
				callCount++
				if callCount > 1 {
					return false // multiple calls = spill pressure, unary cost matters
				}
				continue
			}
			for _, s := range inst.Src {
				if paramRegs[s] {
					return false
				}
			}
		}
	}
	return true
}

// candidateParamOnlyChoices generates candidate ContractChoices varying only
// param classes, keeping return classes fixed at their current values.
// This matches ApplyContracts behavior which never overrides return classes.
func candidateParamOnlyChoices(f *Func, ct CostTable) []*ContractChoice {
	paramCandidates := make([][]RegClass, len(f.Contract.Params))
	for i, p := range f.Contract.Params {
		paramCandidates[i] = plausibleClasses(p.Ty, p.Class)
	}

	// Fixed return classes.
	retClasses := make([]RegClass, len(f.Contract.Returns))
	for i, r := range f.Contract.Returns {
		retClasses[i] = r.Class
	}

	// Cartesian product of param candidates only.
	var results []*ContractChoice
	var recurse func(pi int, ch *ContractChoice)
	recurse = func(pi int, ch *ContractChoice) {
		if pi >= len(paramCandidates) {
			results = append(results, ch)
			return
		}
		for _, cls := range paramCandidates[pi] {
			next := ch.clone()
			next.ParamClasses[pi] = cls
			if choiceHasParamConflict(next.ParamClasses[:pi+1], ct) {
				continue
			}
			recurse(pi+1, next)
		}
	}
	base := &ContractChoice{
		ParamClasses:  make([]RegClass, len(paramCandidates)),
		ReturnClasses: make([]RegClass, len(retClasses)),
	}
	copy(base.ReturnClasses, retClasses)
	if len(paramCandidates) == 0 {
		return []*ContractChoice{base}
	}
	recurse(0, base)
	return results
}

// pfccoEdgeCost computes the adapter move cost at call sites from caller to
// callee calleeName, given specific convention choices for both.
func pfccoEdgeCost(caller *Func, callerCh *ContractChoice, calleeName string, calleeCh *ContractChoice, ct CostTable) int {
	total := 0
	ri := collectRegInfo(caller)

	// For caller's own params, use the candidate class.
	paramOverride := make(map[Reg]RegClass, len(caller.Contract.Params))
	for i, p := range caller.Contract.Params {
		if i < len(callerCh.ParamClasses) {
			paramOverride[p.Reg] = callerCh.ParamClasses[i]
		}
	}

	for _, b := range caller.Blocks {
		for _, inst := range b.Insts {
			if inst.Op != OpCall || inst.Sym != calleeName {
				continue
			}
			for ai, arg := range inst.Args {
				if ai >= len(calleeCh.ParamClasses) {
					break
				}
				var argClass RegClass
				if cls, isParam := paramOverride[arg]; isParam {
					argClass = cls
				} else if argInfo, haveInfo := ri[arg]; haveInfo {
					argClass = argInfo.Cls
				} else {
					argClass = ClassGeneral
				}
				wantClass := calleeCh.ParamClasses[ai]
				if argClass != wantClass {
					total += classMoveCost(argClass, wantClass, ct)
				}
			}
		}
	}
	return total
}

// pfccoSolve runs PBQP reduction on the PFCCO graph.
// Returns assignment[nodeIdx] = choiceIdx for each function.
func pfccoSolve(nodes []pfccoNode, edges []pfccoEdge) []int {
	n := len(nodes)
	if n == 0 {
		return nil
	}

	// Working cost vectors (modified during reduction).
	costs := make([][]int, n)
	for i := range nodes {
		costs[i] = make([]int, len(nodes[i].costs))
		copy(costs[i], nodes[i].costs)
	}

	// Build adjacency: for each node, list of edge indices.
	adj := make([][]int, n)
	for ei, e := range edges {
		adj[e.callerIdx] = append(adj[e.callerIdx], ei)
		adj[e.calleeIdx] = append(adj[e.calleeIdx], ei)
	}

	// Track removed nodes.
	removed := make([]bool, n)
	assignment := make([]int, n)
	for i := range assignment {
		assignment[i] = -1
	}

	// R1 stack for deferred back-assignment.
	type r1Entry struct {
		node     int
		neighbor int
		edgeIdx  int
	}
	var r1Stack []r1Entry

	degree := func(node int) int {
		d := 0
		for _, ei := range adj[node] {
			e := edges[ei]
			other := e.callerIdx
			if other == node {
				other = e.calleeIdx
			}
			if !removed[other] {
				d++
			}
		}
		return d
	}

	// R0/R1 reduction loop.
	changed := true
	for changed {
		changed = false
		for i := 0; i < n; i++ {
			if removed[i] {
				continue
			}
			deg := degree(i)

			if deg == 0 {
				// R0: assign cheapest.
				best := pfccoMinCost(costs[i])
				assignment[i] = best
				removed[i] = true
				changed = true
			} else if deg == 1 {
				// R1: fold into neighbor, defer assignment.
				var nbIdx, edgeIdx int
				for _, ei := range adj[i] {
					e := edges[ei]
					other := e.callerIdx
					if other == i {
						other = e.calleeIdx
					}
					if !removed[other] {
						nbIdx = other
						edgeIdx = ei
						break
					}
				}

				// Update neighbor's cost vector.
				e := edges[edgeIdx]
				isCaller := (e.callerIdx == i)
				nbCosts := costs[nbIdx]

				for j := range nbCosts {
					if nbCosts[j] >= InfCost {
						continue
					}
					// Find best cost for node i compatible with neighbor at j.
					bestI := InfCost
					for k, c := range costs[i] {
						if c >= InfCost {
							continue
						}
						var edgeCost int
						if isCaller {
							edgeCost = e.costMatrix[k][j] 						} else {
							edgeCost = e.costMatrix[j][k] 						}
						total := c + edgeCost
						if total < bestI {
							bestI = total
						}
					}
					if bestI < InfCost {
						nbCosts[j] += bestI
					}
				}

				r1Stack = append(r1Stack, r1Entry{node: i, neighbor: nbIdx, edgeIdx: edgeIdx})
				removed[i] = true
				changed = true
			}
		}
	}

	// RN: assign remaining nodes greedily (most constrained first).
	for {
		bestNode := -1
		bestMinCost := -1
		for i := 0; i < n; i++ {
			if removed[i] {
				continue
			}
			minC := pfccoMinCostVal(costs[i])
			if bestNode == -1 || minC > bestMinCost {
				bestNode = i
				bestMinCost = minC
			}
		}
		if bestNode == -1 {
			break
		}

		// Assign greedily: pick choice that minimizes node cost + edge costs to assigned neighbors.
		bestChoice := pfccoMinCost(costs[bestNode])
		bestTotal := InfCost

		for k, c := range costs[bestNode] {
			if c >= InfCost {
				continue
			}
			total := c
			for _, ei := range adj[bestNode] {
				e := edges[ei]
				other := e.callerIdx
				if other == bestNode {
					other = e.calleeIdx
				}
				if assignment[other] < 0 {
					continue // not yet assigned
				}
				isCaller := (e.callerIdx == bestNode)
				var ec int
				if isCaller {
					ec = e.costMatrix[k][assignment[other]] 				} else {
					ec = e.costMatrix[assignment[other]][k] 				}
				total += ec
			}
			if total < bestTotal {
				bestTotal = total
				bestChoice = k
			}
		}

		assignment[bestNode] = bestChoice
		removed[bestNode] = true
	}

	// Back-assign R1 stack in reverse order.
	for i := len(r1Stack) - 1; i >= 0; i-- {
		entry := r1Stack[i]
		e := edges[entry.edgeIdx]
		isCaller := (e.callerIdx == entry.node)
		nbChoice := assignment[entry.neighbor]

		bestChoice := 0
		bestCost := InfCost
		for k, c := range nodes[entry.node].costs {
			if c >= InfCost {
				continue
			}
			var ec int
			if isCaller {
				ec = e.costMatrix[k][nbChoice] 			} else {
				ec = e.costMatrix[nbChoice][k] 			}
			total := c + ec
			if total < bestCost {
				bestCost = total
				bestChoice = k
			}
		}
		assignment[entry.node] = bestChoice
	}

	return assignment
}

// pfccoMinCost returns the index of the minimum cost in a cost vector.
func pfccoMinCost(costs []int) int {
	best := 0
	for i, c := range costs {
		if c < costs[best] {
			best = i
		}
	}
	return best
}

// pfccoMinCostVal returns the minimum cost value in a cost vector.
func pfccoMinCostVal(costs []int) int {
	min := InfCost
	for _, c := range costs {
		if c < min {
			min = c
		}
	}
	return min
}
