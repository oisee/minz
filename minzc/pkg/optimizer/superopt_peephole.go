package optimizer

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// SuperoptRule represents a single superoptimizer-proven replacement rule.
type SuperoptRule struct {
	ReplacementAsm string
	BytesSaved     int
	CyclesSaved    int
}

// SuperoptPeepholePass applies superoptimizer-proven rules to adjacent instruction pairs.
type SuperoptPeepholePass struct {
	rules      map[string]SuperoptRule // key = "INST1 : INST2" normalized
	count      int
	bytesSaved int
}

// superoptRuleJSON is the JSON structure from z80-optimizer rules.json.
type superoptRuleJSON struct {
	SourceAsm        string `json:"source_asm"`
	ReplacementAsm   string `json:"replacement_asm"`
	SourceBytes      int    `json:"source_bytes"`
	ReplacementBytes int    `json:"replacement_bytes"`
	BytesSaved       int    `json:"bytes_saved"`
	CyclesSaved      int    `json:"cycles_saved"`
}

// directiveKeywords are assembly directives that should be skipped during normalization.
var directiveKeywords = map[string]bool{
	"ORG": true, "DB": true, "DW": true, "DS": true, "EQU": true,
	"END": true, "INCLUDE": true, "SECTION": true, "DEFS": true,
	"DEFB": true, "DEFW": true, "ALIGN": true, "INCBIN": true,
}

// smcMarkers are SMC-related substrings that mark lines as untouchable.
var smcMarkers = []string{"_imm", "PATCH_TABLE", "@keep", "@no-opt", "@preserve"}

// operandImmRe matches decimal numbers in operand positions.
// It matches:
//   - Numbers after comma+space: ", 255"
//   - Numbers in parentheses: "(123)"
//   - Numbers after space (operand start): "LD A, 255"
//
// But not register-embedded digits (B is never a digit context anyway).
var operandImmRe = regexp.MustCompile(`\b(\d+)\b`)

// dollarHexRe matches $XX hex addresses in MinZ output.
var dollarHexRe = regexp.MustCompile(`\$([0-9A-Fa-f]+)`)

// z80Registers that should not be treated as numeric immediates.
var z80Registers = map[string]bool{
	"A": true, "B": true, "C": true, "D": true, "E": true, "H": true, "L": true,
	"AF": true, "BC": true, "DE": true, "HL": true, "SP": true, "IX": true, "IY": true,
	"IXH": true, "IXL": true, "IYH": true, "IYL": true,
}

// is16BitRegContext returns true if the instruction operates on a 16-bit register,
// meaning numeric immediates should be formatted as 16-bit (4 hex digits).
func is16BitRegContext(line string) bool {
	upper := strings.ToUpper(line)
	// Check for 16-bit register references in the instruction
	for _, reg := range []string{"HL", "BC", "DE", "SP", "IX", "IY"} {
		if strings.Contains(upper, reg) {
			return true
		}
	}
	// JP and CALL always use 16-bit addresses
	if strings.HasPrefix(upper, "JP ") || strings.HasPrefix(upper, "CALL ") {
		return true
	}
	return false
}

// decToHex converts a decimal string to z80-optimizer hex format.
// For 8-bit context: "255" → "0FFh", "0" → "00h", "67" → "43h"
// For 16-bit context: "1" → "0001h", "0" → "0000h"
func decToHex(decStr string, is16bit bool) string {
	val, err := strconv.ParseUint(decStr, 10, 16)
	if err != nil {
		return decStr // not a valid number, return as-is
	}

	var hex string
	if is16bit {
		hex = fmt.Sprintf("%04X", val)
	} else {
		hex = fmt.Sprintf("%02X", val)
	}

	// Add leading 0 if first character is A-F (z80-optimizer convention)
	if hex[0] >= 'A' && hex[0] <= 'F' {
		hex = "0" + hex
	}
	return hex + "h"
}

// normalizeForSuperopt normalizes a MinZ assembly line to z80-optimizer format.
// Returns "" for lines that should be skipped (labels, comments, directives, SMC).
func normalizeForSuperopt(line string) string {
	// 1. Trim whitespace
	s := strings.TrimSpace(line)

	// 2. Skip empty lines, full-line comments
	if s == "" {
		return ""
	}
	if s[0] == ';' {
		return ""
	}

	// 3. Skip SMC markers (check before stripping comments — markers may be in comments)
	for _, marker := range smcMarkers {
		if strings.Contains(s, marker) {
			return ""
		}
	}

	// 4. Strip trailing comment: find "  ;" or "\t;"
	if idx := strings.Index(s, "  ;"); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if idx := strings.Index(s, "\t;"); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}

	// 5. Skip if empty after comment stripping, or is a label
	if s == "" {
		return ""
	}
	if s[len(s)-1] == ':' {
		return ""
	}

	// 6. Skip directives
	firstWord := s
	if idx := strings.IndexByte(s, ' '); idx >= 0 {
		firstWord = s[:idx]
	}
	if directiveKeywords[strings.ToUpper(firstWord)] {
		return ""
	}

	// 6. Convert $XX hex addresses to XXh format
	s = dollarHexRe.ReplaceAllStringFunc(s, func(match string) string {
		hex := strings.ToUpper(match[1:]) // strip $
		if hex[0] >= 'A' && hex[0] <= 'F' {
			hex = "0" + hex
		}
		return hex + "h"
	})

	// 7. Convert decimal immediates to hex format
	is16bit := is16BitRegContext(s)
	s = operandImmRe.ReplaceAllStringFunc(s, func(match string) string {
		// Don't convert if this is part of a register name or already hex
		if z80Registers[strings.ToUpper(match)] {
			return match
		}
		// Check if it's actually a number (all digits)
		if _, err := strconv.ParseUint(match, 10, 16); err != nil {
			return match
		}
		return decToHex(match, is16bit)
	})

	// 8. Collapse multiple spaces to single space
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}

	return s
}

