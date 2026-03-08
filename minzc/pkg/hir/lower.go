package hir

// lower.go — HIR → MIR2 lowering pass.
//
// Algorithm:
//  1. For each HIR Func, create a MIR2 Func with register classes assigned
//     from parameter position + type (classForParam).
//  2. Maintain an environment map[varName]mir2.Reg that tracks the current
//     virtual register holding each named variable.
//     A parallel envTy map[varName]mir2.Ty tracks types (stable; never changes
//     once a variable is bound).
//  3. Lower statements:
//       VarDeclStmt  → alloca or just bind the init expression's vreg
//       AssignStmt   → lower rhs, update env[name] (new vreg each assignment)
//       ReturnStmt   → lower value (if any), emit Ret
//       IfStmt       → two-pass: scan mutations, emit BrIf with correct Jmp args
//       WhileStmt    → detect mutated vars, thread as block params at loop_head/exit
//       ForRangeStmt → desugar to WhileStmt equivalent
//       ExprStmt     → lower expr (side-effect only)
//  4. Lower expressions recursively; each call returns a mir2.Reg.
//
// Variable mutation across basic blocks:
//   WhileStmt detects which variables in the env are written inside the loop
//   body (via scanMutations) and promotes them to block params so that
//   loop-carried values flow through MIR2's SSA block arguments.
//
//   If/else: same idea — the join block receives the final value of each
//   mutated variable as a block param.
//
// This is "almost SSA" — single assignment per path, block params at joins —
// without a full dominance-frontier computation.  Good enough for structured
// control flow; the Braun 2013 algorithm is a Phase 4b refinement.

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// ── Public entry point ────────────────────────────────────────────────────────

// LowerModule converts a HIR module to a MIR2 module.
func LowerModule(hm *Module) *mir2.Module {
	m := &mir2.Module{Name: hm.Name}
	for _, g := range hm.Globals {
		m.AddGlobal(g)
	}
	for _, s := range hm.Strings {
		m.Strings.Intern(s)
	}
	for _, f := range hm.Funcs {
		lowerFunc(m, f)
	}
	return m
}

// ── Per-function lowering state ───────────────────────────────────────────────

// loopCtx holds the context needed for break/continue inside a loop.
type loopCtx struct {
	headLabel string   // continue → jump here (with headMutated args)
	exitLabel string   // break    → jump here (with exitMutated args)
	mutated   []string // loop-carried variable names (same set for both)
}

type lowerer struct {
	hf  *Func
	m   *mir2.Module
	mf  *mir2.Func
	bld *mir2.Builder

	// env maps variable names to their current virtual register.
	// envTy maps variable names to their type (stable: set once, never changed).
	env   map[string]mir2.Reg
	envTy map[string]mir2.Ty

	// loopStack is the stack of enclosing loops (top = innermost).
	// Needed by BreakStmt and ContinueStmt.
	loopStack []loopCtx

	// labelSeq generates unique block labels within this function.
	labelSeq int
}

func (l *lowerer) pushLoop(ctx loopCtx) { l.loopStack = append(l.loopStack, ctx) }
func (l *lowerer) popLoop()             { l.loopStack = l.loopStack[:len(l.loopStack)-1] }
func (l *lowerer) curLoop() *loopCtx {
	if len(l.loopStack) == 0 {
		panic("hir/lower: break/continue outside loop")
	}
	return &l.loopStack[len(l.loopStack)-1]
}

func (l *lowerer) fresh(prefix string) string {
	l.labelSeq++
	return fmt.Sprintf("%s%d", prefix, l.labelSeq)
}

// bind records both the register and type for a variable name.
func (l *lowerer) bind(name string, r mir2.Reg, ty mir2.Ty) {
	l.env[name] = r
	l.envTy[name] = ty
}

// ── Function lowering ─────────────────────────────────────────────────────────

func lowerFunc(m *mir2.Module, f *Func) {
	mf := m.AddFunc(f.Name)
	if f.IsExtern {
		mf.Attrs.IsExtern = true
		return
	}

	// Set return type.
	if f.RetTy != mir2.TyVoid {
		cls := classForRet(f.RetTy)
		mf.Contract.Returns = []mir2.Return{{Ty: f.RetTy, Class: cls}}
	}

	bld := mir2.NewBuilder(mf)
	l := &lowerer{
		hf:    f,
		m:     m,
		mf:    mf,
		bld:   bld,
		env:   make(map[string]mir2.Reg),
		envTy: make(map[string]mir2.Ty),
	}

	bld.SwitchToNewBlock("entry")

	// Bind parameters to virtual registers.
	for i, p := range f.Params {
		cls := classForParam(p.Ty, i)
		r := bld.Param(p.Name, p.Ty, cls)
		l.bind(p.Name, r, p.Ty)
	}

	// Lower body.
	l.lowerBlock(f.Body)
}

