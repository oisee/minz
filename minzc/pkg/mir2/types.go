// Package mir2 is a clean, target-independent intermediate representation
// for the MinZ compiler. It replaces MIR1 (pkg/ir) which mixed three
// abstraction levels in a single 118-opcode enum.
//
// MIR2 design principles:
//   - ~30 core opcodes (everything else is intrinsic calls)
//   - Explicit basic blocks with terminators (no flat label+jump list)
//   - Register classes instead of Z80-specific hints
//   - 3 target-independent SMC primitives (patch_slot / load_patched / patch)
//   - Typed every instruction; no implicit conversions
//   - Multi-return tuples; flag-class bools (no materialised CY register)
//   - Comptime attribute for @-functions; host-call attribute for @emit
package mir2

import "fmt"

// Ty is the type of a MIR2 value.
// All types are immutable singleton pointers — use == for equality.
type Ty interface {
	// Width returns storage width in bits. 0 for void.
	Width() int
	String() string
	isTy() // sealed: only types in this package implement Ty
}

// ── Primitive singletons ──────────────────────────────────────────────────────

var (
	TyVoid = &voidTy{}

	// Unsigned integers
	TyU8  = &intTy{bits: 8, signed: false, name: "u8"}
	TyU16 = &intTy{bits: 16, signed: false, name: "u16"}
	TyU24 = &intTy{bits: 24, signed: false, name: "u24"}
	TyU32 = &intTy{bits: 32, signed: false, name: "u32"}

	// Signed integers
	TyI8  = &intTy{bits: 8, signed: true, name: "i8"}
	TyI16 = &intTy{bits: 16, signed: true, name: "i16"}
	TyI24 = &intTy{bits: 24, signed: true, name: "i24"}
	TyI32 = &intTy{bits: 32, signed: true, name: "i32"}

	// Boolean: 1-bit semantic, stored as u8 unless allocated to ClassFlag.
	// When allocated to ClassFlag the backend uses a CPU flag register directly.
	TyBool = &intTy{bits: 1, signed: false, name: "bool"}

	// Pointer: target-word-wide address.
	// Width() returns 16 (Z80 default); backends may override via PtrWidth.
	TyPtr = &ptrTy{width: 16}

	// ── Fixed-point (Q notation) ──────────────────────────────────────────────
	// Format: f<integer_bits>.<fractional_bits>
	// Total width = integer + fractional bits.
	// All are unsigned unless prefixed with 'q' (planned).
	//
	// Common uses on Z80/retro:
	//   TyF8_8  — 16-bit total, common for angles/fractions in demos
	//   TyF8_16 — 24-bit, high-precision fixed (eZ80, Agon)
	//   TyF16_8 — 24-bit, large range + 8-bit fraction
	//   TyF16_16 — 32-bit, high precision (4-register on Z80, expensive)

	TyF0_8  = &fixedTy{intBits: 0, fracBits: 8}  // pure fraction .8  (0.0 – 0.996)
	TyF0_16 = &fixedTy{intBits: 0, fracBits: 16} // pure fraction .16 (0.0 – 0.99998)
	TyF8_8  = &fixedTy{intBits: 8, fracBits: 8}  // 0..255 + 1/256 steps
	TyF8_16 = &fixedTy{intBits: 8, fracBits: 16} // 0..255 + 1/65536 steps (24-bit)
	TyF16_8 = &fixedTy{intBits: 16, fracBits: 8} // 0..65535 + 1/256 steps (24-bit)
	TyF16_16 = &fixedTy{intBits: 16, fracBits: 16} // 0..65535 + high-precision (32-bit)
)

// ── Void ─────────────────────────────────────────────────────────────────────

type voidTy struct{}

func (t *voidTy) Width() int     { return 0 }
func (t *voidTy) String() string { return "void" }
func (t *voidTy) isTy()          {}

// ── Integer (covers bool too) ─────────────────────────────────────────────────

type intTy struct {
	bits   int
	signed bool
	name   string
}

func (t *intTy) Width() int     { return t.bits }
func (t *intTy) String() string { return t.name }
func (t *intTy) IsSigned() bool { return t.signed }
func (t *intTy) isTy()          {}

// ── Pointer ───────────────────────────────────────────────────────────────────

type ptrTy struct {
	width int // 16 = Z80, 24 = eZ80/Agon, 32 = x86, 64 = x86-64
}

func (t *ptrTy) Width() int     { return t.width }
func (t *ptrTy) String() string { return "ptr" }
func (t *ptrTy) isTy()          {}

// PtrFor returns a pointer type sized for the given target word width in bits.
// Common: PtrFor(16)=Z80, PtrFor(24)=eZ80, PtrFor(32)=x86, PtrFor(64)=x86-64.
func PtrFor(wordBits int) Ty { return &ptrTy{width: wordBits} }

