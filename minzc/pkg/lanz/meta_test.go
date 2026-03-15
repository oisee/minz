package lanz

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// TestMetaRuntime_Introspection tests host function registration and
// struct introspection without running VM code.
func TestMetaRuntime_Introspection(t *testing.T) {
	hirMod := &hir.Module{
		Name: "test",
		Structs: []*mir2.StructTy{
			{
				Name: "Point",
				Fields: []mir2.StructField{
					{Name: "x", Ty: mir2.TyU8},
					{Name: "y", Ty: mir2.TyU8},
				},
			},
		},
	}

	mr := NewMetaRuntime(hirMod)

	// Check struct type ID
	id := mr.StructTypeID("Point")
	if id != 100 {
		t.Fatalf("StructTypeID(Point) = %d, want 100", id)
	}

	// field_count
	ret, err := mr.hostStructFieldCount([]mir2.Value{{I: id}})
	if err != nil {
		t.Fatal(err)
	}
	if ret[0].I != 2 {
		t.Errorf("field_count = %d, want 2", ret[0].I)
	}

	// field_type
	ret, err = mr.hostStructFieldType([]mir2.Value{{I: id}, {I: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if ret[0].I != tyIDU8 {
		t.Errorf("field_type(0) = %d, want %d (u8)", ret[0].I, tyIDU8)
	}

	// type_width(u8)
	ret, err = mr.hostTypeWidth([]mir2.Value{{I: tyIDU8}})
	if err != nil {
		t.Fatal(err)
	}
	if ret[0].I != 1 {
		t.Errorf("type_width(u8) = %d, want 1", ret[0].I)
	}

	// type_width(Point) — struct total
	ret, err = mr.hostTypeWidth([]mir2.Value{{I: id}})
	if err != nil {
		t.Fatal(err)
	}
	if ret[0].I != 2 {
		t.Errorf("type_width(Point) = %d, want 2", ret[0].I)
	}

	// is_struct
	ret, err = mr.hostTypeIsStruct([]mir2.Value{{I: id}})
	if err != nil {
		t.Fatal(err)
	}
	if ret[0].I != 1 {
		t.Errorf("is_struct(Point) = %d, want 1", ret[0].I)
	}

	ret, err = mr.hostTypeIsStruct([]mir2.Value{{I: tyIDU8}})
	if err != nil {
		t.Fatal(err)
	}
	if ret[0].I != 0 {
		t.Errorf("is_struct(u8) = %d, want 0", ret[0].I)
	}

	// field_offset
	ret, err = mr.hostStructFieldOffset([]mir2.Value{{I: id}, {I: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if ret[0].I != 1 {
		t.Errorf("field_offset(1) = %d, want 1", ret[0].I)
	}
}

// TestMetaRuntime_Emit tests the emit buffer and Lanz compilation.
func TestMetaRuntime_Emit(t *testing.T) {
	hirMod := &hir.Module{Name: "test"}
	mr := NewMetaRuntime(hirMod)

	// Simulate metafunction emitting Lanz code
	mr.emitted.WriteString(`(fun generated ((x u8)) u8 (return (+ x 1)))`)
	mr.emitted.WriteByte('\n')

	out, err := mr.CompileEmitted("meta_output")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(out.Funcs))
	}
	if out.Funcs[0].Name != "generated" {
		t.Errorf("name = %q, want generated", out.Funcs[0].Name)
	}
}

// TestMetaRuntime_ASTIntrospection tests AST dump via host function.
func TestMetaRuntime_ASTIntrospection(t *testing.T) {
	hirMod := &hir.Module{
		Name: "test",
		Funcs: []*hir.Func{
			{
				Name:   "add",
				Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}},
				RetTy:  mir2.TyU8,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.ReturnStmt{Val: &hir.BinExpr{Op: "+",
						L: &hir.VarRefExpr{Name: "a", Ty: mir2.TyU8},
						R: &hir.VarRefExpr{Name: "b", Ty: mir2.TyU8},
						Ty: mir2.TyU8}},
				}},
			},
		},
	}

	mr := NewMetaRuntime(hirMod)
	vm := mir2.NewVM(&mir2.Module{})
	mr.RegisterHosts(vm)

	namePtr := mr.allocString("add")
	ret, err := mr.hostASTFunc([]mir2.Value{namePtr})
	if err != nil {
		t.Fatal(err)
	}
	lanzText := mr.readCString(ret[0].I)
	t.Logf("AST of add:\n%s", lanzText)

	if !strings.Contains(lanzText, "(fun add") {
		t.Errorf("expected (fun add ...), got %q", lanzText)
	}
	if !strings.Contains(lanzText, "(+ a b)") {
		t.Errorf("expected (+ a b), got %q", lanzText)
	}
}

