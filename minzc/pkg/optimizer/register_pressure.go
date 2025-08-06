package optimizer

import (
	"fmt"
	"sort"

	"github.com/minz/minzc/pkg/ir"
)

// RegisterPressureOptimizer minimizes register pressure through instruction scheduling
type RegisterPressureOptimizer struct {
	liveness    map[int]LiveRange  // Register -> live range
	spillCosts  map[int]float64    // Register -> spill cost
	metrics     *OptimizationMetrics
}

// LiveRange represents the live range of a register
type LiveRange struct {
	Start       int     // Instruction index where variable is defined
	End         int     // Instruction index where variable is last used
	Register    int     // Virtual register number
	Frequency   int     // Usage frequency
	LoopDepth   int     // Nesting level in loops
	SpillCost   float64 // Cost of spilling this register
}

// DependencyGraph represents instruction dependencies
type DependencyGraph struct {
	Instructions []ir.Instruction
	Dependencies map[int][]int  // Instruction index -> list of dependencies
	ReadySet     []int          // Instructions with no dependencies
}

// OptimizationMetrics tracks register pressure optimization statistics
type OptimizationMetrics struct {
	// Register pressure metrics
	OriginalMaxPressure  int
	OptimizedMaxPressure int
	SpillsEliminated     int
	InstructionsReordered int
	
	// Performance improvements
	CyclesSaved   int
	BytesSaved    int
	
	// Optimization details
	LiveRangesSplit      int
	RematerializedValues int
	RegisterResetPoints  int
}

// NewRegisterPressureOptimizer creates a new register pressure optimizer
func NewRegisterPressureOptimizer() *RegisterPressureOptimizer {
	return &RegisterPressureOptimizer{
		liveness:   make(map[int]LiveRange),
		spillCosts: make(map[int]float64),
		metrics:    &OptimizationMetrics{},
	}
}

// OptimizeFunction performs register pressure optimization on a function
func (rpo *RegisterPressureOptimizer) OptimizeFunction(function *ir.Function) {
	// Phase 1: Analyze current register pressure
	originalPressure := rpo.analyzePressure(function)
	rpo.metrics.OriginalMaxPressure = originalPressure.MaxSimultaneous
	
	// Phase 2: Build live ranges
	rpo.buildLiveRanges(function)
	
	// Phase 3: Find high-pressure regions
	criticalRegions := rpo.findCriticalRegions(function)
	
	// Phase 4: Apply pressure-reducing optimizations
	for _, region := range criticalRegions {
		rpo.reducePressureInRegion(function, region)
	}
	
	// Phase 5: Measure improvement
	optimizedPressure := rpo.analyzePressure(function)
	rpo.metrics.OptimizedMaxPressure = optimizedPressure.MaxSimultaneous
	rpo.metrics.SpillsEliminated = originalPressure.SpillCount - optimizedPressure.SpillCount
	
	// Calculate performance savings
	rpo.calculatePerformanceSavings(originalPressure, optimizedPressure)
}

// RegisterPressureInfo contains pressure analysis results
type RegisterPressureInfo struct {
	MaxSimultaneous int          // Peak number of live registers
	SpillCount      int          // Estimated number of spills needed
	PressureByInst  []int        // Pressure at each instruction
	CriticalPath    []InstRange  // High-pressure instruction ranges
}

// InstRange represents a range of instructions
type InstRange struct {
	Start    int // Starting instruction index
	End      int // Ending instruction index
	Pressure int // Register pressure in this range
}

// analyzePressure analyzes register pressure throughout the function
func (rpo *RegisterPressureOptimizer) analyzePressure(function *ir.Function) RegisterPressureInfo {
	live := make(map[int]bool) // Currently live registers
	pressureByInst := make([]int, len(function.Instructions))
	maxPressure := 0
	
	for i, inst := range function.Instructions {
		// Remove registers that die at this instruction
		rpo.updateDeadRegisters(inst, live, i)
		
		// Add registers that are born at this instruction  
		rpo.updateLiveRegisters(inst, live)
		
		// Record pressure at this point
		currentPressure := len(live)
		pressureByInst[i] = currentPressure
		
		if currentPressure > maxPressure {
			maxPressure = currentPressure
		}
	}
	
	// Find critical high-pressure regions
	criticalPath := rpo.findHighPressureRegions(pressureByInst)
	
	// Estimate spills needed (Z80 has ~7 general-purpose registers)
	spillCount := 0
	if maxPressure > 7 {
		spillCount = maxPressure - 7
	}
	
	return RegisterPressureInfo{
		MaxSimultaneous: maxPressure,
		SpillCount:      spillCount,
		PressureByInst:  pressureByInst,
		CriticalPath:    criticalPath,
	}
}

