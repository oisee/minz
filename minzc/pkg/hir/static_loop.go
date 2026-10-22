package hir

import "github.com/minz/minzc/pkg/mir2"

// CountedLoopPolicy chooses profitability policy, independently of legality.
type CountedLoopPolicy uint8

const (
	CountedDefault CountedLoopPolicy = iota
	CountedOrdinary
	CountedPermit
)

func (l *lowerer) recordStaticLoop(header string, fact mir2.StaticLoop) {
	if l.mf.StaticLoops == nil {
		l.mf.StaticLoops = map[string]mir2.StaticLoop{}
	}
	l.mf.StaticLoops[header] = fact
}

// staticLoopFact proves only a constant, unsigned, non-wrapping +1 recurrence.
// Early transfers, shadowing, additional writes and aliasing refuse the proof.
// A refused fact remains an unknown-trip loop; it never changes lowering.
func (l *lowerer) staticLoopFact(st *WhileStmt) mir2.StaticLoop {
	fact := mir2.StaticLoop{ID: l.loopCostIndex, Condition: dumpExpr(st.Cond), Trips: -1}
	l.loopCostIndex++
	cmp, ok := st.Cond.(*BinExpr)
	if !ok || cmp.Op != "<" && cmp.Op != "!=" || len(st.Body.Body) == 0 {
		return fact
	}
	iv, ok := countedValue(cmp.L).(*VarRefExpr)
	if !ok || iv.Volatile || l.localCells[iv.Name] || l.addressTaken[iv.Name] || iv.Ty != mir2.TyU8 && iv.Ty != mir2.TyU16 {
		return fact
	}
	if !l.countedRegHasType(l.env[iv.Name], iv.Ty) {
		return fact
	}
	lo, known := l.countedEndpointConstant(l.env[iv.Name])
	if !known {
		return fact
	}
	var hi int64
	switch b := countedValue(cmp.R).(type) {
	case *IntLitExpr:
		if !mir2.IsInt(b.Ty) {
			return fact
		}
		bits := b.Ty.Width()
		if mir2.IsSigned(b.Ty) {
			bits--
		}
		if b.Val < 0 || bits < 32 && b.Val >= int64(1)<<bits {
			return fact
		}
		hi = b.Val
	case *VarRefExpr:
		if b.Ty != iv.Ty || b.Volatile || l.localCells[b.Name] || l.addressTaken[b.Name] {
			return fact
		}
		hi, known = l.countedEndpointConstant(l.env[b.Name])
	default:
		return fact
	}
	if !known || hi < lo || hi >= int64(1)<<iv.Ty.Width() {
		return fact
	}
	n := len(st.Body.Body)
	step, ok := st.Body.Body[n-1].(*AssignStmt)
	if !ok {
		return fact
	}
	target, ok := step.Target.(*VarRefExpr)
	if !ok || target.Name != iv.Name || target.Ty != iv.Ty || target.Volatile {
		return fact
	}
	value := step.Val
	if c, ok := value.(*CastExpr); ok && c.Ty == iv.Ty {
		value = c.X
	}
	add, ok := value.(*BinExpr)
	if !ok || add.Op != "+" || !mir2.IsInt(add.Ty) || add.Ty.Width() < iv.Ty.Width() {
		return fact
	}
	one, ok := add.R.(*IntLitExpr)
	if !ok || one.Val != 1 {
		return fact
	}
	src, ok := countedValue(add.L).(*VarRefExpr)
	if !ok || src.Ty != iv.Ty {
		return fact
	}
	if src.Name != iv.Name {
		if n < 2 {
			return fact
		}
		d, ok := st.Body.Body[n-2].(*VarDeclStmt)
		if !ok || d.Name != src.Name || d.Ty != iv.Ty || d.Volatile {
			return fact
		}
		v, ok := d.Init.(*VarRefExpr)
		if !ok || v.Name != iv.Name {
			return fact
		}
		n--
	}
	body := &Block{Body: st.Body.Body[:n-1]}
	muts := scanMutations(body, l.env)
	if muts[iv.Name] {
		return fact
	}
	if b, ok := countedValue(cmp.R).(*VarRefExpr); ok && muts[b.Name] {
		return fact
	}
	safe := true
	walkGraceBlocks(body, "", func(_ string, b *Block) {
		for _, s := range b.Body {
			switch s.(type) {
			case *BreakStmt, *ContinueStmt, *ReturnStmt, *GotoStmt, *LabelStmt:
				safe = false
			}
			if d, ok := s.(*VarDeclStmt); ok && d.Name == iv.Name {
				safe = false
			}
			visitStmtExprs(s, func(e Expr) {
				switch e.(type) {
				case *CallExpr, *CallIndirectExpr, *LetInExpr, *GenNextExpr, *GenMakeExpr:
					safe = false
				}
			})
		}
	})
	if safe {
		fact.Trips = hi - lo
	}
	return fact
}
