package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"testing"
)

func TestCPMTUIAdmissionAudit(t *testing.T) {
	b, e := os.ReadFile("../../../examples/nanz/tetris_cpm.nanz")
	if e != nil {
		t.Fatal(e)
	}
	h, e := nanz.ParseWithOpts(string(b), "tetris_cpm.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
	if e != nil {
		t.Fatal(e)
	}
	s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	rows := []any{}
	for _, f := range s.MIR2Module.Funcs {
		if !f.Attrs.HasAsm {
			continue
		}
		p := map[string]any{}
		for _, v := range f.Contract.Params {
			p[v.Name] = map[string]any{"type": v.Ty.String(), "loc": s.Allocation.Locs[v.Reg]}
		}
		rows = append(rows, map[string]any{"name": f.Name, "params": p, "returns": f.Contract.Returns, "blocks": len(f.Blocks), "asm": f.Blocks[0].Insts})
	}
	h, e = nanz.ParseWithOpts(string(b), "tetris_cpm.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024})
	if e == nil {
		t.Fatal("expected refusal before admission implementation")
	}
	if d := os.Getenv("CPM_TUI_EXPORT"); d != "" {
		os.MkdirAll(d, 0755)
		j, _ := json.MarshalIndent(rows, "", "  ")
		os.WriteFile(filepath.Join(d, "asm-contracts.json"), j, 0644)
		os.WriteFile(filepath.Join(d, "refusal.txt"), []byte(e.Error()), 0644)
		os.WriteFile(filepath.Join(d, "source.nanz"), b, 0644)
		os.WriteFile(filepath.Join(d, "target.a80"), []byte(s.Assembly), 0644)
		os.WriteFile(filepath.Join(d, "final.mir"), []byte(s.MIR2Module.Dump()), 0644)
	}
	t.Log(e)
}
