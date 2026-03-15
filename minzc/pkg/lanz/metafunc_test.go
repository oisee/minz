package lanz

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func makeTestRuntime() *MetaRuntime {
	return NewMetaRuntime(&hir.Module{
		Name: "test",
		Structs: []*mir2.StructTy{
			{
				Name: "Point",
				Fields: []mir2.StructField{
					{Name: "x", Ty: mir2.TyU8},
					{Name: "y", Ty: mir2.TyU8},
				},
			},
			{
				Name: "Color",
				Fields: []mir2.StructField{
					{Name: "r", Ty: mir2.TyU8},
					{Name: "g", Ty: mir2.TyU8},
					{Name: "b", Ty: mir2.TyU8},
				},
			},
			{
				Name: "Pos16",
				Fields: []mir2.StructField{
					{Name: "x", Ty: mir2.TyU16},
					{Name: "y", Ty: mir2.TyU16},
				},
			},
		},
	})
}

func TestMetaFunc_Sizeof(t *testing.T) {
	mr := makeTestRuntime()
	m, err := mr.RunMeta("sizeof", []MetaArg{{TypeID: 100, Name: "Point"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Name != "sizeof_Point" {
		t.Errorf("name = %q, want sizeof_Point", f.Name)
	}
	// Return value should be 2 (u8 + u8)
	ret := f.Body.Body[0].(*hir.ReturnStmt)
	lit := ret.Val.(*hir.IntLitExpr)
	if lit.Val != 2 {
		t.Errorf("sizeof_Point returns %d, want 2", lit.Val)
	}
}

func TestMetaFunc_Sizeof_Color(t *testing.T) {
	mr := makeTestRuntime()
	m, err := mr.RunMeta("sizeof", []MetaArg{{TypeID: 101, Name: "Color"}})
	if err != nil {
		t.Fatal(err)
	}
	ret := m.Funcs[0].Body.Body[0].(*hir.ReturnStmt)
	lit := ret.Val.(*hir.IntLitExpr)
	if lit.Val != 3 {
		t.Errorf("sizeof_Color returns %d, want 3", lit.Val)
	}
}

func TestMetaFunc_Sizeof_U16Fields(t *testing.T) {
	mr := makeTestRuntime()
	m, err := mr.RunMeta("sizeof", []MetaArg{{TypeID: 102, Name: "Pos16"}})
	if err != nil {
		t.Fatal(err)
	}
	ret := m.Funcs[0].Body.Body[0].(*hir.ReturnStmt)
	lit := ret.Val.(*hir.IntLitExpr)
	if lit.Val != 4 {
		t.Errorf("sizeof_Pos16 returns %d, want 4", lit.Val)
	}
}

func TestMetaFunc_FieldCount(t *testing.T) {
	mr := makeTestRuntime()
	m, err := mr.RunMeta("field_count", []MetaArg{{TypeID: 101, Name: "Color"}})
	if err != nil {
		t.Fatal(err)
	}
	ret := m.Funcs[0].Body.Body[0].(*hir.ReturnStmt)
	lit := ret.Val.(*hir.IntLitExpr)
	if lit.Val != 3 {
		t.Errorf("field_count_Color returns %d, want 3", lit.Val)
	}
}

func TestMetaFunc_DeriveSizeof(t *testing.T) {
	mr := makeTestRuntime()
	m, err := mr.RunMeta("derive_sizeof", []MetaArg{{TypeID: 100, Name: "Point"}})
	if err != nil {
		t.Fatal(err)
	}
	// Should generate: sizeof_Point, offsetof_Point_x, offsetof_Point_y
	if len(m.Funcs) != 3 {
		t.Fatalf("expected 3 funcs, got %d", len(m.Funcs))
	}
	names := []string{}
	for _, f := range m.Funcs {
		names = append(names, f.Name)
	}
	t.Logf("Generated: %v", names)
	// Check offset values
	for _, f := range m.Funcs {
		ret := f.Body.Body[0].(*hir.ReturnStmt)
		lit := ret.Val.(*hir.IntLitExpr)
		switch f.Name {
		case "sizeof_Point":
			if lit.Val != 2 {
				t.Errorf("sizeof_Point = %d, want 2", lit.Val)
			}
		case "offsetof_Point_x":
			if lit.Val != 0 {
				t.Errorf("offsetof_Point_x = %d, want 0", lit.Val)
			}
		case "offsetof_Point_y":
			if lit.Val != 1 {
				t.Errorf("offsetof_Point_y = %d, want 1", lit.Val)
			}
		}
	}
}

func TestMetaFunc_DeriveEq(t *testing.T) {
	mr := makeTestRuntime()
	lanzText, err := metaDeriveEq(mr, []MetaArg{{TypeID: 100, Name: "Point"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("derive_eq Lanz:\n%s", lanzText)
	// Should parse and contain Point_eq with 2 params
	m, err := Compile(lanzText, "test")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if m.Funcs[0].Name != "Point_eq" {
		t.Errorf("name = %q, want Point_eq", m.Funcs[0].Name)
	}
	if len(m.Funcs[0].Params) != 2 {
		t.Errorf("params = %d, want 2", len(m.Funcs[0].Params))
	}
}

func TestMetaFunc_DeriveDebug(t *testing.T) {
	mr := makeTestRuntime()
	lanzText, err := metaDeriveDebug(mr, []MetaArg{{TypeID: 100, Name: "Point"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("derive_debug Lanz:\n%s", lanzText)
	if !strings.Contains(lanzText, "Point_debug") {
		t.Error("expected Point_debug in output")
	}
	if !strings.Contains(lanzText, "print_u8") {
		t.Error("expected print_u8 calls for u8 fields")
	}
	// Should parse
	m, err := Compile(lanzText, "test")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if m.Funcs[0].Name != "Point_debug" {
		t.Errorf("name = %q, want Point_debug", m.Funcs[0].Name)
	}
}

func TestMetaFunc_SizeofPrimitive(t *testing.T) {
	mr := makeTestRuntime()
	m, err := mr.RunMeta("sizeof", []MetaArg{{TypeID: tyIDU16, Name: "u16"}})
	if err != nil {
		t.Fatal(err)
	}
	ret := m.Funcs[0].Body.Body[0].(*hir.ReturnStmt)
	lit := ret.Val.(*hir.IntLitExpr)
	if lit.Val != 2 {
		t.Errorf("sizeof_u16 = %d, want 2", lit.Val)
	}
}

func TestMetaFunc_UnknownMeta(t *testing.T) {
	mr := makeTestRuntime()
	_, err := mr.RunMeta("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown metafunction")
	}
}
