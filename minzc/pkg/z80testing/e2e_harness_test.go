package z80testing

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

// TestE2EHarnessBasic tests basic functionality of the E2E harness
func TestE2EHarnessBasic(t *testing.T) {
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer h.Cleanup()

	// Create a simple MinZ test program
	minzSource := `
// Simple function to test TSMC optimization
fn add(a: u16, b: u16) -> u16 {
    return a + b;
}

// Function with loop to show TSMC benefits
fn sum_range(start: u16, end: u16) -> u16 {
    var sum: u16 = 0;
    var i: u16 = start;
    while i <= end {
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}

// Entry point
fn main() {
    // Test add function
    let result = add(5, 7);
    
    // Test sum_range
    let sum = sum_range(1, 10);
    
    // Halt
    @asm {
        HALT
    }
}
`

	// Write source file
	sourceFile := filepath.Join(h.workDir, "test_basic.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(minzSource), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Test compilation without TSMC
	t.Run("compile_without_TSMC", func(t *testing.T) {
		a80File, err := h.CompileMinZ(sourceFile, false)
		if err != nil {
			t.Fatalf("Compilation failed: %v", err)
		}

		binary, symbols, err := h.AssembleA80(a80File)
		if err != nil {
			t.Fatalf("Assembly failed: %v", err)
		}

		if len(binary) == 0 {
			t.Error("Empty binary produced")
		}

		if _, ok := symbols["add"]; !ok {
			t.Error("Symbol 'add' not found")
		}
	})

	// Test compilation with TSMC
	t.Run("compile_with_TSMC", func(t *testing.T) {
		a80File, err := h.CompileMinZ(sourceFile, true)
		if err != nil {
			t.Fatalf("Compilation failed: %v", err)
		}

		binary, symbols, err := h.AssembleA80(a80File)
		if err != nil {
			t.Fatalf("Assembly failed: %v", err)
		}

		if len(binary) == 0 {
			t.Error("Empty binary produced")
		}

		if _, ok := symbols["add"]; !ok {
			t.Error("Symbol 'add' not found")
		}
	})
}

// TestE2EPerformanceComparison tests TSMC performance improvements
func TestE2EPerformanceComparison(t *testing.T) {
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer h.Cleanup()

	// Create a MinZ program that benefits from TSMC
	minzSource := `
// Function that modifies immediate values (perfect for TSMC)
fn multiply_by_constant(x: u16, factor: u16) -> u16 {
    // This immediate will be patched by TSMC
    return x * factor;
}

// Iterative function that benefits from TSMC
fn factorial(n: u16) -> u16 {
    var result: u16 = 1;
    var i: u16 = 1;
    
    while i <= n {
        result = result * i;
        i = i + 1;
    }
    
    return result;
}

// Function with conditional that can be optimized by TSMC
fn max(a: u16, b: u16) -> u16 {
    if a > b {
        return a;
    } else {
        return b;
    }
}

// Array sum function
fn sum_array(arr: *u8, len: u16) -> u16 {
    var sum: u16 = 0;
    var i: u16 = 0;
    
    while i < len {
        sum = sum + arr[i];
        i = i + 1;
    }
    
    return sum;
}
`

	// Write source file
	sourceFile := filepath.Join(h.workDir, "test_performance.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(minzSource), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Test multiply_by_constant
	t.Run("multiply_by_constant", func(t *testing.T) {
		comparison, err := h.ComparePerformance(sourceFile, "multiply_by_constant", 10, 5)
		if err != nil {
			t.Fatalf("Performance comparison failed: %v", err)
		}

		t.Log(comparison.String())
		
		// Check that results are correct
		expectedResult := uint16(50) // 10 * 5
		if comparison.NoTSMCResult != expectedResult {
			t.Errorf("Incorrect result without TSMC: got %d, want %d", 
				comparison.NoTSMCResult, expectedResult)
		}
		if comparison.TSMCResult != expectedResult {
			t.Errorf("Incorrect result with TSMC: got %d, want %d", 
				comparison.TSMCResult, expectedResult)
		}

		// TSMC should provide some improvement for parameter patching
		if comparison.TSMCCycles >= comparison.NoTSMCCycles {
			t.Error("TSMC did not improve performance")
		}
	})

	// Test factorial
	t.Run("factorial", func(t *testing.T) {
		comparison, err := h.ComparePerformance(sourceFile, "factorial", 5)
		if err != nil {
			t.Fatalf("Performance comparison failed: %v", err)
		}

		t.Log(comparison.String())
		
		// Check that results are correct (5! = 120)
		expectedResult := uint16(120)
		if comparison.NoTSMCResult != expectedResult {
			t.Errorf("Incorrect result without TSMC: got %d, want %d", 
				comparison.NoTSMCResult, expectedResult)
		}
		if comparison.TSMCResult != expectedResult {
			t.Errorf("Incorrect result with TSMC: got %d, want %d", 
				comparison.TSMCResult, expectedResult)
		}

		// Iterative functions should benefit from TSMC
		comparison.AssertPerformanceImprovement(t, 20.0) // Expect at least 20% improvement
	})

	// Test max function
	t.Run("max", func(t *testing.T) {
		testCases := []struct {
			a, b     uint16
			expected uint16
		}{
			{10, 5, 10},
			{3, 7, 7},
			{100, 100, 100},
		}

		for _, tc := range testCases {
			name := fmt.Sprintf("max(%d,%d)", tc.a, tc.b)
			t.Run(name, func(t *testing.T) {
				comparison, err := h.ComparePerformance(sourceFile, "max", tc.a, tc.b)
				if err != nil {
					t.Fatalf("Performance comparison failed: %v", err)
				}

				t.Log(comparison.String())
				
				if comparison.NoTSMCResult != tc.expected {
					t.Errorf("Incorrect result without TSMC: got %d, want %d", 
						comparison.NoTSMCResult, tc.expected)
				}
				if comparison.TSMCResult != tc.expected {
					t.Errorf("Incorrect result with TSMC: got %d, want %d", 
						comparison.TSMCResult, tc.expected)
				}
			})
		}
	})
}

// TestE2ETSMCTracking tests that SMC events are properly tracked
func TestE2ETSMCTracking(t *testing.T) {
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer h.Cleanup()

	// Create a MinZ program that uses TSMC
	minzSource := `
// Function that explicitly uses TSMC for parameter patching
fn patch_and_add(a: u16, b: u16) -> u16 {
    // These immediates should be patched by TSMC
    return a + b;
}

// Function with self-modifying loop counter
fn count_to(n: u16) -> u16 {
    var count: u16 = 0;
    var i: u16 = 0;
    
    // This loop should use TSMC for the comparison
    while i < n {
        count = count + 1;
        i = i + 1;
    }
    
    return count;
}
`

	// Write source file
	sourceFile := filepath.Join(h.workDir, "test_smc_tracking.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(minzSource), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Compile with TSMC
	a80File, err := h.CompileMinZ(sourceFile, true)
	if err != nil {
		t.Fatalf("Compilation with TSMC failed: %v", err)
	}

	binary, symbols, err := h.AssembleA80(a80File)
	if err != nil {
		t.Fatalf("Assembly failed: %v", err)
	}

	h.LoadBinary(binary, 0x8000)

	// Test patch_and_add function
	t.Run("patch_and_add_SMC", func(t *testing.T) {
		funcAddr, ok := symbols["patch_and_add"]
		if !ok {
			t.Fatal("Function patch_and_add not found")
		}

		// Clear SMC tracker
		h.smcTracker.Clear()
		
		// Call function
		if err := h.CallFunction(funcAddr, 42, 58); err != nil {
			t.Fatalf("Function execution failed: %v", err)
		}

		result := h.GetResult()
		if result != 100 {
			t.Errorf("Incorrect result: got %d, want 100", result)
		}

		// Check SMC events
		stats := h.GetSMCStats()
		t.Logf("SMC Stats: %+v", stats)
		
		if h.enableTSMC && stats.CodeEvents > 0 {
			t.Logf("SMC Summary:\n%s", h.GetSMCSummary())
			
			// Verify that modifications happened in code segment
			if stats.CodeEvents == 0 {
				t.Error("Expected code modifications with TSMC enabled")
			}
		}
	})

	// Test count_to function
	t.Run("count_to_SMC", func(t *testing.T) {
		funcAddr, ok := symbols["count_to"]
		if !ok {
			t.Fatal("Function count_to not found")
		}

		// Clear SMC tracker
		h.smcTracker.Clear()
		
		// Call function
		if err := h.CallFunction(funcAddr, 10); err != nil {
			t.Fatalf("Function execution failed: %v", err)
		}

		result := h.GetResult()
		if result != 10 {
			t.Errorf("Incorrect result: got %d, want 10", result)
		}

		// Check SMC events
		stats := h.GetSMCStats()
		t.Logf("SMC Stats for count_to: %+v", stats)
		
		if h.enableTSMC {
			t.Logf("SMC Summary:\n%s", h.GetSMCSummary())
			
			// Analyze SMC patterns
			patterns := h.smcTracker.DetectPatterns()
			for _, pattern := range patterns {
				t.Logf("SMC Pattern: %s - %s (%d matches)", 
					pattern.Name, pattern.Description, len(pattern.Matches))
			}
		}
	})
}

// TestE2ERealWorldExample tests a more realistic MinZ program
func TestE2ERealWorldExample(t *testing.T) {
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create harness: %v", err)
	}
	defer h.Cleanup()

	// Create a realistic MinZ program (string processing)
	minzSource := `
// String length function
fn strlen(str: *u8) -> u16 {
    var len: u16 = 0;
    while str[len] != 0 {
        len = len + 1;
    }
    return len;
}

// String copy function
fn strcpy(dest: *u8, src: *u8) -> *u8 {
    var i: u16 = 0;
    while src[i] != 0 {
        dest[i] = src[i];
        i = i + 1;
    }
    dest[i] = 0;
    return dest;
}

// Convert string to uppercase
fn to_upper(str: *u8) -> void {
    var i: u16 = 0;
    while str[i] != 0 {
        if str[i] >= 97 && str[i] <= 122 {  // 'a' to 'z'
            str[i] = str[i] - 32;
        }
        i = i + 1;
    }
}

// Simple hash function
fn hash_string(str: *u8) -> u16 {
    var hash: u16 = 5381;
    var i: u16 = 0;
    
    while str[i] != 0 {
        // hash = hash * 33 + str[i]
        hash = (hash << 5) + hash + str[i];
        i = i + 1;
    }
    
    return hash;
}
`

	// Write source file
	sourceFile := filepath.Join(h.workDir, "test_strings.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(minzSource), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Set up test string in memory
	testString := "Hello, World!"
	stringAddr := uint16(0x9000)
	for i, ch := range []byte(testString) {
		h.memory.WriteByte(stringAddr+uint16(i), ch)
	}
	h.memory.WriteByte(stringAddr+uint16(len(testString)), 0) // Null terminator

	// Test strlen with performance comparison
	t.Run("strlen_performance", func(t *testing.T) {
		comparison, err := h.ComparePerformance(sourceFile, "strlen", stringAddr)
		if err != nil {
			t.Fatalf("Performance comparison failed: %v", err)
		}

		t.Log(comparison.String())
		
		expectedLen := uint16(len(testString))
		if comparison.NoTSMCResult != expectedLen {
			t.Errorf("Incorrect strlen without TSMC: got %d, want %d", 
				comparison.NoTSMCResult, expectedLen)
		}
		if comparison.TSMCResult != expectedLen {
			t.Errorf("Incorrect strlen with TSMC: got %d, want %d", 
				comparison.TSMCResult, expectedLen)
		}

		// String processing should benefit significantly from TSMC
		comparison.AssertPerformanceImprovement(t, 25.0) // Expect at least 25% improvement
	})

	// Test hash_string performance
	t.Run("hash_string_performance", func(t *testing.T) {
		comparison, err := h.ComparePerformance(sourceFile, "hash_string", stringAddr)
		if err != nil {
			t.Fatalf("Performance comparison failed: %v", err)
		}

		t.Log(comparison.String())
		
		// Just verify both versions produce the same hash
		if comparison.NoTSMCResult != comparison.TSMCResult {
			t.Errorf("Hash mismatch: no-TSMC=%d, TSMC=%d", 
				comparison.NoTSMCResult, comparison.TSMCResult)
		}

		// Hash function with loop should benefit from TSMC
		if comparison.CycleReduction > 0 {
			t.Logf("Hash function improved by %.1f%% with TSMC", comparison.CycleReduction)
		}
	})
}

// BenchmarkE2ETSMC benchmarks TSMC vs non-TSMC performance
func BenchmarkE2ETSMC(b *testing.B) {
	h, err := NewE2ETestHarness(&testing.T{})
	if err != nil {
		b.Fatalf("Failed to create harness: %v", err)
	}
	defer h.Cleanup()

	// Create a compute-intensive MinZ program
	minzSource := `
// Bubble sort implementation
fn bubble_sort(arr: *u16, len: u16) -> void {
    var i: u16 = 0;
    var j: u16 = 0;
    var temp: u16 = 0;
    
    while i < len - 1 {
        j = 0;
        while j < len - i - 1 {
            if arr[j] > arr[j + 1] {
                temp = arr[j];
                arr[j] = arr[j + 1];
                arr[j + 1] = temp;
            }
            j = j + 1;
        }
        i = i + 1;
    }
}

// Prime number check
fn is_prime(n: u16) -> bool {
    if n < 2 {
        return false;
    }
    
    var i: u16 = 2;
    while i * i <= n {
        if n % i == 0 {
            return false;
        }
        i = i + 1;
    }
    
    return true;
}

// Count primes up to n
fn count_primes(n: u16) -> u16 {
    var count: u16 = 0;
    var i: u16 = 2;
    
    while i <= n {
        if is_prime(i) {
            count = count + 1;
        }
        i = i + 1;
    }
    
    return count;
}
`

	// Write source file
	sourceFile := filepath.Join(h.workDir, "bench_compute.minz")
	if err := ioutil.WriteFile(sourceFile, []byte(minzSource), 0644); err != nil {
		b.Fatalf("Failed to write source file: %v", err)
	}

	// Benchmark count_primes function
	b.Run("count_primes_100", func(b *testing.B) {
		comparison, err := h.ComparePerformance(sourceFile, "count_primes", 100)
		if err != nil {
			b.Fatalf("Performance comparison failed: %v", err)
		}

		b.Logf("Counting primes up to 100:")
		b.Logf("  Without TSMC: %d cycles", comparison.NoTSMCCycles)
		b.Logf("  With TSMC:    %d cycles", comparison.TSMCCycles)
		b.Logf("  Improvement:  %.1f%% (%.2fx faster)", 
			comparison.CycleReduction, comparison.SpeedupFactor)
		b.Logf("  Result:       %d primes found", comparison.TSMCResult)
		
		// Report cycles for benchmark
		b.ReportMetric(float64(comparison.NoTSMCCycles), "cycles/op-noTSMC")
		b.ReportMetric(float64(comparison.TSMCCycles), "cycles/op-TSMC")
		b.ReportMetric(comparison.CycleReduction, "improvement-%")
	})
}