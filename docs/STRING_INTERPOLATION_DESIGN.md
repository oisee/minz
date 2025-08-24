# MinZ String Interpolation Design - All Three Approaches! 🎉

## Core Concept: Unified @to_string System

All string interpolation is syntax sugar over a core `@to_string` metafunction that executes at compile-time via CTIE.

## The Three Approaches (User Choice!)

### 1. Ruby-Style `#{}` (Your Favorite!)
```minz
let name = "Alice";
let age = 42;
let msg = "Hello #{name}, you are #{age} years old!";
// Transforms to: @to_string("Hello {name}, you are {age} years old!")
```

### 2. Python-Style `f""` Strings
```minz
let name = "Alice";
let age = 42;
let msg = f"Hello {name}, you are {age} years old!";
// Transforms to: @to_string("Hello {name}, you are {age} years old!")
```

### 3. Explicit `@to_string()` Call
```minz
let name = "Alice";
let age = 42;
let msg = @to_string("Hello {name}, you are {age} years old!");
// Direct call - no transformation needed
```

## Implementation Strategy

### Phase 1: Lexer/Parser Recognition
```go
// In lexer - detect special string patterns
func (l *Lexer) scanString() {
    if l.peek() == '#' && l.peekNext() == '{' {
        // Ruby-style interpolation detected
        return l.scanInterpolatedString("ruby")
    }
}

// In parser - handle f-strings
if token == "f" && next == STRING {
    return parseInterpolatedString("python")
}
```

### Phase 2: AST Transformation
All three syntaxes transform to the same AST:
```go
type InterpolatedString struct {
    Parts []InterpolationPart
}

type InterpolationPart struct {
    Type  string // "literal" or "expression"
    Value string // "Hello " or "name"
}
```

### Phase 3: Semantic Analysis
Transform to @to_string metafunction call:
```go
// Ruby: "Hello #{name}" 
// Python: f"Hello {name}"
// Both become:
MetafunctionCall{
    Name: "@to_string",
    Template: "Hello {name}",
    Context: currentScope
}
```

### Phase 4: CTIE Execution
For compile-time known values:
```minz
const NAME = "Alice";
let msg = "Hello #{NAME}";  // Becomes "Hello Alice" at compile-time!
```

For runtime values:
```minz
let name = get_user_name();  // Runtime value
let msg = "Hello #{name}";   // Generates efficient concat code
```

## Complex Type Support via Stringable Interface

```minz
interface Stringable {
    fun to_string(self) -> str;
}

struct Point {
    x: u8,
    y: u8
}

impl Stringable for Point {
    fun to_string(self) -> str {
        // Recursive interpolation!
        return "(#{self.x}, #{self.y})";
    }
}

fun main() -> void {
    let p = Point { x: 10, y: 20 };
    
    // All three work!
    let msg1 = "Point: #{p}";           // Ruby style
    let msg2 = f"Point: {p}";           // Python style  
    let msg3 = @to_string("Point: {p}"); // Explicit
    
    // All compile to the same efficient code!
}
```

## Compile-Time Optimization Examples

### Simple Variable
```minz
const PI = 3;
let msg = "Pi is approximately #{PI}";
// Compile-time result: "Pi is approximately 3"
// Assembly: Just loads address of static string!
```

### Complex Expression (CTIE)
```minz
fun square(x: u8) -> u8 { return x * x; }

const VAL = 5;
let msg = "5 squared is #{square(VAL)}";
// CTIE executes square(5) at compile-time
// Result: "5 squared is 25"
```

### Mixed Runtime/Compile-Time
```minz
const PREFIX = "User";
let id = get_user_id();  // Runtime
let msg = "#{PREFIX} ID: #{id}";
// Partial compile-time optimization
// "User ID: " is static, only id is dynamic
```

## Z80 Code Generation

### Compile-Time Known String
```minz
const NAME = "Alice";
let msg = "Hello #{NAME}!";
```
Generates:
```asm
str_hello:
    DB 12, "Hello Alice!"  ; Length-prefixed
    
; Usage - just load address!
LD HL, str_hello
```

### Runtime Interpolation
```minz
let name = get_name();
let msg = "Hello #{name}!";
```
Generates efficient concat:
```asm
; Static parts
str_part1: DB 6, "Hello "
str_part2: DB 1, "!"

; Build at runtime
LD HL, buffer
CALL copy_str_part1
CALL copy_name
CALL copy_str_part2
```

## Benefits

1. **Developer Choice** - Use your favorite syntax!
2. **Zero-Cost** - Compile-time when possible
3. **Type-Safe** - Compiler validates all interpolations
4. **Extensible** - User types via Stringable
5. **SMC-Friendly** - Can patch string addresses
6. **CTIE-Powered** - Maximum optimization

## Implementation Priority

1. **Core @to_string** - The foundation
2. **Ruby #{} syntax** - Your preference! 
3. **Python f"" syntax** - Popular alternative
4. **Stringable interface** - For complex types
5. **CTIE optimization** - Make it blazing fast

## Example Test Suite

```minz
// test_interpolation.minz
fun test_all_styles() -> void {
    let name = "MinZ";
    let version = 15;
    
    // All three produce identical output!
    let ruby_style = "Hello from #{name} v0.#{version}!";
    let python_style = f"Hello from {name} v0.{version}!";
    let explicit = @to_string("Hello from {name} v0.{version}!");
    
    @assert(ruby_style == python_style);
    @assert(python_style == explicit);
    
    @print("✅ All interpolation styles work!");
}
```

This gives developers maximum flexibility while maintaining MinZ's zero-cost abstraction philosophy! 🚀