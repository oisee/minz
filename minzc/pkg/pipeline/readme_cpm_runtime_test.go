package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"testing"
)

func TestReadmeCPMRuntimeAdmission(t *testing.T) {
	for _, name := range []string{"hello_cpm", "hello_cpm_fib"} {
		t.Run(name, func(t *testing.T) {
			s, e := os.ReadFile("../../../examples/nanz/" + name + ".nanz")
			if e != nil {
				t.Fatal(e)
			}
			h, e := nanz.Parse(string(s), "examples/nanz/"+name+".nanz")
			if e != nil {
				t.Fatal(e)
			}
			o := Options{ContractOpt: true, Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024, AssertMode: "none"}
			steps, e := CompileHIRSteps(h, o)
			if dir := os.Getenv("MINZ_README_CPM_ARTIFACTS"); dir != "" {
				dir = filepath.Join(dir, name)
				os.MkdirAll(dir, 0755)
				b, _ := json.MarshalIndent(map[string]any{"diagnostic": implCPMErrorString(e), "options": o, "module": steps.MIR2Module, "allocation": steps.Allocation}, "", "  ")
				os.WriteFile(filepath.Join(dir, "before.json"), b, 0644)
				if steps.MIR2Module != nil {
					os.WriteFile(filepath.Join(dir, "before.mir"), []byte(steps.MIR2Module.Dump()), 0644)
				}
			}
			if e == nil {
				t.Fatal("expected baseline refusal")
			}
			t.Log(e)
		})
	}
}
