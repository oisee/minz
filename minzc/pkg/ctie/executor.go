package ctie

import (
	"fmt"
	"math"
	"github.com/minz/minzc/pkg/ir"
)

// Value represents a compile-time value
type Value interface {
	Type() ir.Type
	String() string
	ToInt() int64
	ToBool() bool
	Clone() Value
}

// IntValue represents an integer at compile time
type IntValue struct {
	Val  int64
	Size ir.Type // u8, u16, i8, i16, etc.
}

func (v IntValue) Type() ir.Type    { return v.Size }
func (v IntValue) String() string   { return fmt.Sprintf("%d", v.Val) }
func (v IntValue) ToInt() int64     { return v.Val }
func (v IntValue) ToBool() bool     { return v.Val != 0 }
func (v IntValue) Clone() Value     { return IntValue{Val: v.Val, Size: v.Size} }

// BoolValue represents a boolean at compile time
type BoolValue struct {
	Val bool
}

func (v BoolValue) Type() ir.Type    { return &ir.BasicType{Kind: ir.TypeBool} }
func (v BoolValue) String() string   { return fmt.Sprintf("%v", v.Val) }
func (v BoolValue) ToInt() int64     { if v.Val { return 1 } else { return 0 } }
func (v BoolValue) ToBool() bool     { return v.Val }
func (v BoolValue) Clone() Value     { return BoolValue{Val: v.Val} }

// ArrayValue represents an array at compile time
type ArrayValue struct {
	Elements []Value
	ElemType ir.Type
}

func (v ArrayValue) Type() ir.Type    { return v.ElemType } // Return element type for now
func (v ArrayValue) String() string   { return fmt.Sprintf("[%d elements]", len(v.Elements)) }
func (v ArrayValue) ToInt() int64     { return int64(len(v.Elements)) }
func (v ArrayValue) ToBool() bool     { return len(v.Elements) > 0 }
func (v ArrayValue) Clone() Value {
	cloned := make([]Value, len(v.Elements))
	for i, elem := range v.Elements {
		cloned[i] = elem.Clone()
	}
	return ArrayValue{Elements: cloned, ElemType: v.ElemType}
}

// ExecutionContext holds the state for compile-time execution
type ExecutionContext struct {
	Stack      []Value
	Locals     map[string]Value
	Globals    map[string]Value
	Memory     map[int64]Value
	ReturnVal  Value
	PC         int // Program counter
	CallDepth  int
	MaxDepth   int
	InstCount  int
	MaxInsts   int
}

// CompileTimeExecutor executes MIR at compile time
type CompileTimeExecutor struct {
	module       *ir.Module
	purity       *PurityAnalyzer
	cache        map[string]Value // Memoization cache
	diagnostics  []string
}

// NewCompileTimeExecutor creates a new compile-time executor
func NewCompileTimeExecutor(module *ir.Module) *CompileTimeExecutor {
	return &CompileTimeExecutor{
		module:  module,
		purity:  NewPurityAnalyzer(module),
		cache:   make(map[string]Value),
	}
}

// Execute runs a function at compile time with given arguments
func (e *CompileTimeExecutor) Execute(fn *ir.Function, args []Value) (Value, error) {
	// Check if function is pure
	if !e.purity.IsPure(fn) {
		return nil, fmt.Errorf("function %s is not pure, cannot execute at compile-time", fn.Name)
	}

	// Check memoization cache for const functions
	if e.purity.IsConst(fn) {
		cacheKey := e.makeCacheKey(fn.Name, args)
		if cached, ok := e.cache[cacheKey]; ok {
			return cached, nil
		}
	}

	// Create execution context
	ctx := &ExecutionContext{
		Stack:     make([]Value, 0, 256),
		Locals:    make(map[string]Value),
		Globals:   make(map[string]Value),
		Memory:    make(map[int64]Value),
		CallDepth: 0,
		MaxDepth:  100,
		InstCount: 0,
		MaxInsts:  10000, // Prevent infinite loops
	}

	// Set up parameters
	for i, param := range fn.Params {
		if i < len(args) {
			ctx.Locals[param.Name] = args[i]
		}
	}

	// Execute function
	result, err := e.executeFunction(fn, ctx)
	if err != nil {
		return nil, err
	}

	// Cache result if const
	if e.purity.IsConst(fn) {
		cacheKey := e.makeCacheKey(fn.Name, args)
		e.cache[cacheKey] = result
	}

	return result, nil
}

