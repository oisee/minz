package pipeline

// ZX Spectrum build of the zvdb demo (task Z4): kernel equivalence against the
// Python reference, a whole-screen proof of the scripted session, and the
// keypress-to-settled timings.
//
//   TestZVDBZXKernelMatchesReference   runs examples/nanz/zvdb_dump_zx.nanz on an
//                                      in-process 48K machine and compares every
//                                      ranking it leaves in memory with
//                                      examples/nanz/zvdb_demo_expected.txt
//   TestZVDBZXScriptedSessionScreens   drives examples/nanz/zvdb_demo_zx.nanz
//                                      through start, 3 down, 1 up, 30 down,
//                                      `2`, `3`, `1` with native key-matrix
//                                      presses, and diffs all 6912 screen bytes
//                                      after every step against an independent
//                                      model — plus the pane text decoded back
//                                      to characters, and the settle timings
//   TestZVDBZXGlobalNamespace           guards the demo against the compiler's
//                                      unmangled variable-home labels

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

// ── the reference file ───────────────────────────────────────────────────────

type zvdbRefRow struct {
	sel, rank, idx, score int
	label                 string
}

// zvdbZXReference parses examples/nanz/zvdb_demo_expected.txt: the ranking the
// Python generator predicts, plus the record labels it carries.
func zvdbZXReference(t *testing.T) (rows []zvdbRefRow, labels []string, bySel map[int][]zvdbRefRow) {
	t.Helper()
	raw, err := os.ReadFile(zxNarrowRepoRoot + "/examples/nanz/zvdb_demo_expected.txt")
	if err != nil {
		t.Fatal(err)
	}
	bySel = map[int][]zvdbRefRow{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 5 {
			t.Fatalf("reference row has %d fields: %q", len(f), line)
		}
		var r zvdbRefRow
		r.sel, _ = strconv.Atoi(f[0])
		r.rank, _ = strconv.Atoi(f[1])
		r.idx, _ = strconv.Atoi(f[2])
		r.score, _ = strconv.Atoi(f[3])
		r.label = f[4]
		rows = append(rows, r)
		bySel[r.sel] = append(bySel[r.sel], r)
	}
	labels = make([]string, len(bySel))
	for _, r := range rows {
		if r.idx < len(labels) {
			labels[r.idx] = r.label
		}
	}
	for i := range labels {
		if labels[i] == "" {
			t.Fatalf("the reference never mentions record %d, so its label is unknown", i)
		}
	}
	return rows, labels, bySel
}

// ── literals the demo prints, kept in step with its source ─────────────────

const (
	zvdbZXTitleHead  = " zvdb Z80 sim  recs:"
	zvdbZXTitleTail  = "  256-bit hyperplanes"
	zvdbZXStatusHead = " JK/67 mv 123 w Q quit scan:"
	zvdbZXStatusTail = " recs scanned"
	zvdbZXHeadLeft   = "#   record"
	zvdbZXHeadRight  = "scr nearest records"
)

// zvdbZXLiteralsChecked fails as soon as the demo source stops carrying the
// literals above, so this model cannot silently drift from the program.
func zvdbZXLiteralsChecked(t *testing.T) {
	t.Helper()
	src, err := os.ReadFile(zxNarrowRepoRoot + "/examples/nanz/zvdb_demo_zx.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{zvdbZXTitleHead, zvdbZXTitleTail, zvdbZXStatusHead,
		zvdbZXStatusTail, zvdbZXHeadLeft, zvdbZXHeadRight} {
		if !strings.Contains(string(src), `c"`+l+`"`) {
			t.Fatalf("literal %q not found in examples/nanz/zvdb_demo_zx.nanz — refresh the model", l)
		}
	}
}

// ── geometry, mirroring the demo's layout() ─────────────────────────────────

type zvdbZXLayout struct {
	width, cols                            int
	leftW, leftPx, rightX, rightW, rightPx int
	divPx, divB0, divB1                    int
	labelLW, labelRW, labelW               int
	nRows, rowItems, rowSep, rowStatus     int
}

