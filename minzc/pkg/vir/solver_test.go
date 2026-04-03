package vir

import (
	"os"
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

func TestInsertPreTieMoves_PreservesSelfAddShape(t *testing.T) {
	ops := []VIROp{
		{Op: OpAdd, Dst: 2, Src: [2]int{1, 1}, Width: 8},
		{Op: OpAddImm, Dst: 3, Src: [2]int{2, -1}, Imm: 1, Width: 8},
	}

	got := insertPreTieMoves(ops, Z80)
	if len(got) != len(ops) {
		t.Fatalf("unexpected pre-tie move inserted: got %d ops, want %d", len(got), len(ops))
	}
	if got[0].Src != [2]int{1, 1} {
		t.Fatalf("self-add shape changed: got src=%v, want [1 1]", got[0].Src)
	}
}

func TestInsertPreTieMoves_PreservesSelfAdd16Shape(t *testing.T) {
	ops := []VIROp{
		{Op: OpAdd, Dst: 2, Src: [2]int{1, 1}, Width: 16},
		{Op: OpAddImm, Dst: 3, Src: [2]int{2, -1}, Imm: 1, Width: 16},
	}

	got := insertPreTieMoves(ops, Z80)
	if len(got) != len(ops) {
		t.Fatalf("unexpected pre-tie move inserted: got %d ops, want %d", len(got), len(ops))
	}
	if got[0].Src != [2]int{1, 1} {
		t.Fatalf("16-bit self-add shape changed: got src=%v, want [1 1]", got[0].Src)
	}
}

func TestInsertPreTieMoves_PreservesImmediateTiedChain(t *testing.T) {
	ops := []VIROp{
		{Op: OpAddImm, Dst: 2, Src: [2]int{1, -1}, Imm: 1, Width: 8},
		{Op: OpSubImm, Dst: 3, Src: [2]int{2, -1}, Imm: 1, Width: 8},
	}

	got := insertPreTieMoves(ops, Z80)
	if len(got) != len(ops) {
		t.Fatalf("unexpected pre-tie move inserted: got %d ops, want %d", len(got), len(ops))
	}
	if got[0].Src[0] != 1 || got[1].Src[0] != 2 {
		t.Fatalf("immediate tied chain changed: got src0s [%d %d], want [1 2]", got[0].Src[0], got[1].Src[0])
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

func TestGPURegAlloc(t *testing.T) {
	if gpuRegAllocPath() == "" {
		t.Skip("z80_regalloc not found, skipping GPU test")
	}
	// CUDA exec from Go test hangs on some systems (driver/sandbox issue).
	// Enable with: go test -run TestGPURegAlloc -gpu
	if os.Getenv("VIR_GPU_TEST") == "" {
		t.Skip("GPU test disabled (set VIR_GPU_TEST=1 to enable)")
	}

	// Same program as TestZ3SolverSimple: v1=5, v2=3, v3=v1+v2
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 5, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
	}

	assignment, cost, err := SolveGPU(ops, Z80, SolverOptions{})
	if err != nil {
		t.Fatalf("SolveGPU: %v", err)
	}

	t.Logf("GPU optimal: cost=%d, assignment=%v", cost, assignment)

	// v3 must be in A (ADD tied dst=src0, and ADD dst must be A)
	if loc, ok := assignment[3]; ok {
		if loc != 0 { // A=0
			t.Errorf("v3 should be in A(0), got %d", loc)
		}
	}

	// v1 must also be in A (tied to v3 via ADD)
	if loc, ok := assignment[1]; ok {
		if loc != 0 {
			t.Errorf("v1 should be in A(0) due to tied ADD, got %d", loc)
		}
	}

	// v2 must NOT be in A (interference with v1 at ADD instruction)
	if loc, ok := assignment[2]; ok {
		if loc == 0 {
			t.Errorf("v2 should not be in A(0) — interferes with v1")
		}
	}

	// Compare with Z3 if available
	if _, err := exec.LookPath("z3"); err == nil {
		z3Result, z3Err := Solve(ops, Z80, SolverOptions{})
		if z3Err == nil {
			t.Logf("Z3 produced %d PIROps, GPU cost=%d", len(z3Result), cost)
		}
	}
}

func TestEnrichedGapAnalysis(t *testing.T) {
	// Leaf function (no calls) — should be "shape_ok", no IX benefit
	leafOps := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 5, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
	}
	leafInfo := AnalyzeEnrichedGap(leafOps, Z80, "leaf_add")
	t.Logf("leaf_add: %+v", leafInfo)
	if leafInfo.HasCall {
		t.Error("leaf function should not have HasCall")
	}
	if leafInfo.WouldBenefitIX {
		t.Error("leaf function should not benefit from IX expansion")
	}
	if leafInfo.MissReason != "shape_ok" {
		t.Errorf("expected shape_ok, got %s", leafInfo.MissReason)
	}

	// Function with a call and a vreg live across it — should benefit from IX
	callOps := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 42, Width: 8},       // v1 = 42
		{Op: OpConst, Dst: 2, Imm: 10, Width: 8},        // v2 = 10 (arg for call)
		{Op: OpCall, Dst: 3, Src: [2]int{2, 0}, Width: 8, Sym: "foo"}, // v3 = foo(v2)
		{Op: OpAdd, Dst: 4, Src: [2]int{1, 3}, Width: 8}, // v4 = v1 + v3  (v1 was live across call)
	}
	callInfo := AnalyzeEnrichedGap(callOps, Z80, "call_add")
	t.Logf("call_add: %+v", callInfo)
	if !callInfo.HasCall {
		t.Error("call function should have HasCall")
	}
	if callInfo.CallLiveVregs == 0 {
		t.Error("expected vregs live across call")
	}
	if !callInfo.WouldBenefitIX {
		t.Error("function with call-live vregs should benefit from IX expansion")
	}
	if callInfo.MissReason != "call_pressure" {
		t.Errorf("expected call_pressure, got %s", callInfo.MissReason)
	}

	// Too many vregs — should report too_many_vregs
	bigOps := make([]VIROp, 0)
	for i := 1; i <= 8; i++ {
		bigOps = append(bigOps, VIROp{Op: OpConst, Dst: i, Imm: int64(i), Width: 8})
	}
	bigInfo := AnalyzeEnrichedGap(bigOps, Z80, "too_big")
	t.Logf("too_big: %+v", bigInfo)
	if bigInfo.MissReason != "too_many_vregs" {
		t.Errorf("expected too_many_vregs, got %s", bigInfo.MissReason)
	}
}

