package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// This regression also checks the actual implicit-counter edge convention,
// not just the presence of a mnemonic: all non-counter mutated vars must carry
// their updated values through both back and exit edges.
func TestForEachDJNZLoweringAndControlFallback(t *testing.T) {
	for _, control := range []bool{false, true} {
		body := hir.Blk(hir.Assign(hir.Var("s", mir2.TyU16), hir.Add(hir.Var("s", mir2.TyU16), hir.Cast(hir.Var("x", mir2.TyU8), mir2.TyU16), mir2.TyU16)))
		if control {
			body.Body = append([]hir.Stmt{hir.If(hir.Eq(hir.Var("x", mir2.TyU8), hir.U8(2)), hir.Blk(hir.Continue()), nil)}, body.Body...)
		}
		h := &hir.Module{Funcs: []*hir.Func{{Name: "probe", Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}}, RetTy: mir2.TyU16, Body: hir.Blk(hir.Decl("s", mir2.TyU16, hir.U16(65535)), &hir.ForEachStmt{Var: "x", ElemTy: mir2.TyU8, Ptr: hir.Addr("data"), Start: hir.U8(0), Len: hir.Var("n", mir2.TyU8), Body: body}, hir.Ret(hir.Var("s", mir2.TyU16)))}}}
		h.Globals = []mir2.Global{{Name: "data", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: 5}, Init: []byte{1, 2, 3, 4, 5}}}
		m := hir.LowerModule(h)
		if err := mir2.Verify(m); err != nil {
			t.Fatal(err)
		}
		djnz := 0
		for _, b := range m.Funcs[0].Blocks {
			if _, ok := b.Term.(*mir2.TermDJNZ); ok {
				djnz++
			}
		}
		if !control && djnz != 1 || control && djnz != 0 {
			t.Fatal("incorrect control-flow lowering", control, djnz)
		}
		for _, n := range []int64{0, 1, 5} {
			vm := mir2.NewVM(m)
			got, err := vm.Call("probe", []mir2.Value{{I: n}})
			want := uint16(65535)
			for i := int64(1); i <= n; i++ {
				if !control || i != 2 {
					want += uint16(i)
				}
			}
			if err != nil || got[0].I != int64(want) {
				t.Fatal("loop-carried independent arithmetic", control, n, got, err, want)
			}
		}
	}
}

// Calls and nested control flow must preserve the live counter and carried
// sum, even when physical register allocation differs from MIR preferences.
func TestForEachDJNZCallsVMZ80(t *testing.T) {
	for _, grace := range []bool{false, true} {
		for _, n := range []int64{0, 1, 5} {
			h := &hir.Module{Globals: []mir2.Global{{Name: "data", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: 5}, Init: []byte{1, 2, 3, 4, 5}}}}
			h.Funcs = []*hir.Func{
				{Name: "convert", Params: []hir.Param{{Name: "v", Ty: mir2.TyU8}}, RetTy: mir2.TyU16, Body: hir.Blk(hir.Ret(hir.Add(hir.Cast(hir.Var("v", mir2.TyU8), mir2.TyU16), hir.U16(257), mir2.TyU16)))},
				{Name: "probe", Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}}, RetTy: mir2.TyU16, Body: hir.Blk(
					hir.Decl("s", mir2.TyU16, hir.U16(65535)),
					&hir.ForEachStmt{Var: "x", ElemTy: mir2.TyU8, Ptr: hir.Addr("data"), Start: hir.U8(0), Len: hir.Var("n", mir2.TyU8), Body: hir.Blk(hir.Assign(hir.Var("s", mir2.TyU16), hir.Add(hir.Var("s", mir2.TyU16), hir.Call("convert", mir2.TyU16, hir.Var("x", mir2.TyU8)), mir2.TyU16)))},
					hir.Ret(hir.Var("s", mir2.TyU16)))},
			}
			want := int64(uint16(65535 + n*257 + n*(n+1)/2))
			m := hir.LowerModule(h)
			if err := mir2.Verify(m); err != nil {
				t.Fatal(err)
			}
			got, err := mir2.NewVM(m).Call("probe", []mir2.Value{{I: n}})
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatal("call arithmetic", n, got, err, want)
			}
			measureGraceZ80HIR(t, h, "probe", Options{AssertMode: "none", UseHIRGrace: grace}, []int64{n}, nil, want)
		}
	}
}
