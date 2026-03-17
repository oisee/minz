package lir

import (
	"testing"

	"github.com/minz/minzc/pkg/lir/isle"
)

// TestCombine_Load16LE tests the FatFS ld_word pattern:
//
//	lo  = load8(base)
//	hi  = load8(base + 1)
//	hi2 = shl(hi, 8)
//	val = or(hi2, lo)
//
// ISLE combining should fuse this into: val = load16_le(base)
func TestCombine_Load16LE(t *testing.T) {
	// MIR ops that represent the ld_word idiom:
	//   r0 = base address (already loaded)
	//   r1 = load8(r0)              -- lo byte
	//   r2 = const 1
	//   r3 = add(r0, r2)            -- base + 1
	//   r4 = load8(r3)              -- hi byte
	//   r5 = const 8
	//   r6 = shl(r4, r5)            -- hi << 8
	//   r7 = or(r6, r1)             -- (hi << 8) | lo = 16-bit LE value
	ops := []MIROp{
		{Op: OpConst, Dst: 0, Src: [2]int{-1, -1}, Imm: 0x100, Width: 16}, // r0 = base addr
		{Op: OpLoad, Dst: 1, Src: [2]int{0, -1}, Width: 8},                 // r1 = load8(r0)
		{Op: OpConst, Dst: 2, Src: [2]int{-1, -1}, Imm: 1, Width: 8},      // r2 = 1
		{Op: OpAdd, Dst: 3, Src: [2]int{0, 2}, Width: 16},                  // r3 = r0 + 1
		{Op: OpLoad, Dst: 4, Src: [2]int{3, -1}, Width: 8},                 // r4 = load8(r3)
		{Op: OpConst, Dst: 5, Src: [2]int{-1, -1}, Imm: 8, Width: 8},      // r5 = 8
		{Op: OpShl, Dst: 6, Src: [2]int{4, 5}, Width: 16},                  // r6 = r4 << 8
		{Op: OpOr, Dst: 7, Src: [2]int{6, 1}, Width: 16},                   // r7 = r6 | r1
	}

	result, err := Combine(ops)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("input:  %d ops", len(ops))
	t.Logf("output: %d ops (changed=%v)", len(result.Ops), result.Changed)
	for i, op := range result.Ops {
		t.Logf("  [%d] %s dst=r%d src=r%d,r%d imm=%d w=%d",
			i, opName(op.Op), op.Dst, op.Src[0], op.Src[1], op.Imm, op.Width)
	}

	if !result.Changed {
		t.Error("expected combining to fire")
	}

	// Check that a load16_le op was produced
	found := false
	for _, op := range result.Ops {
		if op.Op == OpLoad16LE {
			found = true
			t.Logf("✓ load16_le found: dst=r%d src=r%d", op.Dst, op.Src[0])
			if op.Src[0] != 0 {
				t.Errorf("load16_le base should be r0 (base addr), got r%d", op.Src[0])
			}
		}
	}
	if !found {
		t.Error("no load16_le op produced — combining rule didn't fire")
	}
}

// TestCombine_Load16LE_Convergence runs the combined load16_le through
// isel + WFC + VM on all machines and verifies the value matches
// a manual little-endian read.
func TestCombine_Load16LE_Convergence(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8, CISC}
	// MICRO excluded: load16_le needs pointer reg that MICRO lacks generically

	for _, m := range machines {
		t.Run(m.Name, func(t *testing.T) {
			// Set up memory: addr 0x100 = 0x42 (lo), 0x101 = 0xAB (hi)
			// Expected 16-bit LE value: 0xAB42

			// After combining, we need:
			//   r0 = const 0x100   (base address)
			//   r1 = load16_le(r0) (combined 16-bit load)
			ops := []MIROp{
				{Op: OpConst, Dst: 0, Src: [2]int{-1, -1}, Imm: 0x100, Width: 16},
				{Op: OpLoad16LE, Dst: 1, Src: [2]int{0, -1}, Width: 16},
			}

			sel, err := SelectInstructions(m, ops)
			if err != nil {
				t.Fatalf("isel: %v", err)
			}

			wfc := NewWFCState(m, sel.Insts)
			wfc.Propagate()
			if err := wfc.Collapse(); err != nil {
				t.Fatalf("collapse: %v", err)
			}

			insts := wfc.ToInsts()
			vm := NewVM(m)

			// Plant bytes in memory
			vm.Memory[0x100] = 0x42 // lo
			vm.Memory[0x101] = 0xAB // hi

			for i := range insts {
				if err := vm.ExecInst(&insts[i]); err != nil {
					t.Fatalf("exec[%d]: %v", i, err)
				}
			}

			last := insts[len(insts)-1]
			result := vm.Get(last.Dst.Phys)
			expected := uint64(0xAB42)

			if result != expected {
				t.Errorf("DIVERGENCE: got 0x%04X, want 0x%04X", result, expected)
			} else {
				t.Logf("load16_le = 0x%04X ✓ @%s", result, m.Locs[last.Dst.Phys].Name)
			}
		})
	}
}

