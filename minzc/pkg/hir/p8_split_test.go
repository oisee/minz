package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestP8SplitReturnDependency(t *testing.T) {
	f := &Func{Name: "compute", RetTy: mir2.TyU16, Params: []Param{{Name: "a", Ty: mir2.TyU16}}, Body: &Block{Body: []Stmt{
		&VarDeclStmt{Name: "x", Ty: mir2.TyU16, Init: &VarRefExpr{Name: "a", Ty: mir2.TyU16}},
		&VarDeclStmt{Name: "y", Ty: mir2.TyU16, Init: &IntLitExpr{Val: 7, Ty: mir2.TyU16}},
		&VarDeclStmt{Name: "z", Ty: mir2.TyU16, Init: &IntLitExpr{Val: 8, Ty: mir2.TyU16}},
		&ReturnStmt{Val: &VarRefExpr{Name: "x", Ty: mir2.TyU16}},
	}}}
	m := &Module{Name: "p8", Funcs: []*Func{f}}
	candidates := FindSplitPoints(f, []int{9, 9, 9, 9})
	if len(candidates) == 0 {
		t.Fatal("no split")
	}
	sub := ApplySplit(m, f, candidates[0])
	m.Funcs = append(m.Funcs, sub)
	mm := LowerModule(m)
	if mm.FuncByName(sub.Name) == nil {
		t.Fatal("split callee omitted by free-variable detection")
	}
	result, err := mir2.NewVM(mm).Call("compute", []mir2.Value{{I: 1234}})
	if err != nil || len(result) != 1 || result[0].I != 1234 {
		t.Fatalf("return lost across split: %v %v", result, err)
	}
	mm.RenumberRegs()
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, mf := range mm.Funcs {
		a := mir2.PBQPAllocate(mf, mir2.ComputeLiveness(mf), mir2.Z80CostTable{})
		for r, l := range a.Locs {
			ar.Locs[r] = l
		}
		ar.Spilled = append(ar.Spilled, a.Spilled...)
	}
	param := mm.FuncByName("compute").Contract.Params[0].Reg
	// Use distinct parameter locations so this tests the forwarding caller
	// rather than the codegen's identity-function EQU alias optimization.
	ar.Locs[param] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "BC"}
	code, cgErr := mir2.Z80Codegen(mm, ar)
	if cgErr != nil {
		t.Fatalf("codegen: %v", cgErr)
	}
	asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD %s,1234\nCALL compute\nDI\nHALT\n", ar.Loc(param).Name) + code
	res, e := z80asm.NewAssembler().AssembleString(asm)
	if e != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble %v %v\n%s", e, res.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	z.LoadMemory(0x8000, res.Binary)
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00})
	for n := 0; !z.IsHalted(); n++ {
		if n > 10000 {
			t.Fatalf("no HALT\n%s", asm)
		}
		z.Step()
	}
	if got := z.GetRegisters().HL; got != uint16(result[0].I) {
		t.Fatalf("Z80 got %d MIR2 got %d\n%s", got, result[0].I, asm)
	}
}

func TestP8SplitConditionalDependency(t *testing.T) {
	f := &Func{Name: "compute", RetTy: mir2.TyU16, Params: []Param{{Name: "a", Ty: mir2.TyU16}}, Body: &Block{Body: []Stmt{
		&VarDeclStmt{Name: "x", Ty: mir2.TyU16, Init: &VarRefExpr{Name: "a", Ty: mir2.TyU16}},
		&VarDeclStmt{Name: "y", Ty: mir2.TyU16, Init: &IntLitExpr{Val: 7, Ty: mir2.TyU16}},
		&VarDeclStmt{Name: "z", Ty: mir2.TyU16, Init: &CondExpr{Cond: &BoolLitExpr{Val: true}, Then: &VarRefExpr{Name: "x", Ty: mir2.TyU16}, Else: &IntLitExpr{Val: 0, Ty: mir2.TyU16}, Ty: mir2.TyU16}},
		&ReturnStmt{Val: &VarRefExpr{Name: "z", Ty: mir2.TyU16}},
	}}}
	m := &Module{Name: "p8", Funcs: []*Func{f}}
	c := FindSplitPoints(f, []int{9, 9, 9, 9})
	sub := ApplySplit(m, f, c[0])
	m.Funcs = append(m.Funcs, sub)
	mm := LowerModule(m)
	if mm.FuncByName(sub.Name) == nil {
		t.Fatal("conditional dependencies omitted from split interface")
	}
	got, err := mir2.NewVM(mm).Call("compute", []mir2.Value{{I: 1234}})
	if err != nil || len(got) != 1 || got[0].I != 1234 {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestP8ReturnedSplitIsNotSplitAgain(t *testing.T) {
	f := &Func{Name: "compute", RetTy: mir2.TyU16, Body: &Block{}}
	var args []Expr
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "x", "y", "z"} {
		f.Params = append(f.Params, Param{Name: name, Ty: mir2.TyU16})
		args = append(args, &VarRefExpr{Name: name, Ty: mir2.TyU16})
	}
	for i := 0; i < 4; i++ {
		f.Body.Body = append(f.Body.Body, &ExprStmt{Expr: &CallExpr{Fn: "side", Args: args, Ty: mir2.TyVoid}})
	}
	f.Body.Body = append(f.Body.Body, &ReturnStmt{Val: &CallExpr{Fn: "compute$split_1", Args: args[:3], Ty: mir2.TyU16}})
	m := &Module{Name: "p8", Funcs: []*Func{f}}
	var results []SplitResult
	if subs := splitRecursive(m, f, &results, 0); len(subs) != 0 {
		t.Fatalf("returned split call was split again: %v", results)
	}
}
