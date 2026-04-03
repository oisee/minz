package mir2

import "testing"

// TestPointerWalkDedup_Positive: a loop body with two identical ptr_add(base, offset)
// should be deduplicated to one ptr_add + one copy.
func TestPointerWalkDedup_Positive(t *testing.T) {
	f := &Func{Name: "dedup_test", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Term:  &TermJmp{Target: "head", Args: []Reg{Reg(1)}},
	}
	head := &Block{
		Label: "head",
		Params: []BlockParam{
			{Dst: Reg(10), Ty: TyU8, Class: ClassGeneral},
		},
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(11), Imm: 0, Ty: TyU8},
			{Op: OpCmp, Dst: Reg(12), Src: [2]Reg{Reg(10), Reg(11)}, Ty: TyBool, Cond: CmpNe},
		},
		Term: &TermBrIf{
			Cond: Reg(12),
			Then: "body",
			Else: "exit",
		},
	}
	body := &Block{
		Label: "body",
		Insts: []*Inst{
			// base address
			{Op: OpAddrOf, Dst: Reg(20), Sym: "buf", Ty: TyPtr, Cls: ClassPointer},
			// widen index
			{Op: OpExt, Dst: Reg(21), Src: [2]Reg{Reg(10)}, SrcTy: TyU8, Ty: TyU16},
			// first ptr_add(base, offset) — will be kept
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			// load via first pointer
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
			// second ptr_add(base, offset) — DUPLICATE, should be deduped
			{Op: OpPtrAdd, Dst: Reg(24), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			// store via second pointer
			{Op: OpStore, Src: [2]Reg{Reg(24), Reg(23)}, Ty: TyU8},
			// loop counter decrement
			{Op: OpConst, Dst: Reg(25), Imm: 1, Ty: TyU8},
			{Op: OpSub, Dst: Reg(26), Src: [2]Reg{Reg(10), Reg(25)}, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(26)}},
	}
	exit := &Block{
		Label: "exit",
		Term:  &TermRet{Vals: nil},
	}
	f.Blocks = []*Block{entry, head, body, exit}

	var stats PointerWalkStats
	changed := ApplyPointerWalkDedup(f, &stats)
	if !changed {
		t.Fatal("expected transform to fire")
	}
	if stats.BlocksEdited != 1 {
		t.Fatalf("expected 1 block edited, got %d", stats.BlocksEdited)
	}
	if stats.PtrAddsDeduped != 1 {
		t.Fatalf("expected 1 ptr_add deduped, got %d", stats.PtrAddsDeduped)
	}

	// The second ptr_add (Reg(24)) should now be OpCopy of Reg(22)
	dupInst := body.Insts[4] // was the second OpPtrAdd
	if dupInst.Op != OpMove {
		t.Fatalf("expected duplicate ptr_add to become OpMove, got %v", dupInst.Op)
	}
	if dupInst.Src[0] != Reg(22) {
		t.Fatalf("expected OpMove src to be %%r22, got %%r%d", dupInst.Src[0])
	}

	// The store should now use Reg(22) instead of Reg(24)
	storeInst := body.Insts[5]
	if storeInst.Op != OpStore {
		t.Fatalf("expected store at index 5, got %v", storeInst.Op)
	}
	if storeInst.Src[0] != Reg(22) {
		t.Fatalf("expected store ptr to be %%r22 (deduped), got %%r%d", storeInst.Src[0])
	}
}

// TestPointerWalkDedup_Negative_NotInLoop: same pattern but NOT in a loop.
// Transform should NOT fire.
func TestPointerWalkDedup_Negative_NotInLoop(t *testing.T) {
	f := &Func{Name: "no_loop", nextReg: 100}

	// Straight-line: entry -> body -> exit (no backedge = no loop)
	entry := &Block{
		Label: "entry",
		Term:  &TermJmp{Target: "body"},
	}
	body := &Block{
		Label: "body",
		Insts: []*Inst{
			{Op: OpAddrOf, Dst: Reg(20), Sym: "buf", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpConst, Dst: Reg(21), Imm: 5, Ty: TyU16},
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
			{Op: OpPtrAdd, Dst: Reg(24), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpStore, Src: [2]Reg{Reg(24), Reg(23)}, Ty: TyU8},
		},
		Term: &TermJmp{Target: "exit"},
	}
	exit := &Block{
		Label: "exit",
		Term:  &TermRet{Vals: nil},
	}
	f.Blocks = []*Block{entry, body, exit}

	var stats PointerWalkStats
	changed := ApplyPointerWalkDedup(f, &stats)
	if changed {
		t.Fatal("expected transform NOT to fire outside a loop")
	}
}

// TestPointerWalkDedup_Negative_DifferentOffsets: two ptr_adds with same base
// but different offsets — should NOT dedup.
func TestPointerWalkDedup_Negative_DifferentOffsets(t *testing.T) {
	f := &Func{Name: "diff_offsets", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Term:  &TermJmp{Target: "head", Args: []Reg{Reg(1)}},
	}
	head := &Block{
		Label: "head",
		Params: []BlockParam{
			{Dst: Reg(10), Ty: TyU8, Class: ClassGeneral},
		},
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(11), Imm: 0, Ty: TyU8},
			{Op: OpCmp, Dst: Reg(12), Src: [2]Reg{Reg(10), Reg(11)}, Ty: TyBool, Cond: CmpNe},
		},
		Term: &TermBrIf{
			Cond: Reg(12),
			Then: "body",
			Else: "exit",
		},
	}
	body := &Block{
		Label: "body",
		Insts: []*Inst{
			{Op: OpAddrOf, Dst: Reg(20), Sym: "buf", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpExt, Dst: Reg(21), Src: [2]Reg{Reg(10)}, SrcTy: TyU8, Ty: TyU16},
			{Op: OpConst, Dst: Reg(27), Imm: 3, Ty: TyU16},
			// ptr_add(base, index) — one offset
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
			// ptr_add(base, 3) — DIFFERENT offset
			{Op: OpPtrAdd, Dst: Reg(24), Src: [2]Reg{Reg(20), Reg(27)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpStore, Src: [2]Reg{Reg(24), Reg(23)}, Ty: TyU8},
			{Op: OpConst, Dst: Reg(25), Imm: 1, Ty: TyU8},
			{Op: OpSub, Dst: Reg(26), Src: [2]Reg{Reg(10), Reg(25)}, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(26)}},
	}
	exit := &Block{
		Label: "exit",
		Term:  &TermRet{Vals: nil},
	}
	f.Blocks = []*Block{entry, head, body, exit}

	var stats PointerWalkStats
	changed := ApplyPointerWalkDedup(f, &stats)
	if changed {
		t.Fatal("expected transform NOT to fire with different offsets")
	}
}
