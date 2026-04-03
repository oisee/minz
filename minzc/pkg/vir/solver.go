// solver.go — Unified VIR→PIR solver.
//
// Simultaneously selects instruction patterns AND physical registers for each
// VIROp in a basic block. This replaces the 5-layer pipeline (isel → WFC →
// fixInvalidZ80Template → spill_reload → validate-reject).
//
// Architecture:
//
//	Z3 solver (primary): encodes joint isel+regalloc as SMT, provably optimal.
//	WFC solver (future):  fast greedy approximation, validated against Z3.
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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/mir2"
)

// SolverOptions controls the unified solver.
type SolverOptions struct {
	Timeout time.Duration // Z3 timeout (default 5s)
	Z3Path  string        // path to z3 binary (default: "z3")
	Verbose bool          // print SMT-LIB2 and model

	// ParamLocs maps vreg → physical register index for function parameters.
	// When set, the solver constrains param vregs to these locations to
	// match the bootstrap ABI (from PBQP allocation).
	ParamLocs map[int]int

	// FuncParamLocs maps function name → (vreg → physical register index).
	// Per-function version of ParamLocs for multi-function modules.
	FuncParamLocs map[string]map[int]int

	// OptSize enables size optimizations (Grace reroll: repeated CALL patterns
	// → data table + DJNZ loop). Trades ~4T/iteration for code size reduction.
	OptSize bool

	// Strict mode: treat warnings as errors, log all implicit decisions.
	// ON by default during development. Turn OFF when --vir becomes prod-ready.
	// Catches: validateNoClobber, missing edge moves, missing move patterns,
	// clobber exclusions, table ABI mismatches, peephole skips.
	Strict bool

	// PBQPAlloc: pre-computed PBQP allocation for fallback codegen.
	// When set and VIR fails, CodegenModule uses mir2.Z80Codegen for
	// the failed function. Safety net — always correct, SDCC-quality.
	PBQPAlloc *mir2.AllocResult

	// UseGrace enables MIR2-level Grace optimization passes before VIR lowering.
	// Runs: DSE, DeadBlockArgElim, CondRetSink, SplitJoinRet, FuseAbsDiff, etc.
	// Default: false (opt-in until validated on full corpus).
	// Each eliminated MIR2 op/block saves ~88 Z3 variables (4 per vreg × 22 locs).
	UseGrace bool

	// GraceStats accumulates per-rule Grace statistics across the module.
	// Caller provides; CodegenModule writes into it.
	GraceStats *mir2.GraceStats

	// UseVIRDSE enables dead VIROp elimination after lowering (pre-solver).
	// Removes pure ops (add/sub/and/or/xor/shl/shr/neg/mul + imm variants)
	// whose dst vreg is never used in the block. Conservative: intra-block only.
	// Always safe to enable. Default: false until corpus validated.
	UseVIRDSE bool

	// UseInliner enables VIR-level inlining of small single-block callees
	// called from loop-body blocks. Default: false until validated on corpus.
	// Enable via --vir-inline flag or VIR_INLINE=1 env var.
	UseInliner bool

	// InlineMaxOps: max VIR op count for a callee to be inlined. Default: 15.
	InlineMaxOps int
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
		opts.Timeout = 30 * time.Second
	}

	// Check z3 exists
	if _, err := exec.LookPath(opts.Z3Path); err != nil {
		return nil, fmt.Errorf("z3 not found: %w", err)
	}

	// Phase 1: global locations (fast, handles ~75% of functions)
	opsCopy := make([]VIROp, len(ops))
	copy(opsCopy, ops)

	result, err := solveWithPasses(ops, desc, opts, false)
	if err == nil {
		return result, nil
	}

	// Phase 2: per-instruction locations (thorough, handles register
	// pressure and call clobber by letting vregs move between locations)
	return solvePerInstruction(opsCopy, desc, opts)
}

// solveWithPasses runs pre-solver passes and Z3.
func solveWithPasses(ops []VIROp, desc *MachineDesc, opts SolverOptions, splitCalls bool) ([]PIROp, error) {
	if splitCalls {
		ops = splitVRegsAtCalls(ops, desc)
	}

	ops = insertPreTieMoves(ops, desc)
	ops = insertSaveMoves(ops, desc)
	ops = insertSpillReloads(ops, desc)
	ops = coalesceVRegs(ops)

	prob := buildProblem(ops, desc)
	smt := prob.generateSMT()

	if os.Getenv("VIR_DEBUG_SMT") != "" {
		os.WriteFile("/tmp/vir_phase1_debug.smt2", []byte(smt), 0644)
		fmt.Fprintf(os.Stderr, "[PHASE1-SMT] %d bytes, %d ops, %d vregs\n", len(smt), len(ops), len(prob.vregs))
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[vir/solver] SMT-LIB2 (%d ops, %d vars, splitCalls=%v):\n%s\n",
			len(ops), len(prob.vregs), splitCalls, smt)
	}

	model, err := runZ3(smt, opts)
	if err != nil {
		return nil, fmt.Errorf("z3 solve: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[vir/solver] model:\n%s\n", model)
	}

	return prob.parseSolution(model, desc)
}

// ── Call vreg splitting ──────────────────────────────────────────────────────

