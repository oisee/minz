// Headless-only MZX: no Ebitengine, no GLFW, no audio.
// Build: go build -tags mzx_headless -o mzx-headless ./cmd/mzx
//
// Supports: --screenshot, --dump-frames, --dump-keyframes, --save-snapshot,
// --frames, --load, --set, --run, --console-to-port, --console-io
//
//go:build mzx_headless

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
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/spectrum/rzx"
)

//go:embed roms/48.rom
var embedded48ROM []byte

func main() {
	modelFlag := flag.String("model", "48k", "Machine model: 48k, 128k, pentagon")
	romFlag := flag.String("rom", "", "Path to ROM file")
	snapshotFlag := flag.String("snapshot", "", "Path to .sna snapshot file")
	tapFlag := flag.String("tap", "", "Path to .tap tape file")
	screenshotFlag := flag.String("screenshot", "", "Save screenshot to PNG")
	framesFlag := flag.String("frames", "50", "Frame count")
	dumpFrames := flag.String("dump-frames", "", "Save every frame as PNG to directory")
	dumpKeyframes := flag.String("dump-keyframes", "", "Save frames only when screen changes")
	dumpSCR := flag.String("dump-scr", "", "Save every frame as .scr (6912 bytes) to directory")
	maxFrames := flag.Int("max-frames", 5000, "Max frames to run")
	saveSnapshotFlag := flag.String("save-snapshot", "", "Save .sna snapshot after running")
	noBorderFlag := flag.Bool("no-border", false, "Capture 256x192 screen only")
	var loadFlags stringSlice
	flag.Var(&loadFlags, "load", "Load binary: FILE@ADDR (repeatable)")
	setFlag := flag.String("set", "", "Set CPU registers: PC=8000,SP=FFFF,DI,IM=1")
	runFlag := flag.String("run", "", "Load and run: FILE@ADDR (shortcut)")
	rzxFlag := flag.String("rzx", "", "Replay RZX recording file (overrides --frames)")
	flag.Parse()

	// Build machine
	var machine *spectrum.Machine
	var err error

	switch *modelFlag {
	case "48k":
		romData := embedded48ROM
		if *romFlag != "" {
			romData, err = os.ReadFile(*romFlag)
			if err != nil {
				log.Fatalf("ROM: %v", err)
			}
		}
		machine, err = spectrum.New48K(romData)
	default:
		log.Fatalf("headless mode only supports 48k model (got %s)", *modelFlag)
	}
	if err != nil {
		log.Fatalf("Machine init: %v", err)
	}

	// Load snapshot
	if *snapshotFlag != "" {
		snap, err := formats.LoadSNA(*snapshotFlag)
		if err != nil {
			log.Fatalf("SNA: %v", err)
		}
		formats.ApplySnapshot(machine, snap)
	}

	// Load TAP
	if *tapFlag != "" {
		// TAP loading in headless: just load via trap
		tapData, err := formats.LoadTAP(*tapFlag)
		if err != nil {
			log.Fatalf("TAP: %v", err)
		}
		_ = tapData // TODO: install tape trap
	}

	// --run shortcut
	if *runFlag != "" {
		loadFlags = append(loadFlags, *runFlag)
		parts := strings.SplitN(*runFlag, "@", 2)
		if len(parts) == 2 {
			*setFlag = fmt.Sprintf("PC=%s,SP=FFFF,DI,IM=1", parts[1])
		}
	}

	// --load binaries
	for _, lf := range loadFlags {
		parts := strings.SplitN(lf, "@", 2)
		if len(parts) != 2 {
			log.Fatalf("--load: expected FILE@ADDR, got %q", lf)
		}
		data, err := os.ReadFile(parts[0])
		if err != nil {
			log.Fatalf("--load %s: %v", parts[0], err)
		}
		addr, err := strconv.ParseUint(strings.TrimPrefix(parts[1], "0x"), 16, 16)
		if err != nil {
			addr2, err2 := strconv.ParseUint(parts[1], 10, 16)
			if err2 != nil {
				log.Fatalf("--load: bad address %q", parts[1])
			}
			addr = addr2
		}
		for j, b := range data {
			machine.Memory.Write(uint16(addr)+uint16(j), b, false)
		}
		fmt.Fprintf(os.Stderr, "Loaded %d bytes at $%04X\n", len(data), addr)
	}

	// --set registers
	if *setFlag != "" {
		for _, kv := range strings.Split(*setFlag, ",") {
			kv = strings.TrimSpace(kv)
			if kv == "DI" {
				machine.CPU.SetIFF1(false)
				machine.CPU.SetIFF2(false)
				continue
			}
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				continue
			}
			v, _ := strconv.ParseUint(parts[1], 16, 16)
			switch strings.ToUpper(parts[0]) {
			case "PC":
				machine.CPU.SetPC(uint16(v))
			case "SP":
				machine.CPU.SetSP(uint16(v))
			case "IM":
				machine.CPU.SetIM(byte(v))
			}
		}
	}

	// Parse frame count
	frames, _ := strconv.Atoi(*framesFlag)
	if frames < 1 {
		frames = 50
	}
	if frames > *maxFrames {
		frames = *maxFrames
	}

	// Create dump dirs
	if *dumpFrames != "" {
		os.MkdirAll(*dumpFrames, 0755)
	}
	if *dumpKeyframes != "" {
		os.MkdirAll(*dumpKeyframes, 0755)
	}
	if *dumpSCR != "" {
		os.MkdirAll(*dumpSCR, 0755)
	}

	// ── RZX replay mode ─────────────────────────────────────────────────
	if *rzxFlag != "" {
		rec, err := rzx.ReadFile(*rzxFlag)
		if err != nil {
			log.Fatalf("RZX: %v", err)
		}
		player, err := rzx.NewPlayer(rec, machine)
		if err != nil {
			log.Fatalf("RZX player: %v", err)
		}
		fmt.Fprintf(os.Stderr, "MZX RZX replay: %s (%d frames, creator: %s)\n",
			*rzxFlag, player.TotalFrames(), rec.Creator)

		saved := 0
		for player.Next() {
			i := player.Frame
			if *dumpSCR != "" {
				path := fmt.Sprintf("%s/frame_%04d.scr", *dumpSCR, i)
				os.WriteFile(path, player.ScreenSCR(), 0644)
			}
			if *dumpFrames != "" {
				path := fmt.Sprintf("%s/frame_%04d.png", *dumpFrames, i)
				saveScreenPNG(machine, path, *noBorderFlag)
				saved++
			}
			if i >= *maxFrames {
				fmt.Fprintf(os.Stderr, "RZX: stopped at frame %d (max-frames)\n", i)
				break
			}
		}
		if *screenshotFlag != "" {
			saveScreenPNG(machine, *screenshotFlag, *noBorderFlag)
			fmt.Fprintf(os.Stderr, "Screenshot: %s\n", *screenshotFlag)
		}
		if *saveSnapshotFlag != "" {
			formats.SaveSNA(*saveSnapshotFlag, machine)
			fmt.Fprintf(os.Stderr, "Snapshot: %s\n", *saveSnapshotFlag)
		}
		if saved > 0 {
			fmt.Fprintf(os.Stderr, "Saved %d frames\n", saved)
		}
		fmt.Fprintf(os.Stderr, "RZX replay done (%d/%d frames).\n", player.Frame, player.TotalFrames())
		return
	}

	fmt.Fprintf(os.Stderr, "MZX headless: %s, running %d frames\n", *modelFlag, frames)

	var prevScreen []byte
	saved := 0

	for i := 0; i < frames; i++ {
		machine.RunFrame()

		// SCR dump (raw 6912-byte VRAM)
		if *dumpSCR != "" {
			scr := make([]byte, 6912)
			for j := 0; j < 6912; j++ {
				scr[j] = machine.Memory.ReadScreen(uint16(j))
			}
			path := fmt.Sprintf("%s/frame_%04d.scr", *dumpSCR, i+1)
			os.WriteFile(path, scr, 0644)
		}

		// PNG dump
		if *dumpFrames != "" {
			path := fmt.Sprintf("%s/frame_%04d.png", *dumpFrames, i+1)
			saveScreenPNG(machine, path, *noBorderFlag)
			saved++
		}

		// Keyframe dump (only when screen changes)
		if *dumpKeyframes != "" {
			scr := make([]byte, 6912)
			for j := 0; j < 6912; j++ {
				scr[j] = machine.Memory.ReadScreen(uint16(j))
			}
			if prevScreen == nil || !bytesEqual(scr, prevScreen) {
				path := fmt.Sprintf("%s/frame_%04d.png", *dumpKeyframes, i+1)
				saveScreenPNG(machine, path, *noBorderFlag)
				saved++
				prevScreen = make([]byte, len(scr))
				copy(prevScreen, scr)
			}
		}
	}

	// Final screenshot
	if *screenshotFlag != "" {
		saveScreenPNG(machine, *screenshotFlag, *noBorderFlag)
		fmt.Fprintf(os.Stderr, "Screenshot: %s\n", *screenshotFlag)
	}

	// Save snapshot
	if *saveSnapshotFlag != "" {
		if err := formats.SaveSNA(*saveSnapshotFlag, machine); err != nil {
			log.Fatalf("Save SNA: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Snapshot: %s\n", *saveSnapshotFlag)
	}

	if saved > 0 {
		fmt.Fprintf(os.Stderr, "Saved %d frames\n", saved)
	}
	fmt.Fprintf(os.Stderr, "Done.\n")
}

