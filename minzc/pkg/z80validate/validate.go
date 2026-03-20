// Package z80validate validates Z80 assembly output using the MZA assembler.
//
// Shared by both MIR2 and LIR backends. Any MZA error = invalid Z80
// instruction = codegen bug to fix.
package z80validate

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
)

// Error describes one invalid instruction in emitted assembly.
type Error struct {
	Line   int
	Text   string
	Reason string
}

func (e Error) String() string {
	return fmt.Sprintf("line %d: %s — %s", e.Line, e.Text, e.Reason)
}

// Validate checks emitted Z80 assembly by assembling it through MZA.
// Returns nil if all instructions are valid.
func Validate(asm string) []Error {
	wrapped := PrepareForValidation(asm)

	// Build line lookup from the ORIGINAL asm (not wrapped) so we can
	// show the actual instruction text in error messages.
	origLines := strings.Split(asm, "\n")

	// Count how many lines PrepareForValidation prepended (ORG + dummy labels).
	// The wrapped output is: ORG line + dummy labels + original asm + trailing labels.
	// We need the offset from wrapped line numbers to original line numbers.
	// Find where the original asm starts in the wrapped output by searching for
	// the first original line.
	wrappedLines := strings.Split(wrapped, "\n")
	prefixLines := 0
	if len(origLines) > 0 {
		for i, wl := range wrappedLines {
			if wl == origLines[0] {
				prefixLines = i
				break
			}
		}
	}

	a := z80asm.NewAssembler()
	a.AllowUndocumented = true // IXH, IXL, SLL etc.
	result, err := a.AssembleString(wrapped)
	if err != nil {
		return []Error{{
			Line:   0,
			Text:   "(parse error)",
			Reason: err.Error(),
		}}
	}

	if len(result.Errors) == 0 {
		return nil
	}

	var errs []Error
	for _, ae := range result.Errors {
		// Map wrapped line number back to original asm line.
		origLine := ae.Line - prefixLines
		text := ""
		if origLine >= 1 && origLine <= len(origLines) {
			text = strings.TrimSpace(origLines[origLine-1])
		}
		if text == "" {
			text = ae.Context
		}
		errs = append(errs, Error{
			Line:   origLine,
			Text:   text,
			Reason: ae.Message,
		})
	}
	return errs
}

// LogErrors logs Z80 validation errors to stdout with ±2 lines of context.
func LogErrors(funcName string, errs []Error) {
	fmt.Printf("[Z80-VALIDATE] %s: %d invalid instruction(s)\n", funcName, len(errs))
	// Deduplicate (MZA sometimes reports same error twice).
	seen := make(map[string]bool)
	for _, e := range errs {
		key := fmt.Sprintf("%d:%s", e.Line, e.Text)
		if seen[key] {
			continue
		}
		seen[key] = true
		fmt.Printf("  line %d: >>> %s <<<\n", e.Line, e.Text)
	}
}

// LogErrorsWithContext logs errors with surrounding assembly lines for context.
func LogErrorsWithContext(funcName, asm string, errs []Error) {
	lines := strings.Split(asm, "\n")
	fmt.Printf("[Z80-VALIDATE] %s: %d invalid instruction(s)\n", funcName, len(errs))
	seen := make(map[string]bool)
	for _, e := range errs {
		key := fmt.Sprintf("%d:%s", e.Line, e.Text)
		if seen[key] {
			continue
		}
		seen[key] = true
		// Show ±2 lines of context
		start := e.Line - 3 // 0-based: line-1, then -2 more
		if start < 0 {
			start = 0
		}
		end := e.Line + 2 // +2 after
		if end > len(lines) {
			end = len(lines)
		}
		for i := start; i < end; i++ {
			marker := "  "
			if i == e.Line-1 {
				marker = ">>"
			}
			fmt.Printf("  %s %3d: %s\n", marker, i+1, lines[i])
		}
		fmt.Println()
	}
}

// PrepareForValidation wraps assembly output for MZA validation.
// Adds ORG, defines dummy labels for branch targets so we only get
// errors from actual invalid instructions, not undefined symbols.
func PrepareForValidation(asm string) string {
	var sb strings.Builder
	sb.WriteString("    ORG $8000\n")

	defined := make(map[string]bool)
	referenced := make(map[string]bool)

	lines := strings.Split(asm, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if strings.HasSuffix(trimmed, ":") && !strings.ContainsAny(trimmed[:len(trimmed)-1], " \t,") {
			label := strings.TrimSuffix(trimmed, ":")
			defined[label] = true
			continue
		}
		if idx := strings.Index(trimmed, ":"); idx > 0 {
			candidate := trimmed[:idx]
			if !strings.ContainsAny(candidate, " \t,()") {
				defined[candidate] = true
			}
		}
		CollectBranchTargets(trimmed, referenced)
	}

	for label := range referenced {
		if !defined[label] {
			sb.WriteString(fmt.Sprintf("%s:\n", label))
		}
	}

	sb.WriteString(asm)
	sb.WriteString("\n")

	for label := range referenced {
		if !defined[label] {
			sb.WriteString(fmt.Sprintf("%s_end:\n", label))
		}
	}

	return sb.String()
}

// CollectBranchTargets extracts label names from JP/CALL/JR/DJNZ/LD instructions.
func CollectBranchTargets(line string, refs map[string]bool) {
	upper := strings.ToUpper(strings.TrimSpace(line))
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
			if strings.Contains(rest, ",") {
				parts := strings.SplitN(rest, ",", 2)
				rest = strings.TrimSpace(parts[len(parts)-1])
			}
			rest = strings.TrimSpace(rest)
			if rest != "" && !strings.HasPrefix(rest, "(") &&
				!strings.HasPrefix(rest, "$") && !strings.HasPrefix(rest, "0") &&
				(rest[0] < '0' || rest[0] > '9') {
				refs[rest] = true
			}
			return
		}
	}

	trimmed := strings.TrimSpace(line)
	trimmedUpper := strings.ToUpper(trimmed)
	ldPrefixes := []string{"LD HL,", "LD DE,", "LD BC,", "LD IX,", "LD IY,", "LD SP,"}
	for _, prefix := range ldPrefixes {
		if strings.HasPrefix(trimmedUpper, prefix) {
			rest := strings.TrimSpace(trimmed[len(prefix):])
			if rest != "" && !strings.HasPrefix(rest, "(") &&
				!strings.HasPrefix(rest, "$") && !strings.HasPrefix(rest, "0") &&
				(rest[0] < '0' || rest[0] > '9') {
				refs[rest] = true
			}
			return
		}
	}
}
