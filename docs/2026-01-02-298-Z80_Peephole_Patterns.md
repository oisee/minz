# Z80 Peephole Optimization Patterns

**Status**: Implemented
**Date**: 2025-12-22
**Patterns**: 44 total (35 original + 9 new)

## Idempotent Operations (multiple → one)

| Pattern | Reduction | Reason |
|---------|-----------|--------|
| `XOR A; XOR A; ...` | `XOR A` | A XOR A = 0, 0 XOR 0 = 0 |
| `AND A; AND A; ...` | `AND A` | A AND A = A, idempotent |
| `OR A; OR A; ...` | `OR A` | A OR A = A, idempotent |
| `SCF; SCF; ...` | `SCF` | Set carry, already set |
| `CCF; CCF` | *eliminate* | Complement twice = no change |

## Redundant Operations (eliminate entirely)

| Pattern | Elimination | Reason |
|---------|-------------|--------|
| `LD r, r` | *eliminate* | LD A,A = NOP |
| `ADD A, 0` | *eliminate* | No change |
| `SUB 0` | *eliminate* | No change |
| `AND $FF` | *eliminate* | No change (8-bit) |
| `OR 0` | *eliminate* | No change |
| `XOR 0` | *eliminate* | No change |
| `SLA r` × 0 | *eliminate* | Shift by 0 |
| `CP 0` after `XOR A` | *eliminate* | Z flag already set |
| `CP 0` after `AND A` | *eliminate* | Z flag already set |
| `OR A` after `XOR A` | *eliminate* | Z flag already set, A=0 |

## Canceling Operations

| Pattern | Elimination | Reason |
|---------|-------------|--------|
| `INC r; DEC r` | *eliminate* | Cancel out |
| `DEC r; INC r` | *eliminate* | Cancel out |
| `PUSH rr; POP rr` | *eliminate* | No effect |
| `EX DE,HL; EX DE,HL` | *eliminate* | Swap twice = no change |
| `EXX; EXX` | *eliminate* | Swap twice = no change |
| `CPL; CPL` | *eliminate* | Complement twice = original |
| `NEG; NEG` | *eliminate* | Negate twice = original |
| `CCF; CCF` | *eliminate* | Complement carry twice |

## Strength Reduction

| Pattern | Replacement | Savings |
|---------|-------------|---------|
| `LD A, 0` | `XOR A` | 1 byte |
| `LD r, 0; LD r', 0; ...` | `XOR A; LD r,A; LD r',A` | n-1 bytes |
| `ADD A, 1` | `INC A` | 1 byte |
| `SUB 1` | `DEC A` | 1 byte |
| `ADD A, $FF` | `DEC A` | 1 byte (wrapping) |
| `SUB $FF` | `INC A` | 1 byte (wrapping) |
| `ADD HL, HL` | `SLA L; RL H` | Consider for size |
| `LD A, r; OR A` | `LD A, r` + check after | Flag already set |
| `CP 1; JR Z` | `DEC A; JR Z` | If A can be modified |

## Jump Optimizations

| Pattern | Replacement | Savings |
|---------|-------------|---------|
| `JP label` → `label:` | *eliminate* | 3 bytes |
| `JR label` → `label:` | *eliminate* | 2 bytes |
| `JP cond, X; JP Y; X:` | `JP !cond, Y; X:` | 3 bytes |
| `JR Z, X; JR Y; X:` | `JR NZ, Y; X:` | 2 bytes |

## Load Optimizations

| Pattern | Replacement | Reason |
|---------|-------------|--------|
| `LD A, (HL); LD (HL), A` | `LD A, (HL)` | Store redundant |
| `LD (HL), A; LD A, (HL)` | `LD (HL), A` | Load redundant |
| `LD A, r; LD r, A` | `LD A, r` | Second LD redundant |
| `LD HL, X; LD HL, Y` | `LD HL, Y` | First load dead |
| `LD A, 0; LD B, A` | `XOR A; LD B, A` | Use XOR for zero |

## Flag-Aware Patterns

| Pattern | Condition | Optimization |
|---------|-----------|--------------|
| `AND A` | After XOR A | Eliminate (Z already set) |
| `OR A` | After XOR A | Eliminate (Z already set) |
| `CP 0` | After XOR A | Eliminate (Z already set) |
| `AND A` | Before JR NZ/Z | Keep (sets flags) |

## Implementation Priority

### High Priority (Common patterns)
1. LD r,r elimination
2. Consecutive identical ops (XOR A, AND A, OR A)
3. INC/DEC cancellation
4. Jump to next instruction
5. ADD A,0 / SUB 0 / AND $FF / OR 0 / XOR 0

### Medium Priority
1. PUSH/POP same register
2. EX twice
3. CP 0 after flag-setting ops
4. Strength reduction for +1/-1

### Low Priority (Rare)
1. CCF twice
2. CPL twice
3. NEG twice

## SDCC Comparison (2025-12-22)

Compared MinZ output vs SDCC 4.2.0 with `--opt-code-size`:

### SDCC Strengths
- **Constant folding**: Computes known values at compile time
- **Register calling convention**: Parameters in A, L - no memory access
- **ADD A,A for ×2**: 4 T-states, 1 byte (vs SLA A: 8 T-states, 2 bytes)

### MinZ Strengths
- **SMC optimization**: Self-modifying code for loop optimizations
- **Zero-cost lambdas**: Direct CALL, no vtables

### Patterns Learned from SDCC
| Pattern | Savings | Implemented |
|---------|---------|-------------|
| `SLA A` → `ADD A,A` | 1 byte, 4 T-states | ✅ Pattern 44 |
| Constant folding | Variable | Future work |

### New Patterns Added (Dec 2025)
| # | Pattern | Category |
|---|---------|----------|
| 36-42 | LD r,r elimination | Redundant |
| 37 | ADD A,0/SUB 0/AND $FF/OR 0/XOR 0 | Redundant |
| 38 | EXX;EXX | Canceling |
| 39 | CCF;CCF | Canceling |
| 40 | CPL;CPL | Canceling |
| 41 | INC r;DEC r / DEC r;INC r | Canceling (flag-safe) |
| 42 | AND A;AND A / OR A;OR A / SCF;SCF | Idempotent |
| 43 | CP 0/OR A/AND A after XOR A | Flag-aware |
| 44 | SLA A → ADD A,A | Strength reduction |

## Flag Safety Mechanism

INC/DEC affect flags (Z, S, H, P/V but NOT carry). The optimizer traces forward to check if flags are used:

```
INC A        ; Sets Z flag if A wraps to 0
DEC A        ; Sets Z flag based on result
LD B, C      ; Doesn't affect flags - keep tracing
XOR A        ; MODIFIES flags → safe to eliminate INC/DEC pair

vs

INC A
DEC A
LD B, C      ; Doesn't affect flags - keep tracing
JR Z, label  ; USES flags → DON'T eliminate!
```

### Protection Annotations
Add these comments to prevent optimization:
- `; @keep` - Never optimize this instruction
- `; @no-opt` - Same as @keep
- `; @preserve` - Same as @keep
- `; INLINE ASM` - Block is inline assembly
