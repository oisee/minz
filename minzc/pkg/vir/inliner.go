// inliner.go — MIR2-level function inliner for small callees in loop bodies.
//
// Works at MIR2 level (before LowerFunc) so multi-block callees are handled
// naturally — clone callee CFG into caller, LowerFunc sees the enlarged function.
//
// Eligible callee:
//   - No back-edges (DAG: if/else diamonds, no internal loops)
//   - Total VIR op count across all blocks ≤ MaxOps
//   - Not extern, not recursive
//
// Inlining steps for each qualifying CALL in a loop-body block:
//   1. Compute vreg offset = max(callerVregs) + 1
//   2. Clone callee blocks (vreg += offset, labels = callsite_prefix + callee_label)
//   3. Substitute callee param vregs → caller arg vregs (from inst.Args)
//   4. Split caller block at the CALL instruction:
//        B_before: ops[0..callIdx-1], Term = TermJmp(cloned_entry, no args)
//        cloned callee blocks: TermRet → TermJmp(B_after, [retVreg+offset])
//        B_after: ops[callIdx+1..end], Params = [callDst], Term = original term
//   5. Splice all new blocks into f.Blocks after the original caller block
//
// This is standard compiler practice (LLVM inlines at LLVM-IR level).
// After inlining, LowerFunc + solver see one enlarged function with no CALL.
package vir

import (
	"fmt"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// InlineOptions controls MIR2-level inlining behavior.
type InlineOptions struct {
	// MaxOps: max total VIR ops across all callee blocks to qualify for inlining.
	// Default: 20.
	MaxOps int

	// LoopOnly: only inline calls in blocks whose label contains "body".
	// Default: true.
	LoopOnly bool

	// Verbose: print inlining decisions to stderr.
	Verbose bool
}

// DefaultInlineOptions returns sensible defaults.
func DefaultInlineOptions() InlineOptions {
	return InlineOptions{MaxOps: 20, LoopOnly: true}
}

// InlineResult records a single inlined call site.
type InlineResult struct {
	CallerName string
	CalleeName string
	BlockLabel string
	OpsInlined int
}

// InlineMIR2 inlines small DAG callees into f at MIR2 level.
// Operates before LowerFunc. Returns true if any inlining was performed.
// m is used for callee lookup; desc is used to count VIR ops per callee.
func InlineMIR2(f *mir2.Func, m *mir2.Module, desc *MachineDesc, opts InlineOptions) (bool, []InlineResult) {
	if opts.MaxOps <= 0 {
		opts.MaxOps = 20
	}

	// Pre-compute inlineable callees: low VIR op count, DAG structure.
	type calleeInfo struct {
		f      *mir2.Func
		virOps int
	}
	eligible := map[string]*calleeInfo{}
	for _, callee := range m.Funcs {
		if callee == f || callee.Attrs.IsExtern || callee.Attrs.IsRecursive {
			continue
		}
		if hasBackEdgeMIR2(callee) {
			continue // loops inside callee — too complex
		}
		// Lower to VIR to count ops (no solver).
		vCallee, err := LowerFunc(callee, desc, m)
		if err != nil {
			continue
		}
		total := 0
		for _, blk := range vCallee.Blocks {
			total += len(blk.Ops)
		}
		if total > opts.MaxOps {
			continue
		}
		eligible[callee.Name] = &calleeInfo{f: callee, virOps: total}
	}

	if len(eligible) == 0 {
		return false, nil
	}

	inlined := false
	var results []InlineResult

	// Process blocks in order. We may append new blocks to f.Blocks during
	// iteration — iterate by index and recheck len each iteration.
	for bi := 0; bi < len(f.Blocks); bi++ {
		blk := f.Blocks[bi]
		if opts.LoopOnly && !isLoopBodyBlock(blk.Label) {
			continue
		}

		// Scan for a qualifying OpCall.
		// Only inline the first qualifying call per block to keep things simple;
		// loop re-runs the outer loop for the newly split blocks.
		for callIdx, inst := range blk.Insts {
			if inst.Op != mir2.OpCall || inst.Sym == "" {
				continue
			}
			ci, ok := eligible[inst.Sym]
			if !ok {
				continue
			}

			// Perform the inline.
			newBlocks, retVreg := inlineCallAt(f, bi, callIdx, ci.f, m)
			if newBlocks == nil {
				continue
			}

			// Splice new blocks into f.Blocks after bi.
			// f.Blocks[bi] is now B_before (already mutated by inlineCallAt).
			// newBlocks = [cloned callee blocks..., B_after]
			tail := append([]*mir2.Block{}, f.Blocks[bi+1:]...)
			f.Blocks = append(f.Blocks[:bi+1], newBlocks...)
			f.Blocks = append(f.Blocks, tail...)

			_ = retVreg

			r := InlineResult{
				CallerName: f.Name,
				CalleeName: inst.Sym,
				BlockLabel: blk.Label,
				OpsInlined: ci.virOps,
			}
			results = append(results, r)
			inlined = true

			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "[inline-mir2] %s/%s: inlined %s (%d VIR ops)\n",
					f.Name, blk.Label, inst.Sym, ci.virOps)
			}

			break // restart scan for this block position (now B_before)
		}
	}

	return inlined, results
}

