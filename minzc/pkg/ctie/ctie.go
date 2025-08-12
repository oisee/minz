package ctie

import (
	"fmt"
	"strings"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/semantic"
)

// Engine is the main Compile-Time Interface Execution engine
type Engine struct {
	module      *ir.Module
	ast         *ast.File  // Changed from Program
	semantic    *semantic.Analyzer
	purity      *PurityAnalyzer
	executor    *CompileTimeExecutor
	constTracker *ConstTracker
	// specializer *InterfaceSpecializer  // TODO: implement later
	statistics  *Statistics
	config      *Config
}

// Config holds CTIE configuration
type Config struct {
	EnableExecute     bool
	EnableSpecialize  bool
	EnableProof       bool
	EnableDerive      bool
	EnableAnalysis    bool
	MaxExecutionTime  int // milliseconds
	MaxSpecializations int
	DebugOutput       bool
	OptimizationLevel int
}

// Statistics tracks CTIE metrics
type Statistics struct {
	FunctionsAnalyzed   int
	FunctionsExecuted   int
	ValuesComputed      int
	BytesEliminated     int
	SpecializationsCreated int
	ProofsVerified      int
	DerivationsGenerated int
	CompilationTime     int64 // microseconds
}

// NewEngine creates a new CTIE engine
func NewEngine(module *ir.Module, ast *ast.File, sem *semantic.Analyzer) *Engine {
	engine := &Engine{
		module:       module,
		ast:          ast,
		semantic:     sem,
		purity:       NewPurityAnalyzer(module),
		executor:     NewCompileTimeExecutor(module),
		constTracker: NewConstTracker(module),
		statistics:   &Statistics{},
		config:       DefaultConfig(),
	}
	
	// Initialize components
	engine.purity.InitBuiltins()
	
	return engine
}

// DefaultConfig returns the default CTIE configuration
func DefaultConfig() *Config {
	return &Config{
		EnableExecute:      true,
		EnableSpecialize:   true,
		EnableProof:        true,
		EnableDerive:       true,
		EnableAnalysis:     true,
		MaxExecutionTime:   1000, // 1 second
		MaxSpecializations: 100,
		DebugOutput:        false,
		OptimizationLevel:  2,
	}
}

// Process runs the CTIE optimization pass
func (e *Engine) Process() error {
	if e.config.DebugOutput {
		fmt.Println("=== Starting CTIE Pass ===")
	}
	
	// Phase 1: Analyze purity
	if err := e.analyzePurity(); err != nil {
		return fmt.Errorf("purity analysis failed: %v", err)
	}
	
	// Phase 2: Process @execute directives
	if e.config.EnableExecute {
		if err := e.processExecuteDirectives(); err != nil {
			return fmt.Errorf("@execute processing failed: %v", err)
		}
	}
	
	// Phase 3: Process @specialize directives
	if e.config.EnableSpecialize {
		if err := e.processSpecializeDirectives(); err != nil {
			return fmt.Errorf("@specialize processing failed: %v", err)
		}
	}
	
	// Phase 4: Process @proof directives
	if e.config.EnableProof {
		if err := e.processProofDirectives(); err != nil {
			return fmt.Errorf("@proof processing failed: %v", err)
		}
	}
	
	// Phase 5: Process @derive directives
	if e.config.EnableDerive {
		if err := e.processDeriveDirectives(); err != nil {
			return fmt.Errorf("@derive processing failed: %v", err)
		}
	}
	
	// Phase 6: Usage analysis and optimization
	if e.config.EnableAnalysis {
		if err := e.performUsageAnalysis(); err != nil {
			return fmt.Errorf("usage analysis failed: %v", err)
		}
	}
	
	if e.config.DebugOutput {
		e.printStatistics()
	}
	
	return nil
}

// analyzePurity performs purity analysis on all functions
func (e *Engine) analyzePurity() error {
	results := e.purity.AnalyzeModule()
	e.statistics.FunctionsAnalyzed = len(results)
	
	if e.config.DebugOutput {
		fmt.Println(e.purity.GetPurityReport())
	}
	
	return nil
}

// processExecuteDirectives handles @execute directives in interfaces
func (e *Engine) processExecuteDirectives() error {
	// Process interfaces from AST
	// For now, simplified implementation
	
	// Process ALL functions to find const call sites
	// We analyze all functions, not just pure ones, because
	// impure functions can still call pure functions with const args
	for _, fn := range e.module.Functions {
		if err := e.processFunctionExecute(fn); err != nil {
			return err
		}
	}
	
	return nil
}

// processInterfaceExecute handles @execute in an interface
func (e *Engine) processInterfaceExecute(iface *ast.InterfaceDecl) error {
	// Simplified for initial implementation
	// Full directive processing will come later
	if e.config.DebugOutput {
		fmt.Printf("Processing interface: %s\n", iface.Name)
	}
	
	return nil
}

