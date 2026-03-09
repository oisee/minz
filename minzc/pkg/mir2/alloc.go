package mir2

import "slices"

// Register allocator for MIR2.
//
// Pipeline:
//
//	collectRegInfo          → map[Reg]{Ty, RegClass} for every virtual reg
//	BuildInterferenceGraph  → which virtuals cannot share a physical location
//	Allocate                → greedy graph-colouring using Z80CostTable
//
// The allocator is a simple greedy pass:
//  1. Sort virtual regs by (interference degree ↓, class tier ↑) so the most
//     constrained regs are coloured first.
//  2. For each reg, find all physical locations NOT used by its neighbours.
//  3. Among remaining locations, pick the minimum-cost one for this reg's class.
//  4. If nothing fits, spill to a fresh LocMem slot ($F0xx).
//
// The result is an AllocResult: map[Reg]PhysLoc covering every virtual reg in f.

// ── RegInfo ───────────────────────────────────────────────────────────────────

// RegInfo records the type and preferred class of a virtual register.
type RegInfo struct {
	Ty  Ty
	Cls RegClass
}

// collectRegInfo walks the function and builds a RegInfo map for every def.
// Sources that appear without a definition (shouldn't happen in well-formed MIR2)
// get a fallback entry.
func collectRegInfo(f *Func) map[Reg]RegInfo {
	m := make(map[Reg]RegInfo)

	add := func(r Reg, ty Ty, cls RegClass) {
		if r != NoReg {
			m[r] = RegInfo{Ty: ty, Cls: cls}
		}
	}

	// Function contract params.
	for _, p := range f.Contract.Params {
		add(p.Reg, p.Ty, p.Class)
	}

	for _, b := range f.Blocks {
		// Block params.
		for _, p := range b.Params {
			add(p.Dst, p.Ty, p.Class)
		}
		// Instructions.
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				add(inst.Dst, inst.Ty, inst.Cls)
			}
		}
	}
	return m
}

// ── InterferenceGraph ─────────────────────────────────────────────────────────

// InterferenceGraph records, for each virtual register, the set of registers
// it cannot share a physical location with.
type InterferenceGraph struct {
	adj map[Reg]*RegSet
}

func newIGraph() *InterferenceGraph {
	return &InterferenceGraph{adj: make(map[Reg]*RegSet)}
}

func (g *InterferenceGraph) ensure(r Reg) *RegSet {
	if s, ok := g.adj[r]; ok {
		return s
	}
	s := newRegSet()
	g.adj[r] = s
	return s
}

func (g *InterferenceGraph) addEdge(a, b Reg) {
	if a == NoReg || b == NoReg || a == b {
		return
	}
	g.ensure(a).Add(b)
	g.ensure(b).Add(a)
}

// Neighbors returns the set of registers interfering with r.
func (g *InterferenceGraph) Neighbors(r Reg) *RegSet {
	if s, ok := g.adj[r]; ok {
		return s
	}
	return newRegSet()
}

// Degree returns the number of registers interfering with r.
func (g *InterferenceGraph) Degree(r Reg) int {
	return g.Neighbors(r).Len()
}

