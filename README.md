# MinZ Programming Language

MinZ is a minimal systems programming language designed for Z80-based computers, particularly the ZX Spectrum. It provides a modern, type-safe syntax while compiling to efficient Z80 assembly code.

## Features

- **Modern Syntax**: Rust-inspired syntax with type inference
- **Type Safety**: Static typing with compile-time checks
- **Structured Types**: Structs and enums for organized data
- **Module System**: Organize code with imports and visibility control
- **Low-Level Control**: Direct memory access and inline assembly
- **Z80 Optimized**: Generates efficient Z80 assembly code
- **Shadow Registers**: Full support for Z80's alternative register set
- **Lua Metaprogramming**: Full Lua interpreter at compile time for code generation
- **Self-Modifying Code**: Advanced optimization using SMC for performance-critical code
- **High-Performance Iterators**: Two specialized modes for array processing with minimal overhead
- **Standard Library**: Built-in modules for common operations

## Language Overview

### Basic Types
- `u8`, `u16`: Unsigned integers (8-bit, 16-bit)
- `i8`, `i16`: Signed integers (8-bit, 16-bit)
- `bool`: Boolean type
- `void`: No return value
- Arrays: `[T; N]` or `[N]T` where T is element type, N is size
- Pointers: `*T`, `*mut T`

### Example Programs

#### Hello World
```minz
fun main() -> void {
    // Simple function that returns
    let x: u8 = 42;
}
```

#### Arithmetic Operations
```minz
fun calculate(a: u8, b: u8) -> u16 {
    let sum: u16 = a + b;
    let product: u16 = a * b;
    return sum + product;
}

fun main() -> void {
    let result = calculate(5, 10);
}
```

#### Control Flow
```minz
fun max(a: i16, b: i16) -> i16 {
    if a > b {
        return a;
    } else {
        return b;
    }
}

fun count_to_ten() -> void {
    let mut i: u8 = 0;
    while i < 10 {
        i = i + 1;
    }
}
```

#### Arrays and Pointers
```minz
fun sum_array(arr: *u8, len: u8) -> u16 {
    let mut sum: u16 = 0;
    let mut i: u8 = 0;
    
    while i < len {
        sum = sum + arr[i];
        i = i + 1;
    }
    
    return sum;
}
```

#### Structs
```minz
struct Point {
    x: i16,
    y: i16,
}

struct Player {
    position: Point,
    health: u8,
    score: u16,
}

fun move_player(player: *mut Player, dx: i16, dy: i16) -> void {
    player.position.x = player.position.x + dx;
    player.position.y = player.position.y + dy;
}
```

#### Enums
```minz
enum Direction {
    North,
    South,
    East,
    West,
}

enum GameState {
    Menu,
    Playing,
    GameOver,
}

fun turn_right(dir: Direction) -> Direction {
    case dir {
        Direction.North => Direction.East,
        Direction.East => Direction.South,
        Direction.South => Direction.West,
        Direction.West => Direction.North,
    }
}
```

#### Inline Assembly
```minz
fun set_border_color(color: u8) -> void {
    asm("
        ld a, {0}
        out ($fe), a
    " : : "r"(color));
}
```

#### High-Performance Iterators

MinZ provides two specialized iterator modes for efficient array processing:

##### INTO Mode - Ultra-Fast Field Access
```minz
struct Particle {
    x: u8,
    y: u8,
    velocity: i8,
}

let particles: [Particle; 100];

fun update_particles() -> void {
    // INTO mode copies each element to a static buffer
    // Fields are accessed with direct memory addressing (7 T-states)
    loop particles into p {
        p.x = p.x + p.velocity;
        p.y = p.y + 1;
        // Modified element is automatically copied back
    }
}
```

##### REF TO Mode - Memory-Efficient Access
```minz
let scores: [u16; 50];

fun calculate_total() -> u16 {
    let mut total: u16 = 0;
    
    // REF TO mode uses pointer access (11 T-states)
    // No copying overhead - ideal for read operations
    loop scores ref to score {
        total = total + score;
    }
    
    return total;
}
```

