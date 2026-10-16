package mir2

import (
	"fmt"
	"strings"
	"testing"
)

func TestZ80AggregateAddressCopy(t *testing.T) {
	for _, ty := range []Ty{&StructTy{Name: "Byte", Fields: []StructField{{Name: "v", Ty: TyU8}}}, NewArray(TyU8, 1), &StructTy{Name: "Large", Fields: []StructField{{Name: "v", Ty: TyU32}}}} {
		if NativeSpillWidth(ty) != 2 || ReturnLocation(ClassAcc, ty) != "HL" {
			t.Fatal("aggregate representation", ty)
		}
		for _, pair := range []string{"HL", "DE", "BC", "IX", "IY"} {
			for _, edge := range []bool{false, true} {
				var sb strings.Builder
				f := &Func{Name: "copy", Blocks: []*Block{{Label: "target", Params: []BlockParam{{Dst: 2, Ty: ty}}}}}
				g := &z80cg{fn: f, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: pair}, 2: {Kind: LocMem, Name: "slot", Offset: 0xc100}}}, sb: &sb, holdsPhys: map[string]string{}}
				if edge {
					g.emitParallelCopy(g.buildBlockCopies(f, "target", []Reg{1}))
				} else {
					g.emitCallArgs([]Reg{1}, &Func{Contract: Contract{Params: []Param{{Reg: 2, Ty: ty}}}})
				}
				initial := checksumCopyRegisters()

				// Copy back through the same native representation; both address bytes
				// must survive even for a one-byte object. No layout-size mutation.
				var back strings.Builder
				g.sb = &back
				back.WriteString("LD " + pair + ",0\n")
				if edge {
					g.emitSingleCopy(Z80SpillLabel("copy", 2), pair, ty)
				} else {
					g.emitSingleCopy("$C100", pair, ty)
				}
				asm := sb.String() + back.String() + "JP stop\n" + Z80SpillLabel("copy", 2) + ": DW 0\nstop:\n"
				r := runChecksumCopyAsm(t, asm, initial).GetRegisters()
				if checksumCopyBytes(r)[highByte(pair)] != checksumCopyBytes(initial)[highByte(pair)] || checksumCopyBytes(r)[lowByte(pair)] != checksumCopyBytes(initial)[lowByte(pair)] || r.SP != initial.SP || r.F != initial.F {
					t.Fatalf("address pair %s edge%v ty%v got%+v\n%s", pair, edge, ty, r, asm)
				}
			}
		}
	}
}

func TestZ80AllocaAddressAndLiveRegisters(t *testing.T) {
	for _, dst := range []string{"HL", "DE", "BC", "IX", "IY", "slot"} {
		for _, n := range []int64{1, 2, 17} {
			t.Run(fmt.Sprintf("%s/%d", dst, n), func(t *testing.T) {
				var sb strings.Builder
				f := &Func{Name: "alloca"}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: dst}}}
				if dst == "slot" {
					ar.Locs[1] = PhysLoc{Kind: LocMem, Name: dst}
				}
				g := &z80cg{fn: f, ar: ar, sb: &sb, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}}
				g.genInst(&Inst{Op: OpAlloca, Dst: 1, Ty: TyPtr, Imm: n})
				if dst == "slot" {
					sb.WriteString("LD IX,(" + Z80SpillLabel("alloca", 1) + ")\n")
				}
				sb.WriteString("JP stop\n" + Z80SpillLabel("alloca", 1) + ": DW 0\nstop:\n")
				initial := checksumCopyRegisters()
				r := runChecksumCopyAsm(t, sb.String(), initial).GetRegisters()
				got := map[string]uint16{"HL": r.HL, "DE": r.DE, "BC": r.BC, "IX": r.IX, "IY": r.IY, "slot": r.IX}[dst]
				if got != initial.SP-uint16(n) || r.SP != initial.SP-uint16(n) || r.A != initial.A || r.F != initial.F {
					t.Fatalf("alloca %s got%+v wantaddress%x\n%s", dst, r, initial.SP-uint16(n), sb.String())
				}
				if dst != "HL" && r.HL != initial.HL {
					t.Fatalf("live HL clobbered %x\n%s", r.HL, sb.String())
				}
			})
		}
	}
}

