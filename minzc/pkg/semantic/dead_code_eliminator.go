package semantic

import (
	"fmt"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// DeadCodeEliminator removes unreachable code and unused variables
type DeadCodeEliminator struct {
	usedVars    map[string]bool
	reachable   map[*ir.Instruction]bool
	sideEffects map[string]bool
	metrics     *OptimizationMetrics
	analyzer    *Analyzer
}

// NewDeadCodeEliminator creates a new dead code eliminator
func NewDeadCodeEliminator(analyzer *Analyzer, metrics *OptimizationMetrics) *DeadCodeEliminator {
	return &DeadCodeEliminator{
		usedVars:    make(map[string]bool),
		reachable:   make(map[*ir.Instruction]bool),
		sideEffects: make(map[string]bool),
		metrics:     metrics,
		analyzer:    analyzer,
	}
}

// EliminateDeadCode performs dead code elimination on the entire module
func (dce *DeadCodeEliminator) EliminateDeadCode(module *ir.Module) {
	// Initialize side effect analysis
	dce.initializeSideEffectAnalysis(module)
	
	// Process each function
	for _, function := range module.Functions {
		dce.eliminateDeadCodeInFunction(function)
	}
	
	// Remove unused global variables
	dce.eliminateUnusedGlobals(module)
}

// eliminateDeadCodeInFunction performs dead code elimination within a function
func (dce *DeadCodeEliminator) eliminateDeadCodeInFunction(function *ir.Function) {
	originalCount := len(function.Instructions)
	
	// Phase 1: Mark reachable instructions (forward pass)
	dce.markReachableInstructions(function)
	
	// Phase 2: Mark used variables (backward pass)
	dce.markUsedVariables(function)
	
	// Phase 3: Remove dead instructions
	function.Instructions = dce.removeDeadInstructions(function.Instructions)
	
	// Phase 4: Remove unused local variables
	dce.removeUnusedLocals(function)
	
	eliminatedInstructions := originalCount - len(function.Instructions)
	if eliminatedInstructions > 0 {
		dce.metrics.DeadCodeEliminated += eliminatedInstructions
		
		// Estimate performance savings
		cycleSavings := dce.estimateCycleSavings(eliminatedInstructions)
		dce.metrics.CyclesSavedFolding += cycleSavings
	}
}

// markReachableInstructions performs forward analysis to find reachable code
func (dce *DeadCodeEliminator) markReachableInstructions(function *ir.Function) {
	// Clear previous analysis
	dce.reachable = make(map[*ir.Instruction]bool)
	
	// All instructions start as reachable (simplified analysis)
	// In a more sophisticated implementation, we would build a control flow graph
	for i := range function.Instructions {
		inst := &function.Instructions[i]
		dce.reachable[inst] = true
		
		// Mark unreachable code after unconditional jumps/returns
		if dce.isUnconditionalJump(inst) {
			// Mark subsequent instructions as unreachable until next label
			for j := i + 1; j < len(function.Instructions); j++ {
				nextInst := &function.Instructions[j]
				if dce.isLabel(nextInst) {
					break // Labels are reachable entry points
				}
				dce.reachable[nextInst] = false
			}
		}
	}
}

// markUsedVariables performs backward analysis to find used variables
func (dce *DeadCodeEliminator) markUsedVariables(function *ir.Function) {
	dce.usedVars = make(map[string]bool)
	
	// Backward pass: mark variables that are used
	for i := len(function.Instructions) - 1; i >= 0; i-- {
		inst := &function.Instructions[i]
		
		if !dce.reachable[inst] {
			continue // Skip unreachable instructions
		}
		
		// Mark variables used by this instruction
		dce.markUsesInInstruction(inst)
		
		// If instruction defines a variable that's not used, it's dead
		// (unless it has side effects)
		if dce.definesVariable(inst) && !dce.hasSideEffects(inst) {
			varName := dce.getDefinedVariable(inst)
			if !dce.usedVars[varName] {
				dce.reachable[inst] = false // Mark as dead
			}
		}
	}
}

// markUsesInInstruction marks all variables used by an instruction
func (dce *DeadCodeEliminator) markUsesInInstruction(inst *ir.Instruction) {
	switch inst.Op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod:
		// Binary operations use two registers
		dce.usedVars[fmt.Sprintf("r%d", inst.Left)] = true
		dce.usedVars[fmt.Sprintf("r%d", inst.Right)] = true
		
	case ir.OpLoad:
		// Load uses a variable
		dce.usedVars[inst.Source] = true
		
	case ir.OpStore:
		// Store uses the source register
		dce.usedVars[fmt.Sprintf("r%d", inst.Source)] = true
		
	case ir.OpCall:
		// Function calls use their arguments
		for _, arg := range inst.Args {
			dce.usedVars[fmt.Sprintf("r%d", arg)] = true
		}
		
	case ir.OpRet:
		// Return uses the return value register
		if inst.Source != 0 {
			dce.usedVars[fmt.Sprintf("r%d", inst.Source)] = true
		}
		
	case ir.OpBranch:
		// Conditional branches use condition register
		if inst.Condition != 0 {
			dce.usedVars[fmt.Sprintf("r%d", inst.Condition)] = true
		}
	}
}

