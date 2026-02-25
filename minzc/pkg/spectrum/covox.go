package spectrum

import "sync"

// Covox implements a simple 8-bit parallel DAC (Covox Speech Thing).
// Port $FB (mono): each OUT writes one unsigned 8-bit PCM sample.
// The samples are accumulated per frame and resampled to the audio output rate.
type Covox struct {
	cpuClockHz   int
	frameTStates int

	// Per-frame sample accumulation (like beeper, but 8-bit levels).
	changes []covoxChange

	// Ring buffer for output samples.
	mu       sync.Mutex
	buf      [8192]float32
	bufWrite int
	bufRead  int
	bufCount int

	lastLevel float32

	enabled bool
}

type covoxChange struct {
	tstate int
	level  byte // 0-255 unsigned PCM
}

// NewCovox creates a Covox DAC for the given video mode.
func NewCovox(mode *VideoMode) *Covox {
	c := &Covox{
		cpuClockHz:   mode.CPUClockHz,
		frameTStates: mode.TStatesPerFrame(),
		changes:      make([]covoxChange, 0, 256),
		enabled:      true,
	}
	// Pre-fill with 1 frame of silence.
	prefill := BeeperSampleRate / 50
	c.bufCount = prefill
	c.bufWrite = prefill
	return c
}

// WriteSample records a Covox output at the given T-state.
func (c *Covox) WriteSample(val byte, tstate int) {
	c.changes = append(c.changes, covoxChange{tstate: tstate, level: val})
}

// EndFrame downsamples the frame's Covox data to PCM output.
func (c *Covox) EndFrame() {
	if !c.enabled {
		c.changes = c.changes[:0]
		return
	}

	samplesPerFrame := BeeperSampleRate / 50 // 882

	if len(c.changes) == 0 {
		// Silent frame: produce samples at last level to keep buffer fed.
		c.mu.Lock()
		if c.bufCount < BeeperSampleRate/50 {
			need := BeeperSampleRate/50 - c.bufCount
			for i := 0; i < need && c.bufCount < len(c.buf); i++ {
				c.buf[c.bufWrite] = c.lastLevel
				c.bufWrite = (c.bufWrite + 1) % len(c.buf)
				c.bufCount++
			}
		}
		c.mu.Unlock()
		return
	}

	// Downsample: for each output sample, find the last Covox write before that point.
	changeIdx := 0
	currentLevel := c.lastLevel

	tmp := make([]float32, samplesPerFrame)
	for i := 0; i < samplesPerFrame; i++ {
		tEnd := ((i + 1) * c.frameTStates) / samplesPerFrame
		for changeIdx < len(c.changes) && c.changes[changeIdx].tstate < tEnd {
			// Convert unsigned 8-bit (0-255) to float (-1.0 to +1.0)
			currentLevel = float32(c.changes[changeIdx].level)/128.0 - 1.0
			changeIdx++
		}
		tmp[i] = currentLevel
	}
	c.lastLevel = currentLevel

	c.mu.Lock()
	for i := 0; i < samplesPerFrame; i++ {
		if c.bufCount < len(c.buf) {
			c.buf[c.bufWrite] = tmp[i]
			c.bufWrite = (c.bufWrite + 1) % len(c.buf)
			c.bufCount++
		}
	}
	c.mu.Unlock()

	c.changes = c.changes[:0]
}

// ReadSamples drains up to len(out) samples from the buffer.
func (c *Covox) ReadSamples(out []float32) int {
	c.mu.Lock()
	n := len(out)
	if n > c.bufCount {
		n = c.bufCount
	}
	for i := 0; i < n; i++ {
		out[i] = c.buf[c.bufRead]
		c.bufRead = (c.bufRead + 1) % len(c.buf)
	}
	c.bufCount -= n
	c.mu.Unlock()
	return n
}

// Available returns the number of samples available for reading.
func (c *Covox) Available() int {
	c.mu.Lock()
	n := c.bufCount
	c.mu.Unlock()
	return n
}

// SetEnabled enables or disables Covox output.
func (c *Covox) SetEnabled(enabled bool) {
	c.enabled = enabled
}
