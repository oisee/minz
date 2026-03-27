// detect.go — Automatic detection of GPU-parallelizable patterns in MIR2.
//
// Recognizes three patterns at the MIR2 level:
//
//   1. Bounded map:    loop over 0..N applying pure function → collect results
//   2. Bounded filter: loop with conditional store → compact results
//   3. Bounded reduce: loop accumulating into single value → tree reduction
//
// Detection works on the CFG: find back-edges (loops), check that the loop body
// is pure (no calls to impure functions, no stores outside result buffer), and
// verify the induction variable has a bounded type (u8=256, u16=65536).
//
// Usage:
//
//	candidates := mir2gpu.DetectGPUCandidates(module)
//	for _, c := range candidates {
//	    fmt.Printf("func %s: %s over %d threads\n", c.Func.Name, c.Kind, c.NumThreads)
//	}
package mir2gpu

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// GPUCandidateKind classifies the parallelization strategy.
type GPUCandidateKind int

const (
	GPUMap    GPUCandidateKind = iota // each thread computes f(i) → results[i]
	GPUFilter                        // each thread evaluates predicate, atomic compact
	GPUReduce                        // parallel tree reduction
)

func (k GPUCandidateKind) String() string {
	return [...]string{"map", "filter", "reduce"}[k]
}

// GPUCandidate describes a detected GPU-parallelizable loop.
type GPUCandidate struct {
	Func       *mir2.Func       // containing function
	Kind       GPUCandidateKind // map/filter/reduce
	NumThreads int              // bounded by induction var type (256 for u8, 65536 for u16)
	LoopHeader *mir2.Block      // loop header block
	LoopBody   []*mir2.Block    // blocks forming the loop body
	InductionVar mir2.Reg       // induction variable (loop counter)
	BoundReg   mir2.Reg         // upper bound register (or NoReg if constant)
	BoundConst int64            // upper bound constant (if BoundReg == NoReg)
	BodyFunc   string           // name of called function in body (if single-call pattern)
	ResultReg  mir2.Reg         // register written to output
	IsPure     bool             // body has no side effects
}

// DetectGPUCandidates scans all functions in a module for GPU-parallelizable loops.
func DetectGPUCandidates(m *mir2.Module) []GPUCandidate {
	var candidates []GPUCandidate
	for _, f := range m.Funcs {
		candidates = append(candidates, detectInFunc(f)...)
	}
	return candidates
}

// detectInFunc finds GPU candidates in a single function.
func detectInFunc(f *mir2.Func) []GPUCandidate {
	if len(f.Blocks) < 2 {
		return nil // no loops possible
	}

	var candidates []GPUCandidate

	// Build block index
	blockIdx := make(map[string]int)
	for i, b := range f.Blocks {
		blockIdx[b.Label] = i
	}

	// Find back-edges (loop headers)
	// A back-edge is an edge from block B to block H where H dominates B
	// (or simplified: H has a smaller index than B in RPO order)
	for _, b := range f.Blocks {
		if b.Term == nil {
			continue
		}
		for _, succ := range b.Term.Successors() {
			succIdx, ok := blockIdx[succ]
			if !ok {
				continue
			}
			curIdx := blockIdx[b.Label]
			if succIdx <= curIdx {
				// Back-edge found: b → succ (succ is loop header)
				header := f.Blocks[succIdx]
				c := analyzeLoop(f, header, b, blockIdx)
				if c != nil {
					candidates = append(candidates, *c)
				}
			}
		}
	}

	return candidates
}

