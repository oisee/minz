// Copyright 2021 The CC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cparse

import (
	"fmt"
	"sort"
	"strings"

	"modernc.org/sortutil"
	"modernc.org/token"
)

var (
	_ Type = (*ArrayType)(nil)
	_ Type = (*EnumType)(nil)
	_ Type = (*FunctionType)(nil)
	_ Type = (*InvalidType)(nil)
	_ Type = (*PointerType)(nil)
	_ Type = (*PredefinedType)(nil)
	_ Type = (*StructType)(nil)
	_ Type = (*UnionType)(nil)

	_ sort.Interface = values{}
)

var (
	// Invalid is a singleton representing an invalid/undetermined type.  Invalid
	// is comparable.
	Invalid Type = &InvalidType{}

	integerKinds = [maxKind]bool{
		Bool:      true,
		Char:      true,
		Enum:      true,
		Int128:    true,
		Int:       true,
		Long:      true,
		LongLong:  true,
		SChar:     true,
		Short:     true,
		UChar:     true,
		UInt128:   true,
		UInt:      true,
		ULong:     true,
		ULongLong: true,
		UShort:    true,
	}

	realFloatingPointKinds = [maxKind]bool{
		Decimal128: true,
		Decimal32:  true,
		Decimal64:  true,
		Double:     true,
		Float128:   true,
		Float128x:  true,
		Float16:    true,
		Float32:    true,
		Float32x:   true,
		Float64:    true,
		Float64x:   true,
		Float:      true,
		LongDouble: true,
	}

	complexKinds = [maxKind]bool{
		ComplexChar:       true,
		ComplexDouble:     true,
		ComplexFloat:      true,
		ComplexInt:        true,
		ComplexLong:       true,
		ComplexLongDouble: true,
		ComplexLongLong:   true,
		ComplexShort:      true,
		ComplexUInt:       true,
		ComplexUShort:     true,
	}

	correspondingRealKinds = [maxKind]Kind{
		ComplexChar:       Char,
		ComplexDouble:     Double,
		ComplexFloat:      Float,
		ComplexInt:        Int,
		ComplexLong:       Long,
		ComplexLongDouble: LongDouble,
		ComplexLongLong:   LongLong,
		ComplexShort:      Short,
		ComplexUInt:       UInt,
		ComplexUShort:     UShort,
	}

	// Keep Bool first and sorted by rank.
	intConvRanks = [maxKind]int{
		Bool:      1,
		Char:      2,
		SChar:     2,
		UChar:     2,
		Short:     3,
		UShort:    3,
		Int:       4,
		UInt:      4,
		Long:      5,
		ULong:     5,
		LongLong:  6,
		ULongLong: 6,
		Int128:    7,
		UInt128:   7,
	}

	realKinds       [maxKind]bool
	arithmeticKinds [maxKind]bool
)

func init() {
	for i, v := range integerKinds {
		realKinds[i] = realKinds[i] || v
	}
	for i, v := range realFloatingPointKinds {
		realKinds[i] = realKinds[i] || v
	}
	arithmeticKinds = realKinds
	for i, v := range complexKinds {
		arithmeticKinds[i] = arithmeticKinds[i] || v
	}
}

type Kind int

const (
	InvalidKind Kind = iota

	Array             // array
	Bool              // _Bool
	Char              // char
	ComplexChar       // _Complex char
	ComplexDouble     // _Complex double
	ComplexFloat      // _Complex float
	ComplexFloat16    // _Complex _Float16
	ComplexInt        // _Complex int
	ComplexLong       // _Complex long
	ComplexLongDouble // _Complex long double
	ComplexLongLong   // _Complex long long
	ComplexShort      // _Complex short
	ComplexUInt       // _Complex unsigned
	ComplexUShort     // _Complex unsigned short
	Decimal128        // _Decimal128
	Decimal32         // _Decimal32
	Decimal64         // _Decimal64
	Double            // double
	Enum              // enum
	Float             // float
	Float128          // _Float128
	Float128x         // _Float128x
	Float16           // _Float16
	Float32           // _Float32
	Float32x          // _Float32x
	Float64           // _Float64
	Float64x          // _Float64x
	Function          // function
	Int               // int
	Int128            // __int128
	Long              // long
	LongDouble        // long double
	LongLong          // long long
	Ptr               // pointer
	SChar             // signed char
	Short             // short
	Struct            // struct
	UChar             // unsigned char
	UInt              // unsigned
	UInt128           // unsigned __int128
	ULong             // unsigned long
	ULongLong         // unsigned long long
	UShort            // unsigned short
	Union             // union
	Void              // void

	maxKind
)

type typer struct{ typ Type }

func newTyper(t Type) typer { return typer{typ: t} }

// Type returns the type of a node or an *InvalidType type value, if the type
// is unknown/undetermined.
func (t typer) Type() Type {
	if t.typ != nil {
		return t.typ
	}

	return Invalid
}

// Type is the representation of a C type.
//
// The dynamic type of a Type is one of
//
//	*ArrayType
//	*EnumType
//	*FunctionType
//	*InvalidType
//	*PointerType
//	*PredefinedType
//	*StructType
//	*UnionType
type Type interface {
	// Align reports the minimum alignment required by a type.
	Align() int

	Attributes() *Attributes

	// Bitfield backlink to the bit fields a type comes from, if any.
	BitField() *Field

	// Decay returns a pointer to array element for array types, a pointer to a
	// function for function types and itself for all other type kinds.
	Decay() Type

	// FieldAlign reports the minimum alignment required by a type when it's used
	// in a struct/union.
	FieldAlign() int

	// IsCompatible reports type compatibility as defined in [0]6.2.7/1.
	IsCompatible(Type) bool

	// IsIncomplete reports whether the size of a type is not determined.
	IsIncomplete() bool

	// Kind reports the kind of a type.
	Kind() Kind

	// Pointer returns a pointer to a type.
	Pointer() Type

	// Size reports the size of a type in bytes. Incomplete or invalid types may
	// report a negative size.
	Size() int64

	// String produces a human readable representation of a type or an
	// approximation of the same. The particular form is not specified and
	// may change. Namely, the returned value is not suitable for directly
	// determining type identity.
	String() string

	// Typedef returns the associated typedef declarator of this type, if any.
	Typedef() *Declarator

	// Undecay reverses Decay() if the type is a pointer and was produced by
	// Decay() and the result was different than the Decay() receiver. Otherwise
	// Undecay() returns its receiver.
	Undecay() Type

	// VectorSize reports N from __attribute__((vector_size(N))). Valid if
	// > 0.
	VectorSize() int64

	clone() Type
	setAttr(*Attributes) Type
	setName(d *Declarator) Type
	str(b *strings.Builder, useTag bool) *strings.Builder
	isGenericAssociationCompatible(assoc Type) bool
	isIncompleteInner(map[Type]struct{}) bool
}

func mergeAttr(t Type, a *Attributes) (r Type, err error) {
	if a == nil {
		return t, nil
	}

	attr := t.Attributes()
	if attr == nil {
		return t.setAttr(a), nil
	}

	new, err := attr.merge(nil, a)
	if err != nil {
		return t, err
	}

	return t.setAttr(new), nil
}

type noBitField struct{}

// BitField implements Type.
func (noBitField) BitField() *Field { return nil }

type namer struct{ d *Declarator }

// Typedef implements Type
func (n namer) Typedef() *Declarator { return n.d }

type InvalidType struct{}

func (n *InvalidType) clone() Type {
	return n
}

// BitField implements Type.
func (n *InvalidType) BitField() *Field { return nil }

// Pointer implements Type.
func (n *InvalidType) Pointer() Type { return Invalid }

// setAttr implements Type.
func (n *InvalidType) setAttr(*Attributes) Type { return Invalid }

// VectorSize implements Type.
func (n *InvalidType) VectorSize() int64 { return -1 }

// Attributes implements Type.
func (n *InvalidType) Attributes() *Attributes { return nil }

// Align implements Type.
func (n *InvalidType) Align() int { return 1 }

// Decay implements Type.
func (n *InvalidType) Decay() Type { return n }

// Undecay implements Type.
func (n *InvalidType) Undecay() Type { return n }

// FieldAlign implements Type.
func (n *InvalidType) FieldAlign() int { return 1 }

// Name implements Type.
func (n *InvalidType) Typedef() *Declarator { return nil }

// setName implements Type.
func (n *InvalidType) setName(*Declarator) Type { return n }

func (n *InvalidType) IsCompatible(Type) bool                         { return false }
func (n *InvalidType) isGenericAssociationCompatible(assoc Type) bool { return n.IsCompatible(assoc) }

// String implements Type.
func (n *InvalidType) String() string { return "<invalid type>" }

func (n *InvalidType) str(b *strings.Builder, useTag bool) *strings.Builder {
	b.WriteString("<invalid type>")
	return b
}

// IsIncomplete implements Type.
func (n *InvalidType) IsIncomplete() bool { return n.isIncompleteInner(nil) }

func (n *InvalidType) isIncompleteInner(map[Type]struct{}) bool { return true }

// Kind implements Type.
func (n *InvalidType) Kind() Kind { return InvalidKind }

// Size implements Type.
func (n *InvalidType) Size() int64 { return -1 }

