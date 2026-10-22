package mir2

import "testing"

func TestCoalesceLoopShiftBackedge(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("serial_loop")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	input := b.Param("x", TyU8, ClassGeneral)
	b.Jmp("loop", input)
	loop := b.SwitchToNewBlock("loop")
	x := b.BlockParam(loop, TyU8, ClassGeneral)
	bit := b.BitGet(x, 0, TyU8, ClassGeneral)
	zero := b.Const(0, TyU8, ClassGeneral)
	flag := b.Cmp(CmpNe, bit, zero, ClassFlag, false)
	count := b.Const(1, TyU8, ClassGeneral)
	shift := b.Shr(x, count, TyU8, ClassGeneral)
	b.BrIf(flag, "loop", []Reg{shift}, "exit", []Reg{shift})
	exit := b.SwitchToNewBlock("exit")
	out := b.BlockParam(exit, TyU8, ClassGeneral)
	b.Ret(out)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
	for r, home := range map[Reg]string{input: "C", x: "C", bit: "A", zero: "D", flag: "F", count: "D", shift: "A", out: "C"} {
		kind := LocReg
		if home == "F" {
			kind = LocFlag
		}
		ar.Locs[r] = PhysLoc{Kind: kind, Name: home}
	}
	coalesceAllocResult(f, ar, ComputeLiveness(f))
	if ar.Locs[x].Name != "C" || ar.Locs[shift].Name != "C" {
		t.Fatalf("loop input=%s shift=%s; want in-place C shift", ar.Locs[x].Name, ar.Locs[shift].Name)
	}
}

func TestCoalesceLoopShiftImmediateCount(t *testing.T) {
	for _, registerUse := range []bool{false, true} {
		m := &Module{}
		f := m.AddFunc("serial_word_state")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		input := b.Param("x", TyU8, ClassGeneral)
		word := b.Param("word", TyU16, ClassPointer)
		b.Jmp("loop", input, word)
		loop := b.SwitchToNewBlock("loop")
		x := b.BlockParam(loop, TyU8, ClassGeneral)
		w := b.BlockParam(loop, TyU16, ClassPointer)
		bit := b.BitGet(x, 0, TyU8, ClassGeneral)
		zero := b.Const(0, TyU8, ClassGeneral)
		flag := b.Cmp(CmpNe, bit, zero, ClassFlag, false)
		count := b.Const(1, TyU8, ClassGeneral)
		shift := b.Shr(x, count, TyU8, ClassGeneral)
		k := b.Const(3, TyU16, ClassPair)
		next := b.Shl(w, k, TyU16, ClassPointer)
		if registerUse {
			b.Cur.Insts[len(b.Cur.Insts)-1].Op = OpAdd
		}
		b.BrIf(flag, "loop", []Reg{shift, next}, "exit", []Reg{shift})
		exit := b.SwitchToNewBlock("exit")
		out := b.BlockParam(exit, TyU8, ClassGeneral)
		b.Ret(out)
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
		for r, home := range map[Reg]string{input: "C", word: "HL", x: "C", w: "HL", bit: "A", zero: "D", flag: "F", count: "D", shift: "A", k: "BC", next: "HL", out: "C"} {
			kind := LocReg
			if home == "F" {
				kind = LocFlag
			}
			ar.Locs[r] = PhysLoc{Kind: kind, Name: home}
		}
		coalesceAllocResult(f, ar, ComputeLiveness(f))
		want := "C"
		if registerUse {
			want = "A"
		}
		if ar.Locs[shift].Name != want {
			t.Fatalf("registerUse=%v shift=%s want %s", registerUse, ar.Locs[shift].Name, want)
		}
	}
}
