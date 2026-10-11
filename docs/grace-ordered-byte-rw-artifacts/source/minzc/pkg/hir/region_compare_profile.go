package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

type RegionCompareProof struct {
	SiteID       string `json:"site_id"`
	Function     string `json:"function"`
	SourceSHA256 string `json:"source_sha256"`
	Enabled      bool   `json:"enabled"`
}

func EligibleRegionCompareSites(m *Module) ([]string, error) {
	if m != nil && (m.LoopLineage != nil || m.RegionComparePolicy != nil || m.OrderedBytePolicy != nil || m.OrderedByteRWPolicy != nil || m.PointerWalkPolicy != nil || m.InvariantBasePolicy != nil || m.ForwardCopyPolicy != nil || m.GraceRulePolicy != nil) {
		return nil, fmt.Errorf("region compare: original source already has policy/lineage")
	}
	if m == nil || !m.SourceUnitsComplete {
		return nil, fmt.Errorf("region compare: complete original source required")
	}
	sites, e := BuildLoopSiteMap(m)
	if e != nil {
		return nil, e
	}
	if sites.UnmappedLoops != 0 {
		return nil, fmt.Errorf("region compare: unmapped original loops")
	}
	out := []string{}
	for _, s := range sites.Sites {
		if m.SourceUnits[s.Origin.Unit] != s.Origin.SourceSHA256 {
			return nil, fmt.Errorf("region compare: source SHA unbound")
		}
		if s.HIRPath == "body/s1" && findRegionCompare(m.FuncByName(s.Function), forwardCopyReserved(m)) != nil {
			out = append(out, s.ID)
		}
	}
	return out, nil
}

// ApplyRegionCompareSiteChoices runs the explicit source-site Grace rule.
// The rule requires both its named opt-in and a seeded selected-site policy.
// Missing/false choices keep source loops; all other HIR rules are disabled.
func ApplyRegionCompareSiteChoices(m *Module, c *ProfileCompatibility, choices map[string]bool) error {
	if m == nil || c == nil || m.LoopLineage != nil || m.GraceRulePolicy != nil || m.PointerWalkPolicy != nil || m.InvariantBasePolicy != nil || m.ForwardCopyPolicy != nil || m.RegionComparePolicy != nil || m.OrderedBytePolicy != nil || m.OrderedByteRWPolicy != nil {
		return fmt.Errorf("region compare: fresh original required")
	}
	current, e := BuildProfileCompatibility(m, c.Context)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(current, c) {
		return fmt.Errorf("region compare: stale compatibility")
	}
	ids, e := EligibleRegionCompareSites(m)
	if e != nil {
		return e
	}
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	for id := range choices {
		if !known[id] {
			return fmt.Errorf("region compare: unknown site")
		}
	}
	saved := *m
	oldBodies := map[*Func]*Block{}
	for _, f := range m.Funcs {
		if f.Body != nil {
			oldBodies[f] = &Block{Body: append([]Stmt(nil), f.Body.Body...)}
		} else {
			oldBodies[f] = nil
		}
	}
	if e = SeedLoopLineage(m); e != nil {
		return e
	}
	m.RegionComparePolicy = map[string]bool{}
	for id, want := range choices {
		m.RegionComparePolicy[id] = want
	}

	m.GraceRulePolicy = map[string]bool{RegionCompareRule: true}
	m.PointerWalkPolicy = map[string]bool{}
	m.InvariantBasePolicy = map[string]bool{}
	m.ForwardCopyPolicy = map[string]bool{}
	_, runError := RunGrace(m, 0)
	if e := runError; e != nil {
		*m = saved
		for f, b := range oldBodies {
			f.Body = b
		}
		return e
	}
	return nil
}

// ValidateRegionCompareProof independently checks actual ordered reads, range
// arithmetic, early exits, resumed fallback, metadata and source attribution.
// It never constructs an expected transformed body by invoking the rewriter.
func ValidateRegionCompareProof(source, actual *Module, id string, enabled bool) (RegionCompareProof, error) {
	return validateRegionCompareShape(source, actual, id, enabled, true)
}

