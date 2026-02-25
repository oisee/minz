package spectrum

import "sort"

// Tape signal timing constants (in T-states), matching the ZX Spectrum ROM
// tape encoding format exactly.
const (
	// Pilot tone: long pulses to synchronize the ROM loader
	PilotPulseLen   = 2168 // T-states per pilot half-pulse
	PilotHeader     = 8063 // Number of pilot pulses for header blocks
	PilotData       = 3223 // Number of pilot pulses for data blocks

	// Sync pulses: mark the transition from pilot tone to data
	SyncPulse1Len = 667  // T-states for first sync half-pulse
	SyncPulse2Len = 735  // T-states for second sync half-pulse

	// Data pulses: bit encoding
	ZeroBitPulseLen = 855  // T-states per half-pulse for bit 0
	OneBitPulseLen  = 1710 // T-states per half-pulse for bit 1

	// Inter-block pause
	PauseLen = 3500000 // 1 second at 3.5MHz (standard inter-block pause)
)

// TapePulse is one half-cycle of the tape signal: a level held for a duration.
type TapePulse struct {
	Level    bool // true = high, false = low
	Duration int  // T-states this level is held
}

// TapeSignalProvider generates the tape signal for real-time loading.
// The ROM's LD-BYTES reads bit 6 of port $FE in a tight loop.
// This provider pre-computes the complete waveform and serves the
// current level via binary search by T-state.
type TapeSignalProvider struct {
	// edges[i] = T-state at which the signal transitions.
	// Between edges[i] and edges[i+1], the level is levels[i].
	edges  []int64
	levels []bool

	// Absolute T-state counter (accumulates across frames)
	totalTStates int64

	// T-state when the tape started playing
	startTStates int64

	// Playing state
	playing  bool
	finished bool

	// Current block index (for progress reporting)
	blockIndex int
	blockCount int
}

// NewTapeSignalProvider creates a signal provider from raw tape blocks.
// Each block is (flag byte, payload data) — the same format as TAPBlock.
func NewTapeSignalProvider(blocks []TapeBlockData) *TapeSignalProvider {
	tsp := &TapeSignalProvider{
		blockCount: len(blocks),
	}

	var pulses []TapePulse
	for i, block := range blocks {
		// Inter-block pause (except before first block)
		if i > 0 {
			pulses = append(pulses, TapePulse{Level: false, Duration: PauseLen})
		}
		pulses = append(pulses, generateBlockPulses(block.Flag, block.Data)...)
	}

	// Convert pulses to edge/level arrays for binary search
	tsp.buildEdges(pulses)
	return tsp
}

// TapeBlockData holds one tape block for signal generation.
type TapeBlockData struct {
	Flag byte
	Data []byte
}

// generateBlockPulses generates the full pulse sequence for one tape block.
func generateBlockPulses(flag byte, data []byte) []TapePulse {
	var pulses []TapePulse
	level := false

	// Pilot tone
	pilotCount := PilotData
	if flag == 0x00 {
		pilotCount = PilotHeader
	}
	for i := 0; i < pilotCount; i++ {
		pulses = append(pulses, TapePulse{Level: level, Duration: PilotPulseLen})
		level = !level
	}

	// Sync pulses
	pulses = append(pulses, TapePulse{Level: level, Duration: SyncPulse1Len})
	level = !level
	pulses = append(pulses, TapePulse{Level: level, Duration: SyncPulse2Len})
	level = !level

	// Data bytes: flag + payload + checksum
	// Compute checksum
	var checksum byte
	checksum ^= flag
	for _, b := range data {
		checksum ^= b
	}

	// Encode all bytes: flag, then data, then checksum
	allBytes := make([]byte, 0, len(data)+2)
	allBytes = append(allBytes, flag)
	allBytes = append(allBytes, data...)
	allBytes = append(allBytes, checksum)

	for _, b := range allBytes {
		for bit := 7; bit >= 0; bit-- {
			pulseLen := ZeroBitPulseLen
			if b&(1<<uint(bit)) != 0 {
				pulseLen = OneBitPulseLen
			}
			// Each bit = two half-pulses of equal duration
			pulses = append(pulses, TapePulse{Level: level, Duration: pulseLen})
			level = !level
			pulses = append(pulses, TapePulse{Level: level, Duration: pulseLen})
			level = !level
		}
	}

	// Trailing pulse (end marker)
	pulses = append(pulses, TapePulse{Level: level, Duration: 945})

	return pulses
}

// buildEdges converts a pulse sequence into cumulative edge timestamps
// for O(log n) binary search.
func (tsp *TapeSignalProvider) buildEdges(pulses []TapePulse) {
	tsp.edges = make([]int64, len(pulses)+1)
	tsp.levels = make([]bool, len(pulses))

	var t int64
	for i, p := range pulses {
		tsp.edges[i] = t
		tsp.levels[i] = p.Level
		t += int64(p.Duration)
	}
	tsp.edges[len(pulses)] = t // sentinel: end of tape
}

// Play starts the tape from the current absolute T-state count.
func (tsp *TapeSignalProvider) Play(currentTStates int64) {
	tsp.startTStates = currentTStates
	tsp.playing = true
	tsp.finished = false
	tsp.blockIndex = 0
}

// Stop stops the tape.
func (tsp *TapeSignalProvider) Stop() {
	tsp.playing = false
}

// IsPlaying returns true if the tape is currently playing.
func (tsp *TapeSignalProvider) IsPlaying() bool {
	return tsp.playing && !tsp.finished
}

// GetSignal returns the current tape signal (true/false) for the given
// absolute T-state count. This is injected into port $FE bit 6.
//
// Uses binary search through pre-computed edges for O(log n) per call.
func (tsp *TapeSignalProvider) GetSignal(absoluteTStates int64) bool {
	if !tsp.playing || tsp.finished {
		return false
	}

	elapsed := absoluteTStates - tsp.startTStates
	if elapsed < 0 {
		return false
	}

	// Past end of tape?
	if len(tsp.edges) == 0 || elapsed >= tsp.edges[len(tsp.edges)-1] {
		tsp.finished = true
		tsp.playing = false
		return false
	}

	// Binary search: find the last edge <= elapsed
	idx := sort.Search(len(tsp.edges), func(i int) bool {
		return tsp.edges[i] > elapsed
	}) - 1

	if idx < 0 {
		idx = 0
	}
	if idx >= len(tsp.levels) {
		idx = len(tsp.levels) - 1
	}

	return tsp.levels[idx]
}

// TotalDuration returns the total tape duration in T-states.
func (tsp *TapeSignalProvider) TotalDuration() int64 {
	if len(tsp.edges) == 0 {
		return 0
	}
	return tsp.edges[len(tsp.edges)-1]
}