##### Indexed Iteration
```minz
let enemies: [Enemy; 20];

fun find_boss() -> u8 {
    // Both modes support indexed iteration
    loop enemies indexed to enemy, idx {
        if enemy.type == EnemyType.Boss {
            return idx;
        }
    }
    return 255; // Not found
}
```

#### Modules and Imports
```minz
// math/vector.minz
module math.vector;

pub struct Vec2 {
    x: i16,
    y: i16,
}

pub fun add(a: Vec2, b: Vec2) -> Vec2 {
    return Vec2 { x: a.x + b.x, y: a.y + b.y };
}

// main.minz
import math.vector;
import zx.screen;

fun main() -> void {
    let v1 = vector.Vec2 { x: 10, y: 20 };
    let v2 = vector.Vec2 { x: 5, y: 3 };
    let sum = vector.add(v1, v2);
    
    screen.set_border(screen.BLUE);
}
```

#### Lua Metaprogramming
```minz
// Full Lua interpreter at compile time
@lua[[
    function generate_sine_table()
        local table = {}
        for i = 0, 255 do
            local angle = (i * 2 * math.pi) / 256
            table[i + 1] = math.floor(math.sin(angle) * 127 + 0.5)
        end
        return table
    end
    
    -- Load external data
    function load_sprite(filename)
        local file = io.open(filename, "rb")
        local data = file:read("*all")
        file:close()
        return data
    end
]]

// Use Lua-generated data
const SINE_TABLE: [i8; 256] = @lua(generate_sine_table());

// Generate optimized code
@lua_eval(generate_fast_multiply(10))  // Generates optimal mul by 10

// Conditional compilation
@lua_if(os.getenv("DEBUG") == "1")
const MAX_SPRITES: u8 = 16;
@lua_else
const MAX_SPRITES: u8 = 64;
@lua_endif
```

See [LUA_METAPROGRAMMING.md](LUA_METAPROGRAMMING.md) for the complete guide.

#### Shadow Registers
```minz
// Interrupt handler using shadow registers
@interrupt
@shadow_registers
fun vblank_handler() -> void {
    // Automatically uses EXX and EX AF,AF'
    // No need to save/restore registers manually
    frame_counter = frame_counter + 1;
    update_animations();
}

// Fast operations with shadow registers
@shadow
fun fast_copy(dst: *mut u8, src: *u8, len: u16) -> void {
    // Can use both main and shadow register sets
    // for maximum performance
}
```

## Installation

See [INSTALLATION.md](INSTALLATION.md) for detailed installation instructions.

### Quick Start

```bash
# Clone the repository
git clone https://github.com/minz-lang/minz.git
cd minz

# Install dependencies and build
npm install -g tree-sitter-cli
npm install
tree-sitter generate
cd minzc && make build

# Install VS Code extension
cd ../vscode-minz
npm install && npm run compile
code --install-extension .
```

## Compiler (minzc)

The MinZ compiler (`minzc`) translates MinZ source code to Z80 assembly in sjasmplus `.a80` format.

### Usage

```bash
# Compile a MinZ file to Z80 assembly
minzc program.minz

# Specify output file
minzc program.minz -o output.a80

# Enable optimizations
minzc program.minz -O

# Enable self-modifying code optimization
minzc program.minz -O --enable-smc

# Enable debug output
minzc program.minz -d
```

### Compilation Pipeline

1. **Parsing**: Uses tree-sitter to parse MinZ source into an AST
2. **Semantic Analysis**: Type checking and symbol resolution
3. **IR Generation**: Converts AST to intermediate representation
4. **Code Generation**: Produces optimized Z80 assembly

### Intermediate Representation (IR)

The compiler uses a low-level IR that simplifies optimization and code generation. The IR uses virtual registers and simple operations that map efficiently to Z80 instructions. For example:

