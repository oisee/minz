# MZA: Table-Driven Z80 Assembler Encoder

**Status:** Complete (v0.18.0)
**Commits:** `672604c`, `3c5b798`

## Summary

The MZA assembler's instruction encoder was completely rewritten from a hand-coded approach to a **table-driven pattern matching system**. The old encoder (~2000 lines across `encoder.go`, `instruction_sets.go`, `undocumented.go`, `instructions.go`) was replaced by two files: `instruction_table.go` + `pattern_matcher.go`.

## Architecture

### Old Encoder (Removed)
- Hand-coded `switch/case` chains for every instruction variant
- Separate files for main, CB, ED, DD/FD prefix instructions
- Separate file for undocumented opcodes (IX/IY half-register ops)
- Difficult to maintain — adding one instruction meant editing multiple locations
- `resolveValue()` lived in `encoder.go`

### New Table-Driven Encoder
- **`instruction_table.go`**: ~1400 lines of declarative instruction definitions
  - Each entry: `{mnemonic, operand patterns, opcode bytes, size, cycles}`
  - All Z80 instructions including undocumented (IXH, IXL, IYH, IYL ops)
  - All prefix combinations: main, CB, ED, DD, FD, DDCB, FDCB
- **`pattern_matcher.go`**: ~350 lines of matching logic
  - Parses operands against patterns (registers, immediates, indexed, conditions)
  - `resolveValue()` moved here (needed by `directives.go` too)
  - `matchInstruction()` finds the best match from the table
  - Handles displacement byte encoding for (IX+d)/(IY+d)

### Key Design Decisions
1. **Data over code**: Instructions defined as data, not logic
2. **Single source of truth**: One table for all instructions
3. **Pattern-based matching**: Operand types (reg8, reg16, imm8, imm16, indexed, condition) are pattern tokens
4. **AF' in parseReg16()**: Required for `EX AF, AF'` — must be in the register case list

## Metrics

| Metric | Old | New |
|--------|-----|-----|
| Files | 4 | 2 |
| Lines | ~2000 | ~1750 |
| Instructions | ~800 | All Z80 (including undocumented) |
| Adding an instruction | Edit multiple switch cases | Add one table row |

## Test Status
- Existing assembler tests pass
- `TestSymbols` failure is pre-existing (unrelated to encoder)
- Agon-specific tests in `agon_test.go` cover eZ80 extensions
