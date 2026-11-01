package pipeline

// ZX Spectrum narrow-font text engine: machine-level proof and T-state
// measurement for task Z3 (51/42/32 column windows from stdlib/zx/text.nanz).
//
// TestZXNarrowTextDemoAllWidths compiles examples/nanz/zx_narrow_text_demo.nanz,
// runs it on an in-process 48K machine with the real ROM, switches 51 -> 42 ->
// 32 columns with native key-matrix presses, and decodes the 6912-byte screen
// back to characters using the same packed fonts the Z80 builds — asserting the
// text cell for cell, including the right pane's wrap and window scroll, the
// attribute rectangles and the split divider.
//
// TestZXNarrowTextTiming measures the T-states of one glyph, one full row, the
// font build and a whole width switch, for both blit cores, using the
// bench program in examples/nanz/zx_narrow_text_bench.nanz.

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

const zxNarrowRepoRoot = "../../.."      // minzc/pkg/pipeline -> repo root
const zxNarrowROMPath = "../../cmd/mzx/roms/48.rom"

// ── the font model, shared with scripts/zx_narrow_font.py ───────────────────

// zxNarrowTables reads the generated deletion masks out of
// stdlib/zx/narrow_font_tables.nanz, so the decoder and the Z80 builder are
// driven by exactly the bytes that ship.
func zxNarrowTables(t *testing.T) map[int][2][96]byte {
	t.Helper()
	raw, err := os.ReadFile(zxNarrowRepoRoot + "/stdlib/zx/narrow_font_tables.nanz")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?s)global NF([56])_(TOP|BOT): \[u8; 96\] = \[(.*?)\]`)
	tables := map[int][2][96]byte{}
	for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
		width, _ := strconv.Atoi(m[1])
		var vals []int
		for _, field := range strings.FieldsFunc(m[3], func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == '\t' }) {
			v, err := strconv.ParseUint(strings.TrimSpace(field), 0, 8)
			if err != nil {
				t.Fatalf("bad mask byte %q: %v", field, err)
			}
			vals = append(vals, int(v))
		}
		if len(vals) != 96 {
			t.Fatalf("NF%d_%s: %d values, want 96", width, m[2], len(vals))
		}
		half := 0
		if m[2] == "BOT" {
			half = 1
		}
		tab := tables[width]
		for i, v := range vals {
			tab[half][i] = byte(v)
		}
		tables[width] = tab
	}
	if len(tables) != 2 {
		t.Fatalf("parsed %d widths from the tables module, want 2", len(tables))
	}
	return tables
}

// zxNarrowSqueeze drops the masked pixel columns and left-aligns the rest.
func zxNarrowSqueeze(row, mask byte) byte {
	out, kept := byte(0), 0
	for c := 0; c < 8; c++ {
		if mask&(0x80>>c) != 0 {
			continue
		}
		out = out<<1 | (row >> (7 - c) & 1)
		kept++
	}
	return out << (8 - kept)
}

// zxNarrowPacked returns the packed font for a width (the ROM font for 8).
func zxNarrowPacked(t *testing.T, width int) [96][8]byte {
	t.Helper()
	rom, err := os.ReadFile(zxNarrowROMPath)
	if err != nil {
		t.Fatal(err)
	}
	var out [96][8]byte
	for g := 0; g < 96; g++ {
		var top, bot byte
		if width != 8 {
			tab := zxNarrowTables(t)[width]
			top, bot = tab[0][g], tab[1][g]
		}
		for r := 0; r < 8; r++ {
			row := rom[0x3D00+g*8+r]
			mask := top
			if r >= 4 {
				mask = bot
			}
			if width == 8 {
				mask = 0
			}
			out[g][r] = zxNarrowSqueeze(row, mask)
		}
	}
	return out
}

func zxScrOffset(bcol, row, line int) int {
	return ((row & 0x18) << 8) + (line << 8) + ((row & 7) << 5) + bcol
}

// zxNarrowDecode picks the W-pixel cells out of a screen dump and names them.
func zxNarrowDecode(t *testing.T, scr []byte, xpx, y, cols, rows, width int, fonts map[int][96][8]byte) []string {
	t.Helper()
	out := make([]string, 0, rows)
	for r := 0; r < rows; r++ {
		var b strings.Builder
		for c := 0; c < cols; c++ {
			x := xpx + c*width
			s, bcol := x&7, x>>3
			var cell [8]byte
			for line := 0; line < 8; line++ {
				v0 := scr[zxScrOffset(bcol, y+r, line)]
				v1 := byte(0)
				if bcol < 31 {
					v1 = scr[zxScrOffset(bcol+1, y+r, line)]
				}
				g := byte(0)
				for j := 0; j < width; j++ {
					pos := s + j
					bit := 7 - pos
					if pos >= 8 {
						bit = 15 - pos
					}
					src := v0
					if pos >= 8 {
						src = v1
					}
					g = g<<1 | (src >> bit & 1)
				}
				cell[line] = g << (8 - width)
			}
			best := "?"
			for i, glyph := range fonts[width] {
				if glyph == cell {
					best = string(rune(32 + i))
					break
				}
			}
			b.WriteByte(best[0])
		}
		out = append(out, b.String())
	}
	return out
}

// ── compile + run helpers ───────────────────────────────────────────────────

func zxNarrowCompile(t *testing.T, path string) ([]byte, map[string]int) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), "zx-narrow-text", nanz.ParseOpts{StdlibDir: zxNarrowRepoRoot + "/stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	asm := "ORG 0x8000\n" + steps.Assembly
	result, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(result.Errors) > 0 {
		t.Fatal(err, result.Errors)
	}
	return result.Binary, result.Symbols
}

// Loads, runs until the CPU halts or maxFrames have passed, and presses keys at
// frame boundaries. The demo polls the matrix, so a key held for a few frames
// is one keypress.
func zxNarrowRunScreen(t *testing.T, bin []byte, base uint16, maxFrames int, keys func(frame int, k *spectrum.Keyboard), poke func(m *spectrum.Machine)) (scr []byte, tstates int64, halted bool) {
	t.Helper()
	rom, err := os.ReadFile(zxNarrowROMPath)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := spectrum.New48K(rom)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range bin {
		machine.Memory.Write(base+uint16(i), b, false)
	}
	machine.CPU.SetPC(base)
	machine.CPU.SetSP(0xFF58)
	machine.CPU.SetIM(1)
	machine.CPU.SetIFF1(false)
	machine.CPU.SetIFF2(false)
	if poke != nil {
		poke(machine)
	}
	for f := 0; f < maxFrames; f++ {
		if keys != nil {
			keys(f, machine.Keyboard)
		}
		machine.RunFrameFast()
		if machine.CPU.Halted() {
			return machine.ReadVRAM(), machine.AbsoluteTStates(), true
		}
	}
	return machine.ReadVRAM(), machine.AbsoluteTStates(), false
}

// ── the demo ────────────────────────────────────────────────────────────────

var zxNarrowSentenceRe = regexp.MustCompile(`if i == (\d+) \{ return c"([^"]*)" \}`)
var zxNarrowReturnRe = regexp.MustCompile(`return c"([^"]*)"`)
var zxNarrowPrintRe = regexp.MustCompile(`win_print\(c"([^"]*)"\)`)

