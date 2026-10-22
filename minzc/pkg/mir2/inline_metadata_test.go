package mir2

import "testing"

func TestInlinePreservesAbsentReturnMetadata(t *testing.T) {
	for _, op := range []Op{OpField, OpMove, OpPtrAdd} {
		i := &Inst{Op: op, Dst: 2, Src: [2]Reg{1}, Ty: TyPtr, Cls: ClassPointer}
		clone := remapInst(i, map[Reg]Reg{1: 3, 2: 4})
		if !cpmCleanLocalAddress(clone) {
			t.Fatalf("%s clone lost canonical pointer shape: %+v", op, clone)
		}
		if clone.Dst != 4 || clone.Src[0] != 3 {
			t.Fatal("unmapped pointer", clone)
		}
	}
	original := &Inst{Op: OpCall, Dst: 1, ExtraRets: []Reg{2, 3}}
	clone := remapInst(original, map[Reg]Reg{1: 4, 2: 5, 3: 6})
	if clone.Dst != 4 || clone.ExtraRets[0] != 5 || clone.ExtraRets[1] != 6 {
		t.Fatal("unmapped returns", clone)
	}
	clone.ExtraRets[0] = 7
	if original.ExtraRets[0] != 2 {
		t.Fatal("shared return metadata")
	}
}
