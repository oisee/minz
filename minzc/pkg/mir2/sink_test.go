package mir2

import "testing"

// entry: %3 = add %1, %2 ; br_if %c, then, else.  The add moves into `then`
// only when `then` alone reads it and has entry as its only predecessor.
func sinkFixture(thenPreds int, useInElse, useInEntry bool) (*Func, *Inst) {
	add := &Inst{Op: OpAdd, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8}
	cmp := &Inst{Op: OpCmp, Dst: 4, Src: [2]Reg{1, 2}, Ty: TyBool, SrcTy: TyU8, Cond: CmpUge}
	entry := &Block{Label: "entry", Insts: []*Inst{add, cmp}, Term: &TermBrIf{Cond: 4, Then: "then", Else: "else"}}
	then := &Block{Label: "then", Insts: []*Inst{{Op: OpCmp, Dst: 5, Src: [2]Reg{1, 3}, Ty: TyBool, SrcTy: TyU8, Cond: CmpUle}}, Term: &TermRet{Vals: []Reg{5}}}
	elseB := &Block{Label: "else", Term: &TermRet{Vals: []Reg{4}}}
	if thenPreds > 1 {
		elseB.Term = &TermJmp{Target: "then"}
	}
	if useInElse {
		elseB.Term = &TermRet{Vals: []Reg{3}}
	}
	if useInEntry {
		entry.Insts = append(entry.Insts, &Inst{Op: OpMove, Dst: 6, Src: [2]Reg{3}, Ty: TyU8})
	}
	return &Func{Name: "f", Blocks: []*Block{entry, then, elseB}}, add
}

func TestSinkIntoSingleUseSuccessor(t *testing.T) {
	f, add := sinkFixture(1, false, false)
	if !SinkIntoSingleUseSuccessor(f) {
		t.Fatal("no sink")
	}
	if len(f.Blocks[0].Insts) != 1 || f.Blocks[1].Insts[0] != add {
		t.Fatalf("add not moved to the head of then:\n%s", (&Module{Funcs: []*Func{f}}).Dump())
	}
	for name, fx := range map[string][3]bool{"two-preds": {true, false, false}, "used-in-else": {false, true, false}, "used-in-entry": {false, false, true}} {
		preds := 1
		if fx[0] {
			preds = 2
		}
		f, add := sinkFixture(preds, fx[1], fx[2])
		SinkIntoSingleUseSuccessor(f)
		if f.Blocks[0].Insts[0] != add {
			t.Fatalf("%s: add moved:\n%s", name, (&Module{Funcs: []*Func{f}}).Dump())
		}
	}
}

// A chain sinks together and keeps its order: const k feeds only the add.
func TestSinkIntoSingleUseSuccessorChain(t *testing.T) {
	f, add := sinkFixture(1, false, false)
	k := &Inst{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 31}
	add.Src[1] = 7
	k.Dst = 7
	f.Blocks[0].Insts = append([]*Inst{k}, f.Blocks[0].Insts...)
	SinkIntoSingleUseSuccessor(f)
	then := f.Blocks[1].Insts
	if len(then) != 3 || then[0] != k || then[1] != add {
		t.Fatalf("chain not sunk in order:\n%s", (&Module{Funcs: []*Func{f}}).Dump())
	}
}

// A loop header has the latch as a second predecessor: never sink into it.
// A block that branches to itself is excluded too.
func TestSinkIntoSingleUseSuccessorLoop(t *testing.T) {
	add := &Inst{Op: OpAdd, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8}
	entry := &Block{Label: "entry", Insts: []*Inst{add}, Term: &TermBrIf{Cond: 1, Then: "loop", Else: "exit"}}
	loop := &Block{Label: "loop", Insts: []*Inst{{Op: OpMove, Dst: 4, Src: [2]Reg{3}, Ty: TyU8}}, Term: &TermBrIf{Cond: 1, Then: "loop", Else: "exit"}}
	exit := &Block{Label: "exit", Term: &TermRet{}}
	f := &Func{Name: "f", Blocks: []*Block{entry, loop, exit}}
	if SinkIntoSingleUseSuccessor(f) || entry.Insts[0] != add {
		t.Fatalf("sunk into a loop:\n%s", (&Module{Funcs: []*Func{f}}).Dump())
	}
}
