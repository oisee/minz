package semantic

import (
	"fmt"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// Iterator support for semantic analysis

// typesEqual checks if two types are equal
func typesEqual(a, b ir.Type) bool {
	// Simple string comparison for now
	// TODO: Implement proper structural equality
	return a.String() == b.String()
}

// analyzeIteratorChainExpr analyzes an iterator chain and generates optimized code
func (a *Analyzer) analyzeIteratorChainExpr(chain *ast.IteratorChainExpr, irFunc *ir.Function) (ir.Register, error) {
	// Get the source type to determine iteration strategy
	sourceType, err := a.inferType(chain.Source)
	if err != nil {
		return 0, fmt.Errorf("failed to infer source type: %w", err)
	}
	
	// Get element type
	elementType := a.getIterableElementType(sourceType)
	if elementType == nil {
		return 0, fmt.Errorf("cannot iterate over type %s", sourceType)
	}
	
	// Analyze the source to get its address
	sourceReg, err := a.analyzeExpression(chain.Source, irFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to analyze iterator source: %w", err)
	}
	
	// Generate iteration code based on source type
	switch st := sourceType.(type) {
	case *ir.ArrayType:
		return a.generateArrayIteration(chain, sourceReg, st, elementType, irFunc)
	case *ir.PointerType:
		// String iteration
		if basicType, ok := st.Base.(*ir.BasicType); ok && basicType.Kind == ir.TypeU8 {
			return a.generateStringIteration(chain, sourceReg, irFunc)
		}
		return 0, fmt.Errorf("cannot iterate over pointer type %s", sourceType)
	default:
		return 0, fmt.Errorf("cannot iterate over type %s", sourceType)
	}
}

// analyzeIteratorChainType analyzes the type of an iterator chain expression
func (a *Analyzer) analyzeIteratorChainType(chain *ast.IteratorChainExpr) (ir.Type, error) {
	// Infer the type of the source collection
	sourceType, err := a.inferType(chain.Source)
	if err != nil {
		return nil, err
	}
	
	// Verify source is iterable
	elementType := a.getIterableElementType(sourceType)
	if elementType == nil {
		return nil, fmt.Errorf("type %s is not iterable", sourceType)
	}
	
	// Track the current type through the chain
	currentType := elementType
	
	for _, op := range chain.Operations {
		newType, err := a.analyzeIteratorOpType(op, currentType)
		if err != nil {
			return nil, err
		}
		currentType = newType
	}
	
	// The final type depends on the last operation
	if len(chain.Operations) > 0 {
		lastOp := chain.Operations[len(chain.Operations)-1]
		switch lastOp.Type {
		case ast.IterOpCollect:
			// Collect returns an array of the current type
			return &ir.ArrayType{
				Element: currentType,
				Length:  -1, // Dynamic size
			}, nil
		case ast.IterOpForEach:
			// forEach returns void
			return &ir.BasicType{Kind: ir.TypeVoid}, nil
		case ast.IterOpReduce:
			// Reduce returns the accumulator type
			// This would need more analysis of the reduce function
			return currentType, nil
		default:
			// Most operations return an iterator of currentType
			return &ir.IteratorType{
				ElementType: currentType,
			}, nil
		}
	}
	
	return &ir.IteratorType{
		ElementType: currentType,
	}, nil
}

// getIterableElementType returns the element type if the type is iterable
func (a *Analyzer) getIterableElementType(t ir.Type) ir.Type {
	switch typ := t.(type) {
	case *ir.ArrayType:
		return typ.Element
	case *ir.PointerType:
		// Strings are *u8 and are iterable
		if basicType, ok := typ.Base.(*ir.BasicType); ok && basicType.Kind == ir.TypeU8 {
			return &ir.BasicType{Kind: ir.TypeU8} // Characters
		}
	case *ir.IteratorType:
		return typ.ElementType
	}
	return nil
}

// analyzeIteratorOpType analyzes the result type of a single iterator operation
func (a *Analyzer) analyzeIteratorOpType(op ast.IteratorOp, inputType ir.Type) (ir.Type, error) {
	switch op.Type {
	case ast.IterOpMap:
		// Map transforms elements
		// The function should be: fn(T) -> U
		fnType, err := a.inferType(op.Function)
		if err != nil {
			return nil, err
		}
		
		if funcType, ok := fnType.(*ir.FunctionType); ok {
			// Verify input type matches
			if len(funcType.Params) != 1 {
				return nil, fmt.Errorf("map function must take exactly one parameter")
			}
			if !typesEqual(funcType.Params[0], inputType) {
				return nil, fmt.Errorf("map function parameter type %s doesn't match element type %s",
					funcType.Params[0], inputType)
			}
			// Return the output type
			return funcType.Return, nil
		}
		return nil, fmt.Errorf("map requires a function, got %s", fnType)
		
	case ast.IterOpFilter:
		// Filter keeps/removes elements based on predicate
		// The function should be: fn(T) -> bool
		fnType, err := a.inferType(op.Function)
		if err != nil {
			return nil, err
		}
		
		if funcType, ok := fnType.(*ir.FunctionType); ok {
			if len(funcType.Params) != 1 {
				return nil, fmt.Errorf("filter function must take exactly one parameter")
			}
			if !typesEqual(funcType.Params[0], inputType) {
				return nil, fmt.Errorf("filter function parameter type %s doesn't match element type %s",
					funcType.Params[0], inputType)
			}
			if basicType, ok := funcType.Return.(*ir.BasicType); !ok || basicType.Kind != ir.TypeBool {
				return nil, fmt.Errorf("filter function must return bool, got %s", funcType.Return)
			}
			// Filter doesn't change the type
			return inputType, nil
		}
		return nil, fmt.Errorf("filter requires a function, got %s", fnType)
		
	case ast.IterOpForEach:
		// forEach applies a function to each element
		// The function should be: fn(T) -> void
		fnType, err := a.inferType(op.Function)
		if err != nil {
			return nil, err
		}
		
		if funcType, ok := fnType.(*ir.FunctionType); ok {
			if len(funcType.Params) != 1 {
				return nil, fmt.Errorf("forEach function must take exactly one parameter")
			}
			if !typesEqual(funcType.Params[0], inputType) {
				return nil, fmt.Errorf("forEach function parameter type %s doesn't match element type %s",
					funcType.Params[0], inputType)
			}
			// forEach consumes the iterator
			return &ir.BasicType{Kind: ir.TypeVoid}, nil
		}
		return nil, fmt.Errorf("forEach requires a function, got %s", fnType)
		
	case ast.IterOpTake:
		// Take n elements - type stays the same
		return inputType, nil
		
	case ast.IterOpSkip:
		// Skip n elements - type stays the same
		return inputType, nil
		
	case ast.IterOpCollect:
		// Collect into array - handled by caller
		return inputType, nil
		
	default:
		return nil, fmt.Errorf("unsupported iterator operation")
	}
}

// isIteratorMethod checks if a method name is an iterator method
func isIteratorMethod(name string) bool {
	switch name {
	case "iter", "map", "filter", "forEach", "reduce", "collect", "take", "skip", "zip":
		return true
	}
	return false
}

// generateArrayIteration generates code for iterating over an array
func (a *Analyzer) generateArrayIteration(chain *ast.IteratorChainExpr, sourceReg ir.Register, 
	arrayType *ir.ArrayType, elementType ir.Type, irFunc *ir.Function) (ir.Register, error) {
	
	// Check if we can use DJNZ optimization (arrays ≤255 elements)
	useDJNZ := arrayType.Length > 0 && arrayType.Length <= 255
	
	if useDJNZ {
		return a.generateDJNZIteration(chain, sourceReg, arrayType, elementType, irFunc)
	}
	
	// Fall back to standard indexed iteration for large arrays
	return a.generateIndexedIteration(chain, sourceReg, arrayType, elementType, irFunc)
}

// generateDJNZIteration generates optimized DJNZ loop for arrays ≤255 elements
// This uses pointer arithmetic and DJNZ instruction for 3x performance
func (a *Analyzer) generateDJNZIteration(chain *ast.IteratorChainExpr, sourceReg ir.Register,
	arrayType *ir.ArrayType, elementType ir.Type, irFunc *ir.Function) (ir.Register, error) {
	
	// Add debug comment
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpNop,
		Comment: fmt.Sprintf("DJNZ OPTIMIZED LOOP for array[%d]", arrayType.Length),
	})
	
	// Generate labels for the loop
	loopLabel := a.generateLabel("djnz_loop")
	
	// Allocate registers for DJNZ pattern
	counterReg := irFunc.AllocReg()  // B register for DJNZ
	ptrReg := irFunc.AllocReg()      // HL register for pointer
	elementReg := irFunc.AllocReg()  // A register for element
	
	// Initialize counter (B register) with array length
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: counterReg,
		Imm:  int64(arrayType.Length),
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Hint: ir.RegHintB, // Hint to use B register for DJNZ
		Comment: fmt.Sprintf("DJNZ counter = %d", arrayType.Length),
	})
	
	// Initialize pointer to array start
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpMove,
		Dest: ptrReg,
		Src1: sourceReg,
		Type: &ir.PointerType{Base: elementType},
		Hint: ir.RegHintHL, // Hint to use HL register pair
		Comment: "Pointer to array start",
	})
	
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
	for _, op := range chain.Operations {
		newReg, err := a.applyIteratorOperation(op, currentReg, elementType, irFunc)
		if err != nil {
			return 0, fmt.Errorf("failed to apply iterator operation: %w", err)
		}
		currentReg = newReg
	}
	
	// Increment pointer to next element
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpInc,
		Dest: ptrReg,
		Src1: ptrReg,
		Type: &ir.PointerType{Base: elementType},
		Hint: ir.RegHintHL,
		Comment: "Advance to next element",
	})
	
	// DJNZ instruction - decrement counter and jump if not zero
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:    ir.OpDJNZ,
		Src1:  counterReg,
		Label: loopLabel,
		Hint:  ir.RegHintB,
		Comment: "DJNZ - decrement and loop",
	})
	
	// No end label needed - DJNZ handles the exit condition
	
	// Return void register (iterators don't return values)
	return 0, nil
}

