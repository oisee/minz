package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// Keep every scratch byte live to force the predicate through its byte spill
// home, then store through each native pointer encoding. A remains live too.
func TestFlagStoreSpillFallbackPreservesLiveRegisters(t *testing.T) {
	for _, pointer := range []string{"HL", "BC", "DE", "IX", "IY"} {
		for _, truth := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", pointer, truth), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("flag_spill")
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
				var live []Reg
				var ptr Reg
				for _, loc := range []string{"A", "BC", "DE", "HL", "IX", "IY"} {
					ty := TyU16
					if loc == "A" {
						ty = TyU8
					}
					r := b.Param(loc, ty, ClassGeneral)
					ar.Locs[r] = PhysLoc{Kind: LocReg, Name: loc}
					live = append(live, r)
					if loc == pointer {
						ptr = r
					}
				}
				predicate := b.Cmp(CmpEq, live[0], live[0], ClassFlag, false)
				ar.Locs[predicate] = PhysLoc{Kind: LocFlag, Name: "F"}
				b.Store(ptr, predicate, TyBool)
				inst := b.Cur.Insts[len(b.Cur.Insts)-1]
				b.Ret(live...)
				var out strings.Builder
				g := &z80cg{mod: m, fn: f, ar: ar, sb: &out, curBlock: f.Entry(), liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}, pendingFlagReg: predicate, pendingAccReg: live[0]}
				g.genInst(inst)
				if g.err != nil {
					t.Fatal(g.err)
				}
				slot := g.spillLabel(predicate)
				if g.physOverride[predicate] != slot {
					t.Fatalf("predicate did not spill: %v", g.physOverride)
				}
				cp := 90
				if !truth {
					cp = 91
				}
				asm := fmt.Sprintf("ORG 0x8000\nentry:\nLD SP,0xf000\nLD A,90\nLD BC,0x1234\nLD DE,0x2345\nLD HL,0x3456\nLD IX,0x4567\nLD IY,0x5678\nLD %s,0x6200\nCP %d\n", pointer, cp) + out.String() + "DI\nHALT\n" + slot + ": DB 165\n"
				a, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil {
					t.Fatal(err, asm)
				}
				if len(a.Errors) > 0 {
					t.Fatal(a.Errors, asm)
				}
				z := emulator.NewRemogattoZ80()
				if err := z.LoadMemory(0x8000, a.Binary); err != nil {
					t.Fatal(err)
				}
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000})
				for n := 0; n < 300 && !z.IsHalted(); n++ {
					z.Step()
				}
				want := byte(0)
				if truth {
					want = 1
				}
				regs := z.GetRegisters()
				pairs := map[string]uint16{"BC": regs.BC, "DE": regs.DE, "HL": regs.HL, "IX": regs.IX, "IY": regs.IY}
				expected := map[string]uint16{"BC": 0x1234, "DE": 0x2345, "HL": 0x3456, "IX": 0x4567, "IY": 0x5678}
				expected[pointer] = 0x6200
				if !z.IsHalted() || z.Memory()[0x6200] != want || z.Memory()[a.Symbols[slot]] != want || regs.A != 90 || regs.SP != 0xf000 {
					t.Fatalf("store/spill=%d/%d want%d regs=%+v\n%s", z.Memory()[0x6200], z.Memory()[a.Symbols[slot]], want, regs, asm)
				}
				for name, wantPair := range expected {
					if pairs[name] != wantPair {
						t.Fatalf("%s=%04x want%04x\n%s", name, pairs[name], wantPair, asm)
					}
				}
			})
		}
	}
}

