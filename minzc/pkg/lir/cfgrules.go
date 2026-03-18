// cfgrules.go — Layer 4: CFG Block-Level Pattern DSL.
//
// Extends the LIR DSL (Layer 1: Datalog, Layer 2: Cypher, Layer 3: Rewrites)
// with declarative block-level pattern matching and rewriting.
//
// CFG transforms expressed as data-driven rules:
//   - branch-to-next elimination
//   - empty block removal
//   - block merging
//   - branch inversion
//   - loop rotation
//   - DJNZ absorption
//
// Rules are Go struct literals (S-expression parser planned for later).
package lir

// ── Pattern Types ──────────────────────────────────────────────────────────

// BlockPattern describes a CFG shape to match.
type BlockPattern struct {
	Name   string
	Blocks []BlockMatch
	Edges  []BlockEdgeMatch
	Where  []BlockWhere
}

// TermAny is a sentinel value meaning "match any terminator kind".
const TermAny TermKind = -1

// BlockMatch matches a single block by variable name and optional constraints.
type BlockMatch struct {
	Var      string   // variable name: "head", "body", etc.
	TermKind TermKind // required terminator kind (TermAny = any)
}

// BlockEdgeMatch matches a CFG edge between two blocks.
type BlockEdgeMatch struct {
	From string
	To   string
	Kind BlockEdgeKind
}

// BlockEdgeKind classifies a CFG edge.
type BlockEdgeKind int

const (
	EdgeSucc BlockEdgeKind = iota // any successor
	EdgeThen                      // Targets[0] (cond!=0 path)
	EdgeElse                      // Targets[1] (cond==0 path)
	EdgeBack                      // To appears before From in layout
)

// BlockWhere is an additional constraint on a pattern match.
type BlockWhere struct {
	Kind  BlockWhereKind
	Var   string // block variable
	Var2  string // second variable (for BWTargetIs)
	Field string // "inst_count", "param_count"
	Op    string // "==", "!=", ">", "<", ">=", "<="
	Value int    // integer comparand
}

// BlockWhereKind classifies a WHERE constraint.
type BlockWhereKind int

const (
	BWFieldCmp   BlockWhereKind = iota // var.field op value
	BWLayoutNext                        // target(var, 0) == layout_next(var)
	BWTargetIs                          // target(var, N) == var2
	BWPredCount                         // pred_count(var) op value
	BWCustom                            // registered Go predicate
)

// CustomPredicate is a Go function for cross-layer predicates (e.g., DJNZ sub detection).
type CustomPredicate func(prog *Prog, bindings map[string]*Block) bool

// ── Rewrite Rule ───────────────────────────────────────────────────────────

// BlockRewriteRule is a complete Layer 4 rule: pattern + actions + priority.
type BlockRewriteRule struct {
	Name     string
	Pattern  BlockPattern
	Actions  []BlockAction
	Priority int
	Custom   CustomPredicate // optional Go predicate (nil = no custom check)
}

// BlockAction is one rewrite step to apply when a pattern matches.
type BlockAction struct {
	Kind BlockActionKind
	Var  string // target block variable
	Var2 string // second block variable (for Merge, Redirect)
}

// BlockActionKind classifies a rewrite action.
type BlockActionKind int

const (
	BADeleteTerm   BlockActionKind = iota // TermJump → TermNone (fallthrough)
	BAMergeBlocks                          // append Var2's insts+term into Var
	BARedirectEdge                         // redirect predecessors of Var to Var's target
	BADeleteBlock                          // remove block from Prog
	BAInvertBranch                         // swap Targets[0]/[1] and Args[0]/[1]
	BAReplaceTerm                          // copy Var2's branch term to Var
	BAAbsorbSub                            // remove sub(cnt,1), replace term with DJNZ
)

// ── Rewrite Result ─────────────────────────────────────────────────────────

// BlockRewriteResult holds statistics from applying block rules.
type BlockRewriteResult struct {
	Applied int
	ByRule  map[string]int
	Rounds  int
}

// ── Rule Catalog ───────────────────────────────────────────────────────────

// DefaultBlockRules returns the 6 standard CFG optimization rules,
// sorted by priority (highest first).
func DefaultBlockRules() []BlockRewriteRule {
	return []BlockRewriteRule{
		loopRotateDJNZRule(),
		loopRotateRule(),
		emptyBlockElimRule(),
		blockMergeRule(),
		branchToNextRule(),
		branchInversionRule(),
	}
}

// loopRotateDJNZRule: head(Branch,0 insts)→body(Jump)→head, body has sub(cnt,1)
// → absorb sub into DJNZ on body.
// Priority 35 — fires before generic rotation.
func loopRotateDJNZRule() BlockRewriteRule {
	return BlockRewriteRule{
		Name:     "loop_rotate_djnz",
		Priority: 35,
		Pattern: BlockPattern{
			Name: "loop_rotate_djnz",
			Blocks: []BlockMatch{
				{Var: "head", TermKind: TermBranch},
				{Var: "body", TermKind: TermJump},
			},
			Edges: []BlockEdgeMatch{
				{From: "head", To: "body", Kind: EdgeThen},
				{From: "body", To: "head", Kind: EdgeSucc},
			},
			Where: []BlockWhere{
				{Kind: BWFieldCmp, Var: "head", Field: "inst_count", Op: "==", Value: 0},
			},
		},
		Actions: []BlockAction{
			{Kind: BAAbsorbSub, Var: "body", Var2: "head"},
		},
		Custom: djnzSubPredicate,
	}
}

