// vir_graph.go — implements rewrite.IRGraph/IRGraphMut for vir.Func + mir2.Func.
//
// This adapter allows Grace rules (pkg/rewrite/grace) to run on VIROps
// post-lowering, before the Z3 solver. The CFG structure comes from the
// original MIR2 Func (stored as vf.MIRFunc); the instruction view uses VIROps.
//
// Two use cases:
//   1. Pre-solver VIROp simplification (DSE, dead-instruction elimination)
//   2. Post-solver PIROp cleanup (dead register before RET, EX DE,HL elimination)
//
// Note: VIROp blocks are linear — block arguments (PHI destinations) and
// CFG edges come from MIR2. The adapter delegates CFG queries to MIR2.
package vir

import (
	"fmt"
	"strconv"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/rewrite"
)

// ── VIR IRGraph adapter ──────────────────────────────────────────────────────

// VIRGraph wraps a vir.Func + mir2.Func to implement rewrite.IRGraphMut.
// The CFG structure (predecessors, successors, terminators) comes from
// the MIR2 function. The instructions are VIROps.
type VIRGraph struct {
	vf           *Func
	mf           *mir2.Func
	blockByLabel map[string]*Block
	mirByLabel   map[string]*mir2.Block
	predCache    map[string][]string
	succCache    map[string][]string
	layoutOrder  []string
}

// NewVIRGraph creates an IRGraphMut adapter for the given vf + mf pair.
func NewVIRGraph(vf *Func, mf *mir2.Func) *VIRGraph {
	g := &VIRGraph{
		vf:           vf,
		mf:           mf,
		blockByLabel: make(map[string]*Block),
		mirByLabel:   make(map[string]*mir2.Block),
		predCache:    make(map[string][]string),
		succCache:    make(map[string][]string),
	}
	// Index blocks
	for i := range vf.Blocks {
		b := &vf.Blocks[i]
		g.blockByLabel[b.Label] = b
		g.layoutOrder = append(g.layoutOrder, b.Label)
	}
	for _, mb := range mf.Blocks {
		g.mirByLabel[mb.Label] = mb
	}
	// Pre-compute CFG edges from MIR2 terminator info
	// (successors = TermTargets, predecessors = inverse)
	for _, mb := range mf.Blocks {
		if mb.Term == nil {
			continue
		}
		succs := mb.Term.Successors()
		g.succCache[mb.Label] = succs
		for _, s := range succs {
			g.predCache[s] = append(g.predCache[s], mb.Label)
		}
	}
	return g
}

// Blocks returns all block labels in layout order.
func (g *VIRGraph) Blocks() []string { return g.layoutOrder }

// Block returns the IRBlock view for the named block, or nil.
func (g *VIRGraph) Block(label string) rewrite.IRBlock {
	vb, ok := g.blockByLabel[label]
	if !ok {
		return nil
	}
	mb := g.mirByLabel[label]
	return &virIRBlock{vb: vb, mb: mb}
}

// Predecessors returns predecessor block labels.
func (g *VIRGraph) Predecessors(label string) []string { return g.predCache[label] }

// Successors returns successor block labels.
func (g *VIRGraph) Successors(label string) []string { return g.succCache[label] }

// LayoutNext returns the next block in layout order, or "".
func (g *VIRGraph) LayoutNext(label string) string {
	for i, l := range g.layoutOrder {
		if l == label && i+1 < len(g.layoutOrder) {
			return g.layoutOrder[i+1]
		}
	}
	return ""
}

// ── IRGraphMut mutations ─────────────────────────────────────────────────────

// RemoveInst removes VIROp at index i from the named block.
func (g *VIRGraph) RemoveInst(blockLabel string, i int) {
	vb := g.blockByLabel[blockLabel]
	if vb == nil || i < 0 || i >= len(vb.Ops) {
		return
	}
	vb.Ops = append(vb.Ops[:i], vb.Ops[i+1:]...)
}

// ReplaceInst replaces VIROp at index i.
func (g *VIRGraph) ReplaceInst(blockLabel string, i int, newInst rewrite.IRNode) {
	vb := g.blockByLabel[blockLabel]
	if vb == nil || i < 0 || i >= len(vb.Ops) {
		return
	}
	if vn, ok := newInst.(*virIRNode); ok {
		vb.Ops[i] = vn.op
	}
}

