package ctie

import (
	"fmt"
	"math"

	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/mirvm"
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

func (v IntValue) Type() ir.Type  { return v.Size }
func (v IntValue) String() string { return fmt.Sprintf("%d", v.Val) }
func (v IntValue) ToInt() int64   { return v.Val }
func (v IntValue) ToBool() bool   { return v.Val != 0 }
func (v IntValue) Clone() Value   { return IntValue{Val: v.Val, Size: v.Size} }

// BoolValue represents a boolean at compile time
type BoolValue struct {
	Val bool
}

func (v BoolValue) Type() ir.Type { return &ir.BasicType{Kind: ir.TypeBool} }
func (v BoolValue) String() string {
	return fmt.Sprintf("%v", v.Val)
}
func (v BoolValue) ToInt() int64 {
	if v.Val {
		return 1
	}
	return 0
}
func (v BoolValue) ToBool() bool { return v.Val }
func (v BoolValue) Clone() Value { return BoolValue{Val: v.Val} }

// ArrayValue represents an array at compile time
type ArrayValue struct {
	Elements []Value
	ElemType ir.Type
}

func (v ArrayValue) Type() ir.Type  { return v.ElemType }
func (v ArrayValue) String() string { return fmt.Sprintf("[%d elements]", len(v.Elements)) }
func (v ArrayValue) ToInt() int64   { return int64(len(v.Elements)) }
func (v ArrayValue) ToBool() bool   { return len(v.Elements) > 0 }
func (v ArrayValue) Clone() Value {
	cloned := make([]Value, len(v.Elements))
	for i, elem := range v.Elements {
		cloned[i] = elem.Clone()
	}
	return ArrayValue{Elements: cloned, ElemType: v.ElemType}
}

// CompileTimeExecutor executes MIR at compile time using mirvm.VM as the
// execution backend. This replaces the old stack-based executor which had
// impedance mismatch with MIR's register-based instructions.
type CompileTimeExecutor struct {
	module      *ir.Module
	purity      *PurityAnalyzer
	cache       map[string]Value // Memoization cache
	diagnostics []string
	useLegacy   bool // Set true to fall back to old stack-based executor
}

// NewCompileTimeExecutor creates a new compile-time executor backed by mirvm.VM.
func NewCompileTimeExecutor(module *ir.Module) *CompileTimeExecutor {
	return &CompileTimeExecutor{
		module: module,
		purity: NewPurityAnalyzer(module),
		cache:  make(map[string]Value),
	}
}

// SetLegacyMode enables the old stack-based executor as fallback.
func (e *CompileTimeExecutor) SetLegacyMode(legacy bool) {
	e.useLegacy = legacy
}

// Execute runs a function at compile time with given arguments.
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

	var result Value
	var err error

	if e.useLegacy {
		result, err = e.executeLegacy(fn, args)
	} else {
		result, err = e.executeMirvm(fn, args)
	}

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

// executeMirvm runs the function using mirvm.VM — the reliable register-based backend.
func (e *CompileTimeExecutor) executeMirvm(fn *ir.Function, args []Value) (Value, error) {
	// Build a temporary module with the target function and any callees
	tempModule := &ir.Module{
		Functions: e.module.Functions,
		Globals:   e.module.Globals,
	}

	config := mirvm.Config{
		MemorySize: 4096,
		StackSize:  1024,
		MaxSteps:   10000,
	}
	vm := mirvm.New(config)
	if err := vm.LoadModule(tempModule); err != nil {
		return nil, fmt.Errorf("mirvm load failed: %v", err)
	}

	// Set up argument registers (MIR convention: params in r1, r2, ...)
	for i, arg := range args {
		vm.SetRegister(i+1, arg.ToInt())
	}

	// Run the specific function
	retVal, err := vm.RunFunction(fn.Name)
	if err != nil {
		return nil, fmt.Errorf("mirvm execution failed for %s: %v", fn.Name, err)
	}

	// Wrap return value
	resultType := fn.ReturnType
	if resultType == nil {
		resultType = &ir.BasicType{Kind: ir.TypeU8}
	}
	return IntValue{Val: retVal, Size: resultType}, nil
}

// executeLegacy is the old stack-based executor kept as fallback.
func (e *CompileTimeExecutor) executeLegacy(fn *ir.Function, args []Value) (Value, error) {
	ctx := &legacyExecutionContext{
		stack:     make([]Value, 0, 256),
		locals:    make(map[string]Value),
		globals:   make(map[string]Value),
		memory:    make(map[int64]Value),
		callDepth: 0,
		maxDepth:  100,
		instCount: 0,
		maxInsts:  10000,
	}

	// Set up parameters
	for i, param := range fn.Params {
		if i < len(args) {
			ctx.locals[param.Name] = args[i]
		}
	}

	// Build label map for this function
	ctx.labelMap = make(map[string]int)
	for i, inst := range fn.Instructions {
		if inst.Op == ir.OpLabel && inst.Label != "" {
			ctx.labelMap[inst.Label] = i
		}
	}

	return e.executeFunctionLegacy(fn, ctx)
}

// legacyExecutionContext holds the state for the old stack-based executor.
type legacyExecutionContext struct {
	stack     []Value
	locals    map[string]Value
	globals   map[string]Value
	memory    map[int64]Value
	returnVal Value
	pc        int
	callDepth int
	maxDepth  int
	instCount int
	maxInsts  int
	labelMap  map[string]int
}

func (e *CompileTimeExecutor) executeFunctionLegacy(fn *ir.Function, ctx *legacyExecutionContext) (Value, error) {
	ctx.callDepth++
	if ctx.callDepth > ctx.maxDepth {
		return nil, fmt.Errorf("max recursion depth exceeded")
	}
	defer func() { ctx.callDepth-- }()

	for ctx.pc = 0; ctx.pc < len(fn.Instructions); {
		inst := &fn.Instructions[ctx.pc]
		ctx.instCount++
		if ctx.instCount > ctx.maxInsts {
			return nil, fmt.Errorf("max instruction count exceeded (possible infinite loop)")
		}
		if err := e.executeInstructionLegacy(inst, ctx); err != nil {
			return nil, fmt.Errorf("at instruction %d: %v", ctx.pc, err)
		}
		if ctx.returnVal != nil {
			return ctx.returnVal, nil
		}
		ctx.pc++
	}

	if len(ctx.stack) > 0 {
		return ctx.stack[len(ctx.stack)-1], nil
	}
	return nil, nil
}

func (e *CompileTimeExecutor) executeInstructionLegacy(inst *ir.Instruction, ctx *legacyExecutionContext) error {
	switch inst.Op {
	case ir.OpAdd:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 { return a + b })
	case ir.OpSub:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 { return a - b })
	case ir.OpMul:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 { return a * b })
	case ir.OpDiv:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 {
			if b == 0 {
				panic("division by zero")
			}
			return a / b
		})
	case ir.OpMod:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 {
			if b == 0 {
				panic("modulo by zero")
			}
			return a % b
		})
	case ir.OpAnd:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 { return a & b })
	case ir.OpOr:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 { return a | b })
	case ir.OpXor:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 { return a ^ b })
	case ir.OpNot:
		return e.legacyUnaryOp(ctx, func(a int64) int64 { return ^a })
	case ir.OpShl:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 { return a << uint(b) })
	case ir.OpShr:
		return e.legacyBinaryOp(ctx, func(a, b int64) int64 { return a >> uint(b) })
	case ir.OpEq:
		return e.legacyComparison(ctx, func(a, b int64) bool { return a == b })
	case ir.OpNe:
		return e.legacyComparison(ctx, func(a, b int64) bool { return a != b })
	case ir.OpLt:
		return e.legacyComparison(ctx, func(a, b int64) bool { return a < b })
	case ir.OpLe:
		return e.legacyComparison(ctx, func(a, b int64) bool { return a <= b })
	case ir.OpGt:
		return e.legacyComparison(ctx, func(a, b int64) bool { return a > b })
	case ir.OpGe:
		return e.legacyComparison(ctx, func(a, b int64) bool { return a >= b })
	case ir.OpLoadConst:
		val := IntValue{Val: inst.Imm, Size: inst.Type}
		ctx.stack = append(ctx.stack, val)
	case ir.OpLoadVar:
		if val, ok := ctx.locals[inst.Symbol]; ok {
			ctx.stack = append(ctx.stack, val)
		} else {
			return fmt.Errorf("undefined variable: %s", inst.Symbol)
		}
	case ir.OpStoreVar:
		if len(ctx.stack) == 0 {
			return fmt.Errorf("stack underflow")
		}
		val := ctx.stack[len(ctx.stack)-1]
		ctx.stack = ctx.stack[:len(ctx.stack)-1]
		ctx.locals[inst.Symbol] = val
	case ir.OpLoadParam:
		if val, ok := ctx.locals[inst.Symbol]; ok {
			ctx.stack = append(ctx.stack, val)
		} else {
			return fmt.Errorf("undefined parameter: %s", inst.Symbol)
		}
	case ir.OpJump:
		if targetPC, ok := ctx.labelMap[inst.Label]; ok {
			ctx.pc = targetPC - 1
		} else {
			return fmt.Errorf("undefined label: %s", inst.Label)
		}
	case ir.OpJumpIf:
		if len(ctx.stack) == 0 {
			return fmt.Errorf("stack underflow")
		}
		cond := ctx.stack[len(ctx.stack)-1]
		ctx.stack = ctx.stack[:len(ctx.stack)-1]
		if cond.ToBool() {
			if targetPC, ok := ctx.labelMap[inst.Label]; ok {
				ctx.pc = targetPC - 1
			} else {
				return fmt.Errorf("undefined label: %s", inst.Label)
			}
		}
	case ir.OpLabel:
		// No-op — labels resolved via labelMap
	case ir.OpReturn:
		if len(ctx.stack) > 0 {
			ctx.returnVal = ctx.stack[len(ctx.stack)-1]
		}
	case ir.OpCall:
		return e.legacyCall(inst, ctx)
	default:
		return fmt.Errorf("unsupported operation for compile-time execution: %v", inst.Op)
	}
	return nil
}

