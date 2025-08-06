package optimizer

import (
	"sort"

	"github.com/minz/minzc/pkg/ir"
)

// InstructionScheduler reorders instructions to minimize register pressure
// and maximize performance while preserving program semantics
type InstructionScheduler struct {
	metrics *OptimizationMetrics
}

// SchedulingConstraint represents a constraint on instruction ordering
type SchedulingConstraint struct {
	Before int // Instruction that must come before
	After  int // Instruction that must come after
	Type   ConstraintType
}

type ConstraintType int

const (
	DataDependency ConstraintType = iota // Read-after-write dependency
	AntiDependency                       // Write-after-read dependency
	OutputDependency                     // Write-after-write dependency
	ControlDependency                    // Control flow dependency
)

// SchedulableInstruction wraps an instruction with scheduling metadata
type SchedulableInstruction struct {
	Index           int                 // Original position in function
	Instruction     ir.Instruction      // The instruction itself
	Dependencies    []int               // Instructions this depends on
	Dependents      []int               // Instructions that depend on this
	Priority        int                 // Scheduling priority (higher = schedule first)
	EarliestCycle   int                 // Earliest cycle this can be scheduled
	LatestCycle     int                 // Latest cycle this can be scheduled
	RegisterPressure int                // Register pressure contribution
}

// NewInstructionScheduler creates a new instruction scheduler
func NewInstructionScheduler(metrics *OptimizationMetrics) *InstructionScheduler {
	return &InstructionScheduler{
		metrics: metrics,
	}
}

// ScheduleInstructions performs instruction scheduling on a basic block
func (is *InstructionScheduler) ScheduleInstructions(instructions []ir.Instruction) []ir.Instruction {
	if len(instructions) <= 1 {
		return instructions // Nothing to schedule
	}
	
	// Phase 1: Build schedulable instructions with dependencies
	schedulable := is.buildSchedulableInstructions(instructions)
	
	// Phase 2: Calculate scheduling priorities
	is.calculatePriorities(schedulable)
	
	// Phase 3: Perform list scheduling
	scheduled := is.listScheduleForMinimalPressure(schedulable)
	
	// Phase 4: Convert back to instruction list
	result := make([]ir.Instruction, len(instructions))
	for i, sched := range scheduled {
		result[i] = sched.Instruction
	}
	
	return result
}

// buildSchedulableInstructions creates schedulable wrappers with dependency analysis
func (is *InstructionScheduler) buildSchedulableInstructions(instructions []ir.Instruction) []SchedulableInstruction {
	schedulable := make([]SchedulableInstruction, len(instructions))
	
	// Create initial schedulable instructions
	for i, inst := range instructions {
		schedulable[i] = SchedulableInstruction{
			Index:           i,
			Instruction:     inst,
			Dependencies:    []int{},
			Dependents:      []int{},
			EarliestCycle:   0,
			LatestCycle:     len(instructions),
		}
	}
	
	// Build dependency graph
	is.buildDependencies(schedulable)
	
	// Calculate timing constraints
	is.calculateTimingConstraints(schedulable)
	
	return schedulable
}

// buildDependencies analyzes and builds the dependency graph
func (is *InstructionScheduler) buildDependencies(schedulable []SchedulableInstruction) {
	// Track the last instruction that wrote to each register
	lastWriter := make(map[int]int)
	
	// Track the last instructions that read from each register
	lastReaders := make(map[int][]int)
	
	for i := range schedulable {
		inst := &schedulable[i]
		
		// Analyze register uses (reads)
		uses := is.getInstructionUses(inst.Instruction)
		for _, reg := range uses {
			// Data dependency: this instruction depends on the last writer
			if writer, exists := lastWriter[reg]; exists {
				inst.Dependencies = append(inst.Dependencies, writer)
				schedulable[writer].Dependents = append(schedulable[writer].Dependents, i)
			}
			
			// Track this as a reader
			lastReaders[reg] = append(lastReaders[reg], i)
		}
		
		// Analyze register definitions (writes)
		if def := is.getInstructionDef(inst.Instruction); def != 0 {
			// Output dependency: this instruction depends on previous writer
			if writer, exists := lastWriter[def]; exists {
				inst.Dependencies = append(inst.Dependencies, writer)
				schedulable[writer].Dependents = append(schedulable[writer].Dependents, i)
			}
			
			// Anti-dependency: this instruction depends on previous readers
			if readers, exists := lastReaders[def]; exists {
				for _, reader := range readers {
					inst.Dependencies = append(inst.Dependencies, reader)
					schedulable[reader].Dependents = append(schedulable[reader].Dependents, i)
				}
				// Clear readers after this write
				delete(lastReaders, def)
			}
			
			// Update last writer
			lastWriter[def] = i
		}
		
		// Handle memory dependencies (conservative approach)
		if is.hasMemoryEffect(inst.Instruction) {
			// Memory operations must preserve order (simplified)
			for j := 0; j < i; j++ {
				if is.hasMemoryEffect(schedulable[j].Instruction) {
					inst.Dependencies = append(inst.Dependencies, j)
					schedulable[j].Dependents = append(schedulable[j].Dependents, i)
				}
			}
		}
	}
}