// NewSuperoptPeepholePass loads superoptimizer rules from a JSON file and returns
// a pass ready to optimize assembly. Supports .gz compressed files.
func NewSuperoptPeepholePass(rulesPath string) (*SuperoptPeepholePass, error) {
	f, err := os.Open(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open rules file: %w", err)
	}
	defer f.Close()

	// Detect gzip by extension
	var decoder *json.Decoder
	if strings.HasSuffix(rulesPath, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gz.Close()
		decoder = json.NewDecoder(gz)
	} else {
		decoder = json.NewDecoder(f)
	}

	// Read opening bracket
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return nil, fmt.Errorf("expected JSON array, got %v", token)
	}

	pass := &SuperoptPeepholePass{
		rules: make(map[string]SuperoptRule),
	}

	// Stream-decode rules one at a time to avoid loading entire 102MB into memory at once
	for decoder.More() {
		var rule superoptRuleJSON
		if err := decoder.Decode(&rule); err != nil {
			return nil, fmt.Errorf("failed to decode rule: %w", err)
		}

		// Score for tie-breaking: prioritize byte savings, then cycle savings
		score := rule.BytesSaved*2 + rule.CyclesSaved

		key := rule.SourceAsm
		if existing, ok := pass.rules[key]; ok {
			existingScore := existing.BytesSaved*2 + existing.CyclesSaved
			if score <= existingScore {
				continue // existing rule is as good or better
			}
		}

		pass.rules[key] = SuperoptRule{
			ReplacementAsm: rule.ReplacementAsm,
			BytesSaved:     rule.BytesSaved,
			CyclesSaved:    rule.CyclesSaved,
		}
	}

	return pass, nil
}

// NewSuperoptPeepholePassFromMap creates a pass from an in-memory rule map (for testing).
func NewSuperoptPeepholePassFromMap(rules map[string]SuperoptRule) *SuperoptPeepholePass {
	return &SuperoptPeepholePass{
		rules: rules,
	}
}

// Count returns the number of rules applied during optimization.
func (p *SuperoptPeepholePass) Count() int {
	return p.count
}

// BytesSaved returns the total bytes saved during optimization.
func (p *SuperoptPeepholePass) BytesSaved() int {
	return p.bytesSaved
}

// RuleCount returns the number of loaded rules.
func (p *SuperoptPeepholePass) RuleCount() int {
	return len(p.rules)
}

// hexToDecRe matches hex immediates with h suffix in z80-optimizer format.
var hexToDecRe = regexp.MustCompile(`\b0?([0-9A-Fa-f]+)h\b`)

// replacementToMinZ converts a z80-optimizer replacement back to MinZ assembly format.
// "LD A, 0FFh" → "LD A, 255", "XOR A" → "XOR A"
func replacementToMinZ(replacement string) string {
	is16bit := is16BitRegContext(replacement)
	return hexToDecRe.ReplaceAllStringFunc(replacement, func(match string) string {
		// Strip trailing 'h' and leading '0' if it was added for A-F disambiguation
		hex := match[:len(match)-1] // strip 'h'
		if len(hex) > 1 && hex[0] == '0' && hex[1] >= 'A' && hex[1] <= 'F' {
			hex = hex[1:] // strip leading 0
		}
		if len(hex) > 1 && hex[0] == '0' && hex[1] >= 'a' && hex[1] <= 'f' {
			hex = hex[1:]
		}
		val, err := strconv.ParseUint(hex, 16, 16)
		if err != nil {
			return match
		}
		_ = is16bit // decimal is always decimal regardless of width
		return strconv.FormatUint(val, 10)
	})
}

// OptimizeAssembly applies superoptimizer rules to the given assembly text.
func (p *SuperoptPeepholePass) OptimizeAssembly(assembly string) string {
	lines := strings.Split(assembly, "\n")
	result := make([]string, len(lines))
	copy(result, lines)

	i := 0
	for i < len(result)-1 {
		normA := normalizeForSuperopt(result[i])
		if normA == "" {
			i++
			continue
		}

		normB := normalizeForSuperopt(result[i+1])
		if normB == "" {
			i++
			continue
		}

		key := normA + " : " + normB
		if rule, ok := p.rules[key]; ok {
			// Preserve indentation from original line
			indent := ""
			trimmed := strings.TrimLeft(result[i], " \t")
			if len(trimmed) < len(result[i]) {
				indent = result[i][:len(result[i])-len(trimmed)]
			}

			// Convert replacement from z80-optimizer hex format back to MinZ decimal
			replacement := replacementToMinZ(rule.ReplacementAsm)

			result[i] = indent + replacement + "  ; superopt: " + normA + " : " + normB +
				fmt.Sprintf(" (-%db, -%dc)", rule.BytesSaved, rule.CyclesSaved)
			result[i+1] = "" // blank the second line
			p.count++
			p.bytesSaved += rule.BytesSaved
			i += 2
		} else {
			i++
		}
	}

	// Append summary
	if p.count > 0 {
		result = append(result, fmt.Sprintf("; Superoptimizer: %d rules applied, %d bytes saved", p.count, p.bytesSaved))
	}

	return strings.Join(result, "\n")
}
