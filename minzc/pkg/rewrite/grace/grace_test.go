package grace

import (
	"testing"

	"github.com/minz/minzc/pkg/rewrite"
)

// ── Mock IR Graph ──────────────────────────────────────────────────────────

type mockGraph struct {
	blocks  []mockBlock
	predMap map[string][]string
}

type mockBlock struct {
	label    string
	termKind string
	targets  []string
	params   int
	insts    []mockInst
}

type mockInst struct {
	op     string
	pure   bool
	defReg string
	uses   []string
}

func (g *mockGraph) Blocks() []string {
	labels := make([]string, len(g.blocks))
	for i, b := range g.blocks {
		labels[i] = b.label
	}
	return labels
}

func (g *mockGraph) Block(label string) rewrite.IRBlock {
	for i := range g.blocks {
		if g.blocks[i].label == label {
			return &g.blocks[i]
		}
	}
	return nil
}

func (g *mockGraph) Predecessors(label string) []string {
	if g.predMap != nil {
		return g.predMap[label]
	}
	var preds []string
	for _, b := range g.blocks {
		for _, t := range b.targets {
			if t == label {
				preds = append(preds, b.label)
			}
		}
	}
	return preds
}

func (g *mockGraph) Successors(label string) []string {
	b := g.Block(label)
	if b == nil {
		return nil
	}
	return b.TermTargets()
}

func (g *mockGraph) LayoutNext(label string) string {
	for i, b := range g.blocks {
		if b.label == label && i+1 < len(g.blocks) {
			return g.blocks[i+1].label
		}
	}
	return ""
}

func (b *mockBlock) Label() string        { return b.label }
func (b *mockBlock) TermKind() string      { return b.termKind }
func (b *mockBlock) ParamCount() int       { return b.params }
func (b *mockBlock) InstCount() int        { return len(b.insts) }
func (b *mockBlock) TermTargets() []string { return b.targets }

func (b *mockBlock) Inst(i int) rewrite.IRNode {
	if i < 0 || i >= len(b.insts) {
		return nil
	}
	return &b.insts[i]
}

func (inst *mockInst) Op() string       { return inst.op }
func (inst *mockInst) IsPure() bool     { return inst.pure }
func (inst *mockInst) DefReg() string   { return inst.defReg }
func (inst *mockInst) UseRegs() []string { return inst.uses }

// ── Category 1: Block Discovery ────────────────────────────────────────────

