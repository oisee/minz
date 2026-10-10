package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

// ProfileByteAccess records executed raw MIR2 data operations per declared byte.
// Instruction fetches, stack traffic, initialization and snapshot reads are absent.
type ProfileByteAccess struct {
	Reads  []uint64 `json:"reads"`
	Writes []uint64 `json:"writes"`
}
type ProfileAccessSnapshot struct {
	Globals   map[string]ProfileByteAccess `json:"globals"`
	Resources map[string]ProfileByteAccess `json:"resources"`
}

// ProfileAccessRecorder observes only the ordinary OpLoad/OpStore instructions
// supported by the controlled workload. Addresses are resolved from this VM.
// An access outside declared storage invalidates exact-access evidence.
type ProfileAccessRecorder struct {
	regions []profileAccessRegion
	current ProfileAccessSnapshot
	err     error
}
type profileAccessRegion struct {
	name    string
	address int64
	size    int
	global  bool
}

func NewProfileAccessRecorder(m *Module, vm *mir2.VM, w ProfileWorkload, addresses map[string]int64) *ProfileAccessRecorder {
	r := &ProfileAccessRecorder{}
	for _, g := range m.Globals {
		r.regions = append(r.regions, profileAccessRegion{g.Name, vm.GlobalAddr(g.Name), max(mir2.ByteWidth(g.Ty), len(g.Init)), true})
	}
	for name, b := range w.Resources {
		r.regions = append(r.regions, profileAccessRegion{name, addresses[name], len(b), false})
	}
	r.Reset()
	return r
}
func (r *ProfileAccessRecorder) Reset() {
	r.current = ProfileAccessSnapshot{Globals: map[string]ProfileByteAccess{}, Resources: map[string]ProfileByteAccess{}}
	r.err = nil
	for _, region := range r.regions {
		a := ProfileByteAccess{Reads: make([]uint64, region.size), Writes: make([]uint64, region.size)}
		if region.global {
			r.current.Globals[region.name] = a
		} else {
			r.current.Resources[region.name] = a
		}
	}
}
func (r *ProfileAccessRecorder) Observe(a mir2.VMDataAccess) {
	for _, region := range r.regions {
		if a.Addr >= region.address && a.Addr < region.address+int64(region.size) {
			data := r.current.Resources[region.name]
			if region.global {
				data = r.current.Globals[region.name]
			}
			offset := int(a.Addr - region.address)
			switch a.Op {
			case mir2.OpLoad:
				data.Reads[offset]++
			case mir2.OpStore:
				data.Writes[offset]++
			default:
				r.err = fmt.Errorf("unsupported raw data access")
			}
			return
		}
	}
	r.err = fmt.Errorf("raw data access outside declared storage at %d", a.Addr)
}
func (r *ProfileAccessRecorder) Snapshot() (ProfileAccessSnapshot, error) { return r.current, r.err }
