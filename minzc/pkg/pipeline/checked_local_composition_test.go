package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/vir"
)

func TestModuleCompositionRejectsUnsupportedModes(t *testing.T) {
	var pool vir.RegAllocTable
	pool.Init()
	for _, opts := range []Options{{}, {OptSize: true, SkipZ80Emission: true}, {OptSize: true, Backend: "llvm"}, {OptSize: true, CheckedLocalTablePath: "used.json"}, {OptSize: true, CheckedLocalTableSHA256: "abc"}} {
		if out, _, err := ComposeCheckedLocalTables(graceMeasureParse(t, checkedTableSource), opts, &pool, 32); err == nil || out != nil {
			t.Fatal(opts, err)
		}
	}
	if out, _, err := ComposeCheckedLocalTables(nil, Options{OptSize: true}, &pool, 32); err == nil || out != nil {
		t.Fatal("nil module accepted")
	}
	if out, _, err := ComposeCheckedLocalTables(graceMeasureParse(t, checkedTableSource), Options{OptSize: true}, &pool, 0); err == nil || out != nil {
		t.Fatal("unbounded mode accepted")
	}
}
