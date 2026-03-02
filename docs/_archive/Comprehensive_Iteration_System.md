# Comprehensive Iteration System: Arrays, Strings, and u16 Lengths

## 🎯 THE CHALLENGE: Multiple Data Types, Optimal Performance

MinZ needs to handle iteration over:
1. **u8 arrays** - Small arrays (≤255 elements) using DJNZ
2. **u16 arrays** - Large arrays (>255 elements) using 16-bit counters  
3. **Strings** - Both u8 and u16 length-prefixed strings
4. **Mixed data types** - Unified iterator interface

---

## 1. CURRENT STRING ARCHITECTURE (Already Optimal!)

### Length-Prefixed Strings
```minz
// MinZ strings are already optimized!
let msg: *u8 = "Hello!";    // Compiles to: DB 6, "Hello!"
```

### Current String Iteration (DJNZ-Ready!)
```asm
; Our current print_string is already perfect for iteration!
print_string:
    LD B, (HL)         ; B = length from first byte
    INC HL             ; HL -> string data
    LD A, B            ; Check if length is zero
    OR A
    RET Z              ; Return if empty string
print_loop:
    LD A, (HL)         ; Load character (7 T-states)
    RST 16             ; Print character (11 T-states)
    INC HL             ; Next character (6 T-states)
    DJNZ print_loop    ; Decrement B and loop (13/8 T-states)
    RET
```

**This is PERFECT for string iteration!** We just need to generalize it.

---

## 2. OPTIMAL ITERATION PATTERNS BY DATA TYPE

### Pattern 1: Small Arrays/Strings (≤255 elements) - DJNZ
```asm
; For u8 length or known small arrays
small_array_iter:
    LD HL, array_data   ; Pointer to data
    LD B, array_length  ; Length in B (must be ≤255)
small_loop:
    LD A, (HL)         ; Load element
    ; ... process element ...
    INC HL             ; Next element (for u8)
    DJNZ small_loop    ; Super efficient!
; Cost: ~18 T-states per element
```

### Pattern 2: Large Arrays (>255 elements) - 16-bit Counter
```asm
; For u16 length arrays
large_array_iter:
    LD HL, array_data   ; Pointer to data  
    LD BC, array_length ; Length in BC (16-bit)
large_loop:
    LD A, B            ; Check if BC == 0
    OR C
    JR Z, large_done   ; Exit if zero
    LD A, (HL)         ; Load element
    ; ... process element ...
    INC HL             ; Next element
    DEC BC             ; Decrement 16-bit counter
    JR large_loop      ; Continue
large_done:
; Cost: ~25 T-states per element (still good!)
```

### Pattern 3: String Iteration - Both Lengths
```asm
; For strings (auto-detect length type)
string_iter:
    LD A, (HL)         ; Check first byte
    CP 255             ; Is it 255 (extended length marker)?
    JR Z, string_u16   ; If yes, use u16 length
    
string_u8:             ; u8 length string
    LD B, A            ; Length in B
    INC HL             ; Skip length byte
    ; Use DJNZ pattern
    JR string_common
    
string_u16:           ; u16 length string  
    INC HL            ; Skip marker
    LD C, (HL)        ; Low byte of length
    INC HL
    LD B, (HL)        ; High byte of length
    INC HL            ; Skip length word
    ; Use 16-bit counter pattern
    
string_common:
    ; Process characters...
```

---

## 3. ITERATOR INTERFACE DESIGN

### Unified Iterator Structure
```minz
enum IteratorType {
    Small,    // Uses DJNZ (≤255 elements)
    Large,    // Uses 16-bit counter (>255 elements)
    String,   // Auto-detects u8/u16 length
}

struct Iterator[T] {
    ptr: *T,              // Current element pointer (HL)
    remaining: u16,       // Elements remaining (B for small, BC for large)
    iter_type: IteratorType,
    element_size: u8,     // Bytes per element (1=u8, 2=u16, etc.)
}
```

### Method Implementations by Type

#### Small Array Iterator (DJNZ)
```minz
impl Iterator[T] for SmallArray[T] {
    fun forEach(self, action: fn(T) -> void) -> void {
        // Compiles to DJNZ pattern
        asm {
            "LD HL, ${self.ptr}"
            "LD B, ${self.remaining}"
            "loop:"
            "LD A, (HL)"
            // Call action(A)
            "INC HL"
            "DJNZ loop"
        }
    }
}
```

