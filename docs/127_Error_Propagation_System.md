# 🚀 **MinZ Error Propagation System - Design Document**

## **Document 127: Zero-Overhead Error Propagation with `@error`**

---

## **1. Overview**

MinZ introduces revolutionary error propagation that combines modern ergonomics with zero-overhead Z80 assembly generation. The unified `@error` syntax handles both explicit error throwing and automatic error propagation with type conversion.

## **2. Core Concept**

```minz
// Explicit error throwing
@error(ErrorType.Variant);

// Error propagation (rethrow current error)  
let result = risky_operation?() ?? @error;
```

The compiler automatically detects context and generates optimal Z80 code:
- **Same error type**: Single `RET` instruction (zero overhead)
- **Different error types**: Minimal conversion logic
- **Type safety**: Compile-time validation of error type compatibility

## **3. Syntax Specification**

### **3.1 Error Declaration**
```minz
// Function that can throw specific error type
fun divide?(a: u8, b: u8) -> u8 ? MathError

// Function that can throw any error  
fun flexible?() -> u8 ? Any

// Function that can throw multiple specific types (future)
fun multi?() -> u8 ? (MathError | IOError)
```

### **3.2 Error Usage Patterns**

#### **Explicit Error Throwing:**
```minz
fun validate?(input: u8) -> u8 ? ValidationError {
    if input == 0 {
        @error(ValidationError.Zero);      // Explicit error
    }
    if input > 100 {
        @error(ValidationError.TooLarge);  // Explicit error
    }
    return input;
}
```

#### **Error Propagation:**
```minz
fun process_chain?(input: u8) -> u8 ? AppError {
    let step1 = validate?(input) ?? @error;        // ValidationError -> AppError
    let step2 = calculate?(step1) ?? @error;       // MathError -> AppError  
    let step3 = finalize?(step2) ?? @error;        // IOError -> AppError
    return step3;
}
```

#### **Mixed Usage:**
```minz
fun hybrid?(input: u8) -> u8 ? AppError {
    // Propagate with type conversion
    let validated = validate?(input) ?? @error;
    
    // Explicit error for business logic
    if validated == 42 {
        @error(AppError.AnswerToEverything);
    }
    
    // Continue propagation
    let result = compute?(validated) ?? @error;
    return result;
}
```

## **4. Type System Integration**

### **4.1 Error Type Hierarchy**
```minz
// Base error enums
enum MathError {
    DivideByZero,
    Overflow,
    Underflow,
}

enum IOError {
    FileNotFound,
    PermissionDenied,
    DiskFull,
}

// Composite application errors
enum AppError {
    Math(MathError),     // Nested error type
    IO(IOError),         // Nested error type  
    Validation,          // Simple variant
    BusinessLogic,       // Simple variant
}

// Universal error catcher
type Any = Any;  // Special type that catches all errors
```

### **4.2 Automatic Type Conversion**
```minz
fun auto_convert?(x: u8) -> u8 ? AppError {
    // Compiler automatically converts MathError -> AppError.Math
    let result = divide?(x, 2) ?? @error;
    return result;
}
```

### **4.3 Conversion Rules**
1. **Same Type**: Direct propagation (zero overhead)
2. **Compatible Types**: Automatic conversion via predefined mappings
3. **Any Type**: Universal catcher for all error types
4. **Incompatible Types**: Compile-time error with suggested mappings

## **5. Assembly Generation**

### **5.1 Zero-Overhead Propagation (Same Type)**
```minz
fun same_type?(x: u8) -> u8 ? MathError {
    let result = divide?(x, 2) ?? @error;
    return result;
}
```

**Generated Assembly:**
```asm
same_type_q:
    ; Call divide function
    call divide_q
    ret c               ; If carry set, return immediately (zero overhead!)
    
    ; Continue normal execution
    ; result is in A register
    ret
```

### **5.2 Type Conversion (Minimal Overhead)**
```minz
fun convert_type?(x: u8) -> u8 ? AppError {
    let result = divide?(x, 2) ?? @error;  // MathError -> AppError
    return result;
}
```

