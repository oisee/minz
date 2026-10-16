package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestNativeLocalFrameCFGAndReturns(t *testing.T) {
	for _, shape := range []string{"early", "joined", "condret", "tail"} {
		for _, condType := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpUlt, mir2.CmpUgt, mir2.CmpUle} {
			m := &mir2.Module{}
			f := m.AddFunc("framed")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
			k := b.Const(128, mir2.TyU8, mir2.ClassGeneral)
			c := b.Cmp(condType, x, k, mir2.ClassFlag, false)
			locs := map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "C"}, k: {Kind: mir2.LocReg, Name: "B"}, c: {Kind: mir2.LocFlag, Name: "F"}}
			alloc := func(n int64) { p := b.Alloca(n); locs[p] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"} }
			ret := func(v int64) {
				r := b.Const(v, mir2.TyU8, mir2.ClassAcc)
				locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				b.Ret(r)
			}
			if shape == "condret" {
				alloc(3)
				// Allocate before the predicate; ADD HL,SP itself changes carry.
				cmp := b.Cur.Insts[1]
				b.Cur.Insts = append(b.Cur.Insts[:1], b.Cur.Insts[2:]...)
				b.Cur.Insts = append(b.Cur.Insts, cmp)
				r := b.Const(17, mir2.TyU8, mir2.ClassAcc)
				locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				b.Cur.Term = &mir2.TermCondRet{Cond: c, Vals: []mir2.Reg{r}, Then: "yes"}
				b.SwitchToNewBlock("yes")
				ret(23)
			} else if shape == "tail" {
				alloc(3)
				r := b.Call("callee", nil, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
				locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				b.Ret(r)
				g := m.AddFunc("callee")
				g.Contract.Returns = f.Contract.Returns
				gb := mir2.NewBuilder(g)
				gb.SwitchToNewBlock("entry")
				v := gb.Const(31, mir2.TyU8, mir2.ClassAcc)
				locs[v] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				gb.Ret(v)
			} else {
				b.BrIf(c, "yes", nil, "no", nil)
				b.SwitchToNewBlock("yes")
				alloc(2)
				if shape == "joined" {
					b.Jmp("join")
				} else {
					ret(23)
				}
				b.SwitchToNewBlock("no")
				alloc(2)
				if shape == "joined" {
					b.Jmp("join")
					b.SwitchToNewBlock("join")
					ret(29)
				} else {
					alloc(1)
					ret(17)
				}
			}
			asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
			for _, x := range []int{0, 127, 128, 129, 255} {
				take := x == 128
				switch condType {
				case mir2.CmpUlt:
					take = x < 128
				case mir2.CmpUgt:
					take = x > 128
				case mir2.CmpUle:
					take = x <= 128
				}
				want := byte(17)
				if take {
					want = 23
				}
				if shape == "joined" {
					want = 29
				}
				if shape == "tail" {
					want = 31
				}
				r := fix1Execute(t, fmt.Sprintf("ORG 0x8000\nLD SP,0xff00\nLD C,%d\nCALL framed\nDI\nHALT\n", x)+asm)
				if r.A != want || r.SP != 0xff00 {
					t.Fatalf("%s %v x%d got%d want%d SP%x\n%s", shape, condType, x, r.A, want, r.SP, asm)
				}
			}
		}
	}
}
func TestNativeLocalFrameStableLoopAndNoAllocationPath(t *testing.T) {
	for _, shape := range []string{"stable-loop", "no-allocation-return"} {
		m := &mir2.Module{}
		f := m.AddFunc("frame_path")
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
		locs := map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "C"}}
		if shape == "stable-loop" {
			p := b.Alloca(2)
			locs[p] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
			b.Jmp("loop")
			b.SwitchToNewBlock("loop")
		}
		k := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
		cond := b.Cmp(mir2.CmpEq, x, k, mir2.ClassFlag, false)
		locs[k] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"}
		locs[cond] = mir2.PhysLoc{Kind: mir2.LocFlag, Name: "F"}
		other := "allocated"
		if shape == "stable-loop" {
			other = "loop"
		}
		b.BrIf(cond, "exit", nil, other, nil)
		b.SwitchToNewBlock("exit")
		r := b.Const(7, mir2.TyU8, mir2.ClassAcc)
		locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
		b.Ret(r)
		if shape != "stable-loop" {
			b.SwitchToNewBlock("allocated")
			p := b.Alloca(3)
			locs[p] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
			r := b.Const(9, mir2.TyU8, mir2.ClassAcc)
			locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
			b.Ret(r)
		}
		asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
		inputs := []int{0}
		if shape != "stable-loop" {
			inputs = append(inputs, 1)
		}
		for _, x := range inputs {
			want := byte(7)
			if x != 0 {
				want = 9
			}
			r := fix1Execute(t, fmt.Sprintf("ORG 0x8000\nLD SP,0xff00\nLD C,%d\nCALL frame_path\nDI\nHALT\n", x)+asm)
			if r.A != want || r.SP != 0xff00 {
				t.Fatal(shape, x, r)
			}
		}
	}
}