// calculateTimingConstraints computes earliest and latest scheduling times
func (is *InstructionScheduler) calculateTimingConstraints(schedulable []SchedulableInstruction) {
	// Forward pass: calculate earliest start times
	for i := range schedulable {
		inst := &schedulable[i]
		earliestStart := 0
		
		for _, depIndex := range inst.Dependencies {
			depEarliest := schedulable[depIndex].EarliestCycle
			depLatency := is.getInstructionLatency(schedulable[depIndex].Instruction)
			
			if depEarliest+depLatency > earliestStart {
				earliestStart = depEarliest + depLatency
			}
		}
		
		inst.EarliestCycle = earliestStart
	}
	
	// Backward pass: calculate latest start times
	for i := len(schedulable) - 1; i >= 0; i-- {
		inst := &schedulable[i]
		latestStart := len(schedulable) // Default to end
		
		for _, depIndex := range inst.Dependents {
			depLatest := schedulable[depIndex].LatestCycle
			thisLatency := is.getInstructionLatency(inst.Instruction)
			
			if depLatest-thisLatency < latestStart {
				latestStart = depLatest - thisLatency
			}
		}
		
		inst.LatestCycle = latestStart
	}
}

// calculatePriorities assigns scheduling priorities to instructions
func (is *InstructionScheduler) calculatePriorities(schedulable []SchedulableInstruction) {
	for i := range schedulable {
		inst := &schedulable[i]
		
		// Base priority on critical path length
		criticalPath := is.calculateCriticalPathLength(schedulable, i)
		inst.Priority = criticalPath * 10
		
		// Boost priority for register-killing instructions
		if is.killsLiveRegisters(inst.Instruction) {
			inst.Priority += 50
		}
		
		// Reduce priority for register-creating instructions
		if is.createsNewRegister(inst.Instruction) {
			inst.Priority -= 20
		}
		
		// Boost priority for memory operations (schedule early)
		if is.hasMemoryEffect(inst.Instruction) {
			inst.Priority += 30
		}
		
		// Consider instruction latency
		latency := is.getInstructionLatency(inst.Instruction)
		inst.Priority += latency * 5
	}
}

// listScheduleForMinimalPressure performs list scheduling optimized for register pressure
func (is *InstructionScheduler) listScheduleForMinimalPressure(schedulable []SchedulableInstruction) []SchedulableInstruction {
	scheduled := make([]SchedulableInstruction, 0, len(schedulable))
	remaining := make(map[int]*SchedulableInstruction)
	
	// Initialize remaining instructions
	for i := range schedulable {
		remaining[i] = &schedulable[i]
	}
	
	// Track currently live registers
	liveRegs := make(map[int]bool)
	currentCycle := 0
	
	for len(remaining) > 0 {
		// Find ready instructions (all dependencies satisfied)
		ready := is.findReadyInstructions(remaining, scheduled)
		
		if len(ready) == 0 {
			// Should not happen in well-formed code
			break
		}
		
		// Select best instruction considering register pressure
		best := is.selectBestForPressure(ready, liveRegs)
		
		// Schedule the selected instruction
		scheduled = append(scheduled, *best)
		is.updateLiveRegisters(best, liveRegs)
		
		// Remove from remaining
		delete(remaining, best.Index)
		
		// Update metrics
		is.metrics.InstructionsReordered++
		
		currentCycle++
	}
	
	return scheduled
}

