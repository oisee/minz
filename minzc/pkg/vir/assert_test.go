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

// runVIRAsserts compiles Nanz source through VIR and runs both MIR2-VM and Z80 asserts.
func runVIRAsserts(t *testing.T, name, src string) {
	t.Helper()
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	hm, err := nanz.ParseWithOpts(src, name+".nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	m := hir.LowerModule(hm)
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
	}

	// PBQP allocation: per-function, merged into combined (like production pipeline)
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	allocs := make(map[string]*mir2.AllocResult)
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		allocs[f.Name] = ar
		// Merge into combined (same as production pipeline)
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
		combined.Spilled = append(combined.Spilled, ar.Spilled...)
	}

	// Build per-function param locations from PBQP
	funcParamLocs := make(map[string]map[int]int)
	for _, f := range m.Funcs {
		ar := allocs[f.Name]
		if ar == nil {
			continue
		}
		pl := make(map[int]int)
		for _, cp := range f.Contract.Params {
			if loc, ok := ar.Locs[cp.Reg]; ok {
				idx := vir.Z80.LocByName(loc.Name)
				if idx >= 0 {
					pl[int(cp.Reg)] = idx
				}
			}
		}
		funcParamLocs[f.Name] = pl
	}

	// MIR2-VM baseline
	if err := pipeline.RunAssertsMIR2(hm, m); err != nil {
		t.Fatalf("MIR2-VM: %v", err)
	}

	// VIR codegen with per-function param locs
	virAsm, results := vir.CodegenModule(m, vir.SolverOptions{
		Timeout:       30 * time.Second,
		FuncParamLocs: funcParamLocs,
	})
	for _, r := range results {
		if !r.OK {
			t.Fatalf("VIR codegen %s: %s", r.Name, r.Error)
		}
	}

	// Z80 asserts on VIR output
	if err := pipeline.RunAssertsZ80(hm, m, combined, virAsm); err != nil {
		t.Errorf("Z80: %v", err)
	} else {
		t.Logf("%s: %d asserts passed ✓ (MIR2-VM + Z80)", name, len(hm.Asserts))
	}
}

// ── Test suites ─────────────────────────────────────────────────────────────

func TestVIR_Assert_File(t *testing.T) {
	f := filepath.Join("..", "..", "..", "examples", "nanz", "assert_test.nanz")
	src, err := os.ReadFile(f)
	if err != nil {
		t.Skip("assert_test.nanz not found")
	}
	runVIRAsserts(t, "assert_test", string(src))
}

func TestVIR_Assert_Arithmetic(t *testing.T) {
	runVIRAsserts(t, "arithmetic", `
fun add(a: u8, b: u8) -> u8 { return a + b }
fun sub(a: u8, b: u8) -> u8 { return a - b }
fun double(x: u8) -> u8 { return x + x }
fun identity(x: u8) -> u8 { return x }
fun const5() -> u8 { return 5 }
fun inc(x: u8) -> u8 { return x + 1 }

assert add(3, 5) == 8
assert add(0, 0) == 0
assert add(100, 55) == 155
assert add(255, 1) == 0
assert sub(10, 3) == 7
assert sub(5, 5) == 0
assert double(21) == 42
assert double(0) == 0
assert double(127) == 254
assert identity(0) == 0
assert identity(42) == 42
assert identity(255) == 255
assert const5() == 5
assert inc(0) == 1
assert inc(41) == 42
assert inc(255) == 0
`)
}

func TestVIR_Assert_Bitwise(t *testing.T) {
	runVIRAsserts(t, "bitwise", `
fun band(a: u8, b: u8) -> u8 { return a & b }
fun bor(a: u8, b: u8) -> u8 { return a | b }

assert band(0xFF, 0x0F) == 0x0F
assert band(0xF0, 0x0F) == 0
assert band(0xAA, 0xFF) == 0xAA
assert bor(0xF0, 0x0F) == 0xFF
assert bor(0, 0) == 0
assert bor(0x80, 0x01) == 0x81
`)
}

func TestVIR_Assert_Max(t *testing.T) {
	runVIRAsserts(t, "max", `
fun max(a: u8, b: u8) -> u8 {
    if a > b { return a }
    return b
}
assert max(10, 3) == 10
assert max(3, 10) == 10
assert max(5, 5) == 5
assert max(0, 255) == 255
assert max(255, 0) == 255
`)
}

func TestVIR_Assert_Min(t *testing.T) {
	runVIRAsserts(t, "min", `
fun min(a: u8, b: u8) -> u8 {
    if a < b { return a }
    return b
}
assert min(10, 3) == 3
assert min(3, 10) == 3
assert min(5, 5) == 5
assert min(0, 255) == 0
`)
}

func TestVIR_Assert_AbsDiff(t *testing.T) {
	runVIRAsserts(t, "abs_diff", `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b { return a - b }
    return b - a
}
assert abs_diff(10, 3) == 7
assert abs_diff(3, 10) == 7
assert abs_diff(0, 0) == 0
assert abs_diff(100, 100) == 0
assert abs_diff(255, 0) == 255
`)
}

func TestVIR_Assert_ReturnABI(t *testing.T) {
	runVIRAsserts(t, "return_abi", `
fun ret_a(a: u8, b: u8) -> u8 { return a }
fun ret_b(a: u8, b: u8) -> u8 { return b }
fun swap_ret(a: u8, b: u8) -> u8 {
    let t: u8 = a
    return b
}

assert ret_a(42, 99) == 42
assert ret_a(0, 255) == 0
assert ret_b(42, 99) == 99
assert ret_b(0, 255) == 255
assert swap_ret(1, 2) == 2
assert swap_ret(100, 200) == 200
`)
}

func TestVIR_Assert_MultiExpr(t *testing.T) {
	runVIRAsserts(t, "multi_expr", `
fun add_sub(a: u8, b: u8, c: u8) -> u8 {
    return a + b - c
}
fun double_add(a: u8, b: u8) -> u8 {
    return a + a + b
}

assert add_sub(10, 5, 3) == 12
assert add_sub(100, 50, 50) == 100
assert double_add(5, 3) == 13
assert double_add(0, 42) == 42
`)
}
