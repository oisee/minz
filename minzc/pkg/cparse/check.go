// Copyright 2022 The CC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cparse

import (
	"math"
	"math/big"
	"math/bits"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"modernc.org/mathutil"
)

const (
	// type check
	decay flags = 1 << iota
	asmArgList
	implicitFuncDef
	ignoreUndefined
	noRead // in __builtin_constant_p(d) 'd' is not read

	// eval
	addrOf
)

type flags int

func (f flags) add(g flags) flags { return f | g }
func (f flags) del(g flags) flags { return f &^ g }
func (f flags) has(g flags) bool  { return f&g != 0 }

type ExpressionNode interface {
	Node
	Type() Type
	Value() Value
	// Pure returns true if an expression has no side effects.
	Pure() bool
	check(*ctx, flags) Type
	eval(*ctx, flags) Value
}

const longDoublePrec = 256 // mantissa bits

type purer bool

// Pure implements ExpressionNode.
func (p *purer) Pure() bool { return p != nil && bool(*p) }

func (p *purer) setPure(v bool) {
	if p != nil {
		*p = purer(v)
	}
}

type ctx struct {
	ast          *AST
	builtinTypes map[string]Type
	cfg          *Config
	errors       errors
	fnScope      *Scope
	implicitFunc Type
	int64T       Type
	intT         Type
	pcharT       Type
	ptrDiffT0    Type
	pvoidT       Type
	sizeT0       Type
	voidT        Type
	wcharT0      Type

	inLoop      int
	inSwitch    int
	indentN     int // debug
	switchCases int

	checkingSizeof bool
	usesVectors    bool
}

func newCtx(ast *AST, cfg *Config) *ctx {
	c := &ctx{ast: ast, cfg: cfg}
	complexdouble := c.newPredefinedType(ComplexDouble)
	complexlong := c.newPredefinedType(ComplexLong)
	int := c.newPredefinedType(Int)
	long := c.newPredefinedType(Long)
	longlong := c.newPredefinedType(LongLong)
	short := c.newPredefinedType(Short)
	uint := c.newPredefinedType(UInt)
	int128 := c.newPredefinedType(Int128)
	uint128 := c.newPredefinedType(UInt128)
	ulong := c.newPredefinedType(ULong)
	ulonglog := c.newPredefinedType(ULongLong)
	ushort := c.newPredefinedType(UShort)
	c.builtinTypes = map[string]Type{
		ts2String([]TypeSpecifierCase{TypeSpecifierBool}):                                                             c.newPredefinedType(Bool),
		ts2String([]TypeSpecifierCase{TypeSpecifierChar, TypeSpecifierSigned}):                                        c.newPredefinedType(SChar),
		ts2String([]TypeSpecifierCase{TypeSpecifierChar, TypeSpecifierUnsigned}):                                      c.newPredefinedType(UChar),
		ts2String([]TypeSpecifierCase{TypeSpecifierChar}):                                                             c.newPredefinedType(Char),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierChar}):                                       c.newPredefinedType(ComplexChar),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierDouble}):                                     complexdouble,
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierFloat16}):                                    c.newPredefinedType(ComplexFloat16),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierFloat}):                                      c.newPredefinedType(ComplexFloat),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierInt, TypeSpecifierLong}):                     complexlong,
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierInt}):                                        c.newPredefinedType(ComplexInt),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierLong, TypeSpecifierDouble}):                  c.newPredefinedType(ComplexLongDouble),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierLong, TypeSpecifierLong}):                    c.newPredefinedType(ComplexLongLong),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierLong}):                                       complexlong,
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierShort, TypeSpecifierUnsigned}):               c.newPredefinedType(ComplexUShort),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierShort}):                                      c.newPredefinedType(ComplexShort),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex, TypeSpecifierUnsigned}):                                   c.newPredefinedType(ComplexUInt),
		ts2String([]TypeSpecifierCase{TypeSpecifierComplex}):                                                          complexdouble,
		ts2String([]TypeSpecifierCase{TypeSpecifierDecimal128}):                                                       c.newPredefinedType(Decimal128),
		ts2String([]TypeSpecifierCase{TypeSpecifierDecimal32}):                                                        c.newPredefinedType(Decimal32),
		ts2String([]TypeSpecifierCase{TypeSpecifierDecimal64}):                                                        c.newPredefinedType(Decimal64),
		ts2String([]TypeSpecifierCase{TypeSpecifierDouble, TypeSpecifierLong}):                                        c.newPredefinedType(LongDouble),
		ts2String([]TypeSpecifierCase{TypeSpecifierDouble}):                                                           c.newPredefinedType(Double),
		ts2String([]TypeSpecifierCase{TypeSpecifierFloat128}):                                                         c.newPredefinedType(Float128),
		ts2String([]TypeSpecifierCase{TypeSpecifierFloat128x}):                                                        c.newPredefinedType(Float128x),
		ts2String([]TypeSpecifierCase{TypeSpecifierFloat16}):                                                          c.newPredefinedType(Float16),
		ts2String([]TypeSpecifierCase{TypeSpecifierFloat32x}):                                                         c.newPredefinedType(Float32x),
		ts2String([]TypeSpecifierCase{TypeSpecifierFloat32}):                                                          c.newPredefinedType(Float32),
		ts2String([]TypeSpecifierCase{TypeSpecifierFloat64x}):                                                         c.newPredefinedType(Float64x),
		ts2String([]TypeSpecifierCase{TypeSpecifierFloat64}):                                                          c.newPredefinedType(Float64),
		ts2String([]TypeSpecifierCase{TypeSpecifierFloat}):                                                            c.newPredefinedType(Float),
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierLong, TypeSpecifierLong, TypeSpecifierSigned}):   longlong,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierLong, TypeSpecifierLong, TypeSpecifierUnsigned}): ulonglog,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierLong, TypeSpecifierLong}):                        longlong,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierLong, TypeSpecifierSigned}):                      long,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierLong, TypeSpecifierUnsigned}):                    ulong,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierLong}):                                           long,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierShort, TypeSpecifierSigned}):                     short,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierShort, TypeSpecifierUnsigned}):                   ushort,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierShort}):                                          short,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierSigned}):                                         int,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt, TypeSpecifierUnsigned}):                                       uint,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt128, TypeSpecifierSigned}):                                      int128,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt128, TypeSpecifierUnsigned}):                                    uint128,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt128}):                                                           int128,
		ts2String([]TypeSpecifierCase{TypeSpecifierInt}):                                                              int,
		ts2String([]TypeSpecifierCase{TypeSpecifierLong, TypeSpecifierLong, TypeSpecifierSigned}):                     longlong,
		ts2String([]TypeSpecifierCase{TypeSpecifierLong, TypeSpecifierLong, TypeSpecifierUnsigned}):                   ulonglog,
		ts2String([]TypeSpecifierCase{TypeSpecifierLong, TypeSpecifierLong}):                                          longlong,
		ts2String([]TypeSpecifierCase{TypeSpecifierLong, TypeSpecifierSigned}):                                        long,
		ts2String([]TypeSpecifierCase{TypeSpecifierLong, TypeSpecifierUnsigned}):                                      ulong,
		ts2String([]TypeSpecifierCase{TypeSpecifierLong}):                                                             long,
		ts2String([]TypeSpecifierCase{TypeSpecifierShort, TypeSpecifierSigned}):                                       short,
		ts2String([]TypeSpecifierCase{TypeSpecifierShort, TypeSpecifierUnsigned}):                                     ushort,
		ts2String([]TypeSpecifierCase{TypeSpecifierShort}):                                                            short,
		ts2String([]TypeSpecifierCase{TypeSpecifierSigned}):                                                           int,
		ts2String([]TypeSpecifierCase{TypeSpecifierUint128}):                                                          uint128,
		ts2String([]TypeSpecifierCase{TypeSpecifierUnsigned}):                                                         uint,
		ts2String([]TypeSpecifierCase{TypeSpecifierVoid}):                                                             c.newPredefinedType(Void),
	}
	c.ast.kinds = map[Kind]Type{}
	for _, v := range c.builtinTypes {
		c.ast.kinds[v.Kind()] = v
	}
	c.intT = c.ast.kinds[Int]
	c.int64T = c.ast.kinds[LongLong]
	c.pcharT = c.newPointerType(c.ast.kinds[Char])
	c.voidT = c.ast.kinds[Void]
	c.pvoidT = c.newPointerType(c.voidT)
	c.implicitFunc = c.newPointerType(c.newFunctionType(c.intT, nil, false))

	ast.Char = c.ast.kinds[Char]
	ast.Double = c.ast.kinds[Double]
	ast.Float = c.ast.kinds[Float]
	ast.Int = c.intT
	ast.Long = c.ast.kinds[Long]
	ast.LongDouble = c.ast.kinds[LongDouble]
	ast.LongLong = c.ast.kinds[LongLong]
	ast.PVoid = c.pvoidT
	ast.SChar = c.ast.kinds[SChar]
	ast.Short = c.ast.kinds[Short]
	ast.UChar = c.ast.kinds[UChar]
	ast.UShort = c.ast.kinds[UShort]
	ast.UInt = c.ast.kinds[UInt]
	ast.ULong = c.ast.kinds[ULong]
	ast.ULongLong = c.ast.kinds[ULongLong]
	ast.Void = c.voidT
	return c
}

//lint:ignore U1000 debug helper
func (c *ctx) indent() string { return strings.Repeat("·   ", c.indentN+1) }

//lint:ignore U1000 debug helper
func (c *ctx) indentDec() string { c.indentN--; return c.indent() }

//lint:ignore U1000 debug helper
func (c *ctx) indentInc() (r string) { r = c.indent(); c.indentN++; return r }

// return a synthesized declarator representing "int nm".
func (c *ctx) predefinedDeclarator(nm Token, s *Scope) *Declarator {
	r := *c.ast.predefinedDeclarator0
	dd := *r.DirectDeclarator
	dd.Token = nm
	r.DirectDeclarator = &dd
	r.isSynthetic = true
	if s != nil {
		s.declare(nil, nm.SrcStr(), &r)
	}
	return &r
}

func (c *ctx) checkScope(s *Scope) {
	for _, ns := range s.Nodes {
		var (
			ds  []*Declarator
			es  []*Enumerator
			ess []*EnumSpecifier
			lds []*LabelDeclaration
			lss []*LabeledStatement
			ps  []*Parameter
			sus []*StructOrUnionSpecifier
		)
		var firstInitialized *Declarator
		initializers := 0
		for _, n := range ns {
			switch x := n.(type) {
			case *Declarator:
				ds = append(ds, x)
				if x.hasInitializer {
					if initializers == 0 {
						firstInitialized = x
					}
					initializers++
					if initializers > 1 {
						c.errors.add(errorf("%v: multiple definitions of '%s', first definition at %v:", x.Position(), x.Name(), firstInitialized.Position()))
					}
				}
			case *Enumerator:
				es = append(es, x)
			case *EnumSpecifier:
				ess = append(ess, x)
			case *LabelDeclaration:
				lds = append(lds, x)
			case *LabeledStatement:
				lss = append(lss, x)
			case *Parameter:
				ps = append(ps, x)
			case *StructOrUnionSpecifier:
				sus = append(sus, x)
			default:
				c.errors.add(errorf("TODO %T", x))
			}
		}
		if len(ds) > 1 {
			a := ds[0]
			t := a.Type()
			switch {
			case s.Parent != nil:
				if t.Kind() != Function {
					c.errors.add(errorf("%v: redeclaration of '%s' at %v:", a.Position(), a.Name(), ds[1].Position()))
					break
				}

				fallthrough
			case s.Parent == nil:
				for _, b := range ds[1:] {
					u := b.Type()
					if !t.IsCompatible(u) {
						c.errors.add(errorf("%v: conflicting types for '%s', previous declaration at %v:, %s and %s", b.Position(), a.Name(), a.Position(), t, u))
						return
					}
				}

				switch {
				case t.Kind() == Function:
					if a.isFuncDef {
						break
					}

					f := t.(*FunctionType)
					for _, b := range ds[1:] {
						if b.isFuncDef {
							t = b.Type()
							break
						}

						if g := b.Type().(*FunctionType); lessFunctionType(f, g) {
							t = g
							f = g
						}
					}
				case t.IsIncomplete():
					for _, b := range ds[1:] {
						if u := b.Type(); !u.IsIncomplete() {
							t = u
							break
						}
					}
				}
				isStatic := false
				for _, v := range ds {
					if v.IsStatic() {
						isStatic = true
						break
					}
				}
				for _, v := range ds {
					v.isStatic = isStatic
					var err error
					if t, err = mergeAttr(t, v.Type().Attributes()); err != nil {
						c.errors.add(errorf("%v: %v", v.Position(), err))
					}
				}
				for _, v := range ds {
					v.typ = t
				}
			}
		}
		if len(ess) > 1 {
			for _, v := range ess[1:] {
				if !ess[0].Type().IsCompatible(v.Type()) {
					c.errors.add(errorf("%v: incompatible redeclaration of '%s' at %v:", v.Position(), v.Type(), ess[0].Position()))
					break
				}
			}
		}
		if len(es) > 1 {
			for _, v := range es[1:] {
				if !es[0].compatibleRedeclaration(v) {
					c.errors.add(errorf("%v: incompatible redeclaration of enumerator '%s' at %v:", v.Position(), v.Token.Src(), es[0].Position()))
					break
				}
			}
		}
		if len(lds) > 1 {
			c.errors.add(errorf("TODO %T", lds[0]))
		}
		if len(lss) > 1 {
			c.errors.add(errorf("TODO %T", lss[0]))
		}
		if len(ps) > 1 {
			c.errors.add(errorf("TODO %T", lss[0]))
		}
		if len(sus) > 1 {
			for _, v := range sus[1:] {
				if !sus[0].Type().IsCompatible(v.Type()) {
					c.errors.add(errorf("%v: incompatible redeclaration of '%s' at %v:", v.Position(), v.Type(), sus[0].Position()))
					break
				}
			}
		}
	}
}

func lessFunctionType(a, b *FunctionType) bool {
	switch p, q := a.Parameters(), b.Parameters(); {
	case len(p) == 0:
		if len(q) != 0 {
			return true
		}
	default:
		if len(q) == 0 {
			return false
		}

		if p[0].Name() != "" {
			return false
		}

		if q[0].Name() != "" {
			return true
		}
	}
	return false
}

func convert(v Value, t Type) (r Value) {
	if v == nil || v == Unknown {
		return Unknown
	}

	if t.Kind() == Enum {
		t = t.(*EnumType).UnderlyingType()
	}
	if IsIntegerType(t) {
		switch {
		case t.Size() <= 0 || t.Size() > 8:
			return Unknown
		case IsSignedInteger(t):
			switch x := v.(type) {
			case Int64Value:
				if t.Size() < 8 {
					sbit := uint64(1) << (t.Size()*8 - 1)
					switch {
					case x&Int64Value(sbit) != 0:
						x |= ^Int64Value(sbit<<1 - 1)
					default:
						x &= Int64Value(sbit - 1)
					}
				}
				return x
			case UInt64Value:
				//TODO- if t.Size() < 8 {
				//TODO- 	m := uint64(1)<<(t.Size()*8) - 1
				//TODO- 	x &= UInt64Value(m)
				//TODO- }
				sbit := uint64(1) << (t.Size()*8 - 1)
				switch {
				case x&UInt64Value(sbit) != 0:
					x |= ^UInt64Value(sbit<<1 - 1)
				default:
					x &= UInt64Value(sbit - 1)
				}
				return Int64Value(x)
			}
		default:
			m := ^UInt64Value(0)
			if t.Size() < 8 {
				m = UInt64Value(1)<<(8*t.Size()) - 1
			}
			switch x := v.(type) {
			case Int64Value:
				return UInt64Value(x) & m
			case UInt64Value:
				return x & m
			}
		}
	}

	switch t.Kind() {
	case Bool:
		switch x := v.(type) {
		case Int64Value:
			if x != 0 {
				return int1
			}

			return int0
		case UInt64Value:
			if x != 0 {
				return int1
			}

			return int0
		}
	case Ptr:
		switch x := v.(type) {
		case Int64Value:
			return UInt64Value(x)
		case UInt64Value:
			return x
		}
	case Void:
		return VoidValue{}
	}
	switch x := v.(type) {
	case Float64Value:
		switch t.Kind() {
		case Double:
			return x
		}
	}

	return Unknown
}

func (c *ctx) decay(t Type, mode flags) Type {
	if !mode.has(decay) || t == nil {
		return t
	}

	return t.Decay()
}

func (c *ctx) wcharT(n Node) Type {
	if c.wcharT0 == nil {
		if s := c.ast.Scope.Nodes["__predefined_wchar_t"]; len(s) != 0 {
			if d, ok := s[0].(*Declarator); ok && d.isTypename && d.Type() != Invalid {
				c.wcharT0 = d.Type()
			}
		}
		if c.wcharT0 == nil {
			if s := c.ast.Scope.Nodes["wchar_t"]; len(s) != 0 {
				if d, ok := s[0].(*Declarator); ok && d.isTypename && d.Type() != Invalid {
					c.wcharT0 = d.Type()
				}
			}
		}
		if c.wcharT0 == nil {
			c.wcharT0, _ = c.kindToType(c.cfg.DefaultWcharT)
		}
		if c.wcharT0 == nil {
			c.errors.add(errorf("%v: undefined type: wchar_t, falling back to int", n.Position()))
			c.wcharT0 = c.intT
		}
		if !IsIntegerType(c.wcharT0) {
			c.errors.add(errorf("%v: invalid type: wchar_t is not an integer type", n.Position()))
		}
	}
	return c.wcharT0
}

func (c *ctx) ptrDiffT(n Node) Type {
	if c.ptrDiffT0 == nil {
		if s := c.ast.Scope.Nodes["__predefined_ptrdiff_t"]; len(s) != 0 {
			if d, ok := s[0].(*Declarator); ok && d.isTypename && d.Type() != Invalid {
				c.ptrDiffT0 = d.Type()
			}
		}
		if c.ptrDiffT0 == nil {
			if s := c.ast.Scope.Nodes["ptrdiff_t"]; len(s) != 0 {
				if d, ok := s[0].(*Declarator); ok && d.isTypename && d.Type() != Invalid {
					c.ptrDiffT0 = d.Type()
				}
			}
		}
		if c.ptrDiffT0 == nil {
			c.ptrDiffT0, _ = c.kindToType(c.cfg.DefaultPtrdiffT)
		}
		if c.ptrDiffT0 == nil {
			c.errors.add(errorf("%v: undefined type: ptrdiff_t", n.Position()))
			c.ptrDiffT0 = c.intT
		}
		if !IsIntegerType(c.ptrDiffT0) {
			c.errors.add(errorf("%v: invalid type: ptrdiff_t is not an integer type", n.Position()))
		}
	}
	return c.ptrDiffT0
}

func (c *ctx) sizeT(n Node) Type {
	if c.sizeT0 == nil {
		if s := c.ast.Scope.Nodes["__predefined_size_t"]; len(s) != 0 {
			if d, ok := s[0].(*Declarator); ok && d.isTypename && d.Type() != Invalid {
				c.sizeT0 = d.Type()
			}
		}
		if c.sizeT0 == nil {
			if s := c.ast.Scope.Nodes["size_t"]; len(s) != 0 {
				if d, ok := s[0].(*Declarator); ok && d.isTypename && d.Type() != Invalid {
					c.sizeT0 = d.Type()
				}
			}
		}
		if c.sizeT0 == nil {
			c.sizeT0, _ = c.kindToType(c.cfg.DefaultSizeT)
		}
		if c.sizeT0 == nil {
			c.errors.add(errorf("%v: undefined type: size_t", n.Position()))
			c.sizeT0 = c.intT
		}
		if !IsIntegerType(c.sizeT0) {
			c.errors.add(errorf("%v: invalid type: size_t is not an integer type", n.Position()))
		}
	}
	return c.sizeT0
}

func (c *ctx) kindToType(k Kind) (t Type, err error) {
	switch k {
	case Char:
		return c.ast.Char, nil
	case Int:
		return c.ast.Int, nil
	case Long:
		return c.ast.Long, nil
	case LongLong:
		return c.ast.LongLong, nil
	case SChar:
		return c.ast.SChar, nil
	case Short:
		return c.ast.Short, nil
	case UChar:
		return c.ast.UChar, nil
	case UInt:
		return c.ast.UInt, nil
	case ULong:
		return c.ast.ULong, nil
	case ULongLong:
		return c.ast.ULongLong, nil
	case UShort:
		return c.ast.UShort, nil
	default:
		return nil, errorf("kindToType: invalid kind: %v", k)
	}
}

func (c *ctx) fixSyntheticDeclarators(s *Scope) {
	for _, v := range s.Children {
		c.fixSyntheticDeclarators(v)
	}

	p := s.Parent
	if p == nil {
		return
	}

	for _, vs := range s.Nodes {
		for _, v := range vs {
			switch x := v.(type) {
			case *Declarator:
				if !x.IsSynthetic() {
					break
				}

				t := x.NameTok()
				t.seq = math.MaxInt32
				if d := p.ident(t); d != nil {
					if y, ok := d.(*Declarator); ok && !y.IsSynthetic() {
						*x = *y
					}
				}
			}
		}
	}
}

type resolver struct{ resolved *Scope }

// ResolvedIn returns the scope an identifier was resolved in, if any.
func (n resolver) ResolvedIn() *Scope { return n.resolved }

type AST struct {
	ABI    *ABI
	EOF    Token
	Macros map[string]*Macro
	Scope  *Scope // File scope.
	// All struct types created during type checking.
	Structs         map[*StructType]struct{}
	TranslationUnit *TranslationUnit
	// All union types created during type checking.
	Unions map[*UnionType]struct{}

	// These field are valid only after Translate
	Char       Type
	Double     Type
	Float      Type
	Int        Type
	Long       Type
	LongDouble Type
	LongLong   Type
	PVoid      Type
	SChar      Type
	Short      Type
	SizeT      Type
	UChar      Type
	UInt       Type
	ULong      Type
	ULongLong  Type
	UShort     Type
	Void       Type

	kinds                 map[Kind]Type
	predefinedDeclarator0 *Declarator // `int __predefined_declarator`
}

