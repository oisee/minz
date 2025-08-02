package optimizer

import (
	"strings"
)

// Z80InstructionSelectionPass selects optimal Z80 instructions
type Z80InstructionSelectionPass struct {
	patterns []InstructionPattern
}

type InstructionPattern struct {
	Name        string
	Match       []string
	Replace     []string
	Condition   func([]AssemblyLine) bool
	CycleSaving int
}

func NewZ80InstructionSelectionPass() *Z80InstructionSelectionPass {
	return &Z80InstructionSelectionPass{
		patterns: []InstructionPattern{
			{
				Name:        "INC/DEC instead of ADD/SUB 1",
				Match:       []string{"LD A, %r", "ADD A, 1", "LD %r, A"},
				Replace:     []string{"INC %r"},
				CycleSaving: 11, // 15 cycles -> 4 cycles
			},
			{
				Name:        "EX DE,HL optimization",
				Match:       []string{"LD D, H", "LD E, L"},
				Replace:     []string{"EX DE, HL"},
				CycleSaving: 4, // 8 cycles -> 4 cycles
			},
			{
				Name:        "Zero test optimization",
				Match:       []string{"CP 0"},
				Replace:     []string{"OR A"},
				CycleSaving: 3, // 7 cycles -> 4 cycles
			},
			{
				Name:        "Clear accumulator",
				Match:       []string{"LD A, 0"},
				Replace:     []string{"XOR A"},
				CycleSaving: 3, // 7 cycles -> 4 cycles
			},
			{
				Name:        "16-bit increment",
				Match:       []string{"LD BC, 1", "ADD HL, BC"},
				Replace:     []string{"INC HL"},
				CycleSaving: 15, // 10+11 -> 6 cycles
			},
			{
				Name:        "Multiply by 2",
				Match:       []string{"LD B, 2", "CALL multiply"},
				Replace:     []string{"ADD A, A"}, // or SLA A
				CycleSaving: 30, // Avoid multiply routine
			},
			{
				Name:        "Load and test",
				Match:       []string{"LD A, %m", "CP 0"},
				Replace:     []string{"LD A, %m", "OR A"},
				CycleSaving: 3,
			},
			{
				Name:        "Redundant load elimination",
				Match:       []string{"LD %r1, %r2", "LD %r2, %r1"},
				Replace:     []string{"LD %r1, %r2"},
				CycleSaving: 4,
			},
		},
	}
}

func (p *Z80InstructionSelectionPass) Name() string {
	return "Z80 Instruction Selection"
}

func (p *Z80InstructionSelectionPass) Apply(asm []AssemblyLine) ([]AssemblyLine, bool) {
	changed := false
	result := []AssemblyLine{}
	
	i := 0
	for i < len(asm) {
		matched := false
		
		// Try each pattern
		for _, pattern := range p.patterns {
			if match, params := p.matchPattern(asm, i, pattern.Match); match {
				// Apply replacement
				replacement := p.applyReplacement(pattern.Replace, params)
				result = append(result, replacement...)
				i += len(pattern.Match)
				matched = true
				changed = true
				break
			}
		}
		
		if !matched {
			result = append(result, asm[i])
			i++
		}
	}
	
	return result, changed
}

func (p *Z80InstructionSelectionPass) matchPattern(asm []AssemblyLine, start int, pattern []string) (bool, map[string]string) {
	if start+len(pattern) > len(asm) {
		return false, nil
	}
	
	params := make(map[string]string)
	
	for i, pat := range pattern {
		if !p.matchInstruction(asm[start+i], pat, params) {
			return false, nil
		}
	}
	
	return true, params
}

