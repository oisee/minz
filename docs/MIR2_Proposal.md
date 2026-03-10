# MIR2: Clean Target-Independent Intermediate Representation

**Status:** Proposal
**Author:** Alice Vinogradova + Claude
**Date:** 2026-03-06

---

## Motivation

Current MIR has ~80 opcodes mixing three abstraction levels in one enum:
high-level (`OpPrintU8`), mid-level (`OpAdd`), and Z80-specific (`OpDJNZ`, 15 SMC variants).
This makes multi-target compilation impossible without carrying dead Z80 baggage.

MIR2 is a clean-room redesign: **~25 core opcodes**, platform-independent SMC,
typed registers, explicit basic blocks, and a contract system for function ABI.

---

## 1. Basic Blocks (not flat labels+gotos)

MIR1 is a flat `[]Instruction` with `OpLabel`/`OpJump` interleaved.
Liveness analysis must reconstruct CFG from linear scan.

MIR2 uses explicit blocks. Each block has a label, a body, and a **terminator**:

```
block @entry:
    %n = param 0 : u8
    %cmp = lt %n, 2 : bool
    br_if %cmp, @base, @loop       ; terminator — always last

block @base:
    ret %n : u16

block @loop:
    %a = const 0 : u16
    %b = const 1 : u16
    %i = const 2 : u8
    jmp @loop_head                  ; terminator

block @loop_head:
    ; phi-like: %a, %b, %i come from @loop or @loop_body
    %done = gt %i, %n : bool
    br_if %done, @exit, @loop_body

block @loop_body:
    %temp = add %a, %b : u16
    %a2 = move %b : u16
    %b2 = move %temp : u16
    %i2 = add %i, 1 : u8
    jmp @loop_head

block @exit:
    ret %b : u16
```

**Terminators** (the only way to leave a block):
- `jmp @target`
- `br_if %cond, @then, @else`
- `ret %value` / `ret void`
- `unreachable`

No `OpLabel` opcode. No `OpJump` in the middle of a block.
CFG is **structurally explicit** — predecessors/successors derived from terminators.

**Why:** same model as LLVM basic blocks, WASM structured control flow, Cranelift EBBs.
Enables dominance analysis, loop detection, LICM, and proper liveness in one pass.

---

## 2. Core Opcodes (~25)

### 2.1 Arithmetic & Logic (10)

```
%r = add  %a, %b : type
%r = sub  %a, %b : type
%r = mul  %a, %b : type
%r = div  %a, %b : type      ; unsigned
%r = sdiv %a, %b : type      ; signed
%r = mod  %a, %b : type
%r = and  %a, %b : type      ; bitwise
%r = or   %a, %b : type
%r = xor  %a, %b : type
%r = shl  %a, %b : type
%r = shr  %a, %b : type      ; logical shift right
%r = sar  %a, %b : type      ; arithmetic shift right
%r = neg  %a : type           ; unary negate
%r = not  %a : type           ; bitwise complement
```

### 2.2 Comparison (1 opcode, condition as parameter)

```
%r = cmp.<cond> %a, %b : bool

; where <cond> = eq | ne | lt | le | gt | ge | ult | ule | ugt | uge
```

One opcode, condition encoded as attribute. LLVM does `icmp eq`, we do `cmp.eq`.

### 2.3 Data Movement (5)

```
%r = const <value> : type              ; immediate constant
%r = load %ptr : type                  ; load from memory
     store %ptr, %val : type           ; store to memory
%r = move %src : type                  ; register-to-register
%r = addr_of <symbol> : ptr            ; address of global/local/string
```

No `OpLoadVar`/`OpStoreVar`/`OpLoadParam`/`OpLoadField`/`OpStoreField`/`OpLoadIndex`
etc. — all of these become `load`/`store` with computed addresses via `addr_of` + `add`.

### 2.4 Function Calls (2)

```
%r = call @func(%a, %b, ...) : ret_type
%r = call_indirect %ptr(%a, %b, ...) : ret_type
```

