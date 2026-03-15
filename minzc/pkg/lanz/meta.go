package lanz

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// MetaRuntime provides compile-time introspection and code emission for
// metafunctions running on mir2.VM.  Metafunctions read type/AST info via
// host functions and emit Lanz S-expressions that the compiler splices into
// the calling module.
type MetaRuntime struct {
	// Source module context (for introspection).
	HIR     *hir.Module
	Structs []*mir2.StructTy

	// Emit buffer — metafunctions append Lanz text here.
	emitted strings.Builder

	// VM instance (set by RegisterHosts).
	vm *mir2.VM
}

// NewMetaRuntime creates a runtime bound to the given HIR module context.
func NewMetaRuntime(hirMod *hir.Module) *MetaRuntime {
	return &MetaRuntime{
		HIR:     hirMod,
		Structs: hirMod.Structs,
	}
}

// RegisterHosts installs all @meta.* host functions on the given VM.
func (mr *MetaRuntime) RegisterHosts(vm *mir2.VM) {
	mr.vm = vm

	// ── Emit ──────────────────────────────────────────────────────────
	vm.Hosts["@meta.emit"] = mr.hostEmit

	// ── Type introspection ────────────────────────────────────────────
	vm.Hosts["@meta.type.width"] = mr.hostTypeWidth
	vm.Hosts["@meta.type.name"] = mr.hostTypeName
	vm.Hosts["@meta.type.is_struct"] = mr.hostTypeIsStruct

	// ── Struct introspection ──────────────────────────────────────────
	vm.Hosts["@meta.struct.field_count"] = mr.hostStructFieldCount
	vm.Hosts["@meta.struct.field_name"] = mr.hostStructFieldName
	vm.Hosts["@meta.struct.field_type"] = mr.hostStructFieldType
	vm.Hosts["@meta.struct.field_offset"] = mr.hostStructFieldOffset

	// ── AST introspection ─────────────────────────────────────────────
	vm.Hosts["@meta.ast.func"] = mr.hostASTFunc

	// ── String helpers ────────────────────────────────────────────────
	vm.Hosts["@meta.str.concat"] = mr.hostStrConcat
	vm.Hosts["@meta.str.from_int"] = mr.hostStrFromInt
}

// Emitted returns the accumulated Lanz text from all emit() calls.
func (mr *MetaRuntime) Emitted() string {
	return mr.emitted.String()
}

// ClearEmitted resets the emit buffer.
func (mr *MetaRuntime) ClearEmitted() {
	mr.emitted.Reset()
}

// CompileEmitted parses the accumulated Lanz emit buffer into HIR nodes.
// Returns the module's functions, globals, and structs.
func (mr *MetaRuntime) CompileEmitted(name string) (*hir.Module, error) {
	src := mr.emitted.String()
	if strings.TrimSpace(src) == "" {
		return &hir.Module{Name: name}, nil
	}
	return Compile(src, name)
}

// ── Host function implementations ─────────────────────────────────────────

// @meta.emit(ptr) — append NUL-terminated string at ptr to emit buffer.
func (mr *MetaRuntime) hostEmit(args []mir2.Value) ([]mir2.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("@meta.emit: need 1 arg (ptr)")
	}
	s := mr.readCString(args[0].I)
	mr.emitted.WriteString(s)
	mr.emitted.WriteByte('\n')
	return nil, nil
}

// allocString puts a NUL-terminated string on the VM heap, returns pointer Value.
func (mr *MetaRuntime) allocString(s string) mir2.Value {
	data := append([]byte(s), 0)
	return mr.vm.AllocHeap(data)
}

// readCString reads a NUL-terminated string from VM heap.
func (mr *MetaRuntime) readCString(addr int64) string {
	// Read in chunks to handle heap bounds.
	var sb strings.Builder
	for off := int64(0); ; off++ {
		data := mr.vm.ReadHeap(addr+off, 1)
		if data == nil || data[0] == 0 {
			break
		}
		sb.WriteByte(data[0])
	}
	return sb.String()
}

// ── Type introspection ────────────────────────────────────────────────────

// @meta.type.width(ty_id) → byte width (1 for u8, 2 for u16, etc.)
func (mr *MetaRuntime) hostTypeWidth(args []mir2.Value) ([]mir2.Value, error) {
	id := args[0].I
	if id >= 100 {
		// Struct — sum of field widths
		st := mr.findStruct(id)
		if st == nil {
			return []mir2.Value{{I: 0}}, nil
		}
		total := 0
		for _, f := range st.Fields {
			w := f.Ty.Width() / 8
			if w == 0 {
				w = 1
			}
			total += w
		}
		return []mir2.Value{{I: int64(total)}}, nil
	}
	ty := idToTy(id)
	w := ty.Width() / 8
	if w == 0 {
		w = 1
	}
	return []mir2.Value{{I: int64(w)}}, nil
}

// @meta.type.name(ty_id) → ptr to type name string
func (mr *MetaRuntime) hostTypeName(args []mir2.Value) ([]mir2.Value, error) {
	id := args[0].I
	if id >= 100 {
		st := mr.findStruct(id)
		if st != nil {
			return []mir2.Value{mr.allocString(st.Name)}, nil
		}
		return []mir2.Value{mr.allocString("?")}, nil
	}
	ty := idToTy(id)
	return []mir2.Value{mr.allocString(tyStr(ty))}, nil
}

// @meta.type.is_struct(ty_id) → 1 if struct, 0 otherwise
func (mr *MetaRuntime) hostTypeIsStruct(args []mir2.Value) ([]mir2.Value, error) {
	id := args[0].I
	if id >= 100 && mr.findStruct(id) != nil {
		return []mir2.Value{{I: 1}}, nil
	}
	return []mir2.Value{{I: 0}}, nil
}