func zvdbZXGeometry(width int) zvdbZXLayout {
	cols := map[int]int{5: 51, 6: 42, 8: 32}[width]
	leftW := 16
	if width == 5 {
		leftW = 24
	}
	leftPx := leftW * width
	divEnd := leftPx + 8 // left_px is 8-aligned by construction
	rightX := (divEnd + width - 1) / width
	return zvdbZXLayout{
		width: width, cols: cols, leftW: leftW, leftPx: leftPx,
		rightX: rightX, rightW: cols - rightX, rightPx: rightX * width,
		divPx: leftPx + 4, divB0: leftPx >> 3, divB1: (divEnd - 1) >> 3,
		labelLW: leftW - 5, labelRW: (cols - rightX) - 5, labelW: cols - 9,
		nRows: 19, rowItems: 3, rowSep: 22, rowStatus: 23,
	}
}

// zvdbZXText renders the whole screen as one cell grid, exactly what the demo
// prints: every field is padded or truncated to a fixed cell count, so a row
// always covers its whole field and overprinting replaces rather than ORs.
func zvdbZXText(g zvdbZXLayout, cursor, top, nrec int, ranked []zvdbRefRow,
	labels []string, lastScan int) [][]byte {
	grid := make([][]byte, 24)
	for r := range grid {
		grid[r] = make([]byte, g.cols)
		for i := range grid[r] {
			grid[r][i] = ' '
		}
	}
	field := func(row []byte, at, w int, s string) {
		for i := 0; i < w && at+i < len(row); i++ {
			if i < len(s) {
				row[at+i] = s[i]
			} else {
				row[at+i] = ' '
			}
		}
	}
	num := func(row []byte, at, w, n int) {
		s := fmt.Sprintf("%*d", w, n) // right-aligned, like ui_put_num
		if len(s) > w {
			s = s[len(s)-w:]
		}
		field(row, at, w, s)
	}
	headTail := func(row []byte, head, tail string, n int) {
		hw := len(head)
		if hw > g.cols-3 {
			hw = g.cols - 3
		}
		field(row, 0, hw, head)
		num(row, hw, 3, n)
		field(row, hw+3, g.cols-hw-3, tail)
	}
	headTail(grid[0], zvdbZXTitleHead, zvdbZXTitleTail, nrec)
	field(grid[1], 0, 4, "Rec ")
	num(grid[1], 4, 3, cursor)
	field(grid[1], 7, 2, ": ")
	field(grid[1], 9, g.labelW, labels[cursor])
	field(grid[2], 0, g.leftW, zvdbZXHeadLeft)
	field(grid[2], g.rightX, g.rightW, zvdbZXHeadRight)
	for i := 0; i < g.nRows; i++ {
		idx := top + i
		if idx < nrec {
			num(grid[g.rowItems+i], 0, 3, idx)
			field(grid[g.rowItems+i], 4, g.labelLW, labels[idx])
		} else {
			field(grid[g.rowItems+i], 0, g.leftW, "")
		}
		if i < len(ranked) {
			num(grid[g.rowItems+i], g.rightX, 3, ranked[i].score)
			field(grid[g.rowItems+i], g.rightX+4, g.labelRW, labels[ranked[i].idx])
		} else {
			field(grid[g.rowItems+i], g.rightX, g.rightW, "")
		}
	}
	for i := 0; i < g.cols; i++ {
		grid[g.rowSep][i] = '-'
	}
	headTail(grid[g.rowStatus], zvdbZXStatusHead, zvdbZXStatusTail, lastScan)
	return grid
}

// ── the whole-screen model ──────────────────────────────────────────────────

// Attribute bytes. Within a pane every character shares its 8x8 px attribute
// cell with a neighbour, so colour is per rectangle and the cursor highlight is
// a whole inverted row — exactly what the demo does with win_invert_row.
const (
	zxZVDBAttrTitle     = 5<<3 | 0 | 0x40 // black on cyan, bright
	zxZVDBAttrLabel     = 1<<3 | 7 | 0x40 // white on blue, bright
	zxZVDBAttrHeadL     = 1<<3 | 6 | 0x40 // cyan on blue, bright
	zxZVDBAttrHeadR     = 0<<3 | 7 | 0x40 // white on black, bright
	zxZVDBAttrItemsL    = 1<<3 | 6        // cyan on blue
	zxZVDBAttrItemsR    = 0<<3 | 7        // white on black
	zxZVDBAttrItemsLInv = 6<<3 | 1        // the inverted cursor row
)

