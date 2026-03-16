package spectrum

import (
	"testing"
)

func TestRZXReplayPortOverride(t *testing.T) {
	// Create a minimal 48K machine.
	m, err := New48K(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Set up an RZX frame with known IN values.
	inValues := []byte{0xBF, 0xFF, 0x42}

	// Run one frame with RZX replay.
	m.Ports.SetRZXFrame(inValues)

	// Port reads should return recorded values in sequence.
	v0 := m.Ports.ReadPort(0xFE) // keyboard row
	v1 := m.Ports.ReadPort(0xFE) // next read
	v2 := m.Ports.ReadPort(0xFE) // third read

	if v0 != 0xBF {
		t.Errorf("IN[0] = %02X, want BF", v0)
	}
	if v1 != 0xFF {
		t.Errorf("IN[1] = %02X, want FF", v1)
	}
	if v2 != 0x42 {
		t.Errorf("IN[2] = %02X, want 42", v2)
	}

	// Extra reads beyond recorded values should return 0xFF (default).
	v3 := m.Ports.ReadPort(0xFE)
	if v3 != 0xFF {
		t.Errorf("IN[3] = %02X, want FF (overflow)", v3)
	}

	// Disable RZX replay.
	m.Ports.SetRZXFrame(nil)
	if m.Ports.RZXActive() {
		t.Error("RZX should be inactive after SetRZXFrame(nil)")
	}
}

func TestReadVRAM(t *testing.T) {
	m, err := New48K(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Write to VRAM directly.
	m.Memory.Write(0x4000, 0xAA, false) // first bitmap byte
	m.Memory.Write(0x5800, 0x38, false) // first attribute (white paper)

	scr := m.ReadVRAM()
	if len(scr) != 6912 {
		t.Fatalf("ReadVRAM len = %d, want 6912", len(scr))
	}
	if scr[0] != 0xAA {
		t.Errorf("VRAM[0] = %02X, want AA", scr[0])
	}
	if scr[0x1800] != 0x38 {
		t.Errorf("VRAM[attr0] = %02X, want 38", scr[0x1800])
	}
}
