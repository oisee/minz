// grace_runner.go — Wires Grace declarative rules into the MIR2 optimization pipeline.
//
// Each MIR2 optimization pass (DSE, DeadBlockArgElim, SplitJoinRet, CondRetSink,
// FuseAbsDiff) is expressed as a Grace rule + custom predicates/actions. The
// Grace engine matches CFG patterns and delegates complex mutations to Go.
//
// The Go originals remain as the production path. Grace runs in "shadow" mode
// by default: both paths execute, results are compared, statistics are collected.
// Set GraceOnly=true to skip Go originals and rely solely on Grace rules.
package mir2

import (
	"fmt"
	"sync"

	"github.com/minz/minzc/pkg/rewrite"
	"github.com/minz/minzc/pkg/rewrite/grace"
)

// ── Grace Rules (embedded) ──────────────────────────────────────────────────

// Priority scheme matches Go pipeline ordering:
// DSE=50, DeadBlockArg=45, SplitJoinRet=30, CondRetSink=20, FuseAbsDiff=10
// Higher priority = runs first (same as Go: DSE first, FuseAbsDiff last).

const graceCondRetSinkRule = `
(grace cond-ret-sink 20
  (match
    (block ?cur (term-kind "br_if"))
    (block ?else (term-kind "ret"))
    (edge ?cur ?else "else"))
  (where
    (no-params ?else)
    (all-pure ?else)
    (pred-count ?else == 1)
    (no-carry-clobber ?cur ?else))
  (action
    (custom "condRetSink" ?cur ?else)))
`

const graceSplitJoinRetRule = `
(grace split-join-ret 30
  (match
    (block ?join (term-kind "ret")))
  (where
    (param-count ?join == 1)
    (inst-count ?join == 0)
    (is-ret-of-param ?join))
  (action
    (custom "splitJoinRet" ?join)))
`

const graceDeadBlockArgRule = `
(grace dead-block-arg 45
  (match
    (block ?b))
  (where
    (param-count ?b > 0)
    (has-dead-params ?b))
  (action
    (custom "elimDeadParams" ?b)))
`

const graceDSERule = `
(grace dead-store-elim 50
  (match
    (block ?b))
  (where
    (has-dead-insts ?b))
  (action
    (custom "elimDeadInsts" ?b)))
`

const graceEmptyBlockElimRule = `
(grace empty-block-elim 40
  (match
    (block ?e (term-kind "jump")))
  (where
    (inst-count ?e == 0)
    (param-count ?e == 0)
    (is-not-entry ?e))
  (action
    (custom "redirectAndDelete" ?e)))
`

const graceBlockMergeRule = `
(grace block-merge 35
  (match
    (block ?p (term-kind "jump"))
    (block ?s)
    (edge ?p ?s "succ"))
  (where
    (pred-count ?s == 1)
    (param-count ?s == 0))
  (action
    (custom "mergeBlocks" ?p ?s)))
`

const graceTrivialBranchRule = `
(grace trivial-branch 30
  (match
    (block ?b (term-kind "br_if")))
  (where
    (same-targets ?b))
  (action
    (custom "branchToJump" ?b)))
`

// Fuse cond_ret(cmp.ne x,y)+br_if(cmp.ugt x,y) into br_if2.
// This is the GCD pattern: one CP instruction tests equality AND ordering.
const graceFuseCmpBrIf2Rule = `
(grace fuse-cmp-brif2 42
  (match
    (block ?head)
    (block ?body)
    (edge ?head ?body "succ"))
  (where
    (is-cond-ret-ne ?head)
    (is-brif-ugt-same-ops ?head ?body))
  (action
    (custom "fuseToBrIf2" ?head ?body)))
`

// Tail call optimization: a block that ends with CALL f; RET can use JP f.
// This saves 1 byte and 7 T-states per tail call (CALL=17T+RET=10T → JP=10T).
// Narrow i16 arithmetic back to u8 when both operands are u8 and result is
// consumed as u8 (truncated or returned as u8). C89 integer promotion widens
// u8+u8 to i16, but on Z80 this forces HL instead of A. Narrowing saves ~4 insts.
// Mark 16-bit comparisons as flags-only when the LHS value is dead after
// the comparison. Uses liveness analysis to check all successors.
// Z80 codegen then skips ADD HL,rr restore after SBC HL,rr (−1 inst each).
const graceFlagsOnlyCmpRule = `
(grace flags-only-cmp 47
  (match
    (block ?b))
  (where
    (has-flags-only-cmp ?b))
  (action
    (custom "markFlagsOnly" ?b)))
`

const graceNarrowArithRule = `
(grace narrow-arith 48
  (match
    (block ?b))
  (where
    (has-narrowable-arith ?b))
  (action
    (custom "narrowArith" ?b)))
`

const graceTailCallRule = `
(grace tail-call-opt 25
  (match
    (block ?b (term-kind "ret")))
  (where
    (has-tail-call ?b))
  (action
    (custom "tailCallOpt" ?b)))
`

// Tail recursion elimination: when a function's last operation is calling
// itself and returning the result, replace CALL+RET with JMP to entry block.
// This converts O(n) stack usage to O(1) — critical on Z80's limited stack.
const graceTailRecursionRule = `
(grace tail-recursion-elim 55
  (match
    (block ?b (term-kind "ret")))
  (where
    (has-self-tail-call ?b))
  (action
    (custom "elimTailRecursion" ?b)))
`

// Redundant load elimination: when a block loads from the same address
// that a dominating block already loaded, replace with the previous result.
// This catches the max_array pattern: load buf[i] for compare, then load
// buf[i] again in the then-branch for assignment.
const graceRedundantLoadRule = `
(grace redundant-load 43
  (match
    (block ?pred)
    (block ?succ)
    (edge ?pred ?succ "succ"))
  (where
    (has-redundant-load ?pred ?succ))
  (action
    (custom "elimRedundantLoad" ?pred ?succ)))
`

const graceParamForwardRule = `
(grace param-forward-elim 38
  (match
    (block ?tramp (term-kind "jump")))
  (where
    (inst-count ?tramp == 0)
    (param-count ?tramp > 0)
    (is-param-forwarding ?tramp)
    (is-not-entry ?tramp))
  (action
    (custom "elimParamForward" ?tramp)))
`

const graceFuseAbsDiffRule = `
(grace fuse-abs-diff 10
  (match
    (block ?b))
  (where
    (inst-count ?b >= 2)
    (has-abs-diff-pattern ?b))
  (action
    (custom "fuseAbsDiff" ?b)))
`

// Bounded loop unrolling: while x >= K { x -= K } → unrolled if-chain.
// For u8 x and constant K: max iterations = 255/K.
// If max iters ≤ 4: replace loop with sequential if(x>=K){x-=K} blocks.
// Pattern: loop_head(cmp x,K; brif) → loop_body(sub x,K; jmp head) → exit
const graceBoundedLoopUnrollRule = `
(grace bounded-loop-unroll 8
  (match
    (block ?head (term-kind "br_if"))
    (block ?body (term-kind "jump"))
    (edge ?head ?body "then")
    (edge ?body ?head "succ"))
  (where
    (is-bounded-sub-loop ?head ?body))
  (action
    (custom "unrollBoundedLoop" ?head ?body)))
`

// Pointer-walk threading (Shape A): convert indexed addressing in a counted
// loop to pointer increment. Pattern:
//   header: br_if (cmp idx, limit), body, exit
//   body:   addr = add base, idx; ...use addr...; idx' = add idx, stride; jmp header(idx')
// Transform: add pointer block arg, replace addr computation with pointer use,
// thread pointer increment through back-edge.
const gracePointerWalkRule = `
(grace pointer-walk-threading 22
  (match
    (block ?head (term-kind "br_if"))
    (block ?body (term-kind "jump"))
    (edge ?head ?body "then")
    (edge ?body ?head "succ"))
  (where
    (has-pointer-walk-opportunity ?head ?body))
  (action
    (custom "threadPointerWalk" ?head ?body)))
`

// Conditional call folding: BrIf → single-void-call-block → join → CALL cc.
// Same transform as FoldConditionalCalls in condcall.go, but expressed as a
// Grace rule for the declarative path. Priority 15 = after CondRetSink (20)
// so cond_ret has priority over cond_call when both patterns match.
const graceCondCallRule = `
(grace cond-call-fold 15
  (match
    (block ?cur (term-kind "br_if"))
    (block ?call (term-kind "jump"))
    (edge ?cur ?call "then"))
  (where
    (is-single-void-call ?call)
    (pred-count ?call == 1)
    (jumps-to-else ?cur ?call))
  (action
    (custom "foldCondCall" ?cur ?call)))
`

// ── Statistics ──────────────────────────────────────────────────────────────

// GraceStats tracks how many times each Grace rule fired across the module.
type GraceStats struct {
	mu      sync.Mutex
	ByRule  map[string]int // rule name → fire count
	Total   int            // total rule applications
	Funcs   int            // functions processed
	Matches int            // successful pattern matches (even if action didn't change anything)
}