// inlineCallAt performs the inline transformation for the CALL at position
// callIdx in f.Blocks[bi]. Mutates f.Blocks[bi] to be B_before.
// Returns the new blocks to splice in: [callee_clone_blocks..., B_after].
// Returns nil on failure (can't inline for some reason).
// retVreg is the vreg that carries the return value in B_after (or NoReg).
func inlineCallAt(f *mir2.Func, bi, callIdx int, callee *mir2.Func, m *mir2.Module) ([]*mir2.Block, mir2.Reg) {
	callerBlk := f.Blocks[bi]
	callInst := callerBlk.Insts[callIdx]

	// Compute vreg offset: all new vregs will be >= offset.
	offset := scanMaxVreg(f)
	if mv := scanMaxVreg2(callee); mv > 0 {
		// Reserve enough space for all callee vregs.
		_ = mv // offset already accounts for caller; callee vregs go above it
	}
	offset++ // offset = first new vreg ID

	// Build substitution: callee param vreg → caller arg vreg.
	subst := make(map[mir2.Reg]mir2.Reg, len(callee.Contract.Params))
	for pi, p := range callee.Contract.Params {
		if pi < len(callInst.Args) {
			subst[p.Reg] = callInst.Args[pi]
		}
	}

	// Unique label prefix for this inline site.
	prefix := fmt.Sprintf("_inl_%s_%s_b%d_", callee.Name, callerBlk.Label, callIdx)

	// Clone all callee blocks.
	clonedBlocks := make([]*mir2.Block, len(callee.Blocks))
	for i, cb := range callee.Blocks {
		clonedBlocks[i] = cloneBlock(cb, mir2.Reg(offset), subst, prefix)
	}

	// Determine return vreg in caller space and the B_after label.
	callerRetVreg := callInst.Dst // the vreg the caller expects the return in
	afterLabel := callerBlk.Label + "_after"

	// Patch cloned callee terminators:
	// TermRet → TermJmp to B_after, passing the return value as an arg.
	for _, cb := range clonedBlocks {
		switch t := cb.Term.(type) {
		case *mir2.TermRet:
			var jmpArgs []mir2.Reg
			if len(t.Vals) > 0 && callerRetVreg != mir2.NoReg {
				// The return value is the substituted/offset version of the callee's retval.
				retVal := applySubst(t.Vals[0], mir2.Reg(offset), subst)
				jmpArgs = []mir2.Reg{retVal}
			}
			cb.Term = &mir2.TermJmp{Target: afterLabel, Args: jmpArgs}
		case *mir2.TermCondRet:
			// TermCondRet = "if condition, return val; else continue."
			// Can't inline this safely without proper CFG split. Bail out.
			_ = t
			return nil, mir2.NoReg
		}
		// TermJmp, TermBrIf: patch labels (prefix callee's labels).
		cb.Term = patchTermLabels(cb.Term, prefix, callee, afterLabel, mir2.Reg(offset), subst)
	}

	// Build B_after: continuation block with ops after the CALL.
	bAfter := &mir2.Block{Label: afterLabel}
	// If the callee returns a value, B_after has a block param for it.
	if callerRetVreg != mir2.NoReg && len(callee.Contract.Returns) > 0 {
		retTy := callee.Contract.Returns[0].Ty
		retClass := callee.Contract.Returns[0].Class
		bAfter.Params = append(bAfter.Params, mir2.BlockParam{
			Dst: callerRetVreg, Ty: retTy, Class: retClass,
		})
	}
	bAfter.Insts = callerBlk.Insts[callIdx+1:]
	bAfter.Term = callerBlk.Term

	// Mutate B_before: remove call and all ops after it; set term = Jmp to cloned entry.
	// Also remove the arg-setup insts immediately before the call (arg setup moves
	// are emitted by LowerBlock from the CALL inst itself, not as separate MIR2 insts).
	// In MIR2, OpCall carries its args directly — no separate move insts to remove.
	callerBlk.Insts = callerBlk.Insts[:callIdx]
	callerBlk.Term = &mir2.TermJmp{
		Target: clonedBlocks[0].Label, // cloned callee entry
		Args:   nil,
	}

	// Advance the caller's vreg counter past all new vregs.
	// We need to reserve at least offset + callee.NextRegNum() - 1 vreg IDs.
	calleeMax := int(scanMaxVreg2(callee))
	for int(f.NextRegNum()) <= offset+calleeMax {
		f.AllocReg() // advances counter
	}

	newBlocks := append(clonedBlocks, bAfter)
	return newBlocks, callerRetVreg
}

