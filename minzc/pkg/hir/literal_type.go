package hir

import "github.com/minz/minzc/pkg/mir2"

// IntLiteralFits tests the mathematical value, before any machine truncation.
func IntLiteralFits(v int64, ty mir2.Ty) bool {
	if !mir2.IsInt(ty) {
		return false
	}
	bits := ty.Width()
	if mir2.IsSigned(ty) {
		return v >= -(int64(1)<<(bits-1)) && v <= (int64(1)<<(bits-1))-1
	}
	return v >= 0 && v <= (int64(1)<<bits)-1
}

// literalValueAs folds narrowing of an exact source literal. Keep existing
// encodings of fitting values and negative widening constants unchanged.
func literalValueAs(lit *IntLitExpr, ty mir2.Ty) int64 {
	if lit.Untyped && mir2.IsInt(ty) && !IntLiteralFits(lit.Val, ty) && lit.Ty.Width() > ty.Width() {
		return lit.Val & ((int64(1) << ty.Width()) - 1)
	}
	return lit.Val
}

// DefaultLiteralType is only the context-free storage choice. A source
// literal remains untyped until an operation or destination supplies context.
func DefaultLiteralType(v int64) mir2.Ty {
	candidates := []mir2.Ty{mir2.TyU8, mir2.TyU16, mir2.TyU24, mir2.TyU32}
	if v < 0 {
		candidates = []mir2.Ty{mir2.TyI8, mir2.TyI16, mir2.TyI24, mir2.TyI32}
	}
	for _, ty := range candidates {
		if IntLiteralFits(v, ty) {
			return ty
		}
	}
	return nil
}

// LiteralPeerType selects a common type only when a Nanz untyped literal is
// involved. Typed HIR (including C conversions and explicit casts) is untouched.
// A nil result means the caller should use its ordinary typed-operand rules.
func LiteralPeerType(a, b Expr) mir2.Ty {
	al, aLit := a.(*IntLitExpr)
	bl, bLit := b.(*IntLitExpr)
	aLit = aLit && al.Untyped
	bLit = bLit && bl.Untyped
	if !aLit && !bLit {
		return nil
	}
	if !mir2.IsInt(a.ExprTy()) || !mir2.IsInt(b.ExprTy()) {
		return nil
	}
	// RangeType implements the same mathematical domain union, including
	// adoption of the peer type for a representable literal.
	ty := integerDomainType(a.ExprTy(), b.ExprTy(), al, bl, aLit, bLit)
	// The range API retains its historical fallback for unrepresentable typed
	// bounds. Untyped expressions must never silently lose a literal value.
	if aLit && !IntLiteralFits(al.Val, ty) || bLit && !IntLiteralFits(bl.Val, ty) {
		return nil
	}
	return ty
}

// integerDomainType unions mathematical operand domains. Literal values
// contribute one point; typed operands contribute their complete machine range.
func integerDomainType(a, b mir2.Ty, al, bl *IntLitExpr, aLit, bLit bool) mir2.Ty {
	signed := []mir2.Ty{mir2.TyI8, mir2.TyI16, mir2.TyI24, mir2.TyI32}
	unsigned := []mir2.Ty{mir2.TyU8, mir2.TyU16, mir2.TyU24, mir2.TyU32}
	fits := IntLiteralFits
	if aLit && bLit {
		candidates := unsigned
		if al.Val < 0 || bl.Val < 0 {
			candidates = signed
		}
		for _, ty := range candidates {
			if fits(al.Val, ty) && fits(bl.Val, ty) {
				return ty
			}
		}
		// The parser diagnoses values outside the widest supported domain.
		return candidates[len(candidates)-1]
	}
	if aLit != bLit {
		lit, other := al, b
		if bLit {
			lit, other = bl, a
		}
		if fits(lit.Val, other) {
			return other
		}
		candidates := unsigned
		if mir2.IsSigned(other) || lit.Val < 0 {
			candidates = signed
		}
		for _, ty := range candidates {
			if ty.Width() >= other.Width() &&
				(!mir2.IsSigned(ty) || mir2.IsSigned(other) || ty.Width() > other.Width()) && fits(lit.Val, ty) {
				return ty
			}
		}
		// Keep the contextual signedness so the parser diagnoses overflow.
		return other
	}
	if mir2.IsSigned(a) != mir2.IsSigned(b) {
		st, ut := a, b
		if !mir2.IsSigned(st) {
			st, ut = ut, st
		}
		for _, ty := range signed {
			if ty.Width() >= st.Width() && ty.Width() > ut.Width() {
				return ty
			}
		}
		return b
	}
	if b.Width() > a.Width() {
		return b
	}
	return a
}

// Source templates used by exact rewrite proofs must include source literal
// metadata. Ordinary U8/U16 constructors remain explicitly typed for all
// frontends and for newly synthesized rewrite instructions.
func sourceU8(n int64) *IntLitExpr  { return &IntLitExpr{Val: n, Ty: mir2.TyU8, Untyped: true} }
func sourceU16(n int64) *IntLitExpr { return &IntLitExpr{Val: n, Ty: mir2.TyU16, Untyped: true} }
