// cfgengine.go — Layer 4 engine: match finder + action executor.
//
// Takes a Prog and a set of BlockRewriteRules, finds pattern matches,
// and applies rewrites to fixpoint (or maxRounds).
package lir

import "sort"

// ── ProgContext — block property evaluator ──────────────────────────────────

// ProgContext caches block layout and predecessor information for fast matching.
type ProgContext struct {
	Prog       *Prog
	BlockIndex map[string]int      // label → position in layout
	PredMap    map[string][]string  // label → predecessor labels
}

// NewProgContext builds a ProgContext from a Prog.
func NewProgContext(prog *Prog) *ProgContext {
	ctx := &ProgContext{
		Prog:       prog,
		BlockIndex: make(map[string]int, len(prog.Blocks)),
		PredMap:    make(map[string][]string),
	}
	ctx.rebuild()
	return ctx
}

// rebuild recomputes BlockIndex and PredMap from the current Prog state.
func (ctx *ProgContext) rebuild() {
	// Clear maps
	for k := range ctx.BlockIndex {
		delete(ctx.BlockIndex, k)
	}
	for k := range ctx.PredMap {
		delete(ctx.PredMap, k)
	}

	for i, b := range ctx.Prog.Blocks {
		ctx.BlockIndex[b.Label] = i
	}

	for _, b := range ctx.Prog.Blocks {
		for _, t := range b.Term.Targets {
			if t != "" {
				ctx.PredMap[t] = append(ctx.PredMap[t], b.Label)
			}
		}
	}
}

// blockByLabel returns a pointer to the block with the given label, or nil.
func (ctx *ProgContext) blockByLabel(label string) *Block {
	idx, ok := ctx.BlockIndex[label]
	if !ok {
		return nil
	}
	return &ctx.Prog.Blocks[idx]
}

// layoutNext returns the label of the block that follows b in layout order, or "".
func (ctx *ProgContext) layoutNext(label string) string {
	idx, ok := ctx.BlockIndex[label]
	if !ok || idx+1 >= len(ctx.Prog.Blocks) {
		return ""
	}
	return ctx.Prog.Blocks[idx+1].Label
}

// predCount returns the number of predecessor edges to a block.
func (ctx *ProgContext) predCount(label string) int {
	return len(ctx.PredMap[label])
}

// ── Match Finder ───────────────────────────────────────────────────────────

// findMatches returns all valid bindings for a pattern against the current Prog.
// Patterns have 1-2 block variables, so this is O(N) or O(N²).
func findMatches(ctx *ProgContext, rule *BlockRewriteRule) []map[string]*Block {
	pat := &rule.Pattern
	n := len(pat.Blocks)
	if n == 0 {
		return nil
	}

	blocks := ctx.Prog.Blocks
	var results []map[string]*Block

	if n == 1 {
		// Single-variable pattern: linear scan
		for i := range blocks {
			b := &blocks[i]
			if !matchBlock(&pat.Blocks[0], b) {
				continue
			}
			bindings := map[string]*Block{pat.Blocks[0].Var: b}
			if checkEdges(ctx, pat, bindings) && checkWheres(ctx, pat, bindings) && checkCustom(ctx, rule, bindings) {
				results = append(results, bindings)
			}
		}
	} else if n == 2 {
		// Two-variable pattern: O(N²) but N is small (<100 blocks)
		for i := range blocks {
			b0 := &blocks[i]
			if !matchBlock(&pat.Blocks[0], b0) {
				continue
			}
			for j := range blocks {
				if i == j {
					continue
				}
				b1 := &blocks[j]
				if !matchBlock(&pat.Blocks[1], b1) {
					continue
				}
				bindings := map[string]*Block{
					pat.Blocks[0].Var: b0,
					pat.Blocks[1].Var: b1,
				}
				if checkEdges(ctx, pat, bindings) && checkWheres(ctx, pat, bindings) && checkCustom(ctx, rule, bindings) {
					results = append(results, bindings)
				}
			}
		}
	}

	return results
}

// matchBlock checks if a concrete block matches a BlockMatch constraint.
func matchBlock(m *BlockMatch, b *Block) bool {
	if m.TermKind != TermAny && b.Term.Kind != m.TermKind {
		return false
	}
	return true
}

// checkEdges verifies all edge constraints in a pattern.
func checkEdges(ctx *ProgContext, pat *BlockPattern, bindings map[string]*Block) bool {
	for _, edge := range pat.Edges {
		from := bindings[edge.From]
		to := bindings[edge.To]
		if from == nil || to == nil {
			return false
		}
		if !edgeExists(from, to.Label, edge.Kind) {
			return false
		}
	}
	return true
}

