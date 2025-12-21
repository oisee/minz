# ADR Draft: Runtime Code Generation & Dynamic Function Calls

**Status**: PARTIALLY IMPLEMENTED
**Date**: 2025-12-20 (Updated: 2025-12-21)
**Author**: Claude + Human collaboration

## Implementation Status

| Feature | Status | Commit |
|---------|--------|--------|
| `@extern(addr)` | ✅ Done | Basic FFI support |
| RST optimization | ✅ Done | Auto RST for 0x00-0x38 |
| `@norst` | ✅ Done | Force CALL override |
| `@inline_params` | ✅ Done | Inline bytecode (u8, u16) |
| `@abi(reg: X)` | 🔜 Planned | Register mapping |
| `asciiz` strings | 🔜 Planned | Inline null-terminated |
| Token tables | 🔜 Planned | Calculator-style |

## Context

MinZ already embraces self-modifying code (SMC) as a first-class optimization strategy. The natural evolution is to support **runtime code generation** - where MinZ code generates machine code at runtime, then calls it.

This is particularly relevant for Z80 because:
1. Code and data share the same address space (no DEP/NX)
2. `CALL` and `JP` work with any 16-bit address
3. SMC is already culturally accepted in retro development
4. Performance-critical inner loops can be generated based on runtime parameters

## Problem Statement

How do we:
1. Tell the compiler "there will be a function at this address at runtime"
2. Specify the calling convention (registers, memory, stack)
3. Generate the call site correctly
4. Optionally patch parameter values into generated code

## Proposed Solution: Function Prototypes with ABI Annotations

### 1. Extern Function Prototypes

```minz
// Declare a function that exists at a known address
@extern(0xC000)
fun generated_multiply(a: u8, b: u8) -> u16;

// Declare a function whose address is in a variable
@extern(dynamic)
fun plugin_handler(event: u8) -> void;
```

### 2. ABI Annotations - Full Parameter Passing Spectrum

The `@abi` annotation supports ALL Z80 parameter passing mechanisms:

#### 2.1 Register-Based
```minz
// Direct register assignment
@abi(a: A, b: B, return: HL)
@extern(0xC000)
fun fast_add(a: u8, b: u8) -> u16;

// Register pairs for 16-bit
@abi(ptr: HL, value: DE, return: BC)
@extern(0xC010)
fun copy_word(ptr: *u8, value: u16) -> u16;

// Shadow registers (EXX)
@abi(a: B', b: C', return: HL')
@extern(0xC020)
fun shadow_op(a: u8, b: u8) -> u16;
```

#### 2.2 Absolute Address (Memory-Mapped Parameters)
```minz
// Parameters at fixed memory locations
// Compiler generates: LD A, value; LD (0xC001), A before CALL
@abi(factor: @0xC001, return: A)
@extern(0xC000)
fun multiply_by(x: u8, factor: u8) -> u8;

// Multiple absolute addresses
@abi(x: @0xD000, y: @0xD001, result: @0xD002)
@extern(0xC100)
fun external_calc(x: u8, y: u8) -> void;
```

#### 2.3 Relative Address (SMC-Style Immediate Patching)
```minz
// Parameter embedded at offset +1 from function start
// Function code: LD A, <patch_here>; ... ; RET
// Compiler generates: LD A, value; LD (0xC000+1), A; CALL 0xC000
@abi(factor: @+1, return: A)
@extern(0xC000)
fun smc_multiply(factor: u8) -> u8;

// Multiple relative offsets
@abi(x: @+1, y: @+4, return: HL)
@extern(0xC000)
fun smc_calc(x: u8, y: u8) -> u16;

// Relative to call site (for truly self-modifying code)
@abi(param: @call+1)  // Patch the byte after CALL instruction
fun tricky_smc(param: u8) -> void;
```

