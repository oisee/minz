# Research: Compile-Time Logging and Debug Metafunctions for MinZ

## Executive Summary

This research proposes a comprehensive compile-time logging system for MinZ, addressing the current confusion where `@print` generates runtime code instead of compile-time output. We propose a new set of `@log.*` metafunctions for development feedback and debugging.

---

## Current State Analysis

### The @print Confusion

Currently, `@print` in MinZ:
- **Generates runtime assembly code** for printing
- **Does NOT output during compilation**
- Creates confusion for developers expecting compile-time feedback

### User Expectation vs Reality

```minz
fn main() -> u8 {
    @print("Debug: Starting compilation")  // Expected: Shows during compilation
                                           // Reality: Generates CALL print_string
    return 0;
}
```

---

## Proposed Solution: @log.* Metafunction Family

### Core Design Principle

All `@log.*` functions execute **exclusively at compile-time**, providing immediate feedback during compilation without affecting generated code.

### Proposed Metafunction Hierarchy

```
@log
├── @log.out      - Standard compile-time output
├── @log.debug    - Debug-level messages (verbose mode only)
├── @log.info     - Informational messages
├── @log.warn     - Warning messages
├── @log.error    - Error messages (non-fatal)
├── @log.trace    - Trace-level messages (extra verbose)
├── @log.table    - Formatted table output
├── @log.json     - JSON formatted output
├── @log.timing   - Performance timing markers
└── @log.assert   - Compile-time assertions with logging
```

---

## Detailed Metafunction Specifications

### 1. @log.out - Standard Output

```minz
// Basic usage
@log.out("Compiling module: ", module_name)

// With formatting
@log.out("Constant value: {}", CONST_VALUE)

// Multi-line
@log.out("""
    Configuration:
    - Target: {}
    - Optimization: {}
""", target, opt_level)
```

**Output Format:**
```
[COMPILE] Compiling module: graphics
[COMPILE] Constant value: 42
[COMPILE] Configuration:
[COMPILE]   - Target: z80
[COMPILE]   - Optimization: 2
```

### 2. @log.debug - Debug Messages

```minz
@log.debug("Function {} has {} parameters", fn_name, param_count)
@log.debug("Type inference: {} -> {}", expr, inferred_type)
```

**Output (only with --debug flag):**
```
[DEBUG] Function draw_sprite has 4 parameters
[DEBUG] Type inference: x + 5 -> u8
```

### 3. @log.info - Informational Messages

```minz
@log.info("Module loaded: {}", module_path)
@log.info("Optimization pass: {}", pass_name)
```

**Output:**
```
[INFO] Module loaded: std/io.minz
[INFO] Optimization pass: dead_code_elimination
```

### 4. @log.warn - Warning Messages

```minz
@log.warn("Deprecated function: {}", fn_name)
@log.warn("Large stack frame: {} bytes", frame_size)
```

**Output (yellow/orange in terminal):**
```
[WARN] Deprecated function: old_draw_pixel
[WARN] Large stack frame: 512 bytes
```

### 5. @log.error - Non-Fatal Errors

```minz
@log.error("Type mismatch in conditional compilation")
@log.error("Conflicting attributes: {} and {}", attr1, attr2)
```

**Output (red in terminal):**
```
[ERROR] Type mismatch in conditional compilation
[ERROR] Conflicting attributes: @inline and @noinline
```

### 6. @log.trace - Detailed Tracing

```minz
@log.trace("Entering function: {}", fn_name)
@log.trace("Register allocation: {} -> {}", var, reg)
```

**Output (only with --trace flag):**
```
[TRACE] Entering function: calculate_checksum
[TRACE] Register allocation: temp_var -> HL
```

### 7. @log.table - Formatted Tables

```minz
@log.table("Function Sizes", [
    ["Function", "Size", "Cycles"],
    ["main", "45", "120"],
    ["draw", "128", "450"],
    ["update", "67", "200"]
])
```

**Output:**
```
[TABLE] Function Sizes
┌──────────┬──────┬────────┐
│ Function │ Size │ Cycles │
├──────────┼──────┼────────┤
│ main     │ 45   │ 120    │
│ draw     │ 128  │ 450    │
│ update   │ 67   │ 200    │
└──────────┴──────┴────────┘
```

### 8. @log.json - Structured Output