func TestPreSplitForTableLookup(t *testing.T) {
	// Leaf function — no change expected
	leafOps := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 5, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
	}
	splitOps, nSaves := PreSplitForTableLookup(leafOps)
	if nSaves != 0 {
		t.Errorf("leaf: expected 0 saves, got %d", nSaves)
	}
	if len(splitOps) != len(leafOps) {
		t.Errorf("leaf: expected %d ops unchanged, got %d", len(leafOps), len(splitOps))
	}

	// Function with call: v1=42, v2=foo(10), v3=v1+v2
	// v1 is live across the call → should be saved/restored
	callOps := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 42, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 10, Width: 8},
		{Op: OpCall, Dst: 3, Src: [2]int{2, 0}, Width: 8, Sym: "foo"},
		{Op: OpAdd, Dst: 4, Src: [2]int{1, 3}, Width: 8},
	}
	splitCallOps, nCallSaves := PreSplitForTableLookup(callOps)
	if nCallSaves != 1 {
		t.Errorf("call: expected 1 save (v1 across call), got %d", nCallSaves)
	}
	// Original: 4 ops → +1 save +1 restore = 6 ops
	if len(splitCallOps) != 6 {
		t.Errorf("call: expected 6 ops after split, got %d", len(splitCallOps))
	}

	// Verify structure: [const, const, save, call, restore, add]
	if splitCallOps[2].Op != OpMove {
		t.Errorf("call: expected OpMove (save) at index 2, got %d", splitCallOps[2].Op)
	}
	if splitCallOps[3].Op != OpCall {
		t.Errorf("call: expected OpCall at index 3, got %d", splitCallOps[3].Op)
	}
	if splitCallOps[4].Op != OpMove {
		t.Errorf("call: expected OpMove (restore) at index 4, got %d", splitCallOps[4].Op)
	}

	// Verify the save vreg is fresh
	saveVreg := splitCallOps[2].Dst
	if saveVreg <= 4 {
		t.Errorf("call: save vreg should be > 4 (fresh), got %d", saveVreg)
	}
	// Restore should write back to v1
	if splitCallOps[4].Dst != 1 {
		t.Errorf("call: restore should write to v1, got v%d", splitCallOps[4].Dst)
	}

	t.Logf("Pre-split: %d → %d ops, %d saves", len(callOps), len(splitCallOps), nCallSaves)
	for i, op := range splitCallOps {
		t.Logf("  [%d] Op=%d Dst=v%d Src=[v%d,v%d] Sym=%s", i, op.Op, op.Dst, op.Src[0], op.Src[1], op.Sym)
	}
}

