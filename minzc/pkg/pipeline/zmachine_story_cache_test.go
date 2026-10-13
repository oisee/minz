package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestZMachineStoryCacheNative(t *testing.T) {
	script := `import sys,json,base64,os
sys.path.insert(0,'../../../scripts/zmachine')
from story_lz_layout import build
from native_story_cache import emit
from pathlib import Path
path=os.environ.get('MINZ_Z3_CACHE_STORY')
if path:story=Path(path).read_bytes()
else:
 import random
 story=bytearray(random.Random(42).randbytes(70000))
 story[0]=3;story[14:16]=(1024).to_bytes(2,'big')
 # Repeated blocks exercise compressed/raw mixed descriptors.
 story[3072:5120]=bytes(2048)
 story=bytes(story)
image,m=build(story,2048,archive_blocks=True)
print(json.dumps({'Story':base64.b64encode(story).decode(),'Image':base64.b64encode(image).decode(),'Source':emit(m),'Dynamic':m['dynamic_bytes']}))`
	out, err := exec.Command("python3", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Story, Image []byte
		Source       string
		Dynamic      int
	}
	if err := json.Unmarshal(out, &fixture); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "cache.nanz")
	source := fixture.Source + "\nfun main()->u8 { return native_story_byte(0,0) }\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(source, path, nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	pages := []int{0, 1, 3, 4, 6, 7}
	for i, b := range fixture.Image {
		m.Memory.RAM[pages[i/16384]][i%16384] = b
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x8000+uint16(i), b, false)
	}
	m.Memory.SetPagingForce(0x10)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	before := m.Memory.RAM
	calls := 0
	originalRead := false
	read := func(address int) {
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		entry := "native_story_byte"
		if originalRead {
			entry = "native_original_byte"
		}
		m.CPU.SetPC(uint16(a.Symbols[entry]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetHL(uint16(address))
		m.CPU.SetAF(uint16(address>>16) << 8)
		if originalRead {
			m.CPU.SetBC(0x1234)
			m.CPU.SetDE(0x5678)
			m.CPU.SetIX(0x9abc)
			m.CPU.SetIY(0xdef0)
		}
		for i := 0; i < 100000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() {
			t.Fatalf("boundary address%d", address)
		}
		if byte(m.CPU.AF()>>8) != fixture.Story[address] {
			t.Fatalf("address%d got%02x want%02x", address, byte(m.CPU.AF()>>8), fixture.Story[address])
		}
		if originalRead && (m.CPU.BC() != 0x1234 || m.CPU.DE() != 0x5678 || m.CPU.HL() != uint16(address) || m.CPU.IX() != 0x9abc || m.CPU.IY() != 0xdef0) {
			t.Fatal("original reader clobbers streaming ABI")
		}
		calls++
	}
	for i := range fixture.Story {
		read(i)
	}
	// Force block changes and high64 carry, then revisit prior blocks.
	for _, i := range []int{fixture.Dynamic, len(fixture.Story) - 1, 65535, 65536, fixture.Dynamic + 2047, fixture.Dynamic + 2048, fixture.Dynamic, 0} {
		if i < len(fixture.Story) {
			read(i)
		}
	}
	// Alternate original/static reads to force both cache namespaces.
	originalRead = true
	for i := 0; i < fixture.Dynamic; i++ {
		read(i)
	}
	for _, i := range []int{0, fixture.Dynamic - 1, 2047, 2048} {
		if i < fixture.Dynamic {
			read(i)
		}
	}
	originalRead = false
	read(fixture.Dynamic)
	originalRead = true
	read(0)
	originalRead = false
	original63 := fixture.Story[63]
	fixture.Story[63] ^= 255
	m.Memory.RAM[0][63] = fixture.Story[63]
	before[0][63] = fixture.Story[63]
	read(63)
	modified := fixture.Story[63]
	fixture.Story[63] = original63
	originalRead = true
	read(63)
	fixture.Story[63] = modified
	originalRead = false
	read(63)
	cache := uint16(a.Symbols["native_story_cache"])
	for p := 0; p < 8; p++ {
		for i, b := range before[p] {
			if p == 2 {
				address := uint16(0x8000 + i)
				if address >= cache && address < cache+2048 {
					continue
				}
				key := uint16(a.Symbols["native_story_cache_key"])
				miss := uint16(a.Symbols["native_story_cache_misses"])
				if address == key || address >= miss && address < miss+2 || address == uint16(a.Symbols["zx__lz__lz_bits"]) || address == uint16(a.Symbols["zx__lz__lz_control"]) {
					continue
				}
				if i >= 0x3ed0 && i <= 0x3f01 {
					continue
				}
			}
			if m.Memory.RAM[p][i] != b {
				t.Fatalf("unexpected write bank%d offset%x", p, i)
			}
		}
	}
	t.Logf("%d reads verified, module including 2K cache %d B", calls, len(a.Binary))
}
