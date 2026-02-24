// Command mzx is a T-state accurate ZX Spectrum emulator for the MinZ toolchain.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
)

const (
	audioSampleRate = spectrum.BeeperSampleRate
)

// Game implements ebiten.Game.
type Game struct {
	machine  *spectrum.Machine
	screen   *ebiten.Image
	scale    int
	noAudio  bool

	// Audio
	audioCtx    *audio.Context
	audioPlayer *audio.Player

	// Key mapping: ebiten key → []SpecKey (some keys map to Shift+key)
	keyMap map[ebiten.Key][]spectrum.SpecKey
}

func newGame(machine *spectrum.Machine, scale int, noAudio bool) *Game {
	g := &Game{
		machine: machine,
		screen:  ebiten.NewImage(machine.ScreenWidth(), machine.ScreenHeight()),
		scale:   scale,
		noAudio: noAudio,
		keyMap:  buildKeyMap(),
	}

	if !noAudio {
		g.audioCtx = audio.NewContext(audioSampleRate)
		mixer := &audioMixer{beeper: machine.Beeper, ay: machine.AY}
		var err error
		g.audioPlayer, err = g.audioCtx.NewPlayer(mixer)
		if err != nil {
			log.Printf("Warning: audio init failed: %v", err)
			g.noAudio = true
		} else {
			g.audioPlayer.SetBufferSize(audioSampleRate / 25) // ~40ms buffer
			g.audioPlayer.Play()
		}
	}

	return g
}

func (g *Game) Update() error {
	// Handle special keys
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		g.machine.Reset()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.machine.SetPaused(!g.machine.IsPaused())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		path := fmt.Sprintf("mzx_screenshot_%06d.png", g.machine.FrameCount())
		if err := saveScreenshotEx(g.machine, path, false); err != nil {
			log.Printf("Screenshot error: %v", err)
		} else {
			log.Printf("Screenshot saved: %s", path)
		}
	}

	// Sync keyboard state
	g.syncKeyboard()

	// Run one frame
	g.machine.RunFrame()

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

	for key, specKeys := range g.keyMap {
		if ebiten.IsKeyPressed(key) {
			for _, sk := range specKeys {
				g.machine.Keyboard.KeyPress(sk.Row, sk.Bit)
			}
		}
	}
}

