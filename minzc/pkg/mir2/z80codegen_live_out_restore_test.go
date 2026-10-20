package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestZ80LiveOutRestoreParallel(t *testing.T) {
	for _, tt := range []struct {
		name, firstHome, firstSaved, secondHome, secondSaved string
		word                                                 bool
	}{
		{"chain", "B", "_spill_saved", "A", "B", false},
		{"cycle", "B", "A", "A", "B", false},
		{"pairHalf", "HL", "DE", "D", "H", true},
		{"indexPair", "IX", "DE", "D", "IXH", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := &Module{Name: "restore"}
			f := m.AddFunc("restore")
			b := NewBuilder(f)
			block := b.SwitchToNewBlock("entry")
			ty := TyU8
			if tt.word {
				ty = TyU16
			}
			x := b.Param("x", ty, ClassGeneral)
			y := b.Param("y", TyU8, ClassGeneral)
			b.Ret(x, y)
			var sb strings.Builder
			g := &z80cg{sb: &sb, fn: f, curBlock: block, ar: &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocReg, Name: tt.firstHome}, y: {Kind: LocReg, Name: tt.secondHome}}}, liveness: ComputeLiveness(f), physOverride: map[Reg]string{x: tt.firstSaved, y: tt.secondSaved}, holdsPhys: map[string]string{}}
			g.restoreLiveOutOverrides(f, block.Term)
			asm := "ORG 0x8000\n" + sb.String() + "DI\nHALT\n_spill_saved: DB 0x53\n"
			res, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(res.Errors) > 0 {
				t.Fatalf("%v %v\n%s", err, res.Errors, asm)
			}
			for flags := 0; flags < 256; flags++ {
				z := emulator.NewRemogattoZ80()
				z.LoadMemory(0x8000, res.Binary)
				initial := emulator.Registers{PC: 0x8000, SP: 0xff00, A: 0x53, F: byte(flags), BC: 0x2709, DE: 0x5319, HL: 0x2711, IX: 0x2712}
				z.SetRegisters(initial)
				for steps := 0; !z.IsHalted(); steps++ {
					if steps > 100 {
						t.Fatal("no halt")
					}
					z.Step()
				}
				got := z.GetRegisters()
				switch tt.name {
				case "chain", "cycle":
					if got.A != 0x27 || byte(got.BC>>8) != 0x53 {
						t.Fatalf("bad restore: %+v\n%s", got, asm)
					}
				case "pairHalf":
					if got.HL != initial.DE || byte(got.DE>>8) != byte(initial.HL>>8) {
						t.Fatalf("bad restore: %+v\n%s", got, asm)
					}
				case "indexPair":
					if got.IX != initial.DE || byte(got.DE>>8) != byte(initial.IX>>8) {
						t.Fatalf("bad restore: %+v\n%s", got, asm)
					}
				}
				if got.F != initial.F || got.SP != initial.SP {
					t.Fatalf("flags/stack changed: %+v\n%s", got, asm)
				}
			}
		})
	}
}
