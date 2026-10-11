package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

func TestMemoryHostLoopAssociationOriginalParser(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	r, err := RunZ80MemoryHostLoopProfile(factory, w)
	if err != nil {
		t.Fatal(err)
	}
	if r.Profile.CompletedCalls != 30 || r.Profile.FailedCalls != 0 || len(r.Profile.Counts) != 11 {
		t.Fatal("wrong invocation/site boundary")
	}
	found := false
	for i, s := range r.Profile.Sites.Sites {
		if s.Function == "region_match" {
			found = true
			if r.Profile.Counts[i].Entries != 49 || r.Profile.Counts[i].BodyEntries != 59 {
				t.Fatal(r.Profile.Counts[i])
			}
		}
	}
	if !found {
		t.Fatal("region_match absent")
	}
	if r.Baseline.Target.TotalTStates != 1221494 || r.Baseline.Target.BinarySHA256 != "fed98ba78becf9431a5301f31330363cc3005f1f6692fd1f9e9183d80b9d0a51" {
		t.Fatal("instrumentation changed native baseline")
	}
	if dir := os.Getenv("MINZ_MEMORY_HOST_LOOP_ARTIFACTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "loop-baseline-association.json"), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMemoryHostLoopAssociationRejectsDelayedSourceMutation(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	for _, delayed := range []int{4, 5} {
		t.Run(string(rune('0'+delayed)), func(t *testing.T) {
			calls := 0
			var original *hir.Module
			factory := func() (*hir.Module, error) {
				m, _ := memoryHostParserFixture(t)
				calls++
				if calls == 1 {
					original = m
				}
				if calls == delayed {
					original.Name = "mutated-retained-source"
				}
				return m, nil
			}
			if r, err := RunZ80MemoryHostLoopProfile(factory, w); err == nil || r != nil {
				t.Fatal("retained source mutation accepted")
			}
		})
	}
}