// NewGraceStats creates a fresh statistics tracker.
func NewGraceStats() *GraceStats {
	return &GraceStats{ByRule: make(map[string]int)}
}

// Record adds a rule application.
func (s *GraceStats) Record(ruleName string, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ByRule[ruleName] += count
	s.Total += count
}

// String returns a human-readable summary.
func (s *GraceStats) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Total == 0 {
		return fmt.Sprintf("Grace: %d funcs, 0 rewrites", s.Funcs)
	}
	detail := ""
	for name, count := range s.ByRule {
		if detail != "" {
			detail += ", "
		}
		detail += fmt.Sprintf("%s=%d", name, count)
	}
	return fmt.Sprintf("Grace: %d funcs, %d rewrites (%s)", s.Funcs, s.Total, detail)
}

// ── Rule Cache ──────────────────────────────────────────────────────────────

var (
	cachedRules     []grace.GraceRule
	cachedRulesOnce sync.Once
	cachedRulesErr  error
)

func loadAllRules() ([]grace.GraceRule, error) {
	cachedRulesOnce.Do(func() {
		allSrc := graceCondRetSinkRule + graceSplitJoinRetRule + graceDeadBlockArgRule + graceDSERule + graceFuseAbsDiffRule +
			graceEmptyBlockElimRule + graceBlockMergeRule + graceTrivialBranchRule + graceParamForwardRule + graceFuseCmpBrIf2Rule +
			graceTailCallRule + graceNarrowArithRule + graceTailRecursionRule +
			graceRedundantLoadRule + graceFlagsOnlyCmpRule + graceBoundedLoopUnrollRule +
			gracePointerWalkRule + graceCondCallRule
		cachedRules, cachedRulesErr = grace.ParseRules(allSrc)
	})
	return cachedRules, cachedRulesErr
}

// ── Registry Builder ────────────────────────────────────────────────────────