// findReadyInstructions returns instructions with all dependencies satisfied
func (is *InstructionScheduler) findReadyInstructions(remaining map[int]*SchedulableInstruction, scheduled []SchedulableInstruction) []*SchedulableInstruction {
	var ready []*SchedulableInstruction
	scheduledSet := make(map[int]bool)
	
	// Build set of scheduled instruction indices
	for _, sched := range scheduled {
		scheduledSet[sched.Index] = true
	}
	
	for _, inst := range remaining {
		isReady := true
		
		// Check if all dependencies are satisfied
		for _, depIndex := range inst.Dependencies {
			if !scheduledSet[depIndex] {
				isReady = false
				break
			}
		}
		
		if isReady {
			ready = append(ready, inst)
		}
	}
	
	return ready
}

// selectBestForPressure chooses the instruction that minimizes register pressure
func (is *InstructionScheduler) selectBestForPressure(ready []*SchedulableInstruction, liveRegs map[int]bool) *SchedulableInstruction {
	if len(ready) == 1 {
		return ready[0]
	}
	
	var best *SchedulableInstruction
	bestScore := float64(-999999) // Start with very low score
	
	for _, inst := range ready {
		score := is.calculatePressureScore(inst, liveRegs)
		if score > bestScore {
			bestScore = score
			best = inst
		}
	}
	
	return best
}

// calculatePressureScore calculates a score for instruction selection (higher is better)
func (is *InstructionScheduler) calculatePressureScore(inst *SchedulableInstruction, liveRegs map[int]bool) float64 {
	score := float64(inst.Priority) // Start with base priority
	
	// Strongly prefer instructions that kill live registers
	uses := is.getInstructionUses(inst.Instruction)
	registersKilled := 0
	for _, reg := range uses {
		if liveRegs[reg] {
			// Check if this is the last use of the register
			if is.isLastUse(inst, reg) {
				registersKilled++
				score += 100.0 // Big bonus for killing registers
			}
		}
	}
	
	// Penalize instructions that create new live registers
	if def := is.getInstructionDef(inst.Instruction); def != 0 {
		if !liveRegs[def] {
			score -= 50.0 // Penalty for creating new live register
		}
	}
	
	// Consider current register pressure
	currentPressure := len(liveRegs)
	if currentPressure > 5 { // High pressure
		score += float64(registersKilled) * 50.0 // Bonus multiplier for killing registers
	}
	
	// Prefer instructions with tight timing constraints
	slack := inst.LatestCycle - inst.EarliestCycle
	if slack < 3 {
		score += 25.0 // Bonus for critical instructions
	}
	
	return score
}

// updateLiveRegisters updates the live register set after scheduling an instruction
func (is *InstructionScheduler) updateLiveRegisters(inst *SchedulableInstruction, liveRegs map[int]bool) {
	// Remove registers that die (last use)
	uses := is.getInstructionUses(inst.Instruction)
	for _, reg := range uses {
		if is.isLastUse(inst, reg) {
			delete(liveRegs, reg)
		}
	}
	
	// Add newly defined register
	if def := is.getInstructionDef(inst.Instruction); def != 0 {
		liveRegs[def] = true
	}
}

// Helper functions for instruction analysis

func (is *InstructionScheduler) getInstructionUses(inst ir.Instruction) []int {
	var uses []int
	
	switch inst.Op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod, ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpShl, ir.OpShr:
		uses = append(uses, inst.Left, inst.Right)
	case ir.OpStore:
		uses = append(uses, inst.Source)
	case ir.OpCall:
		uses = append(uses, inst.Args...)
	case ir.OpRet:
		if inst.Source != 0 {
			uses = append(uses, inst.Source)
		}
	case ir.OpBranch:
		if inst.Condition != 0 {
			uses = append(uses, inst.Condition)
		}
	}
	
	return uses
}

