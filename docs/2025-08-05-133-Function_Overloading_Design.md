# 133. Function Overloading Design for MinZ

## Overview

Function overloading (перегрузка функций) allows multiple functions with the same name but different parameter types. This is a perfect "quick win" for MinZ because:

1. **Simplifies the standard library** - No more `min8`, `min16`, `max8`, `max16`
2. **No runtime overhead** - Resolution happens at compile time
3. **Easier to implement than generics** - Just name mangling
4. **Natural for users** - Common in many languages

## Design Principles

### 1. Compile-Time Resolution
All overload resolution happens during semantic analysis. By code generation, each call knows exactly which function it's calling.

### 2. Name Mangling
Internally, overloaded functions get mangled names:
- `min(u8, u8)` → `min$u8$u8`
- `min(u16, u16)` → `min$u16$u16`
- `print(u8)` → `print$u8`
- `print(*str)` → `print$ptr_str`

### 3. Exact Match Priority
No implicit conversions during overload resolution (keeps it simple):
```minz
fun abs(x: i8) -> u8 { ... }
fun abs(x: i16) -> u16 { ... }

let a: i8 = -5;
let b = abs(a);  // Calls abs(i8) -> u8
```

## Implementation Plan

### Phase 1: Parser Support
Grammar already supports multiple functions with same name. No changes needed!

### Phase 2: Semantic Analysis
1. **Symbol Table Enhancement**
   ```go
   type FunctionOverloadSet struct {
       Name      string
       Functions []FunctionSymbol  // All overloads
   }
   ```

2. **Overload Resolution Algorithm**
   ```go
   func resolveOverload(name string, argTypes []Type) (*FunctionSymbol, error) {
       overloadSet := symbolTable.GetOverloadSet(name)
       
       // Find exact match
       for _, fn := range overloadSet.Functions {
           if matchesExactly(fn.ParamTypes, argTypes) {
               return &fn, nil
           }
       }
       
       return nil, fmt.Errorf("no matching overload for %s", name)
   }
   ```

### Phase 3: Code Generation
Generate mangled names for assembly:
```go
func mangleFunctionName(name string, paramTypes []Type) string {
    parts := []string{name}
    for _, t := range paramTypes {
        parts = append(parts, mangleType(t))
    }
    return strings.Join(parts, "$")
}
```

## Standard Library Impact

### Before (current ugly state):
```minz
// Math functions
fun min8(a: u8, b: u8) -> u8
fun min16(a: u16, b: u16) -> u16
fun max8(a: u8, b: u8) -> u8
fun max16(a: u16, b: u16) -> u16
fun abs8(x: i8) -> u8
fun abs16(x: i16) -> u16

// Print functions
fun print_u8(value: u8) -> void
fun print_u16(value: u16) -> void
fun print_i8(value: i8) -> void
fun print_i16(value: i16) -> void
fun print_string(s: *str) -> void
fun print_char(c: u8) -> void
```

### After (with overloading):
```minz
// Math functions - beautiful!
fun min(a: u8, b: u8) -> u8
fun min(a: u16, b: u16) -> u16
fun min(a: i8, b: i8) -> i8
fun min(a: i16, b: i16) -> i16

fun max(a: u8, b: u8) -> u8
fun max(a: u16, b: u16) -> u16
fun max(a: i8, b: i8) -> i8
fun max(a: i16, b: i16) -> i16

fun abs(x: i8) -> u8
fun abs(x: i16) -> u16

// Print functions - intuitive!
fun print(value: u8) -> void
fun print(value: u16) -> void
fun print(value: i8) -> void
fun print(value: i16) -> void
fun print(s: *str) -> void
fun print(c: char) -> void  // If we add char type

// Memory operations
fun copy(dest: *u8, src: *u8, count: u16) -> void
fun copy(dest: *u16, src: *u16, count: u16) -> void

fun fill(dest: *u8, value: u8, count: u16) -> void
fun fill(dest: *u16, value: u16, count: u16) -> void
```

## Extended Examples

