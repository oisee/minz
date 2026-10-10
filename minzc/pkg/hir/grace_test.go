package hir_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func graceLower(t *testing.T, h *hir.Module) *mir2.Module {
	t.Helper()
	m := hir.LowerModule(h)
	if err := mir2.Verify(m); err != nil {
		t.Fatal(err)
	}
	return m
}

// This observation compares program-visible effects, not internal registers,
// block layouts or instruction counts. Each side gets fresh VM state.
type graceObservation struct {
	Returns []mir2.Value
	Memory  []byte
	Events  []int64
	Error   string
}

func graceObserve(m *mir2.Module, x int64) graceObservation {
	vm := mir2.NewVM(m)
	vm.MaxSteps = 10000
	o := graceObservation{}
	vm.Hosts["record"] = func(args []mir2.Value) ([]mir2.Value, error) {
		o.Events = append(o.Events, args[0].I)
		return nil, nil
	}
	var err error
	o.Returns, err = vm.Call("test", []mir2.Value{{I: x}})
	if err != nil {
		o.Error = err.Error()
	}
	if addr := vm.GlobalAddr("state"); addr >= 0 {
		o.Memory = append([]byte(nil), vm.ReadHeap(addr, 1)...)
	}
	return o
}

func TestGraceHIRConstantIfDifferentialVM(t *testing.T) {
	for _, truth := range []bool{false, true} {
		t.Run(fmt.Sprint(truth), func(t *testing.T) {
			build := func() *hir.Module {
				x := hir.Var("x", mir2.TyU8)
				arm := func(n int64) *hir.Block {
					value := hir.Add(x, hir.U8(n), mir2.TyU8)
					return hir.Blk(
						&hir.StoreStmt{Ptr: hir.Addr("state"), Val: value},
						&hir.ExprStmt{Expr: hir.Call("record", mir2.TyVoid, value)},
						hir.Ret(value),
					)
				}
				return &hir.Module{Name: "grace", Globals: []mir2.Global{{Name: "state", Ty: mir2.TyU8}}, Funcs: []*hir.Func{
					{Name: "record", IsExtern: true, IsIO: true, Params: []hir.Param{{Name: "v", Ty: mir2.TyU8}}, RetTy: mir2.TyVoid},
					{Name: "test", Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}}, RetTy: mir2.TyU8,
						Body: hir.Blk(&hir.ExprStmt{Expr: hir.Call("record", mir2.TyVoid, x)}, hir.If(hir.Bool(truth), arm(1), arm(2)))},
				}}
			}
			baseline := graceLower(t, build())
			h := build()
			stats, err := hir.RunGrace(h, 0)
			if err != nil {
				t.Fatal(err)
			}
			if stats.Applied != 1 || stats.ByRule["hir-constant-if"] != 1 {
				t.Fatalf("expected one firing: %+v", stats)
			}
			optimized := graceLower(t, h)
			if baseline.Dump() == optimized.Dump() {
				t.Fatal("rule did not change MIR2 shape")
			}
			for x := int64(0); x < 256; x++ {
				a, b := graceObserve(baseline, x), graceObserve(optimized, x)
				if !reflect.DeepEqual(a, b) {
					t.Fatalf("x=%d: before=%+v after=%+v", x, a, b)
				}
				n := int64(2)
				if truth {
					n = 1
				}
				want := int64(uint8(x + n)) // independent width/overflow oracle
				if b.Error != "" || len(b.Returns) != 1 || b.Returns[0].I != want ||
					len(b.Events) != 2 || b.Events[0] != x || b.Events[1] != want || len(b.Memory) != 1 || int64(b.Memory[0]) != want {
					t.Fatalf("x=%d: observed %+v, want return/store %d and events [%d %d]", x, b, want, x, want)
				}
			}
			second, err := hir.RunGrace(h, 0)
			if err != nil || second.Applied != 0 {
				t.Fatalf("not idempotent: %+v %v", second, err)
			}
		})
	}
}

