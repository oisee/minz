package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Opt-in diagnostic only. Native payloads are retained before fixed historical
// references are asserted, so a legitimate drift cannot erase its evidence.
// This never changes production admission or the independent fixture oracle.
func exportMemoryHostReferenceBeforeAssert(t *testing.T, name string, target *MemoryHostTargetEvidence, objects map[string]any) {
	t.Helper()
	root := os.Getenv("MINZ_MEMORY_HOST_REF_REFRESH")
	if root == "" {
		return
	}
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(n string, b []byte) {
		if err := os.WriteFile(filepath.Join(dir, n), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	write("source.nanz", source)
	write("target.bin", target.Binary)
	write("target.a80", []byte(target.Assembly))
	write("final.mir2", []byte(target.FinalMIRDump))
	write("allocation.json", target.AllocationJSON)
	data, err := json.MarshalIndent(target, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write("target.json", data)
	for n, obj := range objects {
		data, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		write(n, data)
	}
}
