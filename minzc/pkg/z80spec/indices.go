package z80spec

// These namespace views deliberately freeze the current index contracts.
// Tests compare them with the compiler tables so renumbering is visible.
var virNames = [...]string{
	"A", "B", "C", "D", "E", "H", "L", "BC", "DE", "HL", "SP", "IX", "IY", "F",
	"IXH", "IXL", "IYH", "IYL", "B'", "C'", "D'", "E'", "H'", "L'", "A'",
	"tsmc0", "tsmc1", "tsmc2", "tsmc3", "tsmc4", "tsmc5", "tsmc6", "tsmc7",
	"mem0", "mem1", "mem2", "mem3", "stk0", "stk1", "stk2", "stk3",
}
var gpuNames = [...]string{"A", "B", "C", "D", "E", "H", "L", "BC", "DE", "HL", "IXH", "IXL", "IYH", "IYL", "gpu_mem"}

func fromIndex(names []string, index int) (Location, bool) {
	if index < 0 || index >= len(names) {
		return Location{}, false
	}
	return Lookup(names[index])
}
func toIndex(names []string, name string) (int, bool) {
	for i, n := range names {
		if n == name {
			return i, true
		}
	}
	return -1, false
}

// FromLIR maps one of the 37 LIR locations to canonical storage.
func FromLIR(index int) (Location, bool) { return fromIndex(virNames[:37], index) }

// ToLIR fails for locations with no LIR counterpart (including stack slots).
func ToLIR(name string) (int, bool) { return toIndex(virNames[:37], name) }

// FromVIR maps one of the 41 VIR locations; _vir_memN becomes memN.
func FromVIR(index int) (Location, bool) { return fromIndex(virNames[:], index) }
func ToVIR(name string) (int, bool)      { return toIndex(virNames[:], name) }

// FromGPU maps the 15-location GPU namespace; slot 14 is the independent spill
// sentinel gpu_mem, which has no direct LIR/VIR index.
func FromGPU(index int) (Location, bool) { return fromIndex(gpuNames[:], index) }
func ToGPU(name string) (int, bool)      { return toIndex(gpuNames[:], name) }
