# ADR-0010: Switch Z80 Codegen Output to Hex Format

## Status: PLANNED

## Context

MinZ's Z80 codegen emits decimal immediates (`LD A, 255`, `CP 68`) while the Z80 ecosystem universally uses hex. The superoptimizer rules (602K from z80-optimizer) use `0FFh` format. Currently we normalize decimal→hex at runtime for every instruction pair lookup, then convert back for output.

## Decision

Switch all numeric immediates in Z80 codegen to `$XX`/`$XXXX` hex format (dollar-prefix), which MZA already parses natively.

## Implementation Plan

### Step 1: Codegen emit sites (~98 occurrences in z80.go)

Replace `%d` format specifiers with hex equivalents:
- 8-bit values: `$%02X` → `$FF`
- 16-bit values: `$%04X` → `$FFFF`
- Data directives: `DB $%02X` / `DW $%04X`

Key locations:
- Data directives (lines ~388, 421, 424, 429): `DB %d` → `DB $%02X`
- Space allocation (lines ~572, 575, 580): `DS %d` → keep decimal (byte count, not value)
- Instruction immediates (lines ~998, 1445, 1450, 2001, 2009, 2012, 2022)
- Indexed addressing (lines ~1294-1295): `IX+%d` → `IX+$%02X`
- SMC parameters already use `#00`/`#C9` — leave as-is

### Step 2: Simplify superopt normalizer

Once codegen emits `$XX`, `normalizeForSuperopt()` only needs:
- `$XX` → `XXh` conversion (already implemented via `dollarHexRe`)
- Remove `operandImmRe` decimal matching (dead code)
- Remove `decToHex()` (dead code)
- `replacementToMinZ()` converts `XXh` → `$XX` instead of decimal

### Step 3: Update tests

- `shadow_test.go` — sjasmplus comparison tests may need format update
- `superopt_peephole_test.go` — normalize test cases change
- Codegen golden tests — output format changes
- Iterator corpus tests — `.a80` output changes

### Step 4: Verify

- Full example compilation regression
- MZA assembles hex output correctly (already proven)
- Superopt still matches rules
- E2E: compile → assemble → emulate cycle unchanged

## Consequences

- Eliminates runtime decimal→hex→decimal conversion in superopt pass
- Output matches Z80 conventions (more readable for retro developers)
- MZA already handles `$XX` — zero assembler changes
- DS/counter arguments stay decimal (they're counts, not values)

## Format Reference

| Before (decimal) | After (hex) |
|---|---|
| `LD A, 255` | `LD A, $FF` |
| `CP 68` | `CP $44` |
| `LD B, 5` | `LD B, $05` |
| `LD HL, 0` | `LD HL, $0000` |
| `DB 65, 66` | `DB $41, $42` |
| `DS 10` | `DS 10` (keep decimal — it's a count) |