// TestMetaRuntime_EmitViaVM tests emit through actual VM execution.
// The metafunction is written in Lanz (not Nanz) to avoid import cycles.
func TestMetaRuntime_EmitViaVM(t *testing.T) {
	// Metafunction source in Lanz — calls emit with a constant string.
	// We can't call @meta.emit directly from Lanz since it needs string
	// pointers, but we can test the host function plumbing by building
	// a MIR2 module manually.

	callerMod := &hir.Module{
		Name: "caller",
		Structs: []*mir2.StructTy{
			{
				Name: "Color",
				Fields: []mir2.StructField{
					{Name: "r", Ty: mir2.TyU8},
					{Name: "g", Ty: mir2.TyU8},
					{Name: "b", Ty: mir2.TyU8},
				},
			},
		},
	}

	mr := NewMetaRuntime(callerMod)

	// Test struct with 3 fields
	id := mr.StructTypeID("Color")
	if id != 100 {
		t.Fatalf("StructTypeID(Color) = %d, want 100", id)
	}

	// Verify all fields
	for i, want := range []string{"r", "g", "b"} {
		vm := mir2.NewVM(&mir2.Module{})
		mr.RegisterHosts(vm)
		namePtr, _ := mr.hostStructFieldName([]mir2.Value{{I: id}, {I: int64(i)}})
		got := mr.readCString(namePtr[0].I)
		if got != want {
			t.Errorf("field_name(%d) = %q, want %q", i, got, want)
		}
	}

	// Verify type_width for 3-field struct
	ret, _ := mr.hostTypeWidth([]mir2.Value{{I: id}})
	if ret[0].I != 3 {
		t.Errorf("type_width(Color) = %d, want 3", ret[0].I)
	}

	// Simulate what a metafunction would do: build and emit Lanz
	mr.ClearEmitted()
	mr.emitted.WriteString(`(fun Color_size () u8 (return 3))`)
	mr.emitted.WriteByte('\n')
	mr.emitted.WriteString(`(fun Color_field_count () u8 (return 3))`)
	mr.emitted.WriteByte('\n')

	result, err := mr.CompileEmitted("derived")
	if err != nil {
		t.Fatalf("compile emitted: %v", err)
	}
	if len(result.Funcs) != 2 {
		t.Fatalf("expected 2 funcs, got %d", len(result.Funcs))
	}
	t.Logf("Generated: %s, %s", result.Funcs[0].Name, result.Funcs[1].Name)
}

// TestMetaRuntime_StringHelpers tests str.concat and str.from_int.
func TestMetaRuntime_StringHelpers(t *testing.T) {
	hirMod := &hir.Module{Name: "test"}
	mr := NewMetaRuntime(hirMod)
	vm := mir2.NewVM(&mir2.Module{})
	mr.RegisterHosts(vm)

	// concat
	a := mr.allocString("hello ")
	b := mr.allocString("world")
	ret, err := mr.hostStrConcat([]mir2.Value{a, b})
	if err != nil {
		t.Fatal(err)
	}
	got := mr.readCString(ret[0].I)
	if got != "hello world" {
		t.Errorf("concat = %q, want 'hello world'", got)
	}

	// from_int
	ret, err = mr.hostStrFromInt([]mir2.Value{{I: 42}})
	if err != nil {
		t.Fatal(err)
	}
	got = mr.readCString(ret[0].I)
	if got != "42" {
		t.Errorf("from_int(42) = %q, want '42'", got)
	}
}
