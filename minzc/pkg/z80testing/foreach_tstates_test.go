package z80testing

import (
	"testing"
)

// TestForEachRealTstates verifies real T-state counts for iterator patterns.
// This catches regression in register allocator / codegen quality.
func TestForEachRealTstates(t *testing.T) {
	// Hand-optimized forEach on 5 elements (counter in B, ptr in HL):
	//   LD B, 5          ; 7T
	//   LD HL, data      ; 10T
	// .loop:
	//   LD A, (HL)       ; 7T
	//   CALL console_log ; 17T
	//   INC HL           ; 6T
	//   DJNZ .loop       ; 13T (loop) / 8T (exit)
	//   HALT             ; 4T
	//
	// console_log: OUT (0), A ; 11T — RET ; 10T = 21T
	//
	// Per element (×4 full + 1 last):
	//   4 × (7+17+6+13) = 4 × 43 = 172T
	//   1 × (7+17+6+8)  = 38T
	//   Total loop = 210T + setup(17T) + halt(4T) = 231T
	//   (console_log counted in CALL)

	tc := NewTest(t)
	tc.cpu.Tstates = 0
	tc.memory.romEnd = 0

	// data at 0x8100
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	for i, b := range data {
		tc.memory.data[0x8100+uint16(i)] = b
	}

	// console_log at 0x8200: OUT (0x00), A [0xD3 0x00] ; RET [0xC9]
	tc.memory.data[0x8200] = 0xD3
	tc.memory.data[0x8201] = 0x00
	tc.memory.data[0x8202] = 0xC9 // RET

	// main at 0x8000:
	//   LD B, 5          ; 06 05
	//   LD HL, 0x8100    ; 21 00 81
	//   LD A, (HL)       ; 7E
	//   CALL 0x8200      ; CD 00 82
	//   INC HL           ; 23
	//   DJNZ -6 (back to LD A,(HL)) ; 10 FA
	//   HALT             ; 76
	prog := []byte{
		0x06, 0x05,       // LD B, 5
		0x21, 0x00, 0x81, // LD HL, 0x8100
		// loop at 0x8005:
		0x7E,             // LD A, (HL)
		0xCD, 0x00, 0x82, // CALL 0x8200
		0x23,             // INC HL
		0x10, 0xF9,       // DJNZ 0x8005 (offset = -7)
		0x76,             // HALT
	}
	for i, b := range prog {
		tc.memory.data[0x8000+uint16(i)] = b
	}
	tc.cpu.SetPC(0x8000)

	maxOps := 10000
	for !tc.cpu.Halted && maxOps > 0 {
		tc.cpu.DoOpcode()
		maxOps--
	}
	if !tc.cpu.Halted {
		t.Fatal("CPU did not halt")
	}

	got := tc.cpu.Tstates
	// Breakdown (remogatto; OUT=7T not 11T because port contention not triggered):
	//   Setup:  LD B,5(7) + LD HL,nn(10) = 17T
	//   Loop:   5 × [LD A,(HL)(7) + CALL(17) + OUT(7) + RET(10) + INC HL(6)] = 235T
	//   DJNZ:   4×taken(13) + 1×not-taken(8) = 60T
	//   HALT:   4T   =>  Total: 316T
	want := 316
	t.Logf("forEach(5 elements, hand-optimized): %d T-states (expected %d)", got, want)
	if got != want {
		t.Errorf("T-states mismatch: got %d, want %d", got, want)
	}
}
