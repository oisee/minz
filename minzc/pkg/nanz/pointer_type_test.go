package nanz

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestPointeeTypeSurvivesSourceRoundTrip(t *testing.T) {
	src := `type WordPtr = ^u16
fun read(p: WordPtr) -> u16 {
 let alias = p
 return alias[1]
}`
	m, err := Parse(src, "pointer.nanz")
	if err != nil {
		t.Fatal(err)
	}
	f := m.FuncByName("read")
	if mir2.PointerElem(f.Params[0].Ty) != mir2.TyU16 {
		t.Fatal("parameter pointee erased")
	}
	if ret := f.Body.Body[1].(*hir.ReturnStmt).Val.(*hir.IndexExpr); ret.ElemTy != mir2.TyU16 {
		t.Fatal("wrong index element")
	}
	// Print retains ^u16 rather than silently lowering the declaration to ptr.
	printed := Print(m)
	again, err := Parse(printed, "roundtrip.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if mir2.PointerElem(again.FuncByName("read").Params[0].Ty) != mir2.TyU16 {
		t.Fatal("roundtrip erased pointee")
	}
}

func TestPointerRejectsUnknownPointee(t *testing.T) {
	_, err := Parse("fun read(p: ^MisspelledType) -> u8 { return p^ }", "pointer.nanz")
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("%v", err)
	}
}
