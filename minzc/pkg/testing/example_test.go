package testing

import (
	"testing"
)

// Example: Testing a simple addition routine
func TestAddition(t *testing.T) {
	test := NewTest(t)
	
	test.Given().
		Register("A", 5).
		Register("B", 3).
		Code(0x8000, 
			0x80, // ADD A, B
			0xC9, // RET
		)
	
	test.When().Execute(2)
	
	test.Then().
		Register("A", 8).
		Flag("Z", false).
		Flag("C", false).
		Cycles(4, 11) // ADD A,B = 4 cycles, RET = 7 cycles
}

// Example: Testing a 16-bit addition routine
func TestAdd16(t *testing.T) {
	test := NewTest(t)
	
	test.Given().
		Register("HL", 0x1234).
		Register("BC", 0x5678).
		Code(0x8000,
			0x09, // ADD HL, BC
			0xC9, // RET
		)
	
	test.When().Execute(2)
	
	test.Then().
		Register("HL", 0x68AC).
		Flag("C", false)
}

// Example: Testing memory operations
func TestMemoryCopy(t *testing.T) {
	test := NewTest(t)
	
	test.Given().
		Register("HL", 0x4000). // Source
		Register("DE", 0x5000). // Destination
		Register("BC", 0x0003). // Count
		Memory(0x4000, 0x11, 0x22, 0x33).
		Code(0x8000,
			0xED, 0xB0, // LDIR
			0xC9,       // RET
		)
	
	test.When().Execute(100) // Enough cycles for LDIR
	
	test.Then().
		Memory(0x5000, 0x11, 0x22, 0x33).
		Register("BC", 0x0000).
		Register("HL", 0x4003).
		Register("DE", 0x5003)
}

// Example: Testing I/O operations
func TestPortOutput(t *testing.T) {
	test := NewTest(t)
	
	test.Given().
		Register("A", 0x42).
		Register("BC", 0x00FE). // Port address in BC
		Code(0x8000,
			0xED, 0x79, // OUT (C), A
			0xC9,       // RET
		)
	
	test.When().Execute(2)
	
	test.Then().
		Port(0x00FE, 0x42)
}

// Example: Testing a subroutine with stack
func TestSubroutineCall(t *testing.T) {
	test := NewTest(t)
	
	// Subroutine that doubles A
	test.Given().
		Register("A", 7).
		Register("SP", 0xFFFE).
		Code(0x8000,
			0xCD, 0x00, 0x90, // CALL 0x9000
			0xC9,             // RET
		).
		Code(0x9000,
			0x87, // ADD A, A (double A)
			0xC9, // RET
		)
	
	test.When().ExecuteUntil(0x8004) // Execute until after CALL returns
	
	test.Then().
		Register("A", 14).
		Register("PC", 0x8004) // After the CALL
}

// Example: Testing conditional jumps
func TestConditionalJump(t *testing.T) {
	test := NewTest(t)
	
	test.Given().
		Register("A", 0).
		Code(0x8000,
			0x3C,       // INC A
			0xFE, 0x05, // CP 5
			0x20, 0xFC, // JR NZ, -4 (loop back)
			0xC9,       // RET
		)
	
	test.When().Execute(50) // Enough for 5 iterations
	
	test.Then().
		Register("A", 5).
		Flag("Z", true) // A == 5, so Z flag is set
}

// Example: Testing with MinZ function
func TestMinZFunction(t *testing.T) {
	test := NewTest(t)
	
	// Simulate a MinZ function: u16 add(u16 a, u16 b)
	// MinZ calling convention: first arg in HL, second in DE, result in HL
	test.Given().
		Register("HL", 0x1000). // First argument
		Register("DE", 0x0234). // Second argument
		Code(0x8000,
			// Function prologue would be here
			0x19, // ADD HL, DE
			// Function epilogue would be here
			0xC9, // RET
		)
	
	test.When().Call(0x8000)
	
	test.Then().
		Register("HL", 0x1234) // Result
}

// Example: Testing memory access patterns
func TestMemoryAccess(t *testing.T) {
	test := NewTest(t)
	
	// Test that ROM area is write-protected
	test.Given().
		Register("HL", 0x0000). // ROM address
		Register("A", 0xFF).
		Memory(0x0000, 0x00). // Initial ROM value
		Code(0x8000,
			0x77, // LD (HL), A - attempt to write to ROM
			0xC9, // RET
		)
	
	test.When().Execute(2)
	
	test.Then().
		Memory(0x0000, 0x00) // ROM unchanged
}

// Example: Complex test with multiple assertions
func TestComplexRoutine(t *testing.T) {
	test := NewTest(t)
	
	// Test a routine that processes an array
	test.Given().
		Register("HL", 0x4000). // Array start
		Register("B", 3).       // Array length
		Memory(0x4000, 1, 2, 3).
		Code(0x8000,
			// Sum array pointed by HL, length in B, result in A
			0xAF,       // XOR A (clear A)
			0x86,       // ADD A, (HL) - label: loop
			0x23,       // INC HL
			0x10, 0xFC, // DJNZ loop (B--, jump if not zero)
			0xC9,       // RET
		)
	
	test.When().Execute(20)
	
	test.Then().
		Register("A", 6).    // 1 + 2 + 3
		Register("B", 0).    // Loop counter exhausted
		Register("HL", 0x4003) // Points past array
}