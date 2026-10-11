package mir2

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestNativeWideVariableShiftFullCount(t *testing.T) {
	values := []uint32{0, 1, 0xffff, 0x10000, 0x80000000, 0xffffffff, 0x12345678}
	counts := []uint32{0, 1, 7, 16, 31, 32, 33, 63, 64, 255, 256, 257, 0x10000, 0x80000000, 0xffffffff}
	for _, resultTy := range []Ty{TyU16, TyU32} {
		for _, mnem := range []string{"SLA", "SRL", "SRA"} {
			for _, countHome := range []string{"BC", "IX", "A", "_spill_shift_count"} {
				countTy := TyU32
				if countHome == "IX" {
					countTy = TyU16
				} else if countHome == "A" {
					countTy = TyU8
				}
				for _, v := range values {
					for _, n := range counts {
						if countTy.Width() < 32 && uint64(n) > ((uint64(1)<<countTy.Width())-1) {
							continue
						}
						t.Run(fmt.Sprintf("%d/%s/%s/%x/%x", resultTy.Width(), mnem, countHome, v, n), func(t *testing.T) {
							var sb strings.Builder
							g := &z80cg{sb: &sb, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocDWord, Name: "DE"}, 2: {Kind: LocDWord, Name: countHome}, 3: {Kind: LocDWord, Name: "HL"}}}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, regInfo: map[Reg]RegInfo{1: {Ty: resultTy}, 2: {Ty: countTy}, 3: {Ty: resultTy}}}
							g.emitf("LD DE,%d", v&65535)
							g.emit("EXX")
							g.emitf("LD DE,%d", v>>16)
							g.emit("LD BC,0x2468")
							g.emit("EXX")
							g.emit("LD A,0x5a")
							switch countHome {
							case "BC":
								g.emitf("LD BC,%d", n&65535)
								g.emit("EXX")
								g.emitf("LD BC,%d", n>>16)
								g.emit("EXX")
							case "IX":
								g.emitf("LD IX,%d", n)
							case "A":
								g.emitf("LD A,%d", n)
							}
							g.genVariableShift(mnem, &Inst{Op: OpShl, Dst: 3, Src: [2]Reg{1, 2}, Ty: resultTy})
							g.emit("LD (0x6100),HL")
							g.emit("LD (0x6104),DE")
							g.emit("LD (0x6108),BC")
							g.emit("EXX")
							g.emit("LD (0x6102),HL")
							g.emit("LD (0x6106),DE")
							g.emit("LD (0x610a),BC")
							g.emit("EXX")
							g.emit("DI")
							g.emit("HALT")
							g.emitf("_spill_shift_count: DB %d,%d,%d,%d", byte(n), byte(n>>8), byte(n>>16), byte(n>>24))
							bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nentry:\n" + sb.String())
							if e != nil {
								t.Fatal(e, sb.String())
							}
							if len(bin.Errors) > 0 {
								t.Fatal(bin.Errors, sb.String())
							}
							z := emulator.NewRemogattoZ80()
							z.LoadMemory(0x8000, bin.Binary)
							z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xfff0})
							for i := 0; !z.IsHalted() && i < 1000; i++ {
								z.Step()
							}
							input := v
							if resultTy.Width() == 16 {
								input &= 65535
							}
							want := input << n
							if mnem == "SRL" {
								want = input >> n
							} else if mnem == "SRA" {
								want = uint32(int32(v) >> n)
								if resultTy.Width() == 16 {
									want = uint32(int16(input) >> n)
								}
							}
							got := binary.LittleEndian.Uint32(z.Memory()[0x6100:0x6104])
							if resultTy.Width() == 16 {
								want &= 65535
								got &= 65535
							}
							original := binary.LittleEndian.Uint32(z.Memory()[0x6104:0x6108])
							r := z.GetRegisters()
							wantA := uint8(0x5a)
							if countHome == "A" {
								wantA = byte(n)
							}
							if got != want || original != v || r.A != wantA || r.SP != 0xfff0 || !z.IsHalted() {
								t.Fatalf("got=%08x want=%08x source=%08x regs=%v\n%s", got, want, original, r, sb.String())
							}
							expectedBC := uint32(0x24680000)
							if countHome == "BC" {
								expectedBC = n
							}
							if countHome == "BC" && binary.LittleEndian.Uint32(z.Memory()[0x6108:0x610c]) != expectedBC {
								t.Fatal("count register lost")
							}
							if countHome == "IX" && r.IX != uint16(n) {
								t.Fatal("index count lost")
							}
						})
					}
				}
			}
		}
	}
}
