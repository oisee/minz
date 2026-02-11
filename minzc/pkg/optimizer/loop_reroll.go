package optimizer

import (
	"fmt"

	"github.com/minz/minzc/pkg/ir"
)

// LoopRerollPass detects unrolled loop patterns and re-rolls them
// This is the inverse of loop unrolling - finding repeated patterns with varying data
//
// Optimization tag: SIZE (reduces code size at cost of speed)
//
// Currently transforms:
//   - putchar sequences → print_string + data
//
// TODO: Generic transformation for any repeated call:
//   - func(1); func(2); func(3); → data_table + loop calling func
//
type LoopRerollPass struct {
	minRepeats int // Minimum repetitions to trigger (default: 3)
}

// NewLoopRerollPass creates a new loop re-rolling pass
func NewLoopRerollPass() *LoopRerollPass {
	return &LoopRerollPass{
		minRepeats: 3,
	}
}

// Name returns the pass name
func (p *LoopRerollPass) Name() string {
	return "LoopReroll"
}

// Run executes the pass on a module
func (p *LoopRerollPass) Run(module *ir.Module) (bool, error) {
	changed := false
	debug := false // Set to true to enable debug output

	for _, fn := range module.Functions {
		if debug {
			fmt.Printf("DEBUG LoopReroll: Function %s has %d instructions\n", fn.Name, len(fn.Instructions))
			for i, inst := range fn.Instructions {
				fmt.Printf("  [%d] Op=%v Imm=%d Symbol=%s\n", i, inst.Op, inst.Imm, inst.Symbol)
			}
		}
		if p.optimizeFunction(fn, module) {
			changed = true
		}
	}

	return changed, nil
}

// PatternTemplate represents a repeating instruction pattern with holes
type PatternTemplate struct {
	Instructions []InstructionMatcher
	Holes        []string // Names of varying parts (e.g., "$char", "$addr")
}

// InstructionMatcher matches an instruction with optional wildcards
type InstructionMatcher struct {
	Op       ir.Opcode
	DestHole string // If set, matches any dest and captures to this name
	Src1Hole string // If set, matches any src1 and captures to this name
	ImmHole  string // If set, matches any imm and captures to this name
	Symbol   string // If set, must match exactly
}

// PatternMatch represents a successful pattern match
type PatternMatch struct {
	StartIndex int
	EndIndex   int
	Repeats    int
	Captures   []map[string]int64 // One map per repeat, hole name → value
}

