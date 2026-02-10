# 132. Zero-Cost Interfaces: Implementation Design

## The Challenge

Traditional interfaces use vtables (virtual method tables) which add:
- Memory overhead (pointer to vtable per object)
- Runtime overhead (indirect calls through vtable)
- Cache misses (jumping to vtable then to method)

For 8-bit systems, this is unacceptable. We need interfaces that compile to direct calls.

## The MinZ Solution: Compile-Time Dispatch

### Core Concept: Tagged Unions + Monomorphization

```minz
interface Drawable {
    fun draw(self, screen: *Screen) -> void;
}

struct Player {
    x: u8, y: u8, sprite: u8
}

struct Enemy {
    x: u8, y: u8, hp: u8, sprite: u8  
}

impl Drawable for Player {
    fun draw(self, screen: *Screen) -> void {
        screen.blit(self.sprite, self.x, self.y);
    }
}

impl Drawable for Enemy {
    fun draw(self, screen: *Screen) -> void {
        if self.hp > 0 {
            screen.blit(self.sprite, self.x, self.y);
        }
    }
}
```

### Behind the Scenes: Monomorphization

When you write:
```minz
fun render_drawable(obj: impl Drawable, screen: *Screen) -> void {
    obj.draw(screen);
}
```

The compiler generates:
```minz
// Two specialized versions!
fun render_drawable$Player(obj: Player, screen: *Screen) -> void {
    Player_draw(obj, screen);  // Direct call!
}

fun render_drawable$Enemy(obj: Enemy, screen: *Screen) -> void {
    Enemy_draw(obj, screen);   // Direct call!
}
```

### For Collections: Tagged Unions

When you need runtime polymorphism:
```minz
type GameObject = Player | Enemy | Powerup;  // Tagged union

fun render_all(objects: [GameObject; 32]) -> void {
    for obj in objects {
        match obj {
            Player(p) => p.draw(screen),   // Direct call
            Enemy(e) => e.draw(screen),     // Direct call
            Powerup(p) => p.draw(screen),   // Direct call
        }
    }
}
```

This compiles to:
```asm
render_all:
    ld hl, objects
    ld b, 32
.loop:
    ld a, (hl)      ; Load type tag
    inc hl
    cp TYPE_PLAYER
    jr z, .draw_player
    cp TYPE_ENEMY
    jr z, .draw_enemy
    ; ... etc
.draw_player:
    call Player_draw    ; Direct call!
    jr .next
.draw_enemy:
    call Enemy_draw     ; Direct call!
.next:
    ; Advance to next object
    djnz .loop
```

## Advanced Pattern: Interface Composition

```minz
interface Collidable {
    fun bounds(self) -> Rect;
    fun on_collision(mut self, other: *GameObject) -> void;
}

interface Updateable {
    fun update(mut self, delta: u8) -> void;
}

// Combine interfaces!
interface Entity: Drawable + Collidable + Updateable {}

struct Boss impl Entity {
    // Must implement all three interfaces
}
```

## Optimization: Small Object Optimization

For interfaces with few implementors:
```minz
// Compiler knows only 3 types implement PowerSource
interface PowerSource {
    fun voltage(self) -> u8;
}

fun check_power(source: impl PowerSource) -> bool {
    source.voltage() > 5
}
```

Compiles to:
```asm
check_power:
    ; Inline the check for all 3 types!
    ld a, (hl)     ; Type tag
    cp TYPE_BATTERY
    jr z, .battery
    cp TYPE_SOLAR
    jr z, .solar
    ; Must be TYPE_NUCLEAR
.nuclear:
    ld a, 12       ; Nuclear always 12V
    jr .compare
.battery:
    inc hl
    ld a, (hl)     ; Battery voltage field
    jr .compare
.solar:
    call get_sunlight
    ; Convert sunlight to voltage
.compare:
    cp 6           ; > 5?
    ret
```

## The Magic: Trait Objects Without Heap

When you absolutely need dynamic dispatch:
```minz
// Stack-allocated trait object!
struct DrawableRef {
    type_id: u8,
    data: [u8; 16]  // Inline storage
}

fun make_drawable_ref(obj: impl Drawable) -> DrawableRef {
    @match_type(obj) {
        Player => DrawableRef {
            type_id: TYPE_PLAYER,
            data: @bit_cast([u8; 16], obj)
        },
        Enemy => DrawableRef {
            type_id: TYPE_ENEMY,
            data: @bit_cast([u8; 16], obj)
        }
    }
}
```