// buildRegistry creates a PredicateRegistry bound to a specific Func.
// Custom predicates and actions capture the Func via closure.
func buildRegistry(f *Func) *grace.PredicateRegistry {
	reg := grace.NewPredicateRegistry()

	// ── Custom Predicates ───────────────────────────────────────────────

	// no-carry-clobber: check that hoisted instrs don't clobber carry
	// when the branch condition is carry-based.
	reg.RegisterPredicate("no-carry-clobber", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		curLabel := bindings[args[0]]
		elseLabel := bindings[args[1]]
		curBlock := f.BlockByLabel(curLabel)
		elseBlock := f.BlockByLabel(elseLabel)
		if curBlock == nil || elseBlock == nil {
			return false
		}
		brif, ok := curBlock.Term.(*TermBrIf)
		if !ok {
			return false
		}
		if condIsCarryBased(f, brif.Cond) && anyFlagClobberer(elseBlock.Insts) {
			return false
		}
		return true
	})

	// is-ret-of-param: check that the block's ret uses exactly its single param
	reg.RegisterPredicate("is-ret-of-param", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil || len(blk.Params) != 1 {
			return false
		}
		ret, ok := blk.Term.(*TermRet)
		if !ok || len(ret.Vals) != 1 {
			return false
		}
		return ret.Vals[0] == blk.Params[0].Dst
	})

	// has-dead-params: check if block has any params with zero uses
	reg.RegisterPredicate("has-dead-params", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil || blk == f.Blocks[0] { // skip entry block
			return false
		}
		uses := countRegUses(f)
		for _, p := range blk.Params {
			if uses[p.Dst] == 0 {
				return true
			}
		}
		return false
	})

	// has-dead-insts: check if block has any pure instructions with unused results
	reg.RegisterPredicate("has-dead-insts", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		uses := countRegUses(f)
		for _, inst := range blk.Insts {
			if inst.Dst != NoReg && uses[inst.Dst] == 0 && isDSEPure(inst.Op) {
				return true
			}
		}
		return false
	})

	// has-abs-diff-pattern: check if block contains cmp.ugt+sub or sub+cmp.ult
	reg.RegisterPredicate("has-abs-diff-pattern", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil || len(blk.Insts) < 2 {
			return false
		}
		for i := 0; i < len(blk.Insts)-1; i++ {
			i0, i1 := blk.Insts[i], blk.Insts[i+1]
			// Pattern A: cmp.ugt(a,b) + sub(b,a)
			if i0.Op == OpCmp && (i0.Cond == CmpUgt || i0.Cond == CmpUge) &&
				i1.Op == OpSub && i1.Src[0] == i0.Src[1] && i1.Src[1] == i0.Src[0] {
				return true
			}
			// Pattern B: sub(a,b) + cmp.ult(a,b)
			if i0.Op == OpSub && i1.Op == OpCmp &&
				(i1.Cond == CmpUlt || i1.Cond == CmpUle) &&
				i0.Src[0] == i1.Src[0] && i0.Src[1] == i1.Src[1] {
				return true
			}
		}
		return false
	})

	// ── Custom Actions ──────────────────────────────────────────────────

	// condRetSink: hoist else insts, replace br_if with cond_ret
	reg.RegisterAction("condRetSink", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		curLabel := bindings["cur"]
		elseLabel := bindings["else"]
		curBlock := f.BlockByLabel(curLabel)
		elseBlock := f.BlockByLabel(elseLabel)
		if curBlock == nil || elseBlock == nil {
			return false
		}
		brif, ok := curBlock.Term.(*TermBrIf)
		if !ok {
			return false
		}
		elseRet, ok := elseBlock.Term.(*TermRet)
		if !ok {
			return false
		}

		// Hoist instructions
		curBlock.Insts = append(curBlock.Insts, elseBlock.Insts...)

		// Replace BrIf with CondRet
		curBlock.Term = &TermCondRet{
			Cond:     brif.Cond,
			Vals:     elseRet.Vals,
			Then:     brif.Then,
			ThenArgs: brif.ThenArgs,
		}

		// Sub-swap optimization
		thenBlock := f.BlockByLabel(brif.Then)
		if thenBlock != nil {
			applySubSwapNeg(elseBlock.Insts, thenBlock)
		}

		// Sub+Cmp reorder for carry fusion
		hoistReorderSubBeforeCmp(curBlock)
		return true
	})

	// splitJoinRet: split join-ret block into per-predecessor rets
	reg.RegisterAction("splitJoinRet", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		joinLabel := bindings["join"]
		joinBlk := f.BlockByLabel(joinLabel)
		if joinBlk == nil {
			return false
		}
		// Delegate to the existing implementation logic
		changed := false
		for _, b := range f.Blocks {
			if b == joinBlk {
				continue
			}
			switch t := b.Term.(type) {
			case *TermJmp:
				if t.Target == joinBlk.Label && len(t.Args) == 1 {
					b.Term = &TermRet{Vals: []Reg{t.Args[0]}}
					changed = true
				}
			case *TermBrIf:
				if t.Then == joinBlk.Label && len(t.ThenArgs) == 1 {
					retLabel := joinBlk.Label + "_ret_then"
					retBlk := &Block{Label: retLabel, Term: &TermRet{Vals: []Reg{t.ThenArgs[0]}}}
					f.Blocks = append(f.Blocks, retBlk)
					t.Then = retLabel
					t.ThenArgs = nil
					changed = true
				}
				if t.Else == joinBlk.Label && len(t.ElseArgs) == 1 {
					retLabel := joinBlk.Label + "_ret_else"
					retBlk := &Block{Label: retLabel, Term: &TermRet{Vals: []Reg{t.ElseArgs[0]}}}
					f.Blocks = append(f.Blocks, retBlk)
					t.Else = retLabel
					t.ElseArgs = nil
					changed = true
				}
			}
		}
		return changed
	})

	// elimDeadParams: remove dead block params and corresponding edge args
	reg.RegisterAction("elimDeadParams", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["b"]
		blk := f.BlockByLabel(label)
		if blk == nil || blk == f.Blocks[0] {
			return false
		}
		uses := countRegUses(f)
		live := make([]bool, len(blk.Params))
		allLive := true
		for i, p := range blk.Params {
			live[i] = uses[p.Dst] > 0
			if !live[i] {
				allLive = false
			}
		}
		if allLive {
			return false
		}
		// Compact params
		newParams := make([]BlockParam, 0, len(blk.Params))
		for i, p := range blk.Params {
			if live[i] {
				newParams = append(newParams, p)
			}
		}
		blk.Params = newParams
		// Patch incoming edges
		for _, pred := range f.Blocks {
			if pred.Term == nil {
				continue
			}
			compactEdgeArgs(pred.Term, label, live)
		}
		return true
	})

	// elimDeadInsts: remove pure instructions with unused results
	reg.RegisterAction("elimDeadInsts", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["b"]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		uses := countRegUses(f)
		changed := false
		keep := blk.Insts[:0]
		for _, inst := range blk.Insts {
			if inst.Dst != NoReg && uses[inst.Dst] == 0 && isDSEPure(inst.Op) {
				changed = true
				continue
			}
			keep = append(keep, inst)
		}
		blk.Insts = keep
		return changed
	})

	// fuseAbsDiff: fuse abs_diff pattern
	reg.RegisterAction("fuseAbsDiff", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["b"]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		// Delegate to the existing implementation
		origLen := len(blk.Insts)
		changed := false
		for i := 0; i < len(blk.Insts)-1; i++ {
			inst0 := blk.Insts[i]
			inst1 := blk.Insts[i+1]

			var cmpInst, subInst *Inst
			var a, bReg Reg
			patternA := inst0.Op == OpCmp &&
				(inst0.Cond == CmpUgt || inst0.Cond == CmpUge) &&
				inst1.Op == OpSub &&
				inst1.Src[0] == inst0.Src[1] && inst1.Src[1] == inst0.Src[0]
			patternB := inst0.Op == OpSub &&
				inst1.Op == OpCmp &&
				(inst1.Cond == CmpUlt || inst1.Cond == CmpUle) &&
				inst0.Src[0] == inst1.Src[0] && inst0.Src[1] == inst1.Src[1]

			if patternA {
				cmpInst, subInst = inst0, inst1
				a, bReg = cmpInst.Src[0], cmpInst.Src[1]
			} else if patternB {
				subInst, cmpInst = inst0, inst1
				a, bReg = subInst.Src[0], subInst.Src[1]
			} else {
				continue
			}

			if patternA {
				subInst.Src[0] = a
				subInst.Src[1] = bReg
			}
			cmpInst.Cond = CmpSubCarry
			cmpInst.Src = [2]Reg{subInst.Dst, bReg}
			cmpInst.SrcTy = subInst.Ty

			blk.Insts[i] = subInst
			blk.Insts[i+1] = cmpInst

			// Check cond_ret target for reverse sub → neg
			if cr, ok := blk.Term.(*TermCondRet); ok {
				for _, tb := range f.Blocks {
					if tb.Label != cr.Then {
						continue
					}
					for ti, tInst := range tb.Insts {
						if tInst.Op == OpSub &&
							tInst.Src[0] == bReg && tInst.Src[1] == a {
							tb.Insts[ti] = &Inst{
								Op:  OpNeg,
								Dst: tInst.Dst,
								Src: [2]Reg{subInst.Dst},
								Ty:  tInst.Ty,
								Cls: tInst.Cls,
							}
						}
					}
				}
			}
			changed = true
		}
		_ = origLen
		return changed
	})

	// ── New Block-Level Predicates ──────────────────────────────────────

	// is-not-entry: block is not the function entry
	reg.RegisterPredicate("is-not-entry", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		return len(f.Blocks) > 0 && f.Blocks[0].Label != label
	})

	// same-targets: br_if where both targets are the same block
	reg.RegisterPredicate("same-targets", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		brif, ok := blk.Term.(*TermBrIf)
		if !ok {
			return false
		}
		return brif.Then == brif.Else
	})

	// is-cond-ret-ne: block ends with cond_ret whose condition is cmp.ne
	reg.RegisterPredicate("is-cond-ret-ne", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		cr, ok := blk.Term.(*TermCondRet)
		if !ok {
			return false
		}
		// Find the cmp instruction producing the condition
		for _, inst := range blk.Insts {
			if inst.Dst == cr.Cond && inst.Op == OpCmp && inst.Cond == CmpNe {
				return true
			}
		}
		return false
	})

	// is-brif-ugt-same-ops: body starts with cmp.ugt of same operands as head's cmp.ne
	reg.RegisterPredicate("is-brif-ugt-same-ops", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		headLabel := bindings[args[0]]
		bodyLabel := bindings[args[1]]
		head := f.BlockByLabel(headLabel)
		body := f.BlockByLabel(bodyLabel)
		if head == nil || body == nil {
			return false
		}
		cr, ok := head.Term.(*TermCondRet)
		if !ok {
			return false
		}
		// Find cmp.ne operands in head
		var cmpLhs, cmpRhs Reg
		found := false
		for _, inst := range head.Insts {
			if inst.Dst == cr.Cond && inst.Op == OpCmp && inst.Cond == CmpNe {
				cmpLhs, cmpRhs = inst.Src[0], inst.Src[1]
				found = true
				break
			}
		}
		if !found {
			return false
		}
		// Check body has br_if with cmp.ugt of same operands
		brif, ok := body.Term.(*TermBrIf)
		if !ok {
			return false
		}
		for _, inst := range body.Insts {
			if inst.Dst == brif.Cond && inst.Op == OpCmp &&
				(inst.Cond == CmpUgt || inst.Cond == CmpGt) &&
				inst.Src[0] == cmpLhs && inst.Src[1] == cmpRhs {
				return true
			}
		}
		return false
	})

	// has-redundant-load: successor block has a load whose address is computed
	// from the SAME values as a load in the predecessor block.
	// Uses value numbering: traces ptr_add(base, ext(i)) back to the underlying
	// block params / function params to check equivalence.
	reg.RegisterPredicate("has-redundant-load", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		predLabel := bindings[args[0]]
		succLabel := bindings[args[1]]
		pred := f.BlockByLabel(predLabel)
		succ := f.BlockByLabel(succLabel)
		if pred == nil || succ == nil {
			return false
		}
		// Value-number each load address in pred
		predLoadKeys := map[string]*Inst{}
		for _, inst := range pred.Insts {
			if inst.Op == OpLoad {
				key := valueKey(inst.Src[0], f)
				if key != "" {
					predLoadKeys[key] = inst
				}
			}
		}
		if len(predLoadKeys) == 0 {
			return false
		}
		// Check successor loads
		for _, inst := range succ.Insts {
			if inst.Op == OpLoad {
				key := valueKey(inst.Src[0], f)
				if key != "" {
					if _, ok := predLoadKeys[key]; ok {
						return true
					}
				}
			}
		}
		return false
	})

	// has-flags-only-cmp: block has a u16 OpCmp where LHS is dead after.
	// Uses global liveness: the LHS vreg has no uses anywhere except as
	// Src[0] of this comparison.
	reg.RegisterPredicate("has-flags-only-cmp", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		uses := countRegUses(f)
		for _, inst := range blk.Insts {
			if inst.Op == OpCmp && !inst.FlagsOnly {
				// LHS is Src[0]. If it has only 1 use (this CMP), it's dead after.
				if uses[inst.Src[0]] <= 1 {
					return true
				}
			}
		}
		return false
	})

	// has-self-tail-call: block ends with ret(call_to_self). The call is to the
	// same function we're in, and the return value is the call's result.
	reg.RegisterPredicate("has-self-tail-call", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil || len(blk.Insts) == 0 {
			return false
		}
		ret, ok := blk.Term.(*TermRet)
		if !ok {
			return false
		}
		lastInst := blk.Insts[len(blk.Insts)-1]
		if lastInst.Op != OpCall || lastInst.Sym != f.Name {
			return false
		}
		// ret must return the call's result
		if len(ret.Vals) == 1 && ret.Vals[0] == lastInst.Dst {
			return true
		}
		if len(ret.Vals) == 0 {
			return true
		}
		return false
	})

	// has-narrowable-arith: block has i16 add/sub/and/or/xor where both sources
	// are u8 (from ext or direct) and result is only used as u8 (truncated or
	// returned as u8). Narrowing to u8 saves HL→A register switch.
	reg.RegisterPredicate("has-narrowable-arith", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		for _, inst := range blk.Insts {
			if isNarrowable(inst, f) {
				return true
			}
		}
		return false
	})

	// has-tail-call: block's last instruction is a call, and the terminator is
	// ret whose value is the call's result. CALL f; RET → can use JP f.
	reg.RegisterPredicate("has-tail-call", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil || len(blk.Insts) == 0 {
			return false
		}
		ret, ok := blk.Term.(*TermRet)
		if !ok {
			return false
		}
		lastInst := blk.Insts[len(blk.Insts)-1]
		if lastInst.Op != OpCall {
			return false
		}
		// ret must return the call's result (or be void)
		if len(ret.Vals) == 0 {
			return true // void function → tail call always safe
		}
		if len(ret.Vals) == 1 && ret.Vals[0] == lastInst.Dst {
			return true // return value is the call result
		}
		return false
	})

	// is-param-forwarding: block is a pure trampoline — jmp target(params[0], params[1], ...)
	// where each arg is the corresponding block param, possibly reordered.
	reg.RegisterPredicate("is-param-forwarding", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil || len(blk.Params) == 0 {
			return false
		}
		jmp, ok := blk.Term.(*TermJmp)
		if !ok {
			return false
		}
		if len(jmp.Args) != len(blk.Params) {
			return false
		}
		// Check that each arg is one of the block params (identity forwarding)
		paramSet := make(map[Reg]bool)
		for _, p := range blk.Params {
			paramSet[p.Dst] = true
		}
		for _, a := range jmp.Args {
			if !paramSet[a] {
				return false
			}
		}
		return true
	})

	// ── New Block-Level Actions ─────────────────────────────────────────

	// redirectAndDelete: redirect all predecessors of block to its target, then delete.
	// Safety: only redirect if the target block has no params (otherwise arg counts mismatch).
	reg.RegisterAction("redirectAndDelete", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["e"]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		jmp, ok := blk.Term.(*TermJmp)
		if !ok || jmp.Target == "" {
			return false
		}
		// Safety: target block must have 0 params, or jmp must carry matching args
		targetBlk := f.BlockByLabel(jmp.Target)
		if targetBlk != nil && len(targetBlk.Params) > 0 && len(jmp.Args) != len(targetBlk.Params) {
			return false
		}
		newTarget := jmp.Target
		newArgs := jmp.Args
		changed := false
		for _, b := range f.Blocks {
			if b.Term == nil {
				continue
			}
			switch t := b.Term.(type) {
			case *TermJmp:
				if t.Target == label {
					t.Target = newTarget
					t.Args = newArgs
					changed = true
				}
			case *TermBrIf:
				if t.Then == label {
					t.Then = newTarget
					t.ThenArgs = newArgs
					changed = true
				}
				if t.Else == label {
					t.Else = newTarget
					t.ElseArgs = newArgs
					changed = true
				}
			case *TermCondRet:
				if t.Then == label {
					t.Then = newTarget
					t.ThenArgs = newArgs
					changed = true
				}
			}
		}
		if changed {
			for i, b := range f.Blocks {
				if b.Label == label {
					f.Blocks = append(f.Blocks[:i], f.Blocks[i+1:]...)
					break
				}
			}
		}
		return changed
	})

	// mergeBlocks: merge successor into predecessor
	reg.RegisterAction("mergeBlocks", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		pLabel := bindings["p"]
		sLabel := bindings["s"]
		pred := f.BlockByLabel(pLabel)
		succ := f.BlockByLabel(sLabel)
		if pred == nil || succ == nil {
			return false
		}
		pred.Insts = append(pred.Insts, succ.Insts...)
		pred.Term = succ.Term
		// Remove successor block
		for i, b := range f.Blocks {
			if b.Label == sLabel {
				f.Blocks = append(f.Blocks[:i], f.Blocks[i+1:]...)
				break
			}
		}
		return true
	})

	// elimRedundantLoad: when successor recomputes a load that predecessor
	// already did (same address by value numbering), pass the loaded value
	// as a block arg and remove the redundant load.
	reg.RegisterAction("elimRedundantLoad", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		predLabel := bindings["pred"]
		succLabel := bindings["succ"]
		pred := f.BlockByLabel(predLabel)
		succ := f.BlockByLabel(succLabel)
		if pred == nil || succ == nil {
			return false
		}

		// Value-number pred loads
		type loadInfo struct {
			inst *Inst
			key  string
		}
		var predLoads []loadInfo
		for _, inst := range pred.Insts {
			if inst.Op == OpLoad {
				key := valueKey(inst.Src[0], f)
				if key != "" {
					predLoads = append(predLoads, loadInfo{inst, key})
				}
			}
		}

		changed := false
		for _, pl := range predLoads {
			for si, succInst := range succ.Insts {
				if succInst.Op != OpLoad {
					continue
				}
				succKey := valueKey(succInst.Src[0], f)
				if succKey != pl.key {
					continue
				}

				// Found redundant load!
				newParam := f.AllocReg()
				succ.Params = append(succ.Params, BlockParam{
					Dst:   newParam,
					Ty:    succInst.Ty,
					Class: succInst.Cls,
				})

				addEdgeArg(pred, succLabel, pl.inst.Dst)

				oldDst := succInst.Dst
				for _, inst := range succ.Insts {
					for si := range inst.Src {
						if inst.Src[si] == oldDst {
							inst.Src[si] = newParam
						}
					}
				}
				if succ.Term != nil {
					remapTermRegs(succ.Term, map[Reg]Reg{oldDst: newParam})
				}

				succ.Insts = append(succ.Insts[:si], succ.Insts[si+1:]...)
				changed = true
				break
			}
			if changed {
				break
			}
		}
		return changed
	})

	// elimTailRecursion: replace CALL self + RET with JMP to a loop head block.
	//
	// Before:
	//   block @entry:        (function params %r1, %r2, ...)
	//     ... body ...
	//   block @recurse:
	//     %result = call @self(%args...)
	//     ret %result
	//
	// After:
	//   block @entry:
	//     jmp @_tail_loop(%r1, %r2, ...)     ← forward params to loop head
	//   block @_tail_loop(%p1, %p2, ...):    ← NEW: loop head with block params
	//     ... body (with params remapped) ...
	//   block @recurse:
	//     jmp @_tail_loop(%args...)           ← loop back instead of call
	//
	reg.RegisterAction("elimTailRecursion", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["b"]
		blk := f.BlockByLabel(label)
		if blk == nil || len(blk.Insts) == 0 || len(f.Blocks) == 0 {
			return false
		}
		lastInst := blk.Insts[len(blk.Insts)-1]
		if lastInst.Op != OpCall || lastInst.Sym != f.Name {
			return false
		}

		entry := f.Blocks[0]

		// Check if loop head already exists (don't transform twice)
		loopLabel := f.Name + "_tail_loop"
		if f.BlockByLabel(loopLabel) != nil {
			return false
		}

		// Create loop head block with params matching function params
		loopParams := make([]BlockParam, len(f.Contract.Params))
		loopParamRegs := make([]Reg, len(f.Contract.Params))
		paramRemap := make(map[Reg]Reg) // old param reg → new loop param reg

		for i, p := range f.Contract.Params {
			newReg := f.AllocReg()
			loopParams[i] = BlockParam{Dst: newReg, Ty: p.Ty, Class: p.Class}
			loopParamRegs[i] = newReg
			paramRemap[p.Reg] = newReg
		}

		// Create the loop head block
		loopBlock := &Block{
			Label:  loopLabel,
			Params: loopParams,
			Insts:  entry.Insts,  // move entry's body to loop head
			Term:   entry.Term,   // move entry's terminator to loop head
		}

		// Remap param references in loop block's instructions
		for _, inst := range loopBlock.Insts {
			for si := range inst.Src {
				if newR, ok := paramRemap[inst.Src[si]]; ok {
					inst.Src[si] = newR
				}
			}
			for ai := range inst.Args {
				if newR, ok := paramRemap[inst.Args[ai]]; ok {
					inst.Args[ai] = newR
				}
			}
		}

		// Remap param references in loop block's terminator
		if loopBlock.Term != nil {
			remapTermRegs(loopBlock.Term, paramRemap)
		}

		// Entry block now just jumps to loop head with original params
		funcParamRegs := make([]Reg, len(f.Contract.Params))
		for i, p := range f.Contract.Params {
			funcParamRegs[i] = p.Reg
		}
		entry.Insts = nil
		entry.Term = &TermJmp{Target: loopLabel, Args: funcParamRegs}

		// Insert loop block after entry
		newBlocks := make([]*Block, 0, len(f.Blocks)+1)
		newBlocks = append(newBlocks, entry)
		newBlocks = append(newBlocks, loopBlock)
		newBlocks = append(newBlocks, f.Blocks[1:]...)
		f.Blocks = newBlocks

		// Replace the tail call with JMP to loop head
		blk.Insts = blk.Insts[:len(blk.Insts)-1]
		blk.Term = &TermJmp{
			Target: loopLabel,
			Args:   lastInst.Args,
		}

		f.Attrs.IsRecursive = false
		return true
	})

	// markFlagsOnly: mark u16 CMP instructions as flags-only when LHS is dead
	reg.RegisterAction("markFlagsOnly", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["b"]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		uses := countRegUses(f)
		changed := false
		for _, inst := range blk.Insts {
			if inst.Op == OpCmp && !inst.FlagsOnly {
				if uses[inst.Src[0]] <= 1 {
					inst.FlagsOnly = true
					changed = true
				}
			}
		}
		return changed
	})

	// narrowArith: narrow i16 arithmetic to u8 when safe
	reg.RegisterAction("narrowArith", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["b"]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		changed := false
		for _, inst := range blk.Insts {
			if isNarrowable(inst, f) {
				inst.Ty = TyU8
				inst.Cls = ClassAcc
				changed = true
			}
		}
		return changed
	})

	// tailCallOpt: mark the last CALL in a block as tail call when followed by RET
	reg.RegisterAction("tailCallOpt", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["b"]
		blk := f.BlockByLabel(label)
		if blk == nil || len(blk.Insts) == 0 {
			return false
		}
		lastInst := blk.Insts[len(blk.Insts)-1]
		if lastInst.Op != OpCall || lastInst.CallAttr.IsTailCall {
			return false
		}
		lastInst.CallAttr.IsTailCall = true
		return true
	})

	// fuseToBrIf2: fuse cond_ret(cmp.ne)+br_if(cmp.ugt) into TermBrIf2
	reg.RegisterAction("fuseToBrIf2", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		headLabel := bindings["head"]
		bodyLabel := bindings["body"]
		head := f.BlockByLabel(headLabel)
		body := f.BlockByLabel(bodyLabel)
		if head == nil || body == nil {
			return false
		}
		cr, ok := head.Term.(*TermCondRet)
		if !ok {
			return false
		}
		brif, ok := body.Term.(*TermBrIf)
		if !ok {
			return false
		}

		// Find cmp.ne in head
		var cmpLhs, cmpRhs Reg
		cmpIdx := -1
		for i, inst := range head.Insts {
			if inst.Dst == cr.Cond && inst.Op == OpCmp && inst.Cond == CmpNe {
				cmpLhs, cmpRhs = inst.Src[0], inst.Src[1]
				cmpIdx = i
				break
			}
		}
		if cmpIdx < 0 {
			return false
		}

		// Find cmp.ugt in body
		bodyCmpIdx := -1
		for i, inst := range body.Insts {
			if inst.Dst == brif.Cond && inst.Op == OpCmp &&
				(inst.Cond == CmpUgt || inst.Cond == CmpGt) &&
				inst.Src[0] == cmpLhs && inst.Src[1] == cmpRhs {
				bodyCmpIdx = i
				break
			}
		}
		if bodyCmpIdx < 0 {
			return false
		}

		// Body must have no other instructions between start and the cmp
		// (otherwise hoisting the cmp would reorder side effects)
		for i := 0; i < bodyCmpIdx; i++ {
			if !isDSEPure(body.Insts[i].Op) {
				return false
			}
		}

		// Build TermBrIf2:
		// cmp.ne(%r3,%r4): Z=eq → return %r3
		// cmp.ugt(%r3,%r4): !C && !Z → then (a>b), else (a<=b but a!=b → a<b)
		//
		// br_if2 %r3, %r4:
		//   eq → create ret block with cr.Vals
		//   lt → brif.Else (a < b)
		//   gt → brif.Then (a > b)

		// Create a new return block for the eq case
		retLabel := headLabel + "_ret_eq"
		retBlk := &Block{Label: retLabel, Term: &TermRet{Vals: cr.Vals}}
		f.Blocks = append(f.Blocks, retBlk)

		// Remove the cmp instruction from head (br_if2 does the comparison)
		head.Insts = append(head.Insts[:cmpIdx], head.Insts[cmpIdx+1:]...)

		// Remove the cmp instruction from body
		body.Insts = append(body.Insts[:bodyCmpIdx], body.Insts[bodyCmpIdx+1:]...)

		// Hoist any remaining body instructions into head
		head.Insts = append(head.Insts, body.Insts...)
		body.Insts = nil

		// Set head's terminator to br_if2
		head.Term = &TermBrIf2{
			Lhs:    cmpLhs,
			Rhs:    cmpRhs,
			Eq:     retLabel,
			EqArgs: nil,
			Lt:     brif.Else,
			LtArgs: brif.ElseArgs,
			Gt:     brif.Then,
			GtArgs: brif.ThenArgs,
		}

		// body is now empty → will be cleaned by EliminateDeadBlocks
		body.Term = &TermRet{}

		return true
	})

	// elimParamForward: eliminate a block that just forwards params to its jmp target.
	// For each predecessor edge to tramp, redirect to tramp's jmp target with
	// args remapped through the param→arg forwarding.
	reg.RegisterAction("elimParamForward", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["tramp"]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		jmp, ok := blk.Term.(*TermJmp)
		if !ok {
			return false
		}

		// Build param index → jmp arg mapping
		// blk.Params[i].Dst → jmp.Args[j] where jmp.Args[j] == blk.Params[i].Dst
		// But args may be reordered, so build: paramReg → position in params
		paramIdx := make(map[Reg]int)
		for i, p := range blk.Params {
			paramIdx[p.Dst] = i
		}
		// argMap[i] = which param index feeds jmp.Args[i]
		// Since is-param-forwarding verified all args are params, we can build this
		argSourceParamIdx := make([]int, len(jmp.Args))
		for i, a := range jmp.Args {
			argSourceParamIdx[i] = paramIdx[a]
		}

		newTarget := jmp.Target
		changed := false

		for _, b := range f.Blocks {
			if b.Term == nil || b == blk {
				continue
			}
			// Remap edge args through the forwarding
			remap := func(edgeArgs []Reg) []Reg {
				if len(edgeArgs) != len(blk.Params) {
					return edgeArgs // arity mismatch, don't touch
				}
				newArgs := make([]Reg, len(jmp.Args))
				for i, srcIdx := range argSourceParamIdx {
					newArgs[i] = edgeArgs[srcIdx]
				}
				return newArgs
			}

			switch t := b.Term.(type) {
			case *TermJmp:
				if t.Target == label {
					t.Args = remap(t.Args)
					t.Target = newTarget
					changed = true
				}
			case *TermBrIf:
				if t.Then == label {
					t.ThenArgs = remap(t.ThenArgs)
					t.Then = newTarget
					changed = true
				}
				if t.Else == label {
					t.ElseArgs = remap(t.ElseArgs)
					t.Else = newTarget
					changed = true
				}
			case *TermCondRet:
				if t.Then == label {
					t.ThenArgs = remap(t.ThenArgs)
					t.Then = newTarget
					changed = true
				}
			}
		}

		if changed {
			for i, b := range f.Blocks {
				if b.Label == label {
					f.Blocks = append(f.Blocks[:i], f.Blocks[i+1:]...)
					break
				}
			}
		}
		return changed
	})

	// branchToJump: convert trivial br_if (same targets) to jump
	reg.RegisterAction("branchToJump", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		label := bindings["b"]
		blk := f.BlockByLabel(label)
		if blk == nil {
			return false
		}
		brif, ok := blk.Term.(*TermBrIf)
		if !ok || brif.Then != brif.Else {
			return false
		}
		blk.Term = &TermJmp{Target: brif.Then, Args: brif.ThenArgs}
		return true
	})

	// ── Bounded loop unrolling ──────────────────────────────────────

	// Predicate: is-bounded-sub-loop checks if head+body form a
	// "while x >= K { x -= K }" loop with bounded iterations.
	// head: CMP x, K; BrIf(cond, body, exit)
	// body: x' = SUB x, K; Jmp head (with x' as phi arg)
	reg.RegisterPredicate("is-bounded-sub-loop", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		if len(args) < 2 {
			return false
		}
		headLabel := bindings[args[0]]
		bodyLabel := bindings[args[1]]
		head := f.BlockByLabel(headLabel)
		body := f.BlockByLabel(bodyLabel)
		if head == nil || body == nil {
			return false
		}

		// Body must have exactly one instruction: SUB with constant
		if len(body.Insts) != 1 {
			return false
		}
		sub := body.Insts[0]
		if sub.Op != OpSub {
			return false
		}

		// Get the constant K — Src[1] must be a const defined in head or body
		var k int64
		for _, blk := range []*Block{head, body} {
			for _, inst := range blk.Insts {
				if inst.Dst == sub.Src[1] && inst.Op == OpConst {
					k = inst.Imm
					break
				}
			}
		}
		if k <= 0 || k > 255 {
			return false
		}

		// u8 max = 255, max iterations = 255/K
		maxIters := 255 / k
		return maxIters <= 4 // only unroll if bounded to ≤4 iterations
	})

	// Action: unrollBoundedLoop replaces the loop with a linear if-chain.
	// For maxIters=1 (K>127): while→if (break back-edge, body→exit).
	// For maxIters=2 (K>85): body→cmp2→brif2→body2→exit.
	// Higher iterations: duplicate cmp+sub blocks in sequence.
	reg.RegisterAction("unrollBoundedLoop", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		headLabel := bindings["head"]
		bodyLabel := bindings["body"]
		head := f.BlockByLabel(headLabel)
		body := f.BlockByLabel(bodyLabel)
		if head == nil || body == nil {
			return false
		}

		brif, ok := head.Term.(*TermBrIf)
		if !ok {
			return false
		}
		exitLabel := brif.Else

		// Get constant K from body's SUB
		sub := body.Insts[0]
		var k int64
		for _, blk := range []*Block{head, body} {
			for _, inst := range blk.Insts {
				if inst.Dst == sub.Src[1] && inst.Op == OpConst {
					k = inst.Imm
				}
			}
		}
		if k <= 0 {
			return false
		}

		maxIters := int(255 / k)
		if maxIters < 1 || maxIters > 4 {
			return false
		}

		// Core transform: break the back-edge (body→head) → body→exit.
		// This converts the loop to a single if-then.
		body.Term = &TermJmp{Target: exitLabel, Args: brif.ElseArgs}

		// For maxIters > 1: duplicate cmp+body blocks after the first body.
		// Chain: head→body1→cmp2→body2→...→exit
		prevBodyLabel := bodyLabel
		for i := 1; i < maxIters; i++ {
			// Duplicate CMP + BrIf block
			newCmpLabel := fmt.Sprintf("unroll_cmp%d", i)
			newCmpBlk := &Block{Label: newCmpLabel}
			for _, inst := range head.Insts {
				dup := *inst
				dup.Dst = f.AllocReg()
				newCmpBlk.Insts = append(newCmpBlk.Insts, &dup)
			}

			// Duplicate SUB block
			newSubLabel := fmt.Sprintf("unroll_sub%d", i)
			newSubBlk := &Block{Label: newSubLabel}
			dupSub := *sub
			dupSub.Dst = f.AllocReg()
			newSubBlk.Insts = append(newSubBlk.Insts, &dupSub)

			// CMP block: brif → subBlk or exit
			cmpReg := Reg(0)
			for _, inst := range newCmpBlk.Insts {
				if inst.Op == OpCmp {
					cmpReg = inst.Dst
				}
			}
			newCmpBlk.Term = &TermBrIf{
				Cond: cmpReg, Then: newSubLabel, Else: exitLabel,
				ElseArgs: brif.ElseArgs,
			}
			newSubBlk.Term = &TermJmp{Target: exitLabel, Args: brif.ElseArgs}

			f.Blocks = append(f.Blocks, newCmpBlk, newSubBlk)

			// Wire previous body → this CMP block
			prevBody := f.BlockByLabel(prevBodyLabel)
			if prevBody != nil {
				prevBody.Term = &TermJmp{Target: newCmpLabel}
			}
			prevBodyLabel = newSubLabel
		}

		return true
	})

	// ── Pointer-walk threading ─────────────────────────────────────────────

	// has-pointer-walk-opportunity: detects Shape A counted loops where the
	// body computes addr = base + idx (idx is a header block param) and
	// increments idx by a constant stride on the back-edge.
	//
	// Requirements:
	//   - header has ≥1 block param (the index)
	//   - body has an OpAdd where one source is a header param and the other
	//     is a constant (the base address), and the result is used by OpLoad/OpStore
	//   - body's back-edge jump increments that param by a constant stride
	//   - stride must be a small constant (1-256) for safety
	reg.RegisterPredicate("has-pointer-walk-opportunity", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		if len(args) < 2 {
			return false
		}
		headLabel := bindings[args[0]]
		bodyLabel := bindings[args[1]]
		head := f.BlockByLabel(headLabel)
		body := f.BlockByLabel(bodyLabel)
		if head == nil || body == nil || len(head.Params) == 0 {
			return false
		}

		_, ok := findPointerWalkCandidate(f, head, body)
		return ok
	})

	// threadPointerWalk: performs the Shape A pointer-threading transform.
	reg.RegisterAction("threadPointerWalk", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		headLabel := bindings["head"]
		bodyLabel := bindings["body"]
		head := f.BlockByLabel(headLabel)
		body := f.BlockByLabel(bodyLabel)
		if head == nil || body == nil {
			return false
		}

		cand, ok := findPointerWalkCandidate(f, head, body)
		if !ok {
			return false
		}

		// 1. Allocate a fresh register for the pointer block param
		ptrReg := f.AllocReg()

		// 2. Add pointer param to header
		head.Params = append(head.Params, BlockParam{
			Dst:   ptrReg,
			Ty:    TyU16,
			Class: ClassPointer,
		})

		// 3. Find body's corresponding param index for the pointer
		//    The body receives header params via the br_if then-args.
		//    We need to add a new param to body too (if body has params).
		bodyPtrReg := ptrReg // if body has no params, header param carries through
		brif, ok := head.Term.(*TermBrIf)
		if !ok {
			return false
		}

		if len(body.Params) > 0 {
			// Body has params — add a new one for the pointer
			bodyPtrReg = f.AllocReg()
			body.Params = append(body.Params, BlockParam{
				Dst:   bodyPtrReg,
				Ty:    TyU16,
				Class: ClassPointer,
			})
			brif.ThenArgs = append(brif.ThenArgs, ptrReg)
		} else {
			// Body has no params — it uses header's regs directly.
			// The pointer reg from header is available in body.
			bodyPtrReg = ptrReg
			// Still need to add to then-args if there are any
			if len(brif.ThenArgs) > 0 {
				brif.ThenArgs = append(brif.ThenArgs, ptrReg)
			}
		}

		// 4. Replace the address computation (add base, idx) with the pointer reg
		//    cand.addrInst is the instruction in body that computes the address.
		//    Replace its result register usage with bodyPtrReg.
		addrReg := cand.addrInst.Dst
		replaceRegInBlock(body, addrReg, bodyPtrReg)

		// Remove the address computation instruction
		removeInstFromBlock(body, cand.addrInstIdx)

		// 5. Add pointer increment instruction in body (before the back-edge jump)
		ptrNextReg := f.AllocReg()
		strideConst := f.AllocReg()
		// Insert stride const and add before the terminator
		body.Insts = append(body.Insts,
			&Inst{Op: OpConst, Dst: strideConst, Imm: cand.stride, Ty: TyU16},
			&Inst{Op: OpAdd, Dst: ptrNextReg, Src: [2]Reg{bodyPtrReg, strideConst}, Ty: TyU16, Cls: ClassPointer},
		)

		// 6. Update back-edge jump args to include ptrNextReg
		bodyJmp, ok := body.Term.(*TermJmp)
		if !ok {
			return false
		}
		bodyJmp.Args = append(bodyJmp.Args, ptrNextReg)

		// 7. Update all OTHER edges to header to include the initial pointer value.
		//    For the pre-header (entry block or setup block that jumps to header),
		//    we need to compute base + initial_idx and pass it as the new arg.
		//    For simplicity: find all predecessors of header and add the base constant
		//    as the initial pointer value.
		for _, blk := range f.Blocks {
			if blk == body {
				continue // already handled
			}
			addInitialPtrArg(f, blk, head.Label, cand.baseImm, cand.idxParamIdx)
		}

		return true
	})

	// ── Conditional call predicates & action ──────────────────────────

	// is-single-void-call: block has exactly one OpCall with Dst==NoReg
	reg.RegisterPredicate("is-single-void-call", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		label := bindings[args[0]]
		blk := f.BlockByLabel(label)
		if blk == nil || len(blk.Insts) != 1 {
			return false
		}
		return blk.Insts[0].Op == OpCall && blk.Insts[0].Dst == NoReg
	})

	// jumps-to-else: the call block's TermJmp target matches the BrIf's else target
	reg.RegisterPredicate("jumps-to-else", func(graph rewrite.IRGraph, bindings grace.BlockBindings, args []string) bool {
		curLabel := bindings[args[0]]
		callLabel := bindings[args[1]]
		curBlock := f.BlockByLabel(curLabel)
		callBlock := f.BlockByLabel(callLabel)
		if curBlock == nil || callBlock == nil {
			return false
		}
		brif, ok := curBlock.Term.(*TermBrIf)
		if !ok {
			return false
		}
		jmp, ok := callBlock.Term.(*TermJmp)
		if !ok {
			return false
		}
		// Call block jumps to BrIf's else (join) block, no block args
		return jmp.Target == brif.Else && len(jmp.Args) == 0 && len(brif.ThenArgs) == 0
	})

	// foldCondCall: rewrite BrIf+call-block into OpCallCond
	reg.RegisterAction("foldCondCall", func(graph rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
		curLabel := bindings["cur"]
		callLabel := bindings["call"]
		curBlock := f.BlockByLabel(curLabel)
		callBlock := f.BlockByLabel(callLabel)
		if curBlock == nil || callBlock == nil {
			return false
		}
		brif, ok := curBlock.Term.(*TermBrIf)
		if !ok {
			return false
		}
		callInst := callBlock.Insts[0]

		condCall := &Inst{
			Op:   OpCallCond,
			Dst:  NoReg,
			Src:  [2]Reg{brif.Cond, 0},
			Sym:  callInst.Sym,
			Args: callInst.Args,
			Ty:   callInst.Ty,
			Cls:  callInst.Cls,
			Cond: findCondForReg(curBlock, brif.Cond),
		}

		curBlock.Insts = append(curBlock.Insts, condCall)
		curBlock.Term = &TermJmp{Target: brif.Else, Args: brif.ElseArgs}

		callBlock.Insts = nil
		callBlock.Term = &TermJmp{Target: brif.Else}
		return true
	})

	return reg
}

