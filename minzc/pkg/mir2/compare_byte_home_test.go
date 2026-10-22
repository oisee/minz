package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// Cost tables permit a flag-class comparison to live in a byte or spill.
// Force each home so coalescing/allocator heuristics cannot hide the contract.
func TestComparisonPublishesAllocatedByteHome(t *testing.T) {
	for _, home := range []mir2.PhysLoc{{Kind: mir2.LocReg, Name: "A"}, {Kind: mir2.LocReg, Name: "B"}, {Kind: mir2.LocReg, Name: "C"}, {Kind: mir2.LocReg, Name: "D"}, {Kind: mir2.LocReg, Name: "E"}, {Kind: mir2.LocReg, Name: "H"}, {Kind: mir2.LocReg, Name: "L"}, {Kind: mir2.LocMem, Name: "mem"}} {
		for _, cond := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUle, mir2.CmpUgt, mir2.CmpUge, mir2.CmpLt, mir2.CmpLe, mir2.CmpGt, mir2.CmpGe} {
			t.Run(home.Name+"/"+cond.String(), func(t *testing.T) {
				m := &mir2.Module{}
				f := m.AddFunc("compare_value")
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassAcc}, {Ty: mir2.TyU8, Class: mir2.ClassCounter}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				ty := mir2.TyU8
				signed := cond == mir2.CmpLt || cond == mir2.CmpLe || cond == mir2.CmpGt || cond == mir2.CmpGe
				if signed {
					ty = mir2.TyI8
				}
				x := b.Param("x", ty, mir2.ClassGeneral)
				k := b.Const(127, ty, mir2.ClassGeneral)
				cmp := b.CmpWithSrcTy(cond, x, k, mir2.ClassFlag, false, ty)
				b.Ret(cmp, x)
				xhome, khome := "A", "E"
				if home.Name == "A" {
					xhome = "C"
				}
				if home.Name == "E" {
					khome = "D"
				}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: xhome}, k: {Kind: mir2.LocReg, Name: khome}, cmp: home}}
				asm, e := mir2.Z80Codegen(m, ar)
				if e != nil {
					t.Fatal(e)
				}
				if !strings.Contains(asm, ".bool_true_") {
					t.Fatalf("missing byte materialization:\n%s", asm)
				}
				program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL compare_value\nHALT\n" + asm)
				if e != nil || len(program.Errors) > 0 {
					t.Fatalf("assemble %v %v", e, program.Errors)
				}
				vm := mir2.NewVM(m)
				for xval := 0; xval < 256; xval++ {
					v := int64(xval)
					if signed {
						v = int64(int8(xval))
					}
					want := false
					switch cond {
					case mir2.CmpEq:
						want = v == 127
					case mir2.CmpNe:
						want = v != 127
					case mir2.CmpLt, mir2.CmpUlt:
						want = v < 127
					case mir2.CmpLe, mir2.CmpUle:
						want = v <= 127
					case mir2.CmpGt, mir2.CmpUgt:
						want = v > 127
					case mir2.CmpGe, mir2.CmpUge:
						want = v >= 127
					}
					expected := int64(0)
					if want {
						expected = 1
					}
					values, e := vm.CallFunc(f, []mir2.Value{{I: v}})
					if e != nil || values[0].I != expected {
						t.Fatalf("VM x=%d values=%v err=%v", xval, values, e)
					}
					z := emulator.NewRemogattoZ80()
					copy(z.Memory()[0x8000:], program.Binary)
					regs := emulator.Registers{PC: 0x8000, SP: 0xff00, A: byte(xval)}
					if xhome == "C" {
						regs.BC = uint16(xval)
					}
					z.SetRegisters(regs)
					for n := 0; n < 500 && !z.IsHalted(); n++ {
						z.Step()
					}
					got := z.GetRegisters()
					if !z.IsHalted() || int64(got.A) != expected || got.BC>>8 != uint16(xval) || got.SP != 0xff00 {
						t.Fatal(fmt.Sprintf("x=%d want=%d raw=%d got=%+v\n%s", xval, expected, xval, got, asm))
					}
				}
			})
		}
	}
}
