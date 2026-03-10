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
typed registers with register classes, explicit basic blocks, and a contract system
for function ABI.

Key design goals:
- Can be both **compiled** (AOT to Z80, x86, WASM, LLVM IR) and **interpreted** (MIR VM for `@` compile-time functions)
- SMC expressed as `patch_slot`/`load_patched`/`patch` — 3 primitives instead of 15 Z80-specific opcodes
- Register classes as constraints, not hints — allocator must satisfy them
- Basic blocks with explicit terminators — enables proper CFG analysis
- `@` compile-time functions compile to MIR2 and execute on the MIR VM

---

## 1. Basic Blocks

MIR1 is a flat `[]Instruction` with `OpLabel`/`OpJump` interleaved.
Liveness analysis must reconstruct CFG from a linear scan.

MIR2 uses explicit blocks. Each block has a label, a body, and a **terminator**:

```
block @entry:
    %n   = param 0 : u8
    %cmp = cmp.le %n, 1 : bool
    br_if %cmp, @base, @loop_init      ; terminator

block @base:
    ret %n : u16                        ; terminator

block @loop_init:
    %a = const 0 : u16
    %b = const 1 : u16
    %i = const 2 : u8
    jmp @loop_head                      ; terminator
```

**Terminators** — the only way to leave a block:
- `jmp @target`
- `br_if %cond, @then, @else`
- `ret %value` / `ret void`
- `unreachable`

No `OpLabel` opcode. No `OpJump` in the middle of a block.
CFG is structurally explicit — predecessors/successors derived from terminators.

Same model as: LLVM basic blocks, WASM structured control flow, Cranelift EBBs.

---

## 2. Core Opcodes (~25)

### Arithmetic & Logic

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
%r = shr  %a, %b : type      ; logical
%r = sar  %a, %b : type      ; arithmetic
%r = neg  %a : type
%r = not  %a : type
```

### Comparison — one opcode, condition as attribute

```
%r = cmp.<cond> %a, %b : bool
; <cond> = eq | ne | lt | le | gt | ge | ult | ule | ugt | uge
```

### Data Movement

```
%r = const <value> : type
%r = load  %ptr : type
     store %ptr, %val : type
%r = move  %src : type
%r = addr_of <symbol> : ptr
```

No `OpLoadVar`/`OpStoreVar`/`OpLoadField`/`OpStoreField`/`OpLoadIndex` etc.
All become `load`/`store` with computed addresses via `addr_of` + `add`.

### Calls

```
%r = call @func(%a, %b, ...) : ret_type
%r = call_indirect %ptr(%a, %b, ...) : ret_type
```

`@` prefix on `call` = compile-time execution on MIR VM (if annotated `[comptime]`).

### Frame

```
%slot = alloca <size> : ptr
%r    = phi [%a, @block1], [%b, @block2] : type   ; SSA phi
```

### Type conversions

```
%r = ext  %a : u8  -> u16   ; zero-extend
%r = sext %a : i8  -> i16   ; sign-extend
%r = trunc %a : u16 -> u8   ; truncate
```

### Control (terminators only)

```
jmp @target
br_if %cond, @then, @else
ret %val
ret void
unreachable
```

### Assembly (one opcode)

```
%r = asm "template" (%inputs...) -> (%outputs...) clobbers(classes...)
```

### SMC (3 primitives)

```
%slot = patch_slot <init> : type
%r    = load_patched %slot : type
        patch %slot, %new_val
```

**Total core: ~25 opcodes.** Everything else is intrinsics.

---

## 3. Intrinsics

Intrinsics look like `call @mir.*` and are resolved by the backend during lowering.
Unknown intrinsics → compile error or no-op (configurable per backend).

### Target-independent

```
call @mir.memcpy(%dst, %src, %len)
call @mir.memset(%dst, %val, %len)
%r = call @mir.abs(%a) : type
%r = call @mir.min(%a, %b) : type
%r = call @mir.max(%a, %b) : type
%r = call @mir.ctpop(%a) : type
%r = call @mir.clz(%a) : type
call @mir.print.u8(%val)
call @mir.print.u16(%val)
call @mir.print.i16(%val)
call @mir.print.str(%addr, %len)
call @mir.print.char(%ch)
call @mir.halt()
```

### Z80-specific

```
%r = call @mir.z80.port_in(%port) : u8
     call @mir.z80.port_out(%port, %val)
     call @mir.z80.djnz(%counter, @label)
     call @mir.z80.di()
     call @mir.z80.ei()
     call @mir.z80.im(%mode)
     call @mir.z80.exx()
