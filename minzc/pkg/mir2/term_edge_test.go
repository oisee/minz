package mir2

import (
	"testing"
)

// ── Edge enumeration contract tests ──────────────────────────────────────────
//
// These tests pin the Term → (target, args, paramOffset) mapping that every
// pass in the compiler relies on.  If a refactoring breaks this mapping,
// these tests catch it immediately — no need to wait for E2E failures.

// Edge is one outgoing CFG edge from a terminator.
type edge struct {
	Target      string
	Args        []Reg
	ParamOffset int // 0 for most edges; 1 for DJNZ body (implicit counter)
}

// collectEdges extracts edges from a terminator via ForEachEdge.
func collectEdges(t Term) []edge {
	var edges []edge
	t.ForEachEdge(func(target string, args []Reg, paramOffset int) {
		edges = append(edges, edge{target, args, paramOffset})
	})
	return edges
}

// ── Successors ───────────────────────────────────────────────────────────────

func TestSuccessors_TermJmp(t *testing.T) {
	term := &TermJmp{Target: "loop", Args: []Reg{1, 2}}
	got := term.Successors()
	if len(got) != 1 || got[0] != "loop" {
		t.Fatalf("want [loop], got %v", got)
	}
}

func TestSuccessors_TermBrIf(t *testing.T) {
	term := &TermBrIf{Cond: 1, Then: "yes", Else: "no"}
	got := term.Successors()
	if len(got) != 2 || got[0] != "yes" || got[1] != "no" {
		t.Fatalf("want [yes no], got %v", got)
	}
}

func TestSuccessors_TermBrIf2(t *testing.T) {
	term := &TermBrIf2{Lhs: 1, Rhs: 2, Eq: "eq", Lt: "lt", Gt: "gt"}
	got := term.Successors()
	if len(got) != 3 || got[0] != "eq" || got[1] != "lt" || got[2] != "gt" {
		t.Fatalf("want [eq lt gt], got %v", got)
	}
}

func TestSuccessors_TermDJNZ(t *testing.T) {
	term := &TermDJNZ{Counter: 1, Body: "body", Exit: "exit"}
	got := term.Successors()
	if len(got) != 2 || got[0] != "body" || got[1] != "exit" {
		t.Fatalf("want [body exit], got %v", got)
	}
}

func TestSuccessors_TermCondRet(t *testing.T) {
	term := &TermCondRet{Cond: 1, Vals: []Reg{2}, Then: "cont"}
	got := term.Successors()
	if len(got) != 1 || got[0] != "cont" {
		t.Fatalf("want [cont], got %v", got)
	}
}

