package semantic

import (
	"fmt"
	"math"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)

// ConstantFolder performs compile-time constant evaluation and folding
type ConstantFolder struct {
	constants map[string]ir.Value
	metrics   *OptimizationMetrics
	analyzer  *Analyzer
}

// OptimizationMetrics tracks optimization statistics
type OptimizationMetrics struct {
	ConstantsFolded        int
	DeadCodeEliminated     int
	CyclesSavedFolding     int
	BytesSavedFolding      int
}

// NewConstantFolder creates a new constant folder
func NewConstantFolder(analyzer *Analyzer) *ConstantFolder {
	return &ConstantFolder{
		constants: make(map[string]ir.Value),
		metrics:   &OptimizationMetrics{},
		analyzer:  analyzer,
	}
}

// FoldConstants performs constant folding on the entire module
func (cf *ConstantFolder) FoldConstants(module *ir.Module) {
	// First pass: collect constant declarations
	cf.collectConstants(module)
	
	// Second pass: fold constant expressions in functions
	for _, function := range module.Functions {
		cf.foldConstantsInFunction(function)
	}
	
	// Third pass: eliminate dead constant declarations
	cf.eliminateDeadConstants(module)
}

// collectConstants finds all constant declarations in the module
func (cf *ConstantFolder) collectConstants(module *ir.Module) {
	for _, global := range module.Globals {
		if global.IsConst && global.Init != nil {
			value := cf.evaluateConstantExpression(global.Init)
			if value.IsValid() {
				cf.constants[global.Name] = value
			}
		}
	}
}

// foldConstantsInFunction performs constant folding within a function
func (cf *ConstantFolder) foldConstantsInFunction(function *ir.Function) {
	// Track local constants within function scope
	localConstants := make(map[string]ir.Value)
	
	for i, inst := range function.Instructions {
		switch inst.Op {
		case ir.OpLoadConst:
			// Track constant loads
			localConstants[fmt.Sprintf("r%d", inst.Dest)] = inst.Value
			
		case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod, ir.OpShl, ir.OpShr, ir.OpAnd, ir.OpOr, ir.OpXor:
			// Try to fold binary operations
			if folded := cf.foldBinaryOperation(inst, localConstants); folded != nil {
				function.Instructions[i] = *folded
				cf.metrics.ConstantsFolded++
				cf.metrics.CyclesSavedFolding += cf.getCycleSavings(inst.Op)
				cf.metrics.BytesSavedFolding += cf.getByteSavings(inst.Op)
				
				// Update local constants tracking
				localConstants[fmt.Sprintf("r%d", folded.Dest)] = folded.Value
			}
			
		case ir.OpCall:
			// Try to fold pure function calls with constant arguments
			if folded := cf.foldPureFunctionCall(inst, localConstants); folded != nil {
				function.Instructions[i] = *folded
				cf.metrics.ConstantsFolded++
				cf.metrics.CyclesSavedFolding += cf.getFunctionCallSavings(inst.Target)
			}
		}
	}
}

// foldBinaryOperation attempts to fold a binary operation with constant operands
func (cf *ConstantFolder) foldBinaryOperation(inst ir.Instruction, constants map[string]ir.Value) *ir.Instruction {
	leftKey := fmt.Sprintf("r%d", inst.Left)
	rightKey := fmt.Sprintf("r%d", inst.Right)
	
	leftVal, leftIsConst := constants[leftKey]
	rightVal, rightIsConst := constants[rightKey]
	
	// Need both operands to be constants
	if !leftIsConst || !rightIsConst {
		return nil
	}
	
	// Perform the operation at compile time
	result, err := cf.performBinaryOperation(inst.Op, leftVal, rightVal)
	if err != nil {
		return nil
	}
	
	// Create a new LoadConst instruction
	return &ir.Instruction{
		Op:    ir.OpLoadConst,
		Dest:  inst.Dest,
		Value: result,
		Type:  inst.Type,
		Comment: fmt.Sprintf("Constant folded: %v %s %v = %v", 
			leftVal, cf.getOpSymbol(inst.Op), rightVal, result),
	}
}

