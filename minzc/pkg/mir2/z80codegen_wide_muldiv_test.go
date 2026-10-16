package mir2

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestNativeWideArithmeticBanksAndSpills(t *testing.T) {
	homes := []string{"HL", "DE", "BC", "spill"}
	values := []uint32{0, 1, 3, 0xffff, 0x10000, 0x80000003, 0xffffffff}
	for _, width := range []int{24, 32} {
		for _, op := range []Op{OpMul, OpDiv, OpMod, OpSDiv, OpSMod} {
			for _, lhs := range homes {
				for _, rhs := range homes {
					for _, dst := range homes {
						t.Run(fmt.Sprintf("%d/%s/%s-%s-%s", width, op, lhs, rhs, dst), func(t *testing.T) {
							ty := Ty(TyU32)
							if width == 24 {
								ty = TyU24
							}
							if op == OpSDiv || op == OpSMod {
								if width == 24 {
									ty = TyI24
								} else {
									ty = TyI32
								}
							}
							var out strings.Builder
							f := &Func{Name: "wide"}
							ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
							for i, home := range []string{lhs, rhs, dst} {
								loc := PhysLoc{Kind: LocDWord, Name: home}
								if home == "spill" {
									loc = PhysLoc{Kind: LocMem}
								}
								ar.Locs[Reg(i+1)] = loc
							}
							g := &z80cg{sb: &out, fn: f, ar: ar, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}}
							g.emit("boot:")
							pairs := []string{"HL", "DE", "BC"}
							for i, pair := range pairs {
								g.emitf("LD %s,(0x%x)", pair, 0x6200+4*i)
								g.emit("EXX")
								g.emitf("LD %s,(0x%x)", pair, 0x6202+4*i)
								g.emit("EXX")
							}
							g.emit("LD A,(0x6310)")
							g.emit("OR A")
							g.emit("JR Z,.interrupt_mode_ready")
							g.emit("EI")
							g.emit(".interrupt_mode_ready:")
							g.emit("LD A,0")
							g.emit("OR A")
							g.emit("LD A,0x5a")
							g.emit("SCF")
							g.emit("PUSH AF")
							g.emit("POP IX")
							g.emit("LD (0x6300),IX")
							g.emit("LD IX,0x2222")
							g.emit("LD IY,0x3333")
							g.emit("CALL wide")
							g.emit("after_wide:")
							for i, pair := range pairs {
								g.emitf("LD (0x%x),%s", 0x6100+4*i, pair)
								g.emit("EXX")
								g.emitf("LD (0x%x),%s", 0x6102+4*i, pair)
								g.emit("EXX")
							}
							g.emit("PUSH AF")
							g.emit("POP DE")
							g.emit("LD (0x610c),DE")
							g.emit("LD (0x610e),IX")
							g.emit("LD (0x6110),IY")
							g.emit("DI")
							g.emit("HALT")
							g.emit("wide:")
							g.genWideArithmetic(&Inst{Op: op, Dst: 3, Src: [2]Reg{1, 2}, Ty: ty})
							g.emit("RET")
							for i := 1; i <= 3; i++ {
								g.emitf("_spill_wide_r%d: DB 0,0,0,0", i)
							}
							bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + out.String())
							if e != nil {
								t.Fatal(e, out.String())
							}
							if len(bin.Errors) > 0 {
								t.Fatal(bin.Errors, out.String())
							}
							mask := uint32((uint64(1) << width) - 1)
							signed := func(v uint32) int64 { shift := 64 - width; return int64(uint64(v)<<shift) >> shift }
							for _, av := range values {
								for _, bv := range values {
									a, b := av&mask, bv&mask
									if lhs == rhs && lhs != "spill" {
										b = a
									}
									if op != OpMul && b == 0 {
										continue
									}
									var want uint32
									switch op {
									case OpMul:
										want = a * b
									case OpDiv:
										want = a / b
									case OpMod:
										want = a % b
									case OpSDiv:
										want = uint32(signed(a) / signed(b))
									case OpSMod:
										want = uint32(signed(a) % signed(b))
									}
									want &= mask
									setup := map[string]uint32{"HL": 0x9abc5678, "DE": 0x24681357, "BC": 0xabcdef01}
									if lhs != "spill" {
										setup[lhs] = a
									}
									if rhs != "spill" {
										setup[rhs] = b
									}
									z := emulator.NewRemogattoZ80()
									z.LoadMemory(0x8000, bin.Binary)
									for i, pair := range pairs {
										binary.LittleEndian.PutUint32(z.Memory()[0x6200+4*i:0x6204+4*i], setup[pair])
									}
									for i, v := range []uint32{a, b, 0xfacefeed} {
										p := bin.Symbols[fmt.Sprintf("_spill_wide_r%d", i+1)]
										binary.LittleEndian.PutUint32(z.Memory()[p:p+4], v)
										if width == 24 {
											z.Memory()[p+3] = 0xaa
										}
									}
									enabled := (a^b)&1 != 0
									if enabled {
										z.Memory()[0x6310] = 1
									}
									z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000})
									checkedInterrupts := false
									for n := 0; n < 20000 && !z.IsHalted(); n++ {
										if z.GetRegisters().PC == uint16(bin.Symbols["after_wide"]) {
											checkedInterrupts = true
											if z.GetIFF1() != enabled || z.GetIFF2() != enabled {
												t.Fatalf("interrupt mode changed: enabled=%v IFF1=%v IFF2=%v", enabled, z.GetIFF1(), z.GetIFF2())
											}
										}
										z.Step()
									}
									if !checkedInterrupts {
										t.Fatal("did not return from kernel")
									}

									for i, pair := range pairs {
										expected := setup[pair]
										if pair == dst {
											expected = want
										}
										got := binary.LittleEndian.Uint32(z.Memory()[0x6100+4*i : 0x6104+4*i])
										if got != expected {
											t.Fatalf("a=%08x b=%08x %s got=%08x want=%08x\n%s", a, b, pair, got, expected, out.String())
										}
									}
									for i, v := range []uint32{a, b, 0xfacefeed} {
										p := bin.Symbols[fmt.Sprintf("_spill_wide_r%d", i+1)]
										expected := v
										if width == 24 {
											expected = expected&mask | 0xaa000000
										}
										if i == 2 && dst == "spill" {
											expected = want
											if width == 24 {
												expected |= 0xaa000000
											}
										}
										got := binary.LittleEndian.Uint32(z.Memory()[p : p+4])
										if got != expected {
											t.Fatalf("spill%d a=%08x b=%08x got=%08x want=%08x", i+1, a, b, got, expected)
										}
									}
									before := binary.LittleEndian.Uint16(z.Memory()[0x6300:0x6302])
									after := binary.LittleEndian.Uint16(z.Memory()[0x610c:0x610e])
									r := z.GetRegisters()
									if before != after || r.SP != 0xf000 || !z.IsHalted() || r.IX != 0x2222 || r.IY != 0x3333 {
										t.Fatalf("flags %04x/%04x regs=%v", before, after, r)
									}
								}
							}
						})
					}
				}
			}
		}
	}
}
