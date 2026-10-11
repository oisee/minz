package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMemoryHostArenaPeelPrototypeOriginal30(t *testing.T) {
	source, w := memoryHostParserFixture(t)
	before := source.SourceUnits
	originalFactory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	original, e := RunZ80MemoryHostBaseline(originalFactory, w)
	if e != nil {
		t.Fatal(e)
	}
	variantFactory := func() (*hir.Module, error) {
		m, _ := memoryHostParserFixture(t)
		e := peelMemoryHostArenaFirstByte(m, w)
		if !reflect.DeepEqual(before, m.SourceUnits) {
			t.Fatal("source SHA changed")
		}
		return m, e
	}
	variant, e := RunZ80MemoryHostBaseline(variantFactory, w)
	if e != nil {
		t.Fatal(e)
	}
	for j, a := range original.Raw.Invocations {
		b := variant.Raw.Invocations[j]
		a.Steps = 0
		b.Steps = 0
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("raw semantic observations changed at %d", j)
		}
		a = original.Target.Invocations[j]
		b = variant.Target.Invocations[j]
		a.Steps = 0
		b.Steps = 0
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("native semantic observations changed at %d", j)
		}
	}
	dir := os.Getenv("MINZ_ARENA_PEEL_ARTIFACTS")
	if dir != "" {
		for name, evidence := range map[string]*MemoryHostBaselineEvidence{"original": original, "variant": variant} {
			p := filepath.Join(dir, name)
			if e = os.MkdirAll(p, 0755); e != nil {
				t.Fatal(e)
			}
			for name, value := range map[string]any{"factory-baseline.json": evidence, "target.json": evidence.Target, "raw.json": evidence.Raw, "workload.json": w} {
				b, e := json.MarshalIndent(value, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(p, name), b, 0644); e != nil {
					t.Fatal(e)
				}
			}
			for name, b := range map[string][]byte{"target.bin": evidence.Target.Binary, "target.a80": []byte(evidence.Target.Assembly), "final.mir2": []byte(evidence.Target.FinalMIRDump), "allocation.json": evidence.Target.AllocationJSON} {
				if e = os.WriteFile(filepath.Join(p, name), b, 0644); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	t.Logf("actual original %dT/%dB variant %dT/%dB", original.Target.TotalTStates, len(original.Target.Binary), variant.Target.TotalTStates, len(variant.Target.Binary))
}
