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
		Functions:     make(map[string]*FunctionStats),
		Instructions:  make(map[string]*InstructionStats),
		SMCImpact:     make(map[uint16]*SMCImpact),
		Bottlenecks:   make([]Bottleneck, 0),
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

// PerformanceReport contains complete performance analysis
type PerformanceReport struct {
	TotalCycles    uint64
	TotalFrames    uint64
	Functions      map[string]*FunctionStats
	Instructions   map[string]*InstructionStats
	SMCImpact      map[uint16]*SMCImpact
	Bottlenecks    []Bottleneck
	Optimizations  []OptimizationOpportunity
}

// FunctionStats tracks function performance
type FunctionStats struct {
	Name       string
	Calls      uint32
	TotalCycles uint64
	MinCycles  uint64
	MaxCycles  uint64
	AvgCycles  float64
	Percentage float64  // % of total execution time
}

// InstructionStats tracks instruction usage
type InstructionStats struct {
	Opcode     string
	Count      uint32
	TotalCycles uint64
	AvgCycles  float64
}

// SMCImpact measures self-modifying code effectiveness
type SMCImpact struct {
	Address      uint16
	Modifications uint32
	CyclesSaved  int64  // Negative means slower
	Description  string
}

// Bottleneck identifies performance problems
type Bottleneck struct {
	Location    uint16
	Function    string
	Cycles      uint64
	Percentage  float64
	Description string
	Suggestion  string
}

