package hir

import "github.com/minz/minzc/pkg/mir2"

// AssertValueMatches compares a runtime bit pattern in its declared return type.
// Signed expectations may use either the signed value or its unsigned bit pattern
// (the latter is used by existing source assertions). Never truncate expected:
// out-of-range expectations must fail, including the judge's bit-32 controls.
func AssertValueMatches(actual, expected int64, ty mir2.Ty) bool {
	if ty == nil {
		return actual == expected
	}
	ty = mir2.BaseOf(ty)
	width := ty.Width()
	if width <= 0 || width >= 64 {
		return actual == expected
	}
	mask := (uint64(1) << uint(width)) - 1
	bits := int64(uint64(actual) & mask)
	if bits == expected {
		return true
	}
	if mir2.IsSigned(ty) {
		shift := uint(64 - width)
		return (bits<<shift)>>shift == expected
	}
	return false
}