// executeFunction executes a single function
func (e *CompileTimeExecutor) executeFunction(fn *ir.Function, ctx *ExecutionContext) (Value, error) {
	// Check recursion depth
	ctx.CallDepth++
	if ctx.CallDepth > ctx.MaxDepth {
		return nil, fmt.Errorf("max recursion depth exceeded")
	}
	defer func() { ctx.CallDepth-- }()

	// Execute instructions
	for ctx.PC = 0; ctx.PC < len(fn.Instructions); {
		inst := &fn.Instructions[ctx.PC]
		
		// Check instruction limit
		ctx.InstCount++
		if ctx.InstCount > ctx.MaxInsts {
			return nil, fmt.Errorf("max instruction count exceeded (possible infinite loop)")
		}

		// Execute instruction
		if err := e.executeInstruction(inst, ctx); err != nil {
			return nil, fmt.Errorf("at instruction %d: %v", ctx.PC, err)
		}

		// Check for return
		if ctx.ReturnVal != nil {
			return ctx.ReturnVal, nil
		}

		ctx.PC++
	}

	// No explicit return - return last stack value or nil
	if len(ctx.Stack) > 0 {
		return ctx.Stack[len(ctx.Stack)-1], nil
	}
	return nil, nil
}

// executeInstruction executes a single MIR instruction
func (e *CompileTimeExecutor) executeInstruction(inst *ir.Instruction, ctx *ExecutionContext) error {
	switch inst.Op {
	// Arithmetic operations
	case ir.OpAdd:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 { return a + b })
	case ir.OpSub:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 { return a - b })
	case ir.OpMul:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 { return a * b })
	case ir.OpDiv:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 {
			if b == 0 {
				panic("division by zero")
			}
			return a / b
		})
	case ir.OpMod:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 {
			if b == 0 {
				panic("modulo by zero")
			}
			return a % b
		})

	// Bitwise operations
	case ir.OpAnd:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 { return a & b })
	case ir.OpOr:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 { return a | b })
	case ir.OpXor:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 { return a ^ b })
	case ir.OpNot:
		return e.executeUnaryOp(ctx, func(a int64) int64 { return ^a })
	case ir.OpShl:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 { return a << uint(b) })
	case ir.OpShr:
		return e.executeBinaryOp(ctx, func(a, b int64) int64 { return a >> uint(b) })

	// Comparison operations
	case ir.OpEq:
		return e.executeComparison(ctx, func(a, b int64) bool { return a == b })
	case ir.OpNe:
		return e.executeComparison(ctx, func(a, b int64) bool { return a != b })
	case ir.OpLt:
		return e.executeComparison(ctx, func(a, b int64) bool { return a < b })
	case ir.OpLe:
		return e.executeComparison(ctx, func(a, b int64) bool { return a <= b })
	case ir.OpGt:
		return e.executeComparison(ctx, func(a, b int64) bool { return a > b })
	case ir.OpGe:
		return e.executeComparison(ctx, func(a, b int64) bool { return a >= b })

	// Stack operations
	case ir.OpLoadConst:
		val := e.parseConstant(inst)
		ctx.Stack = append(ctx.Stack, val)
		return nil

	case ir.OpLoadVar:
		if val, ok := ctx.Locals[inst.Symbol]; ok {
			ctx.Stack = append(ctx.Stack, val)
		} else {
			return fmt.Errorf("undefined variable: %s", inst.Symbol)
		}
		return nil

	case ir.OpStoreVar:
		if len(ctx.Stack) == 0 {
			return fmt.Errorf("stack underflow")
		}
		val := ctx.Stack[len(ctx.Stack)-1]
		ctx.Stack = ctx.Stack[:len(ctx.Stack)-1]
		ctx.Locals[inst.Symbol] = val
		return nil

	case ir.OpLoadParam:
		if val, ok := ctx.Locals[inst.Symbol]; ok {
			ctx.Stack = append(ctx.Stack, val)
		} else {
			return fmt.Errorf("undefined parameter: %s", inst.Symbol)
		}
		return nil

	// Control flow
	case ir.OpJump:
		// Find target label
		targetPC := e.findLabel(inst.Label)
		if targetPC < 0 {
			return fmt.Errorf("undefined label: %s", inst.Label)
		}
		ctx.PC = targetPC - 1 // -1 because PC will be incremented
		return nil

	case ir.OpJumpIf:
		if len(ctx.Stack) == 0 {
			return fmt.Errorf("stack underflow")
		}
		cond := ctx.Stack[len(ctx.Stack)-1]
		ctx.Stack = ctx.Stack[:len(ctx.Stack)-1]
		
		if cond.ToBool() {
			targetPC := e.findLabel(inst.Label)
			if targetPC < 0 {
				return fmt.Errorf("undefined label: %s", inst.Label)
			}
			ctx.PC = targetPC - 1
		}
		return nil

	case ir.OpReturn:
		if len(ctx.Stack) > 0 {
			ctx.ReturnVal = ctx.Stack[len(ctx.Stack)-1]
		}
		return nil

	// Function calls
	case ir.OpCall:
		return e.executeCall(inst, ctx)

	// These opcodes don't exist yet, skip for now
	// TODO: Add compile-time specific opcodes

	default:
		return fmt.Errorf("unsupported operation for compile-time execution: %v", inst.Op)
	}
}