#### 2.4 Stack-Based (Traditional Calling Convention)
```minz
// Push parameters right-to-left, pop result
@abi(params: stack, return: HL)
@extern(0xC200)
fun stack_routine(a: u16, b: u16, c: u16) -> u16;

// Mixed: some in registers, rest on stack
@abi(a: A, rest: stack, return: HL)
@extern(0xC210)
fun mixed_call(a: u8, b: u16, c: u16) -> u16;
```

#### 2.5 Indexed Addressing (IX/IY Based)
```minz
// Parameters via IX-indexed addressing
// Useful for calling into existing frameworks
@abi(x: (IX+0), y: (IX+1), return: A)
@extern(0xC300)
fun indexed_func(x: u8, y: u8) -> u8;

// IY-based (common in some systems)
@abi(param: (IY+$10), return: (IY+$12))
@extern(0xC310)
fun system_call(param: u16) -> u16;
```

#### 2.6 Flag-Based Returns (for Booleans)
```minz
// Return success/failure in Carry Flag
@abi(filename: HL, return: CF)
@extern(0xC400)
fun file_exists(filename: *u8) -> bool;

// Zero flag for comparison results
@abi(a: A, b: B, return: ZF)
@extern(0xC410)
fun compare(a: u8, b: u8) -> bool;

// Multiple flags (rare but possible)
@abi(return: CF | ZF)  // CF=error, ZF=zero_result
fun complex_check() -> (bool, bool);
```

#### 2.7 Out Parameters (Multiple Return Locations)

Out parameters can be in registers, on stack, or at memory addresses:

```minz
// Register-based out parameters
@abi(a: A, result1: @out DE, result2: @out BC)
@extern(0xC500)
fun divmod(a: u8) -> void;  // Quotient in DE, remainder in BC

// Stack-based out parameters
// Caller pushes address, callee writes result there
@abi(input: HL, result: @out stack, return: void)
@extern(0xC510)
fun compute_to_stack(input: *u8) -> void;
// Caller: PUSH result_addr; LD HL, input; CALL 0xC510; POP

// Absolute address out parameters
// Result written to fixed memory location
@abi(x: A, y: B, result: @out @0xD000)
@extern(0xC520)
fun calc_to_memory(x: u8, y: u8) -> void;
// Generates: ... CALL 0xC520; LD A, (0xD000)

// Relative address out parameter (SMC-style)
@abi(input: A, result: @out @+5)  // Result at function+5
@extern(0xC530)
fun smc_compute(input: u8) -> void;

// Multiple out locations (mixed)
@abi(a: A,
     quotient: @out DE,           // In register
     remainder: @out @0xD010,     // At absolute address
     overflow: @out stack)        // On stack
@extern(0xC540)
fun full_divide(a: u8) -> void;
```

#### 2.8 Conditional Return Paths (Success/Error)

Different return locations based on success/failure status:

```minz
// Return via HL on success, A on error, CF indicates which
@abi(return: HL on_success | A on_error, error: CF)
@extern(0xC600)
fun try_parse(input: HL) -> u16 | u8;
// Usage: Check CF first, then read from appropriate register

// Error code via specific flag combinations
@abi(return: HL,
     error: CF,              // CF set = error occurred
     error_code: A)          // Error type in A when CF set
@extern(0xC610)
fun safe_divide(a: DE, b: BC) -> u16;

// Enum-based error returns
@abi(return: HL on_success,
     error: enum ParseError { InvalidChar, Overflow, Empty } in A,
     error_flag: CF)
@extern(0xC620)
fun parse_number(str: HL) -> u16;
// Compiler generates:
//   CALL 0xC620
//   JR C, .handle_error  ; CF=error
//   ; Success: result in HL
//   ...
// .handle_error:
//   ; A contains ParseError enum value

// Multiple success/error variants (pattern matching style)
@abi(return:
       HL when ZF     |      // Zero result
       DE when CF     |      // Carry result
       BC otherwise,         // Normal result
     status: A)              // Status code
@extern(0xC630)
fun complex_compute(x: A) -> u16;
```

