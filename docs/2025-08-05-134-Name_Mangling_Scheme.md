# 134. Name Mangling Scheme for Function Overloading

## The Challenge

Assembly labels must be unique. When we have:
```minz
fun print(value: u8) -> void { ... }
fun print(value: u16) -> void { ... }
fun print(s: *str) -> void { ... }
```

We need to generate unique assembly labels for each.

## Proposed Mangling Scheme

### Basic Format: `name$paramtype1$paramtype2$...`

**IMPORTANT**: Return type is NOT included in the mangled name! Only parameter types are used.

### Type Encoding Rules

| MinZ Type | Mangled Code | Example |
|-----------|--------------|---------|
| `u8`      | `u8`         | `print$u8` |
| `u16`     | `u16`        | `print$u16` |
| `u24`     | `u24`        | `print$u24` |
| `i8`      | `i8`         | `abs$i8` |
| `i16`     | `i16`        | `abs$i16` |
| `bool`    | `b`          | `set$u8$u8$b` |
| `void`    | (omitted)    | - |
| `*T`      | `p_T`        | `print$p_u8` |
| `*str`    | `p_str`      | `print$p_str` |
| `[T; N]`  | `a_T_N`      | `fill$a_u8_10` |

### Examples

```minz
// MinZ function signatures
fun min(a: u8, b: u8) -> u8
fun min(a: u16, b: u16) -> u16
fun print(value: u8) -> void
fun print(value: u16) -> void
fun print(s: *str) -> void
fun copy(dest: *u8, src: *u8, count: u16) -> void
fun set_pixel(x: u8, y: u8, color: u8) -> void
fun process(data: [u8; 16]) -> bool
```

Generated assembly labels:
```asm
min$u8$u8:          ; min(u8, u8)
min$u16$u16:        ; min(u16, u16)
print$u8:           ; print(u8)
print$u16:          ; print(u16)
print$p_str:        ; print(*str)
copy$p_u8$p_u8$u16: ; copy(*u8, *u8, u16)
set_pixel$u8$u8$u8: ; set_pixel(u8, u8, u8)
process$a_u8_16:    ; process([u8; 16])
```

## Special Cases

### 1. Module/Namespace Handling
```minz
// In module "graphics"
fun draw(x: u8, y: u8) -> void

// In module "ui"  
fun draw(x: u8, y: u8) -> void
```

Assembly:
```asm
graphics_draw$u8$u8:
ui_draw$u8$u8:
```

### 2. Methods on Structs
```minz
struct Player {
    x: u8, y: u8
}

impl Player {
    fun move(self, dx: i8, dy: i8) -> void { ... }
}
```

Assembly:
```asm
Player_move$Player$i8$i8:
```

### 3. Interface Methods
```minz
interface Drawable {
    fun draw(self) -> void;
}

impl Drawable for Player {
    fun draw(self) -> void { ... }
}
```

Assembly:
```asm
Player_Drawable_draw$Player:
```

## Alternative Schemes Considered

### 1. Hash-Based (Rejected)
```
print_a7f3d2  ; Hash of signature
```
- ❌ Not human readable
- ❌ Hard to debug

### 2. Sequential Numbering (Rejected)
```
print_1
print_2
print_3
```
- ❌ Order dependent
- ❌ Unstable across compilations

### 3. Full Type Names (Too Long)
```
print_unsigned_8_bit_integer
print_pointer_to_string
```
- ❌ Too verbose
- ❌ Assembly file size bloat

## Implementation in Compiler

```go
// In semantic analyzer
type MangledName struct {
    Original string
    Mangled  string
}

func mangleFunctionName(fn *Function) string {
    parts := []string{fn.Name}
    
    for _, param := range fn.Params {
        parts = append(parts, mangleType(param.Type))
    }
    
    return strings.Join(parts, "$")
}

func mangleType(t Type) string {
    switch t := t.(type) {
    case *BasicType:
        return t.Name  // "u8", "u16", etc.
    case *PointerType:
        return "p_" + mangleType(t.Base)
    case *ArrayType:
        return fmt.Sprintf("a_%s_%d", mangleType(t.Element), t.Size)
    case *StructType:
        return t.Name
    default:
        return "unknown"
    }
}
```

## Benefits of This Scheme

1. **Deterministic** - Same input always produces same output
2. **Readable** - Can understand which function by reading assembly
3. **Unique** - No collisions possible
4. **Debuggable** - Easy to map back to source
5. **Stable** - Order of functions doesn't matter

## Real Example: Standard Library

```minz
// Source
fun print(v: u8) -> void { ... }
fun print(v: u16) -> void { ... }
fun print(s: *str) -> void { ... }

// Usage
let x: u8 = 42;
print(x);         // Calls print$u8
print("Hello");   // Calls print$p_str
```

Generated assembly:
```asm
; Function implementations
print$u8:
    ; Implementation for u8
    call print_u8_internal
    ret

print$u16:
    ; Implementation for u16
    call print_u16_internal
    ret
    
print$p_str:
    ; Implementation for string
    call print_string_internal
    ret

; Call sites
main:
    ld a, 42
    call print$u8      ; Compiler knows to call u8 version
    
    ld hl, hello_str
    call print$p_str   ; Compiler knows to call string version
    ret
```

## Edge Cases Handled

### 1. Const Parameters
```minz
fun process(data: *u8) -> void
fun process(data: *const u8) -> void
```
Mangled as: `process$p_u8` vs `process$p_const_u8`

### 2. Function Pointers
```minz
fun apply(f: fn(u8) -> u8, x: u8) -> u8
```
Mangled as: `apply$fn_u8_u8$u8`

### 3. Nested Types
```minz
fun process(data: *[u8; 16]) -> void
```
Mangled as: `process$p_a_u8_16`

## Conclusion

This mangling scheme provides a clean, readable, and reliable way to support function overloading in MinZ while maintaining compatibility with assembly requirements. The generated names are predictable and debuggable, making development easier.