// AppendInst appends a VIROp to a block.
func (g *VIRGraph) AppendInst(blockLabel string, inst rewrite.IRNode) {
	vb := g.blockByLabel[blockLabel]
	if vb == nil {
		return
	}
	if vn, ok := inst.(*virIRNode); ok {
		vb.Ops = append(vb.Ops, vn.op)
	}
}

// HoistInsts moves all VIROps from src to the end of dst (block merge).
func (g *VIRGraph) HoistInsts(srcLabel, dstLabel string) {
	src := g.blockByLabel[srcLabel]
	dst := g.blockByLabel[dstLabel]
	if src == nil || dst == nil {
		return
	}
	dst.Ops = append(dst.Ops, src.Ops...)
	src.Ops = nil
}

// RemoveBlock removes a block from the graph (marks it empty).
func (g *VIRGraph) RemoveBlock(label string) {
	if vb := g.blockByLabel[label]; vb != nil {
		vb.Ops = nil
		vb.Params = nil
	}
	// Remove from layout order
	for i, l := range g.layoutOrder {
		if l == label {
			g.layoutOrder = append(g.layoutOrder[:i], g.layoutOrder[i+1:]...)
			break
		}
	}
	// Remove from CFG caches
	delete(g.succCache, label)
	for pred, succs := range g.succCache {
		for i, s := range succs {
			if s == label {
				g.succCache[pred] = append(succs[:i], succs[i+1:]...)
				break
			}
		}
	}
	delete(g.predCache, label)
}

// SetTerm updates the terminator of a block (updates CFG caches).
func (g *VIRGraph) SetTerm(blockLabel string, termKind string, targets []string) {
	// Update CFG caches
	oldSuccs := g.succCache[blockLabel]
	for _, s := range oldSuccs {
		preds := g.predCache[s]
		for i, p := range preds {
			if p == blockLabel {
				g.predCache[s] = append(preds[:i], preds[i+1:]...)
				break
			}
		}
	}
	g.succCache[blockLabel] = targets
	for _, t := range targets {
		g.predCache[t] = append(g.predCache[t], blockLabel)
	}
}

// RemoveBlockParam removes block parameter at index i.
func (g *VIRGraph) RemoveBlockParam(blockLabel string, i int) {
	vb := g.blockByLabel[blockLabel]
	if vb == nil || i < 0 || i >= len(vb.Params) {
		return
	}
	vb.Params = append(vb.Params[:i], vb.Params[i+1:]...)
}

// ── virIRBlock ───────────────────────────────────────────────────────────────

type virIRBlock struct {
	vb *Block
	mb *mir2.Block // may be nil for synthetic blocks
}

func (b *virIRBlock) Label() string { return b.vb.Label }

// TermKind returns the MIR2 terminator kind for this block.
func (b *virIRBlock) TermKind() string {
	if b.mb == nil || b.mb.Term == nil {
		// Infer from last VIROp
		if len(b.vb.Ops) > 0 {
			last := b.vb.Ops[len(b.vb.Ops)-1]
			switch last.Op {
			case OpRet:
				return "ret"
			case OpCondRet:
				return "cond_ret"
			}
		}
		return "ret"
	}
	// Delegate to MIR2 term
	switch b.mb.Term.(type) {
	case *mir2.TermRet:
		return "ret"
	case *mir2.TermBrIf:
		return "br_if"
	case *mir2.TermJmp:
		return "jump"
	default:
		return "ret"
	}
}

func (b *virIRBlock) ParamCount() int  { return len(b.vb.Params) }
func (b *virIRBlock) InstCount() int   { return len(b.vb.Ops) }

func (b *virIRBlock) Inst(i int) rewrite.IRNode {
	if i < 0 || i >= len(b.vb.Ops) {
		return nil
	}
	return &virIRNode{op: b.vb.Ops[i]}
}

func (b *virIRBlock) TermTargets() []string {
	if b.mb == nil || b.mb.Term == nil {
		return nil
	}
	return b.mb.Term.Successors()
}

// ── virIRNode ────────────────────────────────────────────────────────────────

type virIRNode struct {
	op VIROp
}

