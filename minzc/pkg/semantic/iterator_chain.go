package semantic

import (
	"fmt"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// Iterator chain and flatMap implementations

// generateChainedIteration handles the chain() operation that concatenates iterators
func (a *Analyzer) generateChainedIteration(chain *ast.IteratorChainExpr, firstSource ir.Register,
	firstArrayType *ir.ArrayType, elementType ir.Type, irFunc *ir.Function) (ir.Register, error) {
	
	// Find the chain operation and its argument
	var chainOp *ast.IteratorOp
	var beforeChainOps []ast.IteratorOp
	var afterChainOps []ast.IteratorOp
	
	for i, op := range chain.Operations {
		if op.Type == ast.IterOpChain {
			chainOp = &chain.Operations[i]
			beforeChainOps = chain.Operations[:i]
			if i+1 < len(chain.Operations) {
				afterChainOps = chain.Operations[i+1:]
			}
			break
		}
	}
	
	if chainOp == nil || chainOp.Function == nil {
		return 0, fmt.Errorf("chain operation requires a second iterator as argument")
	}
	
	// Analyze the second iterator source
	secondSourceReg, err := a.analyzeExpression(chainOp.Function, irFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to analyze second iterator in chain: %w", err)
	}
	
	// Get the type of the second source
	secondSourceType, err := a.inferType(chainOp.Function)
	if err != nil {
		return 0, fmt.Errorf("failed to infer second source type: %w", err)
	}
	
	secondArrayType, ok := secondSourceType.(*ir.ArrayType)
	if !ok {
		return 0, fmt.Errorf("chain can only concatenate arrays, got %s", secondSourceType)
	}
	
	// Verify element types match
	if !typesEqual(firstArrayType.Element, secondArrayType.Element) {
		return 0, fmt.Errorf("chain requires matching element types, got %s and %s",
			firstArrayType.Element, secondArrayType.Element)
	}
	
	// Add debug comment
	irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
		Op:      ir.OpNop,
		Comment: fmt.Sprintf("CHAINED ITERATION: array[%d] + array[%d]", 
			firstArrayType.Length, secondArrayType.Length),
	})
	
	// Create two sub-chains for each array
	firstChain := &ast.IteratorChainExpr{
		Source:     chain.Source,
		Operations: append(beforeChainOps, afterChainOps...),
		StartPos:   chain.StartPos,
	}
	
	// Generate iteration for first array
	err = a.generateArrayIterationChain(firstChain, firstSource, firstArrayType, elementType, irFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to generate first chain iteration: %w", err)
	}
	
	// Create chain for second array with remaining operations
	secondChain := &ast.IteratorChainExpr{
		Source:     chainOp.Function,
		Operations: afterChainOps,
		StartPos:   chainOp.StartPos,
	}
	
	// Generate iteration for second array
	err = a.generateArrayIterationChain(secondChain, secondSourceReg, secondArrayType, elementType, irFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to generate second chain iteration: %w", err)
	}
	
	return 0, nil
}

// generateArrayIterationChain is a helper that generates iteration without analyzing the source
func (a *Analyzer) generateArrayIterationChain(chain *ast.IteratorChainExpr, sourceReg ir.Register,
	arrayType *ir.ArrayType, elementType ir.Type, irFunc *ir.Function) error {
	
	// Determine if we can use DJNZ
	useDJNZ := arrayType.Length > 0 && arrayType.Length <= 255
	
	if useDJNZ {
		_, err := a.generateDJNZIteration(chain, sourceReg, arrayType, elementType, irFunc)
		return err
	}
	
	_, err := a.generateIndexedIteration(chain, sourceReg, arrayType, elementType, irFunc)
	return err
}

// generateFlatMapIteration handles the flatMap() operation
func (a *Analyzer) generateFlatMapIteration(chain *ast.IteratorChainExpr, sourceReg ir.Register,
	arrayType *ir.ArrayType, elementType ir.Type, irFunc *ir.Function) (ir.Register, error) {
	
	// FlatMap is like map but flattens nested arrays
	// For now, we'll implement a simple version that assumes the map function
	// returns arrays of the same type
	
	// Find the flatMap operation
	var flatMapOp *ast.IteratorOp
	var beforeOps []ast.IteratorOp
	var afterOps []ast.IteratorOp
	
	for i, op := range chain.Operations {
		if op.Type == ast.IterOpFlatMap {
			flatMapOp = &chain.Operations[i]
			beforeOps = chain.Operations[:i]
			if i+1 < len(chain.Operations) {
				afterOps = chain.Operations[i+1:]
			}
			break
		}
	}
	
	if flatMapOp == nil || flatMapOp.Function == nil {
		return 0, fmt.Errorf("flatMap operation requires a transformation function")
	}
	
	// For now, return an error as full implementation requires
	// dynamic memory allocation or fixed-size result arrays
	return 0, fmt.Errorf("flatMap not yet fully implemented")
}