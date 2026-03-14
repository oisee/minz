# Report #069 — Arena Allocator, Sandbox Blocks, sizeof(), ConstantCallElim Fix

**Date:** 2026-03-14
**Category:** Language Features / Correctness / Testing Infrastructure
**Status:** All 26/26 Go test packages PASS · 11 arena asserts PASS (MIR2 VM)

---

## Summary

Five items landed in one session:

| Item | Impact |
|------|--------|
| **ConstantCallElim global-access guard** | Correctness fix — functions touching globals no longer folded to wrong constants |
| **`sandbox` blocks** | New syntax for grouped asserts with shared VM state |
| **`sizeof(Type)`** | Compile-time constant, zero runtime cost |
| **Struct-based arena allocator** | Full API: init/alloc/reset/remaining/split, 4 bytes of state |
| **E2E test + showcase** | 3 top-level asserts + 1 sandbox (4 asserts), all passing |

---

## 1. Bug Fix: ConstantCallElim Global-Access Guard

### The Problem

`ConstantCallElim` in `consteval.go` evaluates function calls at compile time by
spinning up a fresh MIR2 VM.  This works for pure functions but produces **wrong
results** for functions that read or write module globals — the fresh VM has all
globals zeroed, so the result depends on uninitialized state.

Concrete failure: `arena_alloc(256)` was being folded to `OpConst 0` because the
evaluation VM never ran `arena_init()`, so `self.ptr == self.end == 0` and the OOM
branch returned 0.

DSE and constprop were unaffected — only `ConstantCallElim` creates a separate VM.

### The Fix

New function `funcAccessesGlobals(f, m)` in `consteval.go` performs a **transitive**
check: if the callee, or any function it calls, contains an `OpAddrOf` instruction
referencing a module global, the call is not eligible for constant folding.

```go
func funcAccessesGlobals(f *Func, m *Module) bool {
    globalNames := make(map[string]bool, len(m.Globals))
    for _, g := range m.Globals {
        globalNames[g.Name] = true
    }
    return funcAccessesGlobalsRec(f, m, globalNames, make(map[string]bool))
}

func funcAccessesGlobalsRec(f *Func, m *Module, globalNames, visited map[string]bool) bool {
    if visited[f.Name] { return false }
    visited[f.Name] = true
    for _, b := range f.Blocks {
        for _, inst := range b.Insts {
            if inst.Op == OpAddrOf && globalNames[inst.Sym] { return true }
            if inst.Op == OpCall && inst.Sym != "" {
                callee := m.FuncByName(inst.Sym)
                if callee != nil && funcAccessesGlobalsRec(callee, m, globalNames, visited) {
                    return true
                }
            }
        }
    }
    return false
}
```

The `visited` map prevents infinite recursion on mutually-recursive functions.

---

## 2. `sandbox` Blocks — Shared VM State for Assert Groups

### Motivation

Top-level `assert` statements each get a fresh VM — fully isolated, order-independent.
This is correct for pure functions but makes it impossible to test **stateful
sequences** like "init arena, then alloc, then alloc again" where each step depends
on mutations from the previous step.

### Syntax

```nanz
sandbox "counter_chain" {
    assert set_counter(10) == 0 via mir2
    assert get_counter() == 10 via mir2    // sees mutation from previous assert
    assert inc_counter() == 11 via mir2
}
```

### Semantics

| Context | VM lifetime | Global state |
|---------|-------------|--------------|
| Top-level `assert` | Fresh VM per assert | Zeroed each time |
| `sandbox { ... }` | One VM for all asserts in the block | Persists between asserts |

### Implementation

**HIR** (`hir.go`):
```go
type Sandbox struct {
    Name    string
    Asserts []Assert
    Line    int
}
```

`Module.Sandboxes []Sandbox` added alongside `Module.Asserts`.

**Parser** (`parse.go`): `parseSandbox()` consumes `sandbox "name" { assert ...; ... }`,
delegates each inner assert to `parseAssert()`.

**MIR2 VM backend** (`pipeline.go`): `RunAssertsMIR2` creates one `mir2.NewVM(m)` per
sandbox and runs all asserts sequentially on it.

**Z80 emulator backend** (`pipeline.go`): `RunAssertsZ80` creates one
`emulator.NewRemogattoZ80()` per sandbox.  A fixed-size NOP-padded trampoline
(64 bytes) ensures the code section starts at a stable address.  Between calls,
only the trampoline is overwritten — globals in Z80 memory persist.  `z.Unhalt()`
clears the halt flag so `Run()` can be called again.

```go
const sandboxTrampolineSize = 64

func runOneAssertZ80Sandbox(z *emulator.RemogattoZ80, a hir.Assert, ..., first bool) error {
    // First call: load full binary (trampoline + code + globals)
    // Subsequent: overwrite only the trampoline region
    if first {
        z.LoadBinary(assertLoadAddr, res.Binary)
    } else {
        for i := 0; i < sandboxTrampolineSize && i < len(res.Binary); i++ {
            z.SetMemory(uint16(assertLoadAddr+i), res.Binary[i])
        }
    }
    z.Unhalt()
    z.SetPC(assertLoadAddr)
    z.Run()
    // ... check result register
}
```

