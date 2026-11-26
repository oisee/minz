package mir2

import (
	"reflect"
	"testing"
)

// mulFixture builds trunc(mul(lhs, rhs)) with a word-typed product that is
// also returned, so the wide consumer must survive any rewrite.
func mulFixture(lhs, rhs *Inst) (*Module, *Func, *Inst, *Inst) {
	m := &Module{}
	f := m.AddFunc("mulf")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassGeneral}, {Reg: 2, Ty: TyU8, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}, {Ty: TyI16, Class: ClassGeneral}}
	lhs.Dst, rhs.Dst = 3, 4
	mul := &Inst{Op: OpMul, Dst: 5, Src: [2]Reg{3, 4}, Ty: TyI16, Cls: ClassPointer}
	trunc := &Inst{Op: OpTrunc, Dst: 6, Src: [2]Reg{5}, Ty: TyU8, SrcTy: TyI16, Cls: ClassAcc}
	f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{lhs, rhs, mul, trunc}, Term: &TermRet{Vals: []Reg{6, 5}}}}
	return m, f, mul, trunc
}

func extU8(src Reg) *Inst { return &Inst{Op: OpExt, Src: [2]Reg{src}, SrcTy: TyU8, Ty: TyI16} }
func constI16(v int64) *Inst {
	return &Inst{Op: OpConst, Imm: v, Ty: TyI16}
}

// A constant factor is admitted from either side, including constants whose
// low byte differs from the constant (>= 256 and negative). The low byte of a
// product depends only on the low bytes of the factors, and the wide product
// keeps its other consumer.
func TestNarrowByteMulConstantFactor(t *testing.T) {
	for _, swap := range []bool{false, true} {
		for _, k := range []int64{0, 1, 2, 3, 7, 10, 100, 255, 256, 300, 0x1234, -1, -7} {
			lhs, rhs := extU8(1), constI16(k)
			if swap {
				lhs, rhs = rhs, lhs
			}
			m, f, mul, trunc := mulFixture(lhs, rhs)
			wide := *mul
			if !narrowTruncatedByteArithmetic(f) || trunc.Op != OpMul || trunc.Ty != TyU8 || trunc.SrcTy != TyU8 || !reflect.DeepEqual(wide, *mul) {
				t.Fatal("not narrowed", swap, k, trunc.Op)
			}
			// The byte multiply keeps the constant second.
			c := trunc.Src[1]
			var cdef *Inst
			for _, i := range f.Blocks[0].Insts {
				if i.Dst == c {
					cdef = i
				}
			}
			if trunc.Src[0] != 1 || cdef == nil || cdef.Op != OpConst || cdef.Imm != int64(uint8(k)) {
				t.Fatal("operand order or constant", swap, k, trunc.Src, cdef)
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			vm := NewVM(m)
			for x := int64(0); x < 256; x++ {
				out, err := vm.Call("mulf", []Value{{I: x}, {I: 0}})
				if err != nil || len(out) != 2 || out[0].I != int64(uint8(x*k)) || out[1].I != int64(uint16(x*k)) {
					t.Fatal(swap, k, x, out, err)
				}
			}
		}
	}
}

// A product of two variable bytes is slower in byte form than the word form
// the generator already optimises, so it is not narrowed.
func TestNarrowByteMulRefusals(t *testing.T) {
	for _, kind := range []string{"variable-variable", "const-const", "word-operand", "signed-source", "sext", "wide-nonbyte-trunc-consumer", "hard-mul", "redefined-mul"} {
		t.Run(kind, func(t *testing.T) {
			lhs, rhs := extU8(1), constI16(3)
			switch kind {
			case "variable-variable":
				rhs = extU8(2)
			case "const-const":
				lhs = constI16(5)
			case "word-operand":
				// A 16-bit parameter is not a zero-extended byte.
				lhs = &Inst{Op: OpMove, Src: [2]Reg{1}, Ty: TyI16}
			case "signed-source":
				lhs.SrcTy = TyI8
			case "sext":
				lhs.Op = OpSext
			}
			_, f, mul, trunc := mulFixture(lhs, rhs)
			b := f.Blocks[0]
			switch kind {
			case "wide-nonbyte-trunc-consumer":
				trunc.Ty = TyU16
			case "hard-mul":
				mul.ClsHard = true
			case "redefined-mul":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: 5, Ty: TyI16, Imm: 9})
			}
			if narrowTruncatedByteArithmetic(f) || trunc.Op != OpTrunc || mul.Ty != TyI16 {
				t.Fatal("unsafe rewrite")
			}
		})
	}
}

// Products nested in other admitted arithmetic keep the proof: (x*3 + 5)*7.
func TestNarrowByteMulNested(t *testing.T) {
	m, f, _, trunc := mulFixture(extU8(1), constI16(3))
	b := f.Blocks[0]
	b.Insts = append(b.Insts[:3],
		&Inst{Op: OpConst, Dst: 7, Ty: TyI16, Imm: 5},
		&Inst{Op: OpAdd, Dst: 8, Ty: TyI16, Src: [2]Reg{5, 7}},
		&Inst{Op: OpConst, Dst: 9, Ty: TyI16, Imm: 263},
		&Inst{Op: OpMul, Dst: 10, Ty: TyI16, Src: [2]Reg{9, 8}},
		trunc)
	trunc.Src[0] = 10
	if !narrowTruncatedByteArithmetic(f) || trunc.Op != OpMul {
		t.Fatal("nested product not narrowed")
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	vm := NewVM(m)
	for x := int64(0); x < 256; x++ {
		out, err := vm.Call("mulf", []Value{{I: x}, {I: 0}})
		if err != nil || len(out) != 2 || out[0].I != int64(uint8((x*3+5)*263)) || out[1].I != int64(uint16(x*3)) {
			t.Fatal(x, out, err)
		}
	}
}
