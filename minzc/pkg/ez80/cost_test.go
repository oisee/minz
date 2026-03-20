package ez80

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestEZ80CostTable_Interface(t *testing.T) {
	var ct mir2.CostTable = EZ80CostTable{}
	locs := ct.Locs()
	if len(locs) == 0 {
		t.Fatal("EZ80CostTable.Locs() returned empty")
	}

	// Should have no LocMem (no $F0xx on eZ80)
	for _, loc := range locs {
		if loc.Kind == mir2.LocMem {
			t.Errorf("eZ80 should not have LocMem locations, found %s", loc.Name)
		}
	}

	// Should have no LocDWord (24-bit native, no shadow pair needed)
	for _, loc := range locs {
		if loc.Kind == mir2.LocDWord {
			t.Errorf("eZ80 should not have LocDWord locations, found %s", loc.Name)
		}
	}
}

func TestEZ80CostTable_AccPrefersA(t *testing.T) {
	ct := EZ80CostTable{}
	costA := ct.Cost(mir2.ClassAcc, mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"})
	costB := ct.Cost(mir2.ClassAcc, mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"})

	if costA != 0 {
		t.Errorf("ClassAcc->A should be 0, got %d", costA)
	}
	if costB <= costA {
		t.Errorf("ClassAcc->B (%d) should be more expensive than ->A (%d)", costB, costA)
	}
}

func TestEZ80CostTable_CounterPrefersB(t *testing.T) {
	ct := EZ80CostTable{}
	costB := ct.Cost(mir2.ClassCounter, mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"})
	costC := ct.Cost(mir2.ClassCounter, mir2.PhysLoc{Kind: mir2.LocReg, Name: "C"})

	if costB != 0 {
		t.Errorf("ClassCounter->B should be 0, got %d", costB)
	}
	if costC <= costB {
		t.Errorf("ClassCounter->C (%d) should be > ->B (%d)", costC, costB)
	}
}

func TestEZ80CostTable_PointerPrefersHL(t *testing.T) {
	ct := EZ80CostTable{}
	costHL := ct.Cost(mir2.ClassPointer, mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"})
	costDE := ct.Cost(mir2.ClassPointer, mir2.PhysLoc{Kind: mir2.LocReg, Name: "DE"})
	costIX := ct.Cost(mir2.ClassPointer, mir2.PhysLoc{Kind: mir2.LocIXY, Name: "IX"})

	if costHL != 0 {
		t.Errorf("ClassPointer->HL should be 0, got %d", costHL)
	}
	if costDE <= costHL {
		t.Errorf("ClassPointer->DE (%d) should be > ->HL (%d)", costDE, costHL)
	}
	// IX should be cheap on eZ80 (LEA available)
	if costIX >= mir2.InfCost {
		t.Errorf("ClassPointer->IX should be viable on eZ80, got InfCost")
	}
}

func TestEZ80CostTable_DWordUsesNativePair(t *testing.T) {
	ct := EZ80CostTable{}
	// On eZ80, ClassDWord should map to regular pair (not shadow)
	costHL := ct.Cost(mir2.ClassDWord, mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"})
	costShadow := ct.Cost(mir2.ClassDWord, mir2.PhysLoc{Kind: mir2.LocShadow, Name: "H'"})

	if costHL != 0 {
		t.Errorf("ClassDWord->HL should be 0 on eZ80 (native 24-bit), got %d", costHL)
	}
	if costShadow <= costHL {
		t.Errorf("ClassDWord->shadow (%d) should be > ->HL (%d)", costShadow, costHL)
	}
}

func TestEZ80CostTable_NoMemLoc(t *testing.T) {
	ct := EZ80CostTable{}
	cost := ct.Cost(mir2.ClassMem, mir2.PhysLoc{Kind: mir2.LocMem, Name: "mem"})
	if cost != mir2.InfCost {
		t.Errorf("ClassMem should be InfCost on eZ80 (no $F0xx), got %d", cost)
	}
}

func TestEZ80CostTable_StackViable(t *testing.T) {
	ct := EZ80CostTable{}
	// Stack spills should be viable on eZ80 (cheaper than Z80)
	costStack := ct.Cost(mir2.ClassAcc, mir2.PhysLoc{Kind: mir2.LocStack, Name: "stack"})
	if costStack >= mir2.InfCost {
		t.Errorf("ClassAcc->stack should be viable on eZ80, got InfCost")
	}
	if costStack != ez80StackRoundTrip {
		t.Errorf("ClassAcc->stack should be %d, got %d", ez80StackRoundTrip, costStack)
	}
}

func TestEZ80CostTable_IXY8Official(t *testing.T) {
	ct := EZ80CostTable{}
	// IXH/IXL should be viable for general registers on eZ80
	costIXH := ct.Cost(mir2.ClassGeneral, mir2.PhysLoc{Kind: mir2.LocIXY8, Name: "IXH"})
	if costIXH >= mir2.InfCost {
		t.Errorf("ClassGeneral->IXH should be viable on eZ80 (official), got InfCost")
	}
	// Should be cheaper than stack
	costStack := ct.Cost(mir2.ClassGeneral, mir2.PhysLoc{Kind: mir2.LocStack, Name: "stack"})
	if costIXH >= costStack {
		t.Errorf("IXH (%d) should be cheaper than stack (%d) on eZ80", costIXH, costStack)
	}
}

func TestEZ80CostTable_CheaperThanZ80(t *testing.T) {
	ez80 := EZ80CostTable{}
	z80 := mir2.Z80CostTable{}

	// eZ80 move costs should be strictly less than Z80
	locB := mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"}
	ez80Cost := ez80.Cost(mir2.ClassAcc, locB)
	z80Cost := z80.Cost(mir2.ClassAcc, locB)

	if ez80Cost >= z80Cost {
		t.Errorf("eZ80 ClassAcc->B (%d) should be < Z80 (%d)", ez80Cost, z80Cost)
	}

	// eZ80 shadow cost should be cheaper
	locShadow := mir2.PhysLoc{Kind: mir2.LocShadow, Name: "B'"}
	ez80Shadow := ez80.Cost(mir2.ClassPair, locShadow)
	z80Shadow := z80.Cost(mir2.ClassPair, locShadow)

	if z80Shadow < mir2.InfCost && ez80Shadow >= z80Shadow {
		t.Errorf("eZ80 ClassPair->shadow (%d) should be < Z80 (%d)", ez80Shadow, z80Shadow)
	}
}
