package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

// Compare complete emitted programs; no assumption that independent wins add.
// This exhaustive domain is validation, not a new prospective PGO selection.
func TestCombinedContractTableGraceNative(t *testing.T) {
	solver := checkedRealSolver(t)
	source, err := os.ReadFile("../../../examples/nanz/06_pbqp_weighted.nanz")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source) + "\nfun weighted_caller(x:u8)->u8 {return weighted(x)+x}\n"
	export := os.Getenv("MINZ_COMBINED_TABLE_EXPORT")
	save := func(name string, data []byte) {
		if export == "" {
			return
		}
		if err := os.MkdirAll(export, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(export, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	save("original-source.nanz", source)
	save("caller-source.nanz", []byte(text))
	type measurement struct {
		Mode                    string
		Grace, Table            bool
		Bytes, HelperT, CallerT int
		TableEntries            int
		Notes                   []string
	}
	rows := []measurement{}
	for _, mode := range []string{"off", "greedy", "pbqp"} {
		for _, grace := range []bool{false, true} {
			opts := Options{ContractOpt: mode != "off", UseGrace: grace, UseHIRGrace: grace, OptSize: true, AssertMode: "none"}
			if mode == "pbqp" {
				opts.ContractStrategy = "pbqp"
			}
			table, notes, err := GenerateCheckedLocalTables(graceMeasureParse(t, text), opts, vir.SolverOptions{Z3Path: solver})
			if err != nil {
				t.Fatal(mode, grace, err)
			}
			path := filepath.Join(t.TempDir(), "table.json")
			if err = table.Save(path); err != nil {
				t.Fatal(err)
			}
			for _, useTable := range []bool{false, true} {
				current := opts
				if useTable {
					current.CheckedLocalTablePath = path
				}
				steps, err := CompileHIRSteps(graceMeasureParse(t, text), current)
				if err != nil {
					t.Fatal(mode, grace, useTable, err)
				}
				size, err := checkedAsmSize(steps.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				vm := mir2.NewVM(steps.MIR2Module)
				rawVM := mir2.NewVM(hir.LowerModule(graceMeasureParse(t, text)))
				row := measurement{Mode: mode, Grace: grace, Table: useTable, Bytes: size, TableEntries: table.Size(), Notes: notes}
				for x := 0; x < 256; x++ {
					for _, name := range []string{"weighted", "weighted_caller"} {
						factor := 10
						if name == "weighted_caller" {
							factor = 11
						}
						want := byte(factor*x + 1)
						for _, model := range []*mir2.VM{rawVM, vm} {
							got, err := model.Call(name, []mir2.Value{{I: int64(x)}})
							if err != nil || len(got) != 1 || got[0].I != int64(want) {
								t.Fatal(mode, grace, useTable, name, x, got, err)
							}
						}
						cost := runCheckedRealByte(t, steps.Assembly, steps.MIR2Module.FuncByName(name), steps.Allocation, byte(x), want)
						if name == "weighted" {
							row.HelperT += cost
						} else {
							row.CallerT += cost
						}
					}
				}
				rows = append(rows, row)
				t.Logf("mode=%s grace=%t table=%t fullBIN=%d helper256=%dT caller256=%dT entries=%d", mode, grace, useTable, row.Bytes, row.HelperT, row.CallerT, row.TableEntries)
				tag := mode + "-plain"
				if grace {
					tag = mode + "-grace"
				}
				if useTable {
					tag += "-table"
				}
				save(tag+".a80", []byte(steps.Assembly))
				assembled, err := z80asm.NewAssembler().AssembleString(steps.Assembly)
				if err != nil || len(assembled.Errors) > 0 {
					t.Fatal(err, assembled.Errors)
				}
				save(tag+".bin", assembled.Binary)
				if useTable && steps.Traces["weighted"].Backend != "checked-local-table" {
					t.Fatal("table not consumed", mode, grace, notes)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				save(mode+"-"+tag+"-table.json", data)
			}
		}
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	save("measurements.json", append(data, '\n'))
}