// buildLiveRanges constructs live ranges for all registers
func (rpo *RegisterPressureOptimizer) buildLiveRanges(function *ir.Function) {
	rpo.liveness = make(map[int]LiveRange)
	
	// Forward pass: find definitions
	for i, inst := range function.Instructions {
		if inst.Dest != 0 {
			// This instruction defines a register
			liveRange := LiveRange{
				Start:     i,
				End:       i, // Will be updated in backward pass
				Register:  inst.Dest,
				Frequency: 1,
				LoopDepth: rpo.getLoopDepth(function, i),
			}
			rpo.liveness[inst.Dest] = liveRange
		}
	}
	
	// Backward pass: find last uses
	for i := len(function.Instructions) - 1; i >= 0; i-- {
		inst := function.Instructions[i]
		
		// Check all register uses in this instruction
		usedRegs := rpo.getUsedRegisters(inst)
		for _, reg := range usedRegs {
			if liveRange, exists := rpo.liveness[reg]; exists {
				if i > liveRange.End {
					liveRange.End = i
					liveRange.Frequency++
					rpo.liveness[reg] = liveRange
				}
			}
		}
	}
	
	// Calculate spill costs
	rpo.calculateSpillCosts()
}

// findCriticalRegions identifies instruction ranges with high register pressure
func (rpo *RegisterPressureOptimizer) findCriticalRegions(function *ir.Function) []InstRange {
	pressureInfo := rpo.analyzePressure(function)
	return pressureInfo.CriticalPath
}

// reducePressureInRegion applies optimizations to reduce pressure in a region
func (rpo *RegisterPressureOptimizer) reducePressureInRegion(function *ir.Function, region InstRange) {
	// Strategy 1: Instruction scheduling
	rpo.scheduleInstructionsForMinimalPressure(function, region)
	
	// Strategy 2: Live range splitting at call boundaries
	rpo.splitLiveRangesAtCalls(function, region)
	
	// Strategy 3: Rematerialization of cheap values
	rpo.rematerializeInsteadOfSpill(function, region)
	
	// Strategy 4: Insert register reset points
	rpo.insertRegisterResetPoints(function, region)
}

// scheduleInstructionsForMinimalPressure reorders instructions to minimize pressure
func (rpo *RegisterPressureOptimizer) scheduleInstructionsForMinimalPressure(function *ir.Function, region InstRange) {
	// Extract instructions in the region
	regionInsts := function.Instructions[region.Start:region.End+1]
	
	// Build dependency graph
	depGraph := rpo.buildDependencyGraph(regionInsts)
	
	// Schedule instructions using list scheduling algorithm
	scheduled := rpo.listScheduling(depGraph)
	
	// Replace instructions in the function
	copy(function.Instructions[region.Start:region.End+1], scheduled)
	
	rpo.metrics.InstructionsReordered += len(scheduled)
}

// buildDependencyGraph constructs a dependency graph for instructions
func (rpo *RegisterPressureOptimizer) buildDependencyGraph(instructions []ir.Instruction) *DependencyGraph {
	graph := &DependencyGraph{
		Instructions: instructions,
		Dependencies: make(map[int][]int),
		ReadySet:     []int{},
	}
	
	// Track which instruction defines each register
	definitions := make(map[int]int) // Register -> defining instruction index
	
	// Build dependencies
	for i, inst := range instructions {
		var deps []int
		
		// Add dependencies for used registers
		usedRegs := rpo.getUsedRegisters(inst)
		for _, reg := range usedRegs {
			if defInst, exists := definitions[reg]; exists {
				deps = append(deps, defInst)
			}
		}
		
		graph.Dependencies[i] = deps
		
		// Record this instruction as defining its destination register
		if inst.Dest != 0 {
			definitions[inst.Dest] = i
		}
		
		// Instructions with no dependencies are ready
		if len(deps) == 0 {
			graph.ReadySet = append(graph.ReadySet, i)
		}
	}
	
	return graph
}

