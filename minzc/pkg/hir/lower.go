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

	// Collect all function names and HIR func map for callback inlining.
	funcNames := make(map[string]bool, len(hm.Funcs))
	hirFuncs := make(map[string]*Func, len(hm.Funcs))
	for _, f := range hm.Funcs {
		funcNames[f.Name] = true
		hirFuncs[f.Name] = f
	}

	// Collect global names for free-variable detection.
	globalNames := make(map[string]bool, len(hm.Globals))
	for _, g := range hm.Globals {
		globalNames[g.Name] = true
	}

	for _, f := range hm.Funcs {
		// Skip standalone lowering of lambdas that capture outer locals.
		// Such functions are always inlined via lowerFusedForEach; lowering
		// them directly would panic on the free-variable references.
		if hasFreeVars(f, globalNames, funcNames) {
			continue
		}
		lowerFuncWithFuncNames(m, f, funcNames, hirFuncs)
	}
	return m
}

// hasFreeVars reports whether f references any variable that is not one of its
// own parameters, not a global, and not a module-level function name.
// Such functions are capture-lambdas that must be inlined rather than lowered
// as standalone MIR2 functions.
func hasFreeVars(f *Func, globalNames, funcNames map[string]bool) bool {
	if f.Body == nil {
		return false
	}
	bound := make(map[string]bool, len(f.Params))
	for _, p := range f.Params {
		bound[p.Name] = true
	}
	return blockHasFreeRef(f.Body, bound, globalNames, funcNames)
}

func blockHasFreeRef(blk *Block, bound, globals, funcs map[string]bool) bool {
	for _, s := range blk.Body {
		if stmtHasFreeRef(s, bound, globals, funcs) {
			return true
		}
	}
	return false
}

func stmtHasFreeRef(s Stmt, bound, globals, funcs map[string]bool) bool {
	switch st := s.(type) {
	case *Block:
		return blockHasFreeRef(st, bound, globals, funcs)
	case *VarDeclStmt:
		if st.Init != nil && exprHasFreeRef(st.Init, bound, globals, funcs) {
			return true
		}
		bound[st.Name] = true // newly bound
	case *AssignStmt:
		if exprHasFreeRef(st.Val, bound, globals, funcs) {
			return true
		}
		if d, ok := st.Target.(*DerefExpr); ok {
			return exprHasFreeRef(d.Ptr, bound, globals, funcs)
		}
		// A plain variable assignment (c = a - b) binds the target name.
		if vr, ok := st.Target.(*VarRefExpr); ok {
			bound[vr.Name] = true
		}
	case *ReturnStmt:
		if st.Val != nil {
			return exprHasFreeRef(st.Val, bound, globals, funcs)
		}
		for _, v := range st.Vals {
			if exprHasFreeRef(v, bound, globals, funcs) {
				return true
			}
		}
	case *TupleLetStmt:
		for _, a := range st.Call.Args {
			if exprHasFreeRef(a, bound, globals, funcs) {
				return true
			}
		}
		for _, name := range st.Names {
			if name != "_" {
				bound[name] = true
			}
		}
	case *ExprStmt:
		return exprHasFreeRef(st.Expr, bound, globals, funcs)
	case *StoreStmt:
		return exprHasFreeRef(st.Ptr, bound, globals, funcs) ||
			exprHasFreeRef(st.Val, bound, globals, funcs)
	case *IfStmt:
		return exprHasFreeRef(st.Cond, bound, globals, funcs) ||
			blockHasFreeRef(st.Then, bound, globals, funcs) ||
			(st.Else != nil && blockHasFreeRef(st.Else, bound, globals, funcs))
	case *WhileStmt:
		return exprHasFreeRef(st.Cond, bound, globals, funcs) ||
			blockHasFreeRef(st.Body, bound, globals, funcs)
	case *ForEachStmt:
		if exprHasFreeRef(st.Ptr, bound, globals, funcs) {
			return true
		}
		if st.Len != nil && exprHasFreeRef(st.Len, bound, globals, funcs) {
			return true
		}
		bound[st.Var] = true // element variable is bound inside the body
		return blockHasFreeRef(st.Body, bound, globals, funcs)
	case *ForRangeStmt:
		if st.Start != nil && exprHasFreeRef(st.Start, bound, globals, funcs) {
			return true
		}
		if st.End != nil && exprHasFreeRef(st.End, bound, globals, funcs) {
			return true
		}
		bound[st.Var] = true // loop variable is bound inside the body
		return blockHasFreeRef(st.Body, bound, globals, funcs)
	case *SwitchStmt:
		if exprHasFreeRef(st.Val, bound, globals, funcs) {
			return true
		}
		for _, c := range st.Cases {
			if blockHasFreeRef(c.Body, bound, globals, funcs) {
				return true
			}
		}
		if st.Default != nil {
			return blockHasFreeRef(st.Default, bound, globals, funcs)
		}
	}
	return false
}

