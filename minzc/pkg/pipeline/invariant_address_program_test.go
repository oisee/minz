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
	"testing"
)

func TestInvariantAddressRealInternProgram(t *testing.T) {
	f := buildTypedInternFixture(t)
	dir := os.Getenv("MINZ_INVARIANT_ADDRESS_EVIDENCE")
	save := func(name string, v any) {
		if dir == "" {
			return
		}
		var b []byte
		var e error
		switch x := v.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		default:
			b, e = json.MarshalIndent(v, "", "  ")
		}
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(dir, name)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	save("source.nanz", f.source)
	save("workload.json", f.workload)
	original, e := f.factory()
	if e != nil {
		t.Fatal(e)
	}
	raw, e := hir.RunLoopProfile(original, f.workload)
	if e != nil {
		t.Fatal(e)
	}
	for i, r := range raw.Results {
		if r[0].I != f.results[i] || !reflect.DeepEqual(raw.Snapshots[i], f.snapshots[i]) {
			t.Fatal("independent raw intern oracle", i)
		}
	}
	save("raw.json", raw)
	var plain, speed int64
	for _, mode := range []string{"default", "grace-size", "grace-speed"} {
		h, e := f.factory()
		if e != nil {
			t.Fatal(e)
		}
		stats := mir2.NewGraceStats()
		// This test isolates the invariant-address-hoist Grace rule, so every
		// mode runs the same unsearched pipeline; under --Osize the variant
		// search would also move the compared baseline.
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: mode != "default", OptSize: mode == "grace-size", AssertMode: "none", GraceStats: stats, DisableVariantSearch: true})
		if e != nil {
			t.Fatal(e)
		}
		run, e := RunZ80ProfileWorkload(steps, original, f.workload, *raw)
		if e != nil {
			t.Fatal(mode, e)
		}
		for i, inv := range run.Invocations {
			if inv.Results[0].I != f.results[i] || !reflect.DeepEqual(inv.Snapshot, f.snapshots[i]) {
				t.Fatal("independent native intern oracle", mode, i)
			}
			for name, a := range inv.Access.Resources {
				if !reflect.DeepEqual(a.Reads, f.reads[i][name]) || !reflect.DeepEqual(a.Writes, f.writes[i][name]) {
					t.Fatal("exact native intern byte traffic", mode, i, name)
				}
			}
		}
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e)
		}
		if mode == "grace-speed" && stats.ByRule["invariant-address-hoist"] == 0 {
			t.Fatal("new pass never fired")
		}
		if mode != "grace-speed" && stats.ByRule["invariant-address-hoist"] > 0 {
			t.Fatal("new pass affected default/size")
		}
		if mode == "grace-size" {
			plain = run.TotalTStates
		}
		if mode == "grace-speed" {
			speed = run.TotalTStates
		}
		save(mode+"/target.a80", steps.Assembly)
		save(mode+"/target.bin", bin.Binary)
		save(mode+"/final.mir2", steps.MIR2Module.Dump())
		save(mode+"/allocation.json", steps.Allocation)
		save(mode+"/native.json", run)
		save(mode+"/metrics.txt", fmt.Sprintf("%d T / %d B\n", run.TotalTStates, len(bin.Binary)))
		t.Logf("%s %d T / %d B hoists %d", mode, run.TotalTStates, len(bin.Binary), stats.ByRule["invariant-address-hoist"])
	}
	if speed >= plain {
		t.Fatalf("new speed pass not useful after other Grace rules: %d vs %d T", speed, plain)
	}
}

// The real region_eq source is reused unchanged; runtime addresses exercise
// the modulo-65536 boundary independently of resource placement in the program.
func TestInvariantAddressNativeWrap(t *testing.T) {
	f := buildTypedInternFixture(t)
	for _, size := range []bool{false, true} {
		for _, n := range []int64{0, 1, 3} {
			h, e := f.factory()
			if e != nil {
				t.Fatal(e)
			}
			want := int64(1)
			memory := map[uint16]byte{0x4000: 7, 0x4001: 8, 0x4002: 9, 0: 7, 1: 8, 2: 9}
			measureGraceZ80HIR(t, h, "region_eq", Options{ContractOpt: true, UseGrace: true, OptSize: size, AssertMode: "none"}, []int64{0x4000, 65535, 0, n}, memory, want)
		}
	}
}
