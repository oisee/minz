package mir2

import "testing"

// asmOperandUndefined reports every Asm.Ins/Asm.Outs register in f that has no
// definition in the function. Renaming passes that rewrite Src/Args/Dst but not
// the asm operand lists leave exactly this shape behind, and the allocator then
// no longer knows the template reads that value.
func asmOperandUndefined(f *Func) []Reg {
	defs := map[Reg]bool{}
	for _, cp := range f.Contract.Params {
		defs[cp.Reg] = true
	}
	var bad []Reg
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defs[p.Dst] = true
		}
		for _, in := range b.Insts {
			if in.Dst != NoReg {
				defs[in.Dst] = true
			}
			if in.Asm == nil {
				continue
			}
			for _, r := range in.Asm.Ins {
				if r != NoReg && !defs[r] {
					bad = append(bad, r)
				}
			}
			for _, r := range in.Asm.Outs {
				if r != NoReg && !defs[r] {
					bad = append(bad, r)
				}
			}
		}
	}
	return bad
}

// coalesceFixture builds entry→head(param)→body→…, the shape whose entry→head
// forward edge PreallocCoalesce merges. The asm in body reads the head param.
func coalesceFixture(t *testing.T) *Func {
	t.Helper()
	m := &Module{}
	f := m.AddFunc("asmcoalesce")
	f.Attrs.HasAsm = true
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	entry := f.NewBlock("entry")
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	exit := f.NewBlock("exit")
	entry.Insts = []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 41}}
	entry.Term = &TermJmp{Target: "head", Args: []Reg{1}}
	head.Params = []BlockParam{{Dst: 2, Ty: TyU8, Class: ClassGeneral}}
	head.Insts = []*Inst{{Op: OpConst, Dst: 3, Ty: TyU8, Imm: 1}, {Op: OpAdd, Dst: 4, Ty: TyU8, Src: [2]Reg{2, 3}}}
	head.Term = &TermJmp{Target: "body", Args: []Reg{4}}
	body.Params = []BlockParam{{Dst: 5, Ty: TyU8, Class: ClassGeneral}}
	body.Insts = []*Inst{{Op: OpAsm, Ty: TyU8, Cls: ClassAcc, Asm: &AsmExtra{Template: "ADD A, 1", Ins: []Reg{5}, Clobbers: AllClasses}}}
	body.Term = &TermJmp{Target: "exit", Args: []Reg{5}}
	exit.Params = []BlockParam{{Dst: 6, Ty: TyU8, Class: ClassGeneral}}
	exit.Term = &TermRet{Vals: []Reg{6}}
	if err := Verify(m); err != nil {
		t.Fatalf("fixture must verify before the pass: %v", err)
	}
	return f
}

// TestPreallocCoalesceRemapsAsmOperands: the remap loop used to rewrite
// Params/Dst/Src/Args/ExtraRets and terminators to union-find roots but leave
// Asm.Ins/Asm.Outs on the pre-merge register, which no longer has a definition.
func TestPreallocCoalesceRemapsAsmOperands(t *testing.T) {
	f := coalesceFixture(t)
	canon := PreallocCoalesce(f)
	if len(canon) == 0 {
		t.Fatal("fixture must merge at least one block-arg/param pair")
	}
	if bad := asmOperandUndefined(f); len(bad) > 0 {
		t.Fatalf("dangling asm operands after PreallocCoalesce: %v", bad)
	}
	// The remapped operand must still be the value the template reads: the head
	// param's root, not a register that only the pre-merge module defined.
	asm := f.BlockByLabel("body").Insts[0]
	if len(asm.Asm.Ins) != 1 {
		t.Fatalf("asm operand list changed length: %v", asm.Asm.Ins)
	}
	if asm.Asm.Ins[0] != f.BlockByLabel("body").Params[0].Dst {
		t.Fatalf("asm Ins %%r%d is not the block param %%r%d", asm.Asm.Ins[0], f.BlockByLabel("body").Params[0].Dst)
	}
}

