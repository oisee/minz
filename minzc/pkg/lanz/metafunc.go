package lanz

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// MetaFunc is a native (Go) compile-time metafunction.
// It receives introspection context and returns Lanz S-expression text
// that the compiler splices into the calling module.
type MetaFunc func(mr *MetaRuntime, args []MetaArg) (string, error)

// MetaArg carries compile-time information about a metafunction argument.
type MetaArg struct {
	StrVal string  // string literal (for "Debug", "Serialize", etc.)
	IntVal int64   // integer literal
	TypeID int64   // type ID (for struct/primitive type arguments)
	Name   string  // identifier name (variable or type name)
}

// BuiltinMetas is the registry of native metafunctions.
var BuiltinMetas = map[string]MetaFunc{
	"sizeof":        metaSizeof,
	"field_count":   metaFieldCount,
	"derive_debug":  metaDeriveDebug,
	"derive_sizeof": metaDeriveSizeof,
	"derive_eq":     metaDeriveEq,
}

// RunMeta executes a named metafunction and returns the generated HIR module.
func (mr *MetaRuntime) RunMeta(name string, args []MetaArg) (*hir.Module, error) {
	fn, ok := BuiltinMetas[name]
	if !ok {
		return nil, fmt.Errorf("unknown metafunction @%s", name)
	}
	lanzText, err := fn(mr, args)
	if err != nil {
		return nil, fmt.Errorf("@%s: %w", name, err)
	}
	if strings.TrimSpace(lanzText) == "" {
		return &hir.Module{Name: "meta_" + name}, nil
	}
	return Compile(lanzText, "meta_"+name)
}

// ── Built-in metafunctions ────────────────────────────────────────────────

// @sizeof(TypeName) → emits a constant function returning byte size.
func metaSizeof(mr *MetaRuntime, args []MetaArg) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("sizeof: need type name")
	}
	tyID := args[0].TypeID
	st := mr.findStruct(tyID)
	if st == nil {
		// Primitive type
		ty := idToTy(tyID)
		w := ty.Width() / 8
		if w == 0 {
			w = 1
		}
		return fmt.Sprintf("(fun sizeof_%s () u8 (return %d))", args[0].Name, w), nil
	}
	size := structSize(st)
	return fmt.Sprintf("(fun sizeof_%s () u8 (return %d))", st.Name, size), nil
}

// @field_count(TypeName) → emits a constant function returning field count.
func metaFieldCount(mr *MetaRuntime, args []MetaArg) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("field_count: need type name")
	}
	st := mr.findStruct(args[0].TypeID)
	if st == nil {
		return "", fmt.Errorf("field_count: %s is not a struct", args[0].Name)
	}
	return fmt.Sprintf("(fun field_count_%s () u8 (return %d))", st.Name, len(st.Fields)), nil
}

// @derive_debug(TypeName) → emits a function that prints struct fields.
// Generated: fun TypeName_debug(self: ptr) -> void
//   calls print_u8/print_u16 for each field with name prefix.
func metaDeriveDebug(mr *MetaRuntime, args []MetaArg) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("derive_debug: need type name")
	}
	st := mr.findStruct(args[0].TypeID)
	if st == nil {
		return "", fmt.Errorf("derive_debug: %s is not a struct", args[0].Name)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "(fun %s_debug ((self ptr)) void\n", st.Name)

	offset := 0
	for _, f := range st.Fields {
		w := fieldWidth(f.Ty)
		printer := "print_u8"
		loadTy := "u8"
		if w == 2 {
			printer = "print_u16"
			loadTy = "u16"
		}
		// Load field value: (load (+ (addr self) offset) ty)
		// Simplified: use field expr
		fmt.Fprintf(&sb, "  (%s (load (cast (+ (cast self u16) %d) ptr) %s))\n",
			printer, offset, loadTy)
		offset += w
	}
	sb.WriteString(")")
	return sb.String(), nil
}

// @derive_sizeof(TypeName) → emits sizeof function + per-field offset functions.
func metaDeriveSizeof(mr *MetaRuntime, args []MetaArg) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("derive_sizeof: need type name")
	}
	st := mr.findStruct(args[0].TypeID)
	if st == nil {
		return "", fmt.Errorf("derive_sizeof: %s is not a struct", args[0].Name)
	}

	var sb strings.Builder
	size := structSize(st)
	fmt.Fprintf(&sb, "(fun sizeof_%s () u8 (return %d))\n", st.Name, size)

	offset := 0
	for _, f := range st.Fields {
		fmt.Fprintf(&sb, "(fun offsetof_%s_%s () u8 (return %d))\n",
			st.Name, f.Name, offset)
		offset += fieldWidth(f.Ty)
	}
	return sb.String(), nil
}

// @derive_eq(TypeName) → emits an equality function comparing all fields.
// Generated: fun TypeName_eq(a: ptr, b: ptr) -> bool
func metaDeriveEq(mr *MetaRuntime, args []MetaArg) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("derive_eq: need type name")
	}
	st := mr.findStruct(args[0].TypeID)
	if st == nil {
		return "", fmt.Errorf("derive_eq: %s is not a struct", args[0].Name)
	}

	if len(st.Fields) == 0 {
		return fmt.Sprintf("(fun %s_eq ((a ptr) (b ptr)) bool (return true))", st.Name), nil
	}

	// Build nested AND of field comparisons
	var sb strings.Builder
	fmt.Fprintf(&sb, "(fun %s_eq ((a ptr) (b ptr)) bool\n", st.Name)

	// Chain: (== (load a+off ty) (load b+off ty)) AND ...
	offset := 0
	parts := []string{}
	for _, f := range st.Fields {
		w := fieldWidth(f.Ty)
		loadTy := "u8"
		if w == 2 {
			loadTy = "u16"
		}
		part := fmt.Sprintf("(== (load (cast (+ (cast a u16) %d) ptr) %s) (load (cast (+ (cast b u16) %d) ptr) %s))",
			offset, loadTy, offset, loadTy)
		parts = append(parts, part)
		offset += w
	}

	// Fold with AND: (& a (& b (& c d)))
	expr := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		expr = fmt.Sprintf("(& %s %s)", parts[i], expr)
	}
	fmt.Fprintf(&sb, "  (return %s))", expr)
	return sb.String(), nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

func structSize(st *mir2.StructTy) int {
	total := 0
	for _, f := range st.Fields {
		total += fieldWidth(f.Ty)
	}
	return total
}

func fieldWidth(ty mir2.Ty) int {
	w := ty.Width() / 8
	if w == 0 {
		w = 1
	}
	return w
}
