package mir2

import "testing"

func TestCollectSolverFriendlyShapeFacts_IndexedLoopAndRepeatedTerms(t *testing.T) {
	f := &Func{Name: "shape_loop", nextReg: 100}

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
			{Op: OpPtrAdd, Dst: Reg(22), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(23), Src: [2]Reg{Reg(22)}, Ty: TyU8, Cls: ClassAcc},
			{Op: OpPtrAdd, Dst: Reg(24), Src: [2]Reg{Reg(20), Reg(21)}, Ty: TyPtr, Cls: ClassPointer},
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

	facts := CollectSolverFriendlyShapeFacts(f)
	if len(facts.Loops) != 1 {
		t.Fatalf("expected 1 loop region, got %d", len(facts.Loops))
	}
	if facts.Loops[0].Header != "head" || facts.Loops[0].Latch != "body" {
		t.Fatalf("unexpected loop region: %+v", facts.Loops[0])
	}

	if len(facts.IndexedAccesses) != 2 {
		t.Fatalf("expected 2 indexed accesses, got %d", len(facts.IndexedAccesses))
	}
	for _, fact := range facts.IndexedAccesses {
		if fact.BlockLabel != "body" {
			t.Fatalf("indexed access in wrong block: %+v", fact)
		}
		if !fact.InLoop {
			t.Fatalf("expected indexed access to be marked in-loop: %+v", fact)
		}
		if fact.BaseKey == "" || fact.PtrKey == "" {
			t.Fatalf("expected keys for indexed access fact: %+v", fact)
		}
	}

	var sawSameTerm, sawSameBase bool
	for _, fact := range facts.RepeatedAddrTerms {
		if fact.BlockLabel != "body" || fact.Count != 2 || !fact.InLoop {
			t.Fatalf("unexpected repeated-address fact: %+v", fact)
		}
		switch fact.Kind {
		case "same_term":
			sawSameTerm = true
		case "same_base":
			sawSameBase = true
		}
	}
	if !sawSameTerm {
		t.Fatal("expected same_term repeated-address fact")
	}
	if !sawSameBase {
		t.Fatal("expected same_base repeated-address fact")
	}
}

func TestCollectSolverFriendlyShapeFacts_NonLoopIndexedAccess(t *testing.T) {
	f := &Func{Name: "shape_nonloop", nextReg: 100}
	entry := &Block{
		Label: "entry",
		Insts: []*Inst{
			{Op: OpAddrOf, Dst: Reg(1), Sym: "buf", Ty: TyPtr, Cls: ClassPointer},
			{Op: OpExt, Dst: Reg(2), Src: [2]Reg{Reg(3)}, SrcTy: TyU8, Ty: TyU16},
			{Op: OpPtrAdd, Dst: Reg(4), Src: [2]Reg{Reg(1), Reg(2)}, Ty: TyPtr, Cls: ClassPointer},
			{Op: OpLoad, Dst: Reg(5), Src: [2]Reg{Reg(4)}, Ty: TyU8, Cls: ClassAcc},
		},
		Term: &TermRet{Vals: []Reg{Reg(5)}},
	}
	f.Blocks = []*Block{entry}
	f.Contract.Params = []Param{{Name: "i", Reg: Reg(3), Ty: TyU8, Class: ClassGeneral}}

	facts := CollectSolverFriendlyShapeFacts(f)
	if len(facts.Loops) != 0 {
		t.Fatalf("expected no loops, got %+v", facts.Loops)
	}
	if len(facts.IndexedAccesses) != 1 {
		t.Fatalf("expected 1 indexed access, got %d", len(facts.IndexedAccesses))
	}
	if facts.IndexedAccesses[0].InLoop {
		t.Fatalf("expected non-loop indexed access, got %+v", facts.IndexedAccesses[0])
	}
	if len(facts.RepeatedAddrTerms) != 0 {
		t.Fatalf("expected no repeated-address facts, got %+v", facts.RepeatedAddrTerms)
	}
}
