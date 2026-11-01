package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// ── VT100 reconstruction ─────────────────────────────────────────────────────

// zvdbScreen is a minimal VT100 model: enough for the demo's output (CUP, ED,
// SGR, printable text). It records the text grid, which rows were last written
// in the selection colours, the column of every '|' written on each row, and
// how many characters were written at or past the right edge — the invariant
// that catches a layout width that no longer fits the screen.
type zvdbScreen struct {
	width, height int
	cells         [][]byte
	row, col      int
	inverse       bool
	inverseRows   []int
	dividers      map[int][]int
	overflow      int
}

func newZvdbScreen(width, height int) *zvdbScreen {
	s := &zvdbScreen{width: width, height: height, dividers: map[int][]int{}}
	s.clear()
	return s
}

func (s *zvdbScreen) clear() {
	s.cells = make([][]byte, s.height)
	for i := range s.cells {
		s.cells[i] = make([]byte, s.width)
		for j := range s.cells[i] {
			s.cells[i][j] = ' '
		}
	}
}

func (s *zvdbScreen) put(ch byte) {
	if s.col >= s.width {
		s.overflow++
	} else {
		if s.row < s.height {
			s.cells[s.row][s.col] = ch
		}
		if ch == '|' {
			s.dividers[s.row] = append(s.dividers[s.row], s.col)
		}
	}
	s.col++
}

func (s *zvdbScreen) feed(stream string) {
	b := []byte(stream)
	for i := 0; i < len(b); {
		if b[i] != 0x1B || i+1 >= len(b) || b[i+1] != '[' {
			if 0x20 <= b[i] && b[i] < 0x7F {
				if s.inverse {
					s.inverseRows = append(s.inverseRows, s.row)
				}
				s.put(b[i])
			}
			i++
			continue
		}
		j := i + 2
		for j < len(b) && 0x30 <= b[j] && b[j] <= 0x3F {
			j++
		}
		for j < len(b) && 0x20 <= b[j] && b[j] <= 0x2F {
			j++
		}
		if j >= len(b) {
			break
		}
		final, params := b[j], string(b[i+2:j])
		switch final {
		case 'H', 'f':
			row, _, col, _ := 1, 0, 1, 0
			fmt.Sscanf(params, "%d;%d", &row, &col)
			s.row, s.col = row-1, col-1
		case 'J':
			if params == "2" || params == "" {
				s.clear()
			}
		case 'K':
			for c := s.col; c < s.width; c++ {
				s.cells[s.row][c] = ' '
			}
		case 'A':
			s.row--
		case 'B':
			s.row++
		case 'C':
			s.col++
		case 'D':
			s.col--
		case 'm':
			s.inverse = params == "90;106"
		}
		i = j + 1
	}
}

func (s *zvdbScreen) line(row int) string { return strings.TrimRight(string(s.cells[row]), " ") }

// zvdbLiveStream drops the shutdown sequence so the screen rendered is the one
// the user saw before quitting: the demo clears the screen on exit.
func zvdbLiveStream(stream string) string {
	if i := strings.LastIndex(stream, "\x1b[2J"); i >= 0 {
		return stream[:i]
	}
	return stream
}

// checkLayout asserts the two invariants that hold for every screen the demo
// draws: the master pane's divider is in the same column on the header row and
// on every item row, and nothing is ever written past the right edge.
func (s *zvdbScreen) checkLayout(t *testing.T, firstRow, rows int) {
	t.Helper()
	if s.overflow != 0 {
		t.Errorf("%d characters written at or past column %d", s.overflow, s.width)
	}
	want := -1
	for row := firstRow; row < firstRow+rows; row++ {
		cols := s.dividers[row]
		if len(cols) == 0 {
			t.Errorf("row %d has no pane divider", row)
			continue
		}
		distinct := map[int]bool{}
		for _, c := range cols {
			distinct[c] = true
		}
		// A row is redrawn many times, but always with the divider in the same
		// column - a jagged divider is what blocker 1 looked like.
		if len(distinct) != 1 {
			t.Errorf("row %d dividers at columns %v, want one column", row, distinct)
			continue
		}
		col := cols[0]
		if row == firstRow {
			want = col
		} else if col != want {
			t.Errorf("row %d divider at column %d, header row has %d", row, col, want)
		}
	}
}

