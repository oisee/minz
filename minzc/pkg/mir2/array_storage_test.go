package mir2

import "testing"

func TestArrayStorageUsesAddressableElementBytes(t *testing.T) {
	for _, tc := range []struct {
		elem          Ty
		length, bytes int
	}{{TyBool, 101, 101}, {TyU8, 101, 101}, {TyU16, 3, 6}, {TyU24, 3, 9}, {&ArrayTy{Elem: TyBool, Len: 3}, 2, 6}} {
		array := &ArrayTy{Elem: tc.elem, Len: tc.length}
		if got := ByteWidth(array); got != tc.bytes {
			t.Fatalf("%s storage=%d want=%d", array, got, tc.bytes)
		}
	}
	if TyBool.Width() != 1 || ByteWidth(TyBool) != 1 {
		t.Fatal("bool significant width/storage changed")
	}
}

func TestStructStorageUsesAddressableFieldBytes(t *testing.T) {
	fields := []StructField{{Name: "a", Ty: TyBool}, {Name: "b", Ty: TyBool}, {Name: "c", Ty: TyBool}, {Name: "value", Ty: TyU8}}
	st := &StructTy{Fields: fields}
	if ByteWidth(st) != 4 {
		t.Fatalf("struct storage=%d want4", ByteWidth(st))
	}
	for i := range fields {
		if st.ByteOffset(i) != i {
			t.Fatalf("field%d offset%d", i, st.ByteOffset(i))
		}
	}
	nested := &StructTy{Fields: []StructField{{Name: "flags", Ty: st}, {Name: "tail", Ty: TyBool}}}
	if ByteWidth(nested) != 5 || nested.ByteOffset(1) != 4 {
		t.Fatal("nested struct storage mismatch")
	}
	union := &StructTy{IsUnion: true, Fields: []StructField{{Name: "flags", Ty: st}, {Name: "bit", Ty: TyBool}}}
	if ByteWidth(union) != 4 || union.ByteOffset(1) != 0 {
		t.Fatal("union byte storage mismatch")
	}
}