```minz
@log.json({
    "phase": "optimization",
    "pass": "peephole",
    "improvements": 23,
    "size_reduction": 145
})
```

**Output:**
```json
[JSON] {"phase":"optimization","pass":"peephole","improvements":23,"size_reduction":145}
```

### 9. @log.timing - Performance Markers

```minz
@log.timing.start("type_checking")
// ... type checking code ...
@log.timing.end("type_checking")

@log.timing.lap("optimization_pass_1")
@log.timing.lap("optimization_pass_2")
```

**Output:**
```
[TIMING] Started: type_checking
[TIMING] Completed: type_checking (124ms)
[TIMING] Lap: optimization_pass_1 (45ms)
[TIMING] Lap: optimization_pass_2 (67ms)
```

### 10. @log.assert - Compile-Time Assertions

```minz
@log.assert(STACK_SIZE <= 256, "Stack size exceeds Z80 limits")
@log.assert(TABLE_SIZE == 256, "Jump table must be exactly 256 bytes")
```

**Output (on failure):**
```
[ASSERT] Assertion failed: Stack size exceeds Z80 limits
[ASSERT] Expression: STACK_SIZE <= 256
[ASSERT] Actual value: STACK_SIZE = 512
```

---

## Advanced Features

### Conditional Logging

```minz
@if(DEBUG_MODE) {
    @log.debug("Detailed state: {}", state)
}

@log.when(VERBOSE, "Extra information: {}", info)
```

### Log Levels Configuration

```bash
# Compile with different log levels
mz program.minz --log-level=debug
mz program.minz --log-level=warn  # Only warnings and errors
mz program.minz --log-level=none  # Silent compilation
```

### Log Output Redirection

```bash
# Redirect compile-time logs to file
mz program.minz --log-file=compilation.log

# Separate streams
mz program.minz --log-stdout=info --log-stderr=warn

# JSON output for tooling
mz program.minz --log-format=json > build.json
```

### Context-Aware Logging

```minz
@log.context("module", "graphics") {
    @log.info("Loading sprites")  // Output: [INFO:graphics] Loading sprites
    @log.debug("Sprite count: {}", count)
}
```

---

## Implementation Strategy

### Phase 1: Core Infrastructure

```go
// metafunction_handler.go
type LogMetafunction struct {
    Level    LogLevel
    Output   io.Writer
    Format   LogFormat
    Context  []string
}

func (l *LogMetafunction) HandleLog(level LogLevel, args []ast.Expression) {
    // Evaluate arguments at compile time
    values := l.evaluateCompileTime(args)
    
    // Format message
    message := l.formatMessage(level, values)
    
    // Output immediately during compilation
    fmt.Fprintln(l.Output, message)
    
    // Do NOT generate any runtime code
}
```

### Phase 2: Integration Points

```go
// During semantic analysis
case *ast.MetafunctionCall:
    if strings.HasPrefix(node.Name, "@log.") {
        handler.ProcessLogMetafunction(node)
        // Remove from AST - no code generation
        return nil
    }
```

### Phase 3: Compile-Time Evaluation

```go
// Compile-time expression evaluator
func evaluateAtCompileTime(expr ast.Expression) interface{} {
    switch e := expr.(type) {
    case *ast.StringLiteral:
        return e.Value
    case *ast.NumberLiteral:
        return e.Value
    case *ast.Identifier:
        // Look up compile-time constants
        return lookupConstant(e.Name)
    case *ast.BinaryExpr:
        // Evaluate if both operands are compile-time known
        left := evaluateAtCompileTime(e.Left)
        right := evaluateAtCompileTime(e.Right)
        return computeBinaryOp(e.Op, left, right)
    }
}
```

---

## Use Cases and Examples

### 1. Development Debugging

```minz
fn optimize_loop(code: []Instruction) -> []Instruction {
    @log.debug("Input instructions: {}", code.len())
    
    let optimized = apply_peephole(code)
    @log.debug("After peephole: {}", optimized.len())
    
    let final = remove_dead_code(optimized)
    @log.info("Optimization complete: {} -> {} instructions", 
              code.len(), final.len())
    
    return final
}
```

### 2. Build Configuration Verification

