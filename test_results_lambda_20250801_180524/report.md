=== Comparison Report ===

## Metrics Summary

```
Configuration  TotalLines  CodeLines  Calls  SMC  IndirectCalls
no_opt              346         182   13     21   20
with_opt            288         145   13     21   20
with_smc            293         150   13     21   20
```

## Function Size Comparison

### no_opt
```asm
examples.lambda_basic_test.add_five_traditional:
; IsSMCDefault=true, IsSMCEnabled=true
```

### with_opt
```asm
examples.lambda_basic_test.add_five_traditional:
; TRUE SMC function with immediate anchors
```

### with_smc
```asm
examples.lambda_basic_test.add_five_traditional:
; TRUE SMC function with immediate anchors
```

## Lambda Implementation

### with_opt
```asm
; Using hierarchical register allocation (physical → shadow → memory)

; Function: examples.lambda_basic_test.add_five_traditional
examples.lambda_basic_test.add_five_traditional:
; TRUE SMC function with immediate anchors
x$immOP:
    LD A, 0        ; x anchor (will be patched)
x$imm0 EQU x$immOP+1
    ; Register 2 already in A
    ; return
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: lambda_examples.lambda_basic_test.test_lambda_basic_0
lambda_examples.lambda_basic_test.test_lambda_basic_0:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r0 = load x
    LD HL, ($F000)
    LD ($F000), HL    ; Virtual register 0 to memory
    ; r3 = load x
    LD HL, ($F000)
    ; r4 = 5
    LD A, 5
    LD C, A         ; Store to physical register C
--
; Using hierarchical register allocation (physical → shadow → memory)

; Function: examples.lambda_basic_test.test_lambda_basic
examples.lambda_basic_test.test_lambda_basic:
```

### with_smc
```asm
; Using hierarchical register allocation (physical → shadow → memory)

; Function: examples.lambda_basic_test.add_five_traditional
examples.lambda_basic_test.add_five_traditional:
; TRUE SMC function with immediate anchors
x$immOP:
    LD A, 0        ; x anchor (will be patched)
x$imm0 EQU x$immOP+1
    ; Register 2 already in A
    ; return
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: lambda_examples.lambda_basic_test.test_lambda_basic_0
lambda_examples.lambda_basic_test.test_lambda_basic_0:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r0 = load x
    LD HL, ($F000)
    LD ($F000), HL    ; Virtual register 0 to memory
    ; r3 = load x
    LD HL, ($F000)
    ; r4 = 5
    LD A, 5
    LD C, A         ; Store to physical register C
--
; Using hierarchical register allocation (physical → shadow → memory)

; Function: examples.lambda_basic_test.test_lambda_basic
examples.lambda_basic_test.test_lambda_basic:
```