#### 2.9 Error Propagation Integration

MinZ's `?` operator could integrate with ABI error conventions:

```minz
// Define error-returning function
@abi(return: HL, error: CF, error_code: A)
@extern(0xC700)
fun risky_operation(x: u8) -> u16 throws IoError;

// Usage with ? operator (if implemented)
fun caller() -> u16 throws IoError {
    let result = risky_operation(42)?;  // Propagates if CF set
    return result * 2;
}
// Compiler generates CF check and early return with error in A
```

#### 2.10 Immediate Embedding (Inline Constants)
```minz
// Value is literally embedded in the instruction stream
// Not patched - becomes part of the call sequence
@abi(opcode: @imm8, return: A)
@extern(0xC800)
fun execute_op(opcode: u8) -> u8;
// Generates: LD A, opcode_value; JP 0xC800 (not CALL!)
```

### ABI Annotation Summary Table

| Syntax | Meaning | Generated Code |
|--------|---------|----------------|
| `a: A` | Parameter in register A | `LD A, value` |
| `ptr: HL` | Parameter in register pair | `LD HL, value` |
| `x: B'` | Parameter in shadow register | `EXX; LD B, value; EXX` |
| `x: @0xC001` | Patch absolute address | `LD A, value; LD (0xC001), A` |
| `x: @+1` | Patch relative to function | `LD A, value; LD (func+1), A` |
| `x: @call+1` | Patch relative to call site | Inline SMC |
| `params: stack` | Push all params | `PUSH value1; PUSH value2...` |
| `x: (IX+0)` | Indexed addressing | `LD (IX+0), value` |
| `return: CF` | Return in Carry Flag | `JR C, ...` to check |
| `x: @out HL` | Out in register | Function writes to register |
| `x: @out stack` | Out on stack | Caller pushes addr, callee writes |
| `x: @out @0xD000` | Out at absolute addr | Writes to fixed memory |
| `x: @imm8` | Immediate in code stream | Literal byte in instruction |
| `return: HL on_success` | Conditional return | Check flag, read appropriate reg |
| `error: CF` | Error flag | CF set = error occurred |
| `error: enum X in A` | Typed error code | Enum value in A when CF set |
| `@rst(N)` | Use RST instead of CALL | Single-byte call to RST address |
| `inline: [u8, u16]` | Inline params after call | `RST N; DB x; DW y` |
| `inline: [asciiz]` | Inline null-term string | `RST N; DB "str", 0` |
| `inline: [tokens]` | Tokenized expression | `RST N; DB tok1, tok2...` |

#### 2.11 Clobber Lists (Register Preservation)
```minz
// Declare which registers the function destroys
@abi(a: A, return: HL, clobbers: [BC, DE])
@extern(0xC700)
fun destructive_op(a: u8) -> u16;
// Compiler wraps with PUSH BC; PUSH DE; ... ; POP DE; POP BC if needed

// Declare which registers are preserved (opposite approach)
@abi(a: A, return: HL, preserves: [IX, IY])
@extern(0xC710)
fun safe_op(a: u8) -> u16;

// All registers clobbered (common for ROM routines)
@abi(clobbers: all)
@extern(0x0010)
fun rom_print_char(char: A) -> void;
```

#### 2.12 Calling Convention Presets
```minz
// ZX Spectrum ROM convention
@abi(preset: zx_rom)  // A=param, HL=return, clobbers most
@extern(0x0010)
fun rst_10(char: u8) -> void;

// CP/M BDOS convention
@abi(preset: cpm_bdos)  // C=function, DE=param, A=return
@extern(0x0005)
fun bdos(func: u8, param: u16) -> u8;

// MSX BIOS convention
@abi(preset: msx_bios)
@extern(0x00A2)
fun chput(char: u8) -> void;

// Custom preset definition
@abi_preset("my_lib",
    params: [A, BC, DE],
    return: HL,
    clobbers: [AF],
    preserves: [IX, IY]
)
```

