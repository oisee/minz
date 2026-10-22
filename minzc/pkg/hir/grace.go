package hir

import (
	_ "embed"
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/rewrite"
	"github.com/minz/minzc/pkg/rewrite/grace"
)

// The first HIR rule deliberately requires a literal boolean. It never drops
// evaluation of an effectful condition or relies on inferred purity.
//
//go:embed rules/constant_if.grace
var graceConstantIfRule string

//go:embed rules/invariant_base.grace
var graceInvariantBaseRule string

//go:embed rules/pointer_walk.grace
var gracePointerWalkRule string

//go:embed rules/forward_copy.grace
var graceForwardCopyRule string

//go:embed rules/region_compare.grace
var graceRegionCompareRule string

// GraceOptions selects the rewrite budget and conservative size policy.
// OptimizeForSize excludes versioned pointer loops, which duplicate the body.
// It does not promise globally minimal machine code.
type GraceOptions struct {
	Functions        map[string]bool // nil: all; otherwise explicit function allowlist
	MaxRewrites      int
	OptimizeForSize  bool
	PointerWalkSites map[string]bool // nil: default; non-nil: only attributed selected sites
}

// RunGrace rewrites HIR in place, before lowering. maxRewrites is a module-wide
// budget; zero uses 100, negative is invalid. Each firing rebuilds the structural
// view and verifies HIR followed by lowered MIR2. This is structural validation,
// not a proof of equivalence: differential VM tests remain necessary.
func RunGrace(m *Module, maxRewrites int) (*grace.RuleResult, error) {
	return RunGraceWithOptions(m, GraceOptions{MaxRewrites: maxRewrites})
}