// ── Fixed-point ───────────────────────────────────────────────────────────────

// fixedTy is a fixed-point number: intBits integer part + fracBits fractional part.
// All arithmetic is done in the underlying integer representation;
// the fixed-point interpretation is purely semantic.
// Backends implement fixed-point mul/div via shift sequences.
type fixedTy struct {
	intBits  int // integer bits (may be 0 for pure-fractional types)
	fracBits int // fractional bits
}

func (t *fixedTy) Width() int { return t.intBits + t.fracBits }
func (t *fixedTy) String() string {
	if t.intBits == 0 {
		return fmt.Sprintf("f.%d", t.fracBits)
	}
	return fmt.Sprintf("f%d.%d", t.intBits, t.fracBits)
}
func (t *fixedTy) isTy()         {}
func (t *fixedTy) IntBits() int  { return t.intBits }
func (t *fixedTy) FracBits() int { return t.fracBits }

// IsFixed reports whether ty is a fixed-point type.
func IsFixed(ty Ty) bool {
	_, ok := ty.(*fixedTy)
	return ok
}

// ── Tuple (multi-return) ──────────────────────────────────────────────────────

// TupleTy is a product type used for multi-return: (T1 [class], T2 [class], ...).
// Single-element tuples are unwrapped by NewTuple.
type TupleTy struct {
	Elems   []Ty
	Classes []RegClass // parallel slice — class for each element
}

func (t *TupleTy) Width() int {
	w := 0
	for _, e := range t.Elems {
		w += e.Width()
	}
	return w
}

func (t *TupleTy) String() string {
	if len(t.Elems) == 0 {
		return "()"
	}
	s := "("
	for i, e := range t.Elems {
		if i > 0 {
			s += ", "
		}
		s += e.String()
		if len(t.Classes) > i {
			s += fmt.Sprintf(" [%s]", t.Classes[i])
		}
	}
	return s + ")"
}

func (t *TupleTy) isTy() {}

// NewTuple returns a tuple type. A single element is returned unwrapped.
func NewTuple(elems []Ty, classes []RegClass) Ty {
	if len(elems) == 1 {
		return elems[0]
	}
	return &TupleTy{Elems: elems, Classes: classes}
}

// ── Array ─────────────────────────────────────────────────────────────────────

// ArrayLayout controls the memory arrangement of [N]Struct arrays.
// The default (LayoutAoS) is row-major, C-style.
// LayoutSoA and LayoutSoA256 are Z80-optimised columnar layouts.
//
// SoA256 (page-aligned Structure of Arrays):
//   - All column arrays start at consecutive 256-byte page boundaries ($C000, $C100, …)
//   - H register = column selector (field index) — INC H switches field (4T)
//   - L register = row selector (element index) — INC L advances, LD L,i is O(1)
//   - B register freed for DJNZ (no pointer pairs needed for three columns)
//   - Constraint: Len ≤ 256, global allocation only, Align = 256
//
// See ADR-0012 for full design rationale and PBQP page-assignment plan.
type ArrayLayout uint8

const (
	LayoutAoS    ArrayLayout = iota // row-major (default, C-style)
	LayoutSoA                       // columnar: one array per struct field, natural alignment
	LayoutSoA256                    // page-aligned SoA: H=field, L=index (ADR-0012)
)

func (l ArrayLayout) String() string {
	switch l {
	case LayoutAoS:
		return "aos"
	case LayoutSoA:
		return "soa"
	case LayoutSoA256:
		return "soa256"
	}
	return "unknown"
}

// ArrayTy is a fixed-size array [Len]Elem.
// Variable-length arrays are expressed as SliceTy (fat pointer) or ptr + length.
type ArrayTy struct {
	Elem   Ty
	Len    int
	Layout ArrayLayout // memory arrangement (default: LayoutAoS)
	Align  int         // alignment in bytes; 0 = natural; 256 = page-aligned (SoA256)
}

func (t *ArrayTy) Width() int { return t.Elem.Width() * t.Len }
func (t *ArrayTy) String() string {
	s := fmt.Sprintf("[%d]%s", t.Len, t.Elem.String())
	if t.Layout != LayoutAoS {
		s += "@" + t.Layout.String()
	}
	if t.Align > 0 {
		s += fmt.Sprintf("@align(%d)", t.Align)
	}
	return s
}
func (t *ArrayTy) isTy() {}

// NewArray returns an array type with the default AoS layout.
func NewArray(elem Ty, length int) *ArrayTy {
	return &ArrayTy{Elem: elem, Len: length}
}

