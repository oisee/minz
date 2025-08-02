# Zero-Cost Interfaces: The Complete Design

## The Achievement: TRUE Zero-Cost Polymorphism on 8-bit Hardware! 🚀

### What Makes This Special?

Traditional polymorphism requires:
- Virtual tables (vtables) - Memory overhead
- Function pointers - Indirection overhead  
- Dynamic dispatch - Runtime overhead

MinZ interfaces have **NONE OF THESE**! They compile to direct function calls!

## How It Works: The Monomorphization Magic

### 1. Interface Definition
```minz
interface Drawable {
    fun draw(self) -> u8?;
    fun get_area(self) -> u16;
}
```

### 2. Implementation
```minz
impl Drawable for Circle {
    fun draw(self) -> u8? { ... }
    fun get_area(self) -> u16 { self.radius * self.radius * 3 }
}
```

### 3. The Compiler Transform

When you write:
```minz
fun render<T: Drawable>(shape: T) -> u8? {
    return shape.draw()?;
}
```

The compiler generates:
```minz
// For each concrete type used:
fun render_Circle(shape: Circle) -> u8? {
    return Circle_draw(shape)?;  // DIRECT CALL!
}

fun render_Rectangle(shape: Rectangle) -> u8? {
    return Rectangle_draw(shape)?;  // DIRECT CALL!
}
```

### 4. Assembly Output (The Proof!)

Generic call:
```minz
render(my_circle)
```

Compiles to:
```asm
CALL render_Circle  ; Direct call - no indirection!
```

NOT this:
```asm
; Traditional vtable approach
LD HL, (shape_vtable)
LD DE, draw_offset
ADD HL, DE
LD E, (HL)
INC HL
LD D, (HL)
CALL DE_indirect    ; SLOW! 20+ cycles overhead
```

## Advanced Features

### 1. Interface Bounds
```minz
fun complex_render<T: Drawable + Serializable>(item: T) -> u8? {
    item.draw()?;      // Direct call to T_draw
    item.serialize()?; // Direct call to T_serialize
    return 0;
}
```

### 2. Arrays of Interfaces (Tagged Unions)

For heterogeneous collections:
```minz
let shapes: [Drawable; 3] = [circle, rect, triangle];
```

Compiles to:
```asm
shapes_storage:
    DB 0        ; Tag: 0 = Circle
    DS 3        ; Circle data (3 bytes)
    DB 1        ; Tag: 1 = Rectangle  
    DS 4        ; Rectangle data (4 bytes)
    DB 2        ; Tag: 2 = Triangle
    DS 6        ; Triangle data (6 bytes)
```

Dispatch:
```asm
dispatch_draw:
    LD A, (HL)      ; Load tag
    OR A
    JR Z, call_Circle_draw
    DEC A
    JR Z, call_Rectangle_draw
    ; Fall through to Triangle
call_Triangle_draw:
    INC HL          ; Skip tag
    CALL Triangle_draw
    RET
```

Still faster than vtables! Only 4-6 cycles overhead vs 20+.

### 3. Interface Inheritance
```minz
interface Shape {
    fun get_area(self) -> u16;
}

interface Drawable extends Shape {
    fun draw(self) -> u8?;
}
```

### 4. Associated Types
```minz
interface Container {
    type Item;
    fun get(self, index: u16) -> Item;
}

impl Container for Array<u8> {
    type Item = u8;
    fun get(self, index: u16) -> u8 { self.data[index] }
}
```

## Performance Analysis

### Memory Cost
- **Traditional vtable**: 2-4 bytes per object + vtable storage
- **MinZ interface**: 0 bytes for monomorphized, 1 byte tag for heterogeneous

### Runtime Cost  
- **Traditional virtual call**: 20-30 cycles (load vtable, compute offset, indirect call)
- **MinZ interface call**: 0 cycles overhead (direct call)
- **MinZ tagged dispatch**: 4-6 cycles (tag check + direct call)

### Code Size Trade-off
- Monomorphization can increase code size (multiple specialized functions)
- But each function is optimized for its specific type
- Dead code elimination removes unused specializations

## Implementation Strategy

### Phase 1: Monomorphization (DONE ✅)
- Generic functions are duplicated for each concrete type
- Interface bounds are resolved at compile time
- Direct calls replace interface method calls

### Phase 2: Tagged Unions (In Progress 🚧)
- Heterogeneous collections use discriminated unions
- Efficient tag-based dispatch
- Compact memory layout

### Phase 3: Trait Objects (Future)
- When dynamic dispatch is truly needed
- Opt-in feature with explicit syntax
- Still optimized for Z80

## Real-World Example: Game Engine

```minz
interface GameObject {
    fun update(self) -> void;
    fun render(self) -> u8?;
    fun collides(self, other: GameObject) -> bool;
}

struct Player { x: u8, y: u8, health: u8 }
struct Enemy { x: u8, y: u8, type: u8 }
struct Powerup { x: u8, y: u8, effect: u8 }

impl GameObject for Player { ... }
impl GameObject for Enemy { ... }
impl GameObject for Powerup { ... }

// Game loop with ZERO interface overhead!
fun game_loop(player: Player, enemies: [Enemy; 10], powerups: [Powerup; 5]) -> void {
    loop {
        player.update();      // Direct call: Player_update
        player.render()?;     // Direct call: Player_render
        
        for enemy in enemies {
            enemy.update();   // Direct call: Enemy_update  
            enemy.render()?;  // Direct call: Enemy_render
            
            if player.collides(enemy) {  // Direct call
                handle_collision();
            }
        }
        
        for powerup in powerups {
            powerup.update(); // Direct call: Powerup_update
            // etc...
        }
    }
}
```

Every single interface method call above compiles to a direct CALL instruction!

## Why This Matters

1. **Modern abstractions on vintage hardware** - Use interfaces without guilt!
2. **Type safety without runtime cost** - Compiler catches errors, not runtime
3. **Code reuse and modularity** - Write generic code that's still fast
4. **Future-proof design** - Easy to add new types without changing existing code

This is what makes MinZ special - we're not compromising. We're proving that good design and performance can coexist, even on 8-bit machines! 🎯