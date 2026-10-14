package vir

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCheckedLiveBaseOffsetPatternsNative(t *testing.T) {
	for _, pair := range []string{"BC", "DE"} {
		for _, offset := range []int{1, 2, 11} {
			t.Run(fmt.Sprintf("%s-plus%d", pair, offset), func(t *testing.T) {
				ops := []VIROp{{Op: OpAddImm, Dst: 2, Src: [2]int{1, -1}, Width: 16, Imm: int64(offset)}, {Op: OpRet, Src: [2]int{2, -1}, Width: 16}}
				assignment := map[int]int{1: Z80.LocByName(pair), 2: Z80.LocByName("HL")}
				cost, e := checkedLocalCost(ops, assignment, Z80, SolverOptions{})
				if e != nil {
					t.Fatal(e)
				}
				assembly, e := emitFromTable(&mir2.Func{Name: "offset"}, &Func{Name: "offset", Blocks: []Block{{Ops: ops}}}, ops, assignment, Z80, SolverOptions{})
				if e != nil {
					t.Fatal(e)
				}
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
				if e != nil {
					t.Fatal(e)
				}
				if len(bin.Errors) > 0 {
					t.Fatal(bin.Errors)
				}
				z := emulator.NewRemogattoZ80()
				memory := z.Memory()
				if e = z.LoadMemory(0x8000, bin.Binary); e != nil {
					t.Fatal(e)
				}
				memory[0xeff0], memory[0xeff1] = 0, 0x70
				unchanged := append([]byte(nil), memory...)
				for base := 0; base < 65536; base++ {
					initial := emulator.Registers{PC: uint16(bin.Symbols["offset"]), SP: 0xeff0, BC: 0x1234, DE: 0x5678, HL: 0xabcd, A: 0xa5, F: byte(base), IX: 0x1357, IY: 0x2468}
					if pair == "BC" {
						initial.BC = uint16(base)
					} else {
						initial.DE = uint16(base)
					}
					z.SetRegisters(initial)
					start := z.GetCycles()
					for step := 0; step < 32 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); step++ {
						z.Step()
					}
					r := z.GetRegisters()
					want := uint16(base + offset)
					flags := initial.F
					if offset == 11 {
						flags = (initial.F & 0xc4) | (byte(want>>8) & 0x28)
						if (base&4095)+offset > 4095 {
							flags |= 0x10
						}
						if base+offset > 65535 {
							flags |= 1
						}
					}
					if r.PC != 0x7000 || r.SP != 0xeff2 || r.HL != want || r.F != flags || r.BC != initial.BC || r.DE != initial.DE || r.A != initial.A || r.IX != initial.IX || r.IY != initial.IY || z.GetCycles()-start != cost {
						t.Fatalf("base%04x got%+v wantHL%04x F%02x T%d/%d", base, r, want, flags, z.GetCycles()-start, cost)
					}
				}
				if !bytes.Equal(memory, unchanged) {
					t.Fatal("address pattern wrote memory")
				}
				t.Logf("65536 bases; independent modulo address/flags, preserved BC/DE/A/IX/IY, PC/SP, exact %d T-state cost", cost)
			})
		}
	}
}
