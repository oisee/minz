package mir2gpu_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2gpu"
	"github.com/minz/minzc/pkg/nanz"
)

func parseMIR2(t *testing.T, src string) *mir2.Module {
	t.Helper()
	hm, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	return m
}

func TestDetect_PureMapFunction(t *testing.T) {
	// A function that computes result from input — pure map candidate
	src := `fun double(x: u8) -> u8 { return x + x }`
	m := parseMIR2(t, src)

	candidates := mir2gpu.DetectGPUCandidates(m)
	// This is a leaf function (no loop), so it's a "whole function = map" candidate
	// The detection currently looks for loops, so this won't match directly
	// But when called from a loop, the loop+call is the candidate
	t.Logf("Candidates: %d", len(candidates))
	t.Logf("%s", mir2gpu.FormatCandidates(candidates))
}

func TestDetect_WhileLoop(t *testing.T) {
	// A while loop with bounded counter
	src := `
fun sum_to(n: u8) -> u8 {
	var result: u8 = 0
	var i: u8 = 0
	while i < n {
		result = result + i
		i = i + 1
	}
	return result
}
`
	m := parseMIR2(t, src)

	candidates := mir2gpu.DetectGPUCandidates(m)
	t.Logf("Candidates: %d", len(candidates))
	t.Logf("%s", mir2gpu.FormatCandidates(candidates))

	// Print MIR2 for debugging
	for _, f := range m.Funcs {
		t.Logf("=== %s ===", f.Name)
		for _, b := range f.Blocks {
			t.Logf("  %s: (params: %d)", b.Label, len(b.Params))
			for _, p := range b.Params {
				t.Logf("    param r%d : %s", p.Dst, p.Ty)
			}
			for _, inst := range b.Insts {
				t.Logf("    r%d = %s", inst.Dst, inst.Op)
			}
			if b.Term != nil {
				t.Logf("    term: %T → %v", b.Term, b.Term.Successors())
			}
		}
	}
}

func TestDetect_ForRange(t *testing.T) {
	// For-in-range is the classic bounded iteration
	src := `
fun fill_table() -> u8 {
	var total: u8 = 0
	for i in 0..10 {
		total = total + i
	}
	return total
}
`
	m := parseMIR2(t, src)

	candidates := mir2gpu.DetectGPUCandidates(m)
	t.Logf("Candidates: %d", len(candidates))
	t.Logf("%s", mir2gpu.FormatCandidates(candidates))

	for _, f := range m.Funcs {
		t.Logf("=== %s ===", f.Name)
		for _, b := range f.Blocks {
			t.Logf("  %s: (params: %d)", b.Label, len(b.Params))
			for _, p := range b.Params {
				t.Logf("    param r%d : %s", p.Dst, p.Ty)
			}
			for _, inst := range b.Insts {
				t.Logf("    r%d = %s imm=%d src=%v", inst.Dst, inst.Op, inst.Imm, inst.Src)
			}
			if b.Term != nil {
				t.Logf("    term: %T → %v", b.Term, b.Term.Successors())
			}
		}
	}
}

func TestDetect_MapCallInLoop(t *testing.T) {
	// Loop calling a function per iteration — classic map pattern
	src := `
fun double(x: u8) -> u8 { return x + x }

fun apply_all() -> u8 {
	var sum: u8 = 0
	for i in 0..10 {
		sum = sum + double(i)
	}
	return sum
}
`
	m := parseMIR2(t, src)

	candidates := mir2gpu.DetectGPUCandidates(m)
	t.Logf("Candidates: %d", len(candidates))
	t.Logf("%s", mir2gpu.FormatCandidates(candidates))

	for _, f := range m.Funcs {
		t.Logf("=== %s ===", f.Name)
		for _, b := range f.Blocks {
			t.Logf("  %s: (params: %d)", b.Label, len(b.Params))
			for _, p := range b.Params {
				t.Logf("    param r%d : %s", p.Dst, p.Ty)
			}
			for _, inst := range b.Insts {
				t.Logf("    r%d = %s imm=%d src=%v sym=%s args=%v", inst.Dst, inst.Op, inst.Imm, inst.Src, inst.Sym, inst.Args)
			}
			if b.Term != nil {
				t.Logf("    term: %T → %v", b.Term, b.Term.Successors())
			}
		}
	}
}

func TestDetect_PureArithmeticLoop(t *testing.T) {
	// Loop that writes computed values — pure map (no accumulator)
	src := `
fun compute(n: u8) -> u8 {
	var i: u8 = 0
	while i < n {
		i = i + 1
	}
	return i
}
`
	m := parseMIR2(t, src)
	candidates := mir2gpu.DetectGPUCandidates(m)
	t.Logf("Candidates: %d", len(candidates))
	t.Logf("%s", mir2gpu.FormatCandidates(candidates))
}

func TestDetect_FormatOutput(t *testing.T) {
	// Verify FormatCandidates with no candidates
	s := mir2gpu.FormatCandidates(nil)
	if s != "No GPU-parallelizable loops detected." {
		t.Errorf("unexpected: %s", s)
	}
}