```minz
@log.info("Build configuration:")
@log.info("  Target: {}", TARGET)
@log.info("  Optimization: {}", OPT_LEVEL)
@log.info("  Features: {}", ENABLED_FEATURES)

@log.assert(TARGET in ["z80", "z180", "ez80"], "Invalid target platform")
```

### 3. Performance Analysis

```minz
@log.timing.start("full_compilation")

@log.timing.start("parsing")
// ... parsing phase ...
@log.timing.end("parsing")

@log.timing.start("type_checking")
// ... type checking ...
@log.timing.end("type_checking")

@log.timing.start("optimization")
@log.timing.lap("pass1")
@log.timing.lap("pass2")
@log.timing.lap("pass3")
@log.timing.end("optimization")

@log.timing.end("full_compilation")
@log.timing.summary()  // Show all timings
```

### 4. Compile-Time Computation Logging

```minz
const TABLE_SIZE = 256

@compile_time {
    let mut checksum = 0
    for i in 0..TABLE_SIZE {
        checksum ^= LOOKUP_TABLE[i]
        @log.trace("Checksum[{}] = {}", i, checksum)
    }
    @log.info("Final checksum: 0x{:02X}", checksum)
}
```

---

## Comparison with Other Languages

### Rust

```rust
// Rust uses println! during build scripts
println!("cargo:warning=Generated {} items", count);
```

### C/C++

```c
#pragma message("Compiling with optimization level " STRINGIFY(OPT_LEVEL))
#warning "This is deprecated"
```

### Zig

```zig
@compileLog("Value is: ", value);
```

### D

```d
pragma(msg, "Compile time message: ", value);
```

### MinZ (Proposed)

```minz
@log.out("Standard compile output: {}", value)
@log.warn("Warning during compilation: {}", issue)
```

**MinZ Advantages:**
- Structured hierarchy of log functions
- Consistent formatting
- Level-based filtering
- JSON/table output options
- Timing and profiling built-in

---

## Benefits

### For Developers

1. **Clear Separation**: `@print` for runtime, `@log.*` for compile-time
2. **Better Debugging**: See what happens during compilation
3. **Performance Insights**: Built-in timing and profiling
4. **Structured Output**: JSON for tooling integration

### For the Compiler

1. **No Runtime Overhead**: Log functions leave no trace in generated code
2. **Clean AST**: Log nodes removed after processing
3. **Tool Integration**: JSON output for IDEs and build tools
4. **Error Diagnosis**: Better compile-time error reporting

### For the Ecosystem

1. **Documentation**: Self-documenting compilation process
2. **Teaching**: Students can see compilation steps
3. **CI/CD**: Structured logs for build pipelines
4. **Debugging**: Reproducible compilation issues

---

## Implementation Roadmap

### Phase 1: Basic @log.out (1 week)
- Implement `@log.out` function
- Compile-time string evaluation
- Basic formatting support

### Phase 2: Log Levels (1 week)
- Add `@log.debug`, `@log.info`, `@log.warn`, `@log.error`
- Command-line level control
- Color output support

### Phase 3: Advanced Features (2 weeks)
- `@log.table` for formatted tables
- `@log.json` for structured output
- `@log.timing` for performance measurement
- `@log.assert` for compile-time assertions

### Phase 4: Integration (1 week)
- IDE protocol support
- Build tool integration
- Documentation generation
- Test framework support

---

## Migration Strategy

### Backward Compatibility

```minz
// Old code continues to work
@print("Hello")  // Still generates runtime print

// New compile-time logging
@log.out("Compiling...")  // Only during compilation
```

### Deprecation Path

1. **v0.14**: Introduce `@log.*` functions
2. **v0.15**: Add warning for `@print` in compile-time context
3. **v0.16**: Recommend `@log.*` for compile-time output
4. **v1.0**: Clear documentation on the distinction

---

## Conclusion

The proposed `@log.*` metafunction family provides a comprehensive solution for compile-time logging in MinZ. It:

1. **Eliminates confusion** between compile-time and runtime output
2. **Provides rich debugging** capabilities during compilation
3. **Enables better tooling** through structured output
4. **Maintains zero runtime cost** - no code generation
5. **Improves developer experience** with clear, immediate feedback

This system would make MinZ compilation more transparent, debuggable, and developer-friendly while maintaining the language's philosophy of zero-cost abstractions and compile-time computation.