package pipeline

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// zvdbExampleRoot is examples/nanz, three levels up from this package.
func zvdbExampleRoot() string { return filepath.Join("..", "..", "..", "examples", "nanz") }

// zvdbCompile builds one of the zvdb CP/M programs through the production
// pipeline and assembles it to a 0x100 image.
func zvdbCompile(t *testing.T, name string) *z80asm.Result {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(zvdbExampleRoot(), name+".nanz"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), name+".nanz", nanz.ParseOpts{
		BaseDir:   zvdbExampleRoot(),
		StdlibDir: filepath.Join("..", "..", "..", "stdlib"),
	})
	if err != nil {
		t.Fatal(err)
	}
	h.Target = hir.TargetCPM
	steps, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a := z80asm.NewAssembler()
	target, err := z80asm.ParseTarget("cpm")
	if err != nil {
		t.Fatal(err)
	}
	a.SetTarget(target)
	image, err := a.AssembleString(steps.Assembly)
	if err != nil || len(image.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, image.Errors)
	}
	if image.Origin != 0x100 {
		t.Fatalf("origin %04X, want 0100", image.Origin)
	}
	return image
}

// zvdbRun loads the image as CP/M and returns the console output. keys is fed
// to BDOS function 1 one byte per call; when it runs out the program sees 0,
// exactly like mze at EOF.
func zvdbRun(t *testing.T, image *z80asm.Result, keys string) string {
	t.Helper()
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	mem := z.Memory()
	copy(mem[0x100:], image.Binary)
	binary.LittleEndian.PutUint16(mem[6:8], 0xFE00) // TPA top, as CCP publishes it
	mem[0] = 0xC9                                   // RET at warm boot: stop cleanly
	mem[0xFDFF] = 0
	mem[0xFDFE] = 0 // return address 0x0000 for main
	z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xFDFE})
	var out bytes.Buffer
	next := 0
	z.SetBDOSHandler(func(function byte, de uint16) (byte, uint16, bool) {
		switch function {
		case 0x01: // console input
			if next < len(keys) {
				ch := keys[next]
				next++
				return ch, 0, true
			}
			return 0, 0, true
		case 0x02: // console output
			out.WriteByte(byte(de))
			return 0, 0, true
		}
		return 0, 0, false
	})
	z.RemogattoZ80.MaxCycles = 2_000_000_000
	if err := z.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return out.String()
}

// TestZVDBDemoTopKMatchesPythonReference proves that the Z80 top-K produced by
// zvdb_kernel.zv_top_k — the same function the demo calls — equals the Python
// reference in examples/nanz/zvdb_demo_expected.txt for every record, on the
// full ranking of all other records.
func TestZVDBDemoTopKMatchesPythonReference(t *testing.T) {
	got := zvdbRun(t, zvdbCompile(t, "zvdb_dump_cpm"), "")
	wantBytes, err := os.ReadFile(filepath.Join(zvdbExampleRoot(), "zvdb_demo_expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, line := range strings.Split(strings.ReplaceAll(string(wantBytes), "\r\n", "\n"), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			want = append(want, line)
		}
	}
	rows := strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n")
	for len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	if len(rows) != len(want) {
		t.Fatalf("Z80 dumped %d rows, reference has %d", len(rows), len(want))
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("row %d:\n  z80: %q\n  py : %q", i, rows[i], want[i])
		}
	}
	t.Logf("%d rows identical", len(rows))
}

// TestZVDBDemoCPMScriptedSession drives the demo with the scripted key session
// (start, 3x down, 1x up, 30x down, q) and checks the incremental-redraw
// contract on the emitted stream: the screen is cleared exactly twice (startup
// and shutdown) and the chrome rows — title, pane headers, separator — are
// written exactly once, while the rows that do change are rewritten per move.
func TestZVDBDemoCPMScriptedSession(t *testing.T) {
	const keys = "\x1b[B\x1b[B\x1b[B" + "k" + "jjjjjjjjjjjjjjjjjjjjjjjjjjjjjj" + "q"
	got := zvdbRun(t, zvdbCompile(t, "zvdb_demo_cpm"), keys)

	chrome := map[string]string{
		"title row (01;01)":     "\x1b[01;01H",
		"left header (03;01)":   "\x1b[03;01H",
		"right header (03;39)":  "\x1b[03;39H",
		"separator row (23;01)": "\x1b[23;01H",
	}
	for name, seq := range chrome {
		if n := strings.Count(got, seq); n != 1 {
			t.Errorf("%s emitted %d times, want exactly 1", name, n)
		}
	}
	// Cleared exactly twice: once at startup, once on quit. Never in between.
	if n := strings.Count(got, "\x1b[2J"); n != 2 {
		t.Errorf("screen clear emitted %d times, want 2 (startup and shutdown)", n)
	}
	// The session makes 34 cursor moves (3 down + 1 up + 30 down); every dynamic
	// row is rewritten once per move plus once for the initial draw.
	dynamic := map[string]string{
		"label row (02;01)":    "\x1b[02;01H",
		"first detail (04;39)": "\x1b[04;39H",
		"status row (24;01)":   "\x1b[24;01H",
	}
	for name, seq := range dynamic {
		if n := strings.Count(got, seq); n != 35 {
			t.Errorf("%s emitted %d times, want 35 (initial draw + 34 moves)", name, n)
		}
	}
	if !strings.Contains(got, "last search: 35 recs") {
		t.Error("status line never reported the scan cost")
	}
}
