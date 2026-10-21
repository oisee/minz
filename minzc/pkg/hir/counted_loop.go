package hir

import "github.com/minz/minzc/pkg/mir2"

// countedValue removes only value-preserving unsigned integer promotions.
func countedValue(e Expr) Expr {
	for {
		c, ok := e.(*CastExpr)
		if ok && c.Ty == c.X.ExprTy() {
			e = c.X
			continue
		}
		if !ok || !mir2.IsInt(c.Ty) || !mir2.IsInt(c.X.ExprTy()) || mir2.IsSigned(c.X.ExprTy()) || c.Ty.Width() <= c.X.ExprTy().Width() {
			return e
		}
		e = c.X
	}
}

// tryCountedLoop accepts a trailing +1 with invariant endpoints. Uncertain
// control flow, memory bounds, aliasing, signed ranges and wide counts keep
// while lowering. Variable bounds must already be immutable SSA values.
// It never rewrites source HIR, which may be reused by another compilation.
func (l *lowerer) tryCountedLoop(st *WhileStmt) bool {
	cmp, ok := st.Cond.(*BinExpr)
	if !ok || (cmp.Op != "<" && cmp.Op != "!=") || len(st.Body.Body) == 0 {
		return false
	}
	iv, ok := countedValue(cmp.L).(*VarRefExpr)
	if !ok || iv.Volatile || l.localCells[iv.Name] || l.addressTaken[iv.Name] {
		return false
	}
	initial, local := l.env[iv.Name]
	ty := l.envTy[iv.Name]
	if !local || iv.Ty != ty || (ty != mir2.TyU8 && ty != mir2.TyU16) {
		return false
	}
	// A range binder's declared type can differ from its start expression.
	// Do not reinterpret a narrow start register as a wide comparison operand.
	if !l.countedRegHasType(initial, ty) {
		return false
	}
	bound := countedValue(cmp.R)
	switch b := bound.(type) {
	case *IntLitExpr:
		max := int64(255)
		if ty == mir2.TyU16 {
			max = 65535
		}
		if !mir2.IsInt(b.Ty) {
			return false
		}
		bits := b.Ty.Width()
		if mir2.IsSigned(b.Ty) {
			bits--
		}
		if bits < 16 && b.Val >= int64(1)<<bits {
			return false
		}
		if b.Val < 0 || b.Val > max {
			return false
		}
	case *VarRefExpr:
		boundReg, localBound := l.env[b.Name]
		if !localBound || !l.countedRegHasType(boundReg, ty) {
			return false
		}
		if b.Volatile || b.Ty != ty || b.Name == iv.Name || l.localCells[b.Name] || l.addressTaken[b.Name] {
			return false
		}
	default:
		return false
	}
	if mir2.IsSigned(cmp.L.ExprTy()) && cmp.L.ExprTy().Width() <= ty.Width() {
		return false
	}
	if mir2.IsSigned(cmp.R.ExprTy()) && cmp.R.ExprTy().Width() <= ty.Width() {
		if _, literal := bound.(*IntLitExpr); !literal {
			return false
		}
	}
	n := len(st.Body.Body)
	step, ok := st.Body.Body[n-1].(*AssignStmt)
	if !ok {
		return false
	}
	target, ok := step.Target.(*VarRefExpr)
	if !ok || target.Name != iv.Name || target.Volatile || target.Ty != ty {
		return false
	}
	value := step.Val
	if c, ok := value.(*CastExpr); ok && c.Ty == ty {
		value = c.X
	}
	add, ok := value.(*BinExpr)
	if !ok || add.Op != "+" || !mir2.IsInt(add.Ty) || add.Ty.Width() < ty.Width() {
		return false
	}
	one, ok := add.R.(*IntLitExpr)
	if !ok || one.Val != 1 {
		return false
	}
	lhs, ok := countedValue(add.L).(*VarRefExpr)
	if !ok || lhs.Ty != ty {
		return false
	}
	// C postfix i++ snapshots i before assigning snapshot+1.
	if lhs.Name != iv.Name {
		if n < 2 {
			return false
		}
		snap, ok := st.Body.Body[n-2].(*VarDeclStmt)
		if !ok || snap.Name != lhs.Name || snap.Ty != ty || snap.Volatile {
			return false
		}
		src, ok := snap.Init.(*VarRefExpr)
		if !ok || src.Name != iv.Name || src.Volatile || src.Ty != ty {
			return false
		}
		n--
	}
	body := &Block{Body: st.Body.Body[:n-1]}
	boundName := ""
	if b, ok := bound.(*VarRefExpr); ok {
		boundName = b.Name
	}
	// Includes bit-field assignments, whose root target is the scalar binder.
	muts := scanMutations(body, l.env)
	if muts[iv.Name] || muts[boundName] {
		return false
	}
	safe := true
	references := map[string]bool{}
	walkGraceBlocks(body, "", func(_ string, block *Block) {
		for _, s := range block.Body {
			switch x := s.(type) {
			case *Block, *IfStmt, *WhileStmt:
			case *ForRangeStmt:
				if x.Var == iv.Name || x.Var == boundName {
					safe = false
				}
			case *ForEachStmt:
				if x.Var == iv.Name || x.Var == boundName {
					safe = false
				}
			case *VarDeclStmt:
				// Shadowing needs binding-aware mutation proofs.
				if x.Name == iv.Name || x.Name == boundName || x.Volatile {
					safe = false
				}
			case *AssignStmt:
				if v, ok := x.Target.(*VarRefExpr); ok {
					if v.Name == iv.Name || v.Name == boundName || v.Volatile {
						safe = false
					}
				}
			case *ExprStmt:
			default:
				safe = false
			}
			visitStmtExprs(s, func(e Expr) {
				switch x := e.(type) {
				case *CallExpr, *CallIndirectExpr, *LetInExpr, *GenNextExpr, *GenMakeExpr:
					safe = false
				case *VarRefExpr:
					if _, exists := l.env[x.Name]; exists {
						references[x.Name] = true
					}
					if x.Volatile {
						safe = false
					}
				case *LoadExpr:
					if x.Volatile {
						safe = false
					}
				case *IndexExpr:
					// Preserve the existing MIR counted-pointer optimisation.
					if exprUsesVar(x.Idx, iv.Name) {
						safe = false
					}
					if x.Volatile {
						safe = false
					}
				}
			})
		}
	})
	if !safe {
		return false
	}
	// Dynamic trip counts introduce an entry subtraction, a second live
	// counter and a zero-trip merge. Their allocation/copy costs can exceed
	// the saved compare even when the source IV is unused. Until lowering
	// can price those costs, retain ordinary while lowering. Literal bounds
	// keep the constant-count path used by fixed-count bit loops.
	if _, fixed := bound.(*IntLitExpr); !fixed {
		b, ok := bound.(*VarRefExpr)
		if !ok {
			return false
		}
		if _, known := l.countedEndpointConstant(l.env[b.Name]); !known {
			return false
		}
	}
	if _, known := l.countedEndpointConstant(initial); !known {
		return false
	}
	// Retaining a separately incremented IV removes much of DJNZ's benefit
	// and adds register pressure. Only replace loops whose body ignores it.
	if references[iv.Name] {
		return false
	}
	var count, hiReg mir2.Reg
	if ty == mir2.TyU16 {
		// Literal endpoints prove a non-wrapping distance no greater than 255.
		hi, known := int64(0), false
		switch b := bound.(type) {
		case *IntLitExpr:
			hi, known = b.Val, true
		case *VarRefExpr:
			hi, known = l.countedConstant(l.env[b.Name])
		}
		lo, loKnown := l.countedConstant(initial)
		if !known || !loKnown || lo < 0 || hi > 65535 || hi < lo || hi-lo > 255 {
			return false
		}
		count = l.bld.Const(hi-lo, mir2.TyU8, mir2.ClassCounter)
		hiReg = l.bld.Const(hi, ty, classForExpr(ty))
	} else {
		hiReg = l.lowerExprAs(bound, ty)
		count = l.bld.Sub(hiReg, initial, ty, mir2.ClassCounter)
	}
	envBefore := cloneMap(l.env)
	// The complete expression walker also sees conditional/aggregate operands
	// and indexed assignment targets. The older use scanner omits those.
	used := references
	for name := range muts {
		used[name] = true
	}
	// If the body ignores i, reconstruct its final value on the taken exit.
	// This avoids a dead cyclic +1 recurrence that local DCE cannot eliminate.
	usesIV := used[iv.Name]
	delete(used, iv.Name)
	carried := sortedKeys(used, envBefore)
	bodyLabel, exitLabel := l.fresh("count_body"), l.fresh("count_exit")
	pred := mir2.CmpUlt
	if cmp.Op == "!=" {
		pred = mir2.CmpNe
	}
	cond := l.bld.Cmp(pred, initial, hiReg, mir2.ClassFlag, false)
	args := append([]mir2.Reg{count}, l.collectArgs(carried)...)
	if usesIV {
		args = append(args, initial)
	}
	zeroExit := append(l.collectArgs(carried), initial)
	l.bld.BrIf(cond, bodyLabel, args, exitLabel, zeroExit)
	loop := l.bld.SwitchToNewBlock(bodyLabel)
	if origin, ok := l.lineage[st]; ok {
		if l.mf.LoopOrigins == nil {
			l.mf.LoopOrigins = map[string]mir2.LoopOrigin{}
		}
		l.mf.LoopOrigins[bodyLabel] = mir2.LoopOrigin{SiteID: origin.SiteID, Role: origin.Role, Rule: origin.Rule}
	}
	cnt := l.bld.BlockParam(loop, mir2.TyU8, mir2.ClassCounter)
	env := cloneMap(envBefore)
	for _, name := range carried {
		env[name] = l.bld.BlockParam(loop, l.envTy[name], classForLoop(l.envTy[name], name))
	}
	if usesIV {
		env[iv.Name] = l.bld.BlockParam(loop, ty, classForLoop(ty, iv.Name))
	}
	l.env = env
	previous := l.inLoopContext
	l.inLoopContext = true
	l.lowerBlock(body)
	l.inLoopContext = previous
	finalIV := hiReg
	if usesIV {
		oneReg := l.bld.Const(1, ty, classForExpr(ty))
		finalIV = l.bld.Add(l.env[iv.Name], oneReg, ty, classForLoop(ty, iv.Name))
	}
	updates := l.collectArgs(carried)
	back := append([]mir2.Reg{}, updates...)
	if usesIV {
		back = append(back, finalIV)
	}
	exitArgs := append(updates, finalIV)
	l.bld.DJNZ(cnt, bodyLabel, back, exitLabel, exitArgs)
	exit := l.bld.SwitchToNewBlock(exitLabel)
	env = cloneMap(envBefore)
	for _, name := range carried {
		env[name] = l.bld.BlockParam(exit, l.envTy[name], classForExpr(l.envTy[name]))
	}
	env[iv.Name] = l.bld.BlockParam(exit, ty, classForExpr(ty))
	l.env = env
	return true
}

