# 036: Z80-Friendly Pointer and Array Design Brainstorm

## The Pointer Problem

Traditional pointers are problematic:
- Easy to misuse (null, dangling, arithmetic errors)
- Hidden costs on Z80 (16-bit operations for addresses)
- Complicate optimization (aliasing analysis)

## Proposal 1: References with TRUE SMC

Instead of pointers, use **references** that leverage our SMC strength:

```minz
fun modify_value(ref x: u8) -> void {
    x = x + 1;  // Direct modification
}

fun main() -> void {
    let value: u8 = 10;
    modify_value(ref value);  // Pass by reference
    // value is now 11
}
```

### Implementation via SMC

For references, we patch the ADDRESS instead of the value:

```asm
; Traditional pointer approach:
LD HL, address_of_x  ; Load pointer
LD A, (HL)          ; Dereference
INC A
LD (HL), A          ; Store back

; SMC reference approach:
x_ref$imm:
LD A, ($0000)       ; Address patched here!
INC A
x_ref_store$imm:
LD ($0000), A       ; Same address patched
```

**Benefits**:
- Zero overhead for access (direct addressing)
- No pointer arithmetic needed
- Safer - can't create invalid references
- Consistent with TRUE SMC philosophy

## Proposal 2: Array Access Without Indexing

Traditional array indexing is expensive on Z80:
```asm
; array[i] traditionally:
LD HL, array_base
LD D, 0
LD E, (index)
ADD HL, DE         ; Expensive!
LD A, (HL)
```

### Option A: Iterator-Only Arrays

```minz
let data: [u8; 10] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

// No direct indexing! Use iterators:
for value in data {
    process(value);
}

// Or with index:
for (i, value) in data.enumerate() {
    if i == 5 {
        // Special case for 5th element
    }
}
```

### Option B: Explicit Element References

```minz
fun get_element(arr: [u8; 10], index: u8) -> ref u8 {
    // Compiler generates optimal code for constant indices
    match index {
        0 => ref arr[0],  // Direct address
        1 => ref arr[1],  // Direct address
        _ => panic("Index out of bounds")
    }
}

// Or with SMC magic:
fun array_ref(arr: [u8; N], index: u8, ref out: u8) -> void {
    // Patches 'out' to point to arr[index]
    @smc_patch_ref(out, arr + index);
}
```

### Option C: Z80-Friendly Array Operations

Design operations that map well to Z80:

```minz
// Instead of arbitrary indexing, provide:
fun array_copy(src: [u8; N], dst: [u8; N]) -> void {
    // Compiles to LDIR
}

fun array_fill(arr: [u8; N], value: u8) -> void {
    // Unrolled stores or LDIR with prepared buffer
}

fun array_find(arr: [u8; N], value: u8) -> Option<u8> {
    // CPIR instruction
}

// For tables, use compile-time indexing:
const SINE_TABLE: [u8; 256] = generate_sine_table();

fun get_sine(angle: u8) -> u8 {
    @table_lookup(SINE_TABLE, angle)  // Special builtin
}
```

## Proposal 3: Memory Blocks

For dynamic data, use explicit memory blocks:

```minz
type MemBlock = {
    base: u16,      // Base address
    size: u16,      // Size in bytes
}

fun mem_read(block: MemBlock, offset: u16) -> u8 {
    return peek(block.base + offset);
}

fun mem_write(block: MemBlock, offset: u16, value: u8) -> void {
    poke(block.base + offset, value);
}

// Screen memory example:
const SCREEN: MemBlock = { base: 0x4000, size: 6144 };

fun set_pixel(x: u8, y: u8) -> void {
    let offset: u16 = calculate_screen_offset(x, y);
    let current: u8 = mem_read(SCREEN, offset);
    mem_write(SCREEN, offset, current | (1 << (x & 7)));
}
```

## Design Philosophy

### 1. Make Costs Explicit
- No hidden pointer arithmetic
- Clear when 16-bit math happens
- Obvious memory access patterns

### 2. Leverage Z80 Strengths
- Direct addressing when possible
- Use block operations (LDIR, CPIR)
- Compile-time address resolution

### 3. Safety Through Simplicity
- No null references
- No pointer arithmetic
- Bounds checking at compile time where possible

## Recommendation: Hybrid Approach

1. **References for single values** - SMC-powered, zero overhead
2. **Iterators for sequential access** - Maps to efficient loops
3. **Memory blocks for dynamic data** - Explicit, flexible
4. **Compile-time indexing for tables** - Via metaprogramming

This gives us:
- Safety (no raw pointers)
- Performance (SMC optimization)
- Flexibility (can still do systems programming)
- Z80-friendly (matches hardware capabilities)

## Implementation Priority

1. **First**: References with SMC patching
2. **Second**: Iterator-based array access
3. **Third**: Memory block operations
4. **Fourth**: Compile-time table indexing

This approach makes MinZ truly unique - not just "C for Z80" but a language that embraces Z80's strengths while providing modern safety.