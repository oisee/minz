package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestNativeCallLiteralCycleExhaustive(t *testing.T) {
	for _, word := range []bool{false, true} {
		ty := TyU8
		src0, src1 := "A", "C"
		if word {
			ty = TyU16
			src0, src1 = "HL", "DE"
		}
		for _, live := range []bool{false, true} {
			for literal := 0; literal < 256; literal++ {
				call := &Inst{Op: OpCall, Sym: "callee", Args: []Reg{1, 2}}
				block := &Block{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: ty, Imm: int64(literal)}, call}, Term: &TermRet{}}
				if live {
					block.Term = &TermRet{Vals: []Reg{1}}
				}
				caller := &Func{Name: "caller", Blocks: []*Block{block}}
				callee := &Func{Name: "callee", Contract: Contract{Params: []Param{{Reg: 3, Ty: ty}, {Reg: 4, Ty: ty}}}}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: src0}, 2: {Kind: LocReg, Name: src1}, 3: {Kind: LocReg, Name: src1}, 4: {Kind: LocReg, Name: src0}}}
				var sb strings.Builder
				g := &z80cg{fn: caller, curBlock: block, ar: ar, sb: &sb, holdsPhys: map[string]string{}}
				g.emitCallArgsForInst(call.Args, callee, call)
				asm := sb.String()
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm + "CALL callee\nDI\nHALT\ncallee:\nRET\n")
				if e != nil || len(bin.Errors) > 0 {
					t.Fatal(e, bin.Errors)
				}
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				z.LoadMemory(0x8000, bin.Binary)
				for x := 0; x < 256; x++ {
					r := checksumCopyRegisters()
					r.F = byte(x)
					if word {
						r.HL = uint16(literal)
						r.DE = uint16(x) * 257
					} else {
						r.A = byte(literal)
						r.BC = r.BC&0xff00 | uint16(x)
					}
					z.Unhalt()
					z.SetRegisters(r)
					for i := 0; i < 100 && !z.IsHalted(); i++ {
						z.Step()
					}
					got := z.GetRegisters()
					if got.SP != r.SP || got.F != r.F || got.IX != r.IX || got.IY != r.IY {
						t.Fatalf("stack/flags/IX word%v live%v %+v asm%s", word, live, got, asm)
					}
					if word {
						if got.HL != r.DE || got.DE != r.HL || got.A != r.A || got.BC != r.BC {
							t.Fatal("word parallel semantics", got, r)
						}
					} else {
						if got.A != byte(r.BC) || byte(got.BC) != r.A || got.BC>>8 != r.BC>>8 || got.DE != r.DE || got.HL != r.HL {
							t.Fatal("byte parallel semantics", got, r)
						}
					}
				}
				if live && strings.Contains(asm, "LD "+src1+", "+fmt.Sprint(literal)) {
					t.Fatal("live literal rematerialized", asm)
				}
			}
		}
	}
}

func TestCallLiteralRefusesNoncanonicalDefinitions(t *testing.T) {
	for _, kind := range []string{"param", "blockparam", "duplicate", "wrongtype", "aftercall", "indirect"} {
		def := &Inst{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 71}
		call := &Inst{Op: OpCall, Args: []Reg{1}}
		b := &Block{Label: "entry", Insts: []*Inst{def, call}, Term: &TermRet{}}
		f := &Func{Name: "test", Blocks: []*Block{b}}
		switch kind {
		case "param":
			f.Contract.Params = []Param{{Reg: 1, Ty: TyU8}}
		case "blockparam":
			b.Params = []BlockParam{{Dst: 1, Ty: TyU8}}
		case "duplicate":
			b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 99})
		case "wrongtype":
			def.Ty = TyI8
		case "aftercall":
			b.Insts = []*Inst{call, def}
		case "indirect":
			call.Op = OpCallIndirect
		}
		g := &z80cg{fn: f, curBlock: b}
		if _, ok := g.callArgumentLiteral(1, TyU8, "C", call); ok {
			t.Fatal("admitted", kind)
		}
	}
}

func TestNativeCallLiteralMixedHalfOrdering(t *testing.T) {
	// Literal A->C belongs to a dependency cycle through BC->HL, H->A.
	// Mixed pair/half overlap retains original source bytes before immediates.
	for x := 0; x < 256; x++ {
		def := &Inst{Op: OpConst, Dst: 1, Ty: TyU8, Imm: int64(x)}
		call := &Inst{Op: OpCall, Args: []Reg{1, 2, 3, 4}}
		b := &Block{Label: "entry", Insts: []*Inst{def, call}, Term: &TermRet{}}
		f := &Func{Name: "caller", Blocks: []*Block{b}}
		callee := &Func{Name: "callee", Contract: Contract{Params: []Param{{Reg: 5, Ty: TyU8}, {Reg: 6, Ty: TyU16}, {Reg: 7, Ty: TyU8}, {Reg: 8, Ty: TyU8}}}}
		ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "A"}, 2: {Kind: LocReg, Name: "BC"}, 3: {Kind: LocReg, Name: "H"}, 4: {Kind: LocReg, Name: "D"}, 5: {Kind: LocReg, Name: "C"}, 6: {Kind: LocReg, Name: "HL"}, 7: {Kind: LocReg, Name: "A"}, 8: {Kind: LocReg, Name: "D"}}}
		var sb strings.Builder
		g := &z80cg{fn: f, curBlock: b, ar: ar, sb: &sb, holdsPhys: map[string]string{}}
		g.emitCallArgsForInst(call.Args, callee, call)
		r := checksumCopyRegisters()
		r.A = byte(x)
		got := runChecksumCopyAsm(t, sb.String(), r).GetRegisters()
		if got.A != byte(r.HL>>8) || got.BC != (r.BC&0xff00|uint16(x)) || got.HL != r.BC || got.DE != r.DE || got.F != r.F || got.SP != r.SP {
			t.Fatalf("mixed literal ordering x%d %+v asm%s", x, got, sb.String())
		}
	}
}
