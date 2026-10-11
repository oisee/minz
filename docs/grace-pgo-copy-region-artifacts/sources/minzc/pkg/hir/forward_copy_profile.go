package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

type ForwardCopyProof struct {
	SiteID   string `json:"site_id"`
	Function string `json:"function"`
	Enabled  bool   `json:"enabled"`
}

func EligibleForwardCopySites(m *Module) ([]string, error) {
	if m == nil {
		return nil, fmt.Errorf("forward copy: nil module")
	}
	if err := Verify(m); err != nil {
		return nil, err
	}
	sites, err := BuildLoopSiteMap(m)
	if err != nil {
		return nil, err
	}
	reserved := forwardCopyReserved(m)
	out := []string{}
	for _, site := range sites.Sites {
		f := m.FuncByName(site.Function)
		if site.HIRPath == "body/s1" && findGraceForwardCopy(f, f.Body, reserved) != nil {
			out = append(out, site.ID)
		}
	}
	return out, nil
}
func forwardCopyReserved(m *Module) map[string]bool {
	reserved := map[string]bool{}
	for _, g := range m.Globals {
		reserved[g.Name] = true
	}
	for _, f := range m.Funcs {
		reserved[f.Name] = true
	}
	return reserved
}

// ValidateForwardCopyProof checks source legality and actual independent shape,
// including both guards, sequential read/store, pointer steps and exact fallback.
func ValidateForwardCopyProof(source, actual *Module, siteID string, enabled bool) (ForwardCopyProof, error) {
	return validateForwardCopyShape(source, actual, siteID, enabled)
}

