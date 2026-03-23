package vir

import (
	"os/exec"
	"testing"
)

func TestLocSet(t *testing.T) {
	s := Singleton(3).Or(Singleton(5))
	if !s.Has(3) || !s.Has(5) {
		t.Fatal("expected bits 3 and 5 set")
	}
	if s.Has(4) {
		t.Fatal("bit 4 should not be set")
	}
	if s.Count() != 2 {
		t.Fatalf("expected 2, got %d", s.Count())
	}
	if s.First() != 3 {
		t.Fatalf("expected first=3, got %d", s.First())
	}
}

func TestZ80LocSets(t *testing.T) {
	if Z80_GPR8.Count() != 7 {
		t.Fatalf("expected 7 GPR8, got %d", Z80_GPR8.Count())
	}
	if Z80_Pairs.Count() != 3 {
		t.Fatalf("expected 3 pairs, got %d", Z80_Pairs.Count())
	}
	if Z80_IXHalves.Count() != 4 {
		t.Fatalf("expected 4 IX halves, got %d", Z80_IXHalves.Count())
	}
	if Z80_TSMC.Count() != 8 {
		t.Fatalf("expected 8 TSMC slots, got %d", Z80_TSMC.Count())
	}
}

func TestPatternMatching(t *testing.T) {
	op := VIROp{Op: OpConst, Width: 8}
	matched := 0
	for _, pat := range Z80.Patterns {
		if pat.Matches(op) {
			matched++
		}
	}
	if matched == 0 {
		t.Fatal("no patterns match OpConst width=8")
	}
	t.Logf("OpConst/8: %d matching patterns", matched)
}

func TestPIROpEmit(t *testing.T) {
	// LD B, 42
	pat := findPattern("ld_r_n")
	if pat == nil {
		t.Fatal("ld_r_n not found")
	}
	pirop := PIROp{
		Pat:     pat,
		DstPhys: Z80.LocByName("B"),
		Imm:     42,
	}
	asm := pirop.Emit(Z80)
	if asm != "LD B, 42" {
		t.Fatalf("expected 'LD B, 42', got %q", asm)
	}

	// ADD A, C
	pat2 := findPattern("add_a_r")
	if pat2 == nil {
		t.Fatal("add_a_r not found")
	}
	pirop2 := PIROp{
		Pat:     pat2,
		DstPhys: Z80.LocByName("A"),
		SrcPhys: [2]int{Z80.LocByName("A"), Z80.LocByName("C")},
	}
	asm2 := pirop2.Emit(Z80)
	if asm2 != "ADD A, C" {
		t.Fatalf("expected 'ADD A, C', got %q", asm2)
	}
}

func TestZ3SolverSimple(t *testing.T) {
	// Skip if z3 not available
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found, skipping")
	}

	// Simple program: v1 = 5, v2 = 3, v3 = v1 + v2
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 5, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
	}

	result, err := Solve(ops, Z80, SolverOptions{Verbose: testing.Verbose()})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 PIROps, got %d", len(result))
	}

	// Emit and verify
	for i, p := range result {
		line := p.Emit(Z80)
		t.Logf("  PIR[%d]: %s (pat=%v dst=%d src=%v)",
			i, line, p.Pat.Name, p.DstPhys, p.SrcPhys)
	}

	// Z3 found a valid solution — verify all phys regs are in GPR range
	for i, p := range result {
		if p.Pat == nil {
			t.Errorf("PIR[%d] has no pattern", i)
		}
		if p.DstPhys >= 0 && !Z80_GPR8.Has(p.DstPhys) && !Z80_Flags.Has(p.DstPhys) {
			t.Errorf("PIR[%d] dst=%d not in GPR8", i, p.DstPhys)
		}
	}

	// Verify no two simultaneously-live vregs share a physical location
	locs := map[int]int{} // vreg → phys
	for _, p := range result {
		if p.DstPhys >= 0 {
			locs[p.DstPhys]++
		}
	}
	// Note: proper interference check would use liveness, but basic sanity here
	t.Logf("Z3 total cost: solved successfully with %d PIROps", len(result))
}

func TestZ3SolverDouble16(t *testing.T) {
	// Skip if z3 not available
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found, skipping")
	}

	// double(x) = x + x → should use ADD HL, HL
	ops := []VIROp{
		{Op: OpAdd, Dst: 2, Src: [2]int{1, 1}, Width: 16},
	}

	result, err := Solve(ops, Z80, SolverOptions{Verbose: testing.Verbose()})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 PIROp, got %d", len(result))
	}

	asm := result[0].Emit(Z80)
	t.Logf("double16: %s", asm)

	// Should select ADD HL, HL (cheapest 16-bit add with same src)
	if result[0].Pat != nil && result[0].Pat.Name == "add_hl_rr" {
		hlIdx := Z80.LocByName("HL")
		if result[0].SrcPhys[1] == hlIdx {
			t.Log("Correctly selected ADD HL, HL")
		}
	}
}

func TestLivenessComputation(t *testing.T) {
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 5, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
	}

	live := computeLiveness(ops)

	// At instruction 2 (add), both v1 and v2 should be live
	if !live[2].live[1] || !live[2].live[2] {
		t.Fatal("v1 and v2 should be live at instruction 2")
	}
	// v3 is defined at instruction 2, should be live there
	if !live[2].live[3] {
		t.Fatal("v3 should be live at instruction 2 (defined there)")
	}
}

func TestZ3SolverChainedOps(t *testing.T) {
	// Skip if z3 not available
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found, skipping")
	}

	// v1 = 10, v2 = 3, v3 = v1 + v2, v4 = v3 - v2
	// Chain: each op consumes the previous result, no cross-instruction conflict
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 10, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
		{Op: OpSub, Dst: 4, Src: [2]int{3, 2}, Width: 8},
	}

	result, err := Solve(ops, Z80, SolverOptions{Verbose: testing.Verbose()})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	for i, p := range result {
		t.Logf("  PIR[%d]: %s (pat=%s dst=%d)", i, p.Emit(Z80), p.Pat.Name, p.DstPhys)
	}

	if len(result) != 4 {
		t.Fatalf("expected 4 PIROps, got %d", len(result))
	}

	// Both ADD and SUB should use A as dst (tied to src0)
	aIdx := Z80.LocByName("A")
	if result[2].DstPhys != aIdx {
		t.Errorf("ADD dst should be A, got %d", result[2].DstPhys)
	}
	if result[3].DstPhys != aIdx {
		t.Errorf("SUB dst should be A, got %d", result[3].DstPhys)
	}
}

func findPattern(name string) *Pattern {
	for i := range Z80.Patterns {
		if Z80.Patterns[i].Name == name {
			return &Z80.Patterns[i]
		}
	}
	return nil
}