func (p *Z80InstructionSelectionPass) matchInstruction(line AssemblyLine, pattern string, params map[string]string) bool {
	parts := strings.Split(pattern, " ")
	if len(parts) < 1 {
		return false
	}
	
	// Check instruction mnemonic
	if parts[0] != line.Instruction {
		return false
	}
	
	// Check operands
	if len(parts) > 1 {
		patternOps := strings.Split(parts[1], ",")
		if len(patternOps) != len(line.Operands) {
			return false
		}
		
		for i, op := range patternOps {
			op = strings.TrimSpace(op)
			actualOp := strings.TrimSpace(line.Operands[i])
			
			if strings.HasPrefix(op, "%") {
				// Parameter capture
				paramName := op[1:]
				if existing, ok := params[paramName]; ok {
					// Must match existing parameter
					if existing != actualOp {
						return false
					}
				} else {
					// Capture new parameter
					params[paramName] = actualOp
				}
			} else {
				// Literal match
				if op != actualOp {
					return false
				}
			}
		}
	}
	
	return true
}

func (p *Z80InstructionSelectionPass) applyReplacement(replacement []string, params map[string]string) []AssemblyLine {
	result := []AssemblyLine{}
	
	for _, repl := range replacement {
		parts := strings.Split(repl, " ")
		line := AssemblyLine{
			Instruction: parts[0],
		}
		
		if len(parts) > 1 {
			ops := strings.Split(parts[1], ",")
			for _, op := range ops {
				op = strings.TrimSpace(op)
				if strings.HasPrefix(op, "%") {
					// Substitute parameter
					paramName := op[1:]
					if val, ok := params[paramName]; ok {
						line.Operands = append(line.Operands, val)
					}
				} else {
					line.Operands = append(line.Operands, op)
				}
			}
		}
		
		result = append(result, line)
	}
	
	return result
}

func (p *Z80InstructionSelectionPass) EstimateCost(asm []AssemblyLine) Cost {
	cycles := 0
	for _, line := range asm {
		cycles += getInstructionCycles(line.Instruction)
	}
	return Cost{
		Cycles: cycles,
		Size:   len(asm),
	}
}

// FlagOptimizationPass optimizes flag usage
type FlagOptimizationPass struct{}

func NewFlagOptimizationPass() *FlagOptimizationPass {
	return &FlagOptimizationPass{}
}

func (p *FlagOptimizationPass) Name() string {
	return "Flag Optimization"
}

func (p *FlagOptimizationPass) Apply(asm []AssemblyLine) ([]AssemblyLine, bool) {
	changed := false
	result := []AssemblyLine{}
	
	for i := 0; i < len(asm); i++ {
		line := asm[i]
		
		// Optimize flag-setting instructions
		if i+1 < len(asm) {
			next := asm[i+1]
			
			// Pattern: Operation that sets flags followed by conditional jump
			if p.setsFlags(line) && p.usesFlags(next) {
				// Check if we can combine or optimize
				if optimized := p.optimizeFlagPair(line, next); optimized != nil {
					result = append(result, optimized...)
					i++ // Skip next instruction
					changed = true
					continue
				}
			}
		}
		
		result = append(result, line)
	}
	
	return result, changed
}

func (p *FlagOptimizationPass) setsFlags(line AssemblyLine) bool {
	switch line.Instruction {
	case "ADD", "SUB", "AND", "OR", "XOR", "CP", "INC", "DEC":
		return true
	}
	return false
}

func (p *FlagOptimizationPass) usesFlags(line AssemblyLine) bool {
	switch line.Instruction {
	case "JZ", "JNZ", "JC", "JNC", "JP", "JM", "JPE", "JPO":
		return true
	}
	return false
}

func (p *FlagOptimizationPass) optimizeFlagPair(setter, user AssemblyLine) []AssemblyLine {
	// Specific optimizations for flag usage
	
	// Example: DEC B; JNZ loop -> DJNZ loop
	if setter.Instruction == "DEC" && len(setter.Operands) == 1 && 
	   setter.Operands[0] == "B" && user.Instruction == "JNZ" {
		return []AssemblyLine{
			{Instruction: "DJNZ", Operands: user.Operands},
		}
	}
	
	return nil
}

func (p *FlagOptimizationPass) EstimateCost(asm []AssemblyLine) Cost {
	return Cost{Size: len(asm)}
}

// AddressingModePass optimizes addressing modes
type AddressingModePass struct{}