// ── Struct introspection ──────────────────────────────────────────────────

func (mr *MetaRuntime) findStruct(tyID int64) *mir2.StructTy {
	idx := int(tyID) - 100
	if idx >= 0 && idx < len(mr.Structs) {
		return mr.Structs[idx]
	}
	// Also try by searching all structs by name isn't needed if we use IDs
	return nil
}

// @meta.struct.field_count(ty) → number of fields
func (mr *MetaRuntime) hostStructFieldCount(args []mir2.Value) ([]mir2.Value, error) {
	st := mr.findStruct(args[0].I)
	if st == nil {
		return []mir2.Value{{I: 0}}, nil
	}
	return []mir2.Value{{I: int64(len(st.Fields))}}, nil
}

// @meta.struct.field_name(ty, i) → ptr to field name
func (mr *MetaRuntime) hostStructFieldName(args []mir2.Value) ([]mir2.Value, error) {
	st := mr.findStruct(args[0].I)
	if st == nil {
		return []mir2.Value{mr.allocString("")}, nil
	}
	i := int(args[1].I)
	if i < 0 || i >= len(st.Fields) {
		return []mir2.Value{mr.allocString("")}, nil
	}
	return []mir2.Value{mr.allocString(st.Fields[i].Name)}, nil
}

// @meta.struct.field_type(ty, i) → type ID of field
func (mr *MetaRuntime) hostStructFieldType(args []mir2.Value) ([]mir2.Value, error) {
	st := mr.findStruct(args[0].I)
	if st == nil {
		return []mir2.Value{{I: 0}}, nil
	}
	i := int(args[1].I)
	if i < 0 || i >= len(st.Fields) {
		return []mir2.Value{{I: 0}}, nil
	}
	return []mir2.Value{{I: tyToID(st.Fields[i].Ty)}}, nil
}

// @meta.struct.field_offset(ty, i) → byte offset of field
func (mr *MetaRuntime) hostStructFieldOffset(args []mir2.Value) ([]mir2.Value, error) {
	st := mr.findStruct(args[0].I)
	if st == nil {
		return []mir2.Value{{I: 0}}, nil
	}
	i := int(args[1].I)
	if i < 0 || i >= len(st.Fields) {
		return []mir2.Value{{I: 0}}, nil
	}
	// Compute offset from field types
	offset := 0
	for j := 0; j < i; j++ {
		w := st.Fields[j].Ty.Width() / 8
		if w == 0 {
			w = 1
		}
		offset += w
	}
	return []mir2.Value{{I: int64(offset)}}, nil
}

// ── AST introspection ─────────────────────────────────────────────────────

// @meta.ast.func(name_ptr) → ptr to Lanz S-expression of function
func (mr *MetaRuntime) hostASTFunc(args []mir2.Value) ([]mir2.Value, error) {
	name := mr.readCString(args[0].I)
	for _, f := range mr.HIR.Funcs {
		if f.Name == name {
			var sb strings.Builder
			dumpFunc(&sb, f)
			return []mir2.Value{mr.allocString(sb.String())}, nil
		}
	}
	return []mir2.Value{mr.allocString("")}, nil
}

// ── String helpers ────────────────────────────────────────────────────────

// @meta.str.concat(a_ptr, b_ptr) → ptr to concatenated string
func (mr *MetaRuntime) hostStrConcat(args []mir2.Value) ([]mir2.Value, error) {
	a := mr.readCString(args[0].I)
	b := mr.readCString(args[1].I)
	return []mir2.Value{mr.allocString(a + b)}, nil
}

// @meta.str.from_int(n) → ptr to decimal string representation
func (mr *MetaRuntime) hostStrFromInt(args []mir2.Value) ([]mir2.Value, error) {
	s := fmt.Sprintf("%d", args[0].I)
	return []mir2.Value{mr.allocString(s)}, nil
}

// ── Type ID mapping ───────────────────────────────────────────────────────
//
// Primitive types use fixed IDs (0–9).  Struct types use 100+index.
// This encoding is internal to the MetaRuntime — metafunctions receive
// these IDs and pass them back to introspection host functions.

const (
	tyIDVoid  = 0
	tyIDU8    = 1
	tyIDU16   = 2
	tyIDI8    = 3
	tyIDI16   = 4
	tyIDBool  = 5
	tyIDPtr   = 6
)

func tyToID(ty mir2.Ty) int64 {
	switch ty {
	case mir2.TyVoid:
		return tyIDVoid
	case mir2.TyU8:
		return tyIDU8
	case mir2.TyU16:
		return tyIDU16
	case mir2.TyI8:
		return tyIDI8
	case mir2.TyI16:
		return tyIDI16
	case mir2.TyBool:
		return tyIDBool
	case mir2.TyPtr:
		return tyIDPtr
	default:
		return tyIDU8 // fallback
	}
}

func idToTy(id int64) mir2.Ty {
	switch id {
	case tyIDVoid:
		return mir2.TyVoid
	case tyIDU8:
		return mir2.TyU8
	case tyIDU16:
		return mir2.TyU16
	case tyIDI8:
		return mir2.TyI8
	case tyIDI16:
		return mir2.TyI16
	case tyIDBool:
		return mir2.TyBool
	case tyIDPtr:
		return mir2.TyPtr
	default:
		return mir2.TyU8
	}
}

// StructTypeID returns the type ID for a named struct (100 + index).
// Returns -1 if not found.
func (mr *MetaRuntime) StructTypeID(name string) int64 {
	for i, st := range mr.Structs {
		if st.Name == name {
			return int64(100 + i)
		}
	}
	return -1
}