// Inspect only direct MIR constants; arithmetic and unknown block parameters
// require a separate range proof and deliberately retain the ordinary path.
func (l *lowerer) countedConstant(r mir2.Reg) (int64, bool) {
	if r == mir2.NoReg {
		return 0, false
	}
	for _, block := range l.bld.F.Blocks {
		for _, inst := range block.Insts {
			if inst.Dst == r && inst.Op == mir2.OpConst {
				return inst.Imm, true
			}
		}
	}
	return 0, false
}

func (l *lowerer) countedRegHasType(r mir2.Reg, ty mir2.Ty) bool {
	for _, p := range l.bld.F.Contract.Params {
		if p.Reg == r {
			return p.Ty == ty
		}
	}
	for _, b := range l.bld.F.Blocks {
		for _, p := range b.Params {
			if p.Dst == r {
				return p.Ty == ty
			}
		}
		for _, inst := range b.Insts {
			if inst.Dst == r {
				return inst.Ty == ty
			}
		}
	}
	return false
}

// countedEndpointConstant follows only conversions that preserve a known,
// nonnegative integer value. C initializers commonly lower 0 as trunc(i16 0).
// A wrapping truncation, negative value or sign-changing extension is refused.
func (l *lowerer) countedEndpointConstant(r mir2.Reg) (int64, bool) {
	for _, block := range l.bld.F.Blocks {
		for _, inst := range block.Insts {
			if inst.Dst != r || !mir2.IsInt(inst.Ty) {
				continue
			}
			bits := inst.Ty.Width()
			if mir2.IsSigned(inst.Ty) {
				bits--
			}
			if bits <= 0 || bits > 32 {
				return 0, false
			}
			v := inst.Imm
			switch inst.Op {
			case mir2.OpConst:
			case mir2.OpTrunc, mir2.OpExt, mir2.OpMove:
				var known bool
				v, known = l.countedEndpointConstant(inst.Src[0])
				if !known {
					return 0, false
				}
			default:
				return 0, false
			}
			return v, v >= 0 && v < int64(1)<<bits
		}
	}
	return 0, false
}
