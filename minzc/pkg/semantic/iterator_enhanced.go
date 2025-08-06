package semantic

import (
	"fmt"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// Enhanced iterator operations for MinZ

// generateEnhancedDJNZIteration generates optimized DJNZ loop with enhanced iterator support
func (a *Analyzer) generateEnhancedDJNZIteration(chain *ast.IteratorChainExpr, sourceReg ir.Register,
	arrayType *ir.ArrayType, elementType ir.Type, irFunc *ir.Function) (ir.Register, error) {
	
	// Pre-process the chain to extract stateful operations
	skipCount := 0
	takeCount := arrayType.Length
	hasEnumerate := false
	
	// Analyze operations to optimize the loop
	for _, op := range chain.Operations {
		switch op.Type {
		case ast.IterOpSkip:
			// Extract skip count from argument
			if lit, ok := op.Function.(*ast.NumberLiteral); ok {
				skipCount = int(lit.Value)
			}
		case ast.IterOpTake:
			// Extract take count from argument
			if lit, ok := op.Function.(*ast.NumberLiteral); ok {
				takeCount = int(lit.Value)
			}
		case ast.IterOpEnumerate:
			hasEnumerate = true
		}
	}
	
	// Adjust effective loop count based on skip/take
	effectiveStart := skipCount
	effectiveCount := takeCount
	if effectiveStart+effectiveCount > arrayType.Length {
		effectiveCount = arrayType.Length - effectiveStart
	}
	
	// Can't use DJNZ if we need more than 255 iterations
	if effectiveCount > 255 {
		return a.generateIndexedIteration(chain, sourceReg, arrayType, elementType, irFunc)
	}
	
	// Add debug comment
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpNop,
		Comment: fmt.Sprintf("ENHANCED DJNZ LOOP: skip=%d, take=%d, total=%d", skipCount, takeCount, effectiveCount),
	})
	
	// Generate labels for the loop
	loopLabel := a.generateLabel("djnz_enhanced_loop")
	
	// Allocate registers for DJNZ pattern
	counterReg := irFunc.AllocReg()  // B register for DJNZ
	ptrReg := irFunc.AllocReg()      // HL register for pointer
	elementReg := irFunc.AllocReg()  // Current element
	
	// Optional index register for enumerate
	var indexReg ir.Register
	if hasEnumerate {
		indexReg = irFunc.AllocReg()
		// Initialize index (accounting for skip)
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpLoadConst,
			Dest: indexReg,
			Imm:  int64(skipCount),
			Type: &ir.BasicType{Kind: ir.TypeU8},
			Comment: "Enumerate index (accounting for skip)",
		})
	}
	
	// Initialize counter (B register) with effective count
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: counterReg,
		Imm:  int64(effectiveCount),
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Hint: ir.RegHintB, // Hint to use B register for DJNZ
		Comment: fmt.Sprintf("DJNZ counter = %d (after skip/take)", effectiveCount),
	})
	
	// Initialize pointer to array start + skip offset
	if skipCount > 0 {
		// Calculate skipped address
		offsetReg := irFunc.AllocReg()
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpLoadConst,
			Dest: offsetReg,
			Imm:  int64(skipCount * elementType.Size()),
			Type: &ir.BasicType{Kind: ir.TypeU16},
			Comment: fmt.Sprintf("Skip offset = %d elements", skipCount),
		})
		
		// Add offset to base pointer
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpAdd,
			Dest: ptrReg,
			Src1: sourceReg,
			Src2: offsetReg,
			Type: &ir.PointerType{Base: elementType},
			Hint: ir.RegHintHL,
			Comment: "Pointer to first element after skip",
		})
	} else {
		// No skip - start from beginning
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpMove,
			Dest: ptrReg,
			Src1: sourceReg,
			Type: &ir.PointerType{Base: elementType},
			Hint: ir.RegHintHL,
			Comment: "Pointer to array start",
		})
	}
	
	// Loop start
	irFunc.EmitLabel(loopLabel)
	
	// Load element through pointer
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLoad,
		Dest: elementReg,
		Src1: ptrReg,
		Type: elementType,
		Comment: "Load element via pointer",
	})
	
	// Apply iterator operations
	currentReg := elementReg
	var continueLabels []string
	
	for _, op := range chain.Operations {
		switch op.Type {
		case ast.IterOpSkip, ast.IterOpTake:
			// Already handled in preprocessing
			continue
			
		case ast.IterOpEnumerate:
			// Create a tuple (index, element)
			// For now, we'll handle this by calling the function with two arguments
			// In the future, we could optimize this with a special calling convention
			continue
			
		case ast.IterOpFilter:
			// Call the filter predicate
			predicateResult, err := a.applyIteratorFunction(op.Function, currentReg, elementType, irFunc)
			if err != nil {
				return 0, fmt.Errorf("failed to apply filter predicate: %w", err)
			}
			
			// Generate continue label for this filter
			continueLabel := a.generateLabel("filter_continue")
			continueLabels = append(continueLabels, continueLabel)
			
			// Jump to continue if predicate is false
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:    ir.OpJumpIfNot,
				Src1:  predicateResult,
				Label: continueLabel,
				Comment: "Skip if filter predicate is false",
			})
			
		case ast.IterOpMap:
			// Apply transformation function
			newReg, err := a.applyIteratorFunction(op.Function, currentReg, elementType, irFunc)
			if err != nil {
				return 0, fmt.Errorf("failed to apply map function: %w", err)
			}
			currentReg = newReg
			
		case ast.IterOpForEach:
			// Call the forEach function with the current element
			if hasEnumerate {
				// Call with (index, element)
				err := a.applyEnumeratedFunction(op.Function, indexReg, currentReg, irFunc)
				if err != nil {
					return 0, fmt.Errorf("failed to apply enumerated forEach: %w", err)
				}
			} else {
				// Call with just element
				_, err := a.applyIteratorFunction(op.Function, currentReg, elementType, irFunc)
				if err != nil {
					return 0, fmt.Errorf("failed to apply forEach function: %w", err)
				}
			}
			
		case ast.IterOpPeek, ast.IterOpInspect:
			// These are like forEach but don't consume the iterator
			// Call the function but keep the original value
			_, err := a.applyIteratorFunction(op.Function, currentReg, elementType, irFunc)
			if err != nil {
				return 0, fmt.Errorf("failed to apply peek/inspect function: %w", err)
			}
			// Keep currentReg unchanged
			
		case ast.IterOpTakeWhile:
			// Generate predicate check
			predicateResult, err := a.applyIteratorFunction(op.Function, currentReg, elementType, irFunc)
			if err != nil {
				return 0, fmt.Errorf("failed to apply takeWhile predicate: %w", err)
			}
			
			// If predicate is false, exit the loop entirely
			exitLabel := a.generateLabel("takewhile_exit")
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:    ir.OpJumpIfNot,
				Src1:  predicateResult,
				Label: exitLabel,
				Comment: "Exit loop if takeWhile predicate is false",
			})
			
			// We need to store this exit label to emit it after the loop
			// For now, we'll use the endLabel pattern from indexed iteration
			// This is a limitation that needs better handling
			
		case ast.IterOpSkipWhile:
			// This is more complex - we need state to track if we're still skipping
			// For now, mark as unimplemented in DJNZ mode
			return 0, fmt.Errorf("skipWhile not yet implemented in DJNZ mode")
			
		default:
			// Apply other operations normally
			newReg, err := a.applyIteratorOperation(op, currentReg, elementType, irFunc)
			if err != nil {
				return 0, fmt.Errorf("failed to apply iterator operation: %w", err)
			}
			currentReg = newReg
		}
	}
	
	// Emit continue labels for filters (right before loop increment)
	for _, label := range continueLabels {
		irFunc.EmitLabel(label)
	}
	
	// Increment enumeration index if needed
	if hasEnumerate {
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpInc,
			Dest: indexReg,
			Src1: indexReg,
			Type: &ir.BasicType{Kind: ir.TypeU8},
			Comment: "Increment enumeration index",
		})
	}
	
	// Increment pointer to next element
	elementSize := elementType.Size()
	if elementSize == 1 {
		// For byte arrays, use INC HL
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpInc,
			Dest: ptrReg,
			Src1: ptrReg,
			Type: &ir.PointerType{Base: elementType},
			Hint: ir.RegHintHL,
			Comment: "Advance to next byte",
		})
	} else {
		// For larger elements, use ADD HL, DE
		sizeReg := irFunc.AllocReg()
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpLoadConst,
			Dest: sizeReg,
			Imm:  int64(elementSize),
			Type: &ir.BasicType{Kind: ir.TypeU16},
			Hint: ir.RegHintDE,
			Comment: fmt.Sprintf("Element size = %d", elementSize),
		})
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpAdd,
			Dest: ptrReg,
			Src1: ptrReg,
			Src2: sizeReg,
			Type: &ir.PointerType{Base: elementType},
			Hint: ir.RegHintHL,
			Comment: "Advance to next element",
		})
	}
	
	// DJNZ instruction - decrement counter and jump if not zero
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:    ir.OpDJNZ,
		Src1:  counterReg,
		Label: loopLabel,
		Hint:  ir.RegHintB,
		Comment: "DJNZ - decrement and loop",
	})
	
	// Return void register (iterators don't return values)
	return 0, nil
}

