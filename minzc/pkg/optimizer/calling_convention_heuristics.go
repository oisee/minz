package optimizer

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// CallingConventionHeuristics determines the optimal calling convention for functions
type CallingConventionHeuristics struct{}

// OptimizationStrategy represents different calling/optimization strategies
type OptimizationStrategy int

const (
	StrategyInline       OptimizationStrategy = iota // Best: eliminate call entirely
	StrategyRegister                                 // Good: register parameter passing
	StrategySMC                                      // Complex: self-modifying code
	StrategyStack                                    // Fallback: stack-based calling
)

// FunctionMetrics holds analysis data for optimization decisions
type FunctionMetrics struct {
	InstructionCount    int
	ParameterCount      int
	LocalVariableCount  int
	HasLoops           bool
	IsRecursive        bool
	CallComplexity     int  // Estimated T-states for function body
	SetupOverhead      int  // T-states for parameter setup
	CallFrequency      int  // Estimated call frequency (if available)
}

// AnalyzeFunctionMetrics calculates metrics for optimization decisions
func (h *CallingConventionHeuristics) AnalyzeFunctionMetrics(fn *ir.Function) *FunctionMetrics {
	metrics := &FunctionMetrics{
		InstructionCount:   len(fn.Instructions),
		ParameterCount:     len(fn.Params),
		LocalVariableCount: len(fn.Locals),
		IsRecursive:        fn.IsRecursive,
	}

	// Analyze instruction complexity
	for _, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv:
			metrics.CallComplexity += 11 // Basic ALU operations
		case ir.OpLoadConst:
			metrics.CallComplexity += 10 // LD r, n
		case ir.OpLoad, ir.OpStore:
			metrics.CallComplexity += 13 // Memory operations
		case ir.OpJump, ir.OpJumpIfNot:
			metrics.HasLoops = true
			metrics.CallComplexity += 12 // Conditional jumps
		case ir.OpCall:
			metrics.CallComplexity += 17 // Function calls
		default:
			metrics.CallComplexity += 8 // Average instruction cost
		}
	}

	// Estimate parameter setup overhead for different strategies
	metrics.SetupOverhead = h.calculateSetupOverhead(metrics.ParameterCount)

	return metrics
}

// calculateSetupOverhead estimates T-states for parameter setup
func (h *CallingConventionHeuristics) calculateSetupOverhead(paramCount int) int {
	// Register passing: ~10 T-states per parameter (LD r, n)
	registerCost := paramCount * 10
	
	// SMC patching: ~23 T-states per parameter (LD HL, n; LD (addr), HL)
	smcCost := paramCount * 23
	
	// Return the most efficient for this parameter count
	if paramCount <= 4 { // Can fit in registers
		return registerCost
	}
	return smcCost
}

// DetermineOptimalStrategy chooses the best optimization approach
func (h *CallingConventionHeuristics) DetermineOptimalStrategy(fn *ir.Function) OptimizationStrategy {
	metrics := h.AnalyzeFunctionMetrics(fn)

	// Strategy 1: Inlining (best performance)
	if h.shouldInline(metrics) {
		return StrategyInline
	}

	// Strategy 2: Register passing (good for simple functions)
	if h.shouldUseRegisters(metrics) {
		return StrategyRegister
	}

	// Strategy 3: SMC (good for complex/frequent functions)
	if h.shouldUseSMC(metrics) {
		return StrategySMC
	}

	// Strategy 4: Stack (fallback for complex cases)
	return StrategyStack
}

// shouldInline determines if function should be inlined
func (h *CallingConventionHeuristics) shouldInline(metrics *FunctionMetrics) bool {
	// Inline small, simple functions
	if metrics.InstructionCount <= 5 && 
	   !metrics.HasLoops && 
	   !metrics.IsRecursive &&
	   metrics.ParameterCount <= 3 {
		// The function body is smaller than call overhead
		callOverhead := 17 + metrics.SetupOverhead // CALL + parameter setup
		if metrics.CallComplexity < callOverhead {
			return true
		}
	}
	return false
}

// shouldUseRegisters determines if register passing is optimal
func (h *CallingConventionHeuristics) shouldUseRegisters(metrics *FunctionMetrics) bool {
	// Register passing is good for:
	// - Simple functions with few parameters
	// - When we have enough registers available
	// - Non-recursive functions
	
	if metrics.IsRecursive {
		return false // Recursive functions need stack or SMC
	}
	
	if metrics.ParameterCount > 4 {
		return false // Not enough registers
	}
	
	if metrics.LocalVariableCount > 6 {
		return false // Too many locals compete for registers
	}
	
	// For simple functions, register passing beats SMC
	if metrics.CallComplexity < 50 && metrics.ParameterCount <= 3 {
		return true
	}
	
	return false
}

// shouldUseSMC determines if SMC is the best approach
func (h *CallingConventionHeuristics) shouldUseSMC(metrics *FunctionMetrics) bool {
	// SMC is beneficial for:
	// - Complex functions where setup cost is amortized
	// - Functions with many parameters (>4)
	// - Recursive functions (eliminates stack overhead)
	// - Functions called frequently
	
	if metrics.ParameterCount > 4 {
		return true // Too many parameters for registers
	}
	
	if metrics.IsRecursive && metrics.ParameterCount > 0 {
		return true // SMC eliminates recursive stack overhead
	}
	
	// For complex functions, SMC setup cost is worth it
	if metrics.CallComplexity > 100 {
		return true
	}
	
	// If estimated to be called frequently, SMC pays off
	if metrics.CallFrequency > 10 {
		return true
	}
	
	return false
}

// GenerateOptimizationReport provides detailed analysis
func (h *CallingConventionHeuristics) GenerateOptimizationReport(fn *ir.Function) string {
	metrics := h.AnalyzeFunctionMetrics(fn)
	strategy := h.DetermineOptimalStrategy(fn)
	
	strategyName := map[OptimizationStrategy]string{
		StrategyInline:   "INLINE",
		StrategyRegister: "REGISTER",
		StrategySMC:      "SMC",
		StrategyStack:    "STACK",
	}
	
	report := fmt.Sprintf(`
Function: %s
Strategy: %s
Metrics:
  - Instructions: %d
  - Parameters: %d
  - Locals: %d
  - Complexity: %d T-states
  - Setup overhead: %d T-states
  - Has loops: %v
  - Is recursive: %v
Reasoning: %s
`, 
		fn.Name,
		strategyName[strategy],
		metrics.InstructionCount,
		metrics.ParameterCount,
		metrics.LocalVariableCount,
		metrics.CallComplexity,
		metrics.SetupOverhead,
		metrics.HasLoops,
		metrics.IsRecursive,
		h.getStrategyReasoning(strategy, metrics),
	)
	
	return report
}

// getStrategyReasoning explains why a strategy was chosen
func (h *CallingConventionHeuristics) getStrategyReasoning(strategy OptimizationStrategy, metrics *FunctionMetrics) string {
	switch strategy {
	case StrategyInline:
		return "Function is small and simple - inlining eliminates call overhead"
	case StrategyRegister:
		return "Simple function with few parameters - register passing is most efficient"
	case StrategySMC:
		if metrics.IsRecursive {
			return "Recursive function benefits from SMC parameter patching"
		}
		if metrics.ParameterCount > 4 {
			return "Too many parameters for registers - SMC is more efficient than stack"
		}
		return "Complex function - SMC setup cost is amortized"
	case StrategyStack:
		return "Complex function with many locals - stack calling convention is most appropriate"
	default:
		return "Unknown strategy"
	}
}