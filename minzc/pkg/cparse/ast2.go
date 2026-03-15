// Copyright 2022 The CC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cparse

// AddressTaken reports whether address of n is taken. The result is valid
// after Translate.
func (n *Declarator) AddressTaken() bool { return n.addrTaken }

// HasInitializer reports whether n has an initializer.
func (n *Declarator) HasInitializer() bool { return n.hasInitializer }

// ReadCount reports the number of times n is read. The result is valid after
// Translate.
func (n *Declarator) ReadCount() int { return n.read }

// SizeofCount reports the number of times n appears in sizeof(expr). The
// result is valid after Translate.
func (n *Declarator) SizeofCount() int { return n.sizeof }

// WriteCount reports the number of times n is written. The result is valid
// after Translate.
func (n *Declarator) WriteCount() int { return n.write }

// Name returns the name of n.
func (n *Declarator) Name() string {
	if n == nil {
		return ""
	}

	if dn := n.DirectDeclarator.name(); dn != nil {
		return dn.Token.SrcStr()
	}

	return ""
}

// NameTok returns the name token of n.
func (n *Declarator) NameTok() (r Token) {
	if n == nil {
		return r
	}

	if dn := n.DirectDeclarator.name(); dn != nil {
		return dn.Token
	}

	return r
}

// IsFuncDef reports whether n declares the name of a function definition.
func (n *Declarator) IsFuncDef() bool {
	if n == nil {
		return false
	}

	return n.isFuncDef
}

func (n *Declarator) isFn() bool {
	if n == nil {
		return false
	}

	return n.DirectDeclarator.isFn()
}

// IsSynthetic reports whether n is synthesized.
func (n *Declarator) IsSynthetic() bool {
	if n == nil {
		return false
	}

	return n.isSynthetic
}

// Linkage describes linkage of identifiers ([0]6.2.2).
type Linkage int

// Values of type Linkage
const (
	External Linkage = iota
	Internal
	None
)

// Linkage reports n's linkage.
func (n *Declarator) Linkage() Linkage {
	if n.IsTypename() || n.IsParam() {
		return None
	}

	if n.IsStatic() && n.LexicalScope().Parent == nil {
		return Internal
	}

	if n.IsExtern() || n.LexicalScope().Parent == nil {
		return External
	}

	if n.LexicalScope().Parent != nil {
		if t := n.Type(); t != nil && t.Kind() == Function && !n.IsFuncDef() {
			if n.IsStatic() {
				return Internal
			}

			return External
		}
	}

	return None
}

// StorageDuration describes storage duration of objects ([0]6.2.4).
type StorageDuration int

// Values of type StorageDuration
const (
	Static StorageDuration = iota
	Automatic
	Allocated
)

func (n *Declarator) StorageDuration() StorageDuration {
	switch l := n.Linkage(); {
	case l == External || l == Internal || n.IsStatic():
		return Static
	case l == None && !n.IsStatic():
		return Automatic
	}

	panic(todo(""))
}

// IsExtern reports whether the storage class specifier 'extern' was present in
// the declaration of n.
func (n *Declarator) IsExtern() bool { return n.isExtern }

// IsConst reports whether the type qualifier 'const' was present in
// the declaration of n.
func (n *Declarator) IsConst() bool { return n.isConst }

// IsInline reports whether the function specifier 'inline' was present in the
// declaration of n.
func (n *Declarator) IsInline() bool { return n.isInline }

// IsVolatile reports whether the type qualifier 'volatile' was present in
// the declaration of n.
func (n *Declarator) IsVolatile() bool { return n.isVolatile }

// IsRegister reports whether the storage class specifier 'register' was
// present in the declaration of n.
func (n *Declarator) IsRegister() bool { return n.isRegister }

// IsStatic reports whether the storage class specifier 'static' was present in
// the declaration of n.
func (n *Declarator) IsStatic() bool { return n.isStatic }

// IsAtomic reports whether the type specifier '_Atomic' was present in the
// declaration of n.
func (n *Declarator) IsAtomic() bool { return n.isAtomic }

// IsThreadLocal reports whether the storage class specifier '_Thread_local'
// was present in the declaration of n.
func (n *Declarator) IsThreadLocal() bool { return n.isThreadLocal }

// IsTypename reports whether n is a typedef.
func (n *Declarator) IsTypename() bool { return n.isTypename }

