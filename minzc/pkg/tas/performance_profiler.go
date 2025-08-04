package tas

import (
	"fmt"
	"sort"
	"strings"
)

// PerformanceProfiler analyzes TAS recordings for optimization opportunities
type PerformanceProfiler struct {
	debugger    *TASDebugger
	functions   map[uint16]*FunctionProfile
	hotspots    []Hotspot
	bottlenecks []Bottleneck
	
	// Analysis configuration
	minFunctionCycles int64 // Minimum cycles to consider a function
	hotspotThreshold  float64 // Percentage of total time to be a hotspot
}

// FunctionProfile tracks performance data for a function
type FunctionProfile struct {
	Address      uint16
	Name         string
	CallCount    int
	TotalCycles  int64
	MinCycles    int64
	MaxCycles    int64
	AvgCycles    float64
	SelfCycles   int64 // Cycles excluding called functions
	ChildCycles  int64 // Cycles in called functions
	
	// Optimization data
	SMCCount     int    // Self-modifying code operations
	LoopCount    int    // Number of loops detected
	MemoryAccess int    // Memory reads/writes
	IOOperations int    // I/O operations
	
	// Call graph
	Callers      map[uint16]int // Who calls this function
	Callees      map[uint16]int // What this function calls
}

// Hotspot represents a performance-critical section
type Hotspot struct {
	Address       uint16
	Function      string
	Cycles        int64
	Percentage    float64
	Type          HotspotType
	OptimizationHint string
}

type HotspotType byte

const (
	HotspotCompute HotspotType = iota
	HotspotLoop
	HotspotMemory
	HotspotIO
	HotspotSMC
)

// Bottleneck represents a performance bottleneck
type Bottleneck struct {
	Description   string
	Impact        int64 // Cycles that could be saved
	Location      uint16
	Type          BottleneckType
	Solution      string
	Confidence    float64
}

type BottleneckType byte

const (
	BottleneckRedundant BottleneckType = iota
	BottleneckInefficient
	BottleneckMemoryPattern
	BottleneckUnoptimizedLoop
	BottleneckExcessiveCall
)

// NewPerformanceProfiler creates a new performance profiler
func NewPerformanceProfiler(debugger *TASDebugger) *PerformanceProfiler {
	return &PerformanceProfiler{
		debugger:         debugger,
		functions:        make(map[uint16]*FunctionProfile),
		minFunctionCycles: 100,
		hotspotThreshold:  0.05, // 5% of total time
	}
}

// Analyze performs comprehensive performance analysis
func (p *PerformanceProfiler) Analyze() *PerformanceReport {
	// Clear previous analysis
	p.functions = make(map[uint16]*FunctionProfile)
	p.hotspots = nil
	p.bottlenecks = nil
	
	// Phase 1: Build function profiles
	p.buildFunctionProfiles()
	
	// Phase 2: Identify hotspots
	p.identifyHotspots()
	
	// Phase 3: Find bottlenecks
	p.findBottlenecks()
	
	// Phase 4: Generate optimization suggestions
	suggestions := p.generateOptimizations()
	
	return &PerformanceReport{
		Functions:    p.functions,
		Hotspots:     p.hotspots,
		Bottlenecks:  p.bottlenecks,
		Suggestions:  suggestions,
		TotalCycles:  p.getTotalCycles(),
	}
}