// pointerWalkCandidate describes a detected pointer-walk opportunity in a loop body.
type pointerWalkCandidate struct {
	idxParamIdx int    // index of the header param used as loop index
	addrInst    *Inst  // the OpAdd instruction computing base + idx
	addrInstIdx int    // index of addrInst in body.Insts
	baseImm     int64  // the constant base address
	stride      int64  // the constant increment per iteration
}

// findPointerWalkCandidate searches for the pattern:
//   addr = add(const_base, idx_param) where addr is used by load/store
//   idx_next = add(idx_param, const_stride) where idx_next flows back to header
func findPointerWalkCandidate(f *Func, head, body *Block) (pointerWalkCandidate, bool) {
	// Build set of header param registers
	headerParamRegs := make(map[Reg]int) // reg → param index
	for i, p := range head.Params {
		headerParamRegs[p.Dst] = i
	}

	// Map body params to header params (via br_if then-args)
	bodyToHeaderParam := make(map[Reg]int)
	brif, ok := head.Term.(*TermBrIf)
	if !ok {
		return pointerWalkCandidate{}, false
	}
	if len(body.Params) > 0 {
		for i, bp := range body.Params {
			if i < len(brif.ThenArgs) {
				if pidx, ok := headerParamRegs[brif.ThenArgs[i]]; ok {
					bodyToHeaderParam[bp.Dst] = pidx
				}
			}
		}
	} else {
		// Body uses header params directly
		for reg, idx := range headerParamRegs {
			bodyToHeaderParam[reg] = idx
		}
	}

	// Find constants defined in body
	bodyConsts := make(map[Reg]int64)
	for _, inst := range body.Insts {
		if inst.Op == OpConst {
			bodyConsts[inst.Dst] = inst.Imm
		}
	}
	// Also check constants in head
	headConsts := make(map[Reg]int64)
	for _, inst := range head.Insts {
		if inst.Op == OpConst {
			headConsts[inst.Dst] = inst.Imm
		}
	}

	// Find addr = add(base_const, idx_param) where addr is used by load/store
	for i, inst := range body.Insts {
		if inst.Op != OpAdd || inst.Ty != TyU16 {
			continue
		}

		// Check both orderings: add(base, idx) or add(idx, base)
		var baseImm int64
		var idxReg Reg
		var idxParamIdx int
		found := false

		// Try src[0]=base_const, src[1]=idx_param
		if base, ok := bodyConsts[inst.Src[0]]; ok {
			if pidx, ok := bodyToHeaderParam[inst.Src[1]]; ok {
				baseImm = base
				idxReg = inst.Src[1]
				idxParamIdx = pidx
				found = true
			}
		}
		if !found {
			if base, ok := headConsts[inst.Src[0]]; ok {
				if pidx, ok := bodyToHeaderParam[inst.Src[1]]; ok {
					baseImm = base
					idxReg = inst.Src[1]
					idxParamIdx = pidx
					found = true
				}
			}
		}
		// Try src[1]=base_const, src[0]=idx_param
		if !found {
			if base, ok := bodyConsts[inst.Src[1]]; ok {
				if pidx, ok := bodyToHeaderParam[inst.Src[0]]; ok {
					baseImm = base
					idxReg = inst.Src[0]
					idxParamIdx = pidx
					found = true
				}
			}
		}
		if !found {
			if base, ok := headConsts[inst.Src[1]]; ok {
				if pidx, ok := bodyToHeaderParam[inst.Src[0]]; ok {
					baseImm = base
					idxReg = inst.Src[0]
					idxParamIdx = pidx
					found = true
				}
			}
		}

		if !found {
			continue
		}

		// Verify addr result is used by load or store in this block
		addrReg := inst.Dst
		usedByMem := false
		for _, user := range body.Insts {
			if user == inst {
				continue
			}
			if (user.Op == OpLoad && user.Src[0] == addrReg) ||
				(user.Op == OpStore && user.Src[0] == addrReg) {
				usedByMem = true
				break
			}
		}
		if !usedByMem {
			continue
		}

		// Find stride: look for idx_next = add(idx_param, const_stride)
		// where idx_next is passed back to header via the back-edge jump.
		jmp, ok := body.Term.(*TermJmp)
		if !ok || jmp.Target != head.Label {
			continue
		}

		// Find which back-edge arg corresponds to idxParamIdx
		if idxParamIdx >= len(jmp.Args) {
			continue
		}
		idxNextReg := jmp.Args[idxParamIdx]

		// Find the instruction that defines idxNextReg
		var stride int64
		strideFound := false
		for _, sinst := range body.Insts {
			if sinst.Dst != idxNextReg || sinst.Op != OpAdd {
				continue
			}
			// Check: add(idx, stride_const) or add(stride_const, idx)
			if sinst.Src[0] == idxReg {
				if s, ok := bodyConsts[sinst.Src[1]]; ok && s >= 1 && s <= 256 {
					stride = s
					strideFound = true
				}
			} else if sinst.Src[1] == idxReg {
				if s, ok := bodyConsts[sinst.Src[0]]; ok && s >= 1 && s <= 256 {
					stride = s
					strideFound = true
				}
			}
		}
		if !strideFound {
			continue
		}
		_ = idxReg

		return pointerWalkCandidate{
			idxParamIdx: idxParamIdx,
			addrInst:    inst,
			addrInstIdx: i,
			baseImm:     baseImm,
			stride:      stride,
		}, true
	}

	return pointerWalkCandidate{}, false
}

