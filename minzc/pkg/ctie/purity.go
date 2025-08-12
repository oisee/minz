// Package ctie implements Compile-Time Interface Execution for MinZ
// This revolutionary system enables interfaces to execute at compile-time,
// transforming from zero-cost to negative-cost abstractions.
package ctie

import (
	"fmt"
	"strings"
	"github.com/minz/minzc/pkg/ir"
)

// PurityLevel indicates how pure a function is
type PurityLevel int

const (
	// Impure - has side effects, cannot be executed at compile time
	Impure PurityLevel = iota
	// Pure - no side effects, can be executed at compile time
	Pure
	// Const - always returns the same value, can be memoized
	Const
)

// PurityAnalyzer determines which functions can be executed at compile-time
type PurityAnalyzer struct {
	module     *ir.Module
	cache      map[string]PurityLevel
	visiting   map[string]bool // For cycle detection
	depth      int
	maxDepth   int
}

// NewPurityAnalyzer creates a new purity analyzer
func NewPurityAnalyzer(module *ir.Module) *PurityAnalyzer {
	return &PurityAnalyzer{
		module:   module,
		cache:    make(map[string]PurityLevel),
		visiting: make(map[string]bool),
		maxDepth: 100, // Prevent infinite recursion
	}
}

// AnalyzeFunction determines the purity level of a function
func (p *PurityAnalyzer) AnalyzeFunction(fn *ir.Function) PurityLevel {
	if fn == nil {
		return Impure
	}

	// Check cache
	if level, ok := p.cache[fn.Name]; ok {
		return level
	}

	// Check for cycles
	if p.visiting[fn.Name] {
		// Assume pure for recursive functions (will verify in body)
		return Pure
	}

	// Mark as visiting
	p.visiting[fn.Name] = true
	defer func() {
		delete(p.visiting, fn.Name)
	}()

	// Check depth limit
	p.depth++
	if p.depth > p.maxDepth {
		p.cache[fn.Name] = Impure
		p.depth--
		return Impure
	}
	defer func() { p.depth-- }()

	// Check for @pure or @const annotations
	// TODO: Add attribute support to IR functions
	// For now, use naming convention
	if strings.HasPrefix(fn.Name, "pure_") {
		p.cache[fn.Name] = Pure
		return Pure
	}
	if strings.HasPrefix(fn.Name, "const_") {
		p.cache[fn.Name] = Const
		return Const
	}

	// Analyze function body
	level := p.analyzeBody(fn)
	
	// Cache result
	p.cache[fn.Name] = level
	return level
}

// analyzeBody checks all instructions in a function body
func (p *PurityAnalyzer) analyzeBody(fn *ir.Function) PurityLevel {
	isPure := true
	isConst := true

	for _, inst := range fn.Instructions {
		switch inst.Op {
		// Pure operations
		case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod:
			// Arithmetic is pure
			continue
		case ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpNot:
			// Bitwise operations are pure
			continue
		case ir.OpEq, ir.OpNe, ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe:
			// Comparisons are pure
			continue
		case ir.OpLoadConst:
			// Constant operations are pure
			continue

		// Stack/local operations (pure if no escaping)
		case ir.OpLoadVar, ir.OpStoreVar:
			// Variables are pure
			continue
		case ir.OpLoadParam:
			// Parameters are pure if not escaped
			continue

		// Function calls - check target purity
		case ir.OpCall:
			if target := p.findFunction(inst.Symbol); target != nil {
				targetPurity := p.AnalyzeFunction(target)
				if targetPurity == Impure {
					return Impure
				}
				if targetPurity == Pure {
					isConst = false // Not const if calling non-const function
				}
			} else {
				// Unknown function - assume impure
				return Impure
			}

		// Impure operations  
		// OpLoadGlobal and OpStoreGlobal don't exist, use generic Load/Store
		case ir.OpLoad, ir.OpStore:
			// Memory access might be impure (unless we can prove it's local)
			if !p.isLocalMemory(&inst) {
				return Impure
			}
			isConst = false

		// Control flow (pure if targets are pure)
		case ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot, ir.OpJumpIfZero, ir.OpJumpIfNotZero:
			// Jumps are pure
			continue
		case ir.OpReturn:
			// Return is pure
			continue

		// Special compile-time operations
		// TODO: Add compile-time specific opcodes

		default:
			// Unknown operation - be conservative
			return Impure
		}
	}

	if isConst {
		return Const
	}
	if isPure {
		return Pure
	}
	return Impure
}