// processFunctionExecute handles compile-time execution of a function
func (e *Engine) processFunctionExecute(fn *ir.Function) error {
	// Track constants through the function
	e.constTracker.Clear()
	e.constTracker.AnalyzeFunction(fn)
	
	// Get all const call sites
	constCalls := e.constTracker.GetConstCallSites()
	
	if e.config.DebugOutput && len(constCalls) > 0 {
		fmt.Printf("Found %d const call sites in %s\n", len(constCalls), fn.Name)
	}
	
	for _, call := range constCalls {
		// Check if the called function is pure
		if !e.purity.IsPure(call.Function) {
			continue
		}
		
		// Execute the function at compile time!
		result, err := e.executor.Execute(call.Function, call.ArgValues)
		if err != nil {
			if e.config.DebugOutput {
				fmt.Printf("Failed to execute %s at compile-time: %v\n", call.FunctionName, err)
			}
			continue
		}
		
		// Replace the call with the computed value!
		e.replaceCallWithValue(fn, call.InstIndex, result)
		e.statistics.FunctionsExecuted++
		e.statistics.ValuesComputed++
		
		if e.config.DebugOutput {
			fmt.Printf("✨ Executed %s at compile-time! Result: %v\n", call.FunctionName, result)
		}
	}
	
	return nil
}

// processSpecializeDirectives handles @specialize directives
func (e *Engine) processSpecializeDirectives() error {
	// Simplified implementation for now
	// Full specialization will come later
	if e.config.DebugOutput {
		fmt.Println("Processing @specialize directives...")
	}
	return nil
}

// processProofDirectives handles @proof directives
func (e *Engine) processProofDirectives() error {
	// Simplified implementation for now
	// Full proof checking will come later
	if e.config.DebugOutput {
		fmt.Println("Processing @proof directives...")
	}
	return nil
}

// processDeriveDirectives handles @derive directives
func (e *Engine) processDeriveDirectives() error {
	// Simplified implementation for now
	// Full derivation will come later
	if e.config.DebugOutput {
		fmt.Println("Processing @derive directives...")
	}
	return nil
}

// performUsageAnalysis analyzes interface usage patterns
func (e *Engine) performUsageAnalysis() error {
	// This would integrate with the usage analyzer
	// For now, just placeholder
	return nil
}

// Helper methods

// findImplementations finds all implementations of an interface method
func (e *Engine) findImplementations(ifaceName, methodName string) []*ir.Function {
	var impls []*ir.Function
	
	fullName := fmt.Sprintf("%s_%s", ifaceName, methodName)
	for _, fn := range e.module.Functions {
		// Check if function implements this interface method
		// TODO: Add proper attribute support
		if fn.Name == fullName || strings.HasSuffix(fn.Name, "_"+methodName) {
			impls = append(impls, fn)
		}
	}
	
	return impls
}

// findCallSites finds all call sites for a function
func (e *Engine) findCallSites(fnName string) []int {
	var callIndices []int
	
	for _, fn := range e.module.Functions {
		for i, inst := range fn.Instructions {
			if inst.Op == ir.OpCall && inst.Symbol == fnName {
				callIndices = append(callIndices, i)
			}
		}
	}
	
	return callIndices
}

// areArgumentsConst checks if all arguments to a call are const
func (e *Engine) areArgumentsConst(call *ir.Instruction) bool {
	// This would need to track const propagation
	// For now, return false (conservative)
	return false
}

// extractConstArguments extracts const argument values
func (e *Engine) extractConstArguments(call *ir.Instruction) []Value {
	// This would extract actual const values
	// For now, return empty
	return []Value{}
}

// replaceCallWithConstant replaces a call with its const result
func (e *Engine) replaceCallWithConstant(call *ir.Instruction, result Value) {
	// Replace the call instruction with a LoadConst
	call.Op = ir.OpLoadConst
	call.Imm = result.ToInt()
	call.Symbol = ""
	
	// Track bytes eliminated (approximate)
	e.statistics.BytesEliminated += 3 // CALL instruction size
}

// replaceCallWithValue replaces a call instruction with a constant value
func (e *Engine) replaceCallWithValue(fn *ir.Function, instIndex int, result Value) {
	if instIndex >= 0 && instIndex < len(fn.Instructions) {
		inst := &fn.Instructions[instIndex]
		
		// Save the original function name for comment
		origFunc := inst.Symbol
		
		// Replace CALL with LoadConst
		inst.Op = ir.OpLoadConst
		inst.Imm = result.ToInt()
		inst.Symbol = ""
		inst.Args = nil
		
		// Add comment about the optimization
		inst.Comment = fmt.Sprintf("CTIE: Computed at compile-time (was CALL %s)", origFunc)
		
		// Track optimization
		e.statistics.BytesEliminated += 3 // Approximate CALL size
	}
}

// printStatistics prints CTIE statistics
func (e *Engine) printStatistics() {
	fmt.Println("\n=== CTIE Statistics ===")
	fmt.Printf("Functions analyzed:     %d\n", e.statistics.FunctionsAnalyzed)
	fmt.Printf("Functions executed:     %d\n", e.statistics.FunctionsExecuted)
	fmt.Printf("Values computed:        %d\n", e.statistics.ValuesComputed)
	fmt.Printf("Bytes eliminated:       %d\n", e.statistics.BytesEliminated)
	fmt.Printf("Specializations:        %d\n", e.statistics.SpecializationsCreated)
	fmt.Printf("Proofs verified:        %d\n", e.statistics.ProofsVerified)
	fmt.Printf("Derivations generated:  %d\n", e.statistics.DerivationsGenerated)
	
	if e.statistics.BytesEliminated > 0 {
		fmt.Printf("\n✨ Saved %d bytes through compile-time execution!\n", e.statistics.BytesEliminated)
	}
}

// SetConfig updates the CTIE configuration
func (e *Engine) SetConfig(config *Config) {
	e.config = config
}

// GetStatistics returns CTIE statistics
func (e *Engine) GetStatistics() *Statistics {
	return e.statistics
}