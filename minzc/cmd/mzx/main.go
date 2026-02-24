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
		if err := saveScreenshot(g.machine, path); err != nil {
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

// saveScreenshot saves the machine's current framebuffer as a PNG file.
func saveScreenshot(m *spectrum.Machine, path string) error {
	fb := m.Framebuffer()
	w := m.ScreenWidth()
	h := m.ScreenHeight()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			img.SetRGBA(x, y, color.RGBA{
				R: fb[off+0],
				G: fb[off+1],
				B: fb[off+2],
				A: fb[off+3],
			})
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
	screenshotFlag := flag.String("screenshot", "", "Save screenshot to PNG and exit (headless)")
	framesFlag := flag.Int("frames", 50, "Frames to run before screenshot (with --screenshot)")
	screenshotOnHalt := flag.Bool("screenshot-on-halt", false, "Screenshot when CPU halts (with --screenshot)")
	screenshotOnStable := flag.Int("screenshot-on-stable", 0, "Screenshot after N frames of unchanged screen (with --screenshot)")
	screenshotAtPC := flag.String("screenshot-at-pc", "", "Screenshot when PC reaches address (hex, with --screenshot)")
	maxFrames := flag.Int("max-frames", 5000, "Max frames before giving up (conditional screenshot)")
	flag.Parse()

	if *romFlag == "" && *snapshotFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: mzx --rom <path> [--snapshot <path>] [options]")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --tap game.tap")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --frames 100")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --screenshot-on-halt")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --screenshot-on-stable 3")
		fmt.Fprintln(os.Stderr, "  mzx --rom 48.rom --snapshot game.sna --screenshot shot.png --screenshot-at-pc 8000")
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

	// Headless screenshot mode — supports several trigger conditions
	if *screenshotFlag != "" {
		var triggerPC uint16
		hasAtPC := false
		if *screenshotAtPC != "" {
			var pcInt int
			if _, err := fmt.Sscanf(*screenshotAtPC, "0x%x", &pcInt); err != nil {
				if _, err := fmt.Sscanf(*screenshotAtPC, "$%x", &pcInt); err != nil {
					if _, err := fmt.Sscanf(*screenshotAtPC, "%x", &pcInt); err != nil {
						log.Fatalf("Invalid --screenshot-at-pc address: %s", *screenshotAtPC)
					}
				}
			}
			triggerPC = uint16(pcInt)
			hasAtPC = true
		}

		isConditional := *screenshotOnHalt || *screenshotOnStable > 0 || hasAtPC

		if !isConditional {
			// Simple mode: run N frames, capture
			frames := *framesFlag
			if frames < 1 {
				frames = 1
			}
			fmt.Printf("Running %d frames for screenshot...\n", frames)
			for i := 0; i < frames; i++ {
				machine.RunFrame()
			}
		} else {
			// Conditional mode: run until trigger or max-frames
			limit := *maxFrames
			stableCount := 0
			var prevScreen []byte

			fmt.Printf("Running up to %d frames, waiting for trigger...\n", limit)
			triggered := false

			for i := 0; i < limit; i++ {
				machine.RunFrame()

				// Check halt condition
				if *screenshotOnHalt && machine.CPU.Halted() {
					fmt.Printf("  HALT detected at frame %d\n", i+1)
					triggered = true
					break
				}

				// Check PC condition (checked after frame, PC is current)
				if hasAtPC && machine.CPU.PC() == triggerPC {
					fmt.Printf("  PC=$%04X reached at frame %d\n", triggerPC, i+1)
					triggered = true
					break
				}

				// Check screen stability
				if *screenshotOnStable > 0 {
					fb := machine.Framebuffer()
					if prevScreen != nil && screensEqual(prevScreen, fb) {
						stableCount++
						if stableCount >= *screenshotOnStable {
							fmt.Printf("  Screen stable for %d frames at frame %d\n",
								stableCount, i+1)
							triggered = true
							break
						}
					} else {
						stableCount = 0
						prevScreen = make([]byte, len(fb))
						copy(prevScreen, fb)
					}
				}
			}

			if !triggered {
				fmt.Printf("  Warning: max frames (%d) reached without trigger\n", limit)
			}
		}

		if err := saveScreenshot(machine, *screenshotFlag); err != nil {
			log.Fatalf("Error saving screenshot: %v", err)
		}
		fmt.Printf("Screenshot saved: %s (%dx%d)\n", *screenshotFlag,
			machine.ScreenWidth(), machine.ScreenHeight())
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

