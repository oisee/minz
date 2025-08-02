package optimizer

import (
	"fmt"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/semantic"
)

// MultiLevelOptimizer orchestrates optimization across all compilation stages
type MultiLevelOptimizer struct {
	semanticPasses []SemanticPass
	mirPasses      []Pass
	asmPasses      []AssemblyPass
	config         OptimizationConfig
	history        *OptimizationHistory
}

// OptimizationConfig controls optimization behavior
type OptimizationConfig struct {
	Level              OptimizationLevel
	MaxIterations      int
	EnableProfiling    bool
	EnableVerification bool
	CostWeights        CostWeights
}

// CostWeights for optimization decisions
type CostWeights struct {
	Cycles    float64
	Size      float64
	Registers float64
}

// OptimizationHistory tracks applied optimizations to prevent loops
type OptimizationHistory struct {
	appliedPatterns map[string]bool
	costHistory     []Cost
	iterationCount  int
}

// Cost represents the cost of code
type Cost struct {
	Cycles    int
	Size      int
	Registers int
}

// SemanticPass operates on AST
type SemanticPass interface {
	Name() string
	Apply(file *ast.File) (*ast.File, bool)
	EstimateCost(file *ast.File) Cost
}

// AssemblyPass operates on assembly code
type AssemblyPass interface {
	Name() string
	Apply(asm []AssemblyLine) ([]AssemblyLine, bool)
	EstimateCost(asm []AssemblyLine) Cost
}

// AssemblyLine represents a line of assembly code
type AssemblyLine struct {
	Label       string
	Instruction string
	Operands    []string
	Comment     string
}

// NewMultiLevelOptimizer creates a new multi-level optimizer
func NewMultiLevelOptimizer(config OptimizationConfig) *MultiLevelOptimizer {
	opt := &MultiLevelOptimizer{
		config: config,
		history: &OptimizationHistory{
			appliedPatterns: make(map[string]bool),
			costHistory:     []Cost{},
		},
	}
	
	// Initialize passes based on optimization level
	if config.Level >= OptLevelBasic {
		opt.initializeBasicPasses()
	}
	
	if config.Level >= OptLevelFull {
		opt.initializeAdvancedPasses()
	}
	
	return opt
}

// initializeBasicPasses sets up basic optimization passes
func (o *MultiLevelOptimizer) initializeBasicPasses() {
	// Semantic passes
	o.semanticPasses = append(o.semanticPasses,
		NewConstantFoldingSemanticPass(),
		NewDeadCodeEliminationSemanticPass(),
		NewSimpleInliningPass(),
	)
	
	// MIR passes (reuse existing)
	o.mirPasses = append(o.mirPasses,
		NewConstantFoldingPass(),
		NewDeadCodeEliminationPass(),
	)
	
	// Assembly passes
	o.asmPasses = append(o.asmPasses,
		NewZ80PeepholePass(),
		NewRedundantLoadStorePass(),
	)
}

// initializeAdvancedPasses sets up advanced optimization passes
func (o *MultiLevelOptimizer) initializeAdvancedPasses() {
	// Additional semantic passes
	o.semanticPasses = append(o.semanticPasses,
		NewLambdaLiftingSemanticPass(),
		NewLoopOptimizationPass(),
		NewCompileTimeEvaluationPass(),
		NewPatternBasedTransformationPass(),
	)
	
	// Additional MIR passes
	o.mirPasses = append(o.mirPasses,
		NewMIRReorderingPass(),
		NewSmartPeepholeOptimizationPass(),
		NewRegisterAllocationPass(),
		NewInliningPass(),
		NewTrueSMCPass(false),
		NewTailRecursionPass(),
	)
	
	// Additional assembly passes
	o.asmPasses = append(o.asmPasses,
		NewZ80InstructionSelectionPass(),
		NewFlagOptimizationPass(),
		NewAddressingModePass(),
		NewFinalPeepholePass(),
	)
}

