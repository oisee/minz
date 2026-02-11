package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// StringIntentPass detects sequences of putchar(constant) calls
// and transforms them into efficient string printing
type StringIntentPass struct {
	minLength int // Minimum chars to trigger optimization (default: 3)
}

// NewStringIntentPass creates a new string intent detection pass
func NewStringIntentPass() *StringIntentPass {
	return &StringIntentPass{
		minLength: 3,
	}
}

// Name returns the pass name
func (p *StringIntentPass) Name() string {
	return "StringIntentDetection"
}

// Run executes the pass on a module
func (p *StringIntentPass) Run(module *ir.Module) (bool, error) {
	changed := false

	for _, fn := range module.Functions {
		if p.optimizeFunction(fn, module) {
			changed = true
		}
	}

	return changed, nil
}

// optimizeFunction looks for putchar sequences in a function
func (p *StringIntentPass) optimizeFunction(fn *ir.Function, module *ir.Module) bool {
	if len(fn.Instructions) < p.minLength*2 {
		return false
	}

	changed := false
	newInstructions := make([]ir.Instruction, 0, len(fn.Instructions))

	i := 0
	for i < len(fn.Instructions) {
		// Try to find a sequence of putchar calls with constants
		seq := p.findPutcharSequence(fn.Instructions, i)

		if len(seq.chars) >= p.minLength {
			// Found a sequence! Replace with StringPrint intent
			stringLabel := p.addStringToModule(module, seq.chars)

			// Emit a single StringPrint instruction
			newInstructions = append(newInstructions, ir.Instruction{
				Op:      ir.OpPrintString,
				Comment: "Optimized string print: " + string(seq.chars),
				AsmCode: stringLabel, // Store label in AsmCode field
			})

			i = seq.endIndex
			changed = true
		} else {
			// No sequence, keep original instruction
			newInstructions = append(newInstructions, fn.Instructions[i])
			i++
		}
	}

	if changed {
		fn.Instructions = newInstructions
	}

	return changed
}

// putcharSequence represents a detected sequence of putchar calls
type putcharSequence struct {
	chars    []byte
	endIndex int
}

// findPutcharSequence looks for consecutive putchar(constant) calls
func (p *StringIntentPass) findPutcharSequence(insts []ir.Instruction, start int) putcharSequence {
	seq := putcharSequence{
		chars:    make([]byte, 0),
		endIndex: start,
	}

	i := start
	for i < len(insts) {
		// Look for pattern: LoadConst followed by CALL to putchar
		if i+1 < len(insts) {
			inst := insts[i]
			nextInst := insts[i+1]

			// Check for LoadConst with immediate value (character)
			if inst.Op == ir.OpLoadConst && inst.Imm >= 0 && inst.Imm <= 255 {
				// Check if next is a CALL to putchar variant
				if nextInst.Op == ir.OpCall && isPutcharCall(nextInst) {
					// Check that the CONST result is used by the CALL
					if nextInst.Src1 == inst.Dest || isArgRegister(inst.Dest, nextInst) {
						seq.chars = append(seq.chars, byte(inst.Imm))
						seq.endIndex = i + 2
						i += 2
						continue
					}
				}
			}
		}

		// Also check for direct OpPrint with constant
		if insts[i].Op == ir.OpPrint {
			// Check if source is a constant
			// This requires tracking the value - for now, break the sequence
		}

		// No match, end sequence
		break
	}

	return seq
}

// isPutcharCall checks if an instruction is a call to a putchar variant
func isPutcharCall(inst ir.Instruction) bool {
	// Check various putchar function names using Symbol field
	name := inst.Symbol
	if name == "" {
		return false
	}

	return name == "putchar" ||
		name == "putchar$u8" ||
		name == "putch" ||
		name == "putch$u8" ||
		name == "bdos.putchar" ||
		name == "bdos.putchar$u8" ||
		name == "cpm.bdos.putchar$u8" ||
		// Add more variants as needed
		containsPutchar(name)
}

// containsPutchar checks if name contains putchar pattern
func containsPutchar(name string) bool {
	for i := 0; i+6 < len(name); i++ {
		if name[i:i+7] == "putchar" || name[i:i+5] == "putch" {
			return true
		}
	}
	// Check for just "putch" at end
	if len(name) >= 5 && name[len(name)-5:] == "putch" {
		return true
	}
	if len(name) >= 7 && name[len(name)-7:] == "putchar" {
		return true
	}
	return false
}

// isArgRegister checks if reg is used as an argument in the call
func isArgRegister(reg ir.Register, callInst ir.Instruction) bool {
	// Check Src1, Src2, etc.
	return callInst.Src1 == reg || callInst.Src2 == reg
}

// addStringToModule adds a string constant to the module's data section
func (p *StringIntentPass) addStringToModule(module *ir.Module, chars []byte) string {
	// Generate unique label
	label := "_str_" + itoa(len(module.Strings))

	// Add to module's string list
	module.Strings = append(module.Strings, &ir.String{
		Label: label,
		Value: string(chars),
	})

	return label
}

// itoa converts int to string (simple implementation)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
