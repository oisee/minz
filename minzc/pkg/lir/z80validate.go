// z80validate.go — Z80 assembly validator for LIR emit safety.
//
// Uses the project's own MZA assembler (z80asm package) to validate emitted
// instructions. MZA knows EVERY valid Z80 opcode, DD/FD prefix conflict,
// register constraint, and immediate range. No need to reimplement.
//
// On validation failure, the pipeline can backtrack and try alternative
// code generation. Each failure is logged for future constraint improvement.
package lir

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
)

// Z80ValidationError describes one invalid instruction in emitted assembly.
type Z80ValidationError struct {
	Line    int
	Text    string
	Reason  string
}

func (e Z80ValidationError) Error() string {
	return fmt.Sprintf("line %d: %s — %s", e.Line, e.Text, e.Reason)
}

// ValidateZ80Asm checks emitted Z80 assembly by assembling it through MZA.
// Returns nil if all instructions are valid.
// Any MZA error = invalid Z80 instruction = LIR codegen bug to fix.
func ValidateZ80Asm(asm string) []Z80ValidationError {
	// Wrap in ORG so MZA has context for address resolution.
	// Replace label references in JP/CALL with dummy forward labels
	// to avoid undefined symbol errors (we only care about instruction validity).
	wrapped := prepareForValidation(asm)

	a := z80asm.NewAssembler()
	a.AllowUndocumented = true // IXH, IXL, SLL etc.
	result, err := a.AssembleString(wrapped)
	if err != nil {
		// Parse-level error — the whole thing is broken
		return []Z80ValidationError{{
			Line:   0,
			Text:   "(parse error)",
			Reason: err.Error(),
		}}
	}

	if len(result.Errors) == 0 {
		return nil
	}

	var errs []Z80ValidationError
	for _, ae := range result.Errors {
		errs = append(errs, Z80ValidationError{
			Line:   ae.Line,
			Text:   ae.Context,
			Reason: ae.Message,
		})
	}
	return errs
}

// LogValidationErrors logs Z80 validation errors for diagnosis.
// Each logged error is a constraint gap to fix in z80.go rules.
func LogValidationErrors(funcName, asm string, errs []Z80ValidationError) {
	fmt.Printf("[Z80-VALIDATE] %s: %d invalid instruction(s)\n", funcName, len(errs))
	for _, e := range errs {
		fmt.Printf("  line %d: %s — %s\n", e.Line, e.Text, e.Reason)
	}
}

// prepareForValidation wraps LIR assembly output for MZA validation.
// Adds ORG, defines dummy labels for branch targets so we only get
// errors from actual invalid instructions, not undefined symbols.
func prepareForValidation(asm string) string {
	var sb strings.Builder
	sb.WriteString("    ORG $8000\n")

	// Collect all label references (JP/CALL/JR/DJNZ targets, label definitions)
	// and all label definitions so we can define missing ones.
	defined := make(map[string]bool)
	referenced := make(map[string]bool)

	lines := strings.Split(asm, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}
		// Label definition
		if strings.HasSuffix(trimmed, ":") && !strings.ContainsAny(trimmed[:len(trimmed)-1], " \t,") {
			label := strings.TrimSuffix(trimmed, ":")
			defined[label] = true
			continue
		}
		// Check for label after instructions like "funcname:"
		if idx := strings.Index(trimmed, ":"); idx > 0 {
			candidate := trimmed[:idx]
			if !strings.ContainsAny(candidate, " \t,()") {
				defined[candidate] = true
			}
		}
		// Collect branch target references
		collectBranchTargets(trimmed, referenced)
	}

	// Define any referenced-but-not-defined labels as dummy targets
	for label := range referenced {
		if !defined[label] {
			sb.WriteString(fmt.Sprintf("%s:\n", label))
		}
	}

	sb.WriteString(asm)
	sb.WriteString("\n")

	// Add trailing labels for forward references
	for label := range referenced {
		if !defined[label] {
			sb.WriteString(fmt.Sprintf("%s_end:\n", label))
		}
	}

	return sb.String()
}

// collectBranchTargets extracts label names from JP/CALL/JR/DJNZ instructions.
func collectBranchTargets(line string, refs map[string]bool) {
	upper := strings.ToUpper(strings.TrimSpace(line))
	// Strip inline comments
	if idx := strings.Index(upper, " ;"); idx >= 0 {
		upper = strings.TrimSpace(upper[:idx])
	}

	prefixes := []string{"JP ", "CALL ", "JR ", "DJNZ ", "JP NZ,", "JP Z,",
		"JP C,", "JP NC,", "JP PE,", "JP PO,", "JP P,", "JP M,",
		"CALL NZ,", "CALL Z,", "CALL C,", "CALL NC,",
		"JR NZ,", "JR Z,", "JR C,", "JR NC,", "JRS "}

	for _, prefix := range prefixes {
		if strings.HasPrefix(upper, prefix) {
			rest := strings.TrimSpace(line[len(prefix):])
			// Handle "JP cc, target" format
			if strings.Contains(rest, ",") {
				parts := strings.SplitN(rest, ",", 2)
				rest = strings.TrimSpace(parts[len(parts)-1])
			}
			// Only track label references, not immediates or (HL)
			rest = strings.TrimSpace(rest)
			if rest != "" && !strings.HasPrefix(rest, "(") &&
				!strings.HasPrefix(rest, "$") && !strings.HasPrefix(rest, "0") &&
				(rest[0] < '0' || rest[0] > '9') {
				refs[rest] = true
			}
			return
		}
	}
}
