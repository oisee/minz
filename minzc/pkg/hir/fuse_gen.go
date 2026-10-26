package hir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// GeneratorFusionStats records admitted immediate-consumer sites. This first
// profile admits one use of a producer, whose body is a single while with a
// leading yield and parameter-only updates. All other generators stay pull.
type GeneratorFusionStats struct {
	Fused     int
	Producers []string
}

// FuseSimpleGenerators removes the private pull protocol for a narrowly
// admitted immediate consumer. Arguments are captured once, in source order;
// producer suffixes run after consumption, including consumer continue paths.
// Manual cursors, multiple consumers and unsupported control retain G1 lowering.
func FuseSimpleGenerators(m *Module) (GeneratorFusionStats, error) {
	var stats GeneratorFusionStats
	if err := Verify(m); err != nil {
		return stats, err
	}
	gens := map[string]*Func{}
	for _, f := range m.Funcs {
		if f.IsGen {
			gens[f.Name] = f
		}
	}
	if len(gens) == 0 {
		return stats, nil
	}
	if len(m.LoopLineage) != 0 || len(m.OrderedByteRWPolicy) != 0 || len(m.OrderedBytePolicy) != 0 || len(m.RegionComparePolicy) != 0 || len(m.InvariantBasePolicy) != 0 || len(m.GraceRulePolicy) != 0 || len(m.ForwardCopyPolicy) != 0 || len(m.PointerWalkPolicy) != 0 {
		return stats, fmt.Errorf("generator fusion cannot yet rewrite explicit HIR loop policies")
	}
	uses := map[string]int{}
	manual := map[string]bool{}
	for _, f := range m.Funcs {
		fusionWalk(f.Body, func(s Stmt) {
			if st, ok := s.(*GenForStmt); ok {
				uses[st.Gen]++
				for _, a := range st.Args {
					walkExpr(a, func(e Expr) { fusionReference(e, gens, manual) })
				}
			}
			visitStmtExprs(s, func(e Expr) { fusionReference(e, gens, manual) })
		})
	}
	reserved := map[string]bool{}
	for _, g := range m.Globals {
		reserved[g.Name] = true
	}
	for _, f := range m.Funcs {
		reserved[f.Name] = true
	}
	removed := map[string]bool{}
	serial := 0
	for _, f := range m.Funcs {
		if f.IsGen || f.Body == nil {
			continue
		}
		used := map[string]bool{}
		for _, p := range f.Params {
			used[p.Name] = true
		}
		for _, p := range f.Captures {
			used[p.Name] = true
		}
		fusionWalk(f.Body, func(s Stmt) {
			switch x := s.(type) {
			case *VarDeclStmt:
				used[x.Name] = true
			case *TupleLetStmt:
				for _, name := range x.Names {
					used[name] = true
				}
			case *ForRangeStmt:
				used[x.Var] = true
			case *ForEachStmt:
				used[x.Var] = true
			case *GenForStmt:
				used[x.Var] = true
			}
			visitStmtExprs(s, func(e Expr) {
				if x, ok := e.(*LetInExpr); ok {
					used[x.Name] = true
				}
			})
		})
		fresh := func(role string) string {
			for {
				serial++
				name := fmt.Sprintf("$gen$fused$%d$%s", serial, role)
				if !used[name] && !reserved[name] {
					used[name] = true
					return name
				}
			}
		}
		var rewrite func(*Block)
		rewrite = func(b *Block) {
			if b == nil {
				return
			}
			for i, s := range b.Body {
				if st, ok := s.(*GenForStmt); ok {
					g := gens[st.Gen]
					if g != nil && uses[g.Name] == 1 && !manual[g.Name] && fusionProducer(g) && fusionFreeBindingsSafe(g, used) && fusionConsumer(st.Body) && len(st.Args) == len(g.Params) && st.ElemTy == g.ElemTy {
						fused, loop := fuseGeneratorFor(g, st, fresh)
						b.Body[i] = fused
						removed[g.Name] = true
						stats.Fused++
						stats.Producers = append(stats.Producers, g.Name)
						if loc, ok := m.SourceLocations[st]; ok {
							m.SourceLocations[loop] = loc
						}
						continue
					}
				}
				fusionChildren(s, rewrite)
			}
		}
		rewrite(f.Body)
	}
	// A generator declaration has no directly callable runtime ABI. Removing
	// exactly the producer whose sole immediate use fused prevents emitting an
	// unreachable $next helper; unrelated unused declarations remain unchanged.
	kept := m.Funcs[:0]
	for _, f := range m.Funcs {
		if !removed[f.Name] {
			kept = append(kept, f)
		}
	}
	m.Funcs = kept
	return stats, Verify(m)
}