func validateRegionCompareShape(source, actual *Module, id string, enabled, standalone bool) (RegionCompareProof, error) {
	fail := func(why string) (RegionCompareProof, error) {
		return RegionCompareProof{}, fmt.Errorf("region compare %s: %s", id, why)
	}
	if source == nil || actual == nil {
		return fail("nil module")
	}
	if e := Verify(source); e != nil {
		return fail(e.Error())
	}
	if e := Verify(actual); e != nil {
		return fail(e.Error())
	}
	if _, e := EligibleRegionCompareSites(source); e != nil {
		return fail(e.Error())
	}
	sites, e := BuildLoopSiteMap(source)
	if e != nil {
		return fail(e.Error())
	}
	var site *LoopSite
	for j := range sites.Sites {
		if sites.Sites[j].ID == id {
			site = &sites.Sites[j]
		}
	}
	if site == nil || site.HIRPath != "body/s1" {
		return fail("unknown original site")
	}
	original, f := source.FuncByName(site.Function), actual.FuncByName(site.Function)
	p := findRegionCompare(original, forwardCopyReserved(source))
	if p == nil || f == nil {
		return fail("not exact original source")
	}
	if source.SourceUnits[site.Origin.Unit] != site.Origin.SourceSHA256 {
		return fail("unbound source SHA")
	}
	a, b := *original, *f
	a.Body = nil
	b.Body = nil
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(source.Globals, actual.Globals) || source.Target != actual.Target {
		return fail("metadata changed")
	}
	if standalone {
		// Compare all unchanged module content; the chosen body and intentional
		// policy/lineage bookkeeping are checked separately below.
		x, y := *source, *actual
		x.RegionComparePolicy = nil
		y.RegionComparePolicy = nil
		x.SourceLocations = nil
		y.SourceLocations = nil
		x.LoopLineage = nil
		y.LoopLineage = nil
		x.GraceRulePolicy = nil
		y.GraceRulePolicy = nil
		x.PointerWalkPolicy = nil
		y.PointerWalkPolicy = nil
		x.InvariantBasePolicy = nil
		y.InvariantBasePolicy = nil
		x.ForwardCopyPolicy = nil
		y.ForwardCopyPolicy = nil
		x.Funcs = make([]*Func, len(source.Funcs))
		y.Funcs = make([]*Func, len(actual.Funcs))
		for j, fn := range source.Funcs {
			copy := *fn
			if findRegionCompare(source.FuncByName(fn.Name), forwardCopyReserved(source)) != nil {
				copy.Body = nil
			}
			x.Funcs[j] = &copy
		}
		for j, fn := range actual.Funcs {
			copy := *fn
			if findRegionCompare(source.FuncByName(fn.Name), forwardCopyReserved(source)) != nil {
				copy.Body = nil
			}
			y.Funcs[j] = &copy
		}
		if !reflect.DeepEqual(x, y) {
			return fail("unrelated module content changed")
		}
	}
	root := f.Body
	eq := reflect.DeepEqual
	if root == nil || len(root.Body) != 3 || !eq(root.Body[0], original.Body.Body[0]) || !eq(root.Body[2], original.Body.Body[2]) {
		return fail("root/index/return changed")
	}
	marker := func(s Stmt, role, rule string) bool {
		l, ok := actual.LoopLineage[s]
		return ok && l.SiteID == id && l.Role == role && l.Rule == rule && l.PreviousRule == ""
	}
	if standalone {
		if e := regionCompareLineage(source, actual); e != nil {
			return fail(e.Error())
		}
	}
	proof := RegionCompareProof{id, site.Function, site.Origin.SourceSHA256, enabled}
	if !enabled {
		if !eq(root.Body[1], p.loop) || !marker(root.Body[1], "original", "") {
			return fail("disabled choice changed")
		}
		return proof, nil
	}
	block, ok := root.Body[1].(*Block)
	if !ok || len(block.Body) != 7 {
		return fail("peeled shape")
	}
	i, n := Var(p.index, mir2.TyU8), Var(p.count, mir2.TyU8)
	off, start := Var(p.offset, mir2.TyU16), Var(p.start, mir2.TyU16)
	returnIf := func(s Stmt, cond Expr, value uint8) bool {
		x, ok := s.(*IfStmt)
		return ok && eq(x.Cond, cond) && x.Else == nil && x.Then != nil && len(x.Then.Body) == 1 && eq(x.Then.Body[0], &ReturnStmt{Val: U8(int64(value))})
	}
	if !returnIf(block.Body[0], Eq(n, U8(0)), 1) || !returnIf(block.Body[3], Eq(n, U8(1)), 1) || !eq(block.Body[2], Assign(i, U8(1))) {
		return fail("zero/one exit or fallback index")
	}
	left := &IndexExpr{Base: Var(p.left, mir2.TyPtr), Idx: start, ElemTy: mir2.TyU8}
	right := &IndexExpr{Base: Var(p.right, mir2.TyPtr), Idx: Add(off, U8(1), mir2.TyU16), ElemTy: mir2.TyU8}
	if !returnIf(block.Body[1], Ne(left, right), 0) {
		return fail("first ordered pair")
	}
	decl := func(s Stmt, ty mir2.Ty, init Expr) (string, bool) {
		x, ok := s.(*VarDeclStmt)
		returnName := ""
		if ok {
			returnName = x.Name
		}
		return returnName, ok && x.Name != "" && x.Ty == ty && x.ArrayLen == 0 && x.At == nil && len(x.Initial) == 0 && eq(x.Init, init)
	}
	ln, ok := decl(block.Body[4], mir2.TyU16, Sub(U16(65535), Cast(n, mir2.TyU16), mir2.TyU16))
	if !ok {
		return fail("remaining limit")
	}
	rn, ok := decl(block.Body[5], mir2.TyU16, Sub(Var(ln, mir2.TyU16), U16(1), mir2.TyU16))
	if !ok {
		return fail("right limit")
	}
	branch, ok := block.Body[6].(*IfStmt)
	if !ok || branch.Then == nil || branch.Else == nil || len(branch.Then.Body) != 3 || len(branch.Else.Body) != 1 || !eq(branch.Else.Body[0], p.loop) {
		return fail("fallback or fast branches")
	}
	// Four independently checked bounds establish logical and physical no-wrap.
	and := func(e Expr) (Expr, Expr, bool) {
		x, ok := e.(*BinExpr)
		if !ok || x.Op != "&&" || x.Ty != mir2.TyBool {
			return nil, nil, false
		}
		return x.L, x.R, true
	}
	g1, rest, ok := and(branch.Cond)
	if !ok {
		return fail("four guards")
	}
	g2, rest, ok := and(rest)
	if !ok {
		return fail("four guards")
	}
	g3, g4, ok := and(rest)
	if !ok {
		return fail("four guards")
	}
	lim, rlim := Var(ln, mir2.TyU16), Var(rn, mir2.TyU16)
	if !eq(g1, Le(start, lim)) || !eq(g2, Le(Cast(Var(p.left, mir2.TyPtr), mir2.TyU16), Sub(lim, start, mir2.TyU16))) || !eq(g3, Le(off, rlim)) || !eq(g4, Le(Cast(Var(p.right, mir2.TyPtr), mir2.TyU16), Sub(rlim, off, mir2.TyU16))) {
		return fail("logical/physical guard")
	}
	lp, ok := decl(branch.Then.Body[0], mir2.TyPtr, Add(Var(p.left, mir2.TyPtr), Add(start, U16(1), mir2.TyU16), mir2.TyPtr))
	if !ok {
		return fail("left tail pointer")
	}
	rp, ok := decl(branch.Then.Body[1], mir2.TyPtr, Add(Var(p.right, mir2.TyPtr), Add(off, U16(2), mir2.TyU16), mir2.TyPtr))
	if !ok {
		return fail("right tail pointer")
	}
	tail, ok := branch.Then.Body[2].(*ForRangeStmt)
	if !ok || tail.Body == nil || len(tail.Body.Body) != 3 || !eq(tail.Start, U8(1)) || !eq(tail.End, n) {
		return fail("tail count")
	}
	if !returnIf(tail.Body.Body[0], Ne(&LoadExpr{Ptr: Var(lp, mir2.TyPtr), Ty: mir2.TyU8}, &LoadExpr{Ptr: Var(rp, mir2.TyPtr), Ty: mir2.TyU8}), 0) || !eq(tail.Body.Body[1], Assign(Var(lp, mir2.TyPtr), Add(Var(lp, mir2.TyPtr), U16(1), mir2.TyPtr))) || !eq(tail.Body.Body[2], Assign(Var(rp, mir2.TyPtr), Add(Var(rp, mir2.TyPtr), U16(1), mir2.TyPtr))) {
		return fail("tail ordered pair/steps")
	}
	used := forwardCopyReserved(source)
	for _, param := range original.Params {
		used[param.Name] = true
	}
	used[p.index] = true
	for _, name := range []string{ln, rn, lp, rp, tail.Var} {
		if name == "" || used[name] {
			return fail("generated binding collision")
		}
		used[name] = true
	}
	if !marker(tail, "fast", RegionCompareRule) || !marker(branch.Else.Body[0], "fallback", RegionCompareRule) {
		return fail("lineage")
	}
	count := 0
	bad := false
	walkGraceBlocks(root, "", func(_ string, b *Block) {
		for _, s := range b.Body {
			if l, ok := actual.LoopLineage[s]; ok && l.SiteID == id {
				count++
				if s != tail && s != branch.Else.Body[0] {
					bad = true
				}
			}
		}
	})
	if bad || count != 2 {
		return fail("extra/misplaced lineage")
	}
	return proof, nil
}