func TestCategory1_BlockDiscovery(t *testing.T) {
	// Find all blocks where terminator is "jump"
	// Cypher: MATCH (b:Block) WHERE b.term='jump'
	g := &mockGraph{blocks: []mockBlock{
		{label: "entry", termKind: "br_if", targets: []string{"then", "else"}},
		{label: "then", termKind: "jump", targets: []string{"merge"}},
		{label: "else", termKind: "ret"},
		{label: "merge", termKind: "ret"},
	}}

	src := `(grace find-jumps 0
		(match (block ?b (term-kind "jump")))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	if len(matches) != 1 {
		t.Fatalf("expected 1 jump block, got %d", len(matches))
	}
	if matches[0]["b"] != "then" {
		t.Errorf("expected 'then', got %s", matches[0]["b"])
	}
}

// ── Category 2: Edge Patterns ──────────────────────────────────────────────

func TestCategory2_EdgePatterns(t *testing.T) {
	// Find then-branch pairs
	// Cypher: MATCH (a)-[:THEN]->(b)
	g := &mockGraph{blocks: []mockBlock{
		{label: "entry", termKind: "br_if", targets: []string{"then", "else"}},
		{label: "then", termKind: "ret"},
		{label: "else", termKind: "ret"},
	}}

	src := `(grace find-then-pairs 0
		(match
			(block ?a (term-kind "br_if"))
			(block ?b)
			(edge ?a ?b "then"))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	if len(matches) != 1 {
		t.Fatalf("expected 1 then-pair, got %d", len(matches))
	}
	if matches[0]["a"] != "entry" || matches[0]["b"] != "then" {
		t.Errorf("expected entry→then, got %s→%s", matches[0]["a"], matches[0]["b"])
	}
}

// ── Category 3: Control Flow Paths ─────────────────────────────────────────

func TestCategory3_ControlFlowPaths(t *testing.T) {
	// Find blocks that jump to another block
	// Cypher: MATCH (a)-[:SUCC]->(b)
	g := &mockGraph{blocks: []mockBlock{
		{label: "entry", termKind: "jump", targets: []string{"loop"}},
		{label: "loop", termKind: "br_if", targets: []string{"loop", "exit"}},
		{label: "exit", termKind: "ret"},
	}}

	src := `(grace find-succ 0
		(match
			(block ?a)
			(block ?b)
			(edge ?a ?b "succ"))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	// entry→loop, loop→loop (self), loop→exit = but loop→loop excluded (same block)
	// Actually: entry→loop, loop→exit = 2 matches with distinct blocks
	// loop→loop is excluded because a != b constraint (unique blocks)
	if len(matches) != 2 {
		t.Fatalf("expected 2 successor pairs, got %d", len(matches))
	}
}

// ── Category 4: Diamond Detection ──────────────────────────────────────────

func TestCategory4_DiamondDetection(t *testing.T) {
	// Detect if-then-else merge diamond
	// Cypher: MATCH (c)-[:THEN]->(t), (c)-[:ELSE]->(e), (t)-[:SUCC]->(m)<-(e)
	g := &mockGraph{blocks: []mockBlock{
		{label: "cond", termKind: "br_if", targets: []string{"then", "else"}},
		{label: "then", termKind: "jump", targets: []string{"merge"}},
		{label: "else", termKind: "jump", targets: []string{"merge"}},
		{label: "merge", termKind: "ret"},
	}}

	src := `(grace diamond 0
		(match
			(block ?c (term-kind "br_if"))
			(block ?t)
			(block ?e)
			(block ?m)
			(edge ?c ?t "then")
			(edge ?c ?e "else")
			(edge ?t ?m "succ")
			(edge ?e ?m "succ"))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	if len(matches) != 1 {
		t.Fatalf("expected 1 diamond, got %d", len(matches))
	}
	m := matches[0]
	if m["c"] != "cond" || m["t"] != "then" || m["e"] != "else" || m["m"] != "merge" {
		t.Errorf("unexpected bindings: %v", m)
	}
}

// ── Category 5: Dominance ──────────────────────────────────────────────────

func TestCategory5_Dominance(t *testing.T) {
	// Check that entry dominates exit (simplified: layout order)
	// Cypher: MATCH (a)-[:DOMINATES]->(b)
	g := &mockGraph{blocks: []mockBlock{
		{label: "entry", termKind: "jump", targets: []string{"mid"}},
		{label: "mid", termKind: "jump", targets: []string{"exit"}},
		{label: "exit", termKind: "ret"},
	}}

	src := `(grace dom-check 0
		(match
			(block ?a)
			(block ?b))
		(where
			(dominates ?a ?b))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	// entry dom mid, entry dom exit, mid dom exit = 3
	if len(matches) != 3 {
		t.Fatalf("expected 3 dominance pairs, got %d", len(matches))
	}
}

// ── Category 6: Instruction Patterns ───────────────────────────────────────

func TestCategory6_InstructionPatterns(t *testing.T) {
	// Find blocks with all pure instructions
	// Cypher: MATCH (b:Block) WHERE all(i IN b.insts WHERE i.pure)
	g := &mockGraph{blocks: []mockBlock{
		{label: "pure_block", termKind: "ret", insts: []mockInst{
			{op: "add", pure: true},
			{op: "sub", pure: true},
		}},
		{label: "impure_block", termKind: "ret", insts: []mockInst{
			{op: "add", pure: true},
			{op: "store", pure: false},
		}},
	}}

	src := `(grace find-pure 0
		(match (block ?b))
		(where (all-pure ?b))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	if len(matches) != 1 {
		t.Fatalf("expected 1 pure block, got %d", len(matches))
	}
	if matches[0]["b"] != "pure_block" {
		t.Errorf("expected pure_block, got %s", matches[0]["b"])
	}
}

// ── Category 7: Aggregation (use-count, fanout) ────────────────────────────

func TestCategory7_Aggregation(t *testing.T) {
	// Find blocks with exactly 1 predecessor
	// Cypher: MATCH (b) WHERE count(predecessors) = 1
	g := &mockGraph{blocks: []mockBlock{
		{label: "entry", termKind: "br_if", targets: []string{"a", "b"}},
		{label: "a", termKind: "jump", targets: []string{"merge"}},
		{label: "b", termKind: "jump", targets: []string{"merge"}},
		{label: "merge", termKind: "ret"},
	}}

	src := `(grace single-pred 0
		(match (block ?b))
		(where (pred-count ?b == 1))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	// "a" has 1 pred (entry), "b" has 1 pred (entry)
	// "merge" has 2 preds, "entry" has 0 preds
	if len(matches) != 2 {
		t.Fatalf("expected 2 single-pred blocks, got %d", len(matches))
	}
}

// ── CondRetSink pattern (real compiler optimization) ────────────────────────

func TestCondRetSink_Pattern(t *testing.T) {
	// The actual optimization pattern from condret.go:
	// cur(br_if) → else(ret, pure insts, 1 pred, no params) → hoist else into cur
	g := &mockGraph{blocks: []mockBlock{
		{label: "cur", termKind: "br_if", targets: []string{"then", "else"}},
		{label: "then", termKind: "ret"},
		{label: "else", termKind: "ret", params: 0, insts: []mockInst{
			{op: "sub", pure: true},
		}},
	}}

	src := `(grace cond-ret-sink 15
		(match
			(block ?cur (term-kind "br_if"))
			(block ?else (term-kind "ret"))
			(edge ?cur ?else "else"))
		(where
			(no-params ?else)
			(all-pure ?else)
			(pred-count ?else == 1))
		(action
			(hoist-insts ?else ?cur)))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	if len(matches) != 1 {
		t.Fatalf("expected 1 cond-ret-sink match, got %d", len(matches))
	}
	if matches[0]["cur"] != "cur" || matches[0]["else"] != "else" {
		t.Errorf("unexpected bindings: %v", matches[0])
	}
}

// ── DSE pattern (dead store elimination) ────────────────────────────────────

func TestDSE_Pattern(t *testing.T) {
	// Find blocks with pure instructions (for dead instruction removal)
	g := &mockGraph{blocks: []mockBlock{
		{label: "entry", termKind: "ret", insts: []mockInst{
			{op: "const", pure: true, defReg: "r1"},
			{op: "add", pure: true, defReg: "r2", uses: []string{"r1"}},
		}},
	}}

	src := `(grace find-pure-blocks 0
		(match (block ?b))
		(where (all-pure ?b))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

// ── Custom predicates ──────────────────────────────────────────────────────

func TestCustomPredicate(t *testing.T) {
	g := &mockGraph{blocks: []mockBlock{
		{label: "a", termKind: "jump", targets: []string{"b"}, insts: []mockInst{
			{op: "special", pure: true},
		}},
		{label: "b", termKind: "ret"},
	}}

	src := `(grace custom-test 0
		(match (block ?b))
		(where (has-special-inst ?b))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	reg := NewPredicateRegistry()
	reg.RegisterPredicate("has-special-inst", func(graph rewrite.IRGraph, bindings BlockBindings, args []string) bool {
		label := bindings[args[0]]
		b := graph.Block(label)
		if b == nil {
			return false
		}
		for i := 0; i < b.InstCount(); i++ {
			if b.Inst(i).Op() == "special" {
				return true
			}
		}
		return false
	})

	matches := FindAllMatches(g, &rules[0], reg)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match with custom predicate, got %d", len(matches))
	}
}

// ── Parser test ─────────────────────────────────────────────────────────────

func TestParseRules(t *testing.T) {
	src := `
;; Dead store elimination
(grace dse 10
	(match
		(block ?b (term-kind "ret"))
		(block ?target))
	(where
		(pred-count ?b == 1)
		(no-params ?b)
		(all-pure ?b))
	(action
		(hoist-insts ?b ?target)
		(delete-block ?b)))
`
	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.Name != "dse" {
		t.Errorf("name: got %s, want dse", r.Name)
	}
	if r.Priority != 10 {
		t.Errorf("priority: got %d, want 10", r.Priority)
	}
	if len(r.Match) != 2 {
		t.Errorf("match clauses: got %d, want 2", len(r.Match))
	}
	if len(r.Where) != 3 {
		t.Errorf("where clauses: got %d, want 3", len(r.Where))
	}
	if len(r.Actions) != 2 {
		t.Errorf("actions: got %d, want 2", len(r.Actions))
	}
}

// ── Back-edge test ──────────────────────────────────────────────────────────

func TestBackEdge(t *testing.T) {
	g := &mockGraph{blocks: []mockBlock{
		{label: "head", termKind: "br_if", targets: []string{"body", "exit"}},
		{label: "body", termKind: "jump", targets: []string{"head"}},
		{label: "exit", termKind: "ret"},
	}}

	src := `(grace loop-detect 0
		(match
			(block ?head)
			(block ?body)
			(edge ?body ?head "back"))
		(action))`

	rules, err := ParseRules(src)
	if err != nil {
		t.Fatal(err)
	}

	matches := FindAllMatches(g, &rules[0], nil)
	if len(matches) != 1 {
		t.Fatalf("expected 1 back-edge, got %d", len(matches))
	}
	if matches[0]["head"] != "head" || matches[0]["body"] != "body" {
		t.Errorf("unexpected: %v", matches[0])
	}
}
