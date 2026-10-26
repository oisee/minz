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
	"maps"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80effects"
)

// ── Public entry point ────────────────────────────────────────────────────────

// LowerOptions configures optional HIR lowering transformations.
type LowerOptions struct {
	// DisableCountedLoops retains ordinary while lowering for differential checks.
	DisableCountedLoops bool
	// CountedLoopPolicy is per function: zero preserves the production policy,
	// CountedOrdinary refuses canonicalisation; CountedPermit bypasses only
	// pressure/nesting profitability gates, never semantic admission.
	CountedLoopPolicy map[string]CountedLoopPolicy
}

// LowerModule converts a HIR module to a MIR2 module.
func LowerModule(hm *Module) *mir2.Module {
	return LowerModuleWithOptions(hm, LowerOptions{})
}

// LowerModuleWithOptions converts HIR with explicit lowering options.
func LowerModuleWithOptions(hm *Module, opts LowerOptions) *mir2.Module {
	if err := ExpandGenerators(hm); err != nil {
		panic(err)
	}
	if err := ResolveErrorABI(hm); err != nil {
		panic(err)
	}
	m := &mir2.Module{Name: hm.Name, PointerBits: hm.PointerBits, Target: hm.Target, NonNullObjects: hm.NonNullObjects}
	for _, f := range hm.Funcs {
		m.NonNullObjects = m.NonNullObjects || f.CValueABI
	}
	for _, g := range hm.Globals {
		m.AddGlobal(g)
	}
	for i, s := range hm.Strings {
		kind := mir2.StrSString // default
		if i < len(hm.StrKinds) {
			kind = hm.StrKinds[i]
		}
		m.Strings.InternKind(s, kind)
	}

	// Collect all function names and HIR func map for callback inlining.
	funcNames := make(map[string]bool, len(hm.Funcs))
	hirFuncs := make(map[string]*Func, len(hm.Funcs))
	for _, f := range hm.Funcs {
		funcNames[f.Name] = true
		hirFuncs[f.Name] = f
	}
	setters, err := smcAccessors(hm)
	if err != nil {
		panic(err)
	}
	for _, f := range setters {
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
		lowerFuncWithFuncNames(m, f, funcNames, hirFuncs, hm.LoopLineage, opts)
	}
	for _, f := range hm.Funcs {
		for _, p := range f.Params {
			if !p.SMC {
				continue
			}
			sym := f.Name + "$" + p.Name
			setter := m.AddFunc(mir2.SMCSetterName(sym))
			setter.SMCSetterFor = sym
			setter.Attrs.FixedIndirectABI = true
			b := mir2.NewBuilder(setter)
			b.SwitchToNewBlock("entry")
			value := b.Param("value", p.Ty, mir2.ClassPointer)
			b.PatchNamed(sym, value, p.Ty)
			b.Ret()
		}
	}
	if err := mir2.LowerStructReturnBuffers(m); err != nil {
		panic(err)
	}
	return m
}

// hasFreeVars reports whether f references any variable that is not one of its
// own parameters, not a global, and not a module-level function name.
// Such functions are capture-lambdas that must be inlined rather than lowered
// as standalone MIR2 functions.
func hasFreeVars(f *Func, globalNames, funcNames map[string]bool) bool {
	if len(f.Captures) > 0 {
		return true
	}
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
	case *CallIndirectExpr:
		if exprHasFreeRef(ex.FnPtr, bound, globals, funcs) {
			return true
		}
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
	case *CondExpr:
		return exprHasFreeRef(ex.Cond, bound, globals, funcs) ||
			exprHasFreeRef(ex.Then, bound, globals, funcs) ||
			exprHasFreeRef(ex.Else, bound, globals, funcs)
	case *LetInExpr:
		if exprHasFreeRef(ex.Init, bound, globals, funcs) {
			return true
		}
		bound[ex.Name] = true
		return exprHasFreeRef(ex.Body, bound, globals, funcs)
	}
	return false
}

// ── Per-function lowering state ───────────────────────────────────────────────

// loopCtx holds the context needed for break/continue inside a loop.
type loopCtx struct {
	headLabel    string     // continue → jump here (with headMutated args)
	exitLabel    string     // break    → jump here (with exitMutated args)
	mutated      []string   // loop-carried variable names (same set for both)
	exitPrefix   []mir2.Reg // ForEach exit also receives its pointer and counter
	scopeDepth   int
	continueStep Stmt // ForRange advances its induction variable before continue
}

type lowerer struct {
	disableCountedLoops bool
	permitCountedLoops  bool
	loopCostIndex       int
	// Only active within one literal-address read/modify/write assignment.
	// Memory operations retain their order; the pure address is computed once.
	literalStorePointers map[int64]mir2.Reg

	errorFlag, errorCode mir2.Reg
	gotoDeadBodies       map[*Block]bool
	gotoDemands          map[string]map[string]bool
	gotoAncestors        map[string][]*Block
	activeBlocks         []*Block
	gotoPending          map[string][]gotoIncoming
	scopeRestores        []func()
	lineage              map[Stmt]LoopLineage
	hf                   *Func
	m                    *mir2.Module
	mf                   *mir2.Func
	bld                  *mir2.Builder
	target               uint8 // platform target from hir.Module.Target

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

	// localArrays tracks local array variables promoted to globals.
	// &name for these should return the env register (address) directly.
	localArrays  map[string]bool
	localCells   map[string]bool // scalar objects represented by an owned address
	addressTaken map[string]bool

	// loopStack is the stack of enclosing loops (top = innermost).
	// Needed by BreakStmt and ContinueStmt.
	loopStack []loopCtx

	// inLoopContext is true whenever we are lowering expressions that form part
	// of a loop — either the loop head condition or the loop body.  It is used
	// to suppress classForALUResult: loop-carried values that live across ALU
	// operations must not be forced into A, which is a transient workspace for
	// comparisons and increments.
	inLoopContext bool

	// labelSeq generates unique block labels within this function.
	labelSeq int

	// gotoLabels maps C label names to MIR2 block labels.
	gotoLabels map[string]string

	// backwardGotoLabels is the set of labels that are targets of backward gotos
	// (i.e., the goto appears after the label in source order). These labels need
	// block parameters for all live variables so the back-edge carries updated values.
	backwardGotoLabels map[string]bool

	// backwardGotoVars records the sorted variable names threaded through each
	// backward-goto label block. Set when the LabelStmt is first lowered.
	backwardGotoVars map[string][]string
}

type gotoIncoming struct {
	block *mir2.Block
	env   map[string]mir2.Reg
}

func (l *lowerer) pushLoop(ctx loopCtx) {
	ctx.scopeDepth = len(l.scopeRestores)
	l.loopStack = append(l.loopStack, ctx)
}
func (l *lowerer) popLoop() { l.loopStack = l.loopStack[:len(l.loopStack)-1] }
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
	delete(l.fnAliases, name)
	delete(l.localArrays, name)
	delete(l.localCells, name)
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
	r := l.bld.AddrOf(name, mir2.ClassPointer)
	if l.target == TargetMZV {
		l.bld.Cur.Insts[len(l.bld.Cur.Insts)-1].Ty = mir2.PtrFor(32)
	}
	return r
}