```

### Compile-time host calls (for `@`-functions)

```
call [host] @mir.emit.instr(%mir_instr)    ; emit MIR2 instruction into module
call [host] @mir.emit.data(%val)           ; emit data byte into data section
call [host] @mir.emit.label(%name)         ; emit a label
call [host] @mir.error(%msg_ptr, %len)     ; compiler error
```

`[host]` = intrinsic resolved by the **host environment**:
- During compilation: host = compiler (writes into module being compiled)
- During runtime VM: error (cannot emit code at runtime)
- During C/WASM lowering: ignored or emulated

---

## 4. Platform-Independent SMC

### Three primitives

```
%slot = patch_slot <initial_value> : type
```
Declares a mutable immediate. Backend decides representation:
- Z80: byte(s) inside instruction encoding
- x86: immediate in MOV/ADD instruction
- C: `static` variable
- WASM: `global` variable

```
%r = load_patched %slot : type
```
Read current value. Z80: just executes the instruction (the immediate IS the value).
C: reads the static variable.

```
patch %slot, %new_value
```
Write new value into the mutable immediate.
Z80: writes byte into instruction stream (`LD (anchor+1), A`).
C: assigns to static variable.

### Patchable categories (via type annotation)

```
%s = patch_slot 42 : u8          ; arithmetic immediate
%s = patch_slot 42 : u16         ; 16-bit immediate / address
%s = patch_slot @label : ptr     ; jump target (relocatable)
%s = patch_slot 0xFE : port      ; I/O port number
%s = patch_slot 3 : bit_pos      ; bit position (0-7)
```

Backend uses the type to determine encoding strategy.

### Example: self-counting function

```
fun @smc_counter() -> u8 {
  block @entry:
    %count = patch_slot 0 : u8
    %val   = load_patched %count : u8
    %next  = add %val, 1 : u8
    patch %count, %next
    ret %val : u8
}
```

Z80 lowers to:
```asm
smc_counter:
    LD A, 0           ; patch_slot — byte at anchor+1 is the live value
    PUSH AF
    INC A
    LD (anchor+1), A  ; patch
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

---

## 5. Register Classes (Colored Virtual Registers)

MIR1 has `RegisterHint` — a soft preference. The allocator can ignore it.
MIR2 uses **register classes** — semantic roles that the allocator *must* satisfy.

### Classes

```
acc        ; accumulator — single-result arithmetic (Z80: A, x86: EAX)
counter    ; loop counter, decrement-and-branch (Z80: B, x86: ECX)
pointer    ; memory pointer (Z80: HL, x86: any GPR)
index      ; secondary pointer / index (Z80: DE, x86: any GPR)
pair_high  ; high byte of pair (Z80: H/D/B)
pair_low   ; low byte of pair (Z80: L/E/C)
general    ; any general-purpose register
flag       ; CPU flag result (Z80: CY/Z, x86: EFLAGS)
```

### Usage

```
%i   = const 10 : u8  [counter]    ; must go in loop counter register
%ptr = addr_of @data : ptr [pointer]
%sum = add %a, %b : u16 [acc]      ; result in accumulator
%ok  = cmp.lt %x, %y : bool [flag] ; result in CPU flag — no materialization
```

`[class]` is a **constraint**, not a hint. Conflict → spill.

### Backend mapping

| Class   | Z80     | x86-64    | ARM    | WASM    |
|---------|---------|-----------|--------|---------|
| acc     | A       | EAX/RAX   | R0     | local   |
| counter | B       | ECX/RCX   | R1     | local   |
| pointer | HL      | any GPR   | any    | local   |
| index   | DE      | any GPR   | any    | local   |
| general | A/B/C/D/E | any GPR | R0-R12 | local   |
| flag    | CY/Z    | EFLAGS    | CPSR   | i32     |

On register-rich architectures most classes collapse to `general`.
On Z80 the constraints are meaningful — `counter` MUST be B for DJNZ.

---

## 6. Function Contracts

### Declaration

```
fun @fibonacci(%n: u8 [acc]) -> u16 [pointer]
    clobbers {acc, counter, index}
{
    ...
}
```

