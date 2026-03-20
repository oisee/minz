// z80validate.go — Z80 assembly validator (thin wrapper over pkg/z80validate).
package lir

import (
	"github.com/minz/minzc/pkg/z80validate"
)

// Z80ValidationError describes one invalid instruction in emitted assembly.
type Z80ValidationError = z80validate.Error

// ValidateZ80Asm checks emitted Z80 assembly by assembling it through MZA.
func ValidateZ80Asm(asm string) []Z80ValidationError {
	return z80validate.Validate(asm)
}

// LogValidationErrors logs Z80 validation errors for diagnosis.
func LogValidationErrors(funcName, asm string, errs []Z80ValidationError) {
	z80validate.LogErrors(funcName, errs)
}