// zvdbZXExpectedScreen renders the 6912 bytes the demo's screen should hold
// from the geometry, the engine's documented blit/attribute rules and the
// reference ranking. Nothing here calls into the engine: the blit, the divider
// rule and the attribute rectangles are re-derived from the contract in
// stdlib/zx/text.nanz, in the same order the demo applies them.
func zvdbZXExpectedScreen(t *testing.T, width, cursor, top, nrec int, ranked []zvdbRefRow,
	labels []string, fonts map[int][96][8]byte) ([]byte, zvdbZXLayout) {
	t.Helper()
	g := zvdbZXGeometry(width)
	scr := make([]byte, 6912)
	// 1. the divider rule split_vertical paints over all 24 rows
	divByte, divBit := g.divPx>>3, byte(0x80>>(g.divPx&7))
	for row := 0; row < 24; row++ {
		for line := 0; line < 8; line++ {
			scr[zxScrOffset(divByte, row, line)] |= divBit
		}
	}
	// 2. the text (a blit replaces the cell's own pixels, so later wins). The
	// gutter columns between the panes are never printed in: the demo writes
	// [0, leftW) and [rightX, cols) of the item rows and of the header row, so
	// the divider rule survives there and nowhere else.
	grid := zvdbZXText(g, cursor, top, nrec, ranked, labels, nrec)
	for row := 0; row < 24; row++ {
		gutter := row == 2 || (row >= g.rowItems && row < g.rowItems+g.nRows)
		for c := 0; c < g.cols; c++ {
			if gutter && c >= g.leftW && c < g.rightX {
				continue
			}
			zxNarrowBlit(scr, int(grid[row][c]), c*width, row, width, fonts[width])
		}
	}
	// 3. the attribute rectangles
	leftB1 := (g.leftPx - 1) >> 3
	rightB0 := g.rightPx >> 3
	for row := 0; row < 24; row++ {
		for b := 0; b < 32; b++ {
			var a byte
			switch {
			case row == 0 || row == g.rowStatus:
				a = zxZVDBAttrTitle
			case row == 1:
				a = zxZVDBAttrLabel
			case row == 2:
				a = zxZVDBAttrHeadL
				if b >= rightB0 {
					a = zxZVDBAttrHeadR
				}
			case row == g.rowSep:
				a = zxZVDBAttrHeadL // chrome() always asks for bright
			case row >= g.rowItems && row < g.rowItems+g.nRows:
				if b >= rightB0 {
					a = zxZVDBAttrItemsR
				} else {
					a = zxZVDBAttrItemsL
					if row == g.rowItems+(cursor-top) && b <= leftB1 {
						a = zxZVDBAttrItemsLInv
					}
				}
			}
			scr[6144+row*32+b] = a
		}
	}
	return scr, g
}

// ── the runner ──────────────────────────────────────────────────────────────

// zvdbZXCompile builds one of the zvdb ZX programs through the production
// pipeline and assembles it at $8000. BaseDir is the examples directory so the
// sibling modules (zvdb_kernel, zvdb_demo_data) resolve, exactly as they do for
// `mz examples/nanz/<name>.nanz` from the repo root.
func zvdbZXCompile(t *testing.T, name string) ([]byte, map[string]int) {
	t.Helper()
	src, err := os.ReadFile(zxNarrowRepoRoot + "/examples/nanz/" + name + ".nanz")
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), name+".nanz", nanz.ParseOpts{
		BaseDir:   zxNarrowRepoRoot + "/examples/nanz",
		StdlibDir: zxNarrowRepoRoot + "/stdlib",
	})
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

// zvdbZXCompileAsm builds a program and also returns its assembly, for the
// label-namespace guard below.
func zvdbZXCompileAsm(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile(zxNarrowRepoRoot + "/examples/nanz/" + name + ".nanz")
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), name+".nanz", nanz.ParseOpts{
		BaseDir:   zxNarrowRepoRoot + "/examples/nanz",
		StdlibDir: zxNarrowRepoRoot + "/stdlib",
	})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	return steps.Assembly
}

