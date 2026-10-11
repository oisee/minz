package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"strings"
	"testing"
)

func TestUnsignedPairPolicyAdmission(t *testing.T) {
	for _, o := range []Options{{UnsignedPairComparePolicy: 99}, {UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed, OptSize: true}, {UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed, Backend: "ez80"}} {
		if _, e := CompileHIRSteps(nil, o); e == nil {
			t.Fatal("invalid steps policy accepted")
		}
		if _, e := CompileHIRWithOptions(nil, o); e == nil {
			t.Fatal("invalid secondary policy accepted")
		}
	}
	calls := 0
	_, e := RunZ80MemoryHostBaselineWithOptions(func() (*hir.Module, error) { calls++; return nil, nil }, hir.MemoryHostWorkload{}, MemoryHostBaselineOptions{UnsignedPairComparePolicy: 99})
	if e == nil || calls != 0 {
		t.Fatal("policy must refuse before factory", calls, e)
	}
	legacy, e := ProfileCompilationID(Options{}, "default")
	if e != nil {
		t.Fatal(e)
	}
	// Reconstruct the exact former Options object by removing the single new
	// field from its ordered JSON before hashing the existing wrapper.
	encodedOptions, e := json.Marshal(Options{})
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(encodedOptions, []byte("UnsignedPairComparePolicy")) {
		t.Fatal("Legacy config gained an optional field")
	}
	oldConfig := append([]byte(`{"Options":`), encodedOptions...)
	oldConfig = append(oldConfig, []byte(`,"AssemblerMode":"default"}`)...)
	if legacy != fmt.Sprintf("%x", sha256.Sum256(oldConfig)) {
		t.Fatal("Legacy compilation ID changed")
	}
	changed, e := ProfileCompilationID(Options{UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed}, "default")
	if e != nil || legacy == changed {
		t.Fatal("ordinary profile policy binding", e)
	}
}

const unsignedPairSource = `@extern fun peek(addr:u16)->u8
global result:u16=0
fun cmp_store(a:u16,b:u16)->u16 {
 if a < b {
 result = a
 return a
 }
 result = b
 return b
}`

func pairSource(t *testing.T) *hir.Module {
	t.Helper()
	m, e := nanz.Parse(unsignedPairSource, "pair-policy.nanz")
	if e != nil {
		t.Fatal(e)
	}
	m.Target = hir.TargetCPM
	return m
}

func TestUnsignedPairPolicyAllocatedSource(t *testing.T) {
	legacy, e := CompileHIRSteps(pairSource(t), Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	explicitLegacy, e := CompileHIRSteps(pairSource(t), Options{AssertMode: "none", UnsignedPairComparePolicy: mir2.UnsignedPairCompareLegacy})
	if e != nil {
		t.Fatal(e)
	}
	if legacy.Assembly != explicitLegacy.Assembly || legacy.MIR2Raw != explicitLegacy.MIR2Raw {
		t.Fatal("legacy default drift")
	}
	sizeDefault, e := CompileHIRSteps(pairSource(t), Options{AssertMode: "none", OptSize: true})
	if e != nil {
		t.Fatal(e)
	}
	sizeLegacy, e := CompileHIRSteps(pairSource(t), Options{AssertMode: "none", OptSize: true, UnsignedPairComparePolicy: mir2.UnsignedPairCompareLegacy})
	if e != nil || sizeDefault.Assembly != sizeLegacy.Assembly {
		t.Fatal("OptSize legacy default drift", e)
	}
	selected, e := CompileHIRSteps(pairSource(t), Options{AssertMode: "none", UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed})
	if e != nil {
		t.Fatal(e)
	}
	if selected.UnsignedPairComparePolicy != mir2.UnsignedPairComparePreserveForSpeed || selected.MIR2Raw != legacy.MIR2Raw {
		t.Fatal("Steps/source binding")
	}
	if !strings.Contains(selected.Assembly, ".ucmp_pair_done_") {
		t.Fatal("automatically allocated typed source did not activate policy")
	}
	if _, e = CompileHIRWithOptions(pairSource(t), Options{UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed}); e != nil {
		t.Fatal(e)
	}
	for _, args := range [][2]int64{{0, 65535}, {65535, 0}, {32768, 32767}, {7, 7}} {
		w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"data": {Address: 0x8000, Bytes: []byte{7}}}, Calls: []hir.MemoryHostInvocation{{Function: "cmp_store", Arguments: []int64{args[0], args[1]}}}}
		factory := func() (*hir.Module, error) { return nanz.Parse(unsignedPairSource, "pair-policy.nanz") }
		old, e := RunZ80MemoryHostBaseline(factory, w)
		if e != nil {
			t.Fatal(e)
		}
		v2Legacy, e := RunZ80MemoryHostBaselineWithOptions(factory, w, MemoryHostBaselineOptions{})
		if e != nil || !reflect.DeepEqual(old, v2Legacy.Baseline) || v2Legacy.CompileOptions.UnsignedPairComparePolicy != mir2.UnsignedPairCompareLegacy {
			t.Fatal("v2 Legacy must equal old v1", e)
		}
		v2, e := RunZ80MemoryHostBaselineWithOptions(factory, w, MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed})
		if e != nil {
			t.Fatal(e)
		}
		if v2.Schema != "minz.memory-host-baseline/v2" || v2.CompileOptions.UnsignedPairComparePolicy != mir2.UnsignedPairComparePreserveForSpeed || old.Schema != "minz.memory-host-baseline/v1" {
			t.Fatal("version/policy")
		}
		if !reflect.DeepEqual(old.Raw, v2.Baseline.Raw) || !reflect.DeepEqual(old.Compatibility, v2.Baseline.Compatibility) {
			t.Fatal("source oracle changed")
		}
		if e = arenaSameObservations(old.Target.Invocations, v2.Baseline.Target.Invocations); e != nil {
			t.Fatal(e)
		}
		want := args[0]
		if args[1] < want {
			want = args[1]
		}
		got := v2.Baseline.Target.Invocations[0]
		if len(got.Results) != 1 || got.Results[0].I != want || !reflect.DeepEqual(got.Globals["result"], []byte{byte(want), byte(want >> 8)}) {
			t.Fatal("independent source result/global", got, want)
		}
		encodedV2, e := json.Marshal(v2)
		if e != nil || !strings.Contains(string(encodedV2), `"unsigned_pair_compare_policy":1`) {
			t.Fatal("v2 explicit policy", e)
		}
		if args == [2]int64{0, 65535} {
			source, e := factory()
			if e != nil {
				t.Fatal(e)
			}
			oracle, e := hir.RunMemoryHostBaseline(source, w)
			if e != nil {
				t.Fatal(e)
			}
			changed, e := CompileHIRSteps(pairSource(t), Options{AssertMode: "none", UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed})
			if e != nil {
				t.Fatal(e)
			}
			changed.UnsignedPairComparePolicy = mir2.UnsignedPairCompareLegacy
			target, e := runZ80MemoryHostWorkload(changed, source, w, *oracle)
			if e == nil || target != nil || !strings.Contains(e.Error(), "native annotation changed private baseline binary") {
				t.Fatal("tampered Steps policy must refuse before native", e)
			}
		}
		encoded, e := json.Marshal(old)
		if e != nil || strings.Contains(string(encoded), "compile_options") {
			t.Fatal("legacy schema changed", e)
		}
	}
}