#### Large Array Iterator (16-bit)
```minz
impl Iterator[T] for LargeArray[T] {
    fun forEach(self, action: fn(T) -> void) -> void {
        // Compiles to 16-bit counter pattern
        asm {
            "LD HL, ${self.ptr}"
            "LD BC, ${self.remaining}"
            "loop:"
            "LD A, B"
            "OR C"
            "JR Z, done"
            "LD A, (HL)"
            // Call action(A)
            "INC HL"
            "DEC BC"
            "JR loop"
            "done:"
        }
    }
}
```

#### String Iterator (Auto-detecting)
```minz
impl Iterator[char] for String {
    fun forEach(self, action: fn(char) -> void) -> void {
        // Auto-detect length type and use optimal pattern
        if (self.length <= 255) {
            // Use DJNZ pattern
            self.as_small_array().forEach(action);
        } else {
            // Use 16-bit pattern  
            self.as_large_array().forEach(action);
        }
    }
}
```

---

## 4. STRING SYSTEM ENHANCEMENTS

### Current String Format (u8 length)
```
[ Length: u8 ][ Data: char[] ]
```

### Enhanced String Format (Supporting u16)
```
// Small string (≤254 chars)
[ Length: u8 ][ Data: char[] ]

// Large string (≥255 chars)  
[ Marker: 255 ][ Length: u16 ][ Data: char[] ]
```

### String Length Detection
```minz
fun get_string_length(str: *u8) -> u16 {
    if (str[0] == 255) {
        // u16 length: read next 2 bytes
        return (str[2] << 8) | str[1];
    } else {
        // u8 length: just read first byte
        return str[0] as u16;
    }
}

fun get_string_data(str: *u8) -> *u8 {
    if (str[0] == 255) {
        return str + 3;  // Skip marker + u16 length
    } else {
        return str + 1;  // Skip u8 length
    }
}
```

---

## 5. COMPILER OPTIMIZATIONS

### Length-Based Optimization Selection
```go
// In compiler
func (c *Compiler) optimizeIteration(arrayType Type, length int) IterPattern {
    if length <= 255 && length > 0 {
        return DJNZPattern  // Use DJNZ for small arrays
    } else if length > 255 {
        return Counter16Pattern  // Use 16-bit counter
    } else {
        return RuntimeDetection  // Dynamic length detection
    }
}
```

### Element Size Optimization
```go
func (c *Compiler) getAdvancementCode(elementSize int) string {
    switch elementSize {
    case 1:  // u8, char
        return "INC HL"
    case 2:  // u16, *T
        return "INC HL\nINC HL"
    case 4:  // u32 (future)
        return "LD DE, 4\nADD HL, DE"
    default: // Variable size
        return fmt.Sprintf("LD DE, %d\nADD HL, DE", elementSize)
    }
}
```

### Fusion Optimization Rules
```go
// Iterator chain fusion rules
type FusionRule struct {
    CanUseSmallPattern bool  // All arrays ≤255 elements
    CanUse16BitPattern bool  // Any arrays >255 elements  
    HasStringOps       bool  // Contains string operations
    ElementSizeUniform bool  // All same element size
}

func (r *FusionRule) SelectOptimalPattern() IterPattern {
    if r.CanUseSmallPattern && r.ElementSizeUniform {
        return DJNZFusedPattern  // Optimal fusion
    } else if r.CanUse16BitPattern {
        return Counter16FusedPattern  // Good fusion
    } else {
        return HybridFusedPattern  // Mixed optimization
    }
}
```

---

## 6. REAL-WORLD EXAMPLES

### Small Array Processing (Perfect for DJNZ)
```minz
// Game inventory (≤255 items)
let inventory: [Item; 64] = init_inventory();

inventory.filter!(|item| item.count > 0)     // Remove empty slots
         .map!(|item| item.update_durability()) // Update durability
         .forEach(|item| draw_icon(item));      // Draw UI

// Compiles to single DJNZ loop!
```

### Large Array Processing (16-bit counters)
```minz
// Particle system (>255 particles)
let particles: [Particle; 1000] = init_particles();

particles.filter!(|p| p.active)               // Remove inactive
         .map!(|p| p.update_physics(dt))       // Update physics
         .filter!(|p| p.in_bounds(world))      // Bounds check
         .forEach(|p| draw_particle(p));       // Render

// Compiles to optimized 16-bit counter loop!
```