type PredefinedType struct {
	attributer
	bitField *Field
	c        *ctx
	kind     Kind
	namer
	ptr    Type
	vector *ArrayType
}

func (c *ctx) newPredefinedType(kind Kind) *PredefinedType {
	return &PredefinedType{c: c, kind: kind}
}

func (n *PredefinedType) clone() (r Type) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-DBG
	m := *n
	m.namer.d = nil
	m.bitField = nil
	m.ptr = nil
	return &m
}

// BitField implements Type.
func (n *PredefinedType) BitField() *Field { return n.bitField }

func (n *PredefinedType) setBitField(f *Field) (r *PredefinedType) {
	r = n.clone().(*PredefinedType)
	r.bitField = f
	return r
}

// Pointer implements Type.
func (n *PredefinedType) Pointer() Type {
	if n.ptr == nil {
		n.ptr = n.c.newPointerType(n)
	}
	return n.ptr
}

// VectorSize implements Type.
func (n *PredefinedType) VectorSize() int64 {
	if a := n.Attributes(); a != nil {
		return a.VectorSize()
	}

	return -1
}

// setAttr implements Type.
func (n *PredefinedType) setAttr(a *Attributes) Type {
	var vec *ArrayType
	if sz := a.VectorSize(); sz > 0 {
		vec = n.c.newArrayType(n, sz, nil)
	}
	m := *n
	m.vector = vec
	m.attributer.p = a
	return &m
}

func (n *PredefinedType) IsCompatible(t Type) (r bool) {
	// defer func() { trc("%q %q %v", n, t, r) }() //TODO-
	if n == t {
		return true
	}

	switch x := t.(type) {
	case *PredefinedType:
		return n == x || n.Kind() == x.Kind()
	case *UnionType:
		return x.IsCompatible(n)
	default:
		return false
	}
}

func (n *PredefinedType) isGenericAssociationCompatible(assoc Type) bool {
	return n.IsCompatible(assoc) && !assoc.Attributes().IsConst()
}

// setName implements Type.
func (n *PredefinedType) setName(d *Declarator) Type {
	r := *n
	r.namer = namer{d}
	return &r
}

// Align implements Type.
func (n *PredefinedType) Align() int {
	if n == nil {
		return 1
	}

	if n.attributer.p != nil {
		if v := n.attributer.p.Aligned(); v > 0 {
			return int(v)
		}
	}

	if x, ok := n.c.ast.ABI.Types[n.kind]; ok {
		return x.Align
	}

	return 1
}

// Decay implements Type.
func (n *PredefinedType) Decay() Type { return n }

// Undecay implements Type.
func (n *PredefinedType) Undecay() Type { return n }

// FieldAlign implements Type.
func (n *PredefinedType) FieldAlign() int {
	if n == nil {
		return 1
	}

	if x, ok := n.c.ast.ABI.Types[n.kind]; ok {
		return x.FieldAlign
	}

	return 1
}

// String implements Type.
func (n *PredefinedType) String() string { return n.str(&strings.Builder{}, false).String() }

func (n *PredefinedType) str(b *strings.Builder, useTag bool) *strings.Builder {
	if s := n.Typedef().Name(); s != "" {
		b.WriteString(s)
		return b
	}

	if n.Attributes().IsConst() {
		b.WriteString("const ")
	}
	if n.Attributes().IsVolatile() {
		b.WriteString("volatile ")
	}
	b.WriteString(n.kind.String())
	return b
}

// IsIncomplete implements Type.
func (n *PredefinedType) IsIncomplete() bool {
	return n.isIncompleteInner(nil)
}

func (n *PredefinedType) isIncompleteInner(map[Type]struct{}) bool {
	if n == nil {
		return true
	}

	return n.Size() < 0
}

// Kind implements Type.
func (n *PredefinedType) Kind() Kind { return n.kind }

// Size implements Type.
func (n *PredefinedType) Size() int64 {
	if n == nil {
		return -1
	}

	if IsIntegerType(n) || IsFloatingPointType(n) {
		if v := n.VectorSize(); v > 0 {
			return v
		}
	}

	if x, ok := n.c.ast.ABI.Types[n.kind]; ok {
		return x.Size
	}

	return -1
}

// Parameter represents a function parameter.
type Parameter struct {
	Declarator         *Declarator         // Can be synthetic.
	AbstractDeclarator *AbstractDeclarator // Can be nil.
	name               Token
	typer
	resolver
	visible
}

// Position implements Node.
func (n *Parameter) Position() (r token.Position) {
	if n.Declarator != nil {
		return n.Declarator.Position()
	}

	if n.AbstractDeclarator != nil {
		return n.AbstractDeclarator.Position()
	}

	return n.name.Position()
}

// Name returns the name of n. The result can be a zero value, like in
// `void f(int) { ... }`.
func (n *Parameter) Name() string {
	if n.Declarator != nil {
		return n.Declarator.DirectDeclarator.name().Token.SrcStr()
	}

	return n.name.SrcStr()
}

type FunctionType struct {
	attributer
	c  *ctx
	fp []*Parameter
	namer
	noBitField
	ptr    Type
	result typer
	vectorSizer

	minArgs int
	maxArgs int // -1: unlimited

	hasImplicitResult bool
	isVariadic        bool
	usesVectors       bool
}

func (c *ctx) newFunctionType(result Type, fp []*ParameterDeclaration, isVariadic bool) (r *FunctionType) {
	r = &FunctionType{c: c, result: newTyper(result), minArgs: len(fp), maxArgs: len(fp), isVariadic: isVariadic}
	for _, n := range fp {
		p := &Parameter{}
		p.typ = n.Type()
		switch n.Case {
		case ParameterDeclarationDecl: // DeclarationSpecifiers Declarator
			p.Declarator = n.Declarator
		case ParameterDeclarationAbstract: // DeclarationSpecifiers AbstractDeclarator
			p.AbstractDeclarator = n.AbstractDeclarator
		default:
			c.errors.add(errorf("internal error: %v", n.Case))
		}
		r.fp = append(r.fp, p)
	}
	if isVariadic {
		r.maxArgs = -1
	}
	switch len(fp) {
	case 0:
		r.maxArgs = -1
	case 1:
		if t := fp[0].Type(); t != nil && t.Kind() == Void {
			r.minArgs = 0
			r.maxArgs = 0
		}
	}
	return r
}

func (c *ctx) newFunctionType2(result Type, fp []*Parameter) (r *FunctionType) {
	r = &FunctionType{c: c, result: newTyper(result), fp: fp, minArgs: len(fp), maxArgs: len(fp)}
	if len(fp) == 0 {
		r.maxArgs = -1
	}
	return r
}

func (n *FunctionType) clone() (r Type) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-DBG
	return n //TODO clone
}

// Pointer implements Type.
func (n *FunctionType) Pointer() Type {
	if n.ptr == nil {
		n.ptr = n.c.newPointerType(n)
	}
	return n.ptr
}

// UsesVectors reports whether n uses any vector tyoe.
func (n *FunctionType) UsesVectors() bool { return n.usesVectors }

// IsVariadic reports whether n is variadic.
func (n *FunctionType) IsVariadic() bool { return n.isVariadic }

// MinArgs returns the minimum number of arguments n expects.
func (n *FunctionType) MinArgs() int { return n.minArgs }

// MaxArgs returns the maximum number of arguments n expects. Variadic
// functions return a negative value.
func (n *FunctionType) MaxArgs() int { return n.maxArgs }

// setAttr implements Type.
func (n *FunctionType) setAttr(a *Attributes) Type {
	m := *n
	m.attributer.p = a
	return &m
}

func (n *FunctionType) IsCompatible(t Type) (r bool) {
	// defer func() { trc("%q %q %v", n, t, r) }() //TODO-
	if n == t {
		return true
	}

	switch x := t.(type) {
	case *FunctionType:
		if n == x {
			return true
		}

		resultOk := n.hasImplicitResult || x.hasImplicitResult || n.Result().IsCompatible(x.Result())
		if len(n.fp) == 0 || len(x.fp) == 0 {
			return resultOk
		}

		if len(n.fp) != len(x.fp) || n.minArgs != x.minArgs || n.maxArgs != x.maxArgs || !resultOk {
			return false
		}

		for i, v := range n.fp {
			t := v.Type()
			w := x.fp[i]
			u := w.Type()
			if !t.IsCompatible(u) && !IntegerPromotion(t).IsCompatible(IntegerPromotion(u)) && !t.Decay().IsCompatible(u.Decay()) {
				if Dmesgs {
					Dmesg("%v %v and %v %v", t, t.Kind(), u, u.Kind())
				}
				return false
			}
		}

		return true
	case *UnionType:
		return x.IsCompatible(n)
	default:
		return false
	}
}

func (n *FunctionType) isGenericAssociationCompatible(assoc Type) bool { return n.IsCompatible(assoc) }

// setName implements Type.
func (n *FunctionType) setName(d *Declarator) Type {
	r := *n
	r.namer = namer{d}
	return &r
}

// Result reports the result type of n.
func (n *FunctionType) Result() Type { return n.result.Type() }

// Parameters returns function type parameters.
func (n *FunctionType) Parameters() []*Parameter { return n.fp }

