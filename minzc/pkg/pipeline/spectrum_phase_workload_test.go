package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testSpectrumPhaseOracle(t *testing.T) (SpectrumPhaseWorkload, []byte, string) {
	t.Helper()
	rom, err := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("../../../reports/advanced-demos/meta-specialization/source.nanz")
	if err != nil {
		t.Fatal(err)
	}
	w := SpectrumPhaseWorkload{Schema: "minz.spectrum-phase-workload/v1", ROMSHA256: fmt.Sprintf("%x", sha256.Sum256(rom)), Entry: "main", Stack: 0xeff0, Publications: 64, MaxHardwareFrames: 200, Prepare: "prepare_frame", Publish: "draw_frame"}
	// Archived shape bytes and a direct pixel-coordinate model; neither generated
	// setters, target RAM nor the Nanz descriptor is used to form expectations.
	rows := []string{"00183c7e7e3c1800", "183c7effff7e3c18", "3c7effa5a5ff7e3c", "7effa55a5aa5ff7e", "7effa55a5aa5ff7e", "3c7effa5a5ff7e3c", "183c7effff7e3c18", "00183c7e7e3c1800"}
	wave := []int{0, 1, 1, 0, 0, -1, -1, 0}
	w.ExpectedGlobals = map[string][][]byte{"frame": {}, "actor_visible": {}, "tv": {}}
	for update := 0; update < 64; update++ {
		f := update & 31
		w.ExpectedGlobals["frame"] = append(w.ExpectedGlobals["frame"], []byte{byte(f)})
		w.ExpectedGlobals["actor_visible"] = append(w.ExpectedGlobals["actor_visible"], []byte{1})
		w.ExpectedGlobals["tv"] = append(w.ExpectedGlobals["tv"], []byte{byte(f), byte(f >> 4)})
		screen := make([]byte, 6912)
		for i := 6144; i < len(screen); i++ {
			screen[i] = 0x46
		}
		rgba := make([]byte, 256*192*4)
		for i := 3; i < len(rgba); i += 4 {
			rgba[i] = 255
		}
		for row, encoded := range rows {
			data, err := hex.DecodeString(encoded)
			if err != nil {
				t.Fatal(err)
			}
			shift := wave[(f+row)&7]
			if f >= 16 {
				h := byte(f*13+row*17) ^ 0x5a
				h ^= h >> 3
				if h&3 == 0 {
					shift++
				}
				if h&3 == 1 {
					shift--
				}
			}
			x0 := (4 + (f & 15) + shift) * 8
			y := 56 + (f>>4)*8 + row
			for col, b := range data {
				address := (y&0xc0)<<5 | (y&7)<<8 | (y&0x38)<<2 | (x0/8 + col)
				screen[address] = b
				for bit := 0; bit < 8; bit++ {
					if b&(128>>bit) != 0 {
						offset := (y*256 + x0 + col*8 + bit) * 4
						rgba[offset], rgba[offset+1] = 255, 255
					}
				}
			}
		}
		w.ExpectedScreenSHA256 = append(w.ExpectedScreenSHA256, fmt.Sprintf("%x", sha256.Sum256(screen)))
		w.ExpectedRGBASHA256 = append(w.ExpectedRGBASHA256, fmt.Sprintf("%x", sha256.Sum256(rgba)))
	}
	return w, rom, string(src)
}
func testSpectrumPhaseCompile(t *testing.T, source string, choice uint8) (string, []byte) {
	t.Helper()
	h, err := nanz.ParseWithOpts(source, "phase-scene", nanz.ParseOpts{OptSize: true, StdlibDir: "../../../stdlib", MetaVariants: nanz.NewMetaVariantContext(map[string]uint8{"zx.sprite.preparation.Diamond": choice})})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{OptSize: true, ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	asm := "ORG 0x8000\n" + steps.Assembly
	a, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	return asm, a.Binary
}
func TestSpectrumPhaseWorkloadNative(t *testing.T) {
	w, rom, source := testSpectrumPhaseOracle(t)
	root := os.Getenv("MINZ_SPECTRUM_PHASE_EXPORT")
	if root != "" {
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
		romPath, err := filepath.Abs("../../cmd/mzx/roms/48.rom")
		if err != nil {
			t.Fatal(err)
		}
		w.ROM = romPath
		bytes, err := json.MarshalIndent(w, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "workload.json"), append(bytes, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "source.nanz"), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var results []*SpectrumPhaseEvidence
	for _, choice := range []uint8{0, 1} {
		asm, bin := testSpectrumPhaseCompile(t, source, choice)
		r, err := RunSpectrumPhaseWorkload(asm, bin, rom, w)
		if err != nil {
			t.Fatal(err)
		}
		if r.Publications != 64 || r.FetchedFrames != 64 || len(r.FullBusyCycles) != 63 || r.MaxPublicationIntervalFrames != 1 {
			t.Fatal(r)
		}
		for _, phase := range r.Phases {
			if phase.ReturnSP != phase.EntrySP+2 || phase.Function == w.Prepare && phase.ScreenWrites != 0 {
				t.Fatal(phase)
			}
		}
		t.Logf("choice %d bytes=%d full-busy-max=%d hardware=%d IRQ=%d", choice, len(bin), r.MaxFullBusyTStates, r.HardwareFrames, r.ROMInterrupts)
		if root != "" {
			dir := filepath.Join(root, fmt.Sprintf("candidate-%d", choice))
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			run, err := json.MarshalIndent(r, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{"program.a80": []byte(asm), "program.bin": bin, "native-run.json": append(run, '\n')} {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		results = append(results, r)
	}
	if results[0].MaxFullBusyTStates >= results[1].MaxFullBusyTStates {
		t.Fatal("measured loop tradeoff not observed")
	}
}
func TestSpectrumPhaseWorkloadRefusals(t *testing.T) {
	w, rom, source := testSpectrumPhaseOracle(t)
	asm, bin := testSpectrumPhaseCompile(t, source, 0)
	invalid := w
	invalid.ExpectedRGBASHA256 = append([]string(nil), w.ExpectedRGBASHA256...)
	invalid.ExpectedRGBASHA256[0] = strings.Repeat("0", 64)
	if r, err := RunSpectrumPhaseWorkload(asm, bin, rom, invalid); err == nil || r != nil {
		t.Fatal("bad ULA oracle accepted")
	}
	invalid = w
	invalid.ExpectedGlobals = map[string][][]byte{}
	for k, v := range w.ExpectedGlobals {
		invalid.ExpectedGlobals[k] = append([][]byte(nil), v...)
	}
	invalid.ExpectedGlobals["frame"][0] = []byte{99}
	if r, err := RunSpectrumPhaseWorkload(asm, bin, rom, invalid); err == nil || r != nil || !strings.Contains(err.Error(), "global") {
		t.Fatal("incorrect public state oracle accepted", err)
	}
	invalid = w
	invalid.Prepare, invalid.Publish = invalid.Publish, invalid.Prepare
	if r, err := RunSpectrumPhaseWorkload(asm, bin, rom, invalid); err == nil || r != nil {
		t.Fatal("out-of-order phase contract accepted")
	}
	// Removing frame synchronization must not become an apparent speed win.
	unsynchronized := strings.Replace(source, "EI / HALT / DI", "EI / DI", 1)
	ua, ub := testSpectrumPhaseCompile(t, unsynchronized, 0)
	if r, err := RunSpectrumPhaseWorkload(ua, ub, rom, w); err == nil || r != nil {
		t.Fatal("unsynchronized publication accepted")
	}
	changed := append([]byte(nil), bin...)
	changed[0] ^= 1
	if r, err := RunSpectrumPhaseWorkload(asm, changed, rom, w); err == nil || r != nil {
		t.Fatal("different native BIN accepted")
	}
	for _, code := range []string{"LD A, (0x4000) / LD (0x4000), A", "LD HL, (0x4000) / LD (0x4000), HL"} {
		mutant := strings.Replace(source, "fun prepare_frame(frame:u8)->bool {", "fun prepare_frame(frame:u8)->bool {\n asm z80 {"+code+"}", 1)
		a, b := testSpectrumPhaseCompile(t, mutant, 0)
		if r, err := RunSpectrumPhaseWorkload(a, b, rom, w); err == nil || r != nil || !strings.Contains(err.Error(), "preparation wrote screen") {
			t.Fatalf("transient/same-value store not rejected: %v", err)
		}
	}
}