// ── synthetic data sets ──────────────────────────────────────────────────────

// zvdbSyntheticData writes a data module with n records: pseudo-random vectors
// from a fixed LCG and "rec <i>" labels. It deliberately does not reuse
// scripts/zvdb_demo_data.py so the Go reference below is independent of the
// generator that produced the shipped data.
func zvdbSyntheticData(dir string, n int) (vecs []byte) {
	var labels []byte
	offs := []int{0}
	for i := 0; i < n; i++ {
		labels = append(labels, []byte(fmt.Sprintf("rec %d", i))...)
		labels = append(labels, 0)
		offs = append(offs, len(labels))
	}
	x := uint32(7)
	for i := 0; i < n*32; i++ {
		x = x*1103515245 + 12345
		vecs = append(vecs, byte(x>>16))
	}
	lit := func(a []int, per int) string {
		var sb strings.Builder
		for i := 0; i < len(a); i += per {
			end := i + per
			if end > len(a) {
				end = len(a)
			}
			vals := make([]string, 0, end-i)
			for _, v := range a[i:end] {
				vals = append(vals, fmt.Sprint(v))
			}
			sb.WriteString("    " + strings.Join(vals, ", ") + ",\n")
		}
		return sb.String()
	}
	src := fmt.Sprintf("// synthetic data module, n=%d\nglobal zv_n: u8 = %d\nglobal zv_vec_bytes: u8 = 32\n"+
		"global zv_label_off: [u16; %d] = [\n%s]\n"+
		"global zv_labels: [u8; %d] = [\n%s]\n"+
		"global zv_vecs: [u8; %d] = [\n%s]\n",
		n, n, n+1, lit(offs, 16), len(labels), lit(ints(labels), 20), len(vecs), lit(ints(vecs), 16))
	if err := os.WriteFile(filepath.Join(dir, "zvdb_demo_data.nanz"), []byte(src), 0644); err != nil {
		panic(err)
	}
	return vecs
}

func ints(b []byte) []int {
	out := make([]int, len(b))
	for i, v := range b {
		out[i] = int(v)
	}
	return out
}

var zvdbPopcount = func() [256]int {
	var p [256]int
	for i := 0; i < 256; i++ {
		p[i] = strings.Count(fmt.Sprintf("%08b", i), "1")
	}
	return p
}()

// zvdbReference ranks every record but `sel` against it, using the demo's rule:
// score = 256 - popcount(a XOR b), descending, ties by ascending index.
func zvdbReference(vecs []byte, sel int) [][2]int {
	n := len(vecs) / 32
	pop := zvdbPopcount
	var rows [][2]int
	for j := 0; j < n; j++ {
		if j == sel {
			continue
		}
		ham := 0
		for k := 0; k < 32; k++ {
			ham += pop[vecs[sel*32+k]^vecs[j*32+k]]
		}
		rows = append(rows, [2]int{256 - ham, j})
	}
	sort.SliceStable(rows, func(a, b int) bool {
		if rows[a][0] != rows[b][0] {
			return rows[a][0] > rows[b][0]
		}
		return rows[a][1] < rows[b][1]
	})
	return rows
}

// zvdbBuildDir compiles `name` from dir (which must hold its imports) for CP/M.
func zvdbBuildDir(t *testing.T, dir, name string, rewrite map[string]string) *z80asm.Result {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(dir, name+".nanz"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for pat, rep := range rewrite {
		re := regexp.MustCompile(pat)
		if !re.MatchString(text) {
			t.Fatalf("rewrite %q matched nothing in %s", pat, name)
		}
		text = re.ReplaceAllString(text, rep)
	}
	h, err := nanz.ParseWithOpts(text, name+".nanz", nanz.ParseOpts{BaseDir: dir, StdlibDir: zvdbStdlib()})
	if err != nil {
		t.Fatal(err)
	}
	h.Target = hir.TargetCPM
	steps, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a := z80asm.NewAssembler()
	if target, err := z80asm.ParseTarget("cpm"); err != nil {
		t.Fatal(err)
	} else {
		a.SetTarget(target)
	}
	image, err := a.AssembleString(steps.Assembly)
	if err != nil || len(image.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, image.Errors)
	}
	return image
}

