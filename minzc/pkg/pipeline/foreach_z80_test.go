package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestForEachZ80PreservesAccumulatorAcrossFilter(t *testing.T) {
	for _, data := range [][]byte{{3, 2, 5, 7}, {2, 3, 5, 2}, {2, 2, 2, 2}} {
		for _, hasContinue := range []bool{false, true} {
			for _, useMIR := range []bool{false, true} {
				body := hir.Blk()
				if hasContinue {
					body.Body = append(body.Body, hir.If(&hir.BinExpr{Op: "==", L: hir.Var("x", mir2.TyU8), R: hir.U8(2), Ty: mir2.TyBool}, hir.Blk(hir.Continue()), nil))
				}
				body.Body = append(body.Body, hir.Assign(hir.Var("s", mir2.TyU8), hir.Add(hir.Var("s", mir2.TyU8), hir.Var("x", mir2.TyU8), mir2.TyU8)))
				h := &hir.Module{Funcs: []*hir.Func{{Name: "foreach_probe", Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(hir.Decl("s", mir2.TyU8, hir.U8(0)), &hir.ForEachStmt{Var: "x", ElemTy: mir2.TyU8, Ptr: hir.Addr("data"), Start: hir.U8(0), Len: hir.Var("n", mir2.TyU8), Body: body}, hir.Ret(hir.Var("s", mir2.TyU8)))}}}
				h.Globals = []mir2.Global{{Name: "data", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: 4}, Init: data}}
				for _, n := range []int64{0, 1, 2, 3, 4} {
					want := int64(0)
					for _, x := range data[:n] {
						if !hasContinue || x != 2 {
							want += int64(x)
						}
					}
					h.Asserts = append(h.Asserts, hir.Assert{FuncName: "foreach_probe", Args: []int64{n}, Expected: want, Via: "z80"})
				}
				steps, err := CompileHIRSteps(h, Options{UseGrace: useMIR, AssertMode: "z80"})
				if err != nil {
					t.Fatalf("data=%v continue=%v MIRGrace=%v: %v", data, hasContinue, useMIR, err)
				}

				for _, n := range []int64{0, 1, 2, 3, 4} {
					vm := mir2.NewVM(steps.MIR2Module)
					vm.EnsureHeap(4)
					vm.WriteHeapSlice(0, data)
					want := int64(0)
					for _, x := range data[:n] {
						if !hasContinue || x != 2 {
							want += int64(x)
						}
					}
					got, err := vm.Call("foreach_probe", []mir2.Value{{I: n}})
					if err != nil || len(got) != 1 || got[0].I != want {
						t.Fatalf("continue=%v MIRGrace=%v n%d got%v %v want%d", hasContinue, useMIR, n, got, err, want)
					}
				}
			}
		}
	}
}