func regionCompareLineage(source, actual *Module) error {
	sites, e := BuildLoopSiteMap(source)
	if e != nil {
		return e
	}
	known := map[string]LoopSite{}
	for _, s := range sites.Sites {
		known[s.ID] = s
	}
	counts := map[string]int{}
	reachable := map[Stmt]bool{}
	var bad error
	for _, f := range actual.Funcs {
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for j, s := range b.Body {
				reachable[s] = true
				l, marked := actual.LoopLineage[s]
				loop := false
				switch s.(type) {
				case *WhileStmt, *ForRangeStmt, *ForEachStmt:
					loop = true
				}
				if marked != loop {
					bad = fmt.Errorf("lineage must annotate only every actual loop")
					continue
				}
				if !loop {
					continue
				}
				original, ok := known[l.SiteID]
				if !ok || original.Function != f.Name || l.PreviousRule != "" {
					bad = fmt.Errorf("unknown/cross-function lineage")
					continue
				}
				counts[l.SiteID]++
				if l.Role == "original" || l.Role == "fallback" {
					if origin, ok := actual.SourceLocations[s]; !ok || origin != original.Origin {
						bad = fmt.Errorf("loop source origin/path mismatch")
					}
				} else if _, ok := actual.SourceLocations[s]; ok {
					bad = fmt.Errorf("generated loop invented origin")
				}

				if !reflect.DeepEqual(source.FuncByName(original.Function).Body, actual.FuncByName(original.Function).Body) && findRegionCompare(source.FuncByName(original.Function), forwardCopyReserved(source)) != nil {
					if l.Rule != RegionCompareRule || (l.Role != "fast" && l.Role != "fallback") {
						bad = fmt.Errorf("compound lineage role")
					}
				} else if l.Rule != "" || l.Role != "original" || original.HIRPath != fmt.Sprintf("%s/s%d", path, j) {
					bad = fmt.Errorf("original lineage/path changed")
				}
			}
		})
	}
	for stmt := range actual.LoopLineage {
		if !reachable[stmt] {
			return fmt.Errorf("unreachable lineage entry")
		}
	}
	for id := range known {
		want := 1
		if !reflect.DeepEqual(source.FuncByName(known[id].Function).Body, actual.FuncByName(known[id].Function).Body) && findRegionCompare(source.FuncByName(known[id].Function), forwardCopyReserved(source)) != nil {
			want = 2
		}
		if counts[id] != want {
			return fmt.Errorf("lineage multiplicity")
		}
	}
	return bad
}