// TestZVDBZXGlobalNamespace guards the demo against a name-resolution defect in
// the compiler: an identifier inside an imported function can bind to the
// importing module's same-named global instead of the function's own local, and
// takes the global's type. `top = &NF5_TOP` in zx.narrow_font.nf_build_packed
// (stdlib/zx/narrow_font.nanz) compiles to `LD IYH, B / LD IYL, C` normally, but
// with a `global top: u8` in the main module the same line compiles to
// `LD A, C / LD (top), A` — one byte of the pointer into the main module's
// global — and every glyph is then squeezed with a mask read back through it.
// `left_w` and `right_x` in zx.text.split_vertical_i alias the same way. No
// diagnostic at any stage; the repro is
// reports/zvdb-zx/repro/label-home-collision/ (collide.nanz vs control.nanz).
// Two guards:
//
//  1. every lowercase global of the demo source lives in the zvui_ namespace,
//     which no stdlib local or field can resolve to;
//  2. the emitted assembly references no bare label of the main module from
//     inside an imported module's code, which would catch the same defect even
//     if the naming rule were broken.
func TestZVDBZXGlobalNamespace(t *testing.T) {
	src, err := os.ReadFile(zxNarrowRepoRoot + "/examples/nanz/zvdb_demo_zx.nanz")
	if err != nil {
		t.Fatal(err)
	}
	globalRe := regexp.MustCompile(`(?m)^global\s+([a-z][A-Za-z_0-9]*)`)
	for _, m := range globalRe.FindAllStringSubmatch(string(src), -1) {
		if !strings.HasPrefix(m[1], "zvui_") {
			t.Errorf("global %q is not in the zvui_ namespace: a stdlib local of the same name would silently alias it (see the demo header)", m[1])
		}
	}
	asm := zvdbZXCompileAsm(t, "zvdb_demo_zx")
	lines := strings.Split(asm, "\n")
	bare := map[string]bool{}
	for _, l := range lines {
		if l != "" && l[0] != ' ' && l[0] != '\t' && strings.HasSuffix(l, ":") && !strings.Contains(l, "__") {
			bare[strings.TrimSuffix(l, ":")] = true
		}
	}
	refRe := regexp.MustCompile(`\(([A-Za-z_][A-Za-z_0-9]*)\)`)
	owner := ""
	for _, l := range lines {
		if l == "" {
			continue
		}
		if l[0] != ' ' && l[0] != '\t' {
			if strings.HasSuffix(l, ":") {
				owner = strings.TrimSuffix(l, ":")
			}
			continue
		}
		// only imported modules carry the mangled <importer>__<name> prefix
		if owner == "" || !strings.Contains(owner, "__") {
			continue
		}
		for _, m := range refRe.FindAllStringSubmatch(l, -1) {
			if bare[m[1]] {
				t.Errorf("the main module's label %q is referenced from %q: an imported module's variable home has aliased it", m[1], owner)
			}
		}
	}
}

// zvdbZXEvent is one native key-matrix transition at a frame boundary.
type zvdbZXEvent struct {
	frame, row, bit int
	down            bool
}

// zvdbZXSession runs the demo once, recording a checksum of the screen every
// frame (for the settle timings) and the full 6912 bytes at the capture frames.
func zvdbZXSession(t *testing.T, bin []byte, events []zvdbZXEvent, captures []int, maxFrames int) (map[int][]byte, []uint64) {
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
		machine.Memory.Write(0x8000+uint16(i), b, false)
	}
	machine.CPU.SetPC(0x8000)
	machine.CPU.SetSP(0xFF58)
	machine.CPU.SetIM(1)
	machine.CPU.SetIFF1(false)
	machine.CPU.SetIFF2(false)
	want := map[int]bool{}
	for _, f := range captures {
		want[f] = true
	}
	got := map[int][]byte{}
	hashes := make([]uint64, maxFrames+2)
	next := 0
	for f := 0; f < maxFrames; f++ {
		for next < len(events) && events[next].frame == f {
			e := events[next]
			if e.down {
				machine.Keyboard.KeyPress(e.row, e.bit)
			} else {
				machine.Keyboard.KeyRelease(e.row, e.bit)
			}
			next++
		}
		machine.RunFrameFast()
		vram := machine.ReadVRAM()
		hashes[f+1] = zvdbZXHash(vram)
		if want[f+1] {
			cp := make([]byte, len(vram))
			copy(cp, vram)
			got[f+1] = cp
		}
		if machine.CPU.Halted() {
			break
		}
	}
	return got, hashes
}

func zvdbZXHash(b []byte) uint64 {
	var h uint64 = 1469598103934665603
	for _, v := range b {
		h ^= uint64(v)
		h *= 1099511628211
	}
	return h
}

