package spectrum

const (
	// BeeperSampleRate is the audio output sample rate.
	BeeperSampleRate = 44100

	// beeperBufSize is the circular buffer size (~2 frames worth of audio).
	beeperBufSize = 2048
)

// Beeper implements 1-bit audio with per-T-state sampling accuracy.
type Beeper struct {
	cpuClockHz     int
	frameTStates   int
	earBit         bool   // current EAR output state
	lastChangeTState int  // T-state of last EAR bit change

	// Per-frame accumulation: we record level transitions.
	changes []beeperChange

	// Output sample buffer (circular).
	buf       [beeperBufSize]float32
	bufWrite  int
	bufRead   int
	bufCount  int

	enabled bool
}

type beeperChange struct {
	tstate int
	level  bool
}

// NewBeeper creates a beeper for the given video mode.
func NewBeeper(mode *VideoMode) *Beeper {
	return &Beeper{
		cpuClockHz:   mode.CPUClockHz,
		frameTStates: mode.TStatesPerFrame(),
		changes:      make([]beeperChange, 0, 256),
		enabled:      true,
	}
}

// SetEar updates the EAR bit state at the given T-state within the frame.
func (b *Beeper) SetEar(ear bool, tstate int) {
	if ear == b.earBit {
		return
	}
	b.earBit = ear
	b.lastChangeTState = tstate
	b.changes = append(b.changes, beeperChange{tstate: tstate, level: ear})
}

// EndFrame downsamples the frame's audio data to 44100 Hz and appends
// to the output buffer. Call once per frame after RunFrame completes.
func (b *Beeper) EndFrame() {
	if !b.enabled {
		b.changes = b.changes[:0]
		return
	}

	// Number of output samples for this frame
	samplesPerFrame := BeeperSampleRate / 50 // 882 samples at 50 Hz

	changeIdx := 0
	currentLevel := false
	if len(b.changes) > 0 {
		// Start with opposite of first change (the state before changes)
		currentLevel = !b.changes[0].level
	}

	for i := 0; i < samplesPerFrame; i++ {
		// Map this sample's end point to a T-state
		tEnd := ((i + 1) * b.frameTStates) / samplesPerFrame

		// Advance through changes that fall within this sample
		for changeIdx < len(b.changes) && b.changes[changeIdx].tstate < tEnd {
			currentLevel = b.changes[changeIdx].level
			changeIdx++
		}

		// Simple: use the last known level for this sample
		var sample float32
		if currentLevel {
			sample = 0.5
		} else {
			sample = -0.5
		}

		// Write to circular buffer
		if b.bufCount < beeperBufSize {
			b.buf[b.bufWrite] = sample
			b.bufWrite = (b.bufWrite + 1) % beeperBufSize
			b.bufCount++
		}
	}

	b.changes = b.changes[:0]
}

// ReadSamples drains up to len(out) samples from the buffer.
// Returns the number of samples actually read.
func (b *Beeper) ReadSamples(out []float32) int {
	n := len(out)
	if n > b.bufCount {
		n = b.bufCount
	}
	for i := 0; i < n; i++ {
		out[i] = b.buf[b.bufRead]
		b.bufRead = (b.bufRead + 1) % beeperBufSize
	}
	b.bufCount -= n
	return n
}

// Available returns the number of samples available for reading.
func (b *Beeper) Available() int {
	return b.bufCount
}

// SetEnabled enables or disables audio output.
func (b *Beeper) SetEnabled(enabled bool) {
	b.enabled = enabled
}