// generateIndexedIteration generates standard indexed loop for large arrays
func (a *Analyzer) generateIndexedIteration(chain *ast.IteratorChainExpr, sourceReg ir.Register,
	arrayType *ir.ArrayType, elementType ir.Type, irFunc *ir.Function) (ir.Register, error) {
	
	// Generate labels for the loop
	loopLabel := a.generateLabel("iter_loop")
	endLabel := a.generateLabel("iter_end")
	
	// Allocate registers for iteration
	indexReg := irFunc.AllocReg()
	elementPtrReg := irFunc.AllocReg()
	elementReg := irFunc.AllocReg()
	lengthReg := irFunc.AllocReg()
	
	// Initialize index to 0
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: indexReg,
		Imm:  0,
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Comment: "Iterator index = 0",
	})
	
	// Load array length
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: lengthReg,
		Imm:  int64(arrayType.Length),
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Comment: fmt.Sprintf("Array length = %d", arrayType.Length),
	})
	
	// Loop start
	irFunc.EmitLabel(loopLabel)
	
	// Check if index < length
	condReg := irFunc.AllocReg()
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLt,
		Dest: condReg,
		Src1: indexReg,
		Src2: lengthReg,
		Type: &ir.BasicType{Kind: ir.TypeBool},
		Comment: "Check index < length",
	})
	
	// Jump to end if done
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:    ir.OpJumpIfNot,
		Src1:  condReg,
		Label: endLabel,
	})
	
	// Calculate element address: base + index * element_size
	// For u8 arrays, element_size = 1
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpAdd,
		Dest: elementPtrReg,
		Src1: sourceReg,
		Src2: indexReg,
		Type: &ir.PointerType{Base: elementType},
		Comment: "Calculate element address",
	})
	
	// Load element value
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLoad,
		Dest: elementReg,
		Src1: elementPtrReg,
		Type: elementType,
		Comment: "Load array element",
	})
	
	// Apply iterator operations
	currentReg := elementReg
	for _, op := range chain.Operations {
		newReg, err := a.applyIteratorOperation(op, currentReg, elementType, irFunc)
		if err != nil {
			return 0, fmt.Errorf("failed to apply iterator operation: %w", err)
		}
		currentReg = newReg
	}
	
	// Increment index
	oneReg := irFunc.AllocReg()
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpLoadConst,
		Dest: oneReg,
		Imm:  1,
		Type: &ir.BasicType{Kind: ir.TypeU8},
	})
	
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:   ir.OpAdd,
		Dest: indexReg,
		Src1: indexReg,
		Src2: oneReg,
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Comment: "Increment index",
	})
	
	// Jump back to loop start
	irFunc.EmitJump(loopLabel)
	
	// End label
	irFunc.EmitLabel(endLabel)
	
	// Return void register (iterators don't return values)
	return 0, nil
}

