package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"os"
	"path/filepath"
	"testing"
)

func TestActualLegacyCandidateImportCorpus(t *testing.T) {
	dir := os.Getenv("LEGACY_IMPORT_EXPORT")
	if dir == "" {
		t.Skip("set LEGACY_IMPORT_EXPORT")
	}
	os.MkdirAll(dir, 0755)
	native, e := os.ReadFile("../../../regalloc_table.json")
	if e != nil {
		t.Fatal(e)
	}
	gpu, e := os.ReadFile("../vir/regalloc_exhaustive.json")
	if e != nil {
		t.Fatal(e)
	}
	sample, e := os.ReadFile(filepath.Join(dir, "enrt4v-bounded-sample.json"))
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	for _, name := range []string{"06_pbqp_weighted", "09_function_pointers", "tiny", "20_strref_test", "16_rotate_sled"} {
		src, e := os.ReadFile("../../../examples/nanz/" + name + ".nanz")
		if e != nil {
			t.Fatal(e)
		}
		base, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
		if e != nil {
			t.Fatal(e)
		}
		table := &vir.RegAllocTable{}
		table.Init()
		evidence := map[string]any{}
		for _, f := range base.MIR2Module.Funcs {
			p, e := vir.PrepareLocalTableProblem(f, base.Allocation)
			if e != nil {
				continue
			}
			for format, data := range map[string][]byte{"legacy-native-json": native, "legacy-gpu-json": gpu} {
				ev, e := vir.ImportLegacyCandidatesWithRemap(data, format, p.Ops, vir.Z80, p.Options, table, 6)
				if e != nil {
					t.Fatal(e)
				}
				evidence[f.Name+"/"+format] = ev
				t.Logf("%s/%s %s compatible%d admitted%d", name, f.Name, format, ev.Compatible, ev.Admitted)
			}
		}
		for _, f := range base.MIR2Module.Funcs {
			p, e := vir.PrepareLocalTableProblem(f, base.Allocation)
			if e != nil {
				continue
			}
			ev, e := vir.ImportLegacyCandidatesWithRemap(sample, "legacy-gpu-json", p.Ops, vir.Z80, p.Options, table, 6)
			if e != nil {
				t.Fatal(e)
			}
			evidence[f.Name+"/enrt4v-prefix"] = ev
			t.Logf("%s/%s enrt4v-prefix compatible%d admitted%d", name, f.Name, ev.Compatible, ev.Admitted)
		}

		path := filepath.Join(dir, name+"-table.json")
		table.Save(path)
		use := opts
		use.CheckedLocalTablePath = path
		selected, e := CompileHIRSteps(graceMeasureParse(t, string(src)), use)
		if e != nil {
			t.Fatal(e)
		}
		b, _ := checkedAsmSize(base.Assembly)
		s, _ := checkedAsmSize(selected.Assembly)
		t.Logf("%s module%d→%dB importedentries%d", name, b, s, table.Size())
		evidence["module_bytes"] = []int{b, s}
		for _, f := range base.MIR2Module.Funcs {
			p, e := vir.PrepareLocalTableProblem(f, base.Allocation)
			if e != nil {
				continue
			}
			candidate, e := p.Emit(table)
			if e != nil {
				continue
			}
			if len(f.Contract.Params) == 1 && f.Contract.Params[0].Ty == mir2.TyU8 {
				for x := 0; x < 256; x++ {
					var want byte
					switch f.Name {
					case "double":
						want = byte(2 * x)
					case "low_nibble":
						want = byte(x) & 15
					default:
						continue
					}
					runCheckedRealByte(t, candidate, f, base.Allocation, byte(x), want)
				}
			}
		}
		for n, b := range map[string][]byte{name + "-baseline.a80": []byte(base.Assembly), name + "-selected.a80": []byte(selected.Assembly), name + ".nanz": src} {
			os.WriteFile(filepath.Join(dir, n), b, 0644)
		}
		data, _ := json.MarshalIndent(evidence, "", "  ")
		os.WriteFile(filepath.Join(dir, name+"-import.json"), data, 0644)
	}
}
