package mir2_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func loadStepAllocation(t *testing.T, source string) (*mir2.Module, *mir2.AllocResult) {
	t.Helper()
	m, e := nanz.Parse(source, "load-step.nanz")
	if e != nil {
		t.Fatal(e)
	}
	steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	return steps.MIR2Module, steps.Allocation
}

func assembleOrFatal(t *testing.T, text string) []byte {
	t.Helper()
	a, e := z80asm.NewAssembler().AssembleString(text)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	return a.Binary
}

// The binary a program ships must not depend on whether annotation sinks were
// attached when it was re-emitted for measurement.
func TestByteLoadStepHomeAnnotationSinksPreserveBinary(t *testing.T) {
	module, allocation := loadStepAllocation(t, "fun add2(p: ^u8) -> u8 { return p^ + 2 }\n")
	plain, e := mir2.Z80Codegen(module, allocation)
	if e != nil {
		t.Fatal(e)
	}
	records := []mir2.NativeMemoryAccess{}
	frames := []mir2.NativeCallFrameSnapshot{}
	spills := []mir2.NativeCallSpillSnapshot{}
	annotated, e := mir2.Z80Codegen(module, allocation, mir2.Z80CodegenOptions{
		NativeMemoryAccessOrigins: hir.MemoryHostGlobalOrigins(module),
		NativeMemoryAccesses:      &records,
		NativeCallFrames:          &frames,
		NativeCallSpills:          &spills,
	})
	if e != nil {
		t.Fatal(e)
	}
	plainBytes := assembleOrFatal(t, plain)
	annotatedBytes := assembleOrFatal(t, annotated)
	if !reflect.DeepEqual(plainBytes, []byte{0x7e, 0x3c, 0x3c, 0xc9}) {
		t.Fatalf("plain add2 lost the fused 4-byte form: % x", plainBytes)
	}
	if !reflect.DeepEqual(plainBytes, annotatedBytes) {
		t.Fatalf("annotation sinks changed the emitted binary: plain % x annotated % x", plainBytes, annotatedBytes)
	}
	if strings.Contains(annotated, "LD D, (HL)") || strings.Contains(annotated, "LD A, D") {
		t.Fatalf("annotated emission still copies through the load's allocated home:\n%s", annotated)
	}
}

// preferByteLoadStepHome only rehomes a load whose single consumer is an
// adjacent small add/sub; the load instruction is still emitted, so a semantic
// global access must still be recorded, anchored on that load.
func TestByteLoadStepHomeAnnotationSinksRecordAccesses(t *testing.T) {
	module, allocation := loadStepAllocation(t, "global cell: u8 = 5\nfun addcell() -> u8 { return cell + 2 }\n")
	plain, e := mir2.Z80Codegen(module, allocation)
	if e != nil {
		t.Fatal(e)
	}
	plainBytes := assembleOrFatal(t, plain)
	records := []mir2.NativeMemoryAccess{}
	frames := []mir2.NativeCallFrameSnapshot{}
	spills := []mir2.NativeCallSpillSnapshot{}
	annotated, e := mir2.Z80Codegen(module, allocation, mir2.Z80CodegenOptions{
		NativeMemoryAccessOrigins: hir.MemoryHostGlobalOrigins(module),
		NativeMemoryAccesses:      &records,
		NativeCallFrames:          &frames,
		NativeCallSpills:          &spills,
	})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(plainBytes, assembleOrFatal(t, annotated)) {
		t.Fatal("annotation sinks changed the emitted binary")
	}
	origins := hir.MemoryHostGlobalOrigins(module)
	if len(origins) != 1 {
		t.Fatalf("expected exactly one semantic global load, got %d", len(origins))
	}
	if len(records) != len(origins) {
		t.Fatalf("incomplete native access list: %d origins, %d records", len(origins), len(records))
	}
	for _, r := range records {
		if r.Write || r.Root.Symbol != "cell" || r.Bytes != 1 || r.StorageKind != "global" {
			t.Fatalf("unexpected access record %+v", r)
		}
	}
	if e := mir2.CheckNativeMemoryAccessMarkers(annotated, records); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(annotated, "LD A, (HL)") {
		t.Fatalf("recorded access is not anchored on the rehomed load:\n%s", annotated)
	}
}