// definesVariable checks if an instruction defines a variable
func (dce *DeadCodeEliminator) definesVariable(inst *ir.Instruction) bool {
	switch inst.Op {
	case ir.OpLoadConst, ir.OpLoad, ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod, ir.OpCall:
		return inst.Dest != 0
	}
	return false
}

// getDefinedVariable returns the variable defined by an instruction
func (dce *DeadCodeEliminator) getDefinedVariable(inst *ir.Instruction) string {
	if inst.Dest != 0 {
		return fmt.Sprintf("r%d", inst.Dest)
	}
	return ""
}

// hasSideEffects determines if an instruction has side effects
func (dce *DeadCodeEliminator) hasSideEffects(inst *ir.Instruction) bool {
	switch inst.Op {
	case ir.OpStore:
		return true // Memory writes have side effects
	case ir.OpCall:
		// Check if the called function has side effects
		return dce.sideEffects[inst.Target]
	case ir.OpRet:
		return true // Returns have control flow side effects
	case ir.OpBranch, ir.OpJump:
		return true // Control flow instructions have side effects
	}
	return false
}

// removeDeadInstructions filters out dead instructions
func (dce *DeadCodeEliminator) removeDeadInstructions(instructions []ir.Instruction) []ir.Instruction {
	var filtered []ir.Instruction
	
	for i := range instructions {
		inst := &instructions[i]
		if dce.reachable[inst] {
			filtered = append(filtered, *inst)
		}
	}
	
	return filtered
}

// removeUnusedLocals removes unused local variable declarations
func (dce *DeadCodeEliminator) removeUnusedLocals(function *ir.Function) {
	var filteredLocals []*ir.Variable
	
	for _, local := range function.Locals {
		if dce.usedVars[local.Name] {
			filteredLocals = append(filteredLocals, local)
		} else {
			dce.metrics.DeadCodeEliminated++
		}
	}
	
	function.Locals = filteredLocals
}

// eliminateUnusedGlobals removes unused global variables
func (dce *DeadCodeEliminator) eliminateUnusedGlobals(module *ir.Module) {
	usedGlobals := make(map[string]bool)
	
	// Mark globals used in functions
	for _, function := range module.Functions {
		for _, inst := range function.Instructions {
			if inst.Op == ir.OpLoad && dce.isGlobalVariable(inst.Source) {
				usedGlobals[inst.Source] = true
			}
			if inst.Op == ir.OpStore && dce.isGlobalVariable(inst.Target) {
				usedGlobals[inst.Target] = true
			}
		}
	}
	
	// Filter unused globals
	var filteredGlobals []*ir.Global
	for _, global := range module.Globals {
		if usedGlobals[global.Name] || global.IsExported {
			filteredGlobals = append(filteredGlobals, global)
		} else {
			dce.metrics.DeadCodeEliminated++
		}
	}
	
	module.Globals = filteredGlobals
}

// initializeSideEffectAnalysis builds the side effect database
func (dce *DeadCodeEliminator) initializeSideEffectAnalysis(module *ir.Module) {
	dce.sideEffects = make(map[string]bool)
	
	// Analyze each function for side effects
	for _, function := range module.Functions {
		hasSideEffect := dce.analyzeFunctionSideEffects(function)
		dce.sideEffects[function.Name] = hasSideEffect
	}
	
	// Built-in functions with known side effects
	dce.sideEffects["print_u8"] = true
	dce.sideEffects["print_u16"] = true
	dce.sideEffects["print_string"] = true
	dce.sideEffects["malloc"] = true
	dce.sideEffects["free"] = true
	
	// Pure mathematical functions
	dce.sideEffects["add"] = false
	dce.sideEffects["sub"] = false
	dce.sideEffects["mul"] = false
	dce.sideEffects["abs"] = false
	dce.sideEffects["sqrt"] = false
}

// analyzeFunctionSideEffects determines if a function has side effects
func (dce *DeadCodeEliminator) analyzeFunctionSideEffects(function *ir.Function) bool {
	for _, inst := range function.Instructions {
		switch inst.Op {
		case ir.OpStore:
			// Writing to memory is a side effect
			return true
		case ir.OpCall:
			// Recursive check - if we call a function with side effects
			if dce.sideEffects[inst.Target] {
				return true
			}
		}
	}
	return false
}

