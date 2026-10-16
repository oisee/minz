package mir2

import "testing"

func TestCallABIMetadataCloneOwnsNestedTypes(t *testing.T) {
	leaf := &StructTy{Name: "Leaf", Fields: []StructField{{Name: "value", Ty: TyU8}}}
	outer := &StructTy{Name: "Outer", FieldOffsets: []int{1}, StorageBytes: 3, Fields: []StructField{{Name: "nested", Ty: NewArray(leaf, 2)}}}
	original := &Inst{IndirectParamTys: []Ty{TyU16, TyPtr}, IndirectReturnTys: []Ty{outer}, StructReturnBuffers: []StructReturnBuffer{{ReturnIndex: 0, ParamIndex: 1, Ty: outer}}}
	clone := *original
	CloneCallABIMetadata(&clone, original)
	if clone.IndirectParamTys[0] != TyU16 || clone.IndirectParamTys[1] != TyPtr {
		t.Fatal("canonical primitive identity lost")
	}
	copied := clone.IndirectReturnTys[0].(*StructTy)
	if copied == outer || clone.StructReturnBuffers[0].Ty != copied {
		t.Fatal("composite ownership or internal alias lost")
	}
	copied.Name = "Changed"
	copied.FieldOffsets[0] = 99
	copied.StorageBytes = 99
	copied.Fields[0].Ty.(*ArrayTy).Elem.(*StructTy).Fields[0].Name = "changed"
	clone.IndirectParamTys[0] = TyU8
	clone.StructReturnBuffers[0].ParamIndex = 9
	if outer.Name != "Outer" || outer.FieldOffsets[0] != 1 || outer.StorageBytes != 3 || leaf.Fields[0].Name != "value" || original.IndirectParamTys[0] != TyU16 || original.StructReturnBuffers[0].ParamIndex != 1 {
		t.Fatal("clone mutated original semantic ABI")
	}
	recursive := &StructTy{Name: "Recursive"}
	recursive.Fields = []StructField{{Name: "next", Ty: &ptrTy{width: 16, elem: recursive}}}
	if CloneSemanticType(recursive) == recursive {
		t.Fatal("recursive type not copied")
	}
}
