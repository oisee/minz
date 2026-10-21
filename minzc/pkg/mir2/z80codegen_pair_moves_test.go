package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func pairMoveGenerator(out *strings.Builder) *z80cg {
	f := &Func{Name: "pair_moves"}
	b := f.NewBlock("entry")
	b.Term = &TermRet{}
	return &z80cg{fn: f, curBlock: b, sb: out, ar: &AllocResult{Locs: map[Reg]PhysLoc{}}, regInfo: map[Reg]RegInfo{}, constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, liveness: ComputeLiveness(f), pendingFlagReg: NoReg, pendingAccReg: NoReg}
}

func runPairMove(t *testing.T, code string, initial emulator.Registers) emulator.Registers {
	t.Helper()
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + code + "\nDI\nHALT\n")
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assemble: %v %v\n%s", err, a.Errors, code)
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	if err := z.LoadMemory(0x8000, a.Binary); err != nil {
		t.Fatal(err)
	}
	initial.PC, initial.SP = 0x8000, 0xff00
	z.SetRegisters(initial)
	for n := 0; !z.IsHalted(); n++ {
		if n > 200 {
			t.Fatal("no halt")
		}
		z.Step()
	}
	r := z.GetRegisters()
	if r.SP != 0xff00 {
		t.Fatalf("unbalanced stack: %+v", r)
	}
	return r
}

func pairValue(r emulator.Registers, pair string) uint16 {
	switch pair {
	case "BC":
		return r.BC
	case "DE":
		return r.DE
	case "HL":
		return r.HL
	case "IX":
		return r.IX
	case "IY":
		return r.IY
	}
	panic(pair)
}

func TestZ80PrimaryPairMoves(t *testing.T) {
	for _, src := range []string{"BC", "DE", "HL"} {
		for _, dst := range []string{"BC", "DE", "HL"} {
			t.Run(src+"_to_"+dst, func(t *testing.T) {
				var out strings.Builder
				g := pairMoveGenerator(&out)
				g.emitMov(dst, src, 16)
				if strings.Contains(out.String(), "PUSH") || strings.Contains(out.String(), "POP") {
					t.Fatalf("pair copy must use byte LDs:\n%s", out.String())
				}
				for _, v := range []uint16{0, 0xff, 0x100, 0x80ff, 0xffff} {
					initial := emulator.Registers{BC: 0x1234, DE: 0x5678, HL: 0x9abc, A: 0x67, F: 0xd5}
					switch src {
					case "BC":
						initial.BC = v
					case "DE":
						initial.DE = v
					case "HL":
						initial.HL = v
					}
					r := runPairMove(t, out.String(), initial)
					for _, pair := range []string{"BC", "DE", "HL"} {
						if pair != dst && pairValue(r, pair) != pairValue(initial, pair) {
							t.Fatalf("live %s corrupted: %+v", pair, r)
						}
					}
					if pairValue(r, dst) != v || r.A != initial.A || r.F != initial.F {
						t.Fatalf("%+v\n%s", r, out.String())
					}
				}
			})
		}
	}
}

func TestZ80PairParallelCopyCycles(t *testing.T) {
	for _, pairs := range [][]string{{"HL", "DE"}, {"HL", "BC"}, {"DE", "BC"}, {"HL", "DE", "BC"}} {
		t.Run(strings.Join(pairs, "_"), func(t *testing.T) {
			var out strings.Builder
			g := pairMoveGenerator(&out)
			var copies []parallelCopy
			for i, src := range pairs {
				copies = append(copies, parallelCopy{srcName: src, dstName: pairs[(i+1)%len(pairs)], ty: TyU16})
			}
			g.emitParallelCopy(copies)
			initial := emulator.Registers{BC: 0x1234, DE: 0x5678, HL: 0x9abc, A: 0x67, F: 0xd5}
			r := runPairMove(t, out.String(), initial)
			for _, c := range copies {
				if pairValue(r, c.dstName) != pairValue(initial, c.srcName) {
					t.Fatalf("bad cycle %+v\n%s", r, out.String())
				}
			}
			if r.A != initial.A || r.F != initial.F {
				t.Fatalf("flags/A corrupted %+v", r)
			}
		})
	}
}

func TestZ80DirectBytePairExtension(t *testing.T) {
	for _, dst := range []string{"BC", "DE", "HL"} {
		for _, src := range []string{"A", "B", "C", "D", "E", "H", "L"} {
			t.Run(dst+"_"+src, func(t *testing.T) {
				var out strings.Builder
				g := pairMoveGenerator(&out)
				g.genUnsignedExtBytes(dst, src, 8, 16)
				if strings.Contains(out.String(), "PUSH") {
					t.Fatalf("direct extension must preserve A without stack:\n%s", out.String())
				}
				for value := 0; value < 256; value++ {
					r := runPairMove(t, fmt.Sprintf("LD %s,%d\n", src, value)+out.String(), emulator.Registers{BC: 0x1234, DE: 0x5678, HL: 0x9abc, A: 0x67, F: 0xd5})
					wantA := byte(0x67)
					if src == "A" {
						wantA = byte(value)
					}
					if pairValue(r, dst) != uint16(value) || r.A != wantA || r.F != 0xd5 {
						t.Fatalf("value%d %+v\n%s", value, r, out.String())
					}
				}
			})
		}
	}
}

