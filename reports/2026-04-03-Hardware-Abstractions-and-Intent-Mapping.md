# Hardware Abstractions and Intent Mapping

Date: 2026-04-03
Author: Codex

## Goal

This note sketches which Z80 hardware features are worth reflecting at a higher level in the language or IR, and which ones should remain backend-only choices.

The guiding principle is simple:

- preserve user intent when it is semantically real
- postpone exact instruction choice until backend cost and addressing context are known

## Good Candidates for High-Level Intent

### 1. Bit Access

Source forms like:

- `x.7`
- `x.7 = 1`
- `x.7 = 0`

are excellent high-level intent.

They are semantically clean, architecture-relevant, and map naturally onto either:

- `BIT/SET/RES`
- or generic masking when that is better

This is the model to copy elsewhere.

### 2. Counted Loops

A small counted-loop intent is a strong candidate because it often maps to `DJNZ`.

The source should not expose `DJNZ` directly. It should express:

- a narrow induction variable
- a fixed count
- a simple loop shape

Then the backend can choose `DJNZ` when profitable.

### 3. Carry-Aware Arithmetic

For `u16/u24/u32/fixed`, carry is not an implementation detail on Z80. It is part of the machine's natural arithmetic model.

Useful higher-level intent here includes:

- widening add/sub chains
- add-with-carry / sub-with-borrow style internal ops
- rotate-through-carry patterns for shifts and bitstreams

This should mostly live in IR and optimization, not in user syntax.

### 4. Block Operations

Higher-level intents like:

- copy `n` bytes
- fill `n` bytes
- scan until match

can reasonably map to:

- `LDIR/LDDR`
- `CPIR/CPDR`

The source form is better as an intrinsic or library primitive than as raw assembly-shaped syntax.

### 5. LUT Access

Table lookup intent is already meaningful at the language level, but backend layout decisions still matter:

- static LUT vs generated-at-startup LUT
- `u16` AoS vs split low/high tables
- alignment-sensitive fast paths

The right abstraction split is:

- source expresses “lookup”
- backend chooses the best storage and access shape

## Backend-Only or Mostly Backend-Only Features

### 1. IXH/IXL/IYH/IYL

These should be first-class in the backend model, but not surfaced directly in the language.

The user should express:

- 16-bit values
- byte access
- bit access
- addressable fields and pointers

and the backend should decide whether a byte lives in:

- `L/H`
- `E/D`
- `C/B`
- `IXL/IXH`
- `IYL/IYH`

### 2. Alternate Register Set

`EXX` and `EX AF,AF'` are powerful, but they are too operational to expose directly in ordinary source.

They are better treated as:

- backend scheduling/storage opportunities
- or very explicit advanced intrinsics later, if a real use case demands them

### 3. Exact Prefix Economics

Whether a path should use:

- direct half-reg ops
- `A`-based fallback
- pair shuffles
- memory round-trips

is a backend cost question, not a source-level concern.

The language should preserve enough intent that the backend can make that choice.

## Suggested Mapping Table

| High-Level Intent | Better Source Shape | Likely Z80 Realization |
|---|---|---|
| single-bit read/write | `x.N`, `x.N = 0/1` | `BIT`, `SET`, `RES`, or masks |
| counted byte loop | structured loop / iterator chain | `DJNZ` |
| wider arithmetic | typed `u16/u24/u32/fixed` ops | `ADD/ADC`, `SUB/SBC`, rotates |
| block copy/fill/scan | intrinsic or stdlib helper | `LDIR`, `LDDR`, `CPIR`, `CPDR` |
| computed dispatch | function table / dense switch | `JP (HL)`, `RST`, jump tables |
| table lookup | array indexing / LUT transform | aligned LUT loads, split-byte LUTs |

## Policy Recommendation

When deciding whether to add a new hardware-aware feature, ask:

1. Is the user expressing a real semantic intent, or just a specific instruction?
2. Can that intent survive lowering usefully?
3. Does preserving it materially improve backend choices?
4. Can the same intent still degrade gracefully on non-Z80 targets?

If the answers are mostly yes, it is a good candidate.

If the feature mostly encodes exact instruction choreography, it probably belongs in backend optimization or intrinsics, not in core language syntax.

## Shortlist of Promising Next Intents

The strongest next candidates are:

1. counted loops that can become `DJNZ`
2. carry-aware wider arithmetic normalization
3. block-copy/block-scan intent
4. denser LUT intent and packing/layout choices

These all look like the same pattern that worked for bit selectors: keep the source clean, preserve the intent, and let the backend decide the machine shape late.
