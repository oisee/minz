package hir_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// The expected results come from the source loop's iteration order, rather
// than comparing two variants produced by the same lowering implementation.
func TestLowerLoopTerminalControl(t *testing.T) {
	for _, kind := range []string{"while", "range", "each"} {
		for _, control := range []string{"return", "break", "continue", "conditional-return", "conditional-break", "conditional-continue"} {
			for _, n := range []int64{0, 1, 2, 4} {
				t.Run(fmt.Sprintf("%s/%s/n%d", kind, control, n), func(t *testing.T) {
					s := hir.Var("s", mir2.TyU8)
					i := hir.Var("i", mir2.TyU8)
					body := hir.Blk()
					// The while induction step precedes control transfers; ForRange's
					// induction step belongs to the loop and must also run on continue.
					if kind == "while" {
						body.Body = append(body.Body, hir.Assign(i, hir.Add(i, hir.U8(1), mir2.TyU8)))
					}
					element := i
					second := int64(1)
					if kind == "while" {
						second = 2
					}
					if kind == "each" {
						element = hir.Var("x", mir2.TyU8)
						second = 2
					}
					var transfer hir.Stmt
					switch control {
					case "return", "conditional-return":
						transfer = hir.Ret(hir.U8(9))
					case "break", "conditional-break":
						transfer = hir.Break()
					default:
						transfer = hir.Continue()
					}
					conditional := len(control) > len("conditional-") && control[:len("conditional-")] == "conditional-"
					if conditional {
						body.Body = append(body.Body, hir.If(&hir.BinExpr{Op: "==", L: element, R: hir.U8(second), Ty: mir2.TyBool}, hir.Blk(transfer), nil))
						body.Body = append(body.Body, hir.Assign(s, hir.Add(s, hir.U8(1), mir2.TyU8)))
					} else {
						body.Body = append(body.Body, hir.Assign(s, hir.Add(s, hir.U8(1), mir2.TyU8)), transfer)
					}
					var loop hir.Stmt
					switch kind {
					case "while":
						loop = &hir.WhileStmt{Cond: &hir.BinExpr{Op: "<", L: i, R: hir.U8(n), Ty: mir2.TyBool}, Body: body}
					case "range":
						loop = &hir.ForRangeStmt{Var: "i", Start: hir.U8(0), End: hir.U8(n), Body: body}
					case "each":
						loop = &hir.ForEachStmt{Var: "x", ElemTy: mir2.TyU8, Ptr: hir.Addr("data"), Start: hir.U8(0), Len: hir.U8(n), Body: body}
					}
					hm := &hir.Module{Globals: []mir2.Global{{Name: "data", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: 4}, Init: []byte{1, 2, 3, 4}}}, Funcs: []*hir.Func{{Name: "probe", RetTy: mir2.TyU8, Body: hir.Blk(hir.Decl("s", mir2.TyU8, hir.U8(0)), hir.Decl("i", mir2.TyU8, hir.U8(0)), loop, hir.Ret(s))}}}
					want := n
					switch control {
					case "return":
						if n > 0 {
							want = 9
						}
					case "break":
						if n > 0 {
							want = 1
						}
					case "conditional-return":
						if n >= 2 {
							want = 9
						}
					case "conditional-break":
						if n >= 2 {
							want = 1
						}
					case "conditional-continue":
						if n >= 2 {
							want = n - 1
						}
					}
					runLoweredControlOracle(t, hm, want)
				})
			}
		}
	}
}

func TestLowerNestedLoopControl(t *testing.T) {
	for _, innerKind := range []string{"while", "range", "each"} {
		t.Run(innerKind, func(t *testing.T) {
			s := hir.Var("s", mir2.TyU8)
			j := hir.Var("j", mir2.TyU8)
			innerBody := hir.Blk()
			elem := j
			skip, stop := int64(1), int64(3)
			if innerKind == "while" {
				innerBody.Body = append(innerBody.Body, hir.Assign(j, hir.Add(j, hir.U8(1), mir2.TyU8)))
				skip = 2
				stop = 4
			}
			if innerKind == "each" {
				elem = hir.Var("y", mir2.TyU8)
				skip = 2
				stop = 4
			}
			innerBody.Body = append(innerBody.Body,
				hir.If(&hir.BinExpr{Op: "==", L: elem, R: hir.U8(skip), Ty: mir2.TyBool}, hir.Blk(hir.Continue()), nil),
				hir.If(&hir.BinExpr{Op: "==", L: elem, R: hir.U8(stop), Ty: mir2.TyBool}, hir.Blk(hir.Break()), nil),
				hir.Assign(s, hir.Add(s, hir.U8(1), mir2.TyU8)))
			var inner hir.Stmt
			switch innerKind {
			case "while":
				inner = &hir.WhileStmt{Cond: &hir.BinExpr{Op: "<", L: j, R: hir.U8(4), Ty: mir2.TyBool}, Body: innerBody}
			case "range":
				inner = &hir.ForRangeStmt{Var: "j", Start: hir.U8(0), End: hir.U8(4), Body: innerBody}
			case "each":
				inner = &hir.ForEachStmt{Var: "y", ElemTy: mir2.TyU8, Ptr: hir.Addr("data"), Start: hir.U8(0), Len: hir.U8(4), Body: innerBody}
			}
			outerBody := hir.Blk(hir.If(&hir.BinExpr{Op: "==", L: hir.Var("i", mir2.TyU8), R: hir.U8(1), Ty: mir2.TyBool}, hir.Blk(hir.Continue()), nil), hir.Assign(j, hir.U8(0)), inner)
			hm := &hir.Module{Globals: []mir2.Global{{Name: "data", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: 4}, Init: []byte{1, 2, 3, 4}}}, Funcs: []*hir.Func{{Name: "probe", RetTy: mir2.TyU8, Body: hir.Blk(hir.Decl("s", mir2.TyU8, hir.U8(0)), hir.Decl("j", mir2.TyU8, hir.U8(0)), &hir.ForRangeStmt{Var: "i", Start: hir.U8(0), End: hir.U8(3), Body: outerBody}, hir.Ret(s))}}}
			// Outer i=0,2 execute; each inner loop counts only its first and third elements.
			runLoweredControlOracle(t, hm, 4)
		})
	}
}

func runLoweredControlOracle(t *testing.T, hm *hir.Module, want int64) {
	t.Helper()
	m := hir.LowerModule(hm)
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("MIR verification: %v\n%s", err, m.Dump())
	}
	vm := mir2.NewVM(m)
	vm.MaxSteps = 1000
	vm.EnsureHeap(4)
	vm.WriteHeapSlice(0, []byte{1, 2, 3, 4})
	got, err := vm.Call("probe", nil)
	if err != nil || len(got) != 1 || got[0].I != want {
		t.Fatalf("VM got %v error %v; want %d\n%s", got, err, want, m.Dump())
	}
}