// buildFunctionProfiles analyzes execution to build function profiles
func (p *PerformanceProfiler) buildFunctionProfiles() {
	callStack := make([]uint16, 0, 32)
	functionStarts := make(map[uint16]int64)
	
	// Analyze state history
	for i, state := range p.debugger.stateHistory {
		pc := state.PC
		
		// Detect function calls (CALL instruction)
		if p.isCallInstruction(pc, i) {
			// New function call
			if _, exists := p.functions[pc]; !exists {
				p.functions[pc] = &FunctionProfile{
					Address:   pc,
					Name:      p.getFunctionName(pc),
					MinCycles: int64(^uint64(0) >> 1), // Max int64
					Callers:   make(map[uint16]int),
					Callees:   make(map[uint16]int),
				}
			}
			
			// Track call
			p.functions[pc].CallCount++
			functionStarts[pc] = int64(state.Cycle)
			
			// Update call graph
			if len(callStack) > 0 {
				caller := callStack[len(callStack)-1]
				p.functions[pc].Callers[caller]++
				if p.functions[caller] != nil {
					p.functions[caller].Callees[pc]++
				}
			}
			
			callStack = append(callStack, pc)
		}
		
		// Detect function returns (RET instruction)
		if p.isRetInstruction(pc, i) && len(callStack) > 0 {
			funcAddr := callStack[len(callStack)-1]
			callStack = callStack[:len(callStack)-1]
			
			// Calculate function duration
			if startCycle, exists := functionStarts[funcAddr]; exists {
				duration := int64(state.Cycle) - startCycle
				prof := p.functions[funcAddr]
				
				prof.TotalCycles += duration
				if duration < prof.MinCycles {
					prof.MinCycles = duration
				}
				if duration > prof.MaxCycles {
					prof.MaxCycles = duration
				}
				
				delete(functionStarts, funcAddr)
			}
		}
		
		// Track SMC events
		for _, smc := range p.debugger.smcEvents {
			if smc.PC >= pc && smc.PC < pc+100 { // Rough function boundary
				if prof := p.functions[pc]; prof != nil {
					prof.SMCCount++
				}
			}
		}
		
		// Track I/O
		if p.isIOInstruction(pc, i) {
			if prof := p.getCurrentFunction(pc, callStack); prof != nil {
				prof.IOOperations++
			}
		}
	}
	
	// Calculate averages
	for _, prof := range p.functions {
		if prof.CallCount > 0 {
			prof.AvgCycles = float64(prof.TotalCycles) / float64(prof.CallCount)
		}
	}
}

// identifyHotspots finds performance-critical sections
func (p *PerformanceProfiler) identifyHotspots() {
	totalCycles := p.getTotalCycles()
	if totalCycles == 0 {
		return
	}
	
	// Sort functions by total cycles
	type funcCycles struct {
		addr   uint16
		cycles int64
	}
	
	var sorted []funcCycles
	for addr, prof := range p.functions {
		sorted = append(sorted, funcCycles{addr, prof.TotalCycles})
	}
	
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].cycles > sorted[j].cycles
	})
	
	// Identify hotspots
	for _, fc := range sorted {
		percentage := float64(fc.cycles) / float64(totalCycles)
		if percentage < p.hotspotThreshold {
			break // Rest are below threshold
		}
		
		prof := p.functions[fc.addr]
		hotspot := Hotspot{
			Address:    fc.addr,
			Function:   prof.Name,
			Cycles:     fc.cycles,
			Percentage: percentage * 100,
			Type:       p.classifyHotspot(prof),
		}
		
		// Generate optimization hint
		hotspot.OptimizationHint = p.getOptimizationHint(prof, hotspot.Type)
		
		p.hotspots = append(p.hotspots, hotspot)
	}
}

// findBottlenecks identifies performance bottlenecks
func (p *PerformanceProfiler) findBottlenecks() {
	// Check for redundant operations
	p.findRedundantOperations()
	
	// Check for inefficient patterns
	p.findInefficientPatterns()
	
	// Check for unoptimized loops
	p.findUnoptimizedLoops()
	
	// Check for excessive function calls
	p.findExcessiveCalls()
	
	// Sort bottlenecks by impact
	sort.Slice(p.bottlenecks, func(i, j int) bool {
		return p.bottlenecks[i].Impact > p.bottlenecks[j].Impact
	})
}

// findRedundantOperations finds redundant computations
func (p *PerformanceProfiler) findRedundantOperations() {
	// Look for repeated calculations
	for addr, prof := range p.functions {
		if prof.CallCount > 100 && prof.AvgCycles < 50 {
			// Small function called many times - candidate for inlining
			bottleneck := Bottleneck{
				Description: fmt.Sprintf("Function %s called %d times (avg %0.f cycles)",
					prof.Name, prof.CallCount, prof.AvgCycles),
				Impact:      int64(float64(prof.CallCount) * 10), // Call overhead
				Location:    addr,
				Type:        BottleneckRedundant,
				Solution:    "Consider inlining or caching results",
				Confidence:  0.8,
			}
			p.bottlenecks = append(p.bottlenecks, bottleneck)
		}
	}
}