func NewAddressingModePass() *AddressingModePass {
	return &AddressingModePass{}
}

func (p *AddressingModePass) Name() string {
	return "Addressing Mode Optimization"
}

func (p *AddressingModePass) Apply(asm []AssemblyLine) ([]AssemblyLine, bool) {
	changed := false
	result := []AssemblyLine{}
	
	// Track register contents for optimization
	regContents := make(map[string]string)
	
	for i := 0; i < len(asm); i++ {
		line := asm[i]
		
		// Track LD instructions
		if line.Instruction == "LD" && len(line.Operands) == 2 {
			dest := line.Operands[0]
			src := line.Operands[1]
			
			// Track register loads
			if p.isRegister(dest) {
				regContents[dest] = src
			}
			
			// Optimize indexed addressing
			if p.canUseIndexed(asm, i, regContents) {
				// Convert to indexed addressing
				optimized := p.convertToIndexed(asm, i, regContents)
				result = append(result, optimized...)
				i += p.getIndexedSpan(asm, i) - 1
				changed = true
				continue
			}
		}
		
		result = append(result, line)
	}
	
	return result, changed
}

func (p *AddressingModePass) isRegister(op string) bool {
	switch op {
	case "A", "B", "C", "D", "E", "H", "L", "HL", "BC", "DE", "IX", "IY":
		return true
	}
	return false
}

func (p *AddressingModePass) canUseIndexed(asm []AssemblyLine, start int, regContents map[string]string) bool {
	// Check if we have multiple accesses to array-like structure
	// that could benefit from indexed addressing
	
	// Pattern: LD HL, base; LD A, (HL); INC HL; LD B, (HL)
	// Can become: LD IX, base; LD A, (IX+0); LD B, (IX+1)
	
	if start+3 >= len(asm) {
		return false
	}
	
	// Simple pattern detection
	if asm[start].Instruction == "LD" && asm[start].Operands[0] == "HL" &&
	   asm[start+1].Instruction == "LD" && asm[start+1].Operands[1] == "(HL)" &&
	   asm[start+2].Instruction == "INC" && asm[start+2].Operands[0] == "HL" &&
	   asm[start+3].Instruction == "LD" && asm[start+3].Operands[1] == "(HL)" {
		return true
	}
	
	return false
}

func (p *AddressingModePass) convertToIndexed(asm []AssemblyLine, start int, regContents map[string]string) []AssemblyLine {
	// Convert to indexed addressing
	base := asm[start].Operands[1]
	
	return []AssemblyLine{
		{Instruction: "LD", Operands: []string{"IX", base}},
		{Instruction: "LD", Operands: []string{asm[start+1].Operands[0], "(IX+0)"}},
		{Instruction: "LD", Operands: []string{asm[start+3].Operands[0], "(IX+1)"}},
	}
}

func (p *AddressingModePass) getIndexedSpan(asm []AssemblyLine, start int) int {
	// Return number of instructions replaced
	return 4
}

func (p *AddressingModePass) EstimateCost(asm []AssemblyLine) Cost {
	return Cost{Size: len(asm)}
}

// FinalPeepholePass performs final Z80-specific peephole optimizations
type FinalPeepholePass struct{}

func NewFinalPeepholePass() *FinalPeepholePass {
	return &FinalPeepholePass{}
}

func (p *FinalPeepholePass) Name() string {
	return "Final Peephole (ASM)"
}

func (p *FinalPeepholePass) Apply(asm []AssemblyLine) ([]AssemblyLine, bool) {
	// Run multiple passes until no changes
	totalChanged := false
	
	for {
		changed := false
		
		// Remove redundant instructions
		asm, changed = p.removeRedundant(asm)
		totalChanged = totalChanged || changed
		
		// Optimize register transfers
		var changed2 bool
		asm, changed2 = p.optimizeTransfers(asm)
		totalChanged = totalChanged || changed2
		changed = changed || changed2
		
		// Combine instructions
		var changed3 bool
		asm, changed3 = p.combineInstructions(asm)
		totalChanged = totalChanged || changed3
		changed = changed || changed3
		
		if !changed {
			break
		}
	}
	
	return asm, totalChanged
}