## Real-World Example: Game Entity System

```minz
interface Component {
    const ID: u8;
    fun update(mut self, entity: *Entity) -> void;
}

struct Position impl Component {
    const ID: u8 = 1;
    x: i16, y: i16
    
    fun update(mut self, entity: *Entity) -> void {
        // Update position based on velocity
        if let Some(vel) = entity.get_component::<Velocity>() {
            self.x += vel.dx;
            self.y += vel.dy;
        }
    }
}

struct Sprite impl Component {
    const ID: u8 = 2;
    id: u8, frame: u8
    
    fun update(mut self, entity: *Entity) -> void {
        self.frame = (self.frame + 1) % 4;  // Animate
    }
}

// Entity with inline component storage
struct Entity {
    components: FixedVec<ComponentSlot, 8>
}

// Each slot stores type + data
struct ComponentSlot {
    type_id: u8,
    data: [u8; 15]  // 16 bytes total
}

impl Entity {
    fun update(mut self) -> void {
        // Zero-cost iteration over components
        for slot in self.components.iter_mut() {
            match slot.type_id {
                Position::ID => {
                    let pos = @bit_cast(*Position, &slot.data);
                    pos.update(self);
                },
                Sprite::ID => {
                    let spr = @bit_cast(*Sprite, &slot.data);
                    spr.update(self);
                },
                _ => {}
            }
        }
    }
}
```

## Performance Guarantees

### What Gets Optimized Away:
1. **All interface indirection** - Direct calls everywhere
2. **Type erasure overhead** - Types known at compile time
3. **Dynamic memory allocation** - Everything stack allocated
4. **Virtual method tables** - Don't exist

### Generated Code Quality:
- Interface method call: **Same as direct function call**
- Pattern matching on types: **Jump table (optimal)**
- Monomorphized functions: **Fully inlined when beneficial**
- Component iteration: **Unrolled when component count known**

## Limitations and Workarounds

### Limitation 1: Code Size
Each monomorphized function creates a copy. Solution:
```minz
// Share implementation for similar types
fun draw_sprite_common(sprite: u8, x: u8, y: u8) -> void {
    // Shared implementation
}

impl Drawable for Player {
    fun draw(self, screen: *Screen) -> void {
        draw_sprite_common(self.sprite, self.x, self.y);
    }
}
```

### Limitation 2: Recursive Types
Can't have infinitely recursive interface implementations. Solution:
```minz
// Use explicit bounds
fun process<T: Drawable>(items: [T; const N], depth: u8) -> void {
    if depth == 0 { return; }
    // Process items
}
```

### Limitation 3: Type Erasure
Sometimes you need true type erasure. Solution:
```minz
// Manual vtable when absolutely necessary
struct DrawVTable {
    draw: fn(*void, *Screen) -> void,
    get_bounds: fn(*void) -> Rect
}

// But this is discouraged!
```

## Best Practices

### 1. Prefer Static Dispatch
```minz
// Good: Type known at compile time
fun update_player(player: *Player) -> void { ... }

// Avoid: Dynamic dispatch when not needed  
fun update_thing(thing: impl Updateable) -> void { ... }
```

### 2. Use Tagged Unions for Collections
```minz
// Good: Explicit variants
type Enemy = Goblin | Orc | Dragon;

// Avoid: Trait objects
type Enemy = impl EnemyBehavior;  // Not supported anyway!
```

### 3. Inline Small Types
```minz
// Good: Fits in registers
interface Flags {
    fun to_byte(self) -> u8;
}

// Avoid: Large interfaces
interface GameObject {
    fun get_state(self) -> [u8; 256];  // Too big!
}
```

## Future Optimization: Trait Specialization

```minz
// Specialize for specific types
impl<T> Iterator for T where T: Array {
    // Optimized array iteration
}

impl Iterator for LinkedList {
    // Different strategy for linked lists
}
```

## Conclusion

Zero-cost interfaces in MinZ prove that ergonomic abstractions don't require runtime overhead. By leveraging compile-time type information, monomorphization, and clever code generation, we achieve the impossible: interfaces that are both flexible and free.