package spectrum

import "sync"

const (
	// BeeperSampleRate is the audio output sample rate.
	BeeperSampleRate = 44100

	// beeperBufSize is the circular buffer size (~10 frames worth of audio).
	beeperBufSize = 8192

	// beeperTargetFill is the ideal number of samples in the buffer.
	// 1 frame (882 samples = 20ms) — minimal latency.
	beeperTargetFill = BeeperSampleRate / 50

	// beeperNominalSPF is the nominal samples per frame at 50Hz.
	beeperNominalSPF = BeeperSampleRate / 50 // 882

	// beeperFadeLen is the number of samples for anti-click fade in/out.
	// 32 samples at 44100Hz = ~0.7ms — inaudible but eliminates DC offset clicks.
	beeperFadeLen = 32
)

// Beeper implements 1-bit audio with per-T-state sampling accuracy.
//
// Audio architecture (FUSE-style adaptive rate control):
//
//	Frame execution:  OUT ($FE) → SetEar(level, tstate) records transitions
//	Frame end:        EndFrame() → downsamples transitions to PCM samples
//	Audio callback:   ReadSamples(buf) → drains ring buffer (async goroutine)
type Beeper struct {
	cpuClockHz   int
	frameTStates int
	earBit       bool // current EAR output state

	// Per-frame accumulation: we record level transitions.
	changes []beeperChange

	// Output sample buffer (circular), protected by mutex.
	mu       sync.Mutex
	buf      [beeperBufSize]float32
	bufWrite int
	bufRead  int
	bufCount int

	// Adaptive rate control: fractional sample accumulator.
	sampleAccum float64

	// Anti-click: track whether we're in a tone or silence.
	// Used to apply short fade in/out at transitions.
	lastSample float32

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
	// Pre-fill with 2 frames of silence for startup.
	prefill := beeperTargetFill
	for i := 0; i < prefill; i++ {
		b.buf[i] = 0
	}
	b.bufCount = prefill
	b.bufWrite = prefill
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

// EndFrame downsamples the frame's audio data and appends to the output buffer.
func (b *Beeper) EndFrame() {
	if !b.enabled {
		b.changes = b.changes[:0]
		return
	}

	// Exact fractional samples per frame.
	exactSamples := float64(BeeperSampleRate) * float64(b.frameTStates) / float64(b.cpuClockHz)
	b.sampleAccum += exactSamples
	samplesPerFrame := int(b.sampleAccum)
	b.sampleAccum -= float64(samplesPerFrame)

	// Adaptive adjustment: steer buffer toward target fill level.
	b.mu.Lock()
	fill := b.bufCount
	b.mu.Unlock()

	if len(b.changes) == 0 {
		// Silent frame: only produce samples if buffer is below target.
		// This prevents silence from accumulating and causing latency.
		if fill >= beeperTargetFill {
			return
		}
		// Top up to target, no more.
		need := beeperTargetFill - fill
		if need > samplesPerFrame {
			need = samplesPerFrame
		}
		b.mu.Lock()
		for i := 0; i < need; i++ {
			if b.bufCount < beeperBufSize {
				b.buf[b.bufWrite] = 0
				b.bufWrite = (b.bufWrite + 1) % beeperBufSize
				b.bufCount++
			}
		}
		b.mu.Unlock()
		b.changes = b.changes[:0]
		return
	}

	// Active frame (has EAR transitions).
	// Adaptive rate: ±1-2 samples based on fill level.
	if fill < beeperTargetFill-beeperNominalSPF {
		samplesPerFrame += 2
	} else if fill < beeperTargetFill {
		samplesPerFrame += 1
	} else if fill > beeperTargetFill+beeperNominalSPF {
		samplesPerFrame -= 2
	} else if fill > beeperTargetFill {
		samplesPerFrame -= 1
	}
	if samplesPerFrame < beeperNominalSPF-4 {
		samplesPerFrame = beeperNominalSPF - 4
	}
	if samplesPerFrame > beeperNominalSPF+4 {
		samplesPerFrame = beeperNominalSPF + 4
	}

	changeIdx := 0
	currentLevel := !b.changes[0].level

	// Generate samples into a temporary slice, then apply fade and write.
	tmp := make([]float32, samplesPerFrame)
	for i := 0; i < samplesPerFrame; i++ {
		tEnd := ((i + 1) * b.frameTStates) / samplesPerFrame
		for changeIdx < len(b.changes) && b.changes[changeIdx].tstate < tEnd {
			currentLevel = b.changes[changeIdx].level
			changeIdx++
		}
		if currentLevel {
			tmp[i] = 0.5
		} else {
			tmp[i] = -0.5
		}
	}

	// Anti-click: crossfade from the last sample value to the new waveform.
	// Without this, going from silence (0.0) to tone (±0.5) creates an
	// audible DC offset pop. A 32-sample (~0.7ms) linear ramp eliminates it.
	fadeLen := beeperFadeLen
	if fadeLen > samplesPerFrame {
		fadeLen = samplesPerFrame
	}
	startVal := b.lastSample
	for i := 0; i < fadeLen; i++ {
		t := float32(i+1) / float32(fadeLen) // 0→1 over fadeLen samples
		// Blend from startVal to the actual waveform value
		tmp[i] = startVal*(1-t) + tmp[i]*t
	}

	b.lastSample = tmp[samplesPerFrame-1]

	b.mu.Lock()
	for i := 0; i < samplesPerFrame; i++ {
		if b.bufCount < beeperBufSize {
			b.buf[b.bufWrite] = tmp[i]
			b.bufWrite = (b.bufWrite + 1) % beeperBufSize
			b.bufCount++
		}
	}
	b.mu.Unlock()

	b.changes = b.changes[:0]
}

// ReadSamples drains up to len(out) samples from the buffer.
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
