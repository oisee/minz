package mir2

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestNativeConstantWideShiftBanks(t *testing.T) {
	vals := []uint32{0, 1, 0xffff, 0x10000, 0x80000000, 0xffffffff, 0x12345678}
	cases := [][3]string{{"HL", "DE", "HL"}, {"HL", "DE", "DE"}, {"DE", "BC", "HL"}, {"BC", "HL", "DE"}, {"HL", "HL", "HL"}}
	for _, count := range []int64{0, 1, 7, 16, 31, 32} {
		for _, c := range cases {
			for _, a := range vals {
				for _, b := range vals {
					t.Run(fmt.Sprintf("%d/%s-%s-%s/%x-%x", count, c[0], c[1], c[2], a, b), func(t *testing.T) {
						if c[0] == c[1] {
							b = a
						}
						var sb strings.Builder
						g := &z80cg{sb: &sb, physOverride: map[Reg]string{}}
						setup := map[string]uint32{"HL": 0x9abc5678, "DE": 0x24681357, "BC": 0xabcdef01}
						setup[c[0]] = a
						setup[c[1]] = b
						for pair, v := range setup {
							g.emitf("LD %s,%d", pair, v&65535)
							g.emit("EXX")
							g.emitf("LD %s,%d", pair, v>>16)
							g.emit("EXX")
						}
						g.emit("LD A,0x5a")
						g.emit("SCF")
						g.emit("PUSH AF")
						g.emit("POP IX")
						g.ar = &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocDWord, Name: c[0]}, 2: {Kind: LocDWord, Name: c[2]}}}
						g.constVals = map[Reg]int64{3: count}
						g.genShift32("SLA", &Inst{Dst: 2, Src: [2]Reg{1, 3}, Ty: TyU32})

						pairs := []string{"HL", "DE", "BC"}
						for i, pair := range pairs {
							g.emitf("LD (0x%x),%s", 0x6100+i*4, pair)
							g.emit("EXX")
							g.emitf("LD (0x%x),%s", 0x6102+i*4, pair)
							g.emit("EXX")
						}
						g.emit("DI")
						g.emit("HALT")
						bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + sb.String())
						if e != nil || len(bin.Errors) > 0 {
							t.Fatal(e, bin.Errors, sb.String())
						}
						z := emulator.NewRemogattoZ80()
						z.LoadMemory(0x8000, bin.Binary)
						z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xfff0})
						for i := 0; !z.IsHalted() && i < 1000; i++ {
							z.Step()
						}
						r := z.GetRegisters()

						want := a << uint(count)
						for i, pair := range pairs {
							expected := setup[pair]
							if pair == c[2] {
								expected = want
							}
							got := binary.LittleEndian.Uint32(z.Memory()[0x6100+i*4 : 0x6104+i*4])
							if got != expected {
								t.Fatalf("%s got=%08x want=%08x\n%s", pair, got, expected, sb.String())
							}
						}
						if r.A != 0x5a || r.SP != 0xfff0 || !z.IsHalted() {
							t.Fatal(r, sb.String())
						}
					})
				}
			}
		}
	}
}
