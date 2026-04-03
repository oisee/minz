package mir2

import (
	"testing"
)

// TestPointerThreading_Positive: a simple countdown loop with
// ptr_add(base, ext(index)) in the body. After threading, the loop
// should carry a walking pointer as a block arg.
func TestPointerThreading_Positive(t *testing.T) {
	f := &Func{Name: "thread_test", nextReg: 100}

	// entry: jmp @head(initial_index=10)
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 10, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(1)}},
	}

	// head(%idx: u8): br_if (idx != 0), @body, @exit
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

	// body:
	//   %base = addr_of "buf"
	//   %wide = ext %idx
	//   %ptr = ptr_add %base, %wide    ← should be replaced
	//   %val = load %ptr
	//   %one = const 1
	//   %next = sub %idx, %one
	//   jmp @head(%next)
	body := &Block{
		Label: "body",
		Insts: []*Inst{
			{Op: OpAddrOf, Dst: Reg(20), Sym: "buf", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpExt, Dst: Reg(21), Src: [2]Reg{Reg(10)}, SrcTy: TyU8, Ty: TyU16},
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
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

	origHeadParams := len(head.Params)
	origBodyParams := len(body.Params)

	var stats PointerThreadingStats
	changed := ApplyPointerThreading(f, &stats)

	if !changed {
		t.Fatal("expected pointer threading to fire")
	}

	// Header should have gained a new block param (the walking pointer).
	if len(head.Params) != origHeadParams+1 {
		t.Fatalf("expected header to gain 1 param, had %d now %d", origHeadParams, len(head.Params))
	}
	newParam := head.Params[len(head.Params)-1]
	if newParam.Ty != TyPtr {
		t.Fatalf("new header param should be TyPtr, got %v", newParam.Ty)
	}

	// Body should have gained a new block param too.
	if len(body.Params) != origBodyParams+1 {
		t.Fatalf("expected body to gain 1 param, had %d now %d", origBodyParams, len(body.Params))
	}

	// The original ptr_add in body should now be OpMove (dead).
	ptrAddInst := body.Insts[2] // was OpPtrAdd
	if ptrAddInst.Op != OpMove {
		t.Fatalf("expected original ptr_add to become OpMove, got %v", ptrAddInst.Op)
	}

	// The load should now use the body's walking pointer param, not the old ptr_add result.
	loadInst := body.Insts[3]
	if loadInst.Op != OpLoad {
		t.Fatalf("expected load at index 3, got %v", loadInst.Op)
	}
	bodyWalkReg := body.Params[len(body.Params)-1].Dst
	if loadInst.Src[0] != bodyWalkReg {
		t.Fatalf("expected load to use body walk ptr %%r%d, got %%r%d", bodyWalkReg, loadInst.Src[0])
	}

	// Latch jump should now pass an advanced pointer to header.
	latchJmp := body.Term.(*TermJmp)
	if len(latchJmp.Args) != 2 { // original index + new pointer
		t.Fatalf("expected latch to pass 2 args, got %d", len(latchJmp.Args))
	}

	// Entry jump should now pass initial pointer too.
	entryJmp := entry.Term.(*TermJmp)
	if len(entryJmp.Args) != 2 {
		t.Fatalf("expected entry to pass 2 args (index + initial ptr), got %d", len(entryJmp.Args))
	}

	// Stats.
	if stats.LoopsThreaded != 1 {
		t.Fatalf("expected 1 loop threaded, got %d", stats.LoopsThreaded)
	}
	if stats.PtrAddsRemoved != 1 {
		t.Fatalf("expected 1 ptr_add removed, got %d", stats.PtrAddsRemoved)
	}

	// Semantic check: for countdown loop (idx 10→9→8...), pointer stride
	// must be negative so ptr moves from base+10 to base+9 (not base+11).
	// The advance instruction should use stride = -1, not +1.
	var advanceInst *Inst
	for _, inst := range body.Insts {
		if inst.Op == OpPtrAdd && inst.Dst != Reg(22) { // not the dead original
			advanceInst = inst
		}
	}
	if advanceInst == nil {
		t.Fatal("expected a ptr_add advance instruction in body")
	}
	// Find the stride const feeding the advance.
	var strideConst *Inst
	for _, inst := range body.Insts {
		if inst.Op == OpConst && inst.Dst == advanceInst.Src[1] {
			strideConst = inst
		}
	}
	if strideConst == nil {
		t.Fatal("expected a const instruction for stride")
	}
	if strideConst.Imm != -1 {
		t.Fatalf("countdown loop: stride should be -1, got %d", strideConst.Imm)
	}
}

// TestPointerThreading_Positive_CountUp: count-up loop (idx = idx + 1).
// Pointer should advance by +1.
func TestPointerThreading_Positive_CountUp(t *testing.T) {
	f := &Func{Name: "countup_test", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 0, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(1)}},
	}

	head := &Block{
		Label: "head",
		Params: []BlockParam{
			{Dst: Reg(10), Ty: TyU8, Class: ClassGeneral},
		},
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(11), Imm: 10, Ty: TyU8},
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
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
			{Op: OpConst, Dst: Reg(25), Imm: 1, Ty: TyU8},
			// Count UP: idx = idx + 1
			{Op: OpAdd, Dst: Reg(26), Src: [2]Reg{Reg(10), Reg(25)}, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(26)}},
	}

	exit := &Block{
		Label: "exit",
		Term:  &TermRet{Vals: nil},
	}

	f.Blocks = []*Block{entry, head, body, exit}

	var stats PointerThreadingStats
	changed := ApplyPointerThreading(f, &stats)
	if !changed {
		t.Fatal("expected pointer threading to fire for count-up loop")
	}
	if stats.LoopsThreaded != 1 {
		t.Fatalf("expected 1 loop threaded, got %d", stats.LoopsThreaded)
	}

	// Semantic check: count-up loop → stride should be +1
	var advanceInst *Inst
	for _, inst := range body.Insts {
		if inst.Op == OpPtrAdd && inst.Dst != Reg(22) {
			advanceInst = inst
		}
	}
	if advanceInst == nil {
		t.Fatal("expected a ptr_add advance instruction in body")
	}
	var strideConst *Inst
	for _, inst := range body.Insts {
		if inst.Op == OpConst && inst.Dst == advanceInst.Src[1] {
			strideConst = inst
		}
	}
	if strideConst == nil {
		t.Fatal("expected a const for stride")
	}
	if strideConst.Imm != 1 {
		t.Fatalf("count-up loop: stride should be +1, got %d", strideConst.Imm)
	}
}