// Utility functions

func (dce *DeadCodeEliminator) isUnconditionalJump(inst *ir.Instruction) bool {
	return inst.Op == ir.OpJump || inst.Op == ir.OpRet
}

func (dce *DeadCodeEliminator) isLabel(inst *ir.Instruction) bool {
	return inst.Op == ir.OpLabel
}

func (dce *DeadCodeEliminator) isGlobalVariable(name string) bool {
	// Simplified check - in practice, would consult symbol table
	return len(name) > 0 && name[0] != 'r' // Not a register
}

func (dce *DeadCodeEliminator) estimateCycleSavings(eliminatedInstructions int) int {
	// Conservative estimate: average 5 T-states per eliminated instruction
	return eliminatedInstructions * 5
}

// EliminateDeadCodeInAST performs dead code elimination at the AST level
func (dce *DeadCodeEliminator) EliminateDeadCodeInAST(node ast.Node) ast.Node {
	switch n := node.(type) {
	case *ast.BlockStmt:
		return dce.eliminateDeadStatementsInBlock(n)
	case *ast.IfStmt:
		return dce.eliminateDeadBranchesInIf(n)
	case *ast.ForStmt:
		return dce.eliminateDeadLoops(n)
	default:
		return node
	}
}

// eliminateDeadStatementsInBlock removes unreachable statements from a block
func (dce *DeadCodeEliminator) eliminateDeadStatementsInBlock(block *ast.BlockStmt) *ast.BlockStmt {
	var filteredStmts []ast.Statement
	reachable := true
	
	for _, stmt := range block.Statements {
		if !reachable {
			// Skip unreachable statements
			dce.metrics.DeadCodeEliminated++
			continue
		}
		
		filteredStmts = append(filteredStmts, stmt)
		
		// Check if this statement makes subsequent code unreachable
		if dce.isUnconditionalExit(stmt) {
			reachable = false
		}
	}
	
	return &ast.BlockStmt{Statements: filteredStmts}
}

// eliminateDeadBranchesInIf removes dead branches from if statements
func (dce *DeadCodeEliminator) eliminateDeadBranchesInIf(ifStmt *ast.IfStmt) ast.Statement {
	// Check if condition is a compile-time constant
	if constCond := dce.evaluateConstantCondition(ifStmt.Condition); constCond != nil {
		dce.metrics.DeadCodeEliminated++
		
		if constCond.BoolVal {
			// Condition is always true - keep then branch
			return ifStmt.ThenBranch
		} else {
			// Condition is always false - keep else branch (if any)
			if ifStmt.ElseBranch != nil {
				return ifStmt.ElseBranch
			} else {
				// No else branch - return empty block
				return &ast.BlockStmt{Statements: []ast.Statement{}}
			}
		}
	}
	
	// Cannot eliminate - return original
	return ifStmt
}

// eliminateDeadLoops removes loops that never execute
func (dce *DeadCodeEliminator) eliminateDeadLoops(forStmt *ast.ForStmt) ast.Statement {
	// Check for loops with constant false conditions
	if constCond := dce.evaluateConstantCondition(forStmt.Condition); constCond != nil {
		if !constCond.BoolVal {
			// Loop never executes
			dce.metrics.DeadCodeEliminated++
			return &ast.BlockStmt{Statements: []ast.Statement{}}
		}
	}
	
	return forStmt
}

// evaluateConstantCondition attempts to evaluate a condition as a constant
func (dce *DeadCodeEliminator) evaluateConstantCondition(condition ast.Expression) *ir.Value {
	// This would integrate with the constant folder
	// For now, simplified version
	if literal, ok := condition.(*ast.LiteralExpr); ok {
		if literal.Type == ast.BoolLiteral {
			return &ir.Value{
				Type:    ir.Type{Kind: ir.TypeBool},
				BoolVal: literal.BoolValue,
			}
		}
	}
	
	return nil
}

// isUnconditionalExit checks if a statement unconditionally exits the current block
func (dce *DeadCodeEliminator) isUnconditionalExit(stmt ast.Statement) bool {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		// Check for calls to functions that never return
		if call, ok := s.Expression.(*ast.CallExpr); ok {
			if ident, ok := call.Function.(*ast.IdentifierExpr); ok {
				// Functions that never return
				neverReturn := map[string]bool{
					"exit":  true,
					"panic": true,
					"abort": true,
				}
				return neverReturn[ident.Name]
			}
		}
	}
	return false
}