// applyEnumeratedFunction applies a function that takes (index, element) parameters
func (a *Analyzer) applyEnumeratedFunction(fn ast.Expression, indexReg, elementReg ir.Register, irFunc *ir.Function) error {
	// For now, we'll generate a simple function call with two arguments
	// In the future, this could be optimized with tuple unpacking
	
	switch f := fn.(type) {
	case *ast.LambdaExpr:
		// Inline the lambda body with both parameters
		// This would require enhanced lambda support for multiple parameters
		return fmt.Errorf("enumerated lambdas not yet implemented")
		
	case *ast.Identifier:
		// Call a named function with two arguments
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpPush,
			Src1: elementReg,
			Comment: "Push element for enumerated call",
		})
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:   ir.OpPush,
			Src1: indexReg,
			Comment: "Push index for enumerated call",
		})
		
		resultReg := irFunc.AllocReg()
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:     ir.OpCall,
			Dest:   resultReg,
			Symbol: f.Name,
			Imm:    2, // Two arguments
			Comment: fmt.Sprintf("Call enumerated function %s", f.Name),
		})
		
		return nil
		
	default:
		return fmt.Errorf("unsupported function type for enumeration: %T", fn)
	}
}

// Enhanced chain optimization analysis
type ChainOptimization struct {
	CanUseDJNZ     bool
	SkipCount      int
	TakeCount      int
	HasSideEffects bool
	IsPure         bool
}

