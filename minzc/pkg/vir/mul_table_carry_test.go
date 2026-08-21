package vir

import "testing"

// TestDependsOnEntryCarry covers the guard that decides whether a precomputed
// multiply sequence needs an OR A in front. The motivating case is k=23, whose
// table entry starts "LD B,A / RLA": RLA rotates through carry, so without the
// guard the result depends on the caller's flags (161 with CY=0, 169 with CY=1).
func TestDependsOnEntryCarry(t *testing.T) {
	cases := []struct {
		name string
		ops  []string
		want bool
	}{
		// The bug that motivated this: LD is transparent, RLA consumes.
		{"k23 rla after ld", []string{"LD B,A", "RLA", "ADD A,B", "SBC A,B"}, true},
		{"bare rla", []string{"RLA"}, true},
		{"rra", []string{"RRA"}, true},
		{"adc first", []string{"ADC A, B"}, true},
		{"sbc first", []string{"SBC A, B"}, true},

		// A definer before any consumer makes the entry carry unobservable.
		{"add defines before rla", []string{"ADD A, A", "RLA"}, false},
		{"rlca defines before rra", []string{"RLCA", "RRA"}, false},
		{"neg defines", []string{"NEG", "ADC A, B"}, false},
		{"srl defines", []string{"SRL A", "RL B"}, false},
		{"sub defines", []string{"SUB B", "SBC A, B"}, false},

		// Transparent instructions are skipped, not treated as definers.
		{"ld chain then adc", []string{"LD B,A", "LD C,B", "ADC A,B"}, true},
		{"ld only", []string{"LD B, A"}, false},
		{"empty", nil, false},

		// Unknown mnemonics take the conservative branch.
		{"unknown mnemonic", []string{"SWAP_HL"}, true},

		// Case and spacing should not matter.
		{"lowercase", []string{"ld b,a", "rla"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dependsOnEntryCarry(tc.ops); got != tc.want {
				t.Errorf("dependsOnEntryCarry(%v) = %v, want %v", tc.ops, got, tc.want)
			}
		})
	}
}