// lowerExprAddr lowers an expression to its address (lvalue), rather than its value.
// Used for field-store targets (origin.y = v) and eventually local struct alloca.
func (l *lowerer) lowerExprAddr(ex Expr) mir2.Reg {
	switch e := ex.(type) {
	case *VarRefExpr:
		// Addressable locals and parameters shadow globals just as value reads do.
		if reg, ok := l.env[e.Name]; ok && (l.localCells[e.Name] || l.localArrays[e.Name]) {
			return reg
		}
		if _, ok := l.findGlobal(e.Name); ok {
			return l.globalAddr(e.Name)
		}
		// Local pointer variable (e.g. param `self: ^Acc`): the register value IS the address.
		if reg, ok := l.env[e.Name]; ok {
			return reg
		}
		panic(fmt.Sprintf("hir/lower: cannot take address of local variable %q", e.Name))
	case *DerefExpr:
		return l.lowerExpr(e.Ptr)
	case *StructLitExpr:
		return l.lowerExpr(e)
	case *FieldExpr:
		baseAddr := l.lowerExpr(e.X)
		if e.Offset > 0 {
			return l.bld.Field(baseAddr, int64(e.Offset), mir2.ClassPointer)
		}
		return baseAddr
	case *IndexExpr:
		if l.target == TargetMZV {
			return l.lowerIndexAddr(e)
		}
		// base[i] address — same as lowerIndex but without the final Load
		baseAddr := l.lowerExpr(e.Base)
		if l.target != TargetAgon {
			if offset, ok := l.smallSMCIndexOffset(e); ok {
				if offset == 0 {
					return baseAddr
				}
				return l.bld.Field(baseAddr, offset, mir2.ClassPointer)
			}
		}
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

func bitBaseTy(base Expr) mir2.Ty {
	if base == nil {
		return mir2.TyU8
	}
	return base.ExprTy()
}

func bitMaskFor(baseTy mir2.Ty, bit int64) int64 {
	mask := int64(1) << bit
	if baseTy == nil {
		return mask
	}
	w := baseTy.Width()
	if w <= 0 || w >= 63 {
		return mask
	}
	return mask & ((int64(1) << w) - 1)
}

func (l *lowerer) lowerBitRead(ex *BitExpr) mir2.Reg {
	baseTy := bitBaseTy(ex.X)
	base := l.lowerExpr(ex.X)
	return l.bld.BitGet(base, ex.Bit, baseTy, classForExpr(mir2.TyU8))
}

func (l *lowerer) lowerBitAssign(tgt *BitExpr, val Expr) {
	baseTy := bitBaseTy(tgt.X)
	maskImm := bitMaskFor(baseTy, int64(tgt.Bit))

	constAssign := func() (set bool, value bool) {
		switch v := val.(type) {
		case *IntLitExpr:
			return true, (v.Val & 1) != 0
		case *BoolLitExpr:
			return true, v.Val
		default:
			return false, false
		}
	}

	merge := func(cur mir2.Reg) mir2.Reg {
		if ok, bit := constAssign(); ok {
			if bit {
				return l.bld.BitSet(cur, tgt.Bit, baseTy, classForExpr(baseTy))
			}
			return l.bld.BitReset(cur, tgt.Bit, baseTy, classForExpr(baseTy))
		}
		notMask := l.bld.Const(^maskImm, baseTy, mir2.ClassGeneral)
		rhs := l.lowerExprAs(val, mir2.TyU8)
		rhsBase := l.bld.Ext(rhs, mir2.TyU8, baseTy, classForExpr(baseTy))
		one := l.bld.Const(1, baseTy, mir2.ClassGeneral)
		bitVal := l.bld.And(rhsBase, one, baseTy, classForExpr(baseTy))
		setVal := bitVal
		if tgt.Bit > 0 {
			shift := l.bld.Const(int64(tgt.Bit), baseTy, mir2.ClassGeneral)
			setVal = l.bld.Shl(bitVal, shift, baseTy, classForExpr(baseTy))
		}
		cleared := l.bld.And(cur, notMask, baseTy, classForExpr(baseTy))
		return l.bld.Or(cleared, setVal, baseTy, classForExpr(baseTy))
	}

	switch base := tgt.X.(type) {
	case *VarRefExpr:
		next := merge(l.lowerExpr(base))
		if g, found := l.findGlobal(base.Name); found {
			addr := l.globalAddr(base.Name)
			l.bld.Store(addr, next, g.Ty)
			return
		}
		l.env[base.Name] = next
	case *FieldExpr:
		addr := l.lowerExprAddr(base)
		l.bld.Store(addr, merge(l.lowerExpr(base)), baseTy)
	case *IndexExpr:
		addr := l.lowerExprAddr(base)
		l.bld.Store(addr, merge(l.lowerExpr(base)), baseTy)
	case *LoadExpr:
		ptr := l.lowerExpr(base.Ptr)
		l.bld.Store(ptr, merge(l.lowerExpr(base)), baseTy)
	case *DerefExpr:
		ptr := l.lowerExpr(base.Ptr)
		cur := l.bld.Load(ptr, baseTy, classForExpr(baseTy))
		l.bld.Store(ptr, merge(cur), baseTy)
	default:
		panic(fmt.Sprintf("hir/lower: unsupported BitExpr target base %T", tgt.X))
	}
}

// ── Function lowering ─────────────────────────────────────────────────────────

func lowerFunc(m *mir2.Module, f *Func) {
	lowerFuncWithFuncNames(m, f, nil, nil, nil, LowerOptions{})
}

func lowerFuncWithFuncNames(m *mir2.Module, f *Func, funcNames map[string]bool, hirFuncs map[string]*Func, lineage map[Stmt]LoopLineage, opts LowerOptions) {
	mf := m.AddFunc(f.Name)
	mf.Attrs.Private = f.Private
	mf.Attrs.ReferencedByName = f.ReferencedByName
	mf.Attrs.FallibleCY = f.FallibleCY
	if f.IsExtern {
		mf.Attrs.IsExtern = true
		mf.Attrs.ExternAddr = f.ExternAddr
		mf.Contract.ExternClobbers = f.ExternClobbers
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

	if f.IsExtern {
		bld := mir2.NewBuilder(mf)
		for i, p := range f.Params {
			cls := classForParam(p.Ty, i)
			if p.RegClass != 0 {
				cls = p.RegClass
			}
			bld.Param(p.Name, p.Ty, cls)
			mf.Contract.Params[len(mf.Contract.Params)-1].ClassHard = p.RegClass != 0
		}
		return
	}

	bld := mir2.NewBuilder(mf)
	l := &lowerer{
		hf:                  f,
		disableCountedLoops: opts.DisableCountedLoops || opts.CountedLoopPolicy[f.Name] == CountedOrdinary,
		permitCountedLoops:  opts.CountedLoopPolicy[f.Name] == CountedPermit,
		m:                   m,
		mf:                  mf,
		bld:                 bld,
		target:              m.Target,
		env:                 make(map[string]mir2.Reg),
		envTy:               make(map[string]mir2.Ty),
		hirFuncNames:        funcNames,
		hirFuncs:            hirFuncs,
		fnAliases:           make(map[string]string),
		localArrays:         make(map[string]bool),
		localCells:          make(map[string]bool),
		addressTaken:        make(map[string]bool),
	}

	if lineage != nil {
		l.lineage = map[Stmt]LoopLineage{}
		for node, origin := range lineage {
			l.lineage[node] = origin
		}
	}
	bld.SwitchToNewBlock("entry")

	walkGraceBlocks(f.Body, "", func(_ string, block *Block) {
		for _, stmt := range block.Body {
			visitStmtExprs(stmt, func(expr Expr) {
				if address, ok := expr.(*AddrOfExpr); ok {
					if address.Sym != "" {
						l.addressTaken[address.Sym] = true
					}
					if ref, ok := address.X.(*VarRefExpr); ok {
						l.addressTaken[ref.Name] = true
					}
				}
			})
		}
	})

	// Bind parameters to virtual registers.
	// @smc params are NOT MIR2 contract params — they become PatchSlots in the
	// entry block (LD HL, imm16 with EQU label; auto-patcher synthesised by z80codegen).
	nonSMCIdx := 0
	for _, p := range f.Params {
		if p.SMC {
			sym := f.Name + "$" + p.Name
			r := bld.PatchSlotNamed(sym, 0, p.Ty, mir2.ClassPointer)
			l.bind(p.Name, r, p.Ty)
			continue
		}
		cls := classForParam(p.Ty, nonSMCIdx)
		if p.RegClass != 0 {
			cls = p.RegClass
		}
		r := bld.Param(p.Name, p.Ty, cls)
		l.bind(p.Name, r, p.Ty)
		nonSMCIdx++
	}

	for _, p := range f.Params {
		if !l.addressTaken[p.Name] {
			continue
		}
		if _, aggregate := mir2.BaseOf(p.Ty).(*mir2.StructTy); aggregate {
			continue
		}
		value := l.env[p.Name]
		cell := &mir2.StructTy{Fields: []mir2.StructField{{Name: "value", Ty: p.Ty}}}
		address := l.fixedStructStorage(cell)
		l.storeMemory(address, value, p.Ty, false)
		l.bind(p.Name, address, l.m.PointerType())
		l.localCells[p.Name] = true
	}

	// Pre-create blocks for all goto labels (forward references).
	l.gotoLabels = make(map[string]string)
	collectLabels(f.Body, func(name string) {
		blkLabel := "label_" + name
		bld.F.NewBlock(blkLabel)
		l.gotoLabels[name] = blkLabel
	})

	// Detect backward gotos (loop-via-goto) — these need variable threading.
	l.backwardGotoLabels = detectBackwardGotos(f.Body)
	l.backwardGotoVars = make(map[string][]string)
	l.gotoDeadBodies = map[*Block]bool{}
	l.gotoDemands = gotoLabelDemands(f.Body)
	l.gotoAncestors = map[string][]*Block{}
	l.gotoPending = map[string][]gotoIncoming{}
	collectGotoAncestors(f.Body, nil, l.gotoAncestors)

	// Lower body.
	l.lowerBlock(f.Body)

	// Implicit return for void functions that fall off the end of the current block.
	if !bld.Cur.IsSealed() {
		bld.Ret()
	}
	// Function-wide labels in unreachable lexical branches were precreated.
	for _, block := range bld.F.Blocks {
		if !block.IsSealed() {
			block.Term = &mir2.TermUnreachable{}
		}
	}
}

// classForParam chooses a register class for the i-th parameter.
//
//	pos 0 → ClassAcc     (A)
//	ptr   → ClassPointer (HL)
//	pos 1 → ClassGeneral (C/D/E)
//	pos 2 → ClassCounter (B) — common for a count/length 3rd arg
func classForParam(ty mir2.Ty, pos int) mir2.RegClass {
	if mir2.IsPointer(ty) {
		return mir2.ClassPointer
	}
	if aggregateAddressType(ty) {
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

// aggregateAddressType distinguishes storage layout from the current MIR
// value representation: structs and arrays are passed as object addresses.
func aggregateAddressType(ty mir2.Ty) bool {
	switch mir2.BaseOf(ty).(type) {
	case *mir2.StructTy, *mir2.ArrayTy:
		return true
	}
	return false
}

// classForRet chooses the return class for a single-return function.
func classForRet(ty mir2.Ty) mir2.RegClass {
	if aggregateAddressType(ty) || mir2.IsPointer(ty) || ty == mir2.TyU16 || ty == mir2.TyI16 {
		return mir2.ClassPointer // HL
	}
	return mir2.ClassAcc // A
}

// classForRetPos chooses the return class for position pos in a multi-return function.
//
//	pos 0 → same as classForRet (A for u8, HL for u16)
//	pos 1 → ClassCounter (B) for u8/u16; ClassIndex (DE) for pointers/u16 pairs
//	pos 2 → ClassGeneral (C/D/E)
//
// Note: ClassIndex (DE) is a 16-bit pair and cannot hold an 8-bit return value
// directly — using it for u8 causes the allocator to spill to memory.
// Use ClassCounter (B) for the second u8 return instead.
func classForRetPos(ty mir2.Ty, pos int) mir2.RegClass {
	w := ty.Width()
	if aggregateAddressType(ty) {
		w = 16
	}
	switch pos {
	case 0:
		return classForRet(ty)
	case 1:
		if w <= 8 {
			return mir2.ClassCounter // B — 8-bit register, fits u8
		}
		return mir2.ClassIndex // DE — 16-bit pair for u16/ptr
	case 3:
		if w <= 8 {
			// The third byte's canonical ClassGeneral return is C. A
			// fourth byte must occupy a distinct home across the call ABI.
			return mir2.ClassRegD
		}
		return mir2.ClassGeneral
	default:
		return mir2.ClassGeneral // C/D/E
	}
}

// lowerExprForRet lowers an expression that is the direct value of a ReturnStmt.
// For a LoadExpr or FieldExpr at the top level, it uses classForRet (ClassAcc)
// instead of classForExpr (ClassGeneral), so the load goes directly into A and
// avoids the redundant LD C,(HL); LD A,C sequence.
func (l *lowerer) lowerExprForRet(e Expr) mir2.Reg {
	if literal, ok := e.(*IntLitExpr); ok && len(l.bld.F.Contract.Returns) > 0 {
		ty := l.bld.F.Contract.Returns[0].Ty
		if mir2.IsInt(literal.Ty) && mir2.IsInt(ty) {
			return l.bld.Const(literalValueAs(literal, ty), ty, classForRet(ty))
		}
	}
	// Return conditionals lower each branch in the function's return context.
	// Joining first at the conditional's narrow type can truncate a wider arm.
	if _, conditional := e.(*CondExpr); !conditional && len(l.bld.F.Contract.Returns) > 0 {
		ty := l.bld.F.Contract.Returns[0].Ty
		if mir2.IsInt(e.ExprTy()) && mir2.IsInt(ty) && e.ExprTy().Width() != ty.Width() {
			return l.lowerExprAs(e, ty)
		}
	}
	switch ex := e.(type) {
	case *LoadExpr:
		if ex.Ty.Width() <= 8 && !aggregateAddressType(ex.Ty) {
			ptr := l.lowerExpr(ex.Ptr)
			return l.bld.Load(ptr, ex.Ty, classForRet(ex.Ty))
		}
	case *FieldExpr:
		if ex.Ty.Width() <= 8 && !aggregateAddressType(ex.Ty) {
			base := l.lowerExpr(ex.X)
			if ex.Offset > 0 {
				base = l.bld.Field(base, int64(ex.Offset), mir2.ClassPointer)
			}
			return l.bld.Load(base, ex.Ty, classForRet(ex.Ty))
		}
	case *CondExpr:
		// Desugar `return cond ? X : Y` into two separate Ret blocks:
		//   BrIf(cond, then, else)
		//   then: Ret(X)
		//   else: Ret(Y)
		// This avoids a join block and enables CondRetSink + FuseAbsDiff.
		cond := l.lowerCond(ex.Cond)
		thenLabel := l.fresh("cret_then")
		elseLabel := l.fresh("cret_else")
		l.bld.BrIf(cond, thenLabel, nil, elseLabel, nil)

		l.bld.SwitchToNewBlock(thenLabel)
		thenVal := l.lowerExprForRet(ex.Then)
		if thenVal != mir2.NoReg {
			l.bld.Ret(thenVal)
		}

		l.bld.SwitchToNewBlock(elseLabel)
		elseVal := l.lowerExprForRet(ex.Else)
		if elseVal != mir2.NoReg {
			l.bld.Ret(elseVal)
		}

		// Return a dummy — the Ret instructions are already emitted.
		// The caller (ReturnStmt handler) will see that the block is
		// already terminated and skip emitting another Ret.
		return mir2.NoReg
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
	if mir2.IsPointer(ty) && ty.Width() == 24 {
		return mir2.ClassPointer
	}
	if aggregateAddressType(ty) || (mir2.IsPointer(ty) && mir2.PointerElem(ty) != mir2.TyU8) || ty == mir2.TyU16 || ty == mir2.TyI16 {
		return mir2.ClassPointer
	}
	return mir2.ClassGeneral
}

// classForALUResult returns the preferred class for an 8-bit ALU operation
// result. Z80 ALU ops (ADD, SUB, AND, OR, XOR, NEG) always write to A, so
// ClassAcc avoids a spurious LD C,A after the instruction.
// 16-bit variants write to HL → ClassPointer.
// Other ops (MUL, shifts) are not pure A-writes → ClassGeneral.
func classForALUResult(op string, ty mir2.Ty) mir2.RegClass {
	if aggregateAddressType(ty) || (mir2.IsPointer(ty) && mir2.PointerElem(ty) != mir2.TyU8) || ty == mir2.TyU16 || ty == mir2.TyI16 {
		return mir2.ClassPointer
	}
	switch op {
	case "+", "-", "&", "|", "^":
		return mir2.ClassAcc
	}
	return classForExpr(ty)
}

// classForLoop chooses a good class for a loop-carried variable.
func classForLoop(ty mir2.Ty, _ string) mir2.RegClass {
	if aggregateAddressType(ty) || (mir2.IsPointer(ty) && mir2.PointerElem(ty) != mir2.TyU8) || ty == mir2.TyU16 || ty == mir2.TyI16 {
		return mir2.ClassPointer
	}
	// ClassGeneral keeps loop vars out of A (which comparisons need).
	return mir2.ClassGeneral
}

// ── Statement lowering ────────────────────────────────────────────────────────

// lowerBlock lowers a HIR block (list of statements).
// Returns true if the block already ends with a terminator.
func (l *lowerer) lowerBlock(blk *Block) bool {
	dead := l.gotoDeadBodies[blk]
	delete(l.gotoDeadBodies, blk)
	return l.lowerBlockEntry(blk, dead)
}
func (l *lowerer) lowerBlockEntry(blk *Block, dead bool) bool {
	l.activeBlocks = append(l.activeBlocks, blk)
	defer func() { l.activeBlocks = l.activeBlocks[:len(l.activeBlocks)-1] }()
	// Each lexical declaration hides a binding only for this block. Snapshot
	// when the declaration executes, so earlier assignments to outer variables
	// survive. Restore metadata together with the SSA register on every exit.
	type binding struct {
		reg                     mir2.Reg
		ty                      mir2.Ty
		alias                   string
		hasReg, hasTy, hasAlias bool
		array, cell             bool
	}
	locals := map[string]binding{}
	save := func(name string) {
		if _, saved := locals[name]; saved {
			return
		}
		r, rok := l.env[name]
		ty, tok := l.envTy[name]
		alias, aok := l.fnAliases[name]
		locals[name] = binding{r, ty, alias, rok, tok, aok, l.localArrays[name], l.localCells[name]}
	}
	restore := func() {
		for name, b := range locals {
			if b.hasReg {
				l.env[name] = b.reg
			} else {
				delete(l.env, name)
			}
			if b.hasTy {
				l.envTy[name] = b.ty
			} else {
				delete(l.envTy, name)
			}
			if b.array {
				l.localArrays[name] = true
			} else {
				delete(l.localArrays, name)
			}
			if b.cell {
				l.localCells[name] = true
			} else {
				delete(l.localCells, name)
			}
			if b.hasAlias {
				l.fnAliases[name] = b.alias
			} else {
				delete(l.fnAliases, name)
			}
		}
	}
	l.scopeRestores = append(l.scopeRestores, restore)
	defer func() { restore(); l.scopeRestores = l.scopeRestores[:len(l.scopeRestores)-1] }()
	terminated := dead
	for _, s := range blk.Body {
		if terminated {
			// After a terminator (goto/break/return), only labels are reachable.
			var targetBlocks []*Block
			switch x := s.(type) {
			case *Block:
				targetBlocks = []*Block{x}
			case *IfStmt:
				targetBlocks = []*Block{x.Then, x.Else}
			case *WhileStmt:
				targetBlocks = []*Block{x.Body}
			case *ForRangeStmt:
				targetBlocks = []*Block{x.Body}
			case *ForEachStmt:
				targetBlocks = []*Block{x.Body}
			}
			if conditional, ok := s.(*IfStmt); ok {
				found := false
				for _, body := range []*Block{conditional.Then, conditional.Else} {
					collectLabels(body, func(name string) {
						if len(l.gotoPending[name]) > 0 {
							found = true
						}
					})
				}
				if found {
					unreachable := l.bld.SwitchToNewBlock(l.fresh("goto_unreachable"))
					l.gotoDeadBodies[conditional.Then] = true
					l.gotoDeadBodies[conditional.Else] = true
					terminated = l.lowerIf(conditional)
					unreachable.Term = &mir2.TermUnreachable{}
				}
				continue
			}
			if rng, ok := s.(*ForRangeStmt); ok {
				found := false
				collectLabels(rng.Body, func(name string) {
					if len(l.gotoPending[name]) > 0 {
						found = true
					}
				})
				if found {
					l.bld.SwitchToNewBlock(l.fresh("goto_unreachable"))
					l.gotoDeadBodies[rng.Body] = true
					l.lowerForRange(rng)
					terminated = false
				}
				continue
			}
			if sw, ok := s.(*SwitchStmt); ok {
				found := false
				bodies := []*Block{sw.Default}
				for _, c := range sw.Cases {
					bodies = append(bodies, c.Body)
				}
				for _, body := range bodies {
					collectLabels(body, func(name string) {
						if len(l.gotoPending[name]) > 0 {
							found = true
						}
					})
				}
				if found {
					unreachable := l.bld.SwitchToNewBlock(l.fresh("goto_unreachable"))
					for _, body := range bodies {
						l.gotoDeadBodies[body] = true
					}
					l.lowerSwitch(sw)
					unreachable.Term = &mir2.TermUnreachable{}
					terminated = l.bld.Cur.IsSealed()
				}
				continue
			}
			if loop, ok := s.(*WhileStmt); ok {
				found := false
				collectLabels(loop.Body, func(name string) {
					if len(l.gotoPending[name]) > 0 {
						found = true
					}
				})
				if found {
					refs := map[string]bool{}
					collectExprRefs(loop.Cond, refs)
					missing := false
					for name := range refs {
						if _, ok := l.envTy[name]; ok && l.env[name] == mir2.NoReg {
							missing = true
						}
					}
					if missing {
						// Verify only permits continuations which escape before
						// reading this skipped condition binding.
						terminated = l.lowerBlockEntry(loop.Body, true)
					} else {
						l.bld.SwitchToNewBlock(l.fresh("goto_unreachable"))
						l.gotoDeadBodies[loop.Body] = true
						l.lowerWhile(loop)
						terminated = false
					}
				}
				continue
			}
			for _, nested := range targetBlocks {
				found := false
				collectLabels(nested, func(name string) {
					if len(l.gotoPending[name]) > 0 {
						found = true
					}
				})
				if found {
					terminated = l.lowerBlockEntry(nested, true)
				}
			}
			if len(targetBlocks) > 0 {
				continue
			}
			if ls, ok := s.(*LabelStmt); ok {
				l.lowerStmt(ls)
				terminated = false
				continue
			}
			// A skipped initializer creates no SSA value, but its declaration
			// still establishes lexical identity and type.
			switch decl := s.(type) {
			case *VarDeclStmt:
				save(decl.Name)
				delete(l.env, decl.Name)
				delete(l.fnAliases, decl.Name)
				delete(l.localArrays, decl.Name)
				delete(l.localCells, decl.Name)
				l.envTy[decl.Name] = decl.Ty
			case *TupleLetStmt:
				for i, name := range decl.Names {
					if name != "_" {
						save(name)
						delete(l.env, name)
						delete(l.fnAliases, name)
						delete(l.localArrays, name)
						l.envTy[name] = decl.Tys[i]
					}
				}
			}
			continue
		}
		switch decl := s.(type) {
		case *VarDeclStmt:
			save(decl.Name)
		case *TupleLetStmt:
			for _, name := range decl.Names {
				if name != "_" {
					save(name)
				}
			}
		}
		if l.lowerStmt(s) {
			terminated = true
		}
	}
	if terminated && !l.bld.Cur.IsSealed() {
		l.bld.Unreachable()
	}
	return terminated
}

// lowerStmt lowers one statement.  Returns true if it emitted a terminator.
func (l *lowerer) lowerStmt(s Stmt) bool {
	switch st := s.(type) {

	case *Block:
		return l.lowerBlock(st)

	case *VarDeclStmt:
		if (st.Volatile || l.addressTaken[st.Name]) && st.ArrayLen == 0 {
			_, aggregate := mir2.BaseOf(st.Ty).(*mir2.StructTy)
			_, array := mir2.BaseOf(st.Ty).(*mir2.ArrayTy)
			if !aggregate && !array {
				// Own addressable storage even when declared inside a loop.
				// The environment carries its address through CFG edges.
				cell := &mir2.StructTy{Fields: []mir2.StructField{{Name: "value", Ty: st.Ty}}}
				addr := l.fixedStructStorage(cell)
				if st.Init != nil {
					value := l.lowerStorageExprAs(st.Init, st.Ty)
					l.storeMemory(addr, value, st.Ty, st.Volatile)
				}
				l.bind(st.Name, addr, l.m.PointerType())
				l.localCells[st.Name] = true // reads/writes use the cell, AddrOf uses its address
				return false
			}
		}
		if ty, ok := mir2.BaseOf(st.Ty).(*mir2.ArrayTy); ok && st.ArrayLen == 0 {
			// Array-typed declarations own automatic storage. Legacy ArrayLen
			// declarations retain their existing frontend representation.
			storage := &mir2.StructTy{Fields: []mir2.StructField{{Name: "elements", Ty: ty}}}
			addr := l.fixedStructStorage(storage)
			if st.Init != nil {
				source := l.lowerExpr(st.Init)
				l.copyStructValueEffects(addr, source, storage, l.volatileObject(st.Init), st.Volatile)
			}
			l.bind(st.Name, addr, l.m.PointerType())
			l.localArrays[st.Name] = true
			return false
		}

		if ty, ok := mir2.BaseOf(st.Ty).(*mir2.StructTy); ok && st.ArrayLen == 0 {
			// Source struct locals own their bytes; a call-site result buffer is
			// temporary and can be reused by the next loop iteration.
			dest := l.fixedStructStorage(ty)
			if lit, ok := st.Init.(*StructLitExpr); ok {
				l.emitStructLitFields(lit, func() mir2.Reg { return dest }, st.Volatile)
			} else if st.Init != nil {
				src := l.lowerExpr(st.Init)
				l.copyStructValueEffects(dest, src, ty, l.volatileObject(st.Init), st.Volatile)
			}
			delete(l.fnAliases, st.Name)
			l.bind(st.Name, dest, st.Ty)
			return false
		}
		var r mir2.Reg
		if st.Init != nil {
			// Check for fold/reduce: let result = fold(ptr, init, cb, n)
			if foldReg, ok := l.tryLowerFold(st.Init); ok {
				chain, _ := l.recognizeIterChain(st.Init)
				l.bind(st.Name, l.coerceLocalInteger(foldReg, chain.foldAccTy, st.Ty), st.Ty)
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
					delete(l.env, st.Name)
					delete(l.envTy, st.Name)
					delete(l.localArrays, st.Name)
					l.fnAliases[st.Name] = funcName
				}
				return false
			}
		} else if _, isStruct := st.Ty.(*mir2.StructTy); isStruct {
			// Local struct variable without init: allocate stack space.
			// Returns a pointer register (ClassPointer = HL on Z80).
			size := int64(st.Ty.Width() / 8)
			if size <= 0 {
				size = 1
			}
			r = l.bld.Alloca(size)
		} else if st.ArrayLen > 0 {
			// Local array variable: promote to module-level global.
			// Z80 has no stack frames — static storage is used instead.
			elemBytes := st.Ty.Width() / 8
			if elemBytes <= 0 {
				elemBytes = 1
			}
			totalBytes := st.ArrayLen * elemBytes
			globalName := l.mf.Name + "$" + st.Name
			baseName := globalName
			for {
				if _, exists := l.findGlobal(globalName); !exists {
					break
				}
				globalName = baseName + "$" + l.fresh("scope_")
			}
			initBytes := make([]byte, totalBytes)
			// Pre-populate from Initial (array designated/positional init)
			for i, expr := range st.Initial {
				if expr == nil {
					continue
				}
				if lit, ok := expr.(*IntLitExpr); ok {
					off := i * elemBytes
					if off < totalBytes {
						initBytes[off] = byte(lit.Val)
						if elemBytes >= 2 && off+1 < totalBytes {
							initBytes[off+1] = byte(lit.Val >> 8)
						}
					}
				}
			}
			l.m.Globals = append(l.m.Globals, mir2.Global{
				Name: globalName,
				Ty:   mir2.NewArray(st.Ty, st.ArrayLen),
				Init: initBytes,
			})
			r = l.bld.AddrOf(globalName, mir2.ClassPointer)
			l.localArrays[st.Name] = true
			// For non-constant initializers, emit runtime stores
			for i, expr := range st.Initial {
				if expr == nil {
					continue
				}
				if _, ok := expr.(*IntLitExpr); ok {
					continue // already in initBytes
				}
				val := l.lowerStorageExprAs(expr, st.Ty)
				off := i * elemBytes
				if off == 0 {
					l.bld.Store(r, val, st.Ty)
				} else {
					addr := l.bld.Add(r, l.bld.Const(int64(off), mir2.TyU16, mir2.ClassPointer), mir2.TyU16, mir2.ClassPointer)
					l.bld.Store(addr, val, st.Ty)
				}
			}
		} else {
			r = l.bld.Const(0, st.Ty, classForExpr(st.Ty))
		}
		if st.Init != nil && st.ArrayLen == 0 {
			r = l.coerceLocalInteger(r, st.Init.ExprTy(), st.Ty)
		}
		delete(l.fnAliases, st.Name) // a scalar declaration hides a callable alias
		bindingTy := st.Ty
		if st.ArrayLen > 0 {
			// The environment carries the array base address, not one element.
			// Loop block parameters must retain the full pointer width.
			bindingTy = l.m.PointerType()
		}
		l.bind(st.Name, r, bindingTy)
		if st.ArrayLen > 0 {
			l.localArrays[st.Name] = true
		}
		return false

	case *AssignStmt:
		if target, ok := st.Target.(*DerefExpr); ok {
			_, integer := target.Ptr.(*IntLitExpr)
			_, constant := target.Ptr.(*ConstPtrExpr)
			if (integer && target.Ptr.ExprTy() == l.m.PointerType() || constant) && literalPointerExpression(st.Val) {
				previous := l.literalStorePointers
				l.literalStorePointers = make(map[int64]mir2.Reg)
				defer func() { l.literalStorePointers = previous }()
			}
		}
		// Check for fold/reduce: x = fold(ptr, init, cb, n)
		if foldReg, ok := l.tryLowerFold(st.Val); ok {
			if tgt, ok2 := st.Target.(*VarRefExpr); ok2 {
				chain, _ := l.recognizeIterChain(st.Val)
				l.env[tgt.Name] = l.coerceLocalInteger(foldReg, chain.foldAccTy, tgt.Ty)
				l.envTy[tgt.Name] = tgt.Ty
				return false
			}
		}
		// Struct literal in-place construction: target = StructName{...}
		// Write each field directly into the target memory — no alloca needed.
		if lit, ok := st.Val.(*StructLitExpr); ok {
			switch tgt := st.Target.(type) {
			case *DerefExpr:
				// ptr^ = Struct{...}
				// Use sequential chaining: each field address derived from the
				// PREVIOUS field address + diff, so no base reg stays live across
				// multiple Field ops — all can share HL.
				ptrReg := l.lowerExpr(tgt.Ptr)
				l.emitStructLitFieldsChained(lit, ptrReg, l.volatileObject(tgt))
				return false
			case *VarRefExpr:
				// global_struct = Color{...}
				// Emit a fresh AddrOf per field so each AddrOf has useCount==1 —
				// that allows the HL-chain optimizer in z80codegen to fire:
				//   LD HL,sym ; LD (HL),n ; INC HL ; LD (HL),n ; ...
				if _, found := l.findGlobal(tgt.Name); found {
					name := tgt.Name
					l.emitStructLitFields(lit, func() mir2.Reg { return l.globalAddr(name) }, l.volatileObject(tgt))
					return false
				}
			}
		}
		val := l.lowerExpr(st.Val)
		valTy := st.Val.ExprTy()
		if mir2.IsInt(valTy) && mir2.IsInt(st.Target.ExprTy()) && valTy.Width() != st.Target.ExprTy().Width() {
			val = l.coerceLocalInteger(val, valTy, st.Target.ExprTy())
			valTy = st.Target.ExprTy()
		}
		if ty, ok := mir2.BaseOf(st.Target.ExprTy()).(*mir2.StructTy); ok {
			var dest mir2.Reg
			if tgt, ok := st.Target.(*VarRefExpr); ok {
				if _, global := l.findGlobal(tgt.Name); global {
					dest = l.globalAddr(tgt.Name)
				} else {
					dest = l.env[tgt.Name]
					if dest == mir2.NoReg {
						dest = l.fixedStructStorage(ty)
						l.bind(tgt.Name, dest, tgt.Ty)
					}
				}
			} else {
				dest = l.lowerExprAddr(st.Target)
			}
			l.copyStructValueEffects(dest, val, ty, l.volatileObject(st.Val), l.volatileObject(st.Target))
			return false
		}
		switch tgt := st.Target.(type) {
		case *VarRefExpr:
			// Global variable: store through pointer.
			if g, found := l.findGlobal(tgt.Name); found {
				addr := l.globalAddr(tgt.Name)
				l.storeMemory(addr, val, g.Ty, tgt.Volatile || g.Volatile)
				return false
			}
			if l.localCells[tgt.Name] || tgt.Volatile && l.localArrays[tgt.Name] {
				l.storeMemory(l.env[tgt.Name], val, tgt.Ty, tgt.Volatile)
				return false
			}
			// Re-bind the local variable to the new vreg (SSA-style rename).
			// A goto may skip the declaration initializer and reach a full
			// assignment. The verified target still supplies its lexical type.
			l.env[tgt.Name] = l.coerceLocalInteger(val, valTy, tgt.Ty)
			l.envTy[tgt.Name] = tgt.Ty
		case *DerefExpr:
			ptr := l.lowerExpr(tgt.Ptr)
			l.storeMemory(ptr, val, tgt.Ty, tgt.Volatile || l.volatileAddress(tgt.Ptr))
		case *FieldExpr:
			// Store to struct field: compute field address then store.
			addr := l.lowerExprAddr(tgt)
			l.storeMemory(addr, val, tgt.Ty, tgt.Volatile || l.volatileAddress(tgt.X))
		case *IndexExpr:
			// arr[i] = val: compute element address then store.
			addr := l.lowerExprAddr(tgt)
			l.storeMemory(addr, val, tgt.ElemTy, tgt.Volatile || l.volatileAddress(tgt.Base))
		case *BitExpr:
			l.lowerBitAssign(tgt, st.Val)
		default:
			panic(fmt.Sprintf("hir/lower: unsupported AssignStmt target %T", st.Target))
		}
		return false

	case *ReturnStmt:
		if len(st.Vals) > 0 {
			// Multi-return: coerce each value to its contract register class so
			// PBQP allocates it to the correct physical register.  Without this,
			// all constants get [general] class and may be allocated to the same
			// physical register, causing TermRet to emit invalid parallel copies.
			regs := make([]mir2.Reg, len(st.Vals))
			for i, v := range st.Vals {
				ty := v.ExprTy()
				if i < len(l.bld.F.Contract.Returns) {
					ty = l.bld.F.Contract.Returns[i].Ty
				}
				r := l.lowerExprAs(v, ty)
				cls := classForRetPos(ty, i)
				if cls != 0 {
					r = l.bld.Move(r, ty, cls)
				}
				regs[i] = r
			}
			l.bld.Ret(regs...)
		} else if st.Val != nil {
			// Check for fold/reduce: return fold(ptr, init, cb, n)
			if foldReg, ok := l.tryLowerFold(st.Val); ok {
				chain, _ := l.recognizeIterChain(st.Val)
				if len(l.bld.F.Contract.Returns) > 0 {
					foldReg = l.coerceLocalInteger(foldReg, chain.foldAccTy, l.bld.F.Contract.Returns[0].Ty)
				}
				l.bld.Ret(foldReg)
			} else {
				r := l.lowerExprForRet(st.Val)
				// NoReg means lowerExprForRet already emitted Ret instructions
				// (e.g. CondExpr desugared into two Ret blocks).
				if r != mir2.NoReg {
					l.bld.Ret(r)
				}
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
		// Look up callee HIR params for type-directed literal coercion.
		// This handles the case where an integer literal like "10" is parsed
		// as u8 but the callee expects u16 (e.g. minmax(a: u16, b: u16)).
		var calleeParams []Param
		if callee, ok := l.hirFuncs[st.Call.Fn]; ok && callee != nil {
			calleeParams = callee.Params
		}
		args := make([]mir2.Reg, len(st.Call.Args))
		for i, a := range st.Call.Args {
			// Determine the callee's expected parameter type and class for this position.
			// Use lowerExprAs to widen integer literals to the callee's type,
			// then apply the calling-convention class coercion (same as regular CallExpr).
			paramTy := a.ExprTy()
			if i < len(calleeParams) {
				paramTy = calleeParams[i].Ty
			}
			r := l.lowerExprAs(a, paramTy)
			tgtCls := classForParam(paramTy, i)
			if tgtCls != classForExpr(paramTy) {
				r = l.bld.Move(r, paramTy, tgtCls)
			}
			args[i] = r
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
		if call, ok := st.Expr.(*CallExpr); ok {
			switch call.Fn {
			case "@error":
				if len(call.Args) != 1 {
					panic("@error requires one code")
				}
				code := l.lowerExprAs(call.Args[0], mir2.TyU8)
				l.bld.ErrorRet(code)
				return true
			case "@propagate", "@check":
				if l.errorFlag == mir2.NoReg {
					panic("error propagation requires a preceding fallible direct call")
				}
				fail, next := l.fresh("error"), l.fresh("error_ok")
				flag, code := l.errorFlag, l.errorCode
				condition := l.bld.Cmp(mir2.CmpNe, flag, l.bld.Const(0, mir2.TyBool, mir2.ClassGeneral), mir2.ClassFlag, false)
				l.bld.BrIf(condition, fail, nil, next, nil)
				l.bld.SwitchToNewBlock(fail)
				l.bld.ErrorRet(code)
				l.bld.SwitchToNewBlock(next)
				l.errorFlag, l.errorCode = mir2.NoReg, mir2.NoReg
				return false
			}
		}
		if l.tryLowerIterChain(st.Expr) {
			return false
		}
		l.lowerExpr(st.Expr)
		return false

	case *StoreStmt:
		// When the target is an IndexExpr we need the *address* (TyPtr),
		// not the loaded value.  lowerExpr(IndexExpr) would emit a Load
		// and return the element value (TyU8/etc), causing PBQP to
		// allocate an 8-bit register for the store pointer → invalid codegen.
		var ptr mir2.Reg
		if ie, ok := st.Ptr.(*IndexExpr); ok {
			ptr = l.lowerIndexAddr(ie)
		} else {
			ptr = l.lowerExpr(st.Ptr)
		}
		// ptr^ = Struct{...}: use chained stores instead of alloca round-trip.
		if lit, ok := st.Val.(*StructLitExpr); ok {
			l.emitStructLitFieldsChained(lit, ptr)
			return false
		}
		val := l.lowerExpr(st.Val)
		l.storeMemory(ptr, val, st.Val.ExprTy(), st.Volatile)
		return false

	case *AsmStmt:
		// Inline assembly block.  We currently compile only for Z80; skip blocks
		// whose target tag doesn't match.  Empty target = match any.
		if st.Target != "" && st.Target != "z80" {
			return false // wrong target — silently skip
		}
		// Compute clobber set.
		var clobbers mir2.ClassSet
		switch {
		case st.ClobberAll:
			clobbers = mir2.AllClasses
		case len(st.ClobberRegs) > 0:
			// Explicit clobber list: (clob A, F, HL)
			for _, reg := range st.ClobberRegs {
				clobbers = clobbers.Union(z80RegToClassSet(reg))
			}
		case st.ClobberAuto:
			// Auto-detect clobbers from asm text.
			clobbers = z80AutoClobbers(st.Code)
		default:
			clobbers = mir2.AllClasses
		}
		// "/" is an inline instruction separator — expand to newline.
		// Each line is stripped so the codegen's own "    " prefix applies cleanly.
		var lines []string
		for _, line := range strings.Split(st.Code, "/") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		code := strings.Join(lines, "\n    ")
		// Resolve input operands to virtual registers.
		// If no explicit (in ...) clause, auto-infer from @z80_X-annotated params.
		// Explicit inputs name in-scope values first; legacy physical aliases
		// resolve to the corresponding @z80_X-annotated parameter.
		var ins []mir2.Reg
		if len(st.Ins) == 0 {
			for _, p := range l.hf.Params {
				if p.RegClass != 0 {
					if r, ok := l.env[p.Name]; ok {
						ins = append(ins, r)
					}
				}
			}
		} else {
			for _, op := range st.Ins {
				// AsmOperand names an in-scope value. Resolve it before the
				// legacy physical-register alias syntax; unknown names must
				// never match an unannotated parameter's zero RegClass.
				if r, ok := l.env[op.Name]; ok {
					ins = append(ins, r)
					continue
				}
				wantCls := z80RegNameToClass(op.Name)
				if wantCls == 0 {
					continue
				}
				for _, p := range l.hf.Params {
					if p.RegClass == wantCls {
						if r, ok := l.env[p.Name]; ok {
							ins = append(ins, r)
							break
						}
					}
				}
			}
		}
		var outs []mir2.Reg
		for _, op := range st.Outs {
			if r, ok := l.env[op.Name]; ok {
				outs = append(outs, r)
			}
		}
		// Determine return type and class from (ret REG).
		var retTy mir2.Ty = mir2.TyVoid
		retCls := mir2.ClassAcc
		if st.RetReg != "" {
			retTy, retCls = z80RetRegInfo(st.RetReg)
		}
		l.bld.Asm(code, ins, outs, clobbers, retTy, retCls)
		l.bld.Cur.Insts[len(l.bld.Cur.Insts)-1].Asm.Effects = z80effects.Block(code)
		l.bld.F.Attrs.HasAsm = true
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
		oldEnv, oldTy, oldAlias, oldArrays := l.env, l.envTy, l.fnAliases, l.localArrays
		l.env = cloneMap(l.env)
		l.envTy = maps.Clone(l.envTy)
		l.fnAliases = maps.Clone(l.fnAliases)
		l.localArrays = maps.Clone(l.localArrays)
		for i := len(l.scopeRestores) - 1; i >= ctx.scopeDepth; i-- {
			l.scopeRestores[i]()
		}
		defer func() { l.env = oldEnv; l.envTy = oldTy; l.fnAliases = oldAlias; l.localArrays = oldArrays }()
		args := append(append([]mir2.Reg{}, ctx.exitPrefix...), l.collectArgs(ctx.mutated)...)
		l.bld.Jmp(ctx.exitLabel, args...)
		return true // break is a terminator

	case *ContinueStmt:
		// Switch contexts intercept break, but continue belongs to the
		// nearest enclosing loop and must use that loop's carried values.
		ctx := l.curLoop()
		for i := len(l.loopStack) - 1; i >= 0; i-- {
			if l.loopStack[i].headLabel != "" {
				ctx = &l.loopStack[i]
				break
			}
		}
		oldEnv, oldTy, oldAlias, oldArrays := l.env, l.envTy, l.fnAliases, l.localArrays
		l.env = cloneMap(l.env)
		l.envTy = maps.Clone(l.envTy)
		l.fnAliases = maps.Clone(l.fnAliases)
		l.localArrays = maps.Clone(l.localArrays)
		for i := len(l.scopeRestores) - 1; i >= ctx.scopeDepth; i-- {
			l.scopeRestores[i]()
		}
		defer func() { l.env = oldEnv; l.envTy = oldTy; l.fnAliases = oldAlias; l.localArrays = oldArrays }()
		if ctx.continueStep != nil {
			l.lowerStmt(ctx.continueStep)
		}
		args := l.collectArgs(ctx.mutated)
		l.bld.Jmp(ctx.headLabel, args...)
		return true // continue is a terminator

	case *SwitchStmt:
		return l.lowerSwitch(st)

	case *LabelStmt:
		label := "label_" + st.Name
		// The textual environment can belong to another case/terminated
		// path. Start from a real predecessor and merge differing bindings,
		// even when source liveness does not demand them: an enclosing HIR
		// if/loop can still carry those bindings in its own block arguments.
		incoming := make([]map[string]mir2.Reg, 0, len(l.gotoPending[st.Name])+1)
		for _, edge := range l.gotoPending[st.Name] {
			incoming = append(incoming, edge.env)
		}
		if !l.bld.Cur.IsSealed() {
			incoming = append(incoming, l.env)
		}
		labelEnv := cloneMap(l.env)
		demanded := maps.Clone(l.gotoDemands[st.Name])
		if demanded == nil {
			demanded = map[string]bool{}
		}
		if len(incoming) > 0 {
			labelEnv = cloneMap(incoming[0])
			for name, value := range labelEnv {
				different := false
				for _, edge := range incoming[1:] {
					other, present := edge[name]
					if !present {
						delete(labelEnv, name)
						different = false
						break
					}
					if other != value {
						different = true
					}
				}
				if different {
					demanded[name] = true
				}
			}
		}
		vars := sortedKeys(demanded, labelEnv)
		l.backwardGotoVars[st.Name] = vars
		if !l.bld.Cur.IsSealed() {
			l.bld.Jmp(label, l.collectArgs(vars)...)
		}
		for _, incoming := range l.gotoPending[st.Name] {
			jump := incoming.block.Term.(*mir2.TermJmp)
			for _, name := range vars {
				jump.Args = append(jump.Args, incoming.env[name])
			}
		}
		delete(l.gotoPending, st.Name)
		for _, b := range l.bld.F.Blocks {
			if b.Label == label {
				l.bld.SwitchTo(b)
				break
			}
		}
		env := labelEnv
		for _, name := range vars {
			ty := l.envTy[name]
			env[name] = l.bld.BlockParam(l.bld.Cur, ty, classForExpr(ty))
		}
		l.env = env
		return false

	case *GotoStmt:
		oldEnv, oldTy, oldAlias, oldArrays := l.env, l.envTy, l.fnAliases, l.localArrays
		l.env = cloneMap(l.env)
		l.envTy = maps.Clone(l.envTy)
		l.fnAliases = maps.Clone(l.fnAliases)
		l.localArrays = maps.Clone(l.localArrays)
		depth := 0
		ancestors := l.gotoAncestors[st.Label]
		for i, b := range l.activeBlocks {
			for _, targetBlock := range ancestors {
				if b == targetBlock {
					depth = i + 1
				}
			}
		}
		for i := len(l.scopeRestores) - 1; i >= depth; i-- {
			l.scopeRestores[i]()
		}

		defer func() { l.env = oldEnv; l.envTy = oldTy; l.fnAliases = oldAlias; l.localArrays = oldArrays }()
		target := "label_" + st.Label
		if _, known := l.backwardGotoVars[st.Label]; known {
			// Backward goto: pass current env values as block args.
			varNames := l.backwardGotoVars[st.Label]
			args := l.collectArgs(varNames)
			l.bld.Jmp(target, args...)
		} else {
			l.gotoPending[st.Label] = append(l.gotoPending[st.Label], gotoIncoming{l.bld.Cur, cloneMap(l.env)})
			l.bld.Jmp(target)
		}
		return true // goto is a terminator

	default:
		panic(fmt.Sprintf("hir/lower: unhandled statement type %T", s))
	}
}

// ── If/else lowering (two-pass) ───────────────────────────────────────────────

// lowerIf lowers an IfStmt using a two-pass approach:
//
//	Pass 1: scan for mutations in each branch.
//	Pass 2: emit MIR2 with correct Jmp args to the join block.
//
// Cases:
//
//	(a) Both branches always return → no join block, returns true.
//	(b) Neither/one branch returns → join block collects updated vars as params.
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
	if st.Else != nil {
		left, right := gotoAssignedNames(st.Then), gotoAssignedNames(st.Else)
		for name := range left {
			if (right[name] || gotoStmtTerminates(st.Else)) && l.envTy[name] != nil && envBefore[name] == mir2.NoReg {
				mutated = append(mutated, name)
			}
		}
		if gotoStmtTerminates(st.Then) {
			for name := range right {
				if l.envTy[name] != nil && envBefore[name] == mir2.NoReg && !left[name] {
					mutated = append(mutated, name)
				}
			}
		}
		sort.Strings(mutated)
	}

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

	// ── join block ──
	join := l.bld.SwitchToNewBlock(joinLabel)

	// Determine the post-join environment.
	//
	// Any live branch that reaches the join has emitted a Jmp (or BrIf
	// false-edge) with len(mutated) args.  The join block must therefore
	// have a BlockParam for every mutated variable whenever at least one
	// incoming edge carries args.
	//
	// Cases:
	//   (a) Both paths live (hasElse, !thenReturns, !elseReturns)
	//       → both supplied Jmp args, create params.
	//   (b) No else (!hasElse, !thenReturns)
	//       → then-Jmp + BrIf false-edge both supply args, create params.
	//   (c) Only then returned (thenReturns, hasElse, !elseReturns)
	//       → else-Jmp supplies args, create params.  Env = else's env
	//         projected through the params.
	//   (d) Only else returned (!thenReturns, elseReturns, hasElse)
	//       → then-Jmp supplies args, create params.  Env = then's env
	//         projected through the params.
	//   (e) Only then returned, no else (thenReturns, !hasElse)
	//       → BrIf false-edge supplies args, create params.
	joinEnv := cloneMap(envBefore)
	if len(mutated) > 0 {
		for _, name := range mutated {
			ty := l.envTy[name]
			r := l.bld.BlockParam(join, ty, classForExpr(ty))
			joinEnv[name] = r
		}
	}

	_ = envAfterThen
	_ = envAfterElse
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
	l.lowerWhileWithContinue(st, nil)
}

// A desugared range loop must run its increment even when the body continues.
func (l *lowerer) lowerWhileWithContinue(st *WhileStmt, continueStep Stmt) {
	fact := l.staticLoopFact(st)
	if !l.disableCountedLoops && l.tryCountedLoop(st, fact) {
		return
	}
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
	l.recordStaticLoop(headLabel, fact)
	bodyLabel := l.fresh("loop_body")
	exitLabel := l.fresh("loop_exit")

	// Jump to loop head with initial values.
	initArgs := l.collectArgs(mutated)
	l.bld.Jmp(headLabel, initArgs...)

	// ── loop_head ──
	head := l.bld.SwitchToNewBlock(headLabel)
	if source, ok := l.lineage[st]; ok {
		if l.mf.LoopOrigins == nil {
			l.mf.LoopOrigins = map[string]mir2.LoopOrigin{}
		}
		l.mf.LoopOrigins[bodyLabel] = mir2.LoopOrigin{SiteID: source.SiteID, Role: source.Role, Rule: source.Rule}
	}
	headEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(head, ty, classForLoop(ty, name))
		headEnv[name] = r
	}
	l.env = headEnv

	prevInLoopCtx := l.inLoopContext
	l.inLoopContext = true
	cond := l.lowerCond(st.Cond)
	// True → body (no block params on body, no args needed).
	// False → exit (with block params, pass current head-env values).
	exitArgs := l.collectArgs(mutated)
	l.bld.BrIf(cond, bodyLabel, nil, exitLabel, exitArgs)

	// ── loop_body ──
	l.bld.SwitchToNewBlock(bodyLabel)
	l.env = cloneMap(headEnv) // body directly uses head's vregs
	l.pushLoop(loopCtx{headLabel: headLabel, exitLabel: exitLabel, mutated: mutated, continueStep: continueStep})
	terminated := l.lowerBlock(st.Body)
	l.popLoop()
	l.inLoopContext = prevInLoopCtx
	// A return, break or continue already sealed the current body block.
	if !terminated {
		backArgs := l.collectArgs(mutated)
		l.bld.Jmp(headLabel, backArgs...)
	}

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

// saveLexicalName restores one loop binder without discarding outer mutations.
func (l *lowerer) saveLexicalName(name string) func() {
	r, rok := l.env[name]
	ty, tok := l.envTy[name]
	a, aok := l.fnAliases[name]
	arr := l.localArrays[name]
	return func() {
		if rok {
			l.env[name] = r
		} else {
			delete(l.env, name)
		}
		if tok {
			l.envTy[name] = ty
		} else {
			delete(l.envTy, name)
		}
		if aok {
			l.fnAliases[name] = a
		} else {
			delete(l.fnAliases, name)
		}
		if arr {
			l.localArrays[name] = true
		} else {
			delete(l.localArrays, name)
		}
	}
}

// lowerForRange desugars a ForRangeStmt to a while-equivalent.
// for v in start..end → v = start; hi = end; while v < hi { body; v++ }
//
// The upper bound is evaluated ONCE before the loop and bound to a synthetic
// variable __for_hi_N.  This prevents expressions like (n+1) from being
// re-evaluated as an ADD inside the loop header on every iteration, which
// would force 'n' into the A register as the ADD lhs and corrupt it via
// the body's scratch uses of A (loads, mul16 counter, etc.).
func (l *lowerer) lowerForRange(st *ForRangeStmt) {
	defer l.saveLexicalName(st.Var)()
	ty := RangeType(st.Start, st.End)
	initReg := l.lowerExprAs(st.Start, ty)

	// Both SSA values must have the binding's width, including a byte start
	// with a word end. Merely recording envTy does not widen the register.
	// Precompute the loop upper bound once, outside the loop.
	endReg := l.lowerExprAs(st.End, ty)
	l.bind(st.Var, initReg, ty)
	endName := l.fresh("__for_hi_")
	defer l.saveLexicalName(endName)()
	l.bind(endName, endReg, ty)

	// Synthesise a WhileStmt: while v < hi { body; v = v + 1 }
	one := &IntLitExpr{Val: 1, Ty: ty}
	increment := &AssignStmt{
		Target: &VarRefExpr{Name: st.Var, Ty: ty},
		Val:    &BinExpr{Op: "+", L: &VarRefExpr{Name: st.Var, Ty: ty}, R: one, Ty: ty},
	}
	// Copy the slice: lowering must not overwrite the source HIR backing array.
	whileBody := &Block{Body: []Stmt{st.Body, increment}}
	synthetic := &WhileStmt{
		Cond: &BinExpr{Op: "<", L: &VarRefExpr{Name: st.Var, Ty: ty}, R: &VarRefExpr{Name: endName, Ty: ty}, Ty: mir2.TyBool},
		Body: whileBody,
	}
	if origin, ok := l.lineage[st]; ok {
		l.lineage[synthetic] = origin
	}
	l.lowerWhileWithContinue(synthetic, increment)
}

// lowerForEach lowers a ForEachStmt.
//
// Byte loops without control transfers use an initial zero-trip check and
// a DJNZ back-edge, whose first target parameter receives the decremented
// count. The counter class prefers B; codegen handles its actual allocation.
// Wide loops and bodies with control transfers retain the structure below:
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
	defer l.saveLexicalName(st.Var)()
	envBefore := cloneMap(l.env)
	needsTrampoline := false
	walkGraceBlocks(st.Body, "", func(_ string, b *Block) {
		for _, s := range b.Body {
			switch s.(type) {
			case *ContinueStmt, *BreakStmt, *ReturnStmt, *GotoStmt, *LabelStmt:
				needsTrampoline = true
			}
		}
	})
	stride := int64(mir2.ByteWidth(st.ElemTy))
	if st.ElemStride > 0 {
		stride = int64(st.ElemStride)
	}
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
	cntTy := st.Len.ExprTy()
	if cntTy == mir2.TyU8 && !needsTrampoline {
		l.lowerForEachByteCountdown(st, envBefore, ptrReg, cntReg, stride)
		return
	}

	cntClass := mir2.ClassCounter
	zeroClass := mir2.ClassAcc
	if cntTy.Width() > mir2.TyU8.Width() {
		// ClassCounter targets the byte register B. Wide lengths need a pair,
		// preferably DE so the element pointer can remain in HL.
		cntClass = mir2.ClassIndex
		zeroClass = cntClass
	}

	// Literal operands are loop-invariant for the general control-flow path.
	zero := l.bld.Const(0, cntTy, zeroClass)
	one := l.bld.Const(1, cntTy, cntClass)

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
	if source, ok := l.lineage[st]; ok {
		if l.mf.LoopOrigins == nil {
			l.mf.LoopOrigins = map[string]mir2.LoopOrigin{}
		}
		l.mf.LoopOrigins[bodyLabel] = mir2.LoopOrigin{SiteID: source.SiteID, Role: source.Role, Rule: source.Rule}
	}
	headPtr := l.bld.BlockParam(head, l.m.PointerType(), mir2.ClassPointer) // HL
	headCnt := l.bld.BlockParam(head, cntTy, cntClass)
	headEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(head, ty, classForLoop(ty, name))
		headEnv[name] = r
	}
	l.env = headEnv

	// brif (cnt != 0) → body, else → exit
	cond := l.bld.Cmp(mir2.CmpNe, headCnt, zero, mir2.ClassFlag, false)
	exitArgs := append([]mir2.Reg{headPtr, headCnt}, l.collectArgs(mutated)...)
	l.bld.BrIf(cond, bodyLabel, nil, exitLabel, exitArgs)

	// ── fe_body ──
	l.bld.SwitchToNewBlock(bodyLabel)
	l.env = cloneMap(headEnv)

	// Load element: x = *ptr (or x = ptr for struct pointer iteration)
	if st.PtrIter {
		// Pointer iteration: x IS the pointer, not a loaded value
		l.bind(st.Var, headPtr, l.m.PointerType())
	} else {
		xReg := l.bld.Load(headPtr, st.ElemTy, classForExpr(st.ElemTy))
		l.bind(st.Var, xReg, st.ElemTy)
	}

	// ContinueStmt jumps to fe_cont (not fe_head) so ptr/cnt are advanced first.
	prevInLoopCtx2 := l.inLoopContext
	l.inLoopContext = true
	l.pushLoop(loopCtx{headLabel: contLabel, exitLabel: exitLabel, mutated: mutated, exitPrefix: []mir2.Reg{headPtr, headCnt}})
	terminated := l.lowerBlock(st.Body)
	l.popLoop()
	l.inLoopContext = prevInLoopCtx2

	// mapInPlace: store the (possibly transformed) element value back to ptr.
	if st.MutateInPlace && !terminated {
		if finalVal, ok := l.env[st.Var]; ok {
			l.bld.Store(headPtr, finalVal, st.ElemTy)
		}
	}

	// Continue needs an increment trampoline; retain it for other control
	// transfers too. Ordinary fall-through bodies increment directly.
	if needsTrampoline {
		// Only fall-through reaches the trampoline through this edge.
		if !terminated {
			l.bld.Jmp(contLabel, l.collectArgs(mutated)...)
		}

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
	}

	ptrNext := l.bld.PtrBump(headPtr, stride, mir2.ClassPointer)
	cntNext := l.bld.Sub(headCnt, one, cntTy, cntClass)

	backArgs := append([]mir2.Reg{ptrNext, cntNext}, l.collectArgs(mutated)...)
	l.bld.Jmp(headLabel, backArgs...)

	// ── fe_exit(ptr, cnt, mutated...) ──
	exit := l.bld.SwitchToNewBlock(exitLabel)
	l.bld.BlockParam(exit, l.m.PointerType(), mir2.ClassPointer) // discard ptr
	l.bld.BlockParam(exit, cntTy, cntClass)                      // discard cnt
	exitEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(exit, ty, classForExpr(ty))
		exitEnv[name] = r
	}
	// Element var goes out of scope.
	l.env = exitEnv
}

// Byte fall-through bodies need only one zero-trip check. DJNZ decrements
// the counter on the back-edge, so no zero/one values remain live in the body.
// Bodies with control transfers retain the general trampoline implementation.
func (l *lowerer) lowerForEachByteCountdown(st *ForEachStmt, envBefore map[string]mir2.Reg, ptrReg, cntReg mir2.Reg, stride int64) {
	allUsed := map[string]bool{}
	scanUsedInBlock(st.Body, envBefore, allUsed)
	for name := range scanMutations(st.Body, envBefore) {
		allUsed[name] = true
	}
	delete(allUsed, st.Var)
	mutated := sortedKeys(allUsed, envBefore)
	bodyLabel, exitLabel := l.fresh("fe_body"), l.fresh("fe_exit")
	zero := l.bld.Const(0, mir2.TyU8, mir2.ClassAcc)
	nonzero := l.bld.Cmp(mir2.CmpNe, cntReg, zero, mir2.ClassFlag, false)
	initial := append([]mir2.Reg{cntReg, ptrReg}, l.collectArgs(mutated)...)
	l.bld.BrIf(nonzero, bodyLabel, initial, exitLabel, l.collectArgs(mutated))
	body := l.bld.SwitchToNewBlock(bodyLabel)
	if origin, ok := l.lineage[st]; ok {
		if l.mf.LoopOrigins == nil {
			l.mf.LoopOrigins = map[string]mir2.LoopOrigin{}
		}
		l.mf.LoopOrigins[bodyLabel] = mir2.LoopOrigin{SiteID: origin.SiteID, Role: origin.Role, Rule: origin.Rule}
	}
	cnt := l.bld.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
	ptr := l.bld.BlockParam(body, l.m.PointerType(), mir2.ClassPointer)
	env := cloneMap(envBefore)
	for _, name := range mutated {
		env[name] = l.bld.BlockParam(body, l.envTy[name], classForLoop(l.envTy[name], name))
	}
	l.env = env
	if st.PtrIter {
		l.bind(st.Var, ptr, l.m.PointerType())
	} else {
		l.bind(st.Var, l.bld.Load(ptr, st.ElemTy, classForExpr(st.ElemTy)), st.ElemTy)
	}
	previous := l.inLoopContext
	l.inLoopContext = true
	l.lowerBlock(st.Body)
	l.inLoopContext = previous
	if st.MutateInPlace {
		if final, ok := l.env[st.Var]; ok {
			l.bld.Store(ptr, final, st.ElemTy)
		}
	}
	ptrNext := l.bld.PtrBump(ptr, stride, mir2.ClassPointer)
	updates := l.collectArgs(mutated)
	back := append([]mir2.Reg{ptrNext}, updates...)
	l.bld.DJNZ(cnt, bodyLabel, back, exitLabel, updates)
	exit := l.bld.SwitchToNewBlock(exitLabel)
	env = cloneMap(envBefore)
	for _, name := range mutated {
		env[name] = l.bld.BlockParam(exit, l.envTy[name], classForExpr(l.envTy[name]))
	}
	l.env = env
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
func (l *lowerer) lowerSwitch(st *SwitchStmt) bool {
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
		falseArgs := []mir2.Reg(nil)
		if nextLabels[i] == joinLabel {
			falseArgs = l.collectArgs(mutated)
		}
		l.bld.BrIf(cond, caseLabels[i], nil, nextLabels[i], falseArgs)
		if i+1 < len(st.Cases) {
			l.bld.SwitchToNewBlock(nextLabels[i])
		}
	}

	// An empty dispatch still transfers to the default (or directly to join).
	if len(st.Cases) == 0 {
		var args []mir2.Reg
		if defaultLabel == joinLabel {
			args = l.collectArgs(mutated)
		}
		l.bld.Jmp(defaultLabel, args...)
	}

	// Push switch as a "loop" context so that break inside case bodies
	// jumps to joinLabel (C semantics: break exits the switch).
	l.pushLoop(loopCtx{headLabel: "", exitLabel: joinLabel, mutated: mutated})

	// Emit case bodies.
	for i, c := range st.Cases {
		l.bld.SwitchToNewBlock(caseLabels[i])
		l.env = cloneMap(envBefore)
		done := l.lowerBlock(c.Body)
		if !done {
			l.bld.Jmp(joinLabel, l.collectArgs(mutated)...)
		}
	}

	// Default body.
	if st.Default != nil {
		l.bld.SwitchToNewBlock(defaultLabel)
		l.env = cloneMap(envBefore)
		done := l.lowerBlock(st.Default)
		if !done {
			l.bld.Jmp(joinLabel, l.collectArgs(mutated)...)
		}
	}

	l.popLoop()

	// Count actual incoming edges: unmatched dispatch, case/default
	// fallthrough and break all reach the join. A terminating body alone
	// does not tell us whether it returned or broke out of this switch.
	joinReachable := false
	for _, block := range l.bld.F.Blocks {
		for _, target := range block.Successors() {
			if target == joinLabel {
				joinReachable = true
			}
		}
	}
	join := l.bld.SwitchToNewBlock(joinLabel)
	if !joinReachable {
		// All paths returned → join is dead; emit Unreachable so Verify passes.
		l.bld.Unreachable()
		l.env = cloneMap(envBefore)
		return true
	}
	joinEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		r := l.bld.BlockParam(join, ty, classForExpr(ty))
		joinEnv[name] = r
	}
	_ = join
	l.env = joinEnv
	return false
}

// ── Expression lowering ───────────────────────────────────────────────────────

// coerceLocalInteger respects the declared scalar binding after evaluating its
// source once. HIR accepts implicit integer width conversion at declarations
// and assignments; the SSA register must carry that converted value too.
func (l *lowerer) coerceLocalInteger(src mir2.Reg, from, to mir2.Ty) mir2.Reg {
	if src == mir2.NoReg || !mir2.IsInt(from) || !mir2.IsInt(to) {
		return src
	}
	if from.Width() > to.Width() {
		return l.bld.Trunc(src, from, to, classForExpr(to))
	}
	if from.Width() < to.Width() {
		if mir2.IsSigned(from) {
			return l.bld.Sext(src, from, to, classForExpr(to))
		}
		return l.bld.Ext(src, from, to, classForExpr(to))
	}
	return src
}

// lowerStorageExprAs makes narrowing and signed widening explicit at storage
// and CFG boundaries. Nonnegative literals and unsigned widening may keep their
// smaller registers: the store/join already supplies the destination width.
func (l *lowerer) lowerStorageExprAs(e Expr, wantTy mir2.Ty) mir2.Reg {
	if lit, ok := e.(*IntLitExpr); ok && lit.Val >= 0 && IntLiteralFits(lit.Val, wantTy) {
		return l.lowerExpr(e)
	}
	if mir2.IsInt(e.ExprTy()) && mir2.IsInt(wantTy) && (e.ExprTy().Width() > wantTy.Width() || mir2.IsSigned(e.ExprTy()) && e.ExprTy().Width() < wantTy.Width()) {
		return l.lowerExprAs(e, wantTy)
	}
	return l.lowerExpr(e)
}

// lowerExprAs converts to the destination integer width before ABI placement
// or use. Narrowing keeps low bits; widening follows the source signedness.
func (l *lowerer) lowerExprAs(e Expr, wantTy mir2.Ty) mir2.Reg {
	if lit, ok := e.(*IntLitExpr); ok {
		// A source literal has an exact mathematical value. Fold its contextual
		// integer conversion here, including narrowing, before ABI placement or
		// inlining can substitute a register with the literal's default width.
		if lit.Untyped && mir2.IsInt(wantTy) || lit.Ty != wantTy && lit.Ty.Width() < wantTy.Width() {
			return l.bld.Const(literalValueAs(lit, wantTy), wantTy, classForExpr(wantTy))
		}
	}
	src := l.lowerExpr(e)
	return l.coerceLocalInteger(src, e.ExprTy(), wantTy)
}

// lowerExpr lowers an expression to the register holding its value.
func (l *lowerer) lowerExpr(e Expr) mir2.Reg {
	switch ex := e.(type) {

	case *IntLitExpr:
		if ex.Ty == l.m.PointerType() && l.literalStorePointers != nil {
			if reg, ok := l.literalStorePointers[ex.Val]; ok {
				return reg
			}
			reg := l.bld.Const(ex.Val, ex.Ty, classForExpr(ex.Ty))
			l.literalStorePointers[ex.Val] = reg
			return reg
		}
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
				if _, isArray := g.Ty.(*mir2.ArrayTy); isArray {
					// Array global: return the address (pointer to first element).
					// IndexExpr uses this address as base for ptr_add.
					return addr
				}
				// Scalar global: load and return the value.
				return l.loadMemory(addr, g.Ty, classForExpr(g.Ty), ex.Volatile || g.Volatile)
			}
			panic(fmt.Sprintf("hir/lower: undefined variable %q", ex.Name))
		}
		if l.localCells[ex.Name] || ex.Volatile && l.localArrays[ex.Name] {
			return l.loadMemory(r, ex.Ty, classForExpr(ex.Ty), ex.Volatile)
		}
		return r

	case *BinExpr:
		return l.lowerBinExpr(ex)

	case *UnaryExpr:
		return l.lowerUnaryExpr(ex)

	case *CallExpr:
		if ex.Fn == "@mir.io.print.sstr" || ex.Fn == "@mir.io.print.lstr" {
			return l.lowerCountedStringPrint(ex)
		}
		// @target() intrinsic — returns compile-time platform constant
		if ex.Fn == "@target" {
			return l.bld.Const(int64(l.target), mir2.TyU8, mir2.ClassGeneral)
		}

		// Resolve function name: may be a direct name, a local alias (let f = |x| ...), or
		// a module-level function reference.
		fnName := ex.Fn
		if alias, ok := l.fnAliases[fnName]; ok {
			fnName = alias
		}

		var calleeParams []Param
		smcCall := false
		if callee := l.hirFuncs[fnName]; callee != nil {
			calleeParams = smcCallParams(callee, len(ex.Args))
			smcCall = hasSMCParams(callee)
		}
		args := make([]mir2.Reg, len(ex.Args))
		for i, a := range ex.Args {
			// A struct expression is represented by its address when passed to
			// a concrete method. Its storage width is not the argument width.
			paramTy := a.ExprTy()
			if _, isStruct := a.ExprTy().(*mir2.StructTy); isStruct && i < len(calleeParams) && mir2.IsPointer(calleeParams[i].Ty) {
				paramTy = calleeParams[i].Ty
			}
			var r mir2.Reg
			// Convert integer arguments before ABI placement and inlining.
			if i < len(calleeParams) && mir2.IsInt(paramTy) && mir2.IsInt(calleeParams[i].Ty) {
				paramTy = calleeParams[i].Ty
				r = l.lowerExprAs(a, paramTy)
			} else {
				r = l.lowerExpr(a)
			}
			if !smcCall {
				tgtCls := classForParam(paramTy, i)
				if tgtCls != classForExpr(paramTy) {
					r = l.bld.Move(r, paramTy, tgtCls)
				}
			}
			args[i] = r
		}
		// Evaluate every source argument before changing the callee's state.
		// A nested argument may itself invoke the same SMC function.
		if callee := l.hirFuncs[fnName]; callee != nil && hasSMCParams(callee) && len(ex.Args) == len(callee.Params) {
			var runtime []mir2.Reg
			for i, p := range callee.Params {
				if p.SMC {
					l.bld.CallVoid(mir2.SMCSetterName(fnName+"$"+p.Name), []mir2.Reg{args[i]}, mir2.CallAttrs{})
				} else {
					runtime = append(runtime, args[i])
				}
			}
			args = runtime
			calleeParams = smcCallParams(callee, len(args))
		}
		for i, r := range args {
			if !smcCall {
				break
			}
			paramTy := ex.Args[i].ExprTy()
			if i < len(calleeParams) {
				paramTy = calleeParams[i].Ty
			}
			tgtCls := classForParam(paramTy, i)
			if tgtCls != classForExpr(paramTy) {
				args[i] = l.bld.Move(r, paramTy, tgtCls)
			}
		}

		if callee := l.hirFuncs[fnName]; callee != nil && callee.FallibleCY {
			if l.errorFlag != mir2.NoReg {
				panic("fallible call requires checking preceding outcome")
			}
			var value mir2.Reg
			value, l.errorFlag, l.errorCode = l.bld.CallFallible(fnName, args, ex.Ty, classForRet(ex.Ty))
			return value
		}
		if ex.Ty == mir2.TyVoid {
			l.bld.CallVoid(fnName, args, mir2.CallAttrs{})
			return mir2.NoReg
		}
		cls := classForRet(ex.Ty)
		return l.bld.Call(fnName, args, ex.Ty, cls, mir2.CallAttrs{})

	case *CallIndirectExpr:
		// A local name bound to a known lambda/partial application has a
		// compile-time target. Call it directly; no runtime closure is needed.
		if vr, ok := ex.FnPtr.(*VarRefExpr); ok {
			if fnName, found := l.fnAliases[vr.Name]; found {
				return l.lowerExpr(&CallExpr{Fn: fnName, Args: ex.Args, Ty: ex.Ty})
			}
		}
		fnPtrReg := l.lowerExpr(ex.FnPtr)
		args := make([]mir2.Reg, len(ex.Args))
		for i, a := range ex.Args {
			paramTy := a.ExprTy()
			if ex.Signature != nil && i < len(ex.Signature.Params) {
				paramTy = ex.Signature.Params[i]
			}
			r := l.lowerExprAs(a, paramTy)
			tgtCls := classForParam(paramTy, i)
			if tgtCls != classForExpr(paramTy) {
				r = l.bld.Move(r, paramTy, tgtCls)
			}
			args[i] = r
		}
		cls := classForRet(ex.Ty)
		r := l.bld.CallIndirect(fnPtrReg, args, ex.Ty, cls)
		inst := l.bld.Cur.Insts[len(l.bld.Cur.Insts)-1]
		if ex.Signature != nil {
			inst.IndirectParamTys = append([]mir2.Ty(nil), ex.Signature.Params...)
			inst.IndirectReturnTys = append([]mir2.Ty(nil), ex.Signature.Returns...)
		} else {
			for _, arg := range ex.Args {
				inst.IndirectParamTys = append(inst.IndirectParamTys, arg.ExprTy())
			}
			if ex.Ty != mir2.TyVoid {
				inst.IndirectReturnTys = []mir2.Ty{ex.Ty}
			}
		}
		mir2.CloneCallABIMetadata(inst, inst)
		return r

	case *AddrOfExpr:
		if ex.X != nil {
			r := l.lowerExprAddr(ex.X)
			if l.target == TargetMZV {
				// Host-global addresses narrow to the selected language pointer ABI.
				return l.bld.Trunc(r, mir2.PtrFor(32), l.m.PointerType(), mir2.ClassPointer)
			}
			return r
		}
		// Local array promoted to global: env register IS the address.
		if l.localArrays[ex.Sym] || l.localCells[ex.Sym] {
			if reg, ok := l.env[ex.Sym]; ok {
				return reg
			}
		}
		// Local structs already have addressable storage in the environment.
		// Their names are not linker symbols.
		if ty, ok := l.envTy[ex.Sym]; ok {
			if _, isStruct := mir2.BaseOf(ty).(*mir2.StructTy); isStruct {
				if reg, found := l.env[ex.Sym]; found {
					return reg
				}
			}
		}
		return l.bld.AddrOf(ex.Sym, mir2.ClassPointer)

	case *ConstPtrExpr:
		address := int64(ex.Addr)
		if l.literalStorePointers != nil {
			if reg, ok := l.literalStorePointers[address]; ok {
				return reg
			}
			reg := l.bld.Const(address, l.m.PointerType(), mir2.ClassPointer)
			l.literalStorePointers[address] = reg
			return reg
		}
		return l.bld.Const(address, l.m.PointerType(), mir2.ClassPointer)

	case *DerefExpr:
		ptr := l.lowerExpr(ex.Ptr)
		if _, aggregate := mir2.BaseOf(ex.Ty).(*mir2.StructTy); aggregate {
			return ptr
		}
		return l.loadMemory(ptr, ex.Ty, classForExpr(ex.Ty), ex.Volatile || l.volatileAddress(ex.Ptr))

	case *LoadExpr:
		ptr := l.lowerExpr(ex.Ptr)
		cls := classForExpr(ex.Ty)
		return l.loadMemory(ptr, ex.Ty, cls, ex.Volatile || l.volatileAddress(ex.Ptr))

	case *FieldExpr:
		base := l.lowerExpr(ex.X)
		// Advance pointer by field offset.
		if ex.Offset > 0 {
			base = l.bld.Field(base, int64(ex.Offset), mir2.ClassPointer)
		}
		// Embedded struct field: return address, don't load.
		// The outer FieldExpr/IndexExpr will do the final scalar Load.
		switch mir2.BaseOf(ex.Ty).(type) {
		case *mir2.StructTy, *mir2.ArrayTy:
			return base
		}
		return l.loadMemory(base, ex.Ty, classForExpr(ex.Ty), ex.Volatile || l.volatileAddress(ex.X))

	case *BitExpr:
		return l.lowerBitRead(ex)

	case *CastExpr:
		src := l.lowerExpr(ex.X)
		srcTy := ex.X.ExprTy()
		dstTy := ex.Ty
		sw, dw := srcTy.Width(), dstTy.Width()
		switch {
		case dw > sw:
			if mir2.IsSigned(srcTy) {
				return l.bld.Sext(src, srcTy, dstTy, classForExpr(dstTy))
			}
			return l.bld.Ext(src, srcTy, dstTy, classForExpr(dstTy))
		case dw < sw:
			return l.bld.Trunc(src, srcTy, dstTy, classForExpr(dstTy))
		default:
			return src // same width → no-op
		}

	case *IndexExpr:
		return l.lowerIndex(ex)

	case *StructLitExpr:
		return l.lowerStructLitAlloca(ex)

	case *CondExpr:
		return l.lowerCondExpr(ex)

	case *LetInExpr:
		// Lower init, bind the result to the variable name, then lower body.
		initReg := l.lowerExpr(ex.Init)
		savedReg, hadReg := l.env[ex.Name]
		savedTy, hadTy := l.envTy[ex.Name]
		savedAlias, hadAlias := l.fnAliases[ex.Name]
		savedArray, hadArray := l.localArrays[ex.Name]
		l.bind(ex.Name, l.coerceLocalInteger(initReg, ex.Init.ExprTy(), ex.Ty), ex.Ty)
		result := l.lowerExpr(ex.Body)
		if hadReg {
			l.env[ex.Name] = savedReg
		} else {
			delete(l.env, ex.Name)
		}
		if hadTy {
			l.envTy[ex.Name] = savedTy
		} else {
			delete(l.envTy, ex.Name)
		}
		if hadAlias {
			l.fnAliases[ex.Name] = savedAlias
		} else {
			delete(l.fnAliases, ex.Name)
		}
		if hadArray {
			l.localArrays[ex.Name] = savedArray
		} else {
			delete(l.localArrays, ex.Name)
		}
		return result

	default:
		panic(fmt.Sprintf("hir/lower: unhandled expression type %T", e))
	}
}

// Only 0..3-byte literal displacements use the native INC-pair path.
// Larger offsets retain PtrAdd; Field's special SoA offsets are not indexing.
func (l *lowerer) smallSMCIndexOffset(ex *IndexExpr) (int64, bool) {
	// Ordinary pointers retain the PtrAdd graph consumed by sequential-byte
	// fusion. This first specialization applies only to the named SMC base.
	ref, ok := ex.Base.(*VarRefExpr)
	if !ok {
		return 0, false
	}
	smc := false
	for _, p := range l.hf.Params {
		if p.SMC && p.Name == ref.Name {
			smc = true
			break
		}
	}
	if !smc {
		return 0, false
	}
	index, ok := ex.Idx.(*IntLitExpr)
	if !ok || index.Val < 0 || index.Val > 3 || (index.Ty != mir2.TyU8 && index.Ty != mir2.TyU16) {
		return 0, false
	}
	stride := ex.ElemStride
	if stride == 0 {
		stride = (ex.ElemTy.Width() + 7) / 8
	}
	if stride <= 0 || stride > 3 {
		return 0, false
	}
	offset := index.Val * int64(stride)
	return offset, offset <= 3
}

// lowerIndexAddr lowers arr[i] → ptr_add(base, i * stride), returning the
// pointer register (TyPtr).  Used by StoreStmt which needs the address, not
// the loaded value.
//
// stride = ElemStride if set; otherwise ElemTy.Width()/8 (rounded up to 1).
func (l *lowerer) lowerIndexAddr(ex *IndexExpr) mir2.Reg {
	base := l.lowerExpr(ex.Base)

	stride := ex.ElemStride
	if stride == 0 {
		w := ex.ElemTy.Width()
		stride = (w + 7) / 8
		if stride == 0 {
			stride = 1
		}
	}
	if l.m.PointerType().Width() == 16 && l.target != TargetMZV && l.target != TargetAgon {
		if offset, ok := l.smallSMCIndexOffset(ex); ok {
			if offset == 0 {
				return base
			}
			return l.bld.Field(base, offset, mir2.ClassPointer)
		}
	}
	// A literal global-array index is already a compile-time byte displacement.
	// Keep it immediate so the native backend can use direct global addressing,
	// rather than materializing an index register and doing runtime PtrAdd.
	// The 16-bit target wraps the displacement modulo 65536. Host globals have
	// wider addresses; signed/non-normalized literals retain ordinary lowering.
	// Runtime pointer bases retain PtrAdd, including the sequential-byte fusion
	// DAG; immediate Field alone is not cheaper for those native addresses.
	globalArray := false
	if ref, ok := ex.Base.(*VarRefExpr); ok {
		if global, found := l.findGlobal(ref.Name); found {
			_, globalArray = mir2.BaseOf(global.Ty).(*mir2.ArrayTy)
		}
	}
	if globalArray && l.m.PointerType().Width() == 16 && l.target != TargetMZV && l.target != TargetAgon && stride > 0 {
		if literal, ok := ex.Idx.(*IntLitExpr); ok {
			if (literal.Ty == mir2.TyU8 && literal.Val >= 0 && literal.Val <= 255) ||
				(literal.Ty == mir2.TyU16 && literal.Val >= 0 && literal.Val <= 65535) {
				offset := int64((uint64(literal.Val) * (uint64(stride) & 0xffff)) & 0xffff)
				return l.bld.Field(base, offset, mir2.ClassPointer)
			}
		}
	}
	idx := l.lowerExpr(ex.Idx)

	var offset mir2.Reg
	idxTy := ex.Idx.ExprTy()
	addressTy := mir2.Ty(mir2.TyU16)
	if l.m.PointerBits == 24 {
		addressTy = mir2.TyU24
	}
	hostGlobal := false
	if l.target == TargetMZV {
		if ref, ok := ex.Base.(*VarRefExpr); ok {
			if g, found := l.findGlobal(ref.Name); found {
				_, hostGlobal = mir2.BaseOf(g.Ty).(*mir2.ArrayTy)
			}
		}
		if hostGlobal {
			addressTy = mir2.TyU32
		}
	}
	switch stride {
	case 1:
		if idxTy.Width() < addressTy.Width() {
			offset = l.bld.Ext(idx, idxTy, addressTy, mir2.ClassIndex)
		} else {
			offset = idx
		}
	case 2:
		if idxTy.Width() < addressTy.Width() {
			idx = l.bld.Ext(idx, idxTy, addressTy, mir2.ClassIndex)
		}
		one := l.bld.Const(1, addressTy, mir2.ClassGeneral)
		offset = l.bld.Shl(idx, one, addressTy, mir2.ClassIndex)
	default:
		if idxTy.Width() < addressTy.Width() {
			idx = l.bld.Ext(idx, idxTy, addressTy, mir2.ClassIndex)
		}
		strideReg := l.bld.Const(int64(stride), addressTy, mir2.ClassGeneral)
		offset = l.bld.Mul(idx, strideReg, addressTy, mir2.ClassIndex)
	}

	r := l.bld.PtrAdd(base, offset, mir2.ClassPointer)
	if hostGlobal {
		l.bld.Cur.Insts[len(l.bld.Cur.Insts)-1].Ty = mir2.PtrFor(32)
	}
	return r
}

// lowerIndex lowers arr[i] → ptr_add(base, i * stride) → load.
func (l *lowerer) lowerIndex(ex *IndexExpr) mir2.Reg {
	ptr := l.lowerIndexAddr(ex)
	// Struct element: return address, don't load.
	// The outer FieldExpr will do the final scalar Load.
	switch mir2.BaseOf(ex.ElemTy).(type) {
	case *mir2.StructTy, *mir2.ArrayTy:
		return ptr
	}
	return l.loadMemory(ptr, ex.ElemTy, classForExpr(ex.ElemTy), ex.Volatile || l.volatileAddress(ex.Base))
}

// lowerCondExpr lowers: if Cond { Then } else { Else }  →  result register.
// Emits a BrIf to a then-block and an else-block, both converging at a join
// block with a block parameter that carries the result value.
func (l *lowerer) lowerCondExpr(ex *CondExpr) mir2.Reg {
	cond := l.lowerCond(ex.Cond)
	cls := classForExpr(ex.Ty)

	thenLabel := l.fresh("cond_then")
	elseLabel := l.fresh("cond_else")
	joinLabel := l.fresh("cond_join")

	l.bld.BrIf(cond, thenLabel, nil, elseLabel, nil)

	// then-branch
	thenBlk := l.bld.SwitchToNewBlock(thenLabel)
	_ = thenBlk
	thenVal := l.lowerStorageExprAs(ex.Then, ex.Ty)
	if ex.Ty == mir2.TyVoid {
		l.bld.Jmp(joinLabel)
	} else {
		l.bld.Jmp(joinLabel, thenVal)
	}

	// else-branch
	elseBlk := l.bld.SwitchToNewBlock(elseLabel)
	_ = elseBlk
	elseVal := l.lowerStorageExprAs(ex.Else, ex.Ty)
	if ex.Ty == mir2.TyVoid {
		l.bld.Jmp(joinLabel)
	} else {
		l.bld.Jmp(joinLabel, elseVal)
	}

	// join block: a block parameter receives the value from whichever branch
	joinBlk := l.bld.SwitchToNewBlock(joinLabel)
	if ex.Ty == mir2.TyVoid {
		return mir2.NoReg
	}
	result := l.bld.BlockParam(joinBlk, ex.Ty, cls)
	return result
}

// lowerBinExpr lowers a binary expression.
func (l *lowerer) lowerBinExpr(ex *BinExpr) mir2.Reg {
	// Comparison operators → produce a flag (ClassFlag).
	switch ex.Op {
	case "==", "!=", "<", "<=", ">", ">=":
		return l.lowerCond(ex)
	case "&&", "||":
		return l.lowerLogical(ex)
	}

	ty := ex.Ty
	if common := LiteralPeerType(ex.L, ex.R); common != nil {
		ty = common
	}
	// Use ClassAcc for 8-bit ALU results outside loop contexts — Z80 ALU ops
	// always write to A, avoiding a spurious LD C,A.  Inside loops (head
	// condition or body) values may be loop-carried or live across ALU ops
	// that transiently clobber A, so fall back to ClassGeneral.
	var cls mir2.RegClass
	if !l.inLoopContext {
		cls = classForALUResult(ex.Op, ty)
	} else {
		cls = classForExpr(ty)
	}
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
		if mir2.IsSigned(ty) {
			return l.bld.SDiv(lReg, rReg, ty, cls)
		}
		return l.bld.Div(lReg, rReg, ty, cls)
	case "%":
		if mir2.IsSigned(ty) {
			return l.bld.SMod(lReg, rReg, ty, cls)
		}
		return l.bld.Mod(lReg, rReg, ty, cls)
	case "&":
		return l.bld.And(lReg, rReg, ty, cls)
	case "|":
		return l.bld.Or(lReg, rReg, ty, cls)
	case "^":
		return l.bld.Xor(lReg, rReg, ty, cls)
	case "<<":
		return l.bld.Shl(lReg, rReg, ty, cls)
	case ">>":
		if mir2.IsSigned(ex.L.ExprTy()) || LiteralPeerType(ex.L, ex.R) != nil && mir2.IsSigned(ty) {
			return l.bld.Sar(lReg, rReg, ty, cls)
		}
		return l.bld.Shr(lReg, rReg, ty, cls)
	default:
		panic(fmt.Sprintf("hir/lower: unhandled binary op %q", ex.Op))
	}
}

