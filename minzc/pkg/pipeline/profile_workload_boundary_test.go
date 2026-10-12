package pipeline

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

func TestZ80ProfileWorkloadInvocationMemoryBoundaries(t *testing.T) {
	for _, kind := range []string{"global", "resource"} {
		t.Run(kind, func(t *testing.T) {
			text := `global g:u8=0
fun set() {g=1}
fun reset() {g=0}`
			w := workloadSpec(hir.ProfileInvocation{Function: "set"}, hir.ProfileInvocation{Function: "reset"})
			store := "g"
			if kind == "resource" {
				text = `fun set(p:^u8) {p[0]=1}
fun reset(p:^u8) {p[0]=0}`
				w.Resources["data"] = []byte{0}
				for i := range w.Calls {
					w.Calls[i].Arguments = []hir.ProfileArgument{{Resource: "data"}}
				}
				store = "HL"
			}
			steps, source, oracle := workloadFixture(t, text, w)
			out, err := RunZ80ProfileWorkload(steps, source, w, oracle)
			if err != nil {
				t.Fatalf("unchanged workload: %v\n%s", err, steps.Assembly)
			}
			for ci, inv := range out.Invocations {
				if !reflect.DeepEqual(inv.Snapshot, oracle.Snapshots[ci]) {
					t.Fatalf("boundary %d: %+v want %+v", ci, inv.Snapshot, oracle.Snapshots[ci])
				}
			}
			// First void invocation stores 99; the subsequent invocation resets to 0.
			// A final-memory-only oracle would accept this exact mutant.
			mutant := steps
			mutant.Assembly = strings.Replace(steps.Assembly, "set:", "set:\n LD A, 99\n LD ("+store+"), A\n RET\n", 1)
			if mutant.Assembly == steps.Assembly {
				t.Fatal("missing set entry")
			}
			bad, err := RunZ80ProfileWorkload(mutant, source, w, oracle)
			if bad != nil || err == nil || !strings.Contains(err.Error(), "invocation 0 @set: "+kind+" mismatch") || !strings.Contains(err.Error(), "got [99] want [1]") {
				t.Fatalf("masked first mutation accepted or wrong failure: %+v %v", bad, err)
			}
			// Evidence bytes are independently owned across calls and final state.
			if kind == "global" {
				out.Invocations[1].Snapshot.Globals["g"][0] = 8
				if out.Invocations[0].Snapshot.Globals["g"][0] != 1 || out.Globals["g"][0] != 0 || oracle.Snapshots[1].Globals["g"][0] != 0 {
					t.Fatal("evidence aliases another observation")
				}
			} else {
				out.Invocations[1].Snapshot.Resources["data"][0] = 8
				if out.Invocations[0].Snapshot.Resources["data"][0] != 1 || out.Resources["data"][0] != 0 || oracle.Snapshots[1].Resources["data"][0] != 0 {
					t.Fatal("resource evidence aliases another observation")
				}
			}
		})
	}
}

func TestZ80ProfileWorkloadSnapshotSerialization(t *testing.T) {
	const text = `global g:u8=0
fun set(p:^u8) {p[0]=1
 g=1}
fun reset(p:^u8) {p[0]=0
 g=0}`
	w := workloadSpec(
		hir.ProfileInvocation{Function: "set", Arguments: []hir.ProfileArgument{{Resource: "data"}}},
		hir.ProfileInvocation{Function: "reset", Arguments: []hir.ProfileArgument{{Resource: "data"}}},
	)
	w.Resources = map[string][]byte{"data": {0}, "empty": {}}
	steps, source, oracle := workloadFixture(t, text, w)
	raw, err := json.Marshal(oracle)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"unchanged", "legacy", "null", "short", "extra", "reordered", "missing-global", "extra-global", "global-width", "missing-resource", "extra-resource", "resource-width", "missing-empty-resource", "null-empty-resource", "null-globals", "null-resources"} {
		t.Run(kind, func(t *testing.T) {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			var snapshots []hir.ProfileMemorySnapshot
			if err := json.Unmarshal(fields["snapshots"], &snapshots); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "short":
				snapshots = snapshots[:1]
			case "extra":
				snapshots = append(snapshots, snapshots[1])
			case "reordered":
				snapshots[0], snapshots[1] = snapshots[1], snapshots[0]
			case "missing-global":
				delete(snapshots[0].Globals, "g")
			case "extra-global":
				snapshots[0].Globals["unknown"] = []byte{0}
			case "global-width":
				snapshots[1].Globals["g"] = []byte{0, 0}
			case "missing-resource":
				delete(snapshots[0].Resources, "data")
			case "extra-resource":
				snapshots[0].Resources["unknown"] = []byte{0}
			case "resource-width":
				snapshots[1].Resources["data"] = []byte{0, 0}
			case "missing-empty-resource":
				delete(snapshots[0].Resources, "empty")
			case "null-empty-resource":
				snapshots[0].Resources["empty"] = nil
			case "null-globals":
				snapshots[0].Globals = nil
			case "null-resources":
				snapshots[0].Resources = nil
			}
			encoded, err := json.Marshal(snapshots)
			if err != nil {
				t.Fatal(err)
			}
			fields["snapshots"] = encoded
			if kind == "legacy" {
				delete(fields, "snapshots")
			} else if kind == "null" {
				fields["snapshots"] = json.RawMessage(`null`)
			}
			encoded, err = json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			var decoded hir.ProfileRun
			dec := json.NewDecoder(bytes.NewReader(encoded))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&decoded); err != nil {
				t.Fatal(err)
			}
			out, err := RunZ80ProfileWorkload(steps, source, w, decoded)
			if kind == "unchanged" {
				if err != nil || out == nil {
					t.Fatalf("serialized oracle rejected: %v", err)
				}
			} else if err == nil || out != nil {
				t.Fatalf("accepted %s serialized oracle", kind)
			}
		})
	}
}

func TestZ80ProfileWorkloadEmptyMemorySnapshots(t *testing.T) {
	w := workloadSpec(hir.ProfileInvocation{Function: "empty"})
	steps, source, oracle := workloadFixture(t, `fun empty() {}`, w)
	raw, err := json.Marshal(oracle.Snapshots)
	if err != nil || string(raw) != `[{"globals":{},"resources":{}}]` {
		t.Fatalf("empty snapshot must be explicit: %s %v", raw, err)
	}
	if _, err := RunZ80ProfileWorkload(steps, source, w, oracle); err != nil {
		t.Fatal(err)
	}
	oracle.Snapshots[0] = hir.ProfileMemorySnapshot{}
	if out, err := RunZ80ProfileWorkload(steps, source, w, oracle); out != nil || err == nil {
		t.Fatal("missing memory maps accepted for empty workload")
	}
}
