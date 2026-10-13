package mir2

import "testing"

func TestPointerPointeeTypes(t *testing.T) {
	for _, elem := range []Ty{TyU8, TyU16, TyI16, TyU32, TyPtr} {
		p := PointerTo(elem)
		if !IsPointer(p) || PointerElem(p) != elem || p.Width() != 16 {
			t.Fatalf("%v: %v", elem, p)
		}
		if PointerTo(elem) != p {
			t.Fatal("pointee types must be canonical for type equality")
		}
		if ClassOf(p) != ClassPointer {
			t.Fatalf("pointer class lost: %v", p)
		}
	}
	if PointerTo(TyU8) != TyPtr || PtrFor(16) != TyPtr {
		t.Fatal("legacy byte pointer identity changed")
	}
	if IsPointer(TyU16) || PointerElem(TyU16) != nil {
		t.Fatal("integer classified as pointer")
	}
}