Contract specifies:
- **Parameter locations** — each param has a register class
- **Return location** — register class for return value
- **Clobber set** — which classes this function destroys

### Multi-value return

```
fun @divmod(%a: u16, %b: u16) -> (u16 [acc], u16 [index])
```

Z80: quotient in HL, remainder in DE.
C: struct return.
WASM: native multi-value return.

### Flag return (bool via CPU flag — zero overhead)

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

Z80: `OR A` / `CP 0` — caller uses `JR Z` directly after `CALL`, no `LD` needed.
C: normal `return bool` (flags materialized to `bool`).

### Error propagation (CY flag convention)

```
fun @parse(%input: ptr [pointer]) -> (u16 [pointer], bool [flag.cy])
    clobbers {acc, pointer}
{
  block @error:
    %code = const 1 : u8 [acc]
    %err  = const true : bool [flag.cy]
    ret (%code, %err)

  block @ok:
    %val = ... : u16 [pointer]
    %ok  = const false : bool [flag.cy]
    ret (%val, %ok)
}
```

Caller:
```
%result, %err = call @parse(%ptr) : (u16, bool [flag.cy])
br_if %err, @handle_error, @continue
; Z80: CALL parse / JR C, handle_error — zero overhead
```

`[flag.cy]` and `[flag.z]` are sub-classes of `flag`.

---

## 7. Compile-Time Functions (`@` prefix)

`@` in MinZ = compile-time. In MIR2 this is a function attribute:

```
fun @square(%x: u16) [comptime] -> u16 {
  block @entry:
    %r = mul %x, %x : u16
    ret %r : u16
}
```

`[comptime]` = may execute on MIR VM during compilation.
If all arguments are known constants → fold to `const`.

### Execution modes

| Mode | Host | What happens |
|------|------|-------------|
| Compile-time, all args known | Compiler | MIR VM executes, result = `const` |
| Compile-time, args unknown | Compiler | Error: comptime requires known args |
| Runtime, comptime fn | Runtime | Compiled normally (AOT) |
| `@emit` host calls | Compiler | Writes into module being compiled |

### Gas limits (for compile-time execution)

```
; Implicit limits on comptime execution:
gas_limit:    10000   ; max instruction steps
memory_limit: 4096    ; max bytes allocated
emit_limit:   1000    ; max instructions emitted
```

### Example: compile-time LUT generation

```
fun @generate_lut(%size: u8) [comptime] -> void {
  block @entry:
    %i = const 0 : u8 [counter]
    jmp @loop

  block @loop:
    %done = cmp.ge %i, %size : bool [flag]
    br_if %done, @end, @body

  block @body:
    %val = mul %i, %i : u16
    call [host] @mir.emit.data(%val)    ; emit u16 into data section
    %i2 = add %i, 1 : u8 [counter]
    jmp @loop

  block @end:
    ret void
}
```

---

## 8. Loop Rerolling (existing MinZ optimization — fits MIR2)

MinZ already has loop rerolling: sequences of identical operations on adjacent
memory addresses get collapsed into a counted loop. In MIR2 terms:

```
; Before rerolling:
store %ptr+0, %v0
store %ptr+2, %v1
store %ptr+4, %v2
; ...repeated N times...

; After rerolling (MIR2 with counter class):
%i = const N : u8 [counter]
jmp @loop

block @loop:
    %addr = add %base, %offset : ptr [pointer]
    store %addr, %val
    %offset2 = add %offset, 2 : u16
    ; DJNZ candidate when lowered to Z80
    ; call @mir.z80.djnz(%i, @loop)  ← backend lowers counter+loop to DJNZ
```

The `[counter]` class annotation is the signal to the Z80 backend to emit DJNZ.
No special `OpDJNZ` in core — it's a backend lowering of the counter+jmp pattern.

---

## 9. Lambdas and Closures

Lambdas in MIR2 are not special. They are functions with captured environment.

```
; let f = |x: u8| => u8 { x + captured_y }

fun @lambda_0(%env: ptr [pointer], %x: u8 [acc]) -> u8 [acc] {
  block @entry:
    %y_addr = addr_of %env : ptr
    %y      = load %y_addr : u8
    %result = add %x, %y : u8
    ret %result : u8
}

; At capture site:
%env    = alloca 1 : ptr
          store %env, %current_y : u8
%fn_ptr = addr_of @lambda_0 : ptr
; closure = (%fn_ptr, %env) — two values, no special IR type
```

