package z80testing

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

// TestTSMCPerformanceVerification verifies that TSMC provides 30%+ performance improvement
func TestTSMCPerformanceVerification(t *testing.T) {
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer h.Cleanup()

	// Create a MinZ program specifically designed to showcase TSMC benefits
	minzSource := `
// TSMC Performance Verification Test Program
// These functions are designed to demonstrate TRUE SMC optimization benefits

// Function 1: Direct parameter patching
// TSMC can patch the multiplier directly into the instruction
fn multiply_by_param(value: u16, multiplier: u16) -> u16 {
    // Without TSMC: Load multiplier from memory/register
    // With TSMC: Multiplier is patched into the immediate field
    return value * multiplier;
}

// Function 2: Loop with patchable bounds
// TSMC patches the loop limit directly into the comparison instruction
fn sum_to_limit(limit: u16) -> u16 {
    var sum: u16 = 0;
    var i: u16 = 1;
    
    // The limit comparison becomes a self-modifying immediate
    while i <= limit {
        sum = sum + i;
        i = i + 1;
    }
    
    return sum;
}

// Function 3: Conditional with patchable threshold
// TSMC patches the threshold into the comparison
fn count_above_threshold(arr: *u16, len: u16, threshold: u16) -> u16 {
    var count: u16 = 0;
    var i: u16 = 0;
    
    while i < len {
        // Threshold is patched directly into the CMP instruction
        if arr[i] > threshold {
            count = count + 1;
        }
        i = i + 1;
    }
    
    return count;
}

// Function 4: Nested loops with TSMC optimization
// Both loop bounds can be patched
fn nested_sum(rows: u16, cols: u16) -> u16 {
    var sum: u16 = 0;
    var r: u16 = 0;
    
    while r < rows {
        var c: u16 = 0;
        while c < cols {
            sum = sum + (r * cols + c);
            c = c + 1;
        }
        r = r + 1;
    }
    
    return sum;
}

// Function 5: Function call chain
// Tests TSMC optimization of function calls
fn chain_calc(x: u16, y: u16, z: u16) -> u16 {
    // Each function call can have its parameters patched
    var a = multiply_by_param(x, 2);
    var b = multiply_by_param(y, 3);
    var c = multiply_by_param(z, 4);
    return a + b + c;
}

// Test data array
var test_data: [20]u16 = [
    5, 10, 15, 20, 25, 30, 35, 40, 45, 50,
    55, 60, 65, 70, 75, 80, 85, 90, 95, 100
];
`

	// Write source file
	sourceFile := filepath.Join(h.workDir, "tsmc_verification.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(minzSource), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Test cases with expected performance improvements
	testCases := []struct {
		name             string
		function         string
		args             []uint16
		minImprovement   float64
		expectedResult   uint16
	}{
		{
			name:           "multiply_by_param",
			function:       "multiply_by_param",
			args:           []uint16{123, 7},
			minImprovement: 30.0, // Direct immediate patching should give 30%+
			expectedResult: 861,  // 123 * 7
		},
		{
			name:           "sum_to_limit_small",
			function:       "sum_to_limit",
			args:           []uint16{10},
			minImprovement: 25.0, // Loop bound patching
			expectedResult: 55,   // 1+2+...+10
		},
		{
			name:           "sum_to_limit_large",
			function:       "sum_to_limit",
			args:           []uint16{100},
			minImprovement: 35.0, // Larger loops benefit more
			expectedResult: 5050, // 1+2+...+100
		},
		{
			name:           "count_above_threshold",
			function:       "count_above_threshold",
			args:           []uint16{0x9000, 20, 50}, // arr, len, threshold
			minImprovement: 30.0,                     // Threshold patching in loop
			expectedResult: 10,                       // Half the values > 50
		},
		{
			name:           "nested_sum_small",
			function:       "nested_sum",
			args:           []uint16{3, 4},
			minImprovement: 40.0, // Nested loops benefit significantly
			expectedResult: 66,   // Sum of 0..11
		},
		{
			name:           "chain_calc",
			function:       "chain_calc",
			args:           []uint16{10, 20, 30},
			minImprovement: 35.0,                      // Multiple function calls optimized
			expectedResult: 10*2 + 20*3 + 30*4, // 200
		},
	}

	// Set up test data array at 0x9000
	dataAddr := uint16(0x9000)
	for i := uint16(0); i < 20; i++ {
		value := (i + 1) * 5
		h.memory.WriteByte(dataAddr+i*2, byte(value&0xFF))
		h.memory.WriteByte(dataAddr+i*2+1, byte(value>>8))
	}

	// Run each test case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Adjust args if needed for array pointer
			args := tc.args
			if tc.function == "count_above_threshold" {
				args[0] = dataAddr
			}

			comparison, err := h.ComparePerformance(sourceFile, tc.function, args...)
			if err != nil {
				t.Fatalf("Performance comparison failed: %v", err)
			}

			// Log detailed results
			t.Log(comparison.String())

			// Verify correct results
			if comparison.NoTSMCResult != tc.expectedResult {
				t.Errorf("Incorrect result without TSMC: got %d, want %d",
					comparison.NoTSMCResult, tc.expectedResult)
			}
			if comparison.TSMCResult != tc.expectedResult {
				t.Errorf("Incorrect result with TSMC: got %d, want %d",
					comparison.TSMCResult, tc.expectedResult)
			}

			// Verify performance improvement
			if comparison.CycleReduction < tc.minImprovement {
				t.Errorf("Insufficient performance improvement: got %.1f%%, want at least %.1f%%",
					comparison.CycleReduction, tc.minImprovement)
			} else {
				t.Logf("✓ Achieved %.1f%% improvement (%.2fx speedup)",
					comparison.CycleReduction, comparison.SpeedupFactor)
			}

			// Log SMC activity
			if comparison.TSMCSMCEvents > 0 {
				t.Logf("  SMC events detected: %d", comparison.TSMCSMCEvents)
			}
		})
	}

	// Summary test to verify overall TSMC effectiveness
	t.Run("overall_performance_summary", func(t *testing.T) {
		totalImprovement := 0.0
		count := 0

		// Re-run all tests to calculate average improvement
		for _, tc := range testCases {
			args := tc.args
			if tc.function == "count_above_threshold" {
				args[0] = dataAddr
			}

			comparison, err := h.ComparePerformance(sourceFile, tc.function, args...)
			if err != nil {
				continue
			}

			totalImprovement += comparison.CycleReduction
			count++
		}

		avgImprovement := totalImprovement / float64(count)
		t.Logf("\n=== TSMC Performance Summary ===")
		t.Logf("Average improvement across all tests: %.1f%%", avgImprovement)
		t.Logf("Number of tests: %d", count)

		if avgImprovement < 30.0 {
			t.Errorf("Overall TSMC performance below target: %.1f%% < 30%%", avgImprovement)
		} else {
			t.Logf("✓ TSMC optimization target achieved!")
		}
	})
}

