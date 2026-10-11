package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// Explicit artifact export is useful when a transformed candidate is refused
// before the public runner can produce a complete target attestation.
func TestMemoryHostArenaPeelDiagnostic(t *testing.T) {
	dir := os.Getenv("MINZ_ARENA_PEEL_ARTIFACTS")
	if dir == "" {
		t.Skip("explicit artifact directory required")
	}
	m, w := memoryHostParserFixture(t)
	if e := peelMemoryHostArenaFirstByte(m, w); e != nil {
		t.Fatal(e)
	}
	steps, e := CompileHIRSteps(m, Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	records := []mir2.NativeMemoryAccess{}
	assembly, e := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{NativeMemoryAccessOrigins: hir.MemoryHostGlobalOrigins(steps.MIR2Module), NativeMemoryAccesses: &records})
	if e != nil {
		t.Fatal(e)
	}
	assembly = emitExternStubs(assembly, steps.MIR2Module)
	assembly = strings.Replace(assembly, "peek:\n", "peek:\n    LD A,(HL)\n    RET\n", 1)
	assembly = strings.Replace(assembly, "poke:\n", "poke:\n    LD (HL),C\n    RET\n", 1)
	assembled, e := z80asm.NewAssembler().AssembleString(fmt.Sprintf("ORG 0x%04x\n", w.Layout.CodeOrigin) + assembly)
	if e != nil || len(assembled.Errors) > 0 {
		t.Fatal(e, assembled.Errors)
	}
	p := filepath.Join(dir, "refused-variant-diagnostic")
	if e = os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	for n, v := range map[string]any{"records.json": records, "listing.json": assembled.Listing, "allocation.json": steps.Allocation} {
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(p, n), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for n, b := range map[string][]byte{"target.a80": []byte(assembly), "target.bin": assembled.Binary, "final.mir2": []byte(steps.MIR2Module.Dump())} {
		if e = os.WriteFile(filepath.Join(p, n), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
}
