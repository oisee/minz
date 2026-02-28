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
	hasReduce := false
	var reduceFunc ast.Expression

	// Analyze operations to optimize the loop
	for _, op := range chain.Operations {
		switch op.Type {
		case ast.IterOpSkip:
			// Extract skip count from Argument field
			if op.Argument != nil {
				if lit, ok := op.Argument.(*ast.NumberLiteral); ok {
					skipCount = int(lit.Value)
				}
			}
		case ast.IterOpTake:
			// Extract take count from Argument field
			if op.Argument != nil {
				if lit, ok := op.Argument.(*ast.NumberLiteral); ok {
					takeCount = int(lit.Value)
				}
			}
		case ast.IterOpEnumerate:
			hasEnumerate = true
		case ast.IterOpReduce:
			hasReduce = true
			reduceFunc = op.Function
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

	// Optional accumulator register for reduce
	var accumulatorReg ir.Register
	if hasReduce {
		accumulatorReg = irFunc.AllocReg()
		// For fold-left reduce, we need to:
		// 1. Initialize acc with first element
		// 2. Loop from second element onwards
		// Adjust counts: we'll process n-1 elements in the loop
		if effectiveCount > 1 {
			effectiveCount--
		} else if effectiveCount == 1 {
			// Only one element - just return it directly
			// Load first element as the result
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:   ir.OpLoad,
				Dest: accumulatorReg,
				Src1: sourceReg,
				Type: elementType,
				Comment: "Single-element reduce: load as result",
			})
			return accumulatorReg, nil
		}
	}

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

	// For reduce: initialize accumulator with first element, then advance pointer
	if hasReduce {
		// Load first element into accumulator
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpLoad,
			Dest:    accumulatorReg,
			Src1:    ptrReg,
			Type:    elementType,
			Comment: "Reduce: init accumulator with first element",
		})

		// Advance pointer past first element
		elementSize := elementType.Size()
		if elementSize == 1 {
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:      ir.OpInc,
				Dest:    ptrReg,
				Src1:    ptrReg,
				Type:    &ir.PointerType{Base: elementType},
				Hint:    ir.RegHintHL,
				Comment: "Advance past first element",
			})
		} else {
			sizeReg := irFunc.AllocReg()
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:      ir.OpLoadConst,
				Dest:    sizeReg,
				Imm:     int64(elementSize),
				Type:    &ir.BasicType{Kind: ir.TypeU16},
				Comment: "Element size for pointer advance",
			})
			irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
				Op:      ir.OpAdd,
				Dest:    ptrReg,
				Src1:    ptrReg,
				Src2:    sizeReg,
				Type:    &ir.PointerType{Base: elementType},
				Hint:    ir.RegHintHL,
				Comment: "Advance past first element",
			})
		}
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
			// Try inline optimization for simple comparison lambdas
			if lambda, ok := op.Function.(*ast.LambdaExpr); ok {
				if flagCond, constVal, isSimple := isSimpleComparisonLambda(lambda); isSimple {
					a.tracer.Log("semantic", "Inline filter (enhanced): CP %d + JR %s", constVal, flagCond)
					constReg := irFunc.AllocReg()
					irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
						Op:   ir.OpLoadConst,
						Dest: constReg,
						Imm:  constVal,
						Type: &ir.BasicType{Kind: ir.TypeU8},
						Comment: fmt.Sprintf("Inline filter constant = %d", constVal),
					})

					continueLabel := a.generateLabel("filter_continue")
					continueLabels = append(continueLabels, continueLabel)

					irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
						Op:    ir.OpJumpIfFlag,
						Src1:  currentReg,
						Src2:  constReg,
						Imm:   int64(flagCond),
						Label: continueLabel,
						Comment: fmt.Sprintf("Inline filter: CP %d + JR %s", constVal, flagCond),
					})
					continue
				}
			}

			// Fallback: call the filter predicate as a function
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

		case ast.IterOpReduce:
			// Apply reducer function: acc = reducer(acc, elem)
			// The reduce function should have been captured as reduceFunc
			if reduceFunc != nil {
				newAcc, err := a.applyReducerFunction(reduceFunc, accumulatorReg, currentReg, elementType, irFunc)
				if err != nil {
					return 0, fmt.Errorf("failed to apply reduce function: %w", err)
				}
				// Move result to accumulator if different register
				if newAcc != accumulatorReg {
					irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
						Op:      ir.OpMove,
						Dest:    accumulatorReg,
						Src1:    newAcc,
						Type:    elementType,
						Comment: "Update accumulator with reduce result",
					})
				}
			}

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

	// Return accumulator for reduce, void for other operations
	if hasReduce {
		return accumulatorReg, nil
	}
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

