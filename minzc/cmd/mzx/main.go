// Command mzx is a T-state accurate ZX Spectrum emulator for the MinZ toolchain.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
)

// Version info — injected at build time via ldflags.
var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
	buildNum  = "0"
)

//go:embed roms/48.rom
var embedded48KROM []byte

const (
	audioSampleRate = spectrum.BeeperSampleRate
)

// Game implements ebiten.Game.
type Game struct {
	machine  *spectrum.Machine
	screen   *ebiten.Image
	scale    int
	noAudio  bool

	// Audio (direct oto for low latency)
	otoCtx    *oto.Context
	otoPlayer *oto.Player

	// Key mapping: ebiten key → []SpecKey (some keys map to Shift+key)
	keyMap map[ebiten.Key][]spectrum.SpecKey

	// Keystroke injection queue (for --type flag)
	keystrokeQueue *formats.KeystrokeQueue

	// Turbo mode: run many frames per tick (F3 toggle, F4 hold)
	turbo         bool
	turboFrames   int // frames per Update() in turbo mode

	// Startup: restore stderr after first few frames (CAMetalLayer suppression)
	startupFrames int
}

func newGame(machine *spectrum.Machine, scale int, noAudio bool) *Game {
	g := &Game{
		machine:     machine,
		screen:      ebiten.NewImage(machine.ScreenWidth(), machine.ScreenHeight()),
		scale:       scale,
		noAudio:     noAudio,
		keyMap:      buildKeyMap(),
		turboFrames: 20, // 20x speed (1 second = 20 frames at 50Hz)
	}

	if !noAudio {
		// Use oto directly (bypassing Ebitengine audio) for minimal latency.
		// BufferSize controls the CoreAudio/ALSA buffer — the dominant source
		// of audio latency. 2 frames (7056 bytes = ~40ms) is the minimum
		// that avoids underruns on most systems.
		otoOpts := &oto.NewContextOptions{
			SampleRate:   audioSampleRate,
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
			BufferSize:   40 * time.Millisecond,
		}
		var readyCh chan struct{}
		var err error
		g.otoCtx, readyCh, err = oto.NewContext(otoOpts)
		if err != nil {
			log.Printf("Warning: audio init failed: %v", err)
			g.noAudio = true
		} else {
			<-readyCh // wait for audio hardware to be ready
			mixer := &audioMixer{beeper: machine.Beeper, ay: machine.AY}
			g.otoPlayer = g.otoCtx.NewPlayer(mixer)
			g.otoPlayer.SetBufferSize(audioSampleRate / 50 * 4 * 2) // 40ms player buffer
			g.otoPlayer.Play()
		}
	}

	return g
}

func (g *Game) Update() error {
	// Restore stderr after a few startup frames (suppressed for CAMetalLayer warnings)
	g.startupFrames++
	if g.startupFrames == 3 {
		restoreStderr()
	}

	// Handle special keys (Cmd+Q / Alt+F4 handled natively by Ebitengine)
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		g.machine.Reset()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.machine.SetPaused(!g.machine.IsPaused())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		path := fmt.Sprintf("mzx_screenshot_%06d.png", g.machine.FrameCount())
		if err := saveScreenshotEx(g.machine, path, borderStandard); err != nil {
			log.Printf("Screenshot error: %v", err)
		} else {
			log.Printf("Screenshot saved: %s", path)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		path := fmt.Sprintf("mzx_snapshot_%06d.sna", g.machine.FrameCount())
		if err := formats.SaveSNA(path, g.machine); err != nil {
			log.Printf("Snapshot error: %v", err)
		} else {
			log.Printf("Snapshot saved: %s", path)
		}
	}

	// F3: toggle turbo (max speed), F4: hold for turbo
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.turbo = !g.turbo
		if g.turbo {
			log.Printf("Turbo ON (%dx)", g.turboFrames)
		} else {
			log.Printf("Turbo OFF")
		}
	}
	holdTurbo := ebiten.IsKeyPressed(ebiten.KeyF4)

	// Sync keyboard state
	g.syncKeyboard()

	// Inject queued keystrokes (--type flag)
	if g.keystrokeQueue != nil && !g.keystrokeQueue.Done() {
		g.keystrokeQueue.Update()
	}

	// Run frame(s) — in turbo mode, run many frames per tick (skip ULA rendering)
	if g.turbo || holdTurbo {
		for i := 0; i < g.turboFrames; i++ {
			g.machine.RunFrameFast()
		}
	} else {
		g.machine.RunFrame()
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	fb := g.machine.Framebuffer()
	w := g.machine.ScreenWidth()
	h := g.machine.ScreenHeight()

	// Update our offscreen image from the RGBA framebuffer
	g.screen.WritePixels(fb[:w*h*4])

	// Draw scaled to window
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(float64(g.scale), float64(g.scale))
	screen.DrawImage(g.screen, opts)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.machine.ScreenWidth() * g.scale, g.machine.ScreenHeight() * g.scale
}

func (g *Game) syncKeyboard() {
	g.machine.Keyboard.Reset()

	// Detect PC modifier state
	pcShift := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	pcCtrl := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)

	// Track whether PC Shift has been "consumed" by a shifted-punctuation combo.
	// If consumed, we don't pass it through as Caps Shift.
	shiftConsumed := false

	// --- Shifted punctuation (US keyboard layout) ---
	// When PC Shift is held with a key that produces a different character,
	// translate to the correct Spectrum Symbol Shift combo.
	if pcShift {
		// Shift + number keys → symbols on US keyboard
		shiftNumMap := []struct {
			key  ebiten.Key
			spec []spectrum.SpecKey // Spectrum equivalent
		}{
			{ebiten.KeyDigit1, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key1}},  // ! → SS+1
			{ebiten.KeyDigit2, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key2}},  // @ → SS+2
			{ebiten.KeyDigit3, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key3}},  // # → SS+3
			{ebiten.KeyDigit4, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key4}},  // $ → SS+4
			{ebiten.KeyDigit5, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key5}},  // % → SS+5
			{ebiten.KeyDigit6, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key6}},  // ^ → SS+6 (↑)
			{ebiten.KeyDigit7, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key6}},  // & → SS+6 (closest)
			{ebiten.KeyDigit8, []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyB}},  // * → SS+B
			{ebiten.KeyDigit9, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key8}},  // ( → SS+8
			{ebiten.KeyDigit0, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key9}},  // ) → SS+9
			{ebiten.KeyMinus, []spectrum.SpecKey{spectrum.KeySym, spectrum.Key0}},   // _ → SS+0
			{ebiten.KeyEqual, []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyK}},   // + → SS+K
			{ebiten.KeySemicolon, []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyZ}}, // : → SS+Z
			{ebiten.KeyQuote, []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyP}},   // " → SS+P
			{ebiten.KeyComma, []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyR}},   // < → SS+R
			{ebiten.KeyPeriod, []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyT}},  // > → SS+T
			{ebiten.KeySlash, []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyC}},   // ? → SS+C
		}
		for _, entry := range shiftNumMap {
			if ebiten.IsKeyPressed(entry.key) {
				for _, sk := range entry.spec {
					g.machine.Keyboard.KeyPress(sk.Row, sk.Bit)
				}
				shiftConsumed = true
			}
		}
	}

	// --- Unshifted key mappings ---
	// Only apply when the key ISN'T already handled by a shift combo above.
	for key, specKeys := range g.keyMap {
		if !ebiten.IsKeyPressed(key) {
			continue
		}
		// Skip keys that were handled by shift combos
		if pcShift && g.isShiftComboKey(key) {
			continue
		}
		for _, sk := range specKeys {
			g.machine.Keyboard.KeyPress(sk.Row, sk.Bit)
		}
	}

	// --- Modifiers ---
	// Left Shift → Caps Shift (only if not consumed by a shifted-punctuation combo)
	if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) && !shiftConsumed {
		g.machine.Keyboard.KeyPress(spectrum.KeyShift.Row, spectrum.KeyShift.Bit)
	}
	// Right Shift → Symbol Shift (standalone, for direct Spectrum SS use)
	// Only if not consumed by a shifted-punctuation combo
	if ebiten.IsKeyPressed(ebiten.KeyShiftRight) && !shiftConsumed {
		g.machine.Keyboard.KeyPress(spectrum.KeySym.Row, spectrum.KeySym.Bit)
	}
	// Ctrl → Symbol Shift (always, for explicit SS combos)
	if pcCtrl {
		g.machine.Keyboard.KeyPress(spectrum.KeySym.Row, spectrum.KeySym.Bit)
	}
}