// cloneBlock clones a MIR2 block, applying vreg offset + substitution to all
// vreg references. Block label gets the prefix.
func cloneBlock(b *mir2.Block, offset mir2.Reg, subst map[mir2.Reg]mir2.Reg, prefix string) *mir2.Block {
	nb := &mir2.Block{Label: prefix + b.Label}

	// Clone block params (with offset — these are PHI destinations in the callee).
	for _, p := range b.Params {
		nb.Params = append(nb.Params, mir2.BlockParam{
			Dst:   applySubst(p.Dst, offset, subst),
			Ty:    p.Ty,
			Class: p.Class,
		})
	}

	// Clone instructions.
	for _, inst := range b.Insts {
		ni := *inst // shallow copy
		ni.Dst = applySubst(inst.Dst, offset, subst)
		if inst.Src[0] != mir2.NoReg {
			ni.Src[0] = applySubst(inst.Src[0], offset, subst)
		}
		if inst.Src[1] != mir2.NoReg {
			ni.Src[1] = applySubst(inst.Src[1], offset, subst)
		}
		if len(inst.Args) > 0 {
			ni.Args = make([]mir2.Reg, len(inst.Args))
			for i, a := range inst.Args {
				ni.Args[i] = applySubst(a, offset, subst)
			}
		}
		nb.Insts = append(nb.Insts, &ni)
	}

	return nb
}

// applySubst applies the vreg substitution: if v is in subst, return subst[v];
// if v is NoReg, return NoReg; otherwise return v + offset.
func applySubst(v mir2.Reg, offset mir2.Reg, subst map[mir2.Reg]mir2.Reg) mir2.Reg {
	if v == mir2.NoReg {
		return mir2.NoReg
	}
	if mapped, ok := subst[v]; ok {
		return mapped
	}
	return v + offset
}

// patchTermLabels rewrites a terminator's target labels to use the prefix.
// Labels that point WITHIN the callee get prefixed.
// Labels that were originally TermRet targets should already have been handled.
func patchTermLabels(term mir2.Term, prefix string, callee *mir2.Func, afterLabel string, offset mir2.Reg, subst map[mir2.Reg]mir2.Reg) mir2.Term {
	calleeLabels := make(map[string]bool, len(callee.Blocks))
	for _, b := range callee.Blocks {
		calleeLabels[b.Label] = true
	}

	patchLabel := func(lbl string) string {
		if calleeLabels[lbl] {
			return prefix + lbl
		}
		return lbl
	}
	patchArgs := func(args []mir2.Reg) []mir2.Reg {
		if len(args) == 0 {
			return nil
		}
		out := make([]mir2.Reg, len(args))
		for i, a := range args {
			out[i] = applySubst(a, offset, subst)
		}
		return out
	}

	switch t := term.(type) {
	case *mir2.TermJmp:
		return &mir2.TermJmp{
			Target: patchLabel(t.Target),
			Args:   patchArgs(t.Args),
		}
	case *mir2.TermBrIf:
		return &mir2.TermBrIf{
			Cond:     applySubst(t.Cond, offset, subst),
			Then:     patchLabel(t.Then),
			ThenArgs: patchArgs(t.ThenArgs),
			Else:     patchLabel(t.Else),
			ElseArgs: patchArgs(t.ElseArgs),
		}
	case *mir2.TermRet:
		// Already patched by caller → should not reach here.
		return term
	default:
		return term
	}
}

// hasBackEdgeMIR2 returns true if f contains any back-edge (i.e., has loops).
// Uses DFS with coloring: white=0, gray=1 (in stack), black=2 (done).
func hasBackEdgeMIR2(f *mir2.Func) bool {
	if len(f.Blocks) == 0 {
		return false
	}
	// Build label→index map.
	idx := make(map[string]int, len(f.Blocks))
	for i, b := range f.Blocks {
		idx[b.Label] = i
	}
	color := make([]int, len(f.Blocks)) // 0=white, 1=gray, 2=black

	var dfs func(i int) bool
	dfs = func(i int) bool {
		color[i] = 1 // gray
		var succs []string
		if f.Blocks[i].Term != nil {
			succs = f.Blocks[i].Term.Successors()
		}
		for _, lbl := range succs {
			j, ok := idx[lbl]
			if !ok {
				continue
			}
			if color[j] == 1 {
				return true // back-edge
			}
			if color[j] == 0 {
				if dfs(j) {
					return true
				}
			}
		}
		color[i] = 2 // black
		return false
	}

	return dfs(0)
}

// scanMaxVreg returns the highest vreg ID used in any instruction of f.
func scanMaxVreg(f *mir2.Func) int {
	max := 0
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			if int(p.Dst) > max {
				max = int(p.Dst)
			}
		}
		for _, inst := range b.Insts {
			for _, r := range []mir2.Reg{inst.Dst, inst.Src[0], inst.Src[1]} {
				if int(r) > max {
					max = int(r)
				}
			}
			for _, a := range inst.Args {
				if int(a) > max {
					max = int(a)
				}
			}
		}
	}
	return max
}

// scanMaxVreg2 returns max vreg ID in a MIR2 func (same as scanMaxVreg but takes func).
func scanMaxVreg2(f *mir2.Func) mir2.Reg {
	return mir2.Reg(scanMaxVreg(f))
}

// isLoopBodyBlock returns true if the block label suggests it's inside a loop.
// Covers: loop_body_N, fe_body_N, rng_body_N, while_body_N, for_body_N, etc.
func isLoopBodyBlock(label string) bool {
	return strings.Contains(label, "body")
}