// applyReducerFunction applies a reducer function that takes (acc, element) and returns new acc
func (a *Analyzer) applyReducerFunction(fn ast.Expression, accReg, elemReg ir.Register,
	elemType ir.Type, irFunc *ir.Function) (ir.Register, error) {

	if fn == nil {
		return 0, fmt.Errorf("reduce requires a function")
	}

	switch f := fn.(type) {
	case *ast.LambdaExpr:
		// Lambda with two parameters: |acc, x| expr
		if len(f.Params) != 2 {
			return 0, fmt.Errorf("reduce lambda must have exactly 2 parameters (acc, elem), got %d", len(f.Params))
		}

		// Generate the reducer lambda as a separate function
		return a.generateReducerLambda(f, accReg, elemReg, elemType, irFunc)

	case *ast.Identifier:
		// Named function: call with (acc, elem)
		// Push arguments in reverse order for calling convention
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpPush,
			Src1:    elemReg,
			Comment: "Push element for reducer call",
		})
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpPush,
			Src1:    accReg,
			Comment: "Push accumulator for reducer call",
		})

		resultReg := irFunc.AllocReg()
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpCall,
			Dest:    resultReg,
			Symbol:  f.Name,
			Imm:     2, // Two arguments
			Comment: fmt.Sprintf("Call reducer function %s", f.Name),
		})

		return resultReg, nil

	default:
		return 0, fmt.Errorf("unsupported function type for reduce: %T", fn)
	}
}

// generateReducerLambda generates a lambda function for reduce operations with 2 parameters
func (a *Analyzer) generateReducerLambda(lambda *ast.LambdaExpr, accReg, elemReg ir.Register,
	elemType ir.Type, irFunc *ir.Function) (ir.Register, error) {

	// Generate unique reducer lambda name
	lambdaName := fmt.Sprintf("reducer_lambda_%s_%d", irFunc.Name, a.lambdaCounter)
	a.lambdaCounter++

	// Create a new function for the reducer lambda
	lambdaFunc := &ir.Function{
		Name:              lambdaName,
		Params:            []ir.Parameter{},
		Instructions:      []ir.Instruction{},
		IsSMCDefault:      false,
		IsSMCEnabled:      false,
		CallingConvention: "registers",
	}

	// Add both parameters (acc, elem)
	accParam := lambda.Params[0]
	elemParam := lambda.Params[1]

	lambdaFunc.Params = append(lambdaFunc.Params, ir.Parameter{
		Name: accParam.Name,
		Type: elemType,
	})
	lambdaFunc.Params = append(lambdaFunc.Params, ir.Parameter{
		Name: elemParam.Name,
		Type: elemType,
	})
	lambdaFunc.ReturnType = elemType

	// Save current scope
	savedScope := a.currentScope
	savedFunc := a.currentFunc

	// Create new scope for lambda
	lambdaScope := NewScope(savedScope)
	a.currentScope = lambdaScope
	a.currentFunc = lambdaFunc

	// Define parameters in lambda scope
	accSymbol := &VarSymbol{
		Name: accParam.Name,
		Type: elemType,
	}
	elemSymbol := &VarSymbol{
		Name: elemParam.Name,
		Type: elemType,
	}
	lambdaScope.Define(accParam.Name, accSymbol)
	lambdaScope.Define(elemParam.Name, elemSymbol)

	// Analyze the lambda body
	var resultReg ir.Register
	var err error

	// The body could be an expression or a block
	switch body := lambda.Body.(type) {
	case ast.Expression:
		resultReg, err = a.analyzeExpression(body, lambdaFunc)
	case *ast.BlockStmt:
		// For block statements, analyze the block
		err = a.analyzeBlock(body, lambdaFunc)
		// Use register 0 as default result
		resultReg = 0
	default:
		err = fmt.Errorf("unsupported lambda body type: %T", lambda.Body)
	}

	if err != nil {
		// Restore scope before returning error
		a.currentScope = savedScope
		a.currentFunc = savedFunc
		return 0, fmt.Errorf("failed to analyze reducer lambda body: %w", err)
	}

	// Emit return instruction
	lambdaFunc.Instructions = append(lambdaFunc.Instructions, ir.Instruction{
		Op:      ir.OpReturn,
		Src1:    resultReg,
		Type:    elemType,
		Comment: "Return lambda result",
	})

	// Restore scope
	a.currentScope = savedScope
	a.currentFunc = savedFunc

	// Add the lambda function to the module
	a.module.Functions = append(a.module.Functions, lambdaFunc)

	// Generate call to the reducer lambda
	callResultReg := irFunc.AllocReg()
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpCall,
		Dest:    callResultReg,
		Symbol:  lambdaName,
		Args:    []ir.Register{accReg, elemReg},
		Type:    elemType,
		Comment: fmt.Sprintf("Call reducer lambda %s", lambdaName),
	})

	// For reduce, we inline the result - just mark that we have the result
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpNop,
		Comment: "Inlined return value",
	})

	return callResultReg, nil
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
			if op.Argument != nil {
				if lit, ok := op.Argument.(*ast.NumberLiteral); ok {
					opt.SkipCount = int(lit.Value)
				}
			}
		case ast.IterOpTake:
			if op.Argument != nil {
				if lit, ok := op.Argument.(*ast.NumberLiteral); ok {
					opt.TakeCount = int(lit.Value)
				}
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