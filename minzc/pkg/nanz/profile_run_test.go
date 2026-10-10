package nanz_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func profileLiteral(v int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &v} }

func TestControlledProfileWorkload(t *testing.T) {
	src := `global calls:u8=0
fun test(ptr:^u8,n:u8)->u8 {
 calls=calls+1
 var i:u8=0
 var sum:u8=0
 while i<n {sum=sum+ptr[i]
  ptr[i]=ptr[i]+1
  i=i+1}
 return sum
}`
	m, err := nanz.Parse(src, "controlled.nanz")
	if err != nil {
		t.Fatal(err)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm", MaxSteps: 10000, MaxMemory: 65536, Resources: map[string][]byte{"data": {1, 2, 3}}, Globals: map[string][]byte{"calls": {10}}, Calls: []hir.ProfileInvocation{}}
	for _, n := range []int64{0, 1, 3} {
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "test", Arguments: []hir.ProfileArgument{{Resource: "data"}, profileLiteral(n)}})
	}
	context, err := hir.BuildWorkloadProfileContext(m, w)
	if err != nil {
		t.Fatal(err)
	}
	out, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 3 || out.Results[0][0].I != 0 || out.Results[1][0].I != 1 || out.Results[2][0].I != 7 {
		t.Fatalf("results=%v", out.Results)
	}
	if !bytes.Equal(out.Resources["data"], []byte{3, 3, 4}) || !bytes.Equal(out.Globals["calls"], []byte{13}) {
		t.Fatalf("memory=%v globals=%v", out.Resources, out.Globals)
	}
	if !bytes.Equal(w.Resources["data"], []byte{1, 2, 3}) || w.Globals["calls"][0] != 10 {
		t.Fatal("caller inputs mutated")
	}
	c := out.Profile.Counts[0]
	if c.Entries != 3 || c.BodyEntries != 4 {
		t.Fatalf("counts=%+v", c)
	}
	data, _ := json.Marshal(out.Profile)
	if _, err := hir.LoadLoopProfile(bytes.NewReader(data), m, context); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"order", "memory", "gas", "resource", "global"} {
		raw, _ := json.Marshal(w)
		var changed hir.ProfileWorkload
		json.Unmarshal(raw, &changed)
		switch field {
		case "order":
			changed.Calls[0], changed.Calls[2] = changed.Calls[2], changed.Calls[0]
		case "memory":
			changed.MaxMemory = 32768
		case "gas":
			changed.MaxSteps++
		case "resource":
			changed.Resources["data"][0] = 9
		case "global":
			changed.Globals["calls"][0] = 0
		}
		x, err := hir.BuildWorkloadProfileContext(m, changed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := hir.LoadLoopProfile(bytes.NewReader(data), m, x); err == nil {
			t.Fatalf("%s mismatch accepted", field)
		}
	}
	t.Log(string(data))
}

func TestControlledProfileRefusalsAndPartialFailure(t *testing.T) {
	m, err := nanz.Parse(`fun test(n:u8)->u8 {while n>0 {n=n-1}
 return n}`, "failure.nanz")
	if err != nil {
		t.Fatal(err)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm", MaxSteps: 5, MaxMemory: 65536, Calls: []hir.ProfileInvocation{{Function: "test", Arguments: []hir.ProfileArgument{profileLiteral(255)}}}}
	out, err := hir.RunLoopProfile(m, w)
	if err == nil || out == nil || out.Profile.FailedCalls != 1 || out.Profile.CompletedCalls != 0 {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	context, e := hir.BuildWorkloadProfileContext(m, w)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(out.Profile)
	if _, err := hir.LoadLoopProfile(bytes.NewReader(data), m, context); err == nil {
		t.Fatal("partial failure accepted")
	}
	for _, edit := range []func(*hir.ProfileWorkload){
		func(w *hir.ProfileWorkload) { w.MaxSteps = 0 },
		func(w *hir.ProfileWorkload) { w.MaxMemory = 1 },
		func(w *hir.ProfileWorkload) { w.Calls[0].Function = "missing" },
		func(w *hir.ProfileWorkload) { w.Globals = map[string][]byte{"missing": {0}} },
		func(w *hir.ProfileWorkload) { w.Calls[0].Arguments[0] = hir.ProfileArgument{Resource: "missing"} },
		func(w *hir.ProfileWorkload) {
			w.Resources = map[string][]byte{"data": {0}}
			w.Calls[0].Arguments[0] = hir.ProfileArgument{Resource: "data", Value: new(int64)}
		},
	} {
		raw, _ := json.Marshal(w)
		var bad hir.ProfileWorkload
		json.Unmarshal(raw, &bad)
		edit(&bad)
		if out, err := hir.RunLoopProfile(m, bad); err == nil || out != nil {
			t.Fatalf("invalid setup accepted: %v %v", out, err)
		}
	}
	// A builtin print host must not run or acquire stdout/environment state.
	ioModule, err := nanz.Parse(`fun test()->void {@print_u8(7)}`, "io.nanz")
	if err != nil {
		t.Fatal(err)
	}
	w.Calls = []hir.ProfileInvocation{{Function: "test"}}
	w.MaxSteps = 100
	out, err = hir.RunLoopProfile(ioModule, w)
	if err == nil || out == nil || out.Profile.FailedCalls != 1 {
		t.Fatalf("external host not refused: out=%v err=%v", out, err)
	}
}
