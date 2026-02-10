# Article 090: SMC Lambda Implementation Design

**Author:** Claude Code Assistant & User  
**Date:** August 1, 2025  
**Version:** MinZ v0.7.1+  
**Status:** IMPLEMENTATION DESIGN 🚀

## Overview

This article details the implementation of TRUE SMC-powered lambdas in MinZ, where captured variables are patched directly into the lambda's code at creation time, eliminating all closure overhead.

## Lambda Syntax

```minz
// Simple lambda
let add5 = |x| x + 5;

// Multiple parameters
let add = |x, y| x + y;

// With type annotations
let typed_add = |x: u8, y: u8| -> u16 { (x as u16) + (y as u16) };

// Capturing variables
let multiplier = 3;
let triple = |x| x * multiplier;  // 'multiplier' is captured
```

## Implementation Strategy

### 1. Grammar Extension

Add to `grammar.js`:
```javascript
lambda_expression: $ => seq(
  '|',
  optional($.lambda_parameter_list),
  '|',
  choice(
    $.expression,                    // Single expression
    seq('->', $.type, $.block),     // Typed block
    $.block,                         // Block
  ),
),

lambda_parameter_list: $ => commaSep1($.lambda_parameter),

lambda_parameter: $ => seq(
  $.identifier,
  optional(seq(':', $.type)),
),
```

### 2. AST Structures

```go
// LambdaExpr represents a lambda expression
type LambdaExpr struct {
    Params      []*LambdaParam
    ReturnType  Type           // Optional
    Body        Expression     // Can be expression or block
    Captures    []string       // Variables captured from scope
    StartPos    Position
    EndPos      Position
}

type LambdaParam struct {
    Name     string
    Type     Type  // Optional
    StartPos Position
    EndPos   Position
}
```

### 3. Semantic Analysis

During analysis, we need to:
1. Identify captured variables
2. Generate a unique lambda template function
3. Create the lambda instance with SMC patching

```go
func (a *Analyzer) analyzeLambdaExpr(lambda *ast.LambdaExpr, irFunc *ir.Function) (ir.Register, error) {
    // 1. Analyze lambda body to find captures
    captures := a.findCaptures(lambda)
    
    // 2. Generate lambda template name
    lambdaName := fmt.Sprintf("lambda_%s_%d", irFunc.Name, a.lambdaCounter)
    a.lambdaCounter++
    
    // 3. Create lambda function template
    lambdaFunc := a.createLambdaTemplate(lambdaName, lambda, captures)
    
    // 4. Emit code to create lambda instance
    instanceReg := irFunc.AllocReg()
    
    // Allocate space for lambda instance
    irFunc.Emit(ir.OpAllocLambda, instanceReg, len(lambdaFunc.Instructions))
    
    // Copy template to instance
    irFunc.Emit(ir.OpCopyLambda, instanceReg, lambdaName)
    
    // Patch captured values
    for i, capture := range captures {
        captureReg := a.resolveIdentifier(capture)
        irFunc.Emit(ir.OpPatchLambda, instanceReg, i, captureReg)
    }
    
    return instanceReg, nil
}
```

### 4. Code Generation

Lambda template generation:
```asm
; Lambda template: add5
lambda_main_0:
    ; Captured value slot
lambda_main_0_capture_0:
    LD B, 0        ; Will be patched with captured value
    
    ; Lambda body (x + captured)
    ADD A, B       ; A = parameter + captured
    RET
```

Lambda instantiation:
```asm
; Create lambda instance
    LD HL, lambda_instance_addr    ; Allocate space
    LD DE, lambda_main_0           ; Template address
    LD BC, lambda_size             ; Template size
    LDIR                           ; Copy template
    
    ; Patch captured value (e.g., 5)
    LD A, 5
    LD (lambda_instance_addr + 1), A  ; Patch the immediate
    
    ; Return lambda address in HL
    LD HL, lambda_instance_addr
```

### 5. Calling Lambdas

Lambda calls work like regular function calls:
```asm
    ; Call lambda with parameter in A
    LD A, 10          ; Parameter
    CALL (HL)         ; HL contains lambda address
    ; Result in A (15)
```

## Example: Complete Lambda Flow

```minz
fun make_adder(x: u8) -> fun(u8) -> u8 {
    return |y| x + y;  // Captures 'x'
}

fun main() -> void {
    let add5 = make_adder(5);
    let result = add5(10);  // Should be 15
}
```

Generated code:
```asm
; Lambda template for |y| x + y
lambda_make_adder_0:
lambda_make_adder_0_x:
    LD B, 0        ; Captured 'x' - will be patched
    ADD A, B       ; A = y + x
    RET

make_adder:
    ; Input: A = x
    PUSH AF        ; Save x
    
    ; Allocate lambda instance (e.g., at 0xC000)
    LD HL, lambda_make_adder_0
    LD DE, 0xC000
    LD BC, 4       ; Lambda size
    LDIR           ; Copy template
    
    ; Patch captured x
    POP AF         ; Get x back
    LD (0xC001), A ; Patch into LD B instruction
    
    ; Return lambda address
    LD HL, 0xC000
    RET

main:
    ; Call make_adder(5)
    LD A, 5
    CALL make_adder
    ; HL = lambda instance
    
    ; Store lambda address
    LD (add5_var), HL
    
    ; Call add5(10)
    LD A, 10
    LD HL, (add5_var)
    CALL (HL)
    ; A = 15
    
    RET
```

## Memory Management

For now, lambdas are allocated from a simple pool:
- Static pool at compile time
- Or runtime allocation from heap
- Each lambda instance is independent (can have different captured values)

## Advanced Features (Future)

1. **Multi-capture lambdas**: Patch multiple values
2. **Mutable captures**: Allow modifying captured variables
3. **Lambda optimization**: Inline simple lambdas
4. **TSMC references in lambdas**: Capture by reference

## Benefits

1. **Zero allocation overhead** - No heap allocation for closures
2. **Direct execution** - No indirection through closure objects
3. **Cache friendly** - Code and data in same location
4. **Predictable performance** - No GC pressure

This is the most efficient lambda implementation possible on Z80!