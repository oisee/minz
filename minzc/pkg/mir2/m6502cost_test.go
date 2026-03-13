package mir2_test

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestM6502CostTableInterface(t *testing.T) {
	var ct mir2.CostTable = mir2.M6502CostTable{}
	locs := ct.Locs()
	if len(locs) == 0 {
		t.Fatal("M6502CostTable.Locs() returned empty slice")
	}
	t.Logf("%d physical locations", len(locs))
}

// TestM6502CostPerfectFits checks that each class has at least one zero-cost location.
func TestM6502CostPerfectFits(t *testing.T) {
	ct := mir2.M6502CostTable{}
	locs := ct.Locs()

	cases := []struct {
		cls  mir2.RegClass
		name string
	}{
		{mir2.ClassAcc, "A"},
		{mir2.ClassCounter, "X"},
		{mir2.ClassGeneral, "A"},
		{mir2.ClassPointer, "zp23"},
		{mir2.ClassIndex, "Y"},
		{mir2.ClassPair, "zp23"},
		{mir2.ClassFlag, "P"},
		{mir2.ClassStack, "stack"},
		{mir2.ClassMem, "zp2"},
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
			t.Errorf("class %s: loc %s not found in Locs()", tc.cls, tc.name)
		}
	}
}

// TestM6502CostOrdering verifies key cost invariants.
func TestM6502CostOrdering(t *testing.T) {
	ct := mir2.M6502CostTable{}
	locs := ct.Locs()

	locByName := make(map[string]mir2.PhysLoc)
	for _, l := range locs {
		locByName[l.Name] = l
	}

	cost := func(cls mir2.RegClass, name string) int {
		return ct.Cost(cls, locByName[name])
	}

	// ClassAcc: A < X < ZP < abs
	assertLess(t, "ClassAcc: A < X", cost(mir2.ClassAcc, "A"), cost(mir2.ClassAcc, "X"))
	assertLess(t, "ClassAcc: X < zp2", cost(mir2.ClassAcc, "X"), cost(mir2.ClassAcc, "zp2"))
	assertLess(t, "ClassAcc: zp2 < abs", cost(mir2.ClassAcc, "zp2"), cost(mir2.ClassAcc, "abs"))

	// ClassCounter: X < Y < A
	assertLessEq(t, "ClassCounter: X <= Y", cost(mir2.ClassCounter, "X"), cost(mir2.ClassCounter, "Y"))
	assertLess(t, "ClassCounter: Y < A", cost(mir2.ClassCounter, "Y"), cost(mir2.ClassCounter, "A"))

	// ClassIndex: Y < X < A
	assertLessEq(t, "ClassIndex: Y <= X", cost(mir2.ClassIndex, "Y"), cost(mir2.ClassIndex, "X"))
	assertLess(t, "ClassIndex: X < A", cost(mir2.ClassIndex, "X"), cost(mir2.ClassIndex, "A"))

	// ClassPointer: zp pair = 0, registers = inf
	if cost(mir2.ClassPointer, "A") != mir2.InfCost {
		t.Error("ClassPointer: A should be InfCost (no 16-bit regs)")
	}
	if cost(mir2.ClassPointer, "zp23") != 0 {
		t.Error("ClassPointer: zp23 should be 0")
	}

	// ClassGeneral: A < X/Y < ZP < abs
	assertLess(t, "ClassGeneral: A < X", cost(mir2.ClassGeneral, "A"), cost(mir2.ClassGeneral, "X"))
	assertLess(t, "ClassGeneral: X < zp2", cost(mir2.ClassGeneral, "X"), cost(mir2.ClassGeneral, "zp2"))
	assertLess(t, "ClassGeneral: zp2 < abs", cost(mir2.ClassGeneral, "zp2"), cost(mir2.ClassGeneral, "abs"))
}

func assertLess(t *testing.T, msg string, a, b int) {
	t.Helper()
	if a >= b {
		t.Errorf("%s: got %d >= %d", msg, a, b)
	}
}

func assertLessEq(t *testing.T, msg string, a, b int) {
	t.Helper()
	if a > b {
		t.Errorf("%s: got %d > %d", msg, a, b)
	}
}
