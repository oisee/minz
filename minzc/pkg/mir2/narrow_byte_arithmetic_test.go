package mir2

import (
	"reflect"
	"testing"
)

func truncatedArithmeticFixture(op Op) (*Module, *Func, *Inst) {
	m := &Module{}
	f := m.AddFunc("arith")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassGeneral}, {Reg: 2, Ty: TyU8, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}, {Ty: TyI16, Class: ClassGeneral}}
	narrow := &Inst{Op: OpTrunc, Dst: 6, Src: [2]Reg{5}, Ty: TyU8, SrcTy: TyI16, Cls: ClassAcc, SrcFile: "arithmetic.c", SrcLine: 11}
	f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{
		{Op: OpExt, Dst: 3, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyI16},
		{Op: OpExt, Dst: 4, Src: [2]Reg{2}, SrcTy: TyU8, Ty: TyI16},
		{Op: op, Dst: 5, Src: [2]Reg{3, 4}, Ty: TyI16, Cls: ClassPointer}, narrow,
	}, Term: &TermRet{Vals: []Reg{6, 5}}}}
	return m, f, narrow
}

// C90 6.2.1.1 promotes both bytes to int. Sums 0..510 and differences
// -255..255 fit int16; conversion back to unsigned char is modulo 256.
func TestNarrowByteArithmeticAllPairs(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub} {
		m, f, i := truncatedArithmeticFixture(op)
		wide := *f.Blocks[0].Insts[2]
		if !SimplifyIdentities(f) || i.Op != op || i.Src != [2]Reg{1, 2} || i.SrcTy != TyU8 || i.SrcLine != 11 || i.SrcFile != "arithmetic.c" || !reflect.DeepEqual(wide, *f.Blocks[0].Insts[2]) {
			t.Fatal("rewrite or surviving wide user", op)
		}
		if e := Verify(m); e != nil {
			t.Fatal(e)
		}
		vm := NewVM(m)
		for x := int64(0); x < 256; x++ {
			for y := int64(0); y < 256; y++ {
				want := x + y
				if op == OpSub {
					want = x - y
				}
				out, e := vm.Call("arith", []Value{{I: x}, {I: y}})
				if e != nil || len(out) != 2 || out[0].I != int64(uint8(want)) || out[1].I != int64(uint16(want)) {
					t.Fatal(op, x, y, out, e)
				}
			}
		}
	}
}

func TestNarrowByteArithmeticRefusals(t *testing.T) {
	for _, kind := range []string{"signed-source", "sext", "hard-ext", "hard-wide", "hard-trunc", "late-wide", "mixed-type", "word-result", "redefined-source", "redefined-ext", "redefined-wide", "asm", "flag-return", "fallible"} {
		t.Run(kind, func(t *testing.T) {
			_, f, i := truncatedArithmeticFixture(OpSub)
			b := f.Blocks[0]
			switch kind {
			case "signed-source":
				b.Insts[0].SrcTy = TyI8
			case "sext":
				b.Insts[0].Op = OpSext
			case "hard-ext":
				b.Insts[0].ClsHard = true
			case "hard-wide":
				b.Insts[2].ClsHard = true
			case "hard-trunc":
				i.ClsHard = true
			case "late-wide":
				b.Insts = []*Inst{b.Insts[0], b.Insts[1], i, b.Insts[2]}
			case "mixed-type":
				b.Insts[0].Ty = TyU16
			case "word-result":
				i.Ty = TyU16
			case "redefined-source":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 9})
			case "redefined-ext":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: 3, Ty: TyI16, Imm: 999})
			case "redefined-wide":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: 5, Ty: TyI16, Imm: 999})
			case "asm":
				b.Insts = append(b.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "ADC A,0"}})
			case "flag-return":
				f.Contract.Returns[0].Class = ClassFlag
			case "fallible":
				f.Attrs.FallibleCY = true
			}
			before := *i
			if narrowTruncatedByteArithmetic(f) || !reflect.DeepEqual(before, *i) {
				t.Fatal("unsafe rewrite", kind)
			}
		})
	}
}
