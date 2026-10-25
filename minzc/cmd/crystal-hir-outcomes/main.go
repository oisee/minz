// Research: adapt Crystal's explicit outcomes to the existing Nanz CY ABI and
// prove success, propagation, error codes and stack restoration on MIR2/Z80.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

type testCase struct {
	Symbol string
	Args   []int64
	Failed bool
	Value  int64
	Code   byte
}

func call(fn string, args []hir.Expr, ty mir2.Ty) *hir.CallExpr {
	return &hir.CallExpr{Fn: fn, Args: args, Ty: ty}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: crystal-hir-outcomes input.lanz cases.json output-prefix")
	}
	src, err := os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}
	m, err := lanz.Compile(string(src), "crystal_outcomes")
	if err != nil {
		return err
	}
	var defs []*hir.Func
	for _, f := range m.Funcs {
		if f.Body != nil {
			defs = append(defs, f)
		}
	}
	m.Funcs, m.Asserts = defs, nil
	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		return err
	}
	var cases []testCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return err
	}
	wrapped := map[string]bool{}
	for i, tc := range cases {
		f := m.FuncByName(tc.Symbol)
		if f == nil || f.RetTy != mir2.TyU16 {
			return fmt.Errorf("missing encoded outcome function %s", tc.Symbol)
		}
		if !wrapped[f.Name] {
			wrapped[f.Name] = true
			args := make([]hir.Expr, len(f.Params))
			for j, p := range f.Params {
				args[j] = hir.Var(p.Name, p.Ty)
			}
			out := hir.Var("out", mir2.TyU16)
			cy := &hir.Func{Name: "cy_" + f.Name, Params: f.Params, RetTy: mir2.TyU8, FallibleCY: true,
				Body: hir.Blk(
					&hir.VarDeclStmt{Name: "out", Ty: mir2.TyU16, Init: call(f.Name, args, mir2.TyU16)},
					&hir.IfStmt{Cond: hir.Gt(out, hir.U16(255)), Then: hir.Blk(&hir.ExprStmt{Expr: call("@error", []hir.Expr{hir.Cast(out, mir2.TyU8)}, mir2.TyVoid)})},
					&hir.ReturnStmt{Val: hir.Cast(out, mir2.TyU8)},
				)}
			prop := &hir.Func{Name: "prop_" + f.Name, Params: f.Params, RetTy: mir2.TyU8, FallibleCY: true,
				Body: hir.Blk(
					&hir.VarDeclStmt{Name: "value", Ty: mir2.TyU8, Init: call(cy.Name, args, mir2.TyU8)},
					&hir.ExprStmt{Expr: call("@propagate", nil, mir2.TyVoid)},
					&hir.ReturnStmt{Val: hir.Var("value", mir2.TyU8)},
				)}
			m.Funcs = append(m.Funcs, cy, prop)
		}
		if len(tc.Args) != len(f.Params) {
			return fmt.Errorf("wrong case arity")
		}
		args := make([]hir.Expr, len(tc.Args))
		for j, v := range tc.Args {
			args[j] = &hir.IntLitExpr{Val: v, Ty: f.Params[j].Ty}
		}
		m.Funcs = append(m.Funcs, &hir.Func{Name: fmt.Sprintf("outcome_case_%d", i), RetTy: mir2.TyU8, FallibleCY: true,
			Body: hir.Blk(
				&hir.VarDeclStmt{Name: "value", Ty: mir2.TyU8, Init: call("prop_"+f.Name, args, mir2.TyU8)},
				&hir.ExprStmt{Expr: call("@propagate", nil, mir2.TyVoid)},
				&hir.ReturnStmt{Val: hir.Var("value", mir2.TyU8)},
			)})
	}
	if err := hir.ResolveErrorABI(m); err != nil {
		return err
	}
	raw := hir.LowerModule(hir.CloneModule(m))
	steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true})
	if err != nil {
		return err
	}
	for i, tc := range cases {
		entry := fmt.Sprintf("outcome_case_%d", i)
		for name, ir := range map[string]*mir2.Module{"raw": raw, "optimized": steps.MIR2Module} {
			vm := mir2.NewVM(ir)
			vm.MaxSteps = 100000
			out, err := vm.CallWithOutcome(entry, nil)
			if err != nil || out.Failed != tc.Failed || tc.Failed && out.ErrorCode != tc.Code || !tc.Failed && (len(out.Values) != 1 || out.Values[0].I != tc.Value) {
				return fmt.Errorf("%s case %d: got %+v, %v; want %+v", name, i, out, err, tc)
			}
		}
		boot := fmt.Sprintf("ORG 0x8000\nLD SP,0xff00\nSCF\nCALL %s\nDI\nHALT\n", entry)
		image, err := z80asm.NewAssembler().AssembleString(boot + steps.Assembly)
		if err != nil || len(image.Errors) > 0 {
			return fmt.Errorf("assemble: %v %v", err, image.Errors)
		}
		z := emulator.NewRemogattoZ80()
		if err := z.LoadMemory(0x8000, image.Binary); err != nil {
			return err
		}
		z.SetPC(0x8000)
		for gas := 0; gas < 100000 && !z.IsHalted(); gas++ {
			z.Step()
		}
		r := z.GetRegisters()
		want := byte(tc.Value)
		if tc.Failed {
			want = tc.Code
		}
		if !z.IsHalted() || (r.F&1 != 0) != tc.Failed || r.A != want || r.SP != 0xff00 {
			return fmt.Errorf("native case %d: got %+v; want A=%d CY=%v SP=ff00", i, r, want, tc.Failed)
		}
	}
	if err := os.WriteFile(os.Args[3]+".a80", []byte(steps.Assembly), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(os.Args[3]+".hir", []byte(m.Dump()), 0644); err != nil {
		return err
	}
	fmt.Printf("Nanz CY ABI: %d cases passed on raw MIR2, optimized MIR2 and Z80 (A, Carry, SP)\n", len(cases))
	return nil
}