// BuildInterferenceGraph constructs the interference graph for function f.
//
// Two regs interfere if they are simultaneously live at any program point.
// The graph is built by a backward pass within each block:
//
//	live = LiveOut[b]
//	add terminator operands to live   (they're used at block exit)
//	for each instruction (last→first):
//	  Dst interferes with all currently live regs
//	  remove Dst from live; add Src/Args to live
//	block params interfere with each other and with live at block entry
func BuildInterferenceGraph(f *Func, lr *LivenessResult) *InterferenceGraph {
	g := newIGraph()

	// Ensure every def has a node (even if degree 0).
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			g.ensure(p.Dst)
		}
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				g.ensure(inst.Dst)
			}
		}
	}
	for _, p := range f.Contract.Params {
		g.ensure(p.Reg)
	}

	for bi, b := range f.Blocks {
		// Start with registers live at block exit.
		live := lr.LiveOut[bi].Clone()

		// Add terminator operands: they're used at the very end of the block
		// and may not appear in LiveOut (e.g. function params consumed at jmp).
		addTermUses(live, b.Term)

		// Walk instructions backward.
		for ii := len(b.Insts) - 1; ii >= 0; ii-- {
			inst := b.Insts[ii]

			if inst.Dst != NoReg {
				// Dst is being born here: it interferes with everything alive.
				live.Each(func(r Reg) { g.addEdge(inst.Dst, r) })
				live.Remove(inst.Dst)
			}

			// Sources are now needed (become live going backward).
			for _, src := range inst.Src {
				if src != NoReg {
					live.Add(src)
				}
			}
			for _, arg := range inst.Args {
				if arg != NoReg {
					live.Add(arg)
				}
			}
			if inst.Asm != nil {
				for _, r := range inst.Asm.Ins {
					if r != NoReg {
						live.Add(r)
					}
				}
			}
		}

		// Block params are defined simultaneously at block entry.
		// They interfere with each other and with everything live entering the block.
		for i, p := range b.Params {
			live.Each(func(r Reg) { g.addEdge(p.Dst, r) })
			for j, q := range b.Params {
				if i != j {
					g.addEdge(p.Dst, q.Dst)
				}
			}
		}

		// Function params (entry block): all defined simultaneously.
		if bi == 0 {
			for i, p := range f.Contract.Params {
				live.Each(func(r Reg) {
					if r != p.Reg {
						g.addEdge(p.Reg, r)
					}
				})
				for j, q := range f.Contract.Params {
					if i != j {
						g.addEdge(p.Reg, q.Reg)
					}
				}
			}
		}
	}

	return g
}

func addTermUses(live *RegSet, t Term) {
	switch t := t.(type) {
	case *TermJmp:
		for _, a := range t.Args {
			if a != NoReg {
				live.Add(a)
			}
		}
	case *TermBrIf:
		if t.Cond != NoReg {
			live.Add(t.Cond)
		}
		for _, a := range t.ThenArgs {
			if a != NoReg {
				live.Add(a)
			}
		}
		for _, a := range t.ElseArgs {
			if a != NoReg {
				live.Add(a)
			}
		}
	case *TermBrIf2:
		if t.Lhs != NoReg {
			live.Add(t.Lhs)
		}
		if t.Rhs != NoReg {
			live.Add(t.Rhs)
		}
		for _, a := range t.EqArgs {
			if a != NoReg {
				live.Add(a)
			}
		}
		for _, a := range t.LtArgs {
			if a != NoReg {
				live.Add(a)
			}
		}
		for _, a := range t.GtArgs {
			if a != NoReg {
				live.Add(a)
			}
		}
	case *TermDJNZ:
		if t.Counter != NoReg {
			live.Add(t.Counter)
		}
		for _, a := range t.BodyArgs {
			if a != NoReg {
				live.Add(a)
			}
		}
		for _, a := range t.ExitArgs {
			if a != NoReg {
				live.Add(a)
			}
		}
	case *TermRet:
		for _, v := range t.Vals {
			if v != NoReg {
				live.Add(v)
			}
		}
	}
}

// ── AllocResult ───────────────────────────────────────────────────────────────

// AllocResult maps every virtual register to a physical location.
type AllocResult struct {
	Locs    map[Reg]PhysLoc
	Spilled []Reg // registers that ended up in LocMem (spilled)
}

// Loc returns the physical location assigned to r (zero PhysLoc if not found).
func (a *AllocResult) Loc(r Reg) PhysLoc {
	return a.Locs[r]
}

// ── Greedy allocator ──────────────────────────────────────────────────────────