func zxNarrowDemoText(t *testing.T) (sentences []string, paragraph string) {
	t.Helper()
	src, err := os.ReadFile(zxNarrowRepoRoot + "/examples/nanz/zx_narrow_text_demo.nanz")
	if err != nil {
		t.Fatal(err)
	}
	body := regexp.MustCompile(`(?s)fun sentence\(i: u8\) -> \^u8 \{(.*?)\n\}`).FindStringSubmatch(string(src))[1]
	byIndex := map[int]string{}
	for _, m := range zxNarrowSentenceRe.FindAllStringSubmatch(body, -1) {
		n, _ := strconv.Atoi(m[1])
		byIndex[n] = m[2]
	}
	tail := strings.Split(body, "if i ==")
	fallback := zxNarrowReturnRe.FindAllStringSubmatch(tail[len(tail)-1], -1)
	sentencesFallback := ""
	if len(fallback) > 0 {
		sentencesFallback = fallback[len(fallback)-1][1]
	}
	for i := 0; i < 24; i++ {
		if s, ok := byIndex[i]; ok {
			sentences = append(sentences, s)
		} else {
			sentences = append(sentences, sentencesFallback)
		}
	}
	paragraph = zxNarrowPrintRe.FindStringSubmatch(string(src))[1]
	return sentences, paragraph
}

