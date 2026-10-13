package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// Real Nanz interpreter executes an independent Inform-built Z3 story from
// physical bank0. Host I/O is replaced by explicit native capture boundaries.
func TestZMachineBankedInterpreterNative(t *testing.T) {
	testZMachineBankedInterpreterNative(t, false, "")
}
func TestZMachineBankedHighMemoryNative(t *testing.T) {
	testZMachineBankedInterpreterNative(t, true, "")
}
func TestZMachineBankedZorkStartupNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY to an independently supplied Zork I story")
	}
	testZMachineBankedInterpreterNative(t, false, path)
}
func testZMachineBankedInterpreterNative(t *testing.T, highMemory bool, externalStory string) {
	root := filepath.Clean("../../..")
	story, err := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures/stream3.z3"))
	if err != nil {
		t.Fatal(err)
	}
	if highMemory {
		fixture := filepath.Join(t.TempDir(), "high.z3")
		cmd := exec.Command("python3", filepath.Join(root, "examples/zmachine/z3-fixtures/high_memory.py"), fixture, "--bytes", "98302")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v %s", err, out)
		}
		story, err = os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
	}
	if externalStory != "" {
		story, err = os.ReadFile(externalStory)
		if err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "core.nanz")
	cmd := exec.Command("python3", filepath.Join(root, "scripts/zmachine/prepare_native_core.py"), path, "--story-bytes", strconv.Itoa(len(story)))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{StdlibDir: filepath.Join(root, "stdlib")})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + steps.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	if 0x5b00+len(a.Binary) > 0xbe00 {
		t.Fatalf("resident core reaches %04x; native stack starts be00", 0x5b00+len(a.Binary))
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
	m.Memory.Write(0xbf00, 0, false)
	m.Memory.Write(0xbf01, 0x50, false)
	m.CPU.SetPC(uint16(a.Symbols["main"]))
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	m.CPU.SetTstates(0)
	for i := 0; i < 100000000; i++ {
		if m.CPU.PC() == 0x5000 && m.CPU.SP() == 0xbf02 {
			break
		}
		m.CPU.DoOpcode()
	}
	if m.CPU.PC() != 0x5000 || m.CPU.SP() != 0xbf02 {
		t.Fatalf("PC/SP=%04x/%04x vmPC=%02x%02x%02x steps=%02x%02x", m.CPU.PC(), m.CPU.SP(), m.Memory.Read(uint16(a.Symbols["pc"])+2), m.Memory.Read(uint16(a.Symbols["pc"])+1), m.Memory.Read(uint16(a.Symbols["pc"])), m.Memory.Read(uint16(a.Symbols["steps"])+1), m.Memory.Read(uint16(a.Symbols["steps"])))
	}
	if m.CPU.IFF1() || m.CPU.IFF2() || m.Memory.PagingState() != 0x10 {
		t.Fatal("native boundary state lost")
	}
	lengthAt := uint16(a.Symbols["native_output_length"])
	length := int(m.Memory.Read(lengthAt)) | int(m.Memory.Read(lengthAt+1))<<8
	if length > 1024 {
		t.Fatal("invalid capture length", length)
	}
	got := make([]byte, length)
	for i := range got {
		got[i] = m.Memory.Read(uint16(a.Symbols["native_output"]) + uint16(i))
	}
	wantPath := "stream3.out"
	if highMemory {
		wantPath = "high_memory.out"
	}
	want, err := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures", wantPath))
	if err != nil {
		t.Fatal(err)
	}
	if !highMemory && externalStory == "" {
		for _, expected := range []struct {
			address int
			data    []byte
		}{
			{0x492, []byte{0, 3, 65, 13, 66}}, {0x4b2, []byte{0, 2, 88, 90}}, {0x4d2, []byte{0, 1, 89}},
		} {
			if !bytes.Equal(m.Memory.RAM[0][expected.address:expected.address+len(expected.data)], expected.data) {
				t.Fatalf("stream RAM mismatch at %04x", expected.address)
			}
		}
	}
	if externalStory != "" {
		for _, phrase := range []string{"ZORK I: The Great Underground Empire", "West of House", "There is a small mailbox here."} {
			if !bytes.Contains(got, []byte(phrase)) {
				t.Fatalf("missing %q: output %q", phrase, got)
			}
		}
		if bytes.Contains(got, []byte("Z3 error")) {
			t.Fatalf("interpreter failure %q", got)
		}
	} else if !bytes.Equal(got, want) {
		t.Fatalf("output got %q want %q; vmPC=%02x%02x%02x steps=%d", got, want, m.Memory.Read(uint16(a.Symbols["pc"])+2), m.Memory.Read(uint16(a.Symbols["pc"])+1), m.Memory.Read(uint16(a.Symbols["pc"])), int(m.Memory.Read(uint16(a.Symbols["steps"]))))
	}
	t.Logf("native interpreter %d bytes; %d T; output %q", len(a.Binary), m.CPU.Tstates(), got)
}
