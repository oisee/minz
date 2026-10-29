package mir2

import "testing"

// asmClobberFixture builds a function whose one value is read after an inline
// asm block that writes A, so the value is live across the block.  The block's
// clobber set is what z80AutoClobbers derives from "ADD A, 1".
func asmClobberFixture(t *testing.T, asmDst Reg) (*Func, ClassSet) {
	t.Helper()
	m := &Module{}
	f := m.AddFunc("asmclob")
	f.Attrs.HasAsm = true
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	var clobbers ClassSet
	clobbers.Add(ClassAcc)
	clobbers.Add(ClassGeneral)
	clobbers.Add(ClassFlag)
	b := f.NewBlock("entry")
	ins := []Reg{1}
	if asmDst != NoReg {
		ins = nil // a block with a result register reads its operands, not mutates them
	}
	b.Insts = []*Inst{
		{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 7},
		{Op: OpAsm, Dst: asmDst, Ty: TyU8, Cls: ClassAcc,
			Asm: &AsmExtra{Template: "ADD A, 1", Ins: ins, Clobbers: clobbers}},
		{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 1},
		{Op: OpAdd, Dst: 3, Ty: TyU8, Src: [2]Reg{1, 2}},
	}
	b.Term = &TermRet{Vals: []Reg{3}}
	if err := Verify(m); err != nil {
		t.Fatalf("fixture must verify: %v", err)
	}
	return f, clobbers
}

// TestAsmClobberExcludesLiveAcrossHomes: a value read after an asm block that
// writes A must not be homed anywhere the block can clobber — the template
// would silently overwrite it.
func TestAsmClobberExcludesLiveAcrossHomes(t *testing.T) {
	f, clobbers := asmClobberFixture(t, 4) // 4 is a fresh result register
	ct := Z80CostTable{}
	ar := PBQPAllocate(f, ComputeLiveness(f), ct)
	loc := ar.Locs[1]
	if locHomesClobbers(ct, clobbers, loc) {
		t.Fatalf("%%r1 live across the asm block was homed in %v %q, which the template writes", loc.Kind, loc.Name)
	}
	if loc.Kind == LocReg {
		t.Fatalf("%%r1 should have been forced out of every register, got %v %q", loc.Kind, loc.Name)
	}
}

// TestAsmClobberInOutInputsExempt: a block with no result register can only
// affect the program through the registers it is handed, so its inputs are
// in-out — the template's write to the input's own register is the result, not
// a clobber of it.  `asm (in x) { RLCA }` keeps x in the accumulator.
func TestAsmClobberInOutInputsExempt(t *testing.T) {
	f, _ := asmClobberFixture(t, NoReg)
	ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
	if loc := ar.Locs[1]; loc.Kind != LocReg {
		t.Fatalf("in-out asm input was driven out of the registers, got %v %q", loc.Kind, loc.Name)
	}
}

// TestLocHomesClobbersIgnoresSpillSlots: stack and memory are the escape hatch
// that keeps a function whose template clobbers everything allocatable, so a
// clobber set must never exclude them.
func TestLocHomesClobbersIgnoresSpillSlots(t *testing.T) {
	ct := Z80CostTable{}
	for _, loc := range ct.Locs() {
		if loc.Kind != LocStack && loc.Kind != LocMem {
			continue
		}
		if locHomesClobbers(ct, AllClasses, loc) {
			t.Fatalf("%v %q must never be treated as clobbered", loc.Kind, loc.Name)
		}
	}
	if !locHomesClobbers(ct, AllClasses, PhysLoc{Kind: LocReg, Name: "A"}) {
		t.Fatal("AllClasses must exclude the accumulator")
	}
}
