# MinZ Game Jam: Snake & Tetris for ZX Spectrum

## Executive Summary
Create two classic games (Snake and Tetris) in MinZ for ZX Spectrum, using attribute-based graphics for simplicity. This will showcase MinZ capabilities and find real-world issues.

## Why Attribute-Based Graphics?
- **Simpler:** 32x24 grid instead of pixel manipulation
- **Faster:** One byte sets color for 8x8 pixel block
- **Classic:** Many Spectrum games used this technique
- **Memory:** Only 768 bytes for entire screen state

## Game 1: Snake

### Core Mechanics
```minz
// Attribute memory layout for ZX Spectrum
const ATTR_START: u16 = 22528;  // 0x5800
const ATTR_WIDTH: u8 = 32;
const ATTR_HEIGHT: u8 = 24;

// Colors (PAPER * 8 + INK + BRIGHT * 64)
const COLOR_EMPTY: u8 = 0x00;   // Black on black
const COLOR_SNAKE: u8 = 0x42;   // Green on black, bright
const COLOR_FOOD: u8 = 0x46;    // Yellow on black, bright
const COLOR_WALL: u8 = 0x47;    // White on black, bright

struct Snake {
    head_x: u8,
    head_y: u8,
    tail: [256]u8,  // Packed x,y pairs
    length: u8,
    direction: u8   // 0=up, 1=right, 2=down, 3=left
}

global snake: Snake;
global food_x: u8;
global food_y: u8;
global score: u16;

fun set_attr(x: u8, y: u8, color: u8) -> void {
    let addr = ATTR_START + (y * ATTR_WIDTH) + x;
    @poke(addr, color);
}

fun draw_snake() -> void {
    // Use metaprogramming to unroll
    @minz[[[
        for i in 0..8 {
            @emit("if snake.length > " + i + " {")
            @emit("    let pos = snake.tail[" + i + "];")
            @emit("    set_attr(pos >> 4, pos & 0x0F, COLOR_SNAKE);")
            @emit("}")
        }
    ]]]
}

fun game_loop() -> void {
    while true {
        read_input();
        move_snake();
        check_collision();
        check_food();
        draw_screen();
        delay(snake_speed());
    }
}
```

### Input Handling
```minz
// Sinclair joystick (67890)
fun read_input() -> void {
    let keys = @peek(0x7FFE);  // Read keyboard row
    
    if keys & 0x01 == 0 { snake.direction = 3; }  // 0 - left
    if keys & 0x02 == 0 { snake.direction = 1; }  // 9 - right
    if keys & 0x04 == 0 { snake.direction = 2; }  // 8 - down
    if keys & 0x08 == 0 { snake.direction = 0; }  // 7 - up
}
```

### Optimizations with SMC
```minz
@smc fun draw_optimized() -> void {
    // Self-modify the color value
    @smc_patch(color_byte, current_color);
    
    for addr in ATTR_START..ATTR_START+768 {
        asm {
            LD HL, addr
color_byte:
            LD (HL), 0  // 0 gets patched
        }
    }
}
```

## Game 2: Tetris

### Core Data Structures
```minz
// Tetromino pieces (I, O, T, S, Z, J, L)
const PIECES: [[4][4]u8; 7] = [
    // I-piece
    [[0,0,0,0],
     [1,1,1,1],
     [0,0,0,0],
     [0,0,0,0]],
    // ... other pieces
];

struct Tetris {
    board: [24][10]u8,  // Play area is 10 wide
    current_piece: u8,
    current_x: i8,
    current_y: i8,
    current_rotation: u8,
    lines_cleared: u16,
    level: u8
}

global tetris: Tetris;
```

### Rendering with Attributes
```minz
fun draw_board() -> void {
    // Center the 10-wide play area on 32-wide screen
    const OFFSET_X: u8 = 11;
    
    for y in 0..24 {
        for x in 0..10 {
            let color = piece_to_color(tetris.board[y][x]);
            set_attr(OFFSET_X + x, y, color);
        }
    }
}

// Use different bright colors for each piece type
fun piece_to_color(piece: u8) -> u8 {
    match piece {
        0 => 0x00,  // Empty - black
        1 => 0x41,  // I - red
        2 => 0x42,  // O - green  
        3 => 0x43,  // T - cyan
        4 => 0x44,  // S - magenta
        5 => 0x45,  // Z - blue
        6 => 0x46,  // J - yellow
        7 => 0x47,  // L - white
        _ => 0x00
    }
}
```

### Line Clearing with Zero-Cost Iterators
```minz
fun check_lines() -> void {
    let lines_to_clear = (0..24)
        .filter(|y| row_complete(y))
        .collect();
    
    lines_to_clear
        .iter()
        .forEach(clear_line);
    
    tetris.lines_cleared += lines_to_clear.len();
}

fun row_complete(y: u8) -> bool {
    return (0..10)
        .map(|x| tetris.board[y][x])
        .all(|cell| cell != 0);
}
```

