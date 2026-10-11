package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestArithmeticProspectiveHeldout(t *testing.T) {
	const report = "../../../docs/grace-arithmetic-heldout"
	const path = "../../../examples/nanz/18_tail_recursion.nanz"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SourceSHA256 string            `json:"source_sha256"`
		InputsSHA256 map[string]string `json:"inputs_sha256"`
	}
	b, e := os.ReadFile(report + "/frozen/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &freeze); e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(source)) != freeze.SourceSHA256 {
		t.Fatal("source changed")
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	makeWork := func(name string) hir.ProfileWorkload {
		b, e := os.ReadFile(report + "/frozen/" + name + "-calls.json")
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(b)) != freeze.InputsSHA256[name+"-calls.json"] {
			t.Fatal("frozen calls changed")
		}
		var calls []struct {
			Function string `json:"function"`
			N        int64  `json:"n"`
		}
		if e = json.Unmarshal(b, &calls); e != nil {
			t.Fatal(e)
		}
		w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}}
		for _, c := range calls {
			n := c.N
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: c.Function, Arguments: []hir.ProfileArgument{{Value: &n}}})
		}
		return w
	}
	train, held := makeWork("training"), makeWork("heldout")
	dir := os.Getenv("MINZ_ARITHMETIC_HELDOUT_EVIDENCE")
	save := func(name string, v any) {
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
	save("source.nanz", source)
	save("training-workload.json", train)
	save("heldout-workload.json", held)
	check := func(w hir.ProfileWorkload, r *hir.ProfileRun) {
		for i, c := range w.Calls {
			n := *c.Arguments[0].Value
			var want uint8
			switch c.Function {
			case "fib":
				a, b := uint8(0), uint8(1)
				for j := int64(0); j < n; j++ {
					a, b = b, a+b
				}
				want = a
			case "fact":
				want = 1
				for j := int64(2); j <= n; j++ {
					want *= uint8(j)
				}
			case "sum_to":
				want = uint8(n * (n + 1) / 2)
			case "pow2":
				want = 1
				for j := int64(0); j < n; j++ {
					want *= 2
				}
			default:
				t.Fatal("unknown oracle")
			}
			if len(r.Results[i]) != 1 || r.Results[i][0].I != int64(want) {
				t.Fatalf("independent %s(%d): got %v want %d", c.Function, n, r.Results[i], want)
			}
		}
	}
	h, e := factory()
	if e != nil {
		t.Fatal(e)
	}
	raw, e := hir.RunLoopProfile(h, train)
	if e != nil {
		t.Fatal(e)
	}
	check(train, raw)
	for _, goal := range []string{"speed", "size"} {
		r, e := MeasureCompilerModeVariants(factory, train, goal, false)
		if e != nil {
			save(goal+"/failure.txt", e.Error())
			t.Fatal(goal, e)
		}
		save(goal+"/selection.json", r.Evidence)
		for i, c := range r.Evidence.Candidates {
			q := fmt.Sprintf("%s/candidate-%d", goal, i)
			save(q+"/target.a80", c.Assembly)
			save(q+"/target.bin", c.Binary)
			save(q+"/final.mir2", c.FinalMIR)
			t.Logf("%s mask%d: %d T / %d B", goal, i, c.Run.TotalTStates, len(c.Binary))
		}
		hh, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		oracle, e := hir.RunLoopProfile(hh, held)
		if e != nil {
			t.Fatal(e)
		}
		check(held, oracle)
		save("heldout-oracle.json", oracle)
		run, e := RunZ80ProfileWorkload(r.Steps, hh, held, *oracle)
		if e != nil {
			t.Fatal(e)
		}
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + r.Steps.Assembly)
		if e != nil || len(bin.Errors) > 0 || !reflect.DeepEqual(bin.Binary, r.Binary) {
			t.Fatal("heldout winner binary mismatch", e)
		}
		save(goal+"/heldout.json", run)
		t.Logf("%s selected%d heldout %d T / %d B", goal, r.Evidence.Selected, run.TotalTStates, len(r.Binary))
	}
}
