package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"testing"
)

func TestMemoryHostPreparedCompatibilityExactAndOwned(t *testing.T) {
	m, w := memoryFixture(t)
	owned, ctx, key, e := hir.PrepareMemoryHostWorkloadCompatibility(m, w)
	if e != nil {
		t.Fatal(e)
	}
	oldOwned, oldCtx, e := hir.PrepareMemoryHostWorkload(m, w)
	if e != nil {
		t.Fatal(e)
	}
	oldKey, e := hir.BuildProfileCompatibility(m, oldCtx)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(owned, oldOwned) || !reflect.DeepEqual(ctx, oldCtx) || !reflect.DeepEqual(key, oldKey) {
		t.Fatal("combined key changed admission identity")
	}
	// Returned key/context and caller source/workload own separate dependencies.
	ctx.Dependencies["new"] = "changed"
	if _, ok := key.Context.Dependencies["new"]; ok {
		t.Fatal("key aliases context")
	}
	owned.Regions["ram"].Bytes[0] = 44
	if w.Regions["ram"].Bytes[0] != 9 {
		t.Fatal("workload alias")
	}
	for name, mutate := range map[string]func(*hir.Module, *hir.MemoryHostWorkload){
		"abi":      func(m *hir.Module, w *hir.MemoryHostWorkload) { m.FuncByName("peek").Params[0].Ty = mir2.TyU8 },
		"manifest": func(m *hir.Module, w *hir.MemoryHostWorkload) { m.SourceUnitsComplete = false },
		"origin": func(m *hir.Module, w *hir.MemoryHostWorkload) {
			for node, o := range m.SourceLocations {
				o.SourceSHA256 = "bad"
				m.SourceLocations[node] = o
				break
			}
		},
		"host":   func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Hosts["peek"] = "unknown" },
		"target": func(m *hir.Module, w *hir.MemoryHostWorkload) { w.TargetModel = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			m, w := memoryFixture(t)
			mutate(m, &w)
			_, _, oldErr := hir.PrepareMemoryHostWorkload(m, w)
			_, _, key, newErr := hir.PrepareMemoryHostWorkloadCompatibility(m, w)
			if oldErr == nil || newErr == nil || oldErr.Error() != newErr.Error() || key != nil {
				t.Fatal("different refusal", oldErr, newErr)
			}
		})
	}
	// No cache: a changed semantic body is detected by a fresh same-boundary key.
	m, w = memoryFixture(t)
	_, _, before, e := hir.PrepareMemoryHostWorkloadCompatibility(m, w)
	if e != nil {
		t.Fatal(e)
	}
	m.Globals[0].Init = []byte{17}
	_, _, after, e := hir.PrepareMemoryHostWorkloadCompatibility(m, w)
	if e != nil || reflect.DeepEqual(before, after) {
		t.Fatal("cached semantic fingerprint", e)
	}
}
