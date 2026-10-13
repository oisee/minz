package pipeline

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CZECH is an external, author-licensed corpus, never vendored into MinZ.
// Execute the complete interpreter and screen adapter, observing characters
// at the active screen output function without replacing its body or dispatcher.
func TestZMachineCzechNative(t *testing.T)        { testZMachineCzechNative(t, false) }
func TestZMachineCzechWindowsNative(t *testing.T) { testZMachineCzechNative(t, true) }
func testZMachineCzechNative(t *testing.T, windows bool) {
	storyPath := os.Getenv("MINZ_Z3_CZECH")
	vmPath := os.Getenv("MINZ_Z3_VM")
	if storyPath == "" || vmPath == "" {
		t.Skip("set pinned MINZ_Z3_CZECH and MINZ_Z3_VM")
	}
	story, err := os.ReadFile(storyPath)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(story)) != "6d628af7ab4779224cb87923cce3764e3ad3ab8634758f409b638ba20f91d283" {
		t.Fatal("CZECH fingerprint mismatch")
	}
	root := filepath.Clean("../../..")
	dir := t.TempDir()
	source := filepath.Join(dir, "core.nanz")
	cmd := exec.Command("python3", filepath.Join(root, "scripts/zmachine/prepare_native_core.py"), source, "--story-bytes", fmt.Sprint(len(story)), "--console")
	if windows {
		cmd.Args = append(cmd.Args, "--windows")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate %v %s", err, out)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(data), source, nanz.ParseOpts{BaseDir: dir, StdlibDir: filepath.Join(root, "stdlib")})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	if 0x5b00+len(a.Binary) > 0xbe00 {
		t.Fatal("resident overlaps stack")
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x5b00+uint16(i), b, false)
	}
	pages := []int{0, 1, 3, 4, 6, 7}
	for i, b := range story {
		m.Memory.RAM[pages[i/16384]][i%16384] = b
	}
	m.Memory.SetPagingForce(0x10)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	m.Memory.Write(0xbf10, 0x76, false)
	m.Memory.Write(0xbf00, 0x10, false)
	m.Memory.Write(0xbf01, 0xbf, false)
	m.CPU.SetPC(uint16(a.Symbols["main"]))
	m.CPU.SetSP(0xbf00)
	outputSymbol := a.Symbols["native_putchar"]
	if windows {
		outputSymbol = a.Symbols["zx__console__z3_window_put"]
	}
	if outputSymbol == 0 {
		t.Fatal("missing output symbol")
	}
	var output []byte
	minSP := uint16(0xbf00)
	for i := 0; i < 200000000; i++ {
		if m.CPU.PC() == 0xbf10 && m.CPU.SP() == 0xbf02 {
			break
		}
		if m.CPU.PC() == uint16(outputSymbol) {
			output = append(output, byte(m.CPU.AF()>>8))
		}
		if m.CPU.PC() == uint16(a.Symbols["tui_read_line_or_eof"]) {
			t.Fatal("unexpected input request")
		}
		m.CPU.DoOpcode()
		if m.CPU.SP() < minSP {
			minSP = m.CPU.SP()
		}
	}
	if m.CPU.PC() != 0xbf10 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
		t.Fatalf("boundary PC/SP=%04x/%04x output=%q", m.CPU.PC(), m.CPU.SP(), output)
	}
	if minSP < 0xbe00 {
		t.Fatalf("native stack exceeded reservation: %04x", minSP)
	}
	expected := byte(16)
	if windows {
		expected = 32
	}
	if m.Memory.RAM[0][1]&48 != expected {
		t.Fatal("screen capability flags", m.Memory.RAM[0][1])
	}
	for _, needle := range []string{"Performed 368 tests.", "Passed: 349, Failed: 0, Print tests: 19", "Didn't crash: hooray!"} {
		if !bytes.Contains(output, []byte(needle)) {
			t.Fatalf("missing %q: %s", needle, output)
		}
	}
	for _, needle := range []string{"ERROR [", "Z3 error"} {
		if bytes.Contains(output, []byte(needle)) {
			t.Fatalf("failure: %s", output)
		}
	}
	dynamic := int(story[14])<<8 | int(story[15])
	for i := dynamic; i < len(story); i++ {
		if m.Memory.RAM[pages[i/16384]][i%16384] != story[i] {
			t.Fatalf("static modified at %d", i)
		}
	}
	refDir := t.TempDir()
	core, err := os.ReadFile(filepath.Join(root, "examples/zmachine/zork-west-demo/zvm.nanz"))
	if err != nil {
		t.Fatal(err)
	}
	if windows {
		old := []byte("(story[1] & 15) | 16")
		if !bytes.Contains(core, old) {
			t.Fatal("reference capability anchor missing")
		}
		core = bytes.ReplaceAll(core, old, []byte("(story[1] & 15) | 32"))
	}
	if err = os.WriteFile(filepath.Join(refDir, "zvm.nanz"), core, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(refDir, "zork-west-demo.z3"), story, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(refDir, "text"), 0700); err != nil {
		t.Fatal(err)
	}
	printer, err := os.ReadFile(filepath.Join(root, "stdlib/text/print.nanz"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(refDir, "text/print.nanz"), printer, 0600); err != nil {
		t.Fatal(err)
	}
	reference, err := exec.Command(vmPath, "-H", filepath.Join(refDir, "zvm.nanz")).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, reference) {
		t.Fatalf("native/VM differs native=%q VM=%q", output, reference)
	}
	if d := os.Getenv("MINZ_Z3_TRANSCRIPTS"); d != "" {
		if err = os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(d, fmt.Sprintf("czech-native-windows-%v.txt", windows)), output, 0644); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(d, fmt.Sprintf("czech-vm-windows-%v.txt", windows)), reference, 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("349 assertions, 19 print cases; %d bytes byte-exact VM; resident %d; stack %d; %d T", len(output), len(a.Binary), 0xbf00-int(minSP), m.CPU.Tstates())
}
