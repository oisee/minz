package mir2

import (
	"math"
	"testing"
)

func TestVMMemoryBoundsWithoutOverflow(t *testing.T) {
	for _, op := range []Op{OpLoad, OpStore, OpLoadPatched, OpPatch} {
		for _, addr := range []int64{-1, math.MaxInt64, math.MaxInt64 - 1, 16, 15, 14, 0} {
			m := &Module{}
			f := m.AddFunc("bounds")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			zero := b.Const(0, PtrFor(64), ClassPointer)
			delta := b.Const(addr, PtrFor(64), ClassGeneral)
			p := f.AllocReg()
			f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: OpPtrAdd, Dst: p, Src: [2]Reg{zero, delta}, Ty: PtrFor(64)})
			v := b.Const(0x1234, TyU16, ClassGeneral)
			dst := NoReg
			if op == OpLoad || op == OpLoadPatched {
				dst = f.AllocReg()
			}
			f.Blocks[0].Insts = append(f.Blocks[0].Insts, &Inst{Op: op, Dst: dst, Src: [2]Reg{p, v}, Ty: TyU16})
			b.Ret()
			vm := NewVM(m)
			vm.EnsureHeap(16)
			_, err := vm.Call(f.Name, nil)
			valid := addr == 0 || addr == 14
			if (err == nil) != valid {
				t.Fatalf("%v address %d: error %v, valid %v", op, addr, err, valid)
			}
		}
	}
}
