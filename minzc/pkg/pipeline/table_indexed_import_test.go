package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/vir"
	"os"
	"testing"
)

func TestIndexedActualHistoricalENRT(t *testing.T) {
	if os.Getenv("INDEXED_IMPORT_EXPORT") == "" {
		t.Skip("set INDEXED_IMPORT_EXPORT")
	}
	dir := os.Getenv("INDEXED_IMPORT_EXPORT")
	os.MkdirAll(dir, 0755)
	var evidence []any
	for _, name := range []string{"06_pbqp_weighted", "09_function_pointers", "20_strref_test", "16_rotate_sled"} {
		src, e := os.ReadFile("../../../examples/nanz/" + name + ".nanz")
		if e != nil {
			t.Fatal(e)
		}
		opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
		base, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
		if e != nil {
			t.Fatal(e)
		}
		table := &vir.RegAllocTable{}
		table.Init()
		for _, f := range base.MIR2Module.Funcs {
			p, e := vir.PrepareLocalTableProblem(f, base.Allocation)
			if e != nil {
				continue
			}
			for _, path := range []string{"/mnt/safe/z80-compiler/tables/enriched_4v.enr", "/mnt/safe/z80-compiler/tables/enriched_5v.enr"} {
				ev, e := vir.ImportIndexedENRT(path, p, table)
				evidence = append(evidence, map[string]any{"file": name, "function": f.Name, "evidence": ev, "error": func() string {
					if e != nil {
						return e.Error()
					}
					return ""
				}()})
				if e != nil {
					t.Log(name, f.Name, e)
				} else {
					t.Log(name, f.Name, ev.RecordIndex, ev.Reason, ev.BytesRead)
				}
			}
		}
		table.Save(dir + "/" + name + "-table.json")
		use := opts
		use.CheckedLocalTablePath = dir + "/" + name + "-table.json"
		selected, e := CompileHIRSteps(graceMeasureParse(t, string(src)), use)
		if e != nil {
			t.Fatal(e)
		}
		b, e := checkedAsmSize(base.Assembly)
		if e != nil {
			t.Fatal(e)
		}
		s, e := checkedAsmSize(selected.Assembly)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("%s module%d→%dB entries%d", name, b, s, table.Size())
	}
	b, _ := json.MarshalIndent(evidence, "", "  ")
	os.WriteFile(dir+"/indexed.json", b, 0644)
}
