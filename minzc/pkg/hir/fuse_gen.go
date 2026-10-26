package hir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// GeneratorFusionStats records admitted immediate-consumer sites. This first
// profile admits one use of a producer with up to two nested whiles, a
// leading inner yield and private scalar updates. Other generators stay pull.
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
					if g != nil && uses[g.Name] == 1 && !manual[g.Name] && fusionProducer(g, reserved) && fusionFreeBindingsSafe(g, used) && fusionConsumer(st.Body) && len(st.Args) == len(g.Params) && st.ElemTy == g.ElemTy {
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

// fusionScalar excludes aggregate storage whose identity could escape.
func fusionScalar(ty mir2.Ty) bool {
	return ty == mir2.TyU8 || ty == mir2.TyI8 || ty == mir2.TyU16 || ty == mir2.TyI16 || ty == mir2.TyBool
}
func fusionProducer(g *Func, reserved map[string]bool) bool {
	if !fusionScalar(g.ElemTy) || g.Body == nil || g.IsExtern || len(g.Captures) != 0 || len(g.Body.Body) != 1 {
		return false
	}
	root, ok := g.Body.Body[0].(*WhileStmt)
	if !ok {
		return false
	}
	owned := map[string]bool{}
	available := map[string]bool{}
	for _, p := range g.Params {
		if p.SMC || p.Ty == nil || (!mir2.IsPointer(p.Ty) && !fusionScalar(p.Ty)) {
			return false
		}
		owned[p.Name], available[p.Name] = true, true
	}
	valid := true
	// Unique local names also avoid ambiguous free-global bindings before a
	// declaration. Local declarations are admitted only before the inner loop.
	fusionWalk(g.Body, func(s Stmt) {
		if v, ok := s.(*VarDeclStmt); ok {
			if v.Volatile || v.At != nil || owned[v.Name] || reserved[v.Name] || !fusionScalar(v.Ty) || v.ArrayLen != 0 || len(v.Initial) != 0 || v.Init == nil {
				valid = false
			}
			owned[v.Name] = true
		}
	})
	expr := func(e Expr, scope map[string]bool) bool {
		good := fusionExpr(e)
		walkExpr(e, func(e Expr) {
			switch x := e.(type) {
			case *VarRefExpr:
				if owned[x.Name] && !scope[x.Name] {
					good = false
				}
			case *CallExpr:
				if owned[x.Fn] {
					good = false
				}
			case *AddrOfExpr:
				if owned[x.Sym] {
					good = false
				}
				if v, ok := x.X.(*VarRefExpr); ok && owned[v.Name] {
					good = false
				}
			}
		})
		return good
	}
	assign := func(s Stmt, scope map[string]bool) bool {
		a, ok := s.(*AssignStmt)
		if !ok {
			return false
		}
		v, ok := a.Target.(*VarRefExpr)
		return ok && scope[v.Name] && expr(a.Val, scope)
	}
	var loopOK func(*WhileStmt, map[string]bool, int) bool
	loopOK = func(loop *WhileStmt, outer map[string]bool, depth int) bool {
		if depth > 2 || loop.Body == nil || len(loop.Body.Body) == 0 || len(loop.Body.Body) > 12 || !expr(loop.Cond, outer) {
			return false
		}
		scope := map[string]bool{}
		for name := range outer {
			scope[name] = true
		}
		pivot := -1
		for i, s := range loop.Body.Body {
			switch x := s.(type) {
			case *YieldStmt:
				// Inner yield stays first; no hidden prefix is moved across demand.
				if pivot >= 0 || i != 0 || x.Val == nil || x.Val.ExprTy() != g.ElemTy || !expr(x.Val, scope) {
					return false
				}
				pivot = i
			case *WhileStmt:
				if pivot >= 0 || !loopOK(x, scope, depth+1) {
					return false
				}
				pivot = i
			case *VarDeclStmt:
				if pivot >= 0 || !expr(x.Init, scope) {
					return false
				}
				scope[x.Name] = true
			default:
				if !assign(s, scope) {
					return false
				}
			}
		}
		return pivot >= 0 && len(loop.Body.Body)-pivot-1 <= 4
	}
	return valid && loopOK(root, available, 1)
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
	names := map[string]string{}
	var setup []Stmt
	for i, p := range g.Params {
		name := fresh(p.Name)
		names[p.Name] = name
		setup = append(setup, &VarDeclStmt{Name: name, Ty: p.Ty, Init: site.Args[i]})
	}
	producer := fusionClone(g.Body)
	fusionWalk(producer, func(s Stmt) {
		if v, ok := s.(*VarDeclStmt); ok {
			names[v.Name] = fresh(v.Name)
		}
	})
	rename := func(e Expr) Expr {
		if v, ok := e.(*VarRefExpr); ok {
			if name, found := names[v.Name]; found {
				v.Name = name
			}
		}
		return e
	}
	mapBlockExprs(producer, rename)
	fusionWalk(producer, func(s Stmt) {
		switch x := s.(type) {
		case *VarDeclStmt:
			x.Name = names[x.Name]
		case *YieldStmt:
			x.Val = mapExprChildren(x.Val, rename)
		}
	})
	root := producer.Body[0].(*WhileStmt)
	inner := root
	for _, s := range root.Body.Body {
		if loop, ok := s.(*WhileStmt); ok {
			inner = loop
		}
	}
	// A consumer break must leave every producer loop, before any suspended
	// suffix/outer condition is resumed. A private flag propagates the inner
	// exit; consumers with no such break need no flag.
	abort := ""
	if inner != root && fusionConsumerBreak(site.Body, 0) {
		abort = fresh("stop")
		setup = append(setup, &VarDeclStmt{Name: abort, Ty: mir2.TyBool, Init: &BoolLitExpr{Val: false}})
	}
	yield := inner.Body.Body[0].(*YieldStmt)
	suffix := &Block{Body: inner.Body.Body[1:]}
	consumer := fusionClone(site.Body)
	rewriteFusionContinue(consumer, suffix, 0)
	if abort != "" {
		rewriteFusionBreak(consumer, abort, 0)
	}
	body := []Stmt{&AssignStmt{Target: &VarRefExpr{Name: site.Var, Ty: site.ElemTy}, Val: yield.Val}}
	body = append(body, consumer.Body...)
	body = append(body, suffix.Body...)
	inner.Body = &Block{Body: body}
	if abort != "" {
		var outerBody []Stmt
		for _, s := range root.Body.Body {
			outerBody = append(outerBody, s)
			if s == inner {
				outerBody = append(outerBody, &IfStmt{Cond: &VarRefExpr{Name: abort, Ty: mir2.TyBool}, Then: &Block{Body: []Stmt{&BreakStmt{}}}})
			}
		}
		root.Body.Body = outerBody
	}
	setup = append(setup, &VarDeclStmt{Name: site.Var, Ty: site.ElemTy, Init: &IntLitExpr{Val: 0, Ty: site.ElemTy}}, root)
	return &Block{Body: setup}, root
}

func fusionConsumerBreak(b *Block, depth int) bool {
	found := false
	fusionExitBlocks(b, depth, func(b *Block, i, depth int) {
		if _, ok := b.Body[i].(*BreakStmt); ok && depth == 0 {
			found = true
		}
	})
	return found
}
func rewriteFusionBreak(b *Block, abort string, depth int) {
	fusionExitBlocks(b, depth, func(b *Block, i, depth int) {
		if _, ok := b.Body[i].(*BreakStmt); ok && depth == 0 {
			b.Body[i] = &Block{Body: []Stmt{&AssignStmt{Target: &VarRefExpr{Name: abort, Ty: mir2.TyBool}, Val: &BoolLitExpr{Val: true}}, &BreakStmt{}}}
		}
	})
}

// Visit only structured consumer exits, keeping nested loop exits local.
func fusionExitBlocks(b *Block, depth int, visit func(*Block, int, int)) {
	if b == nil {
		return
	}
	for i, s := range b.Body {
		visit(b, i, depth)
		switch x := s.(type) {
		case *Block:
			fusionExitBlocks(x, depth, visit)
		case *IfStmt:
			fusionExitBlocks(x.Then, depth, visit)
			fusionExitBlocks(x.Else, depth, visit)
		case *WhileStmt:
			fusionExitBlocks(x.Body, depth+1, visit)
		case *ForRangeStmt:
			fusionExitBlocks(x.Body, depth+1, visit)
		case *ForEachStmt:
			fusionExitBlocks(x.Body, depth+1, visit)
		}
	}
}
func rewriteFusionContinue(b, suffix *Block, depth int) {
	fusionExitBlocks(b, depth, func(b *Block, i, depth int) {
		if _, ok := b.Body[i].(*ContinueStmt); ok && depth == 0 {
			copy := fusionClone(suffix)
			copy.Body = append(copy.Body, &ContinueStmt{})
			b.Body[i] = copy
		}
	})
}

// HIR names do not yet carry resolved binding identities. A free producer
// global/function name must not become a caller local after splicing. Refuse
// whole-function binding collisions, even if some are out of lexical scope.
func fusionFreeBindingsSafe(g *Func, callerBindings map[string]bool) bool {
	owned := map[string]bool{}
	for _, p := range g.Params {
		owned[p.Name] = true
	}
	fusionWalk(g.Body, func(s Stmt) {
		if v, ok := s.(*VarDeclStmt); ok {
			owned[v.Name] = true
		}
	})
	safe := true
	check := func(e Expr) {
		switch x := e.(type) {
		case *VarRefExpr:
			if !owned[x.Name] && callerBindings[x.Name] {
				safe = false
			}
		case *CallExpr:
			if callerBindings[x.Fn] || owned[x.Fn] {
				safe = false
			}
		case *AddrOfExpr:
			if !owned[x.Sym] && callerBindings[x.Sym] {
				safe = false
			}
		}
	}
	fusionWalk(g.Body, func(s Stmt) {
		visitStmtExprs(s, check)
		if y, ok := s.(*YieldStmt); ok {
			walkExpr(y.Val, check)
		}
	})
	return safe
}