```
; MinZ: let x = a + b
r1 = load a
r2 = load b
r3 = r1 + r2
store x, r3
```

See [IR_GUIDE.md](IR_GUIDE.md) for detailed information about the IR design and optimization passes.

### Output Format

The compiler generates Z80 assembly compatible with sjasmplus:

```asm
; MinZ generated code
; Generated: 2024-01-20 15:30:00

    ORG $8000

; Function: main
main:
    PUSH IX
    LD IX, SP
    ; Function body
    LD SP, IX
    POP IX
    RET

    END main
```

## Project Structure

```
minz/
├── grammar.js          # Tree-sitter grammar definition
├── src/               # Tree-sitter parser C code
├── queries/           # Syntax highlighting queries
├── minzc/            # Go compiler implementation
│   ├── cmd/minzc/    # CLI tool
│   ├── pkg/ast/      # Abstract syntax tree
│   ├── pkg/parser/   # Parser using tree-sitter
│   ├── pkg/semantic/ # Type checking & analysis
│   ├── pkg/ir/       # Intermediate representation
│   └── pkg/codegen/  # Z80 code generation
└── examples/         # Example MinZ programs
```

## Building from Source

### Prerequisites

- Node.js and npm (for tree-sitter)
- Go 1.21+ (for the compiler)
- tree-sitter CLI

### Build Steps

```bash
# Install tree-sitter CLI
npm install -g tree-sitter-cli

# Generate parser
npm install
tree-sitter generate

# Build the compiler
cd minzc
make build
```

## Language Specification

### Functions
```minz
// Basic function
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

// Public function (can be exported)
pub fun get_version() -> u8 {
    return 1;
}

// Multiple return values
fun divmod(n: u16, d: u16) -> (u16, u16) {
    return (n / d, n % d);
}
```

### Variables
```minz
// Immutable variable
let x: u8 = 10;

// Mutable variable
let mut counter: u16 = 0;
counter = counter + 1;

// Type inference
let y = 42;  // Inferred as i16
```

### Control Flow
```minz
// If statement
if condition {
    // true branch
} else {
    // false branch
}

// While loop
while condition {
    // loop body
}

// For loop (over ranges)
for i in 0..10 {
    // loop body
}

// Loop with break/continue
loop {
    if done {
        break;
    }
    continue;
}
```

### Memory Management
```minz
// Stack allocation
let arr: [u8; 10];

// Pointer operations
let ptr: *mut u8 = &mut arr[0];
*ptr = 42;

// Inline assembly for direct memory access
asm("ld ({0}), a" : : "r"(0x5800));
```

## Technical Documentation

### Architecture and Implementation

- **[MinZ Compiler Architecture](docs/minz-compiler-architecture.md)** - Detailed guide to the compiler implementation, including register allocation, optimization passes, and Z80-specific features
- **[ZVDB Implementation Guide](docs/zvdb-implementation-guide.md)** - Complete documentation of the Zero-Copy Vector Database implementation in MinZ, showcasing advanced optimization techniques

### Examples and Applications

The `examples/` directory contains practical MinZ programs demonstrating:
- Basic language features and syntax
- Z80-optimized algorithms
- ZVDB vector similarity search implementation
- Register allocation optimization examples
- Interrupt handlers with shadow register usage

## Contributing

Contributions are welcome! Please see the technical documentation above for details on the compiler's internal structure.

## License

MinZ is released under the MIT License. See LICENSE file for details.

## Recent Developments

### Revolutionary SMC-First Architecture (2025)

MinZ has pioneered a **superhuman optimization approach** that treats Self-Modifying Code (SMC) as the primary compilation target, not an afterthought. This revolutionary architecture achieves:

- **54% instruction reduction** - From 28 to 13 instructions for simple functions
- **87% fewer memory accesses** - Direct register usage instead of memory choreography
- **63% faster execution** - ~400 to ~150 T-states for basic operations
- **Zero IX register usage** - Even recursive functions use absolute addressing

