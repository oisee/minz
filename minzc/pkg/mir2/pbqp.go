package mir2

// PBQP (Partitioned Boolean Quadratic Program) register allocator.
//
// Replaces the greedy graph-colouring allocator for functions where
// weighted cost vectors produce a better assignment than simple class cost.
//
// Model
// ─────
//
//   Variables : each virtual register r has a set of candidate physical
//               locations (locs where locCompatible(r.Ty, loc) && ct.Cost(r.Cls,loc) < InfCost).
//
//   Node cost : nodeCost[r][i] = useCount[r] × ct.Cost(r.Cls, locs[i])
//               Weighted by use count: a reg used 5× pays 5× the slot cost.
//
//   Edge cost : for interfering pairs (r1,r2), assigning both to the same
//               physical location has cost ∞ (ColourConflict). Otherwise 0.
//
//   Objective : minimise  Σ_r nodeCost[r][colour[r]]
//                       + Σ_(r1,r2)∈E edgeCost[r1][r2][colour[r1]][colour[r2]]
//
// Reduction rules (applied iteratively until none apply)
// ───────────────────────────────────────────────────────
//
//   R0  Degree-0 node: assign cheapest candidate location immediately.
//
//   R1  Degree-1 node: for the (node, neighbour) pair, compute all
//       |cands(node)| × |cands(neighbour)| joint costs and pick the
//       globally-optimal pair (considering the neighbour's current
//       reduced cost vector, updated by folding in the edge).
//       The node is removed; the neighbour's cost vector is updated.
//
//   RN  Degree ≥ 2: use priority heuristic — choose the node whose
//       minimum candidate cost is highest (most constrained), assign it
//       greedily (min cost compatible with already-assigned neighbours),
//       and continue.  This is the same as the old greedy allocator but
//       operates on the weighted cost vectors, so it still improves over
//       the old purely-static ordering.
//
// The R0/R1 passes run to fixpoint before RN takes over.  For typical
// Z80 functions (≤15 live regs), the interference graph is a tree or
// near-tree, so R0/R1 alone solve ≥90% of nodes optimally.

import "slices"

// regState holds the per-virtual-register data used by the PBQP solver:
// the register's class/type info and a mutable cost vector indexed by allLocs.
type regState struct {
	info  RegInfo
	costs []int // cost per locs index; InfCost = excluded
}

// ── Cost vector ───────────────────────────────────────────────────────────────

// nodeCosts returns a map[Reg][]int: for each virtual reg, a slice of costs
// indexed by position in locs.  Cost = useCount[r] × ct.Cost(r.Cls, locs[i]).
// Locations that are incompatible or have InfCost get cost InfCost.
func nodeCosts(f *Func, info map[Reg]RegInfo, ct CostTable, locs []PhysLoc) map[Reg][]int {
	// Count how many times each reg is used (defs + uses in instructions + terms).
	useCount := make(map[Reg]int)

	for _, p := range f.Contract.Params {
		useCount[p.Reg]++ // param = one definition
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			useCount[p.Dst]++ // block param = one def
		}
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				useCount[inst.Dst]++ // definition
			}
			for _, s := range inst.Src {
				if s != NoReg {
					useCount[s]++ // use
				}
			}
			for _, a := range inst.Args {
				if a != NoReg {
					useCount[a]++ // call arg use
				}
			}
		}
		if b.Term != nil {
			for _, r := range b.Term.termUses() {
				if r != NoReg {
					useCount[r]++
				}
			}
		}
	}

	costs := make(map[Reg][]int, len(info))
	for r, ri := range info {
		uc := useCount[r]
		if uc == 0 {
			uc = 1 // always pay at least 1× (the definition itself)
		}
		cv := make([]int, len(locs))
		for i, loc := range locs {
			if !locCompatible(ri.Ty, loc) {
				cv[i] = InfCost
				continue
			}
			c := ct.Cost(ri.Cls, loc)
			if c >= InfCost {
				cv[i] = InfCost
				continue
			}
			cv[i] = uc * c
		}
		costs[r] = cv
	}
	return costs
}

// ── PBQP solver ───────────────────────────────────────────────────────────────

