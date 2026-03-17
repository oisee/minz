package lir

import "testing"

// TestLoopRotate_WhileToDJNZ is the crown jewel: take a while(cnt!=0) loop,
// rotate it to DJNZ form, and verify the result matches the original.
//
// Before rotation:
//
//	entry:  sum=0; cnt=5; one=1; jump @head
//	head:   branch cnt!=0 @body, @exit
//	body:   sum+=cnt; cnt-=one; jump @head
//	exit:   ret sum
//
// After LoopRotateDJNZ:
//
//	entry:  sum=0; cnt=5; one=1; jump @head
//	head:   branch cnt!=0 @body, @exit   (guard, runs once)
//	body:   sum+=cnt; djnz cnt, @body, @exit
//	exit:   ret sum
//
// The sub(cnt, 1) is absorbed into DJNZ.
func TestLoopRotate_WhileToDJNZ(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}

	for _, m := range machines {
		t.Run(m.Name, func(t *testing.T) {
			// Build the while-loop version
			prog := makeSumProg_While(m, 5)

			// Execute unrotated
			vm1 := NewVM(m)
			r1, err := vm1.ExecProg(prog)
			if err != nil {
				t.Fatal("before rotation:", err)
			}
			t.Logf("before: sum(1..5) = %d", r1[0])

			// Rotate
			rotResult := LoopRotateDJNZ(prog)
			t.Logf("rotated %d loops", rotResult.Rotated)

			if rotResult.Rotated == 0 {
				t.Fatal("expected loop rotation")
			}

			// Verify body terminator is now DJNZ
			bodyBlock := findBlock(prog, "body")
			if bodyBlock == nil {
				t.Fatal("body block not found")
			}
			if bodyBlock.Term.Kind != TermDJNZ {
				t.Errorf("body terminator: got %d, want TermDJNZ(%d)",
					bodyBlock.Term.Kind, TermDJNZ)
			}
			t.Logf("body terminator: DJNZ counter@%s → [%s, %s]",
				m.Locs[bodyBlock.Term.Counter.Phys].Name,
				bodyBlock.Term.Targets[0], bodyBlock.Term.Targets[1])

			// Verify sub instruction was absorbed
			for _, inst := range bodyBlock.Insts {
				if inst.Pat != nil && inst.Pat.MIROp == OpSub {
					t.Error("sub instruction should have been absorbed by DJNZ")
				}
			}

			// Execute rotated — must produce same result
			vm2 := NewVM(m)
			r2, err := vm2.ExecProg(prog)
			if err != nil {
				t.Fatal("after rotation:", err)
			}

			if r1[0] != r2[0] {
				t.Errorf("DIVERGENCE: before=%d, after=%d", r1[0], r2[0])
			} else {
				t.Logf("after:  sum(1..5) = %d ✓ (DJNZ, same result)", r2[0])
			}

			if r2[0] != 15 {
				t.Errorf("expected 15, got %d", r2[0])
			}
		})
	}
}

// TestLoopRotate_Basic tests simple rotation (no DJNZ, just branch at bottom).
func TestLoopRotate_Basic(t *testing.T) {
	m := RISC32
	prog := makeSumProg_While(m, 7)

	vm1 := NewVM(m)
	r1, err := vm1.ExecProg(prog)
	if err != nil {
		t.Fatal(err)
	}

	result := LoopRotate(prog)
	t.Logf("rotated %d loops", result.Rotated)

	bodyBlock := findBlock(prog, "body")
	if bodyBlock == nil {
		t.Fatal("body block not found")
	}
	if bodyBlock.Term.Kind != TermBranch {
		t.Errorf("expected TermBranch after rotation, got %d", bodyBlock.Term.Kind)
	}

	vm2 := NewVM(m)
	r2, err := vm2.ExecProg(prog)
	if err != nil {
		t.Fatal(err)
	}

	expected := uint64(28) // sum(1..7) = 28
	if r2[0] != expected {
		t.Errorf("got %d, want %d", r2[0], expected)
	} else {
		t.Logf("rotated while: sum(1..7) = %d ✓", r2[0])
	}

	if r1[0] != r2[0] {
		t.Errorf("DIVERGENCE: before=%d, after=%d", r1[0], r2[0])
	}
}

// TestLoopRotate_MultiMachine_Convergence runs rotation on all machines
// and verifies all produce the same result.
func TestLoopRotate_MultiMachine_Convergence(t *testing.T) {
	machines := []*MachineDesc{RISC32, RISC8}
	var results []uint64

	for _, m := range machines {
		// Test with DJNZ rotation
		prog := makeSumProg_While(m, 10)
		LoopRotateDJNZ(prog)

		vm := NewVM(m)
		r, err := vm.ExecProg(prog)
		if err != nil {
			t.Fatalf("%s: %v", m.Name, err)
		}

		t.Logf("%s: sum(1..10) = %d", m.Name, r[0])
		results = append(results, r[0])
	}

	// All must agree
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("DIVERGENCE: %s=%d, %s=%d",
				machines[0].Name, results[0],
				machines[i].Name, results[i])
		}
	}

	if results[0] != 55 {
		t.Errorf("expected 55, got %d", results[0])
	} else {
		t.Logf("all machines agree: sum(1..10) = 55 ✓ (DJNZ rotated)")
	}
}

func findBlock(prog *Prog, label string) *Block {
	for i := range prog.Blocks {
		if prog.Blocks[i].Label == label {
			return &prog.Blocks[i]
		}
	}
	return nil
}