// Align implements Type.
func (n *FunctionType) Align() int {
	if n.attributer.p != nil {
		if v := n.attributer.p.Aligned(); v > 0 {
			return int(v)
		}
	}

	return 1
}

// Decay implements Type.
func (n *FunctionType) Decay() Type { return n.c.newPointerType2(n, n) }

// Undecay implements Type.
func (n *FunctionType) Undecay() Type { return n }

// FieldAlign implements Type.
func (n *FunctionType) FieldAlign() int { return 1 }

// IsIncomplete implements Type.
func (n *FunctionType) IsIncomplete() bool { return n.isIncompleteInner(nil) }

func (n *FunctionType) isIncompleteInner(map[Type]struct{}) bool { return n == nil }

// Kind implements Type.
func (n *FunctionType) Kind() Kind { return Function }

// Size implements Type.
func (n *FunctionType) Size() int64 {
	if n == nil {
		return -1
	}

	return 1
} // gcc compatibility

// String implements Type.
func (n *FunctionType) String() string { return n.str(&strings.Builder{}, false).String() }

func (n *FunctionType) str(b *strings.Builder, useTag bool) *strings.Builder {
	if s := n.Typedef().Name(); s != "" {
		b.WriteString(s)
		return b
	}

	b.WriteString("function(")
	switch {
	case n.maxArgs == 0:
		b.WriteString("void")
	default:
		for i, v := range n.fp {
			v.Type().str(b, true)
			if i != len(n.fp)-1 {
				b.WriteString(", ")
			}
		}
	}
	b.WriteByte(')')
	if n.Result().Kind() != Void {
		b.WriteString(" returning ")
		n.Result().str(b, true)
	}
	return b
}

type PointerType struct {
	attributer
	c    *ctx
	elem typer
	namer
	noBitField
	ptr     Type
	scope   *Scope
	undecay Type
	vectorSizer
}

// // NewPointerType returns an elem pointer.
// func NewPointerType(elem Type) (r *PointerType) {
// 	panic(todo(""))
// }

func (n *PointerType) clone() (r Type) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-
	return n //TODO clone
}

// Pointer implements Type.
func (n *PointerType) Pointer() Type {
	if n.ptr == nil {
		n.ptr = n.c.newPointerType(n)
	}
	return n.ptr
}

func (c *ctx) newPointerType(elem Type) (r *PointerType) {
	attr := elem.Attributes()
	if attr != nil {
		a2 := *attr
		a2.isVolatile = false
		a2.isConst = false
		elem, _ = mergeAttr(elem, &a2)
	}
	r = &PointerType{c: c, elem: newTyper(elem)}
	r.undecay = r
	return r
}

// setAttr implements Type.
func (n *PointerType) setAttr(a *Attributes) Type {
	m := *n
	m.attributer.p = a
	return &m
}

func (n *PointerType) IsCompatible(t Type) (r bool) {
	// defer func() { trc(":822: %q(%q) %q(%q) %v", n, n.Elem(), t, t.(*PointerType).Elem(), r) }() //TODO-
	if n == t {
		return true
	}

	switch x := t.(type) {
	case *PointerType:
		return n == x || n.elemForIsCompatible().IsCompatible(x.elemForIsCompatible())
	case *UnionType:
		return x.IsCompatible(n)
	default:
		return false
	}
}

func (n *PointerType) isGenericAssociationCompatible(assoc Type) (r bool) {
	if n == assoc {
		return true
	}

	switch x := assoc.(type) {
	case *PointerType:
		return n == x ||
			n.Attributes().IsConst() == x.Attributes().IsConst() && n.Elem().IsCompatible(x.Elem()) ||
			n.Elem().Kind() == Ptr || x.Elem().Kind() == Ptr ||
			n.Elem().Kind() == Function || x.Elem().Kind() == Function
	case *UnionType:
		return x.isGenericAssociationCompatible(assoc)
	default:
		return false
	}
}

// setName implements Type.
func (n *PointerType) setName(d *Declarator) Type {
	r := *n
	r.namer = namer{d}
	return &r
}

func (c *ctx) newPointerType2(elem, undecay Type) *PointerType {
	return &PointerType{c: c, elem: newTyper(elem), undecay: undecay}
}

// Elem returns the type n points to.
func (n *PointerType) Elem() Type {
	return n.elem.Type()
}

func (n *PointerType) elemForIsCompatible() Type {
	switch x := n.elem.typ.(type) {
	case *StructType:
		if n.scope == nil {
			break
		}

		if tag := x.tag.SrcStr(); tag != "" {
			if sus := n.scope.structOrUnion(x.tag); sus != nil {
				return sus.Type()
			}
		}
	}
	return n.elem.Type()
}

// Align implements Type.
func (n *PointerType) Align() int {
	if n == nil {
		return 1
	}

	if n.attributer.p != nil {
		if v := n.attributer.p.Aligned(); v > 0 {
			return int(v)
		}
	}

	if x, ok := n.c.ast.ABI.Types[Ptr]; ok {
		return x.Align
	}

	return 1
}

// Decay implements Type.
func (n *PointerType) Decay() Type { return n }

// Undecay implements Type.
func (n *PointerType) Undecay() Type { return n.undecay }

// FieldAlign implements Type.
func (n *PointerType) FieldAlign() int {
	if n == nil {
		return 1
	}

	if x, ok := n.c.ast.ABI.Types[Ptr]; ok {
		return x.FieldAlign
	}

	return 1
}

// IsIncomplete implements Type.
func (n *PointerType) IsIncomplete() bool { return n.isIncompleteInner(nil) }

func (n *PointerType) isIncompleteInner(map[Type]struct{}) bool { return n == nil }

// Kind implements Type.
func (n *PointerType) Kind() Kind { return Ptr }

// Size implements Type.
func (n *PointerType) Size() int64 {
	if n == nil {
		return -1
	}

	if x, ok := n.c.ast.ABI.Types[Ptr]; ok {
		return x.Size
	}

	return -1
}

// String implements Type.
func (n *PointerType) String() string { return n.str(&strings.Builder{}, false).String() }

func (n *PointerType) str(b *strings.Builder, useTag bool) *strings.Builder {
	if s := n.Typedef().Name(); s != "" {
		b.WriteString(s)
		return b
	}

	if n.Attributes().IsConst() {
		b.WriteString("const ")
	}
	if n.Attributes().IsVolatile() {
		b.WriteString("volatile ")
	}
	b.WriteString("pointer to ")
	n.Elem().str(b, true)
	return b
}

type Field struct {
	accessBytes           int64       // accessBytes < typ.Size() -> bit field.
	declarator            *Declarator // Can be nil.
	mask                  uint64      // Non zero only for bit fields.
	offsetBytes           int64
	outerGroupOffsetBytes int64
	parent                *Field
	parentField2          *Field
	parentType            Type
	typ                   typer
	valueBits             int64

	depth     int
	groupSize int
	// Additional bit offset to offset bytes. Non zero only for bit fields but can
	// be zero even for a bit field, for example, the first bit field after a non
	// bit field will have offsetBits zero.
	offsetBits int
	seq        int
	index      int // index into .fields in structType

	inOverlapGroup        bool
	isBitField            bool
	isFlexibleArrayMember bool
}

func (n *Field) clone() (r *Field) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-
	m := *n
	m.typ.typ = n.typ.typ.clone()
	return &m
}

// ParentType returns the type containing 'n'.
func (n *Field) ParentType() Type { return n.parentType }

// IsFlexibleArrayMember reports whether n is a flexible array member.
//
//	https://en.wikipedia.org/wiki/Flexible_array_member
func (n *Field) IsFlexibleArrayMember() bool { return n.isFlexibleArrayMember }

// IsBitfield reports whether n is a bit field.
func (n *Field) IsBitfield() bool { return n.isBitField }

// InOverlapGroup reports whether n, a bit field, is emdedded in a preceding
// larger bit field group.
//
// A bitfield group is the set of bit fields that share the same .Offset().
// Consider:
//
//	struct {
//		int a:7, b:2, c:1;
//	}
//
//	field	.Offset()	group	.AccessBytes()
//	  a	    0		  0	      1
//	  b	    0		  0	      2
//	  c	    1		  1	      1
//
// Because field b has offset 0 and access bytes 2, group 0 overlaps with group
// 1. Field c will report InOverlapGroup() == true.
func (n *Field) InOverlapGroup() bool { return n.IsBitfield() && n.inOverlapGroup }

// GroupSize is the maximum .AccessByte() of a group.
//
// A bitfield group is the set of bit fields that share the same .Offset().
// Consider:
//
//	struct {
//		int a:7, b:2;
//	}
//
//	field	.Offset()	group	.AccessBytes()	.GroupSize()
//	  a	    0		  0	      1		     2
//	  b	    0		  0	      2		     2
func (n *Field) GroupSize() int { return n.groupSize }

// AccessBytes reports the size in bytes used to access n. AccessBytes can be
// smaller than size of n's type when n is a bit field.
func (n *Field) AccessBytes() int64 { return n.accessBytes }

// Mask reports the mask used to access n. It is non zero only for bit fields.
func (n *Field) Mask() uint64 { return n.mask }

// Declarator reports n's declarator, if any.
func (n *Field) Declarator() *Declarator { return n.declarator }

// ValueBits reports n's size in bits.
func (n *Field) ValueBits() int64 { return n.valueBits }