// Op returns a human-readable opcode name for the VIROp.
func (n *virIRNode) Op() string { return opName(n.op.Op) }

// IsPure reports whether this VIROp has no side effects.
// Pure ops: const, move, add, sub, and, or, xor, shl, shr, neg, cmp + imm variants.
// Impure: store, call, asm_block, ret, cond_ret.
func (n *virIRNode) IsPure() bool {
	switch n.op.Op {
	case OpConst, OpMove, OpAdd, OpSub, OpAnd, OpOr, OpXor,
		OpShl, OpShr, OpNeg, OpCmp, OpMul,
		OpAddImm, OpSubImm, OpAndImm, OpOrImm, OpXorImm, OpCmpImm,
		OpLoad, OpLoad16LE:
		return true
	}
	return false
}

// DefReg returns the defined register as a string (vreg ID).
func (n *virIRNode) DefReg() string {
	if n.op.Dst > 0 {
		return fmt.Sprintf("v%d", n.op.Dst)
	}
	return ""
}

// UseRegs returns the used register names (vreg IDs).
func (n *virIRNode) UseRegs() []string {
	var uses []string
	for _, s := range n.op.Src {
		if s > 0 {
			uses = append(uses, fmt.Sprintf("v%d", s))
		}
	}
	return uses
}

// VIROpFromNode recovers the VIROp from an IRNode (type assertion).
func VIROpFromNode(n rewrite.IRNode) (VIROp, bool) {
	if vn, ok := n.(*virIRNode); ok {
		return vn.op, true
	}
	return VIROp{}, false
}

// NewVIRNode wraps a VIROp as an IRNode.
func NewVIRNode(op VIROp) rewrite.IRNode { return &virIRNode{op: op} }

// ── Convenience: apply VIROp-level Grace DSE ─────────────────────────────────

// ApplyVIRDSE removes dead stores (OpStore where the stored value is
// immediately overwritten before any read). Conservative: only removes
// when the store destination vreg is overwritten in the same block.
// Returns the number of stores removed.
func ApplyVIRDSE(vf *Func) int {
	removed := 0
	for bi := range vf.Blocks {
		b := &vf.Blocks[bi]
		removed += virDSEBlock(b)
	}
	return removed
}

// virDSEBlock eliminates dead VIROps within a single block.
// Conservative: only handles dead instructions whose dst is never used.
func virDSEBlock(b *Block) int {
	if len(b.Ops) == 0 {
		return 0
	}

	// Build use set: vregs that are used by any op
	used := make(map[int]bool)
	for _, op := range b.Ops {
		for _, s := range op.Src {
			if s > 0 {
				used[s] = true
			}
		}
		// AsmIns are also uses
		for _, v := range op.AsmIns {
			if v > 0 {
				used[v] = true
			}
		}
	}

	// Remove ops whose dst is defined but never used (pure ops only)
	// Multiple passes until fixpoint
	changed := true
	totalRemoved := 0
	for changed {
		changed = false
		// Recompute used set (dst of remaining ops may have become unused)
		used = make(map[int]bool)
		for _, op := range b.Ops {
			for _, s := range op.Src {
				if s > 0 {
					used[s] = true
				}
			}
			for _, v := range op.AsmIns {
				if v > 0 {
					used[v] = true
				}
			}
		}
		var newOps []VIROp
		for _, op := range b.Ops {
			// Remove if: pure, has dst, dst never used
			if op.Dst > 0 && !used[op.Dst] && isPureVIROp(op) {
				changed = true
				totalRemoved++
				continue
			}
			newOps = append(newOps, op)
		}
		b.Ops = newOps
	}
	return totalRemoved
}

// isPureVIROp returns true for ops without side effects (safe to eliminate if dead).
func isPureVIROp(op VIROp) bool {
	switch op.Op {
	case OpConst, OpMove, OpAdd, OpSub, OpAnd, OpOr, OpXor,
		OpShl, OpShr, OpNeg, OpMul,
		OpAddImm, OpSubImm, OpAndImm, OpOrImm, OpXorImm:
		return true
	}
	return false
}

// ── Suppress unused import ───────────────────────────────────────────────────

var _ = strconv.Itoa // used in virIRNode if needed