// findInefficientPatterns finds inefficient code patterns
func (p *PerformanceProfiler) findInefficientPatterns() {
	// Check for memory access patterns
	for addr, prof := range p.functions {
		if prof.MemoryAccess > prof.CallCount*100 {
			// Excessive memory access
			bottleneck := Bottleneck{
				Description: fmt.Sprintf("Excessive memory access in %s (%d accesses)",
					prof.Name, prof.MemoryAccess),
				Impact:      int64(prof.MemoryAccess * 3), // Memory access overhead
				Location:    addr,
				Type:        BottleneckMemoryPattern,
				Solution:    "Use registers for temporary values, cache frequently accessed data",
				Confidence:  0.7,
			}
			p.bottlenecks = append(p.bottlenecks, bottleneck)
		}
	}
}

// findUnoptimizedLoops finds loops that could be optimized
func (p *PerformanceProfiler) findUnoptimizedLoops() {
	// Look for DJNZ opportunities
	for addr, prof := range p.functions {
		if prof.LoopCount > 0 && prof.TotalCycles > 1000 {
			// Check if loop uses DJNZ
			if !p.usesDJNZ(addr) {
				bottleneck := Bottleneck{
					Description: fmt.Sprintf("Loop in %s not using DJNZ optimization",
						prof.Name),
					Impact:      prof.TotalCycles / 3, // DJNZ is ~3x faster
					Location:    addr,
					Type:        BottleneckUnoptimizedLoop,
					Solution:    "Use DJNZ instruction for loop counter",
					Confidence:  0.9,
				}
				p.bottlenecks = append(p.bottlenecks, bottleneck)
			}
		}
	}
}

// findExcessiveCalls finds functions with too many calls
func (p *PerformanceProfiler) findExcessiveCalls() {
	for addr, prof := range p.functions {
		// Check for functions that mostly call other functions
		if len(prof.Callees) > 5 && prof.ChildCycles > prof.SelfCycles*2 {
			bottleneck := Bottleneck{
				Description: fmt.Sprintf("Function %s spends most time in calls (%d callees)",
					prof.Name, len(prof.Callees)),
				Impact:      int64(len(prof.Callees) * 17), // CALL overhead
				Location:    addr,
				Type:        BottleneckExcessiveCall,
				Solution:    "Consider flattening call hierarchy or inlining hot paths",
				Confidence:  0.6,
			}
			p.bottlenecks = append(p.bottlenecks, bottleneck)
		}
	}
}

// generateOptimizations creates optimization suggestions
func (p *PerformanceProfiler) generateOptimizations() []OptimizationSuggestion {
	var suggestions []OptimizationSuggestion
	
	// Suggest SMC for hot functions
	for _, hotspot := range p.hotspots {
		if prof := p.functions[hotspot.Address]; prof != nil {
			if prof.SMCCount == 0 && prof.CallCount > 10 {
				suggestions = append(suggestions, OptimizationSuggestion{
					Priority:    High,
					Type:        OptimizeSMC,
					Location:    hotspot.Address,
					Description: fmt.Sprintf("Enable SMC for %s", prof.Name),
					Impact:      fmt.Sprintf("Save ~%d cycles (%.1f%%)", 
						prof.TotalCycles/5, hotspot.Percentage/5),
					Difficulty:  Medium,
				})
			}
		}
	}
	
	// Suggest register allocation improvements
	for _, hotspot := range p.hotspots {
		if hotspot.Type == HotspotMemory {
			suggestions = append(suggestions, OptimizationSuggestion{
				Priority:    High,
				Type:        OptimizeRegisters,
				Location:    hotspot.Address,
				Description: "Improve register allocation to reduce memory access",
				Impact:      fmt.Sprintf("Save ~%d cycles", hotspot.Cycles/10),
				Difficulty:  Hard,
			})
		}
	}
	
	// Suggest loop optimizations
	for _, bottleneck := range p.bottlenecks {
		if bottleneck.Type == BottleneckUnoptimizedLoop {
			suggestions = append(suggestions, OptimizationSuggestion{
				Priority:    High,
				Type:        OptimizeLoop,
				Location:    bottleneck.Location,
				Description: bottleneck.Solution,
				Impact:      fmt.Sprintf("Save ~%d cycles", bottleneck.Impact),
				Difficulty:  Easy,
			})
		}
	}
	
	// Sort by priority and impact
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Priority != suggestions[j].Priority {
			return suggestions[i].Priority > suggestions[j].Priority
		}
		return suggestions[i].Difficulty < suggestions[j].Difficulty
	})
	
	return suggestions
}

// Helper methods

func (p *PerformanceProfiler) getTotalCycles() int64 {
	if len(p.debugger.stateHistory) == 0 {
		return 0
	}
	return int64(p.debugger.stateHistory[len(p.debugger.stateHistory)-1].Cycle)
}

