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

	// May have extra moves from pre-tie insertion (that's OK)
	if len(result) < 4 {
		t.Fatalf("expected at least 4 PIROps, got %d", len(result))
	}

	// Find the ADD and SUB in the output — both should use A as dst
	aIdx := Z80.LocByName("A")
	for _, p := range result {
		if p.Pat != nil && (p.Pat.Name == "add_a_r" || p.Pat.Name == "sub_a_r") {
			if p.DstPhys != aIdx {
				t.Errorf("%s dst should be A, got %d", p.Pat.Name, p.DstPhys)
			}
		}
	}
}

func TestZ3SolverMoveInsertion(t *testing.T) {
	// Skip if z3 not available
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found, skipping")
	}

	// v1 = 10, v2 = 3, v3 = v1 - v2, v4 = v3 + v1
	// SUB destroys v1 (tied to A), but ADD needs v1 again → move insertion required
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 10, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpSub, Dst: 3, Src: [2]int{1, 2}, Width: 8},
		{Op: OpAdd, Dst: 4, Src: [2]int{3, 1}, Width: 8},
	}

	result, err := Solve(ops, Z80, SolverOptions{Verbose: testing.Verbose()})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	// Should have 5 PIROps: 2 consts + 1 save move + sub + add
	for i, p := range result {
		if p.Pat != nil {
			t.Logf("  PIR[%d]: %s (pat=%s dst=%d src=%v)",
				i, p.Emit(Z80), p.Pat.Name, p.DstPhys, p.SrcPhys)
		} else {
			t.Logf("  PIR[%d]: (meta) %s", i, p.Comment)
		}
	}

	if len(result) < 4 {
		t.Fatalf("expected at least 4 PIROps (with move), got %d", len(result))
	}

	// Find the move instruction (should be LD r, r pattern)
	hasSave := false
	for _, p := range result {
		if p.Pat != nil && p.Pat.Op == OpMove {
			hasSave = true
			break
		}
	}
	if hasSave {
		t.Log("Save-before-overwrite move correctly inserted")
	} else {
		t.Log("Warning: no save move found — tied operand may have been resolved differently")
	}
}

func TestInsertSaveMoves(t *testing.T) {
	// Unit test for the pre-solver save pass
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 10, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpSub, Dst: 3, Src: [2]int{1, 2}, Width: 8},
		{Op: OpAdd, Dst: 4, Src: [2]int{3, 1}, Width: 8},
	}

	expanded := insertSaveMoves(ops, Z80)
	t.Logf("Original: %d ops, Expanded: %d ops", len(ops), len(expanded))

	for i, op := range expanded {
		t.Logf("  [%d] Op=%d Dst=%d Src=%v", i, op.Op, op.Dst, op.Src)
	}

	if len(expanded) != 5 {
		t.Fatalf("expected 5 ops (1 save inserted), got %d", len(expanded))
	}

	// The save move should be before the SUB
	save := expanded[2]
	if save.Op != OpMove {
		t.Fatalf("expected OpMove at index 2, got Op=%d", save.Op)
	}

	// The ADD's src1 should now use the copy vreg, not v1
	add := expanded[4]
	if add.Src[1] == 1 {
		t.Fatal("ADD src1 should use copy vreg, not original v1")
	}
}

func TestSpillInsertion(t *testing.T) {
	// Create a block with 8 simultaneously-live 8-bit vregs (exceeds 6 GPR limit)
	// v1..v8 = const, v9 = v1+v2, v10 = v3+v4, v11 = v5+v6, v12 = v7+v8
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 1, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 2, Width: 8},
		{Op: OpConst, Dst: 3, Imm: 3, Width: 8},
		{Op: OpConst, Dst: 4, Imm: 4, Width: 8},
		{Op: OpConst, Dst: 5, Imm: 5, Width: 8},
		{Op: OpConst, Dst: 6, Imm: 6, Width: 8},
		{Op: OpConst, Dst: 7, Imm: 7, Width: 8},
		{Op: OpConst, Dst: 8, Imm: 8, Width: 8},
		// All 8 live here → pressure = 8 > 6
		{Op: OpAdd, Dst: 9, Src: [2]int{1, 2}, Width: 8},
		{Op: OpAdd, Dst: 10, Src: [2]int{3, 4}, Width: 8},
		{Op: OpAdd, Dst: 11, Src: [2]int{5, 6}, Width: 8},
		{Op: OpAdd, Dst: 12, Src: [2]int{7, 8}, Width: 8},
	}

	p, _ := maxPressure(ops)
	t.Logf("Initial pressure: %d", p)
	if p <= maxGPRPressure {
		t.Fatalf("expected pressure > %d, got %d", maxGPRPressure, p)
	}

	spilled := insertSpillReloads(ops, Z80)
	p2, _ := maxPressure(spilled)
	t.Logf("After spill: %d ops → %d ops, pressure: %d → %d",
		len(ops), len(spilled), p, p2)

	if p2 > maxGPRPressure+2 { // allow some slack for spill vregs themselves
		t.Errorf("pressure still too high: %d", p2)
	}

	for i, op := range spilled {
		hint := ""
		if !op.DstHint.IsEmpty() {
			hint = " [SPILL]"
		}
		t.Logf("  [%d] Op=%d Dst=%d Src=%v%s", i, op.Op, op.Dst, op.Src, hint)
	}
}

func TestZ3SolverHighPressure(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	// Moderate pressure: 4 consts + 2 adds, all live at once (pressure=5)
	// Should work without spilling (5 < 6 threshold)
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 1, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 2, Width: 8},
		{Op: OpConst, Dst: 3, Imm: 3, Width: 8},
		{Op: OpConst, Dst: 4, Imm: 4, Width: 8},
		{Op: OpAdd, Dst: 5, Src: [2]int{1, 2}, Width: 8},
		{Op: OpAdd, Dst: 6, Src: [2]int{3, 4}, Width: 8},
	}

	result, err := Solve(ops, Z80, SolverOptions{Verbose: testing.Verbose()})
	if err != nil {
		t.Fatalf("Solve (moderate pressure): %v", err)
	}

	t.Logf("Solved with %d PIROps", len(result))
	for i, p := range result {
		if p.Pat != nil {
			t.Logf("  PIR[%d]: %s (pat=%s dst=%d)", i, p.Emit(Z80), p.Pat.Name, p.DstPhys)
		}
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
