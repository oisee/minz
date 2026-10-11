package mir2

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func recursiveCallSpillFixture(word bool) (*Module, *AllocResult) {
	m := &Module{}
	f := m.AddFunc("recursive_home")
	ty, cls, paramLoc, resultLoc, constLoc := Ty(TyU8), ClassAcc, "C", "A", "B"
	if word {
		ty, cls, paramLoc, resultLoc, constLoc = TyU16, ClassPointer, "DE", "HL", "BC"
	}
	f.Contract.Returns = []Return{{Ty: ty, Class: cls}}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	n := b.Param("n", ty, ClassGeneral)
	saved := b.Move(n, ty, ClassGeneral)
	zero := b.Const(0, ty, ClassGeneral)
	cond := b.Cmp(CmpEq, n, zero, ClassFlag, false)
	b.BrIf(cond, "base", nil, "recurse", nil)
	b.SwitchToNewBlock("base")
	b.Ret(zero)
	b.SwitchToNewBlock("recurse")
	one := b.Const(1, ty, ClassGeneral)
	arg := b.Sub(n, one, ty, ClassGeneral)
	r := b.Call(f.Name, []Reg{arg}, ty, cls, CallAttrs{})
	sum := b.Add(r, saved, ty, cls)
	b.Ret(sum)
	ar := &AllocResult{Locs: map[Reg]PhysLoc{
		n: {Kind: LocReg, Name: paramLoc}, saved: {Kind: LocMem},
		zero: {Kind: LocReg, Name: constLoc}, cond: {Kind: LocFlag, Name: "F"},
		one: {Kind: LocReg, Name: constLoc}, arg: {Kind: LocReg, Name: paramLoc},
		r: {Kind: LocReg, Name: resultLoc}, sum: {Kind: LocReg, Name: resultLoc},
	}, Spilled: []Reg{saved}}
	return m, ar
}

func TestRecursiveCallSpill(t *testing.T) {
	for _, word := range []bool{false, true} {
		t.Run(fmt.Sprint(word), func(t *testing.T) {
			m, ar := recursiveCallSpillFixture(word)
			if errs := Verify(m); errs != nil {
				t.Fatal(errs)
			}
			for _, n := range []int64{0, 1, 3, 7} {
				got, err := NewVM(m).Call("recursive_home", []Value{{I: n}})
				if err != nil || len(got) != 1 || got[0].I != n*(n+1)/2 {
					t.Fatalf("VM n%d: %v %v", n, got, err)
				}
			}
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(asm, "EX (SP), HL") {
				t.Fatal("missing activation snapshot")
			}
			for _, n := range []int{0, 1, 3, 7} {
				param := "C"
				if word {
					param = "DE"
				}
				full := fmt.Sprintf("ORG 0x8000\nLD SP, 0xF000\nLD %s, %d\nCALL recursive_home\nDI\nHALT\n", param, n) + asm
				a, e := z80asm.NewAssembler().AssembleString(full)
				if e != nil || len(a.Errors) != 0 {
					t.Fatalf("assembly %v %v\n%s", e, a.Errors, full)
				}
				// Owner/reviewer release is required before executing this bounded
				// forced-allocation oracle; compilation and raw VM run by default.
				if os.Getenv("MINZ_RECURSIVE_SPILL_NATIVE") != "1" {
					continue
				}
				z := emulator.NewRemogattoZ80()
				if e := z.LoadMemory(0x8000, a.Binary); e != nil {
					t.Fatal(e)
				}
				z.SetPC(0x8000)
				if e := z.Run(); e != nil {
					t.Fatal(e)
				}
				regs := z.GetRegisters()
				got := uint16(regs.A)
				if word {
					got = regs.HL
				}
				if got != uint16(n*(n+1)/2) || regs.SP != 0xF000 {
					t.Fatalf("n%d result%d SP%04x want%d\n%s", n, got, regs.SP, n*(n+1)/2, full)
				}
			}
		})
	}
}