### Zero-cost optimizations (passes, not IR features)

1. **Inline** — when closure doesn't escape, replace `call_indirect` with body
2. **Erase environment** — if all captures are constants, no alloca needed
3. **DJNZ fusion** — `forEach(|x| ...)` → counted loop with inlined body + DJNZ

These are MIR2→MIR2 optimization passes. The IR itself has no special closure type.

---

## 10. Type System

```
u8, u16, u24, u32      ; unsigned integers
i8, i16, i24, i32      ; signed integers
bool                    ; 1-bit semantic, stored as u8
ptr                     ; pointer (size = target word size)
port                    ; I/O port number (u8 semantics, distinct type)
bit_pos                 ; bit position 0-7 (for SMC patch_slot and BIT instructions)
void                    ; no value
(T1, T2, ...)          ; tuple (for multi-return)
[T; N]                  ; fixed-size array
```

No strings, no structs in the type system — those are memory layouts accessed
through `load`/`store` + `addr_of` + computed offsets.

Every instruction is typed: `%r = add %a, %b : u16`.
Type mismatch = compile error. No implicit widening.

---

## 11. Clobber Propagation

### Per-function clobber set

Every function declares what register classes it destroys:

```
fun @foo() -> void clobbers {acc, pointer}
```

### Bottom-up synthesis

For leaf functions — compiler counts which classes are actually written.
For non-leaf functions — union of own writes + all callees' clobber sets.

### Caller-save elimination

At a call site, the caller only saves registers that are:
1. Live across the call, AND
2. In the callee's clobber set

```
%i = const 10 : u8 [counter]
; ...loop...
%r = call @leaf(%x) : u8       ; @leaf clobbers {acc} only
; %i is [counter], @leaf doesn't clobber counter → NO PUSH B / POP B
```

This is the "lazy caller-save elimination" pass, but now driven by
abstract classes rather than physical Z80 register bits.

---

## 12. MIR2 as VM

MIR2 is designed to be both compiled and interpreted.

### What makes a good IR vs a good VM

| Concern | IR-mode | VM-mode |
|---------|---------|---------|
| Representation | In-memory structs | Binary encoding |
| Addresses | Symbolic | Concrete offsets |
| I/O | Intrinsics → backend | Host function table |
| Termination | Assumed finite | Gas limit required |
| Serialization | Optional | Mandatory (.mir2 files) |

### Binary encoding

```
; Text format (for humans, --dump-mir):
%r = add %a, %b : u16

; Binary format (for VM, for .mir2 files):
[opcode:1][dest:2][src1:2][src2:2][type:1]  = 8 bytes/instr, fixed-size
```

Fixed-size encoding enables O(1) random access and simple dispatch loop.

### Host function table

The VM has a table of host functions. `call [host] @mir.*` dispatches through it.

- **Compile-time VM**: host = compiler (emits MIR, reports errors)
- **Runtime embedded VM**: host = Go callbacks
- **No host**: `call [host]` → trap / error

Same mechanism as WASM imports.

---

## 13. What Stays in the Backend (not in MIR2 core)

| Concern | MIR2 level | Backend level |
|---------|-----------|---------------|
| Register names (A, HL, EAX) | Register classes | Physical mapping |
| Instruction encoding | Abstract opcodes | Machine instructions |
| Calling convention details | Contract (classes) | ABI-specific layout |
| Stack frame layout | `alloca` + sizes | SP offsets, frame pointer |
| DJNZ loop pattern | `counter` class + loop blocks | Z80-specific lowering |
| Shadow registers (EXX) | Not visible | Z80 allocator extension |
| Condition codes | `[flag]` class | Target flag register |
| SMC byte patching | `patch_slot`/`patch` | Encoding-aware patching |
| Port I/O | `@mir.z80.port_*` intrinsic | `IN`/`OUT` instructions |

---

## 14. Comparison: MIR1 vs MIR2

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
| Error propagation | `OpSetError`/`OpCheckError` | `bool [flag.cy]` multi-return |
| Lambdas | No IR representation | Function + env pointer |
| Clobber tracking | Z80 register bits | Abstract class-based clobber sets |
| Instruction struct | 15+ fields (god struct) | Typed variants per category |
| Target-specific ops | In core enum | Intrinsic calls |
| Compile-time fns | Ad-hoc `@`-prefix | `[comptime]` attribute + MIR VM |
| LLVM emission | Impossible (Z80 baked in) | Direct 1:1 mapping to LLVM IR |
| VM execution | Partial (mirvm.VM) | Native design target |
| Binary format | None | Fixed 8-byte encoding |

