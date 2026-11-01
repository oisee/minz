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

const zxNarrowRepoRoot = "../../.." // minzc/pkg/pipeline -> repo root
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

// zxNarrowBlit renders one glyph exactly as tx_blit is specified to: OR the
// cell's pixels in under the keep masks for the bytes it touches.
func zxNarrowBlit(scr []byte, ch, xpx, row, width int, font [96][8]byte) {
	var glyph [8]byte
	if ch >= 32 && ch < 128 {
		glyph = font[ch-32]
	}
	b, s := xpx>>3, xpx&7
	var f0, f1 int
	if s+width <= 8 {
		f0 = ((1 << width) - 1) << (8 - width - s)
	} else {
		f0, f1 = 0xFF>>s, (0xFF<<(16-s-width))&0xFF
	}
	m0, m1 := f0^0xFF, f1^0xFF
	for line := 0; line < 8; line++ {
		v := glyph[line]
		lo := int(v >> s)
		hi := 0
		if s > 0 {
			hi = int((v << (8 - s)) & 0xFF)
		}
		off := zxScrOffset(b, row, line)
		scr[off] = (scr[off] & byte(m0)) | byte(lo)
		if m1 != 0 {
			off1 := zxScrOffset(b+1, row, line)
			scr[off1] = (scr[off1] & byte(m1)) | byte(hi)
		}
	}
}

// zxNarrowWrapped mirrors win_print's cursor advance: one cell per character,
// wrap at the window edge, scroll when the last row wraps.
func zxNarrowWrapped(text string, cols int) []string {
	var rows [][]byte
	for _, ch := range []byte(text) {
		if len(rows) == 0 {
			rows = append(rows, make([]byte, 0, cols))
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], ch)
		if len(rows[len(rows)-1]) == cols {
			rows = append(rows, make([]byte, 0, cols))
			if len(rows) > 24 {
				rows = rows[1:]
			}
		}
	}
	for len(rows) < 24 {
		rows = append(rows, make([]byte, 0, cols))
	}
	out := make([]string, 24)
	for i, r := range rows {
		out[i] = string(r)
	}
	return out
}

const (
	zxAttrL    = 5<<3 | 0x40     // win_attr(0, 5, 1): black on cyan, bright
	zxAttrLInv = 0<<3 | 5 | 0x40 // the inverted cursor row
	zxAttrR    = 1<<3 | 7        // win_attr(7, 1, 0): white on blue
)

// zxNarrowExpectedScreen renders the 6912 bytes the demo's screen should hold
// at a given width, from the geometry and the engine's documented rules only.
func zxNarrowExpectedScreen(t *testing.T, width int, sentences []string, paragraph string, fonts map[int][96][8]byte) []byte {
	t.Helper()
	cols := map[int]int{5: 51, 6: 42, 8: 32}[width]
	leftW := 24
	if width != 5 {
		leftW = 16
	}
	leftX, leftCols, rightPx, rightCols, divPx := zxNarrowGeometry(width, cols, leftW)
	scr := make([]byte, 6912)

	for i := 0; i < 24; i++ {
		field := leftCols - 4
		text := fmt.Sprintf("%02d %-*s", i, field, sentences[i][:min(field, len(sentences[i]))])
		for c := 0; c < len(text); c++ {
			zxNarrowBlit(scr, int(text[c]), leftX*width+c*width, i, width, fonts[width])
		}
	}
	for r, line := range zxNarrowWrapped(paragraph, rightCols) {
		for c := 0; c < len(line); c++ {
			zxNarrowBlit(scr, int(line[c]), rightPx+c*width, r, width, fonts[width])
		}
	}
	divByte, divBit := divPx>>3, byte(0x80>>(divPx&7))
	for row := 0; row < 24; row++ {
		for line := 0; line < 8; line++ {
			scr[zxScrOffset(divByte, row, line)] |= divBit
		}
	}
	leftEnd := leftX*width + leftCols*width - 1
	divStart, divEnd := leftW*width, leftW*width+8
	for b := divStart >> 3; b <= (divEnd-1)>>3; b++ {
		for row := 0; row < 24; row++ {
			scr[6144+row*32+b] = zxAttrL
		}
	}
	for row := 0; row < 24; row++ {
		for b := leftX * width >> 3; b <= leftEnd>>3; b++ {
			scr[6144+row*32+b] = zxAttrL
			if row == 23 {
				scr[6144+row*32+b] = zxAttrLInv
			}
		}
		for b := rightPx >> 3; b <= (rightPx+rightCols*width-1)>>3; b++ {
			scr[6144+row*32+b] = zxAttrR
		}
	}
	return scr
}

