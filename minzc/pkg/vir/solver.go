// solver.go — Unified VIR→PIR solver.
//
// Simultaneously selects instruction patterns AND physical registers for each
// VIROp in a basic block. This replaces the 5-layer pipeline (isel → WFC →
// fixInvalidZ80Template → spill_reload → validate-reject).
//
// Architecture:
//   Z3 solver (primary): encodes joint isel+regalloc as SMT, provably optimal.
//   WFC solver (future):  fast greedy approximation, validated against Z3.
//
// For each VIROp the solver sees:
//   - ALL matching patterns (e.g., add_a_r, inc_r for OpAdd)
//   - ALL valid register assignments per pattern
//   - ALL spill tier options (L1-L7)
//   - ALL inter-instruction constraints (interference, clobbers, aliasing)
//
// One decision. No phase boundaries. No information loss.
package vir

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SolverOptions controls the unified solver.
type SolverOptions struct {
	Timeout time.Duration // Z3 timeout (default 5s)
	Z3Path  string        // path to z3 binary (default: "z3")
	Verbose bool          // print SMT-LIB2 and model
}

// Solve converts a basic block of VIROps into PIROps using Z3 SMT solver.
// It simultaneously selects instruction patterns AND physical registers.
func Solve(ops []VIROp, desc *MachineDesc, opts SolverOptions) ([]PIROp, error) {
	if len(ops) == 0 {
		return nil, nil
	}
	if opts.Z3Path == "" {
		opts.Z3Path = "z3"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}

	// Check z3 exists
	if _, err := exec.LookPath(opts.Z3Path); err != nil {
		return nil, fmt.Errorf("z3 not found: %w", err)
	}

	// Pre-solver pass 1: insert pre-tie moves — when a tied-dst-src op needs
	// src0 in a specific register but src0 is also live elsewhere, insert a
	// short-lived copy to avoid global location conflicts.
	ops = insertPreTieMoves(ops, desc)

	// Pre-solver pass 2: insert save moves for destructive (tied) ops
	ops = insertSaveMoves(ops, desc)

	// Pre-solver pass 3: insert spill/reload for register pressure relief
	ops = insertSpillReloads(ops, desc)

	// Build the problem encoding
	prob := buildProblem(ops, desc)

	// Generate SMT-LIB2
	smt := prob.generateSMT()

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[vir/solver] SMT-LIB2 (%d ops, %d vars):\n%s\n",
			len(ops), len(prob.vregs), smt)
	}

	// Run Z3
	model, err := runZ3(smt, opts)
	if err != nil {
		return nil, fmt.Errorf("z3 solve: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[vir/solver] model:\n%s\n", model)
	}

	// Parse model → PIROps
	return prob.parseSolution(model, desc)
}

// ── Pre-tie move insertion ───────────────────────────────────────────────────