func (p *PerformanceProfiler) isCallInstruction(pc uint16, stateIdx int) bool {
	// Check for CALL opcode (0xCD, 0xC4, 0xCC, etc.)
	if stateIdx >= len(p.debugger.stateHistory) {
		return false
	}
	
	mem := p.debugger.stateHistory[stateIdx].Memory
	opcode := mem[pc]
	
	return opcode == 0xCD || // CALL nn
		(opcode&0xC7) == 0xC4 // CALL cc,nn
}

func (p *PerformanceProfiler) isRetInstruction(pc uint16, stateIdx int) bool {
	// Check for RET opcode (0xC9, 0xC0, 0xC8, etc.)
	if stateIdx >= len(p.debugger.stateHistory) {
		return false
	}
	
	mem := p.debugger.stateHistory[stateIdx].Memory
	opcode := mem[pc]
	
	return opcode == 0xC9 || // RET
		(opcode&0xC7) == 0xC0 // RET cc
}

func (p *PerformanceProfiler) isIOInstruction(pc uint16, stateIdx int) bool {
	// Check for IN/OUT opcodes
	if stateIdx >= len(p.debugger.stateHistory) {
		return false
	}
	
	mem := p.debugger.stateHistory[stateIdx].Memory
	opcode := mem[pc]
	
	return opcode == 0xD3 || // OUT (n),A
		opcode == 0xDB || // IN A,(n)
		(mem[pc] == 0xED && (mem[pc+1]&0x40) != 0) // ED-prefixed I/O
}

func (p *PerformanceProfiler) usesDJNZ(funcAddr uint16) bool {
	// Check if function uses DJNZ instruction
	for _, state := range p.debugger.stateHistory {
		if state.PC >= funcAddr && state.PC < funcAddr+256 {
			if state.Memory[state.PC] == 0x10 { // DJNZ
				return true
			}
		}
	}
	return false
}

func (p *PerformanceProfiler) getFunctionName(addr uint16) string {
	// Try to get symbolic name (would need symbol table)
	// For now, use address
	return fmt.Sprintf("func_%04X", addr)
}

func (p *PerformanceProfiler) getCurrentFunction(pc uint16, callStack []uint16) *FunctionProfile {
	// Find which function PC belongs to
	if len(callStack) > 0 {
		return p.functions[callStack[len(callStack)-1]]
	}
	
	// Check if PC is a known function
	if prof := p.functions[pc]; prof != nil {
		return prof
	}
	
	// Find closest function
	var closest uint16
	minDist := uint16(0xFFFF)
	for addr := range p.functions {
		if addr <= pc {
			dist := pc - addr
			if dist < minDist {
				minDist = dist
				closest = addr
			}
		}
	}
	
	return p.functions[closest]
}

func (p *PerformanceProfiler) classifyHotspot(prof *FunctionProfile) HotspotType {
	if prof.SMCCount > 0 {
		return HotspotSMC
	}
	if prof.IOOperations > prof.CallCount {
		return HotspotIO
	}
	if prof.MemoryAccess > prof.CallCount*10 {
		return HotspotMemory
	}
	if prof.LoopCount > 0 {
		return HotspotLoop
	}
	return HotspotCompute
}

func (p *PerformanceProfiler) getOptimizationHint(prof *FunctionProfile, hotType HotspotType) string {
	switch hotType {
	case HotspotSMC:
		return "Already using SMC optimization"
	case HotspotIO:
		return "Consider batching I/O operations"
	case HotspotMemory:
		return "Use registers for frequently accessed values"
	case HotspotLoop:
		if !p.usesDJNZ(prof.Address) {
			return "Convert to DJNZ loop for 3x performance"
		}
		return "Consider unrolling or SMC optimization"
	default:
		if prof.CallCount > 100 {
			return "Consider inlining or SMC parameter patching"
		}
		return "Profile for micro-optimizations"
	}
}

// PerformanceReport contains complete performance analysis
type PerformanceReport struct {
	Functions    map[uint16]*FunctionProfile
	Hotspots     []Hotspot
	Bottlenecks  []Bottleneck
	Suggestions  []OptimizationSuggestion
	TotalCycles  int64
}

// OptimizationSuggestion represents a specific optimization opportunity
type OptimizationSuggestion struct {
	Priority    Priority
	Type        OptimizationType
	Location    uint16
	Description string
	Impact      string
	Difficulty  Difficulty
}

