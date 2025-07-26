package optimizer

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// TrueSMCPass implements TRUE SMC (истинный SMC) with immediate patching
// as described in ADR-001 and SPEC v0.1
type TrueSMCPass struct {
	enabled      bool
	anchors      map[string]*SMCAnchor  // function -> anchors
	patchTable   []PatchEntry
	diagnostics  bool
}

// SMCAnchor represents a parameter anchor point in code
type SMCAnchor struct {
	FunctionName string
	ParamName    string
	ParamIndex   int
	Type         ir.Type
	FirstUseInst int           // Index of first use instruction
	Address      uint16        // Address in generated code (filled during codegen)
	IsImm8       bool          // true for 8-bit immediate
	IsImm16      bool          // true for 16-bit immediate
	Instruction  ir.Opcode     // The instruction that contains the immediate
	Symbol       string        // Anchor symbol (e.g., "x$imm0")
}

// PatchEntry represents an entry in the PATCH-TABLE
type PatchEntry struct {
	Symbol    string    // Anchor symbol (e.g., "x$imm0")
	Address   uint16    // Address to patch
	Size      uint8     // 1 or 2 bytes
	Bank      uint8     // Memory bank
	ParamTag  string    // Parameter name
	Function  string    // Function name
}

// CFGNode represents a node in the control flow graph
type CFGNode struct {
	Instructions []int          // Indices into function's instruction array
	Successors   []*CFGNode
	Predecessors []*CFGNode
	Dominates    []*CFGNode     // Nodes this node dominates
	IDom         *CFGNode       // Immediate dominator
}

// NewTrueSMCPass creates a new TRUE SMC optimization pass
func NewTrueSMCPass(diagnostics bool) Pass {
	return &TrueSMCPass{
		enabled:     true,
		anchors:     make(map[string]*SMCAnchor),
		diagnostics: diagnostics,
	}
}

// Name returns the name of this pass
func (p *TrueSMCPass) Name() string {
	return "TRUE SMC Anchor Optimization"
}

// Run performs TRUE SMC optimization on the module
func (p *TrueSMCPass) Run(module *ir.Module) (bool, error) {
	if !p.enabled {
		return false, nil
	}

	changed := false
	
	for _, function := range module.Functions {
		// Skip functions that can't use SMC
		if !function.IsSMCEnabled || function.Name == "" {
			continue
		}
		
		// Analyze and transform function to use TRUE SMC anchors
		if p.transformFunction(function) {
			changed = true
		}
	}
	
	// Store patch table in module for code generation
	module.PatchTable = p.convertPatchTable()
	
	return changed, nil
}

// transformFunction transforms a function to use TRUE SMC anchors
func (p *TrueSMCPass) transformFunction(fn *ir.Function) bool {
	// Build control flow graph
	cfg := p.buildCFG(fn)
	
	// Find dominators
	p.computeDominators(cfg)
	
	// Find anchor points for each parameter
	anchors := make([]*SMCAnchor, 0, len(fn.Params))
	for i, param := range fn.Params {
		anchor := p.findAnchorPoint(fn, param, i, cfg)
		if anchor != nil {
			anchors = append(anchors, anchor)
			anchorKey := fmt.Sprintf("%s_%s", fn.Name, param.Name)
			p.anchors[anchorKey] = anchor
			
			if p.diagnostics {
				fmt.Printf("TRUE SMC: Created anchor for param %s (index %d) with symbol %s\n", param.Name, i, anchor.Symbol)
			}
		}
	}
	
	// Transform instructions to use anchors
	return p.insertAnchors(fn, anchors)
}

// buildCFG builds a control flow graph for the function
func (p *TrueSMCPass) buildCFG(fn *ir.Function) *CFGNode {
	// Simplified CFG construction - in real implementation would handle all control flow
	root := &CFGNode{
		Instructions: make([]int, 0),
	}
	
	current := root
	for i, inst := range fn.Instructions {
		current.Instructions = append(current.Instructions, i)
		
		// Handle control flow
		switch inst.Op {
		case ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot:
			// Create new basic block after jump
			newNode := &CFGNode{
				Instructions: make([]int, 0),
			}
			current.Successors = append(current.Successors, newNode)
			newNode.Predecessors = append(newNode.Predecessors, current)
			current = newNode
		case ir.OpReturn:
			// End of basic block
			if i < len(fn.Instructions)-1 {
				newNode := &CFGNode{
					Instructions: make([]int, 0),
				}
				current = newNode
			}
		}
	}
	
	return root
}