// isShiftComboKey returns true if this key produces a different character
// when Shift is held on a US keyboard layout.
func (g *Game) isShiftComboKey(key ebiten.Key) bool {
	switch key {
	case ebiten.KeyDigit0, ebiten.KeyDigit1, ebiten.KeyDigit2,
		ebiten.KeyDigit3, ebiten.KeyDigit4, ebiten.KeyDigit5,
		ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8,
		ebiten.KeyDigit9,
		ebiten.KeyMinus, ebiten.KeyEqual,
		ebiten.KeySemicolon, ebiten.KeyQuote,
		ebiten.KeyComma, ebiten.KeyPeriod, ebiten.KeySlash:
		return true
	}
	return false
}

func buildKeyMap() map[ebiten.Key][]spectrum.SpecKey {
	m := map[ebiten.Key][]spectrum.SpecKey{
		// Letters (unshifted: just the letter key)
		ebiten.KeyA: {spectrum.KeyA},
		ebiten.KeyB: {spectrum.KeyB},
		ebiten.KeyC: {spectrum.KeyC},
		ebiten.KeyD: {spectrum.KeyD},
		ebiten.KeyE: {spectrum.KeyE},
		ebiten.KeyF: {spectrum.KeyF},
		ebiten.KeyG: {spectrum.KeyG},
		ebiten.KeyH: {spectrum.KeyH},
		ebiten.KeyI: {spectrum.KeyI},
		ebiten.KeyJ: {spectrum.KeyJ},
		ebiten.KeyK: {spectrum.KeyK},
		ebiten.KeyL: {spectrum.KeyL},
		ebiten.KeyM: {spectrum.KeyM},
		ebiten.KeyN: {spectrum.KeyN},
		ebiten.KeyO: {spectrum.KeyO},
		ebiten.KeyP: {spectrum.KeyP},
		ebiten.KeyQ: {spectrum.KeyQ},
		ebiten.KeyR: {spectrum.KeyR},
		ebiten.KeyS: {spectrum.KeyS},
		ebiten.KeyT: {spectrum.KeyT},
		ebiten.KeyU: {spectrum.KeyU},
		ebiten.KeyV: {spectrum.KeyV},
		ebiten.KeyW: {spectrum.KeyW},
		ebiten.KeyX: {spectrum.KeyX},
		ebiten.KeyY: {spectrum.KeyY},
		ebiten.KeyZ: {spectrum.KeyZ},

		// Numbers (unshifted)
		ebiten.KeyDigit0: {spectrum.Key0},
		ebiten.KeyDigit1: {spectrum.Key1},
		ebiten.KeyDigit2: {spectrum.Key2},
		ebiten.KeyDigit3: {spectrum.Key3},
		ebiten.KeyDigit4: {spectrum.Key4},
		ebiten.KeyDigit5: {spectrum.Key5},
		ebiten.KeyDigit6: {spectrum.Key6},
		ebiten.KeyDigit7: {spectrum.Key7},
		ebiten.KeyDigit8: {spectrum.Key8},
		ebiten.KeyDigit9: {spectrum.Key9},

		// Special keys (no shift awareness needed — handled directly)
		ebiten.KeyEnter:   {spectrum.KeyEnter},
		ebiten.KeySpace:   {spectrum.KeySpace},
		ebiten.KeyEscape:  {spectrum.KeyShift, spectrum.Key1},          // EDIT (CS+1)
		ebiten.KeyTab:     {spectrum.KeyShift, spectrum.KeySym},        // Extended mode (CS+SS)
		ebiten.KeyCapsLock: {spectrum.KeyShift, spectrum.Key2},         // Caps Lock (CS+2)

		// Arrow keys → Caps Shift + 5/6/7/8
		ebiten.KeyArrowLeft:  {spectrum.KeyShift, spectrum.Key5},
		ebiten.KeyArrowDown:  {spectrum.KeyShift, spectrum.Key6},
		ebiten.KeyArrowUp:    {spectrum.KeyShift, spectrum.Key7},
		ebiten.KeyArrowRight: {spectrum.KeyShift, spectrum.Key8},

		// Backspace → Caps Shift + 0 (DELETE)
		ebiten.KeyBackspace: {spectrum.KeyShift, spectrum.Key0},

		// Unshifted punctuation → Symbol Shift combos
		ebiten.KeyComma:     {spectrum.KeySym, spectrum.KeyN},  // , → SS+N
		ebiten.KeyPeriod:    {spectrum.KeySym, spectrum.KeyM},  // . → SS+M
		ebiten.KeySlash:     {spectrum.KeySym, spectrum.KeyV},  // / → SS+V
		ebiten.KeySemicolon: {spectrum.KeySym, spectrum.KeyO},  // ; → SS+O
		ebiten.KeyQuote:     {spectrum.KeySym, spectrum.Key7},  // ' → SS+7
		ebiten.KeyMinus:     {spectrum.KeySym, spectrum.KeyJ},  // - → SS+J
		ebiten.KeyEqual:     {spectrum.KeySym, spectrum.KeyL},  // = → SS+L
	}
	return m
}

// audioMixer implements io.Reader for ebiten audio streaming.
// Mixes beeper (mono) and AY chip (stereo) output.
// AY samples are frame-synchronized: generated in Machine.RunFrame()
// via AY.EndFrame(), then drained here by the audio callback.
type audioMixer struct {
	beeper    *spectrum.Beeper
	ay        *spectrum.AYChip
	beeperBuf []float32
	ayLeft    []float64
	ayRight   []float64
}

