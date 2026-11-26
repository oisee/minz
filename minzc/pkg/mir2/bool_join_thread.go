package mir2

// ThreadBoolJoins removes the boolean that a short-circuit operator (or a
// `?:`) materialises when its only consumer is a branch.
//
// Both frontends lower `a && b` to a value join: C through a conditional
// expression (cond_then/cond_else/cond_join), Nanz through logic_true /
// logic_false / logic_join. When the value is a branch condition the join is
//
//	P1: ... %r = cmp.<cc> x, y ; jmp @J(%r, ...)
//	P2: ... jmp @J(%c0, ...)                       ; %c0 = const 0
//	J(%p: bool, %q...):
//	    [%z = const 0] [%e = ext %p] [%t = cmp.ne|cmp.eq (%p|%e), %z]
//	    br_if (%t|%p), @T(targs), @F(fargs)
//
// and the code generator builds a 0/1 byte from the flags of %r, tests it
// again and branches a second time. This pass redirects each predecessor:
//
//	P1: ... %r = cmp.<cc> x, y ; br_if %r, @T(targs'), @F(fargs')   (swapped for cmp.eq)
//	P2: ... jmp @F(fargs')                                         (selected by the constant)
//
// where targs'/fargs' replace J's parameters by the predecessor's arguments.
// J is deleted when no predecessor is left.
//
// # Proof conditions (each checked; any failure leaves the join untouched)
//
//  1. The function is not fallible (@error/CY) and contains no inline asm.
//  2. J is not the entry, ends in TermBrIf (not BrIf2/DJNZ/CondRet/ret), and
//     its instructions are only the test chain above: OpConst, one OpExt of
//     %p, one OpCmp ne/eq of the tested value against a constant 0 (defined
//     in J or elsewhere). None of
//     them has an effect.
//  3. Every register J defines (parameters and instruction results) is used
//     only inside J. %p and the chain are used only by the chain and the
//     br_if condition; in particular %p is not passed on in targs/fargs. Any
//     other parameter %q may appear only in targs/fargs, where it is
//     substituted by the predecessor's argument on the new edge.
//  4. Values in targs/fargs not defined in J are defined outside J and
//     dominate J, hence every predecessor of J (a path to a predecessor that
//     avoided them would reach J avoiding them), so they are available on
//     the new edge. Removing paths through J only enlarges dominance, and a
//     new path entry→P→T contains everything the old path entry→P→J→T
//     contained except J, so no other use loses its dominating definition.
//  5. J is not a loop header (no predecessor is dominated by J), its br_if
//     targets do not dominate J (J is not a latch), and J carries no
//     LoopOrigin or StaticLoop attribution, so removing it cannot strand
//     loop metadata or change a loop's header/latch structure.
//  6. A predecessor is rewritten only if its terminator is an unconditional
//     TermJmp to J. A predecessor reaching J by a conditional edge (BrIf,
//     BrIf2, DJNZ, CondRet) is left alone and keeps J alive.
//  7. The argument for %p is either a known constant (masked to %p's type;
//     the selected successor is fixed) or the result of an OpCmp that is the
//     predecessor's last instruction, so br_if on it reads the value J's
//     test read (0 or 1 survives the edge's truncation to %p's type), from
//     flags the comparison just set.
//     Any other argument leaves that predecessor alone.
//
// Flag-returning functions (a ClassFlag return) need no special case: the
// pass never changes a return terminator or a value that reaches one
// (condition 3 keeps every J-defined value local to J).
func ThreadBoolJoins(f *Func) bool {
	if !boolJoinFuncAdmitted(f) {
		return false
	}
	changed := false
	// Each successful step removes at least one jmp edge into a join, so the
	// iteration terminates; the bound is a guard, not a tuning knob.
	for iter := 0; iter < 4*len(f.Blocks)+4; iter++ {
		if !threadOneBoolJoin(f) {
			break
		}
		changed = true
	}
	return changed
}

// boolJoinFuncAdmitted is proof condition 1.
func boolJoinFuncAdmitted(f *Func) bool {
	if f.Attrs.FallibleCY || len(f.Blocks) < 2 {
		return false
	}
	for _, b := range f.Blocks {
		if b.Term == nil {
			return false
		}
		for _, inst := range b.Insts {
			if inst.Op == OpAsm {
				return false
			}
		}
	}
	return true
}

