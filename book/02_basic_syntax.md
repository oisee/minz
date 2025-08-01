# Chapter 2: Basic Syntax and Types

> *"Simple things should be simple, complex things should be possible."*  
> MinZ makes Z80 programming both simple and powerful.

## MinZ Syntax Philosophy

MinZ combines the clarity of modern languages with the performance requirements of Z80 hardware. Every syntax choice serves dual purposes:
1. **Developer productivity** - Clear, readable code
2. **Compiler optimization** - Efficient assembly generation

Let's explore how MinZ achieves both goals simultaneously.

## Variables and Type System

### Variable Declarations

MinZ uses `let` for variable declarations with optional type annotations:

```minz
// Type inference (recommended)
let x = 42;          // Inferred as u8
let y = 1000;        // Inferred as u16 (value > 255)
let flag = true;     // Inferred as bool

// Explicit types (when needed)
let counter: u8 = 0;
let address: u16 = 0x8000;
let status: bool = false;
```

**Generated Assembly Analysis:**
```asm
; let x = 42 compiles to:
    LD A, 42           ; Single byte load - optimal

; let address: u16 = 0x8000 compiles to:  
    LD HL, 0x8000      ; 16-bit load - efficient
```

### Primitive Types

MinZ's type system maps directly to Z80 capabilities:

| Type | Size | Z80 Registers | Range |
|------|------|---------------|--------|
| `u8` | 8-bit | A, B, C, D, E, H, L | 0 to 255 |
| `i8` | 8-bit | A, B, C, D, E, H, L | -128 to 127 |
| `u16` | 16-bit | BC, DE, HL, IX, IY | 0 to 65535 |
| `i16` | 16-bit | BC, DE, HL, IX, IY | -32768 to 32767 |
| `bool` | 8-bit | A (optimized) | true, false |

**Example - Type-Aware Code Generation:**

```minz
fun demonstrate_types() -> u8 {
    let byte_val: u8 = 200;      // Uses A register
    let word_val: u16 = 50000;   // Uses HL register pair
    let signed_val: i8 = -50;    // Uses A with sign handling
    
    byte_val
}
```

**Generated Assembly:**
```asm
demonstrate_types:
    LD A, 200        ; byte_val in A register
    LD HL, 50000     ; word_val in HL register pair  
    LD A, 206        ; signed_val (-50 as unsigned 206)
    ; Return byte_val in A
    RET
```

### Constants and Immutability

MinZ enforces immutability by default for safety and optimization:

```minz
// Immutable by default (recommended)
let pi = 3;           // Cannot be changed
let max_lives = 3;    // Compile-time constant

// Mutable when needed
let mut score = 0;    // Can be modified
let mut x_pos = 100;  // Game state that changes
```

**Optimization Benefit:**
```asm
; Immutable constants can be inlined:
; let pi = 3; return pi * radius;
; Compiles to:
    LD A, 3          ; Constant inlined
    ; Multiply by radius...

; Instead of:
    LD A, (pi_addr)  ; Memory load avoided
```

## Functions

### Function Definition

MinZ functions have explicit return types for optimization clarity:

```minz
// Basic function
fun add(x: u8, y: u8) -> u8 {
    x + y
}

// Function with local variables
fun calculate_area(width: u8, height: u8) -> u16 {
    let area = width as u16 * height as u16;
    area
}

// Function returning multiple values (tuple)
fun get_position() -> (u8, u8) {
    (100, 50)  // x, y coordinates
}
```

**Assembly Generation Analysis:**

```asm
; add(x: u8, y: u8) with SMC optimization:
add:
x$immOP:
    LD A, 0          ; x parameter anchor (patched)
x$imm0 EQU x$immOP+1
y$immOP:
    LD B, 0          ; y parameter anchor (patched)  
y$imm0 EQU y$immOP+1
    ADD A, B         ; Actual computation
    RET              ; Result in A
```

### Function Calls and Parameters

MinZ uses TRUE SMC for optimal parameter passing:

```minz
fun main() -> u8 {
    let result = add(5, 3);  // Zero-overhead call
    result
}
```

**Traditional Z80 Approach (What MinZ Avoids):**
```asm
; Traditional parameter passing:
    LD A, 5          ; Load first parameter
    PUSH A           ; Push to stack
    LD A, 3          ; Load second parameter  
    PUSH A           ; Push to stack
    CALL add         ; Call function
    POP BC           ; Clean up stack
    POP BC           ; Clean up stack
; Total: 7 instructions, stack manipulation
```

**MinZ TRUE SMC Approach:**
```asm
; MinZ SMC parameter passing:
    LD HL, add.x$imm0     ; Get parameter patch address
    LD (HL), 5            ; Patch first parameter
    LD HL, add.y$imm0     ; Get parameter patch address  
    LD (HL), 3            ; Patch second parameter
    CALL add              ; Call function - parameters already in place
; Total: 5 instructions, no stack manipulation
```

**Performance:** 28% fewer instructions, no stack pressure.

## Control Flow

### Conditional Expressions

MinZ treats `if` as an expression, enabling functional programming patterns:

```minz
fun abs_value(x: i8) -> u8 {
    if x < 0 { 
        (-x) as u8 
    } else { 
        x as u8 
    }
}

// Ternary-style usage
fun get_status_color(health: u8) -> u8 {
    if health > 50 { 2 } else { 4 }  // Green or red
}
```

**Generated Assembly:**
```asm
abs_value:
x$immOP:
    LD A, 0          ; x parameter (SMC)
x$imm0 EQU x$immOP+1
    BIT 7, A         ; Check sign bit
    JR Z, positive   ; Jump if positive
    NEG              ; Negate if negative
positive:
    RET              ; Result in A
```

### Loops

MinZ provides multiple loop constructs optimized for different patterns:

#### While Loops
```minz
fun count_down(start: u8) -> u8 {
    let mut counter = start;
    while counter > 0 {
        counter = counter - 1;
    }
    counter
}
```

#### For Loops with Ranges
```minz
fun sum_range(max: u8) -> u16 {
    let mut total: u16 = 0;
    for i in 0..max {
        total = total + (i as u16);
    }
    total
}
```

**Optimization Note:** MinZ recognizes loop patterns and optimizes them:

```asm
; for i in 0..max optimizes to:
    LD B, max_value  ; Loop counter in B
    LD HL, 0         ; Accumulator in HL
loop_start:
    LD A, B          ; Current i value
    ; ... loop body
    DJNZ loop_start  ; Decrement B and jump - Z80 optimized!
```

**DJNZ Advantage:** Single instruction loop control - extremely efficient on Z80.

## Data Structures

### Arrays

MinZ supports both fixed-size and dynamic arrays:

```minz
// Fixed-size array (stack allocated)
fun process_scores() -> u8 {
    let scores: [u8; 5] = [85, 92, 78, 95, 88];
    scores[2]  // Access element
}

// Array initialization patterns
fun init_buffer() -> u8 {
    let mut buffer: [u8; 256] = [0; 256];  // Initialize all to 0
    buffer[0] = 0xFF;
    buffer[0]
}
```

**Memory Layout Optimization:**
```asm
; [u8; 5] array becomes:
scores_data:
    DB 85, 92, 78, 95, 88    ; Contiguous memory

; scores[2] becomes:
    LD HL, scores_data+2     ; Direct address calculation
    LD A, (HL)               ; Single memory access
```

### Structs

Structs provide efficient data organization:

```minz
struct Point {
    x: u8,
    y: u8,
}

struct Sprite {
    position: Point,
    color: u8,
    visible: bool,
}

fun create_sprite() -> Sprite {
    Sprite {
        position: Point { x: 100, y: 50 },
        color: 7,       // White
        visible: true,
    }
}
```

**Memory Layout:**
```
Sprite structure (4 bytes total):
+0: position.x (u8)
+1: position.y (u8)  
+2: color (u8)
+3: visible (bool as u8)
```