// insertPreTieMoves handles the case where a vreg is used as src0 of a tied
// pattern, but the vreg is also live at other instructions where it CAN'T be
// in the tied register (because another tied op uses that register).
//
// Example: v1=const, v3=const, v5=add(v1,v2), v6=add(v3,v4)
//   Both ADDs tie src0 to A. v1 and v3 can't both be A simultaneously.
//   Fix: insert v3_copy=move(v3) before the second ADD, use v3_copy as src0.
//   v3_copy is short-lived (only at the ADD instruction) so no conflict.
func insertPreTieMoves(ops []VIROp, desc *MachineDesc) []VIROp {
	// Find which ops have tied patterns
	hasTied := make([]bool, len(ops))
	for i, op := range ops {
		for _, pat := range desc.Patterns {
			if pat.Matches(op) && pat.TiedDstSrc {
				hasTied[i] = true
				break
			}
		}
	}

	// For each tied op: if src0 is also used as src0 in another tied op,
	// OR if src0 is live at another tied op's position, insert a copy.
	// Simple heuristic: for ALL tied ops, if src0 was not just defined
	// (i.e., src0's def is not the immediately preceding instruction),
	// insert a move to create a short-lived copy.

	nextVReg := 0
	for _, op := range ops {
		if op.Dst > nextVReg {
			nextVReg = op.Dst
		}
		for _, s := range op.Src {
			if s > nextVReg {
				nextVReg = s
			}
		}
	}
	nextVReg++

	// Find def positions (-1 = defined in a previous block / parameter)
	defAt := make(map[int]int)
	for i, op := range ops {
		if op.Dst > 0 {
			defAt[op.Dst] = i
		}
	}

	// Count tied ops — only insert copies if there are multiple tied ops
	tiedCount := 0
	for _, t := range hasTied {
		if t {
			tiedCount++
		}
	}
	if tiedCount <= 1 {
		return ops
	}

	var result []VIROp
	for i, op := range ops {
		if hasTied[i] && op.Src[0] > 0 {
			src0 := op.Src[0]
			def, hasDef := defAt[src0]
			// Insert pre-tie copy if:
			// - src0 defined in previous block (hasDef=false), OR
			// - src0 defined more than 1 instruction ago (might conflict)
			needsCopy := !hasDef || i-def > 1
			if needsCopy {
				copyReg := nextVReg
				nextVReg++
				result = append(result, VIROp{
					Op: OpMove, Dst: copyReg, Src: [2]int{src0, -1},
					Width: op.Width,
				})
				newOp := op
				newOp.Src[0] = copyReg
				result = append(result, newOp)
				continue
			}
		}
		result = append(result, op)
	}

	return result
}

// ── Save-before-overwrite pass ───────────────────────────────────────────────

// insertSaveMoves detects cases where a tied-dst-src pattern would destroy a
// vreg that is needed later, and inserts explicit OpMove instructions to save it.
//
// Example: v3 = sub(v1, v2); v4 = add(v3, v1)
//   SUB ties v3←v1 (both A), destroying v1. But v4 needs v1.
//   Fix: insert v1_copy = move(v1) before SUB, rewrite v4's use of v1 → v1_copy.
//
// Result: v1_copy = move(v1); v3 = sub(v1, v2); v4 = add(v3, v1_copy)
func insertSaveMoves(ops []VIROp, desc *MachineDesc) []VIROp {
	// Find which patterns are tied for each op
	hasTied := make([]bool, len(ops))
	for i, op := range ops {
		for _, pat := range desc.Patterns {
			if pat.Matches(op) && pat.TiedDstSrc {
				hasTied[i] = true
				break
			}
		}
	}

	// Find last use of each vreg
	lastUse := make(map[int]int)
	for i, op := range ops {
		for _, s := range op.Src {
			if s > 0 {
				lastUse[s] = i
			}
		}
	}

	// Next available vreg number
	nextVReg := 0
	for _, op := range ops {
		if op.Dst > nextVReg {
			nextVReg = op.Dst
		}
		for _, s := range op.Src {
			if s > nextVReg {
				nextVReg = s
			}
		}
	}
	nextVReg++

	// Build result with inserted saves
	var result []VIROp
	for i, op := range ops {
		if hasTied[i] && op.Src[0] > 0 {
			src0 := op.Src[0]
			// Is src0 used after this instruction?
			if lu, ok := lastUse[src0]; ok && lu > i {
				// src0 will be destroyed by tied pattern — insert save
				copyVReg := nextVReg
				nextVReg++

				// Insert: v_copy = move(src0)
				result = append(result, VIROp{
					Op:    OpMove,
					Dst:   copyVReg,
					Src:   [2]int{src0, -1},
					Width: op.Width,
				})

				// Rewrite all uses of src0 AFTER this instruction to v_copy
				// (but NOT this instruction's src0 — it's consumed by the tied op)
				for j := i + 1; j < len(ops); j++ {
					if ops[j].Src[0] == src0 {
						ops[j].Src[0] = copyVReg
					}
					if ops[j].Src[1] == src0 {
						ops[j].Src[1] = copyVReg
					}
				}
				// Update lastUse for the copy
				lastUse[copyVReg] = lastUse[src0]
			}
		}
		result = append(result, op)
	}

	return result
}

