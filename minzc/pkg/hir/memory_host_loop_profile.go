package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

const MemoryHostLoopProfileSemantics = "minz.memory-host-loop-events/v1"

// Counts include recursive work and the exact prefix of a failed call.
type MemoryHostLoopInvocation struct {
	Observation MemoryHostObservation `json:"observation"`
	Counts      []LoopCounts          `json:"counts"`
	Completed   bool                  `json:"completed"`
}

// MemoryHostLoopProfileRun is input-HIR observation, not target cost or legality
// evidence. Private event calls consume the declared VM instruction budget.
type MemoryHostLoopProfileRun struct {
	Schema         string                     `json:"schema"`
	Stage          string                     `json:"stage"`
	Workload       MemoryHostWorkload         `json:"workload"`
	Compatibility  *ProfileCompatibility      `json:"compatibility"`
	Context        ProfileContext             `json:"context"`
	Sites          *LoopSiteMap               `json:"sites"`
	Counts         []LoopCounts               `json:"counts"`
	CompletedCalls uint64                     `json:"completed_calls"`
	FailedCalls    uint64                     `json:"failed_calls"`
	Invocations    []MemoryHostLoopInvocation `json:"invocations"`
}

// RunMemoryHostLoopProfile constructs its own VM and closed hooks. No supplied
// callback, executable MIR, or claimed counts can enter this API.
func RunMemoryHostLoopProfile(source *Module, w MemoryHostWorkload) (*MemoryHostLoopProfileRun, error) {
	owned, ctx, err := PrepareMemoryHostWorkload(source, w)
	if err != nil {
		return nil, err
	}
	if _, err = BuildFunctionSiteMap(source); err != nil {
		return nil, err
	}
	sites, err := BuildLoopSiteMap(source)
	if err != nil {
		return nil, err
	}
	if sites.UnmappedLoops != 0 {
		return nil, fmt.Errorf("memory host loops: unmapped input loops")
	}
	ctx.SemanticsID += ":" + MemoryHostLoopProfileSemantics
	before, err := BuildProfileCompatibility(source, ctx)
	if err != nil {
		return nil, err
	}
	p, err := newLoopProfiler(source, false)
	if err != nil {
		return nil, err
	}
	m := p.VM.Module
	if err = validateMemoryHostLoopInstrumentation(source, m, owned, p.profileHooks); err != nil {
		return nil, err
	}
	if err = mir2.Verify(m); err != nil {
		return nil, err
	}
	out := &MemoryHostLoopProfileRun{Schema: "minz.memory-host-loop-observations/v1", Stage: "input-hir-instrumented-mir2-vm", Workload: owned, Sites: sites, Counts: append([]LoopCounts{}, p.counts...), Invocations: []MemoryHostLoopInvocation{}}
	previous := append([]LoopCounts{}, p.counts...)
	observe := func(o MemoryHostObservation) {
		delta := make([]LoopCounts, len(p.counts))
		for i, c := range p.counts {
			delta[i] = LoopCounts{SiteID: c.SiteID, Entries: c.Entries - previous[i].Entries, BodyEntries: c.BodyEntries - previous[i].BodyEntries}
		}
		previous = append(previous[:0], p.counts...)
		completed := o.Error == ""
		if completed {
			out.CompletedCalls++
		} else {
			out.FailedCalls++
		}
		out.Invocations = append(out.Invocations, MemoryHostLoopInvocation{Observation: o, Counts: delta, Completed: completed})
	}
	run, runErr := runPreparedMemoryHost(source, m, owned, ctx, p.VM, p.profileHooks, observe)
	out.Counts = append([]LoopCounts{}, p.counts...)
	if run == nil {
		return nil, runErr
	}
	out.Compatibility, out.Context = run.Compatibility, run.Context
	after, err := BuildProfileCompatibility(source, ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		return nil, fmt.Errorf("memory host loops: instrumentation changed input identity")
	}
	return out, runErr
}

// Validation view removes only exact private zero-argument void events, then
// uses the unchanged baseline guard. No namespace-prefix allowance exists.
func validateMemoryHostLoopInstrumentation(source *Module, m *mir2.Module, w MemoryHostWorkload, hooks map[string]mir2.HostFunc) error {
	view := *m
	view.Funcs = nil
	seen := map[string]bool{}
	for _, f := range m.Funcs {
		if _, ok := hooks[f.Name]; ok {
			if source.FuncByName(f.Name) != nil || !f.Attrs.IsExtern || f.Attrs.HasAsm || len(f.Blocks) != 0 || len(f.Contract.Params) != 0 || len(f.Contract.Returns) != 0 || f.Attrs.ExternAddr != 0 || f.Contract.ExternClobbers != nil {
				return fmt.Errorf("memory host loops: invalid private hook declaration")
			}
			seen[f.Name] = true
			continue
		}
		ff := *f
		ff.Blocks = nil
		for _, b := range f.Blocks {
			bb := *b
			bb.Insts = nil
			for _, i := range b.Insts {
				if _, ok := hooks[i.Sym]; ok {
					if i.Op != mir2.OpCall || len(i.Args) != 0 || i.Src != [2]mir2.Reg{} || i.Dst != mir2.NoReg || i.Ty != mir2.TyVoid {
						return fmt.Errorf("memory host loops: invalid private event instruction")
					}
					continue
				}
				bb.Insts = append(bb.Insts, i)
			}
			ff.Blocks = append(ff.Blocks, &bb)
		}
		view.Funcs = append(view.Funcs, &ff)
	}
	if len(seen) != len(hooks) {
		return fmt.Errorf("memory host loops: missing private hook")
	}
	return ValidateMemoryHostMIR(source, &view, w)
}