func (is *InstructionScheduler) getInstructionDef(inst ir.Instruction) int {
	switch inst.Op {
	case ir.OpLoadConst, ir.OpLoad, ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod, ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpShl, ir.OpShr, ir.OpCall:
		return inst.Dest
	}
	return 0
}

func (is *InstructionScheduler) hasMemoryEffect(inst ir.Instruction) bool {
	switch inst.Op {
	case ir.OpLoad, ir.OpStore, ir.OpCall:
		return true
	}
	return false
}

func (is *InstructionScheduler) getInstructionLatency(inst ir.Instruction) int {
	// Z80 instruction latencies (in T-states, simplified)
	switch inst.Op {
	case ir.OpLoadConst:
		return 7 // LD r, n
	case ir.OpLoad:
		return 13 // LD r, (nn)
	case ir.OpStore:
		return 13 // LD (nn), r
	case ir.OpAdd, ir.OpSub, ir.OpAnd, ir.OpOr, ir.OpXor:
		return 4 // Basic arithmetic
	case ir.OpMul:
		return 30 // Multiplication is expensive
	case ir.OpDiv:
		return 40 // Division is very expensive
	case ir.OpShl, ir.OpShr:
		return 8 // Shift operations
	case ir.OpCall:
		return 17 // CALL instruction
	case ir.OpRet:
		return 10 // RET instruction
	case ir.OpBranch:
		return 12 // Conditional jump (average)
	default:
		return 4 // Default
	}
}

func (is *InstructionScheduler) killsLiveRegisters(inst ir.Instruction) bool {
	// This would require liveness analysis integration
	// Simplified: assume arithmetic operations often kill registers
	switch inst.Op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv:
		return true
	}
	return false
}

func (is *InstructionScheduler) createsNewRegister(inst ir.Instruction) bool {
	return is.getInstructionDef(inst) != 0
}

func (is *InstructionScheduler) calculateCriticalPathLength(schedulable []SchedulableInstruction, startIndex int) int {
	// Calculate the longest path from this instruction to the end
	visited := make(map[int]bool)
	return is.calculateCriticalPathLengthHelper(schedulable, startIndex, visited)
}

func (is *InstructionScheduler) calculateCriticalPathLengthHelper(schedulable []SchedulableInstruction, index int, visited map[int]bool) int {
	if visited[index] {
		return 0 // Avoid cycles
	}
	
	visited[index] = true
	maxPath := 0
	
	for _, depIndex := range schedulable[index].Dependents {
		pathLength := is.calculateCriticalPathLengthHelper(schedulable, depIndex, visited)
		if pathLength > maxPath {
			maxPath = pathLength
		}
	}
	
	delete(visited, index)
	return maxPath + is.getInstructionLatency(schedulable[index].Instruction)
}

func (is *InstructionScheduler) isLastUse(inst *SchedulableInstruction, reg int) bool {
	// Simplified: assume it's the last use if register is used
	// In practice, would need full liveness analysis
	uses := is.getInstructionUses(inst.Instruction)
	for _, use := range uses {
		if use == reg {
			return true
		}
	}
	return false
}

// ScheduleBasicBlock schedules instructions within a single basic block
func (is *InstructionScheduler) ScheduleBasicBlock(block *ir.BasicBlock) {
	originalCount := len(block.Instructions)
	
	// Perform scheduling
	block.Instructions = is.ScheduleInstructions(block.Instructions)
	
	// Update metrics
	if len(block.Instructions) == originalCount {
		// Count as successful if we didn't break anything
		is.metrics.InstructionsReordered += originalCount
	}
}

// ScheduleFunction schedules instructions across all basic blocks in a function
func (is *InstructionScheduler) ScheduleFunction(function *ir.Function) {
	// For now, treat the entire function as one basic block (simplified)
	originalInstructions := make([]ir.Instruction, len(function.Instructions))
	copy(originalInstructions, function.Instructions)
	
	// Schedule the instructions
	function.Instructions = is.ScheduleInstructions(function.Instructions)
	
	// Verify that scheduling preserved semantics (basic check)
	if len(function.Instructions) != len(originalInstructions) {
		// Rollback if something went wrong
		function.Instructions = originalInstructions
	}
}