func (e *CompileTimeExecutor) legacyBinaryOp(ctx *legacyExecutionContext, op func(int64, int64) int64) error {
	if len(ctx.stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := ctx.stack[len(ctx.stack)-1]
	a := ctx.stack[len(ctx.stack)-2]
	ctx.stack = ctx.stack[:len(ctx.stack)-2]
	result := op(a.ToInt(), b.ToInt())
	ctx.stack = append(ctx.stack, IntValue{Val: result, Size: a.Type()})
	return nil
}

func (e *CompileTimeExecutor) legacyUnaryOp(ctx *legacyExecutionContext, op func(int64) int64) error {
	if len(ctx.stack) < 1 {
		return fmt.Errorf("stack underflow")
	}
	a := ctx.stack[len(ctx.stack)-1]
	ctx.stack = ctx.stack[:len(ctx.stack)-1]
	result := op(a.ToInt())
	ctx.stack = append(ctx.stack, IntValue{Val: result, Size: a.Type()})
	return nil
}

func (e *CompileTimeExecutor) legacyComparison(ctx *legacyExecutionContext, op func(int64, int64) bool) error {
	if len(ctx.stack) < 2 {
		return fmt.Errorf("stack underflow")
	}
	b := ctx.stack[len(ctx.stack)-1]
	a := ctx.stack[len(ctx.stack)-2]
	ctx.stack = ctx.stack[:len(ctx.stack)-2]
	result := op(a.ToInt(), b.ToInt())
	ctx.stack = append(ctx.stack, BoolValue{Val: result})
	return nil
}

func (e *CompileTimeExecutor) legacyCall(inst *ir.Instruction, ctx *legacyExecutionContext) error {
	var fn *ir.Function
	for _, f := range e.module.Functions {
		if f.Name == inst.Symbol {
			fn = f
			break
		}
	}
	if fn == nil {
		return e.legacyBuiltin(inst.Symbol, ctx)
	}
	if !e.purity.IsPure(fn) {
		return fmt.Errorf("cannot call impure function %s at compile-time", fn.Name)
	}
	argCount := len(fn.Params)
	if len(ctx.stack) < argCount {
		return fmt.Errorf("not enough arguments for %s", fn.Name)
	}
	args := make([]Value, argCount)
	for i := argCount - 1; i >= 0; i-- {
		args[i] = ctx.stack[len(ctx.stack)-1]
		ctx.stack = ctx.stack[:len(ctx.stack)-1]
	}
	result, err := e.Execute(fn, args)
	if err != nil {
		return fmt.Errorf("error calling %s: %v", fn.Name, err)
	}
	if result != nil {
		ctx.stack = append(ctx.stack, result)
	}
	return nil
}

func (e *CompileTimeExecutor) legacyBuiltin(name string, ctx *legacyExecutionContext) error {
	switch name {
	case "abs":
		if len(ctx.stack) < 1 {
			return fmt.Errorf("stack underflow")
		}
		val := ctx.stack[len(ctx.stack)-1].ToInt()
		ctx.stack[len(ctx.stack)-1] = IntValue{Val: int64(math.Abs(float64(val))), Size: &ir.BasicType{Kind: ir.TypeU8}}
		return nil
	case "min":
		if len(ctx.stack) < 2 {
			return fmt.Errorf("stack underflow")
		}
		b := ctx.stack[len(ctx.stack)-1].ToInt()
		a := ctx.stack[len(ctx.stack)-2].ToInt()
		ctx.stack = ctx.stack[:len(ctx.stack)-2]
		if a < b {
			ctx.stack = append(ctx.stack, IntValue{Val: a, Size: &ir.BasicType{Kind: ir.TypeU8}})
		} else {
			ctx.stack = append(ctx.stack, IntValue{Val: b, Size: &ir.BasicType{Kind: ir.TypeU8}})
		}
		return nil
	case "max":
		if len(ctx.stack) < 2 {
			return fmt.Errorf("stack underflow")
		}
		b := ctx.stack[len(ctx.stack)-1].ToInt()
		a := ctx.stack[len(ctx.stack)-2].ToInt()
		ctx.stack = ctx.stack[:len(ctx.stack)-2]
		if a > b {
			ctx.stack = append(ctx.stack, IntValue{Val: a, Size: &ir.BasicType{Kind: ir.TypeU8}})
		} else {
			ctx.stack = append(ctx.stack, IntValue{Val: b, Size: &ir.BasicType{Kind: ir.TypeU8}})
		}
		return nil
	default:
		return fmt.Errorf("unknown builtin: %s", name)
	}
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