func TestGraceHIRNestedLoopAndBudget(t *testing.T) {
	build := func() *hir.Module {
		x := hir.Var("x", mir2.TyU8)
		return &hir.Module{Name: "loop", Funcs: []*hir.Func{{Name: "test", Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}}, RetTy: mir2.TyU8,
			Body: hir.Blk(hir.While(hir.Gt(x, hir.U8(0)), hir.Blk(
				hir.If(hir.Bool(true), hir.Blk(
					hir.If(hir.Bool(false), hir.Blk(hir.Break()), nil),
					hir.Assign(x, hir.Sub(x, hir.U8(1), mir2.TyU8)),
				), nil),
			)), hir.Ret(x)),
		}}}
	}
	baseline := graceLower(t, build())
	h := build()
	first, err := hir.RunGrace(h, 1)
	if err != nil || first.Applied != 1 {
		t.Fatalf("budget: %+v %v", first, err)
	}
	second, err := hir.RunGrace(h, 1)
	if err != nil || second.Applied != 1 {
		t.Fatalf("second firing: %+v %v", second, err)
	}
	m := graceLower(t, h)
	for _, x := range []int64{0, 1, 2, 127, 255} {
		a, b := graceObserve(baseline, x), graceObserve(m, x)
		if !reflect.DeepEqual(a, b) || b.Error != "" || len(b.Returns) != 1 || b.Returns[0].I != 0 {
			t.Fatalf("x=%d: before=%+v after=%+v", x, a, b)
		}
	}
}

func TestGraceHIRPreservesTraps(t *testing.T) {
	for _, selectedTrap := range []bool{false, true} {
		build := func() *hir.Module {
			trap := hir.Blk(hir.Ret(&hir.BinExpr{Op: "/", L: hir.U8(1), R: hir.U8(0), Ty: mir2.TyU8}))
			return &hir.Module{Funcs: []*hir.Func{{Name: "test", Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}}, RetTy: mir2.TyU8,
				Body: hir.Blk(hir.If(hir.Bool(selectedTrap), trap, hir.Blk(hir.Ret(hir.U8(42))))),
			}}}
		}
		before := graceObserve(graceLower(t, build()), 0)
		h := build()
		if _, err := hir.RunGrace(h, 0); err != nil {
			t.Fatal(err)
		}
		after := graceObserve(graceLower(t, h), 0)
		// Trap diagnostics contain changed block labels; compare the cause.
		if selectedTrap {
			if !strings.Contains(before.Error, "division by zero") || !strings.Contains(after.Error, "division by zero") {
				t.Fatalf("trap lost: before=%+v after=%+v", before, after)
			}
		} else if !reflect.DeepEqual(before, after) || after.Error != "" || after.Returns[0].I != 42 {
			t.Fatalf("discarded trap executed: before=%+v after=%+v", before, after)
		}
	}
}

func TestGraceHIRConservativeGuards(t *testing.T) {
	for _, tc := range []struct {
		name string
		body *hir.Block
	}{
		{"dynamic", hir.Blk(hir.If(hir.Call("condition", mir2.TyBool), hir.Blk(hir.Ret(hir.U8(1))), nil))},
		{"binding", hir.Blk(hir.If(hir.Bool(true), hir.Blk(hir.Decl("x", mir2.TyU8, hir.U8(1))), nil))},
		{"goto", hir.Blk(&hir.GotoStmt{Label: "inside"}, hir.If(hir.Bool(false), hir.Blk(&hir.LabelStmt{Name: "inside"}), nil))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &hir.Module{Funcs: []*hir.Func{{Name: "test", Body: tc.body, RetTy: mir2.TyU8}}}
			before := h.Dump()
			s, err := hir.RunGrace(h, 0)
			if err != nil || s.Applied != 0 || before != h.Dump() {
				t.Fatalf("guard: %+v %v", s, err)
			}
		})
	}
	if _, err := hir.RunGrace(nil, 0); err == nil {
		t.Fatal("accepted nil module")
	}
	if _, err := hir.RunGrace(&hir.Module{}, -1); err == nil {
		t.Fatal("accepted negative budget")
	}
}