// ── Pressure-driven spill insertion ──────────────────────────────────────────

// maxGPRPressure is the number of GPR8 registers available for allocation.
// Z80 has A,B,C,D,E,H,L = 7, but A is often tied to ALU ops,
// so effective pressure limit is ~6 for general vregs.
const maxGPRPressure = 6

// insertSpillReloads reduces register pressure by spilling long-lived vregs
// to L2+ tiers (IXH, memory) when more than maxGPRPressure vregs are
// simultaneously live.
//
// Algorithm:
//  1. Compute liveness and find max pressure point
//  2. If pressure <= threshold, return unchanged
//  3. Pick spill candidate: vreg with longest live range and fewest uses
//  4. Insert spill (move to spill vreg) after def, reload before each use
//  5. Constrain spill vreg to L2+ via DstHint
//  6. Repeat until pressure <= threshold
func insertSpillReloads(ops []VIROp, desc *MachineDesc) []VIROp {
	for iteration := 0; iteration < 10; iteration++ {
		pressure, _ := maxPressure(ops)
		if pressure <= maxGPRPressure {
			return ops
		}

		// Find spill candidate at the max-pressure point
		candidate := pickSpillCandidate(ops)
		if candidate <= 0 {
			return ops // no candidate found
		}

		ops = spillVReg(ops, candidate, desc)
	}
	return ops
}

// maxPressure returns the maximum number of simultaneously live GPR-class
// vregs (those without a spill DstHint) and the instruction index.
func maxPressure(ops []VIROp) (int, int) {
	live := computeLiveness(ops)

	// Collect vregs that are spill-hinted (don't count toward GPR pressure)
	spillVRegs := make(map[int]bool)
	for _, op := range ops {
		if op.Dst > 0 && !op.DstHint.IsEmpty() {
			spillVRegs[op.Dst] = true
		}
	}

	maxP, maxI := 0, 0
	for i, l := range live {
		p := 0
		for v := range l.live {
			if !spillVRegs[v] {
				p++
			}
		}
		if p > maxP {
			maxP = p
			maxI = i
		}
	}
	return maxP, maxI
}

// pickSpillCandidate selects the best vreg to spill: longest live range,
// fewest uses, not involved in the current max-pressure instruction's
// direct operands, not already spilled.
func pickSpillCandidate(ops []VIROp) int {
	_, maxI := maxPressure(ops)

	// Collect already-spilled vregs
	spillVRegs := make(map[int]bool)
	for _, op := range ops {
		if op.Dst > 0 && !op.DstHint.IsEmpty() {
			spillVRegs[op.Dst] = true
		}
	}

	live := computeLiveness(ops)

	// Candidate = vreg live at maxI, not src/dst of ops[maxI], not spilled
	directVRegs := make(map[int]bool)
	op := ops[maxI]
	if op.Dst > 0 {
		directVRegs[op.Dst] = true
	}
	for _, s := range op.Src {
		if s > 0 {
			directVRegs[s] = true
		}
	}

	// Count uses per vreg
	useCount := make(map[int]int)
	defAt := make(map[int]int)
	lastUse := make(map[int]int)
	for i, o := range ops {
		if o.Dst > 0 {
			defAt[o.Dst] = i
		}
		for _, s := range o.Src {
			if s > 0 {
				useCount[s]++
				lastUse[s] = i
			}
		}
	}

	// Pick: live at maxI, not direct operand, not already spilled, longest range
	bestVReg := -1
	bestScore := -1
	for vreg := range live[maxI].live {
		if directVRegs[vreg] || spillVRegs[vreg] {
			continue
		}
		def := defAt[vreg]
		lu := lastUse[vreg]
		liveRange := lu - def
		uses := useCount[vreg]
		if uses == 0 {
			uses = 1
		}
		// Score: prefer long ranges with few uses (cheapest to spill)
		score := liveRange * 100 / uses
		if score > bestScore {
			bestScore = score
			bestVReg = vreg
		}
	}

	return bestVReg
}