// lowerLogical preserves short-circuit evaluation and yields a canonical bool.
// The false/true blocks keep flag registers out of the join's block argument.
func (l *lowerer) lowerLogical(ex *BinExpr) mir2.Reg {
	rhsLabel := l.fresh("logic_rhs")
	trueLabel := l.fresh("logic_true")
	falseLabel := l.fresh("logic_false")
	joinLabel := l.fresh("logic_join")

	left := l.lowerCond(ex.L)
	if ex.Op == "&&" {
		l.bld.BrIf(left, rhsLabel, nil, falseLabel, nil)
	} else {
		l.bld.BrIf(left, trueLabel, nil, rhsLabel, nil)
	}
	l.bld.SwitchToNewBlock(rhsLabel)
	right := l.lowerCond(ex.R)
	l.bld.BrIf(right, trueLabel, nil, falseLabel, nil)

	l.bld.SwitchToNewBlock(trueLabel)
	one := l.bld.Const(1, mir2.TyBool, mir2.ClassGeneral)
	l.bld.Jmp(joinLabel, one)
	l.bld.SwitchToNewBlock(falseLabel)
	zero := l.bld.Const(0, mir2.TyBool, mir2.ClassGeneral)
	l.bld.Jmp(joinLabel, zero)
	join := l.bld.SwitchToNewBlock(joinLabel)
	return l.bld.BlockParam(join, mir2.TyBool, mir2.ClassGeneral)
}