// TestZXNarrowTextDemoScreens runs the demo on an in-process 48K machine with
// the real ROM, switches widths with native key-matrix presses and compares the
// whole 6912-byte screen — all 6144 bitmap bytes and all 768 attribute bytes,
// including everything outside the windows — against the model above, for the
// startup screen and for every width transition. Round 1 shipped a
// tx_clear_screen that cleared 8 of 192 pixel lines; this test fails on that
// code because the previous layout's ink survives between the panes.
func TestZXNarrowTextDemoScreens(t *testing.T) {
	bin, _ := zxNarrowCompile(t, zxNarrowRepoRoot+"/examples/nanz/zx_narrow_text_demo.nanz")
	fonts := map[int][96][8]byte{5: zxNarrowPacked(t, 5), 6: zxNarrowPacked(t, 6), 8: zxNarrowPacked(t, 8)}
	sentences, paragraph := zxNarrowDemoText(t)

	// key: 1/2/3 select 51/42/32 columns (matrix row 3, bits 0/1/2)
	cases := []struct {
		name   string
		width  int
		keys   [][3]int // frame, row, bit
		frames int
	}{
		{"w5-startup", 5, nil, 250},
		{"w6-startup", 6, [][3]int{{200, 3, 1}}, 750},
		{"w8-startup", 8, [][3]int{{200, 3, 2}}, 750},
		{"5to6", 6, [][3]int{{200, 3, 1}}, 750},
		{"5to8", 8, [][3]int{{200, 3, 2}}, 750},
		{"6to8", 8, [][3]int{{200, 3, 1}, {900, 3, 2}}, 1600},
		{"8to6", 6, [][3]int{{200, 3, 2}, {900, 3, 1}}, 1600},
		{"6to5", 5, [][3]int{{200, 3, 1}, {900, 3, 0}}, 1600},
		{"8to5", 5, [][3]int{{200, 3, 2}, {900, 3, 0}}, 1600},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			held := map[[2]int]bool{}
			keys := func(frame int, k *spectrum.Keyboard) {
				for _, ev := range tc.keys {
					id := [2]int{ev[1], ev[2]}
					if frame == ev[0] {
						k.KeyPress(ev[1], ev[2])
						held[id] = true
					}
					// hold long enough to span the redraw the key triggers
					if frame == ev[0]+80 && held[id] {
						k.KeyRelease(ev[1], ev[2])
						held[id] = false
					}
				}
			}
			scr, _, _ := zxNarrowRunScreen(t, bin, 0x8000, tc.frames, keys, nil)
			want := zxNarrowExpectedScreen(t, tc.width, sentences, paragraph, fonts)
			var in, out, attr int
			var firstOut, firstAttr string
			cols := map[int]int{5: 51, 6: 42, 8: 32}[tc.width]
			leftW := 24
			if tc.width != 5 {
				leftW = 16
			}
			// zxNarrowGeometry's third result is already the right pane's
			// pixel origin (right cells * width), so do not multiply again;
			// the cell count comes back as the fourth result.
			_, _, rightPx, rightCols, _ := zxNarrowGeometry(tc.width, cols, leftW)
			lb0, lb1 := 0, (leftW*tc.width-1)>>3
			rb0, rb1 := rightPx>>3, (rightPx+rightCols*tc.width-1)>>3
			for row := 0; row < 24; row++ {
				for line := 0; line < 8; line++ {
					for b := 0; b < 32; b++ {
						off := zxScrOffset(b, row, line)
						if scr[off] == want[off] {
							continue
						}
						if (b >= lb0 && b <= lb1) || (b >= rb0 && b <= rb1) {
							in++
						} else {
							out++
							if firstOut == "" {
								firstOut = fmt.Sprintf("byte col %d char row %d line %d: got %#02x want %#02x",
									b, row, line, scr[off], want[off])
							}
						}
					}
				}
				for b := 0; b < 32; b++ {
					off := 6144 + row*32 + b
					if scr[off] != want[off] {
						attr++
						if firstAttr == "" {
							firstAttr = fmt.Sprintf("attr col %d row %d: got %#02x want %#02x", b, row, scr[off], want[off])
						}
					}
				}
			}
			if in+out+attr != 0 {
				t.Errorf("screen differs from the model: %d bitmap bytes inside the panes, %d outside, %d attribute cells\n  first outside: %s\n  first attribute: %s",
					in, out, attr, firstOut, firstAttr)
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
	// The values published in reports/zx-narrow-text/REPORT.md, so a performance regression fails CI
	// instead of only printing. Tolerance 3%: the emulator is deterministic
	// (two runs give identical numbers), so this only trips on real changes.
	want := map[string]int64{
		"blit5": 835, "blit6": 1099, "goto": 562, "putch0": 1960, "putch1": 2812,
		"row5c0": 98167, "row5c1": 182899, "row6c0": 81189, "row6c1": 146749,
		"row8c0": 57838, "row8c1": 85102,
		"scroll5c0": 311292, "scroll6c0": 311292, "scroll8c0": 267330,
		"build5": 1595698, "build6": 1584165, "build8": 43621,
		"setw5": 1857070, "setw6": 1845490, "setw8": 304948,
	}
	near := func(what string, got, want int64) {
		tol := want / 33
		if got < want-tol || got > want+tol {
			t.Errorf("%s: %d T, want %d +/- 3%% (reports/zx-narrow-text/REPORT.md)", what, got, want)
		}
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
			if width == 5 || width == 6 {
				near(fmt.Sprintf("width %d tx_blit(aligned)", width), glyph, want["blit5"])
				near(fmt.Sprintf("width %d tx_blit(straddling)", width), straddle, want["blit6"])
			} else {
				near("width 8 tx_blit(aligned)", glyph, want["blit5"])
				near("width 8 tx_blit(straddling)", straddle, 828)
			}
			near("win_goto", goto_, want["goto"])
			near(fmt.Sprintf("width %d core %d win_putch", width, core), putch, want[fmt.Sprintf("putch%d", core)])
			scroll := measure(8, width, core)
			t.Logf("width=%d core=%d: win_scroll=%dT", width, core, scroll)
			near(fmt.Sprintf("width %d core %d win_scroll", width, core), scroll, want[fmt.Sprintf("scroll%dc0", width)])
			near(fmt.Sprintf("width %d core %d row", width, core), row, want[fmt.Sprintf("row%dc%d", width, core)])
		}
		build := measure(1, width, 0)
		sw := measure(5, width, 0)
		t.Logf("width=%d: narrow_font_build=%dT text_set_width=%dT", width, build, sw)
		near(fmt.Sprintf("width %d narrow_font_build", width), build, want[fmt.Sprintf("build%d", width)])
		near(fmt.Sprintf("width %d text_set_width", width), sw, want[fmt.Sprintf("setw%d", width)])
	}
}