// spillVReg inserts a spill after the def of `vreg` and a reload before
// each use. The spill vreg is constrained to L2+ tiers.
func spillVReg(ops []VIROp, vreg int, desc *MachineDesc) []VIROp {
	// Find def and uses
	defIdx := -1
	var useIdxs []int
	for i, op := range ops {
		if op.Dst == vreg {
			defIdx = i
		}
		for _, s := range op.Src {
			if s == vreg {
				useIdxs = append(useIdxs, i)
				break
			}
		}
	}
	if defIdx < 0 || len(useIdxs) == 0 {
		return ops
	}

	// Allocate spill vreg
	nextVReg := 0
	for _, op := range ops {
		if op.Dst > nextVReg {
			nextVReg = op.Dst
		}
		for _, s := range op.Src {
			if s > nextVReg {
				nextVReg = s
			}
		}
	}
	spillReg := nextVReg + 1

	// Determine width from def
	w := ops[defIdx].Width
	if w == 0 {
		w = 8
	}

	// Spill tier hint: prefer IXH (8T) for 8-bit, memory for 16-bit
	var spillHint LocSet
	if w <= 8 {
		spillHint = desc.LocsOfKind(LocIXHalf)
		if spillHint.IsEmpty() {
			spillHint = desc.LocsOfKind(LocMem)
		}
	} else {
		spillHint = desc.LocsOfKind(LocMem)
	}

	// Build new op list with spill after def and reload before each use
	var result []VIROp
	useSet := make(map[int]bool)
	for _, ui := range useIdxs {
		useSet[ui] = true
	}

	for i, op := range ops {
		// Insert reload before use
		if useSet[i] {
			reloadReg := nextVReg + 2
			nextVReg += 2
			result = append(result, VIROp{
				Op: OpMove, Dst: reloadReg, Src: [2]int{spillReg, -1},
				Width: w,
			})
			// Rewrite this op's uses of vreg → reloadReg
			newOp := op
			for j, s := range newOp.Src {
				if s == vreg {
					newOp.Src[j] = reloadReg
				}
			}
			result = append(result, newOp)
			continue
		}

		result = append(result, op)

		// Insert spill after def
		if i == defIdx {
			result = append(result, VIROp{
				Op: OpMove, Dst: spillReg, Src: [2]int{vreg, -1},
				Width: w, DstHint: spillHint,
			})
		}
	}

	return result
}

// ── Problem encoding ─────────────────────────────────────────────────────────

// problem represents the joint isel+regalloc problem for one basic block.
type problem struct {
	ops      []VIROp
	desc     *MachineDesc
	vregs    map[int]bool // all virtual registers referenced
	patterns [][]int      // patterns[i] = indices into desc.Patterns that match ops[i]
	liveness []livenessAt // per-instruction liveness info
}

type livenessAt struct {
	live map[int]bool // set of vregs live at this point
}

func buildProblem(ops []VIROp, desc *MachineDesc) *problem {
	p := &problem{
		ops:  ops,
		desc: desc,
	}

	// Collect all virtual registers (skip -1 and 0 — 0 is Go zero value, not a real vreg)
	p.vregs = make(map[int]bool)
	for _, op := range ops {
		if op.Dst > 0 {
			p.vregs[op.Dst] = true
		}
		for _, s := range op.Src {
			if s > 0 {
				p.vregs[s] = true
			}
		}
	}

	// Find matching patterns for each op
	p.patterns = make([][]int, len(ops))
	for i, op := range ops {
		for j := range desc.Patterns {
			if desc.Patterns[j].Matches(op) {
				p.patterns[i] = append(p.patterns[i], j)
			}
		}
	}

	// Compute liveness
	p.liveness = computeLiveness(ops)

	return p
}