// zvdbZXSettle returns how many frames pass from a keypress to the last screen
// change it causes. hashes[f+1] is the screen after frame f, so hashes[f+1] !=
// hashes[f] means frame f changed something; the search stops *before* `until`,
// the frame the next key is applied, so that key's redraw cannot be credited to
// this one. scripts/zvdb_zx_verify.py measures the same quantity on mzx's own
// frame dumps, where dump N is the screen after frame N.
func zvdbZXSettle(hashes []uint64, pressFrame, until int) int {
	last := pressFrame
	for f := pressFrame; f < until && f+1 < len(hashes); f++ {
		if hashes[f+1] != hashes[f] {
			last = f
		}
	}
	return last - pressFrame
}

// ── 1. kernel equivalence on the Spectrum build ─────────────────────────────

func TestZVDBZXKernelMatchesReference(t *testing.T) {
	bin, symbols := zvdbZXCompile(t, "zvdb_dump_zx")
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
	for !machine.CPU.Halted() && machine.AbsoluteTStates() < 2_000_000_000 {
		machine.CPU.DoOpcode()
	}
	if !machine.CPU.Halted() {
		t.Fatal("the dumper never halted")
	}
	rd := func(name string) uint16 {
		a, ok := symbols[name]
		if !ok {
			t.Fatalf("symbol %s missing from the built image", name)
		}
		return uint16(a)
	}
	if got := machine.Memory.Read(rd("dump_done")); got != 0xA5 {
		t.Fatalf("dump_done = %#x, want 0xA5 (the dumper did not finish)", got)
	}
	n := int(machine.Memory.Read(rd("dump_n")))
	rows, _, bySel := zvdbZXReference(t)
	if n != len(bySel) {
		t.Fatalf("dump_n = %d, the reference ranks %d records", n, len(bySel))
	}
	const stride = 40 // DUMP_RANK_CAP in zvdb_dump_zx.nanz
	idxBase, valBase := rd("dump_idx"), rd("dump_val")
	matched := 0
	for sel := 0; sel < n; sel++ {
		want := bySel[sel]
		cnt := int(machine.Memory.Read(rd("dump_cnt") + uint16(sel)))
		if cnt != len(want) {
			t.Fatalf("sel %d: dumped %d ranks, the reference has %d", sel, cnt, len(want))
		}
		for r := 0; r < cnt; r++ {
			off := uint16(sel)*stride + uint16(r)
			idx := int(machine.Memory.Read(idxBase + off))
			score := int(machine.Memory.Read(valBase+2*off)) | int(machine.Memory.Read(valBase+2*off+1))<<8
			if idx != want[r].idx || score != want[r].score {
				t.Fatalf("sel %d rank %d: z80 (%d, %d), reference (%d, %d)",
					sel, r, idx, score, want[r].idx, want[r].score)
			}
			matched++
		}
	}
	if matched != len(rows) {
		t.Fatalf("compared %d rows, the reference has %d", matched, len(rows))
	}
	t.Logf("%d rows identical across %d selections, %d T-states",
		matched, n, machine.AbsoluteTStates())
}

// ── 2. the whole-screen proof ───────────────────────────────────────────────