func exprHasFreeRef(e Expr, bound, globals, funcs map[string]bool) bool {
	if e == nil {
		return false
	}
	switch ex := e.(type) {
	case *VarRefExpr:
		return !bound[ex.Name] && !globals[ex.Name] && !funcs[ex.Name]
	case *BinExpr:
		return exprHasFreeRef(ex.L, bound, globals, funcs) ||
			exprHasFreeRef(ex.R, bound, globals, funcs)
	case *UnaryExpr:
		return exprHasFreeRef(ex.X, bound, globals, funcs)
	case *CallExpr:
		for _, a := range ex.Args {
			if exprHasFreeRef(a, bound, globals, funcs) {
				return true
			}
		}
	case *FieldExpr:
		return exprHasFreeRef(ex.X, bound, globals, funcs)
	case *LoadExpr:
		return exprHasFreeRef(ex.Ptr, bound, globals, funcs)
	case *DerefExpr:
		return exprHasFreeRef(ex.Ptr, bound, globals, funcs)
	case *CastExpr:
		return exprHasFreeRef(ex.X, bound, globals, funcs)
	case *IndexExpr:
		return exprHasFreeRef(ex.Base, bound, globals, funcs) ||
			exprHasFreeRef(ex.Idx, bound, globals, funcs)
	}
	return false
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

	// hirFuncNames is the set of all function names in the HIR module.
	// Used to distinguish function references from undefined variables.
	hirFuncNames map[string]bool

	// hirFuncs maps function names to their HIR Func for callback inlining
	// in iterator chain fusion (map/filter/forEach).
	hirFuncs map[string]*Func

	// fnAliases maps local variable names to function names.
	// e.g. "let f = |x| x+1" → fnAliases["f"] = "lambda_0"
	// Calls through "f" are resolved to "lambda_0" at call sites.
	fnAliases map[string]string

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

// findGlobal looks up a global by name in the MIR2 module.
func (l *lowerer) findGlobal(name string) (mir2.Global, bool) {
	for _, g := range l.m.Globals {
		if g.Name == name {
			return g, true
		}
	}
	return mir2.Global{}, false
}

// globalAddr emits AddrOf for a global variable name.
func (l *lowerer) globalAddr(name string) mir2.Reg {
	return l.bld.AddrOf(name, mir2.ClassPointer)
}

// lowerExprAddr lowers an expression to its address (lvalue), rather than its value.
// Used for field-store targets (origin.y = v) and eventually local struct alloca.
func (l *lowerer) lowerExprAddr(ex Expr) mir2.Reg {
	switch e := ex.(type) {
	case *VarRefExpr:
		if _, ok := l.findGlobal(e.Name); ok {
			return l.globalAddr(e.Name)
		}
		// Local pointer variable (e.g. param `self: ^Acc`): the register value IS the address.
		if reg, ok := l.env[e.Name]; ok {
			return reg
		}
		panic(fmt.Sprintf("hir/lower: cannot take address of local variable %q", e.Name))
	case *FieldExpr:
		baseAddr := l.lowerExprAddr(e.X)
		if e.Offset > 0 {
			return l.bld.Field(baseAddr, int64(e.Offset), mir2.ClassPointer)
		}
		return baseAddr
	case *IndexExpr:
		// base[i] address — same as lowerIndex but without the final Load
		baseAddr := l.lowerExpr(e.Base)
		idx := l.lowerExpr(e.Idx)
		stride := int64(e.ElemStride)
		if stride == 0 {
			stride = int64(e.ElemTy.Width() / 8)
		}
		if stride > 1 {
			width := e.ElemTy.Width()
			sizeTy := mir2.Ty(mir2.TyU16)
			if width <= 8 {
				sizeTy = mir2.TyU8
			}
			idx = l.bld.Mul(idx, l.bld.Const(stride, sizeTy, mir2.ClassAcc), sizeTy, mir2.ClassAcc)
		}
		return l.bld.PtrAdd(baseAddr, idx, mir2.ClassPointer)
	default:
		panic(fmt.Sprintf("hir/lower: cannot take address of %T", ex))
	}
}

// ── Function lowering ─────────────────────────────────────────────────────────

func lowerFunc(m *mir2.Module, f *Func) {
	lowerFuncWithFuncNames(m, f, nil, nil)
}

func lowerFuncWithFuncNames(m *mir2.Module, f *Func, funcNames map[string]bool, hirFuncs map[string]*Func) {
	mf := m.AddFunc(f.Name)
	if f.IsExtern {
		mf.Attrs.IsExtern = true
		mf.Attrs.ExternAddr = f.ExternAddr
		return
	}

	// Set return type(s).
	if len(f.RetTys) > 0 {
		// Multi-return: use RetTys slice.
		mf.Contract.Returns = make([]mir2.Return, len(f.RetTys))
		for i, ty := range f.RetTys {
			mf.Contract.Returns[i] = mir2.Return{Ty: ty, Class: classForRetPos(ty, i)}
		}
	} else if f.RetTy != mir2.TyVoid {
		cls := classForRet(f.RetTy)
		mf.Contract.Returns = []mir2.Return{{Ty: f.RetTy, Class: cls}}
	}

	bld := mir2.NewBuilder(mf)
	l := &lowerer{
		hf:           f,
		m:            m,
		mf:           mf,
		bld:          bld,
		env:          make(map[string]mir2.Reg),
		envTy:        make(map[string]mir2.Ty),
		hirFuncNames: funcNames,
		hirFuncs:     hirFuncs,
		fnAliases:    make(map[string]string),
	}

	bld.SwitchToNewBlock("entry")

	// Bind parameters to virtual registers.
	for i, p := range f.Params {
		cls := classForParam(p.Ty, i)
		if p.RegClass != 0 {
			cls = p.RegClass
		}
		r := bld.Param(p.Name, p.Ty, cls)
		l.bind(p.Name, r, p.Ty)
	}

	// Lower body.
	l.lowerBlock(f.Body)

	// Implicit return for void functions that fall off the end of the current block.
	if !bld.Cur.IsSealed() {
		bld.Ret()
	}
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
	if _, isStruct := ty.(*mir2.StructTy); isStruct {
		// Struct params are passed as pointers (address in HL on Z80).
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

// classForRet chooses the return class for a single-return function.
func classForRet(ty mir2.Ty) mir2.RegClass {
	if ty == mir2.TyU16 || ty == mir2.TyI16 {
		return mir2.ClassPointer // HL
	}
	return mir2.ClassAcc // A
}

// classForRetPos chooses the return class for position pos in a multi-return function.
//
//	pos 0 → same as classForRet (A for u8, HL for u16)
//	pos 1 → ClassIndex (DE)
//	pos 2 → ClassCounter (B for u8) or ClassIndex (DE-hi… not ideal; use B)
func classForRetPos(ty mir2.Ty, pos int) mir2.RegClass {
	switch pos {
	case 0:
		return classForRet(ty)
	case 1:
		return mir2.ClassIndex // DE
	default:
		return mir2.ClassCounter // B
	}
}

// lowerExprForRet lowers an expression that is the direct value of a ReturnStmt.
// For a LoadExpr or FieldExpr at the top level, it uses classForRet (ClassAcc)
// instead of classForExpr (ClassGeneral), so the load goes directly into A and
// avoids the redundant LD C,(HL); LD A,C sequence.
func (l *lowerer) lowerExprForRet(e Expr) mir2.Reg {
	switch ex := e.(type) {
	case *LoadExpr:
		if ex.Ty.Width() <= 8 {
			ptr := l.lowerExpr(ex.Ptr)
			return l.bld.Load(ptr, ex.Ty, classForRet(ex.Ty))
		}
	case *FieldExpr:
		if ex.Ty.Width() <= 8 {
			base := l.lowerExpr(ex.X)
			if ex.Offset > 0 {
				base = l.bld.Field(base, int64(ex.Offset), mir2.ClassPointer)
			}
			return l.bld.Load(base, ex.Ty, classForRet(ex.Ty))
		}
	}
	return l.lowerExpr(e)
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
			// Check for fold/reduce: let result = fold(ptr, init, cb, n)
			if foldReg, ok := l.tryLowerFold(st.Init); ok {
				chain, _ := l.recognizeIterChain(st.Init)
				l.bind(st.Name, foldReg, chain.foldAccTy)
				return false
			}
			r = l.lowerExpr(st.Init)
			if r == mir2.NoReg {
				// Init is a function reference (lambda or module function).
				// Record as alias so calls through this variable are resolved.
				if vr, ok := st.Init.(*VarRefExpr); ok {
					funcName := vr.Name
					if alias, isAlias := l.fnAliases[vr.Name]; isAlias {
						funcName = alias
					}
					l.fnAliases[st.Name] = funcName
				}
				return false
			}
		} else {
			r = l.bld.Const(0, st.Ty, classForExpr(st.Ty))
		}
		l.bind(st.Name, r, st.Ty)
		return false

	case *AssignStmt:
		// Check for fold/reduce: x = fold(ptr, init, cb, n)
		if foldReg, ok := l.tryLowerFold(st.Val); ok {
			if tgt, ok2 := st.Target.(*VarRefExpr); ok2 {
				l.env[tgt.Name] = foldReg
				return false
			}
		}
		val := l.lowerExpr(st.Val)
		switch tgt := st.Target.(type) {
		case *VarRefExpr:
			// Global variable: store through pointer.
			if g, found := l.findGlobal(tgt.Name); found {
				addr := l.globalAddr(tgt.Name)
				l.bld.Store(addr, val, g.Ty)
				return false
			}
			// Re-bind the local variable to the new vreg (SSA-style rename).
			// Type is already in envTy; just update the register.
			l.env[tgt.Name] = val
		case *DerefExpr:
			ptr := l.lowerExpr(tgt.Ptr)
			l.bld.Store(ptr, val, tgt.Ty)
		case *FieldExpr:
			// Store to struct field: compute field address then store.
			addr := l.lowerExprAddr(tgt)
			l.bld.Store(addr, val, tgt.Ty)
		case *IndexExpr:
			// arr[i] = val: compute element address then store.
			addr := l.lowerExprAddr(tgt)
			l.bld.Store(addr, val, tgt.ElemTy)
		default:
			panic(fmt.Sprintf("hir/lower: unsupported AssignStmt target %T", st.Target))
		}
		return false

	case *ReturnStmt:
		if len(st.Vals) > 0 {
			// Multi-return.
			regs := make([]mir2.Reg, len(st.Vals))
			for i, v := range st.Vals {
				regs[i] = l.lowerExpr(v)
			}
			l.bld.Ret(regs...)
		} else if st.Val != nil {
			// Check for fold/reduce: return fold(ptr, init, cb, n)
			if foldReg, ok := l.tryLowerFold(st.Val); ok {
				l.bld.Ret(foldReg)
			} else {
				r := l.lowerExprForRet(st.Val)
				l.bld.Ret(r)
			}
		} else {
			l.bld.Ret()
		}
		return true

	case *TupleLetStmt:
		// Build Returns slice for CallMulti.
		returns := make([]mir2.Return, len(st.Tys))
		for i, ty := range st.Tys {
			returns[i] = mir2.Return{Ty: ty, Class: classForRetPos(ty, i)}
		}
		args := make([]mir2.Reg, len(st.Call.Args))
		for i, a := range st.Call.Args {
			args[i] = l.lowerExpr(a)
		}
		regs := l.bld.CallMulti(st.Call.Fn, args, returns, mir2.CallAttrs{})
		for i, name := range st.Names {
			if name == "_" || i >= len(regs) {
				continue
			}
			if regs[i] != mir2.NoReg {
				l.bind(name, regs[i], st.Tys[i])
			}
		}
		return false

	case *ExprStmt:
		if l.tryLowerIterChain(st.Expr) {
			return false
		}
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

	case *ForEachStmt:
		l.lowerForEach(st)
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

	// For no-else ifs, pre-collect envBefore values for the false edge args.
	// We must do this BEFORE emitting any new instructions so the regs are
	// the current values (not post-then).
	var falseEdgeArgs []mir2.Reg
	if !hasElse {
		falseEdgeArgs = l.collectArgs(mutated)
	}

	// Emit the branch.
	condReg := l.lowerCond(st.Cond)
	if hasElse {
		l.bld.BrIf(condReg, thenLabel, nil, elseLabel, nil)
	} else {
		// No else: false edge goes to join with the current (pre-then) values
		// of mutated vars.  The true (then) edge will jump to join with the
		// post-mutation values.  The join block will have block params for each
		// mutated var that receives from both paths.
		l.bld.BrIf(condReg, thenLabel, nil, joinLabel, falseEdgeArgs)
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
		// No else: both paths now supply args (true=then-branch values,
		// false=envBefore values via falseEdgeArgs).  Create block params so
		// the join receives the correct value from whichever path executed.
		joinEnv = cloneMap(envBefore)
		for _, name := range mutated {
			ty := l.envTy[name]
			r := l.bld.BlockParam(join, ty, classForExpr(ty))
			joinEnv[name] = r
		}
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

// lowerForEach lowers a ForEachStmt.
//
// MIR2 structure (ptr → HL, counter → B for DJNZ):
//
//	entry:
//	    ptr0 = lower(Ptr) + Start * stride
//	    cnt0 = lower(Len)
//	    jmp @fe_head(ptr0, cnt0, ...mutated_init...)
//	fe_head(ptr, cnt, ...mutated...):
//	    cond = cnt != 0
//	    brif cond @fe_body @fe_exit(ptr, cnt, ...mutated...)
//	fe_body:
//	    x = Load(ptr, ElemTy)
//	    [body]
//	    ptr_next = PtrBump(ptr, stride)
//	    cnt_next = Sub(cnt, 1)
//	    jmp @fe_head(ptr_next, cnt_next, ...updated_mutated...)
//	fe_exit(ptr, cnt, ...mutated...):   -- ptr/cnt discarded
//	    [continue]
func (l *lowerer) lowerForEach(st *ForEachStmt) {
	envBefore := cloneMap(l.env)
	stride := int64(mir2.ByteWidth(st.ElemTy))
	if stride < 1 {
		stride = 1
	}

	// Compute initial pointer (with optional start offset).
	ptrReg := l.lowerExpr(st.Ptr)
	if st.Start != nil {
		startReg := l.lowerExpr(st.Start)
		if stride > 1 {
			factor := l.bld.Const(stride, mir2.TyU16, mir2.ClassPointer)
			startReg = l.bld.Mul(startReg, factor, mir2.TyU16, mir2.ClassIndex)
		}
		ptrReg = l.bld.PtrAdd(ptrReg, startReg, mir2.ClassPointer)
	}
	// Compute element count.
	cntReg := l.lowerExpr(st.Len)

	// Collect user env vars used/mutated in the body (for block params).
	allUsed := map[string]bool{}
	scanUsedInBlock(st.Body, envBefore, allUsed)
	muts := scanMutations(st.Body, envBefore)
	for name := range muts {
		allUsed[name] = true
	}
	// Don't thread the element variable itself (it's fresh each iteration).
	delete(allUsed, st.Var)
	mutated := sortedKeys(allUsed, envBefore)

	headLabel := l.fresh("fe_head")
	bodyLabel := l.fresh("fe_body")
	contLabel := l.fresh("fe_cont") // continue trampoline: advance ptr/cnt → head
	exitLabel := l.fresh("fe_exit")

	// Jump to loop head: (ptr, cnt, mutated...)
	initArgs := append([]mir2.Reg{ptrReg, cntReg}, l.collectArgs(mutated)...)
	l.bld.Jmp(headLabel, initArgs...)

	// ── fe_head(ptr, cnt, mutated...) ──
	head := l.bld.SwitchToNewBlock(headLabel)
	headPtr := l.bld.BlockParam(head, mir2.TyPtr, mir2.ClassPointer) // HL
	headCnt := l.bld.BlockParam(head, mir2.TyU8, mir2.ClassCounter)  // B → DJNZ
	headEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(head, ty, classForLoop(ty, name))
		headEnv[name] = r
	}
	l.env = headEnv

	// brif (cnt != 0) → body, else → exit
	zero8 := l.bld.Const(0, mir2.TyU8, mir2.ClassAcc)
	cond := l.bld.Cmp(mir2.CmpNe, headCnt, zero8, mir2.ClassFlag, false)
	exitArgs := append([]mir2.Reg{headPtr, headCnt}, l.collectArgs(mutated)...)
	l.bld.BrIf(cond, bodyLabel, nil, exitLabel, exitArgs)

	// ── fe_body ──
	l.bld.SwitchToNewBlock(bodyLabel)
	l.env = cloneMap(headEnv)

	// Load element: x = *ptr
	xReg := l.bld.Load(headPtr, st.ElemTy, classForExpr(st.ElemTy))
	l.bind(st.Var, xReg, st.ElemTy)

	// ContinueStmt jumps to fe_cont (not fe_head) so ptr/cnt are advanced first.
	l.pushLoop(loopCtx{headLabel: contLabel, exitLabel: exitLabel, mutated: mutated})
	l.lowerBlock(st.Body)
	l.popLoop()

	// mapInPlace: store the (possibly transformed) element value back to ptr.
	if st.MutateInPlace {
		if finalVal, ok := l.env[st.Var]; ok {
			l.bld.Store(headPtr, finalVal, st.ElemTy)
		}
	}

	// Fall-through from body → fe_cont with current mutated values.
	l.bld.Jmp(contLabel, l.collectArgs(mutated)...)

	// ── fe_cont(mutated...) — advance ptr/cnt, back-edge to fe_head ──
	// This is the target for ContinueStmt inside the body, as well as the
	// normal fall-through path. It advances ptr/cnt (using headPtr/headCnt
	// which dominate this block) and loops back.
	cont := l.bld.SwitchToNewBlock(contLabel)
	contEnv := cloneMap(headEnv)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(cont, ty, classForExpr(ty))
		contEnv[name] = r
	}
	l.env = contEnv

	ptrNext := l.bld.PtrBump(headPtr, stride, mir2.ClassPointer)
	one := l.bld.Const(1, mir2.TyU8, mir2.ClassCounter)
	cntNext := l.bld.Sub(headCnt, one, mir2.TyU8, mir2.ClassCounter)

	backArgs := append([]mir2.Reg{ptrNext, cntNext}, l.collectArgs(mutated)...)
	l.bld.Jmp(headLabel, backArgs...)

	// ── fe_exit(ptr, cnt, mutated...) ──
	exit := l.bld.SwitchToNewBlock(exitLabel)
	l.bld.BlockParam(exit, mir2.TyPtr, mir2.ClassPointer) // discard ptr
	l.bld.BlockParam(exit, mir2.TyU8, mir2.ClassCounter)  // discard cnt
	exitEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(exit, ty, classForExpr(ty))
		exitEnv[name] = r
	}
	// Element var goes out of scope.
	delete(exitEnv, st.Var)
	l.env = exitEnv
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
// lowerExprAs lowers e, but if e is an integer literal with a narrower type
// than wantTy, emits the constant with wantTy instead.  This handles patterns
// like "0 - r" where r:u16 — the parser types the literal 0 as u8 but the
// arithmetic must be 16-bit.
func (l *lowerer) lowerExprAs(e Expr, wantTy mir2.Ty) mir2.Reg {
	if lit, ok := e.(*IntLitExpr); ok && lit.Ty != wantTy && lit.Ty.Width() < wantTy.Width() {
		return l.bld.Const(lit.Val, wantTy, classForExpr(wantTy))
	}
	return l.lowerExpr(e)
}

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
			// Check if it's a function alias (let f = |x| ...).
			if _, isAlias := l.fnAliases[ex.Name]; isAlias {
				return mir2.NoReg // function reference — resolved at call site
			}
			// Check if it's a module-level function name used as a value.
			if l.hirFuncNames[ex.Name] {
				return mir2.NoReg // function reference — resolved at call site
			}
			// Check if it's a global variable.
			if g, found := l.findGlobal(ex.Name); found {
				addr := l.globalAddr(ex.Name)
				if _, isStruct := g.Ty.(*mir2.StructTy); isStruct {
					// Struct global: return the address (pointer to struct).
					// FieldExpr uses this address as base for offset computation.
					return addr
				}
				// Scalar global: load and return the value.
				return l.bld.Load(addr, g.Ty, classForExpr(g.Ty))
			}
			panic(fmt.Sprintf("hir/lower: undefined variable %q", ex.Name))
		}
		return r

	case *BinExpr:
		return l.lowerBinExpr(ex)

	case *UnaryExpr:
		return l.lowerUnaryExpr(ex)

	case *CallExpr:
		// Resolve function name: may be a direct name, a local alias (let f = |x| ...), or
		// a module-level function reference.
		fnName := ex.Fn
		if alias, ok := l.fnAliases[fnName]; ok {
			fnName = alias
		}

		args := make([]mir2.Reg, len(ex.Args))
		for i, a := range ex.Args {
			r := l.lowerExpr(a)
			// Coerce arg to its calling-convention register class.
			// E.g. arg[0] must be in ClassAcc (A), but classForExpr returns
			// ClassGeneral for safety.  Emit an explicit Move so the allocator
			// puts the value in the right physical register.
			tgtCls := classForParam(a.ExprTy(), i)
			if tgtCls != classForExpr(a.ExprTy()) {
				r = l.bld.Move(r, a.ExprTy(), tgtCls)
			}
			args[i] = r
		}
		if ex.Ty == mir2.TyVoid {
			l.bld.CallVoid(fnName, args, mir2.CallAttrs{})
			return mir2.NoReg
		}
		cls := classForRet(ex.Ty)
		return l.bld.Call(fnName, args, ex.Ty, cls, mir2.CallAttrs{})

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

	ty := ex.Ty
	cls := classForExpr(ty)
	// Type-widen integer literals to match the expression result type.
	// e.g. "0 - r" where r:u16 parses literal 0 as u8; promote to u16 so
	// the codegen sees a 16-bit const, not an 8-bit one.
	lReg := l.lowerExprAs(ex.L, ty)
	rReg := l.lowerExprAs(ex.R, ty)

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
	case *ForEachStmt:
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
		for _, v := range st.Vals {
			scanUsedExpr(v, env, used)
		}
	case *TupleLetStmt:
		for _, a := range st.Call.Args {
			scanUsedExpr(a, env, used)
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
	case *ForEachStmt:
		scanUsedExpr(st.Ptr, env, used)
		if st.Start != nil {
			scanUsedExpr(st.Start, env, used)
		}
		scanUsedExpr(st.Len, env, used)
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

// ── Iterator chain fusion ─────────────────────────────────────────────────────
//
// Recognizes UFCS-rewritten iterator chains of the form:
//
//	ptr.map(f).filter(g).forEach(h, n)
//
// which the parser rewrites to:
//
//	forEach(filter(map(ptr, f), g), h, n)
//
// and fuses them into a single ForEachStmt with inlined bodies — no CALL overhead
// for the lambda/function callbacks.

type iterStage struct {
	kind string // "map" or "filter"
	fn   string // resolved function name
}

type iterChain struct {
	ptr    Expr        // base pointer expression
	len    Expr        // element count
	stages []iterStage // transformation stages (innermost-first)
	mutate bool        // mapInPlace: store transformed element back to ptr
	cbName string      // forEach callback function name
	elemTy mir2.Ty     // element type (from callback's first param)
	// fold fields (isFold=true when chain is fold/reduce, not forEach):
	isFold    bool    // true → fold terminal (returns accumulator)
	foldInit  Expr    // initial accumulator value
	foldAccTy mir2.Ty // accumulator type (cb's return type)
}

// tryLowerIterChain detects iterator chain patterns and lowers them to a fused
// ForEachStmt (avoiding CALL overhead for lambda callbacks).
// Returns true if the pattern was recognized and lowered.
func (l *lowerer) tryLowerIterChain(expr Expr) bool {
	chain, ok := l.recognizeIterChain(expr)
	if !ok {
		return false
	}
	if chain.isFold {
		l.lowerFusedFold(chain) // result discarded (fold as ExprStmt = side-effect only)
	} else {
		l.lowerFusedForEach(chain)
	}
	return true
}

// tryLowerFold recognizes a fold/reduce call and returns the result register.
// Returns (reg, true) on success; (NoReg, false) if expr is not a fold chain.
func (l *lowerer) tryLowerFold(expr Expr) (mir2.Reg, bool) {
	chain, ok := l.recognizeIterChain(expr)
	if !ok || !chain.isFold {
		return mir2.NoReg, false
	}
	return l.lowerFusedFold(chain), true
}

// recognizeIterChain matches forEach(src,cb,n) and mapInPlace(src,cb,n)
// where src may be nested map/filter stages.
func (l *lowerer) recognizeIterChain(expr Expr) (iterChain, bool) {
	call, ok := expr.(*CallExpr)
	if !ok {
		return iterChain{}, false
	}
	// ── fold/reduce ─────────────────────────────────────────────────────────────
	// fold(src, init, cb, n)  where cb(acc, elem) -> acc
	if call.Fn == "fold" || call.Fn == "reduce" {
		if len(call.Args) != 4 {
			return iterChain{}, false
		}
		cbName := l.resolveFunc(call.Args[2])
		if cbName == "" {
			return iterChain{}, false
		}
		// Infer accumulator type from callback's return type (default u8).
		accTy := mir2.Ty(mir2.TyU8)
		elemTy := mir2.Ty(mir2.TyU8)
		if cbFn, ok2 := l.hirFuncs[cbName]; ok2 {
			if cbFn.RetTy != nil && cbFn.RetTy != mir2.TyVoid {
				accTy = cbFn.RetTy
			}
			if len(cbFn.Params) >= 2 {
				elemTy = cbFn.Params[1].Ty
			} else if len(cbFn.Params) == 1 {
				elemTy = cbFn.Params[0].Ty
			}
		}
		return iterChain{
			ptr:       call.Args[0],
			foldInit:  call.Args[1],
			len:       call.Args[3],
			cbName:    cbName,
			elemTy:    elemTy,
			isFold:    true,
			foldAccTy: accTy,
		}, true
	}

	// ── forEach / mapInPlace ─────────────────────────────────────────────────
	isMutate := false
	switch call.Fn {
	case "forEach":
	case "mapInPlace":
		isMutate = true
	default:
		return iterChain{}, false
	}
	if len(call.Args) != 3 {
		return iterChain{}, false
	}

	cbName := l.resolveFunc(call.Args[1])
	if cbName == "" {
		return iterChain{}, false
	}

	chain := iterChain{
		len:    call.Args[2],
		cbName: cbName,
		mutate: isMutate,
		elemTy: mir2.TyU8, // default; refined from callback param below
	}
	if cbFn, ok2 := l.hirFuncs[cbName]; ok2 && len(cbFn.Params) > 0 {
		chain.elemTy = cbFn.Params[0].Ty
	}

	// Unwrap nested map/filter calls (collected outermost-first, reversed below).
	src := call.Args[0]
	for {
		inner, ok2 := src.(*CallExpr)
		if !ok2 {
			break
		}
		if (inner.Fn == "map" || inner.Fn == "filter") && len(inner.Args) == 2 {
			fn := l.resolveFunc(inner.Args[1])
			if fn == "" {
				break
			}
			chain.stages = append(chain.stages, iterStage{kind: inner.Fn, fn: fn})
			src = inner.Args[0]
		} else {
			break
		}
	}
	// Reverse: stages were collected outermost→innermost; we want innermost-first.
	for i, j := 0, len(chain.stages)-1; i < j; i, j = i+1, j-1 {
		chain.stages[i], chain.stages[j] = chain.stages[j], chain.stages[i]
	}
	chain.ptr = src
	return chain, true
}

// resolveFunc resolves a callback VarRefExpr to a function name.
func (l *lowerer) resolveFunc(arg Expr) string {
	vr, ok := arg.(*VarRefExpr)
	if !ok {
		return ""
	}
	if alias, ok2 := l.fnAliases[vr.Name]; ok2 {
		return alias
	}
	if l.hirFuncNames[vr.Name] {
		return vr.Name
	}
	return ""
}

// lowerFusedForEach builds the fused ForEachStmt body and lowers it.
func (l *lowerer) lowerFusedForEach(chain iterChain) {
	const elemVar = "__iter_elem__"
	elemTy := chain.elemTy
	var bodyStmts []Stmt

	// Apply each stage (map / filter).
	for _, stage := range chain.stages {
		fn := l.hirFuncs[stage.fn]
		paramName := elemVar // fallback if func not found
		if fn != nil && len(fn.Params) > 0 {
			paramName = fn.Params[0].Name
		}

		switch stage.kind {
		case "map":
			var rhs Expr
			if fn != nil && fn.Body != nil {
				if ret := extractReturnExpr(fn.Body); ret != nil {
					rhs = renameExpr(ret, paramName, elemVar)
				}
			}
			if rhs == nil {
				// Complex or unknown body — emit a call.
				rhs = &CallExpr{
					Fn:   stage.fn,
					Args: []Expr{&VarRefExpr{Name: elemVar, Ty: elemTy}},
					Ty:   elemTy,
				}
			}
			bodyStmts = append(bodyStmts, &AssignStmt{
				Target: &VarRefExpr{Name: elemVar, Ty: elemTy},
				Val:    rhs,
			})

		case "filter":
			var cond Expr
			if fn != nil && fn.Body != nil {
				if ret := extractReturnExpr(fn.Body); ret != nil {
					cond = renameExpr(ret, paramName, elemVar)
				}
			}
			if cond == nil {
				cond = &CallExpr{
					Fn:   stage.fn,
					Args: []Expr{&VarRefExpr{Name: elemVar, Ty: elemTy}},
					Ty:   mir2.TyBool,
				}
			}
			// if !cond { continue }
			// Prefer inverting comparison operators directly (avoids double-flag issue).
			bodyStmts = append(bodyStmts, &IfStmt{
				Cond: invertCond(cond),
				Then: &Block{Body: []Stmt{&ContinueStmt{}}},
			})
		}
	}

	// Inline the terminal callback.
	// For mapInPlace: the callback is a value-transform (like map); assign result to elemVar.
	// For forEach: the callback is a side-effect action; emit inline stmts.
	cbFn := l.hirFuncs[chain.cbName]
	if chain.mutate {
		// mapInPlace: apply callback as a transform, result stored back by lowerForEach.
		var rhs Expr
		paramName := elemVar
		if cbFn != nil && cbFn.Body != nil {
			if len(cbFn.Params) > 0 {
				paramName = cbFn.Params[0].Name
			}
			if ret := extractReturnExpr(cbFn.Body); ret != nil {
				rhs = renameExpr(ret, paramName, elemVar)
			}
		}
		if rhs == nil {
			// Complex body — emit a call and use its return value.
			rhs = &CallExpr{
				Fn:   chain.cbName,
				Args: []Expr{&VarRefExpr{Name: elemVar, Ty: elemTy}},
				Ty:   elemTy,
			}
		}
		bodyStmts = append(bodyStmts, &AssignStmt{
			Target: &VarRefExpr{Name: elemVar, Ty: elemTy},
			Val:    rhs,
		})
	} else if cbFn == nil || cbFn.Body == nil {
		// Unknown function — emit a call.
		bodyStmts = append(bodyStmts, &ExprStmt{
			Expr: &CallExpr{
				Fn:   chain.cbName,
				Args: []Expr{&VarRefExpr{Name: elemVar, Ty: elemTy}},
				Ty:   mir2.TyVoid,
			},
		})
	} else {
		paramName := elemVar
		if len(cbFn.Params) > 0 {
			paramName = cbFn.Params[0].Name
		}
		for _, s := range cbFn.Body.Body {
			renamed := renameStmt(s, paramName, elemVar)
			// ReturnStmt inside an inlined forEach callback = end of iteration body
			// (not return from the outer function). Skip it.
			if _, isRet := renamed.(*ReturnStmt); isRet {
				continue
			}
			bodyStmts = append(bodyStmts, renamed)
		}
	}

	forEach := &ForEachStmt{
		Var:           elemVar,
		ElemTy:        elemTy,
		Ptr:           chain.ptr,
		Start:         &IntLitExpr{Val: 0, Ty: mir2.TyU8},
		Len:           chain.len,
		MutateInPlace: chain.mutate,
		Body:   &Block{Body: bodyStmts},
	}
	l.lowerForEach(forEach)
}

// lowerFusedFold lowers fold(src, init, cb, n) to a DJNZ accumulator loop.
//
// The accumulator variable "__fold_acc__" is bound in the lowerer's env before
// calling lowerForEach; scanMutations detects it as mutated and threads it
// through block parameters automatically.  After the loop, l.env["__fold_acc__"]
// holds the final register with the accumulated result.
func (l *lowerer) lowerFusedFold(chain iterChain) mir2.Reg {
	const accVar = "__fold_acc__"
	const elemVar = "__iter_elem__"
	accTy := chain.foldAccTy
	elemTy := chain.elemTy

	// 1. Seed the accumulator.
	initReg := l.lowerExpr(chain.foldInit)
	// Promote initReg to accTy class if needed.
	l.bind(accVar, initReg, accTy)

	// 2. Build the loop body: __fold_acc__ = cb(__fold_acc__, __iter_elem__)
	var bodyExpr Expr
	cbFn := l.hirFuncs[chain.cbName]
	if cbFn != nil && cbFn.Body != nil && len(cbFn.Params) >= 2 {
		if ret := extractReturnExpr(cbFn.Body); ret != nil {
			accParamName := cbFn.Params[0].Name
			elemParamName := cbFn.Params[1].Name
			// Double rename: acc param → accVar, elem param → elemVar.
			bodyExpr = renameExpr(ret, accParamName, accVar)
			bodyExpr = renameExpr(bodyExpr, elemParamName, elemVar)
		}
	}
	if bodyExpr == nil {
		// Complex or unknown callback — emit a call.
		bodyExpr = &CallExpr{
			Fn: chain.cbName,
			Args: []Expr{
				&VarRefExpr{Name: accVar, Ty: accTy},
				&VarRefExpr{Name: elemVar, Ty: elemTy},
			},
			Ty: accTy,
		}
	}
	bodyStmts := []Stmt{
		&AssignStmt{
			Target: &VarRefExpr{Name: accVar, Ty: accTy},
			Val:    bodyExpr,
		},
	}

	// 3. Run the ForEachStmt — lowerForEach threads __fold_acc__ via block params.
	forEach := &ForEachStmt{
		Var:    elemVar,
		ElemTy: elemTy,
		Ptr:    chain.ptr,
		Start:  &IntLitExpr{Val: 0, Ty: mir2.TyU8},
		Len:    chain.len,
		Body:   &Block{Body: bodyStmts},
	}
	l.lowerForEach(forEach)

	// 4. Return the final accumulator register.
	return l.env[accVar]
}

// extractReturnExpr extracts the return value from the last statement of a block.
// Returns nil if the block is complex (not a simple single return expression).
func extractReturnExpr(body *Block) Expr {
	if body == nil || len(body.Body) == 0 {
		return nil
	}
	if rs, ok := body.Body[len(body.Body)-1].(*ReturnStmt); ok && rs.Val != nil {
		return rs.Val
	}
	return nil
}

// renameExpr deep-copies an expression, renaming VarRefExpr{Name:from} → {Name:to}.
func renameExpr(e Expr, from, to string) Expr {
	if e == nil {
		return nil
	}
	switch ex := e.(type) {
	case *IntLitExpr:
		return &IntLitExpr{Val: ex.Val, Ty: ex.Ty}
	case *BoolLitExpr:
		return &BoolLitExpr{Val: ex.Val}
	case *VarRefExpr:
		if ex.Name == from {
			return &VarRefExpr{Name: to, Ty: ex.Ty}
		}
		return &VarRefExpr{Name: ex.Name, Ty: ex.Ty}
	case *BinExpr:
		return &BinExpr{Op: ex.Op, L: renameExpr(ex.L, from, to), R: renameExpr(ex.R, from, to), Ty: ex.Ty}
	case *UnaryExpr:
		return &UnaryExpr{Op: ex.Op, X: renameExpr(ex.X, from, to), Ty: ex.Ty}
	case *CastExpr:
		return &CastExpr{X: renameExpr(ex.X, from, to), Ty: ex.Ty}
	case *CallExpr:
		args := make([]Expr, len(ex.Args))
		for i, a := range ex.Args {
			args[i] = renameExpr(a, from, to)
		}
		return &CallExpr{Fn: ex.Fn, Args: args, Ty: ex.Ty}
	case *LoadExpr:
		return &LoadExpr{Ptr: renameExpr(ex.Ptr, from, to), Ty: ex.Ty}
	case *IndexExpr:
		return &IndexExpr{
			Base: renameExpr(ex.Base, from, to), Idx: renameExpr(ex.Idx, from, to),
			ElemTy: ex.ElemTy, ElemStride: ex.ElemStride,
		}
	case *FieldExpr:
		return &FieldExpr{X: renameExpr(ex.X, from, to), Field: ex.Field, Offset: ex.Offset, Ty: ex.Ty}
	case *AddrOfExpr:
		return &AddrOfExpr{Sym: ex.Sym}
	case *ConstPtrExpr:
		return &ConstPtrExpr{ElemTy: ex.ElemTy, Addr: ex.Addr}
	default:
		return e
	}
}

// renameStmt deep-copies a statement, renaming variables as in renameExpr.
func renameStmt(s Stmt, from, to string) Stmt {
	if s == nil {
		return nil
	}
	switch st := s.(type) {
	case *ExprStmt:
		return &ExprStmt{Expr: renameExpr(st.Expr, from, to)}
	case *ReturnStmt:
		newVals := make([]Expr, len(st.Vals))
		for i, v := range st.Vals {
			newVals[i] = renameExpr(v, from, to)
		}
		return &ReturnStmt{Val: renameExpr(st.Val, from, to), Vals: newVals}
	case *AssignStmt:
		return &AssignStmt{Target: renameExpr(st.Target, from, to), Val: renameExpr(st.Val, from, to)}
	case *VarDeclStmt:
		return &VarDeclStmt{Name: st.Name, Ty: st.Ty, Init: renameExpr(st.Init, from, to)}
	case *IfStmt:
		var els *Block
		if st.Else != nil {
			els = renameBlock(st.Else, from, to)
		}
		return &IfStmt{Cond: renameExpr(st.Cond, from, to), Then: renameBlock(st.Then, from, to), Else: els}
	case *Block:
		return renameBlock(st, from, to)
	case *BreakStmt:
		return &BreakStmt{}
	case *ContinueStmt:
		return &ContinueStmt{}
	case *StoreStmt:
		return &StoreStmt{Ptr: renameExpr(st.Ptr, from, to), Val: renameExpr(st.Val, from, to)}
	case *WhileStmt:
		return &WhileStmt{Cond: renameExpr(st.Cond, from, to), Body: renameBlock(st.Body, from, to)}
	default:
		return s
	}
}

// renameBlock deep-copies a block, renaming variables.
func renameBlock(b *Block, from, to string) *Block {
	if b == nil {
		return nil
	}
	stmts := make([]Stmt, len(b.Body))
	for i, s := range b.Body {
		stmts[i] = renameStmt(s, from, to)
	}
	return &Block{Body: stmts}
}

// invertCond returns the logical negation of a condition expression.
// For comparison BinExprs it flips the operator (e.g. > → <=), which lets
// lowerCond produce a direct Cmp without an extra flag-compare for "!".
func invertCond(e Expr) Expr {
	if bin, ok := e.(*BinExpr); ok {
		var inv string
		switch bin.Op {
		case "==":
			inv = "!="
		case "!=":
			inv = "=="
		case "<":
			inv = ">="
		case "<=":
			inv = ">"
		case ">":
			inv = "<="
		case ">=":
			inv = "<"
		}
		if inv != "" {
			return &BinExpr{Op: inv, L: bin.L, R: bin.R, Ty: bin.Ty}
		}
	}
	return &UnaryExpr{Op: "!", X: e, Ty: mir2.TyBool}
}