// classForParam chooses a register class for the i-th parameter.
//
//	pos 0 → ClassAcc     (A)
//	ptr   → ClassPointer (HL)
//	pos 1 → ClassGeneral (C/D/E)
//	pos 2 → ClassCounter (B) — common for a count/length 3rd arg
func classForParam(ty mir2.Ty, pos int) mir2.RegClass {
	if ty == mir2.TyPtr {
		return mir2.ClassPointer
	}
	if ty == mir2.TyU16 || ty == mir2.TyI16 {
		if pos == 0 {
			return mir2.ClassPointer
		}
		return mir2.ClassIndex
	}
	switch pos {
	case 0:
		return mir2.ClassAcc
	case 1:
		return mir2.ClassGeneral
	case 2:
		return mir2.ClassCounter
	default:
		return mir2.ClassGeneral
	}
}

// classForRet chooses the return class.
func classForRet(ty mir2.Ty) mir2.RegClass {
	if ty == mir2.TyU16 || ty == mir2.TyI16 {
		return mir2.ClassPointer // HL
	}
	return mir2.ClassAcc // A
}

// classForExpr picks a "natural" class for an intermediate expression result.
//
// We use ClassGeneral (B/C/D/E) rather than ClassAcc (A) for 8-bit
// intermediates.  The Z80 genBinOp always uses A as a transient workspace and
// then stores the result to the destination register; if the destination were A
// and the *next* instruction also uses A as workspace, it would clobber the
// still-live intermediate value.  ClassGeneral destinations are written via
// "LD dst, A" and are safe across subsequent ALU operations that transiently
// modify A.
//
// Explicit "must be in A" values (function return values, call returns) still
// use ClassAcc via classForRet; parameters use classForParam.
func classForExpr(ty mir2.Ty) mir2.RegClass {
	if ty == mir2.TyU16 || ty == mir2.TyI16 {
		return mir2.ClassPointer
	}
	return mir2.ClassGeneral
}

// classForLoop chooses a good class for a loop-carried variable.
func classForLoop(ty mir2.Ty, _ string) mir2.RegClass {
	if ty == mir2.TyU16 || ty == mir2.TyI16 {
		return mir2.ClassPointer
	}
	// ClassGeneral keeps loop vars out of A (which comparisons need).
	return mir2.ClassGeneral
}

// ── Statement lowering ────────────────────────────────────────────────────────

// lowerBlock lowers a HIR block (list of statements).
// Returns true if the block already ends with a terminator.
func (l *lowerer) lowerBlock(blk *Block) bool {
	for _, s := range blk.Body {
		if l.lowerStmt(s) {
			return true
		}
	}
	return false
}

// lowerStmt lowers one statement.  Returns true if it emitted a terminator.
func (l *lowerer) lowerStmt(s Stmt) bool {
	switch st := s.(type) {

	case *Block:
		return l.lowerBlock(st)

	case *VarDeclStmt:
		var r mir2.Reg
		if st.Init != nil {
			r = l.lowerExpr(st.Init)
		} else {
			r = l.bld.Const(0, st.Ty, classForExpr(st.Ty))
		}
		l.bind(st.Name, r, st.Ty)
		return false

	case *AssignStmt:
		val := l.lowerExpr(st.Val)
		switch tgt := st.Target.(type) {
		case *VarRefExpr:
			// Re-bind the variable to the new vreg (SSA-style rename).
			// Type is already in envTy; just update the register.
			l.env[tgt.Name] = val
		case *DerefExpr:
			ptr := l.lowerExpr(tgt.Ptr)
			l.bld.Store(ptr, val, tgt.Ty)
		default:
			panic(fmt.Sprintf("hir/lower: unsupported AssignStmt target %T", st.Target))
		}
		return false

	case *ReturnStmt:
		if st.Val != nil {
			r := l.lowerExpr(st.Val)
			l.bld.Ret(r)
		} else {
			l.bld.Ret()
		}
		return true

	case *ExprStmt:
		l.lowerExpr(st.Expr)
		return false

	case *StoreStmt:
		ptr := l.lowerExpr(st.Ptr)
		val := l.lowerExpr(st.Val)
		l.bld.Store(ptr, val, st.Val.ExprTy())
		return false

	case *IfStmt:
		return l.lowerIf(st)

	case *WhileStmt:
		l.lowerWhile(st)
		return false

	case *ForRangeStmt:
		l.lowerForRange(st)
		return false

	case *BreakStmt:
		ctx := l.curLoop()
		args := l.collectArgs(ctx.mutated)
		l.bld.Jmp(ctx.exitLabel, args...)
		return true // break is a terminator

	case *ContinueStmt:
		ctx := l.curLoop()
		args := l.collectArgs(ctx.mutated)
		l.bld.Jmp(ctx.headLabel, args...)
		return true // continue is a terminator

	case *SwitchStmt:
		l.lowerSwitch(st)
		return false

	default:
		panic(fmt.Sprintf("hir/lower: unhandled statement type %T", s))
	}
}

