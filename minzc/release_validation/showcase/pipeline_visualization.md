# MinZ Compilation Pipeline: test_strings_simple

This shows how MinZ code transforms through AST → IR → Assembly

## 1. MinZ Source
```minz
// Test string literals

fun test_strings() -> *u8 {
    var hello: *u8 = "Hello, World!";
    return hello;
}

fun main() -> *u8 {
    return test_strings();
}```

## 2. Abstract Syntax Tree (conceptual)
```
File: test_strings_simple.minz
└── Function: main
    ├── VarDecl: s1 (type: *u8)
    │   └── StringLiteral: "Hello, MinZ!"
    ├── VarDecl: s2 (type: *u8)
    │   └── StringLiteral: "Short"
    └── Return
```

## 3. Intermediate Representation
```
; Function main (from compilation log):
Function main: IsRecursive=false, Params=0, SMC=true
; IR instructions would include:
  r1 = string(str_0)  ; Load "Hello, MinZ!"
  store s1, r1        ; Store to variable
  r2 = string(str_1)  ; Load "Short"
  store s2, r2        ; Store to variable
  return              ; Exit function
```

## 4. Generated Z80 Assembly
```asm
; Data section (length-prefixed strings):
str_0:
    DB 13    ; Length
    DB "Hello, World!"

; Code section
    ORG $8000

; Code section:
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
print_string:
    LD B, (HL)         ; B = length from first byte
    INC HL             ; HL -> string data
    LD A, B            ; Check if length is zero
    OR A
```

## Key Optimizations Applied

1. **Length-prefixed strings**: No null terminators, O(1) length access
2. **Smart register allocation**: Using HL for string pointers
3. **Direct addressing**: Variables stored at fixed addresses with SMC
4. **Minimal overhead**: No unnecessary register saves/restores