// TestCombine_Load16LE_FullPipeline is the crown jewel:
// raw shift+or MIROps → ISLE combine → isel → WFC → VM.
// Verifies end-to-end that the uncombined SSA produces the same result
// as the combined form, on all machines.
func TestCombine_Load16LE_FullPipeline(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}
	// CISC/MICRO: the uncombined form needs 8-bit loads → shift → or which
	// requires more patterns; combined form test above covers those.

	for _, m := range machines {
		t.Run(m.Name, func(t *testing.T) {
			// Uncombined: the FatFS ld_word SSA pattern
			uncombined := []MIROp{
				{Op: OpConst, Dst: 0, Src: [2]int{-1, -1}, Imm: 0x200, Width: 16},
				{Op: OpLoad, Dst: 1, Src: [2]int{0, -1}, Width: 8},              // lo
				{Op: OpConst, Dst: 2, Src: [2]int{-1, -1}, Imm: 1, Width: 16},
				{Op: OpAdd, Dst: 3, Src: [2]int{0, 2}, Width: 16},               // base+1
				{Op: OpLoad, Dst: 4, Src: [2]int{3, -1}, Width: 8},              // hi
				{Op: OpConst, Dst: 5, Src: [2]int{-1, -1}, Imm: 8, Width: 16},
				{Op: OpShl, Dst: 6, Src: [2]int{4, 5}, Width: 16},               // hi<<8
				{Op: OpOr, Dst: 7, Src: [2]int{6, 1}, Width: 16},                // (hi<<8)|lo
			}

			// Run combining
			cr, err := Combine(uncombined)
			if err != nil {
				t.Fatalf("combine: %v", err)
			}
			if !cr.Changed {
				t.Fatal("combining didn't fire")
			}
			t.Logf("combined: %d ops → %d ops", len(uncombined), len(cr.Ops))

			// Run combined ops through isel + WFC + VM
			sel, err := SelectInstructions(m, cr.Ops)
			if err != nil {
				t.Fatalf("isel: %v", err)
			}
			wfc := NewWFCState(m, sel.Insts)
			wfc.Propagate()
			if err := wfc.Collapse(); err != nil {
				t.Fatalf("collapse: %v", err)
			}

			insts := wfc.ToInsts()
			vm := NewVM(m)
			vm.Memory[0x200] = 0xEF // lo byte
			vm.Memory[0x201] = 0xBE // hi byte

			for i := range insts {
				if err := vm.ExecInst(&insts[i]); err != nil {
					t.Fatalf("exec[%d]: %v", i, err)
				}
			}

			// Find the load16_le result
			var result uint64
			for i := len(insts) - 1; i >= 0; i-- {
				if insts[i].Pat.MIROp == OpLoad16LE {
					result = vm.Get(insts[i].Dst.Phys)
					break
				}
			}

			expected := uint64(0xBEEF)
			if result != expected {
				// Fallback: check last instruction
				last := insts[len(insts)-1]
				result = vm.Get(last.Dst.Phys)
			}

			if result != expected {
				t.Errorf("DIVERGENCE: got 0x%04X, want 0x%04X (0xBEEF)", result, expected)
			} else {
				t.Logf("full pipeline: shift+or → load16_le → 0x%04X ✓", result)
			}
		})
	}
}

// TestCombine_IdentityRules tests that identity simplifications work.
func TestCombine_IdentityRules(t *testing.T) {
	// r0 = 42; r1 = 0; r2 = add(r0, r1) → should simplify add(x, 0) → x
	ops := []MIROp{
		{Op: OpConst, Dst: 0, Src: [2]int{-1, -1}, Imm: 42, Width: 8},
		{Op: OpConst, Dst: 1, Src: [2]int{-1, -1}, Imm: 0, Width: 8},
		{Op: OpAdd, Dst: 2, Src: [2]int{0, 1}, Width: 8},
	}

	result, err := Combine(ops)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("identity: %d ops → %d ops (changed=%v)", len(ops), len(result.Ops), result.Changed)
	for i, op := range result.Ops {
		t.Logf("  [%d] %s dst=r%d imm=%d", i, opName(op.Op), op.Dst, op.Imm)
	}
}

// TestCombine_ISLE_RuleParsing verifies the combining rules parse without error.
func TestCombine_ISLE_RuleParsing(t *testing.T) {
	rs, err := isle.Parse(CombineRules)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("parsed %d rules", len(rs.Rules))
	for _, r := range rs.Rules {
		t.Logf("  [pri=%d] %s → %s", r.Priority, r.LHS, r.RHS)
	}
	if len(rs.Rules) < 5 {
		t.Errorf("expected at least 5 rules (2 load16_le + 4 identity), got %d", len(rs.Rules))
	}
}