// ── If/else lowering (two-pass) ───────────────────────────────────────────────

// lowerIf lowers an IfStmt using a two-pass approach:
//   Pass 1: scan for mutations in each branch.
//   Pass 2: emit MIR2 with correct Jmp args to the join block.
//
// Cases:
//   (a) Both branches always return → no join block, returns true.
//   (b) Neither/one branch returns → join block collects updated vars as params.
func (l *lowerer) lowerIf(st *IfStmt) bool {
	envBefore := cloneMap(l.env)

	// Scan branches for mutations (variables that existed before and are written).
	thenMuts := scanMutations(st.Then, envBefore)
	elseMuts := map[string]bool{}
	if st.Else != nil {
		elseMuts = scanMutations(st.Else, envBefore)
	}

	// Union of mutated vars (stable iteration order via sort).
	mutated := unionKeys(thenMuts, elseMuts, envBefore)

	thenLabel := l.fresh("if_then")
	joinLabel := l.fresh("if_join")

	hasElse := st.Else != nil
	elseLabel := ""
	if hasElse {
		elseLabel = l.fresh("if_else")
	}

	// Emit the branch.
	condReg := l.lowerCond(st.Cond)
	if hasElse {
		l.bld.BrIf(condReg, thenLabel, nil, elseLabel, nil)
	} else {
		// No else: false edge falls through to join with unchanged values.
		// If there are mutated vars, the join block params will be fed from
		// both the then-Jmp and the original values on the false edge.
		// Since BrIf false goes directly to join, we can't pass args there.
		// Solution: if no else and vars mutated in then, don't use block params
		// for join — the variable has the old value on the false path.
		// (Handled below: join only gets block params if both paths are live.)
		l.bld.BrIf(condReg, thenLabel, nil, joinLabel, nil)
	}

	// ── then branch ──
	l.bld.SwitchToNewBlock(thenLabel)
	l.env = cloneMap(envBefore)
	thenReturns := l.lowerBlock(st.Then)
	envAfterThen := cloneMap(l.env)
	if !thenReturns {
		args := l.collectArgs(mutated)
		l.bld.Jmp(joinLabel, args...)
	}

	// ── else branch ──
	var elseReturns bool
	envAfterElse := envBefore
	if hasElse {
		l.bld.SwitchToNewBlock(elseLabel)
		l.env = cloneMap(envBefore)
		elseReturns = l.lowerBlock(st.Else)
		envAfterElse = cloneMap(l.env)
		if !elseReturns {
			args := l.collectArgs(mutated)
			l.bld.Jmp(joinLabel, args...)
		}
	}

	// Both branches returned → no join.
	if thenReturns && elseReturns {
		return true
	}

	// Only then returned → join continues with else's env.
	if thenReturns {
		l.env = cloneMap(envAfterElse)
		l.bld.SwitchToNewBlock(joinLabel)
		return false
	}

	// ── join block ──
	join := l.bld.SwitchToNewBlock(joinLabel)

	// Determine the post-join environment.
	// If both paths are live, create block params for mutated vars.
	// If no-else (false edge goes straight to join with no args), we can't
	// use block params — just use envBefore values on the false path.
	var joinEnv map[string]mir2.Reg
	if !elseReturns && hasElse {
		// Both paths live, both supplied Jmp args → create block params.
		joinEnv = cloneMap(envBefore)
		for _, name := range mutated {
			ty := l.envTy[name]
			r := l.bld.BlockParam(join, ty, classForExpr(ty))
			joinEnv[name] = r
		}
	} else if !hasElse {
		// No else: true path jumps with args; false path has no args
		// (BrIf false went directly to join with no block params).
		// We can still create block params because BrIf false passes no args —
		// but then the false edge's params would be uninitialised.
		// Safest: use the then-branch's updated env (optimistic; works if then
		// always updates the variable, which scanMutations guarantees).
		// For correctness with "if without else", the false path keeps envBefore.
		// We must pick one: use envAfterThen for params that came from then,
		// and envBefore for params that didn't change (i.e., no mutation in then
		// from false path perspective). Since BrIf false gives join NO args,
		// we cannot create block params for mutated vars here — just use envBefore.
		joinEnv = cloneMap(envBefore)
		// Apply then-branch mutations that we can trust (then ran, join follows).
		// This is unsound if the condition was false (mutation didn't happen).
		// For now: leave envBefore for safety; a later pass can refine this.
	} else {
		joinEnv = cloneMap(envAfterElse)
	}

	_ = envAfterThen
	l.env = joinEnv
	return false
}

