package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func measuredSumFixture(t *testing.T) (func() (*hir.Module, error), hir.ProfileWorkload) {
	t.Helper()
	source, err := os.ReadFile("../../../examples/nanz/01_sum_array.nanz")
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), "examples/nanz/01_sum_array.nanz") }
	n := int64(40)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "z80-pbqp-default-jrs/v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"input": make([]byte, 40)}, Calls: []hir.ProfileInvocation{{Function: "sum_array", Arguments: []hir.ProfileArgument{{Resource: "input"}, {Value: &n}}}}}
	return factory, w
}
func TestMeasureLoopCostModel(t *testing.T) {
	factory, w := measuredSumFixture(t)
	options := Options{AssertMode: "none", UseHIRGrace: true}
	factoryCalls := 0
	measuredFactory := func() (*hir.Module, error) { factoryCalls++; return factory() }
	model, evidence, err := MeasureLoopCostModel(measuredFactory, "sum_array", w, options)
	if err != nil {
		t.Fatal(err)
	}
	if factoryCalls != 3 {
		t.Fatalf("factory called %d times, want 3 independent inputs", factoryCalls)
	}
	// Current instruction costs were independently reconciled against the saved
	// ASM delta; every row must satisfy these costs, not just the fitted samples.
	for _, row := range evidence.Rows {
		wantOriginal := 41 + 54*row.N
		wantPointer := 241 + 37*row.N
		if row.N == 0 {
			wantPointer = 232
		}
		if row.Path == "equality" {
			wantPointer += 7
		}
		if row.Path == "fallback" || row.Path == "wrap" {
			wantPointer = 258 + 191*row.N
		}
		if row.Original != wantOriginal || row.Pointer != wantPointer {
			t.Fatalf("independent ASM cost disagrees: %+v want %d/%d", row, wantOriginal, wantPointer)
		}
		if len(row.OriginalExecutionSHA256) != 64 || len(row.PointerExecutionSHA256) != 64 {
			t.Fatal("execution binaries not bound to evidence")
		}
	}
	if model.OriginalEntry != 41 || model.OriginalIteration != 54 || model.FastEntry != 241 || model.FastIteration != 37 || model.FastZeroEntry != 232 || !model.FastZeroMeasured || model.EqualEntryExtra != 7 || model.FallbackEntry != 258 || model.FallbackIteration != 191 || !model.FallbackMeasured {
		t.Fatalf("unexpected measured coefficients: %+v", model)
	}
	if len(evidence.Rows) != 1022 || model.OriginalBytes != uint64(len(evidence.OriginalBinary)) || model.PointerBytes != uint64(len(evidence.PointerBinary)) {
		t.Fatal("incomplete evidence")
	}
	data, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if model.EvidenceID != fmt.Sprintf("sha256:%x", sha256.Sum256(data)) {
		t.Fatal("evidence digest mismatch")
	}
	if directory := os.Getenv("MINZ_PGO_ARTIFACT_DIR"); directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]any{"loop-cost-model.json": model, "loop-cost-evidence.json": evidence} {
			artifact, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, name), append(artifact, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	h, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	run, err := hir.RunLoopProfile(h, w)
	if err != nil {
		t.Fatal(err)
	}
	observations, err := json.Marshal(run.Profile)
	if err != nil {
		t.Fatal(err)
	}
	context, err := hir.BuildWorkloadProfileContext(h, w)
	if err != nil {
		t.Fatal(err)
	}
	if model.CompilerID != context.CompilerID || model.SemanticsID != context.SemanticsID || model.TargetID != context.TargetID || model.HIRSHA256 != run.Profile.Compatibility.HIRSHA256 {
		t.Fatal("model compatibility binding mismatch")
	}
	for _, field := range []string{"compiler", "semantics", "hir"} {
		stale := model
		switch field {
		case "compiler":
			stale.CompilerID += "stale"
		case "semantics":
			stale.SemanticsID += "stale"
		case "hir":
			stale.HIRSHA256 += "stale"
		}
		if _, err := hir.SelectLoopVariants(bytes.NewReader(observations), h, context, stale, false); err == nil {
			t.Fatalf("accepted stale %s binding", field)
		}
	}
	decisions, err := hir.SelectLoopVariants(bytes.NewReader(observations), h, context, model, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 || decisions[0].Variant != "pointer-walk" {
		t.Fatalf("model cannot select pointer variant: %+v", decisions)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none", UseHIRGrace: true, ProfileSelection: &ProfileSelection{Observations: observations, Workload: w, Model: model, AssemblerMode: "default"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps.ProfileDecisions) != 1 {
		t.Fatal("pipeline did not apply model")
	}
	changed := model
	changed.TargetID += "stale"
	if _, err := hir.SelectLoopVariants(bytes.NewReader(observations), h, context, changed, false); err == nil {
		t.Fatal("accepted stale model")
	}
	fresh, _ := factory()
	options.OptSize = true
	options.ProfileSelection = &ProfileSelection{Observations: observations, Workload: w, Model: model, AssemblerMode: "default"}
	if _, err := CompileHIRSteps(fresh, options); err == nil || !strings.Contains(err.Error(), "compilation configuration mismatch") {
		t.Fatalf("configuration mismatch not refused: %v", err)
	}
	rows := append([]LoopCostRow(nil), evidence.Rows...)
	rows[100].Pointer++
	if _, err := fitMeasuredLoopRows(rows); err == nil {
		t.Fatal("accepted nonlinear timing")
	}
	if _, err := fitMeasuredLoopRows(rows[:100]); err == nil {
		t.Fatal("accepted incomplete domain")
	}
	rows = append([]LoopCostRow(nil), evidence.Rows...)
	rows[100] = rows[101]
	if _, err := fitMeasuredLoopRows(rows); err == nil {
		t.Fatal("accepted duplicate domain row")
	}
}
func TestMeasureLoopCostModelRefusals(t *testing.T) {
	factory, w := measuredSumFixture(t)
	check := func(name string, factory func() (*hir.Module, error), function string, w hir.ProfileWorkload, options Options) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			m, e, err := MeasureLoopCostModel(factory, function, w, options)
			if err == nil || e != nil || m.EvidenceID != "" {
				t.Fatalf("refusal returned usable model: %+v %v %v", m, e, err)
			}
		})
	}
	check("nil factory", nil, "sum_array", w, Options{})
	check("factory error", func() (*hir.Module, error) { return nil, fmt.Errorf("factory failed") }, "sum_array", w, Options{})
	check("nil module", func() (*hir.Module, error) { return nil, nil }, "sum_array", w, Options{})
	check("backend", factory, "sum_array", w, Options{Backend: "ez80"})
	check("unknown backend", factory, "sum_array", w, Options{Backend: "bad"})
	check("skipped emission", factory, "sum_array", w, Options{SkipZ80Emission: true})
	check("profile already selected", factory, "sum_array", w, Options{ProfileSelection: &ProfileSelection{}})
	check("wrong function", factory, "main", w, Options{})
	shared, _ := factory()
	check("shared module", func() (*hir.Module, error) { return shared, nil }, "sum_array", w, Options{})
	check("shared AST", func() (*hir.Module, error) { h, _ := factory(); h.Funcs = shared.Funcs; return h, nil }, "sum_array", w, Options{})
	calls := 0
	check("different HIR", func() (*hir.Module, error) {
		h, _ := factory()
		calls++
		h.Name = fmt.Sprintf("module%d", calls)
		return h, nil
	}, "sum_array", w, Options{})
	check("non u8", func() (*hir.Module, error) {
		h, _ := factory()
		h.FuncByName("sum_array").Params[1].Ty = mir2.TyU16
		return h, nil
	}, "sum_array", w, Options{})
	check("different return", func() (*hir.Module, error) {
		h, _ := factory()
		h.FuncByName("sum_array").Body.Body[3].(*hir.ReturnStmt).Val = hir.U8(0)
		return h, nil
	}, "sum_array", w, Options{})
	check("sticky input", func() (*hir.Module, error) { h, _ := factory(); h.PointerWalkPolicy = map[string]bool{}; return h, nil }, "sum_array", w, Options{})
	check("source manifest missing", func() (*hir.Module, error) { h, _ := factory(); h.SourceUnitsComplete = false; return h, nil }, "sum_array", w, Options{})
	bad := w
	bad.TargetModel = ""
	check("missing target", factory, "sum_array", bad, Options{})
}