// optimizeFunction looks for unrolled loop patterns
func (p *LoopRerollPass) optimizeFunction(fn *ir.Function, module *ir.Module) bool {
	if len(fn.Instructions) < p.minRepeats*2 {
		return false
	}

	// Define patterns to look for
	// IR generates LoadConst for each param, then duplicates in same order
	// For 2 params: p1, p2, p1_dup, p2_dup, Call (interleaved pattern)
	patterns := []PatternTemplate{
		// 1 parameter: LoadConst(p1), LoadConst(p1_dup), Call
		{
			Instructions: []InstructionMatcher{
				{Op: ir.OpLoadConst, ImmHole: "$p1"},
				{Op: ir.OpLoadConst}, // Duplicate
				{Op: ir.OpCall},
			},
			Holes: []string{"$p1"},
		},
		// 2 parameters: p1, p2, p1_dup, p2_dup, Call
		{
			Instructions: []InstructionMatcher{
				{Op: ir.OpLoadConst, ImmHole: "$p1"},
				{Op: ir.OpLoadConst, ImmHole: "$p2"},
				{Op: ir.OpLoadConst}, // p1 duplicate
				{Op: ir.OpLoadConst}, // p2 duplicate
				{Op: ir.OpCall},
			},
			Holes: []string{"$p1", "$p2"},
		},
		// 3 parameters: p1, p2, p3, p1_dup, p2_dup, p3_dup, Call
		{
			Instructions: []InstructionMatcher{
				{Op: ir.OpLoadConst, ImmHole: "$p1"},
				{Op: ir.OpLoadConst, ImmHole: "$p2"},
				{Op: ir.OpLoadConst, ImmHole: "$p3"},
				{Op: ir.OpLoadConst}, // p1 dup
				{Op: ir.OpLoadConst}, // p2 dup
				{Op: ir.OpLoadConst}, // p3 dup
				{Op: ir.OpCall},
			},
			Holes: []string{"$p1", "$p2", "$p3"},
		},
		// 4 parameters: p1, p2, p3, p4, p1_dup, p2_dup, p3_dup, p4_dup, Call
		{
			Instructions: []InstructionMatcher{
				{Op: ir.OpLoadConst, ImmHole: "$p1"},
				{Op: ir.OpLoadConst, ImmHole: "$p2"},
				{Op: ir.OpLoadConst, ImmHole: "$p3"},
				{Op: ir.OpLoadConst, ImmHole: "$p4"},
				{Op: ir.OpLoadConst}, // p1 dup
				{Op: ir.OpLoadConst}, // p2 dup
				{Op: ir.OpLoadConst}, // p3 dup
				{Op: ir.OpLoadConst}, // p4 dup
				{Op: ir.OpCall},
			},
			Holes: []string{"$p1", "$p2", "$p3", "$p4"},
		},
		// 5 parameters
		{
			Instructions: []InstructionMatcher{
				{Op: ir.OpLoadConst, ImmHole: "$p1"},
				{Op: ir.OpLoadConst, ImmHole: "$p2"},
				{Op: ir.OpLoadConst, ImmHole: "$p3"},
				{Op: ir.OpLoadConst, ImmHole: "$p4"},
				{Op: ir.OpLoadConst, ImmHole: "$p5"},
				{Op: ir.OpLoadConst}, {Op: ir.OpLoadConst}, {Op: ir.OpLoadConst},
				{Op: ir.OpLoadConst}, {Op: ir.OpLoadConst}, // duplicates
				{Op: ir.OpCall},
			},
			Holes: []string{"$p1", "$p2", "$p3", "$p4", "$p5"},
		},
		// 6 parameters
		{
			Instructions: []InstructionMatcher{
				{Op: ir.OpLoadConst, ImmHole: "$p1"},
				{Op: ir.OpLoadConst, ImmHole: "$p2"},
				{Op: ir.OpLoadConst, ImmHole: "$p3"},
				{Op: ir.OpLoadConst, ImmHole: "$p4"},
				{Op: ir.OpLoadConst, ImmHole: "$p5"},
				{Op: ir.OpLoadConst, ImmHole: "$p6"},
				{Op: ir.OpLoadConst}, {Op: ir.OpLoadConst}, {Op: ir.OpLoadConst},
				{Op: ir.OpLoadConst}, {Op: ir.OpLoadConst}, {Op: ir.OpLoadConst}, // duplicates
				{Op: ir.OpCall},
			},
			Holes: []string{"$p1", "$p2", "$p3", "$p4", "$p5", "$p6"},
		},
		// 7 parameters
		{
			Instructions: []InstructionMatcher{
				{Op: ir.OpLoadConst, ImmHole: "$p1"},
				{Op: ir.OpLoadConst, ImmHole: "$p2"},
				{Op: ir.OpLoadConst, ImmHole: "$p3"},
				{Op: ir.OpLoadConst, ImmHole: "$p4"},
				{Op: ir.OpLoadConst, ImmHole: "$p5"},
				{Op: ir.OpLoadConst, ImmHole: "$p6"},
				{Op: ir.OpLoadConst, ImmHole: "$p7"},
				{Op: ir.OpLoadConst}, {Op: ir.OpLoadConst}, {Op: ir.OpLoadConst},
				{Op: ir.OpLoadConst}, {Op: ir.OpLoadConst}, {Op: ir.OpLoadConst},
				{Op: ir.OpLoadConst}, // duplicates
				{Op: ir.OpCall},
			},
			Holes: []string{"$p1", "$p2", "$p3", "$p4", "$p5", "$p6", "$p7"},
		},
		// Legacy: putchar pattern (single LoadConst without duplicate)
		{
			Instructions: []InstructionMatcher{
				{Op: ir.OpLoadConst, ImmHole: "$char"},
				{Op: ir.OpCall, Symbol: "putchar"},
			},
			Holes: []string{"$char"},
		},
	}

	changed := false
	debug := false // Enable for pattern match debugging

	for pi, pattern := range patterns {
		matches := p.findPatternMatches(fn.Instructions, pattern)
		if debug && len(matches) > 0 {
			fmt.Printf("DEBUG: Pattern %d found %d matches\n", pi, len(matches))
		}
		for _, match := range matches {
			if debug {
				fmt.Printf("DEBUG: Match at %d-%d with %d repeats\n", match.StartIndex, match.EndIndex, match.Repeats)
			}
			if match.Repeats >= p.minRepeats {
				if p.transformMatch(fn, module, match, pattern) {
					changed = true
				}
			}
		}
	}

	return changed
}

