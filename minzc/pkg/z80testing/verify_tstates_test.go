package z80testing

import (
	"testing"
)

func TestTstateCountingAccuracy(t *testing.T) {
	// NOP=4T, DI=4T, HALT=4T => 3×NOP + DI + HALT = 20T
	// Plus: each instruction fetch reads opcode (3T each) BUT remogatto
	// counts those via ReadByte in the memory accessor.
	// NOP:  fetch(3T) + execute(1T) = 4T
	// DI:   fetch(3T) + execute(1T) = 4T
	// HALT: fetch(3T) + execute(1T) = 4T
	// Total: 5 instructions × 4T = 20T

	tc := NewTest(t)
	tc.cpu.Tstates = 0

	// Load: NOP NOP NOP DI HALT at 0x8000
	tc.memory.romEnd = 0 // no ROM protection
	code := []byte{0x00, 0x00, 0x00, 0xF3, 0x76}
	for i, b := range code {
		tc.memory.data[0x8000+uint16(i)] = b
	}
	tc.cpu.SetPC(0x8000)

	for !tc.cpu.Halted {
		tc.cpu.DoOpcode()
	}

	got := tc.cpu.Tstates
	want := 20
	if got != want {
		t.Errorf("T-states: got %d, want %d (NOP×3 + DI + HALT = 20T)", got, want)
	} else {
		t.Logf("✓ T-states correct: %d (NOP×3 + DI + HALT)", got)
	}

	// Also verify: LD A, n (7T) + HALT (4T) = 11T
	tc.cpu.Tstates = 0
	tc.cpu.Halted = false
	code2 := []byte{0x3E, 0x42, 0x76} // LD A, 0x42 ; HALT
	for i, b := range code2 {
		tc.memory.data[0x9000+uint16(i)] = b
	}
	tc.cpu.SetPC(0x9000)
	for !tc.cpu.Halted {
		tc.cpu.DoOpcode()
	}
	got2 := tc.cpu.Tstates
	want2 := 11
	if got2 != want2 {
		t.Errorf("LD A,n + HALT: got %d T-states, want %d", got2, want2)
	} else {
		t.Logf("✓ LD A,n(7T) + HALT(4T) = %d T-states correct", got2)
	}
}