// listScheduling implements the list scheduling algorithm for minimal register pressure
func (rpo *RegisterPressureOptimizer) listScheduling(graph *DependencyGraph) []ir.Instruction {
	scheduled := make([]ir.Instruction, 0, len(graph.Instructions))
	ready := make([]int, len(graph.ReadySet))
	copy(ready, graph.ReadySet)
	
	live := make(map[int]bool) // Currently live registers
	
	for len(scheduled) < len(graph.Instructions) {
		if len(ready) == 0 {
			// This shouldn't happen in a well-formed dependency graph
			break
		}
		
		// Select the best instruction to schedule next
		bestIdx := rpo.selectBestInstruction(ready, graph.Instructions, live)
		selectedInst := ready[bestIdx]
		
		// Remove from ready list
		ready = append(ready[:bestIdx], ready[bestIdx+1:]...)
		
		// Add to schedule
		scheduled = append(scheduled, graph.Instructions[selectedInst])
		
		// Update live set
		rpo.updateLiveSetForInstruction(graph.Instructions[selectedInst], live)
		
		// Add newly ready instructions
		for i, inst := range graph.Instructions {
			if rpo.isReadyAfterScheduling(i, selectedInst, scheduled, graph) {
				ready = append(ready, i)
			}
		}
	}
	
	return scheduled
}

// selectBestInstruction chooses the instruction that minimizes register pressure
func (rpo *RegisterPressureOptimizer) selectBestInstruction(ready []int, instructions []ir.Instruction, live map[int]bool) int {
	bestIdx := 0
	bestScore := rpo.calculatePressureScore(instructions[ready[0]], live)
	
	for i := 1; i < len(ready); i++ {
		score := rpo.calculatePressureScore(instructions[ready[i]], live)
		if score < bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	
	return bestIdx
}

// calculatePressureScore calculates a score for instruction selection (lower is better)
func (rpo *RegisterPressureOptimizer) calculatePressureScore(inst ir.Instruction, live map[int]bool) float64 {
	score := 0.0
	
	// Prefer instructions that kill live registers
	usedRegs := rpo.getUsedRegisters(inst)
	for _, reg := range usedRegs {
		if live[reg] {
			score -= 10.0 // Negative score for killing live registers
		}
	}
	
	// Penalize instructions that create new live registers
	if inst.Dest != 0 && !live[inst.Dest] {
		score += 5.0
	}
	
	// Consider spill costs
	if liveRange, exists := rpo.liveness[inst.Dest]; exists {
		score += liveRange.SpillCost * 0.1
	}
	
	return score
}

// splitLiveRangesAtCalls splits long live ranges at function call boundaries
func (rpo *RegisterPressureOptimizer) splitLiveRangesAtCalls(function *ir.Function, region InstRange) {
	for i := region.Start; i <= region.End; i++ {
		inst := function.Instructions[i]
		
		if inst.Op == ir.OpCall {
			// This is a natural point to split live ranges
			rpo.insertSpillCodeAtCall(function, i)
			rpo.metrics.LiveRangesSplit++
		}
	}
}

// insertSpillCodeAtCall inserts spill/reload code around function calls
func (rpo *RegisterPressureOptimizer) insertSpillCodeAtCall(function *ir.Function, callIndex int) {
	// Find live registers at this point
	liveRegs := rpo.findLiveRegistersAtPoint(function, callIndex)
	
	// For each live register with high spill cost, consider spilling
	for reg, liveRange := range rpo.liveness {
		if rpo.isLiveAtPoint(liveRange, callIndex) && liveRange.SpillCost < 5.0 {
			// Insert spill before call and reload after
			spillInst := ir.Instruction{
				Op:     ir.OpStore,
				Source: reg,
				Target: fmt.Sprintf("spill_%d", reg),
				Comment: "Register pressure spill",
			}
			
			reloadInst := ir.Instruction{
				Op:    ir.OpLoad,
				Dest:  reg,
				Source: fmt.Sprintf("spill_%d", reg),
				Comment: "Register pressure reload",
			}
			
			// Insert instructions (simplified - would need proper insertion logic)
			rpo.insertInstructionBefore(function, callIndex, spillInst)
			rpo.insertInstructionAfter(function, callIndex+1, reloadInst)
		}
	}
}

// rematerializeInsteadOfSpill rematerializes cheap values instead of spilling
func (rpo *RegisterPressureOptimizer) rematerializeInsteadOfSpill(function *ir.Function, region InstRange) {
	for reg, liveRange := range rpo.liveness {
		if rpo.isInRegion(liveRange, region) && rpo.isCheapToRematerialize(function, reg) {
			// Mark for rematerialization instead of spilling
			rpo.markForRematerialization(function, reg)
			rpo.metrics.RematerializedValues++
		}
	}
}

// insertRegisterResetPoints inserts points where register allocation can restart
func (rpo *RegisterPressureOptimizer) insertRegisterResetPoints(function *ir.Function, region InstRange) {
	for i := region.Start; i <= region.End; i++ {
		inst := function.Instructions[i]
		
		// Function calls are natural reset points
		if inst.Op == ir.OpCall && !rpo.functionResultIsUsed(function, i) {
			rpo.insertRegisterResetPoint(function, i+1)
			rpo.metrics.RegisterResetPoints++
		}
	}
}

// Helper functions

func (rpo *RegisterPressureOptimizer) updateDeadRegisters(inst ir.Instruction, live map[int]bool, instIndex int) {
	// Check if any live registers die at this instruction
	for reg := range live {
		if liveRange, exists := rpo.liveness[reg]; exists {
			if liveRange.End == instIndex {
				delete(live, reg)
			}
		}
	}
}

func (rpo *RegisterPressureOptimizer) updateLiveRegisters(inst ir.Instruction, live map[int]bool) {
	if inst.Dest != 0 {
		live[inst.Dest] = true
	}
}

func (rpo *RegisterPressureOptimizer) getUsedRegisters(inst ir.Instruction) []int {
	var used []int
	
	switch inst.Op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod:
		used = append(used, inst.Left, inst.Right)
	case ir.OpStore:
		used = append(used, inst.Source)
	case ir.OpCall:
		used = append(used, inst.Args...)
	case ir.OpRet:
		if inst.Source != 0 {
			used = append(used, inst.Source)
		}
	}
	
	return used
}

