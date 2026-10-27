package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckedWordShiftComposedNative(t *testing.T) {
	for _, strategy := range []string{"greedy", "pbqp"} {
		for _, grace := range []bool{false, true} {
			for _, amount := range []int{1, 4, 8, 15} {
				source := fmt.Sprintf("fun shifted(x:u16)->u16{return x << %d}", amount)
				opts := Options{OptSize: true, AssertMode: "none", ContractOpt: true, ContractStrategy: strategy, UseGrace: grace, UseHIRGrace: grace}
				// A checked local table is an exact configuration, which variant
				// search never touches; compare it with the same unsearched pipeline.
				baselineOpts := opts
				baselineOpts.DisableVariantSearch = true
				base, err := CompileHIRSteps(graceMeasureParse(t, source), baselineOpts)
				if err != nil {
					t.Fatal(err)
				}
				problem, err := vir.PrepareLocalTableProblem(base.MIR2Module.FuncByName("shifted"), base.Allocation)
				if err != nil {
					t.Fatal(err)
				}
				var pool, target vir.RegAllocTable
				pool.Init()
				target.Init()
				// For long chains use a checked prefix plus the existing return bridge.
				cuts := []int{len(problem.Ops) - 2}
				assignment := map[int]int{}
				for _, op := range problem.Ops[:cuts[0]] {
					if op.Dst > 0 {
						assignment[op.Dst] = vir.Z80.LocByName("HL")
					}
					for _, v := range op.Src {
						if v > 0 {
							assignment[v] = vir.Z80.LocByName("HL")
						}
					}
				}
				for v, loc := range problem.Options.ParamLocs {
					assignment[v] = loc
				}
				if err = pool.AddChecked(problem.Ops[:cuts[0]], assignment, vir.Z80, problem.Options); err != nil {
					t.Fatal(err)
				}
				count, err := pool.ComposeCheckedAutomatically(problem, &target, 64)
				if err != nil || count == 0 {
					t.Fatal(amount, count, err)
				}
				path := t.TempDir() + "/shift.json"
				if err = target.Save(path); err != nil {
					t.Fatal(err)
				}
				opts.CheckedLocalTablePath = path
				selected, err := CompileHIRSteps(graceMeasureParse(t, source), opts)
				if err != nil {
					t.Fatal(err)
				}
				raw := mir2.NewVM(hir.LowerModule(graceMeasureParse(t, source)))
				candidate, err := problem.Emit(&target)
				if err != nil {
					t.Fatal(err)
				}
				lo, hi, ok := checkedFunctionSpan(base.Assembly, "shifted")
				if !ok {
					t.Fatal("function span")
				}
				proposal := base
				proposal.Assembly = base.Assembly[:lo] + candidate + base.Assembly[hi:]
				candidateT := checkedWordNative(t, proposal, "shifted", 1<<amount, raw)
				before := checkedWordNative(t, base, "shifted", 1<<amount, raw)
				after := checkedWordNative(t, selected, "shifted", 1<<amount, raw)
				b, err := checkedAsmSize(base.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				s, err := checkedAsmSize(selected.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				if s > b || after > before {
					t.Fatal("selected code regressed", amount, b, s, before, after)
				}
				if directory := os.Getenv("CHECKED_SHIFT_EXPORT"); directory != "" {
					directory = filepath.Join(directory, fmt.Sprintf("%s-grace%v-shift%d", strategy, grace, amount))
					if err = os.MkdirAll(directory, 0755); err != nil {
						t.Fatal(err)
					}
					for name, data := range map[string]string{"source.nanz": source, "baseline.a80": base.Assembly, "candidate.a80": proposal.Assembly, "selected.a80": selected.Assembly} {
						if err = os.WriteFile(filepath.Join(directory, name), []byte(data), 0644); err != nil {
							t.Fatal(err)
						}
					}
					if err = pool.Save(filepath.Join(directory, "pool.json")); err != nil {
						t.Fatal(err)
					}
					if err = target.Save(filepath.Join(directory, "current.json")); err != nil {
						t.Fatal(err)
					}
				}
				t.Logf("strategy=%s grace=%v", strategy, grace)
				t.Logf("raw candidate shift%d: %d T; all65536 PASS", amount, candidateT)
				t.Logf("shift%d all65536: %d -> %d bytes, %d -> %d T; backend=%s", amount, b, s, before, after, selected.Traces["shifted"].Backend)
			}
		}
	}
}
