// cfgsolver.go — CFG-aware Z3 solver for whole-function optimization.
//
// Encodes ALL blocks of a function with separate per-instruction variables,
// connected by CFG edge constraints. This correctly handles:
// - Conditional branches (then/else paths have independent variables)
// - Loop back-edges (loop head must match loop body exit)
// - Cross-block register state (vreg locations flow along CFG edges)
//
// Encoding:
//   lv{vreg}_b{block}_i{inst} — location of vreg at instruction i in block b
//   Edge A→B: lv{vreg}_bA_i{last} = lv{vreg}_bB_i{0} (for all live vregs)
package vir

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/mir2"
)

// CFGSolution holds the result of a CFG-aware Z3 solve.
type CFGSolution struct {
	BlockPIR  map[string][]PIROp
	ParamLocs map[int]int // vreg → physical register index (from Z3 model)
}

// parallelMove represents a pending register-to-register move before ordering.
type parallelMove struct {
	vreg    int
	prevLoc int // source physical register
	currLoc int // destination physical register
	movePat *Pattern
	remSrc  int // remapped source (for pair→half patterns)
}

// locsOverlap returns true if writing to loc w would clobber a read from loc r.
// Handles 16-bit pair ↔ 8-bit half aliasing (HL↔H/L, DE↔D/E, BC↔B/C).
func locsOverlap(desc *MachineDesc, w, r int) bool {
	if w == r {
		return true
	}
	if w < 0 || r < 0 || w >= len(desc.Locs) || r >= len(desc.Locs) {
		return false
	}
	wName := desc.Locs[w].Name
	rName := desc.Locs[r].Name
	// Check if w (pair) subsumes r (half)
	switch wName {
	case "HL":
		return rName == "H" || rName == "L"
	case "DE":
		return rName == "D" || rName == "E"
	case "BC":
		return rName == "B" || rName == "C"
	}
	// Check if r (pair) subsumes w (half)
	switch rName {
	case "HL":
		return wName == "H" || wName == "L"
	case "DE":
		return wName == "D" || wName == "E"
	case "BC":
		return wName == "B" || wName == "C"
	}
	return false
}

// sortParallelMoves orders pending moves so reads-from-R happen before writes-to-R.
// This prevents parallel-move ordering bugs where move A clobbers the source of move B.
// Uses Kahn's topological sort. Cycles (swaps) are left in original order (rare on Z80).
func sortParallelMoves(desc *MachineDesc, moves []parallelMove) []parallelMove {
	n := len(moves)
	if n <= 1 {
		return moves
	}
	// deps[j] = indices i that must come AFTER j (j reads from what i writes)
	deps := make([][]int, n)
	inDegree := make([]int, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			// If move i writes to a loc that move j reads from → j before i
			if locsOverlap(desc, moves[i].currLoc, moves[j].prevLoc) {
				deps[j] = append(deps[j], i)
				inDegree[i]++
			}
		}
	}
	// Kahn's algorithm
	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}
	result := make([]parallelMove, 0, n)
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		result = append(result, moves[curr])
		for _, next := range deps[curr] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	// If cycle detected, append remaining in original order
	if len(result) < n {
		emitted := make(map[int]bool)
		for _, r := range result {
			for i, m := range moves {
				if m.vreg == r.vreg && m.prevLoc == r.prevLoc && m.currLoc == r.currLoc {
					emitted[i] = true
					break
				}
			}
		}
		for i, m := range moves {
			if !emitted[i] {
				result = append(result, m)
			}
		}
	}
	return result
}

// SolveCFG solves ALL blocks of a function simultaneously with CFG edge constraints.
func SolveCFG(vf *Func, f *mir2.Func, desc *MachineDesc, opts SolverOptions) (map[string][]PIROp, error) {
	sol, err := SolveCFGFull(vf, f, desc, opts)
	if err != nil {
		return nil, err
	}
	return sol.BlockPIR, nil
}