// replaceRegInBlock replaces all uses of oldReg with newReg in a block's instructions and terminator.
func replaceRegInBlock(blk *Block, oldReg, newReg Reg) {
	for _, inst := range blk.Insts {
		if inst.Src[0] == oldReg {
			inst.Src[0] = newReg
		}
		if inst.Src[1] == oldReg {
			inst.Src[1] = newReg
		}
		// Don't replace Dst — the definition is being removed, not renamed
	}
	// Replace in terminator args
	switch t := blk.Term.(type) {
	case *TermJmp:
		for i, r := range t.Args {
			if r == oldReg {
				t.Args[i] = newReg
			}
		}
	case *TermBrIf:
		if t.Cond == oldReg {
			t.Cond = newReg
		}
		for i, r := range t.ThenArgs {
			if r == oldReg {
				t.ThenArgs[i] = newReg
			}
		}
		for i, r := range t.ElseArgs {
			if r == oldReg {
				t.ElseArgs[i] = newReg
			}
		}
	case *TermRet:
		for i, r := range t.Vals {
			if r == oldReg {
				t.Vals[i] = newReg
			}
		}
	}
}

// removeInstFromBlock removes instruction at index i from a block.
func removeInstFromBlock(blk *Block, i int) {
	blk.Insts = append(blk.Insts[:i], blk.Insts[i+1:]...)
}

