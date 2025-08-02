# Zero-Cost Interfaces for MinZ

## Design Philosophy

Zero-cost interfaces in MinZ follow the same principle as our lambda transformation: **interfaces exist only at compile-time and are completely eliminated during compilation**. There are no vtables, no runtime type information, and no dynamic dispatch overhead.

## Interface Syntax

```minz
// Interface definition
interface Drawable {
    fun draw(self) -> u8;
    fun get_size(self) -> u8;
}

// Implementation
struct Circle {
    radius: u8,
}

impl Drawable for Circle {
    fun draw(self) -> u8 {
        // Draw circle logic
        self.radius * 2  // Return diameter as example
    }
    
    fun get_size(self) -> u8 {
        self.radius
    }
}

struct Rectangle {
    width: u8,
    height: u8,
}

impl Drawable for Rectangle {
    fun draw(self) -> u8 {
        // Draw rectangle logic
        self.width + self.height  // Return perimeter as example
    }
    
    fun get_size(self) -> u8 {
        self.width * self.height  // Return area
    }
}

// Usage with compile-time dispatch
fun draw_shape<T: Drawable>(shape: T) -> u8 {
    let size = shape.get_size();
    let result = shape.draw();
    size + result
}

fun main() -> u8 {
    let circle = Circle { radius: 5 };
    let rect = Rectangle { width: 4, height: 3 };
    
    // These calls are monomorphized at compile time
    let circle_result = draw_shape(circle);     // Generates draw_shape_Circle
    let rect_result = draw_shape(rect);         // Generates draw_shape_Rectangle
    
    circle_result + rect_result
}
```

## Compile-Time Transformation

### 1. Monomorphization
Each generic function with interface bounds is specialized for each concrete type:

```minz
// Original generic function
fun draw_shape<T: Drawable>(shape: T) -> u8 {
    shape.draw()
}

// Becomes these specialized functions:
fun draw_shape_Circle(shape: Circle) -> u8 {
    shape.draw()  // Direct call to Circle::draw
}

fun draw_shape_Rectangle(shape: Rectangle) -> u8 {
    shape.draw()  // Direct call to Rectangle::draw
}
```

### 2. Method Resolution
Interface method calls are resolved to direct function calls:

```minz
// Original interface call
shape.draw()

// Becomes direct call (for Circle)
Circle_draw(shape)

// Assembly output (zero overhead)
CALL Circle_draw
```

### 3. Interface Constraints
Interface bounds are checked at compile time and then discarded:

```minz
fun process<T: Drawable + Clone>(item: T) -> u8 {
    let copy = item.clone();  // Clone constraint verified
    item.draw()               // Drawable constraint verified
}

// Becomes (for Circle):
fun process_Circle(item: Circle) -> u8 {
    let copy = Circle_clone(item);  // Direct call
    Circle_draw(item)               // Direct call
}
```

## Advanced Features

### 1. Interface Casting with Type Erasure

```minz
// Type-erased interface object
struct DrawableObject {
    data: *mut u8,        // Pointer to actual object
    draw_fn: *const u8,   // Pointer to draw function
    size_fn: *const u8,   // Pointer to get_size function
}

// Casting to interface (creates fat pointer)
fun to_drawable<T: Drawable>(obj: T) -> DrawableObject {
    DrawableObject {
        data: &obj as *mut u8,
        draw_fn: T::draw as *const u8,
        size_fn: T::get_size as *const u8,
    }
}

// Dynamic dispatch (when needed)
fun draw_dynamic(drawable: DrawableObject) -> u8 {
    // Call through function pointer
    let draw_fn = drawable.draw_fn as fun(*mut u8) -> u8;
    draw_fn(drawable.data)
}
```

### 2. Trait Objects with Method Tables

```minz
// When dynamic dispatch is explicitly needed
fun draw_many(shapes: &[DrawableObject]) -> u8 {
    let total = 0;
    for shape in shapes {
        total += draw_dynamic(shape);
    }
    total
}

// Usage
fun main() -> u8 {
    let circle = Circle { radius: 3 };
    let rect = Rectangle { width: 2, height: 4 };
    
    // Create trait objects only when needed
    let shapes = [
        to_drawable(circle),
        to_drawable(rect),
    ];
    
    draw_many(&shapes)  // Dynamic dispatch
}
```

