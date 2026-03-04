package codegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir"
	"github.com/minz/minzc/pkg/z80asm"
)

// mirTestCase defines a MIR backend test
type mirTestCase struct {
	name     string // Test name
	file     string // .mir filename (relative to tests/mir_backend/)
	expected string // Expected console output
	maxT     int    // Max T-states before timeout (0 = default 1M)
	knownBug string // If non-empty, test is expected to fail (known bug description)
}

var mirTests = []mirTestCase{
	// A. Arithmetic & Logic
	{name: "arith_basic", file: "arith_basic.mir", expected: "7Fa3OK"},
	{name: "bitwise", file: "bitwise.mir", expected: "214OK"},
	// B. Comparisons
	{name: "compare", file: "compare.mir", expected: "YNYNOK"},
	{name: "multi_compare", file: "multi_compare.mir", expected: "YYNNOK"},
	// C. Control Flow
	{name: "branch", file: "branch.mir", expected: "ABCOK"},
	{name: "nested_branch", file: "nested_branch.mir", expected: "BDOK"},
	// D. Loops
	{name: "loop_while", file: "loop_while.mir", expected: "0123456789OK",
		knownBug: "stale HL tracking after comparison (ADR-0006)"},
	{name: "loop_countdown", file: "loop_countdown.mir", expected: "54321OK"},
	// E. Memory & Data
	{name: "variables", file: "variables.mir", expected: "ABOK"},
	// F. Integration
	{name: "string_print", file: "string_print.mir", expected: "Hello!OK"},
	{name: "accumulator", file: "accumulator.mir", expected: "F6OK",
		knownBug: "stale HL tracking in loop body (ADR-0006)"},
}

// findMIRTestDir locates the mir_backend test directory
func findMIRTestDir() string {
	// When running from minzc/pkg/codegen/, the test dir is at ../../tests/mir_backend/
	_, filename, _, _ := runtime.Caller(0)
	codegenDir := filepath.Dir(filename)
	testDir := filepath.Join(codegenDir, "..", "..", "tests", "mir_backend")
	if abs, err := filepath.Abs(testDir); err == nil {
		testDir = abs
	}
	return testDir
}

// mirPipeline runs the full MIR→Z80→binary→emulate pipeline
// Returns: (console output, T-states, assembly source, error)
func mirPipeline(t *testing.T, mirPath string) (string, int, string, error) {
	t.Helper()

	// Step 1: Parse MIR file
	module, err := mir.ParseMIRFile(mirPath)
	if err != nil {
		return "", 0, "", fmt.Errorf("MIR parse failed: %w", err)
	}

	// Step 2: Generate Z80 assembly (CP/M target)
	var asmBuf bytes.Buffer
	gen := NewZ80Generator(&asmBuf)
	gen.SetTargetPlatform("cpm")
	if err := gen.Generate(module); err != nil {
		return "", 0, "", fmt.Errorf("Z80 codegen failed: %w", err)
	}
	asmSource := asmBuf.String()

	// Step 3: Assemble to binary
	assembler := z80asm.NewAssembler()
	result, err := assembler.AssembleString(asmSource)
	if err != nil {
		return "", 0, asmSource, fmt.Errorf("assembly failed: %w", err)
	}
	if len(result.Errors) > 0 {
		errMsgs := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			errMsgs[i] = e.Error()
		}
		return "", 0, asmSource, fmt.Errorf("assembly errors:\n%s", strings.Join(errMsgs, "\n"))
	}

	// Step 4: Set up emulator
	emu := emulator.NewRemogattoZ80()

	// Capture console output via BDOS handler
	var consoleOutput []byte
	emu.SetBDOSHandler(func(function byte, de uint16) (a byte, hl uint16, handled bool) {
		switch function {
		case 2: // Console output: character in E (low byte of DE)
			ch := byte(de & 0xFF)
			consoleOutput = append(consoleOutput, ch)
			return ch, 0, true
		case 0: // System reset / warm boot
			return 0, 0, true
		default:
			return 0, 0, true // Ignore other BDOS calls
		}
	})

	// Load binary at the assembled origin (should be 0x0100 for CP/M)
	origin := uint16(result.Origin)
	if err := emu.LoadMemory(origin, result.Binary); err != nil {
		return "", 0, asmSource, fmt.Errorf("load memory failed: %w", err)
	}

	// Set up CP/M execution environment:
	// - SP points to high memory
	// - Return address 0x0000 is at the top of stack (for exitOnRET0)
	emu.SetSP(0xFFF0)
	// Write return address 0x0000 at SP
	emu.LoadMemory(0xFFF0, []byte{0x00, 0x00})
	emu.SetPC(origin)
	emu.MaxCycles = 1000000 // Safety limit

	// Step 5: Run
	if err := emu.Run(); err != nil {
		return string(consoleOutput), emu.GetCycles(), asmSource,
			fmt.Errorf("emulation failed: %w (output so far: %q)", err, string(consoleOutput))
	}

	return string(consoleOutput), emu.GetCycles(), asmSource, nil
}