// performBinaryOperation executes a binary operation on constant values
func (cf *ConstantFolder) performBinaryOperation(op ir.OpCode, left, right ir.Value) (ir.Value, error) {
	// Handle different value types
	if left.Type != right.Type {
		return ir.Value{}, fmt.Errorf("type mismatch in binary operation")
	}
	
	switch left.Type.Kind {
	case ir.TypeInt:
		return cf.performIntOperation(op, left.IntVal, right.IntVal)
	case ir.TypeUInt:
		return cf.performUIntOperation(op, left.UIntVal, right.UIntVal)
	case ir.TypeBool:
		return cf.performBoolOperation(op, left.BoolVal, right.BoolVal)
	default:
		return ir.Value{}, fmt.Errorf("unsupported type for constant folding: %v", left.Type)
	}
}

// performIntOperation performs integer arithmetic operations
func (cf *ConstantFolder) performIntOperation(op ir.OpCode, left, right int64) (ir.Value, error) {
	var result int64
	
	switch op {
	case ir.OpAdd:
		result = left + right
	case ir.OpSub:
		result = left - right
	case ir.OpMul:
		result = left * right
	case ir.OpDiv:
		if right == 0 {
			return ir.Value{}, fmt.Errorf("division by zero")
		}
		result = left / right
	case ir.OpMod:
		if right == 0 {
			return ir.Value{}, fmt.Errorf("modulo by zero")
		}
		result = left % right
	case ir.OpShl:
		result = left << uint(right)
	case ir.OpShr:
		result = left >> uint(right)
	case ir.OpAnd:
		result = left & right
	case ir.OpOr:
		result = left | right
	case ir.OpXor:
		result = left ^ right
	default:
		return ir.Value{}, fmt.Errorf("unsupported integer operation: %v", op)
	}
	
	return ir.Value{Type: ir.Type{Kind: ir.TypeInt}, IntVal: result}, nil
}

// performUIntOperation performs unsigned integer arithmetic operations
func (cf *ConstantFolder) performUIntOperation(op ir.OpCode, left, right uint64) (ir.Value, error) {
	var result uint64
	
	switch op {
	case ir.OpAdd:
		result = left + right
	case ir.OpSub:
		result = left - right
	case ir.OpMul:
		result = left * right
	case ir.OpDiv:
		if right == 0 {
			return ir.Value{}, fmt.Errorf("division by zero")
		}
		result = left / right
	case ir.OpMod:
		if right == 0 {
			return ir.Value{}, fmt.Errorf("modulo by zero")
		}
		result = left % right
	case ir.OpShl:
		result = left << right
	case ir.OpShr:
		result = left >> right
	case ir.OpAnd:
		result = left & right
	case ir.OpOr:
		result = left | right
	case ir.OpXor:
		result = left ^ right
	default:
		return ir.Value{}, fmt.Errorf("unsupported unsigned operation: %v", op)
	}
	
	return ir.Value{Type: ir.Type{Kind: ir.TypeUInt}, UIntVal: result}, nil
}

// performBoolOperation performs boolean operations
func (cf *ConstantFolder) performBoolOperation(op ir.OpCode, left, right bool) (ir.Value, error) {
	var result bool
	
	switch op {
	case ir.OpAnd:
		result = left && right
	case ir.OpOr:
		result = left || right
	case ir.OpXor:
		result = left != right // XOR for booleans
	default:
		return ir.Value{}, fmt.Errorf("unsupported boolean operation: %v", op)
	}
	
	return ir.Value{Type: ir.Type{Kind: ir.TypeBool}, BoolVal: result}, nil
}

// foldPureFunctionCall attempts to fold calls to pure functions with constant arguments
func (cf *ConstantFolder) foldPureFunctionCall(inst ir.Instruction, constants map[string]ir.Value) *ir.Instruction {
	// Check if function is pure (no side effects)
	if !cf.isPureFunction(inst.Target) {
		return nil
	}
	
	// Check if all arguments are constants
	constArgs := make([]ir.Value, len(inst.Args))
	for i, arg := range inst.Args {
		argKey := fmt.Sprintf("r%d", arg)
		if constVal, isConst := constants[argKey]; isConst {
			constArgs[i] = constVal
		} else {
			return nil // Not all arguments are constant
		}
	}
	
	// Evaluate the function call at compile time
	result := cf.evaluatePureFunctionCall(inst.Target, constArgs)
	if !result.IsValid() {
		return nil
	}
	
	// Create a LoadConst instruction with the result
	return &ir.Instruction{
		Op:    ir.OpLoadConst,
		Dest:  inst.Dest,
		Value: result,
		Type:  inst.Type,
		Comment: fmt.Sprintf("Pure function folded: %s(...) = %v", inst.Target, result),
	}
}

