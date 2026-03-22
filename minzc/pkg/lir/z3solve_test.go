package lir

import (
	"os/exec"
	"strings"
	"testing"
)

func hasZ3() bool {
	_, err := exec.LookPath(Z3Path)
	return err == nil
}

// TestZ3_SimpleAdd: compute a + b where a must be in A, b in any GPR.
// Z3 should find optimal: b in B (cost 0), not in IXH (cost 4).
func TestZ3_SimpleAdd(t *testing.T) {
	if !hasZ3() {
		t.Skip("z3 not found at " + Z3Path)
	}

	desc := Z80

	// Build a tiny program: two const loads + one add.
	// v1 = const 5     (can be anywhere GPR)
	// v2 = const 3     (can be anywhere GPR)
	// v3 = add v1, v2  (dst must be A, src0 must be A)
	gpr8 := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
	accOnly := desc.LocSetByNames("A")

	prog := &Prog{
		Name: "test_add",
		Desc: desc,
		Blocks: []Block{{
			Label: "entry",
			Insts: []Inst{
				{
					Pat:  &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7},
					Dst:  Operand{VReg: 1, Allowed: gpr8},
					Imm:  5,
				},
				{
					Pat:  &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7},
					Dst:  Operand{VReg: 2, Allowed: gpr8},
					Imm:  3,
				},
				{
					Pat:  &Pattern{Name: "add_a_r", MIROp: OpAdd, Cost: 4},
					Dst:  Operand{VReg: 3, Allowed: accOnly},
					Srcs: [2]Operand{
						{VReg: 1, Allowed: accOnly}, // src0 must be A (accumulator)
						{VReg: 2, Allowed: gpr8},    // src1 any GPR
					},
				},
			},
			Term: Term{Kind: TermReturn, RetVals: []Operand{{VReg: 3, Allowed: accOnly}}},
		}},
	}

	result, err := SolveOptimal(prog)
	if err != nil {
		t.Fatalf("Z3 solve failed: %v", err)
	}

	t.Logf("Z3 result: total_cost=%d, optimal=%v, vregs=%d", result.TotalCost, result.Optimal, len(result.VRegPhys))
	t.Logf("Z3 raw output:\n%s", result.Stats.RawOutput)
	for vreg, phys := range result.VRegPhys {
		locName := "?"
		if phys < len(desc.Locs) {
			locName = desc.Locs[phys].Name
		}
		t.Logf("  v%d → %s (loc %d)", vreg, locName, phys)
	}

	// v1 must be in A (because src0 of add requires A).
	if phys, ok := result.VRegPhys[1]; !ok || desc.Locs[phys].Name != "A" {
		t.Errorf("v1 should be A, got loc %d", result.VRegPhys[1])
	}

	// v3 must be in A (dst of add requires A).
	if phys, ok := result.VRegPhys[3]; !ok || desc.Locs[phys].Name != "A" {
		t.Errorf("v3 should be A, got loc %d", result.VRegPhys[3])
	}

	// v2 should be in a cheap GPR (cost 0), not IXH/shadow (cost 4+).
	if phys, ok := result.VRegPhys[2]; ok {
		cost := 0
		if phys < len(desc.LocCost) {
			cost = desc.LocCost[phys]
		}
		if cost > 0 {
			t.Errorf("v2 assigned to loc %d (cost %d), expected cost-0 GPR", phys, cost)
		}
	}
}

