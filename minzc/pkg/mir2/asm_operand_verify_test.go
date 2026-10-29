package mir2

import (
	"strings"
	"testing"
)

// A well-formedness rule for inline-asm operands — "every Asm.Ins/Asm.Outs
// register is defined in the function, or is a param" — is already half in
// Verify, and the other half cannot be added cheaply. These two tests pin both
// facts so the reasoning does not have to be re-derived.

// TestVerifyRejectsDanglingAsmIns: verifyFunc checks every register returned by
// Inst.Uses(), and Uses() returns Asm.Ins for OpAsm, so a dangling asm input is
// already a Verify error. Nothing new needs to be added for the Ins half.
func TestVerifyRejectsDanglingAsmIns(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("probe")
	f.Attrs.HasAsm = true
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	e := f.NewBlock("entry")
	e.Insts = []*Inst{
		{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 7},
		{Op: OpAsm, Ty: TyU8, Cls: ClassAcc, Asm: &AsmExtra{Template: "ADD A, 1", Ins: []Reg{9}, Clobbers: AllClasses}},
	}
	e.Term = &TermRet{Vals: []Reg{1}}
	err := Verify(m)
	if err == nil || !strings.Contains(err.Error(), "use of undefined %r9") {
		t.Fatalf("Verify must reject a dangling Asm.Ins, got %v", err)
	}
}

// TestVerifyCannotRunAfterPreallocCoalesce: the reason the existing Ins rule
// does not catch the PreallocCoalesce bug is ordering, not a missing check.
// PreallocCoalesce merges a loop-head block param into the entry definition, so
// the same register ends up defined twice — which verifyFunc rejects. Re-running
// Verify after the pass would need that invariant weakened, which is not a
// cheap change and would weaken a real correctness check for everything else.
func TestVerifyCannotRunAfterPreallocCoalesce(t *testing.T) {
	m := &Module{Name: "loop_carried"}
	f := m.AddFunc("execute")
	e := f.NewBlock("entry")
	head := f.NewBlock("loop_head1")
	body := f.NewBlock("loop_body2")
	exit := f.NewBlock("loop_exit3")
	e.Insts = []*Inst{
		{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 5},
		{Op: OpConst, Dst: 2, Ty: TyU16, Imm: 0},
	}
	e.Term = &TermJmp{Target: "loop_head1", Args: []Reg{1, 2}}
	// Post-coalescing shape: %r1 is both the entry const and the loop-head param.
	head.Params = []BlockParam{{Dst: 1, Ty: TyU8, Class: ClassGeneral}, {Dst: 2, Ty: TyU16, Class: ClassPointer}}
	head.Insts = []*Inst{
		{Op: OpConst, Dst: 3, Ty: TyU8, Imm: 1},
		{Op: OpAdd, Dst: 4, Ty: TyU8, Src: [2]Reg{1, 3}},
		{Op: OpConst, Dst: 5, Ty: TyU16, Imm: 1},
		{Op: OpAdd, Dst: 6, Ty: TyU16, Src: [2]Reg{2, 5}},
	}
	head.Term = &TermBrIf{Cond: 7, Then: "loop_body2", Else: "loop_exit3", ElseArgs: []Reg{1}}
	body.Insts = []*Inst{{Op: OpAsm, Asm: &AsmExtra{Template: "ADD A, 1", Ins: []Reg{1}, Clobbers: AllClasses}}}
	body.Term = &TermJmp{Target: "loop_head1", Args: []Reg{4, 6}}
	exit.Params = []BlockParam{{Dst: 8, Ty: TyU8, Class: ClassGeneral}}
	exit.Term = &TermRet{Vals: []Reg{8}}
	err := Verify(m)
	if err == nil || !strings.Contains(err.Error(), "duplicate definition of %r1") {
		t.Fatalf("coalesced loop shape must trip the duplicate-definition rule, got %v", err)
	}
}
