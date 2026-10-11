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
// The PBQP solver uses exact R0/R1 reductions and a greedy RN residual
// assignment. It considers conventions jointly but does not guarantee a global
// optimum. Joint choices can improve on the greedy DP
// on diamond call graphs (A→C, B→C where A and B have conflicting preferences
// for C's convention).
//
// Call demands belong to edges; node costs infer only non-call body demands.
// Z80 A/B/C transfers use instruction costs rather than placement constraints.
// Static first-use costs still omit call-save pressure and execution frequency;
// even exact forest reduction does not establish a native-code optimum.
//
// Status: NOT the default. OptimizeContracts remains unchanged.

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

// OptimizeContractsPBQP selects register classes jointly using PBQP reduction.
// The residual graph is solved heuristically; optimality is not guaranteed.
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
			for i, ch := range choices {
				costs[i] = pfccoBodyUnaryCost(f, ch, ct)
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

	// Reject the whole proposed set if reduction did not produce a feasible
	// assignment. Per-function fallback could retain incompatible neighbors.
	if assignment == nil && len(nodes) != 0 {
		cs := make(ContractSet, len(m.Funcs))
		for _, f := range m.Funcs {
			cs[f.Name] = currentChoice(f)
		}
		return cs
	}

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

	// Forward the candidate convention through SSA copies. Otherwise a call-only
	// move would reintroduce the original class at the edge after unary removal.
	for changed := true; changed; {
		changed = false
		for _, b := range caller.Blocks {
			for _, inst := range b.Insts {
				if inst.Op != OpMove || inst.Src[0] == NoReg {
					continue
				}
				if cls, ok := paramOverride[inst.Src[0]]; ok {
					if _, known := paramOverride[inst.Dst]; !known {
						paramOverride[inst.Dst] = cls
						changed = true
					}
				}
			}
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
					total += pfccoAdapterCost(argClass, wantClass, ct)
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
							edgeCost = e.costMatrix[k][j]
						} else {
							edgeCost = e.costMatrix[j][k]
						}
						total := c + edgeCost
						if total < bestI {
							bestI = total
						}
					}
					if bestI < InfCost {
						nbCosts[j] += bestI
					} else {
						// No eliminated-node choice can satisfy this neighbor.
						nbCosts[j] = InfCost
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
					ec = e.costMatrix[k][assignment[other]]
				} else {
					ec = e.costMatrix[assignment[other]][k]
				}
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
		// Reduction has accumulated costs from eliminated neighbors here.
		// Reconstruct using that vector, not the original unary costs.
		for k, c := range costs[entry.node] {
			if c >= InfCost {
				continue
			}
			var ec int
			if isCaller {
				ec = e.costMatrix[k][nbChoice]
			} else {
				ec = e.costMatrix[nbChoice][k]
			}
			total := c + ec
			if total < bestCost {
				bestCost = total
				bestChoice = k
			}
		}
		assignment[entry.node] = bestChoice
	}

	// Check the original graph independently of the reduction state. RN can
	// dead-end, and an all-infinite vector has no admissible minimum.
	for i, k := range assignment {
		if k < 0 || k >= len(nodes[i].costs) || nodes[i].costs[k] >= InfCost {
			return nil
		}
	}
	for _, e := range edges {
		if e.costMatrix[assignment[e.callerIdx]][assignment[e.calleeIdx]] >= InfCost {
			return nil
		}
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

// Call-argument adapters belong to graph edges, including fixed extern callees.
// Removing calls from demand inference prevents preoptimization callee classes
// from appearing in both node and edge costs. Fixed return inference remains.
func pfccoBodyUnaryCost(f *Func, choice *ContractChoice, ct CostTable) int {
	body := *f
	body.Blocks = make([]*Block, len(f.Blocks))
	for i, b := range f.Blocks {
		copyBlock := *b
		copyBlock.Insts = nil
		for _, inst := range b.Insts {
			if inst.Op != OpCall && !(inst.Op == OpMove && !pfccoHasBodyUse(f, inst.Dst, map[Reg]bool{})) {
				copyBlock.Insts = append(copyBlock.Insts, inst)
			}
		}
		body.Blocks[i] = &copyBlock
	}
	total := 0
	for i, p := range f.Contract.Params {
		if i >= len(choice.ParamClasses) {
			break
		}
		// No non-call use means there is no body adapter; graph edges own all
		// demand. Move chains are retained so ALU/return use is still inferred.
		if !pfccoHasBodyUse(&body, p.Reg, map[Reg]bool{}) {
			continue
		}
		natural := inferNaturalClass(&body, p.Reg, nil)
		total += pfccoAdapterCost(choice.ParamClasses[i], natural, ct)
	}
	for i, r := range f.Contract.Returns {
		if i < len(choice.ReturnClasses) {
			total += classMoveCost(inferNaturalRetClass(f, r), choice.ReturnClasses[i], ct) * countRetSites(f)
		}
	}
	return total
}
func pfccoHasBodyUse(f *Func, r Reg, seen map[Reg]bool) bool {
	if seen[r] {
		return false
	}
	seen[r] = true
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			for _, s := range i.Src {
				if s == r {
					if i.Op == OpMove {
						if pfccoHasBodyUse(f, i.Dst, seen) {
							return true
						}
					} else {
						return true
					}
				}
			}
		}
		if b.Term != nil {
			for _, s := range b.Term.termUses() {
				if s == r {
					return true
				}
			}
		}
	}
	return false
}

// A hard A/B/C register placement does not prohibit a transfer into it. The
// existing placement table is retained for all other class pairs; its width,
// flags and index constraints must not be relaxed by this byte-only repair.
func pfccoAdapterCost(from, to RegClass, ct CostTable) int {
	cost := classMoveCost(from, to, ct)
	if _, ok := ct.(Z80CostTable); !ok {
		return cost
	}
	src, dst := bestLocForClass(from, ct), bestLocForClass(to, ct)
	isByte := func(l PhysLoc) bool {
		return l.Kind == LocReg && len(l.Name) == 1 && (l.Name == "A" || l.Name == "B" || l.Name == "C")
	}
	if !isByte(src) || !isByte(dst) {
		return cost
	}
	if src == dst {
		return 0
	}
	return 4 // LD r,r: one byte, four T-states, no flags modified.
}