// OffsetBits report n's additional bit offset to Offset. Non zero only for bit
// fields but can be zero even for a bit field, for example, the first bit
// field after a non bit field will have OffsetBits zero.
func (n *Field) OffsetBits() int { return n.offsetBits }

// Parent reports the parent of n, if any. A field has a parent when it's
// resolved in certain contexts, for example:
//
//	struct {
//		int a;
//		struct {
//			int b;
//			int c;
//			int d;
//		} e;
//		int f;
//	} g;
//
// A postfix expression node for 'g.a' will have the field 'a' attached, with
// no parent.  A postfix expression node for 'g.c' will have the field 'c'
// attached, with parent field 'e'.
func (n *Field) Parent() *Field { return n.parent }

// ParentField reports the parent field of n, if any. ParentField differs from
// Parent in that it works more generally, not only in certain contexts.
func (n *Field) ParentField() *Field { return n.parentField2 }

// Index returns the zero based field declaration index.
func (n *Field) Index() int { return n.index }

//TODO- func (n *Field) path() (r []int) {
//TODO- 	if n.parent != nil {
//TODO- 		r = n.parent.path()
//TODO- 	}
//TODO- 	return append(r, n.index)
//TODO- }

// Type reports the type of f.
func (n *Field) Type() Type { return n.typ.Type() }

// Name reports the name of f, if any.
func (n *Field) Name() string {
	if d := n.declarator; d != nil {
		return d.Name()
	}

	return ""
}

// Offset reports the offset of n in bytes.
func (n *Field) Offset() int64 { return n.offsetBytes }

// OuterGroupOffset reports the non-overlapping group offset of n in bytes. If
// n is not a bit field the value is the same as Offset(). If n is a bit field
// the value differs from Offset for bit fields reporting InOverlapGroup() ==
// true.
func (n *Field) OuterGroupOffset() int64 {
	if !n.IsBitfield() {
		return n.Offset()
	}

	return n.outerGroupOffsetBytes
}

type structType struct {
	fields []*Field
	scope  *Scope
	size   int64
	tag    Token

	align         int
	padding       int
	isIncomplete0 bool
	isUnion       bool
}

func (n *structType) clone() (r *structType) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-
	m := *n
	m.fields = append([]*Field(nil), n.fields...)
	for i, f := range m.fields {
		m.fields[i] = f.clone()
	}
	return &m
}

func (n *structType) isIncomplete(m map[Type]struct{}) (r bool) {
	if n.isIncomplete0 {
		return true
	}

	for _, f := range n.fields {
		if f.IsFlexibleArrayMember() {
			return false
		}

		if f.Type().isIncompleteInner(m) {
			if x, ok := f.Type().(*ArrayType); ok && x.IsVLA() {
				continue
			}

			return true
		}
	}
	return false
}

// Padding reports how many bytes at the end of a struct/union should be
// additionally reserved. Consider:
//
//	struct {
//		int a:1;
//	}
//
// Alignment of the above struct is that of an int, but the 'a' field uses only
// one byte. Padding will report 3 in this case.
func (n *structType) Padding() int { return n.padding }

func (n *structType) IsCompatible(m *structType) (r bool) {
	// defer func() { trc("%v", r) }() //TODO-
	if n == m {
		return true
	}

	if n.size != m.size || n.tag.SrcStr() != m.tag.SrcStr() || len(n.fields) != len(m.fields) {
		return false
	}

	for i, v := range n.fields {
		if w := m.fields[i]; v.Name() != w.Name() || !v.Type().IsCompatible(w.Type()) {
			return false
		}
	}

	return true
}

// func (n *structType) isGenericAssociationCompatible(m *structType) bool { return n.IsCompatible(m) }

// NumFields reports the number of n's fields.
func (n *structType) NumFields() int { return len(n.fields) }

func (n *structType) fieldByIndex(i int) *Field {
	if i >= 0 && i < len(n.fields) {
		return n.fields[i]
	}

	return nil
}

func (n *structType) fieldByName(nm string) *Field {
	for _, v := range n.fields {
		if v.Name() == nm {
			return v
		}
	}

	m := map[string][]*Field{}
	n.collectFields(m, nil, 0, 0, 0)
	for _, v := range m {
		sort.Slice(v, func(i, j int) bool {
			if v[i].depth < v[j].depth {
				return true
			}

			if v[i].depth > v[j].depth {
				return false
			}

			return v[i].seq < v[j].seq
		})
	}
	if s, ok := m[nm]; ok {
		return s[0]
	}

	return nil
}

func (n *structType) collectFields(m map[string][]*Field, parent *Field, depth int, off int64, seq int) int {
	for _, f := range n.fields {
		f.seq = seq
		seq++
		nm := f.Name()
		if nm == "" {
			nm = fmt.Sprintf("%d", f.Index())
		}
		switch {
		case depth != 0:
			f2 := *f
			f2.offsetBytes += off
			f2.depth = depth
			f2.parent = parent
			m[nm] = append(m[nm], &f2)
		default:
			m[nm] = append(m[nm], f)
		}
		switch f.Type().Kind() {
		case Struct:
			seq = f.Type().(*StructType).collectFields(m, f, depth+1, off+f.offsetBytes, seq)
		case Union:
			seq = f.Type().(*UnionType).collectFields(m, f, depth+1, off+f.offsetBytes, seq)
		}
	}
	return seq
}

type StructType struct {
	attrs   *Attributes
	c       *ctx
	forward *StructOrUnionSpecifier
	namer
	noBitField
	ptr Type
	structType
	vectorSizer
}

func (c *ctx) newStructType(scope *Scope, tag Token, fields []*Field, size int64, align int, attr *Attributes) (r *StructType) {
	defer func() {
		c.ast.Structs[r] = struct{}{}
	}()

	r = &StructType{c: c, structType: structType{tag: tag, fields: fields, size: size, align: align, scope: scope}}
	if attr != nil {
		return r.setAttr(attr).(*StructType)
	}

	return r
}

func (n *StructType) clone() (r Type) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-
	if n.forward != nil {
		return n.forward.Type().clone()
	}

	m := *n
	m.namer.d = nil
	m.structType = *m.structType.clone()
	return &m
}

// HasFlexibleArrayMember reports whether n has a flexible array member:
//
//	https://en.wikipedia.org/wiki/Flexible_array_member
func (n *StructType) HasFlexibleArrayMember() bool {
	if n.forward != nil {
		return n.forward.Type().(*StructType).HasFlexibleArrayMember()
	}

	l := len(n.structType.fields)
	return l != 0 && n.structType.fields[l-1].IsFlexibleArrayMember()
}

// FlexibleArrayMember returns the flexible array member of n, if any.
func (n *StructType) FlexibleArrayMember() *Field {
	if n.forward != nil {
		return n.forward.Type().(*StructType).FlexibleArrayMember()
	}

	if !n.HasFlexibleArrayMember() {
		return nil
	}

	return n.structType.fields[len(n.structType.fields)-1]
}

// Pointer implements Type.
func (n *StructType) Pointer() Type {
	if n.ptr == nil {
		n.ptr = n.c.newPointerType(n)
	}
	return n.ptr
}

// LexicalScope provides the scope the definition of n appears in.
func (n *StructType) LexicalScope() *Scope {
	if n.forward != nil {
		return n.forward.LexicalScope()
	}

	return n.scope
}

// Tag returns n's tag, if any.
func (n *StructType) Tag() Token { return n.tag }

// Attributes implemets Type.
func (n *StructType) Attributes() *Attributes {
	if n.forward != nil {
		a, _ := n.forward.Type().Attributes().merge(nil, n.attrs)
		return a
	}

	return n.attrs
}

// setAttr implements Type.
func (n *StructType) setAttr(a *Attributes) (r Type) {
	// defer func() { trc("%T p --> (%p %q)", n, n, r, r) }() //TODO-
	m := *n
	m.attrs, _ = n.Attributes().merge(nil, a)
	return &m
}

func (n *StructType) IsCompatible(t Type) (r bool) {
	// defer func() { trc("--> (%p %p %T) %q %q %v", n, t, t, n, t, r) }() //TODO-
	if n == t {
		return true
	}

	if n.forward != nil {
		return n.forward.Type().IsCompatible(t)
	}

	switch x := t.(type) {
	case *StructType:
		if x.forward != nil {
			return n.IsCompatible(x.forward.Type())
		}

		return n == x || n.structType.IsCompatible(&x.structType)
	default:
		return false
	}
}

func (n *StructType) isGenericAssociationCompatible(assoc Type) bool { return n.IsCompatible(assoc) }

// setName implements Type.
func (n *StructType) setName(d *Declarator) (r Type) {
	// defer func() { trc("%T p --> (%p %q)", n, n, r, r) }() //TODO-
	m := *n
	m.namer = namer{d}
	return &m
}

// FieldByIndex returns a member field by index, if any.
func (n *StructType) FieldByIndex(i int) *Field {
	if n == nil {
		return nil
	}

	if n.forward != nil {
		if x, ok := n.forward.typ.(*StructType); ok {
			return x.FieldByIndex(i)
		}

		return nil
	}

	return n.fieldByIndex(i)
}

