package tas

import (
	"fmt"
	"sort"
)

// TASAnalyzer performs frame-perfect optimization analysis
type TASAnalyzer struct {
	debugger *TASDebugger
}

// NewTASAnalyzer creates optimization analyzer
func NewTASAnalyzer(debugger *TASDebugger) *TASAnalyzer {
	return &TASAnalyzer{
		debugger: debugger,
	}
}

// AnalyzePerformance finds bottlenecks and optimization opportunities
func (a *TASAnalyzer) AnalyzePerformance() *PerformanceReport {
	report := &PerformanceReport{
		Functions:     make(map[uint16]*FunctionProfile),
		Bottlenecks:   make([]Bottleneck, 0),
		Suggestions:   make([]OptimizationSuggestion, 0),
	}
	
	// Analyze function call patterns
	a.analyzeFunctions(report)
	
	// Analyze instruction usage
	a.analyzeInstructions(report)
	
	// Analyze SMC effectiveness
	a.analyzeSMC(report)
	
	// Find bottlenecks
	a.findBottlenecks(report)
	
	// Calculate optimization opportunities
	a.findOptimizations(report)
	
	return report
}

// NOTE: These types are now defined in performance_profiler.go
// We just keep OptimizationOpportunity since it has a different structure

// OptimizationOpportunity suggests improvements  
type OptimizationOpportunity struct {
	OptPriority  int     // 1-10, higher is better
	Location     uint16
	Current      string  // Current code pattern
	Suggested    string  // Optimized version
	CyclesSaved  uint64
	Description  string
}

// analyzeFunctions profiles function performance
func (a *TASAnalyzer) analyzeFunctions(report *PerformanceReport) {
	// Track function entry/exit based on CALL/RET patterns
	callStack := make([]FunctionCall, 0, 32)
	
	for _, state := range a.debugger.stateHistory {
		// Detect CALL instruction (CD xx xx)
		if a.isCall(state) {
			call := FunctionCall{
				Address:     a.getCallTarget(state),
				EntryCycle:  state.Cycle,
				CallerPC:    state.PC,
			}
			callStack = append(callStack, call)
		}
		
		// Detect RET instruction (C9)
		if a.isReturn(state) && len(callStack) > 0 {
			call := callStack[len(callStack)-1]
			callStack = callStack[:len(callStack)-1]
			
			// Update function statistics
			funcAddr := call.Address
			stats, exists := report.Functions[funcAddr]
			if !exists {
				stats = &FunctionProfile{
					Address:   funcAddr,
					Name:      fmt.Sprintf("func_%04X", funcAddr),
					MinCycles: int64(^uint64(0) >> 1),
					Callers:   make(map[uint16]int),
					Callees:   make(map[uint16]int),
				}
				report.Functions[funcAddr] = stats
			}
			
			cycles := int64(state.Cycle - call.EntryCycle)
			stats.CallCount++
			stats.TotalCycles += cycles
			if cycles < stats.MinCycles {
				stats.MinCycles = cycles
			}
			if cycles > stats.MaxCycles {
				stats.MaxCycles = cycles
			}
		}
	}
	
	// Calculate averages and percentages
	for _, stats := range report.Functions {
		if stats.CallCount > 0 {
			stats.AvgCycles = float64(stats.TotalCycles) / float64(stats.CallCount)
		}
	}
}

// FunctionCall tracks function call on stack
type FunctionCall struct {
	Address    uint16
	EntryCycle uint64
	CallerPC   uint16
}

// analyzeInstructions profiles instruction usage
func (a *TASAnalyzer) analyzeInstructions(report *PerformanceReport) {
	// Skip for now since Instructions field doesn't exist in PerformanceReport
	// This would need to be refactored to match the new structure
}

// analyzeSMC measures self-modifying code impact
func (a *TASAnalyzer) analyzeSMC(report *PerformanceReport) {
	// Skip for now since SMCImpact field doesn't exist in PerformanceReport
	// This would need to be refactored to match the new structure
}

// findBottlenecks identifies performance problems
func (a *TASAnalyzer) findBottlenecks(report *PerformanceReport) {
	// Find hot functions (>5% of execution time) 
	totalCycles := report.TotalCycles
	if totalCycles == 0 {
		return
	}
	
	for addr, stats := range report.Functions {
		percentage := float64(stats.TotalCycles) / float64(totalCycles) * 100
		if percentage > 5.0 {
			bottleneck := Bottleneck{
				Location:   addr,
				Description: fmt.Sprintf("Hot function %s: %d calls, %.1f%% of time", 
					stats.Name, stats.CallCount, percentage),
				Impact:     stats.TotalCycles / 3, // Estimate 1/3 could be saved
				Type:       BottleneckInefficient,
				Confidence: 0.7,
			}
			
			// Suggest optimizations based on patterns
			if stats.CallCount > 1000 {
				bottleneck.Solution = "Consider inlining this frequently called function"
				bottleneck.Type = BottleneckExcessiveCall
			} else if float64(stats.MaxCycles) > stats.AvgCycles*2 {
				bottleneck.Solution = "High variance in execution time - check for inefficient paths"
			} else {
				bottleneck.Solution = "Optimize the algorithm or use SMC for parameters"
			}
			
			report.Bottlenecks = append(report.Bottlenecks, bottleneck)
		}
	}
	
	// Sort bottlenecks by impact
	sort.Slice(report.Bottlenecks, func(i, j int) bool {
		return report.Bottlenecks[i].Impact > report.Bottlenecks[j].Impact
	})
}

