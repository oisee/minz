package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// A flag predicate copied into a byte register is built in A. The copy saves
// A (PUSH pair / PUSH AF … POP) unless the caller proves A dead. These tests
// pin hand allocations so each caller context is exercised exactly: x in A,
// y in C, result in A.

var flagCopyConds = []CmpCond{CmpUlt, CmpUge, CmpUle, CmpUgt, CmpEq, CmpNe}

var flagCopyValues = []byte{0, 1, 2, 9, 10, 31, 32, 127, 128, 129, 200, 254, 255}

func flagCopyHolds(cond CmpCond, x, y byte) byte {
	var ok bool
	switch cond {
	case CmpUlt:
		ok = x < y
	case CmpUge:
		ok = x >= y
	case CmpUle:
		ok = x <= y
	case CmpUgt:
		ok = x > y
	case CmpEq:
		ok = x == y
	case CmpNe:
		ok = x != y
	}
	if ok {
		return 1
	}
	return 0
}

// flagCopyRun emits f with the hand allocation, runs it for every (x, y) pair
// of flagCopyValues and compares A with want. It returns f's assembly.
func flagCopyRun(t *testing.T, m *Module, locs map[Reg]PhysLoc, want func(x, y byte) byte) string {
	t.Helper()
	asm, err := Z80Codegen(m, &AllocResult{Locs: locs})
	if err != nil {
		t.Fatal(err)
	}
	bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL flagfn\nHALT\n" + asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatalf("assemble: %v %v\n%s", err, bin.Errors, asm)
	}
	for _, x := range flagCopyValues {
		for _, y := range flagCopyValues {
			z := emulator.NewRemogattoZ80()
			z.Reset()
			z.LoadMemory(0x8000, bin.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00, A: x, BC: 0x5a00 | uint16(y), DE: 0x1234, HL: 0x5678})
			for n := 0; !z.IsHalted(); n++ {
				if n >= 2000 {
					t.Fatalf("no HALT\n%s", asm)
				}
				z.Step()
			}
			got := z.GetRegisters()
			if got.A != want(x, y) || got.SP != 0xff00 {
				t.Fatalf("x=%d y=%d: A=%d want %d SP=%04x\n%s", x, y, got.A, want(x, y), got.SP, asm)
			}
		}
	}
	return asm
}

func funcText(asm, name string) string {
	start := strings.Index(asm, name+":")
	if start < 0 {
		return asm
	}
	rest := asm[start:]
	if end := strings.Index(rest, "\n\n"); end >= 0 {
		return rest[:end]
	}
	return rest
}

func reg8(name string) PhysLoc { return PhysLoc{Kind: LocReg, Name: name} }

// (a) Edge copy, A dead: entry: p = cmp x, y; jmp join(p); join(q in C):
// ret ext q. Nothing reads A between the copy and LD A, C, so no save.
func TestNativeFlagEdgeCopyDeadAccumulatorNoSave(t *testing.T) {
	for _, cond := range flagCopyConds {
		m := &Module{Name: "flagcopy"}
		f := m.AddFunc("flagfn")
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		b := NewBuilder(f)
		x := b.Param("x", TyU8, ClassAcc)
		y := b.Param("y", TyU8, ClassGeneral)
		b.SwitchToNewBlock("entry")
		p := b.Cmp(cond, x, y, ClassFlag, false)
		b.Jmp("join", p)
		join := b.SwitchToNewBlock("join")
		q := b.BlockParam(join, TyBool, ClassGeneral)
		r := b.Ext(q, TyBool, TyU8, ClassAcc)
		b.Ret(r)
		locs := map[Reg]PhysLoc{x: reg8("A"), y: reg8("C"), p: {Kind: LocFlag, Name: "F"}, q: reg8("C"), r: reg8("A")}
		asm := flagCopyRun(t, m, locs, func(x, y byte) byte { return flagCopyHolds(cond, x, y) })
		if body := funcText(asm, "flagfn"); strings.Contains(body, "PUSH") {
			t.Fatalf("cond %v: dead A saved:\n%s", cond, body)
		}
	}
}

