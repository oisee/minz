# ADR: MIR-Level Value Tracking with Codegen Hints

**Status**: Implemented (Phase 1 - XOR A hints only)
**Parked**: INC/DEC hints deferred - assembly peephole handles these well
**Date**: 2025-12-22
**Supersedes**: Partial functionality in assembly_peephole.go

## Context

Currently, optimizations like "use INC instead of LD when value is +1" happen at assembly level via pattern matching. This has limitations:

1. **Lost structure**: Assembly is flat, hard to track across jumps
2. **Duplicate work**: Assembly peephole rediscovers what MIR knew
3. **Limited scope**: Can only see local patterns

Moving value tracking to MIR level allows:
1. Better analysis with structured control flow
2. Hints passed to codegen - emit optimal code directly
3. Assembly peephole becomes "cleanup" for edge cases

## Decision

Implement a **MIR Value Tracking Pass** that:

1. Tracks constant values in virtual registers
2. Detects delta patterns (value changed by +1, -1, or unchanged)
3. Attaches hints to instructions for code generator
4. Code generator uses hints to emit INC/DEC/eliminate directly

### Data Structures

```go
// Add to ir.Instruction
type CodegenHints struct {
    // Value optimization hints
    CanUseINC    bool   // Emit INC instead of LD (delta = +1)
    CanUseDEC    bool   // Emit DEC instead of LD (delta = -1)
    CanEliminate bool   // Skip entirely (same value)
    CanUseXOR    bool   // Use XOR A for zero

    // Known value info
    IsConstant   bool   // Value is compile-time constant
    ConstValue   int64  // The constant value (if IsConstant)

    // Register preference
    PreferA      bool   // Prefer accumulator
    PreferB      bool   // Prefer B (for DJNZ loops)
    PreferHL     bool   // Prefer HL (for pointers)
}
```

### Algorithm

```
For each function:
  valueMap = {} // Register -> ValueInfo

  For each instruction:
    // Update tracking based on instruction
    if OpLoadConst:
      prev = valueMap[dest]
      if prev.IsConstant:
        delta = imm - prev.Value
        if delta == +1: hint.CanUseINC = true
        if delta == -1: hint.CanUseDEC = true
        if delta == 0:  hint.CanEliminate = true
      if imm == 0: hint.CanUseXOR = true
      valueMap[dest] = {IsConstant: true, Value: imm}

    if OpMove:
      valueMap[dest] = valueMap[src]  // Copy tracking

    if OpAdd, OpSub, etc:
      valueMap[dest] = unknown  // Result not constant (unless both operands are)

    if OpLabel, OpJump, OpCall:
      valueMap = {}  // Invalidate all tracking
```

## Implementation Plan

### Phase 1: Add Hints to IR (30 min)
- Add `CodegenHints` struct to `ir.Instruction`
- No behavior change yet

### Phase 2: Value Tracking Pass (1 hour)
- Create `MIRValueTrackingPass` in optimizer
- Track constants and detect deltas
- Set hints on instructions

### Phase 3: Codegen Uses Hints (1 hour)
- Modify `z80.go` to check hints
- Emit INC/DEC/XOR based on hints
- Skip eliminated instructions

### Phase 4: Integration & Testing (30 min)
- Add pass to optimizer pipeline
- Test with examples
- Verify byte savings

## File Changes

| File | Change |
|------|--------|
| `pkg/ir/ir.go` | Add `CodegenHints` struct |
| `pkg/optimizer/mir_value_tracking.go` | New pass |
| `pkg/optimizer/optimizer.go` | Register new pass |
| `pkg/codegen/z80.go` | Check hints before emit |

## Consequences

### Positive
- Cleaner architecture (analysis at MIR, emit at codegen)
- Better optimization (sees more context)
- Assembly peephole simplified (just cleanup)
- Foundation for more MIR optimizations

### Negative
- More complexity in MIR
- Hints must be kept in sync with analysis
- Two places to maintain (MIR tracking + codegen)

### Neutral
- Assembly peephole still catches inline asm patterns

## Testing

```minz
fun test_delta() -> void {
    let a: u8 = 0xFE;
    a = 0xFF;    // Should use INC (delta +1)
    a = 0xFE;    // Should use DEC (delta -1)
    a = 0xFE;    // Should eliminate (delta 0)

    let b: u8 = 0;   // Should use XOR A
    let c: u8 = 0;   // Should use LD C, A (A already 0)
}
```

**Expected output**:
```asm
    LD A, $FE
    INC A         ; Was LD A, $FF (hint: CanUseINC)
    DEC A         ; Was LD A, $FE (hint: CanUseDEC)
    ; eliminated  ; Was LD A, $FE (hint: CanEliminate)
    XOR A         ; hint: CanUseXOR
    LD C, A       ; A already 0, reuse
```

## References

- [296_MIR_Level_Optimizations_Brainstorm.md](296_MIR_Level_Optimizations_Brainstorm.md)
- [295_Register_Value_Propagation.md](295_Register_Value_Propagation.md)
- [294_Consecutive_LD_Zero_Optimization.md](294_Consecutive_LD_Zero_Optimization.md)