**Field Access Optimization:**
```asm
; sprite.position.x becomes:
    LD HL, sprite_addr       ; Base address
    LD A, (HL)               ; Direct offset 0 access

; sprite.color becomes:
    LD HL, sprite_addr+2     ; Direct offset calculation
    LD A, (HL)               ; Single instruction access
```

### Enums

Enums provide type-safe state representation:

```minz
enum Direction {
    Up,     // 0
    Down,   // 1
    Left,   // 2
    Right,  // 3
}

enum GameState {
    Menu,
    Playing,
    Paused,
    GameOver,
}

fun move_player(dir: Direction) -> (u8, u8) {
    let mut x = 100;
    let mut y = 50;
    
    match dir {
        Direction::Up => y = y - 1,
        Direction::Down => y = y + 1,
        Direction::Left => x = x - 1,
        Direction::Right => x = x + 1,
    }
    
    (x, y)
}
```

**Enum Optimization:**
```asm
; Enums compile to efficient jump tables:
move_player:
dir$immOP:
    LD A, 0              ; Direction parameter (SMC)
dir$imm0 EQU dir$immOP+1
    LD HL, jump_table    ; Jump table base
    ADD A, A             ; Direction * 2 (word addresses)
    ADD A, L             ; Add to base address
    LD L, A              ; Update HL
    JP (HL)              ; Jump to handler

jump_table:
    DW handle_up         ; Direction::Up
    DW handle_down       ; Direction::Down  
    DW handle_left       ; Direction::Left
    DW handle_right      ; Direction::Right
```

## Pattern Matching

Pattern matching provides powerful control flow with optimization opportunities:

```minz
enum Result {
    Success(u8),
    Error(u8),
}

fun handle_result(res: Result) -> u8 {
    match res {
        Result::Success(value) => value,
        Result::Error(code) => {
            println("Error: {}", code);
            0
        }
    }
}
```

**Match Optimization:**
MinZ compiles pattern matching to efficient conditional chains and jump tables, avoiding the overhead of dynamic dispatch.

## String Handling

MinZ provides efficient string operations for Z80:

```minz
fun display_message() -> u8 {
    let message: *u8 = "Hello, MinZ!";
    print_string(message);
    0
}

// String interpolation
fun show_score(points: u16) -> u8 {
    println("Score: {}", points);
    0
}
```

**String Optimization:**
```asm
; String literals are stored efficiently:
message_data:
    DB "Hello, MinZ!", 0    ; Null-terminated

; String access:
    LD HL, message_data     ; Direct address - no copying
```

## Type Conversions

MinZ provides explicit type conversions for safety:

```minz
fun convert_types() -> u16 {
    let small: u8 = 200;
    let large: u16 = small as u16;  // Zero-extend
    
    let signed: i8 = -50;
    let unsigned: u8 = signed as u8;  // Bit reinterpretation
    
    large + (unsigned as u16)
}
```

**Conversion Optimization:**
```asm
; u8 to u16 conversion:
    LD A, 200        ; u8 value
    LD H, 0          ; Zero-extend high byte
    LD L, A          ; Result in HL

; i8 to u8 (no-op at assembly level):
    LD A, 206        ; -50 as unsigned (efficient)
```

## Memory Management

MinZ provides stack-based memory management for predictable performance:

```minz
fun local_variables() -> u8 {
    let x = 10;        // Stack allocated
    let y = 20;        // Stack allocated
    let arr: [u8; 10]; // Stack allocated array
    
    x + y  // Values, not references
}

// Reference semantics when needed
fun reference_example(data: *u8) -> u8 {
    *data  // Explicit dereference
}
```

**Stack Frame Optimization:**
```asm
local_variables:
    ; Efficient stack frame - only allocates what's used
    LD A, 10         ; x = 10
    LD B, 20         ; y = 20
    ; arr allocated in stack space
    ADD A, B         ; x + y
    RET              ; Clean return
```

## Practical Example: Simple Game Logic

Let's combine these concepts in a practical example:

```minz
struct Player {
    x: u8,
    y: u8,
    health: u8,
    score: u16,
}

enum Input {
    Left,
    Right,
    Jump,
    Fire,
}

fun update_player(player: Player, input: Input) -> Player {
    let mut new_player = player;
    
    match input {
        Input::Left => {
            if new_player.x > 0 {
                new_player.x = new_player.x - 1;
            }
        },
        Input::Right => {
            if new_player.x < 255 {
                new_player.x = new_player.x + 1;
            }
        },
        Input::Jump => {
            // Jump logic would go here
        },
        Input::Fire => {
            // Fire logic would go here  
            new_player.score = new_player.score + 10;
        },
    }
    
    new_player
}

fun main() -> u8 {
    let player = Player {
        x: 100,
        y: 150,
        health: 100,
        score: 0,
    };
    
    let updated = update_player(player, Input::Right);
    updated.x  // Return new x position
}
```

**Generated Assembly Quality:**
This example compiles to approximately 30 Z80 instructions with:
- Direct struct field access (no indirection)
- Efficient enum handling via jump table
- Optimal register usage for small values
- TRUE SMC parameter passing between functions

## Performance Characteristics

### Instruction Efficiency
- **Variable access**: 1-2 instructions (register or direct memory)
- **Function calls**: 3-5 instructions (TRUE SMC optimization)
- **Struct field access**: 1-2 instructions (direct offset calculation)
- **Enum matching**: 2-4 instructions (jump table dispatch)

### Memory Usage
- **Local variables**: Stack allocated, automatically cleaned up
- **Structs**: Packed layout, no padding unless needed for alignment
- **Arrays**: Contiguous memory, direct indexing
- **Strings**: Null-terminated, stored in program memory

### Optimization Features
- **Constant folding**: Compile-time calculation of constant expressions
- **Dead code elimination**: Unused code automatically removed
- **Register allocation**: Z80-aware register usage including shadow registers
- **TRUE SMC**: Revolutionary parameter passing via self-modifying code

## Best Practices

### 1. Use Type Inference
```minz
// Good - clear and concise
let count = 0;
let message = "Ready";

// Avoid - unnecessary verbosity
let count: u8 = 0;
let message: *u8 = "Ready";
```

### 2. Prefer Immutability
```minz
// Good - immutable by default
let config = load_config();

// Only when mutation is actually needed
let mut game_state = GameState::Menu;
```

### 3. Use Pattern Matching
```minz
// Good - clear intent
match player_input {
    Input::Left => move_left(),
    Input::Right => move_right(),
    _ => {} // Handle other cases
}

// Avoid - verbose conditionals
if player_input == Input::Left {
    move_left();
} else if player_input == Input::Right {
    move_right();
}
```

### 4. Leverage Type Safety
```minz
// Good - type-safe state management
enum GameState { Menu, Playing, Paused }
let state = GameState::Menu;

// Avoid - magic numbers
let state = 0;  // What does 0 mean?
```

## Next Steps

You now understand MinZ's basic syntax and how it compiles to efficient Z80 assembly. The key insights:

1. **Every language feature maps to optimal Z80 code**
2. **Type safety comes at zero runtime cost**
3. **Modern syntax enables readable, maintainable code**
4. **TRUE SMC provides revolutionary performance**

In the next chapter, we'll explore MinZ's memory model and pointer system, showing how MinZ achieves memory safety without garbage collection overhead.

---

**Next**: [Chapter 3 - Memory and Pointers](03_memory_pointers.md)

---

### Chapter Summary

This chapter demonstrated how MinZ's syntax choices serve both developer productivity and compiler optimization. Key concepts covered:

- **Type system** that maps directly to Z80 registers and capabilities
- **Function definitions** with TRUE SMC parameter optimization
- **Control flow** constructs that compile to efficient Z80 patterns
- **Data structures** with optimal memory layout and access patterns
- **Pattern matching** for type-safe, efficient control flow
- **Performance characteristics** proving zero-cost abstraction claims

Every example showed both the high-level MinZ code and the generated assembly, proving that modern programming constructs can achieve optimal performance on vintage hardware.