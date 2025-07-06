# MinZ Programming Language

MinZ is a minimal systems programming language designed for Z80-based computers, particularly the ZX Spectrum. It provides a modern, type-safe syntax while compiling to efficient Z80 assembly code.

## Features

- **Modern Syntax**: Rust-inspired syntax with type inference
- **Type Safety**: Static typing with compile-time checks
- **Structured Types**: Structs and enums for organized data
- **Low-Level Control**: Direct memory access and inline assembly
- **Z80 Optimized**: Generates efficient Z80 assembly code
- **Error Handling**: Built-in error types and propagation
- **Metaprogramming**: Compile-time evaluation and code generation

## Language Overview

### Basic Types
- `u8`, `u16`: Unsigned integers (8-bit, 16-bit)
- `i8`, `i16`: Signed integers (8-bit, 16-bit)
- `bool`: Boolean type
- `void`: No return value
- Arrays: `[T; N]` where T is element type, N is size
- Pointers: `*T`, `*mut T`

### Example Programs

#### Hello World
```minz
fn main() -> void {
    // Simple function that returns
    let x: u8 = 42;
}
```

#### Arithmetic Operations
```minz
fn calculate(a: u8, b: u8) -> u16 {
    let sum: u16 = a + b;
    let product: u16 = a * b;
    return sum + product;
}

fn main() -> void {
    let result = calculate(5, 10);
}
```

#### Control Flow
```minz
fn max(a: i16, b: i16) -> i16 {
    if a > b {
        return a;
    } else {
        return b;
    }
}

fn count_to_ten() -> void {
    let mut i: u8 = 0;
    while i < 10 {
        i = i + 1;
    }
}
```

#### Arrays and Pointers
```minz
fn sum_array(arr: *u8, len: u8) -> u16 {
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

fn move_player(player: *mut Player, dx: i16, dy: i16) -> void {
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

fn turn_right(dir: Direction) -> Direction {
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
fn set_border_color(color: u8) -> void {
    asm("
        ld a, {0}
        out ($fe), a
    " : : "r"(color));
}
```

## Compiler (minzc)

The MinZ compiler (`minzc`) translates MinZ source code to Z80 assembly in sjasmplus `.a80` format.

### Installation

```bash
cd minzc
make build
```

### Usage

```bash
# Compile a MinZ file to Z80 assembly
./minzc program.minz

# Specify output file
./minzc program.minz -o output.a80

# Enable optimizations
./minzc program.minz -O

# Enable debug output
./minzc program.minz -d
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
fn add(a: u8, b: u8) -> u8 {
    return a + b;
}

// Public function (can be exported)
pub fn get_version() -> u8 {
    return 1;
}

// Multiple return values
fn divmod(n: u16, d: u16) -> (u16, u16) {
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

## Contributing

Contributions are welcome! Please see the [COMPILER_ARCHITECTURE.md](COMPILER_ARCHITECTURE.md) file for details on the compiler's internal structure.

## License

MinZ is released under the MIT License. See LICENSE file for details.

## Roadmap

- [x] Struct support
- [x] Enum types
- [ ] Module system
- [ ] Standard library
- [ ] Optimization passes
- [ ] Alternative register set support (EXX, EX AF,AF')
- [ ] Metaprogramming and compile-time evaluation
- [ ] Debugger support
- [ ] VS Code extension
- [ ] Package manager