// PBQPAllocate runs the PBQP allocator on function f.
// It is a drop-in replacement for Allocate.
func PBQPAllocate(f *Func, lr *LivenessResult, ct CostTable) *AllocResult {
	info := collectRegInfo(f)

	// Pre-PBQP: propagate class hints through phi-web edges and return values.
	// This ensures ALU op results (ClassAcc) flow to block params that feed
	// returns, so PBQP assigns them to A without spurious LD C,A copies.
	propagateClassHints(f, info)

	ig := BuildInterferenceGraph(f, lr)
	allLocs := ct.Locs()

	// Collect candidates for each virtual reg.
	states := make(map[Reg]*regState, len(info))
	for r, ri := range info {
		states[r] = &regState{info: ri}
	}

	// Build weighted cost vectors.
	raw := nodeCosts(f, info, ct, allLocs)
	for r, cv := range raw {
		states[r].costs = cv
	}

	// Phase 6e: PBQP affinity nudges — applied before solver to bias allocation.
	applyLUTAffinityNudge(f, states, allLocs)          // LUT index → C/E (BC★/DE★ 14T path)
	applyMul16DEAffinityNudge(f, states, allLocs)      // mul16 rhs → DE (skip LD D/E setup, 8T)
	applyDJNZCounterAffinityNudge(f, states, allLocs)  // DJNZ counter → B (skip LD B,r, 4T)

	result := &AllocResult{Locs: make(map[Reg]PhysLoc)}

	// assigned[r] = index into allLocs, or -1 if not yet assigned.
	assigned := make(map[Reg]int, len(info))
	for r := range info {
		assigned[r] = -1
	}

	// Stack for R1 deferred assignments (processed in reverse after reduction).
	type r1Entry struct {
		node     Reg
		neighbor Reg
	}
	var r1Stack []r1Entry

	// ── R0/R1 reduction passes ────────────────────────────────────────────────

	changed := true
	for changed {
		changed = false
		for r := range states {
			if assigned[r] >= 0 {
				continue
			}
			deg := 0
			var theNeighbor Reg
			ig.Neighbors(r).Each(func(n Reg) {
				if assigned[n] < 0 {
					deg++
					theNeighbor = n
				}
			})

			switch deg {
			case 0: // R0: assign immediately to minimum-cost candidate.
				best := pbqpMinIdx(states[r].costs)
				if best < 0 {
					best = pbqpSpillIdx(allLocs)
				}
				assigned[r] = best
				result.Locs[r] = allLocs[best]
				changed = true

			case 1: // R1: fold edge into neighbor, defer assignment.
				nb := theNeighbor
				// Update neighbor's cost vector to account for the joint cost
				// of the (r, nb) pair.  For each nb candidate j, find the
				// optimal r candidate i that doesn't conflict; add that cost.
				nbCV := states[nb].costs
				rCV := states[r].costs
				for j := range nbCV {
					if nbCV[j] >= InfCost {
						continue
					}
					// Best r cost compatible with nb at j.
					best := InfCost
					for i, c := range rCV {
						if c >= InfCost {
							continue
						}
						// Conflict: r and nb overlap (same or aliased loc) → skip.
						if physicallyConflicts(allLocs[i], allLocs[j]) {
							continue
						}
						if c < best {
							best = c
						}
					}
					if best < InfCost {
						nbCV[j] += best
					} else {
						nbCV[j] = InfCost
					}
				}
				// Remove r from graph temporarily.
				r1Stack = append(r1Stack, r1Entry{node: r, neighbor: nb})
				assigned[r] = -2 // sentinel: pending R1 back-assignment
				ig.removeNode(r)
				changed = true
			}
		}
	}

	// ── RN: greedy for remaining high-degree nodes ────────────────────────────

	remaining := make([]Reg, 0)
	for r := range states {
		if assigned[r] < 0 {
			remaining = append(remaining, r)
		}
	}
	// Sort by descending cost-delta: (2nd_best - best).  The reg that pays the
	// highest penalty for missing its first-choice location is allocated first.
	// This is equivalent to "most to lose" — put it in the best slot first.
	// Ties broken by descending minimum cost, then by reg number.
	slices.SortFunc(remaining, func(a, b Reg) int {
		da := pbqpDelta(states[a].costs)
		db := pbqpDelta(states[b].costs)
		if da != db {
			return db - da // descending delta
		}
		ca := pbqpMinCost(states[a].costs)
		cb := pbqpMinCost(states[b].costs)
		if ca != cb {
			return cb - ca // descending min cost
		}
		return int(a) - int(b)
	})

	nextMem := 0
	for _, r := range remaining {
		rs := states[r]
		// Collect locs blocked by already-assigned neighbours (including aliases).
		blocked := make(map[int]bool)
		ig.Neighbors(r).Each(func(n Reg) {
			if idx := assigned[n]; idx >= 0 {
				blocked[idx] = true
				// Also block sub-register aliases (e.g. if neighbour is in DE,
				// block D and E; if neighbour is in D, block DE).
				for _, alias := range physicalAliases(allLocs[idx]) {
					for j, l := range allLocs {
						if l == alias {
							blocked[j] = true
							break
						}
					}
				}
			}
		})
		best := -1
		bestCost := InfCost + 1
		for i, c := range rs.costs {
			if blocked[i] || c >= InfCost {
				continue
			}
			if c < bestCost {
				bestCost = c
				best = i
			}
		}
		if best < 0 {
			// Spill.
			loc := PhysLoc{Kind: LocMem, Name: "mem", Offset: 0xF000 + nextMem}
			nextMem += (rs.info.Ty.Width() + 7) / 8
			result.Locs[r] = loc
			result.Spilled = append(result.Spilled, r)
			assigned[r] = pbqpSpillIdx(allLocs) // fake index for conflict avoidance
		} else {
			assigned[r] = best
			result.Locs[r] = allLocs[best]
		}
	}

	// ── R1 back-assignment (reverse stack order) ──────────────────────────────
	// Also reset assigned sentinel so coalescing sees correct state.
	for i := len(r1Stack) - 1; i >= 0; i-- {
		e := r1Stack[i]
		r := e.node
		nbIdx := assigned[e.neighbor]

		// Assign r to the minimum-cost location that doesn't conflict with nb.
		rCV := states[r].costs
		best := -1
		bestCost := InfCost + 1
		for i, c := range rCV {
			if c >= InfCost {
				continue
			}
			if nbIdx >= 0 && physicallyConflicts(allLocs[i], allLocs[nbIdx]) {
				continue // conflict with neighbour
			}
			if c < bestCost {
				bestCost = c
				best = i
			}
		}
		if best < 0 {
			loc := PhysLoc{Kind: LocMem, Name: "mem", Offset: 0xF000 + nextMem}
			nextMem += (states[r].info.Ty.Width() + 7) / 8
			result.Locs[r] = loc
			result.Spilled = append(result.Spilled, r)
		} else {
			assigned[r] = best
			result.Locs[r] = allLocs[best]
		}
	}

	// ── Phase 6c: post-allocation copy coalescing ─────────────────────────────
	// Eliminate parallel copies at block boundaries and within-block OpMoves
	// by reassigning block params / OpMove dsts to their source's PhysLoc
	// when no IG neighbour conflict prevents it.
	coalesceAllocResult(f, result, lr)

	return result
}