// NamedFieldByIndex returns the first named member field at or after index, if any.
func (n *StructType) NamedFieldByIndex(i int) (r *Field) {
	for ; ; i++ {
		r = n.FieldByIndex(i)
		if r == nil {
			return nil
		}

		if r.Name() != "" {
			return r
		}
	}
}

// FieldByName returns the shallowest member field by name, if any.
func (n *StructType) FieldByName(nm string) *Field {
	if n == nil {
		return nil
	}

	if n.forward != nil {
		if x, ok := n.forward.typ.(*StructType); ok {
			return x.FieldByName(nm)
		}

		return nil
	}

	return n.fieldByName(nm)
}

// Align implements Type.
func (n *StructType) Align() int {
	if n == nil {
		return 1
	}

	if n.forward != nil {
		return n.forward.Type().Align()
	}

	if n.attrs != nil {
		if v := n.attrs.Aligned(); v > 0 {
			return int(v)
		}
	}

	if n.IsIncomplete() {
		return n.c.intT.Align()
	}

	return n.align
}

// Decay implements Type.
func (n *StructType) Decay() Type { return n }

// Undecay implements Type.
func (n *StructType) Undecay() Type { return n }

// FieldAlign implements Type.
func (n *StructType) FieldAlign() int {
	if n == nil {
		return 1
	}

	if n.forward != nil {
		return n.forward.Type().FieldAlign()
	}

	return n.align
}

// IsIncomplete implements Type.
func (n *StructType) IsIncomplete() bool {
	return n.isIncompleteInner(nil)
}

func (n *StructType) isIncompleteInner(m map[Type]struct{}) (r bool) {
	if n == nil {
		return true
	}

	if _, ok := m[n]; ok {
		return true
	}

	if m == nil {
		m = map[Type]struct{}{}
	}
	m[n] = struct{}{}

	defer func() { delete(m, n) }()

	if n.forward != nil {
		return n.forward.Type().isIncompleteInner(m)
	}

	return n.isIncomplete(m)
}

// Kind implements Type.
func (n *StructType) Kind() Kind { return Struct }

// Size implements Type.
func (n *StructType) Size() int64 {
	if n == nil {
		return -1
	}

	if n.forward != nil {
		return n.forward.Type().Size()
	}

	return n.size
}

// NumFields reports the number of n's fields.
func (n *StructType) NumFields() int {
	if n.forward != nil {
		return n.forward.Type().(*StructType).NumFields()
	}

	return len(n.fields)
}

// String implements Type.
func (n *StructType) String() string { return n.str(&strings.Builder{}, false).String() }

func (n *StructType) str(b *strings.Builder, useTag bool) *strings.Builder {
	if n.forward != nil {
		n.forward.Type().str(b, useTag)
		return b
	}

	if s := n.Typedef().Name(); s != "" {
		b.WriteString(s)
		return b
	}

	b.WriteString("struct")
	if s := n.tag.SrcStr(); s != "" {
		b.WriteByte(' ')
		b.WriteString(s)
		if useTag {
			return b
		}
	}

	b.WriteString(" {")
	for i, v := range n.fields {
		if v.declarator != nil {
			b.WriteString(v.declarator.Name())
			b.WriteByte(' ')
		}
		v.Type().str(b, true)
		if i != len(n.fields)-1 {
			b.WriteString("; ")
		}
	}
	b.WriteByte('}')
	return b
}

type UnionType struct {
	attrs   *Attributes
	c       *ctx
	forward *StructOrUnionSpecifier
	namer
	noBitField
	ptr Type
	structType
	vectorSizer
}

func (c *ctx) newUnionType(scope *Scope, tag Token, fields []*Field, size int64, align int, attr *Attributes) (r *UnionType) {
	defer func() {
		c.ast.Unions[r] = struct{}{}
	}()

	r = &UnionType{c: c, structType: structType{tag: tag, fields: fields, size: size, align: align, isUnion: true, scope: scope}}
	if attr != nil {
		return r.setAttr(attr).(*UnionType)
	}

	return r
}

func (n *UnionType) clone() (r Type) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-
	if n.forward != nil {
		return n.forward.Type().clone()
	}

	m := *n
	m.namer.d = nil
	m.structType = *m.structType.clone()
	return &m
}

// Pointer implements Type.
func (n *UnionType) Pointer() Type {
	if n.ptr == nil {
		n.ptr = n.c.newPointerType(n)
	}
	return n.ptr
}

// LexicalScope provides the scope the definition of n appears in.
func (n *UnionType) LexicalScope() *Scope {
	if n.forward != nil {
		return n.forward.LexicalScope()
	}

	return n.scope
}

// Tag returns n's tag, if any.
func (n *UnionType) Tag() Token { return n.tag }

// Attributes implemets Type.
func (n *UnionType) Attributes() *Attributes {
	if n.forward != nil {
		a, _ := n.forward.Type().Attributes().merge(nil, n.attrs)
		return a
	}

	return n.attrs
}

// setAttr implements Type.
func (n *UnionType) setAttr(a *Attributes) Type {
	m := *n
	m.attrs, _ = n.Attributes().merge(nil, a)
	return &m
}

func (n *UnionType) IsCompatible(t Type) (r bool) {
	// defer func() { trc("%q %q %v", n, t, r) }() //TODO-
	if n == t {
		return true
	}

	if n.forward != nil {
		return n.forward.Type().IsCompatible(t)
	}

	switch x := t.(type) {
	case *UnionType:
		if x.forward != nil {
			return n.IsCompatible(x.forward.Type())
		}

		return n == x || n.structType.IsCompatible(&x.structType)
	default:
		return len(n.fields) == 1 && n.fields[0].Type().IsCompatible(t)
	}
}

func (n *UnionType) isGenericAssociationCompatible(assoc Type) bool { return n.IsCompatible(assoc) }

// setName implements Type.
func (n *UnionType) setName(d *Declarator) Type {
	r := *n
	r.namer = namer{d}
	return &r
}

// FieldByIndex returns a member field by index, if any.
func (n *UnionType) FieldByIndex(i int) *Field {
	if n == nil {
		return nil
	}

	if n.forward != nil {
		if x, ok := n.forward.typ.(*UnionType); ok {
			return x.FieldByIndex(i)
		}

		return nil
	}

	return n.fieldByIndex(i)
}

// NamedFieldByIndex returns the first named member field at or after index, if any.
func (n *UnionType) NamedFieldByIndex(i int) (r *Field) {
	for ; ; i++ {
		r = n.FieldByIndex(i)
		if r == nil {
			return nil
		}

		if r.Name() != "" {
			return r
		}
	}
}

// FieldByName returns member field nm of n or nil if n does not have such member.
func (n *UnionType) FieldByName(nm string) *Field {
	if n == nil {
		return nil
	}

	if n.forward != nil {
		if x, ok := n.forward.typ.(*UnionType); ok {
			return x.FieldByName(nm)
		}

		return nil
	}

	return n.fieldByName(nm)
}

// Align implements Type.
func (n *UnionType) Align() int {
	if n == nil {
		return 1
	}

	if n.forward != nil {
		return n.forward.Type().Align()
	}

	if n.attrs != nil {
		if v := n.attrs.Aligned(); v > 0 {
			return int(v)
		}
	}

	if n.IsIncomplete() {
		return n.c.intT.Align()
	}

	return n.align
}

// Decay implements Type.
func (n *UnionType) Decay() Type { return n }

// Undecay implements Type.
func (n *UnionType) Undecay() Type { return n }

// FieldAlign implements Type.
func (n *UnionType) FieldAlign() int {
	if n == nil {
		return 1
	}

	if n.forward != nil {
		return n.forward.Type().FieldAlign()
	}

	return n.align
}

// IsIncomplete implements Type.
func (n *UnionType) IsIncomplete() bool {
	return n.isIncompleteInner(nil)
}

func (n *UnionType) isIncompleteInner(m map[Type]struct{}) bool {
	if n == nil {
		return true
	}

	if _, ok := m[n]; ok {
		return true
	}

	if m == nil {
		m = map[Type]struct{}{}
	}
	m[n] = struct{}{}

	defer func() { delete(m, n) }()

	if n.forward != nil {
		return n.forward.Type().isIncompleteInner(m)
	}

	return n.isIncomplete(m)
}

// Kind implements Type.
func (n *UnionType) Kind() Kind { return Union }

// Size implements Type.
func (n *UnionType) Size() int64 {
	if n == nil {
		return -1
	}

	if n.forward != nil {
		return n.forward.Type().Size()
	}

	return n.size
}

// NumFields reports the number of n's fields.
func (n *UnionType) NumFields() int {
	if n.forward != nil {
		return n.forward.Type().(*UnionType).NumFields()
	}

	return len(n.fields)
}

// String implements Type.
func (n *UnionType) String() string { return n.str(&strings.Builder{}, false).String() }

func (n *UnionType) str(b *strings.Builder, useTag bool) *strings.Builder {
	if n.forward != nil {
		n.forward.Type().str(b, useTag)
		return b
	}

	if s := n.Typedef().Name(); s != "" {
		b.WriteString(s)
		return b
	}

	b.WriteString("union")
	if s := n.tag.SrcStr(); s != "" {
		b.WriteByte(' ')
		b.WriteString(s)
		if useTag {
			return b
		}
	}

	b.WriteString(" {")
	for i, v := range n.fields {
		if v.declarator != nil {
			b.WriteString(v.declarator.Name())
			b.WriteByte(' ')
		}
		v.Type().str(b, true)
		if i != len(n.fields)-1 {
			b.WriteString("; ")
		}
	}
	b.WriteByte('}')
	return b
}