// generateStringIteration generates code for iterating over a string
func (a *Analyzer) generateStringIteration(chain *ast.IteratorChainExpr, sourceReg ir.Register, 
	irFunc *ir.Function) (ir.Register, error) {
	// TODO: Implement string iteration with u8/u16 length support
	return 0, fmt.Errorf("string iteration not yet implemented")
}

// applyIteratorOperation applies a single iterator operation to the current element
func (a *Analyzer) applyIteratorOperation(op ast.IteratorOp, elementReg ir.Register, 
	elementType ir.Type, irFunc *ir.Function) (ir.Register, error) {
	
	switch op.Type {
	case ast.IterOpMap:
		// Apply transformation function
		if op.Function == nil {
			return 0, fmt.Errorf("map requires a function")
		}
		return a.applyIteratorFunction(op.Function, elementReg, elementType, irFunc)
		
	case ast.IterOpFilter:
		// Apply predicate and skip if false
		if op.Function == nil {
			return 0, fmt.Errorf("filter requires a predicate function")
		}
		// TODO: Implement filter with conditional skip
		return elementReg, nil
		
	case ast.IterOpForEach:
		// Apply function to each element
		if op.Function == nil {
			return 0, fmt.Errorf("forEach requires a function")
		}
		_, err := a.applyIteratorFunction(op.Function, elementReg, elementType, irFunc)
		return elementReg, err
		
	default:
		return 0, fmt.Errorf("iterator operation %v not yet implemented", op.Type)
	}
}