// analyzeLoop checks if a loop starting at header with back-edge from latch
// is a GPU-parallelizable bounded loop.
func analyzeLoop(f *mir2.Func, header, latch *mir2.Block, blockIdx map[string]int) *GPUCandidate {
	// Collect loop body blocks (all blocks between header and latch inclusive)
	headerIdx := blockIdx[header.Label]
	latchIdx := blockIdx[latch.Label]

	var loopBlocks []*mir2.Block
	for i := headerIdx; i <= latchIdx; i++ {
		loopBlocks = append(loopBlocks, f.Blocks[i])
	}

	// Step 1: Find induction variable from header's block params
	if len(header.Params) == 0 {
		return nil // no PHI = no induction variable
	}

	// Look for a block param that gets incremented and compared against a bound
	for _, param := range header.Params {
		inductVar := param.Dst
		inductTy := param.Ty

		// Check: is this an induction variable? (incremented by 1 in latch)
		if !isIncrementedBy1(loopBlocks, inductVar) {
			continue
		}

		// Determine bound from bounded type or explicit comparison
		numThreads := boundFromType(inductTy)
		if numThreads == 0 {
			// Try to find explicit bound from comparison in header
			numThreads, _ = findExplicitBound(header, inductVar)
		}
		if numThreads == 0 {
			continue
		}

		// Step 2: Classify the loop body
		kind, bodyFunc, resultReg, isPure := classifyBody(loopBlocks, header, inductVar)
		if kind < 0 {
			continue // not a recognized pattern
		}

		return &GPUCandidate{
			Func:         f,
			Kind:         kind,
			NumThreads:   numThreads,
			LoopHeader:   header,
			LoopBody:     loopBlocks,
			InductionVar: inductVar,
			BoundConst:   int64(numThreads),
			BodyFunc:     bodyFunc,
			ResultReg:    resultReg,
			IsPure:       isPure,
		}
	}

	return nil
}

// isIncrementedBy1 checks if reg is incremented by 1 somewhere in the loop body.
func isIncrementedBy1(blocks []*mir2.Block, reg mir2.Reg) bool {
	for _, b := range blocks {
		for _, inst := range b.Insts {
			if inst.Op == mir2.OpAdd && inst.Dst != mir2.NoReg {
				// Check: dst = reg + 1  (where reg is one src, 1 is const in the other)
				if inst.Src[0] == reg || inst.Src[1] == reg {
					otherSrc := inst.Src[1]
					if inst.Src[1] == reg {
						otherSrc = inst.Src[0]
					}
					if isConstOne(blocks, otherSrc) {
						return true
					}
				}
			}
		}
	}
	return false
}

// isConstOne checks if a register holds constant value 1.
func isConstOne(blocks []*mir2.Block, reg mir2.Reg) bool {
	for _, b := range blocks {
		for _, inst := range b.Insts {
			if inst.Dst == reg && inst.Op == mir2.OpConst && inst.Imm == 1 {
				return true
			}
		}
	}
	return false
}

// boundFromType returns the iteration count for bounded types.
func boundFromType(ty mir2.Ty) int {
	switch {
	case ty == mir2.TyU8 || ty == mir2.TyI8 || ty == mir2.TyBool:
		return 256
	case ty == mir2.TyU16 || ty == mir2.TyI16:
		return 65536
	}
	return 0
}

// findExplicitBound looks for a comparison like `inductVar < N` in the header.
func findExplicitBound(header *mir2.Block, inductVar mir2.Reg) (int, mir2.Reg) {
	for _, inst := range header.Insts {
		if inst.Op == mir2.OpCmp && (inst.Cond == mir2.CmpUlt || inst.Cond == mir2.CmpLt) {
			if inst.Src[0] == inductVar {
				// i < bound — check if bound is a constant
				boundConst := findConstValue(header, inst.Src[1])
				if boundConst > 0 {
					return int(boundConst), mir2.NoReg
				}
				return 0, inst.Src[1] // dynamic bound
			}
		}
	}
	return 0, mir2.NoReg
}

// findConstValue finds the constant value of a register defined in the block.
func findConstValue(b *mir2.Block, reg mir2.Reg) int64 {
	for _, inst := range b.Insts {
		if inst.Dst == reg && inst.Op == mir2.OpConst {
			return inst.Imm
		}
	}
	return -1
}

