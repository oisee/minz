package z80testing

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRegAllocQuality_ForEachCall measures T-states for a forEach(call) iterator loop.
// This verifies the register allocator improvements reduce memory round-trips.
func TestRegAllocQuality_ForEachCall(t *testing.T) {
	harness, err := NewE2ETestHarness(t)
	if err != nil {
		t.Skipf("E2E harness unavailable: %v", err)
		return
	}
	defer harness.Cleanup()

	source := `
fun console_log(c: u8) -> void {
    asm {
        OUT ($23), A
    }
}

fun main() -> void {
    let arr: [u8; 5] = [65, 66, 67, 68, 69];
    arr.forEach(console_log);
    asm {
        DI
        HALT
    }
}
`
	sourceFile := filepath.Join(harness.workDir, "foreach_perf.minz")
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	a80File, err := harness.CompileMinZ(sourceFile, false)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	binary, symbols, err := harness.AssembleA80(a80File)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	harness.LoadBinary(binary, 0x8000)

	mainAddr, ok := findSymbol(symbols, "main")
	if !ok {
		t.Fatalf("main symbol not found in: %v", symbolNames(symbols))
	}

	err = harness.Execute(mainAddr, 100000)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	cycles := harness.GetCycles()
	perElement := cycles / 5
	t.Logf("forEach(call) 5 elements: %d T-states total, ~%d T-states/element", cycles, perElement)

	// Baseline (2026-03-09, real T-states): 1324T total (~264T/element).
	// MIR1 codegen with $F0xx memory round-trips — most time in LD (addr),A / LD A,(addr).
	// NOTE: old threshold of 500 was based on instruction counts (not T-states).
	// Regression threshold at 2000T catches significant degradation.
	maxAcceptable := 2000
	if cycles > maxAcceptable {
		asmBytes, _ := os.ReadFile(a80File)
		t.Logf("Assembly:\n%s", string(asmBytes))
		t.Errorf("forEach(call) took %d T-states, exceeds maximum %d (possible regression)", cycles, maxAcceptable)
	}
}

// TestRegAllocQuality_MapForEach measures T-states for a map+forEach chain.
func TestRegAllocQuality_MapForEach(t *testing.T) {
	harness, err := NewE2ETestHarness(t)
	if err != nil {
		t.Skipf("E2E harness unavailable: %v", err)
		return
	}
	defer harness.Cleanup()

	source := `
fun add_one(x: u8) -> u8 {
    return x + 1;
}

fun console_log(c: u8) -> void {
    asm {
        OUT ($23), A
    }
}

fun main() -> void {
    let arr: [u8; 5] = [65, 66, 67, 68, 69];
    arr.map(add_one).forEach(console_log);
    asm {
        DI
        HALT
    }
}
`
	sourceFile := filepath.Join(harness.workDir, "map_foreach_perf.minz")
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	a80File, err := harness.CompileMinZ(sourceFile, false)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	binary, symbols, err := harness.AssembleA80(a80File)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	harness.LoadBinary(binary, 0x8000)

	mainAddr, ok := findSymbol(symbols, "main")
	if !ok {
		t.Fatalf("main symbol not found in: %v", symbolNames(symbols))
	}

	err = harness.Execute(mainAddr, 100000)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	cycles := harness.GetCycles()
	perElement := cycles / 5
	t.Logf("map+forEach 5 elements: %d T-states total, ~%d T-states/element", cycles, perElement)

	// Baseline (2026-03-09, real T-states): 1915T total (~383T/element).
	// MIR1 codegen with $F0xx memory round-trips.
	// NOTE: old threshold of 1000 was based on instruction counts (not T-states).
	// Regression threshold at 3000T catches significant degradation.
	maxAcceptable := 3000
	if cycles > maxAcceptable {
		asmBytes, _ := os.ReadFile(a80File)
		t.Logf("Assembly:\n%s", string(asmBytes))
		t.Errorf("map+forEach took %d T-states, exceeds maximum %d", cycles, maxAcceptable)
	}
}

// TestRegAllocQuality_FilterForEach measures T-states for filter+forEach.
func TestRegAllocQuality_FilterForEach(t *testing.T) {
	harness, err := NewE2ETestHarness(t)
	if err != nil {
		t.Skipf("E2E harness unavailable: %v", err)
		return
	}
	defer harness.Cleanup()

	source := `
fun console_log(c: u8) -> void {
    asm {
        OUT ($23), A
    }
}

fun main() -> void {
    let arr: [u8; 5] = [65, 66, 67, 68, 69];
    arr.filter(|x| x > 66).forEach(console_log);
    asm {
        DI
        HALT
    }
}
`
	sourceFile := filepath.Join(harness.workDir, "filter_foreach_perf.minz")
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	a80File, err := harness.CompileMinZ(sourceFile, false)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	binary, symbols, err := harness.AssembleA80(a80File)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	harness.LoadBinary(binary, 0x8000)

	mainAddr, ok := findSymbol(symbols, "main")
	if !ok {
		t.Fatalf("main symbol not found in: %v", symbolNames(symbols))
	}

	err = harness.Execute(mainAddr, 100000)
	if err != nil {
		asmBytes, _ := os.ReadFile(a80File)
		t.Logf("Assembly:\n%s", string(asmBytes))
		t.Fatalf("execution failed: %v", err)
	}

	cycles := harness.GetCycles()
	perElement := cycles / 5
	t.Logf("filter+forEach 5 elements: %d T-states total, ~%d T-states/element", cycles, perElement)

	// Baseline (2026-03-09, real T-states): 1469T total (~293T/element).
	// MIR1 codegen with $F0xx memory round-trips.
	// NOTE: old threshold of 500 was based on instruction counts (not T-states).
	// Regression threshold at 2200T catches significant degradation.
	maxAcceptable := 2200
	if cycles > maxAcceptable {
		asmBytes, _ := os.ReadFile(a80File)
		t.Logf("Assembly:\n%s", string(asmBytes))
		t.Errorf("filter+forEach took %d T-states, exceeds maximum %d", cycles, maxAcceptable)
	}
}