// isPureFunction determines if a function is pure (no side effects)
func (cf *ConstantFolder) isPureFunction(functionName string) bool {
	pureFunctions := map[string]bool{
		"add":    true,
		"sub":    true,
		"mul":    true,
		"div":    true,
		"mod":    true,
		"abs":    true,
		"min":    true,
		"max":    true,
		"sqrt":   true,
		"pow":    true,
	}
	
	return pureFunctions[functionName]
}

// evaluatePureFunctionCall evaluates a pure function call with constant arguments
func (cf *ConstantFolder) evaluatePureFunctionCall(functionName string, args []ir.Value) ir.Value {
	switch functionName {
	case "add":
		if len(args) == 2 && args[0].Type.Kind == args[1].Type.Kind {
			if args[0].Type.Kind == ir.TypeInt {
				return ir.Value{Type: ir.Type{Kind: ir.TypeInt}, IntVal: args[0].IntVal + args[1].IntVal}
			} else if args[0].Type.Kind == ir.TypeUInt {
				return ir.Value{Type: ir.Type{Kind: ir.TypeUInt}, UIntVal: args[0].UIntVal + args[1].UIntVal}
			}
		}
	case "sub":
		if len(args) == 2 && args[0].Type.Kind == args[1].Type.Kind {
			if args[0].Type.Kind == ir.TypeInt {
				return ir.Value{Type: ir.Type{Kind: ir.TypeInt}, IntVal: args[0].IntVal - args[1].IntVal}
			} else if args[0].Type.Kind == ir.TypeUInt {
				return ir.Value{Type: ir.Type{Kind: ir.TypeUInt}, UIntVal: args[0].UIntVal - args[1].UIntVal}
			}
		}
	case "mul":
		if len(args) == 2 && args[0].Type.Kind == args[1].Type.Kind {
			if args[0].Type.Kind == ir.TypeInt {
				return ir.Value{Type: ir.Type{Kind: ir.TypeInt}, IntVal: args[0].IntVal * args[1].IntVal}
			} else if args[0].Type.Kind == ir.TypeUInt {
				return ir.Value{Type: ir.Type{Kind: ir.TypeUInt}, UIntVal: args[0].UIntVal * args[1].UIntVal}
			}
		}
	case "abs":
		if len(args) == 1 && args[0].Type.Kind == ir.TypeInt {
			val := args[0].IntVal
			if val < 0 {
				val = -val
			}
			return ir.Value{Type: ir.Type{Kind: ir.TypeInt}, IntVal: val}
		}
	case "sqrt":
		if len(args) == 1 && args[0].Type.Kind == ir.TypeUInt {
			result := uint64(math.Sqrt(float64(args[0].UIntVal)))
			return ir.Value{Type: ir.Type{Kind: ir.TypeUInt}, UIntVal: result}
		}
	}
	
	return ir.Value{} // Invalid value indicates folding failed
}

// evaluateConstantExpression evaluates an AST expression to a constant value
func (cf *ConstantFolder) evaluateConstantExpression(expr ast.Expression) ir.Value {
	switch e := expr.(type) {
	case *ast.LiteralExpr:
		return cf.convertLiteralToValue(e)
	case *ast.BinaryExpr:
		left := cf.evaluateConstantExpression(e.Left)
		right := cf.evaluateConstantExpression(e.Right)
		if left.IsValid() && right.IsValid() {
			result, err := cf.performBinaryOperation(cf.convertASTOpToIR(e.Op), left, right)
			if err == nil {
				return result
			}
		}
	case *ast.IdentifierExpr:
		// Look up constant value
		if val, exists := cf.constants[e.Name]; exists {
			return val
		}
	}
	
	return ir.Value{} // Invalid value
}

