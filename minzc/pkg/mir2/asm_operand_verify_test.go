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