// loopRotateRule: head(Branch,0 insts)→body(Jump)→head
// → copy head's branch to body (bottom-tested loop).
// Priority 30.
func loopRotateRule() BlockRewriteRule {
	return BlockRewriteRule{
		Name:     "loop_rotate",
		Priority: 30,
		Pattern: BlockPattern{
			Name: "loop_rotate",
			Blocks: []BlockMatch{
				{Var: "head", TermKind: TermBranch},
				{Var: "body", TermKind: TermJump},
			},
			Edges: []BlockEdgeMatch{
				{From: "head", To: "body", Kind: EdgeThen},
				{From: "body", To: "head", Kind: EdgeSucc},
			},
			Where: []BlockWhere{
				{Kind: BWFieldCmp, Var: "head", Field: "inst_count", Op: "==", Value: 0},
			},
		},
		Actions: []BlockAction{
			{Kind: BAReplaceTerm, Var: "body", Var2: "head"},
		},
	}
}

// emptyBlockElimRule: block e with Jump, 0 insts, 0 params
// → redirect predecessors to e's target, delete e.
// Priority 20.
func emptyBlockElimRule() BlockRewriteRule {
	return BlockRewriteRule{
		Name:     "empty_block_elim",
		Priority: 20,
		Pattern: BlockPattern{
			Name: "empty_block_elim",
			Blocks: []BlockMatch{
				{Var: "e", TermKind: TermJump},
			},
			Where: []BlockWhere{
				{Kind: BWFieldCmp, Var: "e", Field: "inst_count", Op: "==", Value: 0},
				{Kind: BWFieldCmp, Var: "e", Field: "param_count", Op: "==", Value: 0},
			},
		},
		Actions: []BlockAction{
			{Kind: BARedirectEdge, Var: "e"},
			{Kind: BADeleteBlock, Var: "e"},
		},
	}
}

// blockMergeRule: p(Jump)→s, pred_count(s)==1, 0 params on s
// → merge s into p.
// Priority 15.
func blockMergeRule() BlockRewriteRule {
	return BlockRewriteRule{
		Name:     "block_merge",
		Priority: 15,
		Pattern: BlockPattern{
			Name: "block_merge",
			Blocks: []BlockMatch{
				{Var: "p", TermKind: TermJump},
				{Var: "s", TermKind: TermAny},
			},
			Edges: []BlockEdgeMatch{
				{From: "p", To: "s", Kind: EdgeSucc},
			},
			Where: []BlockWhere{
				{Kind: BWPredCount, Var: "s", Op: "==", Value: 1},
				{Kind: BWFieldCmp, Var: "s", Field: "param_count", Op: "==", Value: 0},
			},
		},
		Actions: []BlockAction{
			{Kind: BAMergeBlocks, Var: "p", Var2: "s"},
		},
	}
}

// branchToNextRule: b(Jump), target == layout_next(b)
// → delete terminator (fallthrough).
// Priority 10.
func branchToNextRule() BlockRewriteRule {
	return BlockRewriteRule{
		Name:     "branch_to_next",
		Priority: 10,
		Pattern: BlockPattern{
			Name: "branch_to_next",
			Blocks: []BlockMatch{
				{Var: "b", TermKind: TermJump},
			},
			Where: []BlockWhere{
				{Kind: BWLayoutNext, Var: "b"},
			},
		},
		Actions: []BlockAction{
			{Kind: BADeleteTerm, Var: "b"},
		},
	}
}

// branchInversionRule: b(Branch), Targets[0] == layout_next(b)
// → swap then/else targets.
// Priority 5.
func branchInversionRule() BlockRewriteRule {
	return BlockRewriteRule{
		Name:     "branch_inversion",
		Priority: 5,
		Pattern: BlockPattern{
			Name: "branch_inversion",
			Blocks: []BlockMatch{
				{Var: "b", TermKind: TermBranch},
			},
			Where: []BlockWhere{
				{Kind: BWLayoutNext, Var: "b"},
			},
		},
		Actions: []BlockAction{
			{Kind: BAInvertBranch, Var: "b"},
		},
	}
}

// ── Custom Predicates ──────────────────────────────────────────────────────

// djnzSubPredicate checks that the "body" block contains a sub(cnt, 1)
// instruction where cnt matches the head's branch condition register.
func djnzSubPredicate(prog *Prog, bindings map[string]*Block) bool {
	head := bindings["head"]
	body := bindings["body"]
	if head == nil || body == nil {
		return false
	}

	cntPhys := head.Term.Cond.Phys
	for _, inst := range body.Insts {
		if inst.Pat != nil && inst.Pat.MIROp == OpSub && inst.Dst.Phys == cntPhys {
			return true
		}
	}
	return false
}
