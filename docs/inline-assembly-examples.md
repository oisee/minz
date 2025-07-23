# MinZ Inline Assembly Examples

## Basic Usage

### Simple Port Output
```minz
fn set_border_red() -> void {
    asm {
        ld a, 2
        out ($FE), a
    }
    return;
}
```

### Working with Constants
```minz
fn clear_screen() -> void {
    asm {
        ld hl, $4000      ; Screen start
        ld de, $4001
        ld bc, $17FF      ; Screen size - 1
        ld (hl), 0
        ldir
    }
    return;
}
```

### Labels in Assembly
```minz
fn delay() -> void {
    asm {
        ld b, 255
delay_loop:
        djnz delay_loop
    }
    return;
}
```

### Multiple Instructions
```minz
fn draw_pattern() -> void {
    asm {
        ; Draw vertical line
        ld hl, $4000
        ld b, 192         ; 192 lines
        ld de, 32         ; Bytes per line
loop:
        ld (hl), $80      ; Leftmost pixel
        add hl, de
        djnz loop
    }
    return;
}
```

## Current Limitations

### Variable References (!symbol)
The !symbol syntax is implemented but currently has limitations:

1. **Local variables**: Not yet fully working due to how locals are stored
2. **Global variables**: Would work if module-level declarations were supported
3. **Function names**: Would work for calling MinZ functions from assembly

Example (will work in future):
```minz
let screen_buffer: u16 = 0x4000;

fn clear_buffer() -> void {
    asm {
        ld hl, (!screen_buffer)
        ld de, (!screen_buffer + 1)
        ld bc, 6143
        ld (hl), 0
        ldir
    }
    return;
}
```

## Working Examples

### Complete Program - Border Flash
```minz
fn flash_border() -> void {
    let mut i: u8 = 0;
    while i < 10 {
        asm {
            ld a, 2       ; Red
            out ($FE), a
        }
        delay();
        
        asm {
            ld a, 5       ; Cyan
            out ($FE), a
        }
        delay();
        
        i = i + 1;
    }
    return;
}

fn delay() -> void {
    asm {
        ld bc, $FFFF
delay_loop:
        dec bc
        ld a, b
        or c
        jr nz, delay_loop
    }
    return;
}

fn main() -> void {
    flash_border();
    
    asm {
        halt
    }
    
    return;
}
```

### Pixel Plotting (Direct)
```minz
fn plot_pixel() -> void {
    // Plot at (0,0)
    asm {
        ld hl, $4000      ; Screen address
        ld a, $80         ; Leftmost pixel
        ld (hl), a
    }
    return;
}
```

## Best Practices

1. **Keep assembly blocks focused**: One logical operation per block
2. **Comment your assembly**: Especially for complex operations
3. **Use MinZ for logic**: Reserve assembly for hardware access and optimization
4. **Test carefully**: Assembly bypasses MinZ's type safety

## Future Enhancements

1. **Full !symbol resolution**: Access to all MinZ variables and functions
2. **Register constraints**: Specify which registers are used/clobbered
3. **Named assembly blocks**: Reusable assembly routines
4. **Better integration**: Automatic stack management around asm blocks