### 2.5 Stack/Frame (2)

```
%slot = alloca <size> : ptr            ; allocate on stack frame
%r    = phi [%a, @block1], [%b, @block2] : type   ; SSA phi node (optional)
```

### 2.6 Inline Assembly (1)

```
%r = asm "template" (%inputs...) -> (%outputs...) clobbers(regs...)
```

One opcode for all inline asm. Template is target-specific string.
Clobber set explicitly declared.

**Total core: ~25 opcodes.** Everything else is intrinsics.

---

## 3. Intrinsics (not opcodes)

Intrinsics look like `call @mir.*` and are resolved by the backend during lowering.

### 3.1 Target-independent intrinsics

```
; Memory
call @mir.memcpy(%dst, %src, %len)
call @mir.memset(%dst, %val, %len)

; Math
%r = call @mir.abs(%a) : type
%r = call @mir.min(%a, %b) : type
%r = call @mir.max(%a, %b) : type
%r = call @mir.ctpop(%a) : type         ; population count
%r = call @mir.clz(%a) : type           ; count leading zeros

; I/O
call @mir.print.u8(%val)
call @mir.print.u16(%val)
call @mir.print.i16(%val)
call @mir.print.str(%addr, %len)
call @mir.print.char(%ch)

; Lifecycle
call @mir.halt()
```

### 3.2 Target-specific intrinsics (Z80 example)

```
; Port I/O
%r = call @mir.z80.port_in(%port) : u8
     call @mir.z80.port_out(%port, %val)

; Loop primitive
     call @mir.z80.djnz(%counter, @loop_label)

; Interrupt control
     call @mir.z80.di()
     call @mir.z80.ei()
     call @mir.z80.im(%mode)
```

Backend ignores intrinsics it doesn't understand → error or no-op.
LLVM backend skips `@mir.z80.*`, Z80 backend lowers them to native instructions.

---

## 4. Platform-Independent SMC

Three primitives. No Z80 assumptions.

### 4.1 Primitives

```
%slot = patch_slot <initial_value> : type
;   Declares a mutable immediate. Backend decides representation:
;   - Z80: byte(s) inside instruction encoding
;   - x86: immediate in MOV/ADD instruction
;   - C:   static variable
;   - WASM: global variable

%r = load_patched %slot : type
;   Read current value of the mutable immediate.
;   Z80: just executes the instruction (LD A, <imm> — the imm IS the value)
;   C:   reads the static variable
;   Semantically identical to `load` but backend knows it's a patchable site.

patch %slot, %new_value
;   Write new value into the mutable immediate.
;   Z80: writes byte into instruction stream (LD (anchor+1), A)
;   C:   assigns to static variable
;   WASM: global.set
```

### 4.2 SMC in function context

```
fun @smc_counter() -> u8 {
  block @entry:
    %count = patch_slot 0 : u8             ; starts at 0
    %val   = load_patched %count : u8      ; read current
    %next  = add %val, 1 : u8
    patch %count, %next                    ; update for next call
    ret %val : u8
}
```

Z80 lowers to:
```asm
smc_counter:
    LD A, 0           ; ← patch_slot (byte at anchor+1)
    PUSH AF
    INC A
    LD (anchor+1), A  ; ← patch
    POP AF
    RET               ; returns old value in A
```

C lowers to:
```c
uint8_t smc_counter(void) {
    static uint8_t count = 0;
    uint8_t val = count;
    count = val + 1;
    return val;
}
```

### 4.3 Patchable categories (metadata on patch_slot)

```
%s = patch_slot 42 : u8                     ; arithmetic immediate
%s = patch_slot 42 : u16                    ; 16-bit immediate / address
%s = patch_slot @label : ptr                ; jump target (relocatable)
%s = patch_slot 0xFE : port                 ; I/O port number
%s = patch_slot 3 : bit_pos                 ; bit position (0-7)
```

Backend uses the type to determine encoding strategy.

---

## 5. Register Classes (colored virtual registers)

