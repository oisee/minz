package hir

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/minz/minzc/pkg/mir2"
)

// ProfileArgument is a literal value or an offset into a named input resource.
// Resource addresses are resolved after VM construction, never guessed by callers.
type ProfileArgument struct {
	Value    *int64 `json:"value,omitempty"`
	Resource string `json:"resource,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

type ProfileInvocation struct {
	Function  string            `json:"function"`
	Arguments []ProfileArgument `json:"arguments"`
}

// ProfileWorkload describes one fresh VM and ordered calls sharing its memory.
// MaxSteps is cumulative, including instrumentation; MaxMemory includes initial
// globals/function symbols and input resources. Custom hosts/ports/ASM are absent.
type ProfileWorkload struct {
	ExactDataAccess bool                `json:"exact_data_access,omitempty"`
	Schema          string              `json:"schema"`
	TargetModel     string              `json:"target_model"`
	MaxSteps        int64               `json:"max_steps"`
	MaxMemory       int                 `json:"max_memory"`
	Resources       map[string][]byte   `json:"resources"`
	Globals         map[string][]byte   `json:"globals"`
	Calls           []ProfileInvocation `json:"calls"`
}

// ProfileMemorySnapshot owns declared memory at one successful invocation return.
// Snapshots are ordered like Results, including calls returning void. Empty maps
// are serialized explicitly so missing boundary observations cannot pass as empty.
type ProfileMemorySnapshot struct {
	Globals   map[string][]byte `json:"globals"`
	Resources map[string][]byte `json:"resources"`
}

type ProfileRun struct {
	Accesses  []ProfileAccessSnapshot `json:"accesses,omitempty"`
	Profile   LoopProfile             `json:"profile"`
	Results   [][]mir2.Value          `json:"results"`
	Snapshots []ProfileMemorySnapshot `json:"snapshots"`
	Globals   map[string][]byte       `json:"globals"`
	Resources map[string][]byte       `json:"resources"`
}

// prepareProfileWorkload owns a JSON copy: the bytes hashed are the bytes run.
func prepareProfileWorkload(m *Module, w ProfileWorkload) (ProfileWorkload, ProfileContext, error) {
	var owned ProfileWorkload
	if err := RejectValueOnlyErrorProfiles(m); err != nil {
		return owned, ProfileContext{}, err
	}
	data, err := json.Marshal(w)
	if err != nil {
		return owned, ProfileContext{}, err
	}
	if err = json.Unmarshal(data, &owned); err != nil {
		return owned, ProfileContext{}, err
	}
	if owned.Schema != "minz.profile-workload/v1" || owned.MaxSteps <= 0 || owned.MaxMemory <= 0 || owned.MaxMemory > 65536 || len(owned.Calls) == 0 {
		return owned, ProfileContext{}, fmt.Errorf("loop profile: invalid workload schema/limits/calls")
	}
	context, err := BuildAutomaticProfileContext(m, ProfileInputs{TargetModel: owned.TargetModel, Workload: data, Hosts: []byte("minz.controlled-profile/no-hosts-no-ports-no-asm/v1"), Resources: owned.Resources})
	if err != nil {
		return owned, context, err
	}
	globals := map[string]int{}
	for _, g := range m.Globals {
		size := mir2.ByteWidth(g.Ty)
		if len(g.Init) > size {
			size = len(g.Init)
		}
		globals[g.Name] = size
	}
	for name, bytes := range owned.Globals {
		size, ok := globals[name]
		if !ok || len(bytes) != size {
			return owned, context, fmt.Errorf("loop profile: global override %q must match declared storage", name)
		}
	}
	total := 0
	for _, bytes := range owned.Resources {
		if len(bytes) > owned.MaxMemory-total {
			return owned, context, fmt.Errorf("loop profile: resources exceed memory limit")
		}
		total += len(bytes)
	}
	for _, call := range owned.Calls {
		fn := m.FuncByName(call.Function)
		if fn == nil || fn.IsExtern || len(fn.Captures) > 0 || len(call.Arguments) != len(fn.Params) {
			return owned, context, fmt.Errorf("loop profile: invalid invocation %q", call.Function)
		}
		for i, arg := range call.Arguments {
			if fn.Params[i].SMC {
				return owned, context, fmt.Errorf("loop profile: SMC invocation unsupported")
			}
			if arg.Value != nil {
				if ranged, ok := fn.Params[i].Ty.(*mir2.RangedTy); ok {
					if *arg.Value < ranged.Lo || *arg.Value >= ranged.Hi {
						return owned, context, fmt.Errorf("loop profile: argument outside declared range")
					}
				}
				if arg.Resource != "" || arg.Offset != 0 {
					return owned, context, fmt.Errorf("loop profile: ambiguous argument")
				}
				continue
			}
			resource, ok := owned.Resources[arg.Resource]
			if !ok || arg.Resource == "" || arg.Offset < 0 || arg.Offset > len(resource) || !mir2.IsPointer(fn.Params[i].Ty) {
				return owned, context, fmt.Errorf("loop profile: invalid resource pointer")
			}
		}
	}
	if err := ValidateProfileWorkloadMIR(m, LowerModule(m), owned); err != nil {
		return owned, context, err
	}
	return owned, context, nil
}

// BuildWorkloadProfileContext recomputes the same context without running calls.
func BuildWorkloadProfileContext(m *Module, w ProfileWorkload) (ProfileContext, error) {
	_, context, err := prepareProfileWorkload(m, w)
	return context, err
}

// RunLoopProfile controls VM construction, initial state and calls. No VM or host
// callbacks escape to the caller. A runtime error returns a partial observation
// marked failed, which LoadLoopProfile refuses. Invalid setup returns no result.
func RunLoopProfile(m *Module, w ProfileWorkload) (*ProfileRun, error) {
	owned, context, err := prepareProfileWorkload(m, w)
	if err != nil {
		return nil, err
	}
	p, err := NewLoopProfilerWithContext(m, context)
	if err != nil {
		return nil, err
	}
	// Builtin hosts can touch external I/O or environment. Keep only collector hooks.
	if err := validateProfileWorkloadInstrumentation(m, p, owned); err != nil {
		return nil, err
	}
	p.VM.Hosts = map[string]mir2.HostFunc{}
	for name, hook := range p.profileHooks {
		p.VM.Hosts[name] = hook
	}
	installProfileDeniedExternHooks(p.VM, m)
	p.VM.MaxSteps = owned.MaxSteps
	p.VM.MaxMemory = owned.MaxMemory
	reachable, err := ProfileWorkloadReachableFunctions(p.VM.Module, owned)
	if err != nil {
		return nil, err
	}
	for _, fn := range p.VM.Module.Funcs {
		deadRuntime := !reachable[fn.Name] && ProfileDeadPascalRuntime(m, fn)
		for _, b := range fn.Blocks {
			for _, i := range b.Insts {
				switch i.Op {
				case mir2.OpAsm:
					if !deadRuntime {
						return nil, fmt.Errorf("loop profile: noncanonical/reachable ASM unsupported")
					}
					continue
				case mir2.OpIn8, mir2.OpIn16, mir2.OpOut8, mir2.OpOut16:
					return nil, fmt.Errorf("loop profile: ASM/port I/O unsupported by controlled harness")
				}
			}
		}
	}
	used := p.VM.AllocHeap(nil).I
	if used > int64(owned.MaxMemory) {
		return nil, fmt.Errorf("loop profile: initial VM storage exceeds memory limit")
	}
	for name, bytes := range owned.Globals {
		p.VM.WriteHeapBytes(p.VM.GlobalAddr(name), bytes)
	}
	names := []string{}
	for name := range owned.Resources {
		names = append(names, name)
	}
	sort.Strings(names)
	addresses := map[string]int64{}
	for _, name := range names {
		bytes := owned.Resources[name]
		if int64(len(bytes)) > int64(owned.MaxMemory)-used {
			return nil, fmt.Errorf("loop profile: VM and resources exceed memory limit")
		}
		addresses[name] = p.VM.AllocHeap(bytes).I
		used += int64(len(bytes))
	}
	out := &ProfileRun{Results: [][]mir2.Value{}, Snapshots: []ProfileMemorySnapshot{}}
	var accesses *ProfileAccessRecorder
	var traffic *profileTrafficModel
	if owned.ExactDataAccess {
		accesses = NewProfileAccessRecorder(m, p.VM, owned, addresses)
		p.VM.ObserveDataAccess = accesses.Observe
		if traffic, err = newProfileTrafficModel(m, owned); err != nil {
			return nil, err
		}
	}

	snapshotMemory := func() ProfileMemorySnapshot {
		snapshot := ProfileMemorySnapshot{Globals: map[string][]byte{}, Resources: map[string][]byte{}}
		for _, g := range m.Globals {
			size := mir2.ByteWidth(g.Ty)
			if len(g.Init) > size {
				size = len(g.Init)
			}
			snapshot.Globals[g.Name] = p.VM.ReadHeap(p.VM.GlobalAddr(g.Name), size)
		}
		for _, name := range names {
			snapshot.Resources[name] = p.VM.ReadHeap(addresses[name], len(owned.Resources[name]))
		}
		return snapshot
	}
	finish := func() {
		out.Profile = p.Snapshot()
		final := snapshotMemory()
		out.Globals, out.Resources = final.Globals, final.Resources
	}
	for _, call := range owned.Calls {
		args := make([]mir2.Value, len(call.Arguments))
		for i, arg := range call.Arguments {
			if arg.Value != nil {
				args[i].I = *arg.Value
			} else {
				args[i].I = addresses[arg.Resource] + int64(arg.Offset)
			}
		}
		if accesses != nil {
			accesses.Reset()
		}
		result, err := p.Call(call.Function, args)
		if err != nil {
			finish()
			return out, fmt.Errorf("loop profile: invocation %d @%s: %w", len(out.Results), call.Function, err)
		}
		out.Results = append(out.Results, result)
		out.Snapshots = append(out.Snapshots, snapshotMemory())
		if accesses != nil {
			a, err := accesses.Snapshot()
			if err != nil {
				// No successful profile envelope may escape after access
				// validation fails, even though the VM call itself completed.
				return nil, err
			}
			if traffic != nil {
				// The exact traffic model must describe the binary the workload
				// is measured against, so it is replayed on the lowering that
				// carries the emitted memory idioms (fill_bytes16 reloads its
				// destination) rather than on the instrumented scalar loops.
				modelled, err := traffic.replay(m, call, result, out.Snapshots[len(out.Snapshots)-1])
				if err != nil {
					return nil, err
				}
				a = modelled
			}
			out.Accesses = append(out.Accesses, a)
		}
	}
	finish()
	return out, nil
}