// computeLiveness does backward dataflow to find live vregs at each instruction.
func computeLiveness(ops []VIROp) []livenessAt {
	n := len(ops)
	result := make([]livenessAt, n)
	for i := range result {
		result[i].live = make(map[int]bool)
	}

	// Backward pass: vreg is live from its def to its last use
	lastUse := make(map[int]int) // vreg → last instruction index using it
	for i, op := range ops {
		for _, s := range op.Src {
			if s > 0 {
				lastUse[s] = i
			}
		}
	}

	defAt := make(map[int]int) // vreg → instruction index defining it
	for i, op := range ops {
		if op.Dst > 0 {
			defAt[op.Dst] = i
		}
	}

	// Mark live ranges: from def to last use (inclusive)
	for vreg, last := range lastUse {
		def, ok := defAt[vreg]
		if !ok {
			def = 0 // function parameter, live from start
		}
		for i := def; i <= last; i++ {
			result[i].live[vreg] = true
		}
	}

	// Vregs that are defined but never used are live only at their def point
	for vreg, def := range defAt {
		if _, used := lastUse[vreg]; !used {
			result[def].live[vreg] = true
		}
	}

	return result
}

// generateSMT encodes the joint isel+regalloc problem as SMT-LIB2.
func (p *problem) generateSMT() string {
	var b strings.Builder
	b.WriteString("(set-logic QF_LIA)\n")

	// Variable: which pattern for each instruction
	for i, pats := range p.patterns {
		if len(pats) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("(declare-const pat%d Int)\n", i))
		// pat_i ∈ {patterns that match ops[i]}
		if len(pats) == 1 {
			b.WriteString(fmt.Sprintf("(assert (= pat%d %d))\n", i, pats[0]))
		} else {
			b.WriteString(fmt.Sprintf("(assert (or"))
			for _, pi := range pats {
				b.WriteString(fmt.Sprintf(" (= pat%d %d)", i, pi))
			}
			b.WriteString("))\n")
		}
	}

	// Variable: which physical location for each virtual register
	for vreg := range p.vregs {
		b.WriteString(fmt.Sprintf("(declare-const loc_v%d Int)\n", vreg))
		b.WriteString(fmt.Sprintf("(assert (and (>= loc_v%d 0) (< loc_v%d %d)))\n",
			vreg, vreg, len(p.desc.Locs)))
	}

	// DstHint constraints: spill vregs are constrained to specific tiers
	for _, op := range p.ops {
		if op.Dst > 0 && !op.DstHint.IsEmpty() {
			constraint := locSetToSMT(fmt.Sprintf("loc_v%d", op.Dst), op.DstHint)
			b.WriteString(fmt.Sprintf("(assert %s) ; DstHint\n", constraint))
		}
	}

	// Pattern → location constraints + tied operands
	// Track which vreg pairs are tied (can share a physical register)
	type vregPair struct{ a, b int }
	tied := make(map[vregPair]bool)

	for i, op := range p.ops {
		pats := p.patterns[i]
		if len(pats) == 0 {
			continue
		}

		for _, pi := range pats {
			pat := &p.desc.Patterns[pi]
			cond := fmt.Sprintf("(= pat%d %d)", i, pi)

			// If this pattern is selected, dst must be in DstLocs
			if op.Dst > 0 && !pat.DstLocs.IsEmpty() {
				locConstraint := locSetToSMT(fmt.Sprintf("loc_v%d", op.Dst), pat.DstLocs)
				b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, locConstraint))
			}

			// src0 must be in SrcLocs[0]
			if op.Src[0] > 0 && !pat.SrcLocs[0].IsEmpty() {
				locConstraint := locSetToSMT(fmt.Sprintf("loc_v%d", op.Src[0]), pat.SrcLocs[0])
				b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, locConstraint))
			}

			// src1 must be in SrcLocs[1]
			if op.Src[1] > 0 && !pat.SrcLocs[1].IsEmpty() {
				locConstraint := locSetToSMT(fmt.Sprintf("loc_v%d", op.Src[1]), pat.SrcLocs[1])
				b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, locConstraint))
			}

			// Tied operand: dst must equal src0 (same physical register)
			// Z80: ADD A,r means A = A + r — dst and src0 are both A
			if pat.TiedDstSrc && op.Dst > 0 && op.Src[0] > 0 {
				b.WriteString(fmt.Sprintf("(assert (=> %s (= loc_v%d loc_v%d)))\n",
					cond, op.Dst, op.Src[0]))
				// Record this pair as tied (skip interference for them)
				tied[vregPair{op.Dst, op.Src[0]}] = true
				tied[vregPair{op.Src[0], op.Dst}] = true
			}
		}
	}

	// Interference: simultaneously live vregs cannot share the same location
	// EXCEPT tied pairs (dst=src0 in accumulator patterns)
	emitted := make(map[vregPair]bool) // dedup interference constraints
	for i := range p.ops {
		live := p.liveness[i].live
		vregs := make([]int, 0, len(live))
		for v := range live {
			vregs = append(vregs, v)
		}
		for a := 0; a < len(vregs); a++ {
			for c := a + 1; c < len(vregs); c++ {
				va, vc := vregs[a], vregs[c]
				pair := vregPair{va, vc}
				if va > vc {
					pair = vregPair{vc, va}
				}
				if tied[pair] || emitted[pair] {
					continue
				}
				emitted[pair] = true
				b.WriteString(fmt.Sprintf("(assert (not (= loc_v%d loc_v%d)))\n", va, vc))
			}
		}
	}

	// Clobber constraints: vregs live across a clobbering instruction
	// must not be in clobbered locations
	for i, op := range p.ops {
		if op.Clobbers.IsEmpty() {
			continue
		}
		for vreg := range p.liveness[i].live {
			if vreg == op.Dst {
				continue // dst is being defined, not live-through
			}
			// vreg must NOT be in any clobbered location
			op.Clobbers.ForEach(func(loc int) bool {
				b.WriteString(fmt.Sprintf("(assert (not (= loc_v%d %d)))\n", vreg, loc))
				return true
			})
		}
	}

	// Hint constraints (soft — via cost, not hard)
	// DstHint and SrcHint from PFCCO contracts

	// Cost objective: minimize total cost
	b.WriteString("(declare-const total_cost Int)\n")
	b.WriteString("(assert (= total_cost (+\n")

	// Pattern cost
	for i, pats := range p.patterns {
		if len(pats) == 0 {
			continue
		}
		if len(pats) == 1 {
			b.WriteString(fmt.Sprintf("  %d\n", p.desc.Patterns[pats[0]].Cost))
		} else {
			// ite chain: (if (= pat_i p0) cost0 (if (= pat_i p1) cost1 ...))
			b.WriteString("  ")
			for j, pi := range pats {
				if j < len(pats)-1 {
					b.WriteString(fmt.Sprintf("(ite (= pat%d %d) %d ", i, pi, p.desc.Patterns[pi].Cost))
				} else {
					b.WriteString(fmt.Sprintf("%d", p.desc.Patterns[pi].Cost))
				}
			}
			for j := 0; j < len(pats)-1; j++ {
				b.WriteString(")")
			}
			b.WriteString("\n")
		}
	}

	// Location cost (spill tier penalties)
	for vreg := range p.vregs {
		if len(p.desc.LocCost) == 0 {
			continue
		}
		// ite chain over all locations
		b.WriteString("  ")
		for loc := 0; loc < len(p.desc.LocCost); loc++ {
			if loc < len(p.desc.LocCost)-1 {
				b.WriteString(fmt.Sprintf("(ite (= loc_v%d %d) %d ", vreg, loc, p.desc.LocCost[loc]))
			} else {
				b.WriteString(fmt.Sprintf("%d", p.desc.LocCost[loc]))
			}
		}
		for loc := 0; loc < len(p.desc.LocCost)-1; loc++ {
			b.WriteString(")")
		}
		b.WriteString("\n")
	}

	b.WriteString(")))\n")
	b.WriteString("(minimize total_cost)\n")
	b.WriteString("(check-sat)\n")
	b.WriteString("(get-model)\n")

	return b.String()
}

