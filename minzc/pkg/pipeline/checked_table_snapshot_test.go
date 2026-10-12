package pipeline

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckedTablePinnedSnapshotNative(t *testing.T) {
	baseline, path := localTableFixture(t)
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	opts := Options{OptSize: true, AssertMode: "none", CheckedLocalTablePath: path, CheckedLocalTableSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	selected, e := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts)
	if e != nil {
		t.Fatal(e)
	}
	if selected.Traces["add5"].Backend != "checked-local-table" {
		t.Fatal("candidate not selected")
	}
	if _, e := CompileHIRWithOptions(graceMeasureParse(t, checkedTableSource), opts); e != nil {
		t.Fatal("alternate entry", e)
	}
	for x := 0; x < 256; x++ {
		executeLocalTableFunction(t, selected.Assembly, byte(x))
	}
	other := filepath.Join(t.TempDir(), "same-content.json")
	if e := os.WriteFile(other, data, 0644); e != nil {
		t.Fatal(e)
	}
	opts.CheckedLocalTablePath = other
	moved, e := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts)
	if e != nil || moved.Assembly != selected.Assembly {
		t.Fatal("relocated snapshot changed output", e)
	}
	if e := os.WriteFile(other, []byte("[]"), 0644); e != nil {
		t.Fatal(e)
	}
	_, e = CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts)
	if e == nil || !strings.Contains(e.Error(), "SHA256 mismatch") {
		t.Fatal("changed table was not rejected", e)
	}
	if _, e := CompileHIRWithOptions(graceMeasureParse(t, checkedTableSource), opts); e == nil {
		t.Fatal("alternate entry ignored mutation")
	}
	opts.CheckedLocalTableSHA256 = ""
	miss, e := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts)
	if e != nil || miss.Assembly != baseline.Assembly {
		t.Fatal("unpinned miss compatibility", e)
	}
	for _, pin := range []string{"bad", strings.Repeat("g", 64), strings.Repeat("0", 62)} {
		opts.CheckedLocalTableSHA256 = pin
		if _, e := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts); e == nil {
			t.Fatal("invalid pin", pin)
		}
	}
	opts.CheckedLocalTablePath = ""
	opts.CheckedLocalTableSHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	if _, e := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), opts); e == nil {
		t.Fatal("orphan pin")
	}
	if _, e := CompileHIRWithOptions(graceMeasureParse(t, checkedTableSource), opts); e == nil {
		t.Fatal("alternate entry orphan pin")
	}
	t.Log("pinned native256, relocation, content mutation, malformed/orphan pins and unpinned fallback PASS")
}
