package mir2

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestNativeSpilledWordStoresUseTypedWidth(t *testing.T) {
	for _, ty := range []Ty{TyU8, TyU16, TyI16} {
		for _, ptr := range []string{"HL", "DE", "BC", "IX", "IY", "_spill_probe_ptr"} {
			for _, value := range []int64{0, 1, 255, 256, 65535} {
				if ty == TyU8 && value > 255 {
					continue
				}
				t.Run(fmt.Sprintf("%s/%s/%d", ty, ptr, value), func(t *testing.T) {
					m := &Module{}
					f := m.AddFunc("probe")
					f.Contract.Params = []Param{{Name: "p", Reg: 1, Ty: TyPtr, Class: ClassPointer}, {Name: "v", Reg: 2, Ty: ty, Class: ClassGeneral}}
					b := f.NewBlock("entry")
					b.Insts = []*Inst{{Op: OpStore, Ty: TyU16, Src: [2]Reg{1, 2}}}
					b.Term = &TermRet{}
					pl := PhysLoc{Kind: LocReg, Name: ptr}
					if ptr[0] == '_' {
						pl.Kind = LocMem
					}
					ar := &AllocResult{Locs: map[Reg]PhysLoc{1: pl, 2: {Kind: LocMem, Name: "_spill_probe_r2"}}, Spilled: []Reg{2}}
					if pl.Kind == LocMem {
						ar.Locs[1] = PhysLoc{Kind: LocMem, Name: "_spill_probe_r1"}
						ar.Spilled = append(ar.Spilled, 1)
					}
					text, e := Z80Codegen(m, ar)
					if e != nil {
						t.Fatal(e)
					}
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
					if e != nil || len(a.Errors) > 0 {
						t.Fatal(e, a.Errors)
					}
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					mem := z.Memory()
					copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
					copy(mem[0x8000:], a.Binary)
					copy(mem[0x7000:], []byte{0xcd, byte(a.Symbols["probe"]), byte(a.Symbols["probe"] >> 8), 0})
					r := emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357}
					switch ptr {
					case "HL":
						r.HL = 0xc000
					case "DE":
						r.DE = 0xc000
					case "BC":
						r.BC = 0xc000
					case "IX":
						r.IX = 0xc000
					case "IY":
						r.IY = 0xc000
					default:
						p := a.Symbols["_spill_probe_r1"]
						mem[p] = 0
						mem[p+1] = 0xc0
					}
					p := a.Symbols["_spill_probe_r2"]
					mem[p] = byte(value)
					if ty.Width() > 8 {
						mem[p+1] = byte(value >> 8)
					}
					before := append([]byte(nil), mem...)
					z.SetRegisters(r)
					minSP := uint16(0xf000)
					for n := 0; n < 1000 && z.GetPC() != 0x7003; n++ {
						z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
					}
					if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 {
						t.Fatal("return/SP", z.GetRegisters())
					}
					want := append([]byte(nil), before...)
					want[0xc000] = byte(value)
					want[0xc001] = 0
					if ty.Width() > 8 {
						want[0xc001] = byte(value >> 8)
					}
					copy(want[minSP:0xf000], mem[minSP:0xf000])
					if !bytes.Equal(want, mem) {
						for p := range mem {
							if want[p] != mem[p] {
								t.Fatalf("RAM%04x got%x want%x", p, mem[p], want[p])
							}
						}
					}
				})
			}
		}
	}
}