// addInitialPtrArg updates edges from blk to headerLabel to include an initial
// pointer value (base + initial_idx) as the last argument.
func addInitialPtrArg(f *Func, blk *Block, headerLabel string, baseImm int64, idxParamIdx int) {
	switch t := blk.Term.(type) {
	case *TermJmp:
		if t.Target == headerLabel && len(t.Args) > idxParamIdx {
			// Compute initial pointer: base + initial_idx
			idxArg := t.Args[idxParamIdx]
			baseReg := f.AllocReg()
			ptrInitReg := f.AllocReg()
			blk.Insts = append(blk.Insts,
				&Inst{Op: OpConst, Dst: baseReg, Imm: baseImm, Ty: TyU16},
				&Inst{Op: OpAdd, Dst: ptrInitReg, Src: [2]Reg{baseReg, idxArg}, Ty: TyU16, Cls: ClassPointer},
			)
			t.Args = append(t.Args, ptrInitReg)
		}
	case *TermBrIf:
		if t.Then == headerLabel && len(t.ThenArgs) > idxParamIdx {
			idxArg := t.ThenArgs[idxParamIdx]
			baseReg := f.AllocReg()
			ptrInitReg := f.AllocReg()
			blk.Insts = append(blk.Insts,
				&Inst{Op: OpConst, Dst: baseReg, Imm: baseImm, Ty: TyU16},
				&Inst{Op: OpAdd, Dst: ptrInitReg, Src: [2]Reg{baseReg, idxArg}, Ty: TyU16, Cls: ClassPointer},
			)
			t.ThenArgs = append(t.ThenArgs, ptrInitReg)
		}
		if t.Else == headerLabel && len(t.ElseArgs) > idxParamIdx {
			idxArg := t.ElseArgs[idxParamIdx]
			baseReg := f.AllocReg()
			ptrInitReg := f.AllocReg()
			blk.Insts = append(blk.Insts,
				&Inst{Op: OpConst, Dst: baseReg, Imm: baseImm, Ty: TyU16},
				&Inst{Op: OpAdd, Dst: ptrInitReg, Src: [2]Reg{baseReg, idxArg}, Ty: TyU16, Cls: ClassPointer},
			)
			t.ElseArgs = append(t.ElseArgs, ptrInitReg)
		}
	}
}

