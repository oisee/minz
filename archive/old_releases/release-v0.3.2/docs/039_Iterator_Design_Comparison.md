# 039: Iterator Design Comparison

## Three Approaches to Iteration

### 1. Traditional C-Style (What We're Avoiding)

```c
// Expensive index calculation
for (int i = 0; i < enemy_count; i++) {
    enemies[i].x += enemies[i].dx;  // Multiply i by struct size
    enemies[i].y += enemies[i].dy;  // Calculate address again!
}
```

**Z80 Cost**: 
- Index multiplication (no MUL instruction!)
- Address calculation per field access
- ~30-40 cycles per array access

### 2. Reference-Based (My First Proposal)

```minz
// Using SMC references
loop at enemies -> ref enemy {
    enemy.x = enemy.x + enemy.dx;  // Patches addresses
    enemy.y = enemy.y + enemy.dy;
}
```

**Problems**:
- Complex SMC address patching
- Overhead of reference management
- Solving a problem we don't have

### 3. Work Area Approach (The Winner!)

```minz
// Read-only (safe default)
loop at enemies -> enemy {
    draw_enemy(enemy.x, enemy.y);  // Just reading
}

// Explicit modification
loop at enemies -> !enemy {
    enemy.x = enemy.x + enemy.dx;  // Work area
    enemy.y = enemy.y + enemy.dy;  // Work area
    // Automatic write-back at }
}

// Conditional write-back
loop at enemies -> enemy {
    enemy.hp = enemy.hp - damage;
    if enemy.hp > 0 {
        modify enemy;  // Explicit write
    }
}
```

**Benefits**:
- Clear intent (! for modification)
- Safe by default (read-only)
- Efficient work area pattern
- Natural Z80 code generation

## Code Generation Comparison

### Input Code
```minz
loop at sprites -> !sprite {
    sprite.x = sprite.x + 1;
    sprite.y = sprite.y + 1;
}
```

### Method 1: Direct Memory (Risky)
```asm
LD HL, sprites
LD B, SPRITE_COUNT
.loop:
    INC (HL)      ; x++
    INC HL
    INC (HL)      ; y++
    INC HL
    ; ... rest of struct
    DJNZ .loop
```
Fast but: No safety, corrupts on error

### Method 2: Work Area (Recommended)
```asm
LD IX, sprites
LD B, SPRITE_COUNT
.loop:
    ; Load to work area
    LD A, (IX+0)
    LD (work_sprite+0), A
    LD A, (IX+1)
    LD (work_sprite+1), A
    
    ; Modify work area
    LD HL, work_sprite
    INC (HL)      ; x++
    INC HL
    INC (HL)      ; y++
    
    ; Write back
    LD A, (work_sprite+0)
    LD (IX+0), A
    LD A, (work_sprite+1)
    LD (IX+1), A
    
    ; Next
    LD DE, SPRITE_SIZE
    ADD IX, DE
    DJNZ .loop
```
Safer: Changes isolated until write-back

### Method 3: SMC-Optimized Work Area
```asm
; Pre-calculate addresses with SMC
LD HL, sprites
LD B, SPRITE_COUNT
.loop:
    ; Patch load addresses
    LD (load_x+1), HL
    LD (load_y+1), HL
    INC HL
    LD (load_y+1), HL
    DEC HL
    
    ; Load to work area
load_x: LD A, ($0000)    ; SMC!
    INC A
    LD (work_x), A
load_y: LD A, ($0000)    ; SMC!
    INC A
    LD (work_y), A
    
    ; Patch store addresses
    LD (store_x+1), HL
    INC HL
    LD (store_y+1), HL
    INC HL
    
    ; Write back
    LD A, (work_x)
store_x: LD ($0000), A   ; SMC!
    LD A, (work_y)
store_y: LD ($0000), A   ; SMC!
    
    ; Continue
    DJNZ .loop
```

## The Beauty of ! Syntax

```minz
// Clear intent
loop at data -> value {        // Read-only
    print(value);
}

loop at data -> !value {       // Will modify
    value = process(value);
}

// Compiler can:
// 1. Enforce read-only when no !
// 2. Optimize differently
// 3. Warn about unused modifications
// 4. Skip write-back on break/skip
```

## Why Not References?

References solve problems we don't have:
- Single value modification → Just use regular parameters
- Array element access → Iterators are better
- Pointer arithmetic → We're avoiding this!

Work areas solve problems we DO have:
- Safe modification of array elements
- Clear modification intent
- Efficient Z80 patterns
- No pointer confusion

## Conclusion

The `!` syntax for modification intent is brilliant because:
1. **Explicit** - You know what will be modified
2. **Safe** - Read-only by default
3. **Efficient** - Compiler knows when to write back
4. **Z80-friendly** - Maps to natural patterns

This is simpler than references and more powerful for the actual use cases we have!