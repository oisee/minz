package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/vir"
	"os"
	"path/filepath"
	"testing"
)

func TestJointProfilePinnedTableCallerAndMutation(t *testing.T) {
	_, path := localTableFixture(t)
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	source := jointSource + checkedTableSource
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "joint-table.nanz") }
	n := int64(5)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "z80-pbqp-default-jrs/v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"array": {1, 2, 3, 4, 5}}, Globals: map[string][]byte{"calls": {9}}, Calls: []hir.ProfileInvocation{{Function: "caller", Arguments: []hir.ProfileArgument{{Resource: "array"}, {Value: &n}}}, {Function: "main"}}}
	baseline, e := CompileHIRSteps(func() *hir.Module {
		h, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		return h
	}(), Options{OptSize: true, AssertMode: "none", UseHIRGrace: true})
	if e != nil {
		t.Fatal(e)
	}
	problem, e := vir.PrepareLocalTableProblem(baseline.MIR2Module.FuncByName("add5"), baseline.Allocation)
	if e != nil {
		t.Fatal(e)
	}
	assignment := map[int]int{}
	for v, loc := range problem.Options.ParamLocs {
		assignment[v] = loc
	}
	for _, op := range problem.Ops {
		if op.Dst > 0 {
			assignment[op.Dst] = vir.Z80.LocByName("A")
		}
	}
	var table vir.RegAllocTable
	table.Init()
	if e := table.AddChecked(problem.Ops, assignment, vir.Z80, problem.Options); e != nil {
		t.Fatal(e)
	}
	if e := table.Save(path); e != nil {
		t.Fatal(e)
	}
	data, e = os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	h, e := factory()
	if e != nil {
		t.Fatal(e)
	}
	run, e := hir.RunLoopProfile(h, w)
	if e != nil {
		t.Fatal(e)
	}
	profile, e := json.Marshal(run.Profile)
	if e != nil {
		t.Fatal(e)
	}
	options := Options{OptSize: true, AssertMode: "none", UseHIRGrace: true, CheckedLocalTablePath: path, CheckedLocalTableSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	result, e := MeasureProfileVariantSets(factory, profile, w, options, 4)
	if e != nil {
		t.Fatal(e)
	}
	if len(result.Evidence.Candidates) != 4 || len(result.Evidence.EligibleSites) != 2 {
		t.Fatal("incomplete enumeration")
	}
	for i, c := range result.Evidence.Candidates {
		if len(c.Run.Invocations) != 2 || c.Run.Invocations[0].Results[0].I != 30 || c.Run.Invocations[1].Results[0].I != 12 || c.Run.Globals["calls"][0] != 10 {
			t.Fatal("independent caller oracle", i, c.Run)
		}
		t.Logf("mask%d: %d bytes / %d T", i, c.ModuleBytes, c.TotalTStates)
	}
	if result.Steps.Traces["add5"].Backend != "checked-local-table" {
		t.Fatal("table lost in winner")
	}
	best := result.Evidence.Candidates[result.Evidence.Selected]
	for _, c := range result.Evidence.Candidates {
		if c.ModuleBytes < best.ModuleBytes {
			t.Fatal("not Size minimum")
		}
	}
	calls := 0
	mutating := func() (*hir.Module, error) {
		calls++
		if calls == 3 {
			if e := os.WriteFile(path, []byte("[]"), 0644); e != nil {
				return nil, e
			}
		}
		return factory()
	}
	if r, e := MeasureProfileVariantSets(mutating, profile, w, options, 4); e == nil || r != nil {
		t.Fatal("mutated table yielded partial winner", e)
	}
	if e := os.WriteFile(path, data, 0644); e != nil {
		t.Fatal(e)
	}
	if dir := os.Getenv("MINZ_JOINT_TABLE_EXPORT"); dir != "" {
		os.MkdirAll(dir, 0755)
		for name, v := range map[string]any{"evidence.json": result.Evidence, "workload.json": w, "profile.json": run.Profile} {
			b, e := json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
		for name, b := range map[string][]byte{"source.nanz": []byte(source), "table.json": data, "selected.a80": []byte(result.Steps.Assembly), "selected.bin": result.Binary} {
			if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	t.Log("four measured caller combinations and table-mutation rejection PASS")
}