// computeDominators computes dominator tree for CFG
func (p *TrueSMCPass) computeDominators(root *CFGNode) {
	// Simplified dominator computation
	// In real implementation would use proper algorithm
	root.IDom = nil
	
	// Mark root as dominating all reachable nodes
	visited := make(map[*CFGNode]bool)
	var markDominated func(node *CFGNode)
	markDominated = func(node *CFGNode) {
		if visited[node] {
			return
		}
		visited[node] = true
		
		for _, succ := range node.Successors {
			if succ.IDom == nil && succ != root {
				succ.IDom = node
				node.Dominates = append(node.Dominates, succ)
			}
			markDominated(succ)
		}
	}
	markDominated(root)
}

// findAnchorPoint finds the anchor point for a parameter
func (p *TrueSMCPass) findAnchorPoint(fn *ir.Function, param ir.Parameter, paramIndex int, cfg *CFGNode) *SMCAnchor {
	// Find first use of parameter in dominator tree
	firstUse := -1
	var firstUseInst *ir.Instruction
	
	for i, inst := range fn.Instructions {
		// Check if instruction uses this parameter
		if p.usesParameter(inst, paramIndex) {
			if firstUse == -1 {
				firstUse = i
				firstUseInst = &fn.Instructions[i]
			} else {
				// Check if this use dominates the previous first use
				// Simplified check - in real implementation would use proper dominance
				if i < firstUse {
					firstUse = i
					firstUseInst = &fn.Instructions[i]
				}
			}
		}
	}
	
	if firstUse == -1 {
		return nil // Parameter not used
	}
	
	// Determine if we can use immediate operand
	isImm8, isImm16, opcode := p.canUseImmediate(firstUseInst, param.Type)
	
	if !isImm8 && !isImm16 {
		// Need to insert synthetic anchor
		return p.createSyntheticAnchor(fn, param, paramIndex, firstUse)
	}
	
	return &SMCAnchor{
		FunctionName: fn.Name,
		ParamName:    param.Name,
		ParamIndex:   paramIndex,
		Type:         param.Type,
		FirstUseInst: firstUse,
		IsImm8:       isImm8,
		IsImm16:      isImm16,
		Instruction:  opcode,
		Symbol:       fmt.Sprintf("%s$imm0", param.Name),
	}
}

// usesParameter checks if instruction uses the given parameter
func (p *TrueSMCPass) usesParameter(inst ir.Instruction, paramIndex int) bool {
	// Check if instruction loads this parameter
	if inst.Op == ir.OpLoadParam && inst.Src1 == ir.Register(paramIndex) {
		return true
	}
	
	// Check if this is a virtual register that was loaded from parameter
	// This would require data flow analysis in real implementation
	
	return false
}

// canUseImmediate checks if instruction can use immediate operand
func (p *TrueSMCPass) canUseImmediate(inst *ir.Instruction, paramType ir.Type) (bool, bool, ir.Opcode) {
	switch inst.Op {
	case ir.OpLoadParam:
		// We'll transform this to immediate load
		if paramType.Size() == 1 {
			return true, false, ir.OpLoadConst // Will become LD A, n
		} else {
			return false, true, ir.OpLoadConst // Will become LD HL, nn
		}
		
	case ir.OpAdd, ir.OpSub:
		// Can use immediate if one operand is parameter
		if paramType.Size() == 1 {
			return true, false, inst.Op // ADD A, n or SUB n
		}
		
	case ir.OpCmp:
		// Can use immediate compare
		if paramType.Size() == 1 {
			return true, false, inst.Op // CP n
		}
		
	case ir.OpAnd, ir.OpOr, ir.OpXor:
		// Can use immediate logical ops
		if paramType.Size() == 1 {
			return true, false, inst.Op // AND n, OR n, XOR n
		}
	}
	
	return false, false, ir.OpNop
}

