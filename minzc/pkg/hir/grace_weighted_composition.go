package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// WeightedCompositionRecord records independently checked structure. Attribution
// fields are additionally required, but never establish this source proof.
type WeightedCompositionRecord struct {
	SiteID        string `json:"site_id"`
	Function      string `json:"function"`
	Scalar        bool   `json:"scalar"`
	Pointer       bool   `json:"pointer"`
	WeightBinding string `json:"weight_binding,omitempty"`
}

// EligibleWeightedCompositionSites accepts only the original exact weighted
// source, never a supplied hoisted prefix or previously transformed function.
func EligibleWeightedCompositionSites(m *Module) ([]string, error) {
	if err := Verify(m); err != nil {
		return nil, err
	}
	sites, err := BuildLoopSiteMap(m)
	if err != nil {
		return nil, err
	}
	reserved := map[string]bool{}
	for _, g := range m.Globals {
		reserved[g.Name] = true
	}
	for _, f := range m.Funcs {
		reserved[f.Name] = true
	}
	out := []string{}
	for _, site := range sites.Sites {
		f := m.FuncByName(site.Function)
		p := findGracePointerWalk(f, f.Body, reserved)
		if p != nil && p.weight != nil && p.loopIndex == 2 && len(f.Body.Body) == 4 && site.HIRPath == "body/s2" {
			out = append(out, site.ID)
		}
	}
	return out, nil
}

