package spectrum

import "testing"

func TestTapeSignalHeaderBlock(t *testing.T) {
	// A minimal header block: flag=0x00, 17 bytes of zeros
	blocks := []TapeBlockData{
		{Flag: 0x00, Data: make([]byte, 17)},
	}
	tsp := NewTapeSignalProvider(blocks)

	if tsp.TotalDuration() == 0 {
		t.Fatal("expected non-zero tape duration")
	}

	// Header block should have 8063 pilot pulses
	// Total pilot duration = 8063 * 2168 = 17,480,584 T-states
	expectedPilotEnd := int64(PilotHeader) * int64(PilotPulseLen)
	t.Logf("Total duration: %d T-states (%.2f seconds at 3.5MHz)",
		tsp.TotalDuration(), float64(tsp.TotalDuration())/3500000.0)
	t.Logf("Expected pilot end: %d T-states", expectedPilotEnd)

	// Start tape at T=0
	tsp.Play(0)

	// The signal should alternate during pilot tone
	// First pulse starts at T=0 with level=false
	sig0 := tsp.GetSignal(0)
	sig1 := tsp.GetSignal(int64(PilotPulseLen) - 1)      // end of first half-pulse
	sig2 := tsp.GetSignal(int64(PilotPulseLen))           // start of second half-pulse
	sig3 := tsp.GetSignal(int64(PilotPulseLen)*2 - 1)     // end of second half-pulse

	t.Logf("Signal at T=0: %v", sig0)
	t.Logf("Signal at T=%d (end of pulse 0): %v", PilotPulseLen-1, sig1)
	t.Logf("Signal at T=%d (start of pulse 1): %v", PilotPulseLen, sig2)
	t.Logf("Signal at T=%d (end of pulse 1): %v", PilotPulseLen*2-1, sig3)

	// Pilot tone should alternate
	if sig0 == sig2 {
		t.Error("pilot tone should alternate between pulses")
	}

	// Signal should be stable within a pulse
	if sig0 != sig1 {
		t.Error("signal should be stable within a single pulse")
	}

	// Past end of tape
	sigEnd := tsp.GetSignal(tsp.TotalDuration() + 1000)
	if sigEnd != false {
		t.Error("signal past end of tape should be false")
	}
	if tsp.IsPlaying() {
		t.Error("tape should stop after reaching end")
	}
}

func TestTapeSignalDataBlock(t *testing.T) {
	// Data block (flag=0xFF) should have 3223 pilot pulses (shorter than header)
	blocks := []TapeBlockData{
		{Flag: 0xFF, Data: []byte{0xAA}}, // one byte of data
	}
	tsp := NewTapeSignalProvider(blocks)

	expectedPilotDuration := int64(PilotData) * int64(PilotPulseLen)
	t.Logf("Data block pilot duration: %d T-states (%.2f seconds)",
		expectedPilotDuration, float64(expectedPilotDuration)/3500000.0)
	t.Logf("Total tape duration: %d T-states (%.2f seconds)",
		tsp.TotalDuration(), float64(tsp.TotalDuration())/3500000.0)

	// Data blocks are shorter than header blocks
	headerBlocks := []TapeBlockData{
		{Flag: 0x00, Data: []byte{0xAA}},
	}
	headerTsp := NewTapeSignalProvider(headerBlocks)

	if tsp.TotalDuration() >= headerTsp.TotalDuration() {
		t.Error("data block should be shorter than header block (fewer pilot pulses)")
	}
}

func TestTapeSignalBinarySearch(t *testing.T) {
	// Create a simple block and verify binary search gives consistent results
	blocks := []TapeBlockData{
		{Flag: 0x00, Data: make([]byte, 1)},
	}
	tsp := NewTapeSignalProvider(blocks)
	tsp.Play(0)

	// Sample every 100 T-states and verify signal doesn't glitch
	prevSig := tsp.GetSignal(0)
	transitions := 0
	for ts := int64(100); ts < tsp.TotalDuration(); ts += 100 {
		sig := tsp.GetSignal(ts)
		if sig != prevSig {
			transitions++
			prevSig = sig
		}
	}

	t.Logf("Total transitions sampled: %d", transitions)
	// Should have many transitions (pilot + sync + data bits)
	if transitions < 10 {
		t.Error("expected many signal transitions")
	}
}
