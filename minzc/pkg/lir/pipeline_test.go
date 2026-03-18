package lir

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// TestPipeline_SimpleFunction tests the full LIR pipeline on a hand-built
// MIR2 function: add(a, b) -> a + b
func TestPipeline_SimpleFunction(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("add")
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")

	aReg := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bReg := b.Param("b", mir2.TyU8, mir2.ClassGeneral)
	sum := b.Add(aReg, bReg, mir2.TyU8, mir2.ClassAcc)
	b.Ret(sum)

	// Run convergence on all machines
	results := CheckModuleConvergence(m)
	for _, r := range results {
		if r.Error != "" {
			t.Logf("%s/%s: %s (ops=%d)", r.FuncName, r.Machine, r.Error, r.OpCount)
		} else {
			t.Logf("%s/%s: ✓ ops=%d combined=%d insts=%d",
				r.FuncName, r.Machine, r.OpCount, r.Combined, r.InstCount)
		}
	}

	// At least RISC32 should succeed
	for _, r := range results {
		if r.Machine == "risc32" && !r.Match {
			t.Errorf("risc32 should pass for simple add: %s", r.Error)
		}
	}
}

// TestPipeline_HIRToLIR tests the full path: HIR → MIR2 → LIR convergence.
func TestPipeline_HIRToLIR(t *testing.T) {
	hm := &hir.Module{
		Name: "test_hir",
		Funcs: []*hir.Func{
			{
				Name:   "add_pair",
				Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}},
				RetTy:  mir2.TyU8,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.ReturnStmt{
						Val: &hir.BinExpr{
							Op:  "+",
							L:   &hir.VarRefExpr{Name: "a", Ty: mir2.TyU8},
							R:   &hir.VarRefExpr{Name: "b", Ty: mir2.TyU8},
							Ty:  mir2.TyU8,
						},
					},
				}},
			},
		},
	}

	// Lower HIR → MIR2
	m := hir.LowerModule(hm)

	t.Logf("MIR2 module: %d funcs", len(m.Funcs))
	for _, f := range m.Funcs {
		t.Logf("  %s: %d blocks", f.Name, len(f.Blocks))
	}

	// Run LIR convergence
	results := CheckModuleConvergence(m)
	passed, failed := 0, 0
	for _, r := range results {
		if r.Error != "" {
			t.Logf("%s/%s: %s", r.FuncName, r.Machine, r.Error)
			failed++
		} else if r.Match {
			t.Logf("%s/%s: ✓ ops=%d→%d insts=%d",
				r.FuncName, r.Machine, r.OpCount, r.Combined, r.InstCount)
			passed++
		}
	}
	t.Logf("total: %d passed, %d failed", passed, failed)
}

// TestPipeline_LIRCodegen tests assembly text generation from LIR.
func TestPipeline_LIRCodegen(t *testing.T) {
	m := &mir2.Module{Name: "codegen_test"}
	f := m.AddFunc("double")
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")

	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	result := b.Add(x, x, mir2.TyU8, mir2.ClassAcc)
	b.Ret(result)

	asm, err := LIRCodegenFunc(f, m)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("generated assembly:\n%s", asm)

	if asm == "" {
		t.Error("empty assembly")
	}
}

// TestLIRCodegen_TwoParams verifies that add(a: u8 = ClassAcc, b: u8 = ClassGeneral)
// produces ADD A, C (or ADD A, B) — NOT ADD A, A (the bug before param seeding).
func TestLIRCodegen_TwoParams(t *testing.T) {
	m := &mir2.Module{Name: "two_params_test"}
	f := m.AddFunc("add")
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")

	aReg := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bReg := b.Param("b", mir2.TyU8, mir2.ClassGeneral)
	sum := b.Add(aReg, bReg, mir2.TyU8, mir2.ClassAcc)
	b.Ret(sum)

	asm, err := LIRCodegenFunc(f, m)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("generated assembly:\n%s", asm)

	// Must NOT contain "ADD A, A" — that means both params collapsed to A.
	if strings.Contains(asm, "ADD A, A") {
		t.Errorf("both params collapsed to A (ADD A, A); expected ADD A, <other reg>")
	}

	// Must contain ADD A, <something> where something is B, C, D, E, H, or L.
	found := false
	for _, r := range []string{"B", "C", "D", "E", "H", "L"} {
		if strings.Contains(asm, "ADD A, "+r) {
			found = true
			t.Logf("correctly assigned second param to %s", r)
			break
		}
	}
	if !found {
		t.Error("no ADD A, <gpr> found in output")
	}
}

// TestLIRCodegen_OpCall verifies that function calls emit CALL instructions.
func TestLIRCodegen_OpCall(t *testing.T) {
	m := &mir2.Module{Name: "call_test"}

	// callee: double(x: u8) -> u8 = x + x
	callee := m.AddFunc("double")
	cb := mir2.NewBuilder(callee)
	cb.SwitchToNewBlock("entry")
	cx := cb.Param("x", mir2.TyU8, mir2.ClassAcc)
	cresult := cb.Add(cx, cx, mir2.TyU8, mir2.ClassAcc)
	cb.Ret(cresult)

	// caller: main() calls double(5) and uses the result
	caller := m.AddFunc("caller")
	mb := mir2.NewBuilder(caller)
	mb.SwitchToNewBlock("entry")
	five := mb.Const(5, mir2.TyU8, mir2.ClassAcc)
	result := mb.Call("double", []mir2.Reg{five}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	// Use result to prevent DCE
	mb.Ret(result)

	asm, err := LIRCodegenFunc(caller, m)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("generated assembly:\n%s", asm)

	if !strings.Contains(asm, "CALL double") {
		t.Errorf("expected CALL double in output, got:\n%s", asm)
	}
}

// TestPipeline_Z80Patterns verifies the Z80 machine descriptor.
func TestPipeline_Z80Patterns(t *testing.T) {
	t.Logf("Z80: %d locs, %d patterns, %d rules",
		len(Z80.Locs), len(Z80.Patterns), len(Z80.Rules))

	for _, name := range []string{"A", "B", "C", "D", "E", "H", "L", "HL", "DE", "BC", "IX", "F"} {
		if Z80.LocByName(name) < 0 {
			t.Errorf("missing register: %s", name)
		}
	}

	patNames := make(map[string]bool)
	for _, p := range Z80.Patterns {
		patNames[p.Name] = true
	}
	for _, name := range []string{"ld_r_n", "add_a_r", "sub_a_r", "ld_a_hl", "add_hl_rr", "ld16_le_hl", "djnz"} {
		if !patNames[name] {
			t.Errorf("missing pattern: %s", name)
		} else {
			t.Logf("  ✓ %s", name)
		}
	}
}
