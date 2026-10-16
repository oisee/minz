package hir_test

import (
	"encoding/binary"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestAddressableParameterShadowsGlobal(t *testing.T) {
	makeModule := func() *hir.Module {
		x := hir.Var("x", mir2.TyU16)
		f := &hir.Func{Name: "probe", Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, RetTy: mir2.TyU16, Body: hir.Blk(
			&hir.VarDeclStmt{Name: "p", Ty: mir2.TyPtr, Init: hir.AddrExpr(x)},
			&hir.AssignStmt{Target: &hir.DerefExpr{Ptr: hir.Var("p", mir2.TyPtr), Ty: mir2.TyU16}, Val: &hir.IntLitExpr{Val: 7, Ty: mir2.TyU16}},
			hir.Ret(x))}
		return &hir.Module{Name: "shadow", Globals: []mir2.Global{{Name: "x", Ty: mir2.TyU16, Init: []byte{99, 0}}}, Funcs: []*hir.Func{f}, Asserts: []hir.Assert{{FuncName: "probe", Args: []int64{2}, Expected: 7}}}
	}
	raw := hir.LowerModule(makeModule())
	vm := mir2.NewVM(raw)
	v, err := vm.Call("probe", []mir2.Value{{I: 2}})
	if err != nil || len(v) != 1 || v[0].I != 7 {
		t.Fatalf("raw result=%v error=%v", v, err)
	}
	if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("x"), 2)); got != 99 {
		t.Fatalf("address of parameter selected global x=%d", got)
	}
	m := makeModule()
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err = pipeline.RunAssertsMIR2(m, steps.MIR2Module); err != nil {
		t.Fatal(err)
	}
	if err = pipeline.RunAssertsZ80(m, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		t.Fatal(err)
	}
}
