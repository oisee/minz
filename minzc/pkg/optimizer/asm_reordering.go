package optimizer

import (
	"sort"
)

// ASMReorderingPass reorders assembly instructions for better performance
// This is different from MIR reordering because it has full knowledge of:
// - Exact Z80 instruction timings
// - Register constraints and conflicts
// - Memory access patterns
// - Flag dependencies
type ASMReorderingPass struct {
	name string
}

func NewASMReorderingPass() *ASMReorderingPass {
	return &ASMReorderingPass{
		name: "ASM Instruction Reordering",
	}
}

func (p *ASMReorderingPass) Name() string {
	return p.name
}

func (p *ASMReorderingPass) Apply(asm []AssemblyLine) ([]AssemblyLine, bool) {
	// Find reorderable regions (between labels and control flow)
	regions := p.findReorderableRegions(asm)
	
	changed := false
	result := []AssemblyLine{}
	lastEnd := 0
	
	for _, region := range regions {
		// Copy non-reorderable instructions
		result = append(result, asm[lastEnd:region.start]...)
		
		// Reorder this region
		reordered := p.reorderRegion(asm[region.start:region.end])
		if !p.sameOrder(asm[region.start:region.end], reordered) {
			changed = true
		}
		result = append(result, reordered...)
		
		lastEnd = region.end
	}
	
	// Copy remaining instructions
	result = append(result, asm[lastEnd:]...)
	
	return result, changed
}

func (p *ASMReorderingPass) EstimateCost(asm []AssemblyLine) Cost {
	cycles := 0
	for _, line := range asm {
		cycles += getZ80InstructionCycles(line)
	}
	return Cost{
		Cycles: cycles,
		Size:   len(asm),
	}
}

// ReorderableRegion represents a section of code that can be reordered
type ReorderableRegion struct {
	start int
	end   int
}

// findReorderableRegions identifies regions between labels/jumps/calls
func (p *ASMReorderingPass) findReorderableRegions(asm []AssemblyLine) []ReorderableRegion {
	regions := []ReorderableRegion{}
	start := 0
	
	for i, line := range asm {
		// End region at control flow or labels
		if p.isControlFlow(line) || line.Label != "" {
			if i > start {
				regions = append(regions, ReorderableRegion{start, i})
			}
			start = i + 1
		}
	}
	
	// Final region
	if start < len(asm) {
		regions = append(regions, ReorderableRegion{start, len(asm)})
	}
	
	return regions
}

// reorderRegion optimally reorders instructions in a region
func (p *ASMReorderingPass) reorderRegion(region []AssemblyLine) []AssemblyLine {
	if len(region) <= 1 {
		return region
	}
	
	// Build dependency graph
	deps := p.buildASMDependencyGraph(region)
	
	// Apply scheduling strategies
	scheduled := p.scheduleInstructions(region, deps)
	
	return scheduled
}

// ASMDependency tracks dependencies at assembly level
type ASMDependency struct {
	instructions []ASMInstruction
	dependencies map[int][]int // instruction -> depends on
	readAfter    map[int][]int // instruction -> read after write
	writeAfter   map[int][]int // instruction -> write after read
}

type ASMInstruction struct {
	index       int
	line        AssemblyLine
	cycles      int
	readsRegs   []string
	writesRegs  []string
	readsFlags  bool
	writesFlags bool
	readsMem    []string
	writesMem   []string
}

// buildASMDependencyGraph creates dependency graph with Z80 specifics
func (p *ASMReorderingPass) buildASMDependencyGraph(region []AssemblyLine) *ASMDependency {
	deps := &ASMDependency{
		instructions: make([]ASMInstruction, len(region)),
		dependencies: make(map[int][]int),
		readAfter:    make(map[int][]int),
		writeAfter:   make(map[int][]int),
	}
	
	// Analyze each instruction
	for i, line := range region {
		inst := ASMInstruction{
			index:  i,
			line:   line,
			cycles: getZ80InstructionCycles(line),
		}
		
		// Analyze register usage
		inst.readsRegs, inst.writesRegs = p.analyzeRegisterUsage(line)
		inst.readsFlags, inst.writesFlags = p.analyzeFlagUsage(line)
		inst.readsMem, inst.writesMem = p.analyzeMemoryUsage(line)
		
		deps.instructions[i] = inst
	}
	
	// Build dependencies
	for i := 0; i < len(deps.instructions); i++ {
		for j := i + 1; j < len(deps.instructions); j++ {
			if p.hasDependency(deps.instructions[i], deps.instructions[j]) {
				deps.dependencies[j] = append(deps.dependencies[j], i)
			}
		}
	}
	
	return deps
}