// Allocate runs greedy graph-colouring allocation on function f.
//
// Order: regs sorted by (interference degree ↓, class tier ↑).
// Selection: minimum-cost compatible location not used by any neighbour.
// Spill: LocMem slot when all compatible locations are taken.
func Allocate(f *Func, lr *LivenessResult, ct CostTable) *AllocResult {
	info := collectRegInfo(f)
	ig := BuildInterferenceGraph(f, lr)
	allLocs := ct.Locs()

	result := &AllocResult{Locs: make(map[Reg]PhysLoc)}

	// Copy coalescing: collect OpMove pairs (dst → src).
	// When allocating dst, we try to reuse src's PhysLoc to turn the move
	// into a no-op (codegen already skips OpMove when dst.loc == src.loc).
	coalesceHint := make(map[Reg]Reg)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpMove && inst.Src[0] != NoReg && inst.Dst != NoReg {
				coalesceHint[inst.Dst] = inst.Src[0]
			}
		}
	}

	// Collect all virtual regs to allocate.
	regs := make([]Reg, 0, len(info))
	for r := range info {
		regs = append(regs, r)
	}

	// Sort: highest interference degree first; ties broken by class tier (lowest = cheapest = first);
	// final tiebreaker by Reg number so allocation order is fully deterministic across runs.
	slices.SortFunc(regs, func(a, b Reg) int {
		da, db := ig.Degree(a), ig.Degree(b)
		if da != db {
			return db - da // descending degree
		}
		ta := info[a].Cls.Tier()
		tb := info[b].Cls.Tier()
		if ta != tb {
			return ta - tb // ascending tier (cheaper classes first)
		}
		return int(a) - int(b) // ascending Reg number — deterministic tiebreak
	})

	nextMemSlot := 0 // $F000, $F001, $F002, … (byte addresses within $F0xx page)

	for _, r := range regs {
		ri := info[r]

		// Build set of locations already used by interfering neighbours.
		usedByNeighbors := make(map[PhysLoc]bool)
		ig.Neighbors(r).Each(func(n Reg) {
			if loc, ok := result.Locs[n]; ok {
				usedByNeighbors[loc] = true
			}
		})

		// Copy coalescing: if r is the dst of OpMove(src), and src is already
		// allocated to a location compatible with r's class and not blocked by
		// neighbours, reuse that location — the OpMove becomes a no-op.
		if hintSrc, ok := coalesceHint[r]; ok {
			if srcLoc, allocated := result.Locs[hintSrc]; allocated {
				if !usedByNeighbors[srcLoc] &&
					locCompatible(ri.Ty, srcLoc) &&
					ct.Cost(ri.Cls, srcLoc) < InfCost {
					result.Locs[r] = srcLoc
					continue
				}
			}
		}

		// Find minimum-cost compatible available location.
		best := PhysLoc{}
		bestCost := InfCost + 1

		for _, loc := range allLocs {
			if usedByNeighbors[loc] {
				continue
			}
			if !locCompatible(ri.Ty, loc) {
				continue
			}
			c := ct.Cost(ri.Cls, loc)
			if c >= InfCost {
				continue // semantically impossible for this class
			}
			if c < bestCost {
				bestCost = c
				best = loc
			}
		}

		if bestCost > InfCost {
			// All compatible locations taken: spill to a fresh $F0xx memory slot.
			best = PhysLoc{
				Kind:   LocMem,
				Name:   "mem",
				Offset: 0xF000 + nextMemSlot,
			}
			nextMemSlot += (ri.Ty.Width() + 7) / 8
			result.Spilled = append(result.Spilled, r)
		}

		result.Locs[r] = best
	}

	return result
}

// ── Test exports ─────────────────────────────────────────────────────────────

// CollectRegInfoForTest exports collectRegInfo for external test packages.
func CollectRegInfoForTest(f *Func) map[Reg]RegInfo { return collectRegInfo(f) }

// IGAdjForTest exposes the adjacency map of the interference graph for tests.
func IGAdjForTest(g *InterferenceGraph) map[Reg]*RegSet { return g.adj }

// locCompatible reports whether physical location loc can hold a value of type ty.
//
// Rules for Z80:
//   - LocFlag:  only 1-bit (bool) values
//   - LocReg 1-char (A/B/C/D/E/H/L): 8-bit values only
//   - LocReg 2-char (HL/DE/BC): 16-bit values only
//   - LocIXY (IX/IY): 16-bit values
//   - LocIXY8 (IXH/IXL/…): 8-bit values
//   - LocShadow (A'/B'/…): 8-bit values
//   - LocStack: ≤16-bit (PUSH/POP is 16-bit on Z80)
//   - LocMem: any width (backend chooses LD r,(nn) vs LD rr,(nn))
func locCompatible(ty Ty, loc PhysLoc) bool {
	w := ty.Width()
	switch loc.Kind {
	case LocFlag:
		return w == 1
	case LocMem:
		return true
	case LocStack:
		return w <= 16
	case LocReg:
		if len(loc.Name) == 1 {
			return w <= 8
		}
		return w == 16
	case LocIXY:
		return w == 16
	case LocIXY8:
		return w <= 8
	case LocShadow:
		return w <= 8
	}
	return false
}
