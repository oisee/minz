package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"strings"
	"testing"
)

func TestWideningByteAccumulateVMZ80(t *testing.T) {
	for _, preserve := range []bool{false, true} {
		m := &Module{}
		f := m.AddFunc("accumulate")
		b := NewBuilder(f)
		block := b.SwitchToNewBlock("entry")
		acc := b.Param("acc", TyU16, ClassPair)
		x := b.Param("x", TyU8, ClassGeneral)
		wide := b.Ext(x, TyU8, TyU16, ClassPair)
		sum := b.BinOp(OpAdd, acc, wide, TyU16, ClassPair)
		b.Ret(sum)
		if preserve {
			keep := b.Param("keep", TyU8, ClassAcc)
			block.Term = &TermRet{Vals: []Reg{sum, keep}}
		}
		f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
		ar := &AllocResult{Locs: map[Reg]PhysLoc{acc: {Name: "DE"}, x: {Name: "C"}, wide: {Name: "IX", Kind: LocIXY}, sum: {Name: "DE"}}}
		if preserve {
			ar.Locs[f.Contract.Params[2].Reg] = PhysLoc{Name: "A"}
		}
		var out strings.Builder
		g := &z80cg{sb: &out, fn: f, curBlock: block, ar: ar, regInfo: collectRegInfo(f), liveness: ComputeLiveness(f), constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, blockParamRegs: map[Reg]bool{}}
		g.genInst(block.Insts[0])
		g.curInstIdx = 1
		g.genInst(block.Insts[1])
		if !strings.Contains(out.String(), "widening byte add") || strings.Contains(out.String(), "IX") || strings.Contains(out.String(), "ADD HL") {
			t.Fatalf("word temporary survives:\n%s", out.String())
		}
		vm := NewVM(m)
		for _, accval := range []uint16{0, 1, 127, 255, 256, 32767, 65280, 65534, 65535} {
			for xval := 0; xval < 256; xval++ {
				want := accval + uint16(xval)
				args := []Value{{I: int64(accval)}, {I: int64(xval)}}
				if preserve {
					args = append(args, Value{I: 0x5a})
				}
				got, err := vm.Call(f.Name, args)
				z := runRAFragment(t, out.String(), emulator.Registers{DE: accval, BC: uint16(xval), HL: 0x9876, IX: 0x1234, A: 0x5a})
				if err != nil || got[0].I != int64(want) || z.DE != want || z.HL != 0x9876 || z.IX != 0x1234 || preserve && z.A != 0x5a {
					t.Fatalf("acc%04x x%02x: VM%v %v native%+v want%04x", accval, xval, got, err, z, want)
				}
			}
		}
	}

}
