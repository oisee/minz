package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMemoryHostArenaAppendConstruction(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		original, _ := memoryHostParserFixture(t)
		source, w := memoryHostParserFixture(t)
		units := source.SourceUnits
		arenaLoop := source.FuncByName("region_match").Body.Body[1]
		appendLoop := source.FuncByName("intern_region").Body.Body[5]
		arenaOrigin := source.SourceLocations[arenaLoop]
		appendOrigin := source.SourceLocations[appendLoop]
		if e := constructMemoryHostArenaAppend(source, w, mask&1 != 0, mask&2 != 0); e != nil {
			t.Fatal(mask, e)
		}
		ap, cp := 1, 5
		if mask&1 != 0 {
			ap = 4
		}
		if mask&2 != 0 {
			cp = 7
		}
		if source.FuncByName("region_match").Body.Body[ap] != arenaLoop || source.FuncByName("intern_region").Body.Body[cp] != appendLoop || source.SourceLocations[arenaLoop] != arenaOrigin || source.SourceLocations[appendLoop] != appendOrigin || !reflect.DeepEqual(units, source.SourceUnits) {
			t.Fatal("sourceidentity changed", mask)
		}
		if e := hir.ValidateMemoryHostArenaAppendChoices(original, source, w, hir.MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}); e != nil {
			t.Fatal(mask, e)
		}
		if mask == 3 {
			if hir.ValidateMemoryHostArenaPeel(original, source, w) == nil || hir.ValidateMemoryHostAppendCopyHoist(original, source, w) == nil {
				t.Fatal("individualproofacceptedcombineddelta")
			}
		}
		if e := hir.Verify(source); e != nil {
			t.Fatal(mask, e)
		}
	}
}

// Four fresh candidates share one immutable original workload and compiler
// context. Costs are observed separately, never composed from prior results.
func TestMemoryHostArenaAppendPrototypeOriginal30(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	var original *MemoryHostBaselineEvidence
	for mask := 0; mask < 4; mask++ {
		factory := func() (*hir.Module, error) {
			m, _ := memoryHostParserFixture(t)
			original, _ := memoryHostParserFixture(t)
			if e := constructMemoryHostArenaAppend(m, w, mask&1 != 0, mask&2 != 0); e != nil {
				return nil, e
			}
			if e := hir.ValidateMemoryHostArenaAppendChoices(original, m, w, hir.MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}); e != nil {
				return nil, e
			}
			return m, nil
		}
		evidence, e := RunZ80MemoryHostBaseline(factory, w)
		if e != nil {
			t.Fatal(mask, e)
		}
		if original == nil {
			original = evidence
		} else {
			if e := arenaSameObservations(original.Raw.Invocations, evidence.Raw.Invocations); e != nil {
				t.Fatal(mask, e)
			}
			if e := arenaSameObservations(original.Target.Invocations, evidence.Target.Invocations); e != nil {
				t.Fatal(mask, e)
			}
		}
		t.Logf("mask%d actual %dT/%dB SHA%s", mask, evidence.Target.TotalTStates, len(evidence.Target.Binary), evidence.Target.BinarySHA256)
		if d := os.Getenv("MINZ_ARENA_APPEND_ARTIFACTS"); d != "" {
			names := []string{"original", "arena", "append", "both"}
			p := filepath.Join(d, names[mask])
			if e := os.MkdirAll(p, 0755); e != nil {
				t.Fatal(e)
			}
			shape, _ := memoryHostParserFixture(t)
			if e := constructMemoryHostArenaAppend(shape, w, mask&1 != 0, mask&2 != 0); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(p, "input.hir"), []byte(shape.Dump()), 0644); e != nil {
				t.Fatal(e)
			}
			b, e := json.MarshalIndent(evidence, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			for n, b := range map[string][]byte{"factory-baseline.json": b, "target.bin": evidence.Target.Binary, "target.a80": []byte(evidence.Target.Assembly), "final.mir2": []byte(evidence.Target.FinalMIRDump), "allocation.json": evidence.Target.AllocationJSON} {
				if e := os.WriteFile(filepath.Join(p, n), b, 0644); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
}
