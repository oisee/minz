package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func TestConditionalLayoutTokenizerHeldout(t *testing.T) {
	path := "../../../examples/nanz/grace_tokenizer_checked.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	makeWorkload := func(inputs []string) hir.ProfileWorkload {
		w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"arena": bytes.Repeat([]byte{0xa5}, 1024), "tokens": bytes.Repeat([]byte{0xc3}, 512)}}
		for i, text := range inputs {
			name := fmt.Sprint("source", i)
			w.Resources[name] = append([]byte(text), 0, 0x5a)
			n := int64(len(text))
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "tokenizer_checked", Arguments: []hir.ProfileArgument{{Resource: name}, {Resource: "arena"}, {Resource: "tokens"}, {Value: &n}}})
		}
		return w
	}
	train := makeWorkload([]string{"fun add(a: u8, b: u8) -> u8 { return a + b }", "alpha alpha beta alpha", "// comment\nfun"})
	heldout := makeWorkload([]string{"0 007 123456789", "\tfn if else\r\nwhile let var true false return", "_ _a a1 A9"})
	result, err := MeasureConditionalLayoutWorkload(factory, "is_space", train, Options{OptSize: true, ContractOpt: true, AssertMode: "none"}, "speed", 4)
	if err != nil {
		t.Fatal(err)
	}
	check := func(w hir.ProfileWorkload, run *Z80ProfileWorkloadEvidence) {
		t.Helper()
		host := tokenizerHost{arena: append([]byte(nil), w.Resources["arena"]...), tokens: append([]byte(nil), w.Resources["tokens"]...), keywords: map[string]int{}}
		for i, c := range w.Calls {
			name := c.Arguments[0].Resource
			n := int(*c.Arguments[3].Value)
			host.run(w.Resources[name], n)
			inv := run.Invocations[i]
			if len(inv.Results) != 1 || inv.Results[0].I != int64(host.count) || !reflect.DeepEqual(inv.Snapshot.Globals, host.globals(n)) || !bytes.Equal(inv.Snapshot.Resources["arena"], host.arena) || !bytes.Equal(inv.Snapshot.Resources["tokens"], host.tokens) {
				t.Fatal("independent tokenizer results/state", i)
			}
			if inv.Access == nil {
				t.Fatal("missing exact accesses")
			}
			accesses := inv.Access.Resources
			for resource, want := range map[string]hir.ProfileByteAccess{
				"arena":  {Reads: host.arenaReads, Writes: host.arenaWrites},
				"tokens": {Reads: host.tokensReads, Writes: host.tokensWrites},
				name:     {Reads: host.sourceReads, Writes: make([]uint64, len(w.Resources[name]))},
			} {
				if !reflect.DeepEqual(accesses[resource], want) {
					t.Fatal("independent tokenizer byte traffic", i, resource)
				}
			}
		}
	}
	for _, candidate := range result.Evidence.Candidates {
		check(train, candidate.Run)
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	oracle, err := hir.RunLoopProfile(m, heldout)
	if err != nil {
		t.Fatal(err)
	}
	baseline := result.Steps
	baseline.Assembly = result.Evidence.Candidates[0].Assembly
	before, err := RunZ80ProfileWorkload(baseline, m, heldout, *oracle)
	if err != nil {
		t.Fatal(err)
	}
	after, err := RunZ80ProfileWorkload(result.Steps, m, heldout, *oracle)
	if err != nil {
		t.Fatal(err)
	}
	check(heldout, before)
	check(heldout, after)
	chosen := result.Evidence.Candidates[result.Evidence.Selected]
	t.Logf("selected%d: module %d / %d B; train %d / %d T; heldout %d / %d T", result.Evidence.Selected, len(result.Evidence.Candidates[0].Binary), len(result.Binary), result.Evidence.Candidates[0].Run.TotalTStates, chosen.Run.TotalTStates, before.TotalTStates, after.TotalTStates)
	if directory := os.Getenv("MINZ_CONDITIONAL_WORKLOAD_EXPORT"); directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		values := map[string]any{"source.nanz": source, "evidence.json": result.Evidence, "train.json": train, "heldout.json": heldout, "heldout-before.json": before, "heldout-after.json": after, "selected.a80": []byte(result.Steps.Assembly), "selected.bin": result.Binary}
		for name, value := range values {
			data, ok := value.([]byte)
			if !ok {
				data, err = json.MarshalIndent(value, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
			}
			if err = os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