// edgeExists checks whether a specific edge kind exists from a block to a target.
func edgeExists(from *Block, toLabel string, kind BlockEdgeKind) bool {
	switch kind {
	case EdgeSucc:
		for _, t := range from.Term.Targets {
			if t == toLabel {
				return true
			}
		}
	case EdgeThen:
		if len(from.Term.Targets) > 0 && from.Term.Targets[0] == toLabel {
			return true
		}
	case EdgeElse:
		if len(from.Term.Targets) > 1 && from.Term.Targets[1] == toLabel {
			return true
		}
	case EdgeBack:
		// Back-edge: would need layout info, but for now check successor
		for _, t := range from.Term.Targets {
			if t == toLabel {
				return true
			}
		}
	}
	return false
}

// checkWheres verifies all WHERE constraints.
func checkWheres(ctx *ProgContext, pat *BlockPattern, bindings map[string]*Block) bool {
	for _, w := range pat.Where {
		if !checkWhere(ctx, &w, bindings) {
			return false
		}
	}
	return true
}

// checkWhere verifies a single WHERE constraint.
func checkWhere(ctx *ProgContext, w *BlockWhere, bindings map[string]*Block) bool {
	switch w.Kind {
	case BWFieldCmp:
		b := bindings[w.Var]
		if b == nil {
			return false
		}
		val := blockField(b, w.Field)
		return cmpInt(val, w.Op, w.Value)

	case BWLayoutNext:
		b := bindings[w.Var]
		if b == nil {
			return false
		}
		next := ctx.layoutNext(b.Label)
		if next == "" {
			return false
		}
		// For Jump: target(b, 0) == layout_next(b)
		if b.Term.Kind == TermJump && len(b.Term.Targets) > 0 {
			return b.Term.Targets[0] == next
		}
		// For Branch: Targets[0] == layout_next(b) (then-target is next block)
		if b.Term.Kind == TermBranch && len(b.Term.Targets) > 0 {
			return b.Term.Targets[0] == next
		}
		return false

	case BWTargetIs:
		b := bindings[w.Var]
		b2 := bindings[w.Var2]
		if b == nil || b2 == nil {
			return false
		}
		for _, t := range b.Term.Targets {
			if t == b2.Label {
				return true
			}
		}
		return false

	case BWPredCount:
		b := bindings[w.Var]
		if b == nil {
			return false
		}
		return cmpInt(ctx.predCount(b.Label), w.Op, w.Value)

	case BWCustom:
		// Custom predicates handled separately
		return true
	}
	return false
}

// checkCustom verifies the rule's custom predicate (if any).
func checkCustom(ctx *ProgContext, rule *BlockRewriteRule, bindings map[string]*Block) bool {
	if rule.Custom == nil {
		return true
	}
	return rule.Custom(ctx.Prog, bindings)
}

// blockField extracts a named field from a block.
func blockField(b *Block, field string) int {
	switch field {
	case "inst_count":
		return len(b.Insts)
	case "param_count":
		return len(b.Params)
	case "term_kind":
		return int(b.Term.Kind)
	case "target_count":
		return len(b.Term.Targets)
	}
	return 0
}

// cmpInt performs an integer comparison.
func cmpInt(a int, op string, b int) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	case ">":
		return a > b
	case "<":
		return a < b
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	}
	return false
}

// ── Action Executor ────────────────────────────────────────────────────────

// applyActions executes a sequence of rewrite actions on a Prog.
// Returns true if any modification was made.
func applyActions(ctx *ProgContext, actions []BlockAction, bindings map[string]*Block) bool {
	applied := false
	for _, action := range actions {
		if applyAction(ctx, &action, bindings) {
			applied = true
		}
	}
	return applied
}

// applyAction executes a single rewrite action.
func applyAction(ctx *ProgContext, action *BlockAction, bindings map[string]*Block) bool {
	switch action.Kind {
	case BADeleteTerm:
		return actionDeleteTerm(ctx, bindings[action.Var])

	case BAMergeBlocks:
		return actionMergeBlocks(ctx, bindings[action.Var], bindings[action.Var2])

	case BARedirectEdge:
		return actionRedirectEdge(ctx, bindings[action.Var])

	case BADeleteBlock:
		return actionDeleteBlock(ctx, bindings[action.Var])

	case BAInvertBranch:
		return actionInvertBranch(bindings[action.Var])

	case BAReplaceTerm:
		return actionReplaceTerm(bindings[action.Var], bindings[action.Var2])

	case BAAbsorbSub:
		return actionAbsorbSub(bindings[action.Var], bindings[action.Var2])
	}
	return false
}

// actionDeleteTerm: TermJump → TermNone (fallthrough).
func actionDeleteTerm(ctx *ProgContext, b *Block) bool {
	if b == nil {
		return false
	}
	b.Term = Term{Kind: TermNone}
	return true
}

