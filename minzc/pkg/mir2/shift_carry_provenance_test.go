package mir2

import (
	"strings"
	"testing"
)

func TestShiftCarryPhysicalProvenance(t *testing.T) {
	for _, tc := range []struct {
		line      string
		preserves bool
	}{
		{"    LD B, A", true}, {"    INC B", true}, {"    DEC HL", true},
		{"    PUSH HL", true}, {"    POP HL", true}, {"    BIT 7, C", true},
		{"    ADD HL, DE", false}, {"    XOR A", false}, {"    SRL B", false},
		{"    POP AF", false}, {"    EX AF, AF'", false}, {"    opaque", false},
	} {
		t.Run(tc.line, func(t *testing.T) {
			i := &Inst{Op: OpShl}
			g := &z80cg{sb: &strings.Builder{}, carryFrom: i}
			g.emit(tc.line)
			if (g.carryFrom == i) != tc.preserves {
				t.Fatal("incorrect carry provenance", tc.line)
			}
		})
	}
}

func TestShiftCarryRejectsCoalescedVariableCount(t *testing.T) {
	_, f := shiftTriangleFixture()
	if !PrepareShiftCarry(f) {
		t.Fatal("fixture did not prepare")
	}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
	var shift *Inst
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			ar.Locs[i.Dst] = PhysLoc{Kind: LocReg, Name: "C"}
			if i.Op == OpShr {
				shift = i
			}
		}
	}
	g := &z80cg{fn: f, ar: ar, regInfo: collectRegInfo(f), bitCmpPat: map[Reg]bitCmpPat{}, bitCmpSkip: map[Reg]bool{}, blockParamRegs: map[Reg]bool{}}
	g.scanShiftCarryPattern(f.Entry())
	if g.shiftCarryInst != shift {
		t.Fatal("ordinary one-bit shift not selected")
	}
	// genInst deliberately does not put a coalesced parameter in constVals.
	// The shift then uses the variable-count emitter, whose flags cannot be
	// claimed by the constant-one-bit fusion even if one definition is Const(1).
	g.blockParamRegs[shift.Src[1]] = true
	g.scanShiftCarryPattern(f.Entry())
	if g.shiftCarryInst != nil {
		t.Fatal("claimed variable-count carry")
	}
}