func fusionReference(e Expr, gens map[string]*Func, manual map[string]bool) {
	switch x := e.(type) {
	case *GenMakeExpr:
		manual[x.Gen] = true
	case *GenNextExpr:
		manual[x.Gen] = true
	case *CallExpr:
		if gens[x.Fn] != nil {
			manual[x.Fn] = true
		}
	case *AddrOfExpr:
		if gens[x.Sym] != nil {
			manual[x.Sym] = true
		}
	case *VarRefExpr:
		if gens[x.Name] != nil {
			manual[x.Name] = true
		}
	}
}
func fusionChildren(s Stmt, visit func(*Block)) {
	switch x := s.(type) {
	case *Block:
		visit(x)
	case *IfStmt:
		visit(x.Then)
		visit(x.Else)
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
		visit(x.Default)
	}
}
func fusionWalk(b *Block, visit func(Stmt)) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		visit(s)
		fusionChildren(s, func(child *Block) { fusionWalk(child, visit) })
	}
}

// Explicit expression coverage makes new/suspending/assembly nodes refuse
// admission. Calls and volatile memory accesses keep their original order.
func fusionExpr(e Expr) bool {
	if e == nil {
		return true
	}
	good := true
	walkExpr(e, func(n Expr) {
		switch n.(type) {
		case *IntLitExpr, *BoolLitExpr, *ConstPtrExpr, *VarRefExpr, *BinExpr, *UnaryExpr, *CallExpr, *CallIndirectExpr, *FieldExpr, *BitExpr, *AddrOfExpr, *LoadExpr, *DerefExpr, *CastExpr, *IndexExpr, *CondExpr:
		default:
			good = false
		}
	})
	return good
}
func fusionProducer(g *Func) bool {
	if g.ElemTy != mir2.TyU8 && g.ElemTy != mir2.TyI8 && g.ElemTy != mir2.TyU16 && g.ElemTy != mir2.TyI16 && g.ElemTy != mir2.TyBool {
		return false
	}
	if g.Body == nil || g.IsExtern || len(g.Captures) != 0 || len(g.Body.Body) != 1 {
		return false
	}
	loop, ok := g.Body.Body[0].(*WhileStmt)
	if !ok || loop.Body == nil || len(loop.Body.Body) < 1 || len(loop.Body.Body) > 5 || !fusionExpr(loop.Cond) {
		return false
	}
	yield, ok := loop.Body.Body[0].(*YieldStmt)
	if !ok || yield.Val == nil || yield.Val.ExprTy() != g.ElemTy || !fusionExpr(yield.Val) {
		return false
	}
	params := map[string]bool{}
	for _, p := range g.Params {
		if p.SMC || p.Ty == nil || (!mir2.IsPointer(p.Ty) && p.Ty != mir2.TyU8 && p.Ty != mir2.TyU16 && p.Ty != mir2.TyI8 && p.Ty != mir2.TyI16 && p.Ty != mir2.TyBool) {
			return false
		}
		params[p.Name] = true
	}
	for _, s := range loop.Body.Body[1:] {
		a, ok := s.(*AssignStmt)
		if !ok {
			return false
		}
		v, ok := a.Target.(*VarRefExpr)
		if !ok || !params[v.Name] || !fusionExpr(a.Val) {
			return false
		}
	}
	// Addressing producer-owned state would expose its pull frame identity.
	safe := true
	check := func(e Expr) {
		if a, ok := e.(*AddrOfExpr); ok {
			if params[a.Sym] {
				safe = false
			}
			if v, ok := a.X.(*VarRefExpr); ok && params[v.Name] {
				safe = false
			}
		}
	}
	walkExpr(loop.Cond, check)
	walkExpr(yield.Val, check)
	for _, s := range loop.Body.Body[1:] {
		visitStmtExprs(s, check)
	}
	return safe
}
func fusionConsumer(b *Block) bool {
	if b == nil {
		return false
	}
	good := true
	fusionWalk(b, func(s Stmt) {
		switch x := s.(type) {
		case *Block, *IfStmt, *WhileStmt, *ForRangeStmt, *ForEachStmt, *AssignStmt, *StoreStmt, *ExprStmt, *ReturnStmt, *BreakStmt, *ContinueStmt:
		case *VarDeclStmt:
			if x.ArrayLen != 0 || len(x.Initial) != 0 {
				good = false
			}
		default:
			good = false
		}
		visitStmtExprs(s, func(e Expr) {
			if !fusionExpr(e) {
				good = false
			}
		})
	})
	return good
}
func fusionClone(b *Block) *Block {
	return CloneModule(&Module{Funcs: []*Func{{Body: b}}}).Funcs[0].Body
}
func fuseGeneratorFor(g *Func, site *GenForStmt, fresh func(string) string) (*Block, *WhileStmt) {
	params := map[string]string{}
	var setup []Stmt
	for i, p := range g.Params {
		name := fresh(p.Name)
		params[p.Name] = name
		setup = append(setup, &VarDeclStmt{Name: name, Ty: p.Ty, Init: site.Args[i]})
	}
	producer := fusionClone(g.Body)
	rename := func(e Expr) Expr {
		if v, ok := e.(*VarRefExpr); ok {
			if name, found := params[v.Name]; found {
				v.Name = name
			}
		}
		return e
	}
	// mapStmtExprs intentionally does not rewrite YieldStmt; handle its value
	// explicitly before removing that node.
	mapBlockExprs(producer, rename)
	loop := producer.Body[0].(*WhileStmt)
	yield := loop.Body.Body[0].(*YieldStmt)
	value := mapExprChildren(yield.Val, rename)
	suffix := &Block{Body: loop.Body.Body[1:]}
	consumer := fusionClone(site.Body)
	rewriteFusionContinue(consumer, suffix, 0)
	body := []Stmt{&AssignStmt{Target: &VarRefExpr{Name: site.Var, Ty: site.ElemTy}, Val: value}}
	body = append(body, consumer.Body...)
	body = append(body, suffix.Body...)
	loop.Body = &Block{Body: body}
	setup = append(setup, &VarDeclStmt{Name: site.Var, Ty: site.ElemTy, Init: &IntLitExpr{Val: 0, Ty: site.ElemTy}}, loop)
	return &Block{Body: setup}, loop
}
func rewriteFusionContinue(b, suffix *Block, depth int) {
	for i, s := range b.Body {
		switch x := s.(type) {
		case *ContinueStmt:
			if depth == 0 {
				copy := fusionClone(suffix)
				copy.Body = append(copy.Body, &ContinueStmt{})
				b.Body[i] = copy
			}
		case *Block:
			rewriteFusionContinue(x, suffix, depth)
		case *IfStmt:
			rewriteFusionContinue(x.Then, suffix, depth)
			if x.Else != nil {
				rewriteFusionContinue(x.Else, suffix, depth)
			}
		case *WhileStmt:
			rewriteFusionContinue(x.Body, suffix, depth+1)
		case *ForRangeStmt:
			rewriteFusionContinue(x.Body, suffix, depth+1)
		case *ForEachStmt:
			rewriteFusionContinue(x.Body, suffix, depth+1)
		}
	}
}

// HIR names do not yet carry resolved binding identities. A free producer
// global/function name must not become a caller local after splicing. Refuse
// whole-function binding collisions, even if some are out of lexical scope.
func fusionFreeBindingsSafe(g *Func, callerBindings map[string]bool) bool {
	params := map[string]bool{}
	for _, p := range g.Params {
		params[p.Name] = true
	}
	safe := true
	check := func(e Expr) {
		switch x := e.(type) {
		case *VarRefExpr:
			if !params[x.Name] && callerBindings[x.Name] {
				safe = false
			}
		case *CallExpr:
			if callerBindings[x.Fn] || params[x.Fn] {
				safe = false
			}
		case *AddrOfExpr:
			if !params[x.Sym] && callerBindings[x.Sym] {
				safe = false
			}
		}
	}
	loop := g.Body.Body[0].(*WhileStmt)
	walkExpr(loop.Cond, check)
	walkExpr(loop.Body.Body[0].(*YieldStmt).Val, check)
	for _, s := range loop.Body.Body[1:] {
		visitStmtExprs(s, check)
	}
	return safe
}