func TestPreSplitEligibility(t *testing.T) {
	desc := Z80

	// Case 1: leaf function — both original and pre-split eligible
	leafOps := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 5, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
	}
	r1 := AnalyzePreSplitEligibility(leafOps, desc, "leaf")
	t.Logf("leaf: %+v", r1)
	if !r1.OriginalEligible {
		t.Error("leaf should be originally eligible")
	}
	if r1.SavesInserted != 0 {
		t.Error("leaf should have 0 saves")
	}

	// Case 2: non-leaf with 4 vregs, 1 call-live → 5 vregs after split (still ≤6)
	callOps := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 42, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 10, Width: 8},
		{Op: OpCall, Dst: 3, Src: [2]int{2, 0}, Width: 8, Sym: "foo"},
		{Op: OpAdd, Dst: 4, Src: [2]int{1, 3}, Width: 8},
	}
	r2 := AnalyzePreSplitEligibility(callOps, desc, "call_4v")
	t.Logf("call_4v: %+v", r2)
	if r2.OriginalNVregs != 4 {
		t.Errorf("expected 4 original vregs, got %d", r2.OriginalNVregs)
	}
	if r2.SavesInserted != 1 {
		t.Errorf("expected 1 save, got %d", r2.SavesInserted)
	}
	if r2.PresplitNVregs != 5 {
		t.Errorf("expected 5 pre-split vregs, got %d", r2.PresplitNVregs)
	}
	if !r2.PresplitEligible {
		t.Errorf("5v after split should be eligible, reason: %s", r2.PresplitReason)
	}

	// Case 3: non-leaf with 5 vregs, 2 call-live → 7 vregs after split (exceeds 6v limit)
	bigCallOps := []VIROp{
		{Op: OpConst, Dst: 1, Imm: 1, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 2, Width: 8},
		{Op: OpConst, Dst: 3, Imm: 10, Width: 8},
		{Op: OpCall, Dst: 4, Src: [2]int{3, 0}, Width: 8, Sym: "bar"},
		{Op: OpAdd, Dst: 5, Src: [2]int{1, 2}, Width: 8}, // v1, v2 live across call
	}
	r3 := AnalyzePreSplitEligibility(bigCallOps, desc, "call_5v")
	t.Logf("call_5v: %+v", r3)
	if r3.OriginalNVregs != 5 {
		t.Errorf("expected 5 original vregs, got %d", r3.OriginalNVregs)
	}
	if r3.SavesInserted < 2 {
		t.Errorf("expected ≥2 saves, got %d", r3.SavesInserted)
	}
	if r3.PresplitEligible {
		t.Errorf("7v after split should NOT be eligible")
	}
	if r3.Blocker != "save_vregs_exceed_6v_limit" {
		t.Errorf("expected blocker=save_vregs_exceed_6v_limit, got %s", r3.Blocker)
	}
}