// findPatternMatches finds all occurrences of a pattern template
func (p *LoopRerollPass) findPatternMatches(insts []ir.Instruction, pattern PatternTemplate) []PatternMatch {
	var matches []PatternMatch
	patLen := len(pattern.Instructions)

	i := 0
	for i <= len(insts)-patLen {
		// Try to match starting at position i
		match := p.tryMatchPattern(insts, i, pattern)
		if match != nil && match.Repeats >= p.minRepeats {
			matches = append(matches, *match)
			i = match.EndIndex // Skip past this match
		} else {
			i++
		}
	}

	return matches
}

// tryMatchPattern attempts to match a repeating pattern starting at index
func (p *LoopRerollPass) tryMatchPattern(insts []ir.Instruction, start int, pattern PatternTemplate) *PatternMatch {
	patLen := len(pattern.Instructions)
	match := &PatternMatch{
		StartIndex: start,
		Captures:   make([]map[string]int64, 0),
	}

	// First, match the initial occurrence to establish the pattern
	firstCaptures := p.matchSingleOccurrence(insts, start, pattern)
	if firstCaptures == nil {
		return nil
	}
	match.Captures = append(match.Captures, firstCaptures)
	match.Repeats = 1

	// Now try to match subsequent occurrences
	pos := start + patLen
	for pos <= len(insts)-patLen {
		captures := p.matchSingleOccurrence(insts, pos, pattern)
		if captures == nil {
			break // Pattern doesn't continue
		}

		// Verify the non-hole parts match the first occurrence
		if !p.structurallyEqual(insts, start, pos, pattern) {
			break
		}

		match.Captures = append(match.Captures, captures)
		match.Repeats++
		pos += patLen
	}

	match.EndIndex = start + (match.Repeats * patLen)
	return match
}

// matchSingleOccurrence tries to match one occurrence of the pattern
func (p *LoopRerollPass) matchSingleOccurrence(insts []ir.Instruction, start int, pattern PatternTemplate) map[string]int64 {
	if start+len(pattern.Instructions) > len(insts) {
		return nil
	}

	captures := make(map[string]int64)

	for i, matcher := range pattern.Instructions {
		inst := insts[start+i]

		// Check opcode
		if inst.Op != matcher.Op {
			return nil
		}

		// Check fixed symbol if specified
		if matcher.Symbol != "" && !containsSymbol(inst.Symbol, matcher.Symbol) {
			return nil
		}

		// Capture holes
		if matcher.ImmHole != "" {
			captures[matcher.ImmHole] = inst.Imm
		}
		if matcher.Src1Hole != "" {
			captures[matcher.Src1Hole] = int64(inst.Src1)
		}
	}

	return captures
}

// structurallyEqual checks if two pattern occurrences have the same structure
func (p *LoopRerollPass) structurallyEqual(insts []ir.Instruction, idx1, idx2 int, pattern PatternTemplate) bool {
	for i, matcher := range pattern.Instructions {
		inst1 := insts[idx1+i]
		inst2 := insts[idx2+i]

		// Opcodes must match
		if inst1.Op != inst2.Op {
			return false
		}

		// Symbols must match (the function being called)
		if matcher.Symbol == "" && inst1.Symbol != inst2.Symbol {
			return false
		}
	}
	return true
}

// transformMatch transforms a matched pattern into optimized code
func (p *LoopRerollPass) transformMatch(fn *ir.Function, module *ir.Module, match PatternMatch, pattern PatternTemplate) bool {
	debug := false // Enable debug for multi-param capture verification

	// Extract the varying values for each repeat (all captured parameters)
	// For multi-parameter patterns, we need ALL captured values, not just first
	allValues := make([][]int64, len(match.Captures))
	for i, capture := range match.Captures {
		allValues[i] = make([]int64, len(pattern.Holes))
		for j, holeName := range pattern.Holes {
			if v, ok := capture[holeName]; ok {
				allValues[i][j] = v
			}
		}
		if debug {
			fmt.Printf("DEBUG: Repeat %d captures: %v\n", i, allValues[i])
		}
	}

	// Legacy: single-value extraction for putchar compatibility
	values := make([]int64, len(match.Captures))
	for i, capture := range match.Captures {
		for _, holeName := range pattern.Holes {
			if v, ok := capture[holeName]; ok {
				values[i] = v
				break
			}
		}
	}

	// Check if this is a putchar pattern (all values are printable or control chars)
	isPutcharPattern := p.isPutcharLikePattern(fn.Instructions, match, pattern)

	if isPutcharPattern && len(values) >= 3 {
		return p.transformToPrintString(fn, module, match, values)
	}

	// For other patterns, mark for potential loop transformation with details
	if match.StartIndex < len(fn.Instructions) {
		// Get the function being called
		funcName := ""
		patLen := len(pattern.Instructions)
		for i := 0; i < patLen && match.StartIndex+i < len(fn.Instructions); i++ {
			inst := fn.Instructions[match.StartIndex+i]
			if inst.Op == ir.OpCall && inst.Symbol != "" {
				funcName = inst.Symbol
				break
			}
		}

		// Build parameter summary
		paramCount := len(pattern.Holes)
		paramInfo := itoa(paramCount) + "-param"

		fn.Instructions[match.StartIndex].Comment =
			"[LOOP_REROLL:" + paramInfo + "] " + itoa(match.Repeats) + " repeats of " + funcName +
				" → potential data table + loop"
	}

	return false
}

