package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Encodability is decided before skipping AND/zero, so ordinary fallback has
// real values. A live word source and hostile stale mask destination remain.
func TestBitCmpFallbackNativeIndexedAndSpill(t *testing.T) {
	for _, home := range []string{"IX", "IY", "spill"} {
		for _, predicate := range []mir2.CmpCond{mir2.CmpNe, mir2.CmpEq} {
			for _, shift := range []bool{false, true} {
				for value := 0; value < 256; value++ {
					m := &mir2.Module{}
					f := m.AddFunc("testbit")
					f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
					b := mir2.NewBuilder(f)
					b.SwitchToNewBlock("entry")
					x := b.Param("x", mir2.TyU16, mir2.ClassPair)
					r := x
					locations := map[mir2.Reg]mir2.PhysLoc{}
					if shift {
						k := b.Const(8, mir2.TyU16, mir2.ClassPair)
						r = b.Shr(x, k, mir2.TyU16, mir2.ClassPair)
						locations[k] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "BC"}
						locations[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "DE"}
					}
					one := b.Const(1, mir2.TyU16, mir2.ClassPair)
					masked := b.And(r, one, mir2.TyU16, mir2.ClassPair)
					zero := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
					cond := b.Cmp(predicate, masked, zero, mir2.ClassFlag, false)
					b.BrIf(cond, "yes", nil, "no", nil)
					b.SwitchToNewBlock("yes")
					y := b.Const(1, mir2.TyU8, mir2.ClassAcc)
					b.Ret(y)
					b.SwitchToNewBlock("no")
					n := b.Const(0, mir2.TyU8, mir2.ClassAcc)
					b.Ret(n)
					xLoc := mir2.PhysLoc{Kind: mir2.LocIXY, Name: home}
					init := fmt.Sprintf("LD %s,%d\n", home, value)
					arg := value
					if shift {
						arg = value << 8
						init = fmt.Sprintf("LD %s,%d\n", home, arg)
					}
					if home == "spill" {
						xLoc = mir2.PhysLoc{Kind: mir2.LocMem, Name: "$F000", Offset: 0xf000}
						init = fmt.Sprintf("LD HL,%d\nLD (_spill_testbit_r%d),HL\n", arg, x)
					}
					locations[x] = xLoc
					locations[one] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "BC"}
					locations[masked] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
					locations[zero] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
					locations[cond] = mir2.PhysLoc{Kind: mir2.LocFlag, Name: "F"}
					locations[y] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
					locations[n] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
					asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locations}))
					text := "ORG 0x8000\nLD SP,0xFF00\n" + init + "LD HL,1\nLD A,1\nCALL testbit\nDI\nstop:\nHALT\n" + asm
					program, e := z80asm.NewAssembler().AssembleString(text)
					if e != nil || len(program.Errors) > 0 {
						t.Fatal(e, program.Errors, text)
					}
					z := emulator.NewRemogattoZ80()
					z.Reset()
					if e = z.LoadMemory(0x8000, program.Binary); e != nil {
						t.Fatal(e)
					}
					z.SetPC(0x8000)
					if e = z.LoadMemory(0x9000, []byte{0x5a, 0xa5}); e != nil {
						t.Fatal(e)
					}
					for steps := 0; !z.IsHalted(); steps++ {
						if steps >= 500 {
							t.Fatal("native bound")
						}
						z.Step()
					}
					regs := z.GetRegisters()
					if regs.SP != 0xff00 || regs.PC != uint16(program.Symbols["stop"]) || z.GetMemory(0x9000) != 0x5a || z.GetMemory(0x9001) != 0xa5 {
						t.Fatal("stack/PC/guard", regs)
					}
					for _, line := range program.Listing {
						instruction := strings.TrimSpace(strings.SplitN(line.SourceLine, ";", 2)[0])
						if pos := strings.Index(instruction, ":"); pos >= 0 {
							instruction = strings.TrimSpace(instruction[pos+1:])
						}
						fields := strings.Fields(instruction)
						if len(fields) == 0 || len(line.Bytes) == 0 {
							continue
						}
						op := strings.ToUpper(fields[0])
						if op == "DB" || op == "DW" || op == "DS" {
							continue
						}
						for i, b := range line.Bytes {
							if z.GetMemory(uint16(line.Address+i)) != b {
								t.Fatal("instruction changed", home, shift, value, line.Address+i)
							}
						}
					}

					if home == "IX" && regs.IX != uint16(arg) || home == "IY" && regs.IY != uint16(arg) {
						t.Fatal("indexed source changed", regs, arg)
					}
					if home == "spill" {
						addr := program.Symbols[fmt.Sprintf("_spill_testbit_r%d", x)]
						if z.GetMemory(uint16(addr)) != byte(arg) || z.GetMemory(uint16(addr+1)) != byte(arg>>8) {
							t.Fatal("spilled source changed", arg)
						}
					}
					if value == 1 {
						if dir := os.Getenv("MINZ_BITCMP_ADMISSION_ARTIFACTS"); dir != "" {
							if e = os.MkdirAll(dir, 0755); e != nil {
								t.Fatal(e)
							}
							name := fmt.Sprintf("%s-shift-%v-eq-%v", home, shift, predicate == mir2.CmpEq)
							for file, data := range map[string][]byte{name + ".a80": []byte(text), name + ".bin": program.Binary} {
								if e = os.WriteFile(filepath.Join(dir, file), data, 0644); e != nil {
									t.Fatal(e)
								}
							}
						}
					}

					want := byte(value & 1)
					if predicate == mir2.CmpEq {
						want ^= 1
					}
					if regs.A != want {
						t.Fatalf("home%s shifted%v input%d got%d want%d\n%s", home, shift, value, regs.A, value&1, asm)
					}
				}
			}
		}
	}
}