func TestSuccessors_TermRet(t *testing.T) {
	term := &TermRet{Vals: []Reg{1}}
	got := term.Successors()
	if got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestSuccessors_TermUnreachable(t *testing.T) {
	term := &TermUnreachable{}
	got := term.Successors()
	if got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

// ── termUses ─────────────────────────────────────────────────────────────────

func TestTermUses_TermJmp(t *testing.T) {
	term := &TermJmp{Target: "x", Args: []Reg{10, 20}}
	got := term.termUses()
	if len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("want [10 20], got %v", got)
	}
}

func TestTermUses_TermBrIf_IncludesCond(t *testing.T) {
	term := &TermBrIf{Cond: 5, Then: "t", ThenArgs: []Reg{10}, Else: "e", ElseArgs: []Reg{20}}
	got := term.termUses()
	// Must include Cond + ThenArgs + ElseArgs
	if len(got) != 3 || got[0] != 5 || got[1] != 10 || got[2] != 20 {
		t.Fatalf("want [5 10 20], got %v", got)
	}
}

func TestTermUses_TermDJNZ_IncludesCounter(t *testing.T) {
	term := &TermDJNZ{Counter: 5, Body: "b", BodyArgs: []Reg{10}, Exit: "e", ExitArgs: []Reg{20}}
	got := term.termUses()
	// Counter + BodyArgs + ExitArgs
	if len(got) != 3 || got[0] != 5 || got[1] != 10 || got[2] != 20 {
		t.Fatalf("want [5 10 20], got %v", got)
	}
}

// ── DJNZ offset contract ────────────────────────────────────────────────────
//
// THE critical invariant: BodyArgs[i] maps to body.Params[i+1], NOT Params[i].
// Params[0] is the implicit counter (ClassCounter = B register).
// This offset has been the source of THREE separate bugs (Report #067 Bugs A/B/C).

func TestDJNZ_BodyParamOffset(t *testing.T) {
	// Build a minimal function with DJNZ:
	//   block @entry:
	//     djnz %counter, @body(%acc), @exit(%result)
	//   block @body(%b_counter: u8 [counter], %b_acc: u8 [acc]):
	//     ...
	//   block @exit(%e_result: u8 [acc]):
	//     ret %e_result
	counter := Reg(1)
	acc := Reg(2)

	bodyBlock := &Block{
		Label: "body",
		Params: []BlockParam{
			{Dst: 10, Ty: TyU8, Class: ClassCounter}, // Params[0] = implicit counter
			{Dst: 11, Ty: TyU8, Class: ClassAcc},     // Params[1] = accumulator
		},
	}

	term := &TermDJNZ{
		Counter:  counter,
		Body:     "body",
		BodyArgs: []Reg{acc}, // BodyArgs[0] = acc → body.Params[1], NOT Params[0]
		Exit:     "exit",
		ExitArgs: []Reg{acc},
	}

	edges := collectEdges(term)

	// Body edge: offset must be 1
	bodyEdge := edges[0]
	if bodyEdge.Target != "body" {
		t.Fatalf("expected body edge first, got %s", bodyEdge.Target)
	}
	if bodyEdge.ParamOffset != 1 {
		t.Fatalf("DJNZ body ParamOffset: want 1, got %d — THIS IS THE BUG THAT CAUSED BUGS A/B/C", bodyEdge.ParamOffset)
	}

	// Verify: BodyArgs[0] should connect to Params[1], not Params[0]
	argIdx := 0
	paramIdx := argIdx + bodyEdge.ParamOffset
	if paramIdx >= len(bodyBlock.Params) {
		t.Fatalf("paramIdx %d out of range for body block with %d params", paramIdx, len(bodyBlock.Params))
	}
	if bodyBlock.Params[paramIdx].Class != ClassAcc {
		t.Fatalf("BodyArgs[0] should map to acc param (ClassAcc), got %v", bodyBlock.Params[paramIdx].Class)
	}
	// Params[0] is the counter — BodyArgs should NOT touch it
	if bodyBlock.Params[0].Class != ClassCounter {
		t.Fatalf("Params[0] should be ClassCounter, got %v", bodyBlock.Params[0].Class)
	}

	// Exit edge: offset must be 0 (normal)
	exitEdge := edges[1]
	if exitEdge.Target != "exit" {
		t.Fatalf("expected exit edge second, got %s", exitEdge.Target)
	}
	if exitEdge.ParamOffset != 0 {
		t.Fatalf("DJNZ exit ParamOffset: want 0, got %d", exitEdge.ParamOffset)
	}
}

// ── Edge enumeration exhaustiveness ──────────────────────────────────────────

func TestCollectEdges_AllTermTypes(t *testing.T) {
	tests := []struct {
		name      string
		term      Term
		wantCount int
	}{
		{"TermJmp", &TermJmp{Target: "a", Args: []Reg{1}}, 1},
		{"TermBrIf", &TermBrIf{Cond: 1, Then: "a", Else: "b"}, 2},
		{"TermBrIf2", &TermBrIf2{Lhs: 1, Rhs: 2, Eq: "a", Lt: "b", Gt: "c"}, 3},
		{"TermDJNZ", &TermDJNZ{Counter: 1, Body: "a", Exit: "b"}, 2},
		{"TermCondRet", &TermCondRet{Cond: 1, Then: "a"}, 1},
		{"TermRet", &TermRet{Vals: []Reg{1}}, 0},
		{"TermUnreachable", &TermUnreachable{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges := collectEdges(tt.term)
			if len(edges) != tt.wantCount {
				t.Fatalf("want %d edges, got %d: %+v", tt.wantCount, len(edges), edges)
			}
			// Edge targets must match Successors
			succs := tt.term.Successors()
			edgeTargets := make([]string, len(edges))
			for i, e := range edges {
				edgeTargets[i] = e.Target
			}
			// For TermCondRet, Successors has 1 entry, edges also has 1.
			// For others, edges match successors.
			if len(succs) != len(edges) {
				// TermCondRet's Vals don't create an edge, only Then does
				if _, ok := tt.term.(*TermCondRet); !ok {
					t.Fatalf("edges/successors mismatch: edges=%v, succs=%v", edgeTargets, succs)
				}
			}
		})
	}
}

// ── Edge offset invariants ───────────────────────────────────────────────────

func TestEdgeOffsets_OnlyDJNZBodyHasNonZero(t *testing.T) {
	terms := []Term{
		&TermJmp{Target: "a", Args: []Reg{1}},
		&TermBrIf{Cond: 1, Then: "a", ThenArgs: []Reg{2}, Else: "b", ElseArgs: []Reg{3}},
		&TermBrIf2{Lhs: 1, Rhs: 2, Eq: "a", EqArgs: []Reg{3}, Lt: "b", LtArgs: []Reg{4}, Gt: "c", GtArgs: []Reg{5}},
		&TermDJNZ{Counter: 1, Body: "a", BodyArgs: []Reg{2}, Exit: "b", ExitArgs: []Reg{3}},
		&TermCondRet{Cond: 1, Vals: []Reg{2}, Then: "a", ThenArgs: []Reg{3}},
	}

	for _, term := range terms {
		edges := collectEdges(term)
		for _, e := range edges {
			if e.ParamOffset != 0 {
				// Only DJNZ body edge should have non-zero offset
				if _, ok := term.(*TermDJNZ); !ok {
					t.Fatalf("non-DJNZ edge has ParamOffset=%d: %+v from %T", e.ParamOffset, e, term)
				}
				if e.Target != term.(*TermDJNZ).Body {
					t.Fatalf("non-body DJNZ edge has ParamOffset=%d: %+v", e.ParamOffset, e)
				}
			}
		}
	}
}