func TestZ80PtrAddReusesOffsetPair(t *testing.T) {
	for _, off := range []string{"BC", "DE"} {
		for _, base := range []string{"HL", "BC", "DE"} {
			for _, dst := range []string{"HL", "BC", "DE"} {
				t.Run(base+"_"+off+"_to_"+dst, func(t *testing.T) {
					var out strings.Builder
					g := pairMoveGenerator(&out)
					inst := &Inst{Op: OpPtrAdd, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyPtr}
					g.curBlock.Insts = []*Inst{inst}
					for r, name := range map[Reg]string{1: base, 2: off, 3: dst} {
						g.ar.Locs[r] = PhysLoc{Kind: LocReg, Name: name}
						g.regInfo[r] = RegInfo{Ty: TyU16}
					}
					// Every other pair holds a live-through value, including the base
					// or offset where its physical storage does not become the result.
					var live []Reg
					for i, name := range []string{"HL", "BC", "DE"} {
						if name == dst {
							continue
						}
						r := Reg(4 + i)
						g.ar.Locs[r] = PhysLoc{Kind: LocReg, Name: name}
						g.regInfo[r] = RegInfo{Ty: TyU16}
						live = append(live, r)
					}
					g.curBlock.Term = &TermRet{Vals: live}
					g.liveness = ComputeLiveness(g.fn)
					g.genInst(inst)
					if !strings.Contains(out.String(), "ADD HL, "+off) {
						t.Fatalf("must reuse offset pair:\n%s", out.String())
					}
					initial := emulator.Registers{BC: 0x1234, DE: 0x5678, HL: 0x9abc, A: 0x67}
					r := runPairMove(t, out.String(), initial)
					if pairValue(r, dst) != pairValue(initial, base)+pairValue(initial, off) || r.A != initial.A {
						t.Fatalf("bad sum %+v\n%s", r, out.String())
					}
					for _, pair := range []string{"HL", "BC", "DE"} {
						if pair != dst && pairValue(r, pair) != pairValue(initial, pair) {
							t.Fatalf("live %s corrupted: %+v\n%s", pair, r, out.String())
						}
					}
				})
			}
		}
	}
}

func TestZ80SmallPairSteps(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub} {
		for _, cv := range []int64{-2, -1, 1, 2, 65534, 65535} {
			for _, dst := range []string{"BC", "DE", "HL"} {
				t.Run(fmt.Sprintf("%s_%d_%s", op, cv, dst), func(t *testing.T) {
					var out strings.Builder
					g := pairMoveGenerator(&out)
					inst := &Inst{Op: op, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
					g.curBlock.Insts = []*Inst{inst}
					for r, name := range map[Reg]string{1: "DE", 2: "BC", 3: dst} {
						g.ar.Locs[r] = PhysLoc{Kind: LocReg, Name: name}
						g.regInfo[r] = RegInfo{Ty: TyU16}
					}
					g.constVals[2] = cv
					g.liveness = ComputeLiveness(g.fn)
					mnemonic := "ADD"
					step := int16(cv)
					if op == OpSub {
						mnemonic = "SUB"
						step = -step
					}
					g.genBinOp(mnemonic, inst)
					if strings.Contains(out.String(), "ADD ") || strings.Contains(out.String(), "SBC ") {
						t.Fatalf("small step must use INC/DEC:\n%s", out.String())
					}
					for _, v := range []uint16{0, 1, 255, 256, 32767, 65534, 65535} {
						r := runPairMove(t, out.String(), emulator.Registers{BC: 0xbeef, DE: v, HL: 0x9abc, A: 0x67, F: 0xd5})
						if pairValue(r, dst) != v+uint16(step) || r.A != 0x67 || r.F != 0xd5 {
							t.Fatalf("v=%04x %+v\n%s", v, r, out.String())
						}
					}
				})
			}
		}
	}
}

func TestZ80SmallPairStepsPreserveObservedCarry(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub} {
		for _, value := range []int64{1, 2} {
			t.Run(fmt.Sprintf("%s_%d", op, value), func(t *testing.T) {
				var out strings.Builder
				g := pairMoveGenerator(&out)
				g.fn.Attrs.FallibleCY = true
				inst := &Inst{Op: op, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
				g.curBlock.Insts = []*Inst{{Op: OpConst, Dst: 2, Imm: value, Ty: TyU16}, inst}
				for r, name := range map[Reg]string{1: "HL", 2: "DE", 3: "HL"} {
					g.ar.Locs[r] = PhysLoc{Kind: LocReg, Name: name}
					g.regInfo[r] = RegInfo{Ty: TyU16}
				}
				g.constVals[2] = value
				g.liveness = ComputeLiveness(g.fn)
				if computeDeadConsts(g.fn, g.ar)[2] {
					t.Fatal("constant needed for carry-producing arithmetic was suppressed")
				}
				mnemonic := "ADD"
				initial := uint16(0xffff)
				if op == OpSub {
					mnemonic = "SUB"
					initial = 0
				}
				g.genBinOp(mnemonic, inst)
				if strings.Contains(out.String(), "INC ") || strings.Contains(out.String(), "DEC ") {
					t.Fatalf("carry must be produced by arithmetic:\n%s", out.String())
				}
				r := runPairMove(t, out.String(), emulator.Registers{HL: initial, DE: uint16(value), A: 0x67})
				if r.F&1 == 0 {
					t.Fatalf("carry lost: %+v\n%s", r, out.String())
				}
			})
		}
	}
}
