package mir2

import "testing"

func shiftTriangleFixture() (*Module, *Func) {
	m := &Module{}
	f := m.AddFunc("serial_step")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassGeneral)
	a := b.Param("a", TyU8, ClassGeneral)
	mask := b.Const(1, TyU8, ClassGeneral)
	test := b.And(x, mask, TyU8, ClassGeneral)
	zero := b.Const(0, TyU8, ClassGeneral)
	flag := b.Cmp(CmpNe, test, zero, ClassFlag, false)
	b.BrIf(flag, "yes", nil, "join", []Reg{a})
	b.SwitchToNewBlock("yes")
	sum := b.Add(a, a, TyU8, ClassGeneral)
	b.Jmp("join", sum)
	join := b.SwitchToNewBlock("join")
	z := b.BlockParam(join, TyU8, ClassGeneral)
	k := b.Const(1, TyU8, ClassGeneral)
	shift := b.Shr(x, k, TyU8, ClassGeneral)
	out := b.Xor(z, shift, TyU8, ClassGeneral)
	b.Ret(out)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	return m, f
}

func TestPrepareShiftCarryTriangle(t *testing.T) {
	m, f := shiftTriangleFixture()
	if !PrepareShiftCarry(f) {
		t.Fatal("missed triangle")
	}
	if e := Verify(m); e != nil {
		t.Fatal(e)
	}
	head := f.Entry()
	n := len(head.Insts)
	if head.Insts[n-1].Op != OpShr || head.Insts[n-2].Op != OpCmp {
		t.Fatal(m.Dump())
	}
	for x := 0; x < 256; x++ {
		for _, a := range []int{0, 1, 127, 128, 255} {
			want := a
			if x&1 != 0 {
				want = (a * 2) & 255
			}
			want ^= x >> 1
			got, e := NewVM(m).Call(f.Name, []Value{{I: int64(x)}, {I: int64(a)}})
			if e != nil || len(got) != 1 || got[0].I != int64(want) {
				t.Fatal(x, a, got, e, want)
			}
		}
	}
	if PrepareShiftCarry(f) {
		t.Fatal("not idempotent")
	}
}

func TestPrepareShiftCarryRefusals(t *testing.T) {
	for _, kind := range []string{"wrong-bit", "count-two", "arithmetic-right", "arm-store", "arm-call", "shared-arm", "shared-join", "escaping-arm", "looping-arm", "join-source", "arm-source", "hard-shift", "volatile-shift", "hard-count", "shared-test", "shared-zero", "arm-old-source"} {
		t.Run(kind, func(t *testing.T) {
			m, f := shiftTriangleFixture()
			head, arm, join := f.Blocks[0], f.Blocks[1], f.Blocks[2]
			switch kind {
			case "arm-old-source":
				arm.Insts[0].Src[0] = f.Contract.Params[0].Reg
			case "shared-test":
				arm.Insts[0].Src[0] = head.Insts[1].Dst
			case "shared-zero":
				arm.Insts[0].Src[0] = head.Insts[2].Dst
			case "wrong-bit":
				head.Insts[0].Imm = 2
			case "count-two":
				join.Insts[0].Imm = 2
			case "arithmetic-right":
				join.Insts[1].Op = OpSar
			case "arm-store":
				arm.Insts = append(arm.Insts, &Inst{Op: OpStore, Src: [2]Reg{1, 2}, Ty: TyU8})
			case "arm-call":
				arm.Insts = append(arm.Insts, &Inst{Op: OpCall, Sym: "opaque", Ty: TyVoid})
			case "shared-arm":
				f.NewBlock("other").Term = &TermJmp{Target: arm.Label}
			case "shared-join":
				f.NewBlock("other").Term = &TermJmp{Target: join.Label, Args: []Reg{1}}
			case "escaping-arm":
				arm.Term = &TermRet{Vals: []Reg{1}}
			case "looping-arm":
				arm.Term = &TermJmp{Target: head.Label}
			case "join-source":
				join.Insts[1].Src[0] = join.Params[0].Dst
			case "arm-source":
				join.Insts[1].Src[0] = arm.Insts[0].Dst
			case "hard-shift":
				join.Insts[1].ClsHard = true
			case "volatile-shift":
				join.Insts[1].Volatile = true
			case "hard-count":
				join.Insts[0].ClsHard = true
			}
			before := m.Dump()
			if PrepareShiftCarry(f) || m.Dump() != before {
				t.Fatal("unsafe or partial rewrite", kind)
			}
		})
	}
}

// Stage two must leave an already adjacent shift/test alone if the AND value
// survives the branch. Reordering would turn the BIT branch into a boolean.
func TestPrepareShiftCarrySharedPredicate(t *testing.T) {
	m, f := shiftTriangleFixture()
	head, arm, join := f.Blocks[0], f.Blocks[1], f.Blocks[2]
	cmp := head.Insts[len(head.Insts)-1]
	head.Insts = append(head.Insts[:len(head.Insts)-1], join.Insts[0], join.Insts[1], cmp)
	join.Insts = join.Insts[2:]
	arm.Insts[0].Src[0] = head.Insts[1].Dst
	before := m.Dump()
	if PrepareShiftCarry(f) || m.Dump() != before {
		t.Fatal("shared predicate was reordered", m.Dump())
	}
}