MIR1 has `RegisterHint` — soft preference for physical register.
MIR2 replaces this with **register classes** — semantic roles that
the allocator maps to physical resources.

### 5.1 Classes

```
class acc          ; accumulator — single-result arithmetic (Z80: A, x86: EAX)
class counter      ; loop counter, decrement-and-branch (Z80: B, x86: ECX)
class pointer      ; memory pointer, base+offset (Z80: HL, x86: any GPR)
class index        ; secondary pointer / index (Z80: DE, x86: any GPR)
class pair_high    ; high byte of pair (Z80: H/D/B)
class pair_low     ; low byte of pair (Z80: L/E/C)
class general      ; any general-purpose register
class flag         ; CPU flag result (Z80: CY/Z, x86: EFLAGS, ARM: CPSR)
```

### 5.2 Usage in MIR2

```
%i   = const 10 : u8 [counter]            ; want loop counter register
%ptr = addr_of @data : ptr [pointer]       ; want pointer register
%sum = add %a, %b : u16 [acc]             ; result in accumulator
%ok  = cmp.lt %x, %y : bool [flag]        ; result in CPU flag
```

The `[class]` annotation is a **constraint**, not a hint.
Allocator must satisfy it or spill to memory.
If two live ranges with same class overlap → conflict → one must spill.

### 5.3 Backend mapping

| Class     | Z80         | x86-64     | ARM        | WASM    |
|-----------|-------------|------------|------------|---------|
| acc       | A           | EAX/RAX    | R0         | local   |
| counter   | B           | ECX/RCX    | R1         | local   |
| pointer   | HL          | any GPR    | any GPR    | local   |
| index     | DE          | any GPR    | any GPR    | local   |
| general   | A/B/C/D/E   | any GPR    | R0-R12     | local   |
| flag      | CY/Z flags  | EFLAGS     | CPSR       | i32     |

On register-rich architectures (x86, ARM) most classes collapse to `general`.
On Z80 the constraints are meaningful — counter MUST be B for DJNZ.

---

## 6. Function Contracts

Already partially in MIR1 as `FunctionContract`. MIR2 makes it first-class.

### 6.1 Contract declaration

```
fun @fibonacci(%n: u8 [acc]) -> u16 [pointer]
    clobbers {acc, counter, index}
{
    ...
}
```

Contract specifies:
- **Parameter locations** — each param annotated with register class
- **Return location** — register class for return value
- **Clobber set** — which classes this function destroys

### 6.2 Multi-value return

```
fun @divmod(%a: u16, %b: u16) -> (u16 [acc], u16 [index])
;   Returns quotient in acc-class, remainder in index-class.
;   Z80: quotient in HL, remainder in DE (or vice versa — allocator decides)
;   C: struct return or out-parameters
;   WASM: multi-value return (native support)
```

Multi-return is a tuple. Each element has its own class annotation.

### 6.3 Flag return (bool via CPU flag)

```
fun @is_empty(%ptr: ptr [pointer]) -> bool [flag]
    clobbers {acc}
{
  block @entry:
    %len = load %ptr : u8
    %r   = cmp.eq %len, 0 : bool [flag]
    ret %r : bool
}
```