// (b) Edge copy, A live into the target: join reads x (in A) after the copy.
func TestNativeFlagEdgeCopyLiveAccumulatorThroughTarget(t *testing.T) {
	for _, cond := range flagCopyConds {
		m := &Module{Name: "flagcopy"}
		f := m.AddFunc("flagfn")
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		b := NewBuilder(f)
		x := b.Param("x", TyU8, ClassAcc)
		y := b.Param("y", TyU8, ClassGeneral)
		b.SwitchToNewBlock("entry")
		p := b.Cmp(cond, x, y, ClassFlag, false)
		b.Jmp("join", p)
		join := b.SwitchToNewBlock("join")
		q := b.BlockParam(join, TyBool, ClassGeneral)
		e := b.Ext(q, TyBool, TyU8, ClassGeneral)
		r := b.Add(x, e, TyU8, ClassAcc)
		b.Ret(r)
		locs := map[Reg]PhysLoc{x: reg8("A"), y: reg8("C"), p: {Kind: LocFlag, Name: "F"}, q: reg8("C"), e: reg8("C"), r: reg8("A")}
		flagCopyRun(t, m, locs, func(x, y byte) byte { return x + flagCopyHolds(cond, x, y) })
	}
}

// (b) Edge copy, A a source of a later move of the same parallel copy:
// jmp join(p, x) with join(q in C, w in E). A is dead after the edge, but the
// move A -> E is emitted after F -> C, so the predicate must not clobber A.
func TestNativeFlagEdgeCopyAccumulatorSourceOfSameCopy(t *testing.T) {
	for _, cond := range flagCopyConds {
		m := &Module{Name: "flagcopy"}
		f := m.AddFunc("flagfn")
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		b := NewBuilder(f)
		x := b.Param("x", TyU8, ClassAcc)
		y := b.Param("y", TyU8, ClassGeneral)
		b.SwitchToNewBlock("entry")
		p := b.Cmp(cond, x, y, ClassFlag, false)
		b.Jmp("join", p, x)
		join := b.SwitchToNewBlock("join")
		q := b.BlockParam(join, TyBool, ClassGeneral)
		w := b.BlockParam(join, TyU8, ClassGeneral)
		e := b.Ext(q, TyBool, TyU8, ClassGeneral)
		r := b.Add(w, e, TyU8, ClassAcc)
		b.Ret(r)
		locs := map[Reg]PhysLoc{x: reg8("A"), y: reg8("C"), p: {Kind: LocFlag, Name: "F"}, q: reg8("C"), w: reg8("E"), e: reg8("C"), r: reg8("A")}
		flagCopyRun(t, m, locs, func(x, y byte) byte { return x + flagCopyHolds(cond, x, y) })
	}
}

// (a)/(b)/(c) A comparison with a byte home copies its predicate at the
// comparison and keeps F for the branch that follows. Then-arm and else-arm
// results differ, so a branch on corrupted flags is caught, and the byte value
// itself is added in. With live, x (in A) is read after the copy.
func TestNativeFlagCmpByteHomeAccumulatorAndBranch(t *testing.T) {
	for _, live := range []bool{false, true} {
		for _, cond := range flagCopyConds {
			m := &Module{Name: "flagcopy"}
			f := m.AddFunc("flagfn")
			f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
			b := NewBuilder(f)
			x := b.Param("x", TyU8, ClassAcc)
			y := b.Param("y", TyU8, ClassGeneral)
			b.SwitchToNewBlock("entry")
			p := b.Cmp(cond, x, y, ClassFlag, false)
			b.BrIf(p, "yes", nil, "no", nil)
			b.SwitchToNewBlock("yes")
			e1 := b.Ext(p, TyBool, TyU8, ClassGeneral)
			var base1, base2 Reg
			if live {
				base1 = x
			} else {
				base1 = b.Const(10, TyU8, ClassGeneral)
			}
			r1 := b.Add(base1, e1, TyU8, ClassAcc)
			b.Ret(r1)
			b.SwitchToNewBlock("no")
			e2 := b.Ext(p, TyBool, TyU8, ClassGeneral)
			if live {
				base2 = x
			} else {
				base2 = b.Const(20, TyU8, ClassGeneral)
			}
			r2 := b.Sub(base2, e2, TyU8, ClassAcc)
			b.Ret(r2)
			locs := map[Reg]PhysLoc{x: reg8("A"), y: reg8("C"), p: reg8("E"), e1: reg8("D"), e2: reg8("D"), r1: reg8("A"), r2: reg8("A")}
			if !live {
				locs[base1], locs[base2] = reg8("B"), reg8("B")
			}
			want := func(x, y byte) byte {
				h := flagCopyHolds(cond, x, y)
				switch {
				case live && h == 1:
					return x + 1
				case live:
					return x
				case h == 1:
					return 11
				default:
					return 20
				}
			}
			asm := flagCopyRun(t, m, locs, want)
			if body := funcText(asm, "flagfn"); !live && strings.Contains(body, "PUSH") {
				t.Fatalf("cond %v: dead A saved:\n%s", cond, body)
			}
		}
	}
}
