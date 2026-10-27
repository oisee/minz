package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"reflect"
	"testing"
)

func TestMeasuredContractStrategiesTieAndRefusals(t *testing.T) {
	fresh, w := modeFixture()
	r, e := MeasureContractStrategies(fresh, w, "speed", Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Evidence.Candidates) != 3 || r.Evidence.Selected != 0 || r.Steps.Assembly != r.Evidence.Candidates[0].Assembly || !reflect.DeepEqual(r.Binary, r.Evidence.Candidates[0].Binary) {
		t.Fatal("fixed tie/winner")
	}
	for _, candidate := range r.Evidence.Candidates {
		if candidate.Run.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(candidate.Binary)) || len(candidate.Run.Invocations) != len(w.Calls) {
			t.Fatal("candidate differs from executed binary or workload")
		}
	}
	for _, kind := range []string{"shared", "source", "latefailure", "lateABI", "metadata"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			shared, _ := fresh()
			var prior *hir.Module
			factory := func() (*hir.Module, error) {
				calls++
				m, e := fresh()
				if kind == "shared" {
					m = shared
				}
				if kind == "source" && calls == 2 {
					m.Name = "forged"
				}
				if kind == "metadata" && calls == 2 {
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: "arithmetic", Expected: 777})
				}
				if kind == "latefailure" && calls == 5 {
					return nil, errors.New("failed after measurements")
				}
				if kind == "lateABI" && calls == 5 && prior != nil {
					prior.FuncByName("arithmetic").Params[0].Ty = nil
				}
				if calls == 2 {
					prior = m
				}
				return m, e
			}
			result, e := MeasureContractStrategies(factory, w, "size", Options{AssertMode: "none"})
			if e == nil || result != nil {
				t.Fatal("partial result", kind)
			}
		})
	}
	for _, opts := range []Options{{AssertMode: "none", ContractStrategy: "forged"}, {AssertMode: "none", ContractOpt: true}, {AssertMode: "none", CheckedLocalTablePath: "x"}, {AssertMode: "none", UseGrace: true}} {
		result, e := MeasureContractStrategies(fresh, w, "size", opts)
		if e == nil || result != nil {
			t.Fatal("unsupported config")
		}
	}
}
func TestMeasuredContractStrategiesOriginalArena(t *testing.T) {
	const path = "../../../examples/nanz/08_arena_allocator.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(src), path) }
	// Frozen training and heldout choices precede any selector measurement. The
	// heldout checks validation, never participates in selection or retuning.
	train := workloadSpec(hir.ProfileInvocation{Function: "test_split"})
	train.ExactDataAccess = true
	heldout := workloadSpec(hir.ProfileInvocation{Function: "test_alloc_sequence"}, hir.ProfileInvocation{Function: "test_oom"}, hir.ProfileInvocation{Function: "test_small_allocs"}, hir.ProfileInvocation{Function: "test_typed_alloc"})
	heldout.ExactDataAccess = true
	for _, goal := range []string{"size", "speed"} {
		result, e := MeasureContractStrategies(factory, train, goal, Options{AssertMode: "none", OptSize: true})
		if e != nil {
			t.Fatal(e)
		}
		// off wins both goals: smallest whole-module binary, and — since O4+
		// affine pointer strength reduction (2b70eaa74) lowered the all-original
		// candidate from 3803 to 3740 T while PBQP stayed at 3751 T — also the
		// fewest T-states on the frozen training workload. Earlier candidates
		// win ties, so off also wins speed whenever PBQP merely matches it.
		expected := 0
		if result.Evidence.Selected != expected {
			t.Fatalf("%s selected %d, expected %d", goal, result.Evidence.Selected, expected)
		}
		for i, c := range result.Evidence.Candidates {
			if len(c.Run.Invocations) != 1 || c.Run.Invocations[0].Results[0].I != 0 {
				t.Fatal("independent train expected0")
			}
			t.Logf("%s candidate%d B%d T%d", goal, i, len(c.Binary), c.Run.TotalTStates)
		}
		h, _ := factory()
		oracle, e := hir.RunLoopProfile(h, heldout)
		if e != nil {
			t.Fatal(e)
		}
		run, e := RunZ80ProfileWorkload(result.Steps, h, heldout, *oracle)
		if e != nil {
			t.Fatal(e)
		}
		for _, inv := range run.Invocations {
			if len(inv.Results) != 1 || inv.Results[0].I != 0 {
				t.Fatal("independent source expected0", inv)
			}
		}
		t.Logf("%s selected%d heldout%dT", goal, result.Evidence.Selected, run.TotalTStates)
		if dir := os.Getenv("CONTRACT_SELECTOR_EXPORT"); dir != "" {
			os.MkdirAll(dir, 0755)
			for n, v := range map[string]any{goal + ".json": result.Evidence, goal + "-heldout.json": run, "frozen-train.json": train, "frozen-heldout.json": heldout} {
				b, _ := json.MarshalIndent(v, "", "  ")
				os.WriteFile(dir+"/"+n, b, 0644)
			}
			os.WriteFile(dir+"/"+goal+".a80", []byte(result.Steps.Assembly), 0644)
			os.WriteFile(dir+"/"+goal+".bin", result.Binary, 0644)
			os.WriteFile(dir+"/source.nanz", src, 0644)
			for i, c := range result.Evidence.Candidates {
				tag := string(rune('0' + i))
				os.WriteFile(dir+"/"+goal+"-candidate"+tag+".a80", []byte(c.Assembly), 0644)
				os.WriteFile(dir+"/"+goal+"-candidate"+tag+".bin", c.Binary, 0644)
			}
		}
	}
}

func TestMeasuredContractStrategiesOwnsWorkloadAndRejectsDomains(t *testing.T) {
	fresh, w := modeFixture()
	original := *w.Calls[0].Arguments[0].Value
	factory := func() (*hir.Module, error) { *w.Calls[0].Arguments[0].Value = 0; return fresh() }
	r, e := MeasureContractStrategies(factory, w, "speed", Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	if *r.Evidence.Workload.Calls[0].Arguments[0].Value != original || r.Evidence.Candidates[0].Run.Invocations[0].Results[0].I != 1 {
		t.Fatal("caller mutated owned workload")
	}
	for _, text := range []string{`@extern fun peek(addr:u16)->u8
fun f()->u8{return peek(49152)}`, `fun f()->u8{@print("hi") return 1}`} {
		f := func() (*hir.Module, error) { return nanz.Parse(text, "unsupported.nanz") }
		result, e := MeasureContractStrategies(f, workloadSpec(hir.ProfileInvocation{Function: "f"}), "speed", Options{AssertMode: "none"})
		if e == nil || result != nil {
			t.Fatal("memory/output domain silently accepted")
		}
	}
	// A candidate compile cannot succeed if the profile references an absent function.
	result, e := MeasureContractStrategies(fresh, workloadSpec(hir.ProfileInvocation{Function: "forged"}), "size", Options{AssertMode: "none"})
	if e == nil || result != nil {
		t.Fatal("forged workload accepted")
	}
}
