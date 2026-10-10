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

// RunGrace rewrites HIR in place, before lowering. maxRewrites is a module-wide
// budget; zero uses 100, negative is invalid. Each firing rebuilds the structural
// view and verifies lowered MIR2. This is structural validation, not a proof of
// equivalence: differential VM tests remain necessary. HIR has no verifier yet.
func RunGrace(m *Module, maxRewrites int) (*grace.RuleResult, error) {
	result := &grace.RuleResult{ByRule: make(map[string]int)}
	if m == nil || maxRewrites < 0 {
		return result, fmt.Errorf("HIR Grace: invalid module or rewrite budget")
	}
	if maxRewrites == 0 {
		maxRewrites = 100
	}
	rules, err := grace.ParseRules(graceConstantIfRule + "\n" + graceInvariantBaseRule + "\n" + gracePointerWalkRule)
	if err != nil {
		return result, err
	}
	for result.Applied < maxRewrites {
		graph := newHIRGraceGraph(m)
		reg := grace.NewPredicateRegistry()
		reg.RegisterPredicate("hir-has-constant-if", func(_ rewrite.IRGraph, bindings grace.BlockBindings, _ []string) bool {
			return graph.err == nil && graph.candidate(bindings["b"]) >= 0
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
			if err := mir2.Verify(LowerModule(m)); err != nil {
				block.Body[i] = st
				// Save the error outside Grace's bool-only action interface.
				graph.err = fmt.Errorf("HIR Grace hir-constant-if: %w", err)
				return false
			}
			return true
		})
		reg.RegisterPredicate("hir-has-invariant-base", func(_ rewrite.IRGraph, bindings grace.BlockBindings, _ []string) bool {
			return graph.err == nil && len(graph.facts.DB.Query("hir-invariant-base", bindings["b"])) != 0
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
			if err := mir2.Verify(LowerModule(m)); err != nil {
				block.Body = old
				graph.err = fmt.Errorf("HIR Grace hir-invariant-base: %w", err)
				return false
			}
			return true
		})
		reg.RegisterPredicate("hir-has-guarded-pointer-walk", func(_ rewrite.IRGraph, bindings grace.BlockBindings, _ []string) bool {
			return graph.err == nil && len(graph.facts.DB.Query("hir-guarded-pointer-walk", bindings["b"])) != 0
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
			original := b.block.Body[2]
			b.block.Body[2] = p.versionedLoop()
			if err := mir2.Verify(LowerModule(m)); err != nil {
				b.block.Body[2] = original
				graph.err = fmt.Errorf("HIR Grace hir-guarded-pointer-walk: %w", err)
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
	facts   *Facts
}

type hirGraceBlock struct {
	label         string
	block         *Block
	unsafeControl bool
	owner         *Func
}

func newHIRGraceGraph(m *Module) *hirGraceGraph {
	g := &hirGraceGraph{byLabel: make(map[string]*hirGraceBlock), globals: map[string]bool{}, facts: AnalyzeFacts(m)}
	for _, f := range m.Funcs {
		g.globals[f.Name] = true
	}
	for _, v := range m.Globals {
		g.globals[v.Name] = true
	}
	for i, f := range m.Funcs {
		if f.IsExtern || f.Body == nil {
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
	return findGraceHoist(b.owner, b.block, g.globals)
}

// These lowering intrinsics may inline callbacks into the caller environment.
func graceMayInlineCallback(fn string) bool {
	switch fn {
	case "forEach", "mapInPlace", "fold", "reduce", "map", "filter":
		return true
	}
	return false
}
