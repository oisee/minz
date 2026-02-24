package spectrum

import "sync"

const (
	// BeeperSampleRate is the audio output sample rate.
	BeeperSampleRate = 44100

	// beeperBufSize is the circular buffer size (~10 frames worth of audio).
	// Must be large enough to absorb timing jitter between frame production
	// and audio callback consumption.
	beeperBufSize = 8192
)

// Beeper implements 1-bit audio with per-T-state sampling accuracy.
type Beeper struct {
	cpuClockHz   int
	frameTStates int
	earBit       bool // current EAR output state

	// Per-frame accumulation: we record level transitions.
	changes []beeperChange

	// Output sample buffer (circular), protected by mutex since
	// EndFrame (main goroutine) and ReadSamples (audio goroutine)
	// access it concurrently.
	mu       sync.Mutex
	buf      [beeperBufSize]float32
	bufWrite int
	bufRead  int
	bufCount int

	enabled bool
}

type beeperChange struct {
	tstate int
	level  bool
}

// NewBeeper creates a beeper for the given video mode.
func NewBeeper(mode *VideoMode) *Beeper {
	b := &Beeper{
		cpuClockHz:   mode.CPUClockHz,
		frameTStates: mode.TStatesPerFrame(),
		changes:      make([]beeperChange, 0, 256),
		enabled:      true,
	}
	// Pre-fill buffer with silence so the audio callback never
	// underruns during the first few frames before emulation catches up.
	for i := 0; i < beeperBufSize/2; i++ {
		b.buf[i] = 0
	}
	b.bufCount = beeperBufSize / 2
	b.bufWrite = beeperBufSize / 2
	return b
}

// SetEar updates the EAR bit state at the given T-state within the frame.
func (b *Beeper) SetEar(ear bool, tstate int) {
	if ear == b.earBit {
		return
	}
	b.earBit = ear
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

	if len(b.changes) == 0 {
		// No EAR bit changes this frame — output silence.
		// A static EAR bit is a DC offset (inaudible on real hardware due to
		// AC coupling). Outputting 0.0 prevents clicks from buffer underruns.
		b.mu.Lock()
		for i := 0; i < samplesPerFrame; i++ {
			if b.bufCount < beeperBufSize {
				b.buf[b.bufWrite] = 0
				b.bufWrite = (b.bufWrite + 1) % beeperBufSize
				b.bufCount++
			}
		}
		b.mu.Unlock()
		return
	}

	changeIdx := 0
	// Start with the level BEFORE the first transition.
	// Since SetEar filters no-ops, the first change is always a toggle,
	// so the level before it is the opposite of the first change's level.
	currentLevel := !b.changes[0].level

	b.mu.Lock()
	for i := 0; i < samplesPerFrame; i++ {
		// Map this sample's end point to a T-state
		tEnd := ((i + 1) * b.frameTStates) / samplesPerFrame

		// Advance through changes that fall within this sample
		for changeIdx < len(b.changes) && b.changes[changeIdx].tstate < tEnd {
			currentLevel = b.changes[changeIdx].level
			changeIdx++
		}

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
	b.mu.Unlock()

	b.changes = b.changes[:0]
}

// ReadSamples drains up to len(out) samples from the buffer.
// Returns the number of samples actually read.
// Called from the audio goroutine.
func (b *Beeper) ReadSamples(out []float32) int {
	b.mu.Lock()
	n := len(out)
	if n > b.bufCount {
		n = b.bufCount
	}
	for i := 0; i < n; i++ {
		out[i] = b.buf[b.bufRead]
		b.bufRead = (b.bufRead + 1) % beeperBufSize
	}
	b.bufCount -= n
	b.mu.Unlock()
	return n
}

// Available returns the number of samples available for reading.
func (b *Beeper) Available() int {
	b.mu.Lock()
	n := b.bufCount
	b.mu.Unlock()
	return n
}

// SetEnabled enables or disables audio output.
func (b *Beeper) SetEnabled(enabled bool) {
	b.enabled = enabled
}
