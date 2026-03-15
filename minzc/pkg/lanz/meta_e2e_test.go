package lanz_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// TestMetaE2E_NanzToVM tests the complete metafunction pipeline:
// Nanz meta source → compile → MIR2 → VM execution with introspection → emit Lanz → HIR
func TestMetaE2E_NanzToVM(t *testing.T) {
	// The "caller" module has a struct Color{r,g,b} that we want to introspect.
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

	// Metafunction written in Nanz.
	// It uses @extern to declare host introspection functions,
	// then calls emit() with a hardcoded Lanz string.
	// (A real metafunction would build the string dynamically using
	// field_count/field_name/etc., but string building in Nanz is
	// limited to u8/u16 types, so we test with a constant for now.)
	metaSrc := `
@extern fun emit(ptr: u16) -> void
@extern fun struct_field_count(ty: u8) -> u8

fun meta_sizeof(ty: u8) -> u8 {
	let n = struct_field_count(ty)
	emit("(fun Color_size () u8 (return 3))")
	return n
}
`
	// Parse Nanz meta source → HIR
	metaHIR, err := nanz.Parse(metaSrc, "meta_sizeof.nanz")
	if err != nil {
		t.Fatalf("nanz.Parse: %v", err)
	}

	// HIR → MIR2 (direct lowering, no full pipeline needed for VM execution)
	mirMod := hir.LowerModule(metaHIR)

	// Create VM with the compiled meta module
	vm := mir2.NewVM(mirMod)

	// Register introspection host functions
	mr := lanz.NewMetaRuntime(callerMod)
	mr.RegisterHosts(vm)

	// Also register under the short names that @extern uses
	vm.Hosts["emit"] = vm.Hosts["@meta.emit"]
	vm.Hosts["struct_field_count"] = vm.Hosts["@meta.struct.field_count"]

	// Call the metafunction with Color's type ID
	tyID := mr.StructTypeID("Color")
	rets, err := vm.Call("meta_sizeof", []mir2.Value{{I: tyID}})
	if err != nil {
		t.Fatalf("VM.Call: %v", err)
	}

	// Check return value — field count should be 3
	if len(rets) > 0 && rets[0].I != 3 {
		t.Errorf("meta_sizeof returned %d, want 3", rets[0].I)
	}

	// Check emitted Lanz
	emitted := mr.Emitted()
	t.Logf("Emitted Lanz:\n%s", emitted)

	// Compile emitted Lanz → HIR
	result, err := mr.CompileEmitted("generated")
	if err != nil {
		t.Fatalf("CompileEmitted: %v", err)
	}

	if len(result.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(result.Funcs))
	}
	if result.Funcs[0].Name != "Color_size" {
		t.Errorf("func name = %q, want Color_size", result.Funcs[0].Name)
	}

	t.Logf("SUCCESS: Nanz meta → VM → emit Lanz → HIR function %q", result.Funcs[0].Name)
}