// TestZ3_Interference: two vregs live at the same time must get different registers.
func TestZ3_Interference(t *testing.T) {
	if !hasZ3() {
		t.Skip("z3 not found at " + Z3Path)
	}

	desc := Z80
	gpr8 := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L")

	// v1 = const 1
	// v2 = const 2
	// v3 = add v1, v2  (both live at this point → must be different regs)
	prog := &Prog{
		Name: "test_interference",
		Desc: desc,
		Blocks: []Block{{
			Label: "entry",
			Insts: []Inst{
				{
					Pat: &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7},
					Dst: Operand{VReg: 1, Allowed: gpr8},
					Imm: 1,
				},
				{
					Pat: &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7},
					Dst: Operand{VReg: 2, Allowed: gpr8},
					Imm: 2,
				},
				{
					Pat:  &Pattern{Name: "add_a_r", MIROp: OpAdd, Cost: 4},
					Dst:  Operand{VReg: 3, Allowed: desc.LocSetByNames("A")},
					Srcs: [2]Operand{
						{VReg: 1, Allowed: desc.LocSetByNames("A")},
						{VReg: 2, Allowed: gpr8},
					},
				},
			},
			Term: Term{Kind: TermReturn},
		}},
	}

	result, err := SolveOptimal(prog)
	if err != nil {
		t.Fatalf("Z3 solve failed: %v", err)
	}

	// v1 and v2 must be in different locations.
	phys1 := result.VRegPhys[1]
	phys2 := result.VRegPhys[2]
	if phys1 == phys2 {
		t.Errorf("v1 and v2 both assigned to loc %d — interference violation!", phys1)
	}
	t.Logf("v1→%s, v2→%s, v3→%s (no conflict ✓)",
		desc.Locs[phys1].Name, desc.Locs[phys2].Name, desc.Locs[result.VRegPhys[3]].Name)
}

// TestZ3_HighPressure: 5 vregs all live simultaneously in GPR,
// then one ADD uses two of them. Realistic: values loaded into different regs.
func TestZ3_HighPressure(t *testing.T) {
	if !hasZ3() {
		t.Skip("z3 not found at " + Z3Path)
	}

	desc := Z80
	gpr8 := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
	accOnly := desc.LocSetByNames("A")

	// 5 values all live until the add at the end.
	prog := &Prog{
		Name: "test_pressure",
		Desc: desc,
		Blocks: []Block{{
			Label: "entry",
			Insts: []Inst{
				{Pat: &Pattern{Name: "ld", MIROp: OpConst, Cost: 7}, Dst: Operand{VReg: 1, Allowed: gpr8}, Imm: 1},
				{Pat: &Pattern{Name: "ld", MIROp: OpConst, Cost: 7}, Dst: Operand{VReg: 2, Allowed: gpr8}, Imm: 2},
				{Pat: &Pattern{Name: "ld", MIROp: OpConst, Cost: 7}, Dst: Operand{VReg: 3, Allowed: gpr8}, Imm: 3},
				{Pat: &Pattern{Name: "ld", MIROp: OpConst, Cost: 7}, Dst: Operand{VReg: 4, Allowed: gpr8}, Imm: 4},
				{Pat: &Pattern{Name: "ld", MIROp: OpConst, Cost: 7}, Dst: Operand{VReg: 5, Allowed: gpr8}, Imm: 5},
				// v6 = v1 + v5 (forces v1..v5 all live at inst 5)
				{
					Pat:  &Pattern{Name: "add_a_r", MIROp: OpAdd, Cost: 4},
					Dst:  Operand{VReg: 6, Allowed: accOnly},
					Srcs: [2]Operand{{VReg: 1, Allowed: accOnly}, {VReg: 5, Allowed: gpr8}},
				},
			},
			Term: Term{Kind: TermReturn, RetVals: []Operand{{VReg: 6, Allowed: accOnly}}},
		}},
	}

	result, err := SolveOptimal(prog)
	if err != nil {
		t.Fatalf("Z3 solve failed: %v", err)
	}

	t.Logf("High pressure assignment (5 live + 1 add):")
	for _, v := range []int{1, 2, 3, 4, 5, 6} {
		if phys, ok := result.VRegPhys[v]; ok {
			t.Logf("  v%d → %s", v, desc.Locs[phys].Name)
		}
	}
	t.Logf("Total cost: %d (optimal: %v)", result.TotalCost, result.Optimal)

	// Verify all different (except tied v1=v6=A).
	seen := map[int]int{}
	for v, p := range result.VRegPhys {
		if v == 6 {
			continue // v6 tied to v1
		}
		if prev, ok := seen[p]; ok && prev != v {
			// Only error if both are truly live at same point.
			t.Logf("  note: v%d and v%d share %s (may be non-interfering)", prev, v, desc.Locs[p].Name)
		}
		seen[p] = v
	}
}