// analyzeRegisterUsage determines which registers an instruction reads/writes
func (p *ASMReorderingPass) analyzeRegisterUsage(line AssemblyLine) (reads, writes []string) {
	switch line.Instruction {
	case "LD":
		if len(line.Operands) == 2 {
			dest := line.Operands[0]
			src := line.Operands[1]
			
			// Destination is written
			if reg := p.extractRegister(dest); reg != "" {
				writes = append(writes, reg)
			}
			
			// Source is read
			if reg := p.extractRegister(src); reg != "" {
				reads = append(reads, reg)
			}
		}
		
	case "ADD", "SUB", "AND", "OR", "XOR":
		if len(line.Operands) == 2 {
			dest := line.Operands[0]
			src := line.Operands[1]
			
			// Both read and write destination
			if reg := p.extractRegister(dest); reg != "" {
				reads = append(reads, reg)
				writes = append(writes, reg)
			}
			
			// Read source
			if reg := p.extractRegister(src); reg != "" {
				reads = append(reads, reg)
			}
		}
		
	case "INC", "DEC":
		if len(line.Operands) == 1 {
			op := line.Operands[0]
			if reg := p.extractRegister(op); reg != "" {
				reads = append(reads, reg)
				writes = append(writes, reg)
			}
		}
		
	case "CP":
		// Reads operand, writes flags
		if len(line.Operands) == 1 {
			if reg := p.extractRegister(line.Operands[0]); reg != "" {
				reads = append(reads, reg)
			}
		}
		// Always reads A
		reads = append(reads, "A")
	}
	
	return reads, writes
}

// analyzeFlagUsage determines if instruction reads/writes flags
func (p *ASMReorderingPass) analyzeFlagUsage(line AssemblyLine) (reads, writes bool) {
	switch line.Instruction {
	case "ADD", "SUB", "AND", "OR", "XOR", "CP", "INC", "DEC":
		writes = true
	case "JZ", "JNZ", "JC", "JNC", "JP", "JM", "JPE", "JPO":
		reads = true
	case "ADC", "SBC":
		reads = true
		writes = true
	}
	return reads, writes
}

// analyzeMemoryUsage determines memory locations accessed
func (p *ASMReorderingPass) analyzeMemoryUsage(line AssemblyLine) (reads, writes []string) {
	for _, op := range line.Operands {
		if p.isMemoryOperand(op) {
			switch line.Instruction {
			case "LD":
				if op == line.Operands[0] {
					writes = append(writes, op)
				} else {
					reads = append(reads, op)
				}
			default:
				// Conservative: assume read
				reads = append(reads, op)
			}
		}
	}
	return reads, writes
}

// extractRegister extracts register name from operand
func (p *ASMReorderingPass) extractRegister(operand string) string {
	switch operand {
	case "A", "B", "C", "D", "E", "H", "L":
		return operand
	case "HL", "BC", "DE", "IX", "IY", "SP":
		return operand
	case "(HL)", "(BC)", "(DE)":
		// Indirect addressing also uses the register
		return operand[1 : len(operand)-1] // Remove parentheses
	}
	return ""
}

// isMemoryOperand checks if operand is memory access
func (p *ASMReorderingPass) isMemoryOperand(operand string) bool {
	return (operand[0] == '(' && operand[len(operand)-1] == ')') ||
		   (operand[0] == '$') || // Absolute address
		   (operand[0] >= '0' && operand[0] <= '9') // Numeric address
}

// hasDependency checks if inst2 depends on inst1
func (p *ASMReorderingPass) hasDependency(inst1, inst2 ASMInstruction) bool {
	// WAR (Write After Read): inst2 writes what inst1 reads
	for _, read := range inst1.readsRegs {
		for _, write := range inst2.writesRegs {
			if read == write {
				return true
			}
		}
	}
	
	// RAW (Read After Write): inst2 reads what inst1 writes
	for _, write := range inst1.writesRegs {
		for _, read := range inst2.readsRegs {
			if write == read {
				return true
			}
		}
	}
	
	// WAW (Write After Write): inst2 writes what inst1 writes
	for _, write1 := range inst1.writesRegs {
		for _, write2 := range inst2.writesRegs {
			if write1 == write2 {
				return true
			}
		}
	}
	
	// Flag dependencies
	if inst1.writesFlags && inst2.readsFlags {
		return true
	}
	
	// Memory dependencies (conservative)
	if len(inst1.writesMem) > 0 && len(inst2.readsMem) > 0 {
		return true
	}
	if len(inst1.readsMem) > 0 && len(inst2.writesMem) > 0 {
		return true
	}
	
	return false
}