type vectorSizer struct{}

// VectorSize implements Type.
func (vectorSizer) VectorSize() int64 { return -1 }

type ArrayType struct {
	attributer
	c     *ctx
	elem  typer
	elems int64
	expr  ExpressionNode
	namer
	noBitField
	ptr Type
	vectorSizer
}

func (c *ctx) newArrayType(elem Type, elems int64, expr ExpressionNode) (r *ArrayType) {
	r = &ArrayType{c: c, elem: newTyper(elem), elems: elems, expr: expr}
	return r
}

func (n *ArrayType) clone() (r Type) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-
	m := *n
	return &m //TODO clone
}

// Pointer implements Type.
func (n *ArrayType) Pointer() Type {
	if n.ptr == nil {
		n.ptr = n.c.newPointerType(n)
	}
	return n.ptr
}

// setAttr implements Type.
func (n *ArrayType) setAttr(a *Attributes) Type {
	m := *n
	m.attributer.p = a
	return &m
}

func (n *ArrayType) IsCompatible(t Type) (r bool) {
	// defer func() { trc("%q %q %v", n, t, r) }() //TODO-
	if n == t {
		return true
	}

	switch x := t.(type) {
	case *ArrayType:
		return n.Elem().IsCompatible(x.Elem()) && (n.Len() < 0 || x.Len() < 0 || n.Len() == x.Len())
	default:
		return false
	}
}

func (n *ArrayType) isGenericAssociationCompatible(assoc Type) bool { return n.IsCompatible(assoc) }

// setName implements Type.
func (n *ArrayType) setName(d *Declarator) Type {
	r := *n
	r.namer = namer{d}
	return &r
}

func (n *ArrayType) IsVLA() bool {
	for {
		if n.elems < 0 && n.expr != nil && n.expr.Value() == Unknown {
			return true
		}

		x, ok := n.Elem().(*ArrayType)
		if !ok {
			break
		}

		n = x
	}
	return false
}

// Decay implements Type.
func (n *ArrayType) Decay() Type { return n.c.newPointerType2(n.Elem(), n) }

// Undecay implements Type.
func (n *ArrayType) Undecay() Type { return n }

// Elem reports the element type of n.
func (n *ArrayType) Elem() Type { return n.elem.Type() }

// Align implements Type.
func (n *ArrayType) Align() int {
	if n == nil {
		return 1
	}

	if n.attributer.p != nil {
		if v := n.attributer.p.Aligned(); v > 0 {
			return int(v)
		}
	}

	return n.elem.Type().Align()
}

// FieldAlign implements Type.
func (n *ArrayType) FieldAlign() int {
	if n == nil {
		return 1
	}

	return n.elem.Type().FieldAlign()
}

// IsIncomplete implements Type.
func (n *ArrayType) IsIncomplete() bool {
	return n.isIncompleteInner(nil)
}

func (n *ArrayType) isIncompleteInner(m map[Type]struct{}) bool {
	if n == nil {
		return true
	}

	if _, ok := m[n]; ok {
		return true
	}

	if m == nil {
		m = map[Type]struct{}{}
	}
	m[n] = struct{}{}

	defer func() { delete(m, n) }()

	return n.Elem().isIncompleteInner(m) || n.elems < 0
}

// Kind implements Type.
func (n *ArrayType) Kind() Kind { return Array }

// Len reports the number of elements in n or a negative value if n is incomplete.
func (n *ArrayType) Len() int64 { return n.elems }

// Size implements Type.
func (n *ArrayType) Size() int64 {
	if n == nil {
		return -1
	}

	if n.Elem().Kind() != InvalidKind {
		if a, b := n.elems, n.Elem().Size(); a >= 0 && b > 0 {
			return a * b
		}
	}

	return -1
}

// SizeExpression returns the expression determining the number of elements in 'n', if available.
func (n *ArrayType) SizeExpression() ExpressionNode { return n.expr }

// String implements Type.
func (n *ArrayType) String() string { return n.str(&strings.Builder{}, false).String() }

func (n *ArrayType) str(b *strings.Builder, useTag bool) *strings.Builder {
	if s := n.Typedef().Name(); s != "" {
		b.WriteString(s)
		return b
	}

	b.WriteString("array of ")
	if !n.IsIncomplete() {
		fmt.Fprintf(b, "%d ", n.elems)
	}
	n.Elem().str(b, true)
	return b
}

type EnumType struct {
	attr    attributer
	c       *ctx
	enums   []*Enumerator
	forward *EnumSpecifier
	max     uint64
	min     int64
	namer
	noBitField
	ptr   Type
	scope *Scope
	tag   Token
	typ   typer
	vectorSizer

	isIncomplete0 bool
}

func (c *ctx) newEnumType(scope *Scope, tag Token, typ Type, enums []*Enumerator) *EnumType {
	return &EnumType{c: c, tag: tag, typ: newTyper(typ), enums: enums, scope: scope}
}

func (n *EnumType) clone() (r Type) {
	// defer func() { trc("%p %q cloned to %p %q", n, n, r, r) }() //TODO-
	return n //TODO clone
}

// Min returns n's lowest enumeration constant.
func (n *EnumType) Min() int64 {
	if n.forward != nil {
		return n.forward.Type().(*EnumType).Min()
	}

	return n.min
}

// Max returns n's highest enumeration constant.
func (n *EnumType) Max() uint64 {
	if n.forward != nil {
		return n.forward.Type().(*EnumType).Max()
	}

	return n.max
}

// Pointer implements Type.
func (n *EnumType) Pointer() Type {
	if n.ptr == nil {
		n.ptr = n.c.newPointerType(n)
	}
	return n.ptr
}

// LexicalScope provides the scope the definition of n appears in.
func (n *EnumType) LexicalScope() *Scope {
	if n.forward != nil {
		return n.forward.LexicalScope()
	}

	return n.scope
}

// Tag returns n's tag, if any.
func (n *EnumType) Tag() Token { return n.tag }

// Enumerators returns enumerators defined by n.
func (n *EnumType) Enumerators() []*Enumerator {
	if n.forward != nil {
		return n.forward.Type().(*EnumType).Enumerators()
	}

	return n.enums
}

// Attributes implemets Type.
func (n *EnumType) Attributes() *Attributes {
	if n.forward != nil {
		return n.forward.Type().Attributes()
	}

	return n.attr.p
}

// setAttr implements Type.
func (n *EnumType) setAttr(a *Attributes) Type {
	if n.forward != nil {
		return n.forward.Type().setAttr(a)
	}

	m := *n
	m.attr.p = a
	return &m
}

// Type returns n's underlying integer type.
func (n *EnumType) UnderlyingType() Type {
	if n.forward != nil {
		return n.forward.Type().(*EnumType).UnderlyingType()
	}

	return n.typ.Type()
}

func (n *EnumType) IsCompatible(t Type) (r bool) {
	// defer func() { trc("%q %q %v", n, t, r) }() //TODO-
	if n == t {
		return true
	}

	if n.forward != nil {
		return n.forward.Type().IsCompatible(t)
	}

	switch x := t.(type) {
	case *EnumType:
		if x.forward != nil {
			return n.IsCompatible(x.forward.Type())
		}

		if n == x {
			return true
		}

		if n.tag.SrcStr() != x.tag.SrcStr() {
			return false
		}

		if !n.typ.Type().IsCompatible(x.typ.Type()) { //TODO members and values must be the same
			return false
		}

		return true
	case *UnionType:
		return x.IsCompatible(n)
	default:
		return false
	}
}

func (n *EnumType) isGenericAssociationCompatible(assoc Type) bool { return n.IsCompatible(assoc) }

// setName implements Type.
func (n *EnumType) setName(d *Declarator) Type {
	r := *n
	r.namer = namer{d}
	return &r
}

// Align implements Type.
func (n *EnumType) Align() int {
	if n == nil {
		return 1
	}

	if n.forward != nil {
		return n.forward.Type().Align()
	}

	if n.attr.p != nil {
		if v := n.attr.p.Aligned(); v > 0 {
			return int(v)
		}
	}

	return n.typ.Type().Align()
}

// Decay implements Type.
func (n *EnumType) Decay() Type { return n }

// Undecay implements Type.
func (n *EnumType) Undecay() Type { return n }

// FieldAlign implements Type.
func (n *EnumType) FieldAlign() int {
	if n == nil {
		return 1
	}

	if n.forward != nil {
		return n.forward.Type().FieldAlign()
	}

	return n.typ.Type().FieldAlign()
}

// IsIncomplete implements Type.
func (n *EnumType) IsIncomplete() bool {
	if n == nil {
		return true
	}

	if n.forward != nil {
		return n.forward.Type().IsIncomplete()
	}

	return n.isIncomplete0 || n.typ.Type().IsIncomplete()
}

