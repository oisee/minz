package mir2

import (
	"fmt"
	"testing"
)

func TestCoalescePreservesRegisterConstraints(t *testing.T) {
	for _, cls := range []RegClass{ClassRegC, ClassAcc} {
		for _, hard := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/hard%v", cls, hard), func(t *testing.T) {
				f := &Func{Name: "constraints"}
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				src := b.Param("x", TyU8, ClassGeneral)
				dst := b.Move(src, TyU8, cls)
				b.Cur.Insts[len(b.Cur.Insts)-1].ClsHard = hard
				b.Ret(dst)
				name := "C"
				if cls == ClassAcc {
					name = "A"
				}
				original := PhysLoc{Kind: LocReg, Name: name}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{src: {Kind: LocMem, Name: "mem", Offset: 0xf000}, dst: original}}
				coalesceAllocResult(f, ar, ComputeLiveness(f))
				if cls == ClassRegC || hard {
					if ar.Locs[dst] != original {
						t.Fatalf("class=%v hard=%v: register constraint moved to %v", cls, hard, ar.Locs[dst])
					}
				}
			})
		}
	}
	// The ISA-output recoloring runs before copy affinities. It must honor
	// the destination's class too, even though the ALU computes through A.
	f := &Func{Name: "alu_constraints"}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	src := b.Param("x", TyU8, ClassGeneral)
	dst := b.Add(src, src, TyU8, ClassRegC)
	out := b.Move(dst, TyU8, ClassGeneral)
	b.Ret(out)
	c := PhysLoc{Kind: LocReg, Name: "C"}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{src: {Kind: LocReg, Name: "B"}, dst: c, out: c}}
	coalesceAllocResult(f, ar, ComputeLiveness(f))
	if ar.Locs[dst] != c {
		t.Fatalf("register-only ALU result recolored to %v", ar.Locs[dst])
	}
}