#### 2.13 Variadic / Array Parameters
```minz
// Pass array pointer + length
@abi(data: HL, len: BC, return: A)
@extern(0xCA00)
fun process_buffer(data: *u8, len: u16) -> u8;

// Null-terminated string
@abi(str: HL, return: BC)  // str is ASCIIZ
@extern(0xCA10)
fun strlen(str: *u8) -> u16;
```

#### 2.14 Interrupt-Safe Calling
```minz
// Function that can be called from interrupt context
@abi(interrupt_safe: true, a: A, return: A)
@extern(0xC900)
fun safe_handler(event: u8) -> u8;
// Compiler ensures EI/DI around call if in interrupt context

// Function that disables interrupts during execution
@abi(di_wrap: true)
@extern(0xC910)
fun critical_section() -> void;
// Generates: DI; CALL 0xC910; EI
```

#### 2.15 RST Optimization (Single-Byte Calls)

Z80 has 8 RST instructions (RST 0, 8, 16, 24, 32, 40, 48, 56) that are single-byte CALLs to fixed addresses. MinZ can automatically optimize:

```minz
// Explicit RST annotation
@rst(8)
fun fast_print(char: u8) -> void;

// Or auto-optimize from @extern at RST addresses
@extern(0x0010)  // Address 16 = RST 16 candidate
fun rst16_print(char: u8) -> void;

// Usage
fast_print(65);
// Generates: LD A, 65; RST 8   (2 bytes instead of 4 for CALL)
```

RST-eligible addresses: 0x0000, 0x0008, 0x0010, 0x0018, 0x0020, 0x0028, 0x0030, 0x0038

#### 2.16 Inline Bytecode Parameters (Post-Call Data)

Many Z80 systems pass parameters as inline data AFTER the call instruction. The called routine reads parameters from the return address on stack, then adjusts the return address to skip over them.

```minz
// Simple inline bytes
@rst(8)
@abi(func: A, inline: [u8, u16])
fun syscall(func_id: u8, param1: u8, addr: u16) -> void;

syscall(0x10, 5, 0xC000);
// Generates:
//   LD A, $10      ; func_id in A (register param)
//   RST 8
//   DB 5           ; param1 inline (1 byte)
//   DW $C000       ; addr inline (2 bytes)

// Null-terminated inline string
@rst(8)
@abi(inline: [asciiz])
fun print_inline(msg: str) -> void;

print_inline("Hello!");
// Generates:
//   RST 8
//   DB "Hello!", 0

// Calculator-style token stream (Scorpion ZS / Spectrum)
@rst(8)
@abi(inline: [tokens], token_table: "calc_tokens")
fun calculator(expr: str) -> void;

calculator("SIN X + COS Y");
// Generates:
//   RST 8
//   DB $1F         ; SIN token
//   DB $21         ; X token
//   DB $0F         ; + token
//   DB $22         ; COS token
//   DB $23         ; Y token
//   DB $00         ; END token
```

#### 2.17 Mixed Parameter Passing (Registers + Inline)

Complex system calls often mix register parameters with inline data:

```minz
// Scorpion ZS 256 style system call
@rst(8)
@abi(
    func: A,                    // Function ID in A register
    inline: [u8, u16, asciiz],  // Mixed inline: byte, word, string
    return: HL,
    error: CF
)
fun zs_syscall(func: u8, flags: u8, addr: u16, name: str) -> u16;

let result = zs_syscall(0x05, 0x80, 0x4000, "DATA.BIN");
// Generates:
//   LD A, $05          ; func in A
//   RST 8
//   DB $80             ; flags inline
//   DW $4000           ; addr inline
//   DB "DATA.BIN", 0   ; name inline (null-terminated)
//   ; On return: result in HL, CF set on error

// Variable-length inline list
@rst(8)
@abi(inline: [varlist(u8)])  // List terminated by 0xFF or specific marker
fun multi_param(params: [u8]) -> void;

multi_param([1, 2, 3, 4]);
// Generates:
//   RST 8
//   DB 1, 2, 3, 4, $FF  ; values + terminator
```

