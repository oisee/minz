package pipeline

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestMeasuredRuntimeRejectedBeforeFactory(t *testing.T) {
	calls := 0
	factory := func() (*hir.Module, error) { calls++; t.Fatal("runtime reached source factory"); return nil, nil }
	options := Options{Runtime: mir2.Z80RuntimeCPM}
	_, _, err := MeasureLoopCostModel(factory, "loop", hir.ProfileWorkload{}, options)
	if err == nil || !strings.Contains(err.Error(), "application runtimes") {
		t.Fatal(err)
	}
	_, err = MeasureGraceVariantSets(factory, nil, hir.ProfileWorkload{}, options, 1)
	if err == nil || !strings.Contains(err.Error(), "application runtimes") || calls != 0 {
		t.Fatal("measured runtime admitted", err, calls)
	}
}

func TestDefaultRuntimeKeepsProfileIdentity(t *testing.T) {
	b, err := json.Marshal(DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "Runtime") {
		t.Fatal("empty runtime changed old option identity", string(b))
	}
	id, err := ProfileCompilationID(DefaultOptions(), "default")
	if err != nil || id == "" {
		t.Fatal(err)
	}
}