// ── Public API ──────────────────────────────────────────────────────────────

// RunGracePasses runs all Grace-declared optimization passes on a function.
// Returns true if any transformation was applied.
// Stats (if non-nil) accumulates per-rule statistics.
func RunGracePasses(f *Func, stats *GraceStats) bool {
	rules, err := loadAllRules()
	if err != nil {
		return false
	}

	reg := buildRegistry(f)
	graph := FuncAsMutGraph(f)

	result := grace.ApplyRules(graph, rules, reg, 50)

	// Run shape-fact-driven passes after declarative Grace rules.

	// Phase 1: True pointer threading — rewrite indexed loops to carry walking pointers.
	var ptStats PointerThreadingStats
	ptChanged := ApplyPointerThreading(f, &ptStats)

	// Phase 2: PtrAdd CSE — deduplicate remaining repeated ptr_add in loop bodies.
	var pwStats PointerWalkStats
	pwChanged := ApplyPointerWalkDedup(f, &pwStats)

	if stats != nil {
		stats.mu.Lock()
		stats.Funcs++
		stats.Total += result.Applied + ptStats.PtrAddsRemoved + pwStats.PtrAddsDeduped
		for name, count := range result.ByRule {
			stats.ByRule[name] += count
		}
		if ptStats.LoopsThreaded > 0 {
			stats.ByRule["ptr-threading"] += ptStats.LoopsThreaded
		}
		if pwStats.PtrAddsDeduped > 0 {
			stats.ByRule["ptr-add-cse"] += pwStats.PtrAddsDeduped
		}
		stats.mu.Unlock()
	}

	return result.Applied > 0 || ptChanged || pwChanged
}