func (p *audioMixer) Read(data []byte) (int, error) {
	// Ebiten audio expects signed 16-bit stereo PCM at audioSampleRate.
	// We return only as many samples as available — io.Reader allows partial reads.
	// This prevents silence gaps that cause fragmented audio.
	maxSamples := len(data) / 4 // 4 bytes per stereo sample

	// Determine how many samples are available from beeper
	beeperAvail := p.beeper.Available()
	ayAvail := 0
	hasAY := p.ay != nil
	if hasAY {
		ayAvail = p.ay.Available()
	}

	// Use the maximum of beeper/AY availability, capped by request size.
	// If both are empty, return a small amount of silence (never block).
	samples := beeperAvail
	if hasAY && ayAvail > samples {
		samples = ayAvail
	}
	if samples > maxSamples {
		samples = maxSamples
	}
	if samples == 0 {
		// Nothing available yet — return a tiny bit of silence to avoid spinning.
		// 64 samples = ~1.5ms, small enough to be imperceptible.
		samples = 64
		if samples > maxSamples {
			samples = maxSamples
		}
		for i := 0; i < samples*4; i++ {
			data[i] = 0
		}
		return samples * 4, nil
	}

	// Read beeper samples
	if cap(p.beeperBuf) < samples {
		p.beeperBuf = make([]float32, samples)
	}
	p.beeperBuf = p.beeperBuf[:samples]
	n := p.beeper.ReadSamples(p.beeperBuf)
	for i := n; i < samples; i++ {
		p.beeperBuf[i] = 0 // only pads if beeper has fewer than AY
	}

	// Read AY samples
	if hasAY {
		if cap(p.ayLeft) < samples {
			p.ayLeft = make([]float64, samples)
			p.ayRight = make([]float64, samples)
		}
		p.ayLeft = p.ayLeft[:samples]
		p.ayRight = p.ayRight[:samples]
		ayN := p.ay.ReadFrameSamples(p.ayLeft, p.ayRight)
		for i := ayN; i < samples; i++ {
			p.ayLeft[i] = 0
			p.ayRight[i] = 0
		}
	}

	// Mix and convert to signed 16-bit stereo
	for i := 0; i < samples; i++ {
		beeperSample := float64(p.beeperBuf[i])

		var left, right float64
		if hasAY {
			left = beeperSample*0.6 + p.ayLeft[i]*0.8
			right = beeperSample*0.6 + p.ayRight[i]*0.8
		} else {
			left = beeperSample
			right = beeperSample
		}

		// Clamp
		if left > 1.0 {
			left = 1.0
		} else if left < -1.0 {
			left = -1.0
		}
		if right > 1.0 {
			right = 1.0
		} else if right < -1.0 {
			right = -1.0
		}

		sL := int16(left * 32767)
		sR := int16(right * 32767)
		j := i * 4
		data[j+0] = byte(sL)
		data[j+1] = byte(sL >> 8)
		data[j+2] = byte(sR)
		data[j+3] = byte(sR >> 8)
	}

	return samples * 4, nil
}

// screensEqual compares two RGBA framebuffers for pixel-level equality.
func screensEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// borderMode controls how much border area is included in screenshots.
type borderMode int

const (
	borderStandard borderMode = iota // 320x240: 32px side, 24px top/bottom (TV-visible, matches zxs)
	borderNone                       // 256x192: screen only, no border
	borderFull                       // Full ULA output (352x296 for 48K, 352x312 for Pentagon)
)

// Standard TV-visible border sizes (matches zxs default)
const (
	stdBorderSide   = 32 // pixels left and right
	stdBorderTop    = 24 // pixels above screen
	stdBorderBottom = 24 // pixels below screen
)

// saveScreenshotEx saves the framebuffer as PNG with the specified border mode.
func saveScreenshotEx(m *spectrum.Machine, path string, mode borderMode) error {
	fb := m.Framebuffer()
	fullW := m.ScreenWidth()
	fullH := m.ScreenHeight()

	var img *image.RGBA
	switch mode {
	case borderNone:
		// 256x192: screen area only
		img = image.NewRGBA(image.Rect(0, 0, 256, 192))
		for y := 0; y < 192; y++ {
			for x := 0; x < 256; x++ {
				off := ((m.Mode.BorderTop+y)*fullW + m.Mode.BorderLeft + x) * 4
				img.SetRGBA(x, y, color.RGBA{
					R: fb[off+0], G: fb[off+1], B: fb[off+2], A: fb[off+3],
				})
			}
		}

	case borderStandard:
		// 320x240: standard TV-visible area (32px side, 24px top/bottom)
		outW := 256 + stdBorderSide*2 // 320
		outH := 192 + stdBorderTop + stdBorderBottom // 240
		cropX := m.Mode.BorderLeft - stdBorderSide
		cropY := m.Mode.BorderTop - stdBorderTop
		if cropX < 0 {
			cropX = 0
		}
		if cropY < 0 {
			cropY = 0
		}
		img = image.NewRGBA(image.Rect(0, 0, outW, outH))
		for y := 0; y < outH; y++ {
			for x := 0; x < outW; x++ {
				srcX := cropX + x
				srcY := cropY + y
				if srcX < fullW && srcY < fullH {
					off := (srcY*fullW + srcX) * 4
					img.SetRGBA(x, y, color.RGBA{
						R: fb[off+0], G: fb[off+1], B: fb[off+2], A: fb[off+3],
					})
				}
			}
		}

	case borderFull:
		// Full ULA output (352x296 for 48K, 352x312 for Pentagon)
		img = image.NewRGBA(image.Rect(0, 0, fullW, fullH))
		for y := 0; y < fullH; y++ {
			for x := 0; x < fullW; x++ {
				off := (y*fullW + x) * 4
				img.SetRGBA(x, y, color.RGBA{
					R: fb[off+0], G: fb[off+1], B: fb[off+2], A: fb[off+3],
				})
			}
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating screenshot file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encoding PNG: %w", err)
	}
	return nil
}

// ---- Frame spec parser (matches zxs --frame-spec syntax) ----

// frameSpecType identifies the type of frame specification.
type frameSpecType int

const (
	specNone        frameSpecType = iota
	specSingleFrame               // "100"
	specFrameRange                // "100..200"
	specPCTrigger                 // "PC=4000"
	specPCRange                   // "PC=4000..PC=4000+50" or "PC=4000+50"
	specTTrigger                  // "T=100000"
	specTRange                    // "T=100000..T=100000+200"
	specDIHalt                    // "DI:HALT"
)

type frameSpec struct {
	specType    frameSpecType
	startFrame  int    // for specSingleFrame, specFrameRange
	endFrame    int    // for specFrameRange
	startPC     uint16 // for specPCTrigger, specPCRange
	endPC       uint16 // for specPCRange (if different end)
	startT      int    // for specTTrigger, specTRange
	endT        int    // for specTRange
	rangeOffset int    // +N frames after trigger
}

func (s *frameSpec) isEmpty() bool     { return s.specType == specNone }
func (s *frameSpec) isSingleFrame() bool { return s.specType == specSingleFrame }
func (s *frameSpec) isFrameRange() bool  { return s.specType == specFrameRange }

func (s *frameSpec) isSingleTrigger() bool {
	return s.specType == specPCTrigger || s.specType == specTTrigger || s.specType == specDIHalt
}

func (s *frameSpec) hasEnd() bool {
	return s.specType == specFrameRange || s.specType == specPCRange ||
		s.specType == specTRange || s.rangeOffset > 0
}

func (s *frameSpec) matchesStart(m *spectrum.Machine, frame int) bool {
	switch s.specType {
	case specSingleFrame:
		return frame >= s.startFrame
	case specFrameRange:
		return frame >= s.startFrame
	case specPCTrigger, specPCRange:
		return m.CPU.PC() == s.startPC
	case specTTrigger, specTRange:
		return m.CPU.Tstates() >= s.startT
	case specDIHalt:
		return m.CPU.Halted() && !m.CPU.IFF1()
	}
	return false
}

func (s *frameSpec) matchesEnd(m *spectrum.Machine, frame int) bool {
	switch s.specType {
	case specFrameRange:
		return frame >= s.endFrame
	case specPCRange:
		if s.endPC != s.startPC {
			return m.CPU.PC() == s.endPC
		}
		return false // use rangeOffset
	case specTRange:
		return m.CPU.Tstates() >= s.endT
	}
	return false
}

// parseFrameSpec parses zxs-compatible frame spec syntax:
//   "100"                   → single frame
//   "100..200"              → frame range
//   "PC=4000"               → trigger at PC
//   "PC=4000+50"            → 50 frames after PC trigger
//   "PC=4000..PC=5000"      → range between two PC values
//   "T=100000"              → trigger at T-state
//   "T=100000+200"          → 200 frames after T-state trigger
//   "DI:HALT"               → trigger on dead CPU (DI + HALT)
func parseFrameSpec(s string) frameSpec {
	s = strings.TrimSpace(s)
	if s == "" {
		return frameSpec{}
	}

	// DI:HALT
	if strings.ToUpper(s) == "DI:HALT" {
		return frameSpec{specType: specDIHalt}
	}

	// PC=ADDR or PC=ADDR+N or PC=ADDR..PC=ADDR
	if strings.HasPrefix(strings.ToUpper(s), "PC=") {
		return parsePCSpec(s[3:])
	}

	// T=VALUE or T=VALUE+N
	if strings.HasPrefix(strings.ToUpper(s), "T=") {
		return parseTSpec(s[2:])
	}

	// Frame number or range: "100" or "100..200"
	if strings.Contains(s, "..") {
		parts := strings.SplitN(s, "..", 2)
		var start, end int
		fmt.Sscanf(parts[0], "%d", &start)
		fmt.Sscanf(parts[1], "%d", &end)
		return frameSpec{specType: specFrameRange, startFrame: start, endFrame: end}
	}

	var frame int
	fmt.Sscanf(s, "%d", &frame)
	return frameSpec{specType: specSingleFrame, startFrame: frame}
}

func parsePCSpec(s string) frameSpec {
	// PC=ADDR..PC=ADDR
	upper := strings.ToUpper(s)
	if idx := strings.Index(upper, "..PC="); idx >= 0 {
		startStr := s[:idx]
		endStr := s[idx+5:]
		return frameSpec{
			specType: specPCRange,
			startPC:  parseHexAddr(startStr),
			endPC:    parseHexAddr(endStr),
		}
	}
	// PC=ADDR+N
	if idx := strings.Index(s, "+"); idx >= 0 {
		addrStr := s[:idx]
		var offset int
		fmt.Sscanf(s[idx+1:], "%d", &offset)
		return frameSpec{
			specType:    specPCRange,
			startPC:     parseHexAddr(addrStr),
			rangeOffset: offset,
		}
	}
	// PC=ADDR (single trigger)
	return frameSpec{specType: specPCTrigger, startPC: parseHexAddr(s)}
}

func parseTSpec(s string) frameSpec {
	// T=VAL+N
	if idx := strings.Index(s, "+"); idx >= 0 {
		var val, offset int
		fmt.Sscanf(s[:idx], "%d", &val)
		fmt.Sscanf(s[idx+1:], "%d", &offset)
		return frameSpec{
			specType:    specTRange,
			startT:      val,
			rangeOffset: offset,
		}
	}
	// T=VAL..T=VAL
	upper := strings.ToUpper(s)
	if idx := strings.Index(upper, "..T="); idx >= 0 {
		var start, end int
		fmt.Sscanf(s[:idx], "%d", &start)
		fmt.Sscanf(s[idx+4:], "%d", &end)
		return frameSpec{specType: specTRange, startT: start, endT: end}
	}
	// T=VAL
	var val int
	fmt.Sscanf(s, "%d", &val)
	return frameSpec{specType: specTTrigger, startT: val}
}

func parseHexAddr(s string) uint16 {
	s = strings.TrimSpace(s)
	var val int
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		fmt.Sscanf(s[2:], "%x", &val)
	} else if strings.HasPrefix(s, "$") {
		fmt.Sscanf(s[1:], "%x", &val)
	} else {
		fmt.Sscanf(s, "%x", &val)
	}
	return uint16(val)
}

