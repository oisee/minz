package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func wordFieldModule(pair string, offset int64, shared bool) (*mir2.Module, *mir2.AllocResult) {
	m, ar := directPairModule(pair, false, false, false)
	m.Globals[0].Ty = &mir2.ArrayTy{Elem: mir2.TyU16, Len: 8}
	m.Globals[0].Init = make([]byte, 16)
	block := m.Funcs[0].Blocks[0]
	load := block.Insts[1]
	load.Src[0] = 4
	block.Insts = []*mir2.Inst{block.Insts[0], {Op: mir2.OpField, Dst: 4, Src: [2]mir2.Reg{1}, Imm: offset, Ty: mir2.TyPtr}, load}
	ar.Locs[4] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
	if shared {
		block.Insts = append(block.Insts, &mir2.Inst{Op: mir2.OpLoad, Dst: 3, Src: [2]mir2.Reg{4}, Ty: mir2.TyU8})
	}
	return m, ar
}

func TestGlobalWordLiteralFieldNative(t *testing.T) {
	for _, pair := range []string{"HL", "DE", "BC"} {
		for _, offset := range []int64{0, 2, 14} {
			for _, value := range []uint16{0, 255, 256, 0x1234, 0xffff} {
				t.Run(fmt.Sprintf("%s_%d_%04x", pair, offset, value), func(t *testing.T) {
					m, ar := wordFieldModule(pair, offset, false)
					m.Globals[0].Init[offset] = byte(value)
					m.Globals[0].Init[offset+1] = byte(value >> 8)
					if e := mir2.Verify(m); e != nil {
						t.Fatal(e)
					}
					asm, e := mir2.Z80Codegen(m, ar)
					if e != nil {
						t.Fatal(e)
					}
					if !strings.Contains(asm, "LD "+pair+", (word") || strings.Contains(asm, "preserve word-load scratch") {
						t.Fatalf("literal word load not selected:\n%s", asm)
					}
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
					if e != nil || len(a.Errors) != 0 {
						t.Fatal(e, a.Errors)
					}
					z := emulator.NewRemogattoZ80()
					z.Reset()
					if e = z.LoadMemory(0x8000, a.Binary); e != nil {
						t.Fatal(e)
					}
					pc := uint16(a.Symbols["read_word"])
					addr := uint16(a.Symbols["word"]) + uint16(offset)
					before := emulator.Registers{PC: pc, SP: 0xf000, HL: 0x6a53, DE: 0x4567, BC: 0x89ab, IX: 0x1357, IY: 0x2468, A: 0x65, F: 0xa5}
					z.SetRegisters(before)
					prof := emulator.NewProfiler()
					z.SetProfiler(prof)
					ram := make([]byte, 65536)
					for i := range ram {
						ram[i] = z.GetMemory(uint16(i))
					}
					cycles := z.Step()
					got := z.GetRegisters()
					want := before
					opcodeBytes := uint16(4)
					wantT := 20
					switch pair {
					case "HL":
						want.HL = value
						opcodeBytes = 3
						wantT = 16
					case "DE":
						want.DE = value
					case "BC":
						want.BC = value
					}
					if got.HL != want.HL || got.DE != want.DE || got.BC != want.BC || got.IX != want.IX || got.IY != want.IY || got.A != want.A || got.F != want.F || got.SP != want.SP || got.PC != pc+opcodeBytes || cycles != wantT {
						t.Fatalf("registers %+v expected %+v; cycles %d want %d", got, want, cycles, wantT)
					}
					for i := range ram {
						if ram[i] != z.GetMemory(uint16(i)) || prof.WriteCount[i] != 0 {
							t.Fatalf("write at %04x", i)
						}
						if uint16(i) >= pc && uint16(i) < pc+opcodeBytes {
							continue
						}
						expected := uint32(0)
						if uint16(i) == addr || uint16(i) == addr+1 {
							expected = 1
						}
						if prof.ReadCount[i] != expected {
							t.Fatalf("read %04x got %d want %d", i, prof.ReadCount[i], expected)
						}
					}
				})
			}
		}
	}
}

func TestGlobalWordLiteralFieldSharedAddress(t *testing.T) {
	for _, pair := range []string{"DE", "BC"} {
		for _, successor := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_successor_%v", pair, successor), func(t *testing.T) {
				m, ar := wordFieldModule(pair, 2, true)
				m.Globals[0].Init[2], m.Globals[0].Init[3] = 0xef, 0xbe
				if successor {
					b := m.Funcs[0].Blocks[0]
					read := b.Insts[len(b.Insts)-1]
					b.Insts = b.Insts[:len(b.Insts)-1]
					b.Term = &mir2.TermJmp{Target: "next"}
					m.Funcs[0].Blocks = append(m.Funcs[0].Blocks, &mir2.Block{Label: "next", Insts: []*mir2.Inst{read}, Term: &mir2.TermRet{Vals: []mir2.Reg{2}}})
				}
				if e := mir2.Verify(m); e != nil {
					t.Fatal(e)
				}
				asm, e := mir2.Z80Codegen(m, ar)
				if e != nil {
					t.Fatal(e)
				}
				if strings.Contains(asm, "LD "+pair+", (word") {
					t.Fatalf("shared field address incorrectly skipped:\n%s", asm)
				}
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a.Errors)
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				if e = z.LoadMemory(0x8000, a.Binary); e != nil {
					t.Fatal(e)
				}
				if e = z.LoadMemory(0xf000, []byte{0x00, 0x70}); e != nil {
					t.Fatal(e)
				}
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["read_word"]), SP: 0xf000, A: 0x65, F: 0xa5, BC: 0x89ab, DE: 0x4567, HL: 0x6a53})
				prof := emulator.NewProfiler()
				z.SetProfiler(prof)
				for steps := 0; steps < 100 && z.GetPC() != 0x7000; steps++ {
					z.Step()
				}
				r := z.GetRegisters()
				value := r.DE
				if pair == "BC" {
					value = r.BC
				}
				addr := uint16(a.Symbols["word"]) + 2
				if r.PC != 0x7000 || r.SP != 0xf002 || value != 0xbeef || r.A != 0xef || prof.ReadCount[addr] != 2 || prof.ReadCount[addr+1] != 1 {
					t.Fatalf("live pointer lost: %+v reads %d/%d", r, prof.ReadCount[addr], prof.ReadCount[addr+1])
				}
			})
		}
	}
}
