package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestBytePackImplicitConditionNative(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("pack_condition")
	b := f.NewBlock("entry")
	yes := f.NewBlock("yes")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassRegC}, {Reg: 2, Ty: TyU8, Class: ClassRegE}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	b.Insts = []*Inst{{Op: OpExt, Dst: 3, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyU16}, {Op: OpExt, Dst: 4, Src: [2]Reg{2}, SrcTy: TyU8, Ty: TyU16}, {Op: OpConst, Dst: 5, Imm: 8, Ty: TyU16}, {Op: OpShl, Dst: 6, Src: [2]Reg{4, 5}, Ty: TyU16}, {Op: OpOr, Dst: 7, Src: [2]Reg{6, 3}, Ty: TyU16}, {Op: OpConst, Dst: 8, Imm: 0, Ty: TyU8}}
	b.Term = &TermCondRet{Cond: 7, Vals: []Reg{8}, Then: "yes"}
	yes.Insts = []*Inst{{Op: OpConst, Dst: 9, Imm: 1, Ty: TyU8}}
	yes.Term = &TermRet{Vals: []Reg{9}}
	a := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: "C"}, 2: {Name: "E"}, 3: {Name: "BC"}, 4: {Name: "HL"}, 5: {Name: "DE"}, 6: {Name: "HL"}, 7: {Name: "HL"}, 8: {Name: "A"}, 9: {Name: "A"}}}
	asm, err := Z80Codegen(m, a)
	if err != nil {
		t.Fatal(err)
	}
	bin, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatal(err, bin.Errors)
	}
	address := bin.Symbols["pack_condition"]
	if address == 0 {
		address = bin.Symbols["PACK_CONDITION"]
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	z.LoadMemory(uint16(bin.Origin), bin.Binary)
	boot := []byte{0xaf, 0x0e, 1, 0x1e, 0, 0xcd, byte(address), byte(address >> 8), 0xf3, 0x76}
	z.LoadMemory(0xf100, boot)
	z.SetPC(0xf100)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if r := z.GetRegisters(); r.A != 1 || r.SP != 0xf000 {
		t.Fatalf("packed implicit condition result%d want1 SP%04x ASM%s", r.A, r.SP, asm)
	}
	t.Log("baseline-preserving refusal ASM", asm)
	if strings.Contains(asm, "pack disjoint byte lanes") {
		t.Fatal("implicit flag consumer not refused")
	}
}

func TestBytePackImplicitFlagForwarding(t *testing.T) {
	for _, kind := range []string{"move-condret", "edge-brif", "flag-return", "brif2-explicit"} {
		t.Run(kind, func(t *testing.T) {
			f := (&Module{}).AddFunc("forward_flags")
			b := f.NewBlock("entry")
			next := f.NewBlock("next")
			switch kind {
			case "move-condret":
				b.Insts = []*Inst{{Op: OpMove, Dst: 2, Src: [2]Reg{1}, Ty: TyU16}}
				b.Term = &TermCondRet{Cond: 2, Then: "next"}
			case "edge-brif":
				b.Term = &TermJmp{Target: "next", Args: []Reg{1}}
				next.Params = []BlockParam{{Dst: 2, Ty: TyU16}}
				next.Term = &TermBrIf{Cond: 2, Then: "next", Else: "next"}
			case "flag-return":
				f.Contract.Returns = []Return{{Ty: TyBool, Class: ClassFlag}}
				b.Term = &TermRet{Vals: []Reg{1}}
			case "brif2-explicit":
				b.Term = &TermBrIf2{Lhs: 1, Rhs: 2, Eq: "next", Lt: "next", Gt: "next"}
			}
			got := bytePackImplicitFlagUse(f, 1)
			want := kind != "brif2-explicit"
			if got != want {
				t.Fatal("implicit use", got, want)
			}
		})
	}
}

func TestBytePackUsesCurrentOverridesWithoutAccScratch(t *testing.T) {
	f := (&Module{}).AddFunc("current_locations")
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpExt, Dst: 2, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyU16}, {Op: OpConst, Dst: 3, Imm: 8, Ty: TyU16}, {Op: OpShl, Dst: 4, Src: [2]Reg{2, 3}, Ty: TyU16}, {Op: OpExt, Dst: 6, Src: [2]Reg{5}, SrcTy: TyU8, Ty: TyU16}, {Op: OpOr, Dst: 7, Src: [2]Reg{4, 6}, Ty: TyU16}}
	b.Term = &TermRet{Vals: []Reg{7, 8}}
	a := &AllocResult{Locs: map[Reg]PhysLoc{4: {Name: "HL"}, 6: {Name: "BC"}, 7: {Name: "HL"}, 8: {Name: "A"}}}
	for _, kind := range []string{"pending-acc", "relocated-low", "relocated-high", "spill-low"} {
		t.Run(kind, func(t *testing.T) {
			var out strings.Builder
			g := &z80cg{fn: f, curBlock: b, ar: a, sb: &out, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, pendingAccReg: 8}
			switch kind {
			case "relocated-low":
				g.physOverride[6] = "DE"
			case "relocated-high":
				g.physOverride[4] = "DE"
			case "spill-low":
				g.physOverride[6] = "_spill_current_locations_r6"
			}
			got := g.genDisjointByteOr(b.Insts[4])
			want := kind == "pending-acc" || kind == "relocated-low"
			if got != want {
				t.Fatal("current location admission", got, want)
			}
			if !got {
				return
			}
			asm := "ORG 32768\nLD HL,4608\nLD BC,52\nLD DE,86\nLD A,99\n" + out.String() + "RET\n"
			bin, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(bin.Errors) > 0 {
				t.Fatal(err, bin.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 1000
			z.LoadMemory(0x8000, bin.Binary)
			z.LoadMemory(0xf100, []byte{0xcd, 0, 0x80, 0xf3, 0x76})
			z.SetPC(0xf100)
			z.SetSP(0xf000)
			if err := z.Run(); err != nil {
				t.Fatal(err)
			}
			r := z.GetRegisters()
			value := uint16(0x1234)
			if kind == "relocated-low" {
				value = 0x1256
			}
			if r.HL != value || r.A != 99 || r.SP != 0xf000 || g.pendingAccReg != 8 {
				t.Fatalf("scratch/override failure%+v ASM%s", r, asm)
			}
		})
	}
}