// OptimizationOpportunity suggests improvements
type OptimizationOpportunity struct {
	Priority     int     // 1-10, higher is better
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
			funcName := fmt.Sprintf("func_%04X", call.Address)
			stats, exists := report.Functions[funcName]
			if !exists {
				stats = &FunctionStats{
					Name:      funcName,
					MinCycles: ^uint64(0),
				}
				report.Functions[funcName] = stats
			}
			
			cycles := state.Cycle - call.EntryCycle
			stats.Calls++
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
		if stats.Calls > 0 {
			stats.AvgCycles = float64(stats.TotalCycles) / float64(stats.Calls)
			stats.Percentage = float64(stats.TotalCycles) / float64(report.TotalCycles) * 100
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
	for i := 1; i < len(a.debugger.stateHistory); i++ {
		state := a.debugger.stateHistory[i]
		prevState := a.debugger.stateHistory[i-1]
		
		// Extract opcode from memory at previous PC
		opcode := a.decodeOpcode(prevState)
		
		stats, exists := report.Instructions[opcode]
		if !exists {
			stats = &InstructionStats{
				Opcode: opcode,
			}
			report.Instructions[opcode] = stats
		}
		
		stats.Count++
		cycleDiff := state.Cycle - prevState.Cycle
		stats.TotalCycles += cycleDiff
	}
	
	// Calculate averages
	for _, stats := range report.Instructions {
		if stats.Count > 0 {
			stats.AvgCycles = float64(stats.TotalCycles) / float64(stats.Count)
		}
	}
}

// analyzeSMC measures self-modifying code impact
func (a *TASAnalyzer) analyzeSMC(report *PerformanceReport) {
	// Group SMC events by address
	smcByAddr := make(map[uint16][]SMCEvent)
	for _, event := range a.debugger.smcEvents {
		smcByAddr[event.Address] = append(smcByAddr[event.Address], event)
	}
	
	// Analyze impact of each SMC location
	for addr, events := range smcByAddr {
		impact := &SMCImpact{
			Address:       addr,
			Modifications: uint32(len(events)),
		}
		
		// Estimate cycles saved (heuristic)
		if len(events) > 10 {
			// Frequent modification suggests parameter patching
			impact.CyclesSaved = int64(len(events)) * 5  // ~5 cycles saved per patch
			impact.Description = "Parameter patching optimization"
		} else {
			// Rare modification might be initialization
			impact.CyclesSaved = -2  // Small overhead
			impact.Description = "One-time initialization"
		}
		
		report.SMCImpact[addr] = impact
	}
}

// findBottlenecks identifies performance problems
func (a *TASAnalyzer) findBottlenecks(report *PerformanceReport) {
	// Find hot functions (>5% of execution time)
	for name, stats := range report.Functions {
		if stats.Percentage > 5.0 {
			bottleneck := Bottleneck{
				Location:   a.getFunctionAddress(name),
				Function:   name,
				Cycles:     stats.TotalCycles,
				Percentage: stats.Percentage,
				Description: fmt.Sprintf("Hot function: %d calls, %.1f%% of time", 
					stats.Calls, stats.Percentage),
			}
			
			// Suggest optimizations based on patterns
			if stats.Calls > 1000 {
				bottleneck.Suggestion = "Consider inlining this frequently called function"
			} else if float64(stats.MaxCycles) > stats.AvgCycles*2 {
				bottleneck.Suggestion = "High variance in execution time - check for inefficient paths"
			} else {
				bottleneck.Suggestion = "Optimize the algorithm or use SMC for parameters"
			}
			
			report.Bottlenecks = append(report.Bottlenecks, bottleneck)
		}
	}
	
	// Sort bottlenecks by impact
	sort.Slice(report.Bottlenecks, func(i, j int) bool {
		return report.Bottlenecks[i].Percentage > report.Bottlenecks[j].Percentage
	})
}

// findOptimizations suggests code improvements
func (a *TASAnalyzer) findOptimizations(report *PerformanceReport) {
	report.Optimizations = make([]OptimizationOpportunity, 0)
	
	// Check for common optimization patterns
	a.findLoopOptimizations(report)
	a.findRegisterOptimizations(report)
	a.findMemoryOptimizations(report)
	a.findSMCOpportunities(report)
	
	// Sort by priority
	sort.Slice(report.Optimizations, func(i, j int) bool {
		return report.Optimizations[i].Priority > report.Optimizations[j].Priority
	})
}

// findLoopOptimizations looks for loop improvement opportunities
func (a *TASAnalyzer) findLoopOptimizations(report *PerformanceReport) {
	// Look for DEC + JR NZ patterns that could be DJNZ
	for pc, stats := range report.Instructions {
		if stats.Opcode == "DEC B" || stats.Opcode == "DEC C" {
			// Check if followed by conditional jump
			nextPC := a.getNextPC(pc)
			if nextStats, exists := report.Instructions[a.getOpcodeAt(nextPC)]; exists {
				if nextStats.Opcode == "JR NZ" {
					opt := OptimizationOpportunity{
						Priority:    8,
						Location:    a.parsePC(pc),
						Current:     "DEC reg + JR NZ",
						Suggested:   "DJNZ",
						CyclesSaved: uint64(stats.Count) * 3,  // DJNZ saves ~3 cycles
						Description: "Replace DEC+JR NZ with DJNZ for faster loops",
					}
					report.Optimizations = append(report.Optimizations, opt)
				}
			}
		}
	}
}

// findRegisterOptimizations looks for register usage improvements
func (a *TASAnalyzer) findRegisterOptimizations(report *PerformanceReport) {
	// Look for excessive LD instructions
	loadCount := uint32(0)
	for _, stats := range report.Instructions {
		if a.isLoadInstruction(stats.Opcode) {
			loadCount += stats.Count
		}
	}
	
	totalInstructions := uint32(0)
	for _, stats := range report.Instructions {
		totalInstructions += stats.Count
	}
	
	loadPercentage := float64(loadCount) / float64(totalInstructions) * 100
	if loadPercentage > 30 {
		opt := OptimizationOpportunity{
			Priority:    7,
			Description: fmt.Sprintf("%.1f%% of instructions are loads - consider register allocation", loadPercentage),
			Suggested:   "Improve register allocation to reduce memory access",
			CyclesSaved: uint64(loadCount) / 10,  // Estimate 10% reduction possible
		}
		report.Optimizations = append(report.Optimizations, opt)
	}
}

// findMemoryOptimizations looks for memory access improvements
func (a *TASAnalyzer) findMemoryOptimizations(report *PerformanceReport) {
	// Look for repeated memory access patterns
	// This is simplified - real implementation would track actual addresses
	
	if memStats, exists := report.Instructions["LD A,(HL)"]; exists {
		if memStats.Count > 1000 {
			opt := OptimizationOpportunity{
				Priority:    6,
				Current:     "Frequent LD A,(HL)",
				Suggested:   "Cache frequently accessed values in registers",
				CyclesSaved: uint64(memStats.Count) / 5,
				Description: "High frequency memory reads detected",
			}
			report.Optimizations = append(report.Optimizations, opt)
		}
	}
}

// findSMCOpportunities identifies where SMC could help
func (a *TASAnalyzer) findSMCOpportunities(report *PerformanceReport) {
	// Look for functions with consistent parameter values
	for name, stats := range report.Functions {
		if stats.Calls > 100 && len(report.SMCImpact) == 0 {
			opt := OptimizationOpportunity{
				Priority:    9,
				Location:    a.getFunctionAddress(name),
				Current:     "Regular function calls",
				Suggested:   "Enable SMC for parameter patching",
				CyclesSaved: uint64(stats.Calls) * 5,  // ~5 cycles per call
				Description: fmt.Sprintf("Function %s called %d times - SMC could optimize", name, stats.Calls),
			}
			report.Optimizations = append(report.Optimizations, opt)
		}
	}
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
	output := "═══════════════════════════════════════════════════════════════════════════\n"
	output += "                    PERFORMANCE ANALYSIS REPORT                            \n"
	output += "═══════════════════════════════════════════════════════════════════════════\n\n"
	
	// Top functions
	output += "🔥 HOTTEST FUNCTIONS:\n"
	output += "─────────────────────────────────────────────────────────────────────────\n"
	
	// Sort functions by percentage
	var sortedFuncs []string
	for name := range report.Functions {
		sortedFuncs = append(sortedFuncs, name)
	}
	sort.Slice(sortedFuncs, func(i, j int) bool {
		return report.Functions[sortedFuncs[i]].Percentage > 
		       report.Functions[sortedFuncs[j]].Percentage
	})
	
	for i, name := range sortedFuncs {
		if i >= 5 {
			break
		}
		stats := report.Functions[name]
		bar := makeBar(int(stats.Percentage))
		output += fmt.Sprintf("%-20s %s %.1f%% (%d calls, avg %d cycles)\n",
			name, bar, stats.Percentage, stats.Calls, int(stats.AvgCycles))
	}
	
	// Bottlenecks
	output += "\n⚠️  BOTTLENECKS DETECTED:\n"
	output += "─────────────────────────────────────────────────────────────────────────\n"
	for _, b := range report.Bottlenecks {
		output += fmt.Sprintf("• %s (%.1f%%)\n", b.Description, b.Percentage)
		output += fmt.Sprintf("  → %s\n", b.Suggestion)
	}
	
	// Optimization opportunities
	output += "\n💡 OPTIMIZATION OPPORTUNITIES:\n"
	output += "─────────────────────────────────────────────────────────────────────────\n"
	for i, opt := range report.Optimizations {
		if i >= 5 {
			break
		}
		output += fmt.Sprintf("[Priority %d] %s\n", opt.Priority, opt.Description)
		if opt.Current != "" {
			output += fmt.Sprintf("  Current:  %s\n", opt.Current)
			output += fmt.Sprintf("  Suggested: %s\n", opt.Suggested)
		}
		output += fmt.Sprintf("  Potential savings: %d cycles\n", opt.CyclesSaved)
	}
	
	// SMC Impact
	if len(report.SMCImpact) > 0 {
		output += "\n🔧 SELF-MODIFYING CODE IMPACT:\n"
		output += "─────────────────────────────────────────────────────────────────────────\n"
		totalSaved := int64(0)
		for _, impact := range report.SMCImpact {
			totalSaved += impact.CyclesSaved
			if impact.CyclesSaved > 0 {
				output += fmt.Sprintf("• %04X: +%d cycles saved (%s)\n",
					impact.Address, impact.CyclesSaved, impact.Description)
			}
		}
		output += fmt.Sprintf("Total SMC benefit: %d cycles\n", totalSaved)
	}
	
	return output
}