## Implementation Strategy

### 1. AST Representation

```go
// Interface definition
type InterfaceDecl struct {
    Name    string
    Methods []*InterfaceMethod
}

type InterfaceMethod struct {
    Name       string
    Params     []*Parameter
    ReturnType Type
}

// Implementation block
type ImplBlock struct {
    InterfaceName string  // Optional (for trait impls)
    TypeName      string  // The type being implemented for
    Methods       []*FunctionDecl
}

// Generic function with bounds
type GenericParam struct {
    Name   string
    Bounds []string  // Interface names
}
```

### 2. Semantic Analysis

```go
// Interface method resolution
func (a *Analyzer) resolveInterfaceMethod(recv Type, methodName string) (*FunctionDecl, error) {
    // Find all interfaces implemented by the receiver type
    impls := a.findImplementations(recv)
    
    for _, impl := range impls {
        if method := impl.FindMethod(methodName); method != nil {
            // Generate mangled name for the implementation
            mangledName := fmt.Sprintf("%s_%s_%s", 
                impl.TypeName, impl.InterfaceName, methodName)
            return method, nil
        }
    }
    
    return nil, fmt.Errorf("method %s not found for type %s", methodName, recv)
}

// Generic function monomorphization
func (a *Analyzer) monomorphizeFunction(genericFunc *FunctionDecl, typeArgs []Type) *FunctionDecl {
    // Create specialized version
    specialized := &FunctionDecl{
        Name: fmt.Sprintf("%s_%s", genericFunc.Name, joinTypeNames(typeArgs)),
        // ... copy and specialize body
    }
    
    // Replace interface method calls with direct calls
    a.replaceInterfaceCalls(specialized, typeArgs)
    
    return specialized
}
```

### 3. Code Generation

```go
// Generate direct calls instead of dynamic dispatch
func (g *Z80Generator) generateInterfaceCall(call *InterfaceCall) {
    // Resolve to concrete method at compile time
    concreteMethod := g.resolveMethod(call.Receiver.Type, call.MethodName)
    
    // Generate direct call
    g.emit("CALL %s", concreteMethod.MangledName)
    
    // Zero runtime overhead!
}
```

## Performance Characteristics

### Static Dispatch (Zero Cost)
```minz
fun draw_circle(c: Circle) -> u8 {
    c.draw()  // Direct call: CALL Circle_draw
}
```
- **Overhead**: 0 T-states (same as direct function call)
- **Memory**: 0 bytes (no vtables or type info)
- **Code size**: Minimal (direct calls)

### Dynamic Dispatch (When Needed)
```minz
fun draw_any(d: DrawableObject) -> u8 {
    draw_dynamic(d)  // Indirect call through function pointer
}
```
- **Overhead**: ~20 T-states (function pointer call)
- **Memory**: 4 bytes per trait object (data ptr + 2 function ptrs)
- **Code size**: Small vtable per type

## ZX Spectrum Integration

### Memory Layout
```
Interface Method Tables (when needed):
$C000: Circle_vtable:
       .WORD Circle_draw     ; draw method
       .WORD Circle_get_size ; get_size method

$C004: Rectangle_vtable:
       .WORD Rectangle_draw
       .WORD Rectangle_get_size
```

### Assembly Output Example

**Source Code:**
```minz
interface Drawable {
    fun draw(self) -> u8;
}

fun render<T: Drawable>(shape: T) -> u8 {
    shape.draw()
}

fun main() -> u8 {
    let circle = Circle { radius: 5 };
    render(circle)
}
```

**Generated Assembly:**
```asm
; Monomorphized function: render_Circle
render_Circle:
    ; Direct call - zero overhead!
    CALL Circle_draw
    RET

; Main function
main:
    LD A, 5          ; circle.radius = 5
    LD (circle_data), A
    CALL render_Circle  ; Direct call to specialized version
    RET

; Circle implementation
Circle_draw:
    LD A, (circle_data)  ; Load radius
    SLA A               ; radius * 2 (example logic)
    RET
```

## Standard Library Integration

### Core Interfaces

```minz
// stdlib/interfaces.minz
interface Clone {
    fun clone(self) -> Self;
}

interface Display {
    fun display(self) -> String;
}

interface Iterator<T> {
    fun next(self) -> Option<T>;
    fun has_next(self) -> bool;
}

// Automatic implementations
impl Clone for u8 {
    fun clone(self) -> u8 { self }
}

impl Clone for u16 {
    fun clone(self) -> u16 { self }
}

impl Display for u8 {
    fun display(self) -> String {
        u8_to_string(self)
    }
}
```

### Generic Algorithms

```minz
// Generic sorting (monomorphizes for each type)
fun sort<T: Ord>(arr: &mut [T]) {
    // Bubble sort implementation using T::compare
    for i in 0..arr.len() {
        for j in 0..arr.len()-1-i {
            if arr[j].compare(arr[j+1]) > 0 {
                let temp = arr[j].clone();
                arr[j] = arr[j+1].clone();
                arr[j+1] = temp;
            }
        }
    }
}

// Usage generates specialized versions:
// sort_u8, sort_u16, sort_String, etc.
```

## Compilation Pipeline

### Phase 1: Interface Collection
1. Parse all interface definitions
2. Parse all impl blocks
3. Build interface implementation map

### Phase 2: Generic Analysis
1. Identify generic functions with interface bounds
2. Find all instantiation sites
3. Generate monomorphization work list

### Phase 3: Monomorphization
1. Create specialized versions of generic functions
2. Replace interface method calls with direct calls
3. Verify all interface constraints are satisfied

### Phase 4: Code Generation
1. Generate direct calls for static dispatch
2. Generate vtables only for dynamic dispatch
3. Eliminate unused interface methods

## Usage Examples

### Example 1: Graphics System
```minz
interface Renderable {
    fun render(self, x: u8, y: u8) -> u8;
    fun get_bounds(self) -> (u8, u8);
}

struct Sprite {
    data: *const u8,
    width: u8,
    height: u8,
}

impl Renderable for Sprite {
    fun render(self, x: u8, y: u8) -> u8 {
        zx_draw_sprite(self.data, x, y, self.width, self.height)
    }
    
    fun get_bounds(self) -> (u8, u8) {
        (self.width, self.height)
    }
}

// Zero-cost rendering
fun draw_scene<T: Renderable>(objects: &[T]) {
    for obj in objects {
        obj.render(10, 20);  // Direct call!
    }
}
```

### Example 2: Protocol Implementation
```minz
interface Serializable {
    fun serialize(self) -> &[u8];
    fun deserialize(data: &[u8]) -> Self;
}

struct Player {
    x: u8,
    y: u8,
    score: u16,
}

impl Serializable for Player {
    fun serialize(self) -> &[u8] {
        // Pack into byte array
        [self.x, self.y, self.score as u8, (self.score >> 8) as u8]
    }
    
    fun deserialize(data: &[u8]) -> Player {
        Player {
            x: data[0],
            y: data[1], 
            score: (data[2] as u16) | ((data[3] as u16) << 8),
        }
    }
}

// Generic save/load (monomorphizes for each type)
fun save_data<T: Serializable>(obj: T) -> bool {
    let bytes = obj.serialize();
    write_to_storage(bytes)
}
```

## Benefits

1. **Zero Runtime Overhead**: Interface calls compile to direct function calls
2. **Type Safety**: Full interface constraint checking at compile time
3. **Code Reuse**: Generic algorithms work with any implementing type
4. **Memory Efficient**: No vtables unless explicitly needed for dynamic dispatch
5. **ZX Spectrum Friendly**: Optimized for 64KB memory constraints
6. **Rust-like Ergonomics**: Modern interface design with zero-cost abstractions

This design makes MinZ the first 8-bit language with truly zero-cost interfaces, enabling modern programming patterns without sacrificing performance on vintage hardware.