// lowerUnaryExpr lowers a unary expression.
func (l *lowerer) lowerUnaryExpr(ex *UnaryExpr) mir2.Reg {
	xReg := l.lowerExpr(ex.X)
	ty := ex.Ty
	var cls mir2.RegClass
	if !l.inLoopContext {
		cls = classForALUResult(ex.Op, ty)
	} else {
		cls = classForExpr(ty)
	}
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
// For ordering comparisons (<, <=, >, >=), picks signed or unsigned CmpCond
// based on the operand type: i8/i16 → signed (CmpLt etc.), u8/u16 → unsigned (CmpUlt etc.).
// Eq/Ne are sign-agnostic (bit patterns match regardless of interpretation).
func (l *lowerer) lowerCond(e Expr) mir2.Reg {
	if bin, ok := e.(*BinExpr); ok {
		if bin.Op == "&&" || bin.Op == "||" {
			return l.lowerLogical(bin)
		}
		var cmp mir2.CmpCond
		// Select comparison semantics from the nonliteral peer when the
		// literal is on the left; keep non-negative constants narrow.
		operandTy := bin.L.ExprTy()
		wordTy := operandTy
		if mir2.IsInt(operandTy) && mir2.IsInt(bin.R.ExprTy()) {
			if bin.R.ExprTy().Width() > wordTy.Width() {
				wordTy = bin.R.ExprTy()
			}
			if _, literal := bin.L.(*IntLitExpr); literal {
				// On a width tie the peer supplies signedness, not the
				// literal's default unsigned type (e.g. 32767 < x:i16).
				// Keep a wider literal's type and width unchanged.
				if bin.R.ExprTy().Width() == wordTy.Width() {
					wordTy = bin.R.ExprTy()
				}
				operandTy = wordTy
			}
		}
		if mir2.IsSigned(operandTy) && mir2.IsSigned(wordTy) {
			operandTy = wordTy
		}
		if common := LiteralPeerType(bin.L, bin.R); common != nil {
			operandTy, wordTy = common, common
		}
		signed := mir2.IsSigned(operandTy)
		switch bin.Op {
		case "==":
			cmp = mir2.CmpEq
		case "!=":
			cmp = mir2.CmpNe
		case "<":
			if signed {
				cmp = mir2.CmpLt
			} else {
				cmp = mir2.CmpUlt
			}
		case "<=":
			if signed {
				cmp = mir2.CmpLe
			} else {
				cmp = mir2.CmpUle
			}
		case ">":
			if signed {
				cmp = mir2.CmpGt
			} else {
				cmp = mir2.CmpUgt
			}
		case ">=":
			if signed {
				cmp = mir2.CmpGe
			} else {
				cmp = mir2.CmpUge
			}
		default:
			goto fallback
		}
		// Preserve signed values at the common comparison width with explicit
		// HIR conversions; physical register moves only stage stored bits.
		lowerOperand := func(e Expr) mir2.Reg {
			if lit, literal := e.(*IntLitExpr); literal {
				if lit.Val < 0 {
					return l.lowerExprAs(e, wordTy)
				}
				// Non-negative byte constants need no extension at word width.
				// Mark them unsigned so physical zero extension preserves value.
				if mir2.IsSigned(lit.Ty) && lit.Ty.Width() < wordTy.Width() {
					ty := mir2.TyU8
					switch lit.Ty.Width() {
					case 16:
						ty = mir2.TyU16
					case 24:
						ty = mir2.TyU24
					case 32:
						ty = mir2.TyU32
					}
					return l.bld.Const(lit.Val, ty, classForExpr(ty))
				}
				return l.lowerExpr(e)
			}
			r := l.lowerExpr(e)
			if mir2.IsInt(e.ExprTy()) && mir2.IsInt(wordTy) && e.ExprTy().Width() < wordTy.Width() {
				if mir2.IsSigned(e.ExprTy()) {
					return l.bld.Sext(r, e.ExprTy(), wordTy, classForExpr(wordTy))
				}
				// Unsigned values already have the correct stored bit pattern.
				// Keep their physical zero extension at the comparison use.
			}
			return r
		}
		lReg := lowerOperand(bin.L)
		rReg := lowerOperand(bin.R)
		if signed {
			return l.bld.CmpWithSrcTy(cmp, lReg, rReg, mir2.ClassFlag, false, operandTy)
		}
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
		target := st.Target
		for {
			bit, ok := target.(*BitExpr)
			if !ok {
				break
			}
			target = bit.X
		}
		if vr, ok := target.(*VarRefExpr); ok {
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
		// BreakStmt, ContinueStmt, LabelStmt, GotoStmt: no mutations
	}
}

// mapToBool converts a map[string]T to map[string]bool (keys only).
func mapToBool[T any](m map[string]T) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// collectLabels scans a block for LabelStmt nodes and calls emit for each.
func collectLabels(blk *Block, emit func(string)) {
	if blk == nil {
		return
	}
	for _, s := range blk.Body {
		switch st := s.(type) {
		case *LabelStmt:
			emit(st.Name)
		case *Block:
			collectLabels(st, emit)
		case *IfStmt:
			collectLabels(st.Then, emit)
			collectLabels(st.Else, emit)
		case *WhileStmt:
			collectLabels(st.Body, emit)
		case *ForRangeStmt:
			collectLabels(st.Body, emit)
		case *ForEachStmt:
			collectLabels(st.Body, emit)
		case *SwitchStmt:
			for _, c := range st.Cases {
				collectLabels(c.Body, emit)
			}
			collectLabels(st.Default, emit)
		}
	}
}

// detectBackwardGotos finds labels that are targets of backward gotos.
// A backward goto is one where the GotoStmt appears AFTER its target LabelStmt
// in a linear walk of the AST (i.e., it jumps to an earlier point → loop).
func detectBackwardGotos(blk *Block) map[string]bool {
	seen := map[string]bool{}   // labels seen so far
	result := map[string]bool{} // labels that are backward-goto targets
	walkBackwardGotos(blk, seen, result)
	return result
}

func walkBackwardGotos(blk *Block, seen, result map[string]bool) {
	if blk == nil {
		return
	}
	for _, s := range blk.Body {
		switch st := s.(type) {
		case *LabelStmt:
			seen[st.Name] = true
		case *GotoStmt:
			if seen[st.Label] {
				result[st.Label] = true // backward: label before goto
			}
		case *Block:
			walkBackwardGotos(st, seen, result)
		case *IfStmt:
			walkBackwardGotos(st.Then, seen, result)
			walkBackwardGotos(st.Else, seen, result)
		case *WhileStmt:
			walkBackwardGotos(st.Body, seen, result)
		case *ForRangeStmt:
			walkBackwardGotos(st.Body, seen, result)
		case *ForEachStmt:
			walkBackwardGotos(st.Body, seen, result)
		case *SwitchStmt:
			for _, c := range st.Cases {
				walkBackwardGotos(c.Body, seen, result)
			}
			walkBackwardGotos(st.Default, seen, result)
		}
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
	case *CallIndirectExpr:
		scanUsedExpr(ex.FnPtr, env, used)
		for _, a := range ex.Args {
			scanUsedExpr(a, env, used)
		}
	case *FieldExpr:
		scanUsedExpr(ex.X, env, used)
	case *BitExpr:
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
		// BreakStmt, ContinueStmt, LabelStmt, GotoStmt: no expressions to scan
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
	ptr    Expr        // base pointer expression (nil for range sources)
	len    Expr        // element count (nil for range sources — count derived from range)
	stages []iterStage // transformation stages (innermost-first)
	mutate bool        // mapInPlace: store transformed element back to ptr
	cbName string      // forEach callback function name
	elemTy mir2.Ty     // element type (from callback's first param)
	// fold fields (isFold=true when chain is fold/reduce, not forEach):
	isFold    bool    // true → fold terminal (returns accumulator)
	foldInit  Expr    // initial accumulator value
	foldAccTy mir2.Ty // accumulator type (cb's return type)
	// range source fields (isRange=true when source is range(lo..hi)):
	isRange  bool // true → counter-based source, no memory/pointer
	rangeLo  Expr // inclusive start (or the "count-from" value for back-counting)
	rangeHi  Expr // exclusive end  (or 0 for back-counting)
	rangeRev bool // true when range(hi..lo): B starts at lo, counts down to 0 (DJNZ direct)
}

// iterCallbackNeedsCall reports whether an iterator-chain callback must run as
// a real CALL instead of being inlined into the fused loop body. Inlined asm
// blocks keep the standalone-function ABI (parameters in fixed registers) that
// the loop body does not provide, and a return that is not the callback's
// final statement would otherwise return from the enclosing consumer instead
// of ending this element's body.
func iterCallbackNeedsCall(b *Block) bool {
	if b == nil {
		return false
	}
	for i, s := range b.Body {
		if iterStmtNeedsCall(s, i == len(b.Body)-1) {
			return true
		}
	}
	return false
}

func iterStmtNeedsCall(s Stmt, trailing bool) bool {
	switch st := s.(type) {
	case *AsmStmt:
		return true
	case *ReturnStmt:
		return !trailing
	case *IfStmt:
		return iterBlockNeedsCall(st.Then) || iterBlockNeedsCall(st.Else)
	case *WhileStmt:
		return iterBlockNeedsCall(st.Body)
	case *ForEachStmt:
		return iterBlockNeedsCall(st.Body)
	case *ForRangeStmt:
		return iterBlockNeedsCall(st.Body)
	case *SwitchStmt:
		for _, c := range st.Cases {
			if iterBlockNeedsCall(c.Body) {
				return true
			}
		}
		return iterBlockNeedsCall(st.Default)
	case *Block:
		return iterBlockNeedsCall(st)
	}
	return false
}

// iterBlockNeedsCall scans a nested block: a return at any depth here is a
// conditional early exit of the callback, never a droppable trailing return.
func iterBlockNeedsCall(b *Block) bool {
	if b == nil {
		return false
	}
	for _, s := range b.Body {
		if iterStmtNeedsCall(s, false) {
			return true
		}
	}
	return false
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
		if chain.isRange {
			l.lowerRangeFold(chain)
		} else {
			l.lowerFusedFold(chain)
		}
	} else {
		if chain.isRange {
			l.lowerRangeForEach(chain)
		} else {
			l.lowerFusedForEach(chain)
		}
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
	if chain.isRange {
		return l.lowerRangeFold(chain), true
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
	// Chainable:  range(lo..hi).fold(init, cb)  → Args=[range, init, cb] (3 args)
	// Functional: fold(src, init, cb, n)         → Args=[src, init, cb, n] (4 args)
	if call.Fn == "fold" || call.Fn == "reduce" {
		var (
			srcExpr  Expr
			initExpr Expr
			cbArg    Expr
			lenExpr  Expr
		)
		switch len(call.Args) {
		case 3: // chainable: fold(range_src, init, cb)
			srcExpr, initExpr, cbArg = call.Args[0], call.Args[1], call.Args[2]
		case 4: // functional: fold(ptr_src, init, cb, n)
			srcExpr, initExpr, cbArg, lenExpr = call.Args[0], call.Args[1], call.Args[2], call.Args[3]
		default:
			return iterChain{}, false
		}
		cbName := l.resolveFunc(cbArg)
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
		chain := iterChain{
			foldInit:  initExpr,
			cbName:    cbName,
			elemTy:    elemTy,
			isFold:    true,
			foldAccTy: accTy,
		}
		// Unwrap nested map/filter calls (from pipe .apply() expansion or manual chaining).
		innerSrc := srcExpr
		for {
			inner, ok2 := innerSrc.(*CallExpr)
			if !ok2 {
				break
			}
			if (inner.Fn == "map" || inner.Fn == "filter") && len(inner.Args) == 2 {
				fn := l.resolveFunc(inner.Args[1])
				if fn == "" {
					break
				}
				chain.stages = append(chain.stages, iterStage{kind: inner.Fn, fn: fn})
				innerSrc = inner.Args[0]
			} else {
				break
			}
		}
		// Reverse: stages collected outermost→innermost; want innermost-first.
		for i, j := 0, len(chain.stages)-1; i < j; i, j = i+1, j-1 {
			chain.stages[i], chain.stages[j] = chain.stages[j], chain.stages[i]
		}
		if rs, ok2 := innerSrc.(*RangeSourceExpr); ok2 {
			chain.isRange = true
			chain.rangeLo = rs.Lo
			chain.rangeHi = rs.Hi
			chain.rangeRev = rs.Rev
		} else {
			chain.ptr = innerSrc
			chain.len = lenExpr
		}
		return chain, true
	}

	// ── forEach / mapInPlace ─────────────────────────────────────────────────
	// Chainable range form: range(lo..hi).forEach(cb)  → Args=[range, cb] (2 args)
	// Standard form:        forEach(src, cb, n)         → Args=[src, cb, n] (3 args)
	isMutate := false
	switch call.Fn {
	case "forEach":
	case "mapInPlace":
		isMutate = true
	default:
		return iterChain{}, false
	}

	// Unwrap stages before distinguishing range and memory sources. A range
	// chain has two terminal arguments even when map/filter wrap its source.
	if len(call.Args) != 2 && len(call.Args) != 3 {
		return iterChain{}, false
	}

	cbName := l.resolveFunc(call.Args[1])
	if cbName == "" {
		return iterChain{}, false
	}

	chain := iterChain{
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
	if rs, ok := src.(*RangeSourceExpr); ok {
		// mapInPlace requires storage; range values have no address to mutate.
		if chain.mutate || len(call.Args) != 2 {
			return iterChain{}, false
		}
		chain.isRange = true
		chain.rangeLo, chain.rangeHi, chain.rangeRev = rs.Lo, rs.Hi, rs.Rev
	} else {
		if len(call.Args) != 3 {
			return iterChain{}, false
		}
		chain.ptr, chain.len = src, call.Args[2]
	}
	return chain, true
}

// resolveFunc resolves a callback expression to a function name.
// Accepts VarRefExpr (legacy) and AddrOfExpr (function-as-value).
func (l *lowerer) resolveFunc(arg Expr) string {
	var name string
	switch a := arg.(type) {
	case *VarRefExpr:
		name = a.Name
	case *AddrOfExpr:
		if a.X != nil {
			return ""
		}
		name = a.Sym
	default:
		return ""
	}
	if alias, ok := l.fnAliases[name]; ok {
		return alias
	}
	if l.hirFuncNames[name] {
		return name
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
	} else if cbFn == nil || cbFn.Body == nil || iterCallbackNeedsCall(cbFn.Body) {
		// Unknown or unsafe-to-inline function — emit a call.
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
			renamed := renameStmt(s, paramName, elemVar, l.lineage)
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
		Body:          &Block{Body: bodyStmts},
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
		args := []Expr{&VarRefExpr{Name: accVar, Ty: accTy}, &VarRefExpr{Name: elemVar, Ty: elemTy}}
		if cbFn != nil && len(cbFn.Params) == 1 {
			args = args[:1]
		}
		bodyExpr = &CallExpr{Fn: chain.cbName, Args: args, Ty: accTy}
	}
	bodyStmts := []Stmt{
		&AssignStmt{
			Target: &VarRefExpr{Name: accVar, Ty: accTy},
			Val:    bodyExpr,
		},
	}

	bodyStmts = foldStageBody(chain.stages, elemVar, elemTy, bodyStmts)

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

// ── Range-based (counter-only) iterator lowering ─────────────────────────────
//
// range(lo..hi).forEach(cb)   — pure DJNZ loop, no memory access
// range(lo..hi).fold(init,cb) — DJNZ accumulator loop, result in register
//
// The range source exposes a countdown, not a conventional half-open interval:
// range(0..n) and range(n..0) both yield n, n-1, ..., 1. Other forward
// bounds give count = hi-lo; the element is the counter, not lo+offset.
// See Nanz Language Book, chapter 6. lowerRangeLoop emits TermDJNZ directly.
//
// lowerRangeForEach lowers range(lo..hi).forEach(cb) to a counter-only loop.
func (l *lowerer) lowerRangeForEach(chain iterChain) {
	const elemVar = "__iter_elem__"
	elemTy := chain.elemTy
	cbFn := l.hirFuncs[chain.cbName]

	// Build the loop body by inlining the callback (same as lowerFusedForEach
	// but without a Load — the element IS the counter).
	var bodyStmts []Stmt
	if cbFn != nil && cbFn.Body != nil && !iterCallbackNeedsCall(cbFn.Body) && len(cbFn.Params) >= 1 {
		paramName := cbFn.Params[0].Name
		for _, st := range cbFn.Body.Body {
			if _, isRet := st.(*ReturnStmt); isRet {
				continue
			}
			bodyStmts = append(bodyStmts, renameStmt(st, paramName, elemVar, l.lineage))
		}
	} else {
		bodyStmts = []Stmt{&ExprStmt{Expr: &CallExpr{
			Fn:   chain.cbName,
			Args: []Expr{&VarRefExpr{Name: elemVar, Ty: elemTy}},
			Ty:   mir2.TyVoid,
		}}}
	}

	// Prepend stage transformations (map/filter from pipe .apply() expansion).
	bodyStmts = l.prependStageStmts(chain.stages, elemVar, elemTy, bodyStmts)

	l.lowerRangeLoop(chain, elemVar, elemTy, bodyStmts)
}

// prependStageStmts preserves stage order and filter short-circuiting without
// ContinueStmt: continue would jump over a range loop's implicit DJNZ decrement.
func (l *lowerer) prependStageStmts(stages []iterStage, elemVar string, elemTy mir2.Ty, body []Stmt) []Stmt {
	for i := len(stages) - 1; i >= 0; i-- {
		stage := stages[i]
		ty := elemTy
		if stage.kind == "filter" {
			ty = mir2.TyBool
		}
		var value Expr
		if fn := l.hirFuncs[stage.fn]; fn != nil && len(fn.Params) == 1 {
			if ret := extractReturnExpr(fn.Body); ret != nil {
				value = renameExpr(ret, fn.Params[0].Name, elemVar)
			}
		}
		if value == nil {
			value = &CallExpr{Fn: stage.fn, Args: []Expr{&VarRefExpr{Name: elemVar, Ty: elemTy}}, Ty: ty}
		}
		switch stage.kind {
		case "map":
			assign := &AssignStmt{Target: &VarRefExpr{Name: elemVar, Ty: elemTy}, Val: value}
			body = append([]Stmt{assign}, body...)
		case "filter":
			body = []Stmt{&IfStmt{Cond: value, Then: &Block{Body: body}}}
		}
	}
	return body
}

// lowerRangeFold lowers range(lo..hi).fold(init, cb) to a DJNZ accumulator loop.
func (l *lowerer) lowerRangeFold(chain iterChain) mir2.Reg {
	const accVar = "__fold_acc__"
	const elemVar = "__iter_elem__"
	accTy := chain.foldAccTy
	elemTy := chain.elemTy

	// Seed the accumulator.
	initReg := l.lowerExpr(chain.foldInit)
	l.bind(accVar, initReg, accTy)

	// Build the loop body: __fold_acc__ = cb(__fold_acc__, __iter_elem__)
	var bodyExpr Expr
	cbFn := l.hirFuncs[chain.cbName]
	if cbFn != nil && cbFn.Body != nil && len(cbFn.Params) >= 2 {
		if ret := extractReturnExpr(cbFn.Body); ret != nil {
			bodyExpr = renameExpr(ret, cbFn.Params[0].Name, accVar)
			bodyExpr = renameExpr(bodyExpr, cbFn.Params[1].Name, elemVar)
		}
	} else if cbFn != nil && cbFn.Body != nil && len(cbFn.Params) == 1 {
		// Single-param cb: acc-only fold (element unused).
		if ret := extractReturnExpr(cbFn.Body); ret != nil {
			bodyExpr = renameExpr(ret, cbFn.Params[0].Name, accVar)
		}
	}
	if bodyExpr == nil {
		args := []Expr{&VarRefExpr{Name: accVar, Ty: accTy}, &VarRefExpr{Name: elemVar, Ty: elemTy}}
		if cbFn != nil && len(cbFn.Params) == 1 {
			args = args[:1]
		}
		bodyExpr = &CallExpr{Fn: chain.cbName, Args: args, Ty: accTy}
	}
	// Keep stage order and filter short-circuiting. Calls preserve callback
	// prefixes; predicates guard the remaining pipeline without continue,
	// which would bypass the range loop's implicit DJNZ decrement.
	foldAssign := &AssignStmt{Target: &VarRefExpr{Name: accVar, Ty: accTy}, Val: bodyExpr}
	bodyStmts := foldStageBody(chain.stages, elemVar, elemTy, []Stmt{foldAssign})

	l.lowerRangeLoop(chain, elemVar, elemTy, bodyStmts)
	return l.env[accVar]
}

// foldStageBody preserves map/filter order and skips the remaining stages and
// terminal fold when a filter fails. Build from the end to nest the suffix.
func foldStageBody(stages []iterStage, elemVar string, elemTy mir2.Ty, body []Stmt) []Stmt {
	for i := len(stages) - 1; i >= 0; i-- {
		stage := stages[i]
		value := &VarRefExpr{Name: elemVar, Ty: elemTy}
		switch stage.kind {
		case "map":
			assign := &AssignStmt{Target: value, Val: &CallExpr{Fn: stage.fn, Args: []Expr{value}, Ty: elemTy}}
			body = append([]Stmt{assign}, body...)
		case "filter":
			pred := &CallExpr{Fn: stage.fn, Args: []Expr{value}, Ty: mir2.TyBool}
			body = []Stmt{&IfStmt{Cond: pred, Then: &Block{Body: body}}}
		}
	}
	return body
}

// lowerRangeLoop emits a counter-only DJNZ loop for range(lo..hi) sources.
//
// Uses TermDJNZ directly — no CmpNe/AND A inside the loop at all.
//
//	entry:
//	    cnt = countExpr              B = n (ClassCounter)
//	    cond = (cnt != 0)            AND A while cnt may share A — pre-check only
//	    BrIf(cond, rng_body, rng_exit)
//
//	rng_body(cnt: B, mutated...):
//	    elem = cnt                   element = current B value
//	    [body stmts]                 may update mutated vars
//	    DJNZ rng_body(cnt-1, mutated...) | rng_exit(mutated...)
//	    ──> emitted as TermDJNZ; no CmpNe, no register clobber
//
//	rng_exit(mutated...):
//
// The pre-check uses A freely (before any acc block param is live).
// DJNZ handles all subsequent iterations without touching A.
func (l *lowerer) lowerRangeLoop(chain iterChain, elemVar string, elemTy mir2.Ty, bodyStmts []Stmt) {
	envBefore := cloneMap(l.env)

	// Compute loop count (initial B value).
	var countExpr Expr
	if chain.rangeRev {
		countExpr = chain.rangeLo // range(n..0): n iterations
	} else {
		lo := chain.rangeLo
		hi := chain.rangeHi
		if lit, ok := lo.(*IntLitExpr); ok && lit.Val == 0 {
			countExpr = hi
		} else {
			countExpr = &BinExpr{Op: "-", L: hi, R: lo, Ty: hi.ExprTy()}
		}
	}
	cntReg := l.lowerExpr(countExpr) // typically ClassAcc (A from func param)

	// Scan body for mutated outer vars (block params for loop-carried state).
	elemInitReg := l.bld.Const(0, elemTy, mir2.ClassGeneral)
	l.bind(elemVar, elemInitReg, elemTy)
	syntheticBody := &Block{Body: bodyStmts}
	allUsed := map[string]bool{}
	scanUsedInBlock(syntheticBody, envBefore, allUsed)
	muts := scanMutations(syntheticBody, envBefore)
	for name := range muts {
		allUsed[name] = true
	}
	delete(allUsed, elemVar)
	mutated := sortedKeys(allUsed, envBefore)

	bodyLabel := l.fresh("rng_body")
	exitLabel := l.fresh("rng_exit")

	// Pre-check: skip loop if cnt == 0.
	// cntReg is typically in A (ClassAcc from func param position 0).
	// AND A sets Z if A==0 without extra cost; the trampoline sorts out
	// the A→B (counter) and 0→A (acc) parallel copies before the loop body.
	zero8 := l.bld.Const(0, mir2.TyU8, mir2.ClassAcc)
	cond0 := l.bld.Cmp(mir2.CmpNe, cntReg, zero8, mir2.ClassFlag, false)
	entryBodyArgs := append([]mir2.Reg{cntReg}, l.collectArgs(mutated)...)
	exitArgs0 := l.collectArgs(mutated)
	l.bld.BrIf(cond0, bodyLabel, entryBodyArgs, exitLabel, exitArgs0)

	// ── rng_body(cnt: B, mutated...) ──
	body := l.bld.SwitchToNewBlock(bodyLabel)
	bodyCnt := l.bld.BlockParam(body, mir2.TyU8, mir2.ClassCounter) // B
	bodyEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		// For fold loops, put the accumulator in ClassAcc (A) — the result
		// is returned, so keeping it in A avoids a LD A,C copy at exit.
		cls := classForLoop(ty, name)
		if chain.isFold {
			cls = classForRet(ty)
		}
		r := l.bld.BlockParam(body, ty, cls)
		bodyEnv[name] = r
	}
	l.env = bodyEnv

	// Element = current B value.
	l.bind(elemVar, bodyCnt, elemTy)

	// A fold owns its accumulator in A. A forEach body may have several
	// independently live values (including stage results and captures); use
	// ordinary loop expression classes rather than forcing all ALU results to A.
	prevInLoop := l.inLoopContext
	l.inLoopContext = !chain.isFold
	l.pushLoop(loopCtx{headLabel: bodyLabel, exitLabel: exitLabel, mutated: mutated})
	l.lowerBlock(syntheticBody)
	l.popLoop()
	l.inLoopContext = prevInLoop

	// TermDJNZ: B--; if B!=0 goto body(mutated...) else exit(mutated...).
	// Counter (bodyCnt) is implicit — NOT passed in bodyBackArgs or exitArgs1.
	bodyBackArgs := l.collectArgs(mutated)
	exitArgs1 := l.collectArgs(mutated)
	l.bld.DJNZ(bodyCnt, bodyLabel, bodyBackArgs, exitLabel, exitArgs1)

	// ── rng_exit(mutated...) ──
	exit := l.bld.SwitchToNewBlock(exitLabel)
	exitEnv := cloneMap(envBefore)
	for _, name := range mutated {
		ty := l.envTy[name]
		cls := classForExpr(ty)
		if chain.isFold {
			cls = classForRet(ty) // keep acc in A for clean return
		}
		r := l.bld.BlockParam(exit, ty, cls)
		exitEnv[name] = r
	}
	delete(exitEnv, elemVar)
	l.env = exitEnv
}

// extractReturnExpr extracts a single-return expression only when it is the
// entire body. Dropping a prefix can erase effects, local bindings or control flow.
func extractReturnExpr(body *Block) Expr {
	if body == nil || len(body.Body) != 1 {
		return nil
	}
	if rs, ok := body.Body[0].(*ReturnStmt); ok && rs.Val != nil {
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
		return &IntLitExpr{Val: ex.Val, Ty: ex.Ty, Untyped: ex.Untyped}
	case *BoolLitExpr:
		return &BoolLitExpr{Val: ex.Val}
	case *VarRefExpr:
		if ex.Name == from {
			return &VarRefExpr{Name: to, Ty: ex.Ty, Volatile: ex.Volatile}
		}
		return &VarRefExpr{Name: ex.Name, Ty: ex.Ty, Volatile: ex.Volatile}
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
	case *CallIndirectExpr:
		args := make([]Expr, len(ex.Args))
		for i, a := range ex.Args {
			args[i] = renameExpr(a, from, to)
		}
		return &CallIndirectExpr{FnPtr: renameExpr(ex.FnPtr, from, to), Args: args, Ty: ex.Ty, Signature: ex.Signature.Clone()}
	case *LoadExpr:
		return &LoadExpr{Ptr: renameExpr(ex.Ptr, from, to), Ty: ex.Ty, Volatile: ex.Volatile}
	case *IndexExpr:
		return &IndexExpr{
			Base: renameExpr(ex.Base, from, to), Idx: renameExpr(ex.Idx, from, to),
			ElemTy: ex.ElemTy, ElemStride: ex.ElemStride, Volatile: ex.Volatile,
		}
	case *FieldExpr:
		return &FieldExpr{X: renameExpr(ex.X, from, to), Field: ex.Field, Offset: ex.Offset, Ty: ex.Ty, Volatile: ex.Volatile}
	case *BitExpr:
		return &BitExpr{X: renameExpr(ex.X, from, to), Bit: ex.Bit}
	case *AddrOfExpr:
		if ex.X != nil {
			return &AddrOfExpr{Sym: ex.Sym, X: renameExpr(ex.X, from, to)}
		}
		return &AddrOfExpr{Sym: ex.Sym}
	case *ConstPtrExpr:
		return &ConstPtrExpr{ElemTy: ex.ElemTy, Addr: ex.Addr}
	case *LetInExpr:
		return &LetInExpr{
			Name: ex.Name, Ty: ex.Ty,
			Init: renameExpr(ex.Init, from, to),
			Body: renameExpr(ex.Body, from, to),
		}
	default:
		return e
	}
}

// renameStmt deep-copies a statement, renaming variables as in renameExpr.
func renameStmt(s Stmt, from, to string, lineage ...map[Stmt]LoopLineage) (renamed Stmt) {
	// Each inlined copy retains its original site attribution. The lowerer owns
	// this map; adding clone nodes must not mutate the input module metadata.
	defer func() {
		if len(lineage) > 0 && lineage[0] != nil && renamed != nil {
			if origin, ok := lineage[0][s]; ok {
				lineage[0][renamed] = origin
			}
		}
	}()
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
			els = renameBlock(st.Else, from, to, lineage...)
		}
		return &IfStmt{Cond: renameExpr(st.Cond, from, to), Then: renameBlock(st.Then, from, to, lineage...), Else: els}
	case *Block:
		return renameBlock(st, from, to, lineage...)
	case *BreakStmt:
		return &BreakStmt{}
	case *ContinueStmt:
		return &ContinueStmt{}
	case *LabelStmt:
		return &LabelStmt{Name: st.Name}
	case *GotoStmt:
		return &GotoStmt{Label: st.Label}
	case *StoreStmt:
		return &StoreStmt{Ptr: renameExpr(st.Ptr, from, to), Val: renameExpr(st.Val, from, to)}
	case *WhileStmt:
		return &WhileStmt{Cond: renameExpr(st.Cond, from, to), Body: renameBlock(st.Body, from, to, lineage...)}
	case *ForRangeStmt:
		c := *st
		c.Start = renameExpr(st.Start, from, to)
		c.End = renameExpr(st.End, from, to)
		c.Body = renameBlock(st.Body, from, to, lineage...)
		return &c
	case *ForEachStmt:
		c := *st
		c.Ptr = renameExpr(st.Ptr, from, to)
		c.Start = renameExpr(st.Start, from, to)
		c.Len = renameExpr(st.Len, from, to)
		c.Body = renameBlock(st.Body, from, to, lineage...)
		return &c
	case *SwitchStmt:
		c := *st
		c.Val = renameExpr(st.Val, from, to)
		c.Cases = make([]*SwitchCase, len(st.Cases))
		for i := range c.Cases {
			cc := *st.Cases[i]
			cc.Body = renameBlock(st.Cases[i].Body, from, to, lineage...)
			c.Cases[i] = &cc
		}
		c.Default = renameBlock(st.Default, from, to, lineage...)
		return &c
	default:
		return s
	}
}

// renameBlock deep-copies a block, renaming variables.
func renameBlock(b *Block, from, to string, lineage ...map[Stmt]LoopLineage) *Block {
	if b == nil {
		return nil
	}
	stmts := make([]Stmt, len(b.Body))
	for i, s := range b.Body {
		stmts[i] = renameStmt(s, from, to, lineage...)
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

// emitStructLitFields stores each field of lit into the target struct.
// baseProvider is called once per field to get a fresh base address register;
// this ensures each OpAddrOf result has useCount==1, enabling the HL-chain
// optimization in z80codegen (LD HL,sym ; LD (HL),n ; INC HL ; ...).
// Fields may appear in any source order; unmentioned fields are left uninitialized.
// fixedStructStorage allocates one owned object in the function frame, even
// for a source declaration in a conditional or loop body.
func (l *lowerer) fixedStructStorage(ty *mir2.StructTy) mir2.Reg {
	r := l.mf.AllocReg()
	i := &mir2.Inst{Op: mir2.OpAlloca, Dst: r, Ty: l.m.PointerType(), Cls: mir2.ClassPointer, Imm: int64(max(1, mir2.ByteWidth(ty)))}
	l.mf.Entry().Insts = append([]*mir2.Inst{i}, l.mf.Entry().Insts...)
	return r
}

func (l *lowerer) copyStructValue(dest, source mir2.Reg, ty *mir2.StructTy) {
	l.copyStructValueEffects(dest, source, ty, false, false)
}

func (l *lowerer) copyStructValueEffects(dest, source mir2.Reg, ty *mir2.StructTy, sourceVolatile, destVolatile bool) {
	values := make([]mir2.Reg, mir2.ByteWidth(ty))
	for off := range values {
		addr := source
		if off != 0 {
			addr = l.bld.Field(source, int64(off), mir2.ClassPointer)
		}
		values[off] = l.loadMemory(addr, mir2.TyU8, mir2.ClassGeneral, sourceVolatile || mir2.VolatileStorageByte(ty, off))
	}
	for off, value := range values {
		addr := dest
		if off != 0 {
			addr = l.bld.Field(dest, int64(off), mir2.ClassPointer)
		}
		l.storeMemory(addr, value, mir2.TyU8, destVolatile || mir2.VolatileStorageByte(ty, off))
	}
}

func (l *lowerer) emitStructLitFields(lit *StructLitExpr, baseProvider func() mir2.Reg, inherited ...bool) {
	volatile := len(inherited) != 0 && inherited[0]
	for _, fi := range lit.Fields {
		// Find the matching struct field to get its type and byte offset.
		var fieldTy mir2.Ty
		byteOff := 0
		for idx, sf := range lit.St.Fields {
			if sf.Name == fi.Name {
				fieldTy = sf.Ty
				byteOff = lit.St.ByteOffset(idx)
				break
			}
		}
		if fieldTy == nil {
			panic(fmt.Sprintf("hir/lower: struct %q has no field %q", lit.St.Name, fi.Name))
		}
		if nested, ok := fi.Val.(*StructLitExpr); ok {
			off := byteOff
			l.emitStructLitFields(nested, func() mir2.Reg {
				base := baseProvider()
				if off == 0 {
					return base
				}
				return l.bld.Field(base, int64(off), mir2.ClassPointer)
			}, volatile || mir2.VolatileStorageByte(lit.St, byteOff))
			continue
		}
		val := l.lowerStorageExprAs(fi.Val, fieldTy)
		base := baseProvider()
		var addr mir2.Reg
		if byteOff == 0 {
			addr = base
		} else {
			addr = l.bld.Field(base, int64(byteOff), mir2.ClassPointer)
		}
		if ty, ok := mir2.BaseOf(fieldTy).(*mir2.StructTy); ok {
			l.copyStructValueEffects(addr, val, ty, l.volatileObject(fi.Val), volatile || mir2.VolatileStorageByte(lit.St, byteOff))
		} else {
			l.storeMemory(addr, val, fieldTy, volatile || mir2.VolatileStorageByte(lit.St, byteOff))
		}
	}
}

// emitStructLitFieldsChained stores fields in ascending offset order using
// sequential address chaining: each addr = Field(prevAddr, offsetDiff).
// This keeps only one pointer reg live at a time — all can share HL,
// enabling the INC HL / LD (HL),n pattern for @smc and other pointer writes.
//
// Each field value is lowered just before its Store, not upfront.  Lowering
// all values in advance would put two ClassAcc (A) vregs live simultaneously,
// causing the allocator to spill to alloca instead of keeping everything in
// registers.
func (l *lowerer) emitStructLitFieldsChained(lit *StructLitExpr, startAddr mir2.Reg, inherited ...bool) {
	volatile := len(inherited) != 0 && inherited[0]
	for _, field := range lit.St.Fields {
		if _, aggregate := mir2.BaseOf(field.Ty).(*mir2.StructTy); aggregate {
			// Inline subobjects require a value copy (or recursive literal
			// initialization), never an address-sized scalar store.
			l.emitStructLitFields(lit, func() mir2.Reg { return startAddr }, volatile)
			return
		}
	}
	// Build sorted (offset, ty, valExpr) list — store HIR Expr, not lowered Reg.
	type fieldEntry struct {
		byteOff int
		ty      mir2.Ty
		valExpr Expr
	}
	entries := make([]fieldEntry, 0, len(lit.Fields))
	for _, fi := range lit.Fields {
		var fieldTy mir2.Ty
		byteOff := 0
		for idx, sf := range lit.St.Fields {
			if sf.Name == fi.Name {
				fieldTy = sf.Ty
				byteOff = lit.St.ByteOffset(idx)
				break
			}
		}
		if fieldTy == nil {
			panic(fmt.Sprintf("hir/lower: struct %q has no field %q", lit.St.Name, fi.Name))
		}
		entries = append(entries, fieldEntry{byteOff, fieldTy, fi.Val})
	}
	// Sort ascending by byte offset (insertion sort — small N).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].byteOff < entries[j-1].byteOff; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	// Emit stores with sequential chaining: prevAddr → Field(prevAddr, diff) → Store.
	// Lower each val expression immediately before its Store so it is dead by the
	// time the next val is lowered — no A-register live range overlap.
	prevAddr := startAddr
	prevOff := 0
	for i, e := range entries {
		var addr mir2.Reg
		if i == 0 && e.byteOff == 0 {
			addr = startAddr
		} else if i == 0 {
			addr = l.bld.Field(startAddr, int64(e.byteOff), mir2.ClassPointer)
		} else {
			diff := e.byteOff - prevOff
			addr = l.bld.Field(prevAddr, int64(diff), mir2.ClassPointer)
		}
		val := l.lowerStorageExprAs(e.valExpr, e.ty) // lower just before Store — short live range
		l.storeMemory(addr, val, e.ty, volatile || mir2.VolatileStorageByte(lit.St, e.byteOff))
		prevAddr = addr
		prevOff = e.byteOff
	}
}

// lowerStructLitAlloca lowers a StructLitExpr as a standalone expression:
// allocates a temporary stack frame, writes fields into it, and returns the
// pointer.  Used when the literal appears in non-assignment contexts (function
// arguments, return values, etc.).
func (l *lowerer) lowerStructLitAlloca(lit *StructLitExpr) mir2.Reg {
	bytes := int64(lit.St.Width() / 8)
	if bytes == 0 {
		bytes = 1
	}
	base := l.bld.Alloca(bytes)
	l.emitStructLitFields(lit, func() mir2.Reg { return base })
	return base
}

// ── Inline asm helpers ───────────────────────────────────────────────────────

// z80RegNameToClass maps a physical Z80 register name to its primary RegClass,
// matching the @z80_X annotation scheme used on function parameters.
func z80RegNameToClass(reg string) mir2.RegClass {
	switch strings.ToUpper(reg) {
	case "A":
		return mir2.ClassAcc
	case "B":
		return mir2.ClassCounter
	case "C":
		return mir2.ClassRegC
	case "D":
		return mir2.ClassRegD
	case "E":
		return mir2.ClassRegE
	case "H":
		return mir2.ClassRegH
	case "L":
		return mir2.ClassRegL
	case "HL":
		return mir2.ClassPointer
	case "DE":
		return mir2.ClassIndex
	case "BC":
		return mir2.ClassPair
	case "IX":
		return mir2.ClassIX
	case "IY":
		return mir2.ClassIY
	case "IXH", "IXL", "IYH", "IYL":
		return mir2.ClassIXY8
	default:
		return mir2.ClassGeneral
	}
}

// z80RegToClassSet maps a physical Z80 register name to the classes it occupies.
func z80RegToClassSet(reg string) mir2.ClassSet {
	physical := z80effects.Register(reg)
	if physical == 0 {
		return 0
	}
	return z80WrittenClasses(physical)
}

// A write to a byte also invalidates aliases allocated as pairs or hard pins.
func z80WrittenClasses(writes z80effects.Registers) mir2.ClassSet {
	var classes mir2.ClassSet
	add := func(mask z80effects.Registers, values ...mir2.RegClass) {
		if writes&mask != 0 {
			for _, class := range values {
				classes.Add(class)
			}
		}
	}
	add(z80effects.A, mir2.ClassAcc, mir2.ClassGeneral)
	add(z80effects.B, mir2.ClassCounter, mir2.ClassGeneral, mir2.ClassPair, mir2.ClassDWord)
	add(z80effects.C, mir2.ClassRegC, mir2.ClassGeneral, mir2.ClassPair, mir2.ClassDWord)
	add(z80effects.D, mir2.ClassRegD, mir2.ClassGeneral, mir2.ClassIndex, mir2.ClassPair, mir2.ClassDWord)
	add(z80effects.E, mir2.ClassRegE, mir2.ClassGeneral, mir2.ClassIndex, mir2.ClassPair, mir2.ClassDWord)
	add(z80effects.H, mir2.ClassRegH, mir2.ClassGeneral, mir2.ClassPointer, mir2.ClassPair, mir2.ClassDWord)
	add(z80effects.L, mir2.ClassRegL, mir2.ClassGeneral, mir2.ClassPointer, mir2.ClassPair, mir2.ClassDWord)
	add(z80effects.IX, mir2.ClassIX, mir2.ClassIXY8, mir2.ClassGeneral, mir2.ClassPair)
	add(z80effects.IY, mir2.ClassIY, mir2.ClassIXY8, mir2.ClassGeneral, mir2.ClassPair)
	add(z80effects.F, mir2.ClassFlag)
	add(z80effects.BCAlt|z80effects.DEAlt|z80effects.HLAlt, mir2.ClassShadow, mir2.ClassDWord)
	add(z80effects.AFAlt, mir2.ClassAccShadow)
	return classes
}

// z80RetRegInfo returns the MIR2 type and class for an asm (ret REG) clause.
func z80RetRegInfo(reg string) (mir2.Ty, mir2.RegClass) {
	switch strings.ToUpper(reg) {
	case "A":
		return mir2.TyU8, mir2.ClassAcc
	case "B":
		return mir2.TyU8, mir2.ClassCounter
	case "C", "D", "E", "H", "L":
		return mir2.TyU8, mir2.ClassGeneral
	case "HL":
		return mir2.TyU16, mir2.ClassPointer
	case "DE":
		return mir2.TyU16, mir2.ClassIndex
	case "BC":
		return mir2.TyU16, mir2.ClassPair
	default:
		return mir2.TyU8, mir2.ClassAcc
	}
}

// z80AutoClobbers projects explicit physical instruction writes into allocator
// classes. Unknown instructions/calls/data remain conservatively AllClasses.
// Path/stack/INT safety is separate from this write-set computation.
func z80AutoClobbers(code string) mir2.ClassSet {
	var writes z80effects.Registers
	for _, effect := range z80effects.Block(code) {
		if !effect.Known {
			return mir2.AllClasses
		}
		writes |= effect.Writes
	}
	return z80WrittenClasses(writes)
}

func collectGotoAncestors(b *Block, path []*Block, out map[string][]*Block) {
	if b == nil {
		return
	}
	path = append(append([]*Block(nil), path...), b)
	for _, s := range b.Body {
		switch x := s.(type) {
		case *LabelStmt:
			out[x.Name] = path
		case *Block:
			collectGotoAncestors(x, path, out)
		case *IfStmt:
			collectGotoAncestors(x.Then, path, out)
			collectGotoAncestors(x.Else, path, out)
		case *WhileStmt:
			collectGotoAncestors(x.Body, path, out)
		case *ForRangeStmt:
			collectGotoAncestors(x.Body, path, out)
		case *ForEachStmt:
			collectGotoAncestors(x.Body, path, out)
		case *SwitchStmt:
			for _, c := range x.Cases {
				collectGotoAncestors(c.Body, path, out)
			}
			collectGotoAncestors(x.Default, path, out)
		}
	}
}

// A store-value tree without control flow or callbacks keeps all reused
// constants in the same dominating evaluation sequence. Loads are never merged.
func literalPointerExpression(expr Expr) bool {
	switch value := expr.(type) {
	case *IntLitExpr, *BoolLitExpr, *VarRefExpr, *ConstPtrExpr:
		return true
	case *BinExpr:
		switch value.Op {
		case "+", "-", "&", "|", "^":
			return literalPointerExpression(value.L) && literalPointerExpression(value.R)
		default:
			return false
		}
	case *LoadExpr:
		literal, integer := value.Ptr.(*IntLitExpr)
		_, constant := value.Ptr.(*ConstPtrExpr)
		return (integer && mir2.IsPointer(literal.Ty)) || constant
	default:
		return false
	}
}

// Memory effects are attached at emission, after evaluation of the address
// and value. Pure address arithmetic retains its usual optimization freedom.
func (l *lowerer) loadMemory(ptr mir2.Reg, ty mir2.Ty, cls mir2.RegClass, volatile bool) mir2.Reg {
	r := l.bld.Load(ptr, ty, cls)
	l.bld.Cur.Insts[len(l.bld.Cur.Insts)-1].Volatile = volatile
	return r
}
func (l *lowerer) storeMemory(ptr, val mir2.Reg, ty mir2.Ty, volatile bool) {
	l.bld.Store(ptr, val, ty)
	l.bld.Cur.Insts[len(l.bld.Cur.Insts)-1].Volatile = volatile
}

func (l *lowerer) volatileObject(e Expr) bool {
	switch x := e.(type) {
	case *VarRefExpr:
		return x.Volatile || l.volatileAddress(e)
	case *DerefExpr:
		return x.Volatile || l.volatileAddress(x.Ptr)
	case *LoadExpr:
		return x.Volatile || l.volatileAddress(x.Ptr)
	case *FieldExpr:
		return x.Volatile || l.volatileObject(x.X)
	case *IndexExpr:
		return x.Volatile || l.volatileObject(x.Base)
	case *CastExpr:
		return l.volatileObject(x.X)
	}
	return l.volatileAddress(e)
}

// A C volatile aggregate imported into another frontend keeps its storage
// contract even when that frontend has no volatile spelling of its own.
func (l *lowerer) volatileAddress(e Expr) bool {
	switch x := e.(type) {
	case *AddrOfExpr:
		if g, ok := l.findGlobal(x.Sym); ok && g.Volatile {
			return true
		}
		return x.X != nil && l.volatileAddress(x.X)
	case *VarRefExpr:
		if g, ok := l.findGlobal(x.Name); ok && g.Volatile {
			switch g.Ty.(type) {
			case *mir2.ArrayTy, *mir2.StructTy:
				return true
			}
		}
	case *FieldExpr:
		return l.volatileAddress(x.X)
	case *IndexExpr:
		return l.volatileAddress(x.Base)
	case *CastExpr:
		return l.volatileAddress(x.X)
	}
	return false
}
