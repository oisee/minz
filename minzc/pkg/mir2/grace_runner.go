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
		allSrc := graceCondRetSinkRule + graceSplitJoinRetRule + graceDeadBlockArgRule + graceDSERule + graceFuseAbsDiffRule
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

	return reg
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

	if stats != nil {
		stats.mu.Lock()
		stats.Funcs++
		stats.Total += result.Applied
		for name, count := range result.ByRule {
			stats.ByRule[name] += count
		}
		stats.mu.Unlock()
	}

	return result.Applied > 0
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