func zvdbStdlib() string { return filepath.Join("..", "..", "..", "stdlib") }

// zvdbStage copies the kernel and data module plus the named example into a
// temp dir so a synthetic data set or a rewritten layout can be compiled.
func zvdbStage(t *testing.T, records int) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"zvdb_kernel.nanz"} {
		b, err := os.ReadFile(filepath.Join(zvdbExampleRoot(), f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if records == 0 {
		b, err := os.ReadFile(filepath.Join(zvdbExampleRoot(), "zvdb_demo_data.nanz"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "zvdb_demo_data.nanz"), b, 0644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	zvdbSyntheticData(dir, records)
	return dir
}

// ── tests ────────────────────────────────────────────────────────────────────

// TestZVDBDemoCPMScreenContent reconstructs the screen the scripted session
// ends on and checks what is actually drawn, not just the escape bookkeeping:
// the divider lines up, nothing spills past the last column, the label line and
// status line carry the final selection, and the highlighted master row is the
// record the reference says is selected.
func TestZVDBDemoCPMScreenContent(t *testing.T) {
	const keys = "\x1b[B\x1b[B\x1b[B" + "k" + "jjjjjjjjjjjjjjjjjjjjjjjjjjjjjj" + "q"
	s := newZvdbScreen(80, 24)
	s.feed(zvdbLiveStream(zvdbRun(t, zvdbCompile(t, "zvdb_demo_cpm"), keys)))
	s.checkLayout(t, 2, 20)

	if got := s.line(1); !strings.HasPrefix(got, "Rec  32: S/4 HANA is SAP ERP.") {
		t.Errorf("label line = %q", got)
	}
	if got := s.line(23); !strings.Contains(got, "last search: 35 recs") {
		t.Errorf("status line = %q", got)
	}
	// The selection colours also paint the title bar, so the highlighted master
	// row is the last inverse row inside the pane.
	var highlighted int
	for _, r := range s.inverseRows {
		if 3 <= r && r < 22 {
			highlighted = r
		}
	}
	if got := strings.TrimSpace(s.line(highlighted)[:4]); got != "32" {
		t.Errorf("highlighted master row %d shows index %q, want 32", highlighted, got)
	}
}

// TestZVDBDemoWideDataSetDoesNotOverflow is the regression test for the u8
// underflow in ui_put_num: with more than 100 records the index needs three
// digits, and `width - n` used to wrap to ~255 spaces and destroy the screen.
func TestZVDBDemoWideDataSetDoesNotOverflow(t *testing.T) {
	const n = 120
	dir := zvdbStage(t, n)
	b, err := os.ReadFile(filepath.Join(zvdbExampleRoot(), "zvdb_demo_cpm.nanz"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zvdb_demo_cpm.nanz"), b, 0644); err != nil {
		t.Fatal(err)
	}
	image := zvdbBuildDir(t, dir, "zvdb_demo_cpm", nil)

	s := newZvdbScreen(80, 24)
	s.feed(zvdbLiveStream(zvdbRun(t, image, strings.Repeat("j", n-1)+"q")))
	s.checkLayout(t, 2, 20)
	if got := s.line(1); !strings.HasPrefix(got, "Rec 119: rec 119") {
		t.Errorf("label line = %q", got)
	}
	for _, row := range []int{3, 10, 21} {
		if idx := strings.TrimSpace(s.line(row)[:4]); idx == "" || !isDigits(idx) {
			t.Errorf("master row %d index field = %q", row, idx)
		}
	}
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// TestZVDBDemoTopKAtCapacityMatchesReference is the regression test for the
// out-of-bounds shift in zv_top_k: with 252 records the ranking used to fill
// its buffers, put record 0 into its own top-K and report a wrong neighbour.
// Four selections x six ranks is enough to see it and stays cheap (~100M
// T-states, under a second emulated).
func TestZVDBDemoTopKAtCapacityMatchesReference(t *testing.T) {
	const n = 252
	dir := zvdbStage(t, n)
	vecs := zvdbSyntheticData(dir, n)

	selections := []int{0, 63, 126, n - 1}
	var body strings.Builder
	for _, sel := range selections {
		fmt.Fprintf(&body, "    zv_top_k(%d)\n", sel)
		body.WriteString("    emit_num(u16(zv_top_cnt))\n    tui_putch(13)\n    tui_putch(10)\n")
		for rank := 0; rank < 6; rank++ {
			fmt.Fprintf(&body, "    emit(%d, %d)\n", sel, rank)
		}
	}
	driver := `import tui.render
import zvdb_demo_data
import zvdb_kernel

global numbuf: [u8; 6]

fun emit_num(v: u16) -> void {
    var k: u8 = zv_fmt_u16(v, &numbuf)
    var i: u8 = 0
    while i < k {
        tui_putch(numbuf[i])
        i = i + 1
    }
}

fun emit(sel: u8, rank: u8) -> void {
    emit_num(u16(sel))
    tui_putch(9)
    emit_num(u16(rank))
    tui_putch(9)
    emit_num(u16(zv_rank_index(rank)))
    tui_putch(9)
    emit_num(zv_rank_score(rank))
    tui_putch(13)
    tui_putch(10)
}

fun main() -> void {
` + body.String() + `}
`
	if err := os.WriteFile(filepath.Join(dir, "driver.nanz"), []byte(driver), 0644); err != nil {
		t.Fatal(err)
	}
	got := zvdbRun(t, zvdbBuildDir(t, dir, "driver", nil), "")
	lines := strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n")
	i := 0
	for _, sel := range selections {
		want := zvdbReference(vecs, sel)
		if lines[i] != fmt.Sprint(n-1) {
			t.Fatalf("sel %d: zv_top_cnt = %q, want %d", sel, lines[i], n-1)
		}
		i++
		for rank := 0; rank < 6; rank++ {
			wantRow := fmt.Sprintf("%d\t%d\t%d\t%d", sel, rank, want[rank][1], want[rank][0])
			if lines[i] != wantRow {
				t.Errorf("sel %d rank %d:\n  z80: %q\n  ref : %q", sel, rank, lines[i], wantRow)
			}
			i++
		}
	}
	t.Logf("%d selections x 6 ranks plus counts match the independent reference", len(selections))
}

// TestZVDBDemoNarrowScreenLayout proves the REPORT.md §7 porting claim: change
// the two per-port constants to 51 columns and nothing spills past column 51.
func TestZVDBDemoNarrowScreenLayout(t *testing.T) {
	dir := zvdbStage(t, 0)
	b, err := os.ReadFile(filepath.Join(zvdbExampleRoot(), "zvdb_demo_cpm.nanz"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zvdb_demo_cpm.nanz"), b, 0644); err != nil {
		t.Fatal(err)
	}
	image := zvdbBuildDir(t, dir, "zvdb_demo_cpm", map[string]string{
		`const SCR_W: u8 = 80`:  `const SCR_W: u8 = 51`,
		`const LEFT_W: u8 = 36`: `const LEFT_W: u8 = 24`,
	})
	s := newZvdbScreen(51, 24)
	s.feed(zvdbLiveStream(zvdbRun(t, image, "jjq")))
	s.checkLayout(t, 2, 20)
	if got := s.line(1); !strings.HasPrefix(got, "Rec   2: Nature") {
		t.Errorf("label line = %q", got)
	}
	for row, want := range map[int]string{0: " zvdb", 22: "---", 23: " j/k"} {
		if !strings.HasPrefix(s.line(row), want) {
			t.Errorf("row %d = %q, want prefix %q", row, s.line(row), want)
		}
	}
}
