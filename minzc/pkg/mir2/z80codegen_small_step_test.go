package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func runRAFragment(t *testing.T, asm string, initial emulator.Registers) emulator.Registers {
	t.Helper()
	bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nra_fragment:\n" + asm + "\nRET\n")
	if err != nil {
		t.Fatal(err, asm)
	}
	if len(bin.Errors) != 0 {
		t.Fatal(bin.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
		t.Fatal(err)
	}
	initial.PC, initial.SP = 0x8000, 0xeff0
	z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
	z.SetRegisters(initial)
	for n := 0; n < 10000; n++ {
		if z.GetRegisters().PC == 0x7000 {
			return z.GetRegisters()
		}
		z.Step()
	}
	t.Fatal("fragment did not return", asm)
	return emulator.Registers{}
}

func TestByteSmallStepDifferentHomes(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub} {
		for k := int64(1); k <= 3; k++ {
			t.Run(fmt.Sprint(op, k), func(t *testing.T) {
				inst := &Inst{Op: op, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8}
				b := &Block{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 2, Ty: TyU8, Imm: k}, inst}, Term: &TermRet{Vals: []Reg{3}}}
				f := &Func{Name: "step", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8}}, Returns: []Return{{Ty: TyU8}}}, Blocks: []*Block{b}}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "D"}, 2: {Kind: LocReg, Name: "A"}, 3: {Kind: LocReg, Name: "C"}}}
				if !computeDeadConsts(f, ar)[2] {
					t.Fatal("small folded constant retains a register load")
				}
				var out strings.Builder
				g := &z80cg{sb: &out, fn: f, curBlock: b, curInstIdx: 1, ar: ar, constVals: map[Reg]int64{2: k}, deadConsts: map[Reg]bool{2: true}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
				mnem := "ADD"
				if op == OpSub {
					mnem = "SUB"
				}
				g.genBinOp(mnem, inst)
				mnemonic := "INC C"
				if op == OpSub {
					mnemonic = "DEC C"
				}
				if !strings.Contains(out.String(), "LD C, D") || strings.Count(out.String(), mnemonic) != int(k) {
					t.Fatal(out.String())
				}
				vm := NewVM(&Module{Funcs: []*Func{f}})
				for x := 0; x < 256; x++ {
					want := byte(int64(x) + k)
					if op == OpSub {
						want = byte(int64(x) - k)
					}
					got, err := vm.Call(f.Name, []Value{{I: int64(x)}})
					z := runRAFragment(t, out.String(), emulator.Registers{DE: uint16(x) << 8})
					if err != nil || got[0].I != int64(want) || byte(z.BC) != want {
						t.Fatal(x, got, err, z, want)
					}
				}
			})
		}
	}
}

func TestByteSmallStepPreservesFlagContracts(t *testing.T) {
	for _, kind := range []string{"carry-comparison", "asm", "flag-return", "fallible"} {
		t.Run(kind, func(t *testing.T) {
			inst := &Inst{Op: OpSub, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8}
			b := &Block{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 1}, inst}, Term: &TermRet{Vals: []Reg{3}}}
			f := &Func{Name: "flags", Blocks: []*Block{b}}
			switch kind {
			case "carry-comparison":
				b.Insts = append(b.Insts, &Inst{Op: OpCmp, Dst: 4, Ty: TyBool, Cond: CmpSubCarry})
			case "asm":
				b.Insts = append(b.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "ADC A,0"}})
			case "flag-return":
				f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassFlag}}
			case "fallible":
				f.Attrs.FallibleCY = true
			}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "D"}, 2: {Kind: LocReg, Name: "A"}, 3: {Kind: LocReg, Name: "C"}}}
			if computeDeadConsts(f, ar)[2] {
				t.Fatal("constant suppressed across an observed flag boundary")
			}
		})
	}
}

func TestByteSmallStepDoesNotReuseCarry(t *testing.T) {
	sub := &Inst{Op: OpSub, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8}
	cmp := &Inst{Op: OpCmp, Dst: 4, Src: [2]Reg{1, 2}, Ty: TyBool, SrcTy: TyU8, Cond: CmpLt, Cls: ClassFlag}
	b := &Block{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 1}, sub, cmp}, Term: &TermRet{Vals: []Reg{4}}}
	f := &Func{Name: "carry", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8}}}, Blocks: []*Block{b}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "D"}, 2: {Kind: LocReg, Name: "C"}, 3: {Kind: LocReg, Name: "E"}, 4: {Kind: LocReg, Name: "F"}}}
	var out strings.Builder
	g := &z80cg{sb: &out, fn: f, curBlock: b, curInstIdx: 1, ar: ar, constVals: map[Reg]int64{2: 1}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
	g.genInst(sub)
	if !strings.Contains(out.String(), "DEC E") {
		t.Fatal(out.String())
	}
	if g.lastFlagsLhs != "" {
		t.Fatal("DEC carry incorrectly available to comparison fusion")
	}
	g.curInstIdx = 2
	g.genInst(cmp)
	if !g.emitKnownFlagCopy("A", 8) {
		t.Fatal("no predicate")
	}
	for x := 0; x < 256; x++ {
		for _, flag := range []byte{0, 1} {
			z := runRAFragment(t, out.String(), emulator.Registers{DE: uint16(x) << 8, F: flag})
			want := byte(0)
			if x < 1 {
				want = 1
			}
			if z.A != want {
				t.Fatal(x, flag, z, want, out.String())
			}
		}
	}
}
