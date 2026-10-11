package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"os"
	"strings"
	"testing"
)

func bubbleProfileFixture(t *testing.T) (*hir.Module, func() *hir.Module, hir.ProfileWorkload) {
	t.Helper()
	b, e := os.ReadFile("../../../examples/pascal/bubble_sort.pas")
	if e != nil {
		t.Fatal(e)
	}
	fresh := func() *hir.Module {
		h, e := pascal.Compile(string(b), "../../../examples/pascal/bubble_sort.pas")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	w := workloadSpec(hir.ProfileInvocation{Function: "SORT"})
	w.ExactDataAccess = true
	w.Globals = map[string][]byte{"ARR": {5, 4, 3, 2, 1, 0, 255, 128, 127, 64}}
	return fresh(), fresh, w
}
func TestPascalCanonicalDeadRuntimeAdmissionAndDenial(t *testing.T) {
	source, fresh, w := bubbleProfileFixture(t)
	raw := hir.LowerModule(source)
	if e := hir.ValidateProfileWorkloadMIR(source, raw, w); e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []string{"template", "clobbers", "inputs", "abi", "newasm"} {
		t.Run(mutation, func(t *testing.T) {
			c := hir.LowerModule(source)
			f := c.FuncByName("ConOut")
			switch mutation {
			case "template":
				f.Blocks[0].Insts[0].Asm.Template += "\nJP SORT"
			case "clobbers":
				f.Blocks[0].Insts[0].Asm.Clobbers = 0
			case "inputs":
				f.Blocks[0].Insts[0].Asm.Ins[0] = 999
			case "abi":
				f.Contract.Params[0].Ty = mir2.TyU16
			case "newasm":
				c.FuncByName("INITARRAY").Blocks[0].Insts = append(c.FuncByName("INITARRAY").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpAsm, Ty: mir2.TyVoid, Asm: &mir2.AsmExtra{Template: "NOP"}})
			}
			if e := hir.ValidateProfileWorkloadMIR(source, c, w); e == nil {
				t.Fatal("noncanonical mutation admitted")
			}
		})
	}
	mutated := fresh()
	mutated.FuncByName("ConOut").Body.Body[0].(*hir.AsmStmt).Code += " / NOP"
	if _, e := hir.RunLoopProfile(mutated, w); e == nil {
		t.Fatal("mutated canonical source admitted")
	}
	live := fresh()
	live.FuncByName("SORT").Body.Body = append([]hir.Stmt{&hir.ExprStmt{Expr: hir.Call("ConOut", mir2.TyVoid, hir.U8(65))}}, live.FuncByName("SORT").Body.Body...)
	if e := hir.ValidateProfileWorkloadMIR(live, hir.LowerModule(source), w); e == nil {
		t.Fatal("optimizer-erased reachable runtime call admitted")
	}
	escape := hir.LowerModule(source)
	escape.FuncByName("INITARRAY").Blocks[0].Insts = append(escape.FuncByName("INITARRAY").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpAddrOf, Sym: "ConOut", Ty: mir2.TyPtr})
	if e := hir.ValidateProfileWorkloadMIR(source, escape, w); e == nil {
		t.Fatal("dead function address escaped")
	}
	oracle, e := hir.RunLoopProfile(source, w)
	if e != nil {
		t.Fatal(e)
	}
	steps, e := CompileHIRSteps(fresh(), Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	r, e := RunZ80ProfileWorkload(steps, source, w, *oracle)
	if e != nil || r == nil {
		t.Fatal(r, e)
	}
	bad := steps
	bad.Assembly = strings.Replace(steps.Assembly, "SORT:\n", "SORT:\n JP ConOut\n", 1)
	if bad.Assembly == steps.Assembly {
		t.Fatal("no SORT label")
	}
	r, e = RunZ80ProfileWorkload(bad, source, w, *oracle)
	if r != nil || e == nil || !strings.Contains(e.Error(), "execution outside instruction bounds") {
		t.Fatal("dead runtime fetch admitted", r, e)
	}
	w.Calls = []hir.ProfileInvocation{{Function: "main"}}
	if r, e := hir.RunLoopProfile(source, w); r != nil || e == nil {
		t.Fatal("reachable original output runtime admitted", r, e)
	}
}
