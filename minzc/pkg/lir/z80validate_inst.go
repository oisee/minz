// z80validate_inst.go — Per-instruction Z80 validation for WFC reject loop.
//
// ValidateInst checks a single expanded template line against the Z80
// assembler. If it fails, WFC should reject the current assignment and
// try a different variant or register allocation.
package lir

import (
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
)

// ValidateInst checks whether a single Z80 instruction line (or multi-line
// template separated by \n) assembles without errors.
// Returns true if all lines are valid Z80 instructions.
func ValidateInst(line string) bool {
	// Skip empty lines, comments, labels.
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, ";") {
		return true
	}

	// Wrap in minimal context for the assembler.
	src := "    ORG 0\n"
	for _, part := range strings.Split(line, "\n") {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasPrefix(part, ";") {
			continue
		}
		src += "    " + part + "\n"
	}

	a := z80asm.NewAssembler()
	res, err := a.AssembleString(src)
	if err != nil {
		return false
	}
	return len(res.Errors) == 0
}

// ValidateExpandedTemplate checks whether an expanded instruction template
// produces valid Z80 assembly. Used by WFC collapse to reject invalid
// register assignments before committing.
func ValidateExpandedTemplate(inst Inst, desc *MachineDesc) bool {
	if inst.Pat == nil {
		return true // synthetic cell (param def, etc.)
	}
	line := ExpandTemplateNamed(inst, desc)
	return ValidateInst(line)
}