// actionMergeBlocks: append s's insts+term into p, delete s from Prog.
func actionMergeBlocks(ctx *ProgContext, p, s *Block) bool {
	if p == nil || s == nil {
		return false
	}
	p.Insts = append(p.Insts, s.Insts...)
	p.Term = s.Term
	// Remove s from Prog
	removeBlock(ctx.Prog, s.Label)
	return true
}

// actionRedirectEdge: walk all blocks, replace references to e.Label with e's target.
func actionRedirectEdge(ctx *ProgContext, e *Block) bool {
	if e == nil || e.Term.Kind != TermJump || len(e.Term.Targets) == 0 {
		return false
	}
	oldLabel := e.Label
	newLabel := e.Term.Targets[0]
	changed := false

	for i := range ctx.Prog.Blocks {
		b := &ctx.Prog.Blocks[i]
		for j, t := range b.Term.Targets {
			if t == oldLabel {
				b.Term.Targets[j] = newLabel
				changed = true
			}
		}
	}
	return changed
}

// actionDeleteBlock: remove block from Prog.Blocks.
func actionDeleteBlock(ctx *ProgContext, b *Block) bool {
	if b == nil {
		return false
	}
	return removeBlock(ctx.Prog, b.Label)
}

// actionInvertBranch: swap Targets[0]/[1] and Args[0]/[1].
func actionInvertBranch(b *Block) bool {
	if b == nil || b.Term.Kind != TermBranch || len(b.Term.Targets) < 2 {
		return false
	}
	b.Term.Targets[0], b.Term.Targets[1] = b.Term.Targets[1], b.Term.Targets[0]
	if len(b.Term.Args) >= 2 {
		b.Term.Args[0], b.Term.Args[1] = b.Term.Args[1], b.Term.Args[0]
	}
	return true
}

// actionReplaceTerm: copy head's branch term to body, adjusting targets.
// head(Branch cond → [body, exit]) + body(Jump → head) becomes:
// body(Branch cond → [body, exit])
func actionReplaceTerm(body, head *Block) bool {
	if body == nil || head == nil {
		return false
	}
	if head.Term.Kind != TermBranch || len(head.Term.Targets) < 2 {
		return false
	}
	bodyLabel := head.Term.Targets[0]
	exitLabel := head.Term.Targets[1]
	body.Term = Term{
		Kind:    TermBranch,
		Cond:    head.Term.Cond,
		Targets: []string{bodyLabel, exitLabel},
	}
	return true
}

// actionAbsorbSub: find sub(cnt,1) in body, remove it, replace body's term with DJNZ.
func actionAbsorbSub(body, head *Block) bool {
	if body == nil || head == nil {
		return false
	}

	cntPhys := head.Term.Cond.Phys
	exitLabel := ""
	if len(head.Term.Targets) >= 2 {
		exitLabel = head.Term.Targets[1]
	}

	// Find and remove the sub(cnt, 1) instruction
	subIdx := -1
	for i, inst := range body.Insts {
		if inst.Pat != nil && inst.Pat.MIROp == OpSub && inst.Dst.Phys == cntPhys {
			subIdx = i
		}
	}
	if subIdx < 0 {
		return false
	}

	body.Insts = append(body.Insts[:subIdx], body.Insts[subIdx+1:]...)
	body.Term = Term{
		Kind:    TermDJNZ,
		Counter: Operand{Phys: cntPhys},
		Targets: []string{body.Label, exitLabel},
	}
	return true
}

// removeBlock removes a block by label from Prog.Blocks.
func removeBlock(prog *Prog, label string) bool {
	for i, b := range prog.Blocks {
		if b.Label == label {
			prog.Blocks = append(prog.Blocks[:i], prog.Blocks[i+1:]...)
			return true
		}
	}
	return false
}

// ── Top-Level Entry Point ──────────────────────────────────────────────────

// ApplyBlockRules applies block rewrite rules to a Prog until fixpoint or maxRounds.
// Rules are tried in priority order (highest first). On first match+apply, restart from top.
func ApplyBlockRules(prog *Prog, rules []BlockRewriteRule, maxRounds int) *BlockRewriteResult {
	result := &BlockRewriteResult{
		ByRule: make(map[string]int),
	}

	// Sort rules by priority descending
	sorted := make([]BlockRewriteRule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	for round := 0; round < maxRounds; round++ {
		result.Rounds = round + 1
		ctx := NewProgContext(prog)
		fired := false

		for ri := range sorted {
			rule := &sorted[ri]
			matches := findMatches(ctx, rule)
			if len(matches) == 0 {
				continue
			}

			// Apply first match only, then restart
			if applyActions(ctx, rule.Actions, matches[0]) {
				result.Applied++
				result.ByRule[rule.Name]++
				fired = true
				break
			}
		}

		if !fired {
			break
		}
	}

	return result
}
