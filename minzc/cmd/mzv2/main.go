// mzv2 — MIR2 VM runner with TUI display for ZX Spectrum programs.
//
// Compiles Nanz source through HIR→MIR2 (stopping before Z80 codegen),
// then executes on the MIR2 VM with host-function overrides for ZX Spectrum
// primitives. Renders the attribute screen as a 32×24 ANSI color grid.
//
// Usage:
//
//	mzv2 program.nanz
//	mzv2 -trace program.nanz
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"

	"golang.org/x/term"
)

func main() {
	trace := flag.Bool("trace", false, "print each VM call")
	headless := flag.Bool("headless", false, "run without terminal (testing)")
	maxFrames := flag.Int("max-frames", 0, "stop after N frames (0=unlimited)")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: mzv2 [-trace] program.nanz")
		os.Exit(1)
	}
	srcPath := flag.Arg(0)

	src, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mzv2: %v\n", err)
		os.Exit(1)
	}

	// ── Compile: Nanz → HIR → MIR2 (optimised, no regalloc) ──────────────

	baseDir := filepath.Dir(srcPath)
	stdlibDir := findStdlib(srcPath)

	hm, err := nanz.ParseWithOpts(string(src), filepath.Base(srcPath), nanz.ParseOpts{
		BaseDir:   baseDir,
		StdlibDir: stdlibDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mzv2: parse error: %v\n", err)
		os.Exit(1)
	}

	m := hir.LowerModule(hm)

	// Per-function optimisation passes (same as pipeline.go, no regalloc).
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			s := mir2.SimplifyIdentities(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !s && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
		if mir2.BranchEquiv(m, f) {
			mir2.EliminateDeadBlocks(f)
			mir2.DeadStoreElim(f)
		}
		if mir2.CondRetSink(f) {
			mir2.EliminateDeadBlocks(f)
		}
	}

	if err := mir2.Verify(m); err != nil {
		fmt.Fprintf(os.Stderr, "mzv2: MIR2 verify: %v\n", err)
		os.Exit(1)
	}

	// Phase 6f: inline trivial functions.
	if mir2.InlineTrivial(m, 4) {
		for _, f := range m.Funcs {
			mir2.PropagateCopies(f)
			mir2.DeadStoreElim(f)
		}
	}

	fmt.Fprintf(os.Stderr, "mzv2: compiled %d functions, %d globals\n", len(m.Funcs), len(m.Globals))

	// ── VM setup ─────────────────────────────────────────────────────────

	vm := mir2.NewVM(m)
	vm.MaxSteps = 0 // unlimited — game loop runs forever
	vm.MaxMemory = 1 << 20

	// ZX Spectrum 64K address space (separate from VM heap).
	var zxMem [65536]byte

	// Keyboard state: row high byte → composed row byte.
	var keyMu sync.Mutex
	keyState := make(map[byte]byte) // row → bits (0 = pressed)

	// 50Hz tick for HALT timing.
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	traceEnabled := *trace

	// ── Host functions ───────────────────────────────────────────────────

	vm.Hosts["zx_poke"] = func(args []mir2.Value) ([]mir2.Value, error) {
		addr := uint16(args[0].I)
		val := byte(args[1].I)
		zxMem[addr] = val
		if traceEnabled {
			fmt.Fprintf(os.Stderr, "  zx_poke(0x%04X, 0x%02X)\n", addr, val)
		}
		return nil, nil
	}

	vm.Hosts["zx_peek"] = func(args []mir2.Value) ([]mir2.Value, error) {
		addr := uint16(args[0].I)
		return []mir2.Value{{I: int64(zxMem[addr])}}, nil
	}

	vm.Hosts["zx_key_row"] = func(args []mir2.Value) ([]mir2.Value, error) {
		high := byte(args[0].I)
		keyMu.Lock()
		row := keyState[high]
		keyMu.Unlock()
		// ZX Spectrum: 0 = pressed, so default (no keys) = 0xFF
		return []mir2.Value{{I: int64(row ^ 0xFF)}}, nil
	}

	frameCount := 0
	maxF := *maxFrames

	vm.Hosts["zx_halt"] = func(_ []mir2.Value) ([]mir2.Value, error) {
		// Clear key state FIRST (end of previous frame's input window),
		// then wait for tick. Game reads keys AFTER halt returns, so new
		// keypresses arriving during the tick wait will be picked up.
		keyMu.Lock()
		for k := range keyState {
			delete(keyState, k)
		}
		keyMu.Unlock()
		if !*headless {
			<-ticker.C
			renderFrame(&zxMem)
		}
		frameCount++
		if maxF > 0 && frameCount >= maxF {
			return nil, fmt.Errorf("reached %d frames, stopping", maxF)
		}
		return nil, nil
	}

	vm.Hosts["zx_border"] = func(args []mir2.Value) ([]mir2.Value, error) {
		// Cosmetic — ignore for TUI.
		return nil, nil
	}

	// zx_attr_addr is pure math — let the VM execute the Nanz body.
	// zx_screen_addr likewise.

	// ── Terminal raw mode + input goroutine ──────────────────────────────

	var oldState *term.State
	if !*headless {
		oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "mzv2: failed to set raw terminal: %v\n", err)
			os.Exit(1)
		}
		defer term.Restore(int(os.Stdin.Fd()), oldState)

		// Handle signals to restore terminal.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			term.Restore(int(os.Stdin.Fd()), oldState)
			fmt.Print("\033[?25h") // show cursor
			os.Exit(0)
		}()

		// Clear screen + hide cursor.
		fmt.Print("\033[2J\033[H\033[?25l")

		// Input reader goroutine.
		go readInput(&keyMu, keyState, oldState)
	}

	// ── Run ──────────────────────────────────────────────────────────────

	_, err = vm.Call("main", nil)
	fmt.Fprintf(os.Stderr, "mzv2: exited after %d frames\n", frameCount)

	// In headless mode, render final frame to stdout + dump summary to stderr.
	if *headless {
		renderFrame(&zxMem)

		nonZero := 0
		for i := 0x5800; i < 0x5B00; i++ {
			if zxMem[i] != 0 {
				nonZero++
			}
		}
		fmt.Fprintf(os.Stderr, "mzv2: attr cells with color: %d/768\n", nonZero)
	}
	if oldState != nil {
		term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Print("\033[?25h")
	}
	if err != nil {
		// max-frames exit is not an error
		if maxF > 0 && frameCount >= maxF {
			fmt.Fprintf(os.Stderr, "mzv2: stopped after %d frames\n", frameCount)
		} else {
			fmt.Fprintf(os.Stderr, "\nmzv2: VM error: %v\n", err)
			os.Exit(1)
		}
	}
}

