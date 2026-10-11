package mir2

import "testing"

func TestRenumberRegsKeepsAsmInputOutputIdentity(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("asm_identity")
	in := f.AllocReg()
	out := f.AllocReg()
	f.Contract.Params = []Param{{Name: "x", Reg: in, Ty: TyU8, Class: ClassAcc}}
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpAsm, Ty: TyVoid, Asm: &AsmExtra{Template: "LD B,A", Ins: []Reg{in}, Outs: []Reg{out}}}}
	b.Term = &TermRet{Vals: []Reg{out}}
	// Force nonidentity IDs; output exists only in ASM and the terminator.
	f.Contract.Params[0].Reg = 70
	b.Insts[0].Asm.Ins[0] = 70
	b.Insts[0].Asm.Outs[0] = 90
	b.Term = &TermRet{Vals: []Reg{90}}
	m.RenumberRegs()
	if f.Contract.Params[0].Reg != b.Insts[0].Asm.Ins[0] || b.Insts[0].Asm.Outs[0] != b.Term.(*TermRet).Vals[0] || b.Insts[0].Asm.Ins[0] == 70 || b.Insts[0].Asm.Outs[0] == 90 {
		t.Fatal("ASM register identities stale", f.Contract, b.Insts[0].Asm, b.Term)
	}
}