// OpPtrAdd implicitly writes HL even when its allocated result is DE. A
// predicate saved in H must survive address formation until its byte store.
func TestFlagStoreAcrossPointerScratch(t *testing.T) {
	for _, truth := range []bool{false, true} {
		m := &Module{}
		f := m.AddFunc("ptr_scratch")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		ptr := b.Param("ptr", TyPtr, ClassPointer)
		offset := b.Param("offset", TyU16, ClassPair)
		x := b.Param("x", TyU8, ClassAcc)
		predicate := b.Cmp(CmpEq, x, x, ClassFlag, false)
		address := b.PtrAdd(ptr, offset, ClassPointer)
		add := b.Cur.Insts[len(b.Cur.Insts)-1]
		b.Store(address, predicate, TyBool)
		store := b.Cur.Insts[len(b.Cur.Insts)-1]
		b.Ret(x)
		ar := &AllocResult{Locs: map[Reg]PhysLoc{ptr: {Name: "IX"}, offset: {Name: "BC"}, x: {Name: "A"}, predicate: {Name: "F", Kind: LocFlag}, address: {Name: "DE"}}}
		var out strings.Builder
		g := &z80cg{mod: m, fn: f, ar: ar, sb: &out, curBlock: f.Entry(), liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}, pendingFlagReg: predicate, pendingAccReg: x}
		g.genInst(add)
		g.genInst(store)
		if g.err != nil {
			t.Fatal(g.err)
		}
		cp := 90
		if !truth {
			cp = 91
		}
		asm := fmt.Sprintf("ORG 0x8000\nentry:\nLD SP,0xf000\nLD A,90\nLD IX,0x6200\nLD BC,1\nCP %d\n", cp) + out.String() + "DI\nHALT\n"
		a, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil {
			t.Fatal(err, asm)
		}
		if len(a.Errors) > 0 {
			t.Fatal(a.Errors, asm)
		}
		z := emulator.NewRemogattoZ80()
		if e := z.LoadMemory(0x8000, a.Binary); e != nil {
			t.Fatal(e)
		}
		z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000})
		for n := 0; n < 300 && !z.IsHalted(); n++ {
			z.Step()
		}
		want := byte(0)
		if truth {
			want = 1
		}
		if !z.IsHalted() || z.Memory()[0x6201] != want || z.GetRegisters().A != 90 {
			t.Fatalf("truth=%v stored=%d want%d regs=%+v\n%s", truth, z.Memory()[0x6201], want, z.GetRegisters(), asm)
		}
	}
}

func TestFlagStoreRejectsStalePredicate(t *testing.T) {
	for _, otherPending := range []bool{false, true} {
		t.Run(fmt.Sprintf("otherPending=%v", otherPending), func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("stale_flags")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			ptr := b.Param("ptr", TyPtr, ClassPointer)
			x := b.Param("x", TyU8, ClassAcc)
			stale := b.Cmp(CmpEq, x, x, ClassFlag, false)
			pending := NoReg
			if otherPending {
				pending = b.Cmp(CmpNe, x, x, ClassFlag, false)
			}
			b.Store(ptr, stale, TyBool)
			store := b.Cur.Insts[len(b.Cur.Insts)-1]
			b.Ret()
			ar := &AllocResult{Locs: map[Reg]PhysLoc{ptr: {Name: "HL"}, x: {Name: "A"}, stale: {Kind: LocFlag, Name: "F"}}}
			if otherPending {
				ar.Locs[pending] = PhysLoc{Kind: LocFlag, Name: "F"}
			}
			var out strings.Builder
			g := &z80cg{mod: m, fn: f, ar: ar, sb: &out, curBlock: f.Entry(), liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}, pendingFlagReg: pending, pendingAccReg: NoReg}
			g.genInst(store)
			if g.err == nil || !strings.Contains(g.err.Error(), "stale flag") {
				t.Fatalf("expected stale flag store error, got %v; assembly:\n%s", g.err, out.String())
			}
			if out.Len() != 0 || g.pendingFlagReg != pending {
				t.Fatalf("stale store changed codegen state: pending=%v assembly=%s", g.pendingFlagReg, out.String())
			}
		})
	}
}