---

## 15. Migration Path

### Phase 1: `pkg/mir2/` — new package

- `Block`, `Instruction`, `Function`, `Module` types
- Core opcodes (~25) as typed Go structs (not one fat enum)
- Register class system
- Intrinsic call mechanism
- Binary encoding/decoding

### Phase 2: MIR1 → MIR2 translator

- Walk MIR1 `[]Instruction`, reconstruct basic blocks at `OpLabel`/`OpJump`
- Map MIR1 opcodes → MIR2 core + intrinsics
- Map `RegisterHint` → register classes
- Map 15 SMC opcodes → `patch_slot`/`load_patched`/`patch`
- Map `OpPrintU8` etc. → `call @mir.print.u8`

### Phase 3: LLVM text backend (proof of target independence)

- `pkg/codegen/llvm2.go` consuming MIR2 (not MIR1)
- `fibonacci.minz` → MIR2 → `.ll` → `lli` → correct output
- Verifies that MIR2 has no Z80 leakage

### Phase 4: Z80 backend rewired to MIR2

- Lower intrinsics to Z80 instructions
- Map register classes to physical registers
- Existing peephole/optimizer passes adapted

### Phase 5: MIR1 retired

- Remove old `Opcode` enum and fat `Instruction` struct
- MIR2 becomes the only IR in the compiler

---

## Appendix A: Full Opcode Table

### Core (~25)

| Category | Opcodes |
|----------|---------|
| Arithmetic | `add`, `sub`, `mul`, `div`, `sdiv`, `mod`, `neg` |
| Bitwise | `and`, `or`, `xor`, `not`, `shl`, `shr`, `sar` |
| Comparison | `cmp.<cond>` (eq/ne/lt/le/gt/ge/ult/ule/ugt/uge) |
| Memory | `load`, `store`, `addr_of`, `alloca` |
| Movement | `const`, `move`, `ext`, `sext`, `trunc` |
| Terminators | `jmp`, `br_if`, `ret`, `unreachable` |
| Calls | `call`, `call_indirect` |
| SSA | `phi` |
| Assembly | `asm` |
| SMC | `patch_slot`, `load_patched`, `patch` |

### Intrinsic namespaces (extensible, not in enum)

| Prefix | Examples |
|--------|----------|
| `@mir.mem.*` | `memcpy`, `memset`, `memcmp` |
| `@mir.math.*` | `abs`, `min`, `max`, `ctpop`, `clz` |
| `@mir.io.*` | `print.u8`, `print.str`, `print.char`, `halt` |
| `@mir.z80.*` | `port_in`, `port_out`, `djnz`, `di`, `ei`, `exx` |
| `@mir.emit.*` | `instr`, `data`, `label` (host — compile-time only) |
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
    %a0 = const 0 : u16 [pointer]         ; HL on Z80
    %b0 = const 1 : u16 [index]           ; DE on Z80
    %i0 = const 2 : u8  [counter]         ; B on Z80
    jmp @loop_head

  block @loop_head:
    %a = phi [%a0, @loop_init], [%a2, @loop_body] : u16 [pointer]
    %b = phi [%b0, @loop_init], [%b2, @loop_body] : u16 [index]
    %i = phi [%i0, @loop_init], [%i2, @loop_body] : u8  [counter]
    %done = cmp.gt %i, %n : bool [flag]
    br_if %done, @exit, @loop_body

  block @loop_body:
    %temp = add %a, %b : u16              ; temp = a + b
    %a2   = move %b   : u16 [pointer]    ; a = b
    %b2   = move %temp : u16 [index]     ; b = temp
    %i2   = add %i, 1 : u8  [counter]    ; i++
    jmp @loop_head

  block @exit:
    ret %b : u16
}
```

Ideal Z80 lowering from this MIR2:
- `%a` [pointer] → HL
- `%b` [index] → DE
- `%i` [counter] → B → `DJNZ` for the loop terminator
- `ADD HL, DE` for `add %a, %b`
- `EX DE, HL` + register shuffling for the swap (`a=b, b=temp`)
- Zero spills, zero memory round-trips for any of a/b/i
