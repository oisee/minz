package mir2

import "testing"

func TestByteCopyCostKeepsForbiddenLocationsAndNonCopyCosts(t *testing.T) {
	f := (&Module{}).AddFunc("copy_cost")
	b := f.NewBlock("entry")
	b.Insts = []*Inst{
		{Op: OpLoad, Dst: 3, Src: [2]Reg{1}, Ty: TyU8},
		{Op: OpStore, Src: [2]Reg{2, 3}, Ty: TyU8},
	}
	b.Term = &TermRet{Vals: []Reg{4}}
	locs := []PhysLoc{{Name: "HL"}, {Name: "DE"}, {Name: "BC"}}
	states := map[Reg]*regState{
		2: {info: RegInfo{Ty: TyPtr, Cls: ClassPointer}, costs: []int{0, 8, InfCost}},
		4: {info: RegInfo{Ty: TyU8, Cls: ClassAcc}, costs: []int{0, 0, 0}},
	}
	lr := &LivenessResult{LiveOut: []*RegSet{newRegSet()}}
	applyByteMemoryCosts(f, lr, states, locs)
	if got := states[2].costs; got[0] != 0 || got[1] != 33 || got[2] != InfCost {
		t.Fatal(got)
	}
	// Constant-valued fill is outside this model.
	b.Insts[0].Op = OpConst
	states[2].costs = []int{0, 8, InfCost}
	applyByteMemoryCosts(f, lr, states, locs)
	if states[2].costs[1] != 8 {
		t.Fatal("non-copy cost changed")
	}
}
