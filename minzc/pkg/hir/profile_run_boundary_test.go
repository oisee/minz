package hir_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func TestProfileRunMemorySnapshots(t *testing.T) {
	m, err := nanz.Parse(`global g:u8=0
global untouched:[u8;2]=[7,8]
fun set(p:^u8) {g=1
 p[0]=1}
fun reset(p:^u8) {g=0
 p[0]=0}`, "boundaries.nanz")
	if err != nil {
		t.Fatal(err)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536,
		Resources: map[string][]byte{"data": {4, 5}, "untouched": {6}, "empty": {}},
		Calls: []hir.ProfileInvocation{
			{Function: "set", Arguments: []hir.ProfileArgument{{Resource: "data"}}},
			{Function: "reset", Arguments: []hir.ProfileArgument{{Resource: "data"}}},
		}}
	out, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	want := []hir.ProfileMemorySnapshot{
		{Globals: map[string][]byte{"g": {1}, "untouched": {7, 8}}, Resources: map[string][]byte{"data": {1, 5}, "untouched": {6}, "empty": {}}},
		{Globals: map[string][]byte{"g": {0}, "untouched": {7, 8}}, Resources: map[string][]byte{"data": {0, 5}, "untouched": {6}, "empty": {}}},
	}
	if len(out.Results) != 2 || len(out.Results[0]) != 0 || len(out.Results[1]) != 0 || !reflect.DeepEqual(out.Snapshots, want) {
		t.Fatalf("void invocation snapshots: %+v", out)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var decoded hir.ProfileRun
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&decoded); err != nil || !reflect.DeepEqual(decoded.Snapshots, want) {
		t.Fatalf("snapshot round trip: %+v %v", decoded.Snapshots, err)
	}
	// Every observation owns its maps and bytes, including the separate final state.
	out.Snapshots[1].Globals["g"][0] = 9
	out.Snapshots[1].Resources["data"][0] = 9
	out.Snapshots[1].Globals["untouched"][0] = 9
	out.Snapshots[1].Resources["untouched"][0] = 9
	delete(out.Snapshots[1].Resources, "empty")
	if !reflect.DeepEqual(out.Snapshots[0], want[0]) || out.Globals["g"][0] != 0 || out.Resources["data"][0] != 0 || w.Resources["data"][0] != 4 || decoded.Snapshots[1].Globals["g"][0] != 0 {
		t.Fatal("snapshot storage aliases another boundary, final state, input, or serialized copy")
	}
	// The strict loop-profile document stays separate from the raw run envelope.
	context, err := hir.BuildWorkloadProfileContext(m, w)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := json.Marshal(out.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hir.LoadLoopProfile(bytes.NewReader(profile), m, context); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(profile, &fields); err != nil {
		t.Fatal(err)
	}
	fields["snapshots"] = json.RawMessage(`[]`)
	withSnapshots, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hir.LoadLoopProfile(bytes.NewReader(withSnapshots), m, context); err == nil {
		t.Fatal("strict profile loader accepted raw-run snapshots")
	}
}

func TestProfileRunSnapshotsPartialFailure(t *testing.T) {
	m, err := nanz.Parse(`global g:u8=0
fun set() {g=1}
fun fail() {g=2
 while true {g=g+1}}`, "partial-boundaries.nanz")
	if err != nil {
		t.Fatal(err)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100, MaxMemory: 65536,
		Calls: []hir.ProfileInvocation{{Function: "set"}, {Function: "fail"}}}
	out, err := hir.RunLoopProfile(m, w)
	if err == nil || out == nil || len(out.Snapshots) != 1 || len(out.Results) != 1 || out.Profile.CompletedCalls != 1 || out.Profile.FailedCalls != 1 || out.Snapshots[0].Globals["g"][0] != 1 {
		t.Fatalf("partial run: %+v %v", out, err)
	}
}
