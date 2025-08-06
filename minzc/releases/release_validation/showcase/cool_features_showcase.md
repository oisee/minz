# MinZ v0.9.0 Cool Features Showcase

This document highlights the coolest features of MinZ v0.9.0 with real examples.

## 1. Revolutionary String Architecture

## test_strings_simple - Length-prefixed strings in action

### MinZ Source:
```minz
fun test_strings() -> *u8 {
    var hello: *u8 = "Hello, World!";
    return hello;
}
fun main() -> *u8 {
    return test_strings();
}
```

### Generated Z80 Assembly (key parts):
```asm
; Self-modifying code optimization:
......examples.test_strings_simple.test_strings:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; Load string "Hello, World!"
    LD HL, str_0
    ; r3 = load hello
--
......examples.test_strings_simple.main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)

; Main function:
......examples.test_strings_simple.main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r1 = call test_strings
    ; Call to test_strings (args: 0)
    ; Found function, UsesTrueSMC=false
    CALL ......examples.test_strings_simple.test_strings
    ; return r1
    POP DE
    POP BC
    RET

; Runtime print helper functions
```

## 2. Smart String Optimization

### Example: array_initializers
Shows how short strings use direct RST 16 calls:
```asm
; Using absolute addressing for locals (SMC style)
    ; Direct print "RGB: " (5 chars)
    ; Direct print "RGB: " (5 chars)
    LD A, 82
    RST 16             ; Print character
    LD A, 71
    RST 16             ; Print character
    LD A, 66
--
    CALL print_u8_decimal
```

## 3. Self-Modifying Code (SMC)

## simple_true_smc - TRUE SMC in action

### MinZ Source:
```minz
module simple_true_smc
fun add(x: u8, y: u8) -> u8 {
    return x + y
}
fun main() -> void {
    let result = add(5, 3)
}
```

### Generated Z80 Assembly (key parts):
```asm
; Self-modifying code optimization:
......examples.simple_true_smc.add:
; TRUE SMC function with immediate anchors
x$immOP:
    LD A, 0        ; x anchor (will be patched)
x$imm0 EQU x$immOP+1
--
......examples.simple_true_smc.main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC

; Main function:
......examples.simple_true_smc.main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r4 = call add
    ; Call to add (args: 2)
    ; Stack-based parameter passing
    LD HL, ($F006)    ; Virtual register 3 from memory
    PUSH HL       ; Argument 1
    LD HL, ($F004)    ; Virtual register 2 from memory
    PUSH HL       ; Argument 0
    ; Found function, UsesTrueSMC=true
    ; TRUE SMC call to ......examples.simple_true_smc.add
    LD A, ($F004)     ; Virtual register 2 from memory
```

## 4. Enhanced @print with Compile-Time Evaluation

## test_print_interpolation - Compile-time string interpolation

### MinZ Source:
```minz
fun main() -> void {
    let x: u8 = 42;
    let y: u16 = 1000;
    let flag: bool = true;
    
    // Test string interpolation with different types
    @print("The value of x is {x}");
    @print("x = {x}, y = {y}");
    @print("Flag is {flag}");
    
```

### Generated Z80 Assembly (key parts):
```asm
; Smart string optimization in action:
    ; Direct print "x = " (4 chars)
    ; Direct print "x = " (4 chars)
    LD A, 120
    RST 16             ; Print character
--
    ; Direct print ", y = " (6 chars)

; Self-modifying code optimization:
......examples.test_print_interpolation.main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r6 = string(str_0)
    LD HL, str_0
    ; Print "The value of x is " (18 chars via loop)

; Main function:
......examples.test_print_interpolation.main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r6 = string(str_0)
    LD HL, str_0
    ; Print "The value of x is " (18 chars via loop)
    CALL print_string
    ; print_u8(r7)
    LD A, B
    CALL print_u8_decimal
    ; Direct print "x = " (4 chars)
    ; Direct print "x = " (4 chars)
    LD A, 120
    RST 16             ; Print character
    LD A, 32
```

## 5. Zero-Cost @abi Integration

## simple_abi_demo - Seamless assembly integration

### MinZ Source:
```minz
@abi("smc")
fun smc_add(x: u8, y: u8) -> u8 {
    return x + y;
}
@abi("register") 
fun register_multiply(a: u8, b: u8) -> u8 {
    return a * b;
}
@abi("stack")
fun stack_process(data: u16, offset: u8) -> u16 {
```

### Generated Z80 Assembly (key parts):
```asm
; Self-modifying code optimization:
......examples.simple_abi_demo.smc_add:
; TRUE SMC function with immediate anchors
x$immOP:
    LD A, 0        ; x anchor (will be patched)
x$imm0 EQU x$immOP+1
--
......examples.simple_abi_demo.register_multiply:
; IsSMCDefault=false, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC

; Main function:
```

## 6. Lambda Expressions

## lambda_simple_test - Functional programming on Z80

### MinZ Source:
```minz
fun main() -> void {
    // Create a simple lambda
    let add_five = |x| x + 5;
    
    // Call the lambda
    let result = add_five(10);
}
```

### Generated Z80 Assembly (key parts):
```asm
; Self-modifying code optimization:
lambda_......examples.lambda_simple_test.main_0:
; IsSMCDefault=false, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; return
    RET
; Using hierarchical register allocation (physical → shadow → memory)
--
......examples.lambda_simple_test.main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)

; Main function:
......examples.lambda_simple_test.main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r2 = addr()
    LD HL, ($F000)    ; Virtual register 0 from memory
    ; r8 = call_indirect r7
    ; Indirect call through r7
    ; Register-based parameter passing for lambda
    LD HL, ($F00C)    ; Virtual register 6 from memory
    ; Parameter 0 in HL
    PUSH HL       ; Save parameter for lambda
    EX (SP), HL   ; Swap function address with parameter
    EX DE, HL     ; Parameter in DE, function in HL
    EX (SP), HL   ; Function address on stack, parameter in HL
    POP DE        ; Function address in DE
```

## Compilation Statistics

From our test suite of 148 examples:

## Summary
- Total Examples: 148
- Successful: 89
- Failed: 59
- Success Rate: 60%