**Generated Assembly:**
```asm
convert_type_q:
    call divide_q
    jr nc, no_error
    
    ; Convert MathError to AppError
    cp MATH_DIVIDE_BY_ZERO
    jr z, convert_divide_zero
    cp MATH_OVERFLOW  
    jr z, convert_overflow
    ; Default conversion
    ld a, APP_UNKNOWN_MATH
    scf
    ret
    
convert_divide_zero:
    ld a, APP_MATH_ERROR
    scf
    ret
    
convert_overflow:
    ld a, APP_MATH_ERROR
    scf  
    ret
    
no_error:
    ; Continue normal execution
    ret
```

### **5.3 Explicit Error Generation**
```minz
@error(MathError.DivideByZero);
```

**Generated Assembly:**
```asm
ld a, MATH_DIVIDE_BY_ZERO   ; Load error code
scf                         ; Set carry flag
ret                         ; Return with error
```

## **6. Implementation Details**

### **6.1 Grammar Extensions**
```javascript
// Updated compile_time_error rule
compile_time_error: $ => seq(
  '@error',
  optional(seq('(', optional($.expression), ')')),
),

// This handles all cases:
// @error(value)     - explicit error with value
// @error()          - rethrow current error (empty parens)  
// @error            - rethrow current error (no parens)
```

### **6.2 AST Modifications**
```go
type CompileTimeError struct {
    ErrorValue Expression // nil for propagation, expression for explicit
    StartPos   Position
    EndPos     Position
}
```

### **6.3 Semantic Analysis**
```go
func (a *Analyzer) analyzeErrorExpr(errorExpr *ast.CompileTimeError, irFunc *ir.Function) (ir.Register, error) {
    if errorExpr.ErrorValue == nil {
        // Error propagation: @error or @error()
        return a.analyzeErrorPropagation(irFunc)
    } else {
        // Explicit error: @error(value)
        return a.analyzeExplicitError(errorExpr, irFunc)
    }
}

func (a *Analyzer) analyzeErrorPropagation(irFunc *ir.Function) (ir.Register, error) {
    // Validate we're in error propagation context (?? operator)
    if !a.inErrorPropagationContext {
        return 0, fmt.Errorf("@error without value requires error propagation context")
    }
    
    currentErrorType := a.getCurrentFunctionErrorType(irFunc)
    sourceErrorType := a.getSourceErrorTypeFromContext()
    
    if a.typesIdentical(currentErrorType, sourceErrorType) {
        // Zero-overhead same-type propagation
        a.generateZeroOverheadReturn(irFunc)
    } else if a.typesCompatible(currentErrorType, sourceErrorType) {
        // Type conversion required
        a.generateErrorTypeConversion(sourceErrorType, currentErrorType, irFunc)
    } else {
        return 0, fmt.Errorf("cannot convert %s to %s", sourceErrorType, currentErrorType)
    }
    
    return 0, nil
}
```

## **7. Error Type Conversion System**

### **7.1 Predefined Conversions**
```go
type ErrorConversionMap struct {
    From    string
    To      string  
    Mappings map[string]string
}

var builtinConversions = []ErrorConversionMap{
    {
        From: "MathError",
        To:   "AppError", 
        Mappings: map[string]string{
            "DivideByZero": "MathFailure",
            "Overflow":     "NumberTooLarge",
            "Underflow":    "NumberTooSmall",
        },
    },
    {
        From: "IOError",
        To:   "AppError",
        Mappings: map[string]string{
            "FileNotFound":     "ResourceMissing",
            "PermissionDenied": "AccessDenied", 
            "DiskFull":         "StorageFull",
        },
    },
    // Any catches everything
    {
        From: "*",  // Wildcard
        To:   "Any",
        Mappings: nil,  // Pass-through
    },
}
```

### **7.2 Custom Conversion Maps**
```minz
// Future syntax for custom conversions
@convert(MathError -> CustomError) {
    DivideByZero => CustomError.MathIssue,
    Overflow => CustomError.TooBig,
    * => CustomError.UnknownMath,  // Default case
}
```

