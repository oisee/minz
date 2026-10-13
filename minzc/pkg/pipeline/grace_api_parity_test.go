package pipeline

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestMIRGraceAssemblyAPIUsesSameNativePipeline(t *testing.T) {
	fixture := buildTypedInternFixture(t)
	for _, size := range []bool{false, true} {
		h, err := fixture.factory()
		if err != nil {
			t.Fatal(err)
		}
		original, err := fixture.factory()
		if err != nil {
			t.Fatal(err)
		}
		options := Options{ContractOpt: true, UseGrace: true, OptSize: size, AssertMode: "none", GraceStats: mir2.NewGraceStats()}
		steps, err := CompileHIRSteps(h, options)
		if err != nil {
			t.Fatal(err)
		}
		h, err = fixture.factory()
		if err != nil {
			t.Fatal(err)
		}
		options.GraceStats = mir2.NewGraceStats()
		asm, err := CompileHIRWithOptions(h, options)
		if err != nil {
			t.Fatal(err)
		}
		if !size && options.GraceStats.ByRule["invariant-address-hoist"] == 0 {
			t.Fatal("assembly API ignored MIR Grace")
		}
		if size && options.GraceStats.ByRule["invariant-address-hoist"] != 0 {
			t.Fatal("assembly API ignored Size policy")
		}
		a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if err != nil || len(a.Errors) > 0 {
			t.Fatal(err, a.Errors)
		}
		b, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil || len(b.Errors) > 0 {
			t.Fatal(err, b.Errors)
		}
		if !bytes.Equal(a.Binary, b.Binary) {
			t.Fatal("Grace APIs generated different machine code")
		}
		steps.Assembly = asm
		expected, err := hir.RunLoopProfile(original, fixture.workload)
		if err != nil {
			t.Fatal(err)
		}
		run, err := RunZ80ProfileWorkload(steps, original, fixture.workload, *expected)
		if err != nil {
			t.Fatal(err)
		}
		for i, inv := range run.Invocations {
			if inv.Results[0].I != fixture.results[i] || !reflect.DeepEqual(inv.Snapshot, fixture.snapshots[i]) {
				t.Fatal("independent native intern model", size, i)
			}
			for name, access := range inv.Access.Resources {
				if !reflect.DeepEqual(access.Reads, fixture.reads[i][name]) || !reflect.DeepEqual(access.Writes, fixture.writes[i][name]) {
					t.Fatal("independent native memory traffic", size, i, name)
				}
			}
		}
	}
}
