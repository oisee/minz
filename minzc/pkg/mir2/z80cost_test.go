package mir2_test

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestZ80CostTableInterface(t *testing.T) {
	var ct mir2.CostTable = mir2.Z80CostTable{}
	locs := ct.Locs()
	if len(locs) == 0 {
		t.Fatal("Z80CostTable.Locs() returned empty slice")
	}
}

// TestZ80CostPerfectFits checks that each class has at least one zero-cost location.
func TestZ80CostPerfectFits(t *testing.T) {
	ct := mir2.Z80CostTable{}
	locs := ct.Locs()

	cases := []struct {
		cls  mir2.RegClass
		name string
	}{
		{mir2.ClassAcc, "A"},
		{mir2.ClassCounter, "B"},
		{mir2.ClassGeneral, "C"}, // C/D/E/H/L preferred; A and B have slight penalty
		{mir2.ClassPointer, "HL"},
		{mir2.ClassIndex, "DE"},
		{mir2.ClassPair, "HL"},
		{mir2.ClassIX, "IX"},
		{mir2.ClassIY, "IY"},
		{mir2.ClassIXY8, "IXH"},
		{mir2.ClassShadow, "B'"},
		{mir2.ClassAccShadow, "A'"},
		{mir2.ClassStack, "stack"},
		{mir2.ClassMem, "mem"},
		{mir2.ClassFlag, "F"},
	}

	for _, tc := range cases {
		var found bool
		for _, loc := range locs {
			if loc.Name == tc.name {
				c := ct.Cost(tc.cls, loc)
				if c != 0 {
					t.Errorf("class %s + loc %s: want cost 0, got %d", tc.cls, loc.Name, c)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("location %q not found in Z80PhysLocs", tc.name)
		}
	}
}

// TestZ80CostImpossible checks that InfCost is returned for physically impossible combos.
func TestZ80CostImpossible(t *testing.T) {
	ct := mir2.Z80CostTable{}
	locs := ct.Locs()

	impossible := []struct {
		cls  mir2.RegClass
		name string
	}{
		// ClassAcc on flag register: a flag can never be an accumulator value.
		{mir2.ClassAcc, "F"},
		// ClassPointer: 8-bit regs can't hold 16-bit addresses.
		// (locCompatible handles width, but the cost table also marks these impossible.)
		{mir2.ClassPointer, "A"},
		{mir2.ClassPointer, "B"},
		{mir2.ClassIndex, "A"},
		// ClassFlag on index/shadow: flags don't live in these locations.
		{mir2.ClassFlag, "IX"},
		{mir2.ClassFlag, "B'"},
		// ClassIXY8: H and L conflict with the IX/IY DD/FD prefix.
		{mir2.ClassIXY8, "H"},
		{mir2.ClassIXY8, "L"},
	}

	for _, tc := range impossible {
		for _, loc := range locs {
			if loc.Name == tc.name {
				c := ct.Cost(tc.cls, loc)
				if c != mir2.InfCost {
					t.Errorf("class %s + loc %s: want InfCost, got %d", tc.cls, loc.Name, c)
				}
				break
			}
		}
	}
}

// TestZ80CostHierarchy checks that spill tiers have increasing cost for each class.
func TestZ80CostHierarchy(t *testing.T) {
	ct := mir2.Z80CostTable{}

	loc := func(name string) mir2.PhysLoc {
		for _, l := range ct.Locs() {
			if l.Name == name {
				return l
			}
		}
		t.Fatalf("location %q not in Z80PhysLocs", name)
		return mir2.PhysLoc{}
	}

	// For ClassGeneral: A (2) < IXH (8) < stack (21) < mem (28).
	// Shadow registers are InfCost for ClassGeneral — EXX codegen not yet implemented.
	a := ct.Cost(mir2.ClassGeneral, loc("A"))
	ixh := ct.Cost(mir2.ClassGeneral, loc("IXH"))
	stack := ct.Cost(mir2.ClassGeneral, loc("stack"))
	mem := ct.Cost(mir2.ClassGeneral, loc("mem"))

	if !(a < ixh && ixh < stack && stack < mem) {
		t.Errorf("ClassGeneral cost hierarchy broken: A=%d IXH=%d stack=%d mem=%d",
			a, ixh, stack, mem)
	}

	// For ClassPointer: HL (0) < DE (4) < IX (8) < stack (InfCost or high)
	hl := ct.Cost(mir2.ClassPointer, loc("HL"))
	de := ct.Cost(mir2.ClassPointer, loc("DE"))
	ix := ct.Cost(mir2.ClassPointer, loc("IX"))

	if !(hl < de && de < ix) {
		t.Errorf("ClassPointer cost hierarchy broken: HL=%d DE=%d IX=%d", hl, de, ix)
	}

	// For ClassFlag: F (0) < A (8) < stack (21) < mem (28)
	flag := ct.Cost(mir2.ClassFlag, loc("F"))
	flagA := ct.Cost(mir2.ClassFlag, loc("A"))
	flagStack := ct.Cost(mir2.ClassFlag, loc("stack"))
	flagMem := ct.Cost(mir2.ClassFlag, loc("mem"))

	if !(flag < flagA && flagA < flagStack && flagStack < flagMem) {
		t.Errorf("ClassFlag cost hierarchy broken: F=%d A=%d stack=%d mem=%d",
			flag, flagA, flagStack, flagMem)
	}
}

// TestZ80CostShadowDisabledForStandardClasses verifies that shadow registers are
// InfCost for ClassGeneral, ClassAcc, and ClassCounter.
//
// EXX / EX AF,AF' emission is not yet implemented in the codegen. Allowing the
// allocator to assign to shadow locations would silently produce broken code, so
// we guard against it by returning InfCost from the cost table.
// ClassShadow / ClassAccShadow are the dedicated classes for explicit shadow use.
func TestZ80CostShadowDisabledForStandardClasses(t *testing.T) {
	ct := mir2.Z80CostTable{}

	loc := func(name string) mir2.PhysLoc {
		for _, l := range ct.Locs() {
			if l.Name == name {
				return l
			}
		}
		t.Fatalf("location %q not found", name)
		return mir2.PhysLoc{}
	}

	shadowLocs := []string{"A'", "B'", "C'", "D'", "E'", "H'", "L'"}
	classes := []struct {
		cls  mir2.RegClass
		name string
	}{
		{mir2.ClassGeneral, "ClassGeneral"},
		{mir2.ClassAcc, "ClassAcc"},
		{mir2.ClassCounter, "ClassCounter"},
	}

	for _, tc := range classes {
		for _, sname := range shadowLocs {
			c := ct.Cost(tc.cls, loc(sname))
			if c != mir2.InfCost {
				t.Errorf("%s + %s: want InfCost (EXX not implemented), got %d", tc.name, sname, c)
			}
		}
	}
}

// TestZ80CostIXYCheaperThanMemory verifies Tier 1 (index regs) < Tier 4 (memory).
func TestZ80CostIXYCheaperThanMemory(t *testing.T) {
	ct := mir2.Z80CostTable{}

	loc := func(name string) mir2.PhysLoc {
		for _, l := range ct.Locs() {
			if l.Name == name {
				return l
			}
		}
		t.Fatalf("location %q not found", name)
		return mir2.PhysLoc{}
	}

	// For ClassPointer: IX (8) < mem (InfCost — can't do pointer-via-mem!)
	// Better check ClassPair: IX (8) < stack (21) < mem (28)
	ixCostPair := ct.Cost(mir2.ClassPair, loc("IX"))
	stackCost := ct.Cost(mir2.ClassPair, loc("stack"))
	memCost := ct.Cost(mir2.ClassPair, loc("mem"))

	if !(ixCostPair < stackCost && stackCost < memCost) {
		t.Errorf("ClassPair tier order: IX=%d stack=%d mem=%d", ixCostPair, stackCost, memCost)
	}
}

// TestZ80CostIXPreferredOverIY checks IX < IY for ClassIX (and reverse for ClassIY).
func TestZ80CostIXPreferredOverIY(t *testing.T) {
	ct := mir2.Z80CostTable{}

	loc := func(name string) mir2.PhysLoc {
		for _, l := range ct.Locs() {
			if l.Name == name {
				return l
			}
		}
		t.Fatalf("location %q not found", name)
		return mir2.PhysLoc{}
	}

	ixForIX := ct.Cost(mir2.ClassIX, loc("IX"))
	iyForIX := ct.Cost(mir2.ClassIX, loc("IY"))
	if ixForIX >= iyForIX {
		t.Errorf("ClassIX: IX=%d should be < IY=%d", ixForIX, iyForIX)
	}

	iyForIY := ct.Cost(mir2.ClassIY, loc("IY"))
	ixForIY := ct.Cost(mir2.ClassIY, loc("IX"))
	if iyForIY >= ixForIY {
		t.Errorf("ClassIY: IY=%d should be < IX=%d", iyForIY, ixForIY)
	}
}