func (n *EnumType) isIncompleteInner(m map[Type]struct{}) bool {
	if n == nil {
		return true
	}

	if _, ok := m[n]; ok {
		return true
	}

	if m == nil {
		m = map[Type]struct{}{}
	}
	m[n] = struct{}{}

	defer func() { delete(m, n) }()

	if n.forward != nil {
		return n.forward.Type().isIncompleteInner(m)
	}

	return n.isIncomplete0 || n.typ.Type().isIncompleteInner(m)
}

// Kind implements Type.
func (n *EnumType) Kind() Kind { return Enum }

// Size implements Type.
func (n *EnumType) Size() int64 {
	if n == nil {
		return -1
	}

	if n.forward != nil {
		return n.forward.Type().Size()
	}

	return n.typ.Type().Size()
}

// String implements Type.
func (n *EnumType) String() string { return n.str(&strings.Builder{}, false).String() }

func (n *EnumType) str(b *strings.Builder, useTag bool) *strings.Builder {
	if n.forward != nil {
		n.forward.Type().str(b, useTag)
		return b
	}

	if s := n.Typedef().Name(); s != "" {
		b.WriteString(s)
		return b
	}

	b.WriteString("enum ")
	b.WriteString(n.tag.SrcStr())
	if n.forward != nil {
		return b
	}

	b.WriteString(" { ... }") //TODO
	return b
}

// func isObjectType(t Type) bool { return t.Kind() != Function && !t.IsIncomplete() }

func isLvalue(t Type) bool { return t.Kind() != Function && t.Kind() != Void }

func isModifiableLvalue(t Type) bool {
	return t.Kind() != Function && t.Kind() != Void && t.Kind() != Array && !t.IsIncomplete()
}

func isPointerType(t Type) bool { return t.Kind() == Ptr }

func isVectorType(t Type) bool { a := t.Attributes(); return a != nil && a.vectorSize > 0 }

// IsIntegerType reports whether t is an integer type.
func IsIntegerType(t Type) bool { return integerKinds[t.Kind()] }

// IsFloatingPointType reports whether t is a floating point type.
func IsFloatingPointType(t Type) bool { return realFloatingPointKinds[t.Kind()] }

// IsComplexType reports whether t is a complex type.
func IsComplexType(t Type) bool { return complexKinds[t.Kind()] }

// IsScalarType reports whether t is a scalar type.
func IsScalarType(t Type) bool { return IsArithmeticType(t) || t.Kind() == Ptr }

// IsArithmeticType reports whether t is an arithmetic type.
func IsArithmeticType(t Type) bool { return arithmeticKinds[t.Kind()] }

// IsRealType reports whether t is a real type.
func IsRealType(t Type) bool { return realKinds[t.Kind()] }

// IsSignedInteger reports whether t is a signed integer type.
func IsSignedInteger(t Type) bool {
	switch x := t.(type) {
	case *PredefinedType:
		return x.c.ast.ABI.isSignedInteger(x.Kind())
	case *EnumType:
		return IsSignedInteger(x.UnderlyingType())
	default:
		return false
	}
}

// UsualArithmeticConversions returns the common type of a binary operation.
//
//	[0] 6.3.1.8 Usual arithmetic conversions
func UsualArithmeticConversions(a, b Type) (r Type) {
	defer func(a, b Type) {
		x := a.BitField()
		y := b.BitField()
		switch {
		case x != nil && y != nil:
			if y.ValueBits() > x.ValueBits() {
				x = y
			}
			if t, ok := r.(*PredefinedType); ok {
				r = t.setBitField(x)
			}
		case x != nil:
			if t, ok := r.(*PredefinedType); ok {
				r = t.setBitField(x)
			}
		case y != nil:
			if t, ok := r.(*PredefinedType); ok {
				r = t.setBitField(y)
			}
		}
	}(a, b)

	if a.Kind() == Enum {
		a = a.(*EnumType).UnderlyingType()
	}
	if b.Kind() == Enum {
		b = b.(*EnumType).UnderlyingType()
	}
	ak := a.Kind()
	bk := b.Kind()
	if ak == InvalidKind || bk == InvalidKind {
		return Invalid
	}

	// First, if the corresponding real type of either operand is long double, the
	// other operand is converted, without change of type domain, to a type whose
	// corresponding real type is long double.
	if ak == ComplexLongDouble {
		return a
	}

	if bk == ComplexLongDouble {
		return b
	}

	if ak == LongDouble {
		return a
	}

	if bk == LongDouble {
		return b
	}

	if ak == Float128 {
		return a
	}

	if bk == Float128 {
		return b
	}

	if ak == Decimal128 {
		return a
	}

	if bk == Decimal128 {
		return b
	}

	if ak == Decimal64 {
		return a
	}

	if bk == Decimal64 {
		return b
	}

	// Otherwise, if the corresponding real type of either operand is double, the
	// other operand is converted, without change of type domain, to a type whose
	// corresponding real type is double.

	if ak == ComplexDouble {
		return a
	}

	if bk == ComplexDouble {
		return b
	}

	if ak == Double {
		return a
	}

	if bk == Double {
		return b
	}

	// Otherwise, if the corresponding real type of either operand is float, the
	// other operand is converted, without change of type domain, to a type whose
	// corresponding real type is float.

	if ak == ComplexFloat {
		return a
	}

	if bk == ComplexFloat {
		return b
	}

	if ak == Float {
		return a
	}

	if bk == Float {
		return b
	}

	// ---- gcc complex integers
	if IsComplexType(a) || IsComplexType(b) {
		if a.Kind() == b.Kind() {
			return a
		}

		if a.Size() > b.Size() {
			return a
		}

		if b.Size() > a.Size() {
			return b
		}

		panic(todo("", a, b))
	}

	if IsFloatingPointType(a) && IsFloatingPointType(b) && a.Kind() == b.Kind() {
		return a
	}

	// Otherwise, the integer promotions are performed on both operands.

	if !IsIntegerType(a) || !IsIntegerType(b) {
		panic(todo("internal error: %s and %s", a, b))
	}

	ast := a.(*PredefinedType).c.ast
	abi := ast.ABI

	ak = integerPromotionKind(ak)
	bk = integerPromotionKind(bk)
	a = ast.kinds[ak]
	b = ast.kinds[bk]

	// Then the following rules are applied to the promoted operands:

	// If both operands have the same type, then no further conversion is needed.
	if ak == bk {
		return a
	}

	// Otherwise, if both operands have signed integer types or both have unsigned
	// integer types, the operand with the type of lesser integer conversion rank
	// is converted to the type of the operand with greater rank.
	if abi.isSignedInteger(ak) == abi.isSignedInteger(bk) {
		if intConvRanks[ak] > intConvRanks[bk] {
			return a
		}

		return b
	}

	// Otherwise, if the operand that has unsigned integer type has rank greater or
	// equal to the rank of the type of the other operand, then the operand with
	// signed integer type is converted to the type of the operand with unsigned
	// integer type.
	switch {
	case abi.isSignedInteger(ak):
		// b is unsigned
		if intConvRanks[bk] > intConvRanks[ak] {
			return b
		}
	default:
		// a is unsigned
		if intConvRanks[ak] > intConvRanks[bk] {
			return a
		}
	}

	// Otherwise, if the type of the operand with signed integer type can represent
	// all of the values of the type of the operand with unsigned integer type,
	// then the operand with unsigned integer type is converted to the type of the
	// operand with signed integer type.
	var signed Type
	switch {
	case abi.isSignedInteger(ak):
		// a is signed
		signed = a
		if a.Size() > b.Size() {
			return a
		}
	default:
		// b is signed
		signed = b
		if b.Size() > a.Size() {
			return b
		}
	}

	// Otherwise, both operands are converted to the unsigned integer type
	// corresponding to the type of the operand with signed integer type.
	switch signed.Kind() {
	case Char, SChar:
		return ast.kinds[UChar]
	case Short:
		return ast.kinds[UShort]
	case Int:
		return ast.kinds[UInt]
	case Long:
		return ast.kinds[ULong]
	case LongLong:
		return ast.kinds[ULongLong]
	case Int128:
		return ast.kinds[UInt128]
	}

	panic(todo(""))
}

func integerPromotionKind(k Kind) Kind {
	switch k {
	case Bool, Char, SChar, UChar, Short, UShort:
		return Int
	default:
		return k
	}
}

// IntegerPromotion performs the type conversion defined in [0]6.3.1.1-2.
//
// If an int can represent all values of the original type, the value is
// converted to an int; otherwise, it is converted to an unsigned int. These
// are called the integer promotions. All other types are unchanged by the
// integer promotions.
func IntegerPromotion(t Type) Type {
	switch t.Kind() {
	case Bool, Char, SChar, UChar, Short, UShort:
		return t.(*PredefinedType).c.ast.kinds[Int]
	default:
		return t
	}
}

// Attributes represent selected values from __attribute__ constructs,
// qualifiers etc.
//
// See also https://gcc.gnu.org/onlinedocs/gcc/Attribute-Syntax.html
type Attributes struct {
	alias            string
	aliasDecl        *Declarator
	aligned          int64
	vectorSize       int64
	visibility       string
	visibilityDecl   *Declarator
	customAttributes map[string][]Value

	alwaysInline bool // __attribute__ ((__always_inline__))
	gnuInline    bool // __attribute__ ((__gnu_inline__))
	isConst      bool
	isNonZero    bool
	isVolatile   bool
	weak         bool
}