// boolJoinTest describes a join admitted by proof conditions 2 and 3.
type boolJoinTest struct {
	param  int  // index of the tested parameter
	invert bool // true for cmp.eq %p, 0: the branch is taken when %p == 0
	br     *TermBrIf
}

// matchBoolJoin checks conditions 2 and 3 for block j.
func matchBoolJoin(f *Func, j *Block, uses map[Reg]int, dead map[*Inst]bool, consts map[Reg]int64) (boolJoinTest, bool) {
	var no boolJoinTest
	if j == f.Blocks[0] || len(j.Params) == 0 {
		return no, false
	}
	br, ok := j.Term.(*TermBrIf)
	if !ok {
		return no, false
	}
	paramIdx := map[Reg]int{}
	for i, p := range j.Params {
		paramIdx[p.Dst] = i
	}
	defs := map[Reg]*Inst{}
	for _, inst := range j.Insts {
		if dead[inst] {
			continue
		}
		switch inst.Op {
		case OpConst, OpExt, OpCmp:
		default:
			return no, false
		}
		if inst.Dst == NoReg || inst.Volatile {
			return no, false
		}
		defs[inst.Dst] = inst
	}
	isZero := func(r Reg) bool {
		if d := defs[r]; d != nil {
			return d.Op == OpConst && maskToWidth(d.Imm, d.Ty) == 0
		}
		c, known := consts[r]
		return known && c == 0
	}
	// Walk the test chain backwards from the branch condition.
	tested := br.Cond
	invert := false
	chain := map[Reg]bool{}
	if c := defs[tested]; c != nil && c.Op == OpCmp {
		if c.Cond != CmpNe && c.Cond != CmpEq {
			return no, false
		}
		switch {
		case isZero(c.Src[1]):
			tested = c.Src[0]
			chain[c.Src[1]] = true
		case isZero(c.Src[0]):
			tested = c.Src[1]
			chain[c.Src[0]] = true
		default:
			return no, false
		}
		invert = c.Cond == CmpEq
		chain[c.Dst] = true
	}
	var ext *Inst
	if e := defs[tested]; e != nil && e.Op == OpExt {
		chain[e.Dst] = true
		tested = e.Src[0]
		ext = e
	}
	pi, isParam := paramIdx[tested]
	if !isParam {
		return no, false
	}
	// Zero extension preserves whether the value is zero only if it keeps
	// every bit of the parameter (the VM masks an ext to its result type).
	if ext != nil && (ext.Ty == nil || ext.Ty.Width() < j.Params[pi].Ty.Width() || ext.SrcTy != nil && ext.SrcTy != j.Params[pi].Ty) {
		return no, false
	}
	// Every instruction must belong to the chain (condition 2).
	for r := range defs {
		if !chain[r] {
			return no, false
		}
	}
	// Condition 3: J-defined registers are used only inside J; the tested
	// parameter and the chain only by the chain and the condition.
	local := map[Reg]int{}
	for _, inst := range j.Insts {
		if dead[inst] {
			continue
		}
		for _, r := range inst.Uses() {
			local[r]++
		}
	}
	local[br.Cond]++
	for _, r := range br.ThenArgs {
		local[r]++
	}
	for _, r := range br.ElseArgs {
		local[r]++
	}
	for r := range defs {
		if uses[r] != local[r] {
			return no, false
		}
	}
	for _, p := range j.Params {
		if uses[p.Dst] != local[p.Dst] {
			return no, false
		}
	}
	if uses[tested] != 1 {
		return no, false
	}
	for _, args := range [][]Reg{br.ThenArgs, br.ElseArgs} {
		for _, r := range args {
			if r == tested || defs[r] != nil {
				return no, false
			}
		}
	}
	return boolJoinTest{param: pi, invert: invert, br: br}, true
}