// isPutcharLikePattern checks if the pattern is a putchar-like call
func (p *LoopRerollPass) isPutcharLikePattern(insts []ir.Instruction, match PatternMatch, pattern PatternTemplate) bool {
	if match.StartIndex >= len(insts) {
		return false
	}

	// Look for Call instruction in the pattern
	patLen := len(pattern.Instructions)
	for i := 0; i < patLen && match.StartIndex+i < len(insts); i++ {
		inst := insts[match.StartIndex+i]
		if inst.Op == ir.OpCall {
			sym := inst.Symbol
			if containsSymbol(sym, "putchar") || containsSymbol(sym, "putch") ||
				containsSymbol(sym, "print") {
				return true
			}
		}
	}
	return false
}

// transformToPrintString transforms repeated putchar calls to a single print_string
func (p *LoopRerollPass) transformToPrintString(fn *ir.Function, module *ir.Module, match PatternMatch, values []int64) bool {
	// Convert values to string
	chars := make([]byte, len(values))
	for i, v := range values {
		chars[i] = byte(v)
	}
	stringValue := string(chars)

	// Create a unique label for the string
	stringLabel := "_str_reroll_" + itoa(len(module.Strings))

	// Add string to module
	module.Strings = append(module.Strings, &ir.String{
		Label: stringLabel,
		Value: stringValue,
	})

	// Calculate how many instructions to remove
	patternLen := 2 // LoadConst + Call per repeat
	removeCount := match.Repeats * patternLen

	// Create the replacement instruction
	printStringInst := ir.Instruction{
		Op:      ir.OpPrintString,
		Symbol:  stringLabel,
		Comment: "[REROLLED] " + itoa(match.Repeats) + " putchar calls → print_string(\"" + escapeString(stringValue) + "\")",
	}

	// Build new instruction list
	newInsts := make([]ir.Instruction, 0, len(fn.Instructions)-removeCount+1)

	// Copy instructions before the match
	newInsts = append(newInsts, fn.Instructions[:match.StartIndex]...)

	// Add the optimized instruction
	newInsts = append(newInsts, printStringInst)

	// Copy instructions after the match
	if match.EndIndex < len(fn.Instructions) {
		newInsts = append(newInsts, fn.Instructions[match.EndIndex:]...)
	}

	fn.Instructions = newInsts
	return true
}

// escapeString creates a printable representation of a string
func escapeString(s string) string {
	result := make([]byte, 0, len(s)*2)
	for _, c := range []byte(s) {
		switch c {
		case '\n':
			result = append(result, '\\', 'n')
		case '\r':
			result = append(result, '\\', 'r')
		case '\t':
			result = append(result, '\\', 't')
		case '"':
			result = append(result, '\\', '"')
		case '\\':
			result = append(result, '\\', '\\')
		default:
			if c >= 32 && c < 127 {
				result = append(result, c)
			} else {
				// Hex escape for non-printable
				result = append(result, '\\', 'x')
				result = append(result, hexDigit(c>>4), hexDigit(c&0xf))
			}
		}
	}
	return string(result)
}

// hexDigit returns hex character for a nibble
func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}

// containsSymbol checks if the instruction symbol contains the target
func containsSymbol(instSymbol, target string) bool {
	if instSymbol == target {
		return true
	}
	// Check for variations like "module.putchar" or "putchar$u8"
	for i := 0; i+len(target) <= len(instSymbol); i++ {
		if instSymbol[i:i+len(target)] == target {
			return true
		}
	}
	return false
}
