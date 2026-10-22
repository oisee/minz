package mir2

import "testing"

func TestTruncExtensionIdentity(t *testing.T) {
	for _, op := range []Op{OpExt, OpSext} {
		for _, srcTy := range []Ty{TyU8, TyI8} {
			for _, resultTy := range []Ty{TyU8, TyI8} {
				m := &Module{}
				f := m.AddFunc("identity")
				f.Contract.Params = []Param{{Reg: 1, Ty: srcTy}}
				f.Contract.Returns = []Return{{Ty: resultTy}}
				trunc := &Inst{Op: OpTrunc, Dst: 3, Src: [2]Reg{2}, Ty: resultTy, SrcTy: TyU16}
				f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{{Op: op, Dst: 2, Src: [2]Reg{1}, Ty: TyU16, SrcTy: srcTy}, trunc}, Term: &TermRet{Vals: []Reg{3}}}}
				if !SimplifyIdentities(f) || trunc.Op != OpMove || trunc.Src[0] != 1 || trunc.Ty != resultTy || trunc.SrcTy != srcTy {
					t.Fatalf("%s %s -> %s: conversion retained or type lost: %+v", op, srcTy, resultTy, trunc)
				}
				DeadStoreElim(f)
				for _, i := range f.Blocks[0].Insts {
					if i.Op == op {
						t.Fatal("dead extension retained")
					}
				}
				if err := Verify(m); err != nil {
					t.Fatal(err)
				}
				vm := NewVM(m)
				for x := int64(0); x < 256; x++ {
					got, err := vm.Call(f.Name, []Value{{I: x}})
					if err != nil || got[0].I != x {
						t.Fatal(op, srcTy, resultTy, x, got, err)
					}
				}
			}
		}
	}
}

func TestTruncExtensionDifferentWidth(t *testing.T) {
	f := &Func{Name: "differentWidth", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8}}, Returns: []Return{{Ty: TyU16}}}, Blocks: []*Block{{Label: "entry", Insts: []*Inst{
		{Op: OpExt, Dst: 2, Src: [2]Reg{1}, Ty: TyU32, SrcTy: TyU8},
		{Op: OpTrunc, Dst: 3, Src: [2]Reg{2}, Ty: TyU16, SrcTy: TyU32},
	}, Term: &TermRet{Vals: []Reg{3}}}}}
	if err := Verify(&Module{Funcs: []*Func{f}}); err != nil {
		t.Fatal(err)
	}
	SimplifyIdentities(f)
	if f.Blocks[0].Insts[1].Op != OpTrunc {
		t.Fatal("different source/result widths folded")
	}
}

func TestTruncExtensionIdentityRefusals(t *testing.T) {
	for _, kind := range []string{"hard-ext", "hard-trunc", "redefined-source", "redefined-ext", "late-ext", "different-width"} {
		t.Run(kind, func(t *testing.T) {
			ext := &Inst{Op: OpExt, Dst: 2, Src: [2]Reg{1}, Ty: TyU16, SrcTy: TyU8}
			trunc := &Inst{Op: OpTrunc, Dst: 3, Src: [2]Reg{2}, Ty: TyU8, SrcTy: TyU16}
			b := &Block{Label: "entry", Insts: []*Inst{ext, trunc}, Term: &TermRet{Vals: []Reg{3}}}
			f := &Func{Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU8}}}, Blocks: []*Block{b}}
			switch kind {
			case "hard-ext":
				ext.ClsHard = true
			case "hard-trunc":
				trunc.ClsHard = true
			case "redefined-source":
				b.Insts = []*Inst{ext, {Op: OpConst, Dst: 1, Ty: TyU8, Imm: 9}, trunc}
			case "redefined-ext":
				b.Insts = []*Inst{ext, {Op: OpConst, Dst: 2, Ty: TyU16, Imm: 257}, trunc}
			case "late-ext":
				b.Insts = []*Inst{trunc, ext}
			case "different-width":
				trunc.Ty = TyU16
				ext.Ty = TyU32
				trunc.SrcTy = TyU32
			}
			SimplifyIdentities(f)
			if trunc.Op != OpTrunc {
				t.Fatal("unsafe identity", kind, trunc)
			}
		})
	}
}