func newAttributes() *Attributes {
	return &Attributes{
		aligned:    -1,
		vectorSize: -1,
	}
}

func (n *Attributes) setAlias(v string)               { n.alias = v; n.isNonZero = true }
func (n *Attributes) setAliasDecl(d *Declarator)      { n.aliasDecl = d; n.isNonZero = true }
func (n *Attributes) setAligned(v int64)              { n.aligned = v; n.isNonZero = true }
func (n *Attributes) setAlwaysInline()                { n.alwaysInline = true; n.isNonZero = true }
func (n *Attributes) setGNUInline()                   { n.gnuInline = true; n.isNonZero = true }
func (n *Attributes) setVectorSize(v int64)           { n.vectorSize = v; n.isNonZero = true }
func (n *Attributes) setVisibility(s string)          { n.visibility = s; n.isNonZero = true }
func (n *Attributes) setVisibilityDecl(d *Declarator) { n.visibilityDecl = d; n.isNonZero = true }
func (n *Attributes) setWeak()                        { n.weak = true; n.isNonZero = true }

func (n *Attributes) setCustom(attr string, v []Value) {
	if n.customAttributes == nil {
		n.customAttributes = make(map[string][]Value, 1)
	}
	n.customAttributes[attr] = v
	n.isNonZero = true
}

func (n *Attributes) setIsVolatile(v bool) {
	v, n.isVolatile = n.isVolatile, v
	n.isNonZero = v != n.isVolatile
}

func (n *Attributes) setIsConst(v bool) {
	v, n.isConst = n.isConst, v
	n.isNonZero = v != n.isConst
}

func (n *Attributes) merge(nd Node, m *Attributes) (r *Attributes, err error) {
	if n == nil {
		return m, nil
	}

	if m == nil {
		return n, nil
	}

	if !n.isNonZero {
		return m, nil
	}

	if !m.isNonZero {
		return n, nil
	}

	r = &Attributes{isNonZero: true}

	switch {
	case n.aliasDecl == nil && m.aliasDecl == nil:
		// nop
	case n.aliasDecl == nil && m.aliasDecl != nil:
		r.aliasDecl = m.aliasDecl
	case n.aliasDecl != nil && m.aliasDecl == nil:
		r.aliasDecl = n.aliasDecl
	default:
		if n.aliasDecl != m.aliasDecl {
			return nil, errorf("%v: conflicting attributes: (%+v), (%+v)", pos(nd), n, m)
		}

		r.aliasDecl = n.aliasDecl
	}

	switch {
	case n.alias == "" && m.alias == "":
		// nop
	case n.alias == "" && m.alias != "":
		r.alias = m.alias
	case n.alias != "" && m.alias == "":
		r.alias = n.alias
	default:
		if n.alias != m.alias {
			return nil, errorf("%v: conflicting attributes: (%+v), (%+v)", pos(nd), n, m)
		}

		r.alias = n.alias
	}

	switch {
	case n.aligned <= 0 && m.aligned <= 0:
		r.aligned = -1
	case n.aligned <= 0 && m.aligned > 0:
		r.aligned = m.aligned
	case n.aligned > 0 && m.aligned <= 0:
		r.aligned = n.aligned
	default:
		if n.aligned != m.aligned {
			return nil, errorf("%v: conflicting attributes: (%+v), (%+v)", pos(nd), n, m)
		}

		r.aligned = n.aligned
	}

	switch {
	case n.vectorSize <= 0 && m.vectorSize <= 0:
		r.vectorSize = -1
	case n.vectorSize <= 0 && m.vectorSize > 0:
		r.vectorSize = m.vectorSize
	case n.vectorSize > 0 && m.vectorSize <= 0:
		r.vectorSize = n.vectorSize
	default:
		if n.vectorSize != m.vectorSize {
			return nil, errorf("%v: conflicting attributes: (%+v), (%+v)", pos(nd), n, m)
		}

		r.vectorSize = n.vectorSize
	}

	switch {
	case n.visibilityDecl == nil && m.visibilityDecl == nil:
		// nop
	case n.visibilityDecl == nil && m.visibilityDecl != nil:
		r.visibilityDecl = m.visibilityDecl
	case n.visibilityDecl != nil && m.visibilityDecl == nil:
		r.visibilityDecl = n.visibilityDecl
	default:
		switch {
		case n.visibilityDecl.NameTok().seq <= m.visibilityDecl.NameTok().seq:
			r.visibilityDecl = n.visibilityDecl
		default:
			r.visibilityDecl = m.visibilityDecl
		}
	}

	switch {
	case n.visibility == "" && m.visibility == "":
		// nop
	case n.visibility == "" && m.visibility != "":
		r.visibility = m.visibility
	case n.visibility != "" && m.visibility == "":
		r.visibility = n.visibility
	default:
		if n.visibility != m.visibility {
			return nil, errorf("%v: conflicting attributes: (%+v), (%+v)", pos(nd), n, m)
		}

		r.visibility = n.visibility
	}

	switch {
	case n.weak || m.weak:
		r.weak = true
	}

	switch {
	case n.isVolatile || m.isVolatile:
		r.isVolatile = true
	}

	switch {
	case n.isConst || m.isConst:
		r.isConst = true
	}

	switch {
	case n.alwaysInline || m.alwaysInline:
		r.alwaysInline = true
	}

	switch {
	case n.gnuInline || m.gnuInline:
		r.gnuInline = true
	}

	var ok bool
	r.customAttributes, ok = mergeCustomAttributes(n.customAttributes, m.customAttributes)
	if !ok {
		// This is slightly different from how the gcc documentation handles it.
		// GCC only emits a warning, not an error, and it is not written down
		// how gcc actually merges.
		return nil, errorf("%v: conflicting attributes: (%+v), (%+v)", pos(nd), n, m)
	}

	return r, nil
}

func mergeCustomAttributes(n, m map[string][]Value) (r map[string][]Value, ok bool) {
	if len(n)+len(m) != 0 {
		r = make(map[string][]Value)
		for k, v := range m {
			r[k] = v
		}
		for k, v := range n {
			r[k] = append(r[k], v...)
		}
		for k, v := range r {
			r[k] = v[:sortutil.Dedupe(values(v))]
		}
	}
	return r, true
}

type values []Value

func (v values) Len() int      { return len(v) }
func (v values) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v values) Less(i, j int) bool {
	return fmt.Sprintf("%T(%[1]v)", v[i]) < fmt.Sprintf("%T(%[1]v)", v[j])
}

// GNUInline reports whether __attribute__((__gnu_inline__)) is present.
func (n *Attributes) GNUInline() bool { return n != nil && n.gnuInline }

// AlwaysInline reports whether __attribute__((__always_inline__)) is present.
func (n *Attributes) AlwaysInline() bool { return n != nil && n.alwaysInline }

// IsVolatile reports if a type is qualified by the 'volatile' keyword.
func (n *Attributes) IsVolatile() bool { return n != nil && n.isVolatile }

// IsConst reports if a type is qualified by the 'const' keyword.
func (n *Attributes) IsConst() bool { return n != nil && n.isConst }

// Alias returns S from __attribute__((alias("S"))) or "" if not present.
func (n *Attributes) Alias() string { return n.alias }

// AliasDecl returns the declarator, if any, named in __attribute__((alias("S"))) or nil if not present.
func (n *Attributes) AliasDecl() *Declarator { return n.aliasDecl }

// Aligned returns N from __attribute__((aligned(N)) or -1 if not
// present/valid.
func (n *Attributes) Aligned() int64 { return n.aligned }

// VectorSize returns N from __attribute__((vector_size(N)) or -1 if not
// present/valid.
//
// The vector_size attribute is only applicable to integral and floating
// scalars, otherwise it's ignored.
func (n *Attributes) VectorSize() int64 { return n.vectorSize }

// Visibility returns S from __attribute__((visibility("S"))) or "" if not present.
func (n *Attributes) Visibility() string { return n.visibility }

// VisibilityDecl returns the declarator, if any, named in __attribute__((visibility("S"))) or nil if not present.
func (n *Attributes) VisibilityDecl() *Declarator { return n.visibilityDecl }

// Weak reports whether 'weak', as in __attribute__((weak, alias("S"))), is present.
func (n *Attributes) Weak() bool { return n.weak }

// IsAttrSet reports whether an attribute has been set, with or without value. For example,
// with __attribute__((my_attribute1, my_attribute2(42))), it reports true for both my_attribute1
// and my_attribute2
func (n *Attributes) IsAttrSet(name string) (r bool) {
	_, r = n.customAttributes[name]
	return r
}

// AttrValue reports the value associated with a custom attribute, if present. For example,
// with __attribute__((my_attribute1, my_attribute2(42))), it reports nil for my_attribute1
// but a Value containing 42 for my_attribute2
func (n *Attributes) AttrValue(name string) []Value {
	if n != nil {
		return n.customAttributes[name]
	}

	return nil
}

type attributer struct{ p *Attributes }

// Attributes implemets Type.
func (n attributer) Attributes() *Attributes { return n.p }

func isEmpty(t Type) bool {
	switch x := t.(type) {
	case *StructType:
		return x.NumFields() == 0
	case *UnionType:
		return x.NumFields() == 0
	}

	return false
}
