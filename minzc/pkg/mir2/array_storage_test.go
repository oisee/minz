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
