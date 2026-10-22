package mir2

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// Exercise adapters with permuted input homes and canaries in every main
// register, including a predicate in F which is not a pending comparison.
func TestByteLoopNativeRegisterCanaries(t *testing.T) {
	for _, kind := range []string{"scan", "fill", "copy", "scan-scalar", "fill-scalar", "copy-scalar"} {
		for _, layout := range []int{0, 1, 2} {
			t.Run(kind+string(rune('0'+layout)), func(t *testing.T) {
				var code strings.Builder
				g := pairMoveGenerator(&code)
				kind := kind
				if strings.HasSuffix(kind, "-scalar") {
					g.functionOptions.CompactByteLoops = true
					kind = strings.TrimSuffix(kind, "-scalar")
				}
				g.curBlock = g.fn.Entry()
				x := &Inst{Op: OpCall, Ty: TyVoid, ByteLoop: &ByteLoopExtra{CountTy: TyU8}}
				inputs := []string{"HL", "B", "E"}
				if layout == 1 {
					inputs = []string{"DE", "C", "A"}
				} else if layout == 2 {
					inputs = []string{"IX", "L", "IYH"}
				}
				switch kind {
				case "scan":
					x.Sym = scanZeroIntrinsic
					x.Dst = 4
					x.Ty = TyU16
					x.Args = []Reg{1}
					g.ar.Locs[4] = PhysLoc{Kind: LocReg, Name: "IX"}
					inputs = []string{[]string{"HL", "DE", "IY"}[layout]}
				case "fill":
					x.Sym = fillBytesIntrinsic
					x.Args = []Reg{1, 2, 3}
				case "copy":
					x.Sym = copyBytesIntrinsic
					x.Args = []Reg{1, 2, 3}
					inputs = []string{"DE", "B", "HL"}
					if layout == 1 {
						inputs = []string{"HL", "C", "DE"}
					} else if layout == 2 {
						inputs = []string{"IX", "A", "IY"}
					}
				}
				for i, loc := range inputs {
					g.ar.Locs[Reg(i+1)] = PhysLoc{Kind: LocReg, Name: loc}
				}
				var keep []Reg
				for i, loc := range []string{"A", "F", "BC", "DE", "HL", "IX", "IY"} {
					if kind == "scan" && loc == "IX" {
						continue
					}
					r := Reg(i + 10)
					g.ar.Locs[r] = PhysLoc{Kind: LocReg, Name: loc}
					keep = append(keep, r)
				}
				g.curBlock.Insts = []*Inst{x}
				g.curBlock.Term = &TermRet{Vals: keep}
				g.liveness = ComputeLiveness(g.fn)
				g.genByteLoop(x)
				if g.err != nil {
					t.Fatal(g.err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nentry:\n" + code.String() + "\nDI\nHALT\n")
				if err != nil {
					t.Fatal(err, code.String())
				}
				if len(a.Errors) > 0 {
					t.Fatal(a.Errors, code.String())
				}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				r := emulator.Registers{A: 0x67, F: 0x95, BC: 0x1234, DE: 0x4567, HL: 0x3456, IX: 0x6789, IY: 0x789a, SP: 0xf000, PC: 0x8000}
				setPair := func(name string, v uint16) {
					switch name {
					case "BC":
						r.BC = v
					case "DE":
						r.DE = v
					case "HL":
						r.HL = v
					case "IX":
						r.IX = v
					case "IY":
						r.IY = v
					default:
						t.Fatal(name)
					}
				}
				setPair(inputs[0], 0x4000)
				if kind == "copy" {
					setPair(inputs[2], 0x4100)
				}
				z.SetRegisters(r)
				if kind != "scan" {
					if e := z.SetRegister8(inputs[1], 3); e != nil {
						t.Fatal(e)
					}
					if kind == "fill" {
						if e := z.SetRegister8(inputs[2], 0xd3); e != nil {
							t.Fatal(e)
						}
					}
				}
				before := z.GetRegisters()
				copy(mem[0x4000:], []byte{9, 8, 7, 0, 6})
				copy(mem[0x4100:], []byte{4, 5, 6, 7})
				for step := 0; step < 1000 && !z.IsHalted(); step++ {
					z.Step()
				}
				if !z.IsHalted() {
					t.Fatal("no halt")
				}
				after := z.GetRegisters()
				after.PC = before.PC
				if kind == "scan" {
					if after.IX != 3 {
						t.Fatalf("scan result %d", after.IX)
					}
					after.IX = before.IX
				}
				if after != before {
					t.Fatalf("registers changed before=%+v after=%+v\n%s", before, after, code.String())
				}
				if kind == "fill" && !bytes.Equal(mem[0x4000:0x4005], []byte{0xd3, 0xd3, 0xd3, 0, 6}) {
					t.Fatal("fill memory", mem[0x4000:0x4005])
				}
				if kind == "copy" && !bytes.Equal(mem[0x4000:0x4005], []byte{4, 5, 6, 0, 6}) {
					t.Fatal("copy memory", mem[0x4000:0x4005])
				}
			})
		}
	}
}

func TestByteLoopGlobalSymbolLabels(t *testing.T) {
	for _, names := range [][4]string{{"module.limit", "module.value", "module_limit", "module_value"}, {"C", "AF", "v_C", "v_AF"}} {
		t.Run(names[0], func(t *testing.T) {
			var code strings.Builder
			g := pairMoveGenerator(&code)
			g.curBlock = g.fn.Entry()
			g.ar.Locs[1] = PhysLoc{Kind: LocReg, Name: "HL"}
			x := &Inst{Op: OpCall, Sym: fillBytesIntrinsic, Ty: TyVoid, Args: []Reg{1, NoReg, NoReg}, ByteLoop: &ByteLoopExtra{CountTy: TyU8, CountSymbol: names[0], ValueSymbol: names[1]}}
			g.curBlock.Insts = []*Inst{x}
			g.genByteLoop(x)
			if g.err != nil {
				t.Fatal(g.err)
			}
			asm := "ORG 0x8000\nentry:\nLD HL,0x4000\n" + code.String() + "\nDI\nHALT\n" + names[2] + ": DB 3\n" + names[3] + ": DB 211\n"
			a, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil {
				t.Fatal(err, asm)
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors, asm)
			}
			z := emulator.NewRemogattoZ80()
			copy(z.Memory()[0x8000:], a.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000})
			for i := 0; i < 1000 && !z.IsHalted(); i++ {
				z.Step()
			}
			if !z.IsHalted() || z.GetSP() != 0xf000 || !bytes.Equal(z.Memory()[0x4000:0x4004], []byte{211, 211, 211, 0}) {
				t.Fatal("global symbol lowering", z.GetRegisters(), asm)
			}
		})
	}
}