// findOptimizations suggests code improvements
func (a *TASAnalyzer) findOptimizations(report *PerformanceReport) {
	report.Suggestions = make([]OptimizationSuggestion, 0)
	
	// Check for common optimization patterns
	a.findLoopOptimizations(report)
	a.findRegisterOptimizations(report)
	a.findMemoryOptimizations(report)
	a.findSMCOpportunities(report)
	
	// Sort by priority (skip for now since we're not generating suggestions)
	// sort.Slice(report.Suggestions, func(i, j int) bool {
	// 	return report.Suggestions[i].Priority > report.Suggestions[j].Priority
	// })
}

// findLoopOptimizations looks for loop improvement opportunities
func (a *TASAnalyzer) findLoopOptimizations(report *PerformanceReport) {
	// Skip for now since Instructions field doesn't exist
}

// findRegisterOptimizations looks for register usage improvements
func (a *TASAnalyzer) findRegisterOptimizations(report *PerformanceReport) {
	// Skip for now since Instructions field doesn't exist
}

// findMemoryOptimizations looks for memory access improvements
func (a *TASAnalyzer) findMemoryOptimizations(report *PerformanceReport) {
	// Skip for now since Instructions field doesn't exist
}

// findSMCOpportunities identifies where SMC could help
func (a *TASAnalyzer) findSMCOpportunities(report *PerformanceReport) {
	// Skip for now since SMCImpact field doesn't exist
}

// Helper functions

func (a *TASAnalyzer) isCall(state StateSnapshot) bool {
	return state.Memory[state.PC] == 0xCD  // CALL instruction
}

func (a *TASAnalyzer) isReturn(state StateSnapshot) bool {
	return state.Memory[state.PC] == 0xC9  // RET instruction
}

func (a *TASAnalyzer) getCallTarget(state StateSnapshot) uint16 {
	if state.PC+2 < 65535 {
		low := state.Memory[state.PC+1]
		high := state.Memory[state.PC+2]
		return uint16(high)<<8 | uint16(low)
	}
	return 0
}

func (a *TASAnalyzer) decodeOpcode(state StateSnapshot) string {
	// Simplified opcode decoding
	opcode := state.Memory[state.PC]
	
	opcodeNames := map[byte]string{
		0x00: "NOP",
		0x01: "LD BC,nn",
		0x02: "LD (BC),A",
		0x03: "INC BC",
		0x04: "INC B",
		0x05: "DEC B",
		0x06: "LD B,n",
		0x07: "RLCA",
		0x08: "EX AF,AF'",
		0x09: "ADD HL,BC",
		0x0A: "LD A,(BC)",
		0x0B: "DEC BC",
		0x0C: "INC C",
		0x0D: "DEC C",
		0x0E: "LD C,n",
		0x0F: "RRCA",
		0x10: "DJNZ",
		0x18: "JR",
		0x20: "JR NZ",
		0x28: "JR Z",
		0x30: "JR NC",
		0x38: "JR C",
		0x3E: "LD A,n",
		0x7E: "LD A,(HL)",
		0xC9: "RET",
		0xCD: "CALL",
		// ... many more opcodes
	}
	
	if name, exists := opcodeNames[opcode]; exists {
		return name
	}
	return fmt.Sprintf("Op_%02X", opcode)
}

func (a *TASAnalyzer) getFunctionAddress(name string) uint16 {
	// Extract address from function name like "func_8000"
	var addr uint16
	fmt.Sscanf(name, "func_%04X", &addr)
	return addr
}

func (a *TASAnalyzer) getNextPC(instruction string) string {
	// Simplified - would need proper instruction length calculation
	return ""
}

func (a *TASAnalyzer) getOpcodeAt(pc string) string {
	// Simplified
	return ""
}

func (a *TASAnalyzer) parsePC(instruction string) uint16 {
	// Simplified
	return 0
}

func (a *TASAnalyzer) isLoadInstruction(opcode string) bool {
	return len(opcode) >= 2 && opcode[:2] == "LD"
}

// makeBar creates a visual progress bar
func makeBar(percentage int) string {
	width := 20
	filled := percentage * width / 100
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}

// Render creates visual performance report
func (report *PerformanceReport) Render() string {
	// Use the PrintReport method from performance_profiler.go instead
	report.PrintReport()
	return "" // PrintReport prints directly to stdout
}