Backend lowers `ret %r : bool [flag]` to just setting the flag, no materialization:
- Z80: `OR A` / `CP 0` then caller uses `JR Z, ...` directly
- x86: `TEST` then caller uses `JZ`
- C: normal `return` (flags don't exist, materialized to `bool`)

Caller side:
```
%empty = call @is_empty(%ptr) : bool [flag]
br_if %empty, @handle_empty, @continue
;   Z80: no LD, no CP — just JR Z directly after CALL
```

---

## 7. Lambdas and Closures

Lambdas in MIR2 are **not special**. They are functions with captured environment.

### 7.1 Closure representation

```
; Source: let f = |x: u8| => u8 { x + captured_y };

; MIR2: closure = function pointer + environment pointer
fun @lambda_0(%env: ptr [pointer], %x: u8 [acc]) -> u8 [acc]
{
  block @entry:
    %y_addr = add %env, 0 : ptr           ; offset 0 in env struct
    %y      = load %y_addr : u8
    %result = add %x, %y : u8
    ret %result : u8
}

; At capture site:
%env   = alloca 1 : ptr                   ; 1-byte environment
         store %env, %current_y : u8      ; capture y
%closure = struct @lambda_0, %env         ; pair (fn_ptr, env_ptr)
```

### 7.2 Zero-cost optimization (what MinZ already does)

When the closure doesn't escape and is called immediately (iterator chains),
the optimizer can:

1. **Inline** — replace `call_indirect` with the body, env loads become direct values
2. **Erase environment** — if all captures are constants, no alloca needed
3. **DJNZ fusion** — `forEach(|x| ...)` becomes a counted loop with inlined body

These are MIR2 → MIR2 optimization passes, not baked into the IR design.

### 7.3 Iterator chaining

Iterator chains are expressed as nested calls in MIR2.
No special opcodes needed:

```
; arr.iter().map(|x| x*2).filter(|x| x > 5).forEach(|x| print(x))

; Before optimization: 4 function calls per element
; After fusion pass: 1 inlined loop body with DJNZ
```

The fusion optimizer is a **pass**, not an IR feature.
MIR2 gives it clean basic blocks and register classes to work with.

---

## 8. Type System

Minimal, explicit, close to machine:

```
u8, u16, u24, u32           ; unsigned integers
i8, i16, i24, i32           ; signed integers
bool                         ; boolean (1-bit semantic, stored as u8)
ptr                          ; pointer (size = target word)
void                         ; no value
(T1, T2, ...)               ; tuple (for multi-return)
[T; N]                       ; fixed-size array
```

No strings, no structs in the type system — those are memory layouts,
accessed through `load`/`store` + `addr_of` with offsets.

Every instruction is typed: `%r = add %a, %b : u16`.
Type mismatch = compile error. No implicit widening.

Explicit casts:
```
%wide  = ext %narrow : u8 -> u16        ; zero-extend
%swide = sext %narrow : i8 -> i16       ; sign-extend
%trunc = trunc %wide : u16 -> u8        ; truncate
```

---

## 9. Clobber Propagation

### 9.1 Per-function clobber set

Every function declares what it destroys:

```
fun @foo() -> void clobbers {acc, pointer}
```

### 9.2 Bottom-up synthesis

For leaf functions — the compiler counts which classes are actually written.
For non-leaf functions — union of own writes + callees' clobber sets.

```
fun @leaf() -> u8 [acc] clobbers {acc}
;   Only writes to acc-class

fun @caller() -> void clobbers {acc, counter}
;   Writes acc itself, calls @leaf which clobbers acc,
;   uses counter for own loop
```

### 9.3 Caller-save elimination

At a call site, the caller only saves registers that:
1. Are live across the call, AND
2. Are in the callee's clobber set

```
%i = const 10 : u8 [counter]
; ... loop ...
%r = call @leaf(%x) : u8        ; @leaf clobbers {acc} only
; %i is in [counter], @leaf doesn't clobber counter → NO PUSH/POP
```

---

## 10. What Moves Down to Backend

These are NOT in MIR2 core — they live in target-specific lowering:

| Concern | MIR2 level | Backend level |
|---------|-----------|---------------|
| Register names (A, HL, EAX) | Register classes | Physical mapping |
| Instruction encoding | Abstract opcodes | Machine instructions |
| Calling convention details | Contract (classes) | ABI-specific layout |
| Stack frame layout | `alloca` + sizes | SP offsets, frame pointer |
| DJNZ loop pattern | `counter` class + loop blocks | Z80-specific lowering |
| Shadow registers (EXX) | Not visible | Z80 allocator extension |
| Condition codes | `[flag]` class | Target flag register |
| SMC byte patching | `patch_slot` / `patch` | Encoding-aware patching |

---

## 11. Migration Path from MIR1

### Phase 1: New package `pkg/mir2/`
- Define types: `Block`, `Instruction`, `Function`, `Module`
- Core opcodes only (~25)
- Intrinsic call mechanism
- Register class system

### Phase 2: MIR1 → MIR2 translator
- Walk MIR1 `[]Instruction`, reconstruct basic blocks at `OpLabel`/`OpJump`
- Map MIR1 opcodes to MIR2 core + intrinsics
- Map `RegisterHint` → register classes
- Map SMC opcodes → `patch_slot`/`load_patched`/`patch`

### Phase 3: LLVM text backend (`pkg/codegen/llvm.go`)
- Emit LLVM IR from MIR2 — proves target independence
- `fibonacci.minz` → MIR2 → `.ll` → `lli` → correct output

### Phase 4: Rewire Z80 backend to consume MIR2
- Lower intrinsics to Z80 instructions
- Map register classes to physical registers
- Existing peephole/optimizer passes adapted

### Phase 5: Remove MIR1
- Delete old `Opcode` enum, `Instruction` struct
- MIR2 becomes the only IR

---

## 12. Comparison: MIR1 vs MIR2

| Aspect | MIR1 | MIR2 |
|--------|------|------|
| Opcodes | ~80 (mixed levels) | ~25 core + intrinsics |
| Control flow | Flat labels + jumps | Explicit basic blocks |
| Typing | Optional `Type` field | Mandatory on every instruction |
| SSA | No | Optional phi nodes |
| Registers | `RegisterHint` (soft) | Register classes (constraint) |
| SMC | 15 Z80-specific opcodes | 3 target-independent primitives |
| Function ABI | String tag + optional contract | First-class contract with classes |
| Flag returns | Implicit, Z80-specific | `[flag]` class, target-independent |
| Multi-return | Not supported | Tuple return type |
| Lambdas | No IR representation | Function + env pointer (standard) |
| Clobber tracking | `RegisterSet` (Z80 bits) | Abstract class-based clobber sets |
| Instruction struct | 15+ fields (god struct) | Typed variants per opcode category |
| Target-specific ops | In core enum | Intrinsic calls |
| LLVM emission | Impossible (Z80 baked in) | Direct mapping to LLVM IR |
| Compile-time exec | Separate MIR VM, ad-hoc | `[comptime]` / `[host]` attributes |
| Error propagation | Language-level only | `(T [acc], bool [flag.cy])` tuple |
| Binary format | None (Go structs only) | Serializable for VM + IR dual use |

---

## 13. Compile-Time Execution (`@`-functions)

MinZ uses `@` to mark compile-time metafunctions (`@define`, `@if`, `@emit`, `@minz[[[...]]]`).
In MIR2 this maps cleanly to **function attributes** — no new opcodes needed.

### 13.1 Attributes

```
[comptime]  — function executes on the MIR VM during compilation, not emitted to binary
[host]      — function is provided by the compiler host (I/O boundary for compile-time code)
```

### 13.2 Mapping from MinZ syntax

| MinZ syntax | MIR2 representation |
|-------------|---------------------|
| `@minz[[[...]]]` | Anonymous `[comptime]` function, called at module init |
| `@emit("code")` | `call @host.emit(%str) [host]` — compiler inserts generated line |
| `@define("tpl", args)` | Preprocessor (before MIR2, pure text substitution) |
| `@if / @elif / @else` | Conditional compilation — resolved before MIR2 emission |
| `@print` | `call @mir.io.print.*` (runtime, NOT comptime) |

### 13.3 Comptime function example

```
; MinZ source:
; @minz[[[
;   for i in 0..8 {
;     @emit("fun handler_#{i}() -> void { ... }")
;   }
; ]]]

; MIR2:
[comptime]
fun @__comptime_init_0() -> void {
  block @entry:
    %i = const 0 : u8 [counter]
    jmp @loop

  block @loop:
    %done = cmp.ge %i, 8 : bool [flag]
    br_if %done, @exit, @body

  block @body:
    %name = call @host.format("handler_%d", %i) [host] : ptr
    %code = call @host.format("fun %s() -> void { ... }", %name) [host] : ptr
    call @host.emit(%code) [host]
    %i2 = add %i, 1 : u8 [counter]
    jmp @loop

  block @exit:
    ret void
}
```

The `[host]` calls are **not** emitted to any target — they are handled by the compiler
during MIR VM execution. The MIR VM interprets `[comptime]` functions, calling back to
the compiler for `[host]` operations (emit, read file, query type info, etc.).

### 13.4 Host function table

```
@host.emit(line: ptr)              ; inject a source line into the compilation pipeline
@host.type_of(sym: ptr) -> ptr     ; query type of a symbol at compile time
@host.size_of(type: ptr) -> u16    ; query byte size of a type
@host.exists(sym: ptr) -> bool     ; check if a symbol is defined
@host.error(msg: ptr)              ; abort compilation with error message
@host.format(fmt: ptr, ...) -> ptr ; sprintf-like formatting (comptime only)
```

The host table is **extensible** — new `@host.*` functions can be added without changing
the IR specification. They are opaque to the MIR VM; the compiler provides implementations.

---

## 14. Error Propagation (`?` → flag return)

MinZ has `?`-suffixed functions that propagate errors via the carry flag (CY) with
error code in the accumulator (A). In MIR2 this is **syntax sugar over multi-return
with `[flag.cy]`** — no dedicated error opcodes needed.

### 14.1 The key insight

```
; MinZ:
; fun safe_divide?(a: u8, b: u8) -> u8 ! MathError { ... }

; MIR2: just a multi-return function with flag class
fun @safe_divide(%a: u8 [acc], %b: u8 [counter]) -> (u8 [acc], bool [flag.cy])
    clobbers {acc, counter}
{
  block @entry:
    %zero = cmp.eq %b, 0 : bool [flag]
    br_if %zero, @error, @ok

  block @error:
    %err_code = const 1 : u8 [acc]           ; MathError.DivByZero = 1
    %err_flag = const true : bool [flag.cy]   ; set carry = error
    ret (%err_code, %err_flag) : (u8, bool)

  block @ok:
    ; ... actual division ...
    %result = div %a, %b : u8 [acc]
    %ok_flag = const false : bool [flag.cy]   ; clear carry = success
    ret (%result, %ok_flag) : (u8, bool)
}
```

### 14.2 Caller-side `?` lowering

```
; MinZ:
; let r = safe_divide?(10, 2);
; // if error, propagate upward

; MIR2:
block @call_site:
    %a = const 10 : u8 [acc]
    %b = const 2 : u8 [counter]
    (%val, %err) = call @safe_divide(%a, %b) : (u8 [acc], bool [flag.cy])
    br_if %err, @propagate, @continue

block @propagate:
    ; %val already contains error code in [acc]
    %my_err = const true : bool [flag.cy]
    ret (%val, %my_err) : (u8, bool)          ; propagate upward

block @continue:
    ; %val contains result, carry is clear
    ; ... use %val ...
```

### 14.3 Z80 lowering (zero-cost)

```asm
; call safe_divide?(10, 2)
    LD A, 10
    LD B, 2
    CALL safe_divide
    RET C              ; ← entire ? propagation: ONE instruction
    ; A = result, continue...
```

The `?` operator compiles to `RET C` (return if carry set). The error code is already
in A from the callee. **Zero overhead** on the happy path — no checking, no branching,
just the carry flag falling through.

### 14.4 MinZ error macros → MIR2

| MinZ macro | MIR2 equivalent |
|------------|-----------------|
| `@error(code)` | `const code [acc]` + `const true [flag.cy]` + `ret` |
| `@err()` | `cmp.eq %flag, true : bool [flag]` (read carry) |
| `@check_error()` | `br_if %flag, @error_handler, @continue` |
| `@get_error()` | `move %acc : u8` (error code already in accumulator) |
| `@ok()` | `const false : bool [flag.cy]` (clear carry) |

---

## 15. VM Dual-Use: MIR2 as Both IR and Bytecode

MIR2 is designed to serve as **both** a compilation IR and a VM bytecode,
similar to how WASM serves as both compiler target and execution format.

### 15.1 Why dual-use?

Three use cases need MIR execution:

1. **Compile-time metaprogramming** — `[comptime]` functions run on the MIR VM
2. **MZV tool** — standalone MIR VM runner (breakpoints, tracing, PNG export)
3. **REPL** — `mzr` needs to compile-and-execute incrementally

All three currently use ad-hoc evaluation. A proper MIR2 binary format unifies them.

### 15.2 Binary serialization format

```
MIR2 Binary = Header + TypeTable + StringTable + FunctionTable + BlockData

Header:
    magic:    [4]u8 = "MZ2\0"
    version:  u16
    flags:    u16        ; bit 0: has_comptime, bit 1: has_smc
    entry:    u32        ; index of entry function (or 0xFFFFFFFF)

TypeTable:
    count:    u16
    types:    [count]TypeDescriptor

StringTable:
    count:    u16
    strings:  [count]LenPrefixedString

FunctionTable:
    count:    u16
    funcs:    [count]FunctionDescriptor
        name:       u16 (string index)
        attributes: u16 (comptime | host | export | smc)
        contract:   ContractDescriptor
        block_count: u16
        blocks:     [block_count]BlockDescriptor
```

Each instruction is encoded as: `opcode(u8) + type(u8) + class(u8) + operands(variable)`.
Total encoding density: ~4-8 bytes per instruction (comparable to WASM at ~3-6 bytes).

### 15.3 Deterministic memory model

For VM execution, MIR2 requires a defined memory layout:

```
+------------------+
| 0x0000           | ← stack (grows down from stack_top)
|                  |
| stack_top        |
+------------------+
| heap_start       | ← alloca / dynamic allocation
|                  |
+------------------+
| globals_start    | ← global variables, string constants
|                  |
+------------------+
| code_start       | ← SMC: patch_slot targets live here
|                  |
| 0xFFFF           |
+------------------+
```

The VM provides a flat 64K address space (matching Z80). Compile-time execution
uses the same model but with configurable memory size.

### 15.4 Gas and resource limits

For compile-time execution safety (preventing infinite loops in `@minz[[[...]]]`):

```
limits:
    max_instructions:  1_000_000     ; gas limit per comptime function
    max_memory:        65_536        ; bytes (default: 64K)
    max_stack_depth:   256           ; call frames
    max_host_calls:    10_000        ; @host.* call limit
    max_emit_lines:    100_000       ; @host.emit line limit
```

Exceeding any limit = compile error with clear message ("comptime execution exceeded
instruction limit of 1,000,000 — possible infinite loop in @minz[[[...]]]").

### 15.5 Host function binding

The VM executor maintains a host function table:

```go
type HostFunc func(vm *VM, args []Value) ([]Value, error)

type VMConfig struct {
    HostFunctions map[string]HostFunc  // "@host.emit" → implementation
    Limits        ResourceLimits
    Memory        []byte               // pre-allocated address space
    Trace         bool                 // enable instruction tracing
}
```

In compiler context, the host functions interact with the compilation pipeline
(emit source, query types). In MZV standalone mode, they map to system I/O.
In REPL mode, they bridge to the interactive environment.

---

## Appendix A: Full Opcode Table

### Core (25)

| Category | Opcodes |
|----------|---------|
| Arithmetic | `add`, `sub`, `mul`, `div`, `sdiv`, `mod`, `neg` |
| Bitwise | `and`, `or`, `xor`, `not`, `shl`, `shr`, `sar` |
| Comparison | `cmp.<cond>` (eq/ne/lt/le/gt/ge/ult/ule/ugt/uge) |
| Memory | `load`, `store`, `addr_of`, `alloca` |
| Movement | `const`, `move`, `ext`, `sext`, `trunc` |
| Control | `jmp`, `br_if`, `ret`, `unreachable` |
| Calls | `call`, `call_indirect` |
| SSA | `phi` |
| Assembly | `asm` |
| SMC | `patch_slot`, `load_patched`, `patch` |

### Intrinsics (extensible, not in enum)

| Prefix | Examples |
|--------|----------|
| `@mir.mem.*` | `memcpy`, `memset`, `memcmp` |
| `@mir.math.*` | `abs`, `min`, `max`, `ctpop`, `clz` |
| `@mir.io.*` | `print.u8`, `print.str`, `print.char`, `halt` |
| `@mir.z80.*` | `port_in`, `port_out`, `djnz`, `di`, `ei`, `exx` |
| `@mir.x86.*` | (future) |
| `@mir.wasm.*` | (future) |

---

## Appendix B: Fibonacci in MIR2

```
fun @fibonacci(%n: u8 [acc]) -> u16 [pointer]
    clobbers {acc, counter, pointer, index}
{
  block @entry:
    %cmp = cmp.le %n, 1 : bool [flag]
    br_if %cmp, @base, @loop_init

  block @base:
    %n16 = ext %n : u8 -> u16
    ret %n16 : u16

  block @loop_init:
    %a0 = const 0 : u16 [pointer]         ; a in pointer-class (HL)
    %b0 = const 1 : u16 [index]           ; b in index-class (DE)
    %i0 = const 2 : u8 [counter]          ; i in counter-class (B)
    jmp @loop_head

  block @loop_head:
    %a = phi [%a0, @loop_init], [%a2, @loop_body] : u16 [pointer]
    %b = phi [%b0, @loop_init], [%b2, @loop_body] : u16 [index]
    %i = phi [%i0, @loop_init], [%i2, @loop_body] : u8 [counter]
    %done = cmp.gt %i, %n : bool [flag]
    br_if %done, @exit, @loop_body

  block @loop_body:
    %temp = add %a, %b : u16              ; temp = a + b
    %a2   = move %b : u16 [pointer]       ; a = b
    %b2   = move %temp : u16 [index]      ; b = temp
    %i2   = add %i, 1 : u8 [counter]      ; i++
    jmp @loop_head

  block @exit:
    ret %b : u16
}
```

**Ideal Z80 lowering** (what the allocator should produce):

```asm
fibonacci:
    ; %n arrives in A [acc]
    CP 2                ; cmp.le %n, 1 → carry if A < 2
    JR C, .base

    ; loop init
    LD B, A             ; n → B [counter]
    DEC B               ; loop runs n-1 times
    LD HL, 0            ; a = 0 [pointer] → HL
    LD DE, 1            ; b = 1 [index]   → DE

.loop:
    ; Fibonacci step: new_a = old_b, new_b = old_a + old_b
    ;
    ; Key insight: ADD HL,DE does NOT modify DE.
    ;   Before: HL = a, DE = b
    ;   ADD HL, DE → HL = a+b (new_b), DE = b (still old_b = new_a)
    ;   EX DE, HL  → HL = new_a, DE = new_b
    ;
    ; 2 instructions, 15 T-states per iteration. Zero stack traffic.

    ADD HL, DE          ; HL = a + b = new_b          (11T)
    EX DE, HL           ; HL = old_b = new_a          (4T)
                        ; DE = a+b   = new_b
    DJNZ .loop          ; B-- and loop                (13T / 8T last)
    ;                   ; Total: 28T per iteration

    ; Result: b is in DE, return convention wants HL [pointer]
    EX DE, HL           ; move result to HL
    RET

.base:
    LD L, A             ; return n as u16 in HL
    LD H, 0
    RET
```

**28 T-states per iteration.** No PUSH/POP, no temporary register, no memory access.
The register allocator maps `[pointer]→HL`, `[index]→DE`, `[counter]→B`, and the
`EX DE,HL` instruction naturally handles the role swap. This is what MIR2's register
classes + clobber tracking enable — the allocator knows that `ADD HL,DE` clobbers
`[pointer]` but preserves `[index]`, so the swap is safe and optimal.