// splitVRegsAtCalls handles the call clobber problem by splitting vregs that
// are live across CALL instructions. For each such vreg, it inserts:
//
//	v_save = move(v)     ; before call — save to IXH
//	v      = move(v_save) ; after call  — restore from IXH
//
// This splits the vreg's live range so the "before" identity can be in GPR
// and the "save" identity survives the call in IXH.
//
// Only triggers for vregs that are actually used after the call.
func splitVRegsAtCalls(ops []VIROp, desc *MachineDesc) []VIROp {
	// Find calls
	var callIdxs []int
	for i, op := range ops {
		if (op.Op == OpCall || op.Op == OpAsmBlock) && !op.Clobbers.IsEmpty() {
			callIdxs = append(callIdxs, i)
		}
	}
	if len(callIdxs) == 0 {
		return ops
	}

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

	ixhHint := desc.LocsOfKind(LocIXHalf)

	// Process calls back-to-front
	for ci := len(callIdxs) - 1; ci >= 0; ci-- {
		callIdx := callIdxs[ci]
		callOp := ops[callIdx]

		// Find vregs used AFTER this call that were defined BEFORE
		usedAfter := make(map[int]bool)
		for j := callIdx + 1; j < len(ops); j++ {
			for _, s := range ops[j].Src {
				if s > 0 {
					usedAfter[s] = true
				}
			}
		}

		defBefore := make(map[int]bool)
		for j := 0; j < callIdx; j++ {
			if ops[j].Dst > 0 {
				defBefore[ops[j].Dst] = true
			}
		}
		// Cross-block params: used but not defined in this block
		for v := range usedAfter {
			if !defBefore[v] {
				defBefore[v] = true // treat as defined before
			}
		}

		// Identify live-across vregs, sorted by uses-after-call (most used first)
		type laCand struct {
			vreg int
			uses int
		}
		var candidates []laCand
		for v := range usedAfter {
			if defBefore[v] && v != callOp.Dst {
				uses := 0
				for j := callIdx + 1; j < len(ops); j++ {
					for _, s := range ops[j].Src {
						if s == v {
							uses++
						}
					}
				}
				candidates = append(candidates, laCand{v, uses})
			}
		}
		// Sort by uses descending, break ties by vreg ID ascending (deterministic)
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].uses != candidates[j].uses {
				return candidates[i].uses > candidates[j].uses
			}
			return candidates[i].vreg < candidates[j].vreg
		})
		// Limit to 6 saves max (4 IXH + 2 unconstrained)
		if len(candidates) > 6 {
			candidates = candidates[:6]
		}
		var liveAcross []int
		for _, c := range candidates {
			liveAcross = append(liveAcross, c.vreg)
		}

		if len(liveAcross) == 0 {
			continue
		}

		// Build: [before...] [saves] [call] [restores] [after...]
		var newOps []VIROp
		newOps = append(newOps, ops[:callIdx]...)

		// Save each live-across vreg to call-safe location
		saveMap := make(map[int]int) // original → save vreg
		ixhUsed := 0
		for _, v := range liveAcross {
			sv := nextVReg
			nextVReg++
			saveMap[v] = sv

			w := 8 // default
			for _, op := range ops {
				if op.Dst == v && op.Width > 0 {
					w = op.Width
					break
				}
			}

			// 8-bit → IXH (4 slots), overflow/16-bit → no hint (let Z3 pick)
			hint := LocSet(0)
			if w <= 8 && ixhUsed < 4 {
				hint = ixhHint
				ixhUsed++
			}

			newOps = append(newOps, VIROp{
				Op: OpMove, Dst: sv, Src: [2]int{v, -1},
				Width: w, DstHint: hint,
			})
		}

		// The call
		newOps = append(newOps, ops[callIdx])

		// Restore: reload from save vreg back to original
		for _, v := range liveAcross {
			sv := saveMap[v]
			w := 8
			for _, op := range ops {
				if op.Dst == v && op.Width > 0 {
					w = op.Width
					break
				}
			}
			newOps = append(newOps, VIROp{
				Op: OpMove, Dst: v, Src: [2]int{sv, -1}, Width: w,
			})
		}

		newOps = append(newOps, ops[callIdx+1:]...)
		ops = newOps
	}

	return ops
}

// ── Call save/restore insertion (legacy, disabled) ──────────────────────────

// insertCallSaveRestore detects vregs that are live across a CALL instruction
// and inserts save (before call) + restore (after call) moves.
//
// CALLs clobber A,B,C,D,E,H,L,F — any vreg in GPR will be destroyed.
// Strategy: for each live-across vreg, insert:
//
//	v_saved = OpMove(vreg)     ; before call — constrained to IXH (call-safe)
//	vreg    = OpMove(v_saved)  ; after call  — reload from IXH
//
// IXH/IXL/IYH/IYL survive calls (Z80 convention: IX/IY are callee-saved).
// If we run out of IXH slots (4), fall back to PUSH/POP via stack.
func insertCallSaveRestore(ops []VIROp, desc *MachineDesc) []VIROp {
	// Find call positions
	var callIdxs []int
	for i, op := range ops {
		if op.Op == OpCall || op.Op == OpAsmBlock {
			callIdxs = append(callIdxs, i)
		}
	}
	if len(callIdxs) == 0 {
		return ops
	}

	// Only insert saves if the block has enough vregs to warrant it.
	// Blocks with few vregs solve fine without saves — adding them just
	// creates unnecessary pressure.
	uniqueVRegs := make(map[int]bool)
	for _, op := range ops {
		if op.Dst > 0 {
			uniqueVRegs[op.Dst] = true
		}
		for _, s := range op.Src {
			if s > 0 {
				uniqueVRegs[s] = true
			}
		}
	}
	if len(uniqueVRegs) <= maxGPRPressure+2 {
		return ops // low vreg count, no need for call saves
	}

	// Compute liveness
	live := computeLiveness(ops)

	// Find def and last-use per vreg
	defAt := make(map[int]int)
	lastUse := make(map[int]int)
	for i, op := range ops {
		if op.Dst > 0 {
			if _, ok := defAt[op.Dst]; !ok {
				defAt[op.Dst] = i
			}
		}
		for _, s := range op.Src {
			if s > 0 {
				lastUse[s] = i
			}
		}
	}

	// Next vreg number
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

	// IXH hint for 8-bit, stack for 16-bit
	ixhHint := desc.LocsOfKind(LocIXHalf)
	stackHint := desc.LocsOfKind(LocStack)
	if stackHint.IsEmpty() {
		stackHint = desc.LocsOfKind(LocMem)
	}

	// Process each call: find live-across vregs and insert save/restore
	// Work backwards to avoid index invalidation
	type saveInfo struct {
		vreg    int
		saveReg int
		width   int
		hint    LocSet
	}

	for ci := len(callIdxs) - 1; ci >= 0; ci-- {
		callIdx := callIdxs[ci]
		callOp := ops[callIdx]

		// Find vregs live across this call:
		// - defined before callIdx (or cross-block param)
		// - used after callIdx
		// - not the call's own dst
		type candidate struct {
			vreg  int
			width int
			uses  int // uses after call
		}
		var candidates []candidate

		for vreg := range live[callIdx].live {
			if vreg == callOp.Dst {
				continue
			}
			// Count uses after the call
			usesAfter := 0
			for j := callIdx + 1; j < len(ops); j++ {
				for _, s := range ops[j].Src {
					if s == vreg {
						usesAfter++
					}
				}
			}
			if usesAfter == 0 {
				continue
			}

			w := 8
			for _, op := range ops {
				if op.Dst == vreg && op.Width > 0 {
					w = op.Width
					break
				}
			}
			candidates = append(candidates, candidate{vreg, w, usesAfter})
		}

		// Sort by uses descending (save most-used first), break ties by vreg ID (deterministic)
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].uses != candidates[j].uses {
				return candidates[i].uses > candidates[j].uses
			}
			return candidates[i].vreg < candidates[j].vreg
		})

		// Limit saves: at most 4 for 8-bit (IXH slots), 2 for 16-bit (memory)
		maxSaves := 4
		if len(candidates) > maxSaves {
			candidates = candidates[:maxSaves]
		}

		var toSave []saveInfo
		ixhUsed := 0
		for _, c := range candidates {
			hint := stackHint
			if c.width <= 8 && ixhUsed < 4 {
				hint = ixhHint
				ixhUsed++
			}
			saveReg := nextVReg
			nextVReg++
			toSave = append(toSave, saveInfo{
				vreg: c.vreg, saveReg: saveReg, width: c.width, hint: hint,
			})
		}

		if len(toSave) == 0 {
			continue
		}

		// Build new ops: [...before...] [saves] [call] [restores] [...after...]
		var newOps []VIROp
		newOps = append(newOps, ops[:callIdx]...)

		// Insert saves before call
		for _, s := range toSave {
			newOps = append(newOps, VIROp{
				Op: OpMove, Dst: s.saveReg, Src: [2]int{s.vreg, -1},
				Width: s.width, DstHint: s.hint,
			})
		}

		// The call itself
		newOps = append(newOps, ops[callIdx])

		// Insert restores after call
		for _, s := range toSave {
			newOps = append(newOps, VIROp{
				Op: OpMove, Dst: s.vreg, Src: [2]int{s.saveReg, -1},
				Width: s.width,
			})
		}

		// Rest of ops
		newOps = append(newOps, ops[callIdx+1:]...)

		ops = newOps

		// Recompute liveness for the updated ops
		live = computeLiveness(ops)
	}

	return ops
}