func (rpo *RegisterPressureOptimizer) getLoopDepth(function *ir.Function, instIndex int) int {
	// Simplified - would need proper loop analysis
	return 0
}

func (rpo *RegisterPressureOptimizer) calculateSpillCosts() {
	for reg, liveRange := range rpo.liveness {
		// Cost = (frequency * loop_depth^2) / live_range_length
		loopFactor := float64(liveRange.LoopDepth*liveRange.LoopDepth + 1)
		rangeLength := float64(liveRange.End - liveRange.Start + 1)
		
		cost := (float64(liveRange.Frequency) * loopFactor) / rangeLength
		rpo.spillCosts[reg] = cost
		
		// Update the live range with the calculated cost
		liveRange.SpillCost = cost
		rpo.liveness[reg] = liveRange
	}
}

func (rpo *RegisterPressureOptimizer) findHighPressureRegions(pressureByInst []int) []InstRange {
	var regions []InstRange
	threshold := 5 // High pressure threshold
	
	inRegion := false
	regionStart := 0
	
	for i, pressure := range pressureByInst {
		if pressure >= threshold && !inRegion {
			// Start of high-pressure region
			regionStart = i
			inRegion = true
		} else if pressure < threshold && inRegion {
			// End of high-pressure region
			regions = append(regions, InstRange{
				Start:    regionStart,
				End:      i - 1,
				Pressure: rpo.maxPressureInRange(pressureByInst, regionStart, i-1),
			})
			inRegion = false
		}
	}
	
	// Handle region that extends to end
	if inRegion {
		regions = append(regions, InstRange{
			Start:    regionStart,
			End:      len(pressureByInst) - 1,
			Pressure: rpo.maxPressureInRange(pressureByInst, regionStart, len(pressureByInst)-1),
		})
	}
	
	return regions
}

func (rpo *RegisterPressureOptimizer) maxPressureInRange(pressureByInst []int, start, end int) int {
	max := 0
	for i := start; i <= end && i < len(pressureByInst); i++ {
		if pressureByInst[i] > max {
			max = pressureByInst[i]
		}
	}
	return max
}

func (rpo *RegisterPressureOptimizer) updateLiveSetForInstruction(inst ir.Instruction, live map[int]bool) {
	// Remove used registers (they die)
	usedRegs := rpo.getUsedRegisters(inst)
	for _, reg := range usedRegs {
		if liveRange, exists := rpo.liveness[reg]; exists {
			// Check if this is the last use
			// (Simplified - would need proper analysis)
			delete(live, reg)
		}
	}
	
	// Add defined register
	if inst.Dest != 0 {
		live[inst.Dest] = true
	}
}

func (rpo *RegisterPressureOptimizer) isReadyAfterScheduling(instIndex, scheduledInst int, scheduled []ir.Instruction, graph *DependencyGraph) bool {
	// Check if all dependencies of this instruction have been scheduled
	for _, dep := range graph.Dependencies[instIndex] {
		if dep == scheduledInst {
			continue // This dependency is now satisfied
		}
		
		// Check if this dependency is in the scheduled list
		found := false
		for _, schInst := range scheduled {
			if schInst.Dest == graph.Instructions[dep].Dest { // Simplified comparison
				found = true
				break
			}
		}
		
		if !found {
			return false // Still has unscheduled dependencies
		}
	}
	
	return true
}

