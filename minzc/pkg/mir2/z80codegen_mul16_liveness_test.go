package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"strings"
	"testing"
)

func TestMul16DeadScratchPairs(t *testing.T) {
	for _, live := range []string{"", "HL", "H", "L", "BC", "B", "C"} {
		t.Run("live="+live, func(t *testing.T) {
			inst := &Inst{Op: OpMul, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
			vals := []Reg{3}
			if live != "" {
				vals = append(vals, 4)
			}
			b := &Block{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 2, Ty: TyU16, Imm: 5}, inst}, Term: &TermRet{Vals: vals}}
			f := &Func{Name: "multiply", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU16}}}, Blocks: []*Block{b}}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "DE"}, 2: {Kind: LocReg, Name: "BC"}, 3: {Kind: LocReg, Name: "DE"}, 4: {Kind: LocReg, Name: live}}}
			var out strings.Builder
			g := &z80cg{sb: &out, fn: f, curBlock: b, curInstIdx: 1, ar: ar, constVals: map[Reg]int64{2: 5}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
			g.genMul16(inst)
			if live == "" && (strings.Contains(out.String(), "PUSH") || strings.Contains(out.String(), "POP")) {
				t.Fatal("dead scratch saved:\n" + out.String())
			}
			if !strings.Contains(out.String(), "LD B, H") || !strings.Contains(out.String(), "LD C, L") {
				t.Fatal("scratch copied via stack:\n" + out.String())
			}
			for _, x := range []uint16{0, 1, 255, 256, 32768, 65535} {
				z := runRAFragment(t, out.String(), emulator.Registers{DE: x, HL: 0x1234, BC: 0x5678})
				if z.DE != x*5 || (live == "HL" || live == "H" || live == "L") && z.HL != 0x1234 || (live == "BC" || live == "B" || live == "C") && z.BC != 0x5678 {
					t.Fatal(x, z, out.String())
				}
			}
		})
	}
}

// Keep the accumulator in DE and the byte counter in A, so HL/BC really are
// dead scratch across k*5. This checks a loop without depending on PBQP ties.
func TestMul16AccumulatorLoopDeadScratch(t *testing.T) {
	f := &Func{Name: "accumulate", nextReg: 20, Contract: Contract{
		Params:  []Param{{Reg: 1, Ty: TyU8, Class: ClassGeneral}},
		Returns: []Return{{Ty: TyU16, Class: ClassPointer}},
	}, Blocks: []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 0}, {Op: OpConst, Dst: 3, Ty: TyU16, Imm: 0}}, Term: &TermJmp{Target: "head", Args: []Reg{2, 3}}},
		{Label: "head", Params: []BlockParam{{Dst: 6, Ty: TyU8, Class: ClassGeneral}, {Dst: 7, Ty: TyU16, Class: ClassPointer}}, Insts: []*Inst{
			{Op: OpCmp, Dst: 8, Src: [2]Reg{6, 1}, Ty: TyBool, SrcTy: TyU8, Cond: CmpUlt, Cls: ClassFlag},
		}, Term: &TermBrIf{Cond: 8, Then: "body", Else: "exit"}},
		{Label: "body", Insts: []*Inst{
			{Op: OpConst, Dst: 4, Ty: TyU16, Imm: 5},
			{Op: OpMul, Dst: 9, Src: [2]Reg{6, 4}, Ty: TyU16, Cls: ClassPointer},
			{Op: OpAdd, Dst: 10, Src: [2]Reg{7, 9}, Ty: TyU16, Cls: ClassPointer},
			{Op: OpConst, Dst: 5, Ty: TyU8, Imm: 1},
			{Op: OpAdd, Dst: 11, Src: [2]Reg{6, 5}, Ty: TyU8},
		}, Term: &TermJmp{Target: "head", Args: []Reg{11, 10}}},
		{Label: "exit", Term: &TermRet{Vals: []Reg{7}}},
	}}
	m := &Module{Funcs: []*Func{f}}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
	for r, name := range map[Reg]string{1: "IXL", 2: "A", 3: "DE", 4: "BC", 5: "C", 6: "A", 7: "DE", 8: "F", 9: "HL", 10: "DE", 11: "A"} {
		ar.Locs[r] = PhysLoc{Kind: LocReg, Name: name}
	}
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(asm, "PUSH") || strings.Contains(asm, "POP") {
		t.Fatal("dead scratch saved in accumulator loop:\n" + asm)
	}
	for _, n := range []int64{0, 1, 2, 10, 200, 255} {
		want := uint16(5 * n * (n - 1) / 2)
		got, err := NewVM(m).Call(f.Name, []Value{{I: n}})
		z := runRAFragment(t, asm, emulator.Registers{IX: uint16(n)})
		if err != nil || len(got) != 1 || uint16(got[0].I) != want || z.HL != want {
			t.Fatal(n, got, err, z, want, asm)
		}
	}
}

func TestMul16GenericBCSaveLiveness(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "dead", true: "live"}[live], func(t *testing.T) {
			mul := &Inst{Op: OpMul, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
			vals := []Reg{3}
			if live {
				vals = append(vals, 4)
			}
			b := &Block{Label: "entry", Insts: []*Inst{mul}, Term: &TermRet{Vals: vals}}
			f := &Func{Name: "generic", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU16}}}, Blocks: []*Block{b}}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "DE"}, 2: {Kind: LocReg, Name: "BC"}, 3: {Kind: LocReg, Name: "HL"}, 4: {Kind: LocReg, Name: "C"}}}
			var out strings.Builder
			g := &z80cg{sb: &out, fn: f, curBlock: b, ar: ar, constVals: map[Reg]int64{2: 255}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
			g.genMul16(mul)
			if strings.Contains(out.String(), "PUSH BC") != live || strings.Contains(out.String(), "POP BC") != live {
				t.Fatal("BC save does not match liveness", out.String())
			}
			for _, x := range []uint16{0, 1, 255, 256, 32768, 65535} {
				z := runRAFragment(t, out.String(), emulator.Registers{DE: x, BC: 0x1234})
				if z.HL != x*255 || live && z.BC != 0x1234 {
					t.Fatal(x, z, out.String())
				}
			}
		})
	}
}
