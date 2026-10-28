package hir

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/minz/minzc/pkg/mir2"
)

// LowerModuleForTraffic lowers m for exact byte-traffic modelling and applies
// the memory-idiom recognition the production pipeline emits.
//
// An exact-access oracle must model the traffic of the binary that is actually
// measured, not only of the scalar source loops. fill_bytes16 is the one
// admitted idiom whose native lowering is not traffic-neutral: its overlapping
// LDIR reloads the n-1 destination bytes it has just stored (see
// mir2/byte_loop.go). Retiring the same proven loops here lets the mir2 VM
// observe those reads, so expected and observed access counts agree.
//
// Scalar results and final memory are unchanged: recognition only replaces
// proven single-body byte loops with intrinsics of identical semantics. The
// loop-counting instrumentation cannot be used here — its counter hooks are
// calls inside the loop body, which the recognition's plain-body rule refuses —
// which is why profileTrafficModel replays an already-counted workload.
func LowerModuleForTraffic(m *Module) *mir2.Module {
	lowered := LowerModule(m)
	for _, f := range lowered.Funcs {
		mir2.RecognizeByteLoops(f)
	}
	return lowered
}

// profileTrafficModel replays a counted workload on an uninstrumented lowering
// that carries the emitted memory idioms, recording the exact byte traffic the
// compiled binary is expected to perform. Every result and memory snapshot must
// equal the instrumented run's; only the traffic model is allowed to differ.
type profileTrafficModel struct {
	vm        *mir2.VM
	recorder  *ProfileAccessRecorder
	names     []string
	addresses map[string]int64
	resources map[string][]byte
}

func newProfileTrafficModel(m *Module, owned ProfileWorkload) (*profileTrafficModel, error) {
	t := &profileTrafficModel{names: []string{}, addresses: map[string]int64{}, resources: owned.Resources}
	t.vm = mir2.NewVM(LowerModuleForTraffic(m))
	t.vm.MaxSteps = owned.MaxSteps
	t.vm.MaxMemory = owned.MaxMemory
	installProfileDeniedExternHooks(t.vm, m)
	if used := t.vm.AllocHeap(nil).I; used > int64(owned.MaxMemory) {
		return nil, fmt.Errorf("loop profile: traffic model storage exceeds memory limit")
	}
	for name, value := range owned.Globals {
		t.vm.WriteHeapBytes(t.vm.GlobalAddr(name), value)
	}
	for name := range owned.Resources {
		t.names = append(t.names, name)
	}
	sort.Strings(t.names)
	for _, name := range t.names {
		t.addresses[name] = t.vm.AllocHeap(owned.Resources[name]).I
	}
	t.recorder = NewProfileAccessRecorder(m, t.vm, owned, t.addresses)
	t.vm.ObserveDataAccess = t.recorder.Observe
	return t, nil
}

func (t *profileTrafficModel) arguments(call ProfileInvocation) []mir2.Value {
	args := make([]mir2.Value, len(call.Arguments))
	for i, arg := range call.Arguments {
		if arg.Value != nil {
			args[i].I = *arg.Value
			continue
		}
		args[i].I = t.addresses[arg.Resource] + int64(arg.Offset)
	}
	return args
}

func (t *profileTrafficModel) snapshot(m *Module) ProfileMemorySnapshot {
	snapshot := ProfileMemorySnapshot{Globals: map[string][]byte{}, Resources: map[string][]byte{}}
	for _, g := range m.Globals {
		size := mir2.ByteWidth(g.Ty)
		if len(g.Init) > size {
			size = len(g.Init)
		}
		snapshot.Globals[g.Name] = t.vm.ReadHeap(t.vm.GlobalAddr(g.Name), size)
	}
	for _, name := range t.names {
		snapshot.Resources[name] = t.vm.ReadHeap(t.addresses[name], len(t.resources[name]))
	}
	return snapshot
}

// replay executes one already-counted invocation and returns its traffic. A
// divergence in results or memory from the instrumented run is a compiler bug,
// never a modelling difference, and is refused here.
func (t *profileTrafficModel) replay(m *Module, call ProfileInvocation, result []mir2.Value, memory ProfileMemorySnapshot) (ProfileAccessSnapshot, error) {
	t.recorder.Reset()
	replayed, err := t.vm.Call(call.Function, t.arguments(call))
	if err != nil {
		return ProfileAccessSnapshot{}, fmt.Errorf("loop profile: traffic model invocation @%s: %w", call.Function, err)
	}
	if len(replayed) != len(result) {
		return ProfileAccessSnapshot{}, fmt.Errorf("loop profile: traffic model result shape @%s", call.Function)
	}
	for i := range replayed {
		if replayed[i] != result[i] {
			return ProfileAccessSnapshot{}, fmt.Errorf("loop profile: traffic model result %d @%s", i, call.Function)
		}
	}
	got := t.snapshot(m)
	if !reflect.DeepEqual(got, memory) {
		return ProfileAccessSnapshot{}, fmt.Errorf("loop profile: traffic model snapshot @%s", call.Function)
	}
	return t.recorder.Snapshot()
}