// The same arithmetic split_vertical performs on the Z80.
func zxNarrowGeometry(width, cols, leftW int) (leftX, leftCols, rightX, rightCols, divPx int) {
	leftEnd := leftW * width
	divEnd := leftEnd + 8
	if leftEnd%8 != 0 {
		divEnd = (leftEnd + 7) &^ 7
	}
	rightX, px := 0, 0
	for px < divEnd {
		px += width
		rightX++
	}
	return 0, leftW, rightX * width, cols - rightX, leftEnd + (divEnd-leftEnd)>>1
}

func TestZXNarrowTextDemoAllWidths(t *testing.T) {
	bin, _ := zxNarrowCompile(t, zxNarrowRepoRoot+"/examples/nanz/zx_narrow_text_demo.nanz")
	fonts := map[int][96][8]byte{5: zxNarrowPacked(t, 5), 6: zxNarrowPacked(t, 6), 8: zxNarrowPacked(t, 8)}
	sentences, paragraph := zxNarrowDemoText(t)

	cases := []struct {
		width, cols, leftW int
		keyRow, keyBit     int // the key that switches to this width
		keyFrame           int
		frames             int
	}{
		{5, 51, 24, -1, -1, 0, 900},
		{6, 42, 16, 3, 1, 500, 1600},
		{8, 32, 16, 3, 2, 500, 1600},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%dpx", tc.width), func(t *testing.T) {
			keys := func(frame int, k *spectrum.Keyboard) {
				if tc.keyRow < 0 {
					return
				}
				if frame == tc.keyFrame {
					k.KeyPress(tc.keyRow, tc.keyBit)
				}
				if frame == tc.keyFrame+20 {
					k.KeyRelease(tc.keyRow, tc.keyBit)
				}
			}
			scr, _, _ := zxNarrowRunScreen(t, bin, 0x8000, tc.frames, keys, nil)
			run := scr
			leftX, leftCols, rightX, rightCols, divPx := zxNarrowGeometry(tc.width, tc.cols, tc.leftW)

			// left pane: "NN " + the sentence padded into the rest of the row
			got := zxNarrowDecode(t, run, leftX, 0, leftCols, 24, tc.width, fonts)
			for r := 0; r < 24; r++ {
				field := leftCols - 4
				want := fmt.Sprintf("%02d %s", r, sentences[r][:min(field, len(sentences[r]))])
				want += strings.Repeat(" ", leftCols-len(want))
				if got[r] != want {
					t.Errorf("left pane row %d:\n got %q\nwant %q", r, got[r], want)
				}
			}

			// right pane: the paragraph, wrapped and scrolled by win_print
			got = zxNarrowDecode(t, run, rightX, 0, rightCols, 24, tc.width, fonts)
			screen, col, row := [][]string{}, 0, 0
			for _, ch := range paragraph {
				if row == len(screen) {
					screen = append(screen, make([]string, rightCols))
					for i := range screen[len(screen)-1] {
						screen[len(screen)-1][i] = " "
					}
				}
				screen[row][col] = string(ch)
				col++
				if col >= rightCols {
					col, row = 0, row+1
					if row >= 24 {
						screen = screen[1:]
						blank := make([]string, rightCols)
						for i := range blank {
							blank[i] = " "
						}
						screen = append(screen, blank)
						row = 23
					}
				}
			}
			for len(screen) < 24 {
				blank := make([]string, rightCols)
				for i := range blank {
					blank[i] = " "
				}
				screen = append(screen, blank)
			}
			for r := 0; r < 24; r++ {
				want := strings.Join(screen[r], "")
				if got[r] != want {
					t.Errorf("right pane row %d:\n got %q\nwant %q", r, got[r], want)
				}
			}

			// attributes: left pane black on cyan bright with the last row
			// inverted, right pane white on blue
			attr := func(bcol, row int) int { return int(run[6144+row*32+bcol]) }
			lb0, lb1 := leftX>>3, (leftX+leftCols*tc.width-1)>>3
			rb0, rb1 := rightX>>3, (rightX+rightCols*tc.width-1)>>3
			for r := 0; r < 24; r++ {
				for b := lb0; b <= lb1; b++ {
					want := 5<<3 | 0x40
					if r == 23 {
						want = 0<<3 | 5 | 0x40
					}
					if attr(b, r) != want {
						t.Errorf("left attr row %d col %d: %#04x want %#04x", r, b, attr(b, r), want)
					}
				}
				for b := rb0; b <= rb1; b++ {
					if want := 1<<3 | 7; attr(b, r) != want {
						t.Errorf("right attr row %d col %d: %#04x want %#04x", r, b, attr(b, r), want)
					}
				}
			}

			// the divider is a one-pixel rule through all 24 character rows
			byte, bit := divPx>>3, byte(0x80>>(divPx&7))
			for r := 0; r < 24; r++ {
				for line := 0; line < 8; line++ {
					if run[zxScrOffset(byte, r, line)]&bit == 0 {
						t.Errorf("divider missing at char row %d pixel line %d", r, line)
					}
				}
			}
		})
	}
}

