package semantic

import (
	"fmt"
	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// ErrorType represents a type that can contain an error
type ErrorType struct {
	ValueType ast.Type
	IsError   bool
}

// AnalyzeErrorReturn analyzes a function with error return type (?)
func (a *Analyzer) AnalyzeErrorReturn(returnType ast.Type, hasQuestion bool) (ast.Type, error) {
	if !hasQuestion {
		return returnType, nil
	}

	// Create an error-returning type
	// In Z80, this means the function can set CY flag on error
	// with error code in A register
	return &ast.ErrorType{
		ValueType: returnType,
	}, nil
}

// AnalyzeTryExpression analyzes the ? operator for error propagation
func (a *Analyzer) AnalyzeTryExpression(expr *ast.TryExpression) (ast.Type, error) {
	// Analyze the inner expression
	exprType, err := a.AnalyzeExpression(expr.Expression)
	if err != nil {
		return nil, err
	}

	// Check if the expression returns an error type
	errorType, ok := exprType.(*ast.ErrorType)
	if !ok {
		return nil, fmt.Errorf("cannot use ? operator on non-error type %v", exprType)
	}

	// The ? operator unwraps the value or propagates the error
	// In the current function context, we need to ensure the function
	// also returns an error type
	if a.currentFunction != nil {
		funcReturnType := a.currentFunction.ReturnType
		if _, isErrorReturn := funcReturnType.(*ast.ErrorType); !isErrorReturn {
			return nil, fmt.Errorf("cannot use ? operator in function that doesn't return an error type")
		}
	}

	// Return the unwrapped value type
	return errorType.ValueType, nil
}

// GenerateErrorPropagation generates MIR for ? operator
func GenerateErrorPropagation(expr ir.Value, currentBlock *ir.BasicBlock, nextBlock *ir.BasicBlock, errorBlock *ir.BasicBlock) {
	// Check carry flag (error indicator)
	checkCY := &ir.Instruction{
		Op:   ir.OpCheckCarry,
		Type: &ir.BasicType{Kind: ir.TypeBool},
		Src1: expr,
	}
	currentBlock.Instructions = append(currentBlock.Instructions, checkCY)

	// Branch on error
	branch := &ir.Instruction{
		Op:   ir.OpBranchCond,
		Src1: checkCY.Dest,
		Src2: ir.Constant{Value: int64(errorBlock.ID)}, // If CY set, go to error block
		Dest: ir.Register(nextBlock.ID),                // Otherwise continue
	}
	currentBlock.Instructions = append(currentBlock.Instructions, branch)
}

// GenerateErrorReturn generates MIR for returning with error
func GenerateErrorReturn(errorCode ir.Value, block *ir.BasicBlock) {
	// Set carry flag and return error code in A
	setError := &ir.Instruction{
		Op:   ir.OpSetError,
		Type: &ir.BasicType{Kind: ir.TypeU8},
		Src1: errorCode,
	}
	block.Instructions = append(block.Instructions, setError)

	// Return with error
	ret := &ir.Instruction{
		Op: ir.OpReturn,
	}
	block.Instructions = append(block.Instructions, ret)
}

// GenerateSuccessReturn generates MIR for returning without error
func GenerateSuccessReturn(value ir.Value, block *ir.BasicBlock) {
	// Clear carry flag and return value
	clearError := &ir.Instruction{
		Op:   ir.OpClearError,
		Type: value.Type(),
		Src1: value,
	}
	block.Instructions = append(block.Instructions, clearError)

	// Return success
	ret := &ir.Instruction{
		Op:   ir.OpReturn,
		Src1: value,
	}
	block.Instructions = append(block.Instructions, ret)
}

// Error enum constants can be range-based for different modules
const (
	ErrorRangeCore    = 0
	ErrorRangeFile    = 16
	ErrorRangeNetwork = 32
	ErrorRangeUser    = 48
)

// IsErrorEnum checks if an enum is used for error codes
func IsErrorEnum(enumType *ast.EnumType) bool {
	// Check if enum has error-related variants
	for _, variant := range enumType.Variants {
		if variant.Name == "None" || variant.Name == "Ok" {
			return true
		}
	}
	// Or check if enum name contains "Error"
	return enumType.Name != "" && (enumType.Name[len(enumType.Name)-5:] == "Error")
}