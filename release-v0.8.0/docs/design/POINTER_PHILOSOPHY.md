# MinZ Pointer vs Reference Philosophy

## Current State
MinZ currently uses C-style pointer syntax (`*u8`, `*mut u8`) throughout the codebase. This is evident in:
- Function parameters: `fun sum_array(arr: *u8, len: u8) -> u16`
- Struct operations: `fun move_player(player: *mut Player, dx: i16, dy: i16)`
- Memory operations: `fun fast_copy(dst: *mut u8, src: *u8, len: u16)`

## Design Discussion: References vs Pointers

### Original Vision
The original vision was to avoid raw pointers in favor of safer references, similar to Rust's approach:
- References would be automatically dereferenced
- No null pointer issues
- Memory safety by default
- More ergonomic syntax

### Why We Have Pointers Now
1. **Z80 Reality**: Z80 is fundamentally a pointer-based architecture
   - HL, IX, IY are pointer registers
   - Most operations work on memory addresses
   - Assembly integration requires pointer semantics

2. **@abi Integration**: The @abi system maps directly to registers
   - `@abi("register: HL=ptr")` needs pointer types
   - ROM routines expect raw addresses
   - Hardware drivers work with memory-mapped I/O

3. **Performance**: Direct pointer manipulation is essential
   - Array access compiles to `LD A,(HL)`
   - No overhead for safety checks
   - SMC optimization works on raw addresses

## Proposed Philosophy: Pragmatic Safety

### Keep Pointers, Add Safety Features
1. **Explicit Pointer Types**
   ```minz
   *u8        // Immutable pointer (can't change pointed value)
   *mut u8    // Mutable pointer (can change pointed value)
   *const u8  // Constant pointer (can't change pointer itself)
   ```

2. **Automatic Dereferencing for Common Cases**
   ```minz
   fun process(data: *u8) {
       let value = data;      // Auto-deref in assignments
       if data > 10 { ... }   // Auto-deref in comparisons
       print(data);           // Auto-deref in function calls
   }
   ```

3. **Explicit Dereferencing When Needed**
   ```minz
   fun swap(a: *mut u8, b: *mut u8) {
       let temp = *a;  // Explicit deref needed
       *a = *b;        // Explicit deref for assignment
       *b = temp;
   }
   ```

4. **Array References (Future Enhancement)**
   ```minz
   fun sum(data: &[u8]) -> u16 {  // Slice reference
       let mut total = 0;
       for byte in data {          // Automatic iteration
           total += byte;
       }
       return total;
   }
   ```

## Implementation Strategy

### Phase 1: Fix Current Pointer Support (NOW)
- Implement `*ptr` dereferencing operator
- Fix pointer arithmetic
- Ensure @abi pointer passing works

### Phase 2: Add Safety Features (LATER)
- Non-null pointer types: `*!u8`
- Slice references: `&[u8]`
- Automatic dereferencing contexts
- Lifetime tracking (compile-time only)

### Phase 3: Reference Sugar (FUTURE)
- `&expr` for taking references
- `&mut expr` for mutable references
- Method call syntax: `ptr.method()`

## Decision: Keep Pointers, Enhance Gradually

For MinZ v1.0, we will:
1. **Keep pointer syntax** - It maps perfectly to Z80
2. **Fix pointer dereferencing** - Make examples work
3. **Document safety patterns** - Best practices
4. **Plan reference features** - For v2.0

This pragmatic approach:
- Maintains Z80 efficiency
- Enables @abi integration
- Allows gradual safety improvements
- Keeps the language simple

## Examples of Good Pointer Usage

```minz
// Direct hardware access
@abi("register: HL=addr")
fun poke(addr: *mut u8, value: u8) {
    *addr = value;  // Maps to: LD (HL), A
}

// Efficient array processing
fun clear_screen(screen: *mut u8) {
    let mut ptr = screen;
    loop 0..6144 {
        *ptr = 0;
        ptr = ptr + 1;
    }
}

// Safe pattern with bounds
struct Buffer {
    data: *mut u8,
    size: u16,
    used: u16
}

fun buffer_write(buf: *mut Buffer, value: u8) -> bool {
    if buf.used >= buf.size {
        return false;  // Bounds check
    }
    *(buf.data + buf.used) = value;
    buf.used = buf.used + 1;
    return true;
}
```

## Conclusion

MinZ embraces pointers as a fundamental feature that maps naturally to Z80 architecture. Rather than avoiding them, we make them safe through:
- Clear mutability markers
- Idiomatic usage patterns  
- Future safety enhancements
- Documentation of best practices

This positions MinZ as a pragmatic systems language that provides both power and gradual safety.