type Priority byte
const (
	Low Priority = iota
	Medium
	High
	Critical
)

type OptimizationType byte
const (
	OptimizeSMC OptimizationType = iota
	OptimizeLoop
	OptimizeRegisters
	OptimizeInline
	OptimizeMemory
)

type Difficulty byte
const (
	Easy Difficulty = iota
	Medium
	Hard
)

// PrintReport generates a human-readable performance report
func (r *PerformanceReport) PrintReport() {
	fmt.Println("\n🔥 PERFORMANCE ANALYSIS REPORT")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Total Cycles Analyzed: %d\n", r.TotalCycles)
	fmt.Printf("Functions Profiled:    %d\n", len(r.Functions))
	fmt.Printf("Hotspots Found:        %d\n", len(r.Hotspots))
	fmt.Printf("Bottlenecks Found:     %d\n", len(r.Bottlenecks))
	
	// Print hotspots
	if len(r.Hotspots) > 0 {
		fmt.Println("\n🔥 HOTTEST FUNCTIONS:")
		fmt.Println("───────────────────────────────────────────────────────────────")
		for i, hotspot := range r.Hotspots {
			if i >= 5 {
				break
			}
			
			bar := strings.Repeat("█", int(hotspot.Percentage/2))
			fmt.Printf("%d. %s %s %.1f%% (%d cycles)\n",
				i+1, hotspot.Function, bar, hotspot.Percentage, hotspot.Cycles)
			
			if prof := r.Functions[hotspot.Address]; prof != nil {
				fmt.Printf("   Calls: %d, Avg: %.0f cycles, Min: %d, Max: %d\n",
					prof.CallCount, prof.AvgCycles, prof.MinCycles, prof.MaxCycles)
			}
			
			fmt.Printf("   💡 %s\n", hotspot.OptimizationHint)
		}
	}
	
	// Print bottlenecks
	if len(r.Bottlenecks) > 0 {
		fmt.Println("\n⚠️  PERFORMANCE BOTTLENECKS:")
		fmt.Println("───────────────────────────────────────────────────────────────")
		for i, bottleneck := range r.Bottlenecks {
			if i >= 5 {
				break
			}
			
			fmt.Printf("%d. %s\n", i+1, bottleneck.Description)
			fmt.Printf("   Impact: ~%d cycles could be saved\n", bottleneck.Impact)
			fmt.Printf("   Solution: %s\n", bottleneck.Solution)
			fmt.Printf("   Confidence: %.0f%%\n", bottleneck.Confidence*100)
		}
	}
	
	// Print optimization suggestions
	if len(r.Suggestions) > 0 {
		fmt.Println("\n💡 OPTIMIZATION SUGGESTIONS:")
		fmt.Println("───────────────────────────────────────────────────────────────")
		
		priorities := []string{"🔴 CRITICAL", "🟠 HIGH", "🟡 MEDIUM", "🟢 LOW"}
		difficulties := []string{"Easy", "Medium", "Hard"}
		
		for i, suggestion := range r.Suggestions {
			if i >= 10 {
				break
			}
			
			fmt.Printf("%d. %s [%s difficulty]\n",
				i+1, priorities[3-suggestion.Priority], difficulties[suggestion.Difficulty])
			fmt.Printf("   %s\n", suggestion.Description)
			fmt.Printf("   Impact: %s\n", suggestion.Impact)
			fmt.Printf("   Location: 0x%04X\n", suggestion.Location)
		}
	}
	
	// Summary
	fmt.Println("\n📊 OPTIMIZATION SUMMARY:")
	fmt.Println("───────────────────────────────────────────────────────────────")
	
	totalSavings := int64(0)
	for _, bottleneck := range r.Bottlenecks {
		totalSavings += bottleneck.Impact
	}
	
	if totalSavings > 0 {
		percentage := float64(totalSavings) / float64(r.TotalCycles) * 100
		fmt.Printf("Potential savings: %d cycles (%.1f%% improvement)\n",
			totalSavings, percentage)
		
		if percentage > 30 {
			fmt.Println("🚀 MASSIVE optimization potential detected!")
		} else if percentage > 10 {
			fmt.Println("✅ Good optimization opportunities available")
		} else {
			fmt.Println("📈 Incremental improvements possible")
		}
	} else {
		fmt.Println("✨ Code is well-optimized!")
	}
}