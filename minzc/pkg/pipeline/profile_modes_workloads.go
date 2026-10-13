package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
	"reflect"
)

// MeasuredModeWorkloadsEvidence preserves each independently bound initial RAM
// state and its complete four-candidate evidence. Costs are summed only for
// byte-identical executables; size measures one executable, not repeated copies.
type MeasuredModeWorkloadsEvidence struct {
	Schema       string                  `json:"schema"`
	Goal         string                  `json:"goal"`
	Runs         []*MeasuredModeEvidence `json:"runs"`
	TotalTStates []int64                 `json:"total_t_states"`
	Selected     int                     `json:"selected"`
	SHA256       string                  `json:"sha256,omitempty"`
}
type MeasuredModeWorkloadsResult struct {
	Steps    Steps
	Binary   []byte
	Evidence *MeasuredModeWorkloadsEvidence
}

// MeasureCompilerModeWorkloads selects a single production compiler mode over
// several controlled workloads with distinct initial state. It reuses the same
// closed, fail-closed single-workload API and refuses executable/context drift.
// No heldout workload belongs here: replay the returned exact executable later.
func MeasureCompilerModeWorkloads(factory func() (*hir.Module, error), workloads []hir.ProfileWorkload, goal string, optSize bool) (*MeasuredModeWorkloadsResult, error) {
	fail := func(e error) (*MeasuredModeWorkloadsResult, error) {
		return nil, fmt.Errorf("measured compiler-mode workloads: %w", e)
	}
	if factory == nil || len(workloads) == 0 || (goal != "speed" && goal != "size") {
		return fail(fmt.Errorf("invalid factory/workloads/goal"))
	}
	data, e := json.Marshal(workloads)
	if e != nil {
		return fail(e)
	}
	var owned []hir.ProfileWorkload
	if e = json.Unmarshal(data, &owned); e != nil {
		return fail(e)
	}
	// Retain earlier factories and their finalized semantic snapshots across
	// later searches. A later factory must not mutate previously compiled HIR.
	type retainedHIR struct {
		module *hir.Module
		dump   string
		key    []byte
	}
	retained := []retainedHIR{}
	snapshot := func(m *hir.Module) ([]byte, error) {
		return json.Marshal(struct {
			Module   *hir.Module
			Units    map[string]string
			Complete bool
		}{m, m.SourceUnits, m.SourceUnitsComplete})
	}
	checkRetained := func() error {
		for _, old := range retained {
			key, e := snapshot(old.module)
			if e != nil || !bytes.Equal(key, old.key) || old.module.Dump() != old.dump {
				return fmt.Errorf("earlier factory HIR mutated between workloads")
			}
		}
		return nil
	}
	batch := []*hir.Module{}
	originalFactory := factory
	factory = func() (*hir.Module, error) {
		m, e := originalFactory()
		if e != nil {
			return nil, e
		}
		if e = checkRetained(); e != nil {
			return nil, e
		}
		batch = append(batch, m)
		return m, nil
	}
	evidence := &MeasuredModeWorkloadsEvidence{Schema: "minz.measured-compiler-mode-workloads/v1", Goal: goal, TotalTStates: make([]int64, 4)}
	var first *MeasuredModeResult
	var savedHashes []string
	for n, w := range owned {
		result, e := MeasureCompilerModeVariants(factory, w, goal, optSize)
		if e != nil {
			return fail(fmt.Errorf("workload %d: %w", n, e))
		}
		if e := checkRetained(); e != nil {
			return fail(e)
		}
		for _, m := range batch {
			key, e := snapshot(m)
			if e != nil {
				return fail(e)
			}
			retained = append(retained, retainedHIR{m, m.Dump(), key})
		}
		batch = nil
		if result.Evidence == nil || len(result.Evidence.Candidates) != 4 {
			return fail(fmt.Errorf("incomplete candidates"))
		}
		if first == nil {
			first = result
		} else {
			// Workload hashes legitimately differ. Entire executable/HIR/ABI/source and
			// compiler context must stay equal across the independent runs.
			a, b := first.Evidence.Compatibility, result.Evidence.Compatibility
			aa, bb := *a, *b
			aa.Context.WorkloadSHA256 = bb.Context.WorkloadSHA256
			aa.Context.Dependencies = map[string]string{}
			bb.Context.Dependencies = map[string]string{}
			for k, v := range a.Context.Dependencies {
				if _, resource := owned[0].Resources[k]; !resource {
					aa.Context.Dependencies[k] = v
				}
			}
			for k, v := range b.Context.Dependencies {
				if _, resource := w.Resources[k]; !resource {
					bb.Context.Dependencies[k] = v
				}
			}
			if !reflect.DeepEqual(aa, bb) || !reflect.DeepEqual(first.Evidence.FunctionSites, result.Evidence.FunctionSites) {
				return fail(fmt.Errorf("source/context changed between workloads"))
			}
		}
		for i, c := range result.Evidence.Candidates {
			base := first.Evidence.Candidates[i]
			if !reflect.DeepEqual(base.Options, c.Options) || base.CompilationID != c.CompilationID || base.Assembly != c.Assembly || base.FinalMIR != c.FinalMIR || base.FullMIRSHA256 == "" || base.FullMIRSHA256 != c.FullMIRSHA256 || base.AllocationSHA256 == "" || base.AllocationSHA256 != c.AllocationSHA256 || !bytes.Equal(base.Binary, c.Binary) {
				return fail(fmt.Errorf("candidate %d executable changed between workloads", i))
			}
			if c.Run == nil || c.Run.TotalTStates < 0 || c.Run.TotalTStates > int64(^uint64(0)>>1)-evidence.TotalTStates[i] {
				return fail(fmt.Errorf("candidate %d invalid aggregate cost", i))
			}
			evidence.TotalTStates[i] += c.Run.TotalTStates
		}
		evidence.Runs = append(evidence.Runs, result.Evidence)
		payload, err := json.Marshal(result.Evidence)
		if err != nil {
			return fail(err)
		}
		savedHashes = append(savedHashes, fmt.Sprintf("%x", sha256.Sum256(payload)))
		for j, old := range evidence.Runs {
			payload, err := json.Marshal(old)
			if err != nil || fmt.Sprintf("%x", sha256.Sum256(payload)) != savedHashes[j] {
				return fail(fmt.Errorf("retained candidate evidence mutated"))
			}
		}
	}
	for i := 1; i < 4; i++ {
		better := evidence.TotalTStates[i] < evidence.TotalTStates[evidence.Selected]
		if goal == "size" {
			better = len(first.Evidence.Candidates[i].Binary) < len(first.Evidence.Candidates[evidence.Selected].Binary)
		}
		if better {
			evidence.Selected = i
		}
	}
	// Build the selected artifact independently, checking the full HIR identity
	// against every saved context and the exact finalized executable.
	m, e := factory()
	if e != nil {
		return fail(e)
	}
	for n, w := range owned {
		ctx, e := hir.BuildWorkloadProfileContext(m, w)
		if e != nil {
			return fail(e)
		}
		key, e := hir.BuildProfileCompatibility(m, ctx)
		if e != nil {
			return fail(e)
		}
		if !reflect.DeepEqual(key, evidence.Runs[n].Compatibility) {
			return fail(fmt.Errorf("final source/context changed"))
		}
	}
	best := first.Evidence.Candidates[evidence.Selected]
	steps, e := CompileHIRSteps(m, best.Options)
	if e != nil {
		return fail(e)
	}
	program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil {
		return fail(e)
	}
	if len(program.Errors) != 0 || steps.Assembly != best.Assembly || steps.MIR2Module.Dump() != best.FinalMIR || !bytes.Equal(program.Binary, best.Binary) {
		return fail(fmt.Errorf("final selected executable changed"))
	}
	finalMIRBytes, e := json.Marshal(steps.MIR2Module)
	if e != nil {
		return fail(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(finalMIRBytes)) != best.FullMIRSHA256 {
		return fail(fmt.Errorf("final selected MIR metadata differs from measured candidate"))
	}
	allocationBytes, e := json.Marshal(steps.Allocation)
	if e != nil {
		return fail(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(allocationBytes)) != best.AllocationSHA256 {
		return fail(fmt.Errorf("final selected allocation differs from measured candidate"))
	}
	for n, w := range owned {
		original, e := factory()
		if e != nil {
			return fail(e)
		}
		ctx, e := hir.BuildWorkloadProfileContext(original, w)
		if e != nil {
			return fail(e)
		}
		key, e := hir.BuildProfileCompatibility(original, ctx)
		if e != nil || !reflect.DeepEqual(key, evidence.Runs[n].Compatibility) {
			return fail(fmt.Errorf("replay source/context changed"))
		}
		run, e := RunZ80ProfileWorkload(steps, original, w, *evidence.Runs[n].Oracle)
		if e != nil {
			return fail(e)
		}
		if !jointEqual(run, evidence.Runs[n].Candidates[evidence.Selected].Run) {
			return fail(fmt.Errorf("selected training replay changed"))
		}
	}
	for j, old := range evidence.Runs {
		payload, err := json.Marshal(old)
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(payload)) != savedHashes[j] {
			return fail(fmt.Errorf("retained candidate evidence mutated"))
		}
	}
	nowMIR, me := json.Marshal(steps.MIR2Module)
	nowAllocation, ae := json.Marshal(steps.Allocation)
	if me != nil || ae != nil || !bytes.Equal(nowMIR, finalMIRBytes) || !bytes.Equal(nowAllocation, allocationBytes) || steps.MIR2Module.Dump() != best.FinalMIR || steps.Assembly != best.Assembly {
		return fail(fmt.Errorf("returned artifact mutated"))
	}
	data, e = json.Marshal(evidence)
	if e != nil {
		return fail(e)
	}
	evidence.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	return &MeasuredModeWorkloadsResult{Steps: steps, Binary: append([]byte(nil), best.Binary...), Evidence: evidence}, nil
}
