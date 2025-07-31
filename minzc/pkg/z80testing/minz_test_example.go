package z80testing

import (
	"testing"
)

// Example: Testing MinZ compiled functions
func TestMinZAdd(t *testing.T) {
	test := NewMinZTest(t)
	
	// Load compiled MinZ program
	err := test.LoadA80("testdata/math.a80")
	if err != nil {
		t.Fatal(err)
	}
	
	// Load symbol table
	err = test.LoadSymbols("testdata/math.sym")
	if err != nil {
		t.Fatal(err)
	}
	
	// Test the add function
	test.TestStandardFunction("add", []struct {
		Args     []uint16
		Expected uint16
	}{
		{[]uint16{5, 3}, 8},
		{[]uint16{100, 200}, 300},
		{[]uint16{0xFFFF, 1}, 0}, // Overflow test
	})
}

// Example: Testing MinZ array operations
func TestMinZArraySum(t *testing.T) {
	test := NewMinZTest(t)
	
	// Load program
	test.LoadA80("testdata/array_ops.a80")
	test.LoadSymbols("testdata/array_ops.sym")
	
	// Set up array in memory
	arrayAddr := test.symbols["test_array"]
	test.Given().
		Memory(arrayAddr, 1, 2, 3, 4, 5)
	
	// Call sum_array(array_ptr, length)
	test.CallFunction("sum_array", arrayAddr, 5)
	
	// Check result
	test.Then().
		Register("HL", 15) // 1+2+3+4+5
}

// Example: Testing with global variables
func TestMinZWithGlobals(t *testing.T) {
	test := NewMinZTest(t)
	
	test.LoadA80("testdata/globals.a80")
	test.LoadSymbols("testdata/globals.sym")
	
	// Set up global variables
	test.GivenFunction("process_data").
		WithGlobals(map[string]interface{}{
			"counter": uint16(10),
			"flags":   byte(0x03),
			"buffer":  []byte{0x11, 0x22, 0x33},
		})
	
	// Call function
	test.CallFunction("process_data")
	
	// Check that globals were modified
	counterAddr := test.symbols["counter"]
	test.Then().
		Memory(counterAddr, 11, 0) // counter incremented
}

// Example: Testing interrupt handlers
func TestMinZInterruptHandler(t *testing.T) {
	test := NewMinZTest(t)
	
	test.LoadA80("testdata/interrupts.a80")
	test.LoadSymbols("testdata/interrupts.sym")
	
	// Set up interrupt vector
	test.Given().
		Register("I", 0x80).
		Memory(0x8038, 0x00, 0x90) // IM 2 vector points to 0x9000
	
	// Trigger interrupt
	test.When().
		Execute(1) // Execute one instruction
		// TODO: Interrupt(false, 0x38) // Maskable interrupt - not implemented yet
	
	// Verify interrupt handler was called
	test.Then().
		Memory(test.symbols["interrupt_count"], 1, 0)
}

// Example: Testing MinZ struct operations
func TestMinZStructAccess(t *testing.T) {
	test := NewMinZTest(t)
	
	test.LoadA80("testdata/structs.a80")
	test.LoadSymbols("testdata/structs.sym")
	
	// Create a Point struct in memory
	// struct Point { x: u16, y: u16 }
	pointAddr := uint16(0x4000)
	test.Given().
		Memory(pointAddr, 
			0x10, 0x00, // x = 16
			0x20, 0x00, // y = 32
		)
	
	// Call get_distance(point_ptr)
	test.CallFunction("get_distance", pointAddr)
	
	// Function should calculate distance from origin
	// This is a simplified test - actual distance calculation
	// would be more complex
	test.Then().
		Register("HL", 48) // Simplified: x + y
}

// Example: Testing self-modifying code
func TestMinZSelfModifyingCode(t *testing.T) {
	test := NewMinZTest(t)
	
	test.LoadA80("testdata/smc_optimized.a80")
	test.LoadSymbols("testdata/smc_optimized.sym")
	
	// First call - should modify code
	test.CallFunction("optimized_multiply", 5)
	result1 := test.GetResult()
	
	// Second call - should use modified code path
	test.CallFunction("optimized_multiply", 5)
	result2 := test.GetResult()
	
	// Both should give same result but second should be faster
	if result1 != result2 {
		t.Errorf("SMC optimization changed result: %d != %d", result1, result2)
	}
	
	// Could also check cycle counts to verify optimization
}

// Example: Integration test for complete MinZ module
func TestMinZModule(t *testing.T) {
	test := NewMinZTest(t)
	
	// Load entire module
	test.LoadA80("testdata/string_utils.a80")
	test.LoadSymbols("testdata/string_utils.sym")
	
	// Set up test string
	strAddr := uint16(0x5000)
	test.Given().
		Memory(strAddr, []byte("Hello, World!\x00")...)
	
	// Test strlen
	test.CallFunction("strlen", strAddr)
	test.AssertResult(13)
	
	// Test strupr (convert to uppercase)
	test.CallFunction("strupr", strAddr)
	test.Then().
		Memory(strAddr, []byte("HELLO, WORLD!\x00")...)
	
	// Test strchr (find character)
	test.CallFunction("strchr", strAddr, uint16(','))
	expectedAddr := strAddr + 5 // Position of comma
	test.AssertResult(expectedAddr)
}

// Example: Benchmark test
func BenchmarkMinZFunction(b *testing.B) {
	test := NewMinZTest(&testing.T{})
	test.LoadA80("testdata/math.a80")
	test.LoadSymbols("testdata/math.sym")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		test.cpu.Reset()
		test.CallFunction("fast_multiply", 7, 8)
	}
}

// Example: Table-driven tests for MinZ
func TestMinZMathOperations(t *testing.T) {
	operations := []struct {
		name     string
		function string
		cases    []struct {
			args     []uint16
			expected uint16
		}
	}{
		{
			name:     "Addition",
			function: "add",
			cases: []struct {
				args     []uint16
				expected uint16
			}{
				{[]uint16{1, 1}, 2},
				{[]uint16{255, 1}, 256},
				{[]uint16{0xFFFF, 1}, 0},
			},
		},
		{
			name:     "Subtraction",
			function: "sub",
			cases: []struct {
				args     []uint16
				expected uint16
			}{
				{[]uint16{5, 3}, 2},
				{[]uint16{3, 5}, 0xFFFE}, // Underflow
			},
		},
		{
			name:     "Multiplication",
			function: "mul",
			cases: []struct {
				args     []uint16
				expected uint16
			}{
				{[]uint16{7, 8}, 56},
				{[]uint16{255, 2}, 510},
			},
		},
	}
	
	test := NewMinZTest(t)
	test.LoadA80("testdata/math.a80")
	test.LoadSymbols("testdata/math.sym")
	
	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			for _, tc := range op.cases {
				test.cpu.Reset()
				test.CallFunction(op.function, tc.args...)
				test.AssertResult(tc.expected)
			}
		})
	}
}