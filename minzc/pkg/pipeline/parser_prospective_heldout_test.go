package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParserProspectiveHeldout(t *testing.T) {
	const report = "../../../docs/grace-parser-prospective-heldout"
	const path = "../../../examples/nanz/self_lanz_parser.nanz"
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
		t.Fatal("unchanged source identity failed")
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	word := func(v uint16) []byte { return []byte{byte(v), byte(v >> 8)} }
	makeWork := func(name string) (hir.MemoryHostWorkload, []string) {
		b, e := os.ReadFile(report + "/frozen/" + name + "-inputs.json")
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(b)) != freeze.InputsSHA256[name+"-inputs.json"] {
			t.Fatal("frozen input changed")
		}
		var inputs []string
		if e = json.Unmarshal(b, &inputs); e != nil {
			t.Fatal(e)
		}
		w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"nodes": {Address: 0x8000, Bytes: make([]byte, 4096), Writable: true}, "strings": {Address: 0x9000, Bytes: make([]byte, 8192), Writable: true}, "source": {Address: 0xc000, Bytes: make([]byte, 4096)}}, Globals: map[string][]byte{"str_bump": word(0x9000)}, Calls: []hir.MemoryHostInvocation{}}
		for _, input := range inputs {
			w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "parse_sexpr", Arguments: []int64{}, Globals: map[string][]byte{"src_len": word(uint16(len(input))), "pos": word(0), "node_bump": word(0x8000)}, Patches: []hir.MemoryHostPatch{{Region: "source", Clear: true, Bytes: []byte(input)}}})
		}
		return w, inputs
	}
	// Both complete workloads exist before collection/selection.
	train, trainInputs := makeWork("training")
	held, heldInputs := makeWork("heldout")
	dir := os.Getenv("MINZ_PARSER_PROSPECTIVE_EVIDENCE")
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
	independent := func(label string, w hir.MemoryHostWorkload, inputs []string) *hir.MemoryHostRun {
		h, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		run, e := hir.RunMemoryHostBaseline(h, w)
		if e != nil {
			t.Fatal(e)
		}
		host := newLanzHost()
		for i, input := range inputs {
			clear(host.ram[0xc000:0xd000])
			copy(host.ram[0xc000:], input)
			host.pos = 0
			host.len = uint16(len(input))
			host.node = 0x8000
			host.events = []lanzAccess{}
			root := host.parse()
			o := run.Invocations[i]
			events := make([]hir.MemoryHostAccess, len(host.events))
			for j, x := range host.events {
				events[j] = hir.MemoryHostAccess{Kind: x.Op, Address: x.Address, Value: x.Value}
			}
			if o.Error != "" || len(o.Results) != 1 || o.Results[0].I != int64(root) || !reflect.DeepEqual(o.Regions["nodes"], host.ram[0x8000:0x9000]) || !reflect.DeepEqual(o.Regions["strings"], host.ram[0x9000:0xb000]) || !reflect.DeepEqual(o.Regions["source"], host.ram[0xc000:0xd000]) || !reflect.DeepEqual(o.Events, events) {
				t.Fatalf("%s independent parser oracle call %d failed", label, i)
			}
			for name, want := range map[string]uint16{"pos": host.pos, "src_len": host.len, "node_bump": host.node, "str_bump": host.str} {
				if !reflect.DeepEqual(o.Globals[name], word(want)) {
					t.Fatalf("%s independent global %s call %d", label, name, i)
				}
			}
		}
		save(label+"-raw-oracle.json", run)
		return run
	}
	independent("training", train, trainInputs)
	options := MemoryHostBaselineOptions{}
	association, e := RunZ80MemoryHostLoopProfileWithOptions(factory, train, options)
	if e != nil {
		t.Fatal(e)
	}
	payload, e := json.Marshal(association)
	if e != nil {
		t.Fatal(e)
	}
	save("training-association-v2.json", association)
	selection, e := MeasureMemoryHostArenaAppendWithOptions(factory, train, payload, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}, options)
	if e != nil {
		t.Fatal(e)
	}
	save("training-selection-v2.json", selection)
	for i, c := range selection.Candidates {
		q := fmt.Sprintf("candidate-%d", i)
		target := c.Evidence.Baseline.Target
		save(q+"/target.a80", target.Assembly)
		save(q+"/target.bin", target.Binary)
		save(q+"/final.mir2", target.FinalMIRDump)
		save(q+"/allocation.json", target.AllocationJSON)
		save(q+"/evidence.json", c.Evidence)
		t.Logf("training mask %d: %d T / %d B", i, target.TotalTStates, len(target.Binary))
	}
	// Selection complete; heldout is never passed to a selector/profile collector.
	oracle := independent("heldout", held, heldInputs)
	var heldoutBaseline int64
	for _, i := range []int{0, selection.Selected} {
		c := selection.Candidates[i]
		h, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		if e = constructMemoryHostArenaAppend(h, train, c.Choices.ArenaPeel, c.Choices.AppendHoist); e != nil {
			t.Fatal(e)
		}
		steps, e := CompileHIRSteps(h, Options{ContractOpt: false, AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		original, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		r, e := runZ80MemoryHostWorkload(steps, original, held, *oracle)
		if e != nil {
			t.Fatal(e)
		}
		trainTarget := c.Evidence.Baseline.Target
		if !reflect.DeepEqual(r.Binary, trainTarget.Binary) {
			t.Fatal("heldout did not use identical frozen candidate BIN")
		}
		if i == 0 {
			heldoutBaseline = r.TotalTStates
		} else if r.TotalTStates >= heldoutBaseline {
			t.Fatalf("heldout no benefit: selected %d T baseline %d T", r.TotalTStates, heldoutBaseline)
		}
		save(fmt.Sprintf("heldout-mask-%d.json", i), r)
		t.Logf("heldout mask %d: %d T / %d B SHA %s", i, r.TotalTStates, len(r.Binary), r.BinarySHA256)
	}
	save("decision.json", map[string]any{"selected": selection.Selected, "goal": "speed", "contract_opt": false, "compile_policy": "Legacy", "freeze_commit": "c533cf10", "heldout_selection": false})
}