func saveScreenPNG(m *spectrum.Machine, path string, noBorder bool) {
	w, h := 256, 192
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < 192; y++ {
		for x := 0; x < 256; x++ {
			col := x / 8
			row := y / 8
			attrAddr := 0x1800 + row*32 + col
			attr := m.Memory.ReadScreen(uint16(attrAddr))

			pixelY := y
			pixelAddr := uint16(((pixelY & 0xC0) << 5) | ((pixelY & 0x07) << 8) | ((pixelY & 0x38) << 2) | col)
			pixel := m.Memory.ReadScreen(pixelAddr)

			bit := 7 - (x % 8)
			isSet := (pixel>>bit)&1 != 0

			ink := attr & 7
			paper := (attr >> 3) & 7
			bright := (attr >> 6) & 1

			var c byte
			if isSet {
				c = ink
			} else {
				c = paper
			}
			img.Set(x, y, zxColor(c, bright))
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	png.Encode(f, img)
}

func zxColor(c, bright byte) color.RGBA {
	base := [8]color.RGBA{
		{0, 0, 0, 255},       // black
		{0, 0, 0xCD, 255},    // blue
		{0xCD, 0, 0, 255},    // red
		{0xCD, 0, 0xCD, 255}, // magenta
		{0, 0xCD, 0, 255},    // green
		{0, 0xCD, 0xCD, 255}, // cyan
		{0xCD, 0xCD, 0, 255}, // yellow
		{0xCD, 0xCD, 0xCD, 255}, // white
	}
	hi := [8]color.RGBA{
		{0, 0, 0, 255},
		{0, 0, 0xFF, 255},
		{0xFF, 0, 0, 255},
		{0xFF, 0, 0xFF, 255},
		{0, 0xFF, 0, 255},
		{0, 0xFF, 0xFF, 255},
		{0xFF, 0xFF, 0, 255},
		{0xFF, 0xFF, 0xFF, 255},
	}
	if bright != 0 {
		return hi[c&7]
	}
	return base[c&7]
}

func bytesEqual(a, b []byte) bool {
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

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// suppressStderr / restoreStderr stubs for headless
func suppressStderr() {}
func restoreStderr()  {}