// analyzeIteratorChain performs optimization analysis on an iterator chain
func analyzeIteratorChain(chain *ast.IteratorChainExpr, sourceType ir.Type) ChainOptimization {
	opt := ChainOptimization{
		CanUseDJNZ: true,
		IsPure:     true,
	}
	
	// Check source type
	if arrayType, ok := sourceType.(*ir.ArrayType); ok {
		opt.CanUseDJNZ = arrayType.Length > 0 && arrayType.Length <= 255
		opt.TakeCount = arrayType.Length
	} else {
		opt.CanUseDJNZ = false
	}
	
	// Analyze operations
	for _, op := range chain.Operations {
		switch op.Type {
		case ast.IterOpSkip:
			if lit, ok := op.Function.(*ast.NumberLiteral); ok {
				opt.SkipCount = int(lit.Value)
			}
		case ast.IterOpTake:
			if lit, ok := op.Function.(*ast.NumberLiteral); ok {
				opt.TakeCount = int(lit.Value)
			}
		case ast.IterOpForEach:
			opt.HasSideEffects = true
			opt.IsPure = false
		}
	}
	
	// Check if we can still use DJNZ after skip/take
	effectiveCount := opt.TakeCount - opt.SkipCount
	if effectiveCount > 255 || effectiveCount <= 0 {
		opt.CanUseDJNZ = false
	}
	
	return opt
}