// TestTSMCPatternAnalysis analyzes SMC patterns in TSMC-optimized code
func TestTSMCPatternAnalysis(t *testing.T) {
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer h.Cleanup()

	// MinZ program with various TSMC patterns
	minzSource := `
// Different TSMC optimization patterns

// Pattern 1: Simple immediate patching
fn add_immediate(x: u16, imm: u16) -> u16 {
    return x + imm;  // imm gets patched
}

// Pattern 2: Comparison patching
fn compare_equal(x: u16, val: u16) -> bool {
    return x == val;  // val gets patched in CMP
}

// Pattern 3: Jump target patching (conditional)
fn conditional_calc(x: u16, flag: bool) -> u16 {
    if flag {
        return x * 2;
    } else {
        return x * 3;
    }
}

// Pattern 4: Loop increment patching
fn step_sum(start: u16, end: u16, step: u16) -> u16 {
    var sum: u16 = 0;
    var i: u16 = start;
    
    while i <= end {
        sum = sum + i;
        i = i + step;  // step gets patched
    }
    
    return sum;
}
`

	// Write source file
	sourceFile := filepath.Join(h.workDir, "tsmc_patterns.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(minzSource), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Compile with TSMC
	a80File, err := h.CompileMinZ(sourceFile, true)
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	binary, symbols, err := h.AssembleA80(a80File)
	if err != nil {
		t.Fatalf("Assembly failed: %v", err)
	}

	h.LoadBinary(binary, 0x8000)

	// Test each pattern and analyze SMC events
	patterns := []struct {
		function string
		args     []uint16
	}{
		{"add_immediate", []uint16{100, 42}},
		{"compare_equal", []uint16{50, 50}},
		{"conditional_calc", []uint16{25, 1}},
		{"step_sum", []uint16{0, 20, 5}},
	}

	for _, p := range patterns {
		t.Run(p.function, func(t *testing.T) {
			funcAddr, ok := symbols[p.function]
			if !ok {
				t.Skipf("Function %s not found", p.function)
			}

			// Clear SMC tracker
			h.smcTracker.Clear()

			// Call function
			if err := h.CallFunction(funcAddr, p.args...); err != nil {
				t.Fatalf("Function execution failed: %v", err)
			}

			// Analyze SMC events
			events := h.smcTracker.GetCodeEvents()
			if len(events) > 0 {
				t.Logf("SMC Events for %s:", p.function)
				for i, event := range events {
					t.Logf("  [%d] PC=%04X modified %04X: %02X -> %02X (cycle %d)",
						i+1, event.PC, event.Address, event.OldValue, event.NewValue, event.Cycle)
				}

				// Detect patterns
				patterns := h.smcTracker.DetectPatterns()
				for _, pattern := range patterns {
					t.Logf("  Pattern: %s (%d occurrences)", pattern.Name, len(pattern.Matches))
				}
			} else {
				t.Logf("No SMC events detected for %s", p.function)
			}
		})
	}
}

// TestTSMCRealWorldBenchmark tests TSMC on realistic workloads
func TestTSMCRealWorldBenchmark(t *testing.T) {
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer h.Cleanup()

	// Realistic MinZ program - simple graphics renderer
	minzSource := `
// Simple graphics rendering functions

// Fill rectangle with color
fn fill_rect(screen: *u8, x: u16, y: u16, w: u16, h: u16, color: u8) -> void {
    var row: u16 = 0;
    while row < h {
        var col: u16 = 0;
        var offset: u16 = (y + row) * 256 + x;  // Assuming 256-wide screen
        
        while col < w {
            screen[offset + col] = color;
            col = col + 1;
        }
        row = row + 1;
    }
}

// Draw horizontal line
fn draw_hline(screen: *u8, x: u16, y: u16, len: u16, color: u8) -> void {
    var offset: u16 = y * 256 + x;
    var i: u16 = 0;
    
    while i < len {
        screen[offset + i] = color;
        i = i + 1;
    }
}

// Draw vertical line
fn draw_vline(screen: *u8, x: u16, y: u16, len: u16, color: u8) -> void {
    var i: u16 = 0;
    
    while i < len {
        screen[(y + i) * 256 + x] = color;
        i = i + 1;
    }
}

// Simple sprite blit (8x8)
fn blit_sprite(screen: *u8, sprite: *u8, x: u16, y: u16) -> void {
    var row: u16 = 0;
    
    while row < 8 {
        var col: u16 = 0;
        var screen_offset: u16 = (y + row) * 256 + x;
        var sprite_offset: u16 = row * 8;
        
        while col < 8 {
            var pixel = sprite[sprite_offset + col];
            if pixel != 0 {  // 0 = transparent
                screen[screen_offset + col] = pixel;
            }
            col = col + 1;
        }
        row = row + 1;
    }
}
`

	// Write source file
	sourceFile := filepath.Join(h.workDir, "graphics_bench.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(minzSource), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Set up mock screen buffer at 0x4000
	screenAddr := uint16(0x4000)
	
	// Set up test sprite at 0x9000
	spriteAddr := uint16(0x9000)
	testSprite := []byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF,
		0xFF, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0xFF,
		0xFF, 0x00, 0xFF, 0x00, 0x00, 0xFF, 0x00, 0xFF,
		0xFF, 0x00, 0xFF, 0x00, 0x00, 0xFF, 0x00, 0xFF,
		0xFF, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0xFF,
		0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	}
	for i, b := range testSprite {
		h.memory.WriteByte(spriteAddr+uint16(i), b)
	}

	// Benchmark scenarios
	benchmarks := []struct {
		name     string
		function string
		args     []uint16
		desc     string
	}{
		{
			name:     "fill_rect_small",
			function: "fill_rect",
			args:     []uint16{screenAddr, 10, 10, 8, 8, 0xFF},
			desc:     "Fill 8x8 rectangle",
		},
		{
			name:     "fill_rect_large",
			function: "fill_rect",
			args:     []uint16{screenAddr, 0, 0, 32, 24, 0x55},
			desc:     "Fill 32x24 rectangle",
		},
		{
			name:     "draw_hline",
			function: "draw_hline",
			args:     []uint16{screenAddr, 0, 100, 256, 0xFF},
			desc:     "Draw full-width horizontal line",
		},
		{
			name:     "draw_vline",
			function: "draw_vline",
			args:     []uint16{screenAddr, 128, 0, 192, 0xFF},
			desc:     "Draw full-height vertical line",
		},
		{
			name:     "blit_sprite",
			function: "blit_sprite",
			args:     []uint16{screenAddr, spriteAddr, 120, 90},
			desc:     "Blit 8x8 sprite",
		},
	}

	fmt.Printf("\n=== Real-World Graphics Benchmark ===\n")
	fmt.Printf("%-20s %-40s %10s %10s %10s %8s\n", 
		"Function", "Description", "No TSMC", "TSMC", "Reduction", "Speedup")
	fmt.Printf("%s\n", strings.Repeat("-", 96))

	totalNoTSMC := 0
	totalTSMC := 0

	for _, bench := range benchmarks {
		comparison, err := h.ComparePerformance(sourceFile, bench.function, bench.args...)
		if err != nil {
			t.Errorf("%s failed: %v", bench.name, err)
			continue
		}

		fmt.Printf("%-20s %-40s %10d %10d %9.1f%% %7.2fx\n",
			bench.function, bench.desc, 
			comparison.NoTSMCCycles, comparison.TSMCCycles,
			comparison.CycleReduction, comparison.SpeedupFactor)

		totalNoTSMC += comparison.NoTSMCCycles
		totalTSMC += comparison.TSMCCycles

		// Verify performance improvement
		if comparison.CycleReduction < 20.0 {
			t.Logf("Warning: %s showed low improvement: %.1f%%", bench.name, comparison.CycleReduction)
		}
	}

	// Overall summary
	if totalNoTSMC > 0 {
		overallReduction := float64(totalNoTSMC-totalTSMC) / float64(totalNoTSMC) * 100
		overallSpeedup := float64(totalNoTSMC) / float64(totalTSMC)
		
		fmt.Printf("%s\n", strings.Repeat("-", 96))
		fmt.Printf("%-20s %-40s %10d %10d %9.1f%% %7.2fx\n",
			"TOTAL", "All operations combined",
			totalNoTSMC, totalTSMC, overallReduction, overallSpeedup)

		if overallReduction >= 30.0 {
			fmt.Printf("\n✓ TSMC achieves %.1f%% overall performance improvement!\n", overallReduction)
		} else {
			t.Errorf("Overall TSMC improvement below 30%%: %.1f%%", overallReduction)
		}
	}
}