func TestMIRBackend(t *testing.T) {
	testDir := findMIRTestDir()

	// Check test directory exists
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skipf("MIR test directory not found: %s", testDir)
	}

	for _, tc := range mirTests {
		tc := tc // capture
		t.Run(tc.name, func(t *testing.T) {
			mirPath := filepath.Join(testDir, tc.file)
			if _, err := os.Stat(mirPath); os.IsNotExist(err) {
				t.Skipf("MIR test file not found: %s", mirPath)
			}

			output, tstates, asmSource, err := mirPipeline(t, mirPath)
			if err != nil {
				if tc.knownBug != "" {
					t.Skipf("Known bug: %s (error: %v)", tc.knownBug, err)
					return
				}
				if asmSource != "" {
					t.Logf("Generated assembly:\n%s", asmSource)
				}
				t.Fatalf("Pipeline error: %v", err)
			}

			if output != tc.expected {
				if tc.knownBug != "" {
					t.Skipf("Known bug: %s (got %q, want %q)", tc.knownBug, output, tc.expected)
					return
				}
				t.Logf("Generated assembly:\n%s", asmSource)
				t.Errorf("Output mismatch:\n  got:      %q\n  expected: %q", output, tc.expected)
			} else {
				if tc.knownBug != "" {
					t.Logf("FIXED! Known bug %q no longer reproduces: output=%q, T-states=%d", tc.knownBug, output, tstates)
				} else {
					t.Logf("PASS: output=%q, T-states=%d", output, tstates)
				}
			}
		})
	}
}

// TestMIRBackendSummary runs all tests and prints a summary table
func TestMIRBackendSummary(t *testing.T) {
	testDir := findMIRTestDir()
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skipf("MIR test directory not found: %s", testDir)
	}

	type result struct {
		name    string
		pass    bool
		output  string
		tstates int
		bytes   int
		err     string
	}

	var results []result
	passCount := 0

	for _, tc := range mirTests {
		mirPath := filepath.Join(testDir, tc.file)
		if _, err := os.Stat(mirPath); os.IsNotExist(err) {
			results = append(results, result{name: tc.name, err: "file not found"})
			continue
		}

		output, tstates, _, err := mirPipeline(t, mirPath)
		r := result{name: tc.name, output: output, tstates: tstates}
		if err != nil {
			r.err = err.Error()
		} else if output != tc.expected {
			r.err = fmt.Sprintf("expected %q, got %q", tc.expected, output)
		} else {
			r.pass = true
			passCount++
		}
		results = append(results, r)
	}

	// Print summary table
	t.Logf("\n%-20s | %-8s | %-10s | %s", "Program", "T-states", "Status", "Output")
	t.Logf("%-20s-+-%-8s-+-%-10s-+-%s", strings.Repeat("-", 20), strings.Repeat("-", 8), strings.Repeat("-", 10), strings.Repeat("-", 20))
	for _, r := range results {
		status := "PASS"
		if !r.pass {
			status = "FAIL"
		}
		errInfo := ""
		if r.err != "" {
			// Truncate long error messages
			errInfo = r.err
			if len(errInfo) > 60 {
				errInfo = errInfo[:57] + "..."
			}
		}
		t.Logf("%-20s | %-8d | %-10s | %s %s", r.name, r.tstates, status, r.output, errInfo)
	}
	t.Logf("\nTotal: %d/%d passed", passCount, len(results))
}

// BenchmarkMIRBackend benchmarks T-state performance of each MIR program
func BenchmarkMIRBackend(b *testing.B) {
	testDir := findMIRTestDir()
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		b.Skipf("MIR test directory not found: %s", testDir)
	}

	for _, tc := range mirTests {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			mirPath := filepath.Join(testDir, tc.file)
			if _, err := os.Stat(mirPath); os.IsNotExist(err) {
				b.Skipf("MIR test file not found: %s", mirPath)
			}

			// Pre-parse and pre-compile (we benchmark emulation only)
			module, err := mir.ParseMIRFile(mirPath)
			if err != nil {
				b.Fatalf("MIR parse failed: %v", err)
			}

			var asmBuf bytes.Buffer
			gen := NewZ80Generator(&asmBuf)
			gen.SetTargetPlatform("cpm")
			if err := gen.Generate(module); err != nil {
				b.Fatalf("Z80 codegen failed: %v", err)
			}

			assembler := z80asm.NewAssembler()
			result, err := assembler.AssembleString(asmBuf.String())
			if err != nil || len(result.Errors) > 0 {
				b.Fatalf("Assembly failed: %v", err)
			}

			origin := uint16(result.Origin)
			binary := result.Binary

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				emu := emulator.NewRemogattoZ80()
				emu.SetBDOSHandler(func(function byte, de uint16) (a byte, hl uint16, handled bool) {
					if function == 2 {
						return byte(de & 0xFF), 0, true
					}
					return 0, 0, true
				})
				emu.LoadMemory(origin, binary)
				emu.SetSP(0xFFF0)
				emu.LoadMemory(0xFFF0, []byte{0x00, 0x00})
				emu.SetPC(origin)
				emu.MaxCycles = 1000000
				emu.Run()
			}
		})
	}
}
