package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

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
