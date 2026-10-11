package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMemoryHostArenaPeelTargetDomain(t *testing.T) {
	originalFactory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	fill := func(n int) []byte {
		b := make([]byte, n)
		for j := range b {
			b[j] = 0x5a
		}
		return b
	}
	regions := func() map[string]hir.MemoryHostRegion {
		return map[string]hir.MemoryHostRegion{"left": {Address: 0x8000, Bytes: fill(258)}, "right": {Address: 0x9000, Bytes: fill(258)}, "high": {Address: 0xfffe, Bytes: fill(2)}}
	}
	cases := []arenaDomainCase{}
	add := func(name string, n int, arena, base, start uint16, mismatch int) {
		r := regions()
		if mismatch >= 0 {
			q := r["right"]
			q.Bytes[mismatch] = 0xa5
			r["right"] = q
		}
		cases = append(cases, arenaDomainCase{Name: name, Length: n, Arena: arena, SourceBase: base, SourceStart: start, Regions: r})
	}
	for _, n := range []int{0, 1, 2, 255} {
		add(fmt.Sprintf("equal%d", n), n, 0x7fff, 0x9000, 0, -1)
	}
	for _, n := range []int{1, 2, 255} {
		add(fmt.Sprintf("first_mismatch%d", n), n, 0x7fff, 0x9000, 0, 0)
		add(fmt.Sprintf("last_mismatch%d", n), n, 0x7fff, 0x9000, 0, n-1)
	}
	add("same_alias255", 255, 0x7fff, 0x8000, 0, -1)
	add("overlap_plus1_255", 255, 0x7fff, 0x8001, 0, -1)
	add("overlap_minus1_255", 255, 0x8000, 0x8000, 0, -1)
	add("source_wrap255", 255, 0x7fff, 0xff00, 0x9100, -1)
	workloads := map[string][]arenaDomainCase{"successful14": cases}
	workloads["first_left_denied"] = []arenaDomainCase{{Name: "first_left_denied", Length: 1, Arena: 65535, SourceBase: 0x9000, Regions: regions()}}
	workloads["first_right_denied"] = []arenaDomainCase{{Name: "first_right_denied", Length: 1, Arena: 0x7fff, SourceBase: 0xa000, Regions: regions()}}
	workloads["tail_left_denied"] = []arenaDomainCase{{Name: "tail_left_denied", Length: 3, Arena: 0xfffd, SourceBase: 0x9000, Regions: regions()}}
	workloads["tail_right_denied"] = []arenaDomainCase{{Name: "tail_right_denied", Length: 3, Arena: 0x7fff, SourceBase: 0xfffe, Regions: regions()}}
	dir := os.Getenv("MINZ_ARENA_PEEL_ARTIFACTS")
	for _, name := range []string{"successful14", "first_left_denied", "first_right_denied", "tail_left_denied", "tail_right_denied"} {
		t.Run(name, func(t *testing.T) {
			cs := workloads[name]
			w := arenaDomainWorkload(cs)
			for _, kind := range []string{"original", "variant"} {
				factory := originalFactory
				if kind == "variant" {
					factory = func() (*hir.Module, error) {
						source, e := originalFactory()
						if e != nil {
							return nil, e
						}
						actual, e := originalFactory()
						if e != nil {
							return nil, e
						}
						if e = peelMemoryHostArenaFirstByte(actual, w); e != nil {
							return nil, e
						}
						if e = hir.ValidateMemoryHostArenaPeel(source, actual, w); e != nil {
							return nil, e
						}
						return actual, nil
					}
				}
				evidence, e := RunZ80MemoryHostBaseline(factory, w)
				if e != nil {
					t.Fatal(kind, e)
				}
				if len(evidence.Target.Invocations) != len(cs) {
					t.Fatal("invocation prefix count")
				}
				for j, c := range cs {
					results, events, failure := arenaDomainOracle(c)
					for _, run := range []*hir.MemoryHostRun{evidence.Raw, {Invocations: evidence.Target.Invocations}} {
						got := run.Invocations[j]
						if !reflect.DeepEqual(got.Results, results) || !reflect.DeepEqual(got.Events, events) || !reflect.DeepEqual(got.Failure, failure) {
							t.Fatalf("%s %s result/event/failure mismatch", kind, c.Name)
						}
						for n, r := range c.Regions {
							if !reflect.DeepEqual(got.Regions[n], r.Bytes) {
								t.Fatal("resource changed", n)
							}
						}
						for n, initial := range run.Invocations[0].Globals {
							want := initial
							if n == "src_buf" {
								want = []byte{byte(c.SourceBase), byte(c.SourceBase >> 8)}
							}
							if !reflect.DeepEqual(got.Globals[n], want) {
								t.Fatal("global changed", n)
							}
						}
					}
				}
				if dir != "" {
					p := filepath.Join(dir, "target-domain", name, kind)
					if e = os.MkdirAll(p, 0755); e != nil {
						t.Fatal(e)
					}
					for n, v := range map[string]any{"factory-baseline.json": evidence, "cases.json": cs} {
						b, e := json.MarshalIndent(v, "", "  ")
						if e != nil {
							t.Fatal(e)
						}
						if e = os.WriteFile(filepath.Join(p, n), b, 0644); e != nil {
							t.Fatal(e)
						}
					}
					for n, b := range map[string][]byte{"target.bin": evidence.Target.Binary, "target.a80": []byte(evidence.Target.Assembly), "final.mir2": []byte(evidence.Target.FinalMIRDump), "allocation.json": evidence.Target.AllocationJSON} {
						if e = os.WriteFile(filepath.Join(p, n), b, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
				t.Logf("%s %d calls %dT/%dB", kind, len(cs), evidence.Target.TotalTStates, len(evidence.Target.Binary))
			}
		})
	}
}