func TestZ80SpilledAddressPreservesLiveHL(t *testing.T) {
	for _, store := range []bool{false, true} {
		for _, value := range []string{"C", "L", "H", "HL"} {
			for _, word := range []bool{false, true} {
				if !store && (value != "C" || word) {
					continue
				}
				if word && value != "HL" && store {
					continue
				}
				var sb strings.Builder
				f := &Func{Name: "access"}
				ty := TyU8
				if word {
					ty = TyU16
				}
				inst := &Inst{Op: OpLoad, Dst: 2, Src: [2]Reg{1}, Ty: ty}
				if store {
					inst = &Inst{Op: OpStore, Src: [2]Reg{1, 2}, Ty: ty}
				}
				block := &Block{Label: "entry", Insts: []*Inst{inst}, Term: &TermRet{Vals: []Reg{3}}}
				f.Blocks = []*Block{block}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocMem, Name: "slot"}, 2: {Kind: LocReg, Name: value}, 3: {Kind: LocReg, Name: "HL"}}}
				g := &z80cg{fn: f, ar: ar, sb: &sb, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, curBlock: block}
				g.genInst(inst)
				sb.WriteString("JP stop\n" + Z80SpillLabel("access", 1) + ": DW 0xc03d\nstop:\n")
				initial := checksumCopyRegisters()
				r := runChecksumCopyAsm(t, sb.String(), initial)
				regs := r.GetRegisters()
				if regs.HL != initial.HL || regs.SP != initial.SP {
					t.Fatalf("live HL store%v value%s word%v got%+v\n%s", store, value, word, regs, sb.String())
				}
				if store {
					want := checksumCopyBytes(initial)[value]
					if value == "HL" {
						want = byte(initial.HL)
					}
					if r.GetMemory(0xc03d) != want || word && r.GetMemory(0xc03e) != byte(initial.HL>>8) {
						t.Fatalf("store value lost %s word%v got%02x/%02x want%02x\n%s", value, word, r.GetMemory(0xc03d), r.GetMemory(0xc03e), want, sb.String())
					}
				} else if byte(regs.BC) != 0xe9 {
					t.Fatalf("load value lost\n%s", sb.String())
				}
			}
		}
	}
}

func TestZ80SpilledWordResultPreservesLiveHL(t *testing.T) {
	for _, ptr := range []string{"DE", "BC"} {
		for _, value := range []uint16{0, 1, 255, 256, 32768, 65535} {
			var sb strings.Builder
			inst := &Inst{Op: OpLoad, Dst: 2, Src: [2]Reg{1}, Ty: TyU16}
			block := &Block{Label: "entry", Insts: []*Inst{inst}, Term: &TermRet{Vals: []Reg{3}}}
			f := &Func{Name: "spill_word", Blocks: []*Block{block}}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: ptr}, 2: {Kind: LocMem, Name: "slot"}, 3: {Kind: LocReg, Name: "HL"}}}
			g := &z80cg{fn: f, ar: ar, sb: &sb, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, curBlock: block}
			g.genInst(inst)
			sb.WriteString("LD IX,(" + Z80SpillLabel(f.Name, 2) + ")\nJP stop\n" + Z80SpillLabel(f.Name, 2) + ": DW 0\nstop:\n")
			initial := checksumCopyRegisters()
			if ptr == "DE" {
				initial.DE = 0xc03d
			} else {
				initial.BC = 0xc03d
			}
			prefix := fmt.Sprintf("PUSH AF\nLD A,%d\nLD (0xc03d),A\nLD A,%d\nLD (0xc03e),A\nPOP AF\n", byte(value), byte(value>>8))
			r := runChecksumCopyAsm(t, prefix+sb.String(), initial)
			regs := r.GetRegisters()
			if regs.IX != value || regs.HL != initial.HL || regs.SP != initial.SP || regs.A != initial.A || regs.F != initial.F || regs.DE != initial.DE || regs.BC != initial.BC {
				t.Fatalf("scratch changed ptr%s value%x got%+v want%+v\n%s", ptr, value, regs, initial, sb.String())
			}
		}
	}
}