// executeBinaryOp executes a binary operation
func (e *CompileTimeExecutor) executeBinaryOp(ctx *ExecutionContext, op func(int64, int64) int64) error {
	if len(ctx.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	
	b := ctx.Stack[len(ctx.Stack)-1]
	a := ctx.Stack[len(ctx.Stack)-2]
	ctx.Stack = ctx.Stack[:len(ctx.Stack)-2]
	
	result := op(a.ToInt(), b.ToInt())
	ctx.Stack = append(ctx.Stack, IntValue{Val: result, Size: a.Type()})
	
	return nil
}

// executeUnaryOp executes a unary operation
func (e *CompileTimeExecutor) executeUnaryOp(ctx *ExecutionContext, op func(int64) int64) error {
	if len(ctx.Stack) < 1 {
		return fmt.Errorf("stack underflow")
	}
	
	a := ctx.Stack[len(ctx.Stack)-1]
	ctx.Stack = ctx.Stack[:len(ctx.Stack)-1]
	
	result := op(a.ToInt())
	ctx.Stack = append(ctx.Stack, IntValue{Val: result, Size: a.Type()})
	
	return nil
}

// executeComparison executes a comparison operation
func (e *CompileTimeExecutor) executeComparison(ctx *ExecutionContext, op func(int64, int64) bool) error {
	if len(ctx.Stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	
	b := ctx.Stack[len(ctx.Stack)-1]
	a := ctx.Stack[len(ctx.Stack)-2]
	ctx.Stack = ctx.Stack[:len(ctx.Stack)-2]
	
	result := op(a.ToInt(), b.ToInt())
	ctx.Stack = append(ctx.Stack, BoolValue{Val: result})
	
	return nil
}

// executeCall executes a function call
func (e *CompileTimeExecutor) executeCall(inst *ir.Instruction, ctx *ExecutionContext) error {
	// Get function
	var fn *ir.Function
	for _, f := range e.module.Functions {
		if f.Name == inst.Symbol {
			fn = f
			break
		}
	}
	if fn == nil {
		// Check for builtin
		return e.executeBuiltin(inst.Symbol, ctx)
	}

	// Check purity
	if !e.purity.IsPure(fn) {
		return fmt.Errorf("cannot call impure function %s at compile-time", fn.Name)
	}

	// Pop arguments
	argCount := len(fn.Params)
	if len(ctx.Stack) < argCount {
		return fmt.Errorf("not enough arguments for %s", fn.Name)
	}
	
	args := make([]Value, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = ctx.Stack[len(ctx.Stack)-1]
		ctx.Stack = ctx.Stack[:len(ctx.Stack)-1]
	}

	// Execute function
	result, err := e.Execute(fn, args)
	if err != nil {
		return fmt.Errorf("error calling %s: %v", fn.Name, err)
	}

	// Push result
	if result != nil {
		ctx.Stack = append(ctx.Stack, result)
	}
	
	return nil
}

// executeBuiltin executes a builtin function
func (e *CompileTimeExecutor) executeBuiltin(name string, ctx *ExecutionContext) error {
	switch name {
	case "abs":
		if len(ctx.Stack) < 1 {
			return fmt.Errorf("stack underflow")
		}
		val := ctx.Stack[len(ctx.Stack)-1].ToInt()
		ctx.Stack[len(ctx.Stack)-1] = IntValue{Val: int64(math.Abs(float64(val))), Size: &ir.BasicType{Kind: ir.TypeU8}}
		return nil
		
	case "min":
		if len(ctx.Stack) < 2 {
			return fmt.Errorf("stack underflow")
		}
		b := ctx.Stack[len(ctx.Stack)-1].ToInt()
		a := ctx.Stack[len(ctx.Stack)-2].ToInt()
		ctx.Stack = ctx.Stack[:len(ctx.Stack)-2]
		if a < b {
			ctx.Stack = append(ctx.Stack, IntValue{Val: a, Size: &ir.BasicType{Kind: ir.TypeU8}})
		} else {
			ctx.Stack = append(ctx.Stack, IntValue{Val: b, Size: &ir.BasicType{Kind: ir.TypeU8}})
		}
		return nil
		
	case "max":
		if len(ctx.Stack) < 2 {
			return fmt.Errorf("stack underflow")
		}
		b := ctx.Stack[len(ctx.Stack)-1].ToInt()
		a := ctx.Stack[len(ctx.Stack)-2].ToInt()
		ctx.Stack = ctx.Stack[:len(ctx.Stack)-2]
		if a > b {
			ctx.Stack = append(ctx.Stack, IntValue{Val: a, Size: &ir.BasicType{Kind: ir.TypeU8}})
		} else {
			ctx.Stack = append(ctx.Stack, IntValue{Val: b, Size: &ir.BasicType{Kind: ir.TypeU8}})
		}
		return nil
		
	default:
		return fmt.Errorf("unknown builtin: %s", name)
	}
}

// parseConstant parses a constant value from an instruction
func (e *CompileTimeExecutor) parseConstant(inst *ir.Instruction) Value {
	// Use immediate value
	return IntValue{Val: inst.Imm, Size: inst.Type}
}

// findLabel finds the PC for a label
func (e *CompileTimeExecutor) findLabel(label string) int {
	// This would need to be implemented based on how labels are stored
	// For now, return -1 (not found)
	return -1
}

// makeCacheKey creates a cache key for memoization
func (e *CompileTimeExecutor) makeCacheKey(name string, args []Value) string {
	key := name + "("
	for i, arg := range args {
		if i > 0 {
			key += ","
		}
		key += arg.String()
	}
	key += ")"
	return key
}

// GetDiagnostics returns diagnostic messages from execution
func (e *CompileTimeExecutor) GetDiagnostics() []string {
	return e.diagnostics
}

// AddDiagnostic adds a diagnostic message
func (e *CompileTimeExecutor) AddDiagnostic(msg string) {
	e.diagnostics = append(e.diagnostics, msg)
}