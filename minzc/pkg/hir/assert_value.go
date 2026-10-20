package hir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// AssertValueMatches compares a runtime bit pattern in its declared return type.
// Signed expectations may use either the signed value or its unsigned bit pattern
// (the latter is used by existing source assertions). Never truncate expected:
// out-of-range expectations must fail, including the judge's bit-32 controls.
// Actual values must be either the zero-extended bit pattern or, for signed
// types, its full sign extension. In particular bool accepts only raw 0 or 1.
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
	if mir2.IsSigned(ty) {
		shift := uint(64 - width)
		signed := (bits << shift) >> shift
		if actual != bits && actual != signed {
			return false
		}
		return expected == bits || expected == signed
	}
	return actual == bits && bits == expected
}

// FormatAssertValue shows the declared value and preserves the raw result for
// diagnosing invalid extensions and signed bit-pattern returns.
func FormatAssertValue(actual int64, ty mir2.Ty) string {
	value := actual
	if ty != nil {
		ty = mir2.BaseOf(ty)
		if width := ty.Width(); width > 0 && width < 64 {
			bits := int64(uint64(actual) & ((uint64(1) << uint(width)) - 1))
			value = bits
			if mir2.IsSigned(ty) {
				shift := uint(64 - width)
				value = (bits << shift) >> shift
			}
		}
	}
	if value != actual {
		return fmt.Sprintf("%d (raw %d)", value, actual)
	}
	return fmt.Sprintf("%d", value)
}