func (rpo *RegisterPressureOptimizer) findLiveRegistersAtPoint(function *ir.Function, point int) map[int]bool {
	live := make(map[int]bool)
	
	for reg, liveRange := range rpo.liveness {
		if rpo.isLiveAtPoint(liveRange, point) {
			live[reg] = true
		}
	}
	
	return live
}

func (rpo *RegisterPressureOptimizer) isLiveAtPoint(liveRange LiveRange, point int) bool {
	return point >= liveRange.Start && point <= liveRange.End
}

func (rpo *RegisterPressureOptimizer) isInRegion(liveRange LiveRange, region InstRange) bool {
	return liveRange.Start <= region.End && liveRange.End >= region.Start
}

func (rpo *RegisterPressureOptimizer) isCheapToRematerialize(function *ir.Function, reg int) bool {
	// Check if the value is a simple constant or calculation
	liveRange := rpo.liveness[reg]
	
	if liveRange.Start < len(function.Instructions) {
		defInst := function.Instructions[liveRange.Start]
		
		// Constants are cheap to rematerialize
		if defInst.Op == ir.OpLoadConst {
			return true
		}
		
		// Simple arithmetic is cheap if operands are constants
		if defInst.Op == ir.OpAdd || defInst.Op == ir.OpSub {
			leftRange, leftExists := rpo.liveness[defInst.Left]
			rightRange, rightExists := rpo.liveness[defInst.Right]
			
			if leftExists && rightExists {
				leftInst := function.Instructions[leftRange.Start]
				rightInst := function.Instructions[rightRange.Start]
				
				return leftInst.Op == ir.OpLoadConst && rightInst.Op == ir.OpLoadConst
			}
		}
	}
	
	return false
}

func (rpo *RegisterPressureOptimizer) markForRematerialization(function *ir.Function, reg int) {
	// In a full implementation, this would mark the register for rematerialization
	// instead of spilling during register allocation
}

func (rpo *RegisterPressureOptimizer) functionResultIsUsed(function *ir.Function, callIndex int) bool {
	// Check if the result of the function call is used
	if callIndex+1 < len(function.Instructions) {
		nextInst := function.Instructions[callIndex+1]
		callInst := function.Instructions[callIndex]
		
		// Simple check: if the next instruction uses the call result
		usedRegs := rpo.getUsedRegisters(nextInst)
		for _, reg := range usedRegs {
			if reg == callInst.Dest {
				return true
			}
		}
	}
	
	return false
}

func (rpo *RegisterPressureOptimizer) insertRegisterResetPoint(function *ir.Function, index int) {
	// Insert a comment indicating a register allocation reset point
	comment := ir.Instruction{
		Op:      ir.OpComment,
		Comment: "REGISTER_RESET_POINT: Allocation can restart here",
	}
	
	rpo.insertInstructionAfter(function, index, comment)
}

func (rpo *RegisterPressureOptimizer) insertInstructionBefore(function *ir.Function, index int, inst ir.Instruction) {
	// Insert instruction before the given index
	instructions := make([]ir.Instruction, len(function.Instructions)+1)
	copy(instructions[:index], function.Instructions[:index])
	instructions[index] = inst
	copy(instructions[index+1:], function.Instructions[index:])
	function.Instructions = instructions
}

func (rpo *RegisterPressureOptimizer) insertInstructionAfter(function *ir.Function, index int, inst ir.Instruction) {
	// Insert instruction after the given index
	instructions := make([]ir.Instruction, len(function.Instructions)+1)
	copy(instructions[:index+1], function.Instructions[:index+1])
	instructions[index+1] = inst
	copy(instructions[index+2:], function.Instructions[index+1:])
	function.Instructions = instructions
}

func (rpo *RegisterPressureOptimizer) calculatePerformanceSavings(original, optimized RegisterPressureInfo) {
	spillsEliminated := original.SpillCount - optimized.SpillCount
	
	if spillsEliminated > 0 {
		// Each eliminated spill saves approximately:
		// - PUSH: 11 T-states, 1 byte
		// - POP: 10 T-states, 1 byte
		// Total per spill: 21 T-states, 2 bytes
		
		rpo.metrics.CyclesSaved += spillsEliminated * 21
		rpo.metrics.BytesSaved += spillsEliminated * 2
	}
}

// GetMetrics returns the optimization metrics
func (rpo *RegisterPressureOptimizer) GetMetrics() *OptimizationMetrics {
	return rpo.metrics
}