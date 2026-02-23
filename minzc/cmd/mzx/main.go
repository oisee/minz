// Command mzx is a T-state accurate ZX Spectrum emulator for the MinZ toolchain.
package main

import (
	"flag"
	"fmt"
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
		player := &beeperPlayer{beeper: machine.Beeper}
		var err error
		g.audioPlayer, err = g.audioCtx.NewPlayer(player)
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

// beeperPlayer implements io.Reader for ebiten audio streaming.
type beeperPlayer struct {
	beeper *spectrum.Beeper
	buf    []float32
}

func (p *beeperPlayer) Read(data []byte) (int, error) {
	// Ebiten audio expects signed 16-bit stereo PCM at audioSampleRate
	samples := len(data) / 4 // 4 bytes per stereo sample (2 bytes L + 2 bytes R)
	if cap(p.buf) < samples {
		p.buf = make([]float32, samples)
	}
	p.buf = p.buf[:samples]

	n := p.beeper.ReadSamples(p.buf)

	// Fill remaining with silence
	for i := n; i < samples; i++ {
		p.buf[i] = 0
	}

	// Convert float32 mono → signed 16-bit stereo
	for i := 0; i < samples; i++ {
		s := int16(p.buf[i] * 32767)
		lo := byte(s)
		hi := byte(s >> 8)
		j := i * 4
		if j+3 < len(data) {
			data[j+0] = lo   // left low
			data[j+1] = hi   // left high
			data[j+2] = lo   // right low
			data[j+3] = hi   // right high
		}
	}

	return samples * 4, nil
}

func main() {
	modelFlag := flag.String("model", "48k", "Machine model: 48k, pentagon")
	romFlag := flag.String("rom", "", "Path to ROM file (required)")
	rom1Flag := flag.String("rom1", "", "Path to second ROM (128K models)")
	snapshotFlag := flag.String("snapshot", "", "Path to .sna snapshot file")
	scaleFlag := flag.Int("scale", 2, "Display scale factor (1-4)")
	noAudioFlag := flag.Bool("no-audio", false, "Disable audio output")
	flag.Parse()

	if *romFlag == "" && *snapshotFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: mzx --rom <path> [--snapshot <path>] [options]")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		flag.PrintDefaults()
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
	fmt.Printf("Keys: ESC=quit, F1=pause, F5=reset\n")

	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}

