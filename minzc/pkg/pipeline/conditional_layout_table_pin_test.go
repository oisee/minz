package pipeline

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

// A late factory can mutate configured input files after all candidates have
// executed. Even if the measured executable is intact, its configured content
// pin must remain valid before returning a bound selection.
func TestConditionalLayoutWorkloadRechecksTablePin(t *testing.T) {
	_, path := localTableFixture(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fresh, w := conditionalWorkloadFixture()
	options := Options{OptSize: true, AssertMode: "none", CheckedLocalTablePath: path, CheckedLocalTableSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	good, err := MeasureConditionalLayoutWorkload(fresh, "classify", w, options, "speed", 4)
	if err != nil || good == nil {
		t.Fatal("valid pinned input refused", err)
	}
	calls := 0
	factory := func() (*hir.Module, error) {
		calls++
		if calls == 3 {
			if err := os.WriteFile(path, []byte("[]"), 0644); err != nil {
				return nil, err
			}
		}
		return fresh()
	}
	result, err := MeasureConditionalLayoutWorkload(factory, "classify", w, options, "speed", 4)
	if calls != 3 {
		t.Fatal("did not reach post-measurement factory", calls, err)
	}
	if err == nil || result != nil {
		t.Fatal("late configured table mutation yielded a winner")
	}
}
