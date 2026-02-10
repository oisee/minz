# MIR Interpreter Design for @minz Metaprogramming

## 🚀 Revolutionary Concept: Self-Hosted Metaprogramming

The MIR interpreter enables **compile-time execution of MinZ code** through the `@minz` metafunction. This allows MinZ programs to generate MinZ code during compilation - true self-hosted metaprogramming!

## Core Architecture

### MIR Interpreter Engine
```go
// pkg/interpreter/mir_interpreter.go
type MIRInterpreter struct {
    registers  map[Register]int64        // Virtual register values
    memory     map[int64]byte            // Simulated memory space
    stack      []int64                   // Execution stack
    pc         int                       // Program counter
    flags      ProcessorFlags            // Z80-like flags
    functions  map[string]*ir.Function   // Available functions
    labels     map[string]int            // Label to instruction mapping
    
    // Metaprogramming state
    output     strings.Builder           // Generated MinZ code
    symbolGen  int                       // Symbol generator counter
}

type ProcessorFlags struct {
    Zero    bool
    Carry   bool
    Sign    bool
    Parity  bool
}
```

### @minz Metafunction Integration
```minz
// Syntax: @minz("minz_code_string", args...)
@minz("
    fun generate_accessor(field_name: *u8, field_type: *u8) -> *u8 {
        let result = '';
        result += 'fun get_' + field_name + '(self) -> ' + field_type + ' {\\n';
        result += '    return self.' + field_name + ';\\n';
        result += '}\\n';
        return result;
    }
", "position", "u16")

// Expands to generated MinZ code:
fun get_position(self) -> u16 {
    return self.position;
}
```

## Implementation Strategy

### Phase 1: Core MIR Interpreter
```go
func (interp *MIRInterpreter) Execute(function *ir.Function, args []int64) (int64, error) {
    // Set up execution context
    interp.setupFunction(function, args)
    
    // Execute instructions
    for interp.pc < len(function.Instructions) {
        inst := function.Instructions[interp.pc]
        
        switch inst.Op {
        case ir.OpLoadConst:
            interp.registers[inst.Dest] = inst.Imm
            
        case ir.OpAdd:
            val1 := interp.registers[inst.Src1]
            val2 := interp.registers[inst.Src2]
            interp.registers[inst.Dest] = val1 + val2
            interp.updateFlags(interp.registers[inst.Dest])
            
        case ir.OpCall:
            result, err := interp.callFunction(inst.Symbol, inst.Args)
            if err != nil {
                return 0, err
            }
            interp.registers[inst.Dest] = result
            
        case ir.OpReturn:
            return interp.registers[inst.Src1], nil
            
        // Add more opcodes...
        }
        
        interp.pc++
    }
    
    return 0, nil
}
```

### Phase 2: String Manipulation Functions
```go
// Built-in functions for metaprogramming
func (interp *MIRInterpreter) builtinStringConcat(args []int64) (int64, error) {
    str1 := interp.getString(args[0])
    str2 := interp.getString(args[1])
    result := str1 + str2
    return interp.storeString(result), nil
}

func (interp *MIRInterpreter) builtinStringFormat(format int64, args []int64) (int64, error) {
    formatStr := interp.getString(format)
    
    // Simple template substitution
    result := formatStr
    for i, arg := range args {
        placeholder := fmt.Sprintf("{%d}", i)
        value := fmt.Sprintf("%d", arg)
        result = strings.ReplaceAll(result, placeholder, value)
    }
    
    return interp.storeString(result), nil
}
```

### Phase 3: Code Generation Integration
```go
// Semantic analyzer integration
func (a *Analyzer) analyzeMinzMetafunction(call *ast.CallExpr) (ast.Expression, error) {
    // Extract MinZ code string
    codeStr, err := a.evaluateStringLiteral(call.Arguments[0])
    if err != nil {
        return nil, err
    }
    
    // Parse the MinZ code
    metaAST, err := a.parser.ParseString(codeStr)
    if err != nil {
        return nil, fmt.Errorf("@minz parse error: %v", err)
    }
    
    // Convert to MIR
    metaFunction, err := a.convertFunctionToMIR(metaAST)
    if err != nil {
        return nil, err
    }
    
    // Execute in MIR interpreter
    interpreter := NewMIRInterpreter()
    result, err := interpreter.Execute(metaFunction, extractArgs(call.Arguments[1:]))
    if err != nil {
        return nil, fmt.Errorf("@minz execution error: %v", err)
    }
    
    // Parse generated code back into AST
    generatedCode := interpreter.getString(result)
    return a.parser.ParseString(generatedCode)
}
```

## Supported MIR Operations

### Core Operations (Phase 1)
- ✅ **Data Movement**: OpLoadConst, OpMove, OpLoadVar, OpStoreVar
- ✅ **Arithmetic**: OpAdd, OpSub, OpMul, OpDiv, OpMod
- ✅ **Comparison**: OpCmp with flag updates
- ✅ **Control Flow**: OpJump, OpJumpIf, OpCall, OpReturn
- ✅ **Stack**: Push/pop operations for function calls

### String Operations (Phase 2)
- ✅ **String Storage**: Heap-like string management
- ✅ **Concatenation**: Built-in string_concat function
- ✅ **Formatting**: Template-based string formatting
- ✅ **Conversion**: Number to string, type to string