// RunGraceCondRetSink runs only the cond-ret-sink Grace rule.
func RunGraceCondRetSink(f *Func) bool {
	rules, err := grace.ParseRules(graceCondRetSinkRule)
	if err != nil {
		return false
	}
	reg := buildRegistry(f)
	graph := FuncAsMutGraph(f)
	result := grace.ApplyRules(graph, rules, reg, 20)

	// Also apply fusionSubCmpInBlock (always runs, per original)
	if result.Applied > 0 {
		for _, blk := range f.Blocks {
			fusionSubCmpInBlock(blk)
		}
	}
	return result.Applied > 0
}

// RunGraceSplitJoinRet runs only the split-join-ret Grace rule.
func RunGraceSplitJoinRet(f *Func) bool {
	rules, err := grace.ParseRules(graceSplitJoinRetRule)
	if err != nil {
		return false
	}
	reg := buildRegistry(f)
	graph := FuncAsMutGraph(f)
	result := grace.ApplyRules(graph, rules, reg, 20)
	return result.Applied > 0
}

// RunGraceDeadBlockArgElim runs only the dead-block-arg Grace rule.
func RunGraceDeadBlockArgElim(f *Func) bool {
	rules, err := grace.ParseRules(graceDeadBlockArgRule)
	if err != nil {
		return false
	}
	reg := buildRegistry(f)
	graph := FuncAsMutGraph(f)
	result := grace.ApplyRules(graph, rules, reg, 50)
	return result.Applied > 0
}

// RunGraceDSE runs dead store elimination via Grace.
// Runs to fixpoint (internally loops through all blocks).
func RunGraceDSE(f *Func) {
	rules, err := grace.ParseRules(graceDSERule)
	if err != nil {
		return
	}
	reg := buildRegistry(f)
	graph := FuncAsMutGraph(f)
	// Run to fixpoint — removing one dead inst may expose another
	grace.ApplyRules(graph, rules, reg, 100)
}

// valueKey computes a string key representing the "value" of a register,
// tracing through extensions and ptr_add to the underlying block params.
// E.g. ptr_add(%r26, ext(%r27)) → "ptr_add(bp:r26,ext(bp:r27))"
// Returns "" if the value cannot be determined.
func valueKey(r Reg, f *Func) string {
	return valueKeyDepth(r, f, 5) // max depth to prevent infinite loops
}

func valueKeyDepth(r Reg, f *Func, depth int) string {
	if depth <= 0 {
		return ""
	}
	// Check if r is a block param — use the param's position as identity
	for _, b := range f.Blocks {
		for pi, p := range b.Params {
			if p.Dst == r {
				return fmt.Sprintf("bp:%s:%d", b.Label, pi)
			}
		}
	}
	// Check function params
	for pi, p := range f.Contract.Params {
		if p.Reg == r {
			return fmt.Sprintf("fp:%d", pi)
		}
	}
	// Find the defining instruction
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst != r {
				continue
			}
			switch inst.Op {
			case OpExt, OpSext:
				inner := valueKeyDepth(inst.Src[0], f, depth-1)
				if inner == "" {
					return ""
				}
				return fmt.Sprintf("ext(%s)", inner)
			case OpPtrAdd:
				k0 := valueKeyDepth(inst.Src[0], f, depth-1)
				k1 := valueKeyDepth(inst.Src[1], f, depth-1)
				if k0 == "" || k1 == "" {
					return ""
				}
				return fmt.Sprintf("ptr_add(%s,%s)", k0, k1)
			case OpConst:
				return fmt.Sprintf("const(%d)", inst.Imm)
			default:
				return fmt.Sprintf("r%d", r)
			}
		}
	}
	return fmt.Sprintf("r%d", r)
}

// addEdgeArg adds a register to all outgoing edges from block to target.
func addEdgeArg(block *Block, target string, reg Reg) {
	if block.Term == nil {
		return
	}
	switch t := block.Term.(type) {
	case *TermJmp:
		if t.Target == target {
			t.Args = append(t.Args, reg)
		}
	case *TermBrIf:
		if t.Then == target {
			t.ThenArgs = append(t.ThenArgs, reg)
		}
		if t.Else == target {
			t.ElseArgs = append(t.ElseArgs, reg)
		}
	case *TermCondRet:
		if t.Then == target {
			t.ThenArgs = append(t.ThenArgs, reg)
		}
	}
}

// remapTermRegs replaces register references in a terminator using remap.
func remapTermRegs(t Term, remap map[Reg]Reg) {
	repl := func(r *Reg) {
		if newR, ok := remap[*r]; ok {
			*r = newR
		}
	}
	replSlice := func(rs []Reg) {
		for i := range rs {
			repl(&rs[i])
		}
	}
	switch t := t.(type) {
	case *TermJmp:
		replSlice(t.Args)
	case *TermBrIf:
		repl(&t.Cond)
		replSlice(t.ThenArgs)
		replSlice(t.ElseArgs)
	case *TermBrIf2:
		repl(&t.Lhs)
		repl(&t.Rhs)
		replSlice(t.EqArgs)
		replSlice(t.LtArgs)
		replSlice(t.GtArgs)
	case *TermRet:
		replSlice(t.Vals)
	case *TermCondRet:
		repl(&t.Cond)
		replSlice(t.Vals)
		replSlice(t.ThenArgs)
	}
}

// isNarrowable checks if an instruction is a widened arithmetic that can be
// safely narrowed from i16/u16 to u8. This catches C89 integer promotion:
// uint8_t + uint8_t → i16 in C, but we can keep it as u8 on Z80.
func isNarrowable(inst *Inst, f *Func) bool {
	// Only narrow bitwise ops and self-adds (x+x → ADD A,A).
	// Do NOT narrow add/sub of different values — overflow detection may be needed
	// (e.g. saturating add needs i16 to detect > 255).
	switch inst.Op {
	case OpAnd, OpOr, OpXor:
		// Bitwise ops never overflow — safe to narrow
	case OpAdd:
		// Only narrow x+x (same source, ADD A,A pattern)
		if inst.Src[0] != inst.Src[1] {
			return false
		}
	default:
		return false
	}

	// Must be i16/u16 result
	if inst.Ty != TyI16 && inst.Ty != TyU16 {
		return false
	}
	// Both sources must be u8/i8 (possibly extended)
	src0Ty := srcOrigType(inst.Src[0], f)
	src1Ty := srcOrigType(inst.Src[1], f)
	if src0Ty.Width() > 8 || src1Ty.Width() > 8 {
		return false
	}
	// Result must only be used as u8
	return allUsesNarrow(inst.Dst, f)
}

// srcOrigType traces back to find the original type of a register before extension.
func srcOrigType(r Reg, f *Func) Ty {
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst == r {
				if inst.Op == OpExt || inst.Op == OpSext {
					return inst.SrcTy // original type before extension
				}
				return inst.Ty
			}
		}
		for _, p := range b.Params {
			if p.Dst == r {
				return p.Ty
			}
		}
	}
	// Check function params
	if len(f.Blocks) > 0 {
		for _, p := range f.Contract.Params {
			if p.Reg == r {
				return p.Ty
			}
		}
	}
	return TyU16 // unknown → don't narrow
}

// allUsesNarrow checks that all uses of reg are in u8 context.
func allUsesNarrow(r Reg, f *Func) bool {
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			for _, use := range inst.Uses() {
				if use == r {
					// Trunc is always safe (intentional narrowing)
					if inst.Op == OpTrunc {
						continue
					}
					// u8 consumer is safe
					if inst.Ty.Width() <= 8 {
						continue
					}
					return false
				}
			}
		}
		if b.Term != nil {
			for _, use := range b.Term.termUses() {
				if use == r {
					// Check return type
					if ret, ok := b.Term.(*TermRet); ok {
						_ = ret
						// Return as u8 — check function return type
						if len(f.Contract.Returns) > 0 && f.Contract.Returns[0].Ty.Width() <= 8 {
							continue
						}
					}
					return false
				}
			}
		}
	}
	return true
}

// RunGraceFuseAbsDiff runs abs-diff fusion via Grace.
func RunGraceFuseAbsDiff(f *Func) bool {
	rules, err := grace.ParseRules(graceFuseAbsDiffRule)
	if err != nil {
		return false
	}
	reg := buildRegistry(f)
	graph := FuncAsMutGraph(f)
	result := grace.ApplyRules(graph, rules, reg, 20)
	return result.Applied > 0
}