// ---- stringSlice: flag.Value that accumulates repeated flags and splits on commas ----

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }

func (s *stringSlice) Set(val string) error {
	for _, part := range strings.Split(val, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*s = append(*s, part)
		}
	}
	return nil
}

// ---- --load: load raw binary files into memory ----

type loadSpec struct {
	file string
	addr uint16
	page int // -1 = no page (use current mapping)
}

func parseLoadSpec(s string) (loadSpec, error) {
	// Format: FILE@ADDR or FILE@ADDR:PAGE
	atIdx := strings.LastIndex(s, "@")
	if atIdx < 0 {
		return loadSpec{}, fmt.Errorf("missing @ADDR in load spec: %q (expected FILE@ADDR)", s)
	}
	file := s[:atIdx]
	rest := s[atIdx+1:] // ADDR or ADDR:PAGE

	if file == "" {
		return loadSpec{}, fmt.Errorf("empty filename in load spec: %q", s)
	}

	spec := loadSpec{file: file, page: -1}

	if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
		addrStr := rest[:colonIdx]
		pageStr := rest[colonIdx+1:]
		spec.addr = parseHexAddr(addrStr)
		var page int
		if _, err := fmt.Sscanf(pageStr, "%d", &page); err != nil {
			return loadSpec{}, fmt.Errorf("invalid page number %q in load spec", pageStr)
		}
		if page < 0 || page > 7 {
			return loadSpec{}, fmt.Errorf("page must be 0-7, got %d", page)
		}
		spec.page = page
	} else {
		spec.addr = parseHexAddr(rest)
	}

	return spec, nil
}

func applyLoads(m *spectrum.Machine, specs []loadSpec) error {
	for _, spec := range specs {
		data, err := os.ReadFile(spec.file)
		if err != nil {
			return fmt.Errorf("reading %s: %w", spec.file, err)
		}
		if spec.page >= 0 {
			// Write to specific 128K RAM page
			offset := spec.addr & 0x3FFF
			for i, b := range data {
				m.Memory.WriteRAMDirect(spec.page, offset+uint16(i), b)
			}
			fmt.Printf("Loaded %d bytes from %s to $%04X page %d\n", len(data), spec.file, spec.addr, spec.page)
		} else {
			// Write to currently mapped memory
			for i, b := range data {
				m.Memory.WriteByteInternal(spec.addr+uint16(i), b)
			}
			fmt.Printf("Loaded %d bytes from %s to $%04X\n", len(data), spec.file, spec.addr)
		}
	}
	return nil
}

// ---- --set: set CPU registers and interrupt state ----

type setAssignment struct {
	name  string // register name or command (DI, EI)
	value uint16 // parsed hex value (unused for DI/EI)
}

func parseSetSpec(s string) ([]setAssignment, error) {
	var assignments []setAssignment
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		upper := strings.ToUpper(part)
		// Commands (no value)
		if upper == "DI" || upper == "EI" {
			assignments = append(assignments, setAssignment{name: upper})
			continue
		}
		// KEY=VALUE
		eqIdx := strings.Index(part, "=")
		if eqIdx < 0 {
			return nil, fmt.Errorf("invalid assignment %q (expected NAME=VALUE, DI, or EI)", part)
		}
		name := strings.ToUpper(strings.TrimSpace(part[:eqIdx]))
		valStr := strings.TrimSpace(part[eqIdx+1:])
		val := parseHexAddr(valStr)
		assignments = append(assignments, setAssignment{name: name, value: val})
	}
	return assignments, nil
}

func applySetAssignments(m *spectrum.Machine, assignments []setAssignment) error {
	for _, a := range assignments {
		switch a.name {
		case "PC":
			m.CPU.SetPC(a.value)
		case "SP":
			m.CPU.SetSP(a.value)
		case "AF":
			m.CPU.SetAF(a.value)
		case "BC":
			m.CPU.SetBC(a.value)
		case "DE":
			m.CPU.SetDE(a.value)
		case "HL":
			m.CPU.SetHL(a.value)
		case "IX":
			m.CPU.SetIX(a.value)
		case "IY":
			m.CPU.SetIY(a.value)
		case "AF'":
			m.CPU.SetAF_(a.value)
		case "BC'":
			m.CPU.SetBC_(a.value)
		case "DE'":
			m.CPU.SetDE_(a.value)
		case "HL'":
			m.CPU.SetHL_(a.value)
		case "A":
			m.CPU.SetAF((a.value << 8) | (m.CPU.AF() & 0xFF))
		case "I":
			m.CPU.SetI(byte(a.value))
		case "R":
			m.CPU.SetR(byte(a.value))
		case "IM":
			if a.value > 2 {
				return fmt.Errorf("IM must be 0, 1, or 2, got %d", a.value)
			}
			m.CPU.SetIM(byte(a.value))
		case "DI":
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
		case "EI":
			m.CPU.SetIFF1(true)
			m.CPU.SetIFF2(true)
		default:
			return fmt.Errorf("unknown register %q", a.name)
		}
	}
	return nil
}

