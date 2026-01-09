# ADR: Consecutive LD r,0 Optimization

**Status**: Implemented
**Date**: 2025-12-22
**Location**: `minzc/pkg/optimizer/assembly_peephole.go`

## Context

When initializing multiple registers to zero, the naive code generation produces:
```asm
LD H, 0    ; 2 bytes
LD D, 0    ; 2 bytes
LD E, 0    ; 2 bytes
; Total: 6 bytes
```

On Z80, the `XOR A` instruction clears register A to zero in just 1 byte. We can then use `LD r, A` (also 1 byte each) to copy zero to other registers.

## Decision

Implement a peephole optimization that detects 2+ consecutive `LD r, 0` instructions (where r is B, C, D, E, H, or L) and converts them to:

```asm
XOR A           ; 1 byte - clear A to 0
LD H, A         ; 1 byte
LD D, A         ; 1 byte
LD E, A         ; 1 byte
; Total: 4 bytes (saves 2 bytes)
```

### Why Not Register A?

`LD A, 0` is already handled separately - it's always better to use `XOR A` directly for that case (1 byte vs 2 bytes). This optimization focuses on other registers.

### Threshold

We require **2 or more** consecutive `LD r, 0` instructions to trigger the optimization:

| Count | Before (bytes) | After (bytes) | Savings |
|-------|----------------|---------------|---------|
| 1     | 2              | 2 (XOR A + LD r,A) | 0 (no benefit) |
| 2     | 4              | 3             | 1 byte  |
| 3     | 6              | 4             | 2 bytes |
| 4     | 8              | 5             | 3 bytes |
| n     | 2n             | n+1           | n-1 bytes |

### Clobbering Register A

This optimization **clobbers register A**. The current implementation applies unconditionally during the peephole pass. Future improvements could:

1. Check if A is live at that point
2. Use `EX AF, AF'` to preserve A if needed (adds 1 byte overhead)
3. Reorder instructions to find a better position

For now, we accept this limitation since:
- Register clearing typically happens at function entry before A is used
- The reordering pass may naturally group these instructions together
- The savings are still worthwhile in common cases

## Implementation

```go
func (p *AssemblyPeepholePass) optimizeConsecutiveLdZero(lines []string) []string {
    ldZeroPattern := regexp.MustCompile(`^(\s*)LD\s+([BCDEHL])\s*,\s*0\s*(?:;.*)?$`)
    // ... collects consecutive LD r,0 and replaces with XOR A + LD r,A
}
```

The function:
1. Scans for `LD r, 0` patterns
2. Collects consecutive occurrences (skipping empty lines and comments)
3. If 2+ found, emits `XOR A` followed by `LD r, A` for each register
4. Otherwise keeps original instruction

## Alternatives Considered

### 1. Using EX AF, AF' for A preservation

```asm
EX AF, AF'      ; 1 byte - save A
XOR A           ; 1 byte
LD H, A         ; 1 byte
LD D, A         ; 1 byte
EX AF, AF'      ; 1 byte - restore A
; Total: 5 bytes for 2 registers
```

**Rejected**: Only saves bytes when clearing 4+ registers, and requires knowing A is live.

### 2. Using PUSH AF / POP AF

```asm
PUSH AF         ; 1 byte
XOR A           ; 1 byte
LD H, A         ; 1 byte
POP AF          ; 1 byte
; Total: 4 bytes for 1 register
```

**Rejected**: Stack operations are slower and risk corrupting flags.

### 3. Liveness analysis

Do proper liveness analysis to know when A can be safely clobbered.

**Deferred**: Would require dataflow analysis infrastructure. Can be added later.

## Consequences

### Positive
- Saves 1+ bytes when clearing multiple registers
- Simple pattern matching, easy to maintain
- Comments in output indicate optimization was applied

### Negative
- Clobbers register A (may be problematic if A is live)
- Assumes consecutive `LD r, 0` can be grouped (reordering pass helps)

### Neutral
- No performance impact (same cycle count)

## Testing

Verify with:
```bash
# Compile a program with multiple register initializations
./minzc program.minz -o program.a80 -O

# Check for XOR A + LD r,A patterns
grep -A3 "XOR A.*multi-register" program.a80
```

## Related Optimization: Redundant XOR A Elimination

When code generates multiple `XOR A` instructions with `LD r, A` between them:
```asm
XOR A           ; A = 0
LD B, A         ; B = 0, A still = 0
XOR A           ; REDUNDANT! A is already 0
LD C, A         ; C = 0
```

The `eliminateDuplicateXorA()` function tracks when A is known to be 0:
- `XOR A` sets A = 0
- `LD r, A` (r = B,C,D,E,H,L) preserves A
- Any other instruction clears the tracking (A might change)

**Safety**: Does NOT eliminate `XOR A` after labels (potential SMC patch targets).

## Future: Full Register Value Tracking

The current implementation tracks only "A = 0". A more powerful optimization would track all known register values:
```asm
LD B, 42        ; B = 42
LD A, B         ; A = 42
LD C, 42        ; Could become: LD C, A (saves 1 byte!)
```

This requires full copy propagation / value numbering infrastructure.

## References

- Z80 instruction set: `XOR A` = 1 byte, 4 T-states
- Z80 instruction set: `LD r, 0` = 2 bytes, 7 T-states
- Z80 instruction set: `LD r, A` = 1 byte, 4 T-states