### Rotation System
```minz
// Use metaprogramming to generate rotation tables
@define(piece_name, rotations)[[[
    const {0}_ROTATIONS: [[4][4]u8; 4] = {1};
    
    fun rotate_{0}(rotation: u8) -> [[4]u8; 4] {
        return {0}_ROTATIONS[rotation & 0x03];
    }
]]]

@define("I_PIECE", "[...]")  // Define all rotations
@define("T_PIECE", "[...]")
// ... etc
```

## Common Library

### ZX Spectrum Helpers
```minz
// Create a module for Spectrum-specific functions
// File: stdlib/zx_game.minz

pub fun cls_attributes() -> void {
    // Clear attribute area (faster than pixel clear)
    asm {
        LD HL, 22528
        LD DE, 22529
        LD BC, 767
        LD (HL), 0
        LDIR
    }
}

pub fun border(color: u8) -> void {
    asm {
        LD A, color
        OUT (0xFE), A
    }
}

pub fun beep(duration: u16, pitch: u16) -> void {
    // Use ROM routine
    asm {
        LD HL, duration
        LD DE, pitch
        CALL 0x03B5  // ROM BEEPER routine
    }
}

// Fast random number generator
global lfsr: u16 = 0xACE1;
pub fun random() -> u8 {
    // Galois LFSR
    let bit = ((lfsr >> 0) ^ (lfsr >> 2) ^ (lfsr >> 3) ^ (lfsr >> 5)) & 1;
    lfsr = (lfsr >> 1) | (bit << 15);
    return lfsr & 0xFF;
}
```

## Technical Challenges to Solve

### 1. Keyboard Input
- Multiple key detection
- Key repeat handling
- Debouncing

### 2. Timing
- Consistent frame rate
- Difficulty progression
- Smooth animation

### 3. Memory Layout
- Keep game state in fast memory
- Use SMC for hot paths
- Minimize stack usage

### 4. Performance Targets
- 50 FPS minimum (PAL)
- Instant response to input
- No visible tearing

## Development Phases

### Phase 1: Core Libraries
1. Spectrum attribute graphics module
2. Input handling module
3. Sound/beep module
4. Random number generation

### Phase 2: Snake Implementation
1. Basic movement and collision
2. Food generation and scoring
3. Increasing difficulty
4. Game over and restart
5. High score tracking

### Phase 3: Tetris Implementation
1. Piece movement and rotation
2. Collision detection
3. Line clearing
4. Scoring and levels
5. Next piece preview

### Phase 4: Polish
1. Title screens with @minz-generated art
2. Sound effects
3. High score persistence (to tape?)
4. Two-player modes?

## Expected Outcomes

### MinZ Language Benefits
- **Prove zero-cost abstractions** work in real games
- **Test metaprogramming** for game logic
- **Validate SMC optimization** for rendering
- **Find parser bugs** with complex real code
- **Benchmark performance** against hand-coded assembly

### Issues We'll Likely Find
1. Parser struggles with complex expressions
2. Missing stdlib functions we need
3. Optimization opportunities
4. Memory management patterns
5. Module system limitations

## Success Metrics

### Performance
- Snake: Smooth at 50 FPS with 100+ segments
- Tetris: No lag even at level 20
- Both: Under 16KB total size

### Code Quality
- Clean, readable MinZ code
- Good use of language features
- Demonstrates best practices
- Easily portable to other platforms

### Community Impact
- Playable games people want to try
- Source code others can learn from
- Showcase for MinZ capabilities
- Attract contributors to the project

## Resources Needed

### Development Tools
- ZX Spectrum emulator (Fuse, ZEsarUX)
- Real hardware for testing (if available)
- Tape/snapshot creation tools

### Documentation
- Spectrum memory map
- Attribute byte format
- ROM routines documentation
- Keyboard matrix layout

## Timeline

### Week 1-2: Core Libraries
- Get basic Spectrum functions working
- Test attribute graphics
- Implement input handling

### Week 3-4: Snake
- Core gameplay
- Polish and optimization

### Week 5-6: Tetris
- Core gameplay
- Polish and optimization

### Week 7-8: Final Polish
- Title screens
- Documentation
- Release packages

## Code Repository Structure
```
games/
├── lib/
│   ├── zx_graphics.minz
│   ├── zx_input.minz
│   └── zx_sound.minz
├── snake/
│   ├── main.minz
│   ├── game.minz
│   └── Makefile
├── tetris/
│   ├── main.minz
│   ├── pieces.minz
│   ├── board.minz
│   └── Makefile
└── README.md
```

## Next Steps

1. **Fix tree-sitter parser** to handle game code
2. **Create graphics library** for attributes
3. **Start with Snake** (simpler game)
4. **Document everything** as we go
5. **Make it fun!**

This will be the ultimate test of MinZ - if we can make smooth, playable games that rival hand-coded assembly, we've truly achieved our goal of zero-cost abstractions!