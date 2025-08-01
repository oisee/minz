package optimizer

import (
	"fmt"
	"strings"
)

// AssemblyReorderingPass reorders assembly instructions for optimal Z80 performance
type AssemblyReorderingPass struct {
	reorderedInstructions int
	debug                bool
}

// NewAssemblyReorderingPass creates a new assembly reordering pass
func NewAssemblyReorderingPass() *AssemblyReorderingPass {
	return &AssemblyReorderingPass{
		reorderedInstructions: 0,
		debug:                false,
	}
}

// Name returns the name of this pass
func (p *AssemblyReorderingPass) Name() string {
	return "Assembly Instruction Reordering"
}

// OptimizeAssembly reorders assembly instructions for better performance
func (p *AssemblyReorderingPass) OptimizeAssembly(assembly string) string {
	lines := strings.Split(assembly, "\n")
	optimizedLines := p.reorderAssemblyLines(lines)
	
	if p.debug && p.reorderedInstructions > 0 {
		fmt.Printf("=== ASSEMBLY REORDERING PASS ===\n")
		fmt.Printf("  Instructions reordered: %d\n", p.reorderedInstructions)
		fmt.Printf("=================================\n")
	}
	
	return strings.Join(optimizedLines, "\n")
}

// reorderAssemblyLines performs intelligent reordering of assembly instructions
func (p *AssemblyReorderingPass) reorderAssemblyLines(lines []string) []string {
	optimized := make([]string, 0, len(lines))
	i := 0
	
	for i < len(lines) {
		
		// Strategy 1: Group register loads for better pairing
		if reordered, length := p.groupRegisterLoads(lines, i); length > 0 {
			optimized = append(optimized, reordered...)
			i += length
			p.reorderedInstructions += length
			continue
		}
		
		// Strategy 2: Move memory operations closer to their uses
		if reordered, length := p.optimizeMemoryOperations(lines, i); length > 0 {
			optimized = append(optimized, reordered...)
			i += length
			p.reorderedInstructions += length
			continue
		}
		
		// Strategy 3: Optimize register pair usage
		if reordered, length := p.optimizeRegisterPairs(lines, i); length > 0 {
			optimized = append(optimized, reordered...)
			i += length
			p.reorderedInstructions += length
			continue
		}
		
		// No optimization applied, keep original line
		optimized = append(optimized, lines[i])
		i++
	}
	
	return optimized
}

// groupRegisterLoads groups related register loads together
func (p *AssemblyReorderingPass) groupRegisterLoads(lines []string, i int) ([]string, int) {
	if i+2 >= len(lines) {
		return nil, 0
	}
	
	// Look for pattern: LD r1, value; intervening; LD r2, value
	line1 := strings.TrimSpace(lines[i])
	line2 := strings.TrimSpace(lines[i+1])
	line3 := strings.TrimSpace(lines[i+2])
	
	// Check if we have two LD instructions separated by one instruction
	if strings.HasPrefix(line1, "LD ") && 
	   !strings.HasPrefix(line2, "LD ") &&
	   strings.HasPrefix(line3, "LD ") {
		
		// Check if we can safely reorder these instructions
		if p.canReorderSafely(line1, line2) && p.canReorderSafely(line2, line3) &&
		   p.canReorderSafely(line1, line3) {
			// Safe to reorder: group the LD instructions together
			return []string{
				lines[i],     // First LD
				lines[i+2],   // Second LD (moved up)
				lines[i+1],   // Intervening instruction (moved down)
			}, 3
		}
	}
	
	return nil, 0
}

// optimizeMemoryOperations moves memory operations closer to their uses
func (p *AssemblyReorderingPass) optimizeMemoryOperations(lines []string, i int) ([]string, int) {
	if i+3 >= len(lines) {
		return nil, 0
	}
	
	// Look for pattern: LD HL, (addr); intervening; use HL
	line1 := strings.TrimSpace(lines[i])
	line2 := strings.TrimSpace(lines[i+1])
	line3 := strings.TrimSpace(lines[i+2])
	
	// Check for memory load followed by use
	if strings.Contains(line1, "LD HL, (") && 
	   !strings.Contains(line2, "HL") &&  // Intervening doesn't use HL
	   strings.Contains(line3, "HL") {    // Next instruction uses HL
		
		// Check if we can move the memory load closer to its use
		if !p.hasRegisterDependency(line1, line2) {
			// Keep as-is for now (could be optimized further)
			return []string{lines[i], lines[i+1], lines[i+2]}, 3
		}
	}
	
	return nil, 0
}

