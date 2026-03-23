package vir_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/vir"
)

// TestVIR_Assert_Pipeline compiles Nanz programs with asserts through the
// VIR pipeline and verifies correctness via the standard RunAsserts system.
func TestVIR_Assert_Pipeline(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	assertFile := filepath.Join("..", "..", "..", "examples", "nanz", "assert_test.nanz")
	src, err := os.ReadFile(assertFile)
	if err != nil {
		t.Skip("assert_test.nanz not found")
	}

	// Parse Nanz → HIR (includes asserts)
	hm, err := nanz.ParseWithOpts(string(src), "assert_test.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	t.Logf("Parsed: %d funcs, %d asserts", len(hm.Funcs), len(hm.Asserts))

	// HIR → MIR2 (standard pipeline)
	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
	}

	// Allocate (needed for RunAssertsZ80 bootstrap ABI info)
	var combined *mir2.AllocResult
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		combined = mir2.Allocate(f, lr, mir2.Z80CostTable{})
	}

	// First: run MIR2-VM asserts (baseline correctness)
	t.Log("--- MIR2-VM asserts ---")
	if err := pipeline.RunAssertsMIR2(hm, m); err != nil {
		t.Logf("MIR2-VM: %v", err)
	} else {
		t.Logf("MIR2-VM: all %d asserts passed ✓", len(hm.Asserts))
	}

	// Generate VIR assembly
	virAsm, results := vir.CodegenModule(m, vir.SolverOptions{Timeout: 30 * time.Second})
	ok, fail := 0, 0
	for _, r := range results {
		if r.OK {
			ok++
		} else {
			fail++
			t.Logf("VIR codegen fail: %s: %s", r.Name, r.Error)
		}
	}
	t.Logf("VIR codegen: %d/%d funcs OK", ok, ok+fail)
	t.Logf("VIR ASM:\n%s", virAsm)

	// Run Z80 asserts on VIR-generated assembly
	t.Log("--- Z80 asserts (VIR output) ---")
	if err := pipeline.RunAssertsZ80(hm, m, combined, virAsm); err != nil {
		t.Errorf("Z80 assert FAIL: %v", err)
	} else {
		t.Logf("Z80: all %d asserts passed ✓", len(hm.Asserts))
	}
}

// TestVIR_Assert_Custom tests custom programs with asserts through VIR.
func TestVIR_Assert_Custom(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	src := `
fun add(a: u8, b: u8) -> u8 { return a + b }
fun identity(x: u8) -> u8 { return x }
fun const5() -> u8 { return 5 }

assert add(3, 5) == 8
assert add(0, 0) == 0
assert add(100, 55) == 155
assert identity(42) == 42
assert const5() == 5
`
	hm, err := nanz.ParseWithOpts(src, "custom.nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
	}
	var combined *mir2.AllocResult
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		combined = mir2.Allocate(f, lr, mir2.Z80CostTable{})
	}

	// MIR2-VM baseline
	if err := pipeline.RunAssertsMIR2(hm, m); err != nil {
		t.Fatalf("MIR2-VM: %v", err)
	}
	t.Logf("MIR2-VM: %d asserts passed", len(hm.Asserts))

	// VIR codegen
	virAsm, _ := vir.CodegenModule(m, vir.SolverOptions{Timeout: 30 * time.Second})

	// Z80 asserts
	if err := pipeline.RunAssertsZ80(hm, m, combined, virAsm); err != nil {
		t.Errorf("Z80 assert: %v", err)
	} else {
		t.Logf("Z80 (VIR): %d asserts passed ✓", len(hm.Asserts))
	}
}
