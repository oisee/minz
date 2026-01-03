# MinZ vs SDCC Benchmark Analysis

Generated: 2026-01-03

## Summary Results

| Test | SDCC Instructions | MinZ Instructions | Ratio |
|------|-------------------|-------------------|-------|
| Fibonacci | 23 | 58 | 2.5x |
| Arithmetic | ~25 | ~60 | 2.4x |
| Iterator | 48 | 103 | 2.1x |

**Current Status**: SDCC generates more compact code for simple cases.

## Analysis

### Why SDCC is Currently Winning

1. **Mature Optimization Pipeline**
   - SDCC has years of Z80-specific optimizations
   - Well-tuned peephole patterns
   - Efficient comparison code generation

2. **Compact Comparisons**
   SDCC generates:
   ```asm
   cp a, #01
   jr nc, label
   ```
   MinZ generates:
   ```asm
   LD B, A
   LD A, B
   LD C, A
   LD A, B
   CP C
   JR C, label_true
   JR Z, label_true
   LD HL, 0
   JR label_done
   label_true:
   LD HL, 1
   label_done:
   ```

3. **No SMC Overhead**
   - MinZ SMC setup adds instructions for parameter anchors
   - This overhead is amortized over many calls but hurts simple tests

### Where MinZ Should Win

1. **Repeated Function Calls (SMC Advantage)**
   - SMC patches immediates directly: 1 write vs push/pop
   - Fibonacci: 2^n calls - SMC should dominate

2. **Zero-Cost Lambdas**
   - MinZ inlines lambdas as direct code
   - C requires function pointers with CALL overhead

3. **DJNZ Loops**
   - MinZ auto-detects count-down loops
   - Current: Not triggering on for-range

## Optimization Opportunities for MinZ

### High Priority

1. **Comparison Codegen** - Reduce from 8+ instructions to 2-3
2. **For-Range to DJNZ** - Auto-convert eligible loops
3. **SMC Bypass** - Skip SMC for simple leaf functions

### Medium Priority

4. **Register Allocation** - Reduce EXX shadow usage
5. **Dead Store Elimination** - Remove `LD B, A; LD A, B` patterns
6. **Conditional Branches** - Use flags directly

## Honest Assessment

MinZ is **not yet more efficient** than SDCC for simple benchmarks. However:

- MinZ's **architecture is sound** for zero-cost abstractions
- **SMC paradigm is novel** - needs better amortization analysis
- **Optimization pipeline is young** - clear improvement paths exist

The goal isn't to beat SDCC at its own game, but to enable:
- Modern abstractions (lambdas, iterators) with zero overhead
- Self-modifying code for extreme optimization
- Z80-native design patterns

## Next Steps

1. Add DJNZ optimization for for-range loops
2. Simplify comparison codegen
3. Create SMC-specific benchmarks (repeated calls)
4. Profile T-state counts, not just instruction counts
