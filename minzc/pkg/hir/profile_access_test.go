package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestProfileAccessRecorderRejectsUndeclaredAndOwnsCalls(t *testing.T) {
	m := &Module{}
	vm := mir2.NewVM(&mir2.Module{})
	w := ProfileWorkload{Resources: map[string][]byte{"r": {1, 2}}}
	r := NewProfileAccessRecorder(m, vm, w, map[string]int64{"r": 100})
	r.Observe(mir2.VMDataAccess{Op: mir2.OpLoad, Addr: 100, Value: 1})
	first, err := r.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	r.Reset()
	r.Observe(mir2.VMDataAccess{Op: mir2.OpStore, Addr: 101, Value: 3})
	second, err := r.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if first.Resources["r"].Reads[0] != 1 || first.Resources["r"].Writes[1] != 0 || second.Resources["r"].Reads[0] != 0 || second.Resources["r"].Writes[1] != 1 {
		t.Fatal(first, second)
	}
	r.Observe(mir2.VMDataAccess{Op: mir2.OpLoad, Addr: 102})
	if _, err = r.Snapshot(); err == nil {
		t.Fatal("undeclared access accepted")
	}
}