// ---- Unified --frames parser ----

// captureRange is an inclusive [start, end] frame range.
type captureRange struct {
	start, end int
}

// captureSpec is the parsed result of --frames.
type captureSpec struct {
	isCount bool          // plain number: run N frames
	count   int           // frame count (when isCount)
	ranges  []captureRange // sorted capture ranges
	legacy  *frameSpec    // PC=, T=, DI:HALT triggers
}

func (cs *captureSpec) containsFrame(f int) bool {
	for _, r := range cs.ranges {
		if f >= r.start && f <= r.end {
			return true
		}
	}
	return false
}

func (cs *captureSpec) firstStart() int {
	if len(cs.ranges) == 0 {
		return 0
	}
	return cs.ranges[0].start
}

func (cs *captureSpec) lastEnd() int {
	if len(cs.ranges) == 0 {
		return 0
	}
	return cs.ranges[len(cs.ranges)-1].end
}

// parseCaptureSpec parses the unified --frames value.
//
//	"50"                  → count (run 50 frames)
//	"9839..10295"         → single range
//	"100,9839..10295,500" → multi-range
//	"100-200"             → dash range
//	"PC=4000"             → legacy trigger
//	"DI:HALT"             → legacy trigger
func parseCaptureSpec(s string) captureSpec {
	s = strings.TrimSpace(s)
	if s == "" {
		return captureSpec{isCount: true, count: 50}
	}

	// Legacy triggers: PC=, T=, DI:HALT
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "PC=") || strings.HasPrefix(upper, "T=") || upper == "DI:HALT" {
		spec := parseFrameSpec(s)
		return captureSpec{legacy: &spec}
	}

	// Detect range/multi syntax
	hasRangeSyntax := strings.Contains(s, "..") || strings.ContainsAny(s, ",;")
	if !hasRangeSyntax {
		// Check for N-M (dash between digit groups)
		if idx := strings.Index(s, "-"); idx > 0 && idx < len(s)-1 {
			_, err1 := strconv.Atoi(strings.TrimSpace(s[:idx]))
			_, err2 := strconv.Atoi(strings.TrimSpace(s[idx+1:]))
			if err1 == nil && err2 == nil {
				hasRangeSyntax = true
			}
		}
	}

	// Plain count
	if !hasRangeSyntax {
		if n, err := strconv.Atoi(s); err == nil {
			return captureSpec{isCount: true, count: n}
		}
		return captureSpec{isCount: true, count: 50}
	}

	// Parse multi-range (split by , and ;)
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';'
	})
	var ranges []captureRange
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if r, ok := parseCaptureRange(part); ok {
			ranges = append(ranges, r)
		}
	}

	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].start < ranges[j].start
	})

	return captureSpec{ranges: ranges}
}

// parseCaptureRange parses a single range element: "N..M", "N-M", or "N".
func parseCaptureRange(s string) (captureRange, bool) {
	// N..M (dots)
	if idx := strings.Index(s, ".."); idx >= 0 {
		start, err1 := strconv.Atoi(strings.TrimSpace(s[:idx]))
		end, err2 := strconv.Atoi(strings.TrimSpace(s[idx+2:]))
		if err1 == nil && err2 == nil {
			if start > end {
				start, end = end, start
			}
			return captureRange{start, end}, true
		}
		return captureRange{}, false
	}
	// N-M (dash)
	if idx := strings.Index(s, "-"); idx > 0 {
		start, err1 := strconv.Atoi(strings.TrimSpace(s[:idx]))
		end, err2 := strconv.Atoi(strings.TrimSpace(s[idx+1:]))
		if err1 == nil && err2 == nil {
			if start > end {
				start, end = end, start
			}
			return captureRange{start, end}, true
		}
	}
	// Single number (point)
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return captureRange{n, n}, true
	}
	return captureRange{}, false
}