// locSetToSMT generates an SMT constraint that var ∈ locset.
func locSetToSMT(varName string, s LocSet) string {
	locs := make([]int, 0, s.Count())
	s.ForEach(func(i int) bool {
		locs = append(locs, i)
		return true
	})
	if len(locs) == 0 {
		return "false"
	}
	if len(locs) == 1 {
		return fmt.Sprintf("(= %s %d)", varName, locs[0])
	}
	parts := make([]string, len(locs))
	for i, loc := range locs {
		parts[i] = fmt.Sprintf("(= %s %d)", varName, loc)
	}
	return "(or " + strings.Join(parts, " ") + ")"
}

// parseSolution extracts pattern and register assignments from Z3 model.
func (p *problem) parseSolution(model string, desc *MachineDesc) ([]PIROp, error) {
	vals := parseZ3Model(model)

	var result []PIROp
	for i, op := range p.ops {
		pats := p.patterns[i]
		if len(pats) == 0 {
			// No pattern matches — emit as comment
			result = append(result, PIROp{
				Comment: fmt.Sprintf("no pattern for op %d (Op=%d)", i, op.Op),
			})
			continue
		}

		// Read pattern choice
		patKey := fmt.Sprintf("pat%d", i)
		patIdx, ok := vals[patKey]
		if !ok {
			// Default to first pattern
			patIdx = pats[0]
		}
		pat := &desc.Patterns[patIdx]

		// Read register assignments
		dstPhys := -1
		if op.Dst >= 0 {
			key := fmt.Sprintf("loc_v%d", op.Dst)
			if v, ok := vals[key]; ok {
				dstPhys = v
			}
		}

		var srcPhys [2]int
		srcPhys[0] = -1
		srcPhys[1] = -1
		for j, s := range op.Src {
			if s >= 0 {
				key := fmt.Sprintf("loc_v%d", s)
				if v, ok := vals[key]; ok {
					srcPhys[j] = v
				}
			}
		}

		result = append(result, PIROp{
			Pat:     pat,
			DstPhys: dstPhys,
			SrcPhys: srcPhys,
			Imm:     op.Imm,
			Sym:     op.Sym,
		})
	}

	return result, nil
}

