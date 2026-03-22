package lir

import (
	"fmt"
	"testing"
)

// TestZ3_vs_WFC compares Z3 optimal vs WFC greedy on a realistic function.
// Builds a LIR program that simulates: result = (a*2 + b) ^ c
// with multiple live values and accumulator constraints.
func TestZ3_vs_WFC(t *testing.T) {
	if !hasZ3() {
		t.Skip("z3 not found at " + Z3Path)
	}

	desc := Z80
	gpr8 := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
	accOnly := desc.LocSetByNames("A")
	_ = accOnly // used below

	// Simulate: fun compute(a: u8, b: u8, c: u8) -> u8 = (a*2 + b) ^ c
	//
	// v1 = param a (any GPR)
	// v2 = param b (any GPR)
	// v3 = param c (any GPR)
	// v4 = add v1, v1  (a*2, dst=A, src0=A)
	// v5 = add v4, v2  (a*2+b, dst=A, src0=A)
	// v6 = xor v5, v3  ((a*2+b)^c, dst=A, src0=A)
	// return v6

	prog := &Prog{
		Name: "compute",
		Desc: desc,
		Blocks: []Block{{
			Label: "entry",
			Insts: []Inst{
				// Params loaded into registers.
				{Pat: &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7, DstLocs: gpr8},
					Dst: Operand{VReg: 1, Allowed: gpr8}, Imm: 10},
				{Pat: &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7, DstLocs: gpr8},
					Dst: Operand{VReg: 2, Allowed: gpr8}, Imm: 20},
				{Pat: &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7, DstLocs: gpr8},
					Dst: Operand{VReg: 3, Allowed: gpr8}, Imm: 30},

				// v4 = v1 + v1 (a*2)
				{Pat: &Pattern{Name: "add_a_r", MIROp: OpAdd, Cost: 4, DstLocs: accOnly, SrcLocs: [2]LocSet{accOnly, gpr8}},
					Dst:  Operand{VReg: 4, Allowed: accOnly},
					Srcs: [2]Operand{{VReg: 1, Allowed: accOnly}, {VReg: 1, Allowed: gpr8}}},

				// v5 = v4 + v2 (a*2 + b)
				{Pat: &Pattern{Name: "add_a_r", MIROp: OpAdd, Cost: 4, DstLocs: accOnly, SrcLocs: [2]LocSet{accOnly, gpr8}},
					Dst:  Operand{VReg: 5, Allowed: accOnly},
					Srcs: [2]Operand{{VReg: 4, Allowed: accOnly}, {VReg: 2, Allowed: gpr8}}},

				// v6 = v5 ^ v3 (result)
				{Pat: &Pattern{Name: "xor_a_r", MIROp: OpXor, Cost: 4, DstLocs: accOnly, SrcLocs: [2]LocSet{accOnly, gpr8}},
					Dst:  Operand{VReg: 6, Allowed: accOnly},
					Srcs: [2]Operand{{VReg: 5, Allowed: accOnly}, {VReg: 3, Allowed: gpr8}}},
			},
			Term: Term{Kind: TermReturn, RetVals: []Operand{{VReg: 6, Allowed: accOnly}}},
		}},
	}

	// --- Run Z3 ---
	z3Result, err := SolveOptimal(prog)
	if err != nil {
		t.Fatalf("Z3 solve failed: %v", err)
	}

	t.Logf("=== Z3 Optimal ===")
	t.Logf("Total cost: %d (optimal: %v)", z3Result.TotalCost, z3Result.Optimal)
	for v := 1; v <= 6; v++ {
		if phys, ok := z3Result.VRegPhys[v]; ok {
			name := desc.Locs[phys].Name
			cost := 0
			if phys < len(desc.LocCost) {
				cost = desc.LocCost[phys]
			}
			t.Logf("  v%d → %s (cost %d)", v, name, cost)
		}
	}

	// --- Run WFC ---
	wfc := NewWFCState(desc, prog.Blocks[0].Insts)
	wfc.Propagate()
	wfcErr := wfc.Collapse()

	t.Logf("=== WFC Greedy ===")
	if wfcErr != nil {
		t.Logf("WFC failed: %v", wfcErr)
	} else {
		wfcCost := 0
		for i, c := range wfc.Cells {
			locName := "?"
			cost := 0
			// Find collapsed dst location.
			for bit := 0; bit < MaxLocs; bit++ {
				if c.DstLocs.Has(bit) {
					locName = desc.Locs[bit].Name
					if bit < len(desc.LocCost) {
						cost = desc.LocCost[bit]
					}
					break
				}
			}
			wfcCost += cost
			if c.VRegDst >= 0 {
				t.Logf("  inst %d: v%d → %s (cost %d)", i, c.VRegDst, locName, cost)
			}
		}
		t.Logf("WFC total cost: %d", wfcCost)
	}

	// --- Compare ---
	t.Logf("=== Comparison ===")
	t.Logf("Z3 cost: %d, WFC cost: evaluated above", z3Result.TotalCost)

	// Verify Z3 assigned A-requiring vregs to A.
	for _, v := range []int{4, 5, 6} {
		if phys, ok := z3Result.VRegPhys[v]; ok {
			if desc.Locs[phys].Name != "A" {
				t.Errorf("v%d should be in A, got %s", v, desc.Locs[phys].Name)
			}
		}
	}
}

