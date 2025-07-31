# ABI Implementation Design for MinZ

**Date**: July 27, 2025  
**Purpose**: Technical design for implementing adaptive ABI system

## Core Architecture

### ABI Interface
```go
type CallingConvention interface {
    // Analyze function to determine if this ABI is suitable
    CanHandle(fn *ir.Function) bool
    
    // Generate prologue (function entry)
    GeneratePrologue(g *Generator, fn *ir.Function)
    
    // Generate epilogue (function exit)  
    GenerateEpilogue(g *Generator, fn *ir.Function)
    
    // Generate parameter passing (caller side)
    GenerateCall(g *Generator, fn *ir.Function, args []ir.Value)
    
    // Access parameter inside function
    LoadParameter(g *Generator, param *ir.Parameter, index int)
    
    // Store return value
    StoreReturn(g *Generator, value ir.Value)
    
    // Get ABI name
    Name() string
}
```

### ABI Registry
```go
type ABIRegistry struct {
    conventions map[string]CallingConvention
    default     CallingConvention
}

func (r *ABIRegistry) SelectABI(fn *ir.Function) CallingConvention {
    // Check for explicit annotation
    if abi := fn.GetAnnotation("abi"); abi != "" {
        if conv, ok := r.conventions[abi]; ok {
            return conv
        }
    }
    
    // Try each ABI in priority order
    for _, conv := range r.priorityOrder {
        if conv.CanHandle(fn) {
            return conv
        }
    }
    
    return r.default
}
```

## Concrete Implementations

### 1. PhysicalRegisterABI
```go
type PhysicalRegisterABI struct {
    // Allocation strategy for parameters
    allocator *RegisterAllocator
}

func (abi *PhysicalRegisterABI) CanHandle(fn *ir.Function) bool {
    // Calculate register pressure
    regCount := 0
    for _, param := range fn.Params {
        switch param.Type.Size() {
        case 1: regCount += 1  // 8-bit uses one register
        case 2: regCount += 2  // 16-bit uses pair
        default: return false  // Too large
        }
    }
    
    // Z80 has limited registers: A, B, C, D, E, H, L, HL, DE, BC
    return regCount <= 6
}

func (abi *PhysicalRegisterABI) GenerateCall(g *Generator, fn *ir.Function, args []ir.Value) {
    // Example allocation order
    reg8Order := []string{"A", "E", "D", "C", "B", "L", "H"}
    reg16Order := []string{"HL", "DE", "BC"}
    
    reg8Idx := 0
    reg16Idx := 0
    
    for i, arg := range args {
        param := fn.Params[i]
        switch param.Type.Size() {
        case 1:
            g.LoadToReg(arg, reg8Order[reg8Idx])
            reg8Idx++
        case 2:
            g.LoadToRegPair(arg, reg16Order[reg16Idx])
            reg16Idx++
        }
    }
    
    g.Emit("CALL %s", fn.Name)
}
```

### 2. StackFrameABI (IX-based)
```go
type StackFrameABI struct {
    useIX bool  // IX or IY
}

func (abi *StackFrameABI) GeneratePrologue(g *Generator, fn *ir.Function) {
    g.Emit("PUSH IX")
    g.Emit("LD IX, SP")
    
    // Reserve space for locals
    if fn.LocalSize > 0 {
        g.Emit("LD HL, -%d", fn.LocalSize)
        g.Emit("ADD HL, SP")
        g.Emit("LD SP, HL")
    }
}

func (abi *StackFrameABI) LoadParameter(g *Generator, param *ir.Parameter, index int) {
    // Parameters are above saved IX and return address
    // [SP] = locals...
    // [IX] = saved IX  
    // [IX+2] = return address
    // [IX+4] = first parameter
    offset := 4 + param.Offset
    
    switch param.Type.Size() {
    case 1:
        g.Emit("LD A, (IX+%d)", offset)
    case 2:
        g.Emit("LD L, (IX+%d)", offset)
        g.Emit("LD H, (IX+%d)", offset+1)
    }
}
```

### 3. ShadowRegisterABI
```go
type ShadowRegisterABI struct {
    // For interrupt handlers and special functions
}

func (abi *ShadowRegisterABI) CanHandle(fn *ir.Function) bool {
    // Use for interrupt handlers or explicitly marked functions
    return fn.HasAnnotation("interrupt") || 
           fn.HasAnnotation("shadow") ||
           (fn.ParamCount() <= 3 && fn.HasAnnotation("fast"))
}

func (abi *ShadowRegisterABI) GenerateCall(g *Generator, fn *ir.Function, args []ir.Value) {
    // Move parameters to shadow registers
    g.Emit("; Setup shadow registers")
    
    // First param in shadow A
    if len(args) > 0 {
        g.LoadToA(args[0])
        g.Emit("EX AF, AF'")
    }
    
    // Rest in shadow BC, DE, HL
    if len(args) > 1 {
        g.LoadToHL(args[1])
        g.Emit("EXX")
        // Now HL is available again for more params
    }
    
    g.Emit("CALL %s", fn.Name)
}

func (abi *ShadowRegisterABI) GeneratePrologue(g *Generator, fn *ir.Function) {
    if fn.HasAnnotation("interrupt") {
        // No need to save registers - shadows are independent
        g.Emit("; Interrupt handler - using shadow registers")
    } else {
        // Switch to shadow set
        g.Emit("EX AF, AF'")
        g.Emit("EXX")
    }
}
```

