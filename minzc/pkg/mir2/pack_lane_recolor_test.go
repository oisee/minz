package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// packSwapModule is `ret pack_bytes lo, hi` with lo/hi in D/E (crossed for a
// DE result) and, when keepA, a third byte live in A across the pack and
// returned in A.  It returns the module and its forced allocation.
func packSwapModule(keepA bool) (*Module, *AllocResult) {
	m := &Module{}
	f := m.AddFunc("pack_swap")
	b := f.NewBlock("entry")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassRegD}, {Reg: 2, Ty: TyU8, Class: ClassRegE}}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassIndex}}
	b.Insts = []*Inst{{Op: OpPackBytes, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}}
	b.Term = &TermRet{Vals: []Reg{3}}
	locs := map[Reg]PhysLoc{1: {Name: "D"}, 2: {Name: "E"}, 3: {Name: "DE"}}
	if keepA {
		f.Contract.Params = append(f.Contract.Params, Param{Reg: 4, Ty: TyU8, Class: ClassAcc})
		f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU8, Class: ClassAcc})
		b.Term = &TermRet{Vals: []Reg{3, 4}}
		locs[4] = PhysLoc{Name: "A"}
	}
	return m, &AllocResult{Locs: locs}
}

// The crossed-lane fallback of genPackBytes swaps D and E through A.  A is
// saved only when a value is live in it after the pack.
func TestPackBytesCrossedLaneSwapNative(t *testing.T) {
	for _, keepA := range []bool{false, true} {
		m, ar := packSwapModule(keepA)
		asm, err := Z80Codegen(m, ar)
		if err != nil {
			t.Fatal(err)
		}
		if saves := strings.Contains(asm, "PUSH AF"); saves != keepA {
			t.Fatalf("keepA=%v: PUSH AF present=%v\n%s", keepA, saves, asm)
		}
		bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL pack_swap\nDI\nHALT\n" + asm)
		if err != nil || len(bin.Errors) > 0 {
			t.Fatal(err, bin.Errors)
		}
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(0x8000, bin.Binary)
		for x := 0; x < 256; x++ {
			lo, hi, a := byte(x), byte(x*37+11), byte(255-x)
			z.Unhalt()
			z.SetRegisters(emulator.Registers{A: a, F: byte(x), DE: uint16(lo)<<8 | uint16(hi), SP: 0xf000, PC: 0x8000})
			for i := 0; i < 200 && !z.IsHalted(); i++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.DE != uint16(hi)<<8|uint16(lo) || r.SP != 0xf000 || keepA && r.A != a {
				t.Fatalf("keepA=%v x=%d: DE=%04x A=%02x\n%s", keepA, x, r.DE, r.A, asm)
			}
		}
	}
}

// The allocator side: a crossed pair is recoloured into its lanes when no
// neighbour holds them.
func TestRecolorPackLanesUncrosses(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("p")
	b := f.NewBlock("entry")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyPtr, Class: ClassPointer}}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassIndex}}
	b.Insts = []*Inst{
		{Op: OpLoad, Dst: 2, Src: [2]Reg{1}, Ty: TyU8},
		{Op: OpField, Dst: 3, Src: [2]Reg{1}, Imm: 1, Ty: TyPtr, Cls: ClassPointer},
		{Op: OpLoad, Dst: 4, Src: [2]Reg{3}, Ty: TyU8},
		{Op: OpPackBytes, Dst: 5, Src: [2]Reg{2, 4}, Ty: TyU16},
	}
	b.Term = &TermRet{Vals: []Reg{5}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: "HL"}, 2: {Name: "D"}, 3: {Name: "HL"}, 4: {Name: "E"}, 5: {Name: "DE"}}}
	recolorPackLanes(f, ar, ComputeLiveness(f), Z80CostTable{})
	if ar.Locs[2].Name != "E" || ar.Locs[4].Name != "D" {
		t.Fatalf("lanes not uncrossed: lo=%s hi=%s", ar.Locs[2].Name, ar.Locs[4].Name)
	}
}