func TestZVDBZXScriptedSessionScreens(t *testing.T) {
	zvdbZXLiteralsChecked(t)
	bin, _ := zvdbZXCompile(t, "zvdb_demo_zx")
	fonts := map[int][96][8]byte{5: zxNarrowPacked(t, 5), 6: zxNarrowPacked(t, 6), 8: zxNarrowPacked(t, 8)}
	_, recordLabels, bySel := zvdbZXReference(t)
	nrec := len(bySel)

	// key matrix: J = row 6 bit 3, K = row 6 bit 2, 1/2/3 = row 3 bits 0/1/2,
	// CAPS SHIFT = row 0 bit 0 (held together with a digit for the cursor keys)
	const (
		jRow, jBit = 6, 3
		kRow, kBit = 6, 2
	)
	const (
		hold  = 8   // frames a key stays down
		step  = 60  // frames between two keys: a scroll-up move takes ~46 and the demo has no key queue
		wgap  = 150 // frames after a width switch: the font rebuild plus a full redraw takes ~90
		start = 110
	)
	var events []zvdbZXEvent
	type keyPress struct {
		label string
		frame int
	}
	var presses []keyPress // one entry per logical keypress, in schedule order
	press := func(label string, frame, row, bit int) {
		events = append(events, zvdbZXEvent{frame, row, bit, true}, zvdbZXEvent{frame + hold, row, bit, false})
		presses = append(presses, keyPress{label, frame})
	}
	// CAPS SHIFT + a digit: both go down at the same frame boundary and come up
	// together, so the scan always sees them as a pair (the shift alone would be
	// stepped over, which is the point of holding it with a digit)
	pressCombo := func(label string, frame int, keys [][2]int) {
		for _, k := range keys {
			events = append(events, zvdbZXEvent{frame, k[0], k[1], true})
		}
		for _, k := range keys {
			events = append(events, zvdbZXEvent{frame + hold, k[0], k[1], false})
		}
		presses = append(presses, keyPress{label, frame})
	}
	down := func(label string, frame int) { press(label, frame, jRow, jBit) }
	up := func(label string, frame int) { press(label, frame, kRow, kBit) }
	width := func(label string, frame, bit int) { press(label, frame, 3, bit) }

	// The scripted session the task asks for — start, 3 down, 1 up, 30 down
	// (scrolls), `2`, `3`, `1` — then everything the first draft left untested:
	// moves past the last record (the bottom clamp), moves that scroll the list
	// *up* (a different code path from a scroll down, and one win_scroll cannot
	// express at all), a width switch while the list is scrolled, one more
	// scroll-up at the new width, and one CAPS SHIFT + 6 cursor-key move.
	type state struct {
		name        string
		frame       int
		width       int
		cursor, top int
	}
	var checks []state
	f := start
	down("move @51", f)
	f += step
	down("move @51", f)
	f += step
	pressCombo("CS+6 move @51", f, [][2]int{{0, 0}, {4, 4}})
	f += step
	checks = append(checks, state{"3 down (one of them CS+6)", f - 5, 5, 3, 0})
	up("move @51", f)
	f += step
	checks = append(checks, state{"1 up", f - 5, 5, 2, 0})
	// the first sixteen of the thirty do not scroll (cursor 3..18); from cursor
	// 19 on, each one scrolls the list down a row
	for i := 0; i < 30; i++ {
		if i < 16 {
			down("move @51", f)
		} else {
			down("move @51 (scrolled)", f)
		}
		f += step
	}
	checks = append(checks, state{"30 down (scrolled)", f - 5, 5, 32, 14})
	f += 40

	// three width switches, each followed by moves at the new width
	width("switch to 42 columns", f, 1)
	f += wgap
	checks = append(checks, state{"key 2 (42 columns)", f - 30, 6, 32, 14})
	for i := 0; i < 3; i++ {
		up("move @42", f)
		f += step
	}
	checks = append(checks, state{"3 up at 42 columns", f - 5, 6, 29, 14})
	width("switch to 32 columns", f, 2)
	f += wgap
	checks = append(checks, state{"key 3 (32 columns)", f - 30, 8, 29, 14})
	for i := 0; i < 3; i++ {
		down("move @32", f)
		f += step
	}
	checks = append(checks, state{"3 down at 32 columns", f - 5, 8, 32, 14})
	width("switch to 51 columns", f, 0)
	f += wgap
	checks = append(checks, state{"key 1 (51 columns)", f - 30, 5, 32, 14})
	for i := 0; i < 2; i++ {
		down("move @51 (scrolled)", f)
		f += step
	}
	checks = append(checks, state{"2 down at 51 columns (scrolled)", f - 5, 5, 34, 16})

	// past the last record: every further down is clamped and redraws nothing
	for i := 0; i < 2; i++ {
		down("clamp @51", f)
		f += step
	}
	checks = append(checks, state{"2 down past the end (clamped)", f - 5, 5, 34, 16})

	// back up the list until it scrolls the other way: top drops from 16 to 14
	// (at cursor 15 and again at 14, the last two of the twenty), which is the
	// branch win_scroll cannot serve
	for i := 0; i < 20; i++ {
		if i < 18 {
			up("move @51 (up)", f)
		} else {
			up("move @51 (scrolled up)", f)
		}
		f += step
	}
	checks = append(checks, state{"20 up at 51 columns (scrolled up)", f - 5, 5, 14, 14})

	// a width switch while the list is scrolled, then one more scroll-up there
	width("switch to 42 columns", f, 1)
	f += wgap
	checks = append(checks, state{"key 2 while scrolled", f - 30, 6, 14, 14})
	up("move @42 (scrolled up)", f)
	f += step
	checks = append(checks, state{"1 up at 42 columns (scrolled up)", f - 5, 6, 13, 13})
	maxFrames := checks[len(checks)-1].frame + 10

	captures := []int{100} // the startup screen, once the first draw has settled
	for _, c := range checks {
		captures = append(captures, c.frame)
	}
	screens, hashes := zvdbZXSession(t, bin, events, captures, maxFrames)
	all := append([]state{{"start (51 columns)", 100, 5, 0, 0}}, checks...)

	for _, c := range all {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got, ok := screens[c.frame]
			if !ok {
				t.Fatalf("no screen captured at frame %d", c.frame)
			}
			want, g := zvdbZXExpectedScreen(t, c.width, c.cursor, c.top, nrec, bySel[c.cursor], recordLabels, fonts)
			var in, out, attr int
			var firstOut, firstAttr string
			leftB1 := (g.leftPx - 1) >> 3
			rightB0 := g.rightPx >> 3
			rightB1 := (g.rightPx + g.rightW*g.width - 1) >> 3
			for row := 0; row < 24; row++ {
				for line := 0; line < 8; line++ {
					for b := 0; b < 32; b++ {
						off := zxScrOffset(b, row, line)
						if got[off] == want[off] {
							continue
						}
						if (b >= 0 && b <= leftB1) || (b >= rightB0 && b <= rightB1) {
							in++
						} else {
							out++
							if firstOut == "" {
								firstOut = fmt.Sprintf("byte col %d char row %d line %d: got %#02x want %#02x",
									b, row, line, got[off], want[off])
							}
						}
					}
				}
				for b := 0; b < 32; b++ {
					off := 6144 + row*32 + b
					if got[off] != want[off] {
						attr++
						if firstAttr == "" {
							firstAttr = fmt.Sprintf("attr col %d row %d: got %#02x want %#02x", b, row, got[off], want[off])
						}
					}
				}
			}
			if in+out+attr != 0 {
				t.Errorf("screen differs from the model: %d bitmap bytes inside the panes, %d outside, %d attribute cells\n  first outside: %s\n  first attribute: %s",
					in, out, attr, firstOut, firstAttr)
			}
			// the decoded text of both panes must equal what the reference
			// predicts for the selected record
			left := zxNarrowDecode(t, got, 0, g.rowItems, g.leftW, g.nRows, c.width, fonts)
			right := zxNarrowDecode(t, got, g.rightPx, g.rowItems, g.rightW, g.nRows, c.width, fonts)
			grid := zvdbZXText(g, c.cursor, c.top, nrec, bySel[c.cursor], recordLabels, nrec)
			for i := 0; i < g.nRows; i++ {
				if wantL := string(grid[g.rowItems+i][:g.leftW]); left[i] != wantL {
					t.Errorf("master row %d: got %q want %q", i, left[i], wantL)
				}
				if wantR := string(grid[g.rowItems+i][g.rightX : g.rightX+g.rightW]); right[i] != wantR {
					t.Errorf("detail row %d: got %q want %q", i, right[i], wantR)
				}
			}
		})
	}

	// the timings, measured on the same run: frames from a keypress to the last
	// screen change it causes, grouped by what the key was
	byLabel := map[string][]int{}
	for _, p := range presses {
		until := maxFrames
		for _, q := range presses {
			if q.frame > p.frame && q.frame < until {
				until = q.frame
			}
		}
		byLabel[p.label] = append(byLabel[p.label], zvdbZXSettle(hashes, p.frame, until))
	}
	order := []string{"move @51", "CS+6 move @51", "move @51 (scrolled)", "clamp @51",
		"move @51 (up)", "move @51 (scrolled up)", "switch to 42 columns", "move @42",
		"move @42 (scrolled up)", "switch to 32 columns", "move @32",
		"switch to 51 columns"}
	for _, l := range order {
		if v, ok := byLabel[l]; ok {
			t.Logf("%-24s %v frames (max %d)", l+":", v, maxInt(v))
		}
	}
	// A redraw must have finished before the next key arrives: the demo has no
	// key queue, so a press that lands while the screen is still being written
	// is lost. (Measuring the settle alone cannot show this — the search has to
	// stop at the next key, or that key's redraw would be credited to this one.)
	for i, p := range presses {
		next := maxFrames
		if i+1 < len(presses) {
			next = presses[i+1].frame
		}
		for f := next - 3; f <= next && f >= 1; f++ {
			if hashes[f] != hashes[f-1] {
				t.Errorf("%s (frame %d): the screen was still changing at frame %d, %d frames before the next key arrives",
					p.label, p.frame, f, next-f)
			}
		}
	}
}

func maxInt(v []int) int {
	m := 0
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}
