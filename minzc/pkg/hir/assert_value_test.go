package hir

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestAssertValueRejectsInvalidExtension(t *testing.T) {
	for _, c := range []struct {
		name      string
		ty        mir2.Ty
		raw, want int64
		pass      bool
	}{
		{"bool_two_false", mir2.TyBool, 2, 0, false},
		{"bool_two_true", mir2.TyBool, 2, 1, false},
		{"bool_three_true", mir2.TyBool, 3, 1, false},
		{"bool_three_raw", mir2.TyBool, 3, 3, false},
		{"bool_false", mir2.TyBool, 0, 0, true},
		{"bool_true", mir2.TyBool, 1, 1, true},
		{"byte_overflow", mir2.TyU8, 300, 44, false},
		{"byte_overflow_raw", mir2.TyU8, 300, 300, false},
		{"unsigned_negative", mir2.TyU8, -1, 255, false},
		{"signed_bits", mir2.TyI16, 65041, -495, true},
		{"signed_extension", mir2.TyI16, -495, -495, true},
		// Preserve unsigned bit-pattern expectations used by existing assertions.
		{"signed_legacy_expectation", mir2.TyI16, 65041, 65041, true},
		{"signed_extended_legacy_expectation", mir2.TyI16, -495, 65041, true},
		{"signed_bad_extension", mir2.TyI16, 0x1fe11, -495, false},
		{"signed_positive_bad_extension", mir2.TyI16, -65535, 1, false},
		{"signed_control", mir2.TyI16, 65041, -495 + (1 << 32), false},
		{"unsigned_word", mir2.TyU16, 65535, 65535, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := AssertValueMatches(c.raw, c.want, c.ty); got != c.pass {
				t.Fatalf("raw %d, want %d: matched %v, expected %v", c.raw, c.want, got, c.pass)
			}
		})
	}
}