// ── Pre-tie move insertion ───────────────────────────────────────────────────

// insertPreTieMoves handles the case where a vreg is used as src0 of a tied
// pattern, but the vreg is also live at other instructions where it CAN'T be
// in the tied register (because another tied op uses that register).
//
// Example: v1=const, v3=const, v5=add(v1,v2), v6=add(v3,v4)
//
//	Both ADDs tie src0 to A. v1 and v3 can't both be A simultaneously.
//	Fix: insert v3_copy=move(v3) before the second ADD, use v3_copy as src0.
//	v3_copy is short-lived (only at the ADD instruction) so no conflict.
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

	// Only insert a pre-tie copy when src0 stays live across ANOTHER tied op
	// whose src0 is a different vreg. That is the real "A register conflict"
	// case this helper is meant to address.
	live := computeLiveness(ops)

	var result []VIROp
	for i, op := range ops {
		if hasTied[i] && op.Src[0] > 0 {
			src0 := op.Src[0]
			needsCopy := false
			for j, otherTied := range hasTied {
				if !otherTied || j == i {
					continue
				}
				otherSrc0 := ops[j].Src[0]
				if otherSrc0 <= 0 || otherSrc0 == src0 {
					continue
				}
				if ops[j].Dst == src0 {
					continue // src0 is born here, not live-through here
				}
				if live[j].live[src0] {
					needsCopy = true
					break
				}
			}
			if needsCopy {
				copyReg := nextVReg
				nextVReg++
				result = append(result, VIROp{
					Op: OpMove, Dst: copyReg, Src: [2]int{src0, -1},
					Width: op.Width,
				})
				newOp := op
				newOp.Src[0] = copyReg
				if op.Src[1] == src0 {
					newOp.Src[1] = copyReg
				}
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
//
//	SUB ties v3←v1 (both A), destroying v1. But v4 needs v1.
//	Fix: insert v1_copy = move(v1) before SUB, rewrite v4's use of v1 → v1_copy.
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
		if score > bestScore || (score == bestScore && vreg < bestVReg) {
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

// ── Vreg coalescing ─────────────────────────────────────────────────────────

// coalesceVRegs merges non-interfering vregs that have compatible widths,
// reducing the total number of unique vregs the solver must assign locations to.
//
// This is critical for Z80 where only 7 GPR + 4 IXH = 11 locations exist.
// Short-lived vregs (constants, reloads) often don't overlap and can share.
//
// Algorithm:
//  1. Compute live ranges (def..lastUse) per vreg
//  2. Collect vreg info (width, DstHint)
//  3. Sort by live range length descending (long-lived first = merge targets)
//  4. Greedy: for each short-lived vreg, try to merge into a non-interfering
//     long-lived one with compatible width and no conflicting DstHint
//  5. Rewrite all references
func coalesceVRegs(ops []VIROp) []VIROp {
	type vregInfo struct {
		id      int
		width   int
		def     int // first instruction index
		lastUse int // last instruction index
		hint    LocSet
	}

	// Collect vreg info
	info := make(map[int]*vregInfo)
	for i, op := range ops {
		if op.Dst > 0 {
			if _, ok := info[op.Dst]; !ok {
				info[op.Dst] = &vregInfo{id: op.Dst, width: op.Width, def: i, lastUse: i, hint: op.DstHint}
			} else {
				info[op.Dst].def = i // update def (might be redefined)
			}
		}
		for _, s := range op.Src {
			if s > 0 {
				if vi, ok := info[s]; ok {
					if i > vi.lastUse {
						vi.lastUse = i
					}
				} else {
					// Used but not defined in this block (cross-block param)
					info[s] = &vregInfo{id: s, width: op.Width, def: -1, lastUse: i}
				}
			}
		}
	}

	if len(info) <= maxGPRPressure {
		return ops // already fits, no coalescing needed
	}

	// Build sorted list: longer-lived first (merge targets)
	vregs := make([]*vregInfo, 0, len(info))
	for _, vi := range info {
		vregs = append(vregs, vi)
	}
	// Sort by live range length descending, break ties by vreg ID ascending
	// (deterministic: immune to Go map iteration order)
	sort.Slice(vregs, func(i, j int) bool {
		li := vregs[i].lastUse - vregs[i].def
		lj := vregs[j].lastUse - vregs[j].def
		if li != lj {
			return li > lj // longer-lived first
		}
		return vregs[i].id < vregs[j].id // stable tie-break
	})

	// Build proper interference from liveness (instruction-level)
	live := computeLiveness(ops)
	interferenceSet := make(map[[2]int]bool)
	for _, l := range live {
		ids := make([]int, 0, len(l.live))
		for v := range l.live {
			ids = append(ids, v)
		}
		for x := 0; x < len(ids); x++ {
			for y := x + 1; y < len(ids); y++ {
				a, b := ids[x], ids[y]
				if a > b {
					a, b = b, a
				}
				interferenceSet[[2]int{a, b}] = true
			}
		}
	}

	interferes := func(a, b *vregInfo) bool {
		x, y := a.id, b.id
		if x > y {
			x, y = y, x
		}
		return interferenceSet[[2]int{x, y}]
	}

	// Compatible: same width, no conflicting hints
	compatible := func(a, b *vregInfo) bool {
		if a.width != b.width && a.width != 0 && b.width != 0 {
			return false
		}
		if !a.hint.IsEmpty() && !b.hint.IsEmpty() && a.hint.And(b.hint).IsEmpty() {
			return false // conflicting hints
		}
		return true
	}

	// Greedy merge: for each short-lived vreg, find a non-interfering partner
	mergeMap := make(map[int]int) // v_b → v_a (b gets rewritten to a)
	merged := make(map[int]bool)  // vregs that have been merged into something

	for i := len(vregs) - 1; i >= 0; i-- { // start from shortest-lived
		b := vregs[i]
		if merged[b.id] {
			continue
		}
		for j := 0; j < i; j++ { // try to merge into longer-lived
			a := vregs[j]
			if merged[a.id] {
				continue
			}
			if !compatible(a, b) {
				continue
			}
			if !interferes(a, b) {
				// Merge b → a
				mergeMap[b.id] = a.id
				merged[b.id] = true
				// Extend a's live range to cover b
				if b.def >= 0 && (a.def < 0 || b.def < a.def) {
					a.def = b.def
				}
				if b.lastUse > a.lastUse {
					a.lastUse = b.lastUse
				}
				// Merge hints
				if !b.hint.IsEmpty() {
					a.hint = a.hint.Or(b.hint)
				}
				break
			}
		}
	}

	if len(mergeMap) == 0 {
		return ops // nothing to coalesce
	}

	// Resolve transitive merges: if a→b and b→c, then a→c
	resolve := func(v int) int {
		for {
			if target, ok := mergeMap[v]; ok {
				v = target
			} else {
				return v
			}
		}
	}

	// Rewrite all vreg references
	result := make([]VIROp, len(ops))
	for i, op := range ops {
		result[i] = op
		if op.Dst > 0 {
			result[i].Dst = resolve(op.Dst)
		}
		for j, s := range op.Src {
			if s > 0 {
				result[i].Src[j] = resolve(s)
			}
		}
	}

	// Remove self-moves created by coalescing (move v→v)
	var cleaned []VIROp
	for _, op := range result {
		if op.Op == OpMove && op.Dst > 0 && op.Src[0] == op.Dst {
			continue // self-move, skip
		}
		cleaned = append(cleaned, op)
	}

	return cleaned
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

	// Vregs that are defined but never used are live only at their def point.
	// Exception: OpCmp/OpCmpImm/OpBitTest produce FLAGS that are consumed by the branch
	// terminator (which is outside VIR ops). These must NOT be propagated to
	// successor blocks via liveness — FLAGS can't be moved across block boundaries.
	for vreg, def := range defAt {
		if _, used := lastUse[vreg]; !used {
			op := ops[def]
			if op.Op == OpCmp || op.Op == OpCmpImm || op.Op == OpBitTest {
				continue // FLAGS consumed by MIR2 branch terminator, not by successor VIROps
			}
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
	// Sort vregs for deterministic Z3 encoding (Go map iteration is random)
	sortedVRegs := make([]int, 0, len(p.vregs))
	for vreg := range p.vregs {
		sortedVRegs = append(sortedVRegs, vreg)
	}
	sort.Ints(sortedVRegs)

	for _, vreg := range sortedVRegs {
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
		// SrcHint: hard constraint — we know where the source vreg lives.
		for j, s := range op.Src {
			if s > 0 && !op.SrcHint[j].IsEmpty() {
				constraint := locSetToSMT(fmt.Sprintf("loc_v%d", s), op.SrcHint[j])
				b.WriteString(fmt.Sprintf("(assert %s) ; SrcHint\n", constraint))
			}
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

	// OpMove coalescing: src and dst of OpMove can share a register (LD A,A is nop).
	for _, op := range p.ops {
		if op.Op == OpMove && op.Dst > 0 && op.Src[0] > 0 {
			tied[vregPair{op.Dst, op.Src[0]}] = true
			tied[vregPair{op.Src[0], op.Dst}] = true
		}
	}

	// Interference: simultaneously live vregs cannot share the same location
	// EXCEPT tied pairs (dst=src0 in accumulator patterns) and move-coalesced pairs
	emitted := make(map[vregPair]bool) // dedup interference constraints
	for i := range p.ops {
		live := p.liveness[i].live
		vregs := make([]int, 0, len(live))
		for v := range live {
			vregs = append(vregs, v)
		}
		sort.Ints(vregs) // deterministic interference pair ordering
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
				// Pair aliasing: HL(9) conflicts with H(5)/L(6), etc.
				for _, alias := range pairAliases {
					b.WriteString(fmt.Sprintf("(assert (=> (= loc_v%d %d) (and (not (= loc_v%d %d)) (not (= loc_v%d %d)))))\n",
						va, alias.pair, vc, alias.hi, vc, alias.lo))
					b.WriteString(fmt.Sprintf("(assert (=> (= loc_v%d %d) (and (not (= loc_v%d %d)) (not (= loc_v%d %d)))))\n",
						vc, alias.pair, va, alias.hi, va, alias.lo))
				}
			}
		}
	}

	// Clobber constraints: vregs live across a clobbering instruction
	// must not be in clobbered locations
	for i, op := range p.ops {
		if op.Clobbers.IsEmpty() {
			continue
		}
		// Consumed vregs: dst + srcs + asm ins/outs — read/written at this instruction
		consumed := make(map[int]bool)
		if op.Dst > 0 {
			consumed[op.Dst] = true
		}
		for _, s := range op.Src {
			if s > 0 {
				consumed[s] = true
			}
		}
		if op.Op == OpAsmBlock {
			for _, v := range op.AsmIns {
				consumed[v] = true
			}
			for _, v := range op.AsmOuts {
				consumed[v] = true
			}
		}

		clobberVRegs := make([]int, 0, len(p.liveness[i].live))
		for vreg := range p.liveness[i].live {
			clobberVRegs = append(clobberVRegs, vreg)
		}
		sort.Ints(clobberVRegs)
		for _, vreg := range clobberVRegs {
			if consumed[vreg] {
				continue // consumed by this instruction, not live-through
			}
			// vreg must NOT be in any clobbered location
			op.Clobbers.ForEach(func(loc int) bool {
				b.WriteString(fmt.Sprintf("(assert (not (= loc_v%d %d)))\n", vreg, loc))
				return true
			})
		}
	}

	// Hard DstHint + SrcHint: params MUST be in their ABI registers.
	// Phase 1 (global): one location per vreg lifetime. If param is
	// pinned to C and CMP needs A → Phase 1 unsat → falls to Phase 2
	// which handles the move via per-instruction variables.
	for _, op := range p.ops {
		if op.Dst > 0 && !op.DstHint.IsEmpty() {
			c := locSetToSMT(fmt.Sprintf("loc_v%d", op.Dst), op.DstHint)
			b.WriteString(fmt.Sprintf("(assert %s) ; hard DstHint\n", c))
		}
		for _, s := range op.Src {
			if s > 0 {
				for j := range op.SrcHint {
					if op.Src[j] == s && !op.SrcHint[j].IsEmpty() {
						c := locSetToSMT(fmt.Sprintf("loc_v%d", s), op.SrcHint[j])
						b.WriteString(fmt.Sprintf("(assert %s) ; hard SrcHint\n", c))
					}
				}
			}
		}
	}

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
	for _, vreg := range sortedVRegs {
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

		pir := PIROp{
			Pat:     pat,
			DstPhys: dstPhys,
			SrcPhys: srcPhys,
			Imm:     op.Imm,
			Sym:     op.Sym,
		}
		if op.Op == OpAsmBlock {
			pir.AsmText = op.AsmTemplate
		}
		result = append(result, pir)
	}

	return result, nil
}

// ── Per-instruction location solver ──────────────────────────────────────────

// solvePerInstruction uses per-instruction location variables:
// loc_v{vreg}_i{inst} — each vreg can be in a different location at each
// instruction point. When locations differ between consecutive instructions,
// a move cost is added to the objective.
//
// This handles:
// - Call clobber: vreg in GPR before call, IXH across call, GPR after
// - Register pressure: vreg can spill to IXH temporarily
// - HL contention: vreg moves between HL and DE as needed
func solvePerInstruction(ops []VIROp, desc *MachineDesc, opts SolverOptions) ([]PIROp, error) {
	// Apply standard pre-solver passes (except coalescing — per-inst handles it natively)
	ops = insertPreTieMoves(ops, desc)
	ops = insertSaveMoves(ops, desc)

	prob := buildProblem(ops, desc)
	smt := generateSMTPerInst(prob)

	if os.Getenv("VIR_DEBUG_SMT") != "" {
		suffix := fmt.Sprintf("%d", len(smt))
		os.WriteFile(fmt.Sprintf("/tmp/vir_phase2_%s.smt2", suffix), []byte(smt), 0644)
		fmt.Fprintf(os.Stderr, "[PHASE2-SMT] %d bytes, %d ops → /tmp/vir_phase2_%s.smt2\n", len(smt), len(ops), suffix)
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[vir/solver] Per-inst SMT (%d ops, %d vregs):\n%s\n",
			len(ops), len(prob.vregs), smt)
	}

	model, err := runZ3(smt, opts)
	if err != nil {
		return nil, fmt.Errorf("z3 per-inst solve: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "[vir/solver] per-inst model:\n%s\n", model)
	}

	return parsePerInstSolution(prob, model, desc)
}

// generateSMTPerInst creates SMT-LIB2 with per-instruction location variables.
func generateSMTPerInst(p *problem) string {
	var b strings.Builder
	b.WriteString("(set-logic QF_LIA)\n")
	nLocs := len(p.desc.Locs)

	// For each instruction: which pattern?
	for i, pats := range p.patterns {
		if len(pats) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("(declare-const pat%d Int)\n", i))
		if len(pats) == 1 {
			b.WriteString(fmt.Sprintf("(assert (= pat%d %d))\n", i, pats[0]))
		} else {
			b.WriteString("(assert (or")
			for _, pi := range pats {
				b.WriteString(fmt.Sprintf(" (= pat%d %d)", i, pi))
			}
			b.WriteString("))\n")
		}
	}

	// Per-instruction location variables: loc_v{vreg}_i{inst}
	// Declare for: vregs live at each instruction + vregs referenced by each instruction
	type vregAtInst struct {
		vreg int
		inst int
	}
	vars := make(map[vregAtInst]bool)

	ensureVar := func(v, i int) {
		key := vregAtInst{v, i}
		if !vars[key] {
			vars[key] = true
			b.WriteString(fmt.Sprintf("(declare-const lv%d_i%d Int)\n", v, i))
			b.WriteString(fmt.Sprintf("(assert (and (>= lv%d_i%d 0) (< lv%d_i%d %d)))\n",
				v, i, v, i, nLocs))
		}
	}

	// From liveness (sorted for deterministic Z3 encoding)
	for i, l := range p.liveness {
		liveVRegs := make([]int, 0, len(l.live))
		for v := range l.live {
			liveVRegs = append(liveVRegs, v)
		}
		sort.Ints(liveVRegs)
		for _, v := range liveVRegs {
			ensureVar(v, i)
		}
	}

	// From instruction operands (dst/src may not be in liveness if short-lived)
	for i, op := range p.ops {
		if op.Dst > 0 {
			ensureVar(op.Dst, i)
		}
		for _, s := range op.Src {
			if s > 0 {
				ensureVar(s, i)
			}
		}
	}

	// Pattern → location constraints (use instruction-local variables)
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

			if op.Dst > 0 && !pat.DstLocs.IsEmpty() {
				c := locSetToSMT(fmt.Sprintf("lv%d_i%d", op.Dst, i), pat.DstLocs)
				b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, c))
			}

			if op.Src[0] > 0 && !pat.SrcLocs[0].IsEmpty() {
				c := locSetToSMT(fmt.Sprintf("lv%d_i%d", op.Src[0], i), pat.SrcLocs[0])
				b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, c))
			}

			if op.Src[1] > 0 && !pat.SrcLocs[1].IsEmpty() {
				c := locSetToSMT(fmt.Sprintf("lv%d_i%d", op.Src[1], i), pat.SrcLocs[1])
				b.WriteString(fmt.Sprintf("(assert (=> %s %s))\n", cond, c))
			}

			// Tied: dst = src0 at this instruction
			if pat.TiedDstSrc && op.Dst > 0 && op.Src[0] > 0 {
				b.WriteString(fmt.Sprintf("(assert (=> %s (= lv%d_i%d lv%d_i%d)))\n",
					cond, op.Dst, i, op.Src[0], i))
				tied[vregPair{op.Dst, op.Src[0]}] = true
				tied[vregPair{op.Src[0], op.Dst}] = true
			}
		}
	}

	// OpMove coalescing: src and dst can share a register (LD A,A is nop)
	for _, op := range p.ops {
		if op.Op == OpMove && op.Dst > 0 && op.Src[0] > 0 {
			tied[vregPair{op.Dst, op.Src[0]}] = true
			tied[vregPair{op.Src[0], op.Dst}] = true
		}
	}

	// Interference: at each instruction, live vregs must be in different locations
	emitted := make(map[[3]int]bool) // dedup: [inst, va, vb]
	for i := range p.ops {
		live := p.liveness[i].live
		vregs := make([]int, 0, len(live))
		for v := range live {
			vregs = append(vregs, v)
		}
		sort.Ints(vregs) // deterministic interference pair ordering
		for a := 0; a < len(vregs); a++ {
			for c := a + 1; c < len(vregs); c++ {
				va, vc := vregs[a], vregs[c]
				if tied[vregPair{va, vc}] {
					continue
				}
				key := [3]int{i, va, vc}
				if va > vc {
					key = [3]int{i, vc, va}
				}
				if emitted[key] {
					continue
				}
				emitted[key] = true
				b.WriteString(fmt.Sprintf("(assert (not (= lv%d_i%d lv%d_i%d)))\n",
					va, i, vc, i))
				// Pair aliasing
				for _, alias := range pairAliases {
					b.WriteString(fmt.Sprintf("(assert (=> (= lv%d_i%d %d) (and (not (= lv%d_i%d %d)) (not (= lv%d_i%d %d)))))\n",
						va, i, alias.pair, vc, i, alias.hi, vc, i, alias.lo))
					b.WriteString(fmt.Sprintf("(assert (=> (= lv%d_i%d %d) (and (not (= lv%d_i%d %d)) (not (= lv%d_i%d %d)))))\n",
						vc, i, alias.pair, va, i, alias.hi, va, i, alias.lo))
				}
			}
		}
	}

	// Clobber constraints: at clobbering instructions, live-through vregs
	// must NOT be in clobbered locations
	for i, op := range p.ops {
		if op.Clobbers.IsEmpty() {
			continue
		}
		consumed := make(map[int]bool)
		if op.Dst > 0 {
			consumed[op.Dst] = true
		}
		for _, s := range op.Src {
			if s > 0 {
				consumed[s] = true
			}
		}
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
			op.Clobbers.ForEach(func(loc int) bool {
				b.WriteString(fmt.Sprintf("(assert (not (= lv%d_i%d %d)))\n", vreg, i, loc))
				return true
			})
		}
	}

	// DstHint and SrcHint hard constraints
	for i, op := range p.ops {
		if op.Dst > 0 && !op.DstHint.IsEmpty() {
			c := locSetToSMT(fmt.Sprintf("lv%d_i%d", op.Dst, i), op.DstHint)
			b.WriteString(fmt.Sprintf("(assert %s) ; DstHint\n", c))
		}
		// SrcHint: hard for pin/return moves (DstHint set), soft for others
		for j, s := range op.Src {
			if s > 0 && !op.SrcHint[j].IsEmpty() && !op.DstHint.IsEmpty() {
				if _, ok := vars[vregAtInst{s, i}]; ok {
					c := locSetToSMT(fmt.Sprintf("lv%d_i%d", s, i), op.SrcHint[j])
					b.WriteString(fmt.Sprintf("(assert %s) ; SrcHint hard (pin/ret)\n", c))
				}
			}
		}
	}

	// Cost objective
	b.WriteString("(declare-const total_cost Int)\n")
	b.WriteString("(assert (= total_cost (+\n  0\n") // start with 0 for valid syntax

	// Pattern cost
	for i, pats := range p.patterns {
		if len(pats) == 0 {
			continue
		}
		if len(pats) == 1 {
			b.WriteString(fmt.Sprintf("  %d\n", p.desc.Patterns[pats[0]].Cost))
		} else {
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

	// Move cost: for each vreg, for each consecutive instruction pair where
	// it's live at both, add penalty if location changes (cost = 4T per move)
	moveCost := 4
	sortedVRegsPI := make([]int, 0, len(p.vregs))
	for vreg := range p.vregs {
		sortedVRegsPI = append(sortedVRegsPI, vreg)
	}
	sort.Ints(sortedVRegsPI)
	for _, vreg := range sortedVRegsPI {
		for i := 0; i < len(p.ops)-1; i++ {
			if !p.liveness[i].live[vreg] || !p.liveness[i+1].live[vreg] {
				continue
			}
			// (ite (= lv_i lv_i+1) 0 moveCost)
			b.WriteString(fmt.Sprintf("  (ite (= lv%d_i%d lv%d_i%d) 0 %d)\n",
				vreg, i, vreg, i+1, moveCost))
		}
	}

	// DstHint: HARD at first live instruction for each param vreg.
	// Per-instruction solver can move the vreg to a different register
	// at later instructions (paying 4T move cost). But at entry, the
	// vreg MUST be in the ABI register for caller compatibility.
	paramHardDone := make(map[int]bool)
	for i, op := range p.ops {
		if op.Dst > 0 && !op.DstHint.IsEmpty() && !paramHardDone[op.Dst] {
			if _, ok := vars[vregAtInst{op.Dst, i}]; ok {
				c := locSetToSMT(fmt.Sprintf("lv%d_i%d", op.Dst, i), op.DstHint)
				b.WriteString(fmt.Sprintf("(assert %s) ; hard param at i%d\n", c, i))
				paramHardDone[op.Dst] = true
			}
		}
	}

	// SrcHint: HARD at first occurrence of each param vreg as source.
	// NOTE: must be OUTSIDE the cost expression (which hasn't closed yet).
	// We'll collect them and emit after the cost expression closes.
	var hardSrcHints []string
	srcHintDone := make(map[int]bool)
	if os.Getenv("VIR_DEBUG_SMT") != "" {
		for i, op := range p.ops {
			for j := range op.SrcHint {
				if !op.SrcHint[j].IsEmpty() {
					fmt.Fprintf(os.Stderr, "[PHASE2-HINT] op%d src[%d]=v%d SrcHint=%v\n", i, j, op.Src[j], op.SrcHint[j])
				}
			}
		}
	}
	for i, op := range p.ops {
		for j, s := range op.Src {
			if s > 0 && !op.SrcHint[j].IsEmpty() && !srcHintDone[s] {
				if os.Getenv("VIR_DEBUG_SMT") != "" {
					_, exists := vars[vregAtInst{s, i}]
					fmt.Fprintf(os.Stderr, "[PHASE2-HARD] v%d at i%d: exists=%v\n", s, i, exists)
				}
				if _, ok := vars[vregAtInst{s, i}]; ok {
					c := locSetToSMT(fmt.Sprintf("lv%d_i%d", s, i), op.SrcHint[j])
					hardSrcHints = append(hardSrcHints, fmt.Sprintf("(assert %s) ; hard SrcHint v%d at i%d\n", c, s, i))
					if os.Getenv("VIR_DEBUG_SMT") != "" {
						fmt.Fprintf(os.Stderr, "[PHASE2-ASSERT] (assert %s) for v%d\n", c, s)
					}
					srcHintDone[s] = true
				}
			}
		}
	}

	// Soft hints for remaining instructions
	for i, op := range p.ops {
		for j, s := range op.Src {
			if s > 0 && !op.SrcHint[j].IsEmpty() {
				if _, ok := vars[vregAtInst{s, i}]; ok {
					op.SrcHint[j].ForEach(func(loc int) bool {
						b.WriteString(fmt.Sprintf("  (ite (= lv%d_i%d %d) 0 8) ; soft SrcHint\n",
							s, i, loc))
						return false
					})
				}
			}
		}
	}

	// Soft SrcHint: prefer param vregs in their ABI registers (cost penalty)
	for i, op := range p.ops {
		for j, s := range op.Src {
			if s > 0 && !op.SrcHint[j].IsEmpty() {
				if _, ok := vars[vregAtInst{s, i}]; ok {
					op.SrcHint[j].ForEach(func(loc int) bool {
						b.WriteString(fmt.Sprintf("  (ite (= lv%d_i%d %d) 0 8) ; soft SrcHint\n",
							s, i, loc))
						return false
					})
				}
			}
		}
	}

	b.WriteString(")))\n") // close cost expression

	// Emit hard SrcHint constraints (collected above, outside cost ITE)
	for _, h := range hardSrcHints {
		b.WriteString(h)
	}

	// For large per-instruction problems, skip minimize.
	// Z3's opt module returns "unknown" quickly on complex ITE chains.
	// Satisfiability alone gives correct code (not provably optimal).
	totalVars := len(vars)
	if totalVars > 100 {
		b.WriteString("(check-sat)\n")
	} else {
		b.WriteString("(minimize total_cost)\n")
		b.WriteString("(check-sat)\n")
	}
	b.WriteString("(get-model)\n")

	return b.String()
}

// parsePerInstSolution extracts pattern and register assignments from Z3 model
// for the per-instruction encoding. Uses the location at the instruction
// where the vreg is defined/used for the PIROp.
func parsePerInstSolution(p *problem, model string, desc *MachineDesc) ([]PIROp, error) {
	vals := parseZ3Model(model)

	var raw []PIROp // before move insertion
	for i, op := range p.ops {
		pats := p.patterns[i]
		if len(pats) == 0 {
			raw = append(raw, PIROp{
				Comment: fmt.Sprintf("no pattern for op %d (Op=%d)", i, op.Op),
			})
			continue
		}

		patKey := fmt.Sprintf("pat%d", i)
		patIdx, ok := vals[patKey]
		if !ok {
			patIdx = pats[0]
		}
		pat := &desc.Patterns[patIdx]

		dstPhys := -1
		if op.Dst > 0 {
			key := fmt.Sprintf("lv%d_i%d", op.Dst, i)
			if v, ok := vals[key]; ok {
				dstPhys = v
			}
		}

		var srcPhys [2]int
		srcPhys[0] = -1
		srcPhys[1] = -1
		for j, s := range op.Src {
			if s > 0 {
				key := fmt.Sprintf("lv%d_i%d", s, i)
				if v, ok := vals[key]; ok {
					srcPhys[j] = v
				}
			}
		}

		pir := PIROp{
			Pat:     pat,
			DstPhys: dstPhys,
			SrcPhys: srcPhys,
			Imm:     op.Imm,
			Sym:     op.Sym,
		}
		if op.Op == OpAsmBlock {
			pir.AsmText = op.AsmTemplate
		}
		raw = append(raw, pir)
	}

	final := insertPerInstMoves(raw, p, vals, desc)
	return final, nil
}

// findMovePattern finds a move pattern for source → dest physical registers.
// Tries exact-width first, then cross-width (truncation/extension) patterns.
// Skips EX-based patterns (swaps) — inter-instruction moves must be copies.
// Falls back to pair→half mapping when the solver assigns an 8-bit vreg to a
// 16-bit pair location (e.g., IY→A: try IYL→A instead).
// Returns the pattern AND remapped src/dst via the pointer args (if non-nil).
func findMovePattern(desc *MachineDesc, srcPhys, dstPhys int) *Pattern {
	_, pat := findMovePatternRemap(desc, srcPhys, dstPhys)
	return pat
}

// findMovePatternRemap returns (remappedSrc, pattern) or (remappedDst, pattern).
// The caller should use the remapped values for PIROp emission.
func findMovePatternRemap(desc *MachineDesc, srcPhys, dstPhys int) (int, *Pattern) {
	pat := findMovePatternDirect(desc, srcPhys, dstPhys)
	if pat != nil {
		return srcPhys, pat
	}
	halfMap := map[string]string{
		"BC": "C", "DE": "E", "HL": "L", "IX": "IXL", "IY": "IYL",
	}
	// Fallback: if src is a 16-bit pair and dst is 8-bit, try the low half.
	if srcPhys >= 0 && srcPhys < len(desc.Locs) && desc.Locs[srcPhys].Width == 16 {
		if halfName, ok := halfMap[desc.Locs[srcPhys].Name]; ok {
			halfIdx := desc.LocByName(halfName)
			if halfIdx >= 0 {
				if p := findMovePatternDirect(desc, halfIdx, dstPhys); p != nil {
					return halfIdx, p // remapped src
				}
			}
		}
	}
	// Also try: dst is 16-bit pair, src is 8-bit → try the low half
	if dstPhys >= 0 && dstPhys < len(desc.Locs) && desc.Locs[dstPhys].Width == 16 {
		if halfName, ok := halfMap[desc.Locs[dstPhys].Name]; ok {
			halfIdx := desc.LocByName(halfName)
			if halfIdx >= 0 {
				if p := findMovePatternDirect(desc, srcPhys, halfIdx); p != nil {
					return srcPhys, p // dst remapped (caller updates DstPhys)
				}
			}
		}
	}
	return srcPhys, nil
}

func findMovePatternDirect(desc *MachineDesc, srcPhys, dstPhys int) *Pattern {
	dstWidth := 8
	if dstPhys >= 0 && dstPhys < len(desc.Locs) {
		dstWidth = desc.Locs[dstPhys].Width
	}
	// Try exact width first
	for i := range desc.Patterns {
		pat := &desc.Patterns[i]
		if pat.Op != OpMove || pat.Width != dstWidth {
			continue
		}
		if strings.Contains(pat.Template, "EX ") {
			continue
		}
		if pat.DstLocs.Has(dstPhys) && pat.SrcLocs[0].Has(srcPhys) {
			return pat
		}
	}
	// Try any width (covers truncation 16→8 and extension 8→16 patterns)
	for i := range desc.Patterns {
		pat := &desc.Patterns[i]
		if pat.Op != OpMove || pat.Width == dstWidth {
			continue // already tried
		}
		if strings.Contains(pat.Template, "EX ") {
			continue
		}
		if pat.DstLocs.Has(dstPhys) && pat.SrcLocs[0].Has(srcPhys) {
			return pat
		}
	}
	return nil
}

// insertPerInstMoves post-processes per-inst PIROps: for each vreg whose
// location changed between consecutive instructions, insert a LD move.
func insertPerInstMoves(pirOps []PIROp, p *problem, vals map[string]int, desc *MachineDesc) []PIROp {
	var result []PIROp
	for i, pirop := range pirOps {
		// Before each instruction, check if ANY live vreg needs a move.
		// Not just Src operands — also vregs that changed location for
		// pattern requirements (e.g., is_neg moves C→A for CMP pattern).
		if i > 0 && i < len(p.ops) {
			// Collect all vregs to check: srcs + all live vregs at this point
			checkVregs := make(map[int]bool)
			op := p.ops[i]
			if op.Src[0] > 0 {
				checkVregs[op.Src[0]] = true
			}
			if op.Src[1] > 0 {
				checkVregs[op.Src[1]] = true
			}
			if i < len(p.liveness) {
				for v := range p.liveness[i].live {
					if v > 0 {
						checkVregs[v] = true
					}
				}
			}
			sortedVregs := make([]int, 0, len(checkVregs))
			for v := range checkVregs {
				sortedVregs = append(sortedVregs, v)
			}
			sort.Ints(sortedVregs)
			for _, vreg := range sortedVregs {
				if vreg <= 0 {
					continue
				}
				// Find where this vreg was at previous instruction vs now
				currKey := fmt.Sprintf("lv%d_i%d", vreg, i)
				currLoc, hasCurr := vals[currKey]
				if !hasCurr {
					continue
				}
				// Search backwards for last known location
				for pi := i - 1; pi >= 0; pi-- {
					prevKey := fmt.Sprintf("lv%d_i%d", vreg, pi)
					prevLoc, hasPrev := vals[prevKey]
					if hasPrev {
						if prevLoc != currLoc {
							movePat := findMovePattern(desc, prevLoc, currLoc)
							if movePat != nil {
								result = append(result, PIROp{
									Pat:     movePat,
									DstPhys: currLoc,
									SrcPhys: [2]int{prevLoc, -1},
								})
							}
						}
						break
					}
				}
			}
		}
		result = append(result, pirop)
	}
	return result
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
	cmd := exec.Command(opts.Z3Path,
		"-T:"+strconv.Itoa(int(opts.Timeout.Seconds())),
		f.Name())
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
		// Check if it's a real timeout or just Z3 giving up
		reason := ""
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "(error") || strings.HasPrefix(line, "(:reason") {
				reason = line
				break
			}
		}
		return "", fmt.Errorf("z3 timeout after %v (reason: %s)", opts.Timeout, reason)
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