// scheduleInstructions reorders based on dependencies and performance
func (p *ASMReorderingPass) scheduleInstructions(region []AssemblyLine, deps *ASMDependency) []AssemblyLine {
	// Critical path scheduling algorithm
	
	// Calculate priority for each instruction
	priorities := p.calculatePriorities(deps)
	
	// Sort by priority (highest first)
	indices := make([]int, len(region))
	for i := range indices {
		indices[i] = i
	}
	
	sort.Slice(indices, func(i, j int) bool {
		return priorities[indices[i]] > priorities[indices[j]]
	})
	
	// Schedule instructions respecting dependencies
	scheduled := make([]AssemblyLine, 0, len(region))
	ready := make([]bool, len(region))
	scheduled_set := make(map[int]bool)
	
	// Mark instructions with no dependencies as ready
	for i := range region {
		if len(deps.dependencies[i]) == 0 {
			ready[i] = true
		}
	}
	
	// Schedule loop
	for len(scheduled) < len(region) {
		// Find highest priority ready instruction
		bestIdx := -1
		bestPriority := -1
		
		for _, idx := range indices {
			if ready[idx] && !scheduled_set[idx] && priorities[idx] > bestPriority {
				bestIdx = idx
				bestPriority = priorities[idx]
			}
		}
		
		if bestIdx == -1 {
			// No ready instructions - shouldn't happen with valid dependencies
			// Fall back to original order for remaining
			for i := range region {
				if !scheduled_set[i] {
					scheduled = append(scheduled, region[i])
					scheduled_set[i] = true
				}
			}
			break
		}
		
		// Schedule this instruction
		scheduled = append(scheduled, region[bestIdx])
		scheduled_set[bestIdx] = true
		
		// Update ready status for dependent instructions
		for i := range region {
			if !ready[i] && !scheduled_set[i] {
				allDepsScheduled := true
				for _, depIdx := range deps.dependencies[i] {
					if !scheduled_set[depIdx] {
						allDepsScheduled = false
						break
					}
				}
				if allDepsScheduled {
					ready[i] = true
				}
			}
		}
	}
	
	return scheduled
}

// calculatePriorities assigns priority to each instruction
func (p *ASMReorderingPass) calculatePriorities(deps *ASMDependency) []int {
	priorities := make([]int, len(deps.instructions))
	
	// Base priority on instruction characteristics
	for i, inst := range deps.instructions {
		priority := 0
		
		// Higher priority for:
		// - Instructions with many dependents
		// - Memory operations (get them started early)
		// - Instructions on critical path
		
		// Count how many instructions depend on this one
		dependentCount := 0
		for j := range deps.instructions {
			for _, depIdx := range deps.dependencies[j] {
				if depIdx == i {
					dependentCount++
					break
				}
			}
		}
		
		priority += dependentCount * 10
		
		// Memory operations get priority
		if len(inst.readsMem) > 0 || len(inst.writesMem) > 0 {
			priority += 20
		}
		
		// Longer instructions get priority (start early)
		priority += inst.cycles
		
		priorities[i] = priority
	}
	
	return priorities
}

// Utility functions

func (p *ASMReorderingPass) isControlFlow(line AssemblyLine) bool {
	switch line.Instruction {
	case "JMP", "JP", "JR", "JZ", "JNZ", "JC", "JNC", "JPE", "JPO", "JM", "DJNZ":
		return true
	case "CALL", "RST", "RET", "RETI", "RETN":
		return true
	}
	return false
}

func (p *ASMReorderingPass) sameOrder(original, reordered []AssemblyLine) bool {
	if len(original) != len(reordered) {
		return false
	}
	
	for i := range original {
		if original[i].Instruction != reordered[i].Instruction ||
		   len(original[i].Operands) != len(reordered[i].Operands) {
			return false
		}
		
		for j := range original[i].Operands {
			if original[i].Operands[j] != reordered[i].Operands[j] {
				return false
			}
		}
	}
	
	return true
}

// getZ80InstructionCycles returns accurate Z80 cycle counts
func getZ80InstructionCycles(line AssemblyLine) int {
	switch line.Instruction {
	case "LD":
		if len(line.Operands) == 2 {
			src := line.Operands[1]
			dest := line.Operands[0]
			
			// LD reg, imm
			if isImmediate(src) && isRegister8(dest) {
				return 7
			}
			// LD reg, reg
			if isRegister8(src) && isRegister8(dest) {
				return 4
			}
			// LD reg, (HL)
			if src == "(HL)" && isRegister8(dest) {
				return 7
			}
			// LD (HL), reg
			if dest == "(HL)" && isRegister8(src) {
				return 7
			}
		}
		return 7 // Default
		
	case "ADD", "SUB", "AND", "OR", "XOR":
		return 4
		
	case "INC", "DEC":
		if len(line.Operands) == 1 {
			if line.Operands[0] == "(HL)" {
				return 11
			}
		}
		return 4
		
	case "CP":
		return 4
		
	case "JR":
		return 12 // Taken
		
	case "JP":
		return 10
		
	case "CALL":
		return 17
		
	case "RET":
		return 10
		
	default:
		return 4 // Conservative default
	}
}

func isImmediate(operand string) bool {
	return len(operand) > 0 && (operand[0] >= '0' && operand[0] <= '9' || operand[0] == '$')
}

func isRegister8(operand string) bool {
	switch operand {
	case "A", "B", "C", "D", "E", "H", "L":
		return true
	}
	return false
}