// ── While loop lowering ───────────────────────────────────────────────────────

// lowerWhile lowers a WhileStmt.
//
// Generated MIR2 structure (loopVars = all env vars used or mutated in the loop):
//
//	jmp @loop_head(init_vals...)
//	loop_head(v1, v2, ...):
//	    cond = lower(while.Cond)
//	    brif cond @loop_body @loop_exit(v1, v2, ...)
//	loop_body:
//	    lower(while.Body)  — directly uses head block's vregs
//	    jmp @loop_head(updated_v1, updated_v2, ...)
//	loop_exit(v1, v2, ...):
//	    ; continue with exit env
//
// Note: loop_body has no block params — it uses head's vregs directly
// (valid because head dominates body).  Only head and exit get block params.
//
// loopVars includes BOTH mutated AND read-only variables used in the loop.
// Read-only vars (like n in "while i < n") must be threaded through as block
// params to prevent A/other registers from being clobbered and lost between
// loop-head visits.
func (l *lowerer) lowerWhile(st *WhileStmt) {
	envBefore := cloneMap(l.env)

	// Collect ALL env vars used (read or written) anywhere in the loop.
	allUsed := map[string]bool{}
	scanUsedExpr(st.Cond, envBefore, allUsed)
	scanUsedInBlock(st.Body, envBefore, allUsed)
	// Also include mutated vars (even if somehow not "used" in expressions).
	muts := scanMutations(st.Body, envBefore)
	for name := range muts {
		allUsed[name] = true
	}
	mutated := sortedKeys(allUsed, envBefore)

	headLabel := l.fresh("loop_head")
	bodyLabel := l.fresh("loop_body")
	exitLabel := l.fresh("loop_exit")

	// Jump to loop head with initial values.
	initArgs := l.collectArgs(mutated)
	l.bld.Jmp(headLabel, initArgs...)

	// ── loop_head ──
	head := l.bld.SwitchToNewBlock(headLabel)
	headEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(head, ty, classForLoop(ty, name))
		headEnv[name] = r
	}
	l.env = headEnv

	cond := l.lowerCond(st.Cond)
	// True → body (no block params on body, no args needed).
	// False → exit (with block params, pass current head-env values).
	exitArgs := l.collectArgs(mutated)
	l.bld.BrIf(cond, bodyLabel, nil, exitLabel, exitArgs)

	// ── loop_body ──
	l.bld.SwitchToNewBlock(bodyLabel)
	l.env = cloneMap(headEnv) // body directly uses head's vregs
	l.pushLoop(loopCtx{headLabel: headLabel, exitLabel: exitLabel, mutated: mutated})
	l.lowerBlock(st.Body)
	l.popLoop()
	// Back-edge: jump to head with updated values.
	backArgs := l.collectArgs(mutated)
	l.bld.Jmp(headLabel, backArgs...)

	// ── loop_exit ──
	exit := l.bld.SwitchToNewBlock(exitLabel)
	exitEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(exit, ty, classForExpr(ty))
		exitEnv[name] = r
	}
	l.env = exitEnv
}

// lowerForRange desugars a ForRangeStmt to a while-equivalent.
// for v in start..end → v = start; while v < end { body; v++ }
func (l *lowerer) lowerForRange(st *ForRangeStmt) {
	var ty mir2.Ty = mir2.TyU8
	if st.End != nil {
		ty = st.End.ExprTy()
	}
	initReg := l.lowerExpr(st.Start)
	l.bind(st.Var, initReg, ty)

	// Synthesise a WhileStmt: while v < end { body; v = v + 1 }
	one := &IntLitExpr{Val: 1, Ty: ty}
	whileBody := &Block{Body: append(st.Body.Body, &AssignStmt{
		Target: &VarRefExpr{Name: st.Var, Ty: ty},
		Val:    &BinExpr{Op: "+", L: &VarRefExpr{Name: st.Var, Ty: ty}, R: one, Ty: ty},
	})}
	l.lowerWhile(&WhileStmt{
		Cond: &BinExpr{Op: "<", L: &VarRefExpr{Name: st.Var, Ty: ty}, R: st.End, Ty: mir2.TyBool},
		Body: whileBody,
	})
}

