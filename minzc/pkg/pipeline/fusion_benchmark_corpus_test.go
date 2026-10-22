package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// This is the benchmark's actual frozen corpus, not a parallel set of examples.
// The external matrix additionally covers C, SDCC and all three Nanz profiles.
func TestFusionBenchmarkCorpus(t *testing.T) {
	bench := "../../../research/c89-benchmark"
	help, err := exec.Command("python3", filepath.Join(bench, "run.py"), "--help").CombinedOutput()
	if err != nil || !bytes.Contains(help, []byte("main,heldout,all,fusion")) {
		t.Fatalf("benchmark driver must expose --suite fusion: %v\n%s", err, help)
	}
	root := filepath.Join(bench, "fusion")
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	type entry struct{ File, SHA256, Workload, WorkloadSHA256 string }
	// JSON uses underscores for the workload hash, unlike the source hash.
	type manifestEntry struct {
		File           string `json:"file"`
		SHA256         string `json:"sha256"`
		Workload       string `json:"workload"`
		WorkloadSHA256 string `json:"workload_sha256"`
	}
	var sources []entry
	hashes := map[string]string{}
	for _, folder := range []string{"cases", "nanz-idiomatic", "hand-asm"} {
		var manifest struct {
			Cases []manifestEntry `json:"cases"`
		}
		if err := json.Unmarshal(read(filepath.Join(root, folder, "manifest.json")), &manifest); err != nil {
			t.Fatal(err)
		}
		if len(manifest.Cases) != 8 {
			t.Fatalf("%s: want eight frozen cases", folder)
		}
		seen := map[string]bool{}
		for _, e := range manifest.Cases {
			name := e.File[:len(e.File)-len(filepath.Ext(e.File))]
			if seen[name] {
				t.Fatalf("duplicate case %s", name)
			}
			seen[name] = true
			if got := fmt.Sprintf("%x", sha256.Sum256(read(filepath.Join(root, folder, e.File)))); got != e.SHA256 {
				t.Fatalf("unfrozen source %s", e.File)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(read(filepath.Join(root, e.Workload)))); got != e.WorkloadSHA256 {
				t.Fatalf("unfrozen workload %s", e.Workload)
			}
			if old, ok := hashes[name]; ok && old != e.WorkloadSHA256 {
				t.Fatalf("different language workloads: %s", name)
			}
			hashes[name] = e.WorkloadSHA256
			if folder == "nanz-idiomatic" {
				sources = append(sources, entry{e.File, e.SHA256, e.Workload, e.WorkloadSHA256})
			}
		}
	}
	if len(hashes) != 8 {
		t.Fatal("reference case names differ")
	}
	for _, source := range sources {
		t.Run(source.File, func(t *testing.T) {
			var workload struct {
				Sizes   map[string]int `json:"sizes"`
				Samples []struct {
					ID       string            `json:"id"`
					Inputs   map[string][]byte `json:"inputs"`
					Expected uint16            `json:"expected"`
					After    map[string][]byte `json:"after"`
				} `json:"samples"`
			}
			if err := json.Unmarshal(read(filepath.Join(root, source.Workload)), &workload); err != nil {
				t.Fatal(err)
			}
			edges := map[uint16]bool{}
			for _, s := range workload.Samples {
				edges[binary.LittleEndian.Uint16(s.Inputs["n"])] = true
			}
			for _, n := range []uint16{0, 1, 255, 256} {
				if !edges[n] {
					t.Fatalf("missing length %d", n)
				}
			}
			src := string(read(filepath.Join(root, "nanz-idiomatic", source.File)))
			parse := func() *hir.Module {
				m, err := nanz.Parse(src, source.File)
				if err != nil {
					t.Fatal(err)
				}
				return m
			}
			raw := hir.LowerModule(parse())
			steps, err := CompileHIRSteps(parse(), DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			// Fusion must consume the iterator operators. Named min/max callbacks
			// may still be real calls; they are measured, not forbidden here.
			if regexp.MustCompile(`(?m)^\s*CALL\s+(map|filter|fold|forEach|iter|zip|take|skip|lambda_[0-9]+)\s*$`).MatchString(steps.Assembly) {
				t.Fatal("unfused iterator or simple lambda call")
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil || len(a.Errors) != 0 {
				t.Fatalf("assemble: %v %v", err, a.Errors)
			}
			for _, s := range workload.Samples {
				for tier, m := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
					vm := mir2.NewVM(m)
					for name, data := range s.Inputs {
						vm.WriteHeapSlice(vm.GlobalAddr(name), data)
					}
					vm.WriteHeapSlice(vm.GlobalAddr("result"), []byte{0xef, 0xbe})
					if _, err := vm.Call("execute", nil); err != nil {
						t.Fatalf("%s %s: %v", s.ID, tier, err)
					}
					if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)); got != s.Expected {
						t.Fatalf("%s %s: got %d want %d", s.ID, tier, got, s.Expected)
					}
					for name, want := range s.After {
						if !bytes.Equal(vm.ReadHeap(vm.GlobalAddr(name), len(want)), want) {
							t.Fatalf("%s %s: memory %s", s.ID, tier, name)
						}
					}
				}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				addr := func(name string) int {
					v, ok := a.Symbols[name]
					if !ok {
						t.Fatalf("missing symbol %s", name)
					}
					return v
				}
				for name, data := range s.Inputs {
					copy(mem[addr(name):], data)
				}
				binary.LittleEndian.PutUint16(mem[addr("result"):], 0xbeef)
				binary.LittleEndian.PutUint16(mem[0xeffe:], 0xf100)
				z.SetRegisters(emulator.Registers{PC: uint16(addr("execute")), SP: 0xeffe})
				for i := 0; i < 1000000 && z.GetPC() != 0xf100; i++ {
					z.Step()
				}
				regs := z.GetRegisters()
				if regs.PC != 0xf100 || regs.SP != 0xf000 {
					t.Fatalf("%s: PC/SP %04x/%04x", s.ID, regs.PC, regs.SP)
				}
				if got := binary.LittleEndian.Uint16(mem[addr("result"):]); got != s.Expected {
					t.Fatalf("%s native: got %d want %d", s.ID, got, s.Expected)
				}
				for name, want := range s.After {
					if !bytes.Equal(mem[addr(name):addr(name)+len(want)], want) {
						t.Fatalf("%s native: memory %s", s.ID, name)
					}
				}
			}
		})
	}
}
