package mir2

import "testing"

func TestCPMBDOSAdapterLiteralAuthority(t *testing.T) {
	f := &Func{Name: "console", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8, Class: ClassAcc}}}}
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpAsm, Asm: &AsmExtra{Template: "LD E,A\nLD C,2\nCALL 0x0005", Clobbers: AllClasses}}}
	b.Term = &TermRet{}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: "A"}}}
	if !cpmSourceBDOS2(f, ar) {
		t.Fatal("implicit physical A adapter rejected")
	}
	for _, bad := range []string{"CALL 5+0", "CALL BDOS", "CALL 0x0009", "CALL -5", "CALL 65541", "CALL 5\nCALL 5", "CALL 5 ; extra"} {
		b.Insts[0].Asm.Template = "LD E,A\nLD C,2\n" + bad
		if cpmSourceBDOS2(f, ar) {
			t.Fatal("untrusted literal accepted", bad)
		}
	}
	b.Insts[0].Asm.Template = "LD E,A\nLD C,2\nCALL 0x0005"
	ar.Locs[1] = PhysLoc{Name: "C"}
	if cpmSourceBDOS2(f, ar) {
		t.Fatal("implicit wrong physical input accepted")
	}
}