// applyIteratorFunction applies a function to an element in an iterator chain
func (a *Analyzer) applyIteratorFunction(function ast.Expression, elementReg ir.Register,
	elementType ir.Type, irFunc *ir.Function) (ir.Register, error) {
	
	// Handle different function types
	switch fn := function.(type) {
	case *ast.Identifier:
		// Direct function reference like print_u8
		funcName := fn.Name
		
		// Look up the function
		sym := a.currentScope.Lookup(funcName)
		if sym == nil && a.currentModule != "" {
			prefixedName := a.prefixSymbol(funcName)
			sym = a.currentScope.Lookup(prefixedName)
			funcName = prefixedName
		}
		
		if sym == nil {
			// Check for built-in functions
			if funcName == "print_u8" || funcName == "print_u16" {
				// Handle built-in print functions
				irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
					Op:      ir.OpCall,
					Symbol:  funcName,
					Args:    []ir.Register{elementReg},
					Comment: fmt.Sprintf("Call %s", funcName),
				})
				return 0, nil // print functions return void
			}
			return 0, fmt.Errorf("undefined function in iterator: %s", fn.Name)
		}
		
		// Generate function call
		funcSym, ok := sym.(*FuncSymbol)
		if !ok {
			return 0, fmt.Errorf("%s is not a function", funcName)
		}
		
		// Create call instruction
		resultReg := irFunc.AllocReg()
		irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
			Op:      ir.OpCall,
			Dest:    resultReg,
			Symbol:  funcName,
			Args:    []ir.Register{elementReg},
			Type:    funcSym.ReturnType,
			Comment: fmt.Sprintf("Call %s", funcName),
		})
		
		return resultReg, nil
		
	case *ast.LambdaExpr:
		// Lambda expression - need to generate inline or as separate function
		// TODO: Implement lambda support in iterators
		return 0, fmt.Errorf("lambda expressions in iterators not yet implemented")
		
	default:
		return 0, fmt.Errorf("unsupported function type in iterator: %T", function)
	}
}

// transformIteratorMethodCall transforms method calls into iterator chains
// This is called during expression analysis when we see patterns like array.map(f)
func (a *Analyzer) transformIteratorMethodCall(object ast.Expression, method string, 
	args []ast.Expression) (*ast.IteratorChainExpr, error) {
	
	// Check if this is an iterator method
	if !isIteratorMethod(method) {
		return nil, nil // Not an iterator method
	}
	
	// Build the iterator chain
	chain := &ast.IteratorChainExpr{
		Source:   object,
		StartPos: object.Pos(),
	}
	
	// Add the operation
	var opType ast.IteratorOpType
	switch method {
	case "iter":
		// Just start the chain, no operation needed
		return chain, nil
	case "map":
		opType = ast.IterOpMap
	case "filter":
		opType = ast.IterOpFilter
	case "forEach":
		opType = ast.IterOpForEach
	case "reduce":
		opType = ast.IterOpReduce
	case "collect":
		opType = ast.IterOpCollect
	case "take":
		opType = ast.IterOpTake
	case "skip":
		opType = ast.IterOpSkip
	case "zip":
		opType = ast.IterOpZip
	default:
		return nil, fmt.Errorf("unknown iterator method: %s", method)
	}
	
	// Most iterator methods take a function argument
	var function ast.Expression
	if len(args) > 0 {
		function = args[0]
	}
	
	chain.Operations = append(chain.Operations, ast.IteratorOp{
		Type:     opType,
		Function: function,
	})
	
	return chain, nil
}