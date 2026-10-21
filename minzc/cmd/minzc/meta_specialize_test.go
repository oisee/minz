package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMetaSpecializeMeasuredWinnerAndOutput(t *testing.T) {
	for _, ext := range []string{".a80", ".bin", ".asm"} {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "choice.nanz")
			definitions := `fun @build()->void {
    if meta_variant(c"example.copy",2,0)==0 {
     emit(c"fun copy(x:u8)->u8 {var n:u8=0 var result:u8=0 while n<x {result=result+1 n=n+1} return result}")
    } else {emit(c"fun copy(x:u8)->u8 {return x}")}
   }`
			if err := os.WriteFile(filepath.Join(dir, "generator.nanz"), []byte(definitions), 0600); err != nil {
				t.Fatal(err)
			}
			text := `import generator
@build()
assert copy(0)==0
assert copy(1)==1
assert copy(17)==17
assert copy(255)==255
`
			if err := os.WriteFile(src, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			platform := "generic"
			if ext == ".asm" {
				platform = "zxspectrum"
			}
			f := metaSpecializeFlags{Goal: "size", Target: platform, Evidence: filepath.Join(dir, "evidence"), Output: filepath.Join(dir, "winner"+ext), Limit: 32}
			if err := runMetaSpecialize(src, f); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(f.Evidence, "selection.json"))
			if err != nil {
				t.Fatal(err)
			}
			var report metaSpecializeEvidence
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}
			if ext == ".asm" && report.Candidates[0].Bytes >= 49152 {
				t.Fatal("measured fixed-size Spectrum container")
			}
			if len(report.Candidates) != 2 || report.Selected != 1 || report.Candidates[1].Bytes >= report.Candidates[0].Bytes {
				t.Fatal(report)
			}
			chosen := report.Candidates[report.Selected]
			bin, err := os.ReadFile(filepath.Join(f.Evidence, chosen.Directory, "program.bin"))
			if err != nil {
				t.Fatal(err)
			}
			if len(bin) != chosen.Bytes || fmt.Sprintf("%x", sha256.Sum256(bin)) != chosen.BinarySHA256 {
				t.Fatal("binary metric mismatch")
			}
			artifact := "program.a80"
			if ext == ".bin" {
				artifact = "program.bin"
			}
			expected, err := os.ReadFile(filepath.Join(f.Evidence, chosen.Directory, artifact))
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(f.Output)
			if err != nil || string(actual) != string(expected) {
				t.Fatal("selected output mismatch", err)
			}
			if strings.Contains(string(expected), "meta_variant") {
				t.Fatal("host leaked into output")
			}
			if err := runMetaSpecialize(src, f); err == nil {
				t.Fatal("existing evidence overwritten")
			}
		})
	}
}

func TestMetaSpecializeCancelsIncompleteSearch(t *testing.T) {
	for _, body := range []string{
		`if meta_variant(c"x",2,0)==1 {meta_error(c"candidate refused")} emit(c"fun f()->u8 {return 0}")`,
		`if meta_variant(c"x",2,0)==1 {emit(c"fun f()->u8 {return 1}")} else {emit(c"fun f()->u8 {return 0}")}`,
		`if meta_variant(c"x",2,0)==1 {meta_variant(c"new",2,0)} emit(c"fun f()->u8 {return 0}")`,
		`if meta_variant(c"x",2,0)==1 {emit(c"assert f()==1")} emit(c"fun f()->u8 {return 0}")`,
		`if meta_variant(c"x",2,0)==1 {emit(c"""sandbox "generated" {assert f()==0}""")} emit(c"fun f()->u8 {return 0}")`,
	} {
		dir := t.TempDir()
		src := filepath.Join(dir, "failure.nanz")
		if err := os.WriteFile(src, []byte("fun @build()->void {"+body+"}\n@build()\nassert f()==0\n"), 0600); err != nil {
			t.Fatal(err)
		}
		f := metaSpecializeFlags{Goal: "size", Target: "generic", Evidence: filepath.Join(dir, "evidence"), Output: filepath.Join(dir, "winner.bin"), Limit: 32}
		if err := runMetaSpecialize(src, f); err == nil {
			t.Fatalf("incomplete/invalid candidate accepted: %s", body)
		}
		for _, path := range []string{f.Output, f.Evidence} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("partial winner published", path)
			}
		}
	}
}

