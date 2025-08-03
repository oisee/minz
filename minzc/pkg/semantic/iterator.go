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
	// For now, we'll generate unoptimized code
	// The fusion optimizer will transform this later
	
	// Analyze the source collection
	sourceReg, err := a.analyzeExpression(chain.Source, irFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to analyze iterator source: %w", err)
	}
	
	// Get the type of the source - removed for now since we don't use it yet
	// TODO: Use sourceType for type checking when generating operations
	_, err = a.inferType(chain.Source)
	if err != nil {
		return 0, fmt.Errorf("failed to infer source type: %w", err)
	}
	
	// For now, just return the source register
	// TODO: Generate iterator operations
	return sourceReg, nil
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