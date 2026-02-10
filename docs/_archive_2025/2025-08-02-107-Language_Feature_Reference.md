# MinZ Language Feature Reference

## Complete Feature List with Examples

### 1. Basic Types
- **Unsigned**: `u8`, `u16`
- **Signed**: `i8`, `i16`  
- **Boolean**: `bool`
- **Type Casting**: `as` operator

```minz
let a: u8 = 255;
let b: u16 = a as u16;  // Zero-extend
let c: bool = true;
```

### 2. Arrays and Pointers
- **Fixed Arrays**: `[T; N]`
- **Pointers**: `*T`, `*mut T`
- **References**: `&expr`
- **Dereference**: `*ptr`

```minz
let arr: [u8; 5] = [1, 2, 3, 4, 5];
let ptr: *u8 = &arr[0];
let value = *ptr;
```

### 3. Functions
- **Basic Functions**: `fun name(params) -> type`
- **SMC Optimization**: Automatic for most functions
- **Void Return**: `-> void`
- **Multiple Parameters**: Up to hardware limits

```minz
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}
```

### 4. Structs and Enums
- **Struct Definition**: Named fields with types
- **Enum Variants**: With or without data
- **Pattern Matching**: Destructuring support

```minz
struct Point { x: u8, y: u8 }
enum Option<T> { Some(T), None }
```

### 5. Control Flow
- **Conditionals**: `if`, `else if`, `else`
- **Loops**: `while`, `for`
- **Loop Control**: `break`, `continue`
- **Pattern Matching**: `match` with guards

```minz
for i in 0..10 {
    if i % 2 == 0 { continue; }
    sum = sum + i;
}
```

### 6. Lambda Expressions ✨
**Zero-cost abstractions - compiles to regular functions!**
```minz
let double = |x: u8| => u8 { x * 2 };
let result = double(5);  // Direct call, no closure overhead!
```

### 7. Error Handling ✨
**Native Z80 carry flag integration - 1 cycle overhead!**
```minz
fun open_file(name: *u8) -> File? {
    let handle = fopen(name)?;  // Propagate on error (RET C)
    return File { handle };     // Success (OR A; RET)
}
```

### 8. Interfaces ✨
**Zero-cost through monomorphization!**
```minz
interface Drawable {
    fun draw(self) -> u8;
}

impl Drawable for Circle {
    fun draw(self) -> u8 { /* direct call! */ }
}
```

### 9. Inline Assembly & @abi
**Seamless integration with existing code!**
```minz
@asm {
    LD A, 42
    OUT (254), A  ; Set border color
}

@abi("register: HL=addr")
extern fun rom_routine(addr: u16) -> void;
```

### 10. Advanced Features

#### Tail Recursion Optimization 🚧
```minz
fun factorial_tail(n: u8, acc: u16) -> u16 {
    if n <= 1 { return acc; }
    return factorial_tail(n - 1, n * acc);  // Becomes loop!
}
```

#### Pattern Matching 🚧
```minz
match msg {
    Message.Move { x, y } => x + y,
    Message.Write(text) => process(text),
    _ => 0,
}
```

#### Multiple Returns (SMC) 📋
```minz
@smc_returns
fun divmod(n: u16, d: u8) -> (u16, u8) {
    return (n / d, n % d);  // Direct write to destinations!
}
```

#### Metaprogramming ✅
```minz
@lua {
    function generate_table()
        -- Compile-time code generation
    end
}

const TABLE: [u8; 256] = @lua(generate_table());
```

## Optimization Features

### TRUE SMC (Self-Modifying Code) ✅
- 3-5x faster function calls
- Parameters patch directly into instructions
- Zero stack overhead for most calls

### Register Allocation ✅
- Hierarchical: Physical → Shadow → Memory
- Full Z80 register set utilization
- Shadow registers for interrupts

### Zero-Cost Abstractions ✅
- Lambdas: Compile to named functions
- Interfaces: Monomorphize to direct calls
- Error handling: Native CPU flags

## Platform Integration

### ZX Spectrum
```minz
import zx.screen;
zx.screen.set_pixel(x, y, color);
```

### CP/M
```minz
import cpm.file;
let file = cpm.file.open("data.txt")?;
```

### MSX
```minz
import msx.vdp;
msx.vdp.set_mode(SCREEN_2);
```

## Compilation Modes

### Standard Optimization
```bash
minzc program.minz -O
```

### TRUE SMC Enabled
```bash
minzc program.minz -O --enable-smc
```

### Debug Mode
```bash
minzc program.minz --debug
```

## Status Legend
- ✅ **Working**: Fully implemented and tested
- 🚧 **In Progress**: Partially implemented
- 📋 **Designed**: Specification complete, implementation pending
- ❌ **Not Started**: Planned for future

---

*MinZ brings modern programming to Z80 systems without compromising performance. Every abstraction is designed to compile away completely, proving that good design and efficiency are not mutually exclusive!*