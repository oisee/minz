package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
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

// TestUnsignedPairPolicyOriginalParserExport measures only the public v2 API.
// The explicit environment gate avoids adding this artifact run to broad tests.
func TestUnsignedPairPolicyOriginalParserExport(t *testing.T) {
	dir := os.Getenv("MINZ_UNSIGNED_PAIR_POLICY_ARTIFACTS")
	if dir == "" {
		t.Skip("explicit two-case public API artifact export")
	}
	refPath := os.Getenv("MINZ_UNSIGNED_PAIR_POLICY_REFERENCE")
	if refPath == "" {
		t.Fatal("current production reference required")
	}
	b, e := os.ReadFile(refPath)
	if e != nil {
		t.Fatal(e)
	}
	var reference MemoryHostBaselineEvidence
	if e = json.Unmarshal(b, &reference); e != nil {
		t.Fatal(e)
	}
	source, e := os.ReadFile("../../../docs/grace-parser-prospective-heldout/artifacts/source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(source)) != "0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526" {
		t.Fatal("original source changed")
	}
	_, w := memoryHostParserFixture(t)
	factory := func() (*hir.Module, error) {
		m, e := nanz.Parse(string(source), "../examples/nanz/self_lanz_parser.nanz")
		if e == nil {
			m.Target = hir.TargetCPM
		}
		return m, e
	}
	var original *MemoryHostBaselineEvidenceV2
	for _, policy := range []mir2.UnsignedPairComparePolicy{mir2.UnsignedPairCompareLegacy, mir2.UnsignedPairComparePreserveForSpeed} {
		name := "legacy"
		if policy == mir2.UnsignedPairComparePreserveForSpeed {
			name = "preserve-speed"
		}
		out := filepath.Join(dir, name)
		if e = os.MkdirAll(out, 0755); e != nil {
			t.Fatal(e)
		}
		v2, e := RunZ80MemoryHostBaselineWithOptions(factory, w, MemoryHostBaselineOptions{UnsignedPairComparePolicy: policy})
		if e != nil {
			if writeErr := os.WriteFile(filepath.Join(out, "error.txt"), []byte(e.Error()+"\n"), 0644); writeErr != nil {
				t.Fatal(writeErr)
			}
			t.Fatal(e)
		}
		payload, e := json.MarshalIndent(v2, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		target := v2.Baseline.Target
		for name, data := range map[string][]byte{"baseline-v2.json": payload, "source.nanz": source, "target.a80": []byte(target.Assembly), "target.bin": target.Binary, "final.mir2": []byte(target.FinalMIRDump), "allocation.json": target.AllocationJSON} {
			if e = os.WriteFile(filepath.Join(out, name), data, 0644); e != nil {
				t.Fatal(e)
			}
		}
		if v2.Schema != "minz.memory-host-baseline/v2" || v2.CompileOptions.UnsignedPairComparePolicy != policy {
			t.Fatal("resolved v2 policy")
		}
		if policy == mir2.UnsignedPairCompareLegacy {
			a, b := target, reference.Target
			if a.BinarySHA256 != b.BinarySHA256 || a.Assembly != b.Assembly || a.AdapterSHA256 != b.AdapterSHA256 || a.TotalTStates != b.TotalTStates || a.ExecutedSteps != b.ExecutedSteps || a.MIRSHA256 != b.MIRSHA256 || a.AllocationSHA256 != b.AllocationSHA256 || a.SidecarSHA256 != b.SidecarSHA256 || a.CallFramesSHA256 != b.CallFramesSHA256 || !reflect.DeepEqual(a.Invocations, b.Invocations) || !reflect.DeepEqual(a.InvocationTStates, b.InvocationTStates) {
				t.Fatal("public Legacy disagrees with current production")
			}
			original = v2
		} else {
			if !reflect.DeepEqual(original.Baseline.Raw, v2.Baseline.Raw) || !reflect.DeepEqual(original.Baseline.Compatibility, v2.Baseline.Compatibility) || !reflect.DeepEqual(original.Baseline.FunctionSites, v2.Baseline.FunctionSites) {
				t.Fatal("public policy changed source oracle")
			}
			if e = arenaSameObservations(original.Baseline.Target.Invocations, target.Invocations); e != nil {
				t.Fatal(e)
			}
		}
		t.Logf("%s publicv2%dT/%dB steps%d BIN%s context%s", name, target.TotalTStates, len(target.Binary), target.ExecutedSteps, target.BinarySHA256, target.Context.CompilerID)
	}
}