func TestMetaSelectionsCombinationOrderAndLimit(t *testing.T) {
	vs := []nanz.MetaVariant{{Key: "a", Alternatives: 2, Default: 1}, {Key: "b", Alternatives: 3, Default: 2}}
	got, err := metaSelections(vs, 6)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]uint8{{"a": 1, "b": 2}, {"a": 1, "b": 0}, {"a": 1, "b": 1}, {"a": 0, "b": 2}, {"a": 0, "b": 0}, {"a": 0, "b": 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if _, err := metaSelections(vs, 5); err == nil {
		t.Fatal("combination cap ignored")
	}
	got[0]["a"] = 99
	if got[1]["a"] != 1 {
		t.Fatal("selection alias")
	}
	if _, err := metaSelections(nil, 32); err == nil {
		t.Fatal("empty choice set accepted")
	}
}

func TestMetaSpecializeSpectrumSpeedAndBudget(t *testing.T) {
	for _, tc := range []struct {
		goal     string
		budget   int64
		selected int
		success  bool
	}{
		// The 28200-T budget must sit between the two candidates' measured max
		// full busy T (27718 unrolled, 28689 compact after pair-move codegen) so
		// only the faster candidate fits.
		{"size", 0, 1, true}, {"speed", 0, 0, true}, {"budget", 32000, 1, true}, {"budget", 28200, 0, true}, {"budget", 1, -1, false},
	} {
		t.Run(fmt.Sprintf("%s-%d", tc.goal, tc.budget), func(t *testing.T) {
			dir := t.TempDir()
			f := metaSpecializeFlags{Goal: tc.goal, BudgetTStates: tc.budget, Target: "zxspectrum", Evidence: filepath.Join(dir, "evidence"), Output: filepath.Join(dir, "winner.bin"), Stdlib: "../../../stdlib", Workload: "../../../reports/advanced-demos/meta-specialization/workload.json", Limit: 32}
			err := runMetaSpecialize("../../../reports/advanced-demos/meta-specialization/source.nanz", f)
			if !tc.success {
				if err == nil {
					t.Fatal("unmet budget accepted")
				}
				if _, err := os.Stat(f.Output); !os.IsNotExist(err) {
					t.Fatal("ineligible output published")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(f.Evidence, "selection.json"))
			if err != nil {
				t.Fatal(err)
			}
			var report metaSpecializeEvidence
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}
			if report.Selected != tc.selected || len(report.Candidates) != 2 || report.SpectrumWorkload == nil || report.WorkloadSHA256 == "" {
				t.Fatal(report)
			}
			for _, candidate := range report.Candidates {
				if candidate.Run == nil || candidate.Run.Publications != 64 || len(candidate.Run.FullBusyCycles) != 63 {
					t.Fatal("incomplete native measurement")
				}
			}
			selected := report.Candidates[report.Selected]
			out, err := os.ReadFile(f.Output)
			if err != nil || fmt.Sprintf("%x", sha256.Sum256(out)) != selected.BinarySHA256 {
				t.Fatal("output is not measured winner", err)
			}
			if tc.goal == "speed" && selected.Bytes <= report.Candidates[1].Bytes {
				t.Fatal("pure Speed unexpectedly constrained bytes")
			}
			if tc.goal == "budget" && (selected.Run.MaxFullBusyTStates > tc.budget || selected.Run.MaxPublicationIntervalFrames > 1) {
				t.Fatal("budget or consecutive-publication condition ignored")
			}
		})
	}
}

func TestMetaSpecializeSpectrumFlagRefusals(t *testing.T) {
	for _, f := range []metaSpecializeFlags{
		{Goal: "speed", Target: "zxspectrum", Limit: 32},
		{Goal: "budget", Target: "zxspectrum", BudgetTStates: 69889, Workload: "missing.json", Limit: 32},
		{Goal: "size", Target: "zxspectrum", BudgetTStates: 1, Limit: 32},
		{Goal: "speed", Target: "generic", Workload: "missing.json", Limit: 32},
	} {
		if err := runMetaSpecialize("not-read.nanz", f); err == nil {
			t.Fatal("invalid native search flags accepted")
		}
	}
}

func TestMetaSpecializeSnapshotNeedsWorkload(t *testing.T) {
	dir := t.TempDir()
	f := metaSpecializeFlags{Goal: "size", Target: "zxspectrum", Limit: 32, Evidence: filepath.Join(dir, "evidence"), Output: filepath.Join(dir, "bad.sna")}
	if err := runMetaSpecialize("not-read.nanz", f); err == nil || !strings.Contains(err.Error(), ".sna requires") {
		t.Fatal("snapshot without explicit boot workload accepted", err)
	}
	if _, err := os.Stat(f.Output); !os.IsNotExist(err) {
		t.Fatal("refused snapshot created output")
	}
}

func TestMetaSpecializeSpectrumSnapshotBoot(t *testing.T) {
	for _, goal := range []string{"speed", "budget"} {
		t.Run(goal, func(t *testing.T) {
			dir := t.TempDir()
			f := metaSpecializeFlags{Goal: goal, Target: "zxspectrum", Evidence: filepath.Join(dir, "evidence"), Output: filepath.Join(dir, "winner.sna"), Stdlib: "../../../stdlib", Workload: "../../../reports/advanced-demos/meta-specialization/workload.json", Limit: 32}
			if goal == "budget" {
				f.BudgetTStates = 32000
			}
			if err := runMetaSpecialize("../../../reports/advanced-demos/meta-specialization/source.nanz", f); err != nil {
				t.Fatal(err)
			}
			var report metaSpecializeEvidence
			payload, err := os.ReadFile(filepath.Join(f.Evidence, "selection.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(payload, &report); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(f.Output)
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != report.SnapshotSHA256 {
				t.Fatal("selected snapshot differs from evidence")
			}
			snap, err := formats.ParseSNA(data)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(f.Evidence, "selected.bin"))
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(raw)) != report.Candidates[report.Selected].BinarySHA256 {
				t.Fatal("measured BIN changed")
			}
			for i, b := range raw {
				if snap.RAM[0x4000+i] != b {
					t.Fatal("snapshot did not embed measured BIN", i)
				}
			}
			asm, err := os.ReadFile(filepath.Join(f.Evidence, "selected.a80"))
			if err != nil {
				t.Fatal(err)
			}
			a, err := z80asm.NewAssembler().AssembleString(string(asm))
			if err != nil || len(a.Errors) > 0 {
				t.Fatal(err)
			}
			w := report.SpectrumWorkload
			rom, err := os.ReadFile(w.ROM)
			if err != nil {
				t.Fatal(err)
			}
			m, err := spectrum.New48K(rom)
			if err != nil {
				t.Fatal(err)
			}
			formats.ApplySnapshot(m, snap)
			if m.CPU.PC() != uint16(a.Symbols[w.Entry]) || m.CPU.SP() != w.Stack || m.CPU.IM() != 1 || m.CPU.IY() != 0x5c3a || m.CPU.IFF1() || m.CPU.IFF2() {
				t.Fatal("incorrect boot state")
			}
			fetched := 0
			for frames := 0; frames < w.MaxHardwareFrames && fetched < 64; frames++ {
				m.RunFrame()
				if m.Memory.ReadByteInternal(uint16(a.Symbols["actor_visible"])) == 0 {
					continue
				}
				// Main advances frame immediately after publication and stages the next
				// placement before HALT; the displayed image belongs to frame-1.
				pose := (int(m.Memory.ReadByteInternal(uint16(a.Symbols["frame"]))) - 1) & 31
				screen := make([]byte, 6912)
				for i := range screen {
					screen[i] = m.Memory.ReadByteInternal(0x4000 + uint16(i))
				}
				if fmt.Sprintf("%x", sha256.Sum256(screen)) != w.ExpectedScreenSHA256[pose] {
					t.Fatal("snapshot screen oracle", frames, pose)
				}
				rgba := make([]byte, 256*192*4)
				fb := m.ULA.Framebuffer()
				for y := 0; y < 192; y++ {
					offset := ((y+m.Mode.BorderTop)*m.Mode.TotalPixelWidth + m.Mode.BorderLeft) * 4
					copy(rgba[y*256*4:(y+1)*256*4], fb[offset:offset+256*4])
				}
				if fmt.Sprintf("%x", sha256.Sum256(rgba)) != w.ExpectedRGBASHA256[pose] {
					t.Fatal("snapshot fetched image oracle", frames, pose)
				}
				fetched++
			}
			if fetched != 64 {
				t.Fatal("snapshot did not produce 64 verified native frames", fetched)
			}
			t.Logf("%s: selected=%d bytes=%d; 64 exact full-screen/fetched ULA frames from saved SNA", goal, report.Selected, len(raw))
		})
	}
}