// OptimizeProgram runs the complete multi-level optimization pipeline
func (o *MultiLevelOptimizer) OptimizeProgram(file *ast.File) (*ir.Module, error) {
	// Phase 1: Semantic optimization
	optimizedAST, err := o.optimizeSemantic(file)
	if err != nil {
		return nil, fmt.Errorf("semantic optimization failed: %w", err)
	}
	
	// Phase 2: Generate initial MIR
	analyzer := semantic.NewAnalyzer()
	module, err := analyzer.Analyze(optimizedAST)
	if err != nil {
		return nil, fmt.Errorf("MIR generation failed: %w", err)
	}
	
	// Phase 3: MIR optimization
	optimizedMIR, err := o.optimizeMIR(module)
	if err != nil {
		return nil, fmt.Errorf("MIR optimization failed: %w", err)
	}
	
	// Phase 4: Assembly optimization will be done in code generator
	// Store optimization state for code generator (metadata approach)
	if optimizedMIR.Functions != nil && len(optimizedMIR.Functions) > 0 {
		optimizedMIR.Functions[0].SetMetadata("optimization_state", "applied")
	}
	
	return optimizedMIR, nil
}

// optimizeSemantic runs semantic-level optimizations
func (o *MultiLevelOptimizer) optimizeSemantic(file *ast.File) (*ast.File, error) {
	return o.runUntilFixpoint(file, o.semanticPasses, "semantic")
}

// optimizeMIR runs MIR-level optimizations
func (o *MultiLevelOptimizer) optimizeMIR(module *ir.Module) (*ir.Module, error) {
	maxIterations := o.config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}
	
	for iteration := 0; iteration < maxIterations; iteration++ {
		changed := false
		initialCost := o.calculateMIRCost(module)
		
		for _, pass := range o.mirPasses {
			passChanged, err := pass.Run(module)
			if err != nil {
				return nil, fmt.Errorf("pass %s failed: %w", pass.Name(), err)
			}
			
			if passChanged {
				newCost := o.calculateMIRCost(module)
				
				// Check if optimization improved or maintained cost
				if o.shouldAcceptChange(initialCost, newCost, pass.Name()) {
					changed = true
					o.recordOptimization(pass.Name(), iteration)
				} else {
					// Revert changes (would need module cloning)
					// For now, trust that passes are beneficial
				}
			}
		}
		
		if !changed {
			break // Fixpoint reached
		}
		
		// Check for oscillation
		if o.detectOscillation() {
			break
		}
	}
	
	return module, nil
}

// runUntilFixpoint runs passes until no changes occur
func (o *MultiLevelOptimizer) runUntilFixpoint(
	file *ast.File,
	passes []SemanticPass,
	phase string,
) (*ast.File, error) {
	maxIterations := o.config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}
	
	for iteration := 0; iteration < maxIterations; iteration++ {
		changed := false
		initialCost := o.calculateSemanticCost(file)
		
		for _, pass := range passes {
			newFile, passChanged := pass.Apply(file)
			
			if passChanged {
				newCost := pass.EstimateCost(newFile)
				
				if o.shouldAcceptChange(initialCost, newCost, pass.Name()) {
					file = newFile
					changed = true
					o.recordOptimization(pass.Name(), iteration)
				}
			}
		}
		
		if !changed {
			break // Fixpoint reached
		}
		
		if o.detectOscillation() {
			break
		}
	}
	
	return file, nil
}

// shouldAcceptChange decides whether to accept an optimization
func (o *MultiLevelOptimizer) shouldAcceptChange(before, after Cost, passName string) bool {
	// Calculate weighted cost
	beforeScore := o.calculateScore(before)
	afterScore := o.calculateScore(after)
	
	// Accept if:
	// 1. Strictly better
	if afterScore < beforeScore {
		return true
	}
	
	// 2. Equal cost but reduces a critical metric
	if afterScore == beforeScore {
		// Prefer fewer registers (helps prevent spills)
		if after.Registers < before.Registers {
			return true
		}
		// Prefer smaller code
		if after.Size < before.Size {
			return true
		}
	}
	
	// 3. Special cases for specific passes
	switch passName {
	case "Lambda Lifting":
		// Always accept lambda lifting (enables other opts)
		return true
	case "Dead Code Elimination":
		// Always accept if size decreased
		return after.Size <= before.Size
	}
	
	return false
}

// calculateScore computes weighted cost score
func (o *MultiLevelOptimizer) calculateScore(cost Cost) float64 {
	return float64(cost.Cycles)*o.config.CostWeights.Cycles +
		float64(cost.Size)*o.config.CostWeights.Size +
		float64(cost.Registers)*o.config.CostWeights.Registers
}