// RunGraceWithOptions applies structural rules under the given budget and policy.
func RunGraceWithOptions(m *Module, options GraceOptions) (*grace.RuleResult, error) {
	if options.PointerWalkSites == nil && m != nil {
		options.PointerWalkSites = m.PointerWalkPolicy
	}
	maxRewrites := options.MaxRewrites
	result := &grace.RuleResult{ByRule: make(map[string]int)}
	if m == nil || maxRewrites < 0 {
		return result, fmt.Errorf("HIR Grace: invalid module or rewrite budget")
	}
	if maxRewrites == 0 {
		maxRewrites = 100
	}
	if err := Verify(m); err != nil {
		return result, err
	}
	rules, err := grace.ParseRules(graceConstantIfRule + "\n" + graceInvariantBaseRule + "\n" + gracePointerWalkRule + "\n" + graceForwardCopyRule + "\n" + graceRegionCompareRule)
	if err != nil {
		return result, err
	}
	for result.Applied < maxRewrites {
		graph := newHIRGraceGraph(m)
		if options.Functions != nil {
			labels := graph.labels[:0]
			for _, label := range graph.labels {
				if options.Functions[graph.byLabel[label].owner.Name] {
					labels = append(labels, label)
				} else {
					delete(graph.byLabel, label)
				}
			}
			graph.labels = labels
		}
		reg := grace.NewPredicateRegistry()
		reg.RegisterPredicate("hir-has-constant-if", func(_ rewrite.IRGraph, bindings grace.BlockBindings, _ []string) bool {
			return (m.GraceRulePolicy == nil || m.GraceRulePolicy["hir-constant-if"]) && graph.err == nil && graph.candidate(bindings["b"]) >= 0
		})
		reg.RegisterAction("hirSelectBranch", func(_ rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
			label := bindings["b"]
			i := graph.candidate(label)
			if i < 0 {
				return false
			}
			block := graph.byLabel[label].block
			st := block.Body[i].(*IfStmt)
			selected := st.Else
			if st.Cond.(*BoolLitExpr).Val {
				selected = st.Then
			}
			if selected == nil {
				selected = Blk()
			}
			block.Body[i] = selected // keep a structured block, never flatten
			if err := verifyGraceModule(m); err != nil {
				block.Body[i] = st
				// Save the error outside Grace's bool-only action interface.
				graph.err = fmt.Errorf("HIR Grace hir-constant-if: %w", err)
				return false
			}
			return true
		})
		reg.RegisterPredicate("hir-has-invariant-base", func(_ rewrite.IRGraph, bindings grace.BlockBindings, _ []string) bool {
			return (m.GraceRulePolicy == nil || m.GraceRulePolicy["hir-invariant-base"]) && graph.err == nil && graph.hoist(bindings["b"]) != nil
		})
		reg.RegisterAction("hirHoistBase", func(_ rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
			h := graph.hoist(bindings["b"])
			if h == nil {
				return false
			}
			block := graph.byLabel[bindings["b"]].block
			value := Var(h.name, h.expr.Ty)
			loop := *h.loop
			loop.Cond = replaceGraceExpr(h.loop.Cond, h.expr, value)
			loop.Body = rewriteGraceBlock(h.loop.Body, h.expr, value)
			old := block.Body
			body := append([]Stmt(nil), old[:h.index]...)
			body = append(body, Decl(h.name, h.expr.Ty, h.expr))
			body = append(body, &loop)
			block.Body = append(body, old[h.index+1:]...)
			if origin, ok := m.LoopLineage[h.loop]; ok {
				origin.Role = "invariant-base"
				origin.Rule = "hir-invariant-base"
				m.LoopLineage[&loop] = origin
			}
			if err := verifyGraceModule(m); err != nil {
				block.Body = old
				delete(m.LoopLineage, &loop)
				graph.err = fmt.Errorf("HIR Grace hir-invariant-base: %w", err)
				return false
			}
			return true
		})
		reg.RegisterPredicate("hir-has-guarded-pointer-walk", func(_ rewrite.IRGraph, bindings grace.BlockBindings, _ []string) bool {
			if options.OptimizeForSize || graph.err != nil || len(graph.facts.DB.Query("hir-guarded-pointer-walk", bindings["b"])) == 0 {
				return false
			}
			if options.PointerWalkSites != nil {
				b := graph.byLabel[bindings["b"]]
				plan := findGracePointerWalk(b.owner, b.block, graph.globals)
				if plan == nil {
					return false
				}
				origin, ok := m.LoopLineage[plan.loop]
				return ok && options.PointerWalkSites[origin.SiteID]
			}
			return true
		})
		reg.RegisterAction("hirGuardPointerWalk", func(_ rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
			if graph.err != nil {
				return false
			}
			b := graph.byLabel[bindings["b"]]
			if b == nil {
				return false
			}
			p := findGracePointerWalk(b.owner, b.block, graph.globals)
			if p == nil {
				return false
			}
			original := b.block.Body[p.loopIndex]
			previousOrigin, hadOrigin := m.LoopLineage[p.loop]
			versioned := p.versionedLoop()
			b.block.Body[p.loopIndex] = versioned
			if origin, ok := m.LoopLineage[p.loop]; ok {
				fast := origin
				if origin.Rule == "hir-invariant-base" && p.weight != nil && p.loopIndex == 3 {
					fast.PreviousRule = origin.Rule
				}
				fast.Role = "fast"
				fast.Rule = "hir-guarded-pointer-walk"
				fallback := origin
				fallback.Role = "fallback"
				fallback.Rule = fast.Rule
				fallback.PreviousRule = fast.PreviousRule
				m.LoopLineage[versioned.Then.Body[0]] = fast
				m.LoopLineage[p.loop] = fallback
			}
			if err := verifyGraceModule(m); err != nil {
				b.block.Body[p.loopIndex] = original
				delete(m.LoopLineage, versioned.Then.Body[0])
				if hadOrigin {
					m.LoopLineage[p.loop] = previousOrigin
				}
				graph.err = fmt.Errorf("HIR Grace hir-guarded-pointer-walk: %w", err)
				return false
			}
			return true
		})
		reg.RegisterPredicate("hir-has-forward-copy", func(_ rewrite.IRGraph, bindings grace.BlockBindings, _ []string) bool {
			if (options.OptimizeForSize && m.ForwardCopyPolicy == nil) || m.GraceRulePolicy == nil || !m.GraceRulePolicy[ForwardCopyRule] {
				return false
			}
			b := graph.byLabel[bindings["b"]]
			if b == nil {
				return false
			}
			plan := findGraceForwardCopy(b.owner, b.block, graph.globals)
			if plan == nil {
				return false
			}
			if m.ForwardCopyPolicy != nil {
				origin, ok := m.LoopLineage[plan.loop]
				return ok && m.ForwardCopyPolicy[origin.SiteID]
			}
			return true
		})
		reg.RegisterAction("hirForwardCopy", func(_ rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
			b := graph.byLabel[bindings["b"]]
			if b == nil {
				return false
			}
			p := findGraceForwardCopy(b.owner, b.block, graph.globals)
			if p == nil {
				return false
			}
			old := b.block.Body[1]
			versioned := p.versionedLoop()
			b.block.Body[1] = versioned
			fast := versioned.Then.Body[1]
			origin, mapped := m.LoopLineage[p.loop]
			if mapped {
				f := origin
				f.Role = "fast"
				f.Rule = ForwardCopyRule
				m.LoopLineage[fast] = f
				s := origin
				s.Role = "fallback"
				s.Rule = ForwardCopyRule
				m.LoopLineage[p.loop] = s
			}
			if err := verifyGraceModule(m); err != nil {
				if mapped {
					delete(m.LoopLineage, fast)
					m.LoopLineage[p.loop] = origin
				}
				b.block.Body[1] = old
				graph.err = fmt.Errorf("HIR Grace forward copy: %w", err)
				return false
			}
			return true
		})
		reg.RegisterPredicate("hir-has-region-compare", func(_ rewrite.IRGraph, bindings grace.BlockBindings, _ []string) bool {
			if graph.err != nil || m.RegionComparePolicy == nil || m.GraceRulePolicy == nil || !m.GraceRulePolicy[RegionCompareRule] {
				return false
			}
			b := graph.byLabel[bindings["b"]]
			if b == nil || b.block != b.owner.Body {
				return false
			}
			p := findRegionCompare(b.owner, graph.globals)
			if p == nil {
				return false
			}
			origin, ok := m.LoopLineage[p.loop]
			return ok && origin.Role == "original" && origin.Rule == "" && m.RegionComparePolicy[origin.SiteID]
		})
		reg.RegisterAction("hirRegionCompare", func(_ rewrite.IRGraphMut, bindings grace.BlockBindings) bool {
			b := graph.byLabel[bindings["b"]]
			if b == nil {
				return false
			}
			p := findRegionCompare(b.owner, graph.globals)
			if p == nil {
				return false
			}
			origin, ok := m.LoopLineage[p.loop]
			if !ok {
				return false
			}
			old := b.block.Body[1]
			replacement, tail := p.rewrite()
			b.block.Body[1] = replacement
			m.LoopLineage[p.loop] = LoopLineage{SiteID: origin.SiteID, Role: "fallback", Rule: RegionCompareRule}
			m.LoopLineage[tail] = LoopLineage{SiteID: origin.SiteID, Role: "fast", Rule: RegionCompareRule}
			if e := verifyGraceModule(m); e != nil {
				b.block.Body[1] = old
				m.LoopLineage[p.loop] = origin
				delete(m.LoopLineage, tail)
				graph.err = fmt.Errorf("HIR Grace region compare: %w", e)
				return false
			}
			return true
		})
		one := grace.ApplyRules(graph, rules, reg, 1)
		result.Rounds++
		if graph.err != nil {
			return result, graph.err
		}
		if one.Applied == 0 {
			break
		}
		result.Applied += one.Applied
		for name, count := range one.ByRule {
			result.ByRule[name] += count
		}
	}
	return result, nil
}