// NewArraySoA256 returns a page-aligned columnar array type (ADR-0012).
// Panics if length > 256 — L register is 8-bit.
func NewArraySoA256(elem Ty, length int) *ArrayTy {
	if length > 256 {
		panic(fmt.Sprintf("NewArraySoA256: length %d > 256 (L register is 8-bit)", length))
	}
	return &ArrayTy{Elem: elem, Len: length, Layout: LayoutSoA256, Align: 256}
}

// ── Struct ────────────────────────────────────────────────────────────────────

// StructField is one named field within a StructTy.
type StructField struct {
	Name string
	Ty   Ty
}

// StructTy is a product type with named, packed fields.
// Field access lowers to pointer arithmetic at the byte offset of the field.
//
// Phase 3: AoS struct access via OpField(base, byte_offset).
// Phase 3+: SoA lowering splits [N]StructTy into N separate column arrays.
type StructTy struct {
	Name   string
	Fields []StructField
}

func (t *StructTy) Width() int {
	w := 0
	for _, f := range t.Fields {
		w += f.Ty.Width()
	}
	return w
}

func (t *StructTy) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "struct{...}"
}

func (t *StructTy) isTy() {}

// ByteOffset returns the byte offset of field i in a packed AoS layout.
func (t *StructTy) ByteOffset(i int) int {
	off := 0
	for j := 0; j < i && j < len(t.Fields); j++ {
		off += ByteWidth(t.Fields[j].Ty)
	}
	return off
}

// ── Slice ─────────────────────────────────────────────────────────────────────

// SliceTy is a fat pointer: (base_ptr, len) pair.
// Used for iterator-style array access — the natural model for Z80 where
// pointer bumping (INC HL, ADD HL,BC) is cheap but index multiply is not.
//
// On Z80: base_ptr → HL (16-bit), len → B (8-bit, DJNZ) or BC (16-bit).
// Phase 3: lowering creates a (ptr, count) slice from any ArrayTy.
type SliceTy struct {
	Elem Ty
}

func (t *SliceTy) Width() int     { return 32 } // 16-bit ptr + 16-bit len on Z80
func (t *SliceTy) String() string { return "[]" + t.Elem.String() }
func (t *SliceTy) isTy()          {}

// ── Ranged ────────────────────────────────────────────────────────────────────

// RangedTy is a base integer type annotated with a compile-time value range
// [Lo, Hi).  The range is inclusive-lo, exclusive-hi following Go convention.
//
// Example:  u8<0..64> → RangedTy{Base: TyU8, Lo: 0, Hi: 64}
//
// Semantics:
//   - Width(), IsSigned() etc. delegate to Base — the type is storage-identical
//     to its base type; no runtime overhead.
//   - The range annotation is used by the optimiser only:
//     • LUTGen detects single-ranged-param functions (range ≤ 256) and replaces
//       the call with a table-lookup.
//     • Future: bounds-check elimination for array indexing.
//   - Two RangedTy values are equal iff Base, Lo, and Hi all match.
type RangedTy struct {
	Base Ty
	Lo   int64 // inclusive lower bound
	Hi   int64 // exclusive upper bound  (Hi - Lo = range size)
}

func (t *RangedTy) Width() int     { return t.Base.Width() }
func (t *RangedTy) String() string { return fmt.Sprintf("%s<%d..%d>", t.Base, t.Lo, t.Hi-1) }
func (t *RangedTy) isTy()          {}

// NewRanged creates a ranged type.  hi is the exclusive upper bound (i.e. the
// value one past the last legal value — same convention as Go slices/ranges).
func NewRanged(base Ty, lo, hi int64) *RangedTy {
	return &RangedTy{Base: base, Lo: lo, Hi: hi}
}

// IsRanged reports whether ty is a ranged integer type.
func IsRanged(ty Ty) bool {
	_, ok := ty.(*RangedTy)
	return ok
}

// RangeOf returns the [Lo, Hi) bounds of ty.
// If ty is not a RangedTy it returns (0, 0, false).
func RangeOf(ty Ty) (lo, hi int64, ok bool) {
	if r, isR := ty.(*RangedTy); isR {
		return r.Lo, r.Hi, true
	}
	return 0, 0, false
}

// BaseOf unwraps a RangedTy and returns the underlying base type.
// For any other type it returns ty unchanged.
func BaseOf(ty Ty) Ty {
	if r, ok := ty.(*RangedTy); ok {
		return r.Base
	}
	return ty
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// ByteWidth returns the byte width of a type (Width()/8, rounded up).
func ByteWidth(ty Ty) int {
	w := ty.Width()
	if w == 0 {
		return 0
	}
	return (w + 7) / 8
}

// IsInt reports whether ty is an integer (including bool).
func IsInt(ty Ty) bool {
	_, ok := ty.(*intTy)
	return ok
}

// IsSigned reports whether ty is a signed integer type.
func IsSigned(ty Ty) bool {
	if it, ok := ty.(*intTy); ok {
		return it.signed
	}
	return false
}