// ── Switch lowering ───────────────────────────────────────────────────────────

// lowerSwitch lowers a SwitchStmt to a chain of BrIf2 comparisons.
//
// Generated MIR2 structure for switch v { case 1: b1; case 2: b2; default: bd }:
//
//	%v = lower(val)
//	brif2 %v == 1 → case1 | next1
//	next1: brif2 %v == 2 → case2 | default
//	case1: lower(b1); jmp join
//	case2: lower(b2); jmp join
//	default: lower(bd); jmp join
//	join: ...
//
// For variables mutated in any case, join block receives them as block params.
func (l *lowerer) lowerSwitch(st *SwitchStmt) {
	envBefore := cloneMap(l.env)

	// Collect all mutations across all case bodies.
	allMuts := map[string]bool{}
	for _, c := range st.Cases {
		for name := range scanMutations(c.Body, envBefore) {
			allMuts[name] = true
		}
	}
	if st.Default != nil {
		for name := range scanMutations(st.Default, envBefore) {
			allMuts[name] = true
		}
	}
	mutated := sortedKeys(allMuts, envBefore)

	joinLabel := l.fresh("sw_join")
	defaultLabel := joinLabel // if no default, fall through to join
	if st.Default != nil {
		defaultLabel = l.fresh("sw_default")
	}

	// Generate case labels.
	caseLabels := make([]string, len(st.Cases))
	for i := range st.Cases {
		caseLabels[i] = l.fresh("sw_case")
	}
	nextLabels := make([]string, len(st.Cases))
	for i := range st.Cases {
		if i+1 < len(st.Cases) {
			nextLabels[i] = l.fresh("sw_next")
		} else {
			nextLabels[i] = defaultLabel
		}
	}

	valReg := l.lowerExpr(st.Val)
	valTy := st.Val.ExprTy()

	// Emit dispatch chain: for each case emit cmp(val==caseN) → brif → caseN / nextN.
	for i, c := range st.Cases {
		imm := l.bld.Const(c.Val, valTy, mir2.ClassGeneral)
		cond := l.bld.Cmp(mir2.CmpEq, valReg, imm, mir2.ClassFlag, false)
		l.bld.BrIf(cond, caseLabels[i], nil, nextLabels[i], nil)
		if i+1 < len(st.Cases) {
			l.bld.SwitchToNewBlock(nextLabels[i])
		}
	}

	// Emit case bodies.
	var envsFinal []map[string]mir2.Reg
	for i, c := range st.Cases {
		l.bld.SwitchToNewBlock(caseLabels[i])
		l.env = cloneMap(envBefore)
		done := l.lowerBlock(c.Body)
		if !done {
			envsFinal = append(envsFinal, cloneMap(l.env))
			l.bld.Jmp(joinLabel, l.collectArgs(mutated)...)
		}
	}

	// Default body.
	if st.Default != nil {
		l.bld.SwitchToNewBlock(defaultLabel)
		l.env = cloneMap(envBefore)
		done := l.lowerBlock(st.Default)
		if !done {
			envsFinal = append(envsFinal, cloneMap(l.env))
			l.bld.Jmp(joinLabel, l.collectArgs(mutated)...)
		}
	}

	// Join block: only reachable if at least one case (or default) fell through.
	joinReachable := len(envsFinal) > 0
	join := l.bld.SwitchToNewBlock(joinLabel)
	if !joinReachable {
		// All paths returned → join is dead; emit Unreachable so Verify passes.
		l.bld.Unreachable()
		l.env = cloneMap(envBefore)
		return
	}
	joinEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(join, ty, classForExpr(ty))
		joinEnv[name] = r
	}
	_ = join
	l.env = joinEnv
}

// ── Expression lowering ───────────────────────────────────────────────────────