// convertLiteralToValue converts an AST literal to an IR value
func (cf *ConstantFolder) convertLiteralToValue(literal *ast.LiteralExpr) ir.Value {
	switch literal.Type {
	case ast.IntLiteral:
		return ir.Value{Type: ir.Type{Kind: ir.TypeInt}, IntVal: literal.IntValue}
	case ast.UIntLiteral:
		return ir.Value{Type: ir.Type{Kind: ir.TypeUInt}, UIntVal: literal.UIntValue}
	case ast.BoolLiteral:
		return ir.Value{Type: ir.Type{Kind: ir.TypeBool}, BoolVal: literal.BoolValue}
	case ast.StringLiteral:
		return ir.Value{Type: ir.Type{Kind: ir.TypeString}, StringVal: literal.StringValue}
	}
	return ir.Value{}
}

// convertASTOpToIR converts AST binary operators to IR opcodes
func (cf *ConstantFolder) convertASTOpToIR(op ast.BinaryOp) ir.OpCode {
	switch op {
	case ast.Add:
		return ir.OpAdd
	case ast.Sub:
		return ir.OpSub
	case ast.Mul:
		return ir.OpMul
	case ast.Div:
		return ir.OpDiv
	case ast.Mod:
		return ir.OpMod
	case ast.Shl:
		return ir.OpShl
	case ast.Shr:
		return ir.OpShr
	case ast.And:
		return ir.OpAnd
	case ast.Or:
		return ir.OpOr
	case ast.Xor:
		return ir.OpXor
	default:
		return ir.OpInvalid
	}
}

// eliminateDeadConstants removes unused constant declarations
func (cf *ConstantFolder) eliminateDeadConstants(module *ir.Module) {
	usedConstants := make(map[string]bool)
	
	// Mark constants used in functions
	for _, function := range module.Functions {
		for _, inst := range function.Instructions {
			if inst.Op == ir.OpLoadConst {
				// Mark constant as used (this is simplified)
				usedConstants[fmt.Sprintf("const_%v", inst.Value)] = true
			}
		}
	}
	
	// Remove unused global constants
	filteredGlobals := make([]*ir.Global, 0)
	for _, global := range module.Globals {
		if !global.IsConst || usedConstants[global.Name] {
			filteredGlobals = append(filteredGlobals, global)
		} else {
			cf.metrics.DeadCodeEliminated++
		}
	}
	
	module.Globals = filteredGlobals
}

// Utility functions for optimization metrics

func (cf *ConstantFolder) getCycleSavings(op ir.OpCode) int {
	// Estimated T-state savings for Z80
	switch op {
	case ir.OpAdd, ir.OpSub:
		return 4 // Saved ADD/SUB instruction
	case ir.OpMul:
		return 30 // Multiplication is expensive on Z80
	case ir.OpDiv:
		return 40 // Division is very expensive
	case ir.OpShl, ir.OpShr:
		return 8 // Shift operations
	default:
		return 4 // General arithmetic
	}
}

func (cf *ConstantFolder) getByteSavings(op ir.OpCode) int {
	// Estimated byte savings (instruction eliminated)
	return 1 // Most Z80 instructions are 1-3 bytes, average 1
}

func (cf *ConstantFolder) getFunctionCallSavings(functionName string) int {
	// Function call overhead savings in T-states
	switch functionName {
	case "add", "sub":
		return 17 // CALL + RET overhead
	case "mul":
		return 45 // Call overhead + multiplication routine
	case "div":
		return 80 // Call overhead + division routine
	default:
		return 17 // Basic call overhead
	}
}

func (cf *ConstantFolder) getOpSymbol(op ir.OpCode) string {
	switch op {
	case ir.OpAdd:
		return "+"
	case ir.OpSub:
		return "-"
	case ir.OpMul:
		return "*"
	case ir.OpDiv:
		return "/"
	case ir.OpMod:
		return "%"
	case ir.OpShl:
		return "<<"
	case ir.OpShr:
		return ">>"
	case ir.OpAnd:
		return "&"
	case ir.OpOr:
		return "|"
	case ir.OpXor:
		return "^"
	default:
		return "?"
	}
}

// GetMetrics returns the optimization metrics
func (cf *ConstantFolder) GetMetrics() *OptimizationMetrics {
	return cf.metrics
}