### String Processing (Auto-optimized)
```minz
// Text processing (length auto-detected)
let message: *u8 = get_user_input();  // Could be any length

message.chars()                        // Character iterator
       .filter(|c| c.is_alphanumeric()) // Only letters/numbers
       .map(|c| c.to_uppercase())      // Convert to uppercase
       .forEach(|c| print_char(c));    // Display result

// Automatically uses DJNZ for short strings, 16-bit for long strings!
```

### Mixed Data Processing
```minz
// Game entities with different sizes
let small_enemies: [Enemy; 32] = load_enemies();      // DJNZ
let large_bullets: [Bullet; 512] = load_bullets();    // 16-bit  
let player_name: *u8 = get_player_name();             // Auto

// All use optimal iteration patterns automatically!
small_enemies.forEach(|e| e.update_ai());
large_bullets.forEach(|b| b.update_physics());
player_name.chars().forEach(|c| validate_char(c));
```

---

## 7. PERFORMANCE ANALYSIS

### Iteration Performance by Type

| Array Type | Length | Pattern | T-States/Element | Memory Usage |
|------------|--------|---------|------------------|--------------|
| u8 array | ≤255 | DJNZ | 18-25 | Minimal |
| u16 array | >255 | 16-bit counter | 25-35 | Low |
| u8 string | ≤254 chars | DJNZ | 18-25 | Length + data |
| u16 string | ≥255 chars | 16-bit counter | 25-35 | 3 + data |
| Mixed | Variable | Hybrid | 20-30 avg | Adaptive |

### Memory Layout Efficiency
```
// Optimal string layouts
Small string: [ 6 ][ H e l l o ! ]           (7 bytes total)
Large string: [ 255 ][ 44 1 ][ ... 300 chars ... ]  (304 bytes total)

// Array layouts  
u8 array: [ data... ]                        (n bytes)
u16 array: [ data... ]                       (n*2 bytes)
```

---

## 8. IMPLEMENTATION ROADMAP

### Phase 1: String Iterator Enhancement ✨
- [ ] Implement u16 string length support
- [ ] Add string length auto-detection
- [ ] Create string character iterator
- [ ] Test with both small and large strings

### Phase 2: Array Size Detection 🔍
- [ ] Add compile-time array size analysis
- [ ] Implement DJNZ vs 16-bit pattern selection
- [ ] Create fusion rules for mixed sizes
- [ ] Optimize element advancement

### Phase 3: Unified Iterator Interface 🚀
- [ ] Create generic Iterator[T] trait
- [ ] Implement size-aware implementations
- [ ] Add automatic pattern selection
- [ ] Test performance across all types

### Phase 4: Advanced Optimizations ⚡
- [ ] Cross-array fusion optimization
- [ ] String-specific optimizations
- [ ] Memory layout improvements
- [ ] Benchmark against hand-written assembly

---

## 9. TESTING STRATEGY

### Comprehensive Test Suite
```minz
// Test all iteration patterns
fun test_small_arrays() {
    let arr: [u8; 10] = [1,2,3,4,5,6,7,8,9,10];
    arr.forEach(|x| assert(x > 0));  // Should use DJNZ
}

fun test_large_arrays() {
    let arr: [u8; 500] = init_large_array();
    arr.forEach(|x| process(x));     // Should use 16-bit counter
}

fun test_strings() {
    let short: *u8 = "Hello!";       // Should use DJNZ
    let long: *u8 = create_long_string(300); // Should use 16-bit
    
    short.chars().forEach(|c| validate(c));
    long.chars().forEach(|c| validate(c));
}

fun test_mixed_iteration() {
    // Test arrays of different sizes in same program
    test_small_arrays();
    test_large_arrays(); 
    test_strings();
}
```

### Performance Verification
```bash
# Verify optimal patterns are used
./minzc test_iteration.minz -o test.a80 --verbose-optimization
grep "DJNZ\|DEC BC" test.a80  # Should show appropriate patterns
```

---

## 🎯 THE RESULT: UNIFIED, OPTIMAL ITERATION

This comprehensive system gives us:

1. **Automatic optimization** - Compiler selects best pattern
2. **Universal interface** - Same `.forEach()` works on everything  
3. **Maximum performance** - Each data type gets optimal code
4. **String power** - Both u8 and u16 length strings supported
5. **Future-proof** - Easy to add new data types

**MinZ becomes the ONLY language with zero-cost iteration across all data types on 8-bit hardware!** 🚀