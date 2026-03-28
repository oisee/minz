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

	// Inject live-through vregs at CALL/clobber points.
	// A vreg is live-through block B if it's in liveIn[B] AND
	// (liveOut[B] OR used-after-call within B).
	// At each CALL in B, the vreg needs a clobber constraint.
	for bi, bp := range blocks {
		liveIn := blockLiveIn[bi]
		_ = blockLiveOut[bi] // used for future live-through analysis
		if len(liveIn) == 0 {
			continue
		}

		// Inject live-in vregs at CLOBBER instructions (CALL/AsmBlock) only.
		// This ensures clobber constraints fire without over-constraining
		// interference at every instruction (which causes unsat for large functions).
		for vreg := range liveIn {
			injected := false
			for i, op := range bp.ops {
				if !op.Clobbers.IsEmpty() && i < len(bp.prob.liveness) {
					if !bp.prob.liveness[i].live[vreg] {
						bp.prob.liveness[i].live[vreg] = true
						injected = true
					}
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

	ensureVar := func(vreg, block, inst int) string {
		name := fmt.Sprintf("lv%d_b%d_i%d", vreg, block, inst)
		key := varKey{vreg, block, inst}
		if !vars[key] {
			vars[key] = true
			b.WriteString(fmt.Sprintf("(declare-const %s Int)\n", name))
			b.WriteString(fmt.Sprintf("(assert (and (>= %s 0) (< %s %d)))\n", name, name, nLocs))
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

	// Check which blocks CONTAIN a CALL (clobbers GPR).
	// Any vreg live across an edge into such a block must be call-safe.
	blockContainsCall := make(map[int]bool)
	for bi, bp := range blocks {
		for _, op := range bp.ops {
			if (op.Op == OpCall || op.Op == OpAsmBlock) && !op.Clobbers.IsEmpty() {
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
				// HARD equality for PHI-mapped vregs (they MUST match — no move possible)
				// Soft for same-ID vregs (move can be inserted)
				if toVreg != vreg {
					b.WriteString(fmt.Sprintf("(assert (= %s %s))\n", fromVar, toVar))
				} else {
					edgeMoves = append(edgeMoves, edgeMove{fromVar, toVar})
				}
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
								movePat := findMovePattern(desc, prevLoc, currLoc)
								if movePat != nil {
									pirOps = append(pirOps, PIROp{
										Pat: movePat, DstPhys: currLoc,
										SrcPhys: [2]int{prevLoc, -1},
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
		smtFile := fmt.Sprintf("/tmp/vir_cfg_%s.smt2", f.Name)
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

		// Sort for deterministic emission order
		edgeVRegs := make([]int, 0)
		for vreg := range fromLive {
			if toLive != nil && toLive[vreg] {
				edgeVRegs = append(edgeVRegs, vreg)
			}
		}
		sort.Ints(edgeVRegs)

		var edgeMoveOps []PIROp
		for _, vreg := range edgeVRegs {
			fromKey := fmt.Sprintf("lv%d_b%d_i%d", vreg, edge.fromBlock, fromLastIdx)
			toKey := fmt.Sprintf("lv%d_b%d_i%d", vreg, edge.toBlock, 0)
			fromLoc, hasFrom := vals[fromKey]
			toLoc, hasTo := vals[toKey]
			if hasFrom && hasTo && fromLoc != toLoc {
				movePat := findMovePattern(desc, fromLoc, toLoc)
				if movePat != nil {
					edgeMoveOps = append(edgeMoveOps, PIROp{
						Pat: movePat, DstPhys: toLoc,
						SrcPhys:  [2]int{fromLoc, -1},
						Comment: fmt.Sprintf("edge move v%d: %s→%s", vreg, desc.Locs[fromLoc].Name, desc.Locs[toLoc].Name),
					})
				} else {
					fmt.Fprintf(os.Stderr, "[CFG-solver] WARNING: no edge move pattern for v%d: %s(%d) → %s(%d) at edge b%d→b%d\n",
						vreg, desc.Locs[fromLoc].Name, fromLoc, desc.Locs[toLoc].Name, toLoc, edge.fromBlock, edge.toBlock)
				}
			}
		}

		if len(edgeMoveOps) > 0 {
			// Prepend edge moves to destination block's PIR
			existing := result[toBP.label]
			result[toBP.label] = append(edgeMoveOps, existing...)
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