// ValidateRegionCompareChoices is the complete module contract. Per-site shape
// validation must be composed through this API when more than one site exists.
func ValidateRegionCompareChoices(source, actual *Module, choices map[string]bool) ([]RegionCompareProof, error) {
	ids, e := EligibleRegionCompareSites(source)
	if e != nil {
		return nil, e
	}
	if actual == nil {
		return nil, fmt.Errorf("region compare: nil actual")
	}
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	for id := range choices {
		if !known[id] {
			return nil, fmt.Errorf("region compare: unknown requested choice")
		}
	}
	if e := Verify(actual); e != nil {
		return nil, e
	}
	if actual.RegionComparePolicy == nil || actual.GraceRulePolicy == nil || actual.PointerWalkPolicy == nil || actual.InvariantBasePolicy == nil || actual.ForwardCopyPolicy == nil || len(actual.GraceRulePolicy) != 1 || !actual.GraceRulePolicy[RegionCompareRule] || len(actual.PointerWalkPolicy) != 0 || len(actual.InvariantBasePolicy) != 0 || len(actual.ForwardCopyPolicy) != 0 {
		return nil, fmt.Errorf("region compare: unrelated/missing policies")
	}
	reachable := map[Stmt]bool{}
	for _, f := range actual.Funcs {
		walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
			for _, s := range b.Body {
				reachable[s] = true
			}
		})
	}
	for stmt := range actual.SourceLocations {
		if !reachable[stmt] {
			return nil, fmt.Errorf("region compare: detached source origin")
		}
	}
	if e := regionCompareLineage(source, actual); e != nil {
		return nil, e
	}
	for id, want := range actual.RegionComparePolicy {
		if !known[id] || choices[id] != want {
			return nil, fmt.Errorf("region compare: policy differs from requested choice")
		}
	}
	for id, want := range choices {
		if actual.RegionComparePolicy[id] != want {
			return nil, fmt.Errorf("region compare: selected policy missing")
		}
	}
	// The rewrite retains every original origin node (the original while moves
	// into fallback) and never invents origins for generated loop nodes.
	for stmt, origin := range source.SourceLocations {
		got, ok := actual.SourceLocations[stmt]
		if ok && got == origin {
			continue
		}
		// Fresh parses have distinct node addresses: locate unchanged origin-bearing
		// nodes by their exact diagnostic origin and structural content.
		found := false
		for other, loc := range actual.SourceLocations {
			if loc == origin && reflect.DeepEqual(stmt, other) {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("region compare: original source origin changed")
		}
	}
	if len(source.SourceLocations) != len(actual.SourceLocations) {
		return nil, fmt.Errorf("region compare: invented source origin")
	}
	proofs := []RegionCompareProof{}
	for _, id := range ids {
		proof, e := ValidateRegionCompareProof(source, actual, id, choices[id])
		if e != nil {
			return nil, e
		}
		proofs = append(proofs, proof)
	}
	if len(ids) == 0 {
		if e := Verify(actual); e != nil {
			return nil, e
		}
		x, y := *source, *actual
		x.LoopLineage = nil
		y.LoopLineage = nil
		x.RegionComparePolicy = nil
		y.RegionComparePolicy = nil
		x.SourceLocations = nil
		y.SourceLocations = nil
		x.GraceRulePolicy = nil
		y.GraceRulePolicy = nil
		x.PointerWalkPolicy = nil
		y.PointerWalkPolicy = nil
		x.InvariantBasePolicy = nil
		y.InvariantBasePolicy = nil
		x.ForwardCopyPolicy = nil
		y.ForwardCopyPolicy = nil
		if !reflect.DeepEqual(x, y) {
			return nil, fmt.Errorf("region compare: unrelated zero-site module changed")
		}
	}
	return proofs, nil
}