#### 2.18 Inline Token Tables

For calculator-style interfaces, define token mappings:

```minz
// Define token table for expression parsing
@token_table("calc_tokens")
const CALC_TOKENS = {
    "+": 0x0F, "-": 0x10, "*": 0x04, "/": 0x05,
    "SIN": 0x1F, "COS": 0x20, "TAN": 0x21,
    "X": 0x21, "Y": 0x22, "Z": 0x23,
    "END": 0x00
};

@rst(0x28)  // Spectrum calculator RST
@abi(inline: [tokens], token_table: CALC_TOKENS)
fun fp_calc(expr: str) -> void;

fp_calc("SIN X * COS Y");
// Compiler tokenizes the string at compile time!
// Generates:
//   RST $28
//   DB $1F, $21, $04, $20, $22, $00  ; SIN X * COS Y END
```

### 3. Function Pointers

```minz
// Type for function pointers
type FnPtr<(u8, u8) -> u16> = *fun;

// Store function address
let multiply_fn: FnPtr<(u8, u8) -> u16> = 0xC000;

// Call through pointer
let result = multiply_fn(5, 3);

// Or more explicit syntax
let result = @call(multiply_fn, 5, 3);
```

### 4. Runtime Code Generation Helper

```minz
// Allocate executable buffer
let code_buffer: [u8; 32] = @executable_buffer();

// Generate code at runtime
code_buffer[0] = 0x3E;  // LD A, n
code_buffer[1] = factor;
code_buffer[2] = 0x47;  // LD B, A
code_buffer[3] = 0xC9;  // RET

// Create callable from buffer
@abi(return: A)
let generated_fn = @as_function(code_buffer);

// Call it
let result = generated_fn();
```

### 5. SMC-Style Parameter Injection

```minz
// Declare function with patchable parameters
@extern(0xC000)
@patch(factor at 0xC001)  // "factor" parameter is at offset 1
fun multiply_by(x: u8, factor: u8) -> u8;

// At call site, compiler generates:
//   LD A, factor_value
//   LD (0xC001), A      ; Patch the immediate
//   LD A, x_value
//   CALL 0xC000
```

## Alternative: Prototype-Based Approach

Instead of annotations, use a more type-centric approach:

```minz
// Define a function prototype (signature only)
prototype FastMultiply = fun(a: u8, b: u8) -> u16
    @abi(a: A, b: B, return: HL);

// Bind prototype to address
let multiply: FastMultiply = @bind(0xC000);

// Or bind to dynamic address
let dynamic_fn: FastMultiply = @bind(code_buffer_addr);

// Call normally
let result = multiply(5, 3);
```

## Use Cases

### 1. Plugin System
```minz
// Plugin loader
fun load_plugin(addr: u16) -> void {
    // Read plugin header, get entry point
    let entry = @read_u16(addr + 2);

    // Create callable
    @abi(event: A, return: void)
    let handler = @bind(entry) as PluginHandler;

    // Register it
    plugins[plugin_count] = handler;
    plugin_count += 1;
}
```

### 2. JIT-Style Optimization
```minz
// Generate optimized unroller based on runtime count
fun generate_unrolled_copy(count: u8, dest: *u8) -> FnPtr {
    let code = executable_alloc(count * 4 + 1);
    let ptr = 0;

    for i in 0..count {
        code[ptr] = 0x7E;      // LD A, (HL)
        code[ptr+1] = 0x12;    // LD (DE), A
        code[ptr+2] = 0x23;    // INC HL
        code[ptr+3] = 0x13;    // INC DE
        ptr += 4;
    }
    code[ptr] = 0xC9;          // RET

    return @as_function(code);
}

// Use it
let fast_copy = generate_unrolled_copy(8);
fast_copy();  // 8 iterations unrolled!
```

