package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/vir"
	"os"
	"path/filepath"
	"testing"
)

func TestCompositionActualInventory(t *testing.T) {
	dir := os.Getenv("COMPOSITION_EXPORT")
	if dir == "" {
		t.Skip("set COMPOSITION_EXPORT")
	}
	os.MkdirAll(dir, 0755)
	type row struct {
		File, Function, Error, Direct, Quotient string
		Vregs, Ops                              int
		Problem                                 *vir.LocalTableProblem `json:",omitempty"`
	}
	rows := []row{}
	paths, _ := filepath.Glob("../../../examples/nanz/*.nanz")
	for _, path := range paths {
		src, e := os.ReadFile(path)
		os.WriteFile(filepath.Join(dir, filepath.Base(path)), src, 0644)
		if e != nil {
			t.Fatal(e)
		}
		h, e := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{BaseDir: filepath.Dir(path), StdlibDir: "../../../stdlib"})
		if e != nil {
			rows = append(rows, row{File: filepath.Base(path), Error: "parse: " + e.Error()})
			continue
		}
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: true, AssertMode: "none"})
		if e != nil {
			rows = append(rows, row{File: filepath.Base(path), Error: "compile: " + e.Error()})
			continue
		}
		for _, f := range steps.MIR2Module.Funcs {
			r := row{File: filepath.Base(path), Function: f.Name}
			p, e := vir.PrepareLocalTableProblem(f, steps.Allocation)
			if e != nil {
				r.Error = e.Error()
			} else {
				ids := map[int]bool{}
				for _, op := range p.Ops {
					if op.Dst > 0 {
						ids[op.Dst] = true
					}
					for _, s := range op.Src {
						if s > 0 {
							ids[s] = true
						}
					}
				}
				r.Vregs = len(ids)
				r.Ops = len(p.Ops)
				tab := &vir.RegAllocTable{}
				tab.Init()
				if r.Vregs > 5 {
					r.Problem = p
					_, e = vir.ImportIndexedENRT("/mnt/safe/z80-compiler/tables/enriched_5v.enr", p, tab)
					if e != nil {
						r.Direct = e.Error()
					} else {
						r.Direct = "read completed"
					}
					_, e = vir.ImportIndexedENRTQuotient("/mnt/safe/z80-compiler/tables/enriched_4v.enr", p, tab)
					if e != nil {
						r.Quotient = e.Error()
					} else {
						r.Quotient = "read completed"
					}
					t.Logf("%s/%s nv%d ops%d quotient:%s", r.File, r.Function, r.Vregs, r.Ops, r.Quotient)
				}
			}
			rows = append(rows, r)
		}
	}
	data, _ := json.MarshalIndent(rows, "", "  ")
	os.WriteFile(dir+"/inventory.json", data, 0644)
}