### Advanced Features (Phase 3)
- 🚧 **Symbol Generation**: Unique identifier creation
- 🚧 **AST Introspection**: Query AST properties during execution
- 🚧 **Type Reflection**: Runtime type information access
- 🚧 **Conditional Generation**: Generate different code based on conditions

## Example Use Cases

### 1. Getter/Setter Generation
```minz
struct Player {
    x: u16,
    y: u16,
    health: u8,
}

// Generate all accessors
@minz("
    fun generate_accessors(struct_name: *u8, fields: []*u8, types: []*u8) -> *u8 {
        let result = '';
        for i in 0..len(fields) {
            result += 'fun get_' + fields[i] + '(self: *' + struct_name + ') -> ' + types[i] + ' {\\n';
            result += '    return self.' + fields[i] + ';\\n';
            result += '}\\n\\n';
            
            result += 'fun set_' + fields[i] + '(self: *' + struct_name + ', value: ' + types[i] + ') -> void {\\n';
            result += '    self.' + fields[i] + ' = value;\\n';
            result += '}\\n\\n';
        }
        return result;
    }
", "Player", ["x", "y", "health"], ["u16", "u16", "u8"])
```

### 2. Compile-Time Computation
```minz
// Generate lookup tables at compile time
const SINE_TABLE: [u8; 256] = @minz("
    fun generate_sine_table() -> [u8; 256] {
        let table: [u8; 256];
        for i in 0..256 {
            let angle = (i * 3.14159 * 2) / 256;
            table[i] = (sin(angle) * 127 + 128) as u8;
        }
        return table;
    }
");
```

### 3. DSL Implementation
```minz
// State machine DSL
@minz("
    fun state_machine(states: []*u8, transitions: []Transition) -> *u8 {
        let code = 'enum State {\\n';
        for state in states {
            code += '    ' + state + ',\\n';
        }
        code += '}\\n\\n';
        
        code += 'fun transition(current: State, event: u8) -> State {\\n';
        code += '    match (current, event) {\\n';
        for trans in transitions {
            code += '        (' + trans.from + ', ' + trans.event + ') => ' + trans.to + ',\\n';
        }
        code += '        _ => current,\\n';
        code += '    }\\n';
        code += '}\\n';
        
        return code;
    }
", ["Idle", "Running", "Stopped"], [
    {from: "Idle", event: "1", to: "Running"},
    {from: "Running", event: "2", to: "Stopped"},
    {from: "Stopped", event: "3", to: "Idle"}
])
```

## Technical Challenges

### Memory Management
- **String Heap**: Managed string storage during interpretation
- **Garbage Collection**: Simple mark-and-sweep for unused strings
- **Memory Limits**: Bounds checking to prevent infinite loops

### Error Handling
- **Parse Errors**: Clear error messages for malformed @minz code
- **Runtime Errors**: Stack traces and debugging information
- **Type Errors**: Type checking for generated code

### Performance
- **Caching**: Cache compiled @minz functions
- **Optimization**: Basic optimization passes for interpreted code
- **Limits**: Execution time and memory limits

## Integration Points

### Parser Integration
```go
// pkg/parser/metafunction.go
func (p *Parser) parseMinzMetafunction(node *SExpNode) (ast.Expression, error) {
    if len(node.Children) < 2 {
        return nil, fmt.Errorf("@minz requires at least one argument")
    }
    
    // Extract code string
    codeNode := node.Children[1]
    if codeNode.Type != "string" {
        return nil, fmt.Errorf("@minz first argument must be string literal")
    }
    
    // Create metafunction call
    return &ast.MinzMetafunctionCall{
        Code: strings.Trim(codeNode.Value, "\""),
        Args: p.parseArguments(node.Children[2:]),
    }, nil
}
```

### Code Generation
```go
// pkg/codegen/metafunction.go
func (g *Generator) generateMinzCall(call *ast.MinzMetafunctionCall) error {
    // Execute metafunction during code generation
    interpreter := NewMIRInterpreter()
    result, err := interpreter.ExecuteMinzCode(call.Code, call.Args)
    if err != nil {
        return err
    }
    
    // Insert generated code into output
    g.output.WriteString(result)
    return nil
}
```

## Development Phases

### Phase 1: Foundation (v0.9.4)
- MIR interpreter core with basic operations
- String manipulation built-ins
- Simple @minz metafunction parsing
- Basic code generation integration

### Phase 2: Power Features (v0.9.5)
- Advanced string operations and formatting
- Control flow in interpreted code
- Error handling and debugging
- Performance optimizations

### Phase 3: Advanced Metaprogramming (v1.0.0)
- AST introspection capabilities
- Type reflection and generation
- DSL support and complex transformations
- Full integration with module system

## Success Metrics

- ✅ **Compile-time code generation** working
- ✅ **Type-safe metaprogramming** with full checking
- ✅ **Performance** - metafunctions don't slow compilation significantly
- ✅ **Usability** - clear syntax and error messages
- ✅ **Power** - can implement complex DSLs and generators

## Conclusion

The MIR interpreter for @minz metaprogramming represents a **paradigm shift** in systems programming. By enabling compile-time code generation with the full power of MinZ, we unlock:

- **Metaprogramming without macros** - type-safe, debuggable
- **DSL implementation** - domain-specific languages in MinZ
- **Code generation** - eliminate boilerplate automatically  
- **Compile-time computation** - calculate tables, optimize constants

This feature positions MinZ as a **next-generation systems language** that combines the performance of low-level programming with the productivity of high-level metaprogramming!

---

*The future of programming: where code writes code, at compile time, with full type safety!* 🚀