// removeNode removes r from the interference graph (for R1 reduction).
func (g *InterferenceGraph) removeNode(r Reg) {
	if neighbors, ok := g.adj[r]; ok {
		neighbors.Each(func(n Reg) {
			if ns, ok := g.adj[n]; ok {
				ns.Remove(r)
			}
		})
		delete(g.adj, r)
	}
}

// pbqpMinIdx returns the index of the minimum finite cost in cv, or -1 if all
// are InfCost.
func pbqpMinIdx(cv []int) int {
	best, bestCost := -1, InfCost
	for i, c := range cv {
		if c < bestCost {
			bestCost = c
			best = i
		}
	}
	return best
}

// pbqpMinCost returns the minimum finite cost in cv, or InfCost if all are ∞.
func pbqpMinCost(cv []int) int {
	best := InfCost
	for _, c := range cv {
		if c < best {
			best = c
		}
	}
	return best
}

// pbqpDelta returns the difference between the second-best and best finite
// cost in cv.  A large delta means the reg pays a high penalty if it misses
// its first-choice location — it should be allocated first.
// Returns 0 if cv has fewer than two finite entries.
func pbqpDelta(cv []int) int {
	best, second := InfCost, InfCost
	for _, c := range cv {
		if c < best {
			second = best
			best = c
		} else if c < second {
			second = c
		}
	}
	if second >= InfCost {
		return 0
	}
	return second - best
}

// physicallyConflicts reports whether two physical locations alias each other.
// Two locations conflict when one is a sub-register of the other
// (e.g. D and DE both alias because D is the high byte of DE).
// Uses physicalAliases from alloc.go for the sub-register mapping.
func physicallyConflicts(a, b PhysLoc) bool {
	if a == b {
		return true
	}
	for _, alias := range physicalAliases(a) {
		if alias == b {
			return true
		}
	}
	return false
}

// pbqpSpillIdx returns the index of the first LocMem location in locs, or 0.
func pbqpSpillIdx(locs []PhysLoc) int {
	for i, l := range locs {
		if l.Kind == LocMem {
			return i
		}
	}
	return 0
}