// ── Z3 execution ─────────────────────────────────────────────────────────────

func runZ3(smt string, opts SolverOptions) (string, error) {
	// Write SMT to temp file
	f, err := os.CreateTemp("", "vir-solver-*.smt2")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(smt); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	// Run z3
	cmd := exec.Command(opts.Z3Path, "-T:"+strconv.Itoa(int(opts.Timeout.Seconds())), f.Name())
	out, err := cmd.CombinedOutput()
	output := string(out)

	// Z3 exits 0 for sat/unsat, non-zero for errors
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return "", fmt.Errorf("z3 exec: %w", err)
		}
	}

	if strings.Contains(output, "unsat") {
		return "", fmt.Errorf("unsatisfiable: no valid pattern+register assignment exists")
	}
	if strings.Contains(output, "unknown") {
		return "", fmt.Errorf("z3 timeout after %v", opts.Timeout)
	}
	if !strings.Contains(output, "sat") {
		return "", fmt.Errorf("unexpected z3 output: %s", output)
	}

	return output, nil
}

// parseZ3Model extracts variable assignments from Z3 model output.
// Z3 model format can be single-line or multi-line:
//
//	(define-fun pat0 () Int 3)
//	(define-fun loc_v1 () Int
//	  0)
func parseZ3Model(model string) map[string]int {
	vals := make(map[string]int)
	lines := strings.Split(model, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "(define-fun ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		name := parts[1]

		// Try single-line: (define-fun pat0 () Int 3)
		if len(parts) >= 5 {
			valStr := strings.TrimRight(parts[len(parts)-1], ")")
			if val, err := strconv.Atoi(valStr); err == nil {
				vals[name] = val
				continue
			}
		}

		// Multi-line: value is on the next line
		if i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			valStr := strings.TrimRight(nextLine, ")")
			valStr = strings.TrimSpace(valStr)
			if val, err := strconv.Atoi(valStr); err == nil {
				vals[name] = val
			}
		}
	}
	return vals
}