// classifyBody determines the GPU pattern of a loop body.
// Returns (kind, bodyFuncName, resultReg, isPure) or (-1, ...) if not recognized.
func classifyBody(blocks []*mir2.Block, header *mir2.Block, inductVar mir2.Reg) (GPUCandidateKind, string, mir2.Reg, bool) {
	// Build alias set: registers that are copies of inductVar
	aliases := map[mir2.Reg]bool{inductVar: true}
	for _, b := range blocks {
		for _, inst := range b.Insts {
			if inst.Op == mir2.OpMove && aliases[inst.Src[0]] {
				aliases[inst.Dst] = true
			}
		}
	}

	var calls []callInfo
	hasStore := false
	hasSideEffects := false
	var resultReg mir2.Reg
	hasAccumulator := false

	for _, b := range blocks {
		for _, inst := range b.Insts {
			switch inst.Op {
			case mir2.OpCall:
				ci := callInfo{sym: inst.Sym, dst: inst.Dst}
				// Check if induction var (or alias) is passed as argument
				for _, arg := range inst.Args {
					if aliases[arg] {
						ci.usesInductVar = true
					}
				}
				calls = append(calls, ci)

			case mir2.OpStore:
				hasStore = true

			case mir2.OpLoad:
				// Loads from arrays indexed by induction var are fine

			case mir2.OpAdd, mir2.OpOr, mir2.OpXor, mir2.OpAnd, mir2.OpMul:
				// Check for accumulator pattern in SSA: block param flows through
				// arithmetic op and result is passed back to same param position.
				// In SSA: param r8 → r12 = add(r8, r11) → jmp back with r12 at param position.
				// Detect: one src is a header param (accumulator), result is used in back-edge.
				for _, src := range inst.Src[:2] {
					if src == mir2.NoReg {
						continue
					}
					if isHeaderParam(header, src) {
						hasAccumulator = true
						resultReg = inst.Dst
					}
				}
			}
		}
	}

	// Check for impure intrinsics
	for _, c := range calls {
		if isImpureCall(c.sym) {
			hasSideEffects = true
		}
	}

	isPure := !hasSideEffects && !hasStore

	// Pattern 1: REDUCE — accumulator pattern (acc = acc + f(i))
	// Must check before MAP: reduce loops also have a single call
	if hasAccumulator && isPure {
		bodyFunc := ""
		if len(calls) == 1 && calls[0].usesInductVar {
			bodyFunc = calls[0].sym
		}
		return GPUReduce, bodyFunc, resultReg, true
	}

	// Pattern 2: MAP — single pure call with induction var, result stored
	if len(calls) == 1 && calls[0].usesInductVar && calls[0].dst != mir2.NoReg && isPure {
		return GPUMap, calls[0].sym, calls[0].dst, true
	}

	// Pattern 3: MAP — no calls, just arithmetic on induction var
	if len(calls) == 0 && !hasStore && !hasSideEffects {
		// Find the return value — it's in the terminator
		for _, b := range blocks {
			if b.Term == nil {
				continue
			}
			if ret, ok := b.Term.(*mir2.TermRet); ok && len(ret.Vals) > 0 {
				return GPUMap, "", ret.Vals[0], true
			}
		}
		return GPUMap, "", mir2.NoReg, true
	}

	// Pattern 4: FILTER — conditional store
	if hasStore && len(calls) <= 1 {
		return GPUFilter, "", mir2.NoReg, isPure
	}

	return -1, "", mir2.NoReg, false
}

// isHeaderParam checks if reg is defined as a block parameter of the given header.
func isHeaderParam(header *mir2.Block, reg mir2.Reg) bool {
	for _, p := range header.Params {
		if p.Dst == reg {
			return true
		}
	}
	return false
}

type callInfo struct {
	sym            string
	dst            mir2.Reg
	usesInductVar  bool
}

// isImpureCall checks if a function call has side effects.
func isImpureCall(sym string) bool {
	// Intrinsics that are impure (I/O, memory modification)
	switch {
	case len(sym) > 8 && sym[:8] == "@mir.io.":
		return true // I/O is always impure
	case len(sym) > 9 && sym[:9] == "@mir.mem.":
		return true // memory operations are impure
	case len(sym) > 9 && sym[:9] == "@mir.z80.":
		return true // hardware I/O is impure
	}
	return false
}

// ── Reporting ──────────────────────────────────────────────────────────────────

// FormatCandidates returns a human-readable summary of detected candidates.
func FormatCandidates(candidates []GPUCandidate) string {
	if len(candidates) == 0 {
		return "No GPU-parallelizable loops detected."
	}
	var s string
	for i, c := range candidates {
		pure := ""
		if c.IsPure {
			pure = " (pure)"
		}
		body := c.BodyFunc
		if body == "" {
			body = "inline"
		}
		s += fmt.Sprintf("  [%d] %s.%s: %s → %d threads, body=%s%s\n",
			i+1, c.Func.Name, c.LoopHeader.Label, c.Kind, c.NumThreads, body, pure)
	}
	return s
}