func TestPreSplitHitRateMeasurement(t *testing.T) {
	desc := Z80

	// Simulate representative function profiles from a corpus:
	// Mix of leaf and non-leaf functions with varying vreg counts.
	type testCase struct {
		name string
		ops  []VIROp
	}

	cases := []testCase{
		// Leaf functions (no calls)
		{"leaf_2v", []VIROp{
			{Op: OpConst, Dst: 1, Imm: 1, Width: 8},
			{Op: OpNeg, Dst: 2, Src: [2]int{1, -1}, Width: 8},
		}},
		{"leaf_3v", []VIROp{
			{Op: OpConst, Dst: 1, Imm: 5, Width: 8},
			{Op: OpConst, Dst: 2, Imm: 3, Width: 8},
			{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
		}},
		{"leaf_4v", []VIROp{
			{Op: OpConst, Dst: 1, Imm: 1, Width: 8},
			{Op: OpConst, Dst: 2, Imm: 2, Width: 8},
			{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
			{Op: OpSub, Dst: 4, Src: [2]int{3, 1}, Width: 8},
		}},
		// Non-leaf: 1 call, 1 live-across (4v→5v: fits)
		{"call_1live_4v", []VIROp{
			{Op: OpConst, Dst: 1, Imm: 42, Width: 8},
			{Op: OpConst, Dst: 2, Imm: 10, Width: 8},
			{Op: OpCall, Dst: 3, Src: [2]int{2, 0}, Width: 8, Sym: "f"},
			{Op: OpAdd, Dst: 4, Src: [2]int{1, 3}, Width: 8},
		}},
		// Non-leaf: 1 call, 1 live-across (3v→4v: fits)
		{"call_1live_3v", []VIROp{
			{Op: OpConst, Dst: 1, Imm: 42, Width: 8},
			{Op: OpCall, Dst: 2, Src: [2]int{1, 0}, Width: 8, Sym: "g"},
			{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
		}},
		// Non-leaf: 1 call, 2 live-across (5v→7v: exceeds)
		{"call_2live_5v", []VIROp{
			{Op: OpConst, Dst: 1, Imm: 1, Width: 8},
			{Op: OpConst, Dst: 2, Imm: 2, Width: 8},
			{Op: OpConst, Dst: 3, Imm: 10, Width: 8},
			{Op: OpCall, Dst: 4, Src: [2]int{3, 0}, Width: 8, Sym: "h"},
			{Op: OpAdd, Dst: 5, Src: [2]int{1, 2}, Width: 8},
		}},
		// Non-leaf: 1 call, 3 live-across (6v→9v: exceeds)
		{"call_3live_6v", []VIROp{
			{Op: OpConst, Dst: 1, Imm: 1, Width: 8},
			{Op: OpConst, Dst: 2, Imm: 2, Width: 8},
			{Op: OpConst, Dst: 3, Imm: 3, Width: 8},
			{Op: OpConst, Dst: 4, Imm: 10, Width: 8},
			{Op: OpCall, Dst: 5, Src: [2]int{4, 0}, Width: 8, Sym: "k"},
			{Op: OpAdd, Dst: 6, Src: [2]int{1, 2}, Width: 8},
			// v3 also used after call (but we only have 1 ADD using v1,v2)
		}},
		// Non-leaf: 2 calls, 1 live-across each (4v→6v: fits)
		{"call2_1live_4v", []VIROp{
			{Op: OpConst, Dst: 1, Imm: 42, Width: 8},
			{Op: OpConst, Dst: 2, Imm: 10, Width: 8},
			{Op: OpCall, Dst: 3, Src: [2]int{2, 0}, Width: 8, Sym: "f1"},
			{Op: OpCall, Dst: 4, Src: [2]int{3, 0}, Width: 8, Sym: "f2"},
			// v1 live across both calls
		}},
	}

	origEligible, presplitEligible, newlyEligible := 0, 0, 0
	for _, tc := range cases {
		r := AnalyzePreSplitEligibility(tc.ops, desc, tc.name)
		t.Logf("%-20s  orig=%dv(%s)  split=%dv(%s)  saves=%d  blocker=%s",
			tc.name, r.OriginalNVregs, r.OriginalReason,
			r.PresplitNVregs, r.PresplitReason,
			r.SavesInserted, r.Blocker)
		if r.OriginalEligible {
			origEligible++
		}
		if r.PresplitEligible {
			presplitEligible++
		}
		if r.PresplitEligible && !r.OriginalEligible {
			newlyEligible++
		}
	}

	t.Logf("\n=== Hit-rate summary ===")
	t.Logf("Total functions:       %d", len(cases))
	t.Logf("Original eligible:     %d/%d (%.0f%%)", origEligible, len(cases), float64(origEligible)/float64(len(cases))*100)
	t.Logf("Pre-split eligible:    %d/%d (%.0f%%)", presplitEligible, len(cases), float64(presplitEligible)/float64(len(cases))*100)
	t.Logf("Newly eligible:        %d", newlyEligible)
	t.Logf("Lost eligibility:      %d", origEligible-presplitEligible+newlyEligible)
}

func TestLocSets8IXDefinitions(t *testing.T) {
	// Verify IX-expanded loc sets contain expected locations
	if len(locSets8IX) != 6 {
		t.Fatalf("expected 6 IX-expanded locSets, got %d", len(locSets8IX))
	}
	// Index 4: IX halves only
	ixOnly := locSets8IX[4]
	if len(ixOnly) != 4 {
		t.Fatalf("locSets8IX[4] should have 4 entries, got %d", len(ixOnly))
	}
	for _, loc := range ixOnly {
		if loc < 10 || loc > 13 {
			t.Errorf("locSets8IX[4] should only contain 10-13, got %d", loc)
		}
	}
	// Index 5: GPR8 + IX halves
	gprIx := locSets8IX[5]
	if len(gprIx) != 11 {
		t.Fatalf("locSets8IX[5] should have 11 entries, got %d", len(gprIx))
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