func validateForwardCopyShape(source, actual *Module, siteID string, enabled bool) (ForwardCopyProof, error) {
	fail := func(why string) (ForwardCopyProof, error) {
		return ForwardCopyProof{}, fmt.Errorf("forward copy %s: %s", siteID, why)
	}
	if source == nil || actual == nil {
		return fail("nil module")
	}
	if err := Verify(actual); err != nil {
		return fail(err.Error())
	}
	ids, err := EligibleForwardCopySites(source)
	if err != nil {
		return fail(err.Error())
	}
	found := false
	for _, id := range ids {
		if id == siteID {
			found = true
		}
	}
	if !found {
		return fail("not an original exact copy site")
	}
	sites, _ := BuildLoopSiteMap(source)
	name := ""
	for _, site := range sites.Sites {
		if site.ID == siteID {
			name = site.Function
		}
	}
	original, f := source.FuncByName(name), actual.FuncByName(name)
	if f == nil {
		return fail("missing function")
	}
	a, b := *original, *f
	a.Body = nil
	b.Body = nil
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(source.Globals, actual.Globals) || source.Target != actual.Target {
		return fail("function/global metadata changed")
	}
	eq := reflect.DeepEqual
	root := f.Body
	if root == nil || len(root.Body) != 3 || !eq(root.Body[0], original.Body.Body[0]) || !eq(root.Body[2], original.Body.Body[2]) {
		return fail("root, zero index, or return changed")
	}
	plan := findGraceForwardCopy(original, original.Body, forwardCopyReserved(source))
	marker := func(stmt Stmt, role, rule string) bool {
		l, ok := actual.LoopLineage[stmt]
		return ok && l.SiteID == siteID && l.Role == role && l.Rule == rule && l.PreviousRule == ""
	}
	if !enabled {
		if !eq(root.Body[1], original.Body.Body[1]) || !marker(root.Body[1], "original", "") {
			return fail("original choice rewritten or unattributed")
		}
		return ForwardCopyProof{siteID, name, false}, nil
	}
	version, ok := root.Body[1].(*IfStmt)
	if !ok || version.Then == nil || version.Else == nil || len(version.Then.Body) != 3 || len(version.Else.Body) != 1 || !eq(version.Else.Body[0], original.Body.Body[1]) {
		return fail("invalid guarded branches or changed fallback")
	}
	cond, ok := version.Cond.(*BinExpr)
	if !ok || cond.Op != "&&" || cond.Ty != mir2.TyBool {
		return fail("both address guards required")
	}
	checkGuard := func(expr Expr, base string) bool {
		cmp, ok := expr.(*BinExpr)
		if !ok || cmp.Op != "<=" || cmp.Ty != mir2.TyBool {
			return false
		}
		cast, ok := cmp.L.(*CastExpr)
		if !ok || cast.Ty != mir2.TyU16 || !graceRef(cast.X, base, mir2.TyPtr) {
			return false
		}
		sub, ok := cmp.R.(*BinExpr)
		if !ok || sub.Op != "-" || sub.Ty != mir2.TyU16 {
			return false
		}
		max, ok := sub.L.(*IntLitExpr)
		if !ok || max.Val != 65535 || max.Ty != mir2.TyU16 {
			return false
		}
		bound, ok := sub.R.(*CastExpr)
		return ok && bound.Ty == mir2.TyU16 && graceRef(bound.X, plan.bound, mir2.TyU8)
	}
	if !checkGuard(cond.L, plan.src) || !checkGuard(cond.R, plan.dst) {
		return fail("guard base/range changed")
	}
	dst, ok := version.Then.Body[0].(*VarDeclStmt)
	if !ok || dst.Ty != mir2.TyPtr || dst.ArrayLen != 0 || dst.At != nil || len(dst.Initial) != 0 || !graceRef(dst.Init, plan.dst, mir2.TyPtr) {
		return fail("invalid destination pointer binding")
	}
	each, ok := version.Then.Body[1].(*ForEachStmt)
	if !ok || !each.PtrIter || each.MutateInPlace || each.ElemTy != mir2.TyU8 || each.ElemStride != 1 || !graceRef(each.Ptr, plan.src, mir2.TyPtr) || !eq(each.Start, U8(0)) || !graceRef(each.Len, plan.bound, mir2.TyU8) || each.Body == nil || len(each.Body.Body) != 2 {
		return fail("invalid source pointer loop")
	}
	fresh := func(name string) bool {
		if name == "" || forwardCopyReserved(source)[name] {
			return false
		}
		for _, p := range original.Params {
			if p.Name == name {
				return false
			}
		}
		collision := false
		walkGraceBlocks(original.Body, "", func(_ string, b *Block) {
			for _, stmt := range b.Body {
				if d, ok := stmt.(*VarDeclStmt); ok && d.Name == name {
					collision = true
				}
				visitStmtExprs(stmt, func(e Expr) {
					if v, ok := e.(*VarRefExpr); ok && v.Name == name {
						collision = true
					}
				})
			}
		})
		return !collision
	}
	if dst.Name == each.Var || !fresh(dst.Name) || !fresh(each.Var) {
		return fail("pointer binding collision")
	}
	store, ok := each.Body.Body[0].(*StoreStmt)
	if !ok || !graceRef(store.Ptr, dst.Name, mir2.TyPtr) {
		return fail("destination store/order changed")
	}
	load, ok := store.Val.(*LoadExpr)
	if !ok || load.Ty != mir2.TyU8 || !graceRef(load.Ptr, each.Var, mir2.TyPtr) {
		return fail("sequential byte read changed")
	}
	step, ok := each.Body.Body[1].(*AssignStmt)
	if !ok || !graceRef(step.Target, dst.Name, mir2.TyPtr) {
		return fail("destination pointer step changed")
	}
	add, ok := step.Val.(*BinExpr)
	if !ok || add.Op != "+" || add.Ty != mir2.TyPtr || !graceRef(add.L, dst.Name, mir2.TyPtr) || !eq(add.R, U16(1)) {
		return fail("nonunit destination step")
	}
	exit, ok := version.Then.Body[2].(*AssignStmt)
	if !ok || !graceRef(exit.Target, plan.index, mir2.TyU8) || !graceRef(exit.Val, plan.bound, mir2.TyU8) {
		return fail("final index changed")
	}
	if !marker(each, "fast", ForwardCopyRule) || !marker(version.Else.Body[0], "fallback", ForwardCopyRule) {
		return fail("copy lineage mismatch")
	}
	return ForwardCopyProof{siteID, name, true}, nil
}

func ApplyForwardCopySiteChoices(m *Module, compatibility *ProfileCompatibility, choices map[string]bool) error {
	if m == nil || compatibility == nil || m.RegionComparePolicy != nil || m.LoopLineage != nil || m.ForwardCopyPolicy != nil || m.PointerWalkPolicy != nil || m.GraceRulePolicy != nil || m.InvariantBasePolicy != nil {
		return fmt.Errorf("forward copy: requires fresh original HIR and compatibility")
	}
	current, err := BuildProfileCompatibility(m, compatibility.Context)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, compatibility) {
		return fmt.Errorf("forward copy: stale source/HIR compatibility")
	}
	ids, err := EligibleForwardCopySites(m)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	for id := range choices {
		if !known[id] {
			return fmt.Errorf("forward copy: unknown requested site")
		}
	}
	if err := SeedLoopLineage(m); err != nil {
		return err
	}
	m.ForwardCopyPolicy = map[string]bool{}
	for id, want := range choices {
		m.ForwardCopyPolicy[id] = want
	}
	m.PointerWalkPolicy = map[string]bool{}
	m.InvariantBasePolicy = map[string]bool{}
	m.GraceRulePolicy = map[string]bool{ForwardCopyRule: true}
	_, err = RunGrace(m, 0)
	return err
}
