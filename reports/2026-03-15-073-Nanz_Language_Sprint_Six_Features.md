# Report #073 — Nanz Language Sprint: Six Features in One Day

**Date:** 2026-03-15
**Scope:** Enums, type aliases, modules, strings, pipe/trans, showcase examples
**Status:** All features implemented, E2E verified, 9 showcase examples (ex20–ex28)

---

## Summary

One-day sprint that closed six of the largest gaps between MinZ and Nanz. All features compile through the full HIR → MIR2 → Z80 pipeline with E2E emulator verification.

| Feature | Examples | Tests | Status |
|---------|----------|-------|--------|
| Enum declarations | ex21, ex26 | 5 asserts | ✅ |
| Type aliases | ex22 | 2 asserts | ✅ |
| Module imports | ex23, ex24, ex25 | 6 asserts | ✅ |
| Three string types | ex27 | 4 functions | ✅ |
| Pipe/trans pipelines | ex28 | 2 E2E (fold) | ✅ |
| `|>` prefix syntax | — | parser | ✅ |

---

## 1. Enum Declarations

Nanz now supports enums with auto-incrementing or explicit values:

```nanz
enum State { IDLE, RUNNING, PAUSED, GAME_OVER }
enum Color { RED = 1, GREEN = 2, BLUE = 4, WHITE = 7 }

assert get_default_state() == 3        // GAME_OVER
assert mix_colors() == 5               // RED + BLUE
assert identity(State.GAME_OVER) == State.GAME_OVER
```

Access uses dot syntax (`State.IDLE`), not `::`. Enum values resolve to integer constants at compile time — zero runtime cost.

**Z80 output:** Enum values inline as immediate operands (`LD A, 3`). No tables, no indirection.

## 2. Type Aliases

Structural aliases give semantic names to primitive types:

```nanz
type PlayerID = u8
type Score = u16
type Coord = u8

fun damage(target: PlayerID, amount: u8) -> u8 {
    return amount
}
```

Currently structural (transparent to the type system). Newtype aliases (opaque, preventing accidental mixing) are designed but deferred.

## 3. Module System

Four import styles, all working end-to-end:

```nanz
// Unqualified — import specific symbols
import mathlib.ops { add, double }
add(double(5), 1)    // → 11

// Qualified — prefix access
import mathlib.ops
ops.add(5, 3)        // → 8

// Alias
import mathlib.ops as m
m.add(5, 3)

// Glob
import mathlib.ops { * }
add(5, 3)            // all symbols imported
```

**Implementation:** HIR module merging — imported functions are spliced into the caller's compilation unit. In Z80 asm output, module separator is `$` (avoids MZA `.local` label conflict). In Nanz source, separator is `.` (natural dot syntax).

## 4. Three String Types

All three string representations now compile to Z80:

| Type | Syntax | Encoding | Use case |
|------|--------|----------|----------|
| SString | `"hello"` | u8-prefix length + data | Default, fast length access |
| LString | `l"hello"` | u16-prefix length + data | Strings > 255 bytes |
| CString | `c"hello"` | NUL-terminated | FFI, ROM compatibility |

Triple-quote variants supported: `c"""multi\nline"""`.

**Z80 output** (ex27_strings.a80):
```z80
greet:
    LD HL, _mir2_str_0       ; pointer to string data
.print_str_0:
    LD A, (HL)               ; load byte
    AND A                    ; check NUL
    JR Z, .print_str_done_0
    OUT (0x23), A            ; emit to stdout
    INC HL
    JR .print_str_0
.print_str_done_0:
    RET

_mir2_str_0:
    DB 72, 101, 108, 108, 111, 44, 32, 87, 111, 114, 108, 100, 33, 0
    ; "Hello, World!\0"
```

Ruby-style interpolation works with compile-time constant folding:
```nanz
@print("Sum: #{2 + 3}")     // → "Sum: 5" at compile time
@print("Value: #{x}")       // → runtime: compute x, print
```

StringPool in MIR2 deduplicates identical strings across functions.

## 5. Pipe/Trans: Named Reusable Iterator Pipelines

The headline feature — named, composable, zero-cost iterator pipelines:

```nanz
pipe doubled { map(|x: u8| x + x) }
trans composed { use doubled; map(|x: u8| x + 1) }

fun sum_doubled() -> u8 {
    return range(0..5).apply(doubled).fold(0, add_acc)
}
// range counts 5,4,3,2,1 → ×2 → 10,8,6,4,2 → sum = 30
```

- `pipe` and `trans` are synonyms (like `fn`/`fun`)
- `use` composes pipelines — snapshot semantics at definition time
- `.apply(pipe)` connects a pipe to a source
- Fusion optimizer inlines stages into DJNZ loop body

**Z80 output** (ex28_pipe.a80) — stages fully fused:
```z80
sum_doubled:
    LD A, 0              ; accumulator = 0
    LD C, 5              ; range count
    ...
.sum_doubled_rng_body1:
    LD E, A              ; save acc
    LD A, B              ; load element
    ADD A, B             ; doubled (x + x)
    ADD A, E             ; fold (acc + x)
    DJNZ .sum_doubled_rng_body1
```

Lambda functions are generated but never called — the fusion optimizer inlines them directly. Zero CALL/RET overhead.

## 6. `|>` Prefix Syntax

Optional `|>` prefix inside pipe declarations for visual clarity:

```nanz
pipe pipeline {
    |> map(|x: u8| x + x)
    |> filter(|x: u8| x > 3)
}
```

Semantically identical to the non-prefixed form. Parser strips `|>` tokens.

---

## Architecture Decisions

Three new ADRs support these features:

- **[ADR-0020](docs/adr/0020-instruction-level-constraints.md)** — Instruction-level constraints (EdgeCost for PBQP)
- **[ADR-0021](docs/adr/0021-banked-memory-and-u24-pointers.md)** — Banked memory model & u24 virtual pointers
- **[ADR-0022](docs/adr/0022-enum-pipe-type-alias.md)** — Enum, pipe/trans, and type alias design

## Bug Discovery

The arena allocator showcase (ex20) revealed **BUG-008**: five compounding Z80 codegen bugs when IX-family registers are used as both pointer base and data destination. Full RCA in [Report #071](2026-03-14-071-Arena_Z80_Codegen_IXL_Bug.md).

---

## What's Next

- **Pipe type inference** — defer lambda type resolution to `.apply()` time for generic pipes
- **`|>` value pipe operator** — F#-style function chaining with tuple auto-unpack
- **BUG-008 fix** — EdgeCost constraints in PBQP to prevent IX/IY conflicts
- **Language Book v5** — incorporate enums, modules, strings, pipe/trans chapters
