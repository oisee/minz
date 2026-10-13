package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
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
	if e != nil {
		t.Fatal("canonical guarded runtime", e)
	}
	if os.Getenv("CPM_TUI_EXPORT") != "" {
		t.Fatal("historical before-export is immutable; use MINZ_CPM_TETRIS_ARTIFACTS for new evidence")
	}
	_ = rows
}