// ── T-states ────────────────────────────────────────────────────────────────

func TestZXNarrowTextTiming(t *testing.T) {
	bin, _ := zxNarrowCompile(t, zxNarrowRepoRoot+"/examples/nanz/zx_narrow_text_bench.nanz")
	// The bench reads (iterations, phase, width, core) from $5B00-$5B03, so one
	// build covers every measurement and nothing is constant-folded away.
	const N = 40
	// RunFrameFast always finishes the current frame, which quantises
	// AbsoluteTStates to whole 69888 T frames. For the bench the CPU is stepped
	// directly to its HALT instead, so the reading is exact.
	stepped := func(n, sel, width, core byte) int64 {
		rom, err := os.ReadFile(zxNarrowROMPath)
		if err != nil {
			t.Fatal(err)
		}
		machine, err := spectrum.New48K(rom)
		if err != nil {
			t.Fatal(err)
		}
		for i, b := range bin {
			machine.Memory.Write(0x8000+uint16(i), b, false)
		}
		machine.CPU.SetPC(0x8000)
		machine.CPU.SetSP(0xFF58)
		machine.CPU.SetIM(1)
		machine.CPU.SetIFF1(false)
		machine.CPU.SetIFF2(false)
		machine.Memory.Write(0x5B00, n, false)
		machine.Memory.Write(0x5B01, sel, false)
		machine.Memory.Write(0x5B02, width, false)
		machine.Memory.Write(0x5B03, core, false)
		for !machine.CPU.Halted() && machine.AbsoluteTStates() < 400_000_000 {
			machine.CPU.DoOpcode()
		}
		if !machine.CPU.Halted() {
			t.Fatalf("bench n=%d sel=%d width=%d core=%d never halted", n, sel, width, core)
		}
		return machine.AbsoluteTStates()
	}
	measure := func(sel, width, core byte) int64 {
		return (stepped(N, sel, width, core) - stepped(0, sel, width, core)) / N
	}
	for _, width := range []byte{5, 6, 8} {
		cols := map[byte]int{5: 51, 6: 42, 8: 32}[width]
		for _, core := range []byte{0, 1} {
			glyph := measure(2, width, core)
			straddle := measure(7, width, core)
			goto_ := measure(6, width, core)
			putch := measure(4, width, core)
			row := measure(3, width, core)
			t.Logf("width=%d cols=%d core=%d: tx_blit(aligned)=%dT tx_blit(straddling)=%dT win_goto=%dT win_putch=%dT row=%dT (%.0f T/glyph)",
				width, cols, core, glyph, straddle, goto_, putch, row, float64(row)/float64(cols))
		}
		t.Logf("width=%d: narrow_font_build=%dT text_set_width=%dT", width,
			measure(1, width, 0), measure(5, width, 0))
	}
}