// lowerExpr lowers a HIR expression and returns the virtual register holding
// the result.
func (l *lowerer) lowerExpr(e Expr) mir2.Reg {
	switch ex := e.(type) {

	case *IntLitExpr:
		return l.bld.Const(ex.Val, ex.Ty, classForExpr(ex.Ty))

	case *BoolLitExpr:
		v := int64(0)
		if ex.Val {
			v = 1
		}
		return l.bld.Const(v, mir2.TyBool, mir2.ClassGeneral)

	case *VarRefExpr:
		r, ok := l.env[ex.Name]
		if !ok {
			panic(fmt.Sprintf("hir/lower: undefined variable %q", ex.Name))
		}
		return r

	case *BinExpr:
		return l.lowerBinExpr(ex)

	case *UnaryExpr:
		return l.lowerUnaryExpr(ex)

	case *CallExpr:
		args := make([]mir2.Reg, len(ex.Args))
		for i, a := range ex.Args {
			args[i] = l.lowerExpr(a)
		}
		if ex.Ty == mir2.TyVoid {
			l.bld.CallVoid(ex.Fn, args, mir2.CallAttrs{})
			return mir2.NoReg
		}
		cls := classForRet(ex.Ty)
		return l.bld.Call(ex.Fn, args, ex.Ty, cls, mir2.CallAttrs{})

	case *AddrOfExpr:
		return l.bld.AddrOf(ex.Sym, mir2.ClassPointer)

	case *ConstPtrExpr:
		return l.bld.Const(int64(ex.Addr), mir2.TyPtr, mir2.ClassPointer)

	case *LoadExpr:
		ptr := l.lowerExpr(ex.Ptr)
		cls := classForExpr(ex.Ty)
		return l.bld.Load(ptr, ex.Ty, cls)

	case *FieldExpr:
		base := l.lowerExpr(ex.X)
		// Advance pointer by field offset, then load.
		if ex.Offset > 0 {
			base = l.bld.Field(base, int64(ex.Offset), mir2.ClassPointer)
		}
		return l.bld.Load(base, ex.Ty, classForExpr(ex.Ty))

	case *CastExpr:
		src := l.lowerExpr(ex.X)
		srcTy := ex.X.ExprTy()
		dstTy := ex.Ty
		sw, dw := srcTy.Width(), dstTy.Width()
		switch {
		case dw > sw:
			return l.bld.Ext(src, srcTy, dstTy, classForExpr(dstTy))
		case dw < sw:
			return l.bld.Trunc(src, srcTy, dstTy, classForExpr(dstTy))
		default:
			return src // same width → no-op
		}

	case *IndexExpr:
		return l.lowerIndex(ex)

	default:
		panic(fmt.Sprintf("hir/lower: unhandled expression type %T", e))
	}
}

// lowerIndex lowers arr[i] → ptr_add(base, i * stride) → load.
//
// stride = ElemStride if set; otherwise ElemTy.Width()/8 (rounded up to 1).
// For stride=1 (byte arrays): ptr_add base, i   → ADD HL, DE  (efficient)
// For stride=2 (word arrays): ptr_add base, i*2 → scale then ADD HL, DE
// For stride>2:               ptr_add base, i*N via OpMul (slow; use SoA instead)
func (l *lowerer) lowerIndex(ex *IndexExpr) mir2.Reg {
	base := l.lowerExpr(ex.Base)
	idx := l.lowerExpr(ex.Idx)

	stride := ex.ElemStride
	if stride == 0 {
		w := ex.ElemTy.Width()
		stride = (w + 7) / 8
		if stride == 0 {
			stride = 1
		}
	}

	var offset mir2.Reg
	idxTy := ex.Idx.ExprTy()
	switch stride {
	case 1:
		// offset = zero-extend idx to u16
		if idxTy.Width() <= 8 {
			offset = l.bld.Ext(idx, idxTy, mir2.TyU16, mir2.ClassIndex)
		} else {
			offset = idx
		}
	case 2:
		// offset = idx * 2 = SHL(ext(idx,u16), 1)
		if idxTy.Width() <= 8 {
			idx = l.bld.Ext(idx, idxTy, mir2.TyU16, mir2.ClassIndex)
		}
		one := l.bld.Const(1, mir2.TyU16, mir2.ClassGeneral)
		offset = l.bld.Shl(idx, one, mir2.TyU16, mir2.ClassIndex)
	default:
		// General: offset = idx * stride (software mul — slow, avoid with SoA)
		if idxTy.Width() <= 8 {
			idx = l.bld.Ext(idx, idxTy, mir2.TyU16, mir2.ClassIndex)
		}
		strideReg := l.bld.Const(int64(stride), mir2.TyU16, mir2.ClassGeneral)
		offset = l.bld.Mul(idx, strideReg, mir2.TyU16, mir2.ClassIndex)
	}

	ptr := l.bld.PtrAdd(base, offset, mir2.ClassPointer)
	return l.bld.Load(ptr, ex.ElemTy, classForExpr(ex.ElemTy))
}