// This view exposes lexical blocks, not CFG blocks. Edges and register facts
// are intentionally unavailable. Only the registered HIR custom action mutates
// it; CFG mutation operations are rejected rather than silently misinterpreted.
type hirGraceGraph struct {
	labels  []string
	byLabel map[string]*hirGraceBlock
	err     error
	globals map[string]bool
	module  *Module
	facts   *Facts
}

type hirGraceBlock struct {
	label         string
	block         *Block
	unsafeControl bool
	owner         *Func
}

func newHIRGraceGraph(m *Module) *hirGraceGraph {
	g := &hirGraceGraph{module: m, byLabel: make(map[string]*hirGraceBlock), globals: map[string]bool{}, facts: AnalyzeFacts(m)}
	for _, f := range m.Funcs {
		g.globals[f.Name] = true
	}
	for _, v := range m.Globals {
		g.globals[v.Name] = true
	}
	for i, f := range m.Funcs {
		if f.IsExtern || f.Body == nil || hasVolatileAccess(m, f.Body) {
			continue
		}
		unsafe := false
		walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
			for _, s := range b.Body {
				switch s.(type) {
				case *LabelStmt, *GotoStmt:
					unsafe = true // labels can make a discarded branch reachable
				}
			}
		})
		walkGraceBlocks(f.Body, fmt.Sprintf("f%d", i), func(label string, b *Block) {
			g.labels = append(g.labels, label)
			g.byLabel[label] = &hirGraceBlock{label: label, block: b, unsafeControl: unsafe, owner: f}
		})
	}
	return g
}