---

## 3. `sizeof(Type)` — Compile-Time Constant

Resolves at parse time to an `IntLitExpr` — zero runtime cost, no IR node generated.

```nanz
sizeof(u8)     // → 1
sizeof(u16)    // → 2
sizeof(Enemy)  // → 4 (sum of field byte widths: u8+u8+u8+u8)
sizeof(Arena)  // → 4 (u16+u16)
```

Implementation in `parse.go` (~20 LOC):

```go
func (p *parser) resolveTypeSize(name string, line int) (int, error) {
    switch name {
    case "u8", "i8", "bool":  return 1, nil
    case "u16", "i16", "ptr": return 2, nil
    case "u24", "i24":        return 3, nil
    case "u32", "i32":        return 4, nil
    default:
        if st, ok := p.structs[name]; ok {
            return mir2.ByteWidth(st), nil
        }
        return 0, fmt.Errorf("line %d: sizeof: unknown type %q", line, name)
    }
}
```

In `parsePrimary()`, when the parser sees `sizeof(`:
1. Consume `sizeof`, `(`
2. Read type name identifier
3. Consume `)`
4. Call `resolveTypeSize` — return `IntLitExpr{Val: size, Ty: TyU8 or TyU16}`

Values > 255 automatically use `TyU16`.

---

## 4. Struct-Based Arena Allocator

A bump allocator in 4 bytes of state — the simplest possible dynamic memory for Z80.

### Data Layout

```nanz
struct Arena {
    ptr: u16    // next free address (offset +0)
    end: u16    // one past last valid address (offset +2)
}
```

`sizeof(Arena) == 4`.  Stored as a global or allocated from another arena.

### API

| Method | Signature | Z80 Size | T-states |
|--------|-----------|----------|----------|
| `Arena.init` | `(self: ^Arena, base: u16, size: u16)` | 18 bytes | ~80T |
| `Arena.alloc` | `(self: ^Arena, n: u16) -> u16` | 30 bytes | ~80T happy, ~40T OOM |
| `Arena.reset` | `(self: ^Arena, base: u16)` | 5 bytes | 26T |
| `Arena.remaining` | `(self: ^Arena) -> u16` | ~16 bytes | ~60T |
| `arena_split` | `(a: ^Arena, start: u16, size: u16) -> u16` | 16 bytes | ~60T |

All methods use `^Arena` pointer receiver — zero-cost UFCS, no vtable, direct CALL.

### Arena Chaining via `arena_split`

Multiple arenas can be carved from a single memory pool:

```nanz
global perm:  Arena    // permanent — never reset
global level: Arena    // reset on level change
global frame: Arena    // reset every frame

fun setup_memory() {
    let next = arena_split(&perm,  0xC000, 256)    // permanent: 256B
    let next2 = arena_split(&level, next,  2048)   // level: 2K
    let next3 = arena_split(&frame, next2, 1024)   // frame: 1K
}
```

Memory layout after setup:
```
0xC000 ┌─────────┐ perm (256B)
0xC100 ├─────────┤ level (2048B)
0xC900 ├─────────┤ frame (1024B)
0xCD00 └─────────┘ free
```

### Typed Allocation with sizeof

```nanz
struct Enemy { x: u8, y: u8, hp: u8, sprite: u8 }

let e = perm.alloc(sizeof(Enemy))    // sizeof(Enemy) == 4, resolved at parse time
// e == 0xC000 (first allocation from perm arena)
```

### Generated Z80 Assembly

**Arena.init** — 18 bytes, stores base to `ptr`, base+size to `end`:
```z80
; fun Arena_init(self: ptr = HL, base: u16 = BC, size: u16 = DE)
Arena_init:
    LD (HL), C         ; ptr.lo = base.lo
    INC HL
    LD (HL), B         ; ptr.hi = base.hi
    DEC HL
    PUSH BC
    POP HL
    ADD HL, DE         ; HL = base + size
    LD D, H
    LD E, L
    INC HL
    INC HL
    LD (HL), E         ; end.lo
    INC HL
    LD (HL), D         ; end.hi
    DEC HL
    RET
```

**Arena.alloc** — 30 bytes, bump allocator with OOM check:
```z80
; fun Arena_alloc(self: ptr = HL, n: u16 = BC) -> u16 = HL
Arena_alloc:
    LD E, (HL)         ; result = self.ptr
    INC HL
    LD D, (HL)
    DEC HL
    LD H, D
    LD L, E
    ADD HL, BC         ; next = result + n
    PUSH HL
    POP BC
    PUSH HL
    POP IX
    INC IX             ; IX = &self.end
    INC IX
    LD IXL, (IX+0)     ; load end.lo
    LD IXH, (IX+1)     ; load end.hi
    LD H, IXH
    LD L, IXL
    PUSH HL
    OR A
    SBC HL, BC         ; end - next
    POP HL
    JRS NC, .Arena_alloc_if_join2
.Arena_alloc_if_then1:
    LD A, 0            ; OOM: return 0
    LD H, A
    LD L, A
    RET
.Arena_alloc_if_join2:
    LD (HL), C         ; self.ptr = next
    INC HL
    LD (HL), B
    DEC HL
    LD H, D            ; return result
    LD L, E
    RET
```