func main() {
	modelFlag := flag.String("model", "48k", "Machine model: 48k, 128k, pentagon")
	romFlag := flag.String("rom", "", "Path to ROM file")
	rom1Flag := flag.String("rom1", "", "Path to second ROM (128K models)")
	snapshotFlag := flag.String("snapshot", "", "Path to .sna snapshot file")
	tapFlag := flag.String("tap", "", "Path to .tap tape file")
	trdFlag := flag.String("trd", "", "Path to .trd disk image")
	sclFlag := flag.String("scl", "", "Path to .scl disk image (converted to .trd)")
	trdLoad := flag.String("trd-load", "", "File to load from .trd/.scl (name:ext:addr, e.g. 'GAME:C:32768')")
	trdDir := flag.Bool("trd-dir", false, "List disk directory and exit (use with --trd or --scl)")
	tapRealtimeFlag := flag.Bool("tap-realtime", false, "Load tape in real-time (with audio, minutes to load)")
	execFlag := flag.String("exec", "", "Execute BASIC command after boot (e.g. 'LOAD \"\"' or 'RANDOMIZE USR 32768')")
	typeFlag := flag.String("type", "", "Type text via keystroke injection (fallback for non-standard ROMs)")
	consoleFlag := flag.Bool("console", false, "Mirror BASIC text output (RST $10) to stdout")
	scaleFlag := flag.Int("scale", 2, "Display scale factor (1-4)")
	noAudioFlag := flag.Bool("no-audio", false, "Disable all audio output")
	noBeeperFlag := flag.Bool("no-beeper", false, "Disable beeper audio (EAR bit)")
	noAYFlag := flag.Bool("no-ay", false, "Disable AY-3-8912 audio")
	// Single-shot screenshot (convenience)
	screenshotFlag := flag.String("screenshot", "", "Save single screenshot to PNG and exit (headless)")

	// Unified --frames: count, range, or multi-spec with auto turbo-skip
	framesFlag := flag.String("frames", "50", "Frame spec: N, N..M, N-M,K (multi), PC=ADDR, DI:HALT")

	// Frame dump (sequence capture, like zxs)
	dumpFrames := flag.String("dump-frames", "", "Save every frame as PNG to directory")
	dumpKeyframes := flag.String("dump-keyframes", "", "Save frames only when screen changes")
	skipFlag := flag.Bool("skip", false, "Turbo-skip frames before capture range (use with --frames range)")
	noBorderFlag := flag.Bool("no-border", false, "Capture 256x192 screen only (no border)")
	fullBorderFlag := flag.Bool("full-border", false, "Capture full ULA output (352x296) for T-state accuracy")
	maxFrames := flag.Int("max-frames", 5000, "Max frames to run in headless mode")
	// Raw binary loading and CPU register setup
	var loadFlags stringSlice
	flag.Var(&loadFlags, "load", "Load binary: FILE@ADDR or FILE@ADDR:PAGE (repeatable, comma-separated)")
	setFlag := flag.String("set", "", "Set CPU registers: PC=8000,SP=FFFF,DI,IM=1 (hex values)")
	runFlag := flag.String("run", "", "Load and run binary: FILE@ADDR (shortcut for --load FILE@ADDR --set PC=ADDR,SP=FFFF,DI,IM=1)")
	saveSnapshotFlag := flag.String("save-snapshot", "", "Save .sna snapshot after running frames (headless)")
	snapshotAtTState := flag.String("snapshot-at-tstate", "", "Save .sna at exact T-state: TSTATE or TSTATE:FILE.sna")
	versionFlag := flag.Bool("version", false, "Print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `mzx - MinZ ZX Spectrum Emulator

USAGE:
  mzx [flags]                         Interactive mode (opens window)
  mzx [flags] --screenshot FILE.png   Headless: capture one frame
  mzx [flags] --dump-frames DIR       Headless: capture frame sequence

MACHINE:
  --model MODEL       Machine model: 48k, 128k, pentagon (default: 48k)
  --rom FILE          Custom ROM file (16K for 48K, 32K for 128K)
  --rom1 FILE         Second ROM for 128K models
  --scale N           Display scale 1-4 (default: 2)

LOADING:
  --snapshot FILE.sna    Load .sna snapshot
  --tap FILE.tap         Load .tap tape file (trap-loaded by default)
  --tap-realtime         Real-time tape loading (with audio, slow)
  --trd FILE.trd         Load .trd TR-DOS disk image
  --scl FILE.scl         Load .scl disk image (auto-converted to TRD)
  --trd-dir              List disk directory and exit
  --trd-load NAME:E:ADDR Load specific file from disk

BARE-METAL CODE:
  --run FILE@ADDR        Load binary + set PC/SP/DI/IM1 (quick start)
  --load FILE@ADDR       Load raw binary to memory address (hex)
  --load FILE@ADDR:PAGE  Load to specific 128K RAM page (0-7)
  --set REGS             Set CPU registers (hex): PC=8000,SP=FFFF,DI,IM=1
                         Supports: PC,SP,AF,BC,DE,HL,IX,IY,AF',BC',DE',HL',
                                   A,I,R,IM,DI,EI

AUTOMATION:
  --exec CMD             Execute BASIC command: --exec 'LOAD ""'
  --type TEXT             Type text via keystroke injection
                         Delay syntax: {N} = wait N frames
                         Example: --type "{50}R" (wait 50 frames, press R)
  --console              Mirror BASIC output (RST $10) to stdout

HEADLESS CAPTURE:
  --screenshot FILE.png  Save one screenshot and exit
  --dump-frames DIR      Save every frame as PNG
  --dump-keyframes DIR   Save frames only when screen changes
  --save-snapshot FILE   Save .sna snapshot after running
  --frames SPEC          Frame specification (default: 50):
                           N         Run N frames
                           N..M      Frame range (capture frames N to M)
                           N-M,K-L   Multiple ranges (comma-separated)
                           PC=ADDR   Trigger at PC value (hex)
                           PC=ADDR+N Capture N frames after PC trigger
                           T=TSTATES Trigger at T-state count
                           DI:HALT   Trigger when CPU halts with DI
  --skip                 Turbo-skip frames before capture range
  --max-frames N         Max frames in headless mode (default: 5000)
  --no-border            Capture 256x192 screen only (no border)
  --full-border          Capture full ULA output including border

SNAPSHOTS:
  --snapshot-at-tstate T       Save .sna at exact T-state count
  --snapshot-at-tstate T:FILE  Save to specific file at T-state

AUDIO:
  --no-audio             Disable all audio
  --no-beeper            Disable beeper (EAR bit)
  --no-ay                Disable AY-3-8912 sound chip

KEYBOARD (interactive):
  F3       Toggle turbo mode (20x speed)
  F4       Hold for turbo
  F6       Play/stop tape (real-time mode)
  F12      Save snapshot (mzx_snapshot_NNNNNN.sna)
  Escape   Quit

EXAMPLES:
  mzx                                              Open 48K Spectrum
  mzx --model pentagon --tap DEMO.TAP              Load Pentagon demo
  mzx --run code.bin@8000                           Run bare-metal binary
  mzx --run code.bin@8000 --screenshot out.png      Run and screenshot
  mzx --tap GAME.TAP --frames 200 --screenshot s.png
  mzx --run demo.bin@8000 --frames DI:HALT --screenshot final.png
  mzx --snapshot saved.sna --save-snapshot copy.sna
  mzx --snapshot-at-tstate 500000:freeze.sna --run code.bin@8000
  mzx --model pentagon --tap DEMO.TAP --type "{50}R" --dump-keyframes ./out/

VERSION: %s (build %s, %s)
`, version, buildNum, buildDate)
	}
	flag.Parse()

	// --run FILE@ADDR: expand to --load + --set
	if *runFlag != "" {
		spec, err := parseLoadSpec(*runFlag)
		if err != nil {
			log.Fatalf("Invalid --run spec: %v", err)
		}
		loadFlags = append(loadFlags, *runFlag)
		// Set PC to load address, SP=top of RAM, DI, IM=1
		runSet := fmt.Sprintf("PC=%04X,SP=FFFF,DI,IM=1", spec.addr)
		if *setFlag != "" {
			*setFlag = runSet + "," + *setFlag // --set overrides can follow
		} else {
			*setFlag = runSet
		}
	}

	if *versionFlag {
		fmt.Printf("mzx %s (build %s)\n", version, buildNum)
		fmt.Printf("  commit: %s\n", gitCommit)
		fmt.Printf("  built:  %s\n", buildDate)
		return
	}

	// --help / -h is handled by flag.Parse() automatically.
	// With no arguments, we boot the 48K Spectrum (embedded ROM).

	scale := *scaleFlag
	if scale < 1 {
		scale = 1
	}
	if scale > 4 {
		scale = 4
	}

	// Resolve border mode: --no-border → 256x192, --full-border → full ULA, default → 320x240
	captureBorder := borderStandard
	if *noBorderFlag {
		captureBorder = borderNone
	} else if *fullBorderFlag {
		captureBorder = borderFull
	}

	var machine *spectrum.Machine
	var err error

	switch *modelFlag {
	case "48k":
		var romData []byte
		if *romFlag != "" {
			romData, err = os.ReadFile(*romFlag)
			if err != nil {
				log.Fatalf("Error reading ROM: %v", err)
			}
		} else {
			romData = embedded48KROM
		}
		machine, err = spectrum.New48K(romData)
		if err != nil {
			log.Fatalf("Error creating 48K machine: %v", err)
		}

	case "128k", "pentagon":
		var rom0, rom1 []byte
		if *romFlag != "" {
			rom0, err = os.ReadFile(*romFlag)
			if err != nil {
				log.Fatalf("Error reading ROM 0: %v", err)
			}
		} else {
			rom0 = embedded48KROM
		}
		if *rom1Flag != "" {
			rom1, err = os.ReadFile(*rom1Flag)
			if err != nil {
				log.Fatalf("Error reading ROM 1: %v", err)
			}
		}
		machine, err = spectrum.NewPentagon128(rom0, rom1)
		if err != nil {
			log.Fatalf("Error creating Pentagon machine: %v", err)
		}

	default:
		log.Fatalf("Unknown model: %s (supported: 48k, 128k, pentagon)", *modelFlag)
	}

	// Disable audio components based on flags
	if *noAudioFlag || *noBeeperFlag {
		machine.Beeper.SetEnabled(false)
	}
	if *noAudioFlag || *noAYFlag {
		if machine.AY != nil {
			machine.AY.SetEnabled(false)
		}
	}

	// Load snapshot if provided
	if *snapshotFlag != "" {
		snap, err := formats.LoadSNA(*snapshotFlag)
		if err != nil {
			log.Fatalf("Error loading snapshot: %v", err)
		}
		formats.ApplySnapshot(machine, snap)
		fmt.Printf("Loaded snapshot: %s\n", *snapshotFlag)
	}

	// Apply --load: write raw binary files into memory
	if len(loadFlags) > 0 {
		var specs []loadSpec
		for _, s := range loadFlags {
			spec, err := parseLoadSpec(s)
			if err != nil {
				log.Fatalf("Invalid --load spec: %v", err)
			}
			specs = append(specs, spec)
		}
		if err := applyLoads(machine, specs); err != nil {
			log.Fatalf("Error applying --load: %v", err)
		}
	}

	// Apply --set: configure CPU registers
	if *setFlag != "" {
		assignments, err := parseSetSpec(*setFlag)
		if err != nil {
			log.Fatalf("Invalid --set spec: %v", err)
		}
		if err := applySetAssignments(machine, assignments); err != nil {
			log.Fatalf("Error applying --set: %v", err)
		}
		// Print what was set
		var parts []string
		for _, a := range assignments {
			if a.name == "DI" || a.name == "EI" {
				parts = append(parts, a.name)
			} else {
				parts = append(parts, fmt.Sprintf("%s=$%04X", a.name, a.value))
			}
		}
		fmt.Printf("Set: %s\n", strings.Join(parts, ", "))
	}

	// Install .tap loading (trap or real-time)
	needsAutoLoad := false
	tapRealtime := false
	if *tapFlag != "" {
		tap, err := formats.LoadTAP(*tapFlag)
		if err != nil {
			log.Fatalf("Error loading .tap file: %v", err)
		}
		if *tapRealtimeFlag {
			// Real-time: pre-compute waveform, feed through port $FE bit 6
			tapRealtime = true
			fmt.Printf("Loaded tape (real-time): %s (%d blocks)\n", *tapFlag, tap.BlockCount())
			formats.InstallRealtimeTAP(machine, tap)
		} else {
			// Trap: intercept at $0556 and inject data instantly
			formats.InstallTAPTrap(machine, tap)
			fmt.Printf("Loaded tape (trap): %s (%d blocks)\n", *tapFlag, tap.BlockCount())
		}
		needsAutoLoad = true
	}

	// Install .trd or .scl traps / load file if provided
	diskPath := *trdFlag
	if *sclFlag != "" {
		diskPath = *sclFlag
	}
	if diskPath != "" {
		var trd *formats.TRDFile
		if *sclFlag != "" {
			trd, err = formats.LoadSCL(diskPath)
			if err != nil {
				log.Fatalf("Error loading .scl file: %v", err)
			}
			fmt.Printf("Loaded disk (SCL→TRD): %s\n", diskPath)
		} else {
			trd, err = formats.LoadTRD(diskPath)
			if err != nil {
				log.Fatalf("Error loading .trd file: %v", err)
			}
			fmt.Printf("Loaded disk: %s\n", diskPath)
		}
		// --trd-dir: list directory and exit
		if *trdDir {
			entries := trd.ListDirectory()
			if len(entries) == 0 {
				fmt.Println("  (empty disk)")
			} else {
				fmt.Printf("  %-8s  %s  %5s  %6s  %s\n", "Name", "Ext", "Start", "Length", "Sectors")
				fmt.Printf("  %-8s  %s  %5s  %6s  %s\n", "--------", "---", "-----", "------", "-------")
				for _, e := range entries {
					fmt.Printf("  %-8s  %c    $%04X  %6d  %d\n",
						e.Name, e.Extension, e.Start, e.Length, e.Sectors)
				}
			}
			fmt.Printf("  %d file(s)\n", len(entries))
			return
		}

		formats.InstallTRDTraps(machine, trd)

		// Optionally load a specific file from the disk
		if *trdLoad != "" {
			name, ext, addr, parseErr := parseTRDLoad(*trdLoad)
			if parseErr != nil {
				log.Fatalf("Invalid --trd-load format: %v (expected name:ext:addr)", parseErr)
			}
			if err := formats.LoadTRDFile(machine, trd, name, ext, addr); err != nil {
				log.Fatalf("Error loading file from disk: %v", err)
			}
			fmt.Printf("Loaded file from disk: %s.%c at $%04X\n", name, ext, addr)
		} else {
			// No explicit --trd-load: try autoboot from "boot" file
			if err := formats.AutoBootTRD(machine, trd); err != nil {
				fmt.Printf("Autoboot: %v (continuing with ROM boot)\n", err)
			}
		}
	}

	// Auto-load from tape if no snapshot was loaded
	if needsAutoLoad && *snapshotFlag == "" {
		if tapRealtime {
			// Start tape playback + LOAD ""
			machine.PlayTape()
			formats.WaitROMInit(machine, 100)
			formats.ExecBASIC(machine, formats.TokenizeLOAD())
			fmt.Println("Real-time tape loading started (F6=play/stop tape)...")
		} else {
			formats.AutoLoadTAP(machine)
		}
	}

	// --console: mirror RST $10 output to stdout
	if *consoleFlag {
		formats.InstallConsoleCapture(machine, os.Stdout)
		fmt.Println("[console mode: BASIC output mirrored to stdout]")
	}

	// --exec: execute arbitrary BASIC command (tokenized, requires compatible ROM)
	if *execFlag != "" {
		tokens, err := formats.TokenizeBASIC(*execFlag)
		if err != nil {
			log.Fatalf("Cannot tokenize BASIC command: %v", err)
		}
		formats.WaitROMInit(machine, 100)
		formats.ExecBASIC(machine, tokens)
		fmt.Printf("Executing: %s -> %s\n", *execFlag, formats.FormatTokenized(tokens))
	}

	// --type: inject keystrokes (works with any ROM)
	var keystrokeQueue *formats.KeystrokeQueue
	if *typeFlag != "" {
		keystrokeQueue = formats.NewKeystrokeQueue(machine, 3, 2)
		keystrokeQueue.TypeText(*typeFlag)
		fmt.Printf("Typing: %q\n", *typeFlag)
	}
	_ = keystrokeQueue // used in Update() for interactive mode

	// --snapshot-at-tstate: T-state precise snapshot saving
	if *snapshotAtTState != "" {
		var target int64
		savePath := fmt.Sprintf("mzx_snapshot_t%s.sna", *snapshotAtTState)

		// Parse "TSTATE" or "TSTATE:FILE.sna"
		parts := strings.SplitN(*snapshotAtTState, ":", 2)
		t, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			log.Fatalf("Invalid T-state value %q: %v", parts[0], err)
		}
		target = t
		if len(parts) == 2 {
			savePath = parts[1]
		}

		machine.SetTStateTrap(target, func(actual int64) {
			if err := formats.SaveSNA(savePath, machine); err != nil {
				log.Printf("Error saving snapshot at T=%d: %v", actual, err)
			} else {
				fmt.Printf("Snapshot saved at T=%d (target %d): %s\n", actual, target, savePath)
			}
		})
		fmt.Printf("T-state trap set at T=%d → %s\n", target, savePath)
	}

	// Headless mode: --screenshot (single), --dump-frames, --dump-keyframes, --save-snapshot
	isHeadless := *screenshotFlag != "" || *dumpFrames != "" || *dumpKeyframes != "" || *saveSnapshotFlag != ""

	if isHeadless {
		cs := parseCaptureSpec(*framesFlag)
		limit := *maxFrames

		// --- Path A: Plain count + single screenshot/snapshot ---
		if cs.isCount && *dumpFrames == "" && *dumpKeyframes == "" {
			frames := cs.count
			if frames < 1 {
				frames = 1
			}
			fmt.Printf("Running %d frames...\n", frames)
			for i := 0; i < frames; i++ {
				machine.RunFrame()
			}
			if *screenshotFlag != "" {
				if err := saveScreenshotEx(machine, *screenshotFlag, captureBorder); err != nil {
					log.Fatalf("Error saving screenshot: %v", err)
				}
				fmt.Printf("Screenshot saved: %s\n", *screenshotFlag)
			}
			if *saveSnapshotFlag != "" {
				if err := formats.SaveSNA(*saveSnapshotFlag, machine); err != nil {
					log.Fatalf("Error saving snapshot: %v", err)
				}
				fmt.Printf("Snapshot saved: %s\n", *saveSnapshotFlag)
			}
			return
		}

		// --- Output setup ---
		dumpDir := *dumpFrames
		isKeyframeMode := false
		if *dumpKeyframes != "" {
			dumpDir = *dumpKeyframes
			isKeyframeMode = true
		}
		singleFile := *screenshotFlag
		if singleFile != "" {
			dumpDir = ""
		}
		if dumpDir != "" {
			if err := os.MkdirAll(dumpDir, 0755); err != nil {
				log.Fatalf("Error creating dump directory: %v", err)
			}
		}

		var prevScreen []byte
		capturedCount := 0

		// --- Path B: Legacy triggers (PC=, T=, DI:HALT) ---
		if cs.legacy != nil {
			spec := *cs.legacy
			triggered := false
			inRange := false
			rangeEndFrame := 0

			fmt.Printf("Running up to %d frames (trigger mode)...\n", limit)
			for frame := 0; frame < limit; frame++ {
				if keystrokeQueue != nil && !keystrokeQueue.Done() {
					keystrokeQueue.Update()
				}
				machine.RunFrame()

				if !inRange && spec.matchesStart(machine, frame) {
					inRange = true
					triggered = true
					rangeEndFrame = frame + spec.rangeOffset
					fmt.Printf("  Trigger START at frame %d\n", frame)
				}
				if inRange && spec.hasEnd() {
					if spec.rangeOffset > 0 && frame >= rangeEndFrame {
						fmt.Printf("  Trigger END at frame %d (%d captured)\n", frame, capturedCount)
						break
					}
					if spec.matchesEnd(machine, frame) {
						fmt.Printf("  Trigger END at frame %d\n", frame)
						break
					}
				}
				if triggered && !spec.hasEnd() && spec.isSingleTrigger() {
					if singleFile != "" {
						if err := saveScreenshotEx(machine, singleFile, captureBorder); err != nil {
							log.Fatalf("Error saving screenshot: %v", err)
						}
						fmt.Printf("Screenshot saved: %s (frame %d)\n", singleFile, frame)
					}
					return
				}

				shouldCapture := triggered && inRange
				if shouldCapture && isKeyframeMode {
					fb := machine.Framebuffer()
					if prevScreen != nil && screensEqual(prevScreen, fb) {
						continue
					}
					prevScreen = make([]byte, len(fb))
					copy(prevScreen, fb)
				}
				if shouldCapture && dumpDir != "" {
					path := fmt.Sprintf("%s/frame_%06d.png", dumpDir, frame)
					if err := saveScreenshotEx(machine, path, captureBorder); err != nil {
						log.Printf("Warning: failed to save frame %d: %v", frame, err)
					}
					capturedCount++
				}
			}

			if dumpDir != "" {
				fmt.Printf("Captured %d frames to %s\n", capturedCount, dumpDir)
			}
			return
		}

		// --- Path C: Range-based capture (with auto turbo-skip) ---

		// Convert plain count to range for dump mode
		if cs.isCount {
			cs.ranges = []captureRange{{0, cs.count - 1}}
			cs.isCount = false
		}

		if len(cs.ranges) == 0 {
			fmt.Println("No capture ranges specified")
			return
		}

		// Print capture plan
		for _, r := range cs.ranges {
			if r.start == r.end {
				fmt.Printf("  Capture frame %d\n", r.start)
			} else {
				fmt.Printf("  Capture frames %d..%d\n", r.start, r.end)
			}
		}

		// Auto-raise limit to cover all ranges
		lastEnd := cs.lastEnd()
		if lastEnd+10 > limit {
			limit = lastEnd + 10
		}

		// Turbo-skip to first range (only with --skip flag)
		startFrame := 0
		if *skipFlag {
			skipTo := cs.firstStart() - 2 // 2 frames early for ULA warmup
			if skipTo < 0 {
				skipTo = 0
			}
			if skipTo > 0 {
				fmt.Printf("Turbo-skipping %d frames...\n", skipTo)
				t0 := time.Now()
				for i := 0; i < skipTo; i++ {
					// Inject keystrokes during turbo-skip so --skip and
					// non-skip modes produce identical results.
					if keystrokeQueue != nil && !keystrokeQueue.Done() {
						keystrokeQueue.Update()
					}
					machine.RunFrameFast()
				}
				elapsed := time.Since(t0)
				fmt.Printf("Turbo-skipped %d frames in %v (%.0f fps)\n",
					skipTo, elapsed.Round(time.Millisecond), float64(skipTo)/elapsed.Seconds())
				startFrame = skipTo
			}
		}

		fmt.Printf("Running frames %d..%d...\n", startFrame, lastEnd)

		for frame := startFrame; frame <= lastEnd; frame++ {
			if keystrokeQueue != nil && !keystrokeQueue.Done() {
				keystrokeQueue.Update()
			}
			machine.RunFrame()

			if !cs.containsFrame(frame) {
				continue
			}

			// Keyframe dedup
			if isKeyframeMode {
				fb := machine.Framebuffer()
				if prevScreen != nil && screensEqual(prevScreen, fb) {
					continue
				}
				prevScreen = make([]byte, len(fb))
				copy(prevScreen, fb)
			}

			// Single-file capture: grab first match
			if singleFile != "" {
				if err := saveScreenshotEx(machine, singleFile, captureBorder); err != nil {
					log.Fatalf("Error saving screenshot: %v", err)
				}
				fmt.Printf("Screenshot saved: %s (frame %d)\n", singleFile, frame)
				return
			}

			// Dump capture
			if dumpDir != "" {
				path := fmt.Sprintf("%s/frame_%06d.png", dumpDir, frame)
				if err := saveScreenshotEx(machine, path, captureBorder); err != nil {
					log.Printf("Warning: failed to save frame %d: %v", frame, err)
				}
				capturedCount++
			}
		}

		if dumpDir != "" {
			fmt.Printf("Captured %d frames to %s\n", capturedCount, dumpDir)
		}
		return
	}

	// Configure Ebitengine
	ebiten.SetWindowSize(machine.ScreenWidth()*scale, machine.ScreenHeight()*scale)
	ebiten.SetWindowTitle("MZX - ZX Spectrum Emulator")
	ebiten.SetTPS(50)           // 50 Hz PAL
	ebiten.SetVsyncEnabled(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// Suppress stderr during Ebitengine init to hide CAMetalLayer warnings on macOS.
	// Restored in Game.Update() after the first few frames.
	suppressStderr()

	game := newGame(machine, scale, *noAudioFlag)
	game.keystrokeQueue = keystrokeQueue

	fmt.Printf("MZX ZX Spectrum Emulator\n")
	fmt.Printf("Model: %s (%dx%d, %d T-states/frame)\n",
		machine.Mode.Name,
		machine.ScreenWidth(), machine.ScreenHeight(),
		machine.Mode.TStatesPerFrame())
	fmt.Printf("Scale: %dx\n", scale)
	fmt.Printf("Keys: F1=pause, F2=screenshot, F3=turbo, F4=hold-turbo, F5=reset\n")

	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}

// parseTRDLoad parses "name:ext:addr" format for --trd-load.
func parseTRDLoad(s string) (name string, ext byte, addr uint16, err error) {
	var addrInt int
	parts := splitColon(s)
	if len(parts) != 3 {
		return "", 0, 0, fmt.Errorf("expected name:ext:addr")
	}
	name = parts[0]
	if len(parts[1]) != 1 {
		return "", 0, 0, fmt.Errorf("extension must be a single character")
	}
	ext = parts[1][0]
	_, err = fmt.Sscanf(parts[2], "%d", &addrInt)
	if err != nil {
		// Try hex
		_, err = fmt.Sscanf(parts[2], "0x%x", &addrInt)
		if err != nil {
			_, err = fmt.Sscanf(parts[2], "$%x", &addrInt)
			if err != nil {
				return "", 0, 0, fmt.Errorf("invalid address: %s", parts[2])
			}
		}
	}
	return name, ext, uint16(addrInt), nil
}

func splitColon(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}


