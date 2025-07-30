# 038: Z80-Native Iterator Design (ABAP-Inspired)

## The DJNZ Insight

Z80's DJNZ (Decrement Jump if Not Zero) is perfect for loops:
```asm
LD B, 10
loop:
    ; do something
    DJNZ loop  ; Single instruction: dec B, jump if not zero
```

## Proposal: Z80-Native Loop Constructs

### 1. DO N TIMES - Maps to DJNZ

```minz
// Simple repeat - compiles to DJNZ
do 10 times {
    draw_pixel();
}

// With variable count
let count: u8 = get_count();
do count times {
    process();
}
```

Compiles to:
```asm
LD B, 10        ; Or LD B, (count)
.loop:
    CALL draw_pixel
    DJNZ .loop
```

### 2. LOOP AT Table - Direct Memory Iteration

```minz
let data: [u8; 10] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

// Value copy (safe, one byte at a time)
loop at data -> value {
    process(value);
}

// Direct buffer access (zero-copy)
loop at data into buffer {
    buffer.value = buffer.value * 2;  // Modify in place
}
```

### 3. Explicit Modification with ! Syntax

The `!` prefix makes modification intent crystal clear:

```minz
// Read-only iteration (default, safe)
loop at sprite_table -> sprite {
    draw_sprite(sprite.x, sprite.y);  // Just reading
    // sprite.x = 10;  // COMPILE ERROR! Can't modify
}

// Modification with explicit write-back
loop at sprite_table -> !sprite {
    sprite.x = sprite.x + 1;
    sprite.y = sprite.y + 1;
    // Automatic write-back at end of loop body
}

// Conditional modification
loop at enemies -> !enemy {
    if enemy.hp > 0 {
        enemy.hp = enemy.hp - 1;
        // Write-back happens here
    } else {
        skip;  // No write-back for skipped iterations
    }
}
```

Implementation strategies:

**Read-only (no !)**:
```asm
LD IX, sprite_table
LD B, SPRITE_COUNT
.loop:
    ; Copy to work area (read-only)
    LD A, (IX+0)    ; sprite.x
    LD (work_x), A
    LD A, (IX+1)    ; sprite.y
    LD (work_y), A
    
    ; Use work area
    LD A, (work_x)
    LD L, A
    LD A, (work_y)
    LD H, A
    CALL draw_sprite
    
    ; Next sprite
    LD DE, SPRITE_SIZE
    ADD IX, DE
    DJNZ .loop
```

**Modifying (with !)**:
```asm
LD IX, sprite_table
LD B, SPRITE_COUNT
.loop:
    ; Copy to work area
    LD A, (IX+0)
    LD (work_x), A
    LD A, (IX+1)
    LD (work_y), A
    
    ; Modify work area
    LD A, (work_x)
    INC A
    LD (work_x), A
    LD A, (work_y)
    INC A
    LD (work_y), A
    
    ; Write back
    LD A, (work_x)
    LD (IX+0), A
    LD A, (work_y)
    LD (IX+1), A
    
    ; Next sprite
    LD DE, SPRITE_SIZE
    ADD IX, DE
    DJNZ .loop
```

### 4. Z80-Specific Iterator Modes

```minz
// Mode 1: Copy to work area (safe but slower)
loop at enemies -> enemy {
    if enemy.hp == 0 {
        skip;  // Continue to next
    }
    update_enemy(enemy);
}

// Mode 2: Direct pointer (fast but careful)
loop at enemies via HL {
    // HL register points to current enemy
    asm {
        LD A, (HL)    ; Load HP
        OR A
        JR Z, skip    ; Skip if dead
    }
    update_enemy_at_hl();
}

// Mode 3: Indexed access (when you need position)
loop at bullets indexed by DE {
    // DE = index, HL = pointer
    if is_offscreen(HL) {
        remove_bullet(DE);
    }
}
```

## Implementation Strategy

### Phase 1: DO N TIMES
```minz
do 5 times {
    beep();
}
```

IR:
```
r1 = 5
do_times_1:
    call beep
    r1 = r1 - 1
    jump_if_not_zero r1, do_times_1
```

ASM:
```asm
LD B, 5
.loop:
    CALL beep
    DJNZ .loop
```

### Phase 2: Simple Table Loop
```minz
loop at data -> value {
    sum = sum + value;
}
```

IR:
```
r1 = &data[0]
r2 = 10  ; count
loop_1:
    r3 = load_byte r1
    r4 = sum + r3
    sum = r4
    r1 = r1 + 1
    r2 = r2 - 1
    jump_if_not_zero r2, loop_1
```

### Phase 3: Struct Table with Write-Back

```minz
type Enemy = struct {
    x: u8,
    y: u8,
    hp: u8
};

let enemies: [Enemy; 10];

loop at enemies into enemy {
    enemy.x = enemy.x + enemy.dx;
    enemy.y = enemy.y + enemy.dy;
    // Automatic write-back
}
```

This could use self-modifying code for the write-back address!

## Why This Beats Traditional Approaches

### Traditional C-style:
```c
for (int i = 0; i < 10; i++) {
    enemies[i].x += enemies[i].dx;
}
```
Requires: index calculation, multiplication, addition

### MinZ Z80-native:
```minz
loop at enemies into enemy {
    enemy.x = enemy.x + enemy.dx;
}
```
Just: pointer increment, direct access

## No References Needed!

The key insight: for iteration, we don't need general references. We need:

1. **Work areas** - Compiler-managed temporary space
2. **Direct pointers** - When performance matters
3. **Automatic write-back** - For safety

This is simpler than references and maps better to Z80!

## Explicit MODIFY Statement

For fine-grained control over write-back:

```minz
// Manual write-back control
loop at enemies -> enemy {  // No ! = read-only by default
    enemy.hp = enemy.hp - damage;  // Modify work area only
    
    if enemy.hp > 0 {
        modify enemy;  // Explicit write-back
    } else {
        // Don't write back dead enemies
        remove_from_list();
    }
}

// Batch modifications
loop at particles -> particle {
    particle.x = particle.x + particle.vx;
    particle.y = particle.y + particle.vy;
    particle.life = particle.life - 1;
    
    if particle.life > 0 {
        modify particle;  // Write all changes
    }
}
```

## Syntax Summary

```minz
// Repeat N times (DJNZ)
do 10 times { }
do count times { }

// Iterate values (read-only)
loop at array -> value { }

// Iterate with auto write-back
loop at array -> !value { }

// Manual write-back
loop at array -> value {
    value.field = 42;
    modify value;  // Explicit write
}

// Direct memory iteration
loop at array via HL { }

// With index
loop at array indexed by DE { }

// Early exit
loop at data -> value {
    if value == 0 {
        break;  // Exit loop
    }
    if value == 255 {
        skip;   // Continue (no write-back if !)
    }
}
```

## Conclusion

ABAP's influence + Z80's DJNZ = Perfect match!

No need for complex reference systems. Just:
- Simple, explicit iteration modes
- Compiler-managed work areas
- Direct mapping to Z80 patterns

This makes MinZ truly a Z80-first language, not just "high-level assembly".