// createSyntheticAnchor creates a synthetic anchor before first use
func (p *TrueSMCPass) createSyntheticAnchor(fn *ir.Function, param ir.Parameter, paramIndex int, beforeIndex int) *SMCAnchor {
	// Insert a load instruction that will contain the immediate
	anchor := &SMCAnchor{
		FunctionName: fn.Name,
		ParamName:    param.Name,
		ParamIndex:   paramIndex,
		Type:         param.Type,
		FirstUseInst: beforeIndex,
		Symbol:       fmt.Sprintf("%s$imm0", param.Name),
	}
	
	if param.Type.Size() == 1 {
		anchor.IsImm8 = true
		anchor.Instruction = ir.OpLoadConst // Will become LD A, n
	} else {
		anchor.IsImm16 = true
		anchor.Instruction = ir.OpLoadConst // Will become LD HL, nn
	}
	
	return anchor
}

// insertAnchors inserts anchor instructions into the function
func (p *TrueSMCPass) insertAnchors(fn *ir.Function, anchors []*SMCAnchor) bool {
	if len(anchors) == 0 {
		return false
	}
	
	// Sort anchors by insertion point to maintain correct indices
	// In real implementation would use proper sorting
	
	changed := false
	insertedCount := 0
	
	// First, create a map from parameter index to anchor for efficient lookup
	paramIndexToAnchor := make(map[int]*SMCAnchor)
	for _, anchor := range anchors {
		paramIndexToAnchor[anchor.ParamIndex] = anchor
	}
	
	// Transform parameter loads to use their specific anchors
	for i := range fn.Instructions {
		inst := &fn.Instructions[i]
		if inst.Op == ir.OpLoadParam {
			// Find the anchor for this specific parameter
			paramIndex := int(inst.Src1)
			if anchor, ok := paramIndexToAnchor[paramIndex]; ok {
				// Transform to anchor load
				inst.Op = ir.OpTrueSMCLoad
				inst.Symbol = anchor.Symbol
				inst.Comment = fmt.Sprintf("Load from anchor %s", anchor.Symbol)
				changed = true
				
				if p.diagnostics {
					fmt.Printf("TRUE SMC: Transformed OpLoadParam for param %d to use anchor %s\n", paramIndex, anchor.Symbol)
				}
			}
		}
	}
	
	// Add all anchors to patch table
	for _, anchor := range anchors {
		p.patchTable = append(p.patchTable, PatchEntry{
			Symbol:   anchor.Symbol,
			Size:     uint8(anchor.Type.Size()),
			ParamTag: anchor.ParamName,
			Function: anchor.FunctionName,
		})
		
		insertedCount++
	}
	
	// Mark function as using TRUE SMC
	fn.UsesTrueSMC = true
	
	if p.diagnostics && changed {
		fmt.Printf("TRUE SMC: Function %s - inserted %d anchors\n", fn.Name, insertedCount)
	}
	
	return changed
}

// convertPatchTable converts internal patch table to IR format
func (p *TrueSMCPass) convertPatchTable() []ir.PatchEntry {
	result := make([]ir.PatchEntry, len(p.patchTable))
	for i, entry := range p.patchTable {
		result[i] = ir.PatchEntry{
			Symbol:   entry.Symbol,
			Address:  entry.Address,
			Size:     entry.Size,
			Bank:     entry.Bank,
			ParamTag: entry.ParamTag,
			Function: entry.Function,
		}
	}
	return result
}

// GenerateReport generates SMC anchor report if diagnostics enabled
func (p *TrueSMCPass) GenerateReport() string {
	if !p.diagnostics || len(p.anchors) == 0 {
		return ""
	}
	
	report := "=== TRUE SMC Anchor Report ===\n"
	report += fmt.Sprintf("Total anchors: %d\n\n", len(p.anchors))
	
	for _, anchor := range p.anchors {
		report += fmt.Sprintf("Function: %s\n", anchor.FunctionName)
		report += fmt.Sprintf("  Parameter: %s (index %d)\n", anchor.ParamName, anchor.ParamIndex)
		report += fmt.Sprintf("  Symbol: %s\n", anchor.Symbol)
		report += fmt.Sprintf("  Type: %s", anchor.Type.String())
		if anchor.IsImm8 {
			report += " (8-bit immediate)"
		} else if anchor.IsImm16 {
			report += " (16-bit immediate)"
		}
		report += "\n"
		report += fmt.Sprintf("  First use at instruction: %d\n", anchor.FirstUseInst)
		if anchor.Address != 0 {
			report += fmt.Sprintf("  Generated at address: $%04X\n", anchor.Address)
		}
		report += "\n"
	}
	
	return report
}