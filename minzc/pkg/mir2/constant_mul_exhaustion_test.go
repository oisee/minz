package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

// Width oracle: Nanz Language Book v7 §3.1 u8; multiplication truncates modulo256.
func TestConstantByteMulExhaustedScratchNative(t *testing.T) {
	for _, coeff := range []int{3, 5, 6, 7, 9, 10, 11, 12, 15} {
		for _, keepA := range []bool{false, true} {
			t.Run(fmt.Sprintf("coeff%d-liveA%v", coeff, keepA), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("multiply_busy")
				i := &Inst{Op: OpMul, Dst: 2, Src: [2]Reg{1, 3}, Ty: TyU8}
				b := &Block{Label: "entry", Insts: []*Inst{i}, Term: &TermRet{Vals: []Reg{2, 4, 5, 6, 7, 8}}}
				f.Blocks = []*Block{b}
				a := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: "A"}, 2: {Name: "A"}, 3: {Name: "C"}, 4: {Name: "BC"}, 5: {Name: "DE"}, 6: {Name: "HL"}, 7: {Name: "IX", Kind: LocIXY}, 8: {Name: "IY", Kind: LocIXY}}}
				if keepA {
					a.Locs[2] = PhysLoc{Name: "_spill_mul_result", Kind: LocMem}
					b.Term = &TermRet{Vals: []Reg{1, 2, 4, 5, 6, 7, 8}}
				}
				var out strings.Builder
				g := &z80cg{fn: f, mod: m, curBlock: b, ar: a, sb: &out, constVals: map[Reg]int64{3: int64(coeff)}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
				if g.freeScratch8(i) != "" {
					t.Fatal("not exhausted")
				}
				g.genMul(i)
				if dir := os.Getenv("MINZ_CONSTANT_MUL_ARTIFACTS"); dir != "" {
					os.MkdirAll(dir, 0755)
					if err := os.WriteFile(fmt.Sprintf("%s/coeff%d-liveA%v.a80", dir, coeff, keepA), []byte(out.String()), 0644); err != nil {
						t.Fatal(err)
					}
				}
				asm := "ORG 32768\n" + out.String() + "RET\n_spill_multiply_busy_r1: DB 0\n_spill_multiply_busy_r2: DB 0\n"
				bin, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil || len(bin.Errors) > 0 {
					t.Fatal(err, bin.Errors, asm)
				}
				// Independent reference sequence with D scratch and no surrounding saves.
				refCode := strings.ReplaceAll(out.String(), "    PUSH DE\n", "")
				refCode = strings.ReplaceAll(refCode, "    POP DE\n", "")
				refBin, err := z80asm.NewAssembler().AssembleString("ORG 32768\n" + refCode + "RET\n_spill_multiply_busy_r1: DB 0\n_spill_multiply_busy_r2: DB 0\n")
				if err != nil || len(refBin.Errors) > 0 {
					t.Fatal(err, refBin.Errors)
				}
				for x := 0; x < 256; x++ {
					for _, flags := range []byte{0, 1, 0x40, 0xff} {
						z := emulator.NewRemogattoZ80()
						z.MaxCycles = 1000
						z.LoadMemory(0x8000, bin.Binary)
						z.LoadMemory(0xf100, []byte{0xcd, 0, 0x80, 0xf3, 0x76})
						z.SetRegisters(emulator.Registers{PC: 0xf100, SP: 0xf000, A: byte(x), F: flags, BC: 0x4567, DE: 0x1234, HL: 0x89ab, IX: 0x9abc, IY: 0xabcd})
						if err := z.Run(); err != nil {
							t.Fatal(err)
						}
						r := z.GetRegisters()
						ref := emulator.NewRemogattoZ80()
						ref.MaxCycles = 1000
						ref.LoadMemory(0x8000, refBin.Binary)
						ref.LoadMemory(0xf100, []byte{0xcd, 0, 0x80, 0xf3, 0x76})
						ref.SetRegisters(emulator.Registers{PC: 0xf100, SP: 0xf000, A: byte(x), F: flags, BC: 0x4567, DE: 0x1234, HL: 0x89ab, IX: 0x9abc, IY: 0xabcd})
						if err := ref.Run(); err != nil {
							t.Fatal(err)
						}
						if r.F != ref.GetRegisters().F {
							t.Fatal("sequence output flags changed", r.F, ref.GetRegisters().F)
						}
						want := byte(x * coeff)
						got := r.A
						if keepA {
							address := bin.Symbols["_spill_multiply_busy_r2"]
							if address == 0 {
								address = bin.Symbols["_SPILL_MULTIPLY_BUSY_R2"]
							}
							got = z.GetMemory(uint16(address))
							address = bin.Symbols["_spill_multiply_busy_r1"]
							if address == 0 {
								address = bin.Symbols["_SPILL_MULTIPLY_BUSY_R1"]
							}
							if z.GetMemory(uint16(address)) != byte(x) {
								t.Fatal("liveA lost", x, asm)
							}
						}
						if got != want || r.BC != 0x4567 || r.DE != 0x1234 || r.HL != 0x89ab || r.IX != 0x9abc || r.IY != 0xabcd || r.SP != 0xf000 {
							t.Fatalf("coeff%d x%d F%02x got%02x want%02x regs%+v ASM%s", coeff, x, flags, got, want, r, asm)
						}
					}
				}
			})
		}
	}
}