// TestPointerThreading_Positive_U16Access: loop accessing u16 values.
// accessWidthBytes should be detected as 2.
func TestPointerThreading_Positive_U16Access(t *testing.T) {
	f := &Func{Name: "u16_test", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 5, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(1)}},
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
			{Op: OpAddrOf, Dst: Reg(20), Sym: "table", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpExt, Dst: Reg(21), Src: [2]Reg{Reg(10)}, SrcTy: TyU8, Ty: TyU16},
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			// u16 load — accessWidthBytes should be detected as 2
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU16, Cls: ClassPair},
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

	var stats PointerThreadingStats
	changed := ApplyPointerThreading(f, &stats)
	if !changed {
		t.Fatal("expected pointer threading to fire for u16 access loop")
	}

	// The transform should fire — u16 access is detected and recorded.
	// Stride is still -1 (byte offset semantics from original code).
	// accessWidthBytes=2 is informational for this pass.
	if stats.LoopsThreaded != 1 {
		t.Fatalf("expected 1 loop threaded, got %d", stats.LoopsThreaded)
	}

	// Verify the original ptr_add was replaced.
	if body.Insts[2].Op != OpMove {
		t.Fatalf("expected original ptr_add to become OpMove, got %v", body.Insts[2].Op)
	}
}

// TestPointerThreading_Positive_MultiAccess: loop body loads buf[idx] and
// stores to buf[idx+1]. Both should be rewritten to use the walked pointer.
func TestPointerThreading_Positive_MultiAccess(t *testing.T) {
	f := &Func{Name: "multi_access", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 10, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(1)}},
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
			// ptr1 = base + idx (delta=0)
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			// load from buf[idx]
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
			// offset = ext(idx) + 1
			{Op: OpConst, Dst: Reg(30), Imm: 1, Ty: TyU16},
			{Op: OpAdd, Dst: Reg(31), Src: [2]Reg{Reg(21), Reg(30)}, Ty: TyU16},
			// ptr2 = base + (idx + 1) (delta=1)
			{Op: OpPtrAdd, Dst: Reg(32), Src: [2]Reg{Reg(20), Reg(31)}, Ty: TyPtr, Cls: ClassPointer},
			// store to buf[idx+1]
			{Op: OpStore, Src: [2]Reg{Reg(32), Reg(23)}, Ty: TyU8},
			// loop counter
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

	var stats PointerThreadingStats
	changed := ApplyPointerThreading(f, &stats)
	if !changed {
		t.Fatal("expected pointer threading to fire for multi-access loop")
	}
	if stats.LoopsThreaded != 1 {
		t.Fatalf("expected 1 loop threaded, got %d", stats.LoopsThreaded)
	}
	if stats.PtrAddsRemoved != 2 {
		t.Fatalf("expected 2 ptr_adds removed, got %d", stats.PtrAddsRemoved)
	}

	// First ptr_add (delta=0) should become OpMove.
	found0 := false
	for _, inst := range body.Insts {
		if inst.Dst == Reg(22) && inst.Op == OpMove {
			found0 = true
		}
	}
	if !found0 {
		t.Fatal("expected first ptr_add (delta=0) to become OpMove")
	}

	// Second ptr_add (delta=1) should be rewritten to ptr_add(walkPtr, 1).
	bodyWalkReg := body.Params[len(body.Params)-1].Dst
	found1 := false
	for _, inst := range body.Insts {
		if inst.Dst == Reg(32) && inst.Op == OpPtrAdd && inst.Src[0] == bodyWalkReg {
			found1 = true
		}
	}
	if !found1 {
		t.Fatal("expected second ptr_add (delta=1) to be rewritten as ptr_add(walkPtr, 1)")
	}
}