// SolveCFGFull is like SolveCFG but also returns param register assignments.
func SolveCFGFull(vf *Func, f *mir2.Func, desc *MachineDesc, opts SolverOptions) (*CFGSolution, error) {
	if opts.Z3Path == "" {
		opts.Z3Path = "z3"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if _, err := exec.LookPath(opts.Z3Path); err != nil {
		return nil, fmt.Errorf("z3 not found: %w", err)

	}

	// Build per-block problems
	type blockProblem struct {
		label string
		ops   []VIROp
		prob  *problem
	}
	var blocks []blockProblem
	blockIdx := make(map[string]int) // label → index

	// Build param hints early (before pre-solver rewrites)
	paramHintsEarly := make(map[int]int)
	if opts.FuncParamLocs != nil {
		if pl, ok := opts.FuncParamLocs[f.Name]; ok {
			for v, p := range pl { paramHintsEarly[v] = p }
		}
	}
	for v, p := range opts.ParamLocs {
		if _, ok := paramHintsEarly[v]; !ok { paramHintsEarly[v] = p }
	}

	for bi, block := range vf.Blocks {
		ops := block.Ops
		// Apply pre-solver passes per block
		ops = insertPreTieMoves(ops, desc)
		ops = insertSaveMoves(ops, desc)

		// Note: we do NOT propagate param constraints to copy vregs from
		// pre-tie. The copy and original would both be constrained to the
		// same register, creating an interference conflict (both live at
		// the move instruction). Instead, param constraints apply only to
		// original vregs. The move pattern naturally copies the value.

		prob := buildProblem(ops, desc)

		// Inject block parameter vregs into liveness from instruction 0
		// up to their last use (or all instructions if they pass through).
		// Block params (PHI destinations) are live from block entry but
		// computeLiveness misses them before their first use in the block
		// (and entirely if they're only passed to successor terminators).
		// Only inject block param liveness for blocks that CONTAIN CALLs.
		// These are the blocks where the param vreg needs to survive clobber.
		// For blocks without CALLs (e.g., simple conditionals), injection
		// over-constrains and causes unsat.
		hasCall := false
		for _, op := range ops {
			if (op.Op == OpCall || op.Op == OpAsmBlock) && !op.Clobbers.IsEmpty() {
				hasCall = true
				break
			}
		}
		if hasCall && len(block.Params) > 0 {
			if os.Getenv("VIR_DEBUG_EDGES") != "" {
				fmt.Fprintf(os.Stderr, "[PARAMS] %s b%d: params=%v (block has CALL)\n", f.Name, bi, block.Params)
			}
			for _, paramVreg := range block.Params {
				prob.vregs[paramVreg] = true
				for i := range prob.liveness {
					prob.liveness[i].live[paramVreg] = true
				}
			}
		}

		label := block.Label
		if label == "" {
			label = fmt.Sprintf("block%d", bi)
		}
		blockIdx[label] = len(blocks)
		blocks = append(blocks, blockProblem{label, ops, prob})
	}

	// Build CFG edges from MIR2 terminators + PHI maps from block params.
	// MIR2 uses block arguments instead of PHI nodes:
	//   TermJmp{Target: "join", Args: [r73]} → Block{Params: [{Dst: r79}]}
	// This means r73 (in the source block) becomes r79 (in the dest block).
	type cfgEdge struct {
		fromBlock int
		toBlock   int
		phiMap    map[int]int // old vreg → new vreg (from term Args → block Params)
	}
	var edges []cfgEdge

	for bi, mirBlock := range f.Blocks {
		if mirBlock.Term == nil {
			continue
		}

		// Extract args from the terminator for each successor
		type succInfo struct {
			label string
			args  []mir2.Reg
		}
		var succs []succInfo

		switch t := mirBlock.Term.(type) {
		case *mir2.TermJmp:
			succs = append(succs, succInfo{t.Target, t.Args})
		case *mir2.TermBrIf:
			succs = append(succs, succInfo{t.Then, t.ThenArgs})
			succs = append(succs, succInfo{t.Else, t.ElseArgs})
		default:
			for _, label := range mirBlock.Term.Successors() {
				succs = append(succs, succInfo{label, nil})
			}
		}

		for _, succ := range succs {
			si, ok := blockIdx[succ.label]
			if !ok {
				continue
			}

			// Build PHI map: term args[i] → VIR block params[i]
			// Uses VIR block params (populated by LowerFunc from MIR2 block params).
			// Both args and params use the same vreg ID space.
			phi := make(map[int]int)
			destVIR := vf.Blocks[si]
			for pi, arg := range succ.args {
				if pi < len(destVIR.Params) {
					phi[int(arg)] = destVIR.Params[pi]
				}
			}

			edges = append(edges, cfgEdge{bi, si, phi})
			if os.Getenv("VIR_DEBUG_EDGES") != "" && len(phi) > 0 {
				fmt.Fprintf(os.Stderr, "[PHI] %s: edge b%d→b%d phi=%v\n", f.Name, bi, si, phi)
			}

			// Force terminator-arg vregs into liveness at the last instruction
			// of the source block. These vregs are used by the PHI but don't
			// appear in VIR ops — without this they're invisible to liveness.
			if len(succ.args) > 0 && len(blocks[bi].prob.liveness) > 0 {
				lastIdx := len(blocks[bi].prob.liveness) - 1
				for _, arg := range succ.args {
					blocks[bi].prob.liveness[lastIdx].live[int(arg)] = true
					blocks[bi].prob.vregs[int(arg)] = true
				}
			}
		}
	}

	// Compute per-block live-in (from incoming edges) and live-out (to outgoing edges).
	// Vregs that are live-in AND live-out but not referenced within the block
	// are "live-through" — they must get clobber constraints at CALL instructions.
	blockLiveIn := make([]map[int]bool, len(blocks))
	blockLiveOut := make([]map[int]bool, len(blocks))
	for i := range blocks {
		blockLiveIn[i] = make(map[int]bool)
		blockLiveOut[i] = make(map[int]bool)
	}
	for _, edge := range edges {
		fromBP := blocks[edge.fromBlock]
		fromLastIdx := len(fromBP.ops) - 1
		if fromLastIdx < 0 {
			continue
		}
		for vreg := range fromBP.prob.liveness[fromLastIdx].live {
			blockLiveOut[edge.fromBlock][vreg] = true
			blockLiveIn[edge.toBlock][vreg] = true
		}
	}

	// Inject function params into blocks that use them but don't define them.
	// Per-block liveness misses cross-block params (e.g., param b used in block 2
	// but not in block 0). Without injection, the Z3 model has no variable for b
	// in block 0, preventing edge moves from being emitted.
	funcParams := make(map[int]bool) // set of function param vregs
	for _, cp := range f.Contract.Params {
		if cp.Reg != mir2.NoReg {
			funcParams[int(cp.Reg)] = true
		}
	}
	if len(funcParams) > 0 {
		// First: identify which params are used in ANY non-entry block
		paramUsedLater := make(map[int]bool)
		for bi, bp := range blocks {
			if bi == 0 {
				continue // skip entry
			}
			for paramV := range funcParams {
				for _, op := range bp.ops {
					for _, s := range op.Src {
						if s == paramV {
							paramUsedLater[paramV] = true
						}
					}
				}
			}
		}

		for bi, bp := range blocks {
			for paramV := range funcParams {
				// Entry block: inject params used in later blocks at ALL instructions.
				// This ensures: (1) Z3 variables exist for edge move constraints,
				// (2) interference with other params is correct (a≠b at entry),
				// (3) move cost pairs are generated for consecutive instructions.
				if bi == 0 && paramUsedLater[paramV] && len(bp.prob.liveness) > 0 {
					for li := range bp.prob.liveness {
						bp.prob.liveness[li].live[paramV] = true
					}
					bp.prob.vregs[paramV] = true
				}

				// Non-entry blocks: inject at i0 and use points
				if bi > 0 {
					usedInBlock := false
					for _, op := range bp.ops {
						for _, s := range op.Src {
							if s == paramV {
								usedInBlock = true
								break
							}
						}
						if usedInBlock {
							break
						}
					}
					if !usedInBlock {
						continue
					}
					for i, op := range bp.ops {
						if i >= len(bp.prob.liveness) {
							break
						}
						needInject := (i == 0) // at block entry
						for _, s := range op.Src {
							if s == paramV {
								needInject = true
								break
							}
						}
						if needInject && !bp.prob.liveness[i].live[paramV] {
							bp.prob.liveness[i].live[paramV] = true
							bp.prob.vregs[paramV] = true
						}
					}
				}
			}
		}
		// Recompute blockLiveIn/Out now that params are injected
		for i := range blockLiveIn {
			blockLiveIn[i] = make(map[int]bool)
			blockLiveOut[i] = make(map[int]bool)
		}
		for _, edge := range edges {
			fromBP := blocks[edge.fromBlock]
			fromLastIdx := len(fromBP.ops) - 1
			if fromLastIdx < 0 {
				continue
			}
			for vreg := range fromBP.prob.liveness[fromLastIdx].live {
				blockLiveOut[edge.fromBlock][vreg] = true
				blockLiveIn[edge.toBlock][vreg] = true
			}
		}
	}

	// Inject live-in vregs into per-block liveness for two purposes:
	// 1. At CLOBBER instructions (CALL): clobber constraints need the vreg
	// 2. At block entry (inst 0) and where used: edge moves and proper allocation
	for bi, bp := range blocks {
		liveIn := blockLiveIn[bi]
		_ = blockLiveOut[bi] // used for future live-through analysis
		if len(liveIn) == 0 {
			continue
		}

		for vreg := range liveIn {
			injected := false
			// Inject at CLOBBER instructions (CALL/AsmBlock)
			for i, op := range bp.ops {
				if !op.Clobbers.IsEmpty() && i < len(bp.prob.liveness) {
					if !bp.prob.liveness[i].live[vreg] {
						bp.prob.liveness[i].live[vreg] = true
						injected = true
					}
				}
			}
			// Inject at instruction 0 (for edge move constraints) and at
			// all instructions where this vreg is used as a source.
			// This ensures the Z3 model has variables for cross-block params
			// at the block boundary AND at their use points.
			for i, op := range bp.ops {
				if i >= len(bp.prob.liveness) {
					break
				}
				needInject := (i == 0) // always at block entry
				for _, s := range op.Src {
					if s == vreg {
						needInject = true
						break
					}
				}
				if needInject && !bp.prob.liveness[i].live[vreg] {
					bp.prob.liveness[i].live[vreg] = true
					injected = true
				}
			}
			if injected {
				bp.prob.vregs[vreg] = true
			}
		}
	}

	// Generate unified SMT
	nLocs := len(desc.Locs)
	var b strings.Builder
	b.WriteString("(set-logic QF_LIA)\n")

	// Declare per-block, per-instruction variables
	type varKey struct {
		vreg, block, inst int
	}
	vars := make(map[varKey]bool)

	// Pre-scan: when a single forced pattern requires the destination to occupy
	// a NonAllocatable loc (e.g. CMP dst must be F/flags), the generic exclusion
	// would create an immediate contradiction. Collect those (vreg,block,inst)
	// keys so ensureVar can skip the exclusion for exactly those forced locs.
	type forcedKey struct{ vreg, block, inst int }
	allowedForcedLocs := make(map[forcedKey]LocSet)
	for bi, bp := range blocks {
		p := bp.prob
		for i, pats := range p.patterns {
			if len(pats) != 1 {
				continue // skip multi-choice instructions
			}
			pi := pats[0]
			op := p.ops[i]
			if op.Dst <= 0 {
				continue
			}
			pat := &desc.Patterns[pi]
			forced := pat.DstLocs.And(desc.NonAllocatable)
			if !forced.IsEmpty() {
				allowedForcedLocs[forcedKey{op.Dst, bi, i}] = forced
			}
		}
	}

	ensureVar := func(vreg, block, inst int) string {
		name := fmt.Sprintf("lv%d_b%d_i%d", vreg, block, inst)
		key := varKey{vreg, block, inst}
		if !vars[key] {
			vars[key] = true
			b.WriteString(fmt.Sprintf("(declare-const %s Int)\n", name))
			b.WriteString(fmt.Sprintf("(assert (and (>= %s 0) (< %s %d)))\n", name, name, nLocs))
			// Exclude non-allocatable locations (F, SP) from general vreg storage.
			// Exception: if a forced single-choice pattern requires this vreg to be
			// at a NonAllocatable loc (e.g. CMP destination → F register), skip that
			// exclusion to avoid an immediate contradiction in the SMT.
			allowedForced := allowedForcedLocs[forcedKey{vreg, block, inst}]
			desc.NonAllocatable.ForEach(func(i int) bool {
				if allowedForced.Has(i) {
					return true // pattern forces it here — don't exclude
				}
				b.WriteString(fmt.Sprintf("(assert (not (= %s %d))) ; exclude %s\n", name, i, desc.Locs[i].Name))
				return true
			})
		}
		return name
	}

	// Per-block constraints
	for bi, bp := range blocks {
		p := bp.prob

		// Pattern variables
		for i, pats := range p.patterns {
			if len(pats) == 0 {
				continue
			}
			patVar := fmt.Sprintf("pat_b%d_%d", bi, i)
			b.WriteString(fmt.Sprintf("(declare-const %s Int)\n", patVar))
			if len(pats) == 1 {
				b.WriteString(fmt.Sprintf("(assert (= %s %d))\n", patVar, pats[0]))
			} else {
				b.WriteString("(assert (or")
				for _, pi := range pats {
					b.WriteString(fmt.Sprintf(" (= %s %d)", patVar, pi))
				}
				b.WriteString("))\n")
			}
		}

		// Location variables for all referenced vregs
		for i, op := range p.ops {
			if op.Dst > 0 {
				ensureVar(op.Dst, bi, i)
			}
			for _, s := range op.Src {
				if s > 0 {
					ensureVar(s, bi, i)
				}
			}
		}
		// From liveness (sorted for deterministic Z3 encoding)
		// Debug liveness injection
		if os.Getenv("VIR_DEBUG_LIVENESS") != "" {
			for i, l := range p.liveness {
				vs := make([]int, 0)
				for v := range l.live { vs = append(vs, v) }
				sort.Ints(vs)
				if len(vs) > 0 {
					fmt.Fprintf(os.Stderr, "[DEBUG] %s b%d i%d live: %v\n", f.Name, bi, i, vs)
				}
			}
		}
		for i, l := range p.liveness {
			liveVRegs := make([]int, 0, len(l.live))
			for v := range l.live {
				liveVRegs = append(liveVRegs, v)
			}
			sort.Ints(liveVRegs)
			for _, v := range liveVRegs {
				ensureVar(v, bi, i)
			}
		}

		// Pattern → location constraints
		type vregPair struct{ a, b int }
		tied := make(map[vregPair]bool)

		for i, op := range p.ops {
			pats := p.patterns[i]
			for _, pi := range pats {
				pat := &desc.Patterns[pi]
				cond := fmt.Sprintf("(= pat_b%d_%d %d)", bi, i, pi)

				if op.Dst > 0 && !pat.DstLocs.IsEmpty() {
					v := fmt.Sprintf("lv%d_b%d_i%d", op.Dst, bi, i)
					c := locSetToSMT(v, pat.DstLocs)
					b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, c))
				}
				if op.Src[0] > 0 && !pat.SrcLocs[0].IsEmpty() {
					v := fmt.Sprintf("lv%d_b%d_i%d", op.Src[0], bi, i)
					c := locSetToSMT(v, pat.SrcLocs[0])
					b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, c))
				}
				if op.Src[1] > 0 && !pat.SrcLocs[1].IsEmpty() {
					v := fmt.Sprintf("lv%d_b%d_i%d", op.Src[1], bi, i)
					c := locSetToSMT(v, pat.SrcLocs[1])
					b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, c))
				}
				if pat.TiedDstSrc && op.Dst > 0 && op.Src[0] > 0 {
					dv := fmt.Sprintf("lv%d_b%d_i%d", op.Dst, bi, i)
					sv := fmt.Sprintf("lv%d_b%d_i%d", op.Src[0], bi, i)
					b.WriteString(fmt.Sprintf("(assert (=> %s (= %s %s)))\n", cond, dv, sv))
					tied[vregPair{op.Dst, op.Src[0]}] = true
					tied[vregPair{op.Src[0], op.Dst}] = true
				}
			}
		}

		// Build coalesce set: pairs of vregs that CAN share a register.
		// OpMove src→dst: hardware LD r,r is valid when src=dst (nop).
		// OpAsmBlock: input/output vregs can share (consumed/produced).
		asmCoalesce := make(map[vregPair]bool)
		for _, op := range bp.ops {
			// OpMove: src and dst can coalesce (LD A,A is valid nop)
			if op.Op == OpMove && op.Dst > 0 && op.Src[0] > 0 {
				asmCoalesce[vregPair{op.Dst, op.Src[0]}] = true
				asmCoalesce[vregPair{op.Src[0], op.Dst}] = true
			}
			if op.Op == OpAsmBlock && op.Dst > 0 {
				for _, s := range op.Src {
					if s > 0 {
						asmCoalesce[vregPair{op.Dst, s}] = true
						asmCoalesce[vregPair{s, op.Dst}] = true
					}
				}
				for _, v := range op.AsmIns {
					asmCoalesce[vregPair{op.Dst, v}] = true
					asmCoalesce[vregPair{v, op.Dst}] = true
				}
			}
		}

		// Interference within block (sorted for deterministic Z3 encoding)
		emitted := make(map[[3]int]bool)
		for i := range p.ops {
			liveVRegs := make([]int, 0, len(p.liveness[i].live))
			for v := range p.liveness[i].live {
				liveVRegs = append(liveVRegs, v)
			}
			sort.Ints(liveVRegs)
			for a := 0; a < len(liveVRegs); a++ {
				for c := a + 1; c < len(liveVRegs); c++ {
					va, vc := liveVRegs[a], liveVRegs[c]
					if tied[vregPair{va, vc}] || asmCoalesce[vregPair{va, vc}] {
						continue
					}
					key := [3]int{i, va, vc}
					if emitted[key] {
						continue
					}
					emitted[key] = true
					av := fmt.Sprintf("lv%d_b%d_i%d", va, bi, i)
					bv := fmt.Sprintf("lv%d_b%d_i%d", vc, bi, i)
					b.WriteString(fmt.Sprintf("(assert (not (= %s %s)))\n", av, bv))
					// Register pair aliasing: if va=HL(9), vb can't be H(5) or L(6)
					// because HL physically contains H and L.
					for _, alias := range pairAliases {
						b.WriteString(fmt.Sprintf("(assert (=> (= %s %d) (and (not (= %s %d)) (not (= %s %d)))))\n",
							av, alias.pair, bv, alias.hi, bv, alias.lo))
						b.WriteString(fmt.Sprintf("(assert (=> (= %s %d) (and (not (= %s %d)) (not (= %s %d)))))\n",
							bv, alias.pair, av, alias.hi, av, alias.lo))
					}
				}
			}
		}

		// Clobber constraints (sorted for deterministic Z3 encoding)
		for i, op := range p.ops {
			if op.Clobbers.IsEmpty() {
				continue
			}
			// Build set of vregs that are consumed by this instruction
			// (dst + srcs) — these are read/written at the instruction
			// and should not be excluded by clobber constraints.
			consumed := make(map[int]bool)
			if op.Dst > 0 {
				consumed[op.Dst] = true
			}
			for _, s := range op.Src {
				if s > 0 {
					consumed[s] = true
				}
			}
			// OpAsmBlock: all asm inputs/outputs are consumed
			if op.Op == OpAsmBlock {
				for _, v := range op.AsmIns {
					consumed[v] = true
				}
				for _, v := range op.AsmOuts {
					consumed[v] = true
				}
			}

			clobVRegs := make([]int, 0, len(p.liveness[i].live))
			for vreg := range p.liveness[i].live {
				clobVRegs = append(clobVRegs, vreg)
			}
			sort.Ints(clobVRegs)
			for _, vreg := range clobVRegs {
				if consumed[vreg] {
					continue
				}
				v := fmt.Sprintf("lv%d_b%d_i%d", vreg, bi, i)
				op.Clobbers.ForEach(func(loc int) bool {
					b.WriteString(fmt.Sprintf("(assert (not (= %s %d)))\n", v, loc))
					return true
				})
			}
		}

		// DstHint / SrcHint
		for i, op := range p.ops {
			if op.Dst > 0 && !op.DstHint.IsEmpty() {
				v := fmt.Sprintf("lv%d_b%d_i%d", op.Dst, bi, i)
				c := locSetToSMT(v, op.DstHint)
				b.WriteString(fmt.Sprintf("(assert %s)\n", c))
			}
			for j, s := range op.Src {
				if s > 0 && !op.SrcHint[j].IsEmpty() && !op.DstHint.IsEmpty() {
					v := fmt.Sprintf("lv%d_b%d_i%d", s, bi, i)
					c := locSetToSMT(v, op.SrcHint[j])
					b.WriteString(fmt.Sprintf("(assert %s)\n", c))
				}
			}
		}
	}

	// Param location constraints: entry block (b0) params must be in PBQP registers.
	// Param location constraints: pin each param vreg to its PBQP register
	// at the FIRST instruction that references it, in ANY block.
	// (Was: only block 0, missing params first used in later blocks.)
	if len(paramHintsEarly) > 0 {
		applied := make(map[int]bool)
		for bi, bp := range blocks {
			for vreg, phys := range paramHintsEarly {
				if applied[vreg] {
					continue
				}
				for i, op := range bp.ops {
					usesVreg := false
					if op.Dst == vreg { usesVreg = true }
					for _, s := range op.Src {
						if s == vreg { usesVreg = true }
					}
					if usesVreg {
						v := ensureVar(vreg, bi, i)
						locName := "?"
						if phys < len(desc.Locs) {
							locName = desc.Locs[phys].Name
						}
						b.WriteString(fmt.Sprintf("(assert (= %s %d)) ; param vreg %d in %s\n",
							v, phys, vreg, locName))
						applied[vreg] = true
						break
					}
				}
			}
		}
	}

	// CFG edge constraints: vreg locations should match across block boundaries.
	// We use SOFT constraints (move cost) instead of hard equality, so the solver
	// can insert moves at block boundaries when needed (e.g., abs_diff: b needs
	// to move from C to A at the else-block entry for SUB A,r pattern).
	type edgeMove struct {
		fromVar, toVar string
	}
	var edgeMoves []edgeMove

	// Check which blocks CONTAIN a CALL (clobbers ALL GPRs).
	// Any vreg live across an edge into such a block must be call-safe.
	// NOTE: OpAsmBlock with specific clobbers is NOT treated as a CALL here —
	// its clobbers are handled at instruction-level. Only genuine OpCall ops
	// (which clobber all GPRs per calling convention) trigger this constraint.
	blockContainsCall := make(map[int]bool)
	for bi, bp := range blocks {
		for _, op := range bp.ops {
			if op.Op == OpCall && !op.Clobbers.IsEmpty() {
				blockContainsCall[bi] = true
				break
			}
		}
	}

	for _, edge := range edges {
		fromBP := blocks[edge.fromBlock]
		toBP := blocks[edge.toBlock]

		fromLastIdx := len(fromBP.ops) - 1
		if fromLastIdx < 0 {
			continue
		}

		fromLive := fromBP.prob.liveness[fromLastIdx].live

		// Does the destination block have a CALL early that would clobber GPR?
		destHasCall := blockContainsCall[edge.toBlock]

		sortedVRegs := make([]int, 0, len(fromLive))
		for vreg := range fromLive {
			sortedVRegs = append(sortedVRegs, vreg)
		}
		sort.Ints(sortedVRegs)

		for _, vreg := range sortedVRegs {
			// Determine the destination vreg: either same ID or PHI-mapped
			toVreg := vreg
			if mapped, ok := edge.phiMap[vreg]; ok {
				toVreg = mapped
			}

			// Check if the destination vreg is live in the destination block
			toBlockHasVreg := false
			if toBP.prob.vregs[toVreg] {
				toBlockHasVreg = true
			}
			// Also check toLive at instruction 0
			if len(toBP.prob.liveness) > 0 && toBP.prob.liveness[0].live[toVreg] {
				toBlockHasVreg = true
			}
			// PHI-mapped vregs are always live at block entry
			if _, ok := edge.phiMap[vreg]; ok {
				toBlockHasVreg = true
			}

			if !toBlockHasVreg {
				continue
			}

			fromVar := ensureVar(vreg, edge.fromBlock, fromLastIdx)
			toVar := ensureVar(toVreg, edge.toBlock, 0)

			if destHasCall {
				// Vreg must survive a CALL at the start of the dest block.
				// HARD constraint: same location in both blocks AND must be
				// in a call-safe register (IXH=14, IXL=15, IYH=16, IYL=17).
				b.WriteString(fmt.Sprintf("(assert (= %s %s))\n", fromVar, toVar))
				b.WriteString(fmt.Sprintf("(assert (or (= %s 14) (= %s 15) (= %s 16) (= %s 17)))\n",
					fromVar, fromVar, fromVar, fromVar))
			} else {
				// SOFT equality for all edge vregs (PHI-mapped and same-ID alike).
				// PHI moves are computed and emitted at source-block tail — after the last
				// regular PIR op and before the block terminator.  On Z80, LD r,r' does
				// NOT set any flags, so a PHI move between a CP and a conditional JR is safe.
				edgeMoves = append(edgeMoves, edgeMove{fromVar, toVar})
			}
		}
	}

	// Cost objective
	b.WriteString("(declare-const total_cost Int)\n")
	b.WriteString("(assert (= total_cost (+ 0\n")

	// Pattern costs
	for bi, bp := range blocks {
		for i, pats := range bp.prob.patterns {
			if len(pats) == 0 {
				continue
			}
			patVar := fmt.Sprintf("pat_b%d_%d", bi, i)
			if len(pats) == 1 {
				b.WriteString(fmt.Sprintf("  %d\n", desc.Patterns[pats[0]].Cost))
			} else {
				b.WriteString("  ")
				for j, pi := range pats {
					if j < len(pats)-1 {
						b.WriteString(fmt.Sprintf("(ite (= %s %d) %d ", patVar, pi, desc.Patterns[pi].Cost))
					} else {
						b.WriteString(fmt.Sprintf("%d", desc.Patterns[pi].Cost))
					}
				}
				for j := 0; j < len(pats)-1; j++ {
					b.WriteString(")")
				}
				b.WriteString("\n")
			}
		}

		// Move costs within block (sorted for deterministic Z3 encoding)
		cfgSortedVRegs := make([]int, 0, len(bp.prob.vregs))
		for vreg := range bp.prob.vregs {
			cfgSortedVRegs = append(cfgSortedVRegs, vreg)
		}
		sort.Ints(cfgSortedVRegs)
		for _, vreg := range cfgSortedVRegs {
			for i := 0; i < len(bp.ops)-1; i++ {
				if !bp.prob.liveness[i].live[vreg] || !bp.prob.liveness[i+1].live[vreg] {
					continue
				}
				v1 := fmt.Sprintf("lv%d_b%d_i%d", vreg, bi, i)
				v2 := fmt.Sprintf("lv%d_b%d_i%d", vreg, bi, i+1)
				b.WriteString(fmt.Sprintf("  (ite (= %s %s) 0 4)\n", v1, v2))
			}
		}
	}

	// Edge move costs: penalty when vreg changes location across block boundary
	for _, em := range edgeMoves {
		b.WriteString(fmt.Sprintf("  (ite (= %s %s) 0 4)\n", em.fromVar, em.toVar))
	}

	b.WriteString(")))\n")

	// Skip minimize for large problems
	if len(vars) > 150 {
		b.WriteString("(check-sat)\n")
	} else {
		b.WriteString("(minimize total_cost)\n")
		b.WriteString("(check-sat)\n")
	}
	b.WriteString("(get-model)\n")

	smt := b.String()
	if opts.Verbose {
		fmt.Printf("[CFG-solver] %d blocks, %d edges, %d vars\n", len(blocks), len(edges), len(vars))
	}
	// Run Z3
	model, err := runZ3(smt, opts)
	if err != nil {
		// Dump SMT on failure for debugging
		if os.Getenv("VIR_DUMP_SMT") != "" {
			smtFile := fmt.Sprintf("/tmp/vir_cfg_%s.smt2", f.Name)
			os.WriteFile(smtFile, []byte(smt), 0644)
			fmt.Fprintf(os.Stderr, "[CFG-solver] dumped SMT to %s (%d bytes)\n", smtFile, len(smt))
		}
		return nil, fmt.Errorf("CFG solve: %w", err)
	}

	vals := parseZ3Model(model)

	// Parse solution per block
	var moveErrors []string
	result := make(map[string][]PIROp)
	for bi, bp := range blocks {
		var pirOps []PIROp
		for i, op := range bp.ops {
			pats := bp.prob.patterns[i]
			if len(pats) == 0 {
				pirOps = append(pirOps, PIROp{Comment: fmt.Sprintf("no pattern op %d", i)})
				continue
			}

			patKey := fmt.Sprintf("pat_b%d_%d", bi, i)
			patIdx := pats[0]
			if v, ok := vals[patKey]; ok {
				patIdx = v
			}
			pat := &desc.Patterns[patIdx]

			dstPhys := -1
			if op.Dst > 0 {
				if v, ok := vals[fmt.Sprintf("lv%d_b%d_i%d", op.Dst, bi, i)]; ok {
					dstPhys = v
				}
			}

			var srcPhys [2]int
			srcPhys[0] = -1
			srcPhys[1] = -1
			for j, s := range op.Src {
				if s > 0 {
					if v, ok := vals[fmt.Sprintf("lv%d_b%d_i%d", s, bi, i)]; ok {
						srcPhys[j] = v
					}
				}
			}

			// Insert inter-instruction moves for ALL live vregs that change location.
			// Not just src operands — also block params and other live-through vregs.
			// Moves are collected first and sorted by dependency (reads-before-writes)
			// to avoid parallel-move ordering bugs (e.g., DE→HL clobbering HL before
			// HL→IXH has saved the value).
			if i > 0 {
				moveVregs := make([]int, 0)
				// Start with src operands (original behavior)
				for _, s := range []int{op.Src[0], op.Src[1]} {
					if s > 0 { moveVregs = append(moveVregs, s) }
				}
				// Add all live vregs at this instruction (catches block params)
				if i < len(bp.prob.liveness) {
					for v := range bp.prob.liveness[i].live {
						found := false
						for _, mv := range moveVregs {
							if mv == v { found = true; break }
						}
						if !found && v > 0 { moveVregs = append(moveVregs, v) }
					}
					sort.Ints(moveVregs)
				}
				var pendingMoves []parallelMove
				for _, vreg := range moveVregs {
					if vreg <= 0 {
						continue
					}
					currKey := fmt.Sprintf("lv%d_b%d_i%d", vreg, bi, i)
					currLoc, hasCurr := vals[currKey]
					if !hasCurr {
						continue
					}
					for pi := i - 1; pi >= 0; pi-- {
						prevKey := fmt.Sprintf("lv%d_b%d_i%d", vreg, bi, pi)
						prevLoc, hasPrev := vals[prevKey]
						if hasPrev {
							if prevLoc != currLoc {
								remSrc, movePat := findMovePatternRemap(desc, prevLoc, currLoc)
								if movePat != nil {
									if os.Getenv("VIR_DEBUG_EDGES") != "" {
										fmt.Fprintf(os.Stderr, "[INTRA-MOVE] v%d: %s(%d)→%s(%d) at b%d i%d\n",
											vreg, desc.Locs[prevLoc].Name, prevLoc, desc.Locs[currLoc].Name, currLoc, bi, i)
									}
									pendingMoves = append(pendingMoves, parallelMove{
										vreg: vreg, prevLoc: prevLoc, currLoc: currLoc,
										movePat: movePat, remSrc: remSrc,
									})
								} else {
									srcName, dstName := "?", "?"
									if prevLoc >= 0 && prevLoc < len(desc.Locs) {
										srcName = desc.Locs[prevLoc].Name
									}
									if currLoc >= 0 && currLoc < len(desc.Locs) {
										dstName = desc.Locs[currLoc].Name
									}
									moveErr := fmt.Sprintf("no move pattern for v%d: %s(%d) → %s(%d) at block %d inst %d",
										vreg, srcName, prevLoc, dstName, currLoc, bi, i)
									fmt.Fprintf(os.Stderr, "[CFG-solver] %s\n", moveErr)
									moveErrors = append(moveErrors, moveErr)
								}
							}
							break
						}
					}
				}
				// Sort pending moves: reads from R must come before writes to R
				pendingMoves = sortParallelMoves(desc, pendingMoves)
				for _, pm := range pendingMoves {
					pirOps = append(pirOps, PIROp{
						Pat: pm.movePat, DstPhys: pm.currLoc,
						SrcPhys: [2]int{pm.remSrc, -1},
					})
				}
			}

			pir := PIROp{
				Pat: pat, DstPhys: dstPhys, SrcPhys: srcPhys,
				Imm: op.Imm, Sym: op.Sym,
			}
			if op.Op == OpAsmBlock {
				pir.AsmText = op.AsmTemplate
			}
			pirOps = append(pirOps, pir)
		}
		result[bp.label] = pirOps
	}

	// Emit edge moves: if Z3 assigned different locations for a vreg across
	// a block boundary, insert LD at the start of the destination block.
	// Without this, cross-block location changes are phantom — the 4T cost
	// is paid in the objective but no instruction implements the move.
	if opts.Verbose || os.Getenv("VIR_DEBUG_EDGES") != "" {
		fmt.Fprintf(os.Stderr, "[CFG-solver] %s: %d edges to check for moves\n", f.Name, len(edges))
	}

	// Dump SMT for debugging when requested
	if os.Getenv("VIR_DUMP_SMT") != "" {
		suffix := "2"
		if len(paramHintsEarly) > 0 {
			suffix = "1_constrained"
		}
		smtFile := fmt.Sprintf("/tmp/vir_cfg_%s_%s.smt2", f.Name, suffix)
		os.WriteFile(smtFile, []byte(smt), 0644)
		fmt.Fprintf(os.Stderr, "[CFG-solver] dumped SMT to %s (%d bytes)\n", smtFile, len(smt))
	}
	for _, edge := range edges {
		fromBP := blocks[edge.fromBlock]
		toBP := blocks[edge.toBlock]
		fromLastIdx := len(fromBP.ops) - 1
		if fromLastIdx < 0 {
			continue
		}

		fromLive := fromBP.prob.liveness[fromLastIdx].live
		var toLive map[int]bool
		if len(toBP.prob.liveness) > 0 {
			toLive = toBP.prob.liveness[0].live
		}

		// Collect vregs that need edge moves:
		// 1. Live at both ends of the edge (classic case)
		// 2. Used in destination but not defined there (cross-block live-through,
		//    e.g., function params used in later blocks)
		edgeVRegs := make([]int, 0)
		edgeVRegSet := make(map[int]bool)
		for vreg := range fromLive {
			if toLive != nil && toLive[vreg] {
				edgeVRegSet[vreg] = true
			}
		}
		// Add vregs used in destination block but defined elsewhere
		// (function params, cross-block values). These may not appear in
		// per-block liveness if only used in successor blocks.
		for _, op := range toBP.ops {
			for _, s := range op.Src {
				if s > 0 && !edgeVRegSet[s] {
					// Check if this vreg is defined in the destination block
					definedInTo := false
					for _, dop := range toBP.ops {
						if dop.Dst == s {
							definedInTo = true
							break
						}
					}
					if !definedInTo {
						edgeVRegSet[s] = true
					}
				}
			}
		}
		for v := range edgeVRegSet {
			edgeVRegs = append(edgeVRegs, v)
		}
		sort.Ints(edgeVRegs)

		var edgePending []parallelMove
		for _, vreg := range edgeVRegs {
			fromKey := fmt.Sprintf("lv%d_b%d_i%d", vreg, edge.fromBlock, fromLastIdx)
			toKey := fmt.Sprintf("lv%d_b%d_i%d", vreg, edge.toBlock, 0)
			fromLoc, hasFrom := vals[fromKey]
			toLoc, hasTo := vals[toKey]

			// If vreg has no Z3 variable in the source block (live-through param
			// not used in that block), use its initial ABI location from FuncParamLocs.
			if !hasFrom && hasTo {
				if paramLocs := paramHintsEarly; len(paramLocs) > 0 {
					if pl, ok := paramLocs[vreg]; ok {
						fromLoc = pl
						hasFrom = true
					}
				}
			}

			if hasFrom && hasTo && fromLoc != toLoc {
				remappedSrc, movePat := findMovePatternRemap(desc, fromLoc, toLoc)
				if movePat != nil {
					if os.Getenv("VIR_DEBUG_EDGES") != "" {
						fmt.Fprintf(os.Stderr, "[EDGE-MOVE] v%d: %s(%d)→%s(%d) at b%d→b%d\n",
							vreg, desc.Locs[fromLoc].Name, fromLoc, desc.Locs[toLoc].Name, toLoc, edge.fromBlock, edge.toBlock)
					}
					edgePending = append(edgePending, parallelMove{
						vreg: vreg, prevLoc: fromLoc, currLoc: toLoc,
						movePat: movePat, remSrc: remappedSrc,
					})
				} else {
					fmt.Fprintf(os.Stderr, "[CFG-solver] WARNING: no edge move pattern for v%d: %s(%d) → %s(%d) at edge b%d→b%d\n",
						vreg, desc.Locs[fromLoc].Name, fromLoc, desc.Locs[toLoc].Name, toLoc, edge.fromBlock, edge.toBlock)
				}
			}
		}

		if len(edgePending) > 0 {
			// Sort edge moves: reads-before-writes to avoid parallel-move ordering bugs
			edgePending = sortParallelMoves(desc, edgePending)
			edgeMoveOps := make([]PIROp, len(edgePending))
			for i, pm := range edgePending {
				edgeMoveOps[i] = PIROp{
					Pat: pm.movePat, DstPhys: pm.currLoc,
					SrcPhys: [2]int{pm.remSrc, -1},
					Comment: fmt.Sprintf("edge move v%d: %s→%s", pm.vreg, desc.Locs[pm.prevLoc].Name, desc.Locs[pm.currLoc].Name),
				}
			}
			// Prepend edge moves to destination block's PIR
			existing := result[toBP.label]
			result[toBP.label] = append(edgeMoveOps, existing...)
		}

		// PHI edge moves: vregs that change identity across a block edge (block args).
		// Unlike same-ID edge moves (prepended to destination), PHI moves are APPENDED
		// to the source block — they execute on ALL outgoing paths, which is safe because:
		//   1. Z80 LD r,r' does NOT affect flags, so a PHI move between CP and JR is valid.
		//   2. Any path that doesn't need the move (loc already matches) sees a no-op.
		// Conflicts across different edges are resolved by sortParallelMoves.
		if len(edge.phiMap) > 0 {
			var phiPending []parallelMove
			for fromVreg, toVreg := range edge.phiMap {
				fromKey := fmt.Sprintf("lv%d_b%d_i%d", fromVreg, edge.fromBlock, fromLastIdx)
				toKey := fmt.Sprintf("lv%d_b%d_i%d", toVreg, edge.toBlock, 0)
				fromLoc, hasFrom := vals[fromKey]
				toLoc, hasTo := vals[toKey]
				if !hasFrom || !hasTo || fromLoc == toLoc {
					continue // no move needed
				}
				remappedSrc, movePat := findMovePatternRemap(desc, fromLoc, toLoc)
				if movePat == nil {
					fmt.Fprintf(os.Stderr, "[CFG-solver] WARNING: no PHI move pattern for v%d→v%d: %s(%d)→%s(%d) at b%d→b%d\n",
						fromVreg, toVreg, desc.Locs[fromLoc].Name, fromLoc, desc.Locs[toLoc].Name, toLoc, edge.fromBlock, edge.toBlock)
					continue
				}
				if os.Getenv("VIR_DEBUG_EDGES") != "" {
					fmt.Fprintf(os.Stderr, "[PHI-MOVE] v%d→v%d: %s(%d)→%s(%d) at b%d→b%d\n",
						fromVreg, toVreg, desc.Locs[fromLoc].Name, fromLoc, desc.Locs[toLoc].Name, toLoc, edge.fromBlock, edge.toBlock)
				}
				phiPending = append(phiPending, parallelMove{
					vreg: fromVreg, prevLoc: fromLoc, currLoc: toLoc,
					movePat: movePat, remSrc: remappedSrc,
				})
			}
			if len(phiPending) > 0 {
				phiPending = sortParallelMoves(desc, phiPending)
				for _, pm := range phiPending {
					result[fromBP.label] = append(result[fromBP.label], PIROp{
						Pat: pm.movePat, DstPhys: pm.currLoc,
						SrcPhys: [2]int{pm.remSrc, -1},
						Comment: fmt.Sprintf("phi move v%d→v%d: %s→%s", pm.vreg, edge.phiMap[pm.vreg], desc.Locs[pm.prevLoc].Name, desc.Locs[pm.currLoc].Name),
					})
				}
			}
		}
	}

	// Extract param register assignments from Z3 model.
	// For each function param, find the physical register Z3 assigned
	// at instruction 0 of block 0 (entry point).
	paramLocs := make(map[int]int)
	for _, cp := range f.Contract.Params {
		vreg := int(cp.Reg)
		key := fmt.Sprintf("lv%d_b0_i0", vreg)
		if v, ok := vals[key]; ok {
			paramLocs[vreg] = v
		} else {
			// Try first instruction that references this vreg
			if len(blocks) > 0 {
				for i, op := range blocks[0].ops {
					usesVreg := op.Dst == vreg
					for _, s := range op.Src {
						if s == vreg {
							usesVreg = true
						}
					}
					if usesVreg {
						k := fmt.Sprintf("lv%d_b0_i%d", vreg, i)
						if v2, ok2 := vals[k]; ok2 {
							paramLocs[vreg] = v2
							break
						}
					}
				}
			}
		}
	}

	// If any move patterns were missing, return error → triggers PBQP fallback
	if len(moveErrors) > 0 {
		return nil, fmt.Errorf("missing move patterns: %s", moveErrors[0])
	}

	return &CFGSolution{BlockPIR: result, ParamLocs: paramLocs}, nil
}
