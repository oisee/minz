// Package z80spec describes Z80 storage and physical overlap independently of
// compiler IRs. Compiler namespaces are views of this table, not location IDs.
package z80spec

// Kind classifies storage. Flags describe the physical eight-bit F register;
// compiler boolean locations may expose only one bit of that storage.
type Kind uint8

const (
	GPR8 Kind = iota
	Pair16
	Index
	IndexHalf
	Flags
	Shadow
	SMCSlot
	MemorySlot
	StackSlot
	DWordPair
)

// Location is an immutable table entry returned by value. Names are case sensitive.
type Location struct {
	Name string
	Bits int
	Kind Kind
}

type entry struct {
	Location
	// A bit per independent byte of storage. Disjoint halves never overlap.
	storage uint64
	halves  [2]string // high byte, low byte (only for sixteen-bit pairs)
}

var table = buildTable()

func buildTable() []entry {
	var result []entry
	var next uint
	add := func(name string, bits int, kind Kind) {
		result = append(result, entry{Location: Location{name, bits, kind}, storage: 1 << next})
		next++
	}
	for _, name := range []string{"A", "B", "C", "D", "E", "H", "L"} {
		add(name, 8, GPR8)
	}
	add("F", 8, Flags)
	for _, name := range []string{"IXH", "IXL", "IYH", "IYL"} {
		add(name, 8, IndexHalf)
	}
	add("SP", 16, Index)
	for _, name := range []string{"B'", "C'", "D'", "E'", "H'", "L'", "A'", "F'"} {
		add(name, 8, Shadow)
	}
	for _, name := range []string{"tsmc0", "tsmc1", "tsmc2", "tsmc3", "tsmc4", "tsmc5", "tsmc6", "tsmc7"} {
		add(name, 8, SMCSlot)
	}
	for _, name := range []string{"mem0", "mem1", "mem2", "mem3", "mem", "gpu_mem"} {
		add(name, 16, MemorySlot)
	}
	for _, name := range []string{"stk0", "stk1", "stk2", "stk3", "stack"} {
		add(name, 16, StackSlot)
	}
	mask := func(name string) uint64 {
		for _, e := range result {
			if e.Name == name {
				return e.storage
			}
		}
		panic("unknown component: " + name)
	}
	for _, pair := range []struct {
		name, hi, lo string
		kind         Kind
	}{
		{"BC", "B", "C", Pair16}, {"DE", "D", "E", Pair16}, {"HL", "H", "L", Pair16},
		{"IX", "IXH", "IXL", Index}, {"IY", "IYH", "IYL", Index}, {"AF", "A", "F", Pair16},
		{"BC'", "B'", "C'", Shadow}, {"DE'", "D'", "E'", Shadow}, {"HL'", "H'", "L'", Shadow}, {"AF'", "A'", "F'", Shadow},
	} {
		result = append(result, entry{Location{pair.name, 16, pair.kind}, mask(pair.hi) | mask(pair.lo), [2]string{pair.hi, pair.lo}})
	}
	for _, pair := range []string{"BC", "DE", "HL"} {
		result = append(result, entry{Location: Location{pair + "32", 32, DWordPair}, storage: mask(pair) | mask(pair+"'")})
	}
	return result
}

// Locations returns a copy of the complete canonical table.
func Locations() []Location {
	result := make([]Location, len(table))
	for i, e := range table {
		result[i] = e.Location
	}
	return result
}

func find(name string) (entry, bool) {
	for _, e := range table {
		if e.Name == name {
			return e, true
		}
	}
	return entry{}, false
}

// Lookup returns a canonical location, or false for an unknown name.
func Lookup(name string) (Location, bool) { e, ok := find(name); return e.Location, ok }

// Width returns the physical width in bits, or zero for an unknown name.
func Width(name string) int { e, _ := find(name); return e.Bits }

// Overlaps reports whether two known locations share any physical bit.
// Spill slots in different namespaces are independent abstract storage: the
// generic MIR2 and GPU sentinels are not equated with numbered LIR/VIR slots.
func Overlaps(a, b string) bool {
	x, ok := find(a)
	y, other := find(b)
	return ok && other && x.storage&y.storage != 0
}

// Halves returns the high and low byte names of a sixteen-bit pair.
// SP and DWord composites have no byte-half relation in this model.
func Halves(pair string) (hi, lo string, ok bool) {
	e, found := find(pair)
	return e.halves[0], e.halves[1], found && e.halves[0] != ""
}

// Parent returns a byte's immediate sixteen-bit container, if any.
func Parent(half string) (string, bool) {
	for _, e := range table {
		if e.halves[0] == half || e.halves[1] == half {
			if half != "" {
				return e.Name, true
			}
		}
	}
	return "", false
}
