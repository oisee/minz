package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestPascalBubbleSortProspectiveHeldout(t *testing.T) {
	const frozen = "../../../reports/bubble-sort-heldout/frozen"
	const sourcePath = "../../../examples/pascal/bubble_sort.pas"
	src, e := os.ReadFile(frozen + "/source.pas")
	if e != nil {
		t.Fatal(e)
	}
	current, e := os.ReadFile(sourcePath)
	if e != nil || !bytes.Equal(src, current) {
		t.Fatal("original whole source changed", e)
	}
	var freeze struct {
		SourceSHA256 string            `json:"source_sha256"`
		InputsSHA256 map[string]string `json:"inputs_sha256"`
	}
	data, e := os.ReadFile(frozen + "/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &freeze); e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(src)) != freeze.SourceSHA256 {
		t.Fatal("frozen source digest")
	}
	factory := func() (*hir.Module, error) { return pascal.Compile(string(src), sourcePath) }
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_BUBBLE_SORT_EVIDENCE")
		if dir == "" {
			return
		}
		var b []byte
		var e error
		switch x := v.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		default:
			b, e = json.MarshalIndent(v, "", "  ")
		}
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(dir, name)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	load := func(part string) [][]byte {
		b, e := os.ReadFile(frozen + "/" + part + "-inputs.json")
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(b)) != freeze.InputsSHA256[part+"-inputs.json"] {
			t.Fatal("input freeze changed")
		}
		var rows [][]byte
		if e = json.Unmarshal(b, &rows); e != nil {
			t.Fatal(e)
		}
		return rows
	}
	training, heldout := load("training"), load("heldout") // both owned before selection
	work := func(rows [][]byte) hir.ProfileWorkload {
		if len(rows) != 1 {
			t.Fatal("one isolated RAM state per array")
		}
		return hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{"ARR": append([]byte(nil), rows[0]...)}, Calls: []hir.ProfileInvocation{{Function: "SORT"}}}
	}
	trainingWorkloads := []hir.ProfileWorkload{}
	for _, row := range training {
		trainingWorkloads = append(trainingWorkloads, work([][]byte{row}))
	}

	check := func(input []byte, snapshot hir.ProfileMemorySnapshot) {
		want := append([]byte(nil), input...)
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		if !bytes.Equal(snapshot.Globals["ARR"], want) {
			t.Fatal("independent unsigned sort", input, snapshot.Globals["ARR"], want)
		}
		if !bytes.Equal(snapshot.Globals["I"], []byte{9}) || !bytes.Equal(snapshot.Globals["SWAPPED"], []byte{0}) || !bytes.Equal(snapshot.Globals["J"], []byte{0}) {
			t.Fatal("independent loop/global boundary", snapshot)
		}
		// TEMP is the last left operand involved in a swap; model the original
		// repeat/for order instead of accepting the VM's value by agreement.
		a := append([]byte(nil), input...)
		temp := byte(0)
		for {
			swapped := false
			for i := 0; i < 9; i++ {
				if a[i] > a[i+1] {
					temp = a[i]
					a[i] = a[i+1]
					a[i+1] = temp
					swapped = true
				}
			}
			if !swapped {
				break
			}
		}
		if !bytes.Equal(snapshot.Globals["TEMP"], []byte{temp}) {
			t.Fatal("last swap TEMP", snapshot.Globals["TEMP"], temp)
		}
	}
	// Freeze described three training arrays; all three are checked before any
	// selection. Selection API currently admits a single initial global state.
	for i, row := range training {
		h, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		r, e := hir.RunLoopProfile(h, work([][]byte{row}))
		if e != nil {
			t.Fatal(e)
		}
		check(row, r.Snapshots[0])
		save(fmt.Sprintf("training-controls/%d.json", i), r)
	}
	save("source.pas", src)
	save("training-workloads.json", trainingWorkloads)
	for _, goal := range []string{"speed", "size"} {
		result, e := MeasureCompilerModeWorkloads(factory, trainingWorkloads, goal, false)
		if e != nil {
			t.Fatal(e)
		}
		save(goal+"/selection.json", result.Evidence)
		for i, c := range result.Evidence.Runs[0].Candidates {
			save(fmt.Sprintf("%s/candidates/%d.a80", goal, i), c.Assembly)
			save(fmt.Sprintf("%s/candidates/%d.bin", goal, i), c.Binary)
			save(fmt.Sprintf("%s/candidates/%d.mir", goal, i), c.FinalMIR)
			save(fmt.Sprintf("%s/candidates/%d-run.json", goal, i), c.Run)
		}
		chosen := result.Evidence.Runs[0].Candidates[result.Evidence.Selected]
		if !bytes.Equal(chosen.Binary, result.Binary) || chosen.Assembly != result.Steps.Assembly {
			t.Fatal("selected artifact mismatch")
		}
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + result.Steps.Assembly)
		if e != nil || len(bin.Errors) != 0 || !bytes.Equal(bin.Binary, result.Binary) {
			t.Fatal("exact selected binary", e)
		}
		save(goal+"/selected.a80", result.Steps.Assembly)
		save(goal+"/selected.bin", result.Binary)
		var baselineT, selectedT int64
		for i, row := range heldout {
			w := work([][]byte{row})
			h, e := factory()
			if e != nil {
				t.Fatal(e)
			}
			oracle, e := hir.RunLoopProfile(h, w)
			if e != nil {
				t.Fatal(e)
			}
			check(row, oracle.Snapshots[0])
			run, e := RunZ80ProfileWorkload(result.Steps, h, w, *oracle)
			if e != nil {
				t.Fatal(e)
			}
			if run.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(result.Binary)) {
				t.Fatal("heldout executable changed")
			}
			check(row, run.Invocations[0].Snapshot)
			selectedT += run.TotalTStates
			save(fmt.Sprintf("%s/heldout/%d-workload.json", goal, i), w)
			save(fmt.Sprintf("%s/heldout/%d-oracle.json", goal, i), oracle)
			save(fmt.Sprintf("%s/heldout/%d-selected.json", goal, i), run)
			baselineHIR, e := factory()
			if e != nil {
				t.Fatal(e)
			}
			baselineSteps, e := CompileHIRSteps(baselineHIR, Options{AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
			}
			br, e := RunZ80ProfileWorkload(baselineSteps, h, w, *oracle)
			if e != nil {
				t.Fatal(e)
			}
			baselineT += br.TotalTStates
			save(fmt.Sprintf("%s/heldout/%d-baseline.json", goal, i), br)
			if !reflect.DeepEqual(br.Invocations[0].Snapshot, run.Invocations[0].Snapshot) {
				t.Fatal("whole globals/resources differ")
			}
		}
		summary := map[string]any{"goal": goal, "selected": result.Evidence.Selected, "training_baseline_t": result.Evidence.TotalTStates[0], "training_selected_t": result.Evidence.TotalTStates[result.Evidence.Selected], "baseline_bytes": len(result.Evidence.Runs[0].Candidates[0].Binary), "selected_bytes": len(result.Binary), "heldout_baseline_t": baselineT, "heldout_selected_t": selectedT, "heldout_arrays": len(heldout), "same_binary_sha256": fmt.Sprintf("%x", sha256.Sum256(result.Binary))}
		save(goal+"/summary.json", summary)
		t.Log(summary)
	}
}
