package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"os"
	"path/filepath"
	"testing"
)

func TestPascalCPMMainAdmission(t *testing.T) {
	for _, name := range []string{"hello", "factorial"} {
		t.Run(name, func(t *testing.T) {
			s, e := os.ReadFile("../../../reports/pascal-cpm-main/" + name + ".pas")
			if e != nil {
				t.Fatal(e)
			}
			h, e := pascal.Compile(string(s), "examples/pascal/"+name+".pas")
			if e != nil {
				t.Fatal(e)
			}
			steps, e := CompileHIRSteps(h, Options{ContractOpt: true, Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024, AssertMode: "none"})
			if root := os.Getenv("MINZ_PASCAL_CPM_ARTIFACTS"); root != "" {
				p := filepath.Join(root, name)
				os.MkdirAll(p, 0755)
				b, _ := json.MarshalIndent(map[string]any{"diagnostic": implCPMErrorString(e), "module": steps.MIR2Module, "allocation": steps.Allocation}, "", "  ")
				os.WriteFile(filepath.Join(p, "before.json"), b, 0644)
				if steps.MIR2Module != nil {
					os.WriteFile(filepath.Join(p, "before.mir"), []byte(steps.MIR2Module.Dump()), 0644)
				}
			}
			if e == nil {
				t.Fatal("expected unsupported Pascal runtime")
			}
			t.Log(e)
		})
	}
}