func (n *AST) check(cfg *Config) error {
	n.Structs = map[*StructType]struct{}{}
	n.Unions = map[*UnionType]struct{}{}
	c := newCtx(n, cfg)

	defer func() { c.cfg = nil }()

	for l := n.TranslationUnit; l != nil; l = l.TranslationUnit {
		l.ExternalDeclaration.check(c)
	}
	c.fixSyntheticDeclarators(n.Scope)
	c.checkScope(n.Scope)
	n.SizeT = c.sizeT(n.EOF)
	if n.predefinedDeclarator0 != nil {
		// Remove __predefined_declarator
		var prev *TranslationUnit
	loop:
		for l := n.TranslationUnit; l != nil; l = l.TranslationUnit {
			switch ed := l.ExternalDeclaration; ed.Case {
			case ExternalDeclarationDecl:
				if d := ed.Declaration.InitDeclaratorList.InitDeclarator.Declarator; d == n.predefinedDeclarator0 {
					if prev != nil {
						prev.TranslationUnit = l.TranslationUnit
					}
					break loop
				}
			}
			prev = l
		}
	}
	return c.errors.err()
}

// ExternalDeclaration:
//
//	FunctionDefinition  // Case ExternalDeclarationFuncDef
//	|       Declaration         // Case ExternalDeclarationDecl
//	|       AsmStatement        // Case ExternalDeclarationAsmStmt
//	|       ';'                 // Case ExternalDeclarationEmpty
func (n *ExternalDeclaration) check(c *ctx) {
	switch n.Case {
	case ExternalDeclarationFuncDef: // FunctionDefinition
		n.FunctionDefinition.check(c)
	case ExternalDeclarationDecl: // Declaration
		n.Declaration.check(c)
	case ExternalDeclarationAsmStmt: // AsmStatement
		n.AsmStatement.check(c)
	case ExternalDeclarationEmpty: // ';'
		c.errors.add(errorf("TODO %v", n.Case))
	case ExternalDeclarationObjCInterface,
		ExternalDeclarationObjCImplementation,
		ExternalDeclarationObjCProtocol:
		// ObjC declarations: skip type checking (handled in ObjC lowerer)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

// AsmStatement:
//
//	Asm ';'
func (n *AsmStatement) check(c *ctx) {
	n.Asm.check(c)
}

// Asm:
//
//	"asm" AsmQualifierList '(' STRINGLITERAL AsmArgList ')'
func (n *Asm) check(c *ctx) {
	n.AsmQualifierList.check(c)
	n.AsmArgList.check(c)
}

// AsmArgList:
//
//	':' AsmExpressionList
//
// |       AsmArgList ':' AsmExpressionList
func (n *AsmArgList) check(c *ctx) {
	for ; n != nil; n = n.AsmArgList {
		n.AsmExpressionList.check(c)
	}
}

// AsmExpressionList:
//
//	AsmIndex AssignmentExpression
//
// |       AsmExpressionList ',' AsmIndex AssignmentExpression
func (n *AsmExpressionList) check(c *ctx) {
	for ; n != nil; n = n.AsmExpressionList {
		n.AsmIndex.check(c)
		n.AssignmentExpression.check(c, decay|asmArgList|ignoreUndefined)
		n.AssignmentExpression.eval(c, decay|ignoreUndefined)
	}
}

// AsmIndex:
//
//	'[' ExpressionList ']'
func (n *AsmIndex) check(c *ctx) {
	if n == nil {
		return
	}

	if n.ExpressionList != nil {
		n.ExpressionList.check(c, decay|asmArgList|ignoreUndefined)
		n.ExpressionList.eval(c, decay|ignoreUndefined)
	}
}

func (n *AsmQualifierList) check(c *ctx) {
	for ; n != nil; n = n.AsmQualifierList {
		n.AsmQualifier.check(c)
	}
}

// AsmQualifierList:
//
//	AsmQualifier
//
// |       AsmQualifierList AsmQualifier
func (n *AsmQualifier) check(c *ctx) {
	switch n.Case {
	case AsmQualifierVolatile: // "volatile"
		//TODO c.errors.add(errorf("TODO %v", n.Case))
	case AsmQualifierInline: // "inline"
		c.errors.add(errorf("TODO %v", n.Case))
	case AsmQualifierGoto: // "goto"
		//TODO c.errors.add(errorf("TODO %v", n.Case))
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

// FunctionDefinition:
//
//	DeclarationSpecifiers Declarator DeclarationList CompoundStatement
func (n *FunctionDefinition) check(c *ctx) {
	t := c.checkFunctionDefinition(n.scope, n.DeclarationSpecifiers, n.Declarator, n.DeclarationList, n.CompoundStatement)
	if ft, ok := t.(*FunctionType); ok {
		if n.DeclarationSpecifiers != nil {
			if t := n.DeclarationSpecifiers.Type(); t != nil {
				if a := t.Attributes(); a != nil {
					ft = ft.setAttr(a).(*FunctionType)
					n.Declarator.typ = ft
					if s := a.Visibility(); s != "" && a.VisibilityDecl() == nil {
						a.setVisibilityDecl(n.Declarator)
					}
				}
			}
		}

		if c.usesVectors {
			n.usesVectors = true
			return
		}

		if isVectorType(ft.Result()) {
			n.usesVectors = true
			return
		}

		for _, v := range ft.Parameters() {
			if isVectorType(v.Type()) {
				n.usesVectors = true
				return
			}
		}
	}
}

func (c *ctx) checkFunctionDefinition(sc *Scope, ds *DeclarationSpecifiers, d *Declarator, dl *DeclarationList, cs *CompoundStatement) Type {
	c.usesVectors = false
	d.check(c, ds.check(c, &d.isExtern, &d.isStatic, &d.isAtomic, &d.isThreadLocal, &d.isConst, &d.isVolatile, &d.isInline, &d.isRegister, &d.isAuto, &d.isNoreturn, &d.isRestrict, &d.alignas))
	if x, ok := d.Type().(*FunctionType); ok {
		x.hasImplicitResult = true
	}
	switch d.DirectDeclarator.Case {
	case DirectDeclaratorFuncIdent:
		ft, ok := d.Type().(*FunctionType)
		if !ok {
			break
		}

		m := dl.check(c)
		for _, param := range d.DirectDeclarator.IdentifierList.parameters {
			switch d := m[param.name.SrcStr()]; {
			case d == nil:
				d := c.predefinedDeclarator(param.name, nil)
				d.isExtern = false
				d.isParam = true
				d.lexicalScope = (*lexicalScope)(d.DirectDeclarator.params)
				d.resolved = (*Scope)(d.lexicalScope)
				param.Declarator = d
				param.typ = c.intT
			default:
				d.isExtern = false
				d.isParam = true
				d.lexicalScope = (*lexicalScope)(d.DirectDeclarator.params)
				d.resolved = (*Scope)(d.lexicalScope)
				param.Declarator = d
				param.typ = d.Type().Decay()
				d.typ = param.typ
			}
			ft.fp = append(ft.fp, param)
		}
		ft2 := c.newFunctionType2(ft.Result(), ft.fp)
		ft2.hasImplicitResult = ft.hasImplicitResult
		d.typ = ft2
	}
	var sc0 *Scope
	sc0, c.fnScope = c.fnScope, sc
	defer func() { c.fnScope = sc0 }()
	if !c.cfg.Header {
		cs.check(c)
	}
	return d.Type()
}

// DeclarationList:
//
//	Declaration
//
// |       DeclarationList Declaration
func (n *DeclarationList) check(c *ctx) (m map[string]*Declarator) {
	for ; n != nil; n = n.DeclarationList {
		n.Declaration.check(c)
		if m == nil {
			m = map[string]*Declarator{}
		}
		for l := n.Declaration.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			d := l.InitDeclarator.Declarator
			nm := d.Name()
			if x := m[nm]; x != nil {
				c.errors.add(errorf("%v: %s redeclared, previous declaration at %v:", d.Position(), nm, x.Position()))
				continue
			}

			m[nm] = d
		}
	}
	return m
}

// CompoundStatement:
//
//	'{' LabelDeclarationList BlockItemList '}'
func (n *CompoundStatement) check(c *ctx) (r Type) {
	if n == nil {
		return
	}

	r = c.voidT
	for l := n.BlockItemList; l != nil; l = l.BlockItemList {
		r = l.BlockItem.check(c)
	}
	c.checkScope(n.LexicalScope())
	return r
}

func (n *BlockItem) check(c *ctx) (r Type) {
	if n == nil {
		return Invalid
	}

	switch n.Case {
	case BlockItemDecl: // Declaration
		n.Declaration.check(c)
	case BlockItemLabel:
		n.LabelDeclaration.check(c)
	case BlockItemStmt: // Statement
		return n.Statement.check(c)
	case BlockItemFuncDef: // DeclarationSpecifiers Declarator CompoundStatement
		sv := c.usesVectors
		c.usesVectors = false

		defer func() {
			c.usesVectors = sv
		}()

		c.checkFunctionDefinition(n.CompoundStatement.LexicalScope(), n.DeclarationSpecifiers, n.Declarator, nil, n.CompoundStatement)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return c.voidT
}

func (n *LabelDeclaration) check(c *ctx) {
	//TODO
}

func (n *Statement) check(c *ctx) (r Type) {
	if n == nil {
		return Invalid
	}

	switch n.Case {
	case StatementLabeled: // LabeledStatement
		return n.LabeledStatement.check(c)
	case StatementCompound: // CompoundStatement
		return n.CompoundStatement.check(c)
	case StatementExpr: // ExpressionStatement
		return n.ExpressionStatement.check(c)
	case StatementSelection: // SelectionStatement
		n.SelectionStatement.check(c)
	case StatementIteration: // IterationStatement
		n.IterationStatement.check(c)
	case StatementJump: // JumpStatement
		n.JumpStatement.check(c)
	case StatementAsm: // AsmStatement
		n.AsmStatement.check(c)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return c.voidT
}

func (n *LabeledStatement) check(c *ctx) (r Type) {
	if n == nil {
		return nil
	}

	switch n.Case {
	case LabeledStatementLabel: // IDENTIFIER ':' Statement
		return n.Statement.check(c)
	case LabeledStatementCaseLabel: // "case" ConstantExpression ':' Statement
		if c.inSwitch == 0 {
			c.errors.add(errorf("%v: case label not within a switch statement", n.Position()))
		}
		n.caseOrdinal = c.switchCases
		c.switchCases++
		n.ConstantExpression.check(c, decay)
		n.Statement.check(c)
	case LabeledStatementRange: // "case" ConstantExpression "..." ConstantExpression ':' Statement
		if c.inSwitch == 0 {
			c.errors.add(errorf("%v: case label not within a switch statement", n.Position()))
		}
		n.caseOrdinal = c.switchCases
		c.switchCases++
		n.ConstantExpression.check(c, decay)
		n.ConstantExpression2.check(c, decay)
		n.Statement.check(c)
	case LabeledStatementDefault: // "default" ':' Statement
		if c.inSwitch == 0 {
			c.errors.add(errorf("%v: default label not within a switch statement", n.Position()))
		}
		n.caseOrdinal = c.switchCases
		c.switchCases++
		n.Statement.check(c)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return c.voidT
}

func (n *IterationStatement) check(c *ctx) {
	if n == nil {
		return
	}

	switch n.Case {
	case IterationStatementWhile: // "while" '(' ExpressionList ')' Statement
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
		c.inLoop++
		defer func() { c.inLoop-- }()
		n.Statement.check(c)
	case IterationStatementDo: // "do" Statement "while" '(' ExpressionList ')' ';'
		c.inLoop++
		defer func() { c.inLoop-- }()
		n.Statement.check(c)
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
	case IterationStatementFor: // "for" '(' ExpressionList ';' ExpressionList ';' ExpressionList ')' Statement
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
		if n.ExpressionList2 != nil {
			n.ExpressionList2.check(c, decay)
			n.ExpressionList2.eval(c, decay)
		}
		if n.ExpressionList3 != nil {
			n.ExpressionList3.check(c, decay)
			n.ExpressionList3.eval(c, decay)
		}
		c.inLoop++
		defer func() { c.inLoop-- }()
		n.Statement.check(c)
	case IterationStatementForDecl: // "for" '(' Declaration ExpressionList ';' ExpressionList ')' Statement
		n.Declaration.check(c)
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
		if n.ExpressionList2 != nil {
			n.ExpressionList2.check(c, decay)
			n.ExpressionList2.eval(c, decay)
		}
		c.inLoop++
		defer func() { c.inLoop-- }()
		n.Statement.check(c)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

func (n *JumpStatement) check(c *ctx) {
	if n == nil {
		return
	}

out:
	switch n.Case {
	case JumpStatementGoto: // "goto" IDENTIFIER ';'
		for _, nd := range c.fnScope.Nodes[string(n.Token2.Src())] {
			switch nd.(type) {
			case *LabeledStatement:
				break out
			}
		}

		c.errors.add(errorf("%v: undefined label: %s", n.Token2.Position(), n.Token2.Src()))
	case JumpStatementGotoExpr: // "goto" '*' ExpressionList ';'
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
	case JumpStatementContinue: // "continue" ';'
		if c.inLoop == 0 {
			c.errors.add(errorf("%v: continue statement not within a loop", n.Position()))
		}
	case JumpStatementBreak: // "break" ';'
		if c.inLoop+c.inSwitch == 0 {
			c.errors.add(errorf("%v: break statement not within loop or switch", n.Position()))
		}
	case JumpStatementReturn: // "return" ExpressionList ';'
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
		//TODO check assignable to fn result
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

func (n *SelectionStatement) check(c *ctx) {
	if n == nil {
		return
	}

	switch n.Case {
	case SelectionStatementIf: // "if" '(' ExpressionList ')' Statement
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
		n.Statement.check(c)
	case SelectionStatementIfElse: // "if" '(' ExpressionList ')' Statement "else" Statement
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
		n.Statement.check(c)
		n.Statement2.check(c)
	case SelectionStatementSwitch: // "switch" '(' ExpressionList ')' Statement
		if n.ExpressionList != nil {
			n.ExpressionList.check(c, decay)
			n.ExpressionList.eval(c, decay)
		}
		c.inSwitch++
		switchCases := c.switchCases
		c.switchCases = 0
		defer func() {
			c.inSwitch--
			n.switchCases = c.switchCases
			c.switchCases = switchCases
		}()
		n.Statement.check(c)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

func (n *ExpressionStatement) check(c *ctx) (r Type) {
	if n == nil {
		return nil
	}

	if n.ExpressionList == nil {
		return c.voidT
	}

	if n.ExpressionList != nil {
		r = n.ExpressionList.check(c, decay)
		n.ExpressionList.eval(c, decay)
	}
	return r
}

func (n *Declaration) check(c *ctx) {
	switch n.Case {
	case DeclarationDecl: // DeclarationSpecifiers InitDeclaratorList AttributeSpecifierList ';'
		var isExtern, isStatic, isAtomic, isThreadLocal, isConst, isVolatile, isInline, isRegister, isAuto, isNoreturn, isRestrict bool
		var alignas int
		var err error
		t := n.DeclarationSpecifiers.check(c, &isExtern, &isStatic, &isAtomic, &isThreadLocal, &isConst, &isVolatile, &isInline, &isRegister, &isAuto, &isNoreturn, &isRestrict, &alignas)
		if t == nil {
			return
		}

		dsa := t.Attributes()
		attr := n.AttributeSpecifierList.check(c)
		asa := attr
		if t != nil {
			if a := t.Attributes(); a != nil {
				if attr, err = attr.merge(n, a); err != nil {
					c.errors.add(errorf("%v", err))
				}
			}
		}
		for l := n.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
			l.InitDeclarator.check(c, t, isExtern, isStatic, isAtomic, isThreadLocal, isConst, isVolatile, isInline, isRegister, isAuto, alignas)
			d := l.InitDeclarator.Declarator
			if d != nil && d.Type() != nil && attr != nil {
				if d.typ, err = mergeAttr(d.Type(), attr); err != nil {
					c.errors.add(errorf("%v", err))
				}
				if s := d.Type().Attributes().Visibility(); s != "" && d.Type().Attributes().VisibilityDecl() == nil {
					d.Type().Attributes().setVisibilityDecl(d)
				}
			}
			d.fixVolatileAndConst(c)
			if dsa != nil && d.Pointer != nil && asa == nil {
				d.fixPointerAttr(c)
			}
		}
	case DeclarationAssert: // StaticAssertDeclaration
		n.StaticAssertDeclaration.check(c)
	case DeclarationAuto: // "__auto_type" Declarator '=' Initializer ';'
		if n.Initializer.Case != InitializerExpr {
			c.errors.add(errorf("%v: expected assignment expression", n.Initializer.Position()))
			break
		}

		n.Declarator.typ = n.Initializer.AssignmentExpression.check(c, decay)
		n.Declarator.write++
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

// StaticAssertDeclaration:
//
//	"_Static_assert" '(' ConstantExpression ',' STRINGLITERAL ')'
func (n *StaticAssertDeclaration) check(c *ctx) {
	n.ConstantExpression.check(c, decay)
	if !isNonzero(n.ConstantExpression.Value()) {
		s := stringConst(func(msg string, args ...interface{}) {
			c.errors.add(errorf(msg, args...))
		}, n.Token4)
		c.errors.add(errorf("%v: assertion failed: %s", n.ConstantExpression.Position(), s[:len(s)-1]))
	}
}

// InitDeclarator:
//
//	Declarator Asm                  // Case InitDeclaratorDecl
//
// |       Declarator Asm '=' Initializer  // Case InitDeclaratorInit
func (n *InitDeclarator) check(c *ctx, t Type, isExtern, isStatic, isAtomic, isThreadLocal, isConst, isVolatile, isInline, isRegister, isAuto bool, alignas int) {
	if t == nil {
		c.errors.add(errorf("TODO %T internal error", n))
		return
	}

	if n == nil {
		return
	}

	n.Declarator.isAtomic = isAtomic
	n.Declarator.isAuto = isAuto
	n.Declarator.isConst = isConst
	n.Declarator.isExtern = isExtern
	n.Declarator.isInline = isInline
	n.Declarator.isRegister = isRegister
	n.Declarator.isStatic = isStatic
	n.Declarator.isThreadLocal = isThreadLocal
	n.Declarator.isVolatile = isVolatile
	n.Declarator.alignas = alignas
	t = n.Declarator.check(c, t)
	if attr := n.AttributeSpecifierList.check(c); attr != nil {
		var err error
		if n.Declarator.typ, err = mergeAttr(n.Declarator.Type(), attr); err != nil {
			c.errors.add(errorf("%v", err))
		}
	}
	if n.Asm != nil {
		n.Asm.check(c)
	}
	if n.Case == InitDeclaratorInit {
		u := t.clone()
		n.Initializer.check(c, nil, u, 0, nil, nil)
		if t.Kind() == Array && t.IsIncomplete() {
			n.Declarator.typ = u
		}
		n.Declarator.write++
	}
}

// Initializer:
//
//		AssignmentExpression         // Case InitializerExpr
//	|       '{' InitializerList ',' '}'  // Case InitializerInitList
func (n *Initializer) check(c *ctx, currObj, t Type, off int64, l *InitializerList, f *Field) (r *InitializerList) {
	if n == nil || t == nil {
		c.errors.add(errorf("internal error %T(%v) %T(%v)", n, n == nil, t, t == nil))
		return nil
	}

	// fs := ""
	// if f != nil {
	// 	fs = fmt.Sprintf(" (FIELD %q: type %v, off %v '%s')", f.Name(), f.Type(), f.Offset(), NodeSource(n.AssignmentExpression))
	// }
	// trc("==== (%v: %v: %v:)", origin(4), origin(3), origin(2))
	// trc("%sINITIALIZER (A) %v: t %s, off %v, l %p, f %s (n %q, l %q)", c.indentInc(), n.Position(), t, off, l, fs, NodeSource(n), NodeSource(l))
	// defer func() {
	// 	trc("%sEXIT INITIALIZER (Z) %v: t %s, off %v, l %p, f %p (n %q, r -> %q)", c.indentDec(), n.Position(), t, off, l, f, NodeSource(n), NodeSource(r))
	// }()
	n.field = f
	if l != nil {
		r = l.InitializerList
	}

	// The type of the entity to be initialized shall be an array of unknown size
	// or an object type that is not a variable length array type.
	n.typ = t
	n.off = off
	t = n.Type()
	if x, ok := t.(*ArrayType); ok && x.IsVLA() {
		c.errors.add(errorf("%v: cannot initalize a variable length array", n.Position()))
		return r
	}

	// trc("%sINIT (a) %v noff %v, Case %v, nt %s, curr %s, t %s, list %p (%v:)", c.indentInc(), n.Position(), n.Offset(), n.Case, n.Type(), currObj, t, l, origin(2))
	// defer func() { trc("%sEXIT INIT (z)", c.indentDec()) }()
	switch n.Case {
	case InitializerExpr: // AssignmentExpression
		exprT := n.AssignmentExpression.check(c, decay)
		if exprT == Invalid {
			n.val = Unknown
			c.errors.add(errorf("TODO %T <- %T", t, exprT))
			return
		}

		n.val = n.AssignmentExpression.eval(c, decay)
		switch x := t.(type) {
		case *ArrayType:
			return n.checkArray(c, currObj, x, exprT, off, l)
		case *StructType:
			return n.checkStruct(c, currObj, x, exprT, off, l)
		case *UnionType:
			return n.checkUnion(c, currObj, x, exprT, off, l)
		case *PredefinedType:
			n.val = convert(n.Value(), t)
			return r
		case *EnumType:
			n.val = convert(n.Value(), t)
			return r
		case *PointerType:
			if isPointerType(exprT) || IsIntegerType(exprT) {
				if n.Value() != Unknown {
					n.val = convert(n.Value(), t)
				}
				return r
			}

			c.errors.add(errorf("TODO %T <- val %T, currObj %s, t %s, exprT %s", x, n.Value(), currObj, t, exprT))
			return nil
		default:
			c.errors.add(errorf("TODO %T", x))
			return nil
		}
	case InitializerInitList: // '{' InitializerList ',' '}'
		if n.InitializerList == nil {
			n.val = Zero
			return r
		}

		if n.InitializerList.InitializerList == nil {
			switch x := t.(type) {
			case *ArrayType:
				switch x.Elem().Kind() {
				case Char, SChar, UChar:
					e := n.InitializerList.Initializer.AssignmentExpression
					exprT := e.check(c, decay)
					if exprT == Invalid {
						n.val = Unknown
						c.errors.add(errorf("TODO %T <- %T", t, exprT))
						return
					}

					val := e.eval(c, decay)
					switch val.(type) {
					case StringValue:
						n.InitializerList.Initializer.check(c, t, t, off, l, f)
						return r
					}
				}
			}
		}

		if n := n.InitializerList.check(c, t, t, off); n != nil {
			c.errors.add(errorf("%v: too many items in initializer", n.Position()))
		}
		return r
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
		return nil
	}
}

func (n *Initializer) checkArray(c *ctx, currObj Type, t *ArrayType, exprT Type, off int64, l *InitializerList) (r *InitializerList) {
	if n.Case != InitializerExpr {
		c.errors.add(errorf("internal error: %T %v", n, n.Case))
		return nil
	}

	if l != nil {
		r = l.InitializerList
	}
	v := n.Value()
	switch x := t.Elem().(type) {
	case *PredefinedType:
		// [0]6.7.8/14 An array of character type may be initialized by a character
		// string literal, optionally enclosed in braces. Successive characters of the
		// character string literal (including the terminating null character if there
		// is room or if the array is of unknown size) initialize the elements of the
		// array.
		switch x.Kind() {
		case Char, SChar, UChar:
			if x, ok := v.(StringValue); ok {
				if t.IsIncomplete() {
					t.elems = int64(len(x))
				}
				if max, sz := t.Len(), int64(len(x)); sz > max {
					n.val = x[:max]
				}
				return r
			}
		}

		// [0]6.7.8/15 An array with element type compatible with wchar_t may be
		// initialized by a wide string literal, optionally enclosed in braces.
		// Successive wide characters of the wide string literal (including the
		// terminating null wide character if there is room or if the array is of
		// unknown size) initialize the elements of the array.
		if IsIntegerType(x) && x.Size() == c.wcharT(n).Size() {
			switch x := v.(type) {
			case StringValue:
				if t.IsIncomplete() {
					t.elems = int64(len(x))
				}
				if max, sz := t.Len(), int64(len(x)); sz > max {
					n.val = x[:max]
				}
				return r
			case UTF16StringValue:
				if t.IsIncomplete() {
					t.elems = int64(len(x))
				}
				if max, sz := t.Len(), int64(len(x)); sz > max {
					n.val = x[:max]
				}
				return r
			case UTF32StringValue:
				if t.IsIncomplete() {
					t.elems = int64(len(x))
				}
				if max, sz := t.Len(), int64(len(x)); sz > max {
					n.val = x[:max]
				}
				return r
			}
		}
	}

	if l == nil {
		switch x := exprT.Undecay().(type) {
		case *ArrayType:
			switch {
			case t.Len() < 0 && x.Len() > 0:
				t.elems = x.Len()
			case t.Len() > 0 && x.Len() > 0:
				// ok
			default:
				c.errors.add(errorf("TODO %v %v", t.Len(), x.Len()))
			}
		case *PredefinedType:
			if x.vector != nil {
				return n.checkArray(c, currObj, t, x.vector, off, l)
			}

			if x.IsCompatible(t.Elem()) || IsArithmeticType(x) && IsArithmeticType(t.Elem()) {
				if n.Value() != Unknown {
					n.val = convert(n.Value(), t)
				}
				return nil
			}

			c.errors.add(errorf("TODO curr %s, t %s <- %T", currObj, t, x))
		default:
			c.errors.add(errorf("TODO curr %s, t %s <- %T", currObj, t, x))
		}
		return nil
	}

	return l.check(c, currObj, t, off)
}

func (n *Initializer) checkStruct(c *ctx, currObj Type, t *StructType, exprT Type, off int64, l *InitializerList) (r *InitializerList) {
	if n.Case != InitializerExpr {
		c.errors.add(errorf("internal error: %T %v", n, n.Case))
		return
	}

	if l != nil {
		r = l.InitializerList
	}
	if x, ok := exprT.(*StructType); ok && t.IsCompatible(x) {
		return r
	}

	if l == nil {
		f := t.FieldByIndex(0)
		if f == nil {
			c.errors.add(errorf("TODO %T <- %T %T", t, n.Value(), exprT))
			return nil
		}

		n.check(c, currObj, f.Type(), off, nil, f)
		return nil
	}

	return l.check(c, currObj, t, off)
}

func (n *Initializer) checkUnion(c *ctx, currObj Type, t *UnionType, exprT Type, off int64, l *InitializerList) (r *InitializerList) {
	if n.Case != InitializerExpr {
		c.errors.add(errorf("internal error: %T %v", n, n.Case))
		return
	}

	if l != nil {
		r = l.InitializerList
	}
	if x, ok := exprT.(*UnionType); ok && t.IsCompatible(x) {
		return r
	}

	if l == nil {
		f := t.FieldByIndex(0)
		if f == nil {
			c.errors.add(errorf("TODO %T <- %T %T", t, n.Value(), exprT))
			return nil
		}

		n.check(c, currObj, f.Type(), off, nil, f)
		return nil
	}

	return l.check(c, currObj, t, off)
}

func (n *InitializerList) check(c *ctx, currObj, t Type, off int64) (r *InitializerList) {
	if n == nil || currObj == nil || t == nil {
		c.errors.add(errorf("internal error: %T %T", n, currObj))
		return nil
	}

	n.typ = t
	t = n.Type()

	// trc("%sLIST (A) %v: curr %s, t %s, designation %p (%q)", c.indentInc(), n.Position(), currObj, t, n.Designation, NodeSource(n))
	// defer func() {
	// 	trc("%sEXIT LIST (Z) %v: curr %s, t %s, designation %p (r -> %q)", c.indentDec(), n.Position(), currObj, t, n.Designation, NodeSource(r))
	// }()
	switch x := t.(type) {
	case *ArrayType:
		return n.checkArray(c, currObj, x, off)
	case *StructType:
		return n.checkStruct(c, currObj, x, off)
	case *UnionType:
		return n.checkUnion(c, currObj, x, off)
	case *PredefinedType:
		if x.VectorSize() > 0 {
			return n.checkArray(c, currObj, x.vector, off)
		}

		return n.checkPredefined(c, currObj, x, off)
	case *PointerType:
		return n.checkPointer(c, currObj, x, off)
	case *EnumType:
		return n.checkEnum(c, currObj, x, off)
	default:
		c.errors.add(errorf("TODO %T", x))
		return nil
	}
}

func (n *InitializerList) checkEnum(c *ctx, currObj Type, t *EnumType, off int64) *InitializerList {
	if n.Designation != nil {
		c.errors.add(errorf("%v: unexpected designation", n.Designation.Position()))
		return nil
	}

	if IsScalarType(t) {
		n.Initializer.check(c, currObj, t, off, nil, nil)
		return n.InitializerList
	}

	c.errors.add(errorf("TODO %T", t))
	return nil
}

func (n *InitializerList) checkPointer(c *ctx, currObj Type, t *PointerType, off int64) *InitializerList {
	if n.Designation != nil {
		c.errors.add(errorf("%v: unexpected designation", n.Designation.Position()))
		return nil
	}

	if IsScalarType(t) {
		n.Initializer.check(c, currObj, t, off, nil, nil)
		return n.InitializerList
	}

	c.errors.add(errorf("TODO %T", t))
	return nil
}

func (n *InitializerList) checkPredefined(c *ctx, currObj Type, t *PredefinedType, off int64) *InitializerList {
	if n.Designation != nil {
		c.errors.add(errorf("%v: unexpected designation", n.Designation.Position()))
		return nil
	}

	if IsScalarType(t) {
		n.Initializer.check(c, currObj, t, off, nil, nil)
		return n.InitializerList
	}

	c.errors.add(errorf("TODO %T", t))
	return nil
}

func (n *InitializerList) checkArray(c *ctx, currObj Type, t *ArrayType, off int64) *InitializerList {
	// trc("%sARRAY %v: curr %s, t %s", c.indentInc(), n.Position(), currObj, t)
	// defer func() { trc("%sEXIT ARRAY", c.indentDec()) }()
	elemT := t.Elem()
	var lo, hi int64
	switch {
	case t.IsIncomplete():
		for n != nil {
			if n.Designation == nil {
				n = n.Initializer.check(c, currObj, elemT, off+lo*elemT.Size(), n, nil)
				lo++
				t.elems = mathutil.MaxInt64(t.elems, lo)
				continue
			}

			if currObj != t {
				return n
			}

			dl := n.Designation.DesignatorList
			dl.Designator.typ = elemT
			n2 := *n
			n2.Designation = nil
			n = &n2
			lo, hi = dl.Designator.index(c)
			if lo < 0 {
				return nil
			}

			if dl.DesignatorList == nil {
				continue
			}

			n = n.checkDesignatorList(dl, c, elemT, off+lo*elemT.Size(), hi >= 0)
			lo = mathutil.MaxInt64(lo, hi)
			lo++
		}
		return nil
	default:
		for n != nil {
			if n.Designation == nil {
				if lo >= t.elems {
					return n
				}

				n = n.Initializer.check(c, currObj, elemT, off+lo*elemT.Size(), n, nil)
				lo++
				continue
			}

			if currObj != t {
				return n
			}

			dl := n.Designation.DesignatorList
			dl.Designator.typ = elemT
			n2 := *n
			n2.Designation = nil
			n = &n2
			lo, hi = dl.Designator.index(c)
			if lo < 0 {
				return nil
			}

			if lo >= t.elems {
				c.errors.add(errorf("%v: index %v out of range for array of %v elements", dl.Designator.Position(), lo, t.elems))
				return nil
			}

			if hi >= t.elems {
				c.errors.add(errorf("%v: index %v out of range for array of %v elements", dl.Designator.Position(), hi, t.elems))
				return nil
			}

			switch {
			case hi < 0:
				n.Initializer.nelems = 1
			default:
				n.Initializer.nelems = hi - lo + 1
			}

			if dl.DesignatorList == nil {
				continue
			}

			n = n.checkDesignatorList(dl.DesignatorList, c, elemT, off+lo*elemT.Size(), hi >= 0)
			lo++
		}
		return nil
	}
}

func (n *InitializerList) checkStruct(c *ctx, currObj Type, t *StructType, off int64) *InitializerList {
	// trc("%sSTRUCT %v: curr %s, t %s", c.indentInc(), n.Position(), currObj, t)
	// defer func() { trc("%sEXIT STRUCT", c.indentDec()) }()
	if t.HasFlexibleArrayMember() {
		if fam := t.FlexibleArrayMember().Type(); fam.IsIncomplete() {
			defer func() {
				if !fam.IsIncomplete() {
					size1 := t.size + fam.Size()
					size2 := roundup(size1, int64(t.Align()))
					t.padding = int(size2 - size1)
					t.size = size2
				}
			}()
		}
	}
	f := t.FieldByIndex(0)
	for n != nil {
		for f != nil && f.IsBitfield() && (f.ValueBits() == 0 || f.Name() == "") {
			f = t.FieldByIndex(f.index + 1)
		}
		if n.Designation == nil {
			if f == nil {
				return n
			}

			n = n.Initializer.check(c, currObj, f.Type(), off+f.Offset(), n, f)
			f = t.FieldByIndex(f.index + 1)
			continue
		}

		if currObj != t {
			return n
		}

		dl := n.Designation.DesignatorList
		n2 := *n
		n2.Designation = nil
		n = &n2
		nm := dl.Designator.name(c)
		if nm == "" {
			return nil
		}

		if f = t.FieldByName(nm); f == nil {
			c.errors.add(errorf("%v: %v has no member %v", n.Position(), t, nm))
			return nil
		}

		dl.Designator.typ = f.Type()
		if dl.DesignatorList == nil {
			n = n.Initializer.check(c, currObj, f.Type(), off+f.Offset(), n, f)
			f = t.FieldByIndex(f.index + 1)
			continue
		}

		n = n.checkDesignatorList(dl.DesignatorList, c, f.Type(), off+f.Offset(), false)
		f = t.FieldByIndex(f.index + 1)
	}
	return nil
}

func (n *InitializerList) checkUnion(c *ctx, currObj Type, t *UnionType, off int64) *InitializerList {
	// trc("%sUNION (A) %v: curr %s, t %s", c.indentInc(), n.Position(), currObj, t)
	// defer func() { trc("%sEXIT UNION (Z)", c.indentDec()) }()
	f := t.FieldByIndex(0)
	if n.Designation == nil {
		for f != nil && f.IsBitfield() && (f.ValueBits() == 0 || f.Name() == "") {
			f = t.FieldByIndex(f.index + 1)
		}
		if f == nil {
			return n
		}

		n.unionField = f
		return n.Initializer.check(c, currObj, f.Type(), off+f.Offset(), n, f)
	}

	if currObj != t {
		return n
	}

	n0 := n
	dl := n.Designation.DesignatorList
	n2 := *n
	n2.Designation = nil
	n = &n2
	nm := dl.Designator.name(c)
	if nm == "" {
		return nil
	}

	if f = t.FieldByName(nm); f == nil {
		c.errors.add(errorf("%v: %v has no member %v", n.Position(), t, nm))
		return nil
	}

	if p := f.Parent(); p != nil {
		f = p
		n = n0
	}

	n0.unionField = f
	dl.Designator.typ = f.Type()
	if dl.DesignatorList == nil {
		return n.Initializer.check(c, f.Type(), f.Type(), off+f.Offset(), n, f)
	}

	return n.checkDesignatorList(dl.DesignatorList, c, f.Type(), off+f.Offset(), false)
}

func (n *Designator) name(c *ctx) string {
	switch n.Case {
	case
		DesignatorIndex,  // '[' ConstantExpression ']'
		DesignatorIndex2: // '[' ConstantExpression "..." ConstantExpression ']'

		c.errors.add(errorf("%v: expected field name"))
	case DesignatorField: // '.' IDENTIFIER
		return n.Token2.SrcStr()
	case DesignatorField2: // IDENTIFIER ':'
		return n.Token.SrcStr()
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return ""
}

func (n *Designator) index(c *ctx) (lo, hi int64) {
	switch n.Case {
	case DesignatorIndex: // '[' ConstantExpression ']'
		lo = arrayIndex(c, n.ConstantExpression)
		hi = arrayIndex(c, n.ConstantExpression2)
		if lo < 0 {
			c.errors.add(errorf("%v: array indices cannot be negative"))
			return -1, -1
		}

		return lo, hi
	case DesignatorIndex2: // '[' ConstantExpression "..." ConstantExpression ']'
		lo = arrayIndex(c, n.ConstantExpression)
		hi = arrayIndex(c, n.ConstantExpression2)
		if lo < 0 || hi < 0 {
			c.errors.add(errorf("%v: array indices cannot be negative"))
			return -1, -1
		}

		if lo >= 0 && hi >= 0 && lo >= hi {
			c.errors.add(errorf("%v: first index shall be smaller than second index"))
			return -1, -1
		}

		return lo, hi
	case
		DesignatorField,  // '.' IDENTIFIER
		DesignatorField2: // IDENTIFIER ':'
		c.errors.add(errorf("%v: expected bracketed array index"))
	}
	return -1, -1
}

func arrayIndex(c *ctx, n ExpressionNode) int64 {
	if n == nil {
		return -1
	}

	v, ok := int64Value(c, n)
	if !ok || v < 0 {
		c.errors.add(errorf("%v: invalid array index", n.Position()))
		return -1
	}

	return v
}

func int64Value(c *ctx, n ExpressionNode) (int64, bool) {
	if n == nil {
		return 0, false
	}

	switch t := n.check(c, decay); {
	case IsIntegerType(t):
		switch x := n.eval(c, 0).(type) {
		case Int64Value:
			return int64(x), true
		case UInt64Value:
			if x <= math.MaxInt64 {
				return int64(x), true
			}
		}
	}
	return 0, false
}

func (n *InitializerList) checkDesignatorList(dl *DesignatorList, c *ctx, currObj Type, off int64, ranged bool) *InitializerList {
	t := currObj
	var fld *Field
	for ; dl != nil; dl = dl.DesignatorList {
		fld = nil
		switch x := t.(type) {
		case *StructType:
			nm := dl.Designator.name(c)
			if nm == "" {
				return nil
			}

			f := x.FieldByName(nm)
			if f == nil {
				c.errors.add(errorf("%v: type %s has no member %s", dl.Designator.Position(), t, nm))
				return nil
			}

			t = f.Type()
			dl.Designator.typ = t
			off += f.Offset()
			fld = f
		case *UnionType:
			nm := dl.Designator.name(c)
			if nm == "" {
				return nil
			}

			f := x.FieldByName(nm)
			if f == nil {
				c.errors.add(errorf("%v: type %s has no member %s", dl.Designator.Position(), t, nm))
				return nil
			}

			t = f.Type()
			dl.Designator.typ = t
			off += f.Offset()
			fld = f
		case *ArrayType:
			dl.Designator.typ = x.Elem()
			lo, hi := dl.Designator.index(c)
			if lo < 0 {
				return nil
			}

			if hi > lo {
				c.errors.add(errorf("TODO: %T", x))
				return nil
			}

			t = x.Elem()
			off += lo * t.Size()
			if hi < 0 {
				hi = lo
			}
			if e := x.Len(); e >= 0 {
				if lo >= e {
					c.errors.add(errorf("%v: index out of range: %v", dl.Position(), lo))
					return nil
				}

				if hi >= e {
					c.errors.add(errorf("%v: index out of range: %v", dl.Position(), hi))
					return nil
				}
			}
			switch {
			case hi < 0:
				n.Initializer.nelems = 1
			default:
				if ranged {
					c.errors.add(errorf("%v: nested ranged desinations", dl.Position()))
					return nil
				}

				n.Initializer.nelems = hi - lo + 1
				ranged = true
			}
		default:
			c.errors.add(errorf("internal error: %T", x))
			return nil
		}
	}
	return n.Initializer.check(c, currObj, t, off, n, fld)
}

// Declarator:
//
//	Pointer DirectDeclarator
func (n *Declarator) check(c *ctx, t Type) (r Type) {
	if t == nil {
		c.errors.add(errorf("TODO %T internal error", n))
		return
	}

	if n == nil {
		return t
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
		}
	}()

	r = n.DirectDeclarator.check(c, n.Pointer.check(c, t))
	if n.isTypename {
		r = r.setName(n)
	}
	n.typ = r
	if attr := t.Attributes(); attr != nil {
		a2 := &Attributes{}
		if attr.weak {
			a2.weak = true
			a2.isNonZero = true
		}
		if attr.alias != "" {
			a2.alias = attr.alias
			a2.isNonZero = true
		}
		if a2.isNonZero {
			var err error
			if n.typ, err = mergeAttr(r, a2); err != nil {
				c.errors.add(errorf("%v: %v", n.Position(), err))
			}
		}
	}
	if n.IsParam() {
		n.typ = r.Decay()
	}
	return n.Type()
}

func (n *Declarator) fixPointerAttr(c *ctx) {
	// trc("%v: n.Type.Attributes=%+v n.Type.Elem.Attributes=%+v", n.Position(), n.Type().Attributes(), n.Type().(*PointerType).Elem().Attributes())
	// defer func() {
	// 	trc("%v: -> n.Type.Attributes=%+v n.Type.Elem.Attributes=%+v", n.Position(), n.Type().Attributes(), n.Type().(*PointerType).Elem().Attributes())
	// }()
	switch {
	case n.Pointer != nil:
		switch x := n.Type().(type) {
		case *PointerType:
			x.attributer.p.customAttributes = nil
		case *ArrayType:
			x.attributer.p.customAttributes = nil
		case *FunctionType:
			x.attributer.p.customAttributes = nil
		default:
			c.errors.add(errorf("TODO %v: %T", n.Position(), x))
		}
	}
}

func (n *Declarator) fixVolatileAndConst(c *ctx) {
	switch {
	case n.IsVolatile() && n.Pointer == nil:
		if n.Type().Attributes().IsVolatile() {
			break
		}

		attr := volatileAttr(n.Type().Attributes(), true)
		n.typ = n.Type().setAttr(attr)
	case n.IsVolatile() && n.Pointer != nil && !n.Pointer.TypeQualifiers.isVolatile():
		// volatile int *p;
		n.isVolatile = false
		switch x := n.Type().(type) {
		case *PointerType:
			if x.attributer.p != nil {
				x.attributer.p.isVolatile = false
			}
			el := x.Elem()
			attr := volatileAttr(el.Attributes(), true)
			el.setAttr(attr)
		case *ArrayType:
			if x.attributer.p != nil {
				x.attributer.p.isVolatile = false
			}
			el := x.Elem()
			attr := volatileAttr(el.Attributes(), true)
			el = el.setAttr(attr)
			x.elem.typ = el
		default:
			c.errors.add(errorf("TODO %v: %T", n.Position(), x))
		}
	case !n.IsVolatile() && n.Pointer != nil && n.Pointer.TypeQualifiers.isVolatile():
		// int *volatile p;
		n.isVolatile = true
		attr := volatileAttr(n.Type().Attributes(), true)
		n.typ, _ = mergeAttr(n.Type(), attr)
	}

	switch {
	case n.IsConst() && n.Pointer == nil:
		if n.Type().Attributes().IsConst() {
			break
		}

		attr := constAttr(n.Type().Attributes(), true)
		n.typ = n.Type().setAttr(attr)
	case n.IsConst() && n.Pointer != nil && !n.Pointer.TypeQualifiers.isConst():
		// const int *p;
		n.isConst = false
		switch x := n.Type().(type) {
		case *PointerType:
			if x.attributer.p != nil {
				x.attributer.p.isConst = false
			}
			el := x.Elem()
			attr := constAttr(el.Attributes(), true)
			el.setAttr(attr)
		case *ArrayType:
			if x.attributer.p != nil {
				x.attributer.p.isConst = false
			}
			el := x.Elem()
			attr := constAttr(el.Attributes(), true)
			el = el.setAttr(attr)
			x.elem.typ = el
		case *FunctionType:
			// nop
		default:
			c.errors.add(errorf("TODO %v: %T", n.Position(), x))
		}
	case !n.IsConst() && n.Pointer != nil && n.Pointer.TypeQualifiers.isConst():
		// int *const p;
		n.isConst = true
		attr := constAttr(n.Type().Attributes(), true)
		n.typ, _ = mergeAttr(n.Type(), attr)
	}
}

// DirectDeclarator:
//
//	IDENTIFIER                                                             // Case DirectDeclaratorIdent
//
// |       '(' Declarator ')'                                                     // Case DirectDeclaratorDecl
// |       DirectDeclarator '[' TypeQualifiers AssignmentExpression ']'           // Case DirectDeclaratorArr
// |       DirectDeclarator '[' "static" TypeQualifiers AssignmentExpression ']'  // Case DirectDeclaratorStaticArr
// |       DirectDeclarator '[' TypeQualifiers "static" AssignmentExpression ']'  // Case DirectDeclaratorArrStatic
// |       DirectDeclarator '[' TypeQualifiers '*' ']'                            // Case DirectDeclaratorStar
// |       DirectDeclarator '(' ParameterTypeList ')'                             // Case DirectDeclaratorFuncParam
// |       DirectDeclarator '(' IdentifierList ')'                                // Case DirectDeclaratorFuncIdent
func (n *DirectDeclarator) check(c *ctx, t Type) (r Type) {
	if t == nil {
		c.errors.add(errorf("TODO %T internal error", n))
		return
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
		}
	}()

	switch n.Case {
	case DirectDeclaratorIdent: // IDENTIFIER
		return t
	case DirectDeclaratorDecl: // '(' Declarator ')'
		return n.Declarator.check(c, t)
	case DirectDeclaratorArr: // DirectDeclarator '[' TypeQualifiers AssignmentExpression ']'
		return n.DirectDeclarator.check(c, c.newArrayType(t, arraySize(c, n.AssignmentExpression), n.AssignmentExpression))
	case DirectDeclaratorStaticArr: // DirectDeclarator '[' "static" TypeQualifiers AssignmentExpression ']'
		return n.DirectDeclarator.check(c, c.newArrayType(t, arraySize(c, n.AssignmentExpression), n.AssignmentExpression))
	case DirectDeclaratorArrStatic: // DirectDeclarator '[' TypeQualifiers "static" AssignmentExpression ']'
		return n.DirectDeclarator.check(c, c.newArrayType(t, arraySize(c, n.AssignmentExpression), n.AssignmentExpression))
	case DirectDeclaratorStar: // DirectDeclarator '[' TypeQualifiers '*' ']'
		c.errors.add(errorf("TODO %v", n.Case))
	case DirectDeclaratorFuncParam: // DirectDeclarator '(' ParameterTypeList ')'
		fp, isVariadic := n.ParameterTypeList.check(c)
		return n.DirectDeclarator.check(c, c.newFunctionType(t, fp, isVariadic))
	case DirectDeclaratorFuncIdent: // DirectDeclarator '(' IdentifierList ')'
		return n.DirectDeclarator.check(c, c.newFunctionType(t, nil, false))
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return t //TODO-
}

func arraySize(c *ctx, n ExpressionNode) int64 {
	if n == nil {
		return -1
	}

	v, ok := int64Value(c, n)
	if !ok { // VLA
		return -1
	}

	if v < 0 {
		c.errors.add(errorf("%v: invalid array size, %s: %v", n.Position(), NodeSource(n), v))
		return -1
	}

	return v
}

// ParameterTypeList:
//
//	ParameterList            // Case ParameterTypeListList
//
// |       ParameterList ',' "..."  // Case ParameterTypeListVar
func (n *ParameterTypeList) check(c *ctx) (fp []*ParameterDeclaration, isVariadic bool) {
	if n == nil {
		return nil, false
	}

	fp = n.ParameterList.check(c)
	return fp, n.Case == ParameterTypeListVar
}

// ParameterList:
//
//	ParameterDeclaration
//
// |       ParameterList ',' ParameterDeclaration
func (n *ParameterList) check(c *ctx) (fp []*ParameterDeclaration) {
	for ; n != nil; n = n.ParameterList {
		n.ParameterDeclaration.check(c)
		fp = append(fp, n.ParameterDeclaration)
	}
	return fp
}

// ParameterDeclaration:
//
//	DeclarationSpecifiers Declarator          // Case ParameterDeclarationDecl
//
// |       DeclarationSpecifiers AbstractDeclarator  // Case ParameterDeclarationAbstract
func (n *ParameterDeclaration) check(c *ctx) {
	var isExtern, isStatic, isAtomic, isThreadLocal, isConst, isVolatile, isInline, isRegister, isAuto, isNoreturn, isRestrict bool
	var alignas int
	switch n.Case {
	case ParameterDeclarationDecl: // DeclarationSpecifiers Declarator
		n.Declarator.isParam = true
		n.typ = n.Declarator.check(c, n.DeclarationSpecifiers.check(c, &isExtern, &isStatic, &isAtomic, &isThreadLocal, &isConst, &isVolatile, &isInline, &isRegister, &isAuto, &isNoreturn, &isRestrict, &alignas))
		n.Declarator.isConst = isConst
		n.Declarator.isVolatile = isVolatile
		n.Declarator.isRegister = isRegister
		n.Declarator.isRestrict = isRestrict
		n.Declarator.isNoreturn = isNoreturn
		if isExtern || isStatic || isAtomic || isThreadLocal || isInline || isAuto || alignas != 0 {
			c.errors.add(errorf("%v: invalid specifier(s) for parameter: abc", n.Declarator.Position()))
		}
	case ParameterDeclarationAbstract: // DeclarationSpecifiers AbstractDeclarator
		t := n.AbstractDeclarator.check(c, n.DeclarationSpecifiers.check(c, &isExtern, &isStatic, &isAtomic, &isThreadLocal, &isConst, &isVolatile, &isInline, &isRegister, &isAuto, &isNoreturn, &isRestrict, &alignas)).Decay()
		n.typ = t
		if n.AbstractDeclarator != nil {
			n.AbstractDeclarator.typ = t
		}
		if isExtern || isStatic || isAtomic || isThreadLocal || isInline || isAuto || isNoreturn || alignas != 0 {
			c.errors.add(errorf("%v: invalid specifier(s) for unnamed parameter", n.Position()))
		}
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

// AbstractDeclarator:
//
//	Pointer                           // Case AbstractDeclaratorPtr
//
// |       Pointer DirectAbstractDeclarator  // Case AbstractDeclaratorDecl
func (n *AbstractDeclarator) check(c *ctx, t Type) (r Type) {
	if t == nil {
		c.errors.add(errorf("TODO %T internal error", n))
		return
	}

	if n == nil {
		return t
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
		}
	}()

	n.typ = n.DirectAbstractDeclarator.check(c, n.Pointer.check(c, t))
	return n.Type()
}

// DirectAbstractDeclarator:
//
//	'(' AbstractDeclarator ')'                                                     // Case DirectAbstractDeclaratorDecl
//
// |       DirectAbstractDeclarator '[' TypeQualifiers AssignmentExpression ']'           // Case DirectAbstractDeclaratorArr
// |       DirectAbstractDeclarator '[' "static" TypeQualifiers AssignmentExpression ']'  // Case DirectAbstractDeclaratorStaticArr
// |       DirectAbstractDeclarator '[' TypeQualifiers "static" AssignmentExpression ']'  // Case DirectAbstractDeclaratorArrStatic
// |       DirectAbstractDeclarator '[' '*' ']'                                           // Case DirectAbstractDeclaratorArrStar
// |       DirectAbstractDeclarator '(' ParameterTypeList ')'                             // Case DirectAbstractDeclaratorFunc
func (n *DirectAbstractDeclarator) check(c *ctx, t Type) (r Type) {
	if t == nil {
		c.errors.add(errorf("TODO %T internal error", n))
		return
	}

	if n == nil {
		return t
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
		}
	}()

	switch n.Case {
	case DirectAbstractDeclaratorDecl: // '(' AbstractDeclarator ')'
		return n.AbstractDeclarator.check(c, t)
	case DirectAbstractDeclaratorArr: // DirectAbstractDeclarator '[' TypeQualifiers AssignmentExpression ']'
		return n.DirectAbstractDeclarator.check(c, c.newArrayType(t, arraySize(c, n.AssignmentExpression), n.AssignmentExpression))
	case DirectAbstractDeclaratorStaticArr: // DirectAbstractDeclarator '[' "static" TypeQualifiers AssignmentExpression ']'
		c.errors.add(errorf("TODO %v", n.Case))
	case DirectAbstractDeclaratorArrStatic: // DirectAbstractDeclarator '[' TypeQualifiers "static" AssignmentExpression ']'
		c.errors.add(errorf("TODO %v", n.Case))
	case DirectAbstractDeclaratorArrStar: // DirectAbstractDeclarator '[' '*' ']'
		c.errors.add(errorf("TODO %v", n.Case))
	case DirectAbstractDeclaratorFunc: // DirectAbstractDeclarator '(' ParameterTypeList ')'
		fp, isVariadic := n.ParameterTypeList.check(c)
		return n.DirectAbstractDeclarator.check(c, c.newFunctionType(t, fp, isVariadic))
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return t //TODO-
}

// Pointer:
//
//	'*' TypeQualifiers          // Case PointerTypeQual
//
// |       '*' TypeQualifiers Pointer  // Case PointerPtr
// |       '^' TypeQualifiers          // Case PointerBlock
func (n *Pointer) check(c *ctx, t Type) (r Type) {
	if t == nil {
		c.errors.add(errorf("TODO %T internal error", n))
		return
	}

	if n == nil {
		return t
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}
	}()

	switch n.Case {
	case PointerTypeQual: // '*' TypeQualifiers
		p := c.newPointerType(t)
		p.scope = n.LexicalScope()
		return p
	case PointerPtr: // '*' TypeQualifiers Pointer
		p := c.newPointerType(t)
		p.scope = n.LexicalScope()
		return n.Pointer.check(c, p)
	case PointerBlock: // '^' TypeQualifiers
		p := c.newPointerType(t)
		p.scope = n.LexicalScope()
		return n.Pointer.check(c, p)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return t //TODO-
}

func ts2String(a []TypeSpecifierCase) string {
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	var b strings.Builder
	for _, v := range a {
		b.WriteByte(byte(v))
	}
	return b.String()
}

func volatileAttr(attr *Attributes, isVolatile bool) *Attributes {
	if !isVolatile {
		return attr
	}

	if attr == nil {
		attr = &Attributes{}
	}
	attr.setIsVolatile(true)
	return attr
}

func constAttr(attr *Attributes, isConst bool) *Attributes {
	if !isConst {
		return attr
	}

	if attr == nil {
		attr = &Attributes{}
	}
	attr.setIsConst(true)
	return attr
}

// DeclarationSpecifiers:
//
//	StorageClassSpecifier DeclarationSpecifiers  // Case DeclarationSpecifiersStorage
//
// |       TypeSpecifier DeclarationSpecifiers          // Case DeclarationSpecifiersTypeSpec
// |       TypeQualifier DeclarationSpecifiers          // Case DeclarationSpecifiersTypeQual
// |       FunctionSpecifier DeclarationSpecifiers      // Case DeclarationSpecifiersFunc
// |       AlignmentSpecifier DeclarationSpecifiers     // Case DeclarationSpecifiersAlignSpec
// |       "__attribute__"                              // Case DeclarationSpecifiersAttr
func (n *DeclarationSpecifiers) check(c *ctx, isExtern, isStatic, isAtomic, isThreadLocal, isConst, isVolatile, isInline, isRegister, isAuto, isNoreturn, isRestrict *bool, alignas *int) (r Type) {
	if n == nil {
		return c.intT
	}

	n0 := n

	var attr *Attributes
	var ts []TypeSpecifierCase
	defer func(n *DeclarationSpecifiers) {
		if r == nil || r == Invalid {
			//panic(todo("%v: %v %v", n.Position(), ts, TypeString(r)))
			c.errors.add(errorf("%v: TODO %T missed/failed type check: %v", n.Position(), n, ts))
			return
		}

		if _, ok := r.(*PredefinedType); ok && r.Size() < 0 { // Not supported on target
			c.errors.add(errorf("%s not supported on %s/%s", r, c.ast.ABI.goos, c.ast.ABI.goarch))
			r = Invalid
		}
		var err error
		attr = volatileAttr(attr, *isVolatile)
		if attr != nil {
			if r, err = mergeAttr(r, attr); err != nil {
				c.errors.add(errorf("%v", err))
			}
		}
		attr = constAttr(attr, *isConst)
		if attr != nil {
			if r, err = mergeAttr(r, attr); err != nil {
				c.errors.add(errorf("%v", err))
			}
		}
		n.typ = r
	}(n)

	var attrs []*Attributes
	for ; n != nil; n = n.DeclarationSpecifiers {
		switch n.Case {
		case DeclarationSpecifiersStorage: // StorageClassSpecifier DeclarationSpecifiers
			n.StorageClassSpecifier.check(c, isExtern, isStatic, isThreadLocal, isRegister, isAuto)
		case DeclarationSpecifiersTypeSpec: // TypeSpecifier DeclarationSpecifiers
			ts = append(ts, n.TypeSpecifier.Case)
			r = n.TypeSpecifier.check(c, isAtomic, *isVolatile, *isConst)
		case DeclarationSpecifiersTypeQual: // TypeQualifier DeclarationSpecifiers
			if attr := n.TypeQualifier.check(c, isConst, isVolatile, isAtomic, isRestrict); attr != nil {
				attrs = append(attrs, attr)
			}
		case DeclarationSpecifiersFunc: // FunctionSpecifier DeclarationSpecifiers
			n.FunctionSpecifier.check(c, isInline, isNoreturn)
		case DeclarationSpecifiersAlignSpec: // AlignmentSpecifier DeclarationSpecifiers
			if v := n.AlignmentSpecifier.check(c).Align(); v > 0 {
				*alignas = v
			}
		case DeclarationSpecifiersAttr:
			if attr := n.AttributeSpecifierList.check(c); attr != nil {
				attrs = append(attrs, attr)
			}
		default:
			c.errors.add(errorf("internal error: %v", n.Case))
		}
	}
	for _, nattr := range attrs {
		var err error
		attr, err = attr.merge(n, nattr)
		if err != nil {
			c.errors.add(errorf("TODO %T", n0.Position()))
		}
	}

	t, ok := c.builtinTypes[ts2String(ts)]
	if ok {
		return t
	}

	switch len(ts) {
	case 0:
		return c.intT
	case 1:
		switch ts[0] {
		case
			TypeSpecifierAtomic,
			TypeSpecifierEnum,
			TypeSpecifierStructOrUnion,
			TypeSpecifierTypeName,
			TypeSpecifierTypeofExpr:

			return r
		}
	}

	return nil
}

// AttributeSpecifierList:
//
//	AttributeSpecifier
//
// |       AttributeSpecifierList AttributeSpecifier
func (n *AttributeSpecifierList) check(c *ctx) *Attributes {
	attr := newAttributes()
	for ; n != nil; n = n.AttributeSpecifierList {
		n.AttributeSpecifier.check(c, attr)
	}
	if attr.isNonZero {
		return attr
	}

	return nil
}

// AttributeSpecifier:
//
//	"__attribute__" '(' '(' AttributeValueList ')' ')'
func (n *AttributeSpecifier) check(c *ctx, attr *Attributes) {
	n.AttributeValueList.check(c, attr)
}

// AttributeValueList:
//
//	AttributeValue
//
// |       AttributeValueList ',' AttributeValue
func (n *AttributeValueList) check(c *ctx, attr *Attributes) {
	for ; n != nil; n = n.AttributeValueList {
		n.AttributeValue.check(c, attr)
	}
}

// AttributeValue:
//
//	IDENTIFIER                                 // Case AttributeValueIdent
//
// |       IDENTIFIER '(' ArgumentExpressionList ')'  // Case AttributeValueExpr
func (n *AttributeValue) check(c *ctx, attr *Attributes) {

	var dummy purer
	switch n.Case {
	case AttributeValueIdent: // IDENTIFIER
		attr.setCustom(n.Token.SrcStr(), []Value{})

		switch n.Token.SrcStr() {
		case
			"__weak__",
			"weak":

			attr.setWeak()
		case "__always_inline__":
			attr.setAlwaysInline()
		case "__gnu_inline__":
			attr.setGNUInline()
		}
	case AttributeValueExpr: // IDENTIFIER '(' ArgumentExpressionList ')'
		n.ArgumentExpressionList.check(c, decay|ignoreUndefined, &dummy)

		attrvalues := []Value{}
		for ev := n.ArgumentExpressionList; ev != nil; ev = ev.ArgumentExpressionList {
			if ev.AssignmentExpression == nil {
				continue
			}
			attrvalues = append(attrvalues, ev.AssignmentExpression.Value())
		}
		attr.setCustom(n.Token.SrcStr(), attrvalues)

		switch n.Token.SrcStr() {
		case
			"__alias__",
			"alias":

			e := n.ArgumentExpressionList.AssignmentExpression
			if n.ArgumentExpressionList.ArgumentExpressionList != nil {
				c.errors.add(errorf("%v: expected one expression", e.Position()))
				break
			}

			x, ok := e.Value().(StringValue)
			if !ok {
				c.errors.add(errorf("%v: expected a string literal", e.Position()))
				return
			}

			nm := strings.TrimRight(string(x), "\x00")
			attr.setAlias(nm)
			if a := c.ast.Scope.Nodes[nm]; len(a) != 0 {
				switch x := a[0].(type) {
				case *Declarator:
					attr.setAliasDecl(x)
				}
			}
		case
			"__aligned__",
			"aligned":

			e := n.ArgumentExpressionList.AssignmentExpression
			if n.ArgumentExpressionList.ArgumentExpressionList != nil {
				c.errors.add(errorf("%v: expected one expression", e.Position()))
				break
			}

			v, ok := int64Value(c, e)
			if !ok {
				c.errors.add(errorf("%v: expected a constant integer value", e.Position()))
				return
			}

			if attr.Aligned() > 0 {
				c.errors.add(errorf("%v: multiple 'aligned' specifications", e.Position()))
				return
			}

			if v <= 0 {
				c.errors.add(errorf("%v: alignment must be positive", e.Position()))
				return
			}

			attr.setAligned(v)
		case
			"__vector_size__",
			"vector_size":

			e := n.ArgumentExpressionList.AssignmentExpression
			if n.ArgumentExpressionList.ArgumentExpressionList != nil {
				c.errors.add(errorf("%v: expected one expression", e.Position()))
				break
			}

			v, ok := int64Value(c, e)
			if !ok {
				c.errors.add(errorf("%v: expected a constant integer value", e.Position()))
				return
			}

			if attr.VectorSize() > 0 {
				c.errors.add(errorf("%v: multiple 'vector_size' specifications", e.Position()))
				return
			}

			if v <= 0 {
				c.errors.add(errorf("%v: vector size must be positive", e.Position()))
				return
			}

			if v&(v-1) != 0 {
				c.errors.add(errorf("%v: vector size must be a power of two: %v", e.Position(), v))
				return
			}

			attr.setVectorSize(v)
			c.usesVectors = true
		case
			"__visibility__",
			"visibility":

			e := n.ArgumentExpressionList.AssignmentExpression
			if n.ArgumentExpressionList.ArgumentExpressionList != nil {
				c.errors.add(errorf("%v: expected one expression", e.Position()))
				break
			}

			x, ok := e.Value().(StringValue)
			if !ok {
				c.errors.add(errorf("%v: expected a string literal", e.Position()))
				return
			}

			nm := strings.TrimRight(string(x), "\x00")
			attr.setVisibility(nm)
		}
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

// ArgumentExpressionList:
//
//	AssignmentExpression
//
// |       ArgumentExpressionList ',' AssignmentExpression
func (n *ArgumentExpressionList) check(c *ctx, mode flags, p *purer) (r []ExpressionNode) {
	pure := true
	for ; n != nil; n = n.ArgumentExpressionList {
		e := n.AssignmentExpression
		e.check(c, mode)
		pure = pure && e.Pure()
		e.eval(c, mode)
		r = append(r, e)
	}
	p.setPure(pure)
	return r
}

// AlignmentSpecifier:
//
//	"_Alignas" '(' TypeName ')'            // Case AlignmentSpecifierType
//
// |       "_Alignas" '(' ConstantExpression ')'  // Case AlignmentSpecifierExpr
func (n *AlignmentSpecifier) check(c *ctx) (r Type) {
	switch n.Case {
	case AlignmentSpecifierType: // "_Alignas" '(' TypeName ')'
		return n.TypeName.check(c)
	case AlignmentSpecifierExpr: // "_Alignas" '(' ConstantExpression ')'
		return n.ConstantExpression.check(c, decay)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
		return nil
	}
}

// FunctionSpecifier:
//
//	"inline"     // Case FunctionSpecifierInline
//
// |       "_Noreturn"  // Case FunctionSpecifierNoreturn
func (n *FunctionSpecifier) check(c *ctx, isInline, isNoreturn *bool) {
	switch n.Case {
	case FunctionSpecifierInline: // "inline"
		*isInline = true
	case FunctionSpecifierNoreturn: // "_Noreturn"
		*isNoreturn = true
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

// TypeQualifier:
//
//	"const"          // Case TypeQualifierConst
//	|       "restrict"       // Case TypeQualifierRestrict
//	|       "volatile"       // Case TypeQualifierVolatile
//	|       "_Atomic"        // Case TypeQualifierAtomic
//	|       "_Nonnull"       // Case TypeQualifierNonnull
//	|       "__attribute__"  // Case TypeQualifierAttr
func (n *TypeQualifier) check(c *ctx, isConst, isVolatile, isAtomic, isRestrict *bool) (r *Attributes) {
	switch n.Case {
	case TypeQualifierConst: // "const"
		*isConst = true
	case TypeQualifierRestrict: // "restrict"
		*isRestrict = true
	case TypeQualifierVolatile: // "volatile"
		*isVolatile = true
	case TypeQualifierAtomic: // "_Atomic"
		*isAtomic = true
	case TypeQualifierNonnull: // "_Nonnull"
		c.errors.add(errorf("TODO %v", n.Case))
	case TypeQualifierAttr: // AttributeSpecifierList
		r = n.AttributeSpecifierList.check(c)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return r
}

// StorageClassSpecifier:
//
//	"typedef"             // Case StorageClassSpecifierTypedef
//
// |       "extern"              // Case StorageClassSpecifierExtern
// |       "static"              // Case StorageClassSpecifierStatic
// |       "auto"                // Case StorageClassSpecifierAuto
// |       "register"            // Case StorageClassSpecifierRegister
// |       "_Thread_local"       // Case StorageClassSpecifierThreadLocal
// |       "__declspec" '(' ')'  // Case StorageClassSpecifierDeclspec
func (n *StorageClassSpecifier) check(c *ctx, isExtern, isStatic, isThreadLocal, isRegister, isAuto *bool) {
	switch n.Case {
	case StorageClassSpecifierTypedef: // "typedef"
		// ok
	case StorageClassSpecifierExtern: // "extern"
		*isExtern = true
	case StorageClassSpecifierStatic: // "static"
		*isStatic = true
	case StorageClassSpecifierAuto: // "auto"
		*isAuto = true
	case StorageClassSpecifierRegister: // "register"
		*isRegister = true
	case StorageClassSpecifierThreadLocal: // "_Thread_local"
		*isThreadLocal = true
	case StorageClassSpecifierDeclspec: // "__declspec" '(' ')'
		c.errors.add(errorf("TODO %v", n.Case))
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
}

// TypeSpecifier:
//
//	"void"                           // Case TypeSpecifierVoid
//
// |       "char"                           // Case TypeSpecifierChar
// |       "short"                          // Case TypeSpecifierShort
// |       "int"                            // Case TypeSpecifierInt
// |       "__int128"                       // Case TypeSpecifierInt128
// |       "__uint128_t"                    // Case TypeSpecifierUint128
// |       "long"                           // Case TypeSpecifierLong
// |       "float"                          // Case TypeSpecifierFloat
// |       "_Float16"                       // Case TypeSpecifierFloat16
// |       "_Decimal128"                    // Case TypeSpecifierDecimal128
// |       "_Decimal32"                     // Case TypeSpecifierDecimal32
// |       "_Decimal64"                     // Case TypeSpecifierDecimal64
// |       "_Float128"                      // Case TypeSpecifierFloat128
// |       "_Float128x"                     // Case TypeSpecifierFloat128x
// |       "double"                         // Case TypeSpecifierDouble
// |       "signed"                         // Case TypeSpecifierSigned
// |       "unsigned"                       // Case TypeSpecifierUnsigned
// |       "_Bool"                          // Case TypeSpecifierBool
// |       "_Complex"                       // Case TypeSpecifierComplex
// |       "_Imaginary"                     // Case TypeSpecifierImaginary
// |       StructOrUnionSpecifier           // Case TypeSpecifierStructOrUnion
// |       EnumSpecifier                    // Case TypeSpecifierEnum
// |       TYPENAME                         // Case TypeSpecifierTypeName
// |       "typeof" '(' ExpressionList ')'  // Case TypeSpecifierTypeofExpr
// |       "typeof" '(' TypeName ')'        // Case TypeSpecifierTypeofType
// |       AtomicTypeSpecifier              // Case TypeSpecifierAtomic
// |       "_Float32"                       // Case TypeSpecifierFloat32
// |       "_Float64"                       // Case TypeSpecifierFloat64
// |       "_Float32x"                      // Case TypeSpecifierFloat32x
// |       "_Float64x"                      // Case TypeSpecifierFloat64x
func (n *TypeSpecifier) check(c *ctx, isAtomic *bool, isVolatile, isConst bool) (r Type) {
	if n == nil {
		return Invalid
	}

	switch n.Case {
	case TypeSpecifierVoid: // "void"
		// ok
	case TypeSpecifierChar: // "char"
		// ok
	case TypeSpecifierShort: // "short"
		// ok
	case TypeSpecifierInt: // "int"
		// ok
	case TypeSpecifierInt128: // "__int128"
		// ok
	case TypeSpecifierUint128: // "__uint128_t"
		// ok
	case TypeSpecifierLong: // "long"
		// ok
	case TypeSpecifierFloat: // "float"
		// ok
	case TypeSpecifierFloat16: // "_Float16"
		// ok
	case TypeSpecifierDecimal128: // "_Decimal128"
		// ok
	case TypeSpecifierDecimal32: // "_Decimal32"
		// ok
	case TypeSpecifierDecimal64: // "_Decimal64"
		// ok
	case TypeSpecifierFloat128: // "_Float128"
		// ok
	case TypeSpecifierFloat128x: // "_Float128x"
		// ok
	case TypeSpecifierDouble: // "double"
		// ok
	case TypeSpecifierSigned: // "signed"
		// ok
	case TypeSpecifierUnsigned: // "unsigned"
		// ok
	case TypeSpecifierBool: // "_Bool"
		// ok
	case TypeSpecifierComplex: // "_Complex"
		// ok
	case TypeSpecifierImaginary: // "_Imaginary"
		c.errors.add(errorf("TODO %v", n.Case))
	case TypeSpecifierStructOrUnion: // StructOrUnionSpecifier
		return n.StructOrUnionSpecifier.check(c, isVolatile, isConst)
	case TypeSpecifierEnum: // EnumSpecifier
		return n.EnumSpecifier.check(c)
	case TypeSpecifierTypeName: // TYPENAME
		if x, ok := n.LexicalScope().ident(n.Token).(*Declarator); ok && x.isTypename {
			return x.Type()
		}

		c.errors.add(errorf("%v: undefined type name: %s", n.Position(), n.Token.Src()))
	case TypeSpecifierTypeofExpr: // "typeof" '(' ExpressionList ')'
		return n.ExpressionList.check(c, 0)
	case TypeSpecifierTypeofType: // "typeof" '(' TypeName ')'
		return n.TypeName.check(c)
	case TypeSpecifierAtomic: // AtomicTypeSpecifier
		*isAtomic = true
		return n.AtomicTypeSpecifier.check(c)
	case TypeSpecifierFloat32: // "_Float32"
		// ok
	case TypeSpecifierFloat64: // "_Float64"
		// ok
	case TypeSpecifierFloat32x: // "_Float32x"
		// ok
	case TypeSpecifierFloat64x: // "_Float64x"
		// ok
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return nil
}

func (n *AtomicTypeSpecifier) check(c *ctx) (r Type) {
	return n.TypeName.check(c)
}

// EnumSpecifier:
//
//	"enum" IDENTIFIER '{' EnumeratorList ',' '}'  // Case EnumSpecifierDef
//
// |       "enum" IDENTIFIER                             // Case EnumSpecifierTag
func (n *EnumSpecifier) check(c *ctx) (r Type) {
	if n == nil {
		return Invalid
	}

	if n.typ != nil {
		return n.Type()
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
		}
	}()

	var ts Type
	if n.EnumTypeSpecifier != nil {
		var isAtomic, isConst, isVolatile, isRestrict bool
		var alignas int
		ts = n.EnumTypeSpecifier.SpecifierQualifierList.check(c, &isAtomic, &isConst, &isVolatile, &isRestrict, &alignas)
	}

	tag := n.Token2
	switch n.Case {
	case EnumSpecifierDef: // "enum" IDENTIFIER EnumTypeSpecifier '{' EnumeratorList ',' '}'
		var t Type
		var iota, min int64
		var max uint64
		var list []*Enumerator
		for l := n.EnumeratorList; l != nil; l = l.EnumeratorList {
			en := l.Enumerator
			list = append(list, en)
			iota = en.check(c, iota)
			switch x := en.Value().(type) {
			case Int64Value:
				v := int64(x)
				min = mathutil.MinInt64(min, v)
				if v >= 0 {
					max = mathutil.MaxUint64(max, uint64(v))
				}
			case UInt64Value:
				v := uint64(x)
				max = mathutil.MaxUint64(max, v)
			}
		}
		switch {
		case c.cfg.UnsignedEnums:
			switch {
			case min >= 0 && max <= math.MaxUint32:
				t = c.ast.kinds[UInt]
			case min >= math.MinInt32 && max <= math.MaxInt32:
				// [0]6.4.4.3/2: An identifier declared as an enumeration constant has type int.
				t = c.intT
			case min >= 0 && max < math.MaxInt64:
				t = c.ast.kinds[ULong]
				if t.Size() < 8 {
					t = c.ast.kinds[ULongLong]
				}
			default:
				t = c.ast.kinds[Long]
				if t.Size() < 8 {
					t = c.ast.kinds[LongLong]
				}
			}
		default:
			switch {
			case min >= math.MinInt32 && max <= math.MaxInt32:
				// [0]6.4.4.3/2: An identifier declared as an enumeration constant has type int.
				t = c.intT
			case min >= 0 && max <= math.MaxUint32:
				t = c.ast.kinds[UInt]
			case max < math.MaxInt64:
				t = c.ast.kinds[Long]
				if t.Size() < 8 {
					t = c.ast.kinds[LongLong]
				}
			default:
				t = c.ast.kinds[ULong]
				if t.Size() < 8 {
					t = c.ast.kinds[ULongLong]
				}
			}
		}
		if ts != nil {
			t = ts
		}
		for _, v := range list {
			v.typ = t
		}
		et := c.newEnumType(n.LexicalScope(), tag, t, list)
		et.min = min
		et.max = max
		n.typ = et
	case EnumSpecifierTag: // "enum" IDENTIFIER
		if x := n.LexicalScope().enum(n.Token2); x != nil {
			switch {
			case x.typ == nil:
				t := c.newEnumType(n.LexicalScope(), tag, ts, nil)
				if ts == nil {
					t.forward = x
				}
				n.typ = t
			default:
				n.typ = x.typ
			}
			break
		}

		t := c.newEnumType(n.LexicalScope(), tag, ts, nil)
		t.isIncomplete0 = true
		n.typ = t
		c.ast.Scope.declare(&c.errors, tag.SrcStr(), n)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

func (n *Enumerator) check(c *ctx, iota int64) int64 {
	switch n.Case {
	case EnumeratorIdent: // IDENTIFIER
		n.typ = c.int64T
		n.val = Int64Value(iota)
	case EnumeratorExpr: // IDENTIFIER '=' ConstantExpression
		n.typ = n.ConstantExpression.check(c, decay)
		switch n.val = n.ConstantExpression.Value(); x := n.Value().(type) {
		case Int64Value:
			return int64(x) + 1
		case UInt64Value:
			return int64(x) + 1
		case *UnknownValue:
			// ok
		default:
			c.errors.add(errorf("internal error: %T", x))
		}
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return iota + 1
}

// StructOrUnionSpecifier:
//
//	StructOrUnion IDENTIFIER '{' StructDeclarationList '}'  // Case StructOrUnionSpecifierDef
//
// |       StructOrUnion IDENTIFIER                                // Case StructOrUnionSpecifierTag
func (n *StructOrUnionSpecifier) check(c *ctx, isVolatile, isConst bool) (r Type) {
	if n == nil {
		return Invalid
	}

	if n.typ != nil {
		return n.Type()
	}

	attr, err := n.AttributeSpecifierList.check(c).merge(n, n.AttributeSpecifierList2.check(c))
	if err != nil {
		c.errors.add(err)
	}
	tag := n.Token
	switch n.Case {
	case StructOrUnionSpecifierDef: // StructOrUnion IDENTIFIER '{' StructDeclarationList '}'
		defer func() {
			if r == nil || r == Invalid {
				c.errors.add(errorf("TODO %T missed/failed type check", n))
			}
		}()

		switch {
		case n.StructOrUnion.Case == StructOrUnionUnion:
			n.typ = c.newUnionType(n.LexicalScope(), tag, nil, -1, 1, attr)
		default:
			n.typ = c.newStructType(n.LexicalScope(), tag, nil, -1, 1, attr)
		}

		n.StructDeclarationList.check(c, n, isVolatile, isConst)
	case StructOrUnionSpecifierTag: // StructOrUnion IDENTIFIER
		if x := n.LexicalScope().structOrUnion(n.Token); x != nil {
			if n.StructOrUnion.Case != x.StructOrUnion.Case {
				c.errors.add(errorf("%v: mismatched struct/union tag", n.Token.Position()))
				break
			}

			switch {
			case x.typ == nil:
				switch {
				case n.StructOrUnion.Case == StructOrUnionUnion:
					t := c.newUnionType(n.LexicalScope(), tag, nil, -1, 1, attr)
					t.forward = x
					n.typ = t
				default:
					t := c.newStructType(n.LexicalScope(), tag, nil, -1, 1, attr)
					t.forward = x
					n.typ = t
				}
			default:
				n.typ = x.typ
			}
			break
		}

		switch {
		case n.StructOrUnion.Case == StructOrUnionUnion:
			t := c.newUnionType(n.LexicalScope(), tag, nil, -1, 1, attr)
			t.isIncomplete0 = true
			n.typ = t
		default:
			t := c.newStructType(n.LexicalScope(), tag, nil, -1, 1, attr)
			t.isIncomplete0 = true
			n.typ = t
		}
		c.ast.Scope.declare(&c.errors, tag.SrcStr(), n)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// StructDeclarationList:
//
//	StructDeclaration
//
// |       StructDeclarationList StructDeclaration
func (n *StructDeclarationList) check(c *ctx, s *StructOrUnionSpecifier, isVolatile, isConst bool) {
	defer func() {
		if s.typ == nil || s.typ == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
		}
	}()

	// trc("==== (A) %v: %s", n.Position(), NodeSource(n))
	// defer trc("==== (Z) %v: %s", n.Position(), NodeSource(n))
	var fields []*Field
	for l := n; l != nil; l = l.StructDeclarationList {
		fields = append(fields, l.StructDeclaration.check(c, isVolatile, isConst)...)
	}
	for i, f := range fields {
		f.index = i
		// trc("fld %q.%d at %p, set parent type %s", f.Name(), f.Index(), f, s.Type())
		f.parentType = s.Type()
	}
	a := &fieldAllocator{align: 1, fields: fields, list: n}
	switch x := s.typ.(type) {
	case *StructType:
		if len(fields) == 0 {
			x.align = 1
			x.size = 0
			break
		}

		n.checkStruct(c, a, fields)
		x.align = a.align
		x.fields = fields
		x.size = roundup(a.brkBytes, int64(a.align))
		x.padding = int(x.size - a.brkBytes)
	case *UnionType:
		if len(fields) == 0 {
			x.align = 1
			x.size = 0
			break
		}

		n.checkUnion(c, a, fields)
		x.align = a.align
		x.fields = fields
		x.size = roundup(a.brkBytes, int64(a.align))
		x.padding = int(x.size - a.brkBytes)
	default:
		c.errors.add(errorf("%v: internal error", n.Position()))
		return
	}

	// trc("==== %v, align %v, size %v, padding %v", s.Type(), s.Type().Align(), s.Type().Size(), s.Type().(interface{ Padding() int }).Padding())
	// for _, f := range fields {
	// 	trc(
	// 		"%q.%d&%p: IsBitfield %v, Offset %v, AccessBytes %v, OffsetBits %v, OuterGroupOffset %v, InOverlapGroup %v, Mask %#0x, ValueBits %v, GroupSize %v",
	// 		f.Name(), f.Index(), f, f.IsBitfield(), f.Offset(), f.AccessBytes(), f.OffsetBits(), f.OuterGroupOffset(), f.InOverlapGroup(), f.Mask(), f.ValueBits(), f.GroupSize(),
	// 	)
	// }
	// trc("----")
}

func (n *StructDeclarationList) checkUnion(c *ctx, a *fieldAllocator, s []*Field) {
	defer a.close()

	for _, f := range s {
		t := f.Type()
		if isEmpty(t) {
			continue
		}

		switch {
		case f.IsBitfield():
			f.mask = a.mask(int(f.ValueBits()))
			f.accessBytes = bits2AccessBytes(f.ValueBits())
			f.groupSize = int(f.accessBytes)
			a.brkBytes = mathutil.MaxInt64(a.brkBytes, int64(f.groupSize))
		default:
			f.accessBytes = t.Size()
			f.groupSize = int(t.Size())
			a.brkBytes = mathutil.MaxInt64(a.brkBytes, t.Size())
		}
	}
}

type fieldAllocator struct {
	align     int
	bits      int64
	brkBytes  int64
	fields    []*Field
	group     []*Field
	groupSize int64
	list      *StructDeclarationList
}

func (a *fieldAllocator) close() {
	a.closeGroup()
	if len(a.fields) == 0 {
		return
	}

	type group struct {
		off, size int64
		overlaps  bool
	}

	fields := map[int64][]*Field{}
	for _, f := range a.fields {
		fields[f.offsetBytes] = append(fields[f.offsetBytes], f)
	}
	var groups []group
	for k, v := range fields {
		ab := int64(-1)
		for _, f := range v {
			ab = mathutil.MaxInt64(ab, f.AccessBytes())
		}
		for _, f := range v {
			if f.IsBitfield() && f.ValueBits() == 0 {
				continue
			}

			if !isEmpty(f.Type()) {
				f.groupSize = int(ab)
			}
		}
		groups = append(groups, group{k, ab, false})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].off < groups[j].off })
	var g group
	for i, v := range groups {
		// trc("g %+v, v %+v", g, v)
		if g.size == 0 || v.off >= g.off+g.size {
			g = v
			// trc("cont g <- %+v", v)
			continue
		}

		for _, f := range fields[v.off] {
			if f.IsBitfield() {
				f.inOverlapGroup = true
				f.outerGroupOffsetBytes = g.off
			}
			groups[i].overlaps = true
		}
		gsz := v.off - g.off
		for _, f := range fields[g.off] {
			f.groupSize = mathutil.Min(f.groupSize, int(gsz))
		}
	}
	for i, f := range a.fields {
		if f.IsBitfield() && f.ValueBits() == 0 && i != len(a.fields)-1 {
			continue
		}

		a.align = mathutil.Max(a.align, f.Type().Align())
	}
}

func (a *fieldAllocator) closeGroup() {
	if len(a.group) == 0 {
		return
	}

	bits := roundup(a.bits, 8)
	// trc("close group, a.groupSize %v, a.bits %v, bits %v", a.groupSize, a.bits, bits)
	for a.groupSize*8 > bits {
		a.groupSize /= 2
		// trc("close group (C), a.groupSize %v", a.groupSize)
	}
	groupSize := int64(1)
	for groupSize*8 < bits {
		groupSize *= 2
		// trc("close group (B), a.groupSize %v", a.groupSize)
	}
	a.brkBytes = roundup(a.brkBytes, groupSize)
	for _, f := range a.group {
		f.accessBytes = groupSize
		f.offsetBytes = a.brkBytes
		f.groupSize = int(groupSize)
		f.outerGroupOffsetBytes = a.brkBytes
	}
	a.brkBytes += groupSize
	a.group = a.group[:0]
	a.groupSize = 0
	a.bits = 0
	// trc("group closed, a.brkBytes %v", a.brkBytes)
}

func (a *fieldAllocator) mask(bits int) (r uint64) {
	r = ^uint64(0)
	if bits < 64 {
		r >>= 64 - bits
	}
	return r
}

func (a *fieldAllocator) bitField(f *Field) {
	t := f.Type()
	groupSize := t.Size()
	valueBits := f.ValueBits()
	if valueBits == 0 {
		f.inOverlapGroup = true
		a.closeGroup()
		a.brkBytes = roundup(a.brkBytes, f.Type().Size())
		return
	}

	// trc("---- ADD BITFIELD %q typ %v, a.groupSize %v, groupSize %v, a.bits %v, valueBits %v, a.brkBytes %v", f.Name(), t, a.groupSize, groupSize, a.bits, valueBits, a.brkBytes)

	if len(a.group) != 0 {
		if a.groupSize < groupSize {
			if roundup(a.brkBytes, groupSize) != a.brkBytes {
				// trc("%q: a.groupSize %v, groupSize %v, a.brkBytes %v, roundup %v", f.Name(), a.groupSize, groupSize, a.brkBytes, roundup(a.brkBytes, groupSize))
				a.newGroup(f)
				return
			}

			a.groupSize = groupSize
		}

		// a.groupSize >= groupSize:
		// trc("a.groupSize %v, groupSize %v", a.groupSize, groupSize)
		groupSize = a.groupSize
		switch sum := a.bits + valueBits; {
		case sum < 8*groupSize:
			// trc("sum %v, 8*groupSize %v", sum, 8*groupSize)
			f.offsetBits = int(a.bits)
			f.mask = a.mask(int(valueBits)) << a.bits
			a.bits += valueBits
			a.group = append(a.group, f)
			return
		case sum == 8*groupSize:
			// trc("sum %v, 8*groupSize %v", sum, 8*groupSize)
			f.mask = a.mask(int(valueBits)) << a.bits
			f.offsetBits = int(a.bits)
			a.bits += valueBits
			a.group = append(a.group, f)
			a.closeGroup()
			return
		default:
			// trc("sum %v, 8*groupSize %v", sum, 8*groupSize)
			a.closeGroup()
		}
	}

	a.newGroup(f)
}

func (a *fieldAllocator) newGroup(f *Field) {
	a.closeGroup()
	t := f.Type()
	groupSize := t.Size()
	valueBits := f.ValueBits()
	a.brkBytes = roundup(a.brkBytes, t.Size())
	a.bits = valueBits
	a.group = append(a.group[:0], f)
	a.groupSize = groupSize
	// trc("%q: newGroup a.bits %v, a.groupSize %v, a.brkBytes %v", f.Name(), a.bits, a.groupSize, a.brkBytes)
	f.mask = a.mask(int(valueBits))
}

func (a *fieldAllocator) field(f *Field) {
	a.closeGroup()
	t := f.Type()
	sz := t.Size()
	if f.isFlexibleArrayMember || isEmpty(t) {
		sz = 0
	}
	a.brkBytes = roundup(a.brkBytes, int64(t.Align()))
	f.accessBytes = sz
	f.groupSize = int(sz)
	f.offsetBytes = a.brkBytes
	a.brkBytes += f.accessBytes
}

func (n *StructDeclarationList) checkStruct(c *ctx, a *fieldAllocator, s []*Field) {
	defer a.close()

	for i, f := range s {
		if ft := f.Type(); (ft.IsIncomplete() || ft.Size() == 0) &&
			ft.Kind() == Array && i == len(s)-1 { // https://en.wikipedia.org/wiki/Flexible_array_member
			switch x := ft.(type) {
			case *ArrayType:
				f.isFlexibleArrayMember = !x.IsVLA()
			default:
				f.isFlexibleArrayMember = true
			}
		}

		switch {
		case f.IsBitfield():
			a.bitField(f)
		default:
			a.field(f)
		}
	}
}

func (n *StructDeclaration) check(c *ctx, isVolatile0, isConst0 bool) (r []*Field) {
	switch n.Case {
	case StructDeclarationDecl: // DeclarationSpecifiers InitDeclaratorList AttributeSpecifierList ';'
		var isAtomic, isConst, isVolatile, isRestrict bool
		var alignas int
		isVolatile = isVolatile0
		isConst = isConst0
		t := n.SpecifierQualifierList.check(c, &isAtomic, &isConst, &isVolatile, &isRestrict, &alignas)
		switch {
		case n.StructDeclaratorList == nil:
			f := &Field{typ: newTyper(t)}
			switch x := f.Type().(type) {
			case *StructType:
				for _, v := range x.fields {
					// trc("fld at %p, %q set parent fdl %q at %p", v, v.Name(), f.Name(), f)
					v.parentField2 = f
				}
			case *UnionType:
				for _, v := range x.fields {
					// trc("fld at %p, %q set parent fdl %q at %p", v, v.Name(), f.Name(), f)
					v.parentField2 = f
				}
			default:
				trc("%v: %T %s", n.Position(), x, x)
			}
			return []*Field{f}
		default:
			return n.StructDeclaratorList.check(c, t, isAtomic, isConst, isVolatile, isRestrict)
		}
	case StructDeclarationAssert: // StaticAssertDeclaration
		n.StaticAssertDeclaration.check(c)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return nil
}

func (n *SpecifierQualifierList) check(c *ctx, isAtomic, isConst, isVolatile, isRestrict *bool, alignas *int) (r Type) {
	if n == nil {
		return c.intT
	}

	var ts []TypeSpecifierCase

	var attr *Attributes
	n0 := n
	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO missed/failed type check %v: %v %T", n.Position(), ts, n))
			return
		}

		if attr != nil {
			var err error
			if r, err = mergeAttr(r, attr); err != nil {
				c.errors.add(errorf("%v", err))
			}
		}
	}()

	var attrs []*Attributes
	for ; n != nil; n = n.SpecifierQualifierList {
		switch n.Case {
		case SpecifierQualifierListTypeSpec: // TypeSpecifier SpecifierQualifierList
			ts = append(ts, n.TypeSpecifier.Case)
			r = n.TypeSpecifier.check(c, isAtomic, *isVolatile, *isConst)
		case SpecifierQualifierListTypeQual: // TypeQualifier SpecifierQualifierList
			if attr := n.TypeQualifier.check(c, isConst, isVolatile, isAtomic, isRestrict); attr != nil {
				attrs = append(attrs, attr)
			}
		case SpecifierQualifierListAlignSpec: // AlignmentSpecifier SpecifierQualifierList
			if v := n.AlignmentSpecifier.check(c).Align(); v > 0 {
				*alignas = v
			}
		default:
			c.errors.add(errorf("internal error: %v", n.Case))
		}
	}
	switch len(attrs) {
	case 0:
		// ok
	case 1:
		attr = attrs[0]
	default:
		c.errors.add(errorf("TODO %T", n0.Position()))
	}
	t, ok := c.builtinTypes[ts2String(ts)]
	if ok {
		return t
	}

	switch len(ts) {
	case 0:
		return c.intT
	case 1:
		switch ts[0] {
		case
			TypeSpecifierAtomic,
			TypeSpecifierEnum,
			TypeSpecifierStructOrUnion,
			TypeSpecifierTypeName,
			TypeSpecifierTypeofExpr,
			TypeSpecifierTypeofType:

			return r
		}
	}

	return nil
}

func (n *StructDeclaratorList) check(c *ctx, t Type, isAtomic, isConst, isVolatile, isRestrict bool) (r []*Field) {
	if t == nil {
		c.errors.add(errorf("TODO %T internal error", n))
		return
	}

	for ; n != nil; n = n.StructDeclaratorList {
		r = append(r, n.StructDeclarator.check(c, t, isAtomic, isConst, isVolatile, isRestrict))
	}
	return r
}

func (n *StructDeclarator) check(c *ctx, t Type, isAtomic, isConst, isVolatile, isRestrict bool) (r *Field) {
	if t == nil {
		c.errors.add(errorf("TODO %T internal error", n))
		return
	}

	defer func() {
		if r == nil {
			return
		}

		switch x := r.Type().(type) {
		case *StructType:
			for _, v := range x.fields {
				// trc("fld at %p, %q set parent fdl %q at %p", v, v.Name(), r.Name(), r)
				v.parentField2 = r
			}
		case *UnionType:
			for _, v := range x.fields {
				// trc("fld at %p, %q set parent fdl %q at %p", v, v.Name(), r.Name(), r)
				v.parentField2 = r
			}
		default:
			// trc("%v: %T %s", n.Position(), x, x)
		}
	}()

	if n.Declarator != nil {
		n.Declarator.isAtomic = isAtomic
		n.Declarator.isConst = isConst
		n.Declarator.isVolatile = isVolatile
		n.Declarator.isRestrict = isRestrict
		defer func() {
			n.Declarator.fixVolatileAndConst(c)
			r.typ.typ = n.Declarator.Type()
		}()
	}
	switch n.Case {
	case StructDeclaratorDecl: // Declarator
		return &Field{declarator: n.Declarator, typ: newTyper(n.Declarator.check(c, t))}
	case StructDeclaratorBitField: // Declarator ':' ConstantExpression
		et := n.ConstantExpression.check(c, decay)
		if !IsIntegerType(et) {
			c.errors.add(errorf("%v: expected integer expression: %s", n.ConstantExpression.Position(), et))
			break
		}

		var bits int64
		switch x := n.ConstantExpression.Value().(type) {
		case Int64Value:
			bits = int64(x)
		case UInt64Value:
			bits = int64(x)
		}
		if bits < 0 || bits > 64 {
			c.errors.add(errorf("%v: value out of range: %v", n.ConstantExpression.Position(), bits))
			break
		}

		return &Field{declarator: n.Declarator, typ: newTyper(n.Declarator.check(c, t)), valueBits: bits, isBitField: true}
	default:
		c.errors.add(errorf("internal error: %v %T", n.Case, n))
	}
	return nil //TODO-
}

// AssignmentExpression:
//
//	ConditionalExpression                       // Case AssignmentExpressionCond
//
// |       UnaryExpression '=' AssignmentExpression    // Case AssignmentExpressionAssign
// |       UnaryExpression "*=" AssignmentExpression   // Case AssignmentExpressionMul
// |       UnaryExpression "/=" AssignmentExpression   // Case AssignmentExpressionDiv
// |       UnaryExpression "%=" AssignmentExpression   // Case AssignmentExpressionMod
// |       UnaryExpression "+=" AssignmentExpression   // Case AssignmentExpressionAdd
// |       UnaryExpression "-=" AssignmentExpression   // Case AssignmentExpressionSub
// |       UnaryExpression "<<=" AssignmentExpression  // Case AssignmentExpressionLsh
// |       UnaryExpression ">>=" AssignmentExpression  // Case AssignmentExpressionRsh
// |       UnaryExpression "&=" AssignmentExpression   // Case AssignmentExpressionAnd
// |       UnaryExpression "^=" AssignmentExpression   // Case AssignmentExpressionXor
// |       UnaryExpression "|=" AssignmentExpression   // Case AssignmentExpressionOr
func (n *AssignmentExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.eval(c, mode)
	}()

	switch n.Case {
	case AssignmentExpressionCond: // ConditionalExpression
		n.typ = n.ConditionalExpression.check(c, mode)
		n.setPure(n.ConditionalExpression.Pure())
	case
		AssignmentExpressionAssign, // UnaryExpression '=' AssignmentExpression
		AssignmentExpressionMul,    // UnaryExpression "*=" AssignmentExpression
		AssignmentExpressionDiv,    // UnaryExpression "/=" AssignmentExpression
		AssignmentExpressionMod,    // UnaryExpression "%=" AssignmentExpression
		AssignmentExpressionAdd,    // UnaryExpression "+=" AssignmentExpression
		AssignmentExpressionSub,    // UnaryExpression "-=" AssignmentExpression
		AssignmentExpressionLsh,    // UnaryExpression "<<=" AssignmentExpression
		AssignmentExpressionRsh,    // UnaryExpression ">>=" AssignmentExpression
		AssignmentExpressionAnd,    // UnaryExpression "&=" AssignmentExpression
		AssignmentExpressionXor,    // UnaryExpression "^=" AssignmentExpression
		AssignmentExpressionOr:     // UnaryExpression "|=" AssignmentExpression

		n.setPure(false)
		n.typ = n.UnaryExpression.check(c, mode)
		if b := n.UnaryExpression.Type().BitField(); b != nil {
			if t, ok := n.typ.(*PredefinedType); ok {
				t.setBitField(b)
			}
		}
		c.assignTo(n.UnaryExpression, n.Case == AssignmentExpressionAssign)
		a := n.Type()
		b := n.AssignmentExpression.check(c, mode)
		if !isModifiableLvalue(a) {
			c.errors.add(errorf("%v: left operand shall be a modifiable lvalue", n.UnaryExpression.Position()))
			break
		}

		switch {
		case
			// — the left operand has qualified or unqualified arithmetic type and the
			// right has arithmetic type;
			IsArithmeticType(a) && IsArithmeticType(b),

			// — the left operand has a qualified or unqualified version of a structure or
			// union type compatible with the type of the right;
			a.Kind() == Struct && b.Kind() == Struct || a.Kind() == Union && b.Kind() == Union,

			// — both operands are pointers to qualified or unqualified versions of
			// compatible types, and the type pointed to by the left has all the qualifiers
			// of the type pointed to by the right;
			//
			// — one operand is a pointer to an object or incomplete type and the other is
			// a pointer to a qualified or unqualified version of void, and the type
			// pointed to by the left has all the qualifiers of the type pointed to by the
			// right;
			isPointerType(a) && isPointerType(b),

			// — the left operand is a pointer and the right is a null pointer constant; or
			isPointerType(a) && IsIntegerType(b),

			// — the left operand has type _Bool and the right is a pointer.
			a.Kind() == Bool && isPointerType(b):

			// ok
		}
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

func (c *ctx) assignTo(n Node, unread bool) {
	switch x := n.(type) {
	case *PrimaryExpression:
		switch x.Case {
		case PrimaryExpressionExpr: // '(' ExpressionList ')'
			c.assignTo(x.ExpressionList, unread)
		case PrimaryExpressionIdent: // IDENTIFIER
			switch y := x.ResolvedTo().(type) {
			case *Declarator:
				if unread {
					y.read--
				}
				y.write++
			default:
				c.errors.add(errorf("internal error: %T", y))
			}
		default:
			c.errors.add(errorf("internal error: %v", x.Case))
		}
	case *PostfixExpression:
		switch x.Case {
		case PostfixExpressionIndex: // PostfixExpression '[' ExpressionList ']'
			//nop
		case PostfixExpressionSelect: // PostfixExpression '.' IDENTIFIER
			c.assignTo(x.PostfixExpression, unread)
		case PostfixExpressionPSelect: // PostfixExpression "->" IDENTIFIER
			// nop
		case PostfixExpressionComplit: // '(' TypeName ')' '{' InitializerList ',' '}'
			// nop
		default:
			c.errors.add(errorf("internal error: %v", x.Case))
		}
	case *UnaryExpression:
		switch x.Case {
		case UnaryExpressionDeref: // '*' CastExpression
			// nop
		case UnaryExpressionReal: // "__real__" UnaryExpression
			c.assignTo(x.UnaryExpression, unread)
		case UnaryExpressionImag: // "__imag__" UnaryExpression
			c.assignTo(x.UnaryExpression, unread)
		default:
			c.errors.add(errorf("internal error: %v", x.Case))
		}
	case *CastExpression:
		switch x.Case {
		case CastExpressionUnary: // UnaryExpression
			c.assignTo(x.UnaryExpression, unread)
		default:
			c.errors.add(errorf("internal error: %v", x.Case))
		}
	default:
		c.errors.add(errorf("internal error: %T", x))
	}
}

// ConditionalExpression:
//
//	LogicalOrExpression                                           // Case ConditionalExpressionLOr
//
// |       LogicalOrExpression '?' ExpressionList ':' ConditionalExpression  // Case ConditionalExpressionCond
func (n *ConditionalExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.eval(c, mode)
	}()

	switch n.Case {
	case ConditionalExpressionLOr: // LogicalOrExpression
		n.typ = n.LogicalOrExpression.check(c, mode)
		n.setPure(n.LogicalOrExpression.Pure())
	case ConditionalExpressionCond: // LogicalOrExpression '?' ExpressionList ':' ConditionalExpression
		mode = mode.add(decay)
		t1 := n.LogicalOrExpression.check(c, mode)
		if !IsScalarType(t1) {
			c.errors.add(errorf("%v: operand shall have scalar type: %s", n.LogicalOrExpression.Position(), t1))
			break
		}

		t2 := t1
		if n.ExpressionList != nil {
			t2 = n.ExpressionList.check(c, mode)
			n.ExpressionList.eval(c, decay)
		}
		switch t3 := n.ConditionalExpression.check(c, mode); {
		case
			// both operands have arithmetic type;
			IsArithmeticType(t2) && IsArithmeticType(t3):
			n.typ = UsualArithmeticConversions(t2, t3)
		case
			// both operands have the same structure or union type;
			(t2.Kind() == Struct || t2.Kind() == Union) && t3.Kind() == t2.Kind():
			n.typ = t2
		case
			// both operands have void type;
			t2.Kind() == Void && t3.Kind() == Void:
			n.typ = t2
		case
			// both operands are pointers to qualified or unqualified versions of compatible types;
			isPointerType(t2) && isPointerType(t3):
			if t2.Undecay().Kind() == Array {
				t2 = t2.(*PointerType).Elem().Pointer()
			}
			n.typ = t2
		case
			// one operand is a pointer and the other is a null pointer constant; or
			isPointerType(t2) && IsIntegerType(t3):
			n.typ = t2
		case
			IsIntegerType(t2) && isPointerType(t3):
			n.typ = t3
		case t2.Kind() == Void:
			n.typ = t2
		case t3.Kind() == Void:
			n.typ = t3
		default:
			c.errors.add(errorf("TODO %v, t1 %v, t2 %v, t3 %v", n.Case, t1, t2, t3))
		}
		switch {
		case isNonzero(n.LogicalOrExpression.Value()):
			n.setPure(n.LogicalOrExpression.Pure() && n.ExpressionList.Pure())
		case isZero(n.LogicalOrExpression.Value()):
			n.setPure(n.LogicalOrExpression.Pure() && n.ConditionalExpression.Pure())
		default:
			n.setPure(n.LogicalOrExpression.Pure() && (n.ExpressionList == nil || n.ExpressionList.Pure()) && (n.ConditionalExpression == nil || n.ConditionalExpression.Pure()))
		}
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// LogicalOrExpression:
//
//	LogicalAndExpression                           // Case LogicalOrExpressionLAnd
//
// |       LogicalOrExpression "||" LogicalAndExpression  // Case LogicalOrExpressionLOr
func (n *LogicalOrExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.eval(c, mode)
	}()

	switch n.Case {
	case LogicalOrExpressionLAnd: // LogicalAndExpression
		n.typ = n.LogicalAndExpression.check(c, mode)
		n.setPure(n.LogicalAndExpression.Pure())
	case LogicalOrExpressionLOr: // LogicalOrExpression "||" LogicalAndExpression
		mode = mode.add(decay)
		switch a, b := n.LogicalOrExpression.check(c, mode), n.LogicalAndExpression.check(c, mode); {
		case !IsScalarType(a) || !IsScalarType(b):
			c.errors.add(errorf("%v: operands shall be scalars: %s and %s", n.Token.Position(), a, b))
		default:
			n.typ = c.intT
		}
		n.setPure(n.LogicalOrExpression.Pure() && n.LogicalAndExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// LogicalAndExpression:
//
//	InclusiveOrExpression                            // Case LogicalAndExpressionOr
//
// |       LogicalAndExpression "&&" InclusiveOrExpression  // Case LogicalAndExpressionLAnd
func (n *LogicalAndExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.eval(c, mode)
	}()

	switch n.Case {
	case LogicalAndExpressionOr: // InclusiveOrExpression
		n.typ = n.InclusiveOrExpression.check(c, mode)
		n.setPure(n.InclusiveOrExpression.Pure())
	case LogicalAndExpressionLAnd: // LogicalAndExpression "&&" InclusiveOrExpression
		mode = mode.add(decay)
		switch a, b := n.LogicalAndExpression.check(c, mode), n.InclusiveOrExpression.check(c, mode); {
		case !IsScalarType(a) || !IsScalarType(b):
			c.errors.add(errorf("%v: operands shall be scalars: %s and %s", n.Token.Position(), a, b))
		default:
			n.typ = c.intT
		}
		n.setPure(n.LogicalAndExpression.Pure() && n.InclusiveOrExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// InclusiveOrExpression:
//
//	ExclusiveOrExpression                            // Case InclusiveOrExpressionXor
//
// |       InclusiveOrExpression '|' ExclusiveOrExpression  // Case InclusiveOrExpressionOr
func (n *InclusiveOrExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.typ = c.notVolatileNotConst(n, n.Type())
		n.eval(c, mode)
	}()

	switch n.Case {
	case InclusiveOrExpressionXor: // ExclusiveOrExpression
		n.typ = n.ExclusiveOrExpression.check(c, mode)
		n.setPure(n.ExclusiveOrExpression.Pure())
	case InclusiveOrExpressionOr: // InclusiveOrExpression '|' ExclusiveOrExpression
		mode = mode.add(decay)
		switch a, b := n.InclusiveOrExpression.check(c, mode), n.ExclusiveOrExpression.check(c, mode); {
		case !IsIntegerType(a) || !IsIntegerType(b):
			c.errors.add(errorf("%v: operands shall have integer type: %s and %s", n.Token.Position(), a, b))
		default:
			n.typ = UsualArithmeticConversions(a, b)
		}
		n.setPure(n.InclusiveOrExpression.Pure() && n.ExclusiveOrExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// ExclusiveOrExpression:
//
//	AndExpression                            // Case ExclusiveOrExpressionAnd
//
// |       ExclusiveOrExpression '^' AndExpression  // Case ExclusiveOrExpressionXor
func (n *ExclusiveOrExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.typ = c.notVolatileNotConst(n, n.Type())
		n.eval(c, mode)
	}()

	switch n.Case {
	case ExclusiveOrExpressionAnd: // AndExpression
		n.typ = n.AndExpression.check(c, mode)
		n.setPure(n.AndExpression.Pure())
	case ExclusiveOrExpressionXor: // ExclusiveOrExpression '^' AndExpression
		mode = mode.add(decay)
		switch a, b := n.ExclusiveOrExpression.check(c, mode), n.AndExpression.check(c, mode); {
		case !IsIntegerType(a) || !IsIntegerType(b):
			c.errors.add(errorf("%v: operands shall have integer type: %s and %s", n.Token.Position(), a, b))
		default:
			n.typ = UsualArithmeticConversions(a, b)
		}
		n.setPure(n.ExclusiveOrExpression.Pure() && n.AndExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// AndExpression:
//
//	EqualityExpression                    // Case AndExpressionEq
//
// |       AndExpression '&' EqualityExpression  // Case AndExpressionAnd
func (n *AndExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.typ = c.notVolatileNotConst(n, n.Type())
		n.eval(c, mode)
	}()

	switch n.Case {
	case AndExpressionEq: // EqualityExpression
		n.typ = n.EqualityExpression.check(c, mode)
		n.setPure(n.EqualityExpression.Pure())
	case AndExpressionAnd: // AndExpression '&' EqualityExpression
		mode = mode.add(decay)
		switch a, b := n.AndExpression.check(c, mode), n.EqualityExpression.check(c, mode); {
		case !IsIntegerType(a) || !IsIntegerType(b):
			c.errors.add(errorf("%v: operands shall have integer type: %s and %s", n.Token.Position(), a, b))
		default:
			n.typ = UsualArithmeticConversions(a, b)
		}
		n.setPure(n.AndExpression.Pure() && n.EqualityExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// EqualityExpression:
//
//	RelationalExpression                          // Case EqualityExpressionRel
//
// |       EqualityExpression "==" RelationalExpression  // Case EqualityExpressionEq
// |       EqualityExpression "!=" RelationalExpression  // Case EqualityExpressionNeq
func (n *EqualityExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.typ = c.notVolatileNotConst(n, n.Type())
		n.eval(c, mode)
	}()

	switch n.Case {
	case EqualityExpressionRel: // RelationalExpression
		n.typ = n.RelationalExpression.check(c, mode)
		n.setPure(n.RelationalExpression.Pure())
	case
		EqualityExpressionEq,  // EqualityExpression "==" RelationalExpression
		EqualityExpressionNeq: // EqualityExpression "!=" RelationalExpression

		mode = mode.add(decay)
		switch a, b := n.EqualityExpression.check(c, mode), n.RelationalExpression.check(c, mode); {
		case
			// both operands have arithmetic type;
			IsArithmeticType(a) && IsArithmeticType(b),

			// both operands are pointers to qualified or unqualified versions of
			// compatible types;
			//
			// one operand is a pointer to an object or incomplete type and the other is a
			// pointer to a qualified or unqualified version of void;
			isPointerType(a) && isPointerType(b),

			// one operand is a pointer and the other is a null pointer constant.
			isPointerType(a) && IsIntegerType(b) || isPointerType(b) && IsIntegerType(a):

			n.typ = c.intT
		default:
			c.errors.add(errorf("%v: invalid operands: %s (%v) and %s (%v)", n.Token.Position(), NodeSource(n.EqualityExpression), a, NodeSource(n.RelationalExpression), b))
		}
		n.setPure(n.EqualityExpression.Pure() && n.RelationalExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// RelationalExpression:
//
//	ShiftExpression                            // Case RelationalExpressionShift
//
// |       RelationalExpression '<' ShiftExpression   // Case RelationalExpressionLt
// |       RelationalExpression '>' ShiftExpression   // Case RelationalExpressionGt
// |       RelationalExpression "<=" ShiftExpression  // Case RelationalExpressionLeq
// |       RelationalExpression ">=" ShiftExpression  // Case RelationalExpressionGeq
func (n *RelationalExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.typ = c.notVolatileNotConst(n, n.Type())
		n.eval(c, mode)
	}()

	switch n.Case {
	case RelationalExpressionShift: // ShiftExpression
		n.typ = n.ShiftExpression.check(c, mode)
		n.setPure(n.ShiftExpression.Pure())
	case
		RelationalExpressionLt,  // RelationalExpression '<' ShiftExpression
		RelationalExpressionGt,  // RelationalExpression '>' ShiftExpression
		RelationalExpressionLeq, // RelationalExpression "<=" ShiftExpression
		RelationalExpressionGeq: // RelationalExpression ">=" ShiftExpression

		n.typ = c.intT
		mode = mode.add(decay)
		switch a, b := n.RelationalExpression.check(c, mode), n.ShiftExpression.check(c, mode); {
		case
			// both operands have real type;
			IsRealType(a) && IsRealType(b),
			// both operands are pointers to qualified or unqualified versions of
			// compatible object types;
			//
			// both operands are pointers to qualified or unqualified versions of
			// compatible incomplete types
			//
			// gcc allows mixing ints and pointers
			IsScalarType(a) && IsScalarType(b):

			// ok
		default:
			c.errors.add(errorf("%v: invalid operands: %s and %s", n.Token.Position(), a, b))
		}
		n.setPure(n.RelationalExpression.Pure() && n.ShiftExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// ShiftExpression:
//
//	AdditiveExpression                       // Case ShiftExpressionAdd
//
// |       ShiftExpression "<<" AdditiveExpression  // Case ShiftExpressionLsh
// |       ShiftExpression ">>" AdditiveExpression  // Case ShiftExpressionRsh
func (n *ShiftExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.typ = c.notVolatileNotConst(n, n.Type())
		n.eval(c, mode)
	}()

	switch n.Case {
	case ShiftExpressionAdd: // AdditiveExpression
		n.typ = n.AdditiveExpression.check(c, mode)
		n.setPure(n.AdditiveExpression.Pure())
	case
		ShiftExpressionLsh, // ShiftExpression "<<" AdditiveExpression
		ShiftExpressionRsh: // ShiftExpression ">>" AdditiveExpression

		mode = mode.add(decay)
		switch a, b := n.ShiftExpression.check(c, mode), n.AdditiveExpression.check(c, mode); {
		case !IsScalarType(a) || !IsScalarType(b):
			c.errors.add(errorf("%v: operands shall be a scalars: %s and %s", n.Token.Position(), a, b))
		default:
			n.typ = IntegerPromotion(a)
			if b := a.BitField(); b != nil {
				if t, ok := n.typ.(*PredefinedType); ok {
					n.typ = t.setBitField(b)
				}
			}
		}
		n.setPure(n.ShiftExpression.Pure() && n.AdditiveExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

func (c *ctx) notVolatileNotConst(n Node, t Type) Type {
	attr := t.Attributes()
	if attr == nil || (!attr.IsVolatile() && !attr.IsConst()) {
		return t
	}

	a := *attr
	a.isVolatile = false
	a.isConst = false
	return t.setAttr(&a)
}

// AdditiveExpression:
//
//	MultiplicativeExpression                         // Case AdditiveExpressionMul
//
// |       AdditiveExpression '+' MultiplicativeExpression  // Case AdditiveExpressionAdd
// |       AdditiveExpression '-' MultiplicativeExpression  // Case AdditiveExpressionSub
func (n *AdditiveExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.typ = c.notVolatileNotConst(n, n.Type())
		n.eval(c, mode)
	}()

	switch n.Case {
	case AdditiveExpressionMul: // MultiplicativeExpression
		n.typ = n.MultiplicativeExpression.check(c, mode)
		n.setPure(n.MultiplicativeExpression.Pure())
	case AdditiveExpressionAdd: // AdditiveExpression '+' MultiplicativeExpression
		mode = mode.add(decay)
		switch a, b := n.AdditiveExpression.check(c, mode), n.MultiplicativeExpression.check(c, mode); {
		case
			// For addition, either both operands shall have arithmetic type
			IsArithmeticType(a) && IsArithmeticType(b):
			n.typ = UsualArithmeticConversions(a, b)
		case
			// or one operand shall be a pointer to an object type and the other shall have
			// integer type.
			isPointerType(a) && IsIntegerType(b):
			n.typ = a
		case IsIntegerType(a) && isPointerType(b):
			n.typ = b
		default:
			c.errors.add(errorf("%v: invalid operands: %s and %s", n.Token.Position(), a, b))
		}
		n.setPure(n.AdditiveExpression.Pure() && n.MultiplicativeExpression.Pure())
	case AdditiveExpressionSub: // AdditiveExpression '-' MultiplicativeExpression
		mode = mode.add(decay)
		switch a, b := n.AdditiveExpression.check(c, mode), n.MultiplicativeExpression.check(c, mode); {
		case
			// both operands have arithmetic type;
			IsArithmeticType(a) && IsArithmeticType(b):
			n.typ = UsualArithmeticConversions(a, b)
		case
			// both operands are pointers to qualified or unqualified versions of
			// compatible object types;
			isPointerType(a) && isPointerType(b):
			n.typ = c.ptrDiffT(n)
		case
			// the left operand is a pointer to an object type and the right operand has
			// integer type.
			isPointerType(a) && IsIntegerType(b):
			n.typ = a
		default:
			c.errors.add(errorf("%v: invalid operands: %s and %s", n.Token.Position(), a, b))
		}
		n.setPure(n.AdditiveExpression.Pure() && n.MultiplicativeExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// MultiplicativeExpression:
//
//	CastExpression                               // Case MultiplicativeExpressionCast
//
// |       MultiplicativeExpression '*' CastExpression  // Case MultiplicativeExpressionMul
// |       MultiplicativeExpression '/' CastExpression  // Case MultiplicativeExpressionDiv
// |       MultiplicativeExpression '%' CastExpression  // Case MultiplicativeExpressionMod
func (n *MultiplicativeExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.typ = c.notVolatileNotConst(n, n.Type())
		n.eval(c, mode)
	}()

	switch n.Case {
	case MultiplicativeExpressionCast: // CastExpression
		n.typ = n.CastExpression.check(c, mode)
		n.setPure(n.CastExpression.Pure())
	case
		MultiplicativeExpressionMul, // MultiplicativeExpression '*' CastExpression
		MultiplicativeExpressionDiv: // MultiplicativeExpression '/' CastExpression

		mode = mode.add(decay)
		switch a, b := n.MultiplicativeExpression.check(c, mode), n.CastExpression.check(c, mode); {
		case !IsArithmeticType(a) || !IsArithmeticType(b):
			c.errors.add(errorf("%v: operands shall have arithmetic type: %s and %s", n.Token.Position(), a, b))
		default:
			n.typ = UsualArithmeticConversions(a, b)
		}
		n.setPure(n.MultiplicativeExpression.Pure() && n.CastExpression.Pure())
	case MultiplicativeExpressionMod: // MultiplicativeExpression '%' CastExpression
		mode = mode.add(decay)
		switch a, b := n.MultiplicativeExpression.check(c, mode), n.CastExpression.check(c, mode); {
		case !IsIntegerType(a) || !IsIntegerType(b):
			c.errors.add(errorf("%v: operands shall have integer type: %s and %s", n.Token.Position(), a, b))
		default:
			n.typ = UsualArithmeticConversions(a, b)
		}
		n.setPure(n.MultiplicativeExpression.Pure() && n.CastExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

// CastExpression:
//
//	UnaryExpression                  // Case CastExpressionUnary
//
// |       '(' TypeName ')' CastExpression  // Case CastExpressionCast
func (n *CastExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.eval(c, mode)
	}()

	switch n.Case {
	case CastExpressionUnary: // UnaryExpression
		n.typ = n.UnaryExpression.check(c, mode)
		n.setPure(n.UnaryExpression.Pure())
	case CastExpressionCast: // '(' TypeName ')' CastExpression
		n.typ = n.TypeName.check(c)
		n.CastExpression.check(c, mode.add(decay))
		n.setPure(n.CastExpression.Pure())
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

func (n *TypeName) check(c *ctx) (r Type) {
	var dummy, isConst bool
	var dummyInt int
	n.typ = n.AbstractDeclarator.check(c, n.SpecifierQualifierList.check(c, &dummy, &isConst, &dummy, &dummy, &dummyInt))
	return n.Type()
}

// UnaryExpression:
//
//	PostfixExpression            // Case UnaryExpressionPostfix
//
// |       "++" UnaryExpression         // Case UnaryExpressionInc
// |       "--" UnaryExpression         // Case UnaryExpressionDec
// |       '&' CastExpression           // Case UnaryExpressionAddrof
// |       '*' CastExpression           // Case UnaryExpressionDeref
// |       '+' CastExpression           // Case UnaryExpressionPlus
// |       '-' CastExpression           // Case UnaryExpressionMinus
// |       '~' CastExpression           // Case UnaryExpressionCpl
// |       '!' CastExpression           // Case UnaryExpressionNot
// |       "sizeof" UnaryExpression     // Case UnaryExpressionSizeofExpr
// |       "sizeof" '(' TypeName ')'    // Case UnaryExpressionSizeofType
// |       "&&" IDENTIFIER              // Case UnaryExpressionLabelAddr
// |       "_Alignof" UnaryExpression   // Case UnaryExpressionAlignofExpr
// |       "_Alignof" '(' TypeName ')'  // Case UnaryExpressionAlignofType
// |       "__imag__" UnaryExpression   // Case UnaryExpressionImag
// |       "__real__" UnaryExpression   // Case UnaryExpressionReal
func (n *UnaryExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check %v", n, n.Case))
			return
		}

		n.eval(c, mode)
	}()

	switch n.Case {
	case UnaryExpressionPostfix: // PostfixExpression
		n.typ = n.PostfixExpression.check(c, mode)
		n.setPure(n.PostfixExpression.Pure())
	case
		UnaryExpressionInc, // "++" UnaryExpression
		UnaryExpressionDec: // "--" UnaryExpression

		n.setPure(false)
		n.typ = n.UnaryExpression.check(c, mode.add(decay))
		c.assignTo(n.UnaryExpression, false)
		if !IsRealType(n.Type()) && !isPointerType(n.Type()) {
			c.errors.add(errorf("%v: operand shall have real or pointer type: %s", n.UnaryExpression.Position(), n.Type()))
		}
	case UnaryExpressionAddrof: // '&' CastExpression
		t := n.CastExpression.check(c, mode.del(decay))
		n.setPure(n.CastExpression.Pure())
		c.takeAddr(n.CastExpression)
		switch {
		case
			// The operand of the unary & operator shall be either a function designator,
			t.Kind() == Function,

			// the result of a [] or unary * operator, or an lvalue that designates an
			// object that is not a bit-field and is not declared with the register
			// storage-class specifier.
			isLvalue(t),

			// gcc extension: gcc-9.1.0/gcc/testsuite/gcc.c-torture/compile/20010327-1.c
			t.Kind() == Void:

			n.typ = c.newPointerType(t)
		default:
			c.errors.add(errorf("%v: invalid operand: %s", n.CastExpression.Position(), t))
		}
	case UnaryExpressionDeref: // '*' CastExpression
		switch t := n.CastExpression.check(c, mode.add(decay)); t.Kind() {
		case Ptr:
			n.typ = c.decay(t.(*PointerType).Elem(), mode)
		default:
			c.errors.add(errorf("%v: operand shall be a pointer: %s", n.CastExpression.Position(), t))
		}
		n.setPure(n.CastExpression.Pure())
	case
		UnaryExpressionPlus,  // '+' CastExpression
		UnaryExpressionMinus: // '-' CastExpression

		n.typ = IntegerPromotion(n.CastExpression.check(c, mode.add(decay)))
		n.setPure(n.CastExpression.Pure())
		if !IsArithmeticType(n.Type()) {
			c.errors.add(errorf("%v: expected arithmetic type: %s", n.Position(), n.CastExpression.Type()))
		}
	case UnaryExpressionCpl: // '~' CastExpression
		t := n.CastExpression.check(c, mode.add(decay))
		n.setPure(n.CastExpression.Pure())
		switch {
		case IsIntegerType(t):
			n.typ = IntegerPromotion(t)
		case IsComplexType(t): // GCC extension, complex conjugate
			n.typ = t
		default:
			c.errors.add(errorf("%v: expected integer type: %s", n.Position(), n.CastExpression.Type()))
		}
	case UnaryExpressionNot: // '!' CastExpression
		t := n.CastExpression.check(c, mode.add(decay))
		n.setPure(n.CastExpression.Pure())
		if !IsScalarType(t) {
			c.errors.add(errorf("%v: expected scalar type: %s", n.Position(), n.CastExpression.Type()))
		}
		n.typ = c.intT
	case UnaryExpressionSizeofExpr: // "sizeof" UnaryExpression
		sv := c.checkingSizeof
		defer func() { c.checkingSizeof = sv }()
		c.checkingSizeof = true
		n.typ = c.sizeT(n)
		t := n.UnaryExpression.check(c, mode).Undecay()
		n.setPure(n.UnaryExpression.Pure())
		if t.Kind() == Function {
			t = c.newPointerType(t)
		}
		if t.IsIncomplete() {
			if x, ok := t.(*ArrayType); ok && x.IsVLA() {
				break
			}

			c.errors.add(errorf("%v: sizeof incomplete type: %s", n.UnaryExpression.Position(), t))
		}
		n.val = UInt64Value(t.Size())
	case UnaryExpressionSizeofType: // "sizeof" '(' TypeName ')'
		n.setPure(true)
		n.typ = c.sizeT(n)
		t := n.TypeName.check(c)
		if t.IsIncomplete() {
			if x, ok := t.(*ArrayType); ok && x.IsVLA() {
				break
			}

			c.errors.add(errorf("%v: sizeof incomplete type: %s", n.TypeName.Position(), t))
		}
		n.val = UInt64Value(t.Size())
	case UnaryExpressionLabelAddr: // "&&" IDENTIFIER
		n.setPure(true)
		n.typ = c.pvoidT
	case UnaryExpressionAlignofExpr: // "_Alignof" UnaryExpression
		n.setPure(true)
		t := n.UnaryExpression.check(c, mode.add(decay))
		n.val, n.typ = UInt64Value(t.Align()), c.sizeT(n)
	case UnaryExpressionAlignofType: // "_Alignof" '(' TypeName ')'
		n.setPure(true)
		t := n.TypeName.check(c)
		n.val, n.typ = UInt64Value(t.Align()), c.sizeT(n)
	case
		UnaryExpressionImag, // "__imag__" UnaryExpression
		UnaryExpressionReal: // "__real__" UnaryExpression

		t := n.UnaryExpression.check(c, mode.add(decay))
		n.setPure(n.UnaryExpression.Pure())
		if !IsComplexType(t) {
			c.errors.add(errorf("%v: expected complex type: %s", n.Position(), t))
			break
		}

		n.typ = c.ast.kinds[correspondingRealKinds[t.Kind()]]
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

func (c *ctx) takeAddr(n Node) {
	switch x := n.(type) {
	case *PrimaryExpression:
		switch x.Case {
		case PrimaryExpressionExpr: // '(' ExpressionList ')'
			c.takeAddr(x.ExpressionList)
		case PrimaryExpressionIdent: // IDENTIFIER
			switch y := x.ResolvedTo().(type) {
			case *Declarator:
				y.addrTaken = true
			default:
				c.errors.add(errorf("internal error: %T", y))
			}
		case PrimaryExpressionString: // STRINGLITERAL
			// nop
		case PrimaryExpressionLString: // LONGSTRINGLITERAL
			// nop
		case PrimaryExpressionInt: // INTCONST
			// nop
		default:
			c.errors.add(errorf("internal error: %v", x.Case))
		}
	case *PostfixExpression:
		switch x.Case {
		case PostfixExpressionIndex: // PostfixExpression '[' ExpressionList ']'
			var e ExpressionNode
			switch {
			case IsIntegerType(x.ExpressionList.Type()):
				e = x.PostfixExpression
			default:
				e = x.ExpressionList
			}
			switch t := e.Type(); t.Kind() {
			case Ptr:
				// ok
			default:
				c.takeAddr(e)
			}
		case PostfixExpressionSelect: // PostfixExpression '.' IDENTIFIER
			c.takeAddr(x.PostfixExpression)
		case PostfixExpressionPSelect: // PostfixExpression "->" IDENTIFIER
			// nop
		case PostfixExpressionComplit: // '(' TypeName ')' '{' InitializerList ',' '}'
			// nop
		default:
			c.errors.add(errorf("internal error: %v", x.Case))
		}
	case *CastExpression:
		switch x.Case {
		case CastExpressionUnary: // UnaryExpression
			c.takeAddr(x.UnaryExpression)
		case CastExpressionCast: // '(' TypeName ')' CastExpression
			c.takeAddr(x.CastExpression)
		default:
			c.errors.add(errorf("internal error: %v", x.Case))
		}
	case *UnaryExpression:
		switch x.Case {
		case UnaryExpressionDeref: // '*' CastExpression
			// nop
		case UnaryExpressionReal: // "__real__" UnaryExpression
			c.takeAddr(x.UnaryExpression)
		case UnaryExpressionImag: // "__imag__" UnaryExpression
			c.takeAddr(x.UnaryExpression)
		case UnaryExpressionAddrof: // '&' CastExpression
			c.takeAddr(x.CastExpression)
		default:
			c.errors.add(errorf("internal error: %v", x.Case))
		}
	default:
		c.errors.add(errorf("%v: %s: internal error: %T", n.Position(), NodeSource(n), x))
	}
}

// PostfixExpression:
//
//	PrimaryExpression                                 // Case PostfixExpressionPrimary
//
// |       PostfixExpression '[' ExpressionList ']'          // Case PostfixExpressionIndex
// |       PostfixExpression '(' ArgumentExpressionList ')'  // Case PostfixExpressionCall
// |       PostfixExpression '.' IDENTIFIER                  // Case PostfixExpressionSelect
// |       PostfixExpression "->" IDENTIFIER                 // Case PostfixExpressionPSelect
// |       PostfixExpression "++"                            // Case PostfixExpressionInc
// |       PostfixExpression "--"                            // Case PostfixExpressionDec
// |       '(' TypeName ')' '{' InitializerList ',' '}'      // Case PostfixExpressionComplit
func (n *PostfixExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check %v", n, n.Case))
			return
		}

		r = c.decay(r, mode)
		n.typ = r
		n.eval(c, mode)
	}()

out:
	switch n.Case {
	case PostfixExpressionPrimary: // PrimaryExpression
		n.typ = n.PrimaryExpression.check(c, mode)
		n.setPure(n.PrimaryExpression.Pure())
	case PostfixExpressionIndex: // PostfixExpression '[' ExpressionList ']'
		// One of the expressions shall have type ‘‘pointer to object type’’, the other
		// expression shall have integer type, and the result has type ‘‘type’’.
		mode = mode.add(decay)
		switch t1, t2 := n.PostfixExpression.check(c, mode), n.ExpressionList.check(c, mode); {
		case isPointerType(t1) && IsIntegerType(t2):
			n.typ = t1.(*PointerType).Elem().Decay()
		case isPointerType(t2) && IsIntegerType(t1):
			n.typ = t2.(*PointerType).Elem().Decay()
		case isVectorType(t1) && IsIntegerType(t2):
			n.typ = t1
		case isVectorType(t2) && IsIntegerType(t1):
			n.typ = t2
		default:
			c.errors.add(errorf("%v: one of the expressions shall be a pointer and the other shall have integer type: %s and %s", n.Token.Position(), t1, t2))
			n.typ = c.intT
			break out
		}
		n.setPure(n.PostfixExpression.Pure() && n.ExpressionList.Pure())
	case PostfixExpressionCall: // PostfixExpression '(' ArgumentExpressionList ')'
		switch isName(n.PostfixExpression) {
		case "__ccgo__types_compatible_p":
			n.PostfixExpression.check(c, mode.add(decay|implicitFuncDef))
			n.typ = c.intT
			args := n.ArgumentExpressionList.check(c, 0, &n.purer)
			if len(args) != 2 {
				c.errors.add(errorf("%v: expected two arguments: (%s)", n.Position(), NodeSource(n.ArgumentExpressionList)))
				break out
			}

			n.val = bool2int(args[0].Type().IsCompatible(args[1].Type()))
			break out
		case "__builtin_constant_p":
			n.PostfixExpression.check(c, mode.add(decay|implicitFuncDef))
			n.typ = c.intT
			args := n.ArgumentExpressionList.check(c, decay|noRead, &n.purer)
			if len(args) != 1 {
				c.errors.add(errorf("%v: expected one argument: (%s)", n.Position(), NodeSource(n.ArgumentExpressionList)))
				break out
			}

			n.val = bool2int(n.Pure() && args[0].Value() != Unknown)
			break out
		case "__builtin_choose_expr":
			n.PostfixExpression.check(c, mode.add(decay|implicitFuncDef))
			args := n.ArgumentExpressionList.check(c, decay|noRead, &n.purer)
			if len(args) != 3 {
				c.errors.add(errorf("%v: expected three argument: (%s)", n.Position(), NodeSource(n.ArgumentExpressionList)))
				break out
			}

			switch {
			case isNonzero(args[0].Value()):
				n.val = args[1].Value()
				n.typ = args[1].Type()
			case isZero(args[0].Value()):
				n.val = args[2].Value()
				n.typ = args[2].Type()
			default:
				trc("%T(%[1]v)", n.Value())
				c.errors.add(errorf("%v: expected an integer constant expression: (%s)", args[0].Position(), NodeSource(args[0])))
			}
			break out
		default:
			mode = mode.add(implicitFuncDef)
		}
		var dummy purer
		t := n.PostfixExpression.check(c, mode.add(decay))
		n.ArgumentExpressionList.check(c, decay, &dummy)
		if t == nil || mode.has(asmArgList) {
			n.typ = c.intT
			break
		}

		if t.Kind() != Ptr {
			c.errors.add(errorf("%v: expected pointer to function: %s", n.Position(), t))
			break
		}

		pt := t.(*PointerType)
		if pt.Elem().Kind() != Function {
			c.errors.add(errorf("%v: expected pointer to function: %s", n.Position(), pt))
			break
		}

		ft := pt.Elem().(*FunctionType)
		n.typ = ft.Result()
	case PostfixExpressionSelect: // PostfixExpression '.' IDENTIFIER
		nm := n.Token2.SrcStr()
		switch t := n.PostfixExpression.check(c, mode.add(decay)); t.Kind() {
		case Struct:
			st := t.(*StructType)
			f := st.FieldByName(nm)
			if f == nil {
				c.errors.add(errorf("%v: type %s has no member named %s", n.Token.Position(), st, nm))
				break
			}

			n.field = f
			n.typ = n.fieldType(c, f)
		case Union:
			st := t.(*UnionType)
			f := st.FieldByName(nm)
			if f == nil {
				c.errors.add(errorf("%v: type %s has no member named %s", n.Token.Position(), st, nm))
				break
			}

			n.field = f
			n.typ = n.fieldType(c, f)
		default:
			c.errors.add(errorf("%v: expected a struct or union: %s", n.Token.Position(), t))
		}
		n.setPure(n.PostfixExpression.Pure())
	case PostfixExpressionPSelect: // PostfixExpression "->" IDENTIFIER
		defer func() { n.setPure(n.PostfixExpression.Pure()) }()

		nm := n.Token2.SrcStr()
		switch t := n.PostfixExpression.check(c, mode.add(decay)); t.Kind() {
		case Ptr:
			switch et := t.(*PointerType).Elem(); et.Kind() {
			case Struct:
				st := et.(*StructType)
				f := st.FieldByName(nm)
				if f == nil {
					c.errors.add(errorf("%v: type %s has no member named %s", n.Token.Position(), st, nm))
					break
				}

				n.field = f
				n.typ = n.fieldType(c, f)
			case Union:
				st := et.(*UnionType)
				f := st.FieldByName(nm)
				if f == nil {
					c.errors.add(errorf("%v: type %s has no member named %s", n.Token.Position(), st, nm))
					break
				}

				n.field = f
				n.typ = n.fieldType(c, f)
			default:
				c.errors.add(errorf("%v: expected a pointer to struct or union: %s", n.Token.Position(), t))
			}
		default:
			c.errors.add(errorf("%v: expected a pointer: %s", n.Token.Position(), t))
		}
	case
		PostfixExpressionInc, // PostfixExpression "++"
		PostfixExpressionDec: // PostfixExpression "--"

		n.setPure(false)
		t := n.PostfixExpression.check(c, mode.add(decay))
		c.assignTo(n.PostfixExpression, false)
		switch {
		case
			// The operand of the postfix increment or decrement operator shall have
			// qualified or unqualified real or pointer type and shall be a modifiable
			// lvalue.
			realKinds[t.Kind()] || isPointerType(t):

			n.typ = t
			if !isModifiableLvalue(t) {
				c.errors.add(errorf("%v: operand shall be a modifiable lvalue: %s", n.PostfixExpression.Position(), t))
			}
		default:
			c.errors.add(errorf("%v: invalid operand: %s", n.PostfixExpression.Position(), t))
		}
	case PostfixExpressionComplit: // '(' TypeName ')' '{' InitializerList ',' '}'
		n.typ = n.TypeName.check(c)
		if n.InitializerList == nil {
			n.val = Zero
			break
		}

		if n := n.InitializerList.check(c, n.Type(), n.Type(), 0); n != nil {
			c.errors.add(errorf("%v: too many items in initializer", n.Position()))
		}
		n.setPure(n.InitializerList.pure(c))
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

func (n *InitializerList) pure(c *ctx) (r bool) {
	r = true
	for ; n != nil; n = n.InitializerList {
		r = r && n.Initializer.pure(c)
	}
	return r
}

func (n *Initializer) pure(c *ctx) (r bool) {
	switch n.Case {
	case InitializerExpr:
		return n.AssignmentExpression.Pure()
	case InitializerInitList:
		return n.InitializerList.pure(c)
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
		return false
	}
}

func (n *PostfixExpression) fieldType(c *ctx, f *Field) (r Type) {
	switch r = f.Type(); x := r.(type) {
	case *PredefinedType:
		if f.IsBitfield() {
			if f.ValueBits() < c.intT.Size()*8 {
				x = c.intT.(*PredefinedType)
			}
			x = x.clone().(*PredefinedType)
			x.bitField = f
			return x
		}
	}
	return r
}

func isName(n Node) string {
	for {
		switch x := n.(type) {
		case *ExpressionList:
			if x.ExpressionList != nil {
				return ""
			}

			n = x.AssignmentExpression
		case *PrimaryExpression:
			if x.Case == PrimaryExpressionIdent {
				return x.Token.SrcStr()
			}

			return ""
		default:
			return ""
		}
	}
}

func normalizedMacroReplacementList(m *Macro) (r []Token) {
	for _, v := range m.ReplacementList() {
		switch v.Ch {
		case ' ', '\n', '\t', '\r', '\f':
			// nop
		default:
			r = append(r, v)
		}
	}
	for len(r) != 0 && r[0].Ch == '(' && r[len(r)-1].Ch == ')' {
		r = r[1 : len(r)-1]
	}
	if len(r) == 1 {
		return r
	}

	return nil
}

// PrimaryExpression:
//
//	IDENTIFIER                 // Case PrimaryExpressionIdent
//
// |       INTCONST                   // Case PrimaryExpressionInt
// |       FLOATCONST                 // Case PrimaryExpressionFloat
// |       CHARCONST                  // Case PrimaryExpressionChar
// |       LONGCHARCONST              // Case PrimaryExpressionLChar
// |       STRINGLITERAL              // Case PrimaryExpressionString
// |       LONGSTRINGLITERAL          // Case PrimaryExpressionLString
// |       '(' ExpressionList ')'     // Case PrimaryExpressionExpr
// |       '(' CompoundStatement ')'  // Case PrimaryExpressionStmt
// |       GenericSelection           // Case PrimaryExpressionGeneric
func (n *PrimaryExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			if !mode.has(ignoreUndefined) {
				c.errors.add(errorf("TODO %T missed/failed type check %v", n, n.Case))
			}
			return
		}

		if n.m != nil && n.m.Value() == Unknown && n.Value() != Unknown {
			ok := true
			if l := normalizedMacroReplacementList(n.m); len(l) == 1 {
				switch l[0].Ch {
				case rune(PPNUMBER):
					switch n.val.(type) {
					case StringValue, UTF16StringValue, UTF32StringValue, nil:
						ok = false
					}
				}
			}
			switch {
			case ok:
				n.m.val = n.val
				n.m.typ = n.typ
			default:
				n.m = nil
			}
		}
		n.typ = c.decay(r, mode)
		r = n.Type()
		n.eval(c, mode)
	}()

out:
	switch n.Case {
	case PrimaryExpressionIdent: // IDENTIFIER
		n.setPure(true)
		var d *Declarator
		switch x := n.LexicalScope().ident(n.Token).(type) {
		case *Declarator:
			d = x
			n.typ = d.Type()
		case *Enumerator:
			n.resolvedTo = x
			n.val = x.val
			n.typ = x.Type()
			break out
		case *Parameter:
			switch {
			case x.Declarator != nil:
				n.resolvedTo = x.Declarator
				n.typ = x.Declarator.Type()
			default:
				n.resolvedTo = x
				n.typ = x.Type()
			}
			break out
		default:
			if !c.cfg.freeStanding && !c.cfg.noBuiltin {
				d = n.LexicalScope().builtin(n.Token)
			}
			if d == nil {
				d = c.predefinedDeclarator(n.Token, n.LexicalScope())
				d.isExtern = true
				d.isParam = false
				d.lexicalScope = (*lexicalScope)(n.LexicalScope())
				d.resolved = (*Scope)(d.lexicalScope)
				n.resolvedTo = d
				if mode.has(implicitFuncDef) {
					n.typ = c.implicitFunc
					d.typ = n.Type().(*PointerType).Elem()
					if mode&noRead == 0 {
						d.read++
					}
					if c.checkingSizeof {
						d.sizeof++
					}
					break out
				}

				if mode.has(ignoreUndefined) {
					n.typ = c.intT
					d.typ = n.Type()
					if mode&noRead == 0 {
						d.read++
					}
					if c.checkingSizeof {
						d.sizeof++
					}
					break out
				}

				c.errors.add(errorf("%v: undefined: %s", n.Position(), n.Token.Src()))
				break out
			}
		}

		n.resolvedTo = d
		if mode&noRead == 0 {
			d.read++
		}
		if c.checkingSizeof {
			d.sizeof++
		}
		n.typ = d.Type()
	case PrimaryExpressionInt: // INTCONST
		n.setPure(true)
		n.val, n.typ = n.intConst(c)
	case PrimaryExpressionFloat: // FLOATCONST
		n.setPure(true)
		n.val, n.typ = n.floatConst(c)
	case PrimaryExpressionChar: // CHARCONST
		n.setPure(true)
		n.val, n.typ = n.charConst(c)
	case PrimaryExpressionLChar: // LONGCHARCONST
		n.setPure(true)
		n.typ = c.wcharT(n)
		n.val, n.typ = n.charConst(c)
	case PrimaryExpressionString: // STRINGLITERAL
		n.setPure(true)
		n.val, n.typ = n.stringConst(c)
	case PrimaryExpressionLString: // LONGSTRINGLITERAL
		n.setPure(true)
		n.val, n.typ = n.stringConst(c)
	case PrimaryExpressionExpr: // '(' ExpressionList ')'
		n.typ = n.ExpressionList.check(c, mode)
		n.val = n.ExpressionList.Value()
		n.setPure(n.ExpressionList.Pure())
	case PrimaryExpressionStmt: // '(' CompoundStatement ')'
		n.typ = n.CompoundStatement.check(c)
		n.setPure(false) //TODO can do better
	case PrimaryExpressionGeneric: // GenericSelection
		n.typ = n.GenericSelection.check(c, mode, &n.purer, &n.val)
	case PrimaryExpressionObjCMessage: // [receiver message:args] (MinZ fork)
		// ObjC messages are type-checked in the ObjC lowerer, not here.
		// Default to int return type for now.
		n.typ = c.intT
	default:
		c.errors.add(errorf("internal error: %v", n.Case))
	}
	return n.Type()
}

func (n *GenericSelection) check(c *ctx, mode flags, p *purer, v *Value) (r Type) {
	n.assoc, n.typ = n.GenericAssociationList.check(c, mode, n.AssignmentExpression.check(c, mode.add(decay)), p, v)
	return n.Type()
}

func (n *GenericAssociationList) check(c *ctx, mode flags, ctrl Type, p *purer, v *Value) (assoc *GenericAssociation, r Type) {
	p.setPure(true)
	n0 := n
	var deflt *GenericAssociation
	// trc("==== %v: ctrl=%s", n.Position(), ctrl)
	for ; n != nil; n = n.GenericAssociationList {
		switch assoc = n.GenericAssociation; assoc.Case {
		case GenericAssociationType: // TypeName ':' AssignmentExpression
			t := assoc.TypeName.check(c)
			// trc("assoc t=%s", t)
			if ctrl.isGenericAssociationCompatible(t) {
				// trc("FOUND")
				r = assoc.AssignmentExpression.check(c, decay)
				*v = assoc.AssignmentExpression.Value()
				p.setPure(assoc.AssignmentExpression.Pure())
				return assoc, r
			}
		case GenericAssociationDefault: //  "default" ':' AssignmentExpression
			if deflt != nil {
				c.errors.add(errorf("%v: multiple defaults, previous at %v", assoc.Position(), deflt.Position()))
				break
			}

			assoc.AssignmentExpression.check(c, decay)
			*v = assoc.AssignmentExpression.Value()
			p.setPure(assoc.AssignmentExpression.Pure())
			deflt = assoc
		default:
			c.errors.add(errorf("TODO internal error: %v", assoc.Case))
		}
	}
	if deflt != nil {
		return deflt, deflt.AssignmentExpression.Type()
	}

	p.setPure(false)
	c.errors.add(errorf("%v: failed to find a matching association for %s", n0.Position(), ctrl))
	return nil, Invalid
}

func (n *PrimaryExpression) floatConst(c *ctx) (v Value, t Type) {
	s := strings.ToLower(n.Token.SrcStr())
	val, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return Float64Value(val), c.ast.kinds[Double]
	}

	// https://gcc.gnu.org/onlinedocs/gcc/Decimal-Float.html
	//
	// Use a suffix ‘df’ or ‘DF’ in a literal constant of type _Decimal32, ‘dd’ or
	// ‘DD’ for _Decimal64, and ‘dl’ or ‘DL’ for _Decimal128.

	// https://gcc.gnu.org/onlinedocs/gcc-9.1.0/gcc/Floating-Types.html
	//
	// Constants with these types use suffixes fn or Fn and fnx or Fnx.

	var bf *big.Float
	// Longer suffixes must come first.
	switch {
	case strings.HasSuffix(s, "f128x"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("f128x")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Float128x]
		}
	case strings.HasSuffix(s, "f32x"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("f32x")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Float32x]
		}
	case strings.HasSuffix(s, "f64x"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("f64x")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Float64x]
		}
	case strings.HasSuffix(s, "f128"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("f128")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Float128]
		}
	case strings.HasSuffix(s, "f16"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("f16")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Float16]
		}
	case strings.HasSuffix(s, "f32"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("f32")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Float32]
		}
	case strings.HasSuffix(s, "f64"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("f64")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Float64]
		}
	case
		strings.HasSuffix(s, "il"),
		strings.HasSuffix(s, "jl"),
		strings.HasSuffix(s, "li"),
		strings.HasSuffix(s, "lj"):

		bf, _, err = big.ParseFloat(s[:len(s)-len("il")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return &ComplexLongDoubleValue{Im: (*LongDoubleValue)(bf)}, c.ast.kinds[ComplexLong]
		}
	case
		strings.HasSuffix(s, "fi"),
		strings.HasSuffix(s, "fj"),
		strings.HasSuffix(s, "if"),
		strings.HasSuffix(s, "jf"):

		if val, err = strconv.ParseFloat(s[:len(s)-len("fi")], 64); err == nil {
			return Complex64Value(complex(0, val)), c.ast.kinds[ComplexFloat]
		}
	case strings.HasSuffix(s, "df"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("df")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Decimal32]
		}
	case strings.HasSuffix(s, "dd"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("dd")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Decimal64]
		}
	case strings.HasSuffix(s, "dl"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("dl")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[Decimal128]
		}
	case strings.HasSuffix(s, "f"):
		if val, err = strconv.ParseFloat(s[:len(s)-len("f")], 64); err == nil {
			return Float64Value(val), c.ast.kinds[Float]
		}
	case strings.HasSuffix(s, "l"):
		bf, _, err = big.ParseFloat(s[:len(s)-len("l")], 0, longDoublePrec, big.ToNearestEven)
		if err == nil {
			return (*LongDoubleValue)(bf), c.ast.kinds[LongDouble]
		}
	case strings.HasSuffix(s, "i"), strings.HasSuffix(s, "j"):
		if val, err = strconv.ParseFloat(s[:len(s)-len("i")], 64); err == nil {
			return Complex128Value(complex(0, val)), c.ast.kinds[ComplexDouble]
		}
	}
	c.errors.add(errorf("TODO %T %v %s FERR %v", n, n.Case, s, err))
	return Unknown, Invalid
}

func (n *PrimaryExpression) stringConst(c *ctx) (v Value, t Type) {
	s := stringConst(func(msg string, args ...interface{}) {
		c.errors.add(errorf(msg, args...))
	}, n.Token)
	switch n.Case {
	case PrimaryExpressionString:
		return StringValue(s), c.newArrayType(c.ast.kinds[Char], int64(len(s)), nil)
	case PrimaryExpressionLString:
		switch t = c.wcharT(n); t.Size() {
		case 1:
			v := StringValue(s)
			return v, c.newArrayType(t, int64(len(s)), nil)
		case 2:
			v := UTF16StringValue(utf16.Encode([]rune(s)))
			return v, c.newArrayType(t, int64(len(v)), nil)
		case 4:
			v := UTF32StringValue([]rune(s))
			return v, c.newArrayType(t, int64(len(v)), nil)
		}
	default:
		c.errors.add(errorf("TODO %v", n.Case))
	}
	return n.Value(), n.Type()
}

func (n *PrimaryExpression) charConst(c *ctx) (v Value, t Type) {
	n.typ = c.intT
	switch n.Case {
	case PrimaryExpressionLChar:
		n.typ = c.wcharT(n)
		fallthrough
	case PrimaryExpressionChar:
		r := charConst(func(msg string, args ...interface{}) {
			c.errors.add(errorf(msg, args...))
		}, n.Token)
		n.val = Int64Value(r)
	default:
		c.errors.add(errorf("TODO %v", n.Case))
	}
	return n.Value(), n.Type()
}

func (n *PrimaryExpression) intConst(c *ctx) (v Value, t Type) {
	s0 := n.Token.SrcStr()
	s := strings.TrimRight(s0, "uUlL")
	prefix := 0
	var base int
	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		prefix = 2
		base = 16
	case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
		prefix = 2
		base = 2
	case strings.HasPrefix(s, "0"):
		base = 8
	default:
		base = 10
	}
	s = s[prefix:]
	val, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		c.errors.add(errorf("%v: %v", n.Position(), err))
		return Unknown, c.intT
	}

	suffix := s0[prefix+len(s):]
	switch suffix = strings.ToLower(suffix); suffix {
	case "":
		if base == 10 {
			return n.intConst2(c, s0, val, Int, Long, LongLong)
		}

		return n.intConst2(c, s0, val, Int, UInt, Long, ULong, LongLong, ULongLong)
	case "u":
		return n.intConst2(c, s0, val, UInt, ULong, ULongLong)
	case "l":
		if base == 10 {
			return n.intConst2(c, s0, val, Long, LongLong)
		}

		return n.intConst2(c, s0, val, Long, ULong, LongLong, ULongLong)
	case "lu", "ul":
		return n.intConst2(c, s0, val, ULong, ULongLong)
	case "ll":
		if base == 10 {
			return n.intConst2(c, s0, val, LongLong)
		}

		return n.intConst2(c, s0, val, LongLong, ULongLong)
	case "llu", "ull":
		return n.intConst2(c, s0, val, ULongLong)
	default:
		c.errors.add(errorf("%v: invalid suffix", n.Position()))
		return Unknown, c.intT
	}
}

func (n *PrimaryExpression) intConst2(c *ctx, s string, val uint64, list ...Kind) (v Value, t Type) {
	abi := c.ast.ABI
	b := bits.Len64(val)
	for _, k := range list {
		sign := 0
		if abi.isSignedInteger(k) {
			sign = 1
		}
		if int(abi.Types[k].Size)*8 >= b+sign {
			switch {
			case sign == 0:
				return UInt64Value(val), c.ast.kinds[k]
			default:
				return Int64Value(val), c.ast.kinds[k]
			}

		}
	}

	c.errors.add(errorf("%v: invalid integer constant", n.Position()))
	return Unknown, c.intT
}

// ExpressionList:
//
//	AssignmentExpression
//
// |       ExpressionList ',' AssignmentExpression
func (n *ExpressionList) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	n0 := n
	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
			return
		}

		n.eval(c, mode)
	}()

	n.setPure(true)
	for ; n != nil; n = n.ExpressionList {
		n0.typ = n.AssignmentExpression.check(c, mode)
		n.setPure(n.Pure() && n.AssignmentExpression.Pure())
	}
	return n0.Type()
}

// ConstantExpression:
//
//	ConditionalExpression
func (n *ConstantExpression) check(c *ctx, mode flags) (r Type) {
	if n == nil {
		return Invalid
	}

	defer func() {
		if r == nil || r == Invalid {
			c.errors.add(errorf("TODO %T missed/failed type check", n))
		}
	}()

	n.typ = n.ConditionalExpression.check(c, mode)
	n.setPure(n.ConditionalExpression.Pure())
	if n.val = n.ConditionalExpression.eval(c, mode); n.Value() == Unknown {
		c.errors.add(errorf("%v: cannot evaluate constant expression: %s", n.Position(), NodeSource(n)))
	}
	return n.Type()
}