func (p *FinalPeepholePass) removeRedundant(asm []AssemblyLine) ([]AssemblyLine, bool) {
	changed := false
	result := []AssemblyLine{}
	
	for i := 0; i < len(asm); i++ {
		// Skip redundant loads
		if i+1 < len(asm) &&
		   asm[i].Instruction == "LD" && asm[i+1].Instruction == "LD" &&
		   len(asm[i].Operands) == 2 && len(asm[i+1].Operands) == 2 &&
		   asm[i].Operands[0] == asm[i+1].Operands[1] &&
		   asm[i].Operands[1] == asm[i+1].Operands[0] {
			// LD A, B; LD B, A -> just LD A, B
			result = append(result, asm[i])
			i++ // Skip redundant instruction
			changed = true
		} else {
			result = append(result, asm[i])
		}
	}
	
	return result, changed
}

func (p *FinalPeepholePass) optimizeTransfers(asm []AssemblyLine) ([]AssemblyLine, bool) {
	// Optimize register transfers
	return asm, false
}

func (p *FinalPeepholePass) combineInstructions(asm []AssemblyLine) ([]AssemblyLine, bool) {
	// Combine multiple instructions into single more efficient ones
	return asm, false
}

func (p *FinalPeepholePass) EstimateCost(asm []AssemblyLine) Cost {
	return Cost{Size: len(asm)}
}

// RedundantLoadStorePass removes redundant memory operations
type RedundantLoadStorePass struct{}

func NewRedundantLoadStorePass() *RedundantLoadStorePass {
	return &RedundantLoadStorePass{}
}

func (p *RedundantLoadStorePass) Name() string {
	return "Redundant Load/Store Elimination"
}

func (p *RedundantLoadStorePass) Apply(asm []AssemblyLine) ([]AssemblyLine, bool) {
	// Track memory contents
	memoryContents := make(map[string]string) // address -> register
	registerContents := make(map[string]string) // register -> value/address
	
	changed := false
	result := []AssemblyLine{}
	
	for _, line := range asm {
		redundant := false
		
		if line.Instruction == "LD" && len(line.Operands) == 2 {
			dest := line.Operands[0]
			src := line.Operands[1]
			
			// Check for redundant load
			if p.isMemoryLocation(src) {
				if reg, ok := memoryContents[src]; ok && reg == dest {
					// Loading same value into same register
					redundant = true
					changed = true
				}
			}
			
			// Update tracking
			if !redundant {
				if p.isRegister(dest) {
					registerContents[dest] = src
					if p.isMemoryLocation(src) {
						memoryContents[src] = dest
					}
				}
			}
		}
		
		if !redundant {
			result = append(result, line)
		}
		
		// Clear tracking on instructions that modify memory/registers
		if p.modifiesState(line) {
			p.clearTracking(line, memoryContents, registerContents)
		}
	}
	
	return result, changed
}

func (p *RedundantLoadStorePass) isRegister(op string) bool {
	switch op {
	case "A", "B", "C", "D", "E", "H", "L":
		return true
	}
	return false
}

func (p *RedundantLoadStorePass) isMemoryLocation(op string) bool {
	return strings.HasPrefix(op, "(") && strings.HasSuffix(op, ")")
}

func (p *RedundantLoadStorePass) modifiesState(line AssemblyLine) bool {
	switch line.Instruction {
	case "CALL", "RST", "EX", "EXX":
		return true
	}
	return false
}

func (p *RedundantLoadStorePass) clearTracking(line AssemblyLine, mem, reg map[string]string) {
	// Conservative: clear all tracking on state-modifying instructions
	for k := range mem {
		delete(mem, k)
	}
	for k := range reg {
		delete(reg, k)
	}
}

func (p *RedundantLoadStorePass) EstimateCost(asm []AssemblyLine) Cost {
	return Cost{Size: len(asm)}
}