// TestPreallocCoalesceRemapsAsmOuts: no frontend populates Asm.Outs today, so
// the Ins test cannot cover it. Same merge, operand on the output list.
func TestPreallocCoalesceRemapsAsmOuts(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("asmouts")
	f.Attrs.HasAsm = true
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	entry := f.NewBlock("entry")
	head := f.NewBlock("head")
	entry.Insts = []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 7}}
	entry.Term = &TermJmp{Target: "head", Args: []Reg{1}}
	head.Params = []BlockParam{{Dst: 2, Ty: TyU8, Class: ClassGeneral}}
	head.Insts = []*Inst{
		{Op: OpAsm, Asm: &AsmExtra{Template: "LD A, 7", Outs: []Reg{2}, Clobbers: AllClasses}},
	}
	head.Term = &TermRet{Vals: []Reg{2}}
	if err := Verify(m); err != nil {
		t.Fatalf("fixture must verify before the pass: %v", err)
	}
	PreallocCoalesce(f)
	if bad := asmOperandUndefined(f); len(bad) > 0 {
		t.Fatalf("dangling asm operands after PreallocCoalesce: %v", bad)
	}
}

// TestPropagateCopiesRemapsAsmIns: PropagateCopies substituted Src/Args and
// terminator uses but not Asm.Ins, so an asm input bound through an inlined
// move kept naming the move's destination while every other use moved on.
// The move itself survives (countRegUses reads Inst.Uses(), which includes
// Asm.Ins), so this is a redundant live value rather than a dangling operand —
// but it leaves the template reading a different virtual register than every
// other consumer of the same source value.
func TestPropagateCopiesRemapsAsmIns(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("asmcopies")
	f.Attrs.HasAsm = true
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	entry := f.NewBlock("entry")
	entry.Insts = []*Inst{
		{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 3},
		{Op: OpMove, Dst: 2, Ty: TyU8, Src: [2]Reg{1, NoReg}},
		{Op: OpAdd, Dst: 3, Ty: TyU8, Src: [2]Reg{2, 1}},
		{Op: OpAsm, Ty: TyU8, Cls: ClassAcc, Asm: &AsmExtra{Template: "ADD A, 1", Ins: []Reg{2}, Clobbers: AllClasses}},
	}
	entry.Term = &TermRet{Vals: []Reg{3}}
	if !PropagateCopies(f) {
		t.Fatal("fixture must contain a propagatable move")
	}
	if got := entry.Insts[2].Src[0]; got != 1 {
		t.Fatalf("ordinary use substituted to %%r1, got %%r%d", got)
	}
	if got := entry.Insts[3].Asm.Ins[0]; got != 1 {
		t.Fatalf("asm use still names the move destination %%r%d, want %%r1", got)
	}
	// With every use substituted, the move really is dead.
	DeadStoreElim(f)
	if bad := asmOperandUndefined(f); len(bad) > 0 {
		t.Fatalf("dangling asm operands after PropagateCopies+DeadStoreElim: %v", bad)
	}
	if len(entry.Insts) != 3 {
		t.Fatalf("move should be dead after substitution: %v", entry.Insts)
	}
}

// TestReplaceRegInBlockRemovesEveryUse: the HIR-Grace versioned-pointer rule
// deletes the defining ptr_add after this rewrite, so any operand class the
// helper skips — call Args and inline-asm inputs both live outside Src — is
// left naming a register with no definition.
func TestReplaceRegInBlockRemovesEveryUse(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("asmreplace")
	f.Attrs.HasAsm = true
	f.Contract.Params = []Param{{Reg: 1, Ty: TyPtr, Class: ClassPointer}, {Reg: 2, Ty: TyU8, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyPtr, Class: ClassPointer}}
	body := f.NewBlock("body")
	// %r9 is the walking pointer the rule threads in as a new block param; it
	// is a definition, so a use left pointing at the deleted ptr_add is the
	// only way this fixture can end up with a dangling operand.
	body.Params = []BlockParam{{Dst: 9, Ty: TyPtr, Class: ClassPointer}}
	body.Insts = []*Inst{
		{Op: OpPtrAdd, Dst: 4, Ty: TyPtr, Src: [2]Reg{1, 2}},
		{Op: OpCall, Sym: "sink", Args: []Reg{4}, Ty: TyVoid},
		{Op: OpAsm, Asm: &AsmExtra{Template: "LD A, (HL)", Ins: []Reg{4}, Clobbers: AllClasses}},
	}
	body.Term = &TermRet{Vals: []Reg{4}}
	replaceRegInBlock(body, 4, 9)
	// Simulate the caller's next step: the definition is removed.
	body.Insts = body.Insts[1:]
	for _, want := range [][]Reg{body.Insts[0].Args, body.Insts[1].Asm.Ins} {
		for _, r := range want {
			if r != 9 {
				t.Fatalf("use not rewritten to the replacement register: %v", r)
			}
		}
	}
	if bad := asmOperandUndefined(f); len(bad) > 0 {
		t.Fatalf("dangling operands after replaceRegInBlock: %v", bad)
	}
}
