package mir2

import "testing"

func TestBytePairProofBoundaries(t *testing.T) {
	for _, kind := range []string{"or", "add", "signed", "shift", "shared", "flags"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("pack")
			lo, hi := f.AllocReg(), f.AllocReg()
			f.Contract.Params = []Param{{Reg: lo, Ty: TyU8}, {Reg: hi, Ty: TyU8}}
			f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			l := b.Ext(lo, TyU8, TyU16, ClassPair)
			h := b.Ext(hi, TyU8, TyU16, ClassPair)
			k := b.Const(8, TyU16, ClassPair)
			shift := b.BinOp(OpShl, h, k, TyU16, ClassPair)
			op := OpOr
			if kind == "add" {
				op = OpAdd
			}
			out := b.BinOp(op, l, shift, TyU16, ClassPair)
			b.Ret(out)
			switch kind {
			case "signed":
				f.Blocks[0].Insts[1].Op = OpSext
				f.Blocks[0].Insts[1].SrcTy = TyI8
			case "shift":
				f.Blocks[0].Insts[2].Imm = 7
			case "shared":
				f.Blocks[0].Term = &TermRet{Vals: []Reg{out, h}}
			case "flags":
				f.Contract.Returns[0].Class = ClassFlag
			}
			n := PackBytePairs(f)
			want := 0
			if kind == "or" || kind == "add" {
				want = 1
			}
			if n != want {
				t.Fatalf("pack count=%d want=%d", n, want)
			}
		})
	}
}