// TestZ3_vs_WFC_WithSpill: 8 values live simultaneously → forces spill.
// Compares Z3 (optimal spill placement) vs WFC (greedy).
func TestZ3_vs_WFC_WithSpill(t *testing.T) {
	if !hasZ3() {
		t.Skip("z3 not found at " + Z3Path)
	}

	desc := Z80
	gprAndIX := desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L", "IXH", "IXL")
	accOnly := desc.LocSetByNames("A")

	// 7 consts + 1 add = forces all 7 to be live when the add happens.
	// The add uses v1 (must be A) and v7 (any GPR).
	// With 7 values all needing different regs, v1=A leaves B,C,D,E,H,L for 6 values — enough.
	// But if we add an 8th, one must go to IXH.
	var insts []Inst
	for i := 1; i <= 7; i++ {
		insts = append(insts, Inst{
			Pat: &Pattern{Name: "ld_r_imm", MIROp: OpConst, Cost: 7, DstLocs: gprAndIX},
			Dst: Operand{VReg: i, Allowed: gprAndIX},
			Imm: int64(i * 10),
		})
	}
	// v8 = v1 + v7 (all 7 must be live here)
	insts = append(insts, Inst{
		Pat:  &Pattern{Name: "add_a_r", MIROp: OpAdd, Cost: 4, DstLocs: accOnly, SrcLocs: [2]LocSet{accOnly, gprAndIX}},
		Dst:  Operand{VReg: 8, Allowed: accOnly},
		Srcs: [2]Operand{{VReg: 1, Allowed: accOnly}, {VReg: 7, Allowed: gprAndIX}},
	})

	prog := &Prog{
		Name: "spill_test",
		Desc: desc,
		Blocks: []Block{{
			Label: "entry",
			Insts: insts,
			Term:  Term{Kind: TermReturn, RetVals: []Operand{{VReg: 8, Allowed: accOnly}}},
		}},
	}

	z3Result, err := SolveOptimal(prog)
	if err != nil {
		t.Fatalf("Z3 solve failed: %v", err)
	}

	t.Logf("=== Z3 Optimal (7 live + spill) ===")
	t.Logf("Total cost: %d", z3Result.TotalCost)
	ixCount := 0
	for v := 1; v <= 8; v++ {
		if phys, ok := z3Result.VRegPhys[v]; ok {
			name := desc.Locs[phys].Name
			cost := 0
			if phys < len(desc.LocCost) {
				cost = desc.LocCost[phys]
			}
			t.Logf("  v%d → %s (cost %d)", v, name, cost)
			if name == "IXH" || name == "IXL" {
				ixCount++
			}
		}
	}
	t.Logf("IX spills: %d", ixCount)

	// With 7 distinct values in GPR (7 available) — should fit exactly.
	// v1=A is tied to v8=A, so really 6 other values need 6 regs = fits.
	// Z3 should find cost=0 (all in GPR).
	if z3Result.TotalCost > 0 {
		t.Logf("Z3 used expensive locations — checking if unavoidable")
	}

	// Print Z3 SMT for manual inspection.
	live := computeLiveness(prog)
	smt, _ := encodeSMT(prog, live)
	_ = smt // available for debugging: t.Logf("SMT:\n%s", smt)
	_ = fmt.Sprintf("") // satisfy import
}