### 3. Computed Dispatch Table
```minz
// Generate jump table at runtime
fun build_dispatch_table(handlers: [*fun; 8]) -> void {
    for i in 0..8 {
        let addr = handlers[i];
        dispatch_table[i*3] = 0xC3;     // JP
        dispatch_table[i*3+1] = addr & 0xFF;
        dispatch_table[i*3+2] = addr >> 8;
    }
}
```

## Implementation Considerations

### Z80 Code Generation

For `@extern` functions with `@abi(a: A, b: B, return: HL)`:

```asm
; Call site for: result = multiply(5, 3)
    LD A, 5         ; First param -> A
    LD B, 3         ; Second param -> B
    CALL 0xC000     ; Call the extern function
    ; Result now in HL
```

For dynamic function pointers:

```asm
; Call site for: result = fn_ptr(5, 3)
    LD A, 5
    LD B, 3
    LD HL, (fn_ptr_addr)  ; Load function address
    CALL call_hl          ; Helper: JP (HL)

call_hl:
    JP (HL)               ; Indirect call
```

### Safety Considerations

1. **No DEP** - Z80 has no memory protection, so this is safe
2. **Buffer Overflows** - Executable buffers could be overwritten
3. **Invalid Code** - Runtime-generated code could crash
4. **Address Conflicts** - Generated code must not overlap other code

Possible safety annotations:
```minz
@unsafe  // Mark entire block as potentially dangerous
{
    let fn = @as_function(buffer);
    fn();
}
```

## Relationship to Function Pointers

This proposal essentially introduces function pointers, which would also enable:

1. **Callbacks**
```minz
fun sort(arr: [u8], compare: fun(u8, u8) -> bool) -> void
```

2. **Higher-Order Functions**
```minz
fun map(arr: [u8], transform: fun(u8) -> u8) -> [u8]
```

3. **Event Handlers**
```minz
type EventHandler = fun(event: u8) -> void;
let on_keypress: EventHandler = handle_key;
```

## Comparison with Existing Features

| Feature | Current MinZ | With This Proposal |
|---------|-------------|-------------------|
| Function calls | Static, compile-time resolved | Dynamic, runtime-resolved |
| SMC | Compiler-generated | User-controlled generation |
| Callbacks | Via iterator lambdas only | First-class function types |
| Extern functions | Not supported | Full ABI control |
| Code generation | Compile-time only | Runtime capable |

## Proposed Syntax Summary

```minz
// 1. Extern at fixed address
@extern(0xC000)
@abi(a: A, b: B, return: HL)
fun multiply(a: u8, b: u8) -> u16;

// 2. Function pointer type
type BinaryOp = fun(u8, u8) -> u16;

// 3. Function pointer variable
let op: BinaryOp = multiply;
let result = op(5, 3);

// 4. Dynamic binding
let dynamic_fn = @bind(addr) as BinaryOp;

// 5. Runtime code generation
let code = @alloc_executable(32);
// ... generate code ...
let generated = @as_function(code) as BinaryOp;
```

## Open Questions

1. **Garbage Collection** - Who frees executable buffers?
2. **Type Safety** - How strictly do we enforce function signatures?
3. **Inlining** - Can extern functions be inlined if we know their code?
4. **Debugging** - How do we debug into generated code?
5. **Syntax** - `@bind` vs `@extern` vs `@cast`?

## Recommendation

**Phase 1**: Implement basic `@extern` with fixed addresses and `@abi` annotations
- Low complexity, high value for FFI

**Phase 2**: Add function pointer types and indirect calls
- Enables callbacks and higher-order functions

**Phase 3**: Runtime code generation helpers
- `@alloc_executable`, `@as_function`
- Most complex, but enables JIT-style optimization

## Decision

**PENDING** - Awaiting discussion and prioritization against other roadmap items.

This could be a v0.16 or v0.17 feature depending on demand and complexity assessment.

---

*This ADR represents early brainstorming. Implementation details may change significantly.*