// TestPointerThreading_Negative_MultiBases: two ptr_adds with different bases
// in the same loop — should reject.
func TestPointerThreading_Negative_MultiBases(t *testing.T) {
	f := &Func{Name: "multi_base", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 5, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(1)}},
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
			{Op: OpAddrOf, Dst: Reg(20), Sym: "buf1", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpAddrOf, Dst: Reg(40), Sym: "buf2", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpExt, Dst: Reg(21), Src: [2]Reg{Reg(10)}, SrcTy: TyU8, Ty: TyU16},
			// ptr from buf1
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
			// ptr from buf2 — different base!
			{Op: OpPtrAdd, Dst: Reg(42), Src: [2]Reg{Reg(40), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpStore, Src: [2]Reg{Reg(42), Reg(23)}, Ty: TyU8},
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

	changed := ApplyPointerThreading(f, nil)
	if changed {
		t.Fatal("expected no transform with multiple different bases")
	}
}

// TestPointerThreading_Negative_NoLoop: straight-line code, no loop.
func TestPointerThreading_Negative_NoLoop(t *testing.T) {
	f := &Func{Name: "no_loop", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpAddrOf, Dst: Reg(20), Sym: "buf", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpConst, Dst: Reg(21), Imm: 5, Ty: TyU16},
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
		},
		Term: &TermRet{Vals: []Reg{Reg(23)}},
	}

	f.Blocks = []*Block{entry}

	changed := ApplyPointerThreading(f, nil)
	if changed {
		t.Fatal("expected no transform on straight-line code")
	}
}

// TestPointerThreading_Negative_VariableStride: loop with non-constant stride.
func TestPointerThreading_Negative_VariableStride(t *testing.T) {
	f := &Func{Name: "var_stride", nextReg: 100}

	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpConst, Dst: Reg(1), Imm: 10, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(1)}},
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

	// Variable stride: next = idx - someReg (not constant)
	body := &Block{
		Label: "body",
		Insts: []*Inst{
			{Op: OpAddrOf, Dst: Reg(20), Sym: "buf", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpExt, Dst: Reg(21), Src: [2]Reg{Reg(10)}, SrcTy: TyU8, Ty: TyU16},
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
			// Non-constant stride: sub idx, val (val from load, not const)
			{Op: OpSub, Dst: Reg(26), Src: [2]Reg{Reg(10), Reg(23)}, Ty: TyU8},
		},
		Term: &TermJmp{Target: "head", Args: []Reg{Reg(26)}},
	}

	exit := &Block{
		Label: "exit",
		Term:  &TermRet{Vals: nil},
	}

	f.Blocks = []*Block{entry, head, body, exit}

	changed := ApplyPointerThreading(f, nil)
	if changed {
		t.Fatal("expected no transform with variable stride")
	}
}