func buildKeyMap() map[ebiten.Key][]spectrum.SpecKey {
	m := map[ebiten.Key][]spectrum.SpecKey{
		// Letters
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

		// Numbers
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

		// Special keys
		ebiten.KeyEnter:     {spectrum.KeyEnter},
		ebiten.KeySpace:     {spectrum.KeySpace},
		ebiten.KeyShiftLeft: {spectrum.KeyShift},
		ebiten.KeyControlLeft: {spectrum.KeySym},

		// Arrow keys → Shift + 5/6/7/8
		ebiten.KeyArrowLeft:  {spectrum.KeyShift, spectrum.Key5},
		ebiten.KeyArrowDown:  {spectrum.KeyShift, spectrum.Key6},
		ebiten.KeyArrowUp:    {spectrum.KeyShift, spectrum.Key7},
		ebiten.KeyArrowRight: {spectrum.KeyShift, spectrum.Key8},

		// Backspace → Shift + 0 (DELETE)
		ebiten.KeyBackspace: {spectrum.KeyShift, spectrum.Key0},
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
	// Ebiten audio expects signed 16-bit stereo PCM at audioSampleRate
	samples := len(data) / 4 // 4 bytes per stereo sample (2 bytes L + 2 bytes R)

	// Read beeper samples (frame-synchronized from Beeper.EndFrame)
	if cap(p.beeperBuf) < samples {
		p.beeperBuf = make([]float32, samples)
	}
	p.beeperBuf = p.beeperBuf[:samples]
	n := p.beeper.ReadSamples(p.beeperBuf)
	for i := n; i < samples; i++ {
		p.beeperBuf[i] = 0
	}

	// Read AY samples from frame buffer (frame-synchronized from AY.EndFrame)
	hasAY := p.ay != nil
	ayN := 0
	if hasAY {
		if cap(p.ayLeft) < samples {
			p.ayLeft = make([]float64, samples)
			p.ayRight = make([]float64, samples)
		}
		p.ayLeft = p.ayLeft[:samples]
		p.ayRight = p.ayRight[:samples]
		ayN = p.ay.ReadFrameSamples(p.ayLeft, p.ayRight)
		// Zero remainder if frame buffer underrun
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
			// Mix: beeper at 60% volume, AY at 80%
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
		if j+3 < len(data) {
			data[j+0] = byte(sL)
			data[j+1] = byte(sL >> 8)
			data[j+2] = byte(sR)
			data[j+3] = byte(sR >> 8)
		}
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

// saveScreenshotEx saves the framebuffer as PNG, optionally cropping border.
func saveScreenshotEx(m *spectrum.Machine, path string, noBorder bool) error {
	fb := m.Framebuffer()
	fullW := m.ScreenWidth()
	fullH := m.ScreenHeight()

	var img *image.RGBA
	if noBorder {
		// Crop to 256x192 screen area (skip border)
		bLeft := m.Mode.BorderLeft
		bTop := m.Mode.BorderTop
		img = image.NewRGBA(image.Rect(0, 0, 256, 192))
		for y := 0; y < 192; y++ {
			for x := 0; x < 256; x++ {
				srcX := bLeft + x
				srcY := bTop + y
				off := (srcY*fullW + srcX) * 4
				img.SetRGBA(x, y, color.RGBA{
					R: fb[off+0], G: fb[off+1], B: fb[off+2], A: fb[off+3],
				})
			}
		}
	} else {
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

func main() {
	modelFlag := flag.String("model", "48k", "Machine model: 48k, pentagon")
	romFlag := flag.String("rom", "", "Path to ROM file")
	rom1Flag := flag.String("rom1", "", "Path to second ROM (128K models)")
	snapshotFlag := flag.String("snapshot", "", "Path to .sna snapshot file")
	tapFlag := flag.String("tap", "", "Path to .tap tape file")
	trdFlag := flag.String("trd", "", "Path to .trd disk image")
	trdLoad := flag.String("trd-load", "", "File to load from .trd (name:ext:addr, e.g. 'GAME:C:32768')")
	scaleFlag := flag.Int("scale", 2, "Display scale factor (1-4)")
	noAudioFlag := flag.Bool("no-audio", false, "Disable audio output")
	// Single-shot screenshot (convenience)
	screenshotFlag := flag.String("screenshot", "", "Save single screenshot to PNG and exit (headless)")
	framesFlag := flag.Int("frames", 50, "Frames to run before screenshot (with --screenshot)")

	// Frame dump (sequence capture, like zxs)
	dumpFrames := flag.String("dump-frames", "", "Save every frame as PNG to directory")
	dumpKeyframes := flag.String("dump-keyframes", "", "Save frames only when screen changes")
	frameSpec := flag.String("frame-spec", "", "Frame range: N, N..M, PC=ADDR, PC=ADDR+N, T=TSTATES, DI:HALT")
	noBorder := flag.Bool("no-border", false, "Capture 256x192 screen only (no border)")
	maxFrames := flag.Int("max-frames", 5000, "Max frames to run in headless mode")
	flag.Parse()

	if *romFlag == "" && *snapshotFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: mzx --rom <path> [--snapshot <path>] [options]")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --tap game.tap")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --frames 100")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --frame-spec DI:HALT")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --frame-spec PC=8000")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --dump-frames ./frames --frame-spec 100..200")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --dump-keyframes ./kf --no-border")
		fmt.Fprintln(os.Stderr, "  mzx --model pentagon --rom 128-0.rom --rom1 trdos.rom --trd game.trd")
		os.Exit(1)
	}

	scale := *scaleFlag
	if scale < 1 {
		scale = 1
	}
	if scale > 4 {
		scale = 4
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
		}
		machine, err = spectrum.New48K(romData)
		if err != nil {
			log.Fatalf("Error creating 48K machine: %v", err)
		}

	case "pentagon":
		var rom0, rom1 []byte
		if *romFlag != "" {
			rom0, err = os.ReadFile(*romFlag)
			if err != nil {
				log.Fatalf("Error reading ROM 0: %v", err)
			}
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
		log.Fatalf("Unknown model: %s (supported: 48k, pentagon)", *modelFlag)
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

	// Install .tap ROM trap if provided
	if *tapFlag != "" {
		tap, err := formats.LoadTAP(*tapFlag)
		if err != nil {
			log.Fatalf("Error loading .tap file: %v", err)
		}
		formats.InstallTAPTrap(machine, tap)
		fmt.Printf("Loaded tape: %s (%d blocks)\n", *tapFlag, tap.BlockCount())
	}

	// Install .trd traps / load file if provided
	if *trdFlag != "" {
		trd, err := formats.LoadTRD(*trdFlag)
		if err != nil {
			log.Fatalf("Error loading .trd file: %v", err)
		}
		formats.InstallTRDTraps(machine, trd)
		fmt.Printf("Loaded disk: %s\n", *trdFlag)

		// Optionally load a specific file from the disk
		if *trdLoad != "" {
			name, ext, addr, parseErr := parseTRDLoad(*trdLoad)
			if parseErr != nil {
				log.Fatalf("Invalid --trd-load format: %v (expected name:ext:addr)", parseErr)
			}
			if err := formats.LoadTRDFile(machine, trd, name, ext, addr); err != nil {
				log.Fatalf("Error loading file from .trd: %v", err)
			}
			fmt.Printf("Loaded file from disk: %s.%c at $%04X\n", name, ext, addr)
		}
	}

	// Headless mode: --screenshot (single), --dump-frames, --dump-keyframes
	isHeadless := *screenshotFlag != "" || *dumpFrames != "" || *dumpKeyframes != ""

	if isHeadless {
		spec := parseFrameSpec(*frameSpec)
		limit := *maxFrames

		if *screenshotFlag != "" && spec.isEmpty() {
			// Simple --screenshot: run N frames, capture one shot
			frames := *framesFlag
			if frames < 1 {
				frames = 1
			}
			fmt.Printf("Running %d frames for screenshot...\n", frames)
			for i := 0; i < frames; i++ {
				machine.RunFrame()
			}
			if err := saveScreenshotEx(machine, *screenshotFlag, *noBorder); err != nil {
				log.Fatalf("Error saving screenshot: %v", err)
			}
			fmt.Printf("Screenshot saved: %s\n", *screenshotFlag)
			return
		}

		// Frame dump / conditional mode
		dumpDir := *dumpFrames
		isKeyframeMode := false
		if *dumpKeyframes != "" {
			dumpDir = *dumpKeyframes
			isKeyframeMode = true
		}

		// For --screenshot with --frame-spec, use single-file output
		singleFile := *screenshotFlag
		if singleFile != "" {
			dumpDir = "" // don't create directory
		}

		// Create dump directory if needed
		if dumpDir != "" {
			if err := os.MkdirAll(dumpDir, 0755); err != nil {
				log.Fatalf("Error creating dump directory: %v", err)
			}
		}

		var prevScreen []byte
		capturedCount := 0
		triggered := spec.isEmpty() // no spec = capture everything; with spec = wait for trigger
		inRange := false
		rangeEndFrame := 0

		fmt.Printf("Running up to %d frames (headless)...\n", limit)

		for frame := 0; frame < limit; frame++ {
			machine.RunFrame()

			// Check triggers
			if !spec.isEmpty() {
				// Start trigger
				if !inRange && spec.matchesStart(machine, frame) {
					inRange = true
					triggered = true
					rangeEndFrame = frame + spec.rangeOffset
					fmt.Printf("  Trigger START at frame %d\n", frame)
				}

				// End trigger
				if inRange && spec.hasEnd() {
					if spec.rangeOffset > 0 && frame >= rangeEndFrame {
						fmt.Printf("  Trigger END at frame %d (%d frames captured)\n", frame, capturedCount)
						break
					}
					if spec.matchesEnd(machine, frame) {
						fmt.Printf("  Trigger END at frame %d\n", frame)
						break
					}
				}

				// Single-frame trigger (no range) — capture and stop
				if triggered && !spec.hasEnd() && spec.isSingleTrigger() {
					if err := saveScreenshotEx(machine, singleFile, *noBorder); err != nil {
						log.Fatalf("Error saving screenshot: %v", err)
					}
					fmt.Printf("  Triggered at frame %d\n", frame)
					fmt.Printf("Screenshot saved: %s\n", singleFile)
					return
				}
			}

			// Should we capture this frame?
			shouldCapture := triggered && (spec.isEmpty() || inRange)

			if shouldCapture && isKeyframeMode {
				fb := machine.Framebuffer()
				if prevScreen != nil && screensEqual(prevScreen, fb) {
					continue // screen unchanged, skip
				}
				prevScreen = make([]byte, len(fb))
				copy(prevScreen, fb)
			}

			if shouldCapture && dumpDir != "" {
				path := fmt.Sprintf("%s/frame_%06d.png", dumpDir, frame)
				if err := saveScreenshotEx(machine, path, *noBorder); err != nil {
					log.Printf("Warning: failed to save frame %d: %v", frame, err)
				}
				capturedCount++
			}

			// For single-frame specs like "100"
			if !spec.isEmpty() && spec.isSingleFrame() && frame >= spec.startFrame {
				if singleFile != "" {
					if err := saveScreenshotEx(machine, singleFile, *noBorder); err != nil {
						log.Fatalf("Error saving screenshot: %v", err)
					}
					fmt.Printf("Screenshot saved: %s (frame %d)\n", singleFile, frame)
				}
				return
			}

			// For frame ranges like "100..200"
			if !spec.isEmpty() && spec.isFrameRange() && frame >= spec.endFrame {
				break
			}
		}

		if singleFile != "" && capturedCount == 0 {
			// Fallback: save current screen if we ran out of frames
			if err := saveScreenshotEx(machine, singleFile, *noBorder); err != nil {
				log.Fatalf("Error saving screenshot: %v", err)
			}
			fmt.Printf("Screenshot saved: %s (max frames reached)\n", singleFile)
		} else if dumpDir != "" {
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

	game := newGame(machine, scale, *noAudioFlag)

	fmt.Printf("MZX ZX Spectrum Emulator\n")
	fmt.Printf("Model: %s (%dx%d, %d T-states/frame)\n",
		machine.Mode.Name,
		machine.ScreenWidth(), machine.ScreenHeight(),
		machine.Mode.TStatesPerFrame())
	fmt.Printf("Scale: %dx\n", scale)
	fmt.Printf("Keys: ESC=quit, F1=pause, F5=reset, F2=screenshot\n")

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