// detectOscillation checks if optimization is stuck in a loop
func (o *MultiLevelOptimizer) detectOscillation() bool {
	if len(o.history.costHistory) < 4 {
		return false
	}
	
	// Check if we've seen the same cost pattern before
	recent := o.history.costHistory[len(o.history.costHistory)-4:]
	
	// Simple check: if costs alternate between two values
	if recent[0] == recent[2] && recent[1] == recent[3] {
		return true
	}
	
	return false
}

// recordOptimization tracks applied optimizations
func (o *MultiLevelOptimizer) recordOptimization(passName string, iteration int) {
	key := fmt.Sprintf("%s:%d", passName, iteration)
	o.history.appliedPatterns[key] = true
	o.history.iterationCount++
}

// Cost calculation helpers

func (o *MultiLevelOptimizer) calculateSemanticCost(file *ast.File) Cost {
	// Estimate cost at AST level - simplified
	cost := Cost{}
	
	// Rough estimation based on declarations
	for _, decl := range file.Declarations {
		if fn, ok := decl.(*ast.FunctionDecl); ok {
			if fn.Body != nil {
				cost.Size += len(fn.Body.Statements)
				cost.Cycles += len(fn.Body.Statements) * 10
			}
		}
	}
	
	return cost
}

func (o *MultiLevelOptimizer) calculateMIRCost(module *ir.Module) Cost {
	cost := Cost{}
	
	for _, fn := range module.Functions {
		for _, inst := range fn.Instructions {
			cost.Size++
			cost.Cycles += estimateInstructionCycles(inst)
		}
		
		// Estimate register pressure
		cost.Registers += fn.UsedRegisters.Count()
	}
	
	return cost
}

// estimateInstructionCycles estimates cycles for a MIR instruction
func estimateInstructionCycles(inst ir.Instruction) int {
	switch inst.Op {
	case ir.OpLoadConst:
		return 7 // LD reg, n
	case ir.OpAdd, ir.OpSub:
		return 4 // ADD/SUB
	case ir.OpMul:
		return 40 // Rough estimate for 8-bit multiply
	case ir.OpCall:
		return 17 // CALL
	case ir.OpLoadVar, ir.OpStoreVar:
		return 13 // LD reg, (addr)
	default:
		return 4 // Default estimate
	}
}

// costVisitor removed - using simplified cost calculation instead

// Stub implementations for demonstration

type ConstantFoldingSemanticPass struct{}

func NewConstantFoldingSemanticPass() *ConstantFoldingSemanticPass {
	return &ConstantFoldingSemanticPass{}
}

func (p *ConstantFoldingSemanticPass) Name() string { return "Constant Folding (Semantic)" }

func (p *ConstantFoldingSemanticPass) Apply(file *ast.File) (*ast.File, bool) {
	// Implementation would fold constant expressions
	return file, false
}

func (p *ConstantFoldingSemanticPass) EstimateCost(file *ast.File) Cost {
	// Estimate cost after constant folding
	return Cost{}
}

// Additional pass stubs...

type Z80PeepholePass struct{}

func NewZ80PeepholePass() *Z80PeepholePass {
	return &Z80PeepholePass{}
}

func (p *Z80PeepholePass) Name() string { return "Z80 Peephole" }

func (p *Z80PeepholePass) Apply(asm []AssemblyLine) ([]AssemblyLine, bool) {
	// Z80-specific peephole patterns
	changed := false
	result := []AssemblyLine{}
	
	for i := 0; i < len(asm); i++ {
		// Example pattern: LD A,B; LD B,A -> LD A,B
		if i+1 < len(asm) &&
			asm[i].Instruction == "LD" && asm[i].Operands[0] == "A" &&
			asm[i+1].Instruction == "LD" && asm[i+1].Operands[1] == "A" &&
			asm[i].Operands[1] == asm[i+1].Operands[0] {
			// Skip redundant instruction
			result = append(result, asm[i])
			i++ // Skip next instruction
			changed = true
		} else {
			result = append(result, asm[i])
		}
	}
	
	return result, changed
}

func (p *Z80PeepholePass) EstimateCost(asm []AssemblyLine) Cost {
	cost := Cost{Size: len(asm)}
	for _, line := range asm {
		cost.Cycles += getInstructionCycles(line.Instruction)
	}
	return cost
}

func getInstructionCycles(inst string) int {
	// Z80 cycle counts
	switch inst {
	case "LD":
		return 4
	case "ADD", "SUB":
		return 4
	case "INC", "DEC":
		return 4
	case "CALL":
		return 17
	case "RET":
		return 10
	default:
		return 4
	}
}