func walkGraceBlocks(b *Block, path string, visit func(string, *Block)) {
	if b == nil {
		return
	}
	visit(path, b)
	for i, s := range b.Body {
		p := fmt.Sprintf("%s/s%d", path, i)
		switch st := s.(type) {
		case *Block:
			walkGraceBlocks(st, p, visit)
		case *IfStmt:
			walkGraceBlocks(st.Then, p+"/then", visit)
			walkGraceBlocks(st.Else, p+"/else", visit)
		case *WhileStmt:
			walkGraceBlocks(st.Body, p+"/while", visit)
		case *ForRangeStmt:
			walkGraceBlocks(st.Body, p+"/range", visit)
		case *ForEachStmt:
			walkGraceBlocks(st.Body, p+"/each", visit)
		case *SwitchStmt:
			for j, c := range st.Cases {
				walkGraceBlocks(c.Body, fmt.Sprintf("%s/case%d", p, j), visit)
			}
			walkGraceBlocks(st.Default, p+"/default", visit)
		}
	}
}

// A branch-local binding must not leak into the surrounding lowerer env.
// Until HIR has explicit scope nodes, leave such branches alone.
func graceHasBindings(b *Block) bool {
	found := false
	walkGraceBlocks(b, "", func(_ string, block *Block) {
		for _, s := range block.Body {
			visitStmtExprs(s, func(e Expr) {
				switch x := e.(type) {
				case *LetInExpr:
					found = true
				case *CallExpr:
					if graceMayInlineCallback(x.Fn) {
						found = true
					}
				}
			})
			switch s.(type) {
			case *VarDeclStmt, *TupleLetStmt, *ForRangeStmt, *ForEachStmt:
				found = true
			}
		}
	})
	return found
}

func (g *hirGraceGraph) candidate(label string) int {
	b := g.byLabel[label]
	if b == nil || b.unsafeControl {
		return -1
	}
	for i, s := range b.block.Body {
		st, ok := s.(*IfStmt)
		if !ok || st.Then == nil {
			continue
		}
		cond, ok := st.Cond.(*BoolLitExpr)
		if !ok {
			continue
		}
		selected := st.Else
		if cond.Val {
			selected = st.Then
		}
		if !graceHasBindings(selected) {
			return i
		}
	}
	return -1
}

