package hir

import "github.com/minz/minzc/pkg/mir2"

// Full unrolling of short constant-trip loops (speed profile only).
//
// The unroll is performed during lowering, from the source WhileStmt whose
// exact trip count staticLoopFact has already proved: constant unsigned
// endpoints, a trailing +1 step, no other write to the induction variable and
// no break/continue/return/goto/label or call in the body. The body is lowered
// N times in order into the current straight-line position. Each copy sees the
// induction variable bound to its constant; registers, labels and SSA joins of
// the body's own if statements are fresh per copy because the ordinary
// lowerer creates them. No loop, counter, StaticLoops or LoopOrigins entry is
// created, so loop metadata describes only the loops that remain.
//
// Lowering the same statements N times preserves the order and the number of
// every memory access, including volatile ones: copy k's accesses are emitted
// after copy k-1's and before copy k+1's, exactly as the iterations ran.
//
// Profitability (each a named constant): never under OptSize (the pipeline
// does not request unrolling there); at most UnrollMaxTrips iterations; only
// inside another loop. The byte budgets are measured, not estimated: the
// pipeline compiles the module once more without unrolling and keeps the
// unrolled product only when each function's emitted growth is within
// UnrollMaxGrowthPerLoop × loops and UnrollMaxGrowthPerFunction, and the
// variant search's static T model scores the function strictly faster
// (pipeline/unroll_gate.go). With several unrolled loops in one function the
// per-loop budget is enforced on their sum, not loop by loop.
const (
	UnrollMaxTrips             = 8
	UnrollMaxGrowthPerLoop     = 48
	UnrollMaxGrowthPerFunction = 96
	// UnrollRequireNested admits only loops nested in another loop, where the
	// saved counter overhead is multiplied by the outer trip count.
	UnrollRequireNested = true
)

// tryUnrollLoop returns true after it has lowered st completely.
func (l *lowerer) tryUnrollLoop(st *WhileStmt, fact mir2.StaticLoop) bool {
	if !l.unrollLoops || fact.Trips < 1 || fact.Trips > UnrollMaxTrips {
		return false
	}
	if UnrollRequireNested && !l.inLoopContext {
		return false
	}
	if l.hf == nil || l.hf.FallibleCY || l.funcHasAsmOrSMC() {
		return false
	}
	if _, measured := l.lineage[st]; measured {
		return false // measured loop sites keep their identity
	}
	cmp, ok := st.Cond.(*BinExpr)
	if !ok {
		return false
	}
	iv, ok := countedValue(cmp.L).(*VarRefExpr)
	if !ok {
		return false
	}
	lo, known := l.countedEndpointConstant(l.env[iv.Name])
	if !known {
		return false
	}
	// staticLoopFact proved a trailing step, optionally preceded by the C
	// postfix snapshot declaration. Neither is part of the unrolled body.
	n := len(st.Body.Body)
	step := st.Body.Body[n-1].(*AssignStmt)
	value := step.Val
	if c, ok := value.(*CastExpr); ok && c.Ty == iv.Ty {
		value = c.X
	}
	src := countedValue(value.(*BinExpr).L).(*VarRefExpr)
	n--
	if src.Name != iv.Name {
		n--
	}
	body := &Block{Body: st.Body.Body[:n]}
	if !unrollableBlock(body) {
		return false
	}
	ty := iv.Ty
	envBefore := cloneMap(l.env)
	for k := int64(0); k < fact.Trips; k++ {
		l.env[iv.Name] = l.bld.Const(lo+k, ty, classForExpr(ty))
		if l.lowerBlock(body) {
			// unrollableBlock admits no terminating statement.
			panic("hir/lower: unrolled loop body terminated")
		}
	}
	// Declarations of the body are scoped to it (lowerBlock restores them);
	// only bindings visible before the loop survive, with their final values.
	for name := range l.env {
		if _, visible := envBefore[name]; !visible {
			delete(l.env, name)
		}
	}
	l.env[iv.Name] = l.bld.Const(lo+fact.Trips, ty, classForExpr(ty))
	if l.unrolled != nil {
		l.unrolled[l.hf.Name]++
	}
	return true
}

// unrollableBlock admits straight-line statements, if statements and blocks
// whose expressions are pure, call-free and non-allocating. Nested loops,
// switch, asm, stores of aggregates, tuple lets and generator forms refuse.
func unrollableBlock(b *Block) bool {
	if b == nil {
		return true
	}
	for _, s := range b.Body {
		switch x := s.(type) {
		case *Block:
			if !unrollableBlock(x) {
				return false
			}
		case *IfStmt:
			if !unrollableExpr(x.Cond) || !unrollableBlock(x.Then) || !unrollableBlock(x.Else) {
				return false
			}
		case *AssignStmt:
			if !unrollableExpr(x.Target) || !unrollableExpr(x.Val) {
				return false
			}
		case *ExprStmt:
			if !unrollableExpr(x.Expr) {
				return false
			}
		case *StoreStmt:
			if !unrollableExpr(x.Ptr) || !unrollableExpr(x.Val) {
				return false
			}
		case *VarDeclStmt:
			if x.Volatile || x.ArrayLen != 0 || x.At != nil || len(x.Initial) != 0 || !mir2.IsInt(x.Ty) && x.Ty != mir2.TyBool {
				return false
			}
			if x.Init != nil && !unrollableExpr(x.Init) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func unrollableExpr(e Expr) bool {
	ok := true
	walkExpr(e, func(x Expr) {
		switch x.(type) {
		case *IntLitExpr, *BoolLitExpr, *VarRefExpr, *BinExpr, *UnaryExpr, *FieldExpr,
			*BitExpr, *LoadExpr, *DerefExpr, *CastExpr, *ConstPtrExpr, *IndexExpr, *CondExpr:
		default:
			ok = false
		}
	})
	return ok
}

// funcHasAsmOrSMC refuses functions with inline assembly anywhere or with
// self-modifying parameters: their code may be addressed or patched by label.
func (l *lowerer) funcHasAsmOrSMC() bool {
	if l.funcAsmKnown {
		return l.funcAsm
	}
	l.funcAsmKnown = true
	for _, p := range l.hf.Params {
		if p.SMC {
			l.funcAsm = true
		}
	}
	var visit func(Stmt)
	visit = func(s Stmt) {
		switch x := s.(type) {
		case *AsmStmt:
			l.funcAsm = true
		case *Block:
			if x != nil {
				for _, c := range x.Body {
					visit(c)
				}
			}
		case *IfStmt:
			visit(x.Then)
			if x.Else != nil {
				visit(x.Else)
			}
		case *WhileStmt:
			visit(x.Body)
		case *ForRangeStmt:
			visit(x.Body)
		case *ForEachStmt:
			visit(x.Body)
		case *GenForStmt:
			visit(x.Body)
		case *SwitchStmt:
			for _, c := range x.Cases {
				visit(c.Body)
			}
			if x.Default != nil {
				visit(x.Default)
			}
		}
	}
	if l.hf.Body != nil {
		visit(l.hf.Body)
	}
	return l.funcAsm
}