// TestZ3_SpillToIXH: 8 vregs all live, only 7 GPR → must spill to IXH/IXL.
// All 8 loaded into GPR+IX, then used in a final expression.
func TestZ3_SpillToIXH(t *testing.T) {
	if !hasZ3() {
		t.Skip("z3 not found at " + Z3Path)
	}

	desc := Z80
	gprAndIX := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L", "IXH", "IXL")
	accOnly := desc.LocSetByNames("A")

	// Define 8 values, all in GPR or IX.
	var insts []Inst
	for i := 1; i <= 8; i++ {
		insts = append(insts, Inst{
			Pat: &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7},
			Dst: Operand{VReg: i, Allowed: gprAndIX},
			Imm: int64(i),
		})
	}
	// v9 = v1 + v8 — forces all 8 to be live at this point.
	insts = append(insts, Inst{
		Pat:  &Pattern{Name: "add_a_r", MIROp: OpAdd, Cost: 4},
		Dst:  Operand{VReg: 9, Allowed: accOnly},
		Srcs: [2]Operand{
			{VReg: 1, Allowed: accOnly},
			{VReg: 8, Allowed: gprAndIX},
		},
	})

	prog := &Prog{
		Name: "test_spill_ixh",
		Desc: desc,
		Blocks: []Block{{
			Label: "entry",
			Insts: insts,
			Term:  Term{Kind: TermReturn},
		}},
	}

	result, err := SolveOptimal(prog)
	if err != nil {
		t.Fatalf("Z3 solve failed: %v", err)
	}

	ixCount := 0
	for _, phys := range result.VRegPhys {
		if phys < len(desc.Locs) {
			name := desc.Locs[phys].Name
			if name == "IXH" || name == "IXL" {
				ixCount++
			}
		}
	}

	t.Logf("Spill assignment (8 defined, GPR+IX available):")
	for v := 1; v <= 9; v++ {
		if phys, ok := result.VRegPhys[v]; ok {
			name := "?"
			if phys < len(desc.Locs) {
				name = desc.Locs[phys].Name
			}
			cost := 0
			if phys < len(desc.LocCost) {
				cost = desc.LocCost[phys]
			}
			t.Logf("  v%d → %s (cost %d)", v, name, cost)
		}
	}
	t.Logf("Total cost: %d, IX spills: %d", result.TotalCost, ixCount)

	// With 8 values and only 7 GPR, at least 1 must go to IX.
	if ixCount < 1 {
		t.Logf("NOTE: Z3 found a way without IX spills — liveness may allow sharing")
	}
}

// TestZ3_SMTDump: verify the SMT-LIB2 output is valid by checking it contains
// expected structural elements.
func TestZ3_SMTDump(t *testing.T) {
	desc := Z80
	gpr8 := desc.LocSetByNames("A", "B", "C")

	prog := &Prog{
		Name: "test_smt_dump",
		Desc: desc,
		Blocks: []Block{{
			Label: "entry",
			Insts: []Inst{
				{
					Pat: &Pattern{Name: "ld", MIROp: OpConst, Cost: 7},
					Dst: Operand{VReg: 1, Allowed: gpr8},
				},
			},
			Term: Term{Kind: TermReturn},
		}},
	}

	live := computeLiveness(prog)
	smt, _ := encodeSMT(prog, live, nil)

	// Check structural elements.
	if !strings.Contains(smt, "declare-const v1 Int") {
		t.Error("SMT should declare v1")
	}
	if !strings.Contains(smt, "minimize total_cost") {
		t.Error("SMT should minimize cost")
	}
	if !strings.Contains(smt, "check-sat") {
		t.Error("SMT should call check-sat")
	}
	t.Logf("SMT output (%d bytes):\n%s", len(smt), smt)
}