// findFunction looks up a function by name
func (p *PurityAnalyzer) findFunction(name string) *ir.Function {
	if p.module == nil {
		return nil
	}
	for _, fn := range p.module.Functions {
		if fn.Name == name {
			return fn
		}
	}
	return nil
}

// isLocalMemory checks if a memory operation is provably local
func (p *PurityAnalyzer) isLocalMemory(inst *ir.Instruction) bool {
	// TODO: Add proper local memory analysis
	// For now, be conservative
	return false
}

// IsPure returns true if the function has no side effects
func (p *PurityAnalyzer) IsPure(fn *ir.Function) bool {
	level := p.AnalyzeFunction(fn)
	return level >= Pure
}

// IsConst returns true if the function always returns the same value
func (p *PurityAnalyzer) IsConst(fn *ir.Function) bool {
	level := p.AnalyzeFunction(fn)
	return level >= Const
}

// CanExecuteAtCompileTime returns true if function can be compile-time executed
func (p *PurityAnalyzer) CanExecuteAtCompileTime(fn *ir.Function) bool {
	// Must be pure
	if !p.IsPure(fn) {
		return false
	}

	// Check for naming conventions (temporary)
	// TODO: Add proper attribute support
	if strings.HasPrefix(fn.Name, "execute_") || strings.HasPrefix(fn.Name, "const_") {
		return true
	}

	// Default: pure functions can be executed at compile time
	return true
}

// AnalyzeModule analyzes all functions in a module
func (p *PurityAnalyzer) AnalyzeModule() map[string]PurityLevel {
	results := make(map[string]PurityLevel)
	
	for _, fn := range p.module.Functions {
		results[fn.Name] = p.AnalyzeFunction(fn)
	}
	
	return results
}

// GetPurityReport generates a human-readable purity report
func (p *PurityAnalyzer) GetPurityReport() string {
	report := "=== Purity Analysis Report ===\n\n"
	
	pureCount := 0
	constCount := 0
	impureCount := 0
	
	for name, level := range p.cache {
		switch level {
		case Const:
			report += fmt.Sprintf("✅ %s: CONST (can be memoized)\n", name)
			constCount++
		case Pure:
			report += fmt.Sprintf("✅ %s: PURE (can execute at compile-time)\n", name)
			pureCount++
		case Impure:
			report += fmt.Sprintf("❌ %s: IMPURE (has side effects)\n", name)
			impureCount++
		}
	}
	
	report += fmt.Sprintf("\n=== Summary ===\n")
	report += fmt.Sprintf("Const functions:  %d\n", constCount)
	report += fmt.Sprintf("Pure functions:   %d\n", pureCount)
	report += fmt.Sprintf("Impure functions: %d\n", impureCount)
	report += fmt.Sprintf("Total:            %d\n", constCount+pureCount+impureCount)
	
	if constCount+pureCount > 0 {
		pct := float64(constCount+pureCount) * 100.0 / float64(constCount+pureCount+impureCount)
		report += fmt.Sprintf("\n%.1f%% of functions can be executed at compile-time!\n", pct)
	}
	
	return report
}

// MarkPure manually marks a function as pure (for builtins)
func (p *PurityAnalyzer) MarkPure(name string) {
	p.cache[name] = Pure
}

// MarkConst manually marks a function as const (for builtins)
func (p *PurityAnalyzer) MarkConst(name string) {
	p.cache[name] = Const
}

// MarkImpure manually marks a function as impure
func (p *PurityAnalyzer) MarkImpure(name string) {
	p.cache[name] = Impure
}

// InitBuiltins marks standard library functions with known purity
func (p *PurityAnalyzer) InitBuiltins() {
	// Pure math functions
	p.MarkPure("abs")
	p.MarkPure("min")
	p.MarkPure("max")
	p.MarkPure("clamp")
	
	// Const functions (no inputs)
	p.MarkConst("pi")
	p.MarkConst("e")
	
	// Impure I/O functions
	p.MarkImpure("print")
	p.MarkImpure("print_u8")
	p.MarkImpure("print_u16")
	p.MarkImpure("input")
	p.MarkImpure("random")
	
	// Memory operations (context-dependent)
	p.MarkImpure("malloc")
	p.MarkImpure("free")
}