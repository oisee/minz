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
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/spectrum/rzx"
)

//go:embed roms/48.rom
var embedded48ROM []byte

func main() {
	demoFlag := flag.String("demo", "", "Launch an embedded demo")
	demoList := flag.Bool("list-demos", false, "List embedded demos")
	demoRoot := flag.String("data-dir", "", "Persistent demo data directory")
	modelFlag := flag.String("model", "48k", "Machine profile: 48k, 128k, pentagon, scorpion-memory")
	romFlag := flag.String("rom", "", "Path to ROM file")
	snapshotFlag := flag.String("snapshot", "", "Path to .sna snapshot file")
	trdFlag := flag.String("trd", "", "Attach a TR-DOS image through emulator API traps")
	trdWriteback := flag.Bool("trd-writeback", false, "persist disk writes to --trd image")
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
	if *demoList {
		listDemos()
		return
	}

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
	case "128k", "pentagon", "scorpion-memory":
		romData := embedded48ROM
		if *romFlag != "" {
			romData, err = os.ReadFile(*romFlag)
			if err != nil {
				log.Fatalf("ROM: %v", err)
			}
		}
		// Match the graphical CLI's fallback: the 48K ROM supplies font
		// data in both slots unless a 32K ROM image is supplied.
		rom0, rom1 := romData, romData
		if len(romData) == 32768 {
			rom0, rom1 = romData[:16384], romData[16384:]
		}
		if *modelFlag == "scorpion-memory" {
			machine, err = spectrum.NewScorpionMemory256(rom0, rom1)
		} else {
			machine, err = spectrum.NewPentagon128(rom0, rom1)
		}
	default:
		log.Fatalf("unknown machine model %s (48k, 128k, pentagon, scorpion-memory)", *modelFlag)
	}
	if err != nil {
		log.Fatalf("Machine init: %v", err)
	}

	if *trdWriteback && *trdFlag == "" {
		log.Fatal("--trd-writeback requires --trd")
	}
	if *modelFlag == "scorpion-memory" && *saveSnapshotFlag != "" {
		log.Fatal("SNA cannot preserve Scorpion extended banks; extended snapshot format not implemented")
	}
	if *trdFlag != "" {
		disk, err := formats.LoadTRD(*trdFlag)
		if err != nil {
			log.Fatalf("TRD: %v", err)
		}
		disk.Writeback = *trdWriteback
		formats.InstallTRDTraps(machine, disk, false)
	}

	// Load snapshot
	if *snapshotFlag != "" {
		snap, err := formats.LoadSNA(*snapshotFlag)
		if err != nil {
			log.Fatalf("SNA: %v", err)
		}
		formats.ApplySnapshot(machine, snap)
	}

	demoSession := ""
	if *demoFlag != "" {
		machine, demoSession, err = loadDemo(*demoFlag, *demoRoot, embedded48ROM)
		d, _ := findDemo(*demoFlag)
		*modelFlag = d.Model
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := saveDemoSession(machine, demoSession); err != nil {
				log.Printf("Session save: %v", err)
			}
		}()
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
		addressParts := strings.SplitN(parts[1], ":", 2)
		addr, err := strconv.ParseUint(strings.TrimPrefix(addressParts[0], "0x"), 16, 16)
		if err != nil {
			log.Fatalf("--load: bad address %q", addressParts[0])
		}
		page := -1
		if len(addressParts) == 2 {
			page, err = strconv.Atoi(addressParts[1])
			if err != nil || page < 0 || page > 15 {
				log.Fatal("--load page must be 0..15")
			}
			if page > 7 && !machine.Memory.ScorpionPagingEnabled() {
				log.Fatal("extended RAM load requires scorpion-memory")
			}
			if int(addr&0x3fff)+len(data) > 16384 {
				log.Fatal("--load crosses physical bank boundary")
			}
		}
		for j, b := range data {
			if page >= 0 {
				machine.Memory.WriteRAMDirect(page, uint16(addr&0x3fff)+uint16(j), b)
			} else {
				machine.Memory.Write(uint16(addr)+uint16(j), b, false)
			}
		}
		fmt.Fprintf(os.Stderr, "Loaded %d bytes at $%04X page %d\n", len(data), addr, page)
	}

	// --set registers
	if *setFlag != "" {
		for _, kv := range strings.Split(*setFlag, ",") {
			kv = strings.TrimSpace(kv)
			upper := strings.ToUpper(kv)
			if upper == "DI" {
				machine.CPU.SetIFF1(false)
				machine.CPU.SetIFF2(false)
				continue
			}
			if upper == "EI" {
				machine.CPU.SetIFF1(true)
				machine.CPU.SetIFF2(true)
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
			if dir := filepath.Dir(*saveSnapshotFlag); dir != "" && dir != "." {
				os.MkdirAll(dir, 0755)
			}
			if err := formats.SaveSNA(*saveSnapshotFlag, machine); err != nil {
				log.Fatalf("Save SNA: %v", err)
			}
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
		if dir := filepath.Dir(*saveSnapshotFlag); dir != "" && dir != "." {
			os.MkdirAll(dir, 0755)
		}
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
	// Preserve the pixels fetched by the ULA during execution, including
	// border timing and partially updated screen rows. Reading final VRAM
	// here would hide tearing and discard diagnostic border stripes.
	w, h := m.ScreenWidth(), m.ScreenHeight()
	x0, y0 := 0, 0
	if noBorder {
		w, h = 256, 192
		x0, y0 = m.Mode.BorderLeft, m.Mode.BorderTop
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fb := m.Framebuffer()
	for y := 0; y < h; y++ {
		start := ((y0+y)*m.ScreenWidth() + x0) * 4
		copy(img.Pix[y*img.Stride:y*img.Stride+w*4], fb[start:start+w*4])
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[mzx] screenshot: %v\n", err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintf(os.Stderr, "[mzx] png encode: %v\n", err)
	}
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
