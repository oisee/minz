package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// ── EliminateDeadBlocks tests ────────────────────────────────────────────────

func TestEliminateDeadBlocks_RemovesUnreachable(t *testing.T) {
	// Build a function where one block is never jumped to:
	//
	//   entry: TermRet [42]
	//   dead:  TermRet [0]   ← nothing jumps here
	//
	// After EliminateDeadBlocks, only "entry" should remain.
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("f")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	entry := b.SwitchToNewBlock("entry")
	v := b.Const(42, mir2.TyU8, mir2.ClassAcc)
	b.Ret(v)

	// dead block — created but nothing jumps to it
	b.SwitchToNewBlock("dead")
	z := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	b.Ret(z)

	if len(f.Blocks) != 2 {
		t.Fatalf("expected 2 blocks before DCE, got %d", len(f.Blocks))
	}

	mir2.EliminateDeadBlocks(f)

	if len(f.Blocks) != 1 {
		t.Errorf("expected 1 block after DCE, got %d: %v", len(f.Blocks), blockLabels(f))
	}
	if f.Blocks[0].Label != entry.Label {
		t.Errorf("surviving block should be %q, got %q", entry.Label, f.Blocks[0].Label)
	}
}

func TestEliminateDeadBlocks_AllReachable(t *testing.T) {
	// Fibonacci has no dead blocks — count must stay the same.
	m := &mir2.Module{Name: "fib"}
	f := buildFib(m)
	before := len(f.Blocks)

	mir2.EliminateDeadBlocks(f)

	if len(f.Blocks) != before {
		t.Errorf("fib: block count changed %d→%d (no dead blocks expected)",
			before, len(f.Blocks))
	}
}

func TestEliminateDeadBlocks_ChainOfDead(t *testing.T) {
	// CFG:  entry → join   (both live)
	//       dead1 → dead2  (both unreachable)
	//
	// Only entry and join should survive.
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("g")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	b.Jmp("join") // seals entry

	b.SwitchToNewBlock("join")
	v := b.Const(7, mir2.TyU8, mir2.ClassAcc)
	b.Ret(v)

	b.SwitchToNewBlock("dead1")
	b.Jmp("dead2")

	b.SwitchToNewBlock("dead2")
	w := b.Const(99, mir2.TyU8, mir2.ClassAcc)
	b.Ret(w)

	if len(f.Blocks) != 4 {
		t.Fatalf("expected 4 blocks before DCE, got %d", len(f.Blocks))
	}

	mir2.EliminateDeadBlocks(f)

	if len(f.Blocks) != 2 {
		t.Errorf("expected 2 blocks after DCE, got %d: %v", len(f.Blocks), blockLabels(f))
	}
	for _, blk := range f.Blocks {
		if blk.Label == "dead1" || blk.Label == "dead2" {
			t.Errorf("dead block %q survived DCE", blk.Label)
		}
	}
}

func TestEliminateDeadBlocks_PreservesSemantics(t *testing.T) {
	// After DCE + ReorderBlocks, Verify must pass and the VM must still give
	// correct results for fibonacci.
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
	}

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify after DCE+reorder: %v", err)
	}

	vm := mir2.NewVM(m)
	cases := [][2]int64{{1, 1}, {5, 5}, {10, 55}}
	for _, tc := range cases {
		got, err := vm.Call("fibonacci", []mir2.Value{{I: tc[0]}})
		if err != nil {
			t.Errorf("fib(%d): %v", tc[0], err)
			continue
		}
		if len(got) != 1 || got[0].I != tc[1] {
			t.Errorf("fib(%d) = %v, want %d", tc[0], got, tc[1])
		}
	}
}

func blockLabels(f *mir2.Func) []string {
	labels := make([]string, len(f.Blocks))
	for i, b := range f.Blocks {
		labels[i] = b.Label
	}
	return labels
}

// ── ReorderBlocks tests ───────────────────────────────────────────────────────

func TestLayoutFibonacciOrder(t *testing.T) {
	m := &mir2.Module{Name: "fib"}
	f := buildFib(m)

	// Original order (definition order):
	//   entry, base, loop_init, loop_head, loop_body, exit
	originalOrder := make([]string, len(f.Blocks))
	for i, b := range f.Blocks {
		originalOrder[i] = b.Label
	}

	mir2.ReorderBlocks(f)

	newOrder := make([]string, len(f.Blocks))
	for i, b := range f.Blocks {
		newOrder[i] = b.Label
	}

	t.Logf("original:  %v", originalOrder)
	t.Logf("reordered: %v", newOrder)

	// Entry must stay first.
	if newOrder[0] != "entry" {
		t.Errorf("entry must be first; got %v", newOrder)
	}

	// Hot blocks (loop_init, loop_head, loop_body) must all come before cold
	// blocks (base, exit).
	hotLabels := []string{"loop_init", "loop_head", "loop_body"}
	coldLabels := []string{"base", "exit"}

	lastHot, firstCold := -1, len(newOrder)
	for i, lbl := range newOrder {
		for _, h := range hotLabels {
			if lbl == h && i > lastHot {
				lastHot = i
			}
		}
		for _, c := range coldLabels {
			if lbl == c && i < firstCold {
				firstCold = i
			}
		}
	}

	if lastHot > firstCold {
		t.Errorf("hot blocks should come before cold blocks; hot ends at %d, cold starts at %d; order=%v",
			lastHot, firstCold, newOrder)
	}
}

func TestLayoutFibonacciHotPathFallThrough(t *testing.T) {
	// After reorder + codegen, the hot path entry→loop_init→loop_head→loop_body
	// should have no JP on the fall-through edges.
	m := &mir2.Module{Name: "fib"}
	f := buildFib(m)

	mir2.ReorderBlocks(f)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)

	t.Log("\n" + asm)

	// The hot path should not have JP loop_init or JP loop_head as
	// unconditional jumps (they should be fall-throughs now).
	// Check: no "JP .fibonacci_loop_init" in output (it's a fall-through).
	if strings.Contains(asm, "JP .fibonacci_loop_init") {
		t.Error("expected fall-through to loop_init (no JP); got unconditional JP")
	}
	// loop_init → loop_head should be a fall-through: verify there is no
	// unconditional JP between the loop_init label and the loop_head label.
	// (The back-edge JP .fibonacci_loop_head from loop_body is unavoidable
	// and intentionally not checked here.)
	{
		initIdx := strings.Index(asm, ".fibonacci_loop_init:")
		headIdx := strings.Index(asm, ".fibonacci_loop_head:")
		if initIdx >= 0 && headIdx > initIdx {
			between := asm[initIdx:headIdx]
			if strings.Contains(between, "JP .fibonacci_loop_head") {
				t.Error("loop_init block should fall through to loop_head (no JP); got unconditional JP")
			}
		}
	}
}

func TestLayoutPreservesSemantics(t *testing.T) {
	// After reordering, Verify must still pass (block structure unchanged).
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify after ReorderBlocks: %v", err)
	}
}

func TestLayoutVMStillWorks(t *testing.T) {
	// The VM must produce correct fibonacci results after block reordering.
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}

	vm := mir2.NewVM(m)
	cases := [][2]int64{{0, 0}, {1, 1}, {5, 5}, {10, 55}}
	for _, tc := range cases {
		got, err := vm.Call("fibonacci", []mir2.Value{{I: tc[0]}})
		if err != nil {
			t.Errorf("fib(%d): %v", tc[0], err)
			continue
		}
		if len(got) != 1 || got[0].I != tc[1] {
			t.Errorf("fib(%d) = %v, want %d", tc[0], got, tc[1])
		}
	}
}