func TestNativeLocalFrameRejectsUnequalJoinAndGrowingLoop(t *testing.T) {
	for _, shape := range []string{"join", "loop"} {
		m := &mir2.Module{}
		f := m.AddFunc("badframe")
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		p := b.Alloca(2)
		if shape == "loop" {
			b.Jmp("entry")
		} else {
			c := b.Const(1, mir2.TyBool, mir2.ClassFlag)
			b.BrIf(c, "yes", nil, "join", nil)
			b.SwitchToNewBlock("yes")
			b.Alloca(1)
			b.Jmp("join")
			b.SwitchToNewBlock("join")
			b.Ret()
		}
		_, e := mir2.Z80Codegen(m, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{p: {Kind: mir2.LocReg, Name: "HL"}}})
		if e == nil || !strings.Contains(e.Error(), "variable local frame depth") {
			t.Fatal(shape, e)
		}
	}
}

// A private local frame can coexist with fully known straight-line ASM that
// never addresses SP. Clobbered register values are distinct from the stored
// local object; reloading the object after ASM must still return its byte.
func TestNativeLocalFrameStraightLineASM(t *testing.T) {
	for _, template := range []string{"NOP", "LD A,31", "IN A,(C)"} {
		t.Run(template, func(t *testing.T) {
			m := &mir2.Module{}
			f := m.AddFunc("framed_asm")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			v := b.Param("value", mir2.TyU8, mir2.ClassAcc)
			ptr := b.Alloca(2)
			b.Store(ptr, v, mir2.TyU8)
			b.Asm(template, nil, nil, mir2.ClassSet(1<<mir2.ClassAcc), mir2.TyVoid, mir2.ClassGeneral)
			r := b.Load(ptr, mir2.TyU8, mir2.ClassAcc)
			b.Ret(r)
			a := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{v: {Kind: mir2.LocReg, Name: "A"}, ptr: {Kind: mir2.LocReg, Name: "HL"}, r: {Kind: mir2.LocReg, Name: "A"}}}
			asm := mustZ80Asm(mir2.Z80Codegen(m, a))
			for _, value := range []int{0, 1, 127, 128, 255} {
				r := fix1Execute(t, fmt.Sprintf("ORG 0x8000\nLD SP,0xff00\nLD A,%d\nCALL framed_asm\nDI\nHALT\n", value)+asm)
				if r.A != byte(value) || r.SP != 0xff00 {
					t.Fatalf("value=%d regs=%v\n%s", value, r, asm)
				}
			}
		})
	}
}

func TestNativeLocalFrameRejectsOpaqueOrSPASM(t *testing.T) {
	for _, template := range []string{"LD SP,0x5800", "PUSH HL\nPOP HL", "EX (SP),HL", "RET", "CALL helper", "JP done", "UNKNOWN"} {
		t.Run(template, func(t *testing.T) {
			m := &mir2.Module{}
			f := m.AddFunc("unsafe_asm")
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			p := b.Alloca(2)
			b.Asm(template, nil, nil, 0, mir2.TyVoid, mir2.ClassGeneral)
			b.Ret()
			_, e := mir2.Z80Codegen(m, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{p: {Kind: mir2.LocReg, Name: "HL"}}})
			if e == nil || !strings.Contains(e.Error(), "without SP access") {
				t.Fatalf("expected frame/ASM contract diagnostic, got %v", e)
			}
		})
	}
}
