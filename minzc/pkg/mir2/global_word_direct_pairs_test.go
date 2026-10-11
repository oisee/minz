package mir2_test

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func directPairModule(dst string, shared, numeric, successor bool) (*mir2.Module, *mir2.AllocResult) {
	f := &mir2.Func{Name: "read_word", Contract: mir2.Contract{Returns: []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}}}
	addr := &mir2.Inst{Op: mir2.OpAddrOf, Dst: 1, Sym: "word", Ty: mir2.TyPtr}
	if numeric {
		addr = &mir2.Inst{Op: mir2.OpConst, Dst: 1, Imm: 0xc000, Ty: mir2.TyPtr}
	}
	block := &mir2.Block{Label: "entry", Insts: []*mir2.Inst{addr, {Op: mir2.OpLoad, Dst: 2, Src: [2]mir2.Reg{1}, Ty: mir2.TyU16}}, Term: &mir2.TermRet{Vals: []mir2.Reg{2}}}
	f.Blocks = []*mir2.Block{block}
	if shared {
		block.Insts = append(block.Insts, &mir2.Inst{Op: mir2.OpLoad, Dst: 3, Src: [2]mir2.Reg{1}, Ty: mir2.TyU8})
	}
	if successor {
		block.Term = &mir2.TermJmp{Target: "next"}
		f.Blocks = append(f.Blocks, &mir2.Block{Label: "next", Insts: []*mir2.Inst{{Op: mir2.OpLoad, Dst: 3, Src: [2]mir2.Reg{1}, Ty: mir2.TyU8}}, Term: &mir2.TermRet{Vals: []mir2.Reg{2}}})
	}
	m := &mir2.Module{Name: "direct-pair", Funcs: []*mir2.Func{f}, Globals: []mir2.Global{{Name: "word", Ty: mir2.TyU16, Init: []byte{0x34, 0x12}}}}
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{1: {Kind: mir2.LocReg, Name: "HL"}, 2: {Kind: mir2.LocReg, Name: dst}, 3: {Kind: mir2.LocReg, Name: "A"}}}
	return m, ar
}

func TestGlobalWordDirectPairNative(t *testing.T) {
	for _, pair := range []string{"DE", "BC"} {
		for _, value := range []uint16{0, 255, 256, 65535, 0x1234, 0x90ff} {
			t.Run(fmt.Sprintf("%s_%04x", pair, value), func(t *testing.T) {
				m, ar := directPairModule(pair, false, false, false)
				if err := mir2.Verify(m); err != nil {
					t.Fatal(err)
				}
				m.Globals[0].Init = []byte{byte(value), byte(value >> 8)}
				if err := mir2.Verify(m); err != nil {
					t.Fatal(err)
				}
				text, e := mir2.Z80Codegen(m, ar)
				if e != nil {
					t.Fatal(e)
				}
				direct := "LD " + pair + ", (word)"
				if !strings.Contains(text, direct) || strings.Contains(text, "LD HL, word") {
					t.Fatalf("direct matcher not applied\n%s", text)
				}
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a.Errors)
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				if e = z.LoadMemory(0x8000, a.Binary); e != nil {
					t.Fatal(e)
				}
				// Enter precisely at the generated word load. Its direct instruction must
				// preserve A/F/HL and read exactly the two little-endian global bytes.
				pc := uint16(a.Symbols["read_word"])
				symbol := uint16(a.Symbols["word"])
				prof := emulator.NewProfiler()
				z.SetProfiler(prof)
				z.SetRegisters(emulator.Registers{PC: pc, SP: 0xf000, HL: 0x6a53, DE: 0x4567, BC: 0x89ab, A: 0x65, F: 0xa5})
				before := z.GetRegisters()
				ram := make([]byte, 65536)
				for i := range ram {
					ram[i] = z.GetMemory(uint16(i))
				}
				cycles := z.Step()
				after := z.GetRegisters()
				got := after.DE
				if pair == "BC" {
					got = after.BC
				}
				if got != value || after.HL != before.HL || after.A != before.A || after.F != before.F || after.SP != before.SP || cycles != 20 {
					t.Fatalf("load state=%+v cycles%d want%04x", after, cycles, value)
				}
				if pair == "DE" && after.BC != before.BC || pair == "BC" && after.DE != before.DE {
					t.Fatal("other pair clobber")
				}
				for i := range ram {
					if z.GetMemory(uint16(i)) != ram[i] || prof.WriteCount[i] != 0 {
						t.Fatal("load wrote memory", i)
					}
					if i >= int(a.Origin) && i < int(a.Origin)+len(a.Binary) {
						continue
					}
					want := uint32(0)
					if uint16(i) == symbol || uint16(i) == symbol+1 {
						want = 1
					}
					if prof.ReadCount[i] != want {
						t.Fatalf("unexpected data read%04x got%d want%d", i, prof.ReadCount[i], want)
					}
				}
				// Global lies within binary data: separately assert both data byte reads.
				if prof.ReadCount[symbol] != 1 || prof.ReadCount[symbol+1] != 1 {
					t.Fatal("word byte reads", symbol)
				}
				expected := []byte{0xed, 0x5b}
				if pair == "BC" {
					expected[1] = 0x4b
				}
				offset := int(pc) - a.Origin
				if !bytes.Equal(a.Binary[offset:offset+2], expected) {
					t.Fatal("wrong opcode")
				}
			})
		}
	}
}

func TestGlobalWordDirectPairRefusals(t *testing.T) {
	for _, pair := range []string{"DE", "BC"} {
		for _, tc := range []struct {
			name                       string
			shared, numeric, successor bool
		}{{"shared", true, false, false}, {"numeric", false, true, false}, {"successor", false, false, true}} {
			t.Run(pair+"_"+tc.name, func(t *testing.T) {
				m, ar := directPairModule(pair, tc.shared, tc.numeric, tc.successor)
				if err := mir2.Verify(m); err != nil {
					t.Fatal(err)
				}
				text, e := mir2.Z80Codegen(m, ar)
				if e != nil {
					t.Fatal(e)
				}
				if strings.Contains(text, "LD "+pair+", (word)") {
					t.Fatalf("noneligible address optimized\n%s", text)
				}
				if !tc.numeric && !strings.Contains(text, "LD HL, word") {
					t.Fatalf("live address suppressed\n%s", text)
				}
			})
		}
	}
}
