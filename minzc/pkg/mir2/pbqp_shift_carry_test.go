package mir2

import "testing"

// shiftCarryFixture: entry(p) computes x = p + 1, tests bit 7 of x, shifts x
// left by one and branches; both arms return the shift.
func shiftCarryFixture() (*Func, Reg, Reg, Reg) {
	f := &Func{Name: "sc"}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	p := b.Param("p", TyU8, ClassGeneral)
	one := b.Const(1, TyU8, ClassGeneral)
	x := b.BinOp(OpAdd, p, one, TyU8, ClassGeneral)
	bit := b.BitGet(x, 7, TyU8, ClassGeneral)
	zero := b.Const(0, TyU8, ClassGeneral)
	k := b.Const(1, TyU8, ClassGeneral)
	cond := b.Cmp(CmpNe, bit, zero, ClassFlag, false)
	shifted := b.Shl(x, k, TyU8, ClassGeneral)
	b.BrIf(cond, "a", nil, "b", nil)
	b.SwitchToNewBlock("a")
	b.Ret(shifted)
	b.SwitchToNewBlock("b")
	b.Ret(shifted)
	return f, p, x, shifted
}

func shiftCarryStates(f *Func, locs []PhysLoc) map[Reg]*regState {
	info := collectRegInfo(f)
	states := map[Reg]*regState{}
	for r, cv := range nodeCosts(f, info, Z80CostTable{}, locs) {
		states[r] = &regState{info: info[r], costs: cv}
	}
	return states
}

// The preference removes only ClassGeneral's per-use A surcharge, adds the
// reward elsewhere, and never touches a parameter or a hard value.
func TestShiftCarryNudgeIsBounded(t *testing.T) {
	f, p, x, shifted := shiftCarryFixture()
	locs := Z80PhysLocs
	idx := func(name string) int {
		for i, l := range locs {
			if l.Kind == LocReg && l.Name == name {
				return i
			}
		}
		t.Fatalf("no %s", name)
		return -1
	}
	a, c := idx("A"), idx("C")
	before := shiftCarryStates(f, locs)
	after := shiftCarryStates(f, locs)
	applyShiftCarryAffinityNudge(f, after, locs)
	for _, r := range []Reg{x, shifted} {
		if after[r].costs[a] >= after[r].costs[c] {
			t.Errorf("r%d: A %d not preferred to C %d", r, after[r].costs[a], after[r].costs[c])
		}
		// Bounded: exactly the surcharge is removed from A, never more.
		if after[r].costs[a] != before[r].costs[a]-2*(before[r].costs[a]/2) || after[r].costs[c] != before[r].costs[c]+ShiftCarryAReward {
			t.Errorf("r%d: costs A %d->%d C %d->%d", r, before[r].costs[a], after[r].costs[a], before[r].costs[c], after[r].costs[c])
		}
	}
	for i := range locs {
		if after[p].costs[i] != before[p].costs[i] {
			t.Fatalf("parameter cost changed at %v", locs[i])
		}
	}
	// A finite penalty already on A survives (only the surcharge is removed).
	extra := shiftCarryStates(f, locs)
	extra[x].costs[a] += 100
	applyShiftCarryAffinityNudge(f, extra, locs)
	if extra[x].costs[a] < 100 {
		t.Fatalf("finite A penalty overridden: %d", extra[x].costs[a])
	}
	// Hard values are never nudged.
	for _, blk := range f.Blocks {
		for _, i := range blk.Insts {
			if i.Dst == shifted {
				i.ClsHard = true
			}
		}
	}
	hard := shiftCarryStates(f, locs)
	saved := append([]int(nil), hard[shifted].costs...)
	applyShiftCarryAffinityNudge(f, hard, locs)
	for i := range saved {
		if hard[shifted].costs[i] != saved[i] {
			t.Fatal("hard value nudged")
		}
	}
}

// A function without blocks (a declaration the pipeline still allocates) is
// left alone instead of indexing its missing entry block.
func TestShiftCarryNudgeEmptyFunction(t *testing.T) {
	applyShiftCarryAffinityNudge(&Func{Name: "empty"}, map[Reg]*regState{}, Z80PhysLocs)
}

// The preference is off unless the table asks for it, and never under size.
func TestShiftCarryNudgeOnlyWhenRequested(t *testing.T) {
	for _, ct := range []Z80CostTable{{}, {OptimizeForSize: true, ShiftCarryPreference: true}} {
		f, _, _, shifted := shiftCarryFixture()
		ar := PBQPAllocate(f, ComputeLiveness(f), ct)
		g, _, _, shifted2 := shiftCarryFixture()
		ar2 := PBQPAllocate(g, ComputeLiveness(g), ct)
		if ar.Loc(shifted) != ar2.Loc(shifted2) {
			t.Fatal("nondeterministic")
		}
	}
	f, _, _, shifted := shiftCarryFixture()
	plain := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
	g, _, _, shifted2 := shiftCarryFixture()
	pref := PBQPAllocate(g, ComputeLiveness(g), Z80CostTable{ShiftCarryPreference: true})
	if pref.Loc(shifted2).Name != "A" {
		t.Fatalf("preferred home %v, want A (plain %v)", pref.Loc(shifted2), plain.Loc(shifted))
	}
}