### 4. HybridABI
```go
type HybridABI struct {
    maxRegParams int
}

func (abi *HybridABI) GenerateCall(g *Generator, fn *ir.Function, args []ir.Value) {
    // First N params in registers, rest on stack
    regOrder := []string{"A", "E", "HL"}
    
    // Push stack params in reverse order
    for i := len(args) - 1; i >= abi.maxRegParams; i-- {
        g.Push(args[i])
    }
    
    // Load register params
    for i := 0; i < abi.maxRegParams && i < len(args); i++ {
        if i < len(regOrder) {
            g.LoadToReg(args[i], regOrder[i])
        }
    }
    
    g.Emit("CALL %s", fn.Name)
    
    // Clean stack
    stackParams := len(args) - abi.maxRegParams
    if stackParams > 0 {
        g.Emit("LD HL, %d", stackParams*2)
        g.Emit("ADD HL, SP")
        g.Emit("LD SP, HL")
    }
}
```

## Function Analysis

### Properties to Analyze
```go
type FunctionProperties struct {
    ParamCount      int
    ParamSize       int
    LocalCount      int  
    LocalSize       int
    IsRecursive     bool
    IsInterrupt     bool
    IsLeaf          bool    // Doesn't call other functions
    RegisterPressure int
    InRAM           bool
    CallCount       int     // How often it's called
}

func AnalyzeFunction(fn *ir.Function) FunctionProperties {
    props := FunctionProperties{}
    
    // Count parameters
    for _, param := range fn.Params {
        props.ParamCount++
        props.ParamSize += param.Type.Size()
    }
    
    // Analyze body
    for _, inst := range fn.Instructions {
        switch inst.Op {
        case ir.OpCall:
            props.IsLeaf = false
            if inst.Target == fn.Name {
                props.IsRecursive = true
            }
        case ir.OpLoadLocal, ir.OpStoreLocal:
            props.LocalCount++
        }
    }
    
    // Check annotations
    props.IsInterrupt = fn.HasAnnotation("interrupt")
    props.InRAM = fn.HasAnnotation("ram") || globalInRAM
    
    return props
}
```

## ABI Selection Algorithm

```go
func SelectOptimalABI(fn *ir.Function, registry *ABIRegistry) CallingConvention {
    props := AnalyzeFunction(fn)
    
    // Priority rules
    switch {
    case props.IsInterrupt:
        return registry.Get("shadow")  // Must be fast, preserve all
        
    case props.IsRecursive:
        return registry.Get("stack")   // Needs reentrant stack
        
    case props.ParamCount == 0:
        return registry.Get("direct")  // No params, no ABI needed
        
    case props.InRAM && props.ParamCount <= 4:
        return registry.Get("smc")     // Fast SMC for RAM
        
    case props.ParamCount <= 3 && props.ParamSize <= 4:
        return registry.Get("register") // Fits in registers
        
    case props.ParamCount <= 6:
        return registry.Get("hybrid")   // Mix of registers and stack
        
    default:
        return registry.Get("stack")    // Fallback to standard
    }
}
```

## Optimization Opportunities

### 1. **Interprocedural Analysis**
```go
// Analyze call graph to optimize ABI selection
type CallGraph struct {
    nodes map[string]*Function
    edges map[string][]string  // caller -> callees
}

func OptimizeABISelection(graph *CallGraph) {
    // Functions called in hot loops should use fastest ABI
    for _, fn := range graph.nodes {
        if graph.IsHotPath(fn) {
            fn.PreferABI("register")
        }
    }
    
    // Functions only called once can use specialized ABI
    for _, fn := range graph.nodes {
        if graph.CallCount(fn) == 1 {
            fn.PreferABI("inline")  // Consider inlining
        }
    }
}
```

### 2. **Custom ABIs for Specific Patterns**
```go
// Detect common patterns and use specialized ABIs
func DetectPattern(fn *ir.Function) string {
    // Simple getter/setter
    if fn.InstructionCount() <= 3 {
        return "inline"
    }
    
    // Memory copy functions
    if fn.HasPattern("LDIR") {
        return "memcpy_abi"  // HL=src, DE=dst, BC=count
    }
    
    // Math operations
    if fn.IsPureMath() {
        return "register"  // Keep everything in registers
    }
    
    return ""
}
```

### 3. **ABI Versioning**
```go
// Support multiple ABIs for same function
type MultiABIFunction struct {
    Name     string
    Versions map[string]*CompiledFunction
    
    // Choose version based on call site
    SelectVersion(context CallContext) *CompiledFunction
}

// Generate multiple versions
func GenerateABIVersions(fn *ir.Function) {
    // Fast path version (registers)
    fastVersion := CompileWithABI(fn, "register")
    
    // Compatible version (stack)
    compatVersion := CompileWithABI(fn, "stack")
    
    // Let linker choose based on call sites
}
```

## Testing Strategy

### ABI Correctness Tests
```minz
// Test that all ABIs produce same results
@test_all_abis
fun test_add(a: u8, b: u8) -> u8 {
    return a + b;
}

// Force specific ABIs and compare
assert(test_add@register(5, 10) == 15);
assert(test_add@stack(5, 10) == 15);
assert(test_add@smc(5, 10) == 15);
```

### Performance Tests
```go
func BenchmarkABIs() {
    functions := []Function{simple, complex, recursive}
    abis := []string{"register", "stack", "smc", "hybrid"}
    
    for _, fn := range functions {
        for _, abi := range abis {
            cycles := MeasureCycles(fn, abi)
            fmt.Printf("%s with %s: %d cycles\n", fn.Name, abi, cycles)
        }
    }
}
```

## Conclusion

The adaptive ABI system will give MinZ a significant advantage:

1. **Performance**: Optimal calling convention for each function
2. **Flexibility**: Support various use cases (ROM, RAM, interrupts)
3. **Compatibility**: Can match external ABIs when needed
4. **Future-proof**: Easy to add new conventions

By treating ABI selection as an optimization problem, MinZ can generate code that's both high-level and highly efficient.