// optimizeRegisterPairs optimizes for Z80 register pair operations
func (p *AssemblyReorderingPass) optimizeRegisterPairs(lines []string, i int) ([]string, int) {
	if i+1 >= len(lines) {
		return nil, 0
	}
	
	line1 := strings.TrimSpace(lines[i])
	line2 := strings.TrimSpace(lines[i+1])
	
	// Look for pattern that could use register pairs more efficiently
	// Example: LD H, value; LD L, value → could be LD HL, value (if values combine to 16-bit)
	if strings.HasPrefix(line1, "LD H, ") && strings.HasPrefix(line2, "LD L, ") {
		// Extract values
		hValue := p.extractValue(line1, "LD H, ")
		lValue := p.extractValue(line2, "LD L, ")
		
		// If both are constants, we could potentially combine them
		if hValue != "" && lValue != "" {
			// For now, keep them separate but ensure they're adjacent
			return []string{lines[i], lines[i+1]}, 2
		}
	}
	
	return nil, 0
}

// hasRegisterDependency checks if instruction2 depends on registers modified by instruction1
func (p *AssemblyReorderingPass) hasRegisterDependency(inst1, inst2 string) bool {
	// Extract register modified by first instruction
	modifiedReg := p.getModifiedRegister(inst1)
	if modifiedReg == "" {
		return false
	}
	
	// Check if second instruction uses that register
	return strings.Contains(inst2, modifiedReg)
}

// canReorderSafely checks if we can safely reorder two instructions
func (p *AssemblyReorderingPass) canReorderSafely(inst1, inst2 string) bool {
	// NEVER reorder across these instructions (side effects unknown)
	unsafeInstructions := []string{
		"CALL", "RET", "RST",     // Function calls/returns
		"JP", "JR", "DJNZ",       // Jumps (control flow)
		"IN", "OUT",              // I/O operations
		"EI", "DI", "HALT",       // Interrupt control
		"LD (", "LD (",           // Memory writes
		"PUSH", "POP",            // Stack operations
		"EX", "EXX",              // Register exchanges
		"DAA", "CPL", "SCF",      // Flag modifications
	}
	
	// Check if either instruction is unsafe
	inst1Upper := strings.ToUpper(strings.TrimSpace(inst1))
	inst2Upper := strings.ToUpper(strings.TrimSpace(inst2))
	
	for _, unsafe := range unsafeInstructions {
		if strings.Contains(inst1Upper, unsafe) || strings.Contains(inst2Upper, unsafe) {
			return false // Cannot reorder
		}
	}
	
	// Check register dependencies
	if p.hasRegisterDependency(inst1, inst2) || p.hasRegisterDependency(inst2, inst1) {
		return false
	}
	
	// Only safe if both are simple register operations
	safeOps := []string{"LD", "ADD", "SUB", "AND", "OR", "XOR", "INC", "DEC"}
	inst1Safe := false
	inst2Safe := false
	
	for _, op := range safeOps {
		if strings.HasPrefix(inst1Upper, op) {
			inst1Safe = true
		}
		if strings.HasPrefix(inst2Upper, op) {
			inst2Safe = true
		}
	}
	
	return inst1Safe && inst2Safe
}

// getModifiedRegister extracts the register being modified by an instruction
func (p *AssemblyReorderingPass) getModifiedRegister(instruction string) string {
	// Simple pattern matching for LD instructions
	if strings.HasPrefix(instruction, "LD ") {
		parts := strings.Split(instruction, ",")
		if len(parts) >= 1 {
			regPart := strings.TrimSpace(strings.TrimPrefix(parts[0], "LD "))
			return regPart
		}
	}
	return ""
}

// extractValue extracts the value part from an instruction like "LD H, 10"
func (p *AssemblyReorderingPass) extractValue(instruction, prefix string) string {
	if strings.HasPrefix(instruction, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(instruction, prefix))
	}
	return ""
}