// ValidateWeightedComposition independently checks the original proof and the
// requested actual shape. It performs no rewriting, compilation, or execution.
func ValidateWeightedComposition(source, actual *Module, siteID string, scalar, pointer bool) (WeightedCompositionRecord, error) {
	fail := func(why string) (WeightedCompositionRecord, error) {
		return WeightedCompositionRecord{}, fmt.Errorf("weighted composition %s: %s", siteID, why)
	}
	if source == nil || actual == nil {
		return fail("nil module")
	}
	if err := Verify(actual); err != nil {
		return fail("invalid actual HIR: " + err.Error())
	}
	if !reflect.DeepEqual(source.Globals, actual.Globals) || source.Target != actual.Target {
		return fail("globals or target changed")
	}
	eligible, err := EligibleWeightedCompositionSites(source)
	if err != nil {
		return fail(err.Error())
	}
	found := false
	for _, id := range eligible {
		if id == siteID {
			found = true
		}
	}
	if !found {
		return fail("not an original exact weighted source site")
	}
	sites, err := BuildLoopSiteMap(source)
	if err != nil {
		return fail(err.Error())
	}
	function := ""
	for _, site := range sites.Sites {
		if site.ID == siteID {
			function = site.Function
		}
	}
	original := source.FuncByName(function)
	f := actual.FuncByName(function)
	if f == nil {
		return fail("missing function")
	}
	originalMeta, actualMeta := *original, *f
	originalMeta.Body = nil
	actualMeta.Body = nil
	if !reflect.DeepEqual(originalMeta, actualMeta) {
		return fail("function metadata changed")
	}
	root := f.Body
	if root == nil {
		return fail("missing root")
	}
	expectedLen := 4
	if scalar {
		expectedLen = 5
	}
	if len(root.Body) != expectedLen {
		return fail("unexpected root prefix")
	}
	eq := func(a, b any) bool { return reflect.DeepEqual(a, b) }
	if !eq(root.Body[0], original.Body.Body[0]) || !eq(root.Body[1], original.Body.Body[1]) || !eq(root.Body[len(root.Body)-1], original.Body.Body[3]) {
		return fail("seed, zero-index, or return changed")
	}
	sum := original.Body.Body[0].(*VarDeclStmt)
	index := original.Body.Body[1].(*VarDeclStmt)
	srcLoop := original.Body.Body[2].(*WhileStmt)
	srcWeight := srcLoop.Body.Body[0].(*AssignStmt).Val.(*BinExpr).R
	weight := srcWeight
	loopIndex := 2
	binding := ""
	if scalar {
		loopIndex = 3
		decl, ok := root.Body[2].(*VarDeclStmt)
		if !ok || decl.Ty != mir2.TyU16 || decl.ArrayLen != 0 || decl.At != nil || len(decl.Initial) != 0 || !eq(decl.Init, srcWeight) {
			return fail("invalid hoisted weight initializer")
		}
		binding = decl.Name
		// Freshness must cover original parameters, bindings, globals, and reads.
		collision := binding == ""
		for _, p := range original.Params {
			if p.Name == binding {
				collision = true
			}
		}
		for _, g := range source.Globals {
			if g.Name == binding {
				collision = true
			}
		}
		for _, fn := range source.Funcs {
			if fn.Name == binding {
				collision = true
			}
		}
		walkGraceBlocks(original.Body, "", func(_ string, b *Block) {
			for _, stmt := range b.Body {
				if d, ok := stmt.(*VarDeclStmt); ok && d.Name == binding {
					collision = true
				}
				visitStmtExprs(stmt, func(e Expr) {
					if v, ok := e.(*VarRefExpr); ok && v.Name == binding {
						collision = true
					}
				})
			}
		})
		if collision {
			return fail("hoisted binding is not fresh")
		}
		weight = Var(binding, mir2.TyU16)
	}
	// Exact updates retain wrapping u16 arithmetic and the original read order.
	checkWeight := func(stmt Stmt) bool {
		a, ok := stmt.(*AssignStmt)
		if !ok || !graceRef(a.Target, sum.Name, mir2.TyU16) {
			return false
		}
		b, ok := a.Val.(*BinExpr)
		return ok && b.Op == "+" && b.Ty == mir2.TyU16 && graceRef(b.L, sum.Name, mir2.TyU16) && eq(b.R, weight)
	}
	checkIndexed := func(loop *WhileStmt) bool {
		return loop != nil && eq(loop.Cond, srcLoop.Cond) && loop.Body != nil && len(loop.Body.Body) == 3 && checkWeight(loop.Body.Body[0]) && eq(loop.Body.Body[1], srcLoop.Body.Body[1]) && eq(loop.Body.Body[2], srcLoop.Body.Body[2])
	}
	expectedMarker := ""
	if scalar {
		expectedMarker = "hir-invariant-base"
	}
	checkMarker := func(stmt Stmt, role, rule, previous string) bool {
		l, ok := actual.LoopLineage[stmt]
		return ok && l.SiteID == siteID && l.Role == role && l.Rule == rule && l.PreviousRule == previous
	}
	if !pointer {
		loop, ok := root.Body[loopIndex].(*WhileStmt)
		if !ok || !checkIndexed(loop) {
			return fail("indexed loop differs from source proof")
		}
		role, rule := "original", ""
		if scalar {
			role, rule = "invariant-base", "hir-invariant-base"
		}
		if !checkMarker(loop, role, rule, "") {
			return fail("indexed attribution mismatch")
		}
	} else {
		guard, ok := root.Body[loopIndex].(*IfStmt)
		if !ok || guard.Then == nil || guard.Else == nil || len(guard.Then.Body) != 2 || len(guard.Else.Body) != 1 {
			return fail("invalid versioned branches")
		}
		// Guard must compare unsigned ptr against 65535-u16(n), not a wrapping ptr+n.
		cond, ok := guard.Cond.(*BinExpr)
		if !ok || cond.Op != "<=" || cond.Ty != mir2.TyBool {
			return fail("invalid guard comparison")
		}
		load := srcLoop.Body.Body[1].(*AssignStmt).Val.(*BinExpr).R.(*CastExpr).X.(*IndexExpr)
		bound := srcLoop.Cond.(*BinExpr).R
		ptrCast, ok := cond.L.(*CastExpr)
		if !ok || ptrCast.Ty != mir2.TyU16 || !eq(ptrCast.X, load.Base) {
			return fail("guard base changed")
		}
		limit, ok := cond.R.(*BinExpr)
		if !ok || limit.Op != "-" || limit.Ty != mir2.TyU16 {
			return fail("invalid guard limit")
		}
		max, ok := limit.L.(*IntLitExpr)
		if !ok || max.Val != 65535 || max.Ty != mir2.TyU16 {
			return fail("invalid address maximum")
		}
		boundCast, ok := limit.R.(*CastExpr)
		if !ok || boundCast.Ty != mir2.TyU16 || !eq(boundCast.X, bound) {
			return fail("guard length changed")
		}
		each, ok := guard.Then.Body[0].(*ForEachStmt)
		if !ok || !each.PtrIter || each.MutateInPlace || each.ElemTy != mir2.TyU8 || each.ElemStride != 1 || !eq(each.Ptr, load.Base) || !eq(each.Start, U8(0)) || !eq(each.Len, bound) || each.Body == nil || len(each.Body.Body) != 2 || !checkWeight(each.Body.Body[0]) {
			return fail("fast pointer loop shape changed")
		}
		for _, p := range f.Params {
			if each.Var == p.Name {
				return fail("element shadows parameter")
			}
		}
		for _, fn := range actual.Funcs {
			if fn.Name == each.Var {
				return fail("element shadows function")
			}
		}
		for _, g := range actual.Globals {
			if each.Var == g.Name {
				return fail("element shadows global")
			}
		}
		if each.Var == sum.Name || each.Var == index.Name || each.Var == binding || each.Var == "" {
			return fail("element binding collision")
		}
		a, ok := each.Body.Body[1].(*AssignStmt)
		if !ok || !graceRef(a.Target, sum.Name, mir2.TyU16) {
			return fail("fast sum target changed")
		}
		add, ok := a.Val.(*BinExpr)
		if !ok || add.Op != "+" || add.Ty != mir2.TyU16 || !graceRef(add.L, sum.Name, mir2.TyU16) {
			return fail("fast accumulator changed")
		}
		ext, ok := add.R.(*CastExpr)
		if !ok || ext.Ty != mir2.TyU16 {
			return fail("missing byte zero-extension")
		}
		read, ok := ext.X.(*LoadExpr)
		if !ok || read.Ty != mir2.TyU8 || !graceRef(read.Ptr, each.Var, mir2.TyPtr) {
			return fail("deferred byte load changed")
		}
		exit, ok := guard.Then.Body[1].(*AssignStmt)
		if !ok || !graceRef(exit.Target, index.Name, mir2.TyU8) || !eq(exit.Val, bound) {
			return fail("final index changed")
		}
		fallback, ok := guard.Else.Body[0].(*WhileStmt)
		if !ok || !checkIndexed(fallback) {
			return fail("fallback differs from indexed source")
		}
		if !checkMarker(each, "fast", "hir-guarded-pointer-walk", expectedMarker) || !checkMarker(fallback, "fallback", "hir-guarded-pointer-walk", expectedMarker) {
			return fail("compound attribution mismatch")
		}
	}
	return WeightedCompositionRecord{SiteID: siteID, Function: function, Scalar: scalar, Pointer: pointer, WeightBinding: binding}, nil
}