// threadOneBoolJoin rewrites the predecessors of the first admitted join
// that has at least one rewritable predecessor. Analyses are recomputed by
// the caller's next iteration.
func threadOneBoolJoin(f *Func) bool {
	uses, dead := boolJoinLiveUses(f)
	consts := collectKnownConsts(f)
	var dom map[string]map[string]bool
	for _, j := range f.Blocks {
		test, ok := matchBoolJoin(f, j, uses, dead, consts)
		if !ok {
			continue
		}
		// Condition 5: metadata and loop structure.
		if _, ok := f.LoopOrigins[j.Label]; ok {
			continue
		}
		if _, ok := f.StaticLoops[j.Label]; ok {
			continue
		}
		if dom == nil {
			dom = reachableDominators(f)
		}
		jdom, reachable := dom[j.Label]
		if !reachable || jdom[test.br.Then] || jdom[test.br.Else] {
			continue
		}
		var preds []*Block
		header := false
		for _, b := range f.Blocks {
			for _, s := range b.Term.Successors() {
				if s == j.Label {
					preds = append(preds, b)
					if dom[b.Label][j.Label] {
						header = true
					}
					break
				}
			}
		}
		if header {
			continue
		}
		changed := false
		for _, p := range preds {
			if threadBoolJoinPred(p, j, test, dead, consts) {
				changed = true
			}
		}
		if !changed {
			continue
		}
		stillReached := false
		for _, b := range f.Blocks {
			for _, s := range b.Term.Successors() {
				if s == j.Label {
					stillReached = true
				}
			}
		}
		if !stillReached {
			f.removeBlock(j.Label)
		}
		return true
	}
	return false
}

// threadBoolJoinPred applies conditions 6 and 7 to one predecessor.
func threadBoolJoinPred(p, j *Block, test boolJoinTest, dead map[*Inst]bool, consts map[Reg]int64) bool {
	jmp, ok := p.Term.(*TermJmp)
	if !ok || jmp.Target != j.Label || len(jmp.Args) != len(j.Params) {
		return false
	}
	subst := func(args []Reg) []Reg {
		out := make([]Reg, len(args))
		for i, r := range args {
			out[i] = r
			for k, param := range j.Params {
				if param.Dst == r {
					out[i] = jmp.Args[k]
				}
			}
		}
		return out
	}
	arg := jmp.Args[test.param]
	thenL, thenA, elseL, elseA := test.br.Then, test.br.ThenArgs, test.br.Else, test.br.ElseArgs
	if test.invert {
		thenL, thenA, elseL, elseA = elseL, elseA, thenL, thenA
	}
	if c, known := consts[arg]; known {
		if maskToWidth(c, j.Params[test.param].Ty) != 0 {
			p.Term = &TermJmp{Target: thenL, Args: subst(thenA)}
		} else {
			p.Term = &TermJmp{Target: elseL, Args: subst(elseA)}
		}
		return true
	}
	// A comparison yields 0 or 1, which every integer parameter type keeps
	// when the edge truncates it, so br_if on it reads what J's test read.
	// Dead pure instructions after it emit no code (DSE removes them before
	// allocation), so they do not separate it from the branch.
	var last *Inst
	for i := len(p.Insts) - 1; i >= 0 && last == nil; i-- {
		if !dead[p.Insts[i]] {
			last = p.Insts[i]
		}
	}
	if last == nil || last.Op != OpCmp || last.Dst != arg {
		return false
	}
	p.Term = &TermBrIf{Cond: arg, Then: thenL, ThenArgs: subst(thenA), Else: elseL, ElseArgs: subst(elseA)}
	return true
}

// boolJoinLiveUses counts register uses as DeadStoreElim would leave them,
// without deleting anything: constant folding leaves dead pure copies (the
// frontend's `!= 0` of a comparison, wide originals of narrowed values) that
// would otherwise hide the comparison ending a predecessor or count as uses
// of a join's parameter. dead holds the instructions DSE would remove.
func boolJoinLiveUses(f *Func) (map[Reg]int, map[*Inst]bool) {
	uses := countRegUses(f)
	dead := map[*Inst]bool{}
	for changed := true; changed; {
		changed = false
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if dead[inst] || inst.Dst == NoReg || uses[inst.Dst] != 0 || !isDSEPure(inst.Op) {
					continue
				}
				dead[inst] = true
				changed = true
				for _, r := range inst.Uses() {
					uses[r]--
				}
			}
		}
	}
	return uses, dead
}