### Option Type Without Generics
```minz
// Instead of OptionU8, OptionU16, OptionPtr...
struct Option {
    tag: u8,
    data: u16  // Can hold u8, u16, or pointer
}

// Overloaded constructors
fun Some(value: u8) -> Option {
    Option { tag: 1, data: value as u16 }
}

fun Some(value: u16) -> Option {
    Option { tag: 2, data: value }
}

fun Some(value: *u8) -> Option {
    Option { tag: 3, data: value as u16 }
}

fun None() -> Option {
    Option { tag: 0, data: 0 }
}

// Overloaded unwrap
fun unwrap(opt: Option) -> u8 {
    if opt.tag != 1 { @panic("unwrap on wrong type"); }
    return opt.data as u8;
}

fun unwrap(opt: Option) -> u16 {
    if opt.tag != 2 { @panic("unwrap on wrong type"); }
    return opt.data;
}

fun unwrap(opt: Option) -> *u8 {
    if opt.tag != 3 { @panic("unwrap on wrong type"); }
    return opt.data as *u8;
}
```

### Collections
```minz
// Fixed-size array operations
fun contains(arr: *u8, len: u16, value: u8) -> bool
fun contains(arr: *u16, len: u16, value: u16) -> bool
fun contains(arr: *i8, len: u16, value: i8) -> bool

fun find(arr: *u8, len: u16, value: u8) -> u16
fun find(arr: *u16, len: u16, value: u16) -> u16

fun sort(arr: *u8, len: u16) -> void
fun sort(arr: *u16, len: u16) -> void
```

### Platform Abstraction
```minz
// Screen operations for different platforms
fun set_pixel(x: u8, y: u8, color: u8) -> void
fun set_pixel(x: u16, y: u16, color: u8) -> void  // For larger screens
fun set_pixel(x: u8, y: u8, r: u8, g: u8, b: u8) -> void  // RGB systems
```

## Overloading Rules

### 1. Parameter Count
Functions can be overloaded on parameter count:
```minz
fun create() -> Entity
fun create(x: u8, y: u8) -> Entity
fun create(x: u8, y: u8, type: u8) -> Entity
```

### 2. Parameter Types
Functions can be overloaded on parameter types:
```minz
fun process(value: u8) -> void
fun process(value: u16) -> void
fun process(value: *Entity) -> void
```

### 3. Const Parameters
Const-ness can differentiate overloads:
```minz
fun get(arr: *u8, index: u16) -> u8
fun get(arr: *const u8, index: u16) -> u8  // Read-only version
```

### 4. Return Type (NOT allowed)
Return type alone cannot differentiate overloads:
```minz
fun convert(x: u8) -> u16  // OK
fun convert(x: u8) -> i16  // ERROR: Already defined
```

## Interface Interaction

Overloading works beautifully with interfaces:
```minz
interface Printable {
    fun print(self) -> void;
}

// Different implementations for different types
impl Printable for Player {
    fun print(self) -> void {
        print(self.x);      // Calls print(u8)
        print(", ");        // Calls print(*str)
        print(self.y);      // Calls print(u8)
    }
}
```

## Assembly Generation

Example of mangled names in generated assembly:
```asm
; Original: fun min(a: u8, b: u8) -> u8
min$u8$u8:
    ld a, (hl)
    inc hl
    cp (hl)
    jr c, .return_a
    ld a, (hl)
.return_a:
    ret

; Original: fun min(a: u16, b: u16) -> u16  
min$u16$u16:
    ; 16-bit comparison logic
    ret

; Call sites get resolved to specific versions
main:
    ; min(x8, y8) where x8,y8 are u8
    call min$u8$u8
    
    ; min(x16, y16) where x16,y16 are u16
    call min$u16$u16
```

## Benefits

1. **Cleaner API** - Users just call `print()`, `min()`, `max()`
2. **Type Safety** - Compiler ensures correct overload is called
3. **No Runtime Cost** - Everything resolved at compile time
4. **Gradual Enhancement** - Can add overloads without breaking existing code
5. **Better IntelliSense** - IDEs can show all variants of a function

## Implementation Priority

1. **Core functions first**: print, min, max, abs
2. **Memory operations**: copy, fill, compare
3. **String operations**: concat, compare, find
4. **Math operations**: sqrt, pow, clamp
5. **Platform abstractions**: set_pixel, get_input

## Conclusion

Function overloading is indeed a "quick win" that will dramatically improve MinZ's usability without adding complexity. It's especially valuable for embedded systems where we want convenience without runtime overhead.