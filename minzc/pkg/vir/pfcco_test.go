package vir_test

import (
	"os/exec"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/vir"
)

func TestZ3_PFCCO_Simple(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	src := `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b { return a - b }
    return b - a
}
fun max(a: u8, b: u8) -> u8 {
    if a > b { return a }
    return b
}
fun main() -> void {
    let r1: u8 = abs_diff(10, 3)
    let r2: u8 = max(r1, 5)
}
`
	hm, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
		lr := mir2.ComputeLiveness(f)
		mir2.Allocate(f, lr, mir2.Z80CostTable{})
	}

	results, err := vir.OptimizePFCCO(m, vir.Z80, vir.PFCCOOptions{Verbose: testing.Verbose()})
	if err != nil {
		t.Fatalf("PFCCO: %v", err)
	}

	t.Log("")
	t.Log("╔════════════════════════════════════════════╗")
	t.Log("║   Z3-PFCCO: Optimal Calling Conventions   ║")
	t.Log("╚════════════════════════════════════════════╝")
	t.Log("")

	for _, r := range results {
		if len(r.ParamLocs) == 0 && r.ReturnName == "" {
			continue
		}
		params := ""
		for i, name := range r.ParamNames {
			if i > 0 {
				params += ", "
			}
			params += name
		}
		ret := r.ReturnName
		if ret == "" {
			ret = "void"
		}
		t.Logf("  fun %-15s params=[%s] → %s", r.FuncName, params, ret)
	}
}

func TestZ3_PFCCO_vs_PBQP(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	src := `
fun add(a: u8, b: u8) -> u8 { return a + b }
fun sub(a: u8, b: u8) -> u8 { return a - b }
fun mul3(a: u8, b: u8, c: u8) -> u8 { return a + b + c }
fun chain(x: u8) -> u8 {
    let y: u8 = add(x, 1)
    let z: u8 = sub(y, 2)
    return z
}
`
	hm, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
		lr := mir2.ComputeLiveness(f)
		mir2.Allocate(f, lr, mir2.Z80CostTable{})
	}

	// Show PBQP contracts
	t.Log("\nPBQP contracts:")
	for _, f := range m.Funcs {
		params := ""
		for i, p := range f.Contract.Params {
			if i > 0 {
				params += ", "
			}
			params += p.Name + "=" + p.Class.String()
		}
		t.Logf("  fun %-10s(%s)", f.Name, params)
	}

	// Z3 PFCCO
	results, err := vir.OptimizePFCCO(m, vir.Z80, vir.PFCCOOptions{Verbose: testing.Verbose()})
	if err != nil {
		t.Fatalf("PFCCO: %v", err)
	}

	t.Log("\nZ3-PFCCO optimal:")
	for _, r := range results {
		if len(r.ParamLocs) == 0 {
			continue
		}
		params := ""
		for i, name := range r.ParamNames {
			if i > 0 {
				params += ", "
			}
			params += name
		}
		t.Logf("  fun %-10s params=[%s] ret=%s", r.FuncName, params, r.ReturnName)
	}
}
