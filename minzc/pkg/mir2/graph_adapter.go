package mir2

import (
	"fmt"

	"github.com/minz/minzc/pkg/rewrite"
)

// ── Read-only adapter ──────────────────────────────────────────────────────

// FuncAsGraph returns a read-only IRGraph view of a MIR2 Func.
func FuncAsGraph(f *Func) rewrite.IRGraph {
	return &funcGraph{f: f}
}

type funcGraph struct {
	f *Func
}

func (g *funcGraph) Blocks() []string {
	labels := make([]string, len(g.f.Blocks))
	for i, b := range g.f.Blocks {
		labels[i] = b.Label
	}
	return labels
}

func (g *funcGraph) Block(label string) rewrite.IRBlock {
	for _, b := range g.f.Blocks {
		if b.Label == label {
			return &blockAdapter{b: b}
		}
	}
	return nil
}

func (g *funcGraph) Predecessors(label string) []string {
	var preds []string
	for _, b := range g.f.Blocks {
		if b.Term == nil {
			continue
		}
		for _, s := range b.Term.Successors() {
			if s == label {
				preds = append(preds, b.Label)
				break
			}
		}
	}
	return preds
}

func (g *funcGraph) Successors(label string) []string {
	for _, b := range g.f.Blocks {
		if b.Label == label && b.Term != nil {
			return b.Term.Successors()
		}
	}
	return nil
}

func (g *funcGraph) LayoutNext(label string) string {
	for i, b := range g.f.Blocks {
		if b.Label == label && i+1 < len(g.f.Blocks) {
			return g.f.Blocks[i+1].Label
		}
	}
	return ""
}

// ── Block adapter ──────────────────────────────────────────────────────────

type blockAdapter struct {
	b *Block
}

func (a *blockAdapter) Label() string { return a.b.Label }

func (a *blockAdapter) TermKind() string {
	if a.b.Term == nil {
		return ""
	}
	switch a.b.Term.(type) {
	case *TermJmp:
		return "jump"
	case *TermBrIf:
		return "br_if"
	case *TermBrIf2:
		return "br_if2"
	case *TermRet:
		return "ret"
	case *TermCondRet:
		return "cond_ret"
	default:
		return "unknown"
	}
}

func (a *blockAdapter) ParamCount() int { return len(a.b.Params) }
func (a *blockAdapter) InstCount() int  { return len(a.b.Insts) }

func (a *blockAdapter) Inst(i int) rewrite.IRNode {
	if i < 0 || i >= len(a.b.Insts) {
		return nil
	}
	return &instAdapter{inst: a.b.Insts[i]}
}

func (a *blockAdapter) TermTargets() []string {
	if a.b.Term == nil {
		return nil
	}
	return a.b.Term.Successors()
}

// ── Instruction adapter ────────────────────────────────────────────────────

type instAdapter struct {
	inst *Inst
}

func (a *instAdapter) Op() string { return a.inst.Op.String() }

func (a *instAdapter) IsPure() bool { return isDSEPure(a.inst.Op) }

func (a *instAdapter) DefReg() string {
	if a.inst.Dst == NoReg {
		return ""
	}
	return fmt.Sprintf("%%r%d", int(a.inst.Dst))
}

func (a *instAdapter) UseRegs() []string {
	uses := a.inst.Uses()
	strs := make([]string, len(uses))
	for i, r := range uses {
		strs[i] = fmt.Sprintf("%%r%d", int(r))
	}
	return strs
}

// ── Mutable adapter ────────────────────────────────────────────────────────

// FuncAsMutGraph returns a mutable IRGraphMut view of a MIR2 Func.
func FuncAsMutGraph(f *Func) rewrite.IRGraphMut {
	return &funcMutGraph{funcGraph: funcGraph{f: f}, f: f}
}

type funcMutGraph struct {
	funcGraph
	f *Func
}

func (g *funcMutGraph) RemoveInst(blockLabel string, i int) {
	b := g.f.BlockByLabel(blockLabel)
	if b == nil || i < 0 || i >= len(b.Insts) {
		return
	}
	b.Insts = append(b.Insts[:i], b.Insts[i+1:]...)
}

func (g *funcMutGraph) ReplaceInst(blockLabel string, i int, newInst rewrite.IRNode) {
	// Not used in current migration — placeholder
}

func (g *funcMutGraph) AppendInst(blockLabel string, inst rewrite.IRNode) {
	// Not used in current migration — placeholder
}

func (g *funcMutGraph) HoistInsts(srcLabel, dstLabel string) {
	src := g.f.BlockByLabel(srcLabel)
	dst := g.f.BlockByLabel(dstLabel)
	if src == nil || dst == nil {
		return
	}
	dst.Insts = append(dst.Insts, src.Insts...)
	src.Insts = nil
}

func (g *funcMutGraph) RemoveBlock(label string) {
	for i, b := range g.f.Blocks {
		if b.Label == label {
			g.f.Blocks = append(g.f.Blocks[:i], g.f.Blocks[i+1:]...)
			return
		}
	}
}

func (g *funcMutGraph) SetTerm(blockLabel string, termKind string, targets []string) {
	b := g.f.BlockByLabel(blockLabel)
	if b == nil {
		return
	}
	switch termKind {
	case "jump":
		if len(targets) > 0 {
			b.Term = &TermJmp{Target: targets[0]}
		}
	case "ret":
		b.Term = &TermRet{}
	}
}

func (g *funcMutGraph) RemoveBlockParam(blockLabel string, i int) {
	b := g.f.BlockByLabel(blockLabel)
	if b == nil || i < 0 || i >= len(b.Params) {
		return
	}
	b.Params = append(b.Params[:i], b.Params[i+1:]...)
}