### SMC-First Philosophy

Traditional compilers treat parameters as memory locations. MinZ treats them as **embedded instructions**:

```asm
; Traditional approach (wasteful):
LD HL, #0000   ; Load parameter
LD ($F006), HL ; Store to memory
; ... later ...
LD HL, ($F006) ; Load from memory

; MinZ SMC approach (optimal):
add_param_a:
    LD HL, #0000   ; Parameter IS the instruction
    LD D, H        ; Use directly!
    LD E, L
```

### Key Innovations

- **Caller-modified parameters**: Function callers directly modify SMC instruction slots
- **Zero-overhead recursion**: Recursive context saved via LDIR, not IX indexing
- **Direct register usage**: Parameters used at point of load, no memory round-trips
- **Peephole optimization**: Aggressive elimination of store/load pairs

### Technical Documentation

- **[Latest Improvements (2025)](minzc/docs/latest-improvements.md)** - High-performance iterators, modern array syntax, and bug fixes
- **[Iterator Design](minzc/docs/iterator-design.md)** - Deep dive into the memory-optimized iterator implementation
- **[Self-Modifying Code Philosophy](docs/SMC_PHILOSOPHY.md)** - The complete MinZ SMC-first approach
- **[Optimization Guide](examples/ideal/OPTIMIZATION_GUIDE.md)** - Current vs ideal code generation examples
- **[Compiler Architecture](docs/minz-compiler-architecture.md)** - Updated with SMC-first design principles

### Latest Features (2025)

- **✅ High-Performance Iterators** - Two specialized modes (INTO/REF TO) for optimal array processing with minimal overhead
- **✅ Modern Array Syntax** - Support for both `[Type; size]` and `[size]Type` array declarations
- **✅ Indexed Iteration** - Built-in support for element indices in loops
- **✅ Direct Memory Operations** - Buffer-aware field access for ultra-fast struct member updates
- **✅ Enhanced Assignment Parsing** - Fixed critical tokenization issues for reliable code generation

### Previous Features (2024)

- **✅ Advanced Register Allocation** - Lean prologue/epilogue generation that only saves registers actually used by functions
- **✅ Shadow Register Optimization** - Automatic use of Z80 alternative registers (EXX, EX AF,AF') for high-performance code
- **✅ Interrupt Handler Optimization** - Ultra-fast interrupt handlers using shadow registers (16 vs 50+ T-states overhead)
- **✅ Self-Modifying Code (SMC)** - Runtime optimization of frequently accessed constants and parameters
- **✅ ZVDB Implementation** - Complete vector similarity search database optimized for Z80 architecture
- **✅ Register Usage Analysis** - Compile-time tracking of register usage for optimal code generation

### Architecture Highlights

- **Register-aware compilation**: Functions are analyzed for register usage patterns
- **Z80-specific optimizations**: Takes advantage of unique Z80 features like shadow registers
- **Memory-efficient design**: Optimized for 64KB address space with smart paging
- **Performance-critical focus**: Designed for real-time applications and interrupt-driven code

## Roadmap

- [x] Struct support
- [x] Enum types  
- [x] Module system with imports and visibility
- [x] Standard library (std.mem, zx.screen, zx.input)
- [x] Alternative register set support (EXX, EX AF,AF')
- [x] Lua Metaprogramming (full Lua 5.1 at compile time)
- [x] Advanced optimization passes (register allocation, SMC, lean prologue/epilogue)
- [x] Self-modifying code optimization
- [x] ZVDB vector database implementation
- [x] High-performance iterators with INTO/REF TO modes
- [x] Modern array syntax ([Type; size])
- [x] Direct memory operations for struct fields
- [ ] Array element assignment (e.g., arr[i].field = value)
- [ ] Iterator chaining and filtering
- [ ] Inline assembly improvements
- [ ] Advanced memory management
- [ ] Debugger support
- [ ] VS Code extension improvements
- [ ] Package manager