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
	for _, op := range []Op{OpAdd, OpSub, OpAnd, OpOr, OpXor} {
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
				switch op {
				case OpSub:
					want = x - y
				case OpAnd:
					want = x & y
				case OpOr:
					want = x | y
				case OpXor:
					want = x ^ y
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
	for _, kind := range []string{"signed-source", "sext", "hard-ext", "hard-wide", "hard-trunc", "late-wide", "mixed-type", "word-result", "redefined-source", "redefined-ext", "redefined-wide", "redefined-trunc", "asm", "flag-return", "fallible"} {
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
			case "redefined-trunc":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: 6, Ty: TyU8, Imm: 9})
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

// Exercise promoted C expression trees while preserving a second word user.
func TestDemandedLowByteArithmetic(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub, OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar} {
		t.Run(op.String(), func(t *testing.T) {
			m, f, trunc := truncatedArithmeticFixture(op)
			b := f.Blocks[0]
			b.Insts[1] = &Inst{Op: OpConst, Dst: 4, Ty: TyI16, Imm: 3}
			b.Insts = append(b.Insts[:3], &Inst{Op: OpConst, Dst: 7, Ty: TyI16, Imm: 263}, &Inst{Op: OpXor, Dst: 8, Ty: TyI16, Src: [2]Reg{5, 7}}, trunc)
			trunc.Src[0] = 8
			if !narrowTruncatedByteArithmetic(f) || trunc.Op != OpXor {
				t.Fatal("promoted tree was not narrowed")
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			vm := NewVM(m)
			for x := int64(0); x < 256; x++ {
				var wide int64
				switch op {
				case OpAdd:
					wide = x + 3
				case OpSub:
					wide = x - 3
				case OpAnd:
					wide = x & 3
				case OpOr:
					wide = x | 3
				case OpXor:
					wide = x ^ 3
				case OpShl:
					wide = x << 3
				case OpShr, OpSar:
					wide = x >> 3
				}
				out, err := vm.Call("arith", []Value{{I: x}, {I: 0}})
				if err != nil || len(out) != 2 || out[0].I != int64(uint8(wide^263)) || out[1].I != int64(uint16(wide)) {
					t.Fatal(x, out, err, wide)
				}
			}
		})
	}
}

func TestDemandedLowByteShiftRefusals(t *testing.T) {
	for _, op := range []Op{OpShl, OpShr, OpSar} {
		for _, k := range []int64{-1, 8, 16, 256} {
			_, f, _ := truncatedArithmeticFixture(op)
			f.Blocks[0].Insts[1] = &Inst{Op: OpConst, Dst: 4, Ty: TyI16, Imm: k}
			if narrowTruncatedByteArithmetic(f) {
				t.Fatal(op, k)
			}
		}
	}
	for _, op := range []Op{OpShr, OpSar} {
		_, f, _ := truncatedArithmeticFixture(op)
		f.Blocks[0].Insts[0] = &Inst{Op: OpConst, Dst: 3, Ty: TyI16, Imm: 0x100}
		f.Blocks[0].Insts[1] = &Inst{Op: OpConst, Dst: 4, Ty: TyI16, Imm: 1}
		if narrowTruncatedByteArithmetic(f) {
			t.Fatal("right shift needs the high byte", op)
		}
	}
}

func TestDemandedLowByteConsumers(t *testing.T) {
	for _, op := range []Op{OpStore, OpCmp} {
		m, f, trunc := truncatedArithmeticFixture(OpAdd)
		b := f.Blocks[0]
		// Two byte consumers of the same wide result reuse one clone, while
		// the word return and comparison still see the unmodified sum.
		f.Contract.Params = append(f.Contract.Params, Param{Reg: 9, Ty: TyPtr, Class: ClassPointer})
		b.Insts = append(b.Insts[:3], &Inst{Op: OpStore, Ty: TyU8, Src: [2]Reg{9, 5}}, trunc,
			&Inst{Op: OpCmp, Dst: 10, Ty: TyBool, SrcTy: TyI16, Cond: CmpEq, Src: [2]Reg{5, 3}})
		if op == OpCmp {
			b.Insts[3] = &Inst{Op: OpCmp, Dst: 11, Ty: TyBool, SrcTy: TyU8, Cond: CmpEq, Src: [2]Reg{3, 4}}
		}
		if !narrowTruncatedByteArithmetic(f) {
			t.Fatal("no rewrite")
		}
		byteAdds := 0
		for _, i := range b.Insts {
			if i.Dst == 10 && i.Src != [2]Reg{5, 3} {
				t.Fatal("word compare narrowed")
			}
			if i.Op == OpStore && i.Src[1] == 5 {
				t.Fatal("store still consumes the word sum")
			}
			if i.Dst == 11 && i.Src != [2]Reg{1, 2} {
				t.Fatal("byte compare retains zero extensions")
			}
			if i.Op == OpAdd && i.Ty == TyU8 {
				byteAdds++
			}
		}
		if byteAdds != 1 {
			t.Fatal("byte consumers should share one byte sum", byteAdds)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		vm := NewVM(m)
		for x := int64(0); x < 256; x++ {
			out, err := vm.Call("arith", []Value{{I: x}, {I: 255}, {I: 0}})
			if err != nil || len(out) != 2 || out[0].I != int64(uint8(x+255)) || out[1].I != x+255 {
				t.Fatal(op, x, out, err)
			}
			if op == OpStore && vm.ReadHeap(0, 1)[0] != byte(x+255) {
				t.Fatal("store", x)
			}
		}
	}
}

func TestDemandedLowByteDepthLimit(t *testing.T) {
	_, f, trunc := truncatedArithmeticFixture(OpAdd)
	b := f.Blocks[0]
	b.Insts = b.Insts[:3]
	r := Reg(5)
	for n := Reg(10); n < 24; n++ {
		b.Insts = append(b.Insts, &Inst{Op: OpXor, Dst: n, Ty: TyI16, Src: [2]Reg{r, 3}})
		r = n
	}
	trunc.Src[0] = r
	b.Insts = append(b.Insts, trunc)
	if narrowTruncatedByteArithmetic(f) {
		t.Fatal("excessive recursion")
	}
}

func TestDemandedLowByteRightShiftOfSumRefused(t *testing.T) {
	for _, op := range []Op{OpShr, OpSar} {
		_, f, trunc := truncatedArithmeticFixture(OpAdd)
		b := f.Blocks[0]
		b.Insts = append(b.Insts[:3], &Inst{Op: OpConst, Dst: 7, Ty: TyI16, Imm: 1}, &Inst{Op: op, Dst: 8, Ty: TyI16, Src: [2]Reg{5, 7}}, trunc)
		trunc.Src[0] = 8
		if narrowTruncatedByteArithmetic(f) {
			t.Fatal("sum may carry into high byte", op)
		}
	}
}

func TestDemandedLowByteLeavesAreNotRoots(t *testing.T) {
	for _, op := range []Op{OpExt, OpConst} {
		_, f, trunc := truncatedArithmeticFixture(OpAdd)
		trunc.Src[0] = 3
		if op == OpConst {
			f.Blocks[0].Insts[0] = &Inst{Op: OpConst, Dst: 3, Ty: TyI16, Imm: 263}
		}
		if narrowTruncatedByteArithmetic(f) {
			t.Fatal("standalone conversion belongs to conversion folding", op)
		}
	}
}
