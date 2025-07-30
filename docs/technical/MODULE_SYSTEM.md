# MinZ Module System Design

## Overview

The MinZ module system provides code organization, namespace management, and separate compilation capabilities. It's designed to be simple yet powerful enough for Z80 development.

## Module Structure

### File-based Modules
- Each `.minz` file is a module
- Module name is derived from the file path
- `src/graphics/sprite.minz` → module `graphics.sprite`

### Module Declaration
```minz
// Optional explicit module declaration
module graphics.sprite;

// Import other modules
import std.io;
import game.player as player;
import graphics.screen;

// Module contents
pub struct Sprite {
    x: u8,
    y: u8,
    data: *u8,
}

pub fn draw_sprite(s: *Sprite) -> void {
    // Implementation
}
```

## Import System

### Import Syntax
```minz
// Import entire module
import graphics.sprite;

// Import with alias
import graphics.sprite as spr;

// Import specific items (future enhancement)
import graphics.sprite.{Sprite, draw_sprite};
```

### Import Resolution
1. Relative to current file's directory
2. Project root `src/` directory
3. Standard library location
4. External dependencies (future)

## Visibility

### Public vs Private
```minz
// Public - accessible from other modules
pub fn render() -> void { }
pub struct Point { x: i16, y: i16 }
pub const SCREEN_WIDTH: u16 = 256;

// Private - module internal only
fn helper() -> void { }
struct Internal { data: u8 }
const BUFFER_SIZE: u8 = 32;
```

### Export Control
```minz
// Re-export imported items
pub use graphics.common.Color;

// Export all (not recommended)
pub use graphics.common.*;
```

## Module Compilation

### Compilation Units
- Each module compiles to a separate object file
- Modules are linked together in final build
- Circular dependencies are detected and reported

### Module Interface Files
- `.minzi` files contain module interfaces
- Generated automatically during compilation
- Used for separate compilation

Example `sprite.minzi`:
```
module graphics.sprite;

pub struct Sprite {
    x: u8,
    y: u8,
    data: *u8,
}

pub fn draw_sprite(s: *Sprite) -> void;
pub fn clear_sprite(s: *Sprite) -> void;
```

## Standard Library Modules

### Core Modules
```minz
import std.mem;      // Memory operations
import std.io;       // Input/output
import std.math;     // Math functions
import std.array;    // Array utilities
```

### Platform Modules
```minz
import zx.screen;    // ZX Spectrum screen
import zx.sound;     // Beeper sound
import zx.tape;      // Tape operations
import zx.input;     // Keyboard/joystick
```

## Implementation Plan

### Phase 1: Basic Imports
1. Parse import statements
2. Resolve module paths
3. Load and parse imported modules
4. Merge symbol tables

### Phase 2: Visibility
1. Implement pub/private visibility
2. Check access controls
3. Generate module interfaces

### Phase 3: Separate Compilation
1. Generate object files per module
2. Implement linker
3. Incremental compilation

### Phase 4: Package Management
1. External dependencies
2. Version management
3. Package repository

## Module Examples

### Main Program
```minz
// main.minz
import game.player;
import game.enemy;
import graphics.screen;
import zx.input;

fn main() -> void {
    screen.clear();
    
    let p = player.create(128, 96);
    let enemies = enemy.spawn_wave(3);
    
    loop {
        if input.is_pressed(input.KEY_Q) {
            break;
        }
        
        player.update(&p);
        enemy.update_all(&enemies);
        screen.render();
    }
}
```

### Player Module
```minz
// game/player.minz
module game.player;

import graphics.sprite;
import zx.input;

pub struct Player {
    sprite: sprite.Sprite,
    health: u8,
    score: u16,
}

pub fn create(x: u8, y: u8) -> Player {
    return Player {
        sprite: sprite.create(x, y, &PLAYER_DATA),
        health: 100,
        score: 0,
    };
}

pub fn update(p: *Player) -> void {
    if input.is_pressed(input.KEY_O) {
        p.sprite.x = p.sprite.x - 1;
    }
    if input.is_pressed(input.KEY_P) {
        p.sprite.x = p.sprite.x + 1;
    }
    
    sprite.draw(&p.sprite);
}

const PLAYER_DATA: [u8; 8] = [
    0b00111100,
    0b01111110,
    0b11111111,
    0b11111111,
    0b11111111,
    0b01111110,
    0b00111100,
    0b00000000,
];
```

## Benefits

1. **Code Organization**: Logical grouping of related functionality
2. **Namespace Management**: Avoid naming conflicts
3. **Encapsulation**: Hide implementation details
4. **Reusability**: Share code across projects
5. **Faster Compilation**: Only recompile changed modules
6. **Team Development**: Multiple developers can work independently