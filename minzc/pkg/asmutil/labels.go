// Package asmutil contains small assembly-format helpers shared by backends.
package asmutil

import "strings"

// z80Reserved are Z80 register names that cannot be used as asm labels.
var z80Reserved = map[string]bool{
	"A": true, "B": true, "C": true, "D": true, "E": true, "F": true,
	"H": true, "L": true, "I": true, "R": true,
	"AF": true, "BC": true, "DE": true, "HL": true, "SP": true,
	"IX": true, "IY": true, "IXH": true, "IXL": true, "IYH": true, "IYL": true,
}

// SanitizeZ80Label preserves the historical backend label spelling.
// Replaces @, ., - and $ with _, and prefixes register names with v_.
// Other characters are unchanged; this is not a complete symbol validator.
func SanitizeZ80Label(s string) string {
	s = strings.ReplaceAll(s, "@", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "$", "_")
	if z80Reserved[strings.ToUpper(s)] {
		return "v_" + s
	}
	return s
}
