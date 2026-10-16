package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"testing"
)

func TestMergeImportedStringsKindsAndIndices(t *testing.T) {
	target := &Module{Strings: []string{"packet", "other"}, StrKinds: []mir2.StringKind{mir2.StrSString, mir2.StrCString}}
	a, b, c := Addr("@mir2.str.0"), Addr("@mir2.str.1"), Addr("@mir2.str.2")
	imported := &Module{Strings: []string{"packet", "Z\n", "packet"}, StrKinds: []mir2.StringKind{mir2.StrCString, mir2.StrCString, mir2.StrCString}, Funcs: []*Func{{Name: "f", RetTy: mir2.TyPtr, Body: Blk(&IfStmt{Cond: &CondExpr{Cond: Bool(true), Then: a, Else: c, Ty: mir2.TyPtr}, Then: Blk(Ret(b))})}}}
	if e := MergeImportedStrings(target, imported); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(target.Strings, []string{"packet", "other", "packet", "Z\n"}) || a.Sym != "@mir2.str.2" || b.Sym != "@mir2.str.3" || c.Sym != "@mir2.str.2" {
		t.Fatal("encoding or duplicate mapping lost", target.Strings, a, b, c)
	}
	if target.StrKinds[0] != mir2.StrSString || target.StrKinds[2] != mir2.StrCString {
		t.Fatal("CString replaced length-prefixed literal")
	}
}
func TestMergeImportedStringsRejectsWithoutPartialUpdate(t *testing.T) {
	target := &Module{Strings: []string{"root"}, StrKinds: []mir2.StringKind{mir2.StrCString}}
	a, b := Addr("@mir2.str.0"), Addr("@mir2.str.99")
	imported := &Module{Strings: []string{"library"}, StrKinds: []mir2.StringKind{mir2.StrCString}, Funcs: []*Func{{Name: "f", RetTy: mir2.TyPtr, Body: Blk(Ret(&CondExpr{Cond: Bool(true), Then: a, Else: b, Ty: mir2.TyPtr}))}}}
	if e := MergeImportedStrings(target, imported); e == nil {
		t.Fatal("invalid source literal index accepted")
	}
	if a.Sym != "@mir2.str.0" || b.Sym != "@mir2.str.99" || len(target.Strings) != 1 {
		t.Fatal("failed merge partially rewrote source or target")
	}
}
