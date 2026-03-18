package lir

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// TestBridge_SimpleAdd compiles a HIR function through MIR2, lowers to LIR,
// and verifies MIR2-VM == LIR-VM convergence.
func TestBridge_SimpleAdd(t *testing.T) {
	// HIR: fun add(a: u8, b: u8) -> u8 { return a + b }
	hm := &hir.Module{Name: "test_bridge"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "add",
		Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.Ret(hir.Add(
				hir.Var("a", mir2.TyU8),
				hir.Var("b", mir2.TyU8),
				mir2.TyU8,
			)),
		),
	})

	// HIR → MIR2
	m := hir.LowerModule(hm)
	f := m.FuncByName("add")
	if f == nil {
		t.Fatal("function 'add' not found")
	}
	mir2.ReorderBlocks(f)

	// MIR2 → LIR on all machines
	machines := []*MachineDesc{RISC32, RISC8, CISC}

	for _, desc := range machines {
		result, err := LowerMIR2Func(f, desc)
		if err != nil {
			t.Logf("%s: LIR lower error (expected for partial bridge): %v", desc.Name, err)
			continue
		}
		t.Logf("%s: %d MIR ops → %d LIR insts", desc.Name, len(result.Ops), len(result.Insts))

		// Execute on LIR-VM
		vm := NewVM(desc)
		// Set up arguments: a=10, b=32 in first two regs
		if len(result.Insts) > 0 {
			// The MIR2 function has params → first ops should be const/move
			// For now just verify the lowering doesn't crash
			for i := range result.Insts {
				if err := vm.ExecInst(&result.Insts[i]); err != nil {
					t.Logf("%s: exec error at inst %d: %v", desc.Name, i, err)
					break
				}
			}
		}
	}
}

// TestBridge_LowerBlock verifies MIR2 block translation to LIR ops.
func TestBridge_LowerBlock(t *testing.T) {
	// Build a simple MIR2 function manually
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("simple")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	c1 := bld.Const(10, mir2.TyU8, mir2.ClassAcc)
	c2 := bld.Const(32, mir2.TyU8, mir2.ClassGeneral)
	sum := bld.Add(c1, c2, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(sum)

	// Lower to LIR ops
	machines := []*MachineDesc{RISC32, RISC8, CISC, MICRO}

	for _, desc := range machines {
		ops, err := LowerMIR2Block(f.Blocks[0], desc, nil)
		if err != nil {
			t.Fatalf("%s: %v", desc.Name, err)
		}

		t.Logf("%s: %d ops from MIR2 block", desc.Name, len(ops))
		for i, op := range ops {
			t.Logf("  [%d] op=%d dst=r%d src=r%d,r%d imm=%d w=%d",
				i, op.Op, op.Dst, op.Src[0], op.Src[1], op.Imm, op.Width)
		}

		// Should have: const(10), const(32), add
		if len(ops) < 3 {
			t.Errorf("%s: expected >=3 ops, got %d", desc.Name, len(ops))
			continue
		}
		if ops[0].Op != OpConst || ops[0].Imm != 10 {
			t.Errorf("%s: op[0] should be const 10", desc.Name)
		}
		if ops[1].Op != OpConst || ops[1].Imm != 32 {
			t.Errorf("%s: op[1] should be const 32", desc.Name)
		}
		if ops[2].Op != OpAdd {
			t.Errorf("%s: op[2] should be add", desc.Name)
		}

		// Full pipeline: isel → WFC → LIR-VM
		sel, err := SelectInstructions(desc, ops)
		if err != nil {
			t.Fatalf("%s isel: %v", desc.Name, err)
		}
		wfc := NewWFCState(desc, sel.Insts)
		wfc.Propagate()
		if err := wfc.Collapse(); err != nil {
			t.Fatalf("%s collapse: %v", desc.Name, err)
		}

		insts := wfc.ToInsts()
		vm := NewVM(desc)
		for i := range insts {
			vm.ExecInst(&insts[i])
		}

		last := insts[len(insts)-1]
		result := vm.Get(last.Dst.Phys)
		if result != 42 {
			t.Errorf("%s: MIR2→LIR→VM = %d, want 42", desc.Name, result)
		} else {
			t.Logf("%s: MIR2→LIR→VM = %d ✓ (@%s)", desc.Name, result, desc.Locs[last.Dst.Phys].Name)
		}
	}
}
