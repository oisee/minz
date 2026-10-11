package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func checksumCopyRegisters() emulator.Registers {
	return emulator.Registers{A: 0x97, F: 0x45, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0xdef0, IY: 0x2468, SP: 0xff00, PC: 0x8000}
}

func checksumCopyBytes(r emulator.Registers) map[string]byte {
	return map[string]byte{"A": r.A, "B": byte(r.BC >> 8), "C": byte(r.BC), "D": byte(r.DE >> 8), "E": byte(r.DE),
		"H": byte(r.HL >> 8), "L": byte(r.HL), "IXH": byte(r.IX >> 8), "IXL": byte(r.IX), "IYH": byte(r.IY >> 8), "IYL": byte(r.IY)}
}

func runChecksumCopyAsm(t *testing.T, asm string, initial emulator.Registers) *emulator.RemogattoZ80 {
	t.Helper()
	res, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm + "DI\nHALT\n")
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("assemble: %v %v\n%s", err, res.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	if err := z.LoadMemory(0x8000, res.Binary); err != nil {
		t.Fatal(err)
	}
	z.SetMemory(0xc03d, 0xe9)
	z.SetMemory(0xe000, 0x6b)
	z.SetMemory(0xe001, 0xd2)
	z.SetRegisters(initial)
	for steps := 0; !z.IsHalted(); steps++ {
		if steps >= 1000 {
			t.Fatalf("no HALT\n%s", asm)
		}
		z.Step()
	}
	return z
}

// Exercise both production entry points with every pair/half combination and
// both input orders. Expected bytes come from the original register snapshot.
func TestZ80ParallelCopyPairHalfOverlap(t *testing.T) {
	type copyCase struct {
		name   string
		copies []parallelCopy
	}
	cases := []copyCase{}
	for _, src := range []string{"BC", "DE", "HL", "IX", "IY"} {
		for _, dst := range []string{"BC", "DE", "HL", "IX", "IY"} {
			if src == dst {
				continue
			}
			for _, half := range []string{highByte(src), lowByte(src)} {
				for _, reverse := range []bool{false, true} {
					copies := []parallelCopy{{srcName: "A", dstName: half, ty: TyU8}, {srcName: src, dstName: dst, ty: TyU16}}
					if reverse {
						copies[0], copies[1] = copies[1], copies[0]
					}
					cases = append(cases, copyCase{fmt.Sprintf("%s_to_%s/%s/reverse_%v", src, dst, half, reverse), copies})
				}
			}
		}
	}
	cases = append(cases,
		copyCase{"mixed_cycle", []parallelCopy{{"BC", "HL", TyU16, false, 0}, {"H", "B", TyU8, false, 0}, {"L", "C", TyU8, false, 0}}},
		copyCase{"mixed_cycle_live_A", []parallelCopy{{"BC", "HL", TyU16, false, 0}, {"H", "B", TyU8, false, 0}, {"L", "C", TyU8, false, 0}, {"A", "A", TyU8, false, 0}}},
		copyCase{"cycle_fanout", []parallelCopy{{"B", "C", TyU8, false, 0}, {"C", "B", TyU8, false, 0}, {"B", "D", TyU8, false, 0}}},
		copyCase{"index_prefix_cycle", []parallelCopy{{"IX", "HL", TyU16, false, 0}, {"H", "IXH", TyU8, false, 0}, {"L", "IXL", TyU8, false, 0}}},
		copyCase{"separate_byte_word_cycles", []parallelCopy{{"A", "B", TyU8, false, 0}, {"B", "A", TyU8, false, 0}, {"HL", "DE", TyU16, false, 0}, {"DE", "HL", TyU16, false, 0}}},
	)
	for _, tc := range cases {
		for _, entry := range []string{"edge", "call"} {
			t.Run(tc.name+"/"+entry, func(t *testing.T) {
				var sb strings.Builder
				f := &Func{Name: "copy"}
				target := &Block{Label: "target"}
				f.Blocks = []*Block{target}
				callee := &Func{Name: "callee"}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
				var args []Reg
				initial := checksumCopyRegisters()
				old := checksumCopyBytes(initial)
				want := checksumCopyBytes(initial)
				for i, c := range tc.copies {
					s, d := Reg(i*2+1), Reg(i*2+2)
					args = append(args, s)
					ar.Locs[s] = PhysLoc{Kind: LocReg, Name: c.srcName}
					ar.Locs[d] = PhysLoc{Kind: LocReg, Name: c.dstName}
					target.Params = append(target.Params, BlockParam{Dst: d, Ty: c.ty})
					callee.Contract.Params = append(callee.Contract.Params, Param{Reg: d, Ty: c.ty})
					if c.ty.Width() == 16 {
						want[highByte(c.dstName)] = old[highByte(c.srcName)]
						want[lowByte(c.dstName)] = old[lowByte(c.srcName)]
					} else {
						want[c.dstName] = old[c.srcName]
					}
				}
				g := &z80cg{fn: f, ar: ar, sb: &sb, holdsPhys: map[string]string{}}
				if entry == "edge" {
					g.emitParallelCopy(g.buildBlockCopies(f, "target", args))
				} else {
					g.emitCallArgs(args, callee)
				}
				got := runChecksumCopyAsm(t, sb.String(), initial).GetRegisters()
				for name, value := range checksumCopyBytes(got) {
					if value != want[name] {
						t.Fatalf("%s=%02x want %02x\n%s", name, value, want[name], sb.String())
					}
				}
				if got.F != initial.F || got.SP != initial.SP {
					t.Fatalf("F/SP=%02x/%04x want %02x/%04x\n%s", got.F, got.SP, initial.F, initial.SP, sb.String())
				}
			})
		}
	}
}