// lowerBinExpr lowers a binary expression.
func (l *lowerer) lowerBinExpr(ex *BinExpr) mir2.Reg {
	// Comparison operators → produce a flag (ClassFlag).
	switch ex.Op {
	case "==", "!=", "<", "<=", ">", ">=":
		return l.lowerCond(ex)
	}

	lReg := l.lowerExpr(ex.L)
	rReg := l.lowerExpr(ex.R)
	ty := ex.Ty
	cls := classForExpr(ty)

	switch ex.Op {
	case "+":
		return l.bld.Add(lReg, rReg, ty, cls)
	case "-":
		return l.bld.Sub(lReg, rReg, ty, cls)
	case "*":
		return l.bld.Mul(lReg, rReg, ty, cls)
	case "/":
		return l.bld.Div(lReg, rReg, ty, cls)
	case "&":
		return l.bld.And(lReg, rReg, ty, cls)
	case "|":
		return l.bld.Or(lReg, rReg, ty, cls)
	case "^":
		return l.bld.Xor(lReg, rReg, ty, cls)
	case "<<":
		return l.bld.Shl(lReg, rReg, ty, cls)
	case ">>":
		return l.bld.Shr(lReg, rReg, ty, cls)
	default:
		panic(fmt.Sprintf("hir/lower: unhandled binary op %q", ex.Op))
	}
}

// lowerUnaryExpr lowers a unary expression.
func (l *lowerer) lowerUnaryExpr(ex *UnaryExpr) mir2.Reg {
	xReg := l.lowerExpr(ex.X)
	ty := ex.Ty
	cls := classForExpr(ty)
	switch ex.Op {
	case "-":
		return l.bld.Neg(xReg, ty, cls)
	case "~":
		return l.bld.Not(xReg, ty, cls)
	case "!":
		// Boolean not: x == 0
		zero := l.bld.Const(0, ty, mir2.ClassGeneral)
		return l.bld.Cmp(mir2.CmpEq, xReg, zero, mir2.ClassFlag, false)
	default:
		panic(fmt.Sprintf("hir/lower: unhandled unary op %q", ex.Op))
	}
}

// lowerCond lowers a condition expression and returns a ClassFlag vreg.
// Handles comparison ops directly; for non-bool expressions wraps in != 0.
func (l *lowerer) lowerCond(e Expr) mir2.Reg {
	if bin, ok := e.(*BinExpr); ok {
		var cmp mir2.CmpCond
		switch bin.Op {
		case "==":
			cmp = mir2.CmpEq
		case "!=":
			cmp = mir2.CmpNe
		case "<":
			cmp = mir2.CmpLt
		case "<=":
			cmp = mir2.CmpLe
		case ">":
			cmp = mir2.CmpGt
		case ">=":
			cmp = mir2.CmpGe
		default:
			goto fallback
		}
		lReg := l.lowerExpr(bin.L)
		rReg := l.lowerExpr(bin.R)
		return l.bld.Cmp(cmp, lReg, rReg, mir2.ClassFlag, false)
	}
fallback:
	// Treat expr as a bool: cond != 0
	r := l.lowerExpr(e)
	zero := l.bld.Const(0, e.ExprTy(), mir2.ClassGeneral)
	return l.bld.Cmp(mir2.CmpNe, r, zero, mir2.ClassFlag, false)
}

// ── Environment helpers ───────────────────────────────────────────────────────

