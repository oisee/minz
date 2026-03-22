// egraph.go — Equality graph for multi-variant instruction lowering.
//
// Instead of the bridge choosing ONE lowering per MIR2 instruction,
// it emits all equivalent variants into an EGraph. WFC picks the
// cheapest valid form during constraint propagation.
//
// Example: MIR2 OpMove(u16 → u8) has three valid Z80 lowerings:
//   variant A: LD A, L          (take low byte of HL, 4T)
//   variant B: LD A, E          (take low byte of DE, 4T)
//   variant C: PUSH HL; POP AF  (via stack, 21T)
//
// The EGraph stores all three. WFC evaluates each against its constraint
// state and picks variant A (cheapest, no conflicts).
//
// Design:
//   - EClass: a set of equivalent MIROp sequences (variants)
//   - EGraph: collection of EClasses with union-find
//   - Each MIR2 instruction produces an EClass (not a single MIROp)
//   - WFC iterates variants per EClass, picks cheapest that propagates

package lir

// EClass is an equivalence class of instruction lowerings.
// All variants produce the same semantic result (same vreg def, same value).
type EClass struct {
	ID       int          // unique class ID
	Variants []EVariant   // all equivalent lowerings
	Best     int          // index of cheapest valid variant (-1 = not yet selected)
}

// EVariant is one possible lowering of a MIR2 instruction (or group).
// A variant is a sequence of MIROps — some lowerings require multiple
// instructions (e.g. truncation = extract low byte + move).
type EVariant struct {
	Ops    []MIROp // instruction sequence for this variant
	Cost   int     // total cost in T-states
	Tag    string  // human-readable name for debugging ("trunc_via_L", "zext_via_HL")
}

// EGraph holds all equivalence classes for one basic block's lowering.
type EGraph struct {
	Classes []*EClass
	parent  []int // union-find parent (class ID → root class ID)
}

// NewEGraph creates an empty e-graph.
func NewEGraph() *EGraph {
	return &EGraph{}
}

// AddClass creates a new equivalence class with the given variants.
// Returns the class ID.
func (eg *EGraph) AddClass(variants ...EVariant) int {
	id := len(eg.Classes)
	eg.Classes = append(eg.Classes, &EClass{
		ID:       id,
		Variants: variants,
		Best:     -1,
	})
	eg.parent = append(eg.parent, id)
	return id
}

// SingleClass creates an EClass with exactly one variant (no alternatives).
// This is the common case for instructions with only one valid lowering.
func (eg *EGraph) SingleClass(ops []MIROp, cost int, tag string) int {
	return eg.AddClass(EVariant{Ops: ops, Cost: cost, Tag: tag})
}

// Merge unions two equivalence classes. All variants from both classes
// become available. Used when ISLE discovers that two different lowerings
// are semantically equivalent.
func (eg *EGraph) Merge(a, b int) {
	ra, rb := eg.find(a), eg.find(b)
	if ra == rb {
		return
	}
	// Merge smaller into larger.
	if len(eg.Classes[ra].Variants) < len(eg.Classes[rb].Variants) {
		ra, rb = rb, ra
	}
	eg.Classes[ra].Variants = append(eg.Classes[ra].Variants, eg.Classes[rb].Variants...)
	eg.parent[rb] = ra
}

func (eg *EGraph) find(id int) int {
	for eg.parent[id] != id {
		eg.parent[id] = eg.parent[eg.parent[id]] // path compression
		id = eg.parent[id]
	}
	return id
}

// Root returns the root class ID for a given class.
func (eg *EGraph) Root(id int) int { return eg.find(id) }

// Class returns the EClass for a given ID (follows union-find).
func (eg *EGraph) Class(id int) *EClass { return eg.Classes[eg.find(id)] }

// Extract returns the selected variant for each class.
// Call after WFC has chosen Best for each class.
// Returns the flattened MIROp sequence in class order.
func (eg *EGraph) Extract() []MIROp {
	var ops []MIROp
	seen := make(map[int]bool)
	for i := range eg.Classes {
		root := eg.find(i)
		if seen[root] {
			continue
		}
		seen[root] = true
		cls := eg.Classes[root]
		if cls.Best >= 0 && cls.Best < len(cls.Variants) {
			ops = append(ops, cls.Variants[cls.Best].Ops...)
		} else if len(cls.Variants) > 0 {
			// Fallback: pick cheapest variant.
			best := 0
			for j := 1; j < len(cls.Variants); j++ {
				if cls.Variants[j].Cost < cls.Variants[best].Cost {
					best = j
				}
			}
			ops = append(ops, cls.Variants[best].Ops...)
		}
	}
	return ops
}

// SelectCheapest picks the cheapest variant for each class.
// This is the simplest extraction strategy — no WFC interaction.
func (eg *EGraph) SelectCheapest() {
	seen := make(map[int]bool)
	for i := range eg.Classes {
		root := eg.find(i)
		if seen[root] {
			continue
		}
		seen[root] = true
		cls := eg.Classes[root]
		best := 0
		for j := 1; j < len(cls.Variants); j++ {
			if cls.Variants[j].Cost < cls.Variants[best].Cost {
				best = j
			}
		}
		cls.Best = best
	}
}

// Stats returns (total classes, total variants, avg variants per class).
func (eg *EGraph) Stats() (int, int, float64) {
	seen := make(map[int]bool)
	classes, variants := 0, 0
	for i := range eg.Classes {
		root := eg.find(i)
		if seen[root] {
			continue
		}
		seen[root] = true
		classes++
		variants += len(eg.Classes[root].Variants)
	}
	avg := 0.0
	if classes > 0 {
		avg = float64(variants) / float64(classes)
	}
	return classes, variants, avg
}