**Arena.reset** — 5 bytes, just stores new base to `ptr`:
```z80
; fun Arena_reset(self: ptr = HL, base: u16 = DE)
Arena_reset:
    LD (HL), E
    INC HL
    LD (HL), D
    DEC HL
    RET
```

**arena_split** — inits one arena, returns next free address:
```z80
; fun arena_split(a: ptr = HL, start: u16 = IX, size: u16 = DE) -> u16 = HL
arena_split:
    LD (HL), IXL       ; a.ptr = start
    INC HL
    LD (HL), IXH
    DEC HL
    PUSH IX
    POP HL
    ADD HL, DE          ; end = start + size
    INC HL
    INC HL
    LD (HL), L          ; a.end.lo
    INC HL
    LD (HL), H          ; a.end.hi
    DEC HL
    PUSH IX
    POP HL
    ADD HL, DE          ; return start + size
    RET
```

### Global Storage

Each `Arena` global emits 4 zero bytes with EQU labels for field access:
```z80
perm:
    DB 0, 0, 0, 0
perm__ptr    EQU  perm
perm__end    EQU  perm + 2
```

---

## 5. E2E Test + Showcase

### Test File: `pkg/nanz/arena_e2e_test.go`

Three top-level asserts (fresh VM each) plus one sandbox (shared VM):

```go
assert test_sizeof() == 0 via mir2           // sizeof for primitives + structs
assert test_split_and_alloc() == 0 via mir2  // split 3 arenas, typed alloc, reset
assert test_oom() == 0 via mir2              // alloc until OOM, verify return 0

sandbox "sequential" {
    assert init_a() == 1024 via mir2         // init 1K arena, check remaining
    assert alloc_enemy() == 0xC000 via mir2  // first alloc
    assert alloc_enemy() == 0xC004 via mir2  // second alloc (ptr bumped)
    assert alloc_enemy() == 0xC008 via mir2  // third alloc (ptr bumped again)
}
```

The sandbox block proves that global state persists between asserts: each
`alloc_enemy()` returns a different address because `a.ptr` advances.

### Showcase: `showcase-src/2026-03-14/`

- `ex20_arena_allocator.nanz` — full source (105 lines)
- `ex20_arena_allocator.a80` — generated Z80 assembly (309 lines)

---

## Test Results

| Suite | Result |
|-------|--------|
| Go test packages | 26/26 PASS |
| Arena top-level asserts | 3/3 PASS (MIR2 VM) |
| Arena sandbox asserts | 4/4 PASS (MIR2 VM) |
| Total arena asserts | 7/7 PASS |

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/mir2/consteval.go` | `funcAccessesGlobals()` transitive guard |
| `pkg/hir/hir.go` | `Sandbox` struct, `Module.Sandboxes` field |
| `pkg/nanz/parse.go` | `parseSandbox()`, `resolveTypeSize()`, sizeof in `parsePrimary()` |
| `pkg/pipeline/pipeline.go` | `RunAssertsMIR2` + `RunAssertsZ80` sandbox loops, `runOneAssertZ80Sandbox` |
| `pkg/emulator/z80_remogatto.go` | `Unhalt()` method |
| `pkg/nanz/arena_e2e_test.go` | E2E test (new file) |
| `reports/showcase-src/2026-03-14/ex20_arena_allocator.nanz` | Showcase source |
| `reports/showcase-src/2026-03-14/ex20_arena_allocator.a80` | Generated Z80 |

---

## Design Notes

### Why Bump Allocation?

On Z80, `malloc`/`free` with free lists is expensive (both code size and T-states).
A bump allocator is the simplest possible scheme:

- **alloc**: one pointer comparison + one pointer advance (~80T)
- **free**: not supported per-object — reset the entire arena instead
- **fragmentation**: zero (contiguous allocation)

The multi-arena pattern (permanent / level / frame) gives lifetime control without
per-object tracking.  This maps perfectly to retro game patterns:
- **Permanent arena**: game-global data (player stats, config) — never freed
- **Level arena**: enemies, tiles, triggers — reset on level transition
- **Frame arena**: scratch buffers, temporary lists — reset every frame

### Why sandbox blocks?

Without sandboxes, testing stateful sequences requires packing all steps into a
single function that returns an error code.  Sandboxes let each step be a separate
assert with its own expected value — clearer test intent and better error messages
(the failing assert is identified by sandbox name + line number).

### sizeof as parse-time constant

Resolving `sizeof` during parsing (not during IR lowering or codegen) means it can
appear anywhere an integer literal can — array sizes, conditional compilation guards,
static assertions.  The type information needed (struct layouts) is already available
in the parser's `structs` map from preceding `struct` declarations.