## **8. Advanced Features**

### **8.1 Error Chaining**
```minz
fun pipeline?(input: &str) -> ProcessedData ? Any {
    return input
        .parse?() ?? @error      // ParseError -> Any
        .validate?() ?? @error   // ValidationError -> Any  
        .transform?() ?? @error  // TransformError -> Any
        .serialize?() ?? @error; // IOError -> Any
}
```

### **8.2 Conditional Propagation**
```minz
fun selective?(x: u8) -> u8 ? AppError {
    let result = risky_op?(x) ?? match {
        MathError.Overflow => handle_overflow_locally(),
        _ => @error,  // Propagate other errors
    };
}
```

### **8.3 Error Context Preservation**
```minz
// Future: Error with context
fun contextual?(filename: &str) -> Data ? IOError {
    let content = read_file?(filename) ?? @error.context("reading config file");
}
```

## **9. Performance Characteristics**

| Scenario | Z80 Instructions | T-States | Overhead |
|----------|------------------|----------|----------|
| Same type propagation | 1 (`RET`) | 10 | **Zero** |
| Simple conversion | ~8 (compare + jump) | ~35 | Minimal |
| Complex conversion | ~15 (multiple compares) | ~60 | Low |
| Explicit error | 3 (`LD`, `SCF`, `RET`) | 18 | Baseline |

## **10. Migration Path**

### **10.1 Phase 1: Basic Implementation**
- [x] Grammar support for `@error` without arguments
- [x] AST modifications for optional error value
- [x] Basic propagation analysis
- [ ] Zero-overhead same-type generation

### **10.2 Phase 2: Type Conversion**
- [ ] Error type compatibility checking
- [ ] Conversion map system
- [ ] Multi-target conversion generation
- [ ] `Any` type support

### **10.3 Phase 3: Advanced Features**
- [ ] Custom conversion syntax
- [ ] Error context chaining
- [ ] Conditional propagation patterns
- [ ] Performance optimization passes

## **11. Examples**

### **11.1 Real-World Usage**
```minz
// File processing pipeline
fun process_config?(filename: &str) -> Config ? AppError {
    let content = file.read?(filename) ?? @error;
    let json = json.parse?(content) ?? @error;  
    let config = Config.validate?(json) ?? @error;
    return config;
}

// Mathematical computation chain
fun compute_formula?(a: u8, b: u8, c: u8) -> u8 ? MathError {
    let step1 = safe_divide?(a, b) ?? @error;
    let step2 = safe_multiply?(step1, c) ?? @error;
    let step3 = safe_add?(step2, 10) ?? @error;
    return step3;
}

// Mixed error handling
fun business_logic?(input: UserInput) -> Result ? BusinessError {
    // Validate input (could throw ValidationError -> BusinessError)
    let validated = validate_input?(input) ?? @error;
    
    // Business rule check (explicit error)
    if validated.amount > MAX_TRANSACTION {
        @error(BusinessError.ExceedsLimit);
    }
    
    // Database operation (could throw DBError -> BusinessError)  
    let saved = save_to_db?(validated) ?? @error;
    
    return Result { id: saved.id, status: "processed" };
}
```

## **12. Conclusion**

The MinZ error propagation system with unified `@error` syntax provides:

- **Zero-overhead error propagation** for same-type scenarios
- **Automatic type conversion** with compile-time validation  
- **Clean, ergonomic syntax** that reads naturally
- **Z80-optimized assembly generation** leveraging CY flag and register A
- **Type safety** preventing error handling bugs
- **Composable error handling** for complex applications

This system brings modern error handling ergonomics to 8-bit development while maintaining the performance characteristics essential for resource-constrained environments.

---

**Status**: 🚧 **Design Complete** - Ready for implementation  
**Next**: Grammar updates, semantic analysis, and assembly generation  
**Impact**: Revolutionary improvement to MinZ developer experience  