func TestZ80ByteLoadPreservesAccumulator(t *testing.T) {
	for _, ptr := range []string{"BC", "DE", "HL", "IX", "IY"} {
		for _, dst := range []string{"A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL"} {
			t.Run(ptr+"_to_"+dst, func(t *testing.T) {
				var sb strings.Builder
				g := &z80cg{sb: &sb, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: ptr}, 2: {Kind: LocReg, Name: dst}}}, holdsPhys: map[string]string{}}
				g.genInst(&Inst{Op: OpLoad, Dst: 2, Src: [2]Reg{1}, Ty: TyU8})
				initial := checksumCopyRegisters()
				switch ptr {
				case "BC":
					initial.BC = 0xc03d
				case "DE":
					initial.DE = 0xc03d
				case "HL":
					initial.HL = 0xc03d
				case "IX":
					initial.IX = 0xc03d
				case "IY":
					initial.IY = 0xc03d
				}
				want := checksumCopyBytes(initial)
				want[dst] = 0xe9
				got := runChecksumCopyAsm(t, sb.String(), initial).GetRegisters()
				for name, value := range checksumCopyBytes(got) {
					if value != want[name] {
						t.Fatalf("%s=%02x want %02x\n%s", name, value, want[name], sb.String())
					}
				}
				if got.F != initial.F || got.SP != initial.SP {
					t.Fatalf("load clobbered F/SP: %+v\n%s", got, sb.String())
				}
			})
		}
	}
}

func TestZ80ByteSpillCopyPreservesAccumulator(t *testing.T) {
	for _, copy := range []parallelCopy{{srcName: "C", dstName: "$E000", ty: TyU8}, {srcName: "$E000", dstName: "D", ty: TyU8}} {
		t.Run(copy.srcName+"_to_"+copy.dstName, func(t *testing.T) {
			var sb strings.Builder
			g := &z80cg{sb: &sb, holdsPhys: map[string]string{}}
			g.emitParallelCopy([]parallelCopy{copy})
			initial := checksumCopyRegisters()
			z := runChecksumCopyAsm(t, sb.String(), initial)
			got := z.GetRegisters()
			want := checksumCopyBytes(initial)
			if copy.dstName == "D" {
				want["D"] = 0x6b
			}
			for name, value := range checksumCopyBytes(got) {
				if value != want[name] {
					t.Fatalf("%s=%02x want %02x\n%s", name, value, want[name], sb.String())
				}
			}
			if copy.dstName == "$E000" && z.GetMemory(0xe000) != byte(initial.BC) {
				t.Fatal("wrong spill byte")
			}
			if got.F != initial.F || got.SP != initial.SP {
				t.Fatalf("spill copy clobbered F/SP: %+v", got)
			}
		})
	}
}

func TestZ80ParallelCopyWordSpillOverlap(t *testing.T) {
	for _, tc := range []struct {
		name               string
		copies             []parallelCopy
		bc, de, hl, memory uint16
	}{
		{"store_before_half", []parallelCopy{{"BC", "$E000", TyU16, false, 0}, {"A", "C", TyU8, false, 0}}, 0x1297, 0x5678, 0x9abc, 0x1234},
		{"half_before_load", []parallelCopy{{"$E000", "BC", TyU16, false, 0}, {"C", "D", TyU8, false, 0}}, 0xd26b, 0x3478, 0x9abc, 0xd26b},
		{"mixed_spill_cycle", []parallelCopy{{"BC", "$E000", TyU16, false, 0}, {"$E000", "HL", TyU16, false, 0}, {"H", "B", TyU8, false, 0}, {"L", "C", TyU8, false, 0}}, 0x9abc, 0x5678, 0xd26b, 0x1234},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sb strings.Builder
			g := &z80cg{sb: &sb, holdsPhys: map[string]string{}}
			g.emitParallelCopy(tc.copies)
			initial := checksumCopyRegisters()
			z := runChecksumCopyAsm(t, sb.String(), initial)
			got := z.GetRegisters()
			memory := uint16(z.GetMemory(0xe000)) | uint16(z.GetMemory(0xe001))<<8
			if got.BC != tc.bc || got.DE != tc.de || got.HL != tc.hl || memory != tc.memory ||
				got.A != initial.A || got.F != initial.F || got.IX != initial.IX || got.IY != initial.IY || got.SP != initial.SP {
				t.Fatalf("registers %+v memory %04x want BC/DE/HL/memory %04x/%04x/%04x/%04x\n%s", got, memory, tc.bc, tc.de, tc.hl, tc.memory, sb.String())
			}
		})
	}
}

func TestParallelCopyCoalescedByteWithWideType(t *testing.T) {
	g := &z80cg{sb: &strings.Builder{}, holdsPhys: map[string]string{}}
	g.emitParallelCopy([]parallelCopy{{srcName: "D", dstName: "D", ty: TyU16}})
	if g.sb.Len() != 0 {
		t.Fatalf("coalesced copy emitted %s", g.sb.String())
	}
}