// Alignas reports whether the _Alignas specifier was present in the
// declaration specifiers of n, if non-zero.
func (n *Declarator) Alignas() int { return n.alignas }

// IsParam reports whether n is a function paramater.
func (n *Declarator) IsParam() bool { return n.isParam }

func (n *DirectDeclarator) name() *DirectDeclarator {
	if n == nil {
		return nil
	}

	switch n.Case {
	case DirectDeclaratorIdent:
		return n
	case DirectDeclaratorDecl:
		return n.Declarator.DirectDeclarator.name()
	default:
		return n.DirectDeclarator.name()
	}
}

func (n *DirectDeclarator) isFn() bool {
	if n == nil {
		return false
	}

	switch n.Case {
	case DirectDeclaratorFuncParam, DirectDeclaratorFuncIdent:
		return true
	}

	return false
}

// ResolvedTo returns the node n resolved to when n.Case is
// PrimaryExpressionIdent.
func (n *PrimaryExpression) ResolvedTo() Node { return n.resolvedTo }

// Macro returns the single, optionally parenthesized token, of an object-like,
// constant macro that produced this node, if any.
func (n *PrimaryExpression) Macro() *Macro { return n.m }

// Associated returns the selected association of n, if any.
func (n *GenericSelection) Associated() *GenericAssociation { return n.assoc }

// Parent returns Initalizer m that has n on its InitializerList, if any.
func (n *Initializer) Parent() *Initializer { return n.parent }

// Offset returns the offset of n within it's containing type.
func (n *Initializer) Offset() int64 { return n.off }

// Len returns the number of array elements initialized. It's normally one, but
// can be more using the [lo ... hi] designator.
func (n *Initializer) Len() int64 {
	if n.nelems == 0 {
		return 1
	}

	return n.nelems
}

// Field returns the field associated with n, if any.
func (n *Initializer) Field() *Field { return n.field }

// Order returns a number that can be used to order initializers in their order
// of appearance in the preprocessed source.
func (n *Initializer) Order() int64 { return n.order }

// UnionField reports the union field initilized by n.
func (n *InitializerList) UnionField() *Field { return n.unionField }

// Field reports the resolved field for cases PostfixExpressionSelect and
// PostfixExpressionPSelect.
func (n *PostfixExpression) Field() *Field { return n.field }

// Cases returns the combined number of "case" and "default" labels in a switch
// statement. Valid for Case == SelectionStatementSwitch.
func (n *SelectionStatement) Cases() int { return n.switchCases }

// LabeledStatements returns labeled statements appearing within n, case
// SelectionStatementSwitch.
func (n *SelectionStatement) LabeledStatements() []*LabeledStatement { return n.labeled }

// CaseOrdinal returns the zero based ordinal number of a labeled statement
// within a switch statement.  Valid only for Case LabeledStatementCaseLabel
// and LabeledStatementDefault.
func (n *LabeledStatement) CaseOrdinal() int { return n.caseOrdinal }

// Switch returns the switch associated with n, case LabeledStatementCaseLabel,
// LabeledStatementDefault, LabeledStatementRange.
func (n *LabeledStatement) Switch() *SelectionStatement { return n.switchCtx }

// Gotos returns a slice of all goto statements in n if n is a function block.
func (n *CompoundStatement) Gotos() []*JumpStatement { return n.gotos }

// Label returns the labeled statement or a label declaration n, case
// JumpStatementGoto, refers to.
func (n *JumpStatement) Label() Node { return n.label }

// UsesVectors reports whether n uses any vector type.
func (n *FunctionDefinition) UsesVectors() bool { return n.usesVectors }

func (n *TypeQualifiers) isVolatile() bool {
	for ; n != nil; n = n.TypeQualifiers {
		switch n.TypeQualifier.Case {
		case TypeQualifierVolatile:
			return true
		}
	}
	return false
}

func (n *TypeQualifiers) isConst() bool {
	for ; n != nil; n = n.TypeQualifiers {
		switch n.TypeQualifier.Case {
		case TypeQualifierConst:
			return true
		}
	}
	return false
}

func (e *Enumerator) compatibleRedeclaration(f *Enumerator) bool {
	if e.Token.SrcStr() != f.Token.SrcStr() || e.Type() == nil || !e.Type().IsCompatible(f.Type()) {
		return false
	}

	return e.Value() != nil && e.Value() == f.Value()
}
