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

// Beeper implements multi-level audio with per-T-state sampling accuracy.
//
// The ZX Spectrum speaker is driven by two bits through different resistors:
//   - Bit 4 (EAR): ~2/3 of speaker volume — used by BEEP
//   - Bit 3 (MIC): ~1/3 of speaker volume — used by SAVE
//
// This produces 4 distinct output levels, not just on/off.
// The ROM BEEP routine sets MIC=1 always and toggles EAR, so a boolean
// model (OR of both bits) sees no transitions and produces silence.
//
// Audio architecture (FUSE-style adaptive rate control):
//
//	Frame execution:  OUT ($FE) → SetLevel(level, tstate) records transitions
//	Frame end:        EndFrame() → downsamples transitions to PCM samples
//	Audio callback:   ReadSamples(buf) → drains ring buffer (async goroutine)
type Beeper struct {
	cpuClockHz   int
	frameTStates int
	level        float32 // current speaker level (0.0 to 1.0)

	// Level at the start of the current frame — used for sample generation
	// before the first transition. Saved at each EndFrame for continuity.
	frameStartLevel float32

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

	// Anti-click: last sample value for crossfade at frame boundaries.
	lastSample float32

	enabled bool
}

type beeperChange struct {
	tstate int
	level  float32
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

// SetLevel updates the speaker level at the given T-state within the frame.
// level should be in the range 0.0 to 1.0.
func (b *Beeper) SetLevel(level float32, tstate int) {
	if level == b.level {
		return
	}
	b.level = level
	b.changes = append(b.changes, beeperChange{tstate: tstate, level: level})
}

// SetEar is a convenience wrapper for boolean on/off (used by tape signal).
func (b *Beeper) SetEar(ear bool, tstate int) {
	if ear {
		b.SetLevel(1.0, tstate)
	} else {
		b.SetLevel(0.0, tstate)
	}
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
		// No transitions this frame.
		// Always produce a full frame of samples to prevent the audio callback
		// from running dry (which injects silence → periodic click).
		// Only skip if the buffer already has 3+ frames of headroom.
		if fill > beeperNominalSPF*3 {
			if b.lastSample != 0 {
				b.lastSample = 0 // reset for next active frame
			}
			return
		}
		// Produce a full frame: fade from lastSample to 0.0, then hold at 0.0
		fadeLen := beeperFadeLen
		if fadeLen > samplesPerFrame {
			fadeLen = samplesPerFrame
		}
		b.mu.Lock()
		for i := 0; i < samplesPerFrame; i++ {
			if b.bufCount >= beeperBufSize {
				break
			}
			var sample float32
			if i < fadeLen && b.lastSample != 0 {
				t := float32(i+1) / float32(fadeLen)
				sample = b.lastSample * (1 - t)
			}
			b.buf[b.bufWrite] = sample
			b.bufWrite = (b.bufWrite + 1) % beeperBufSize
			b.bufCount++
		}
		b.mu.Unlock()
		b.lastSample = 0
		b.changes = b.changes[:0]
		return
	}

	// Active frame (has transitions).
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

	// Use the saved frame-start level for samples before the first transition.
	changeIdx := 0
	currentLevel := b.frameStartLevel

	// Generate samples into a temporary slice, then apply fade and write.
	tmp := make([]float32, samplesPerFrame)
	for i := 0; i < samplesPerFrame; i++ {
		tEnd := ((i + 1) * b.frameTStates) / samplesPerFrame
		for changeIdx < len(b.changes) && b.changes[changeIdx].tstate < tEnd {
			currentLevel = b.changes[changeIdx].level
			changeIdx++
		}
		tmp[i] = currentLevel - 0.5 // center around 0: 0.0→-0.5, 1.0→+0.5
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

	// Save end-of-frame level for next frame's starting point.
	b.frameStartLevel = b.level
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