// ── TUI renderer ────────────────────────────────────────────────────────────

// ZX Spectrum color → ANSI color code.
// ZX: 0=black, 1=blue, 2=red, 3=magenta, 4=green, 5=cyan, 6=yellow, 7=white
// Foreground (30-37), Background (40-47), bright +60.
var zxToANSIFg = [8]int{30, 34, 31, 35, 32, 36, 33, 37}
var zxToANSIBg = [8]int{40, 44, 41, 45, 42, 46, 43, 47}

func renderFrame(zxMem *[65536]byte) {
	var buf []byte
	buf = append(buf, "\033[H"...) // cursor home

	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			attr := zxMem[0x5800+y*32+x]
			ink := attr & 7
			paper := (attr >> 3) & 7
			bright := (attr >> 6) & 1

			// Try OCR: match pixel data against ZX ROM font
			pixels := readCellPixels(zxMem, x, y)
			ch, inverse := ocrCell(pixels)

			if ch > 0x20 { // recognized printable character (not space)
				// Determine foreground/background from ink/paper + inverse
				fg, bg := ink, paper
				if inverse {
					fg, bg = paper, ink
				}
				fgCode := zxToANSIFg[fg]
				bgCode := zxToANSIBg[bg]
				if bright != 0 {
					fgCode += 60
					bgCode += 60
				}
				buf = append(buf, fmt.Sprintf("\033[%d;%dm%c \033[0m", fgCode, bgCode, ch)...)
			} else if attr == 0 && pixels == [8]byte{} {
				// Empty cell — no color, no pixels
				buf = append(buf, ' ', ' ')
			} else {
				// No text match — render as solid color block
				bgCode := zxToANSIBg[paper]
				if bright != 0 {
					bgCode += 60
				}
				if attr == 0 {
					buf = append(buf, ' ', ' ')
				} else {
					buf = append(buf, fmt.Sprintf("\033[%dm  \033[0m", bgCode)...)
				}
			}
		}
		buf = append(buf, '\r', '\n')
	}

	os.Stdout.Write(buf)
}

// ── Input ───────────────────────────────────────────────────────────────────

func readInput(mu *sync.Mutex, keyState map[byte]byte, oldState *term.State) {
	buf := make([]byte, 8)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}

		mu.Lock()
		// Only SET bits here — clearing happens at frame boundary (zx_halt).
		for i := 0; i < n; i++ {
			b := buf[i]

			// Check for escape sequences (arrow keys).
			if b == 0x1B && i+2 < n && buf[i+1] == '[' {
				switch buf[i+2] {
				case 'D': // Left arrow → same as 'o'
					keyState[0xDF] |= 0x02
				case 'C': // Right arrow → same as 'p'
					keyState[0xDF] |= 0x01
				case 'A': // Up arrow → same as 'q' (rotate)
					keyState[0xFB] |= 0x01
				case 'B': // Down arrow → same as 'a' (soft drop)
					keyState[0xFD] |= 0x01
				}
				i += 2
				continue
			}

			switch b {
			case 3: // Ctrl-C
				term.Restore(int(os.Stdin.Fd()), oldState)
				fmt.Print("\033[?25h")
				os.Exit(0)
			case 27: // Esc
				term.Restore(int(os.Stdin.Fd()), oldState)
				fmt.Print("\033[?25h")
				os.Exit(0)
			case 'o', 'O':
				keyState[0xDF] |= 0x02 // row 0xDF bit 1
			case 'p', 'P':
				keyState[0xDF] |= 0x01 // row 0xDF bit 0
			case 'q', 'Q':
				keyState[0xFB] |= 0x01 // row 0xFB bit 0
			case 'a', 'A':
				keyState[0xFD] |= 0x01 // row 0xFD bit 0
			case ' ':
				keyState[0x7F] |= 0x01 // row 0x7F bit 0
			case 'h', 'H':
				keyState[0xBF] |= 0x10 // row 0xBF bit 4
			}
		}
		mu.Unlock()
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// findStdlib locates the stdlib/ directory relative to the source file.
func findStdlib(srcPath string) string {
	// Walk up from source dir looking for stdlib/
	dir, _ := filepath.Abs(filepath.Dir(srcPath))
	for {
		candidate := filepath.Join(dir, "stdlib")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