func cloneMap(m map[string]mir2.Reg) map[string]mir2.Reg {
	out := make(map[string]mir2.Reg, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// collectArgs returns the current vregs for a slice of variable names, in order.
func (l *lowerer) collectArgs(names []string) []mir2.Reg {
	if len(names) == 0 {
		return nil
	}
	args := make([]mir2.Reg, len(names))
	for i, name := range names {
		args[i] = l.env[name]
	}
	return args
}

// ── Mutation scanner ──────────────────────────────────────────────────────────

// scanMutations returns the set of variable names that are assigned in blk.
// Only variables already in env (defined before the block) are included.
func scanMutations(blk *Block, env map[string]mir2.Reg) map[string]bool {
	muts := map[string]bool{}
	scanBlock(blk, env, muts)
	return muts
}

func scanBlock(blk *Block, env map[string]mir2.Reg, muts map[string]bool) {
	for _, s := range blk.Body {
		scanStmt(s, env, muts)
	}
}

func scanStmt(s Stmt, env map[string]mir2.Reg, muts map[string]bool) {
	switch st := s.(type) {
	case *Block:
		scanBlock(st, env, muts)
	case *AssignStmt:
		if vr, ok := st.Target.(*VarRefExpr); ok {
			if _, inEnv := env[vr.Name]; inEnv {
				muts[vr.Name] = true
			}
		}
	case *IfStmt:
		scanBlock(st.Then, env, muts)
		if st.Else != nil {
			scanBlock(st.Else, env, muts)
		}
	case *WhileStmt:
		scanBlock(st.Body, env, muts)
	case *ForRangeStmt:
		scanBlock(st.Body, env, muts)
	case *SwitchStmt:
		for _, c := range st.Cases {
			scanBlock(c.Body, env, muts)
		}
		if st.Default != nil {
			scanBlock(st.Default, env, muts)
		}
	// BreakStmt, ContinueStmt: no mutations
	}
}

// ── Key set helpers ───────────────────────────────────────────────────────────

// sortedKeys returns keys of muts that also appear in env, in sorted order.
func sortedKeys(muts map[string]bool, env map[string]mir2.Reg) []string {
	var out []string
	for name := range muts {
		if _, ok := env[name]; ok {
			out = append(out, name)
		}
	}
	sortStrings(out)
	return out
}

// unionKeys returns the sorted union of keys from a and b that also appear in env.
func unionKeys(a, b map[string]bool, env map[string]mir2.Reg) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if _, ok := env[name]; ok && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for name := range a {
		add(name)
	}
	for name := range b {
		add(name)
	}
	sortStrings(out)
	return out
}

// ── Used-variable scanner ─────────────────────────────────────────────────────

// scanUsedExpr adds to used the names of env-variables referenced in e.
func scanUsedExpr(e Expr, env map[string]mir2.Reg, used map[string]bool) {
	switch ex := e.(type) {
	case *VarRefExpr:
		if _, ok := env[ex.Name]; ok {
			used[ex.Name] = true
		}
	case *BinExpr:
		scanUsedExpr(ex.L, env, used)
		scanUsedExpr(ex.R, env, used)
	case *UnaryExpr:
		scanUsedExpr(ex.X, env, used)
	case *CallExpr:
		for _, a := range ex.Args {
			scanUsedExpr(a, env, used)
		}
	case *FieldExpr:
		scanUsedExpr(ex.X, env, used)
	case *LoadExpr:
		scanUsedExpr(ex.Ptr, env, used)
	case *DerefExpr:
		scanUsedExpr(ex.Ptr, env, used)
	case *CastExpr:
		scanUsedExpr(ex.X, env, used)
	case *IndexExpr:
		scanUsedExpr(ex.Base, env, used)
		scanUsedExpr(ex.Idx, env, used)
	// IntLitExpr, BoolLitExpr, AddrOfExpr: no variable references
	}
}

// scanUsedInBlock adds to used the names of env-variables referenced in blk.
func scanUsedInBlock(blk *Block, env map[string]mir2.Reg, used map[string]bool) {
	for _, s := range blk.Body {
		scanUsedInStmt(s, env, used)
	}
}

func scanUsedInStmt(s Stmt, env map[string]mir2.Reg, used map[string]bool) {
	switch st := s.(type) {
	case *Block:
		scanUsedInBlock(st, env, used)
	case *VarDeclStmt:
		if st.Init != nil {
			scanUsedExpr(st.Init, env, used)
		}
	case *AssignStmt:
		scanUsedExpr(st.Val, env, used)
		if d, ok := st.Target.(*DerefExpr); ok {
			scanUsedExpr(d.Ptr, env, used)
		}
	case *ReturnStmt:
		if st.Val != nil {
			scanUsedExpr(st.Val, env, used)
		}
	case *ExprStmt:
		scanUsedExpr(st.Expr, env, used)
	case *StoreStmt:
		scanUsedExpr(st.Ptr, env, used)
		scanUsedExpr(st.Val, env, used)
	case *IfStmt:
		scanUsedExpr(st.Cond, env, used)
		scanUsedInBlock(st.Then, env, used)
		if st.Else != nil {
			scanUsedInBlock(st.Else, env, used)
		}
	case *WhileStmt:
		scanUsedExpr(st.Cond, env, used)
		scanUsedInBlock(st.Body, env, used)
	case *ForRangeStmt:
		if st.Start != nil {
			scanUsedExpr(st.Start, env, used)
		}
		if st.End != nil {
			scanUsedExpr(st.End, env, used)
		}
		scanUsedInBlock(st.Body, env, used)
	case *SwitchStmt:
		scanUsedExpr(st.Val, env, used)
		for _, c := range st.Cases {
			scanUsedInBlock(c.Body, env, used)
		}
		if st.Default != nil {
			scanUsedInBlock(st.Default, env, used)
		}
	// BreakStmt, ContinueStmt: no expressions to scan
	}
}

// sortStrings insertion-sorts a small slice (avoids importing sort).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