func (g *hirGraceGraph) Blocks() []string { return g.labels }
func (g *hirGraceGraph) Block(label string) rewrite.IRBlock {
	if b := g.byLabel[label]; b != nil {
		return b
	}
	return nil
}
func (*hirGraceGraph) Predecessors(string) []string { return nil }
func (*hirGraceGraph) Successors(string) []string   { return nil }
func (*hirGraceGraph) LayoutNext(string) string     { return "" }
func (*hirGraceGraph) RemoveInst(string, int)       { panic("HIR Grace: CFG mutation unsupported") }
func (*hirGraceGraph) ReplaceInst(string, int, rewrite.IRNode) {
	panic("HIR Grace: CFG mutation unsupported")
}
func (*hirGraceGraph) AppendInst(string, rewrite.IRNode) {
	panic("HIR Grace: CFG mutation unsupported")
}
func (*hirGraceGraph) HoistInsts(string, string)        { panic("HIR Grace: CFG mutation unsupported") }
func (*hirGraceGraph) RemoveBlock(string)               { panic("HIR Grace: CFG mutation unsupported") }
func (*hirGraceGraph) SetTerm(string, string, []string) { panic("HIR Grace: CFG mutation unsupported") }
func (*hirGraceGraph) RemoveBlockParam(string, int)     { panic("HIR Grace: CFG mutation unsupported") }
func (b *hirGraceBlock) Label() string                  { return b.label }
func (*hirGraceBlock) TermKind() string                 { return "hir.block" }
func (*hirGraceBlock) ParamCount() int                  { return 0 }
func (*hirGraceBlock) InstCount() int                   { return 0 }
func (*hirGraceBlock) Inst(int) rewrite.IRNode          { return nil }
func (*hirGraceBlock) TermTargets() []string            { return nil }

func (g *hirGraceGraph) hoist(label string) *graceHoist {
	b := g.byLabel[label]
	if b == nil || b.unsafeControl {
		return nil
	}
	var selected func(*WhileStmt) bool
	if g.module.InvariantBasePolicy != nil {
		selected = func(loop *WhileStmt) bool {
			origin, ok := g.module.LoopLineage[loop]
			return ok && g.module.InvariantBasePolicy[origin.SiteID]
		}
	}
	return findGraceHoistSelected(b.owner, b.block, g.globals, selected)
}

// These lowering intrinsics may inline callbacks into the caller environment.
func graceMayInlineCallback(fn string) bool {
	switch fn {
	case "forEach", "mapInPlace", "fold", "reduce", "map", "filter":
		return true
	}
	return false
}

// Structured memory rewrites need a separate ordering proof for volatile
// accesses. Keep these functions in the normal MIR optimization path instead.
func hasVolatileAccess(m *Module, body *Block) bool {
	globals := map[string]bool{}
	for _, g := range m.Globals {
		globals[g.Name] = g.Volatile || mir2.HasVolatileStorage(g.Ty)
	}
	found := false
	walkGraceBlocks(body, "", func(_ string, b *Block) {
		for _, s := range b.Body {
			if d, ok := s.(*VarDeclStmt); ok {
				found = found || d.Volatile || mir2.HasVolatileStorage(d.Ty)
			}
			if d, ok := s.(*StoreStmt); ok && d.Volatile {
				found = true
			}
			visitStmtExprs(s, func(e Expr) {
				switch x := e.(type) {
				case *VarRefExpr:
					found = found || x.Volatile || globals[x.Name]
				case *LoadExpr:
					found = found || x.Volatile
				case *DerefExpr:
					found = found || x.Volatile
				case *FieldExpr:
					found = found || x.Volatile
				case *IndexExpr:
					found = found || x.Volatile
				}
			})
		}
	})
	return found
}
