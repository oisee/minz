# ADR-013: Z80-Native Iterator Design

**Status**: Accepted

**Date**: 2025-07-27

## Context

MinZ needs iteration constructs for arrays and collections. Traditional approaches have problems:
- C-style `for(i=0; i<n; i++)` requires expensive index calculations on Z80
- Pointer arithmetic is error-prone and not Z80-friendly
- References add complexity without clear benefit for iteration

Z80 has specific strengths:
- DJNZ instruction: single-byte decrement-and-jump
- IX/IY indexed addressing: efficient for sequential access
- Block instructions: LDIR, CPIR for bulk operations

## Decision

Implement Z80-native iteration constructs inspired by ABAP:

1. **`do N times`** - Maps to DJNZ for counting loops
2. **`loop at`** - Table iteration with work areas
3. **`!` syntax** - Explicit modification intent
4. **`modify` statement** - Fine-grained write-back control

No general references or pointer arithmetic.

## Consequences

### Positive
- ✅ Direct mapping to optimal Z80 instructions
- ✅ Safe by default (read-only without `!`)
- ✅ Clear modification intent
- ✅ No null/dangling pointer issues
- ✅ Simpler than reference semantics

### Negative
- ❌ Different from mainstream languages
- ❌ No arbitrary array indexing (by design)
- ❌ Learning curve for new syntax

### Neutral
- Work area overhead (mitigated by safety benefits)
- Compiler complexity for work area management

## Implementation Examples

### Counting Loop
```minz
do 10 times {
    beep();
}
```
→
```asm
LD B, 10
.loop:
    CALL beep
    DJNZ .loop
```

### Read-Only Iteration
```minz
loop at sprites -> sprite {
    draw(sprite.x, sprite.y);
}
```
→
```asm
LD IX, sprites
LD B, SPRITE_COUNT
.loop:
    LD A, (IX+0)    ; Load x
    LD L, A
    LD A, (IX+1)    ; Load y
    LD H, A
    CALL draw
    LD DE, SPRITE_SIZE
    ADD IX, DE
    DJNZ .loop
```

### Modifying Iteration
```minz
loop at enemies -> !enemy {
    enemy.hp = enemy.hp - 1;
}
```
→
```asm
LD IX, enemies
LD B, ENEMY_COUNT
.loop:
    ; Load to work area
    LD A, (IX+2)         ; hp offset
    LD (work_enemy+2), A
    
    ; Modify
    DEC A
    
    ; Write back
    LD (IX+2), A
    
    ; Next
    LD DE, ENEMY_SIZE
    ADD IX, DE
    DJNZ .loop
```

## Alternatives Considered

1. **Traditional for loops**: Too expensive on Z80
2. **Iterator objects**: Too much overhead
3. **References**: Unnecessary complexity
4. **Raw pointers**: Unsafe, error-prone

## Related Decisions

- ADR-001: TRUE SMC as default (influences work area implementation)
- ADR-005: No heap allocation (simplifies iterator lifetime)
- ADR-010: Every cycle counts (drives DJNZ optimization)

## Notes

This design makes MinZ distinctly Z80-focused rather than a generic systems language. The syntax is inspired by ABAP's table operations but adapted for Z80's instruction set.