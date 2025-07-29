# Modern Pointer/Reference Design for MinZ

## Core Philosophy
"Make the common case easy, the complex case possible, with zero overhead"

## Design Principles

### 1. Automatic Dereferencing (Developer Friendly)
```minz
fun process(data: *u8) {
    // Automatic deref in most contexts
    let value = data;        // Reads *data automatically
    if data > 10 { ... }     // Compares *data > 10
    print(data);             // Passes *data to print
    
    // Explicit deref only when needed
    let ptr_value = &data;   // Get the pointer value itself
}
```

### 2. Reference Parameters (Modern & Safe)
```minz
// Pass by reference with & - clear intent
fun swap(a: &mut u8, b: &mut u8) {
    let temp = a;    // Auto-deref: temp = *a
    a = b;           // Auto-deref: *a = *b  
    b = temp;        // Auto-deref: *b = temp
}

fun main() {
    let mut x = 5;
    let mut y = 10;
    swap(&mut x, &mut y);  // Clear at call site
}
```

### 3. Smart Pointer Arithmetic (Z80 Optimized)
```minz
fun process_array(data: *u8, len: u8) {
    // Pointer arithmetic with bounds info
    for i in 0..len {
        data[i] = 0;  // Generates: LD (HL), 0 : INC HL
    }
    
    // Or manual pointer advancement
    let mut ptr = data;
    while ptr < data + len {
        ptr = 0;      // Auto-deref: *ptr = 0
        ptr += 1;     // Advances pointer
    }
}
```

### 4. Array References (Slice Pattern)
```minz
// Array reference knows its length
fun sum(data: &[u8]) -> u16 {
    let mut total: u16 = 0;
    for byte in data {      // Automatic bounds
        total += byte;
    }
    return total;
}

fun main() {
    let arr: [10]u8 = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10};
    let total = sum(&arr);  // Pass whole array
    let partial = sum(&arr[2..5]); // Pass slice
}
```

### 5. Struct References (Zero Overhead)
```minz
struct Point { x: u8, y: u8 }

// Reference parameter - no copying
fun move_point(p: &mut Point, dx: u8, dy: u8) {
    p.x += dx;  // Direct memory update: LD A,(IX+0) : ADD A,dx : LD (IX+0),A
    p.y += dy;  // Direct memory update: LD A,(IX+1) : ADD A,dy : LD (IX+1),A
}

fun main() {
    let mut pos = Point { x: 10, y: 20 };
    move_point(&mut pos, 5, 3);
}
```

## Implementation Strategy

### Phase 1: Core Reference Semantics
1. **Reference Types**: `&T` and `&mut T`
2. **Reference Creation**: `&expr` and `&mut expr`
3. **Auto-deref Contexts**: 
   - Assignment RHS
   - Binary operators
   - Function arguments
   - Conditionals

### Phase 2: Smart Lowering
```minz
// This code:
fun example(data: &mut u8) {
    data = 42;
}

// Lowers to this IR:
// fun example(data: *mut u8) {
//     *data = 42;
// }

// Generates this Z80:
// LD A, 42
// LD (HL), A
```

### Phase 3: Pointer/Reference Unification
- Pointers (`*T`) for FFI and low-level work
- References (`&T`) for safe MinZ code
- Seamless conversion between them
- Same Z80 code generation

## Syntax Examples

### Basic Usage
```minz
// Modern reference style
fun increment(x: &mut u8) {
    x += 1;  // Clean, no explicit deref
}

// Low-level pointer style (when needed)
fun poke(addr: *mut u8, value: u8) {
    *addr = value;  // Explicit deref required
}

// Mixed for pragmatism
fun memcpy(dest: *mut u8, src: &[u8]) {
    for i in 0..src.len {
        *(dest + i) = src[i];
    }
}
```

### Auto-deref Rules
```minz
fun example(ptr: *u8, ref: &u8) {
    // Auto-deref in these contexts:
    let a = ptr;         // a = *ptr
    let b = ref;         // b = *ref
    let c = ptr + 5;     // c = *ptr + 5
    foo(ptr);            // foo(*ptr)
    if ptr > 10 { }     // if *ptr > 10
    
    // NO auto-deref in these contexts:
    let d = &ptr;        // d is **u8 (pointer to pointer)
    bar(&ptr);           // Passes pointer address
    ptr += 1;            // Advances pointer itself
}
```

### Method Syntax (Future)
```minz
impl Point {
    // Self is automatically a reference
    fun distance(&self, other: &Point) -> u16 {
        let dx = (self.x - other.x) as i16;
        let dy = (self.y - other.y) as i16;
        return sqrt(dx * dx + dy * dy);
    }
}

let p1 = Point { x: 10, y: 20 };
let p2 = Point { x: 30, y: 40 };
let dist = p1.distance(&p2);  // Clean method call
```

## Z80 Code Generation

### Reference Parameters
```minz
fun add(a: &u8, b: &u8) -> u8 {
    return a + b;
}
```

Generates:
```asm
; HL = a (pointer), DE = b (pointer)
LD A, (HL)      ; Auto-deref a
EX DE, HL
LD B, (HL)      ; Auto-deref b  
ADD A, B
RET
```

### Mutable References
```minz
fun increment(x: &mut u8) {
    x += 1;
}
```

Generates:
```asm
; HL = x (pointer)
LD A, (HL)      ; Auto-deref for read
INC A
LD (HL), A      ; Auto-deref for write
RET
```

### Array References
```minz
fun clear(data: &mut [u8]) {
    for i in 0..data.len {
        data[i] = 0;
    }
}
```

Generates:
```asm
; HL = data.ptr, BC = data.len
.loop:
    LD (HL), 0
    INC HL
    DEC BC
    LD A, B
    OR C
    JR NZ, .loop
RET
```

## Benefits

1. **Developer Friendly**: No explicit `*` everywhere
2. **Type Safe**: Can't mix pointers and values accidentally  
3. **Zero Overhead**: Same Z80 code as manual pointers
4. **Clear Intent**: `&mut` shows modification at call site
5. **Gradual Adoption**: Can mix with existing pointer code

## Migration Path

### Step 1: Add reference types alongside pointers
### Step 2: Implement auto-deref in common contexts
### Step 3: Encourage references in new code
### Step 4: Keep pointers for FFI/@abi functions

This gives us Rust-like safety and ergonomics with C-like performance!