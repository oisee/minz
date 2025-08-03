# 123: MinZ REPL Design - Interactive Z80 Development

## 🎯 Vision

An interactive REPL (Read-Eval-Print-Loop) for MinZ that compiles and executes Z80 code in real-time, providing instant feedback for learning and experimentation.

## 🏗️ Architecture Overview

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Terminal  │────▶│  MinZ REPL   │────▶│  Emulator   │
│   (Input)   │     │   Engine     │     │  (Z80 Emu)  │
└─────────────┘     └──────────────┘     └─────────────┘
                           │
                    ┌──────▼──────┐
                    │  MinZ       │
                    │  Compiler   │
                    └─────────────┘
```

## 📋 Core Features

### 1. Immediate Mode Execution
```minz
minz> let x: u8 = 42
minz> @print("x = {}\n", x)
x = 42
minz> x = x * 2
minz> @print("x = {}\n", x)
x = 84
```

### 2. Function Definition & Testing
```minz
minz> fun add(a: u8, b: u8) -> u8 { return a + b; }
[Function 'add' compiled to 8 bytes]
minz> add(5, 3)
8
```

### 3. Assembly Inspection
```minz
minz> :asm add
add:
    LD A, (IX+4)    ; Load parameter a
    ADD A, (IX+5)   ; Add parameter b
    RET
```

### 4. Memory Inspection
```minz
minz> :mem 0x8000 32
8000: 21 00 80 DD 21 00 00 DD ...
8010: 39 F5 DD 7E 04 DD 86 05 ...
```

### 5. Performance Profiling
```minz
minz> :profile add(100, 200)
Result: 44 (overflow)
T-states: 47
Memory reads: 2
Memory writes: 0
```

## 🛠️ Implementation Strategy

### Phase 1: Basic REPL Loop (Week 1)
```go
// repl/repl.go
type REPL struct {
    compiler  *Compiler
    emulator  *Z80Emulator
    context   *REPLContext
}

func (r *REPL) Run() {
    for {
        input := r.readInput()
        if r.isCommand(input) {
            r.executeCommand(input)
        } else {
            r.compileAndRun(input)
        }
    }
}
```

### Phase 2: Embedded Z80 Emulator (Week 2)
```go
// emulator/z80.go
type Z80Emulator struct {
    registers Registers
    memory    [65536]byte
    cycles    uint32
}

func (e *Z80Emulator) Execute(code []byte, entry uint16) {
    e.LoadCode(code, entry)
    e.Run()
    e.CaptureOutput()
}
```

### Phase 3: Context Management (Week 3)
```go
// repl/context.go
type REPLContext struct {
    variables  map[string]Variable
    functions  map[string]Function
    codeBase   uint16  // Current code location
    dataBase   uint16  // Current data location
}
```

### Phase 4: Interactive Features (Week 4)
- Tab completion for functions/variables
- History navigation
- Syntax highlighting
- Error recovery

## 🎮 Command Reference

### REPL Commands
| Command | Description | Example |
|---------|-------------|---------|
| `:help` | Show help | `:help` |
| `:asm <func>` | Show assembly | `:asm main` |
| `:mem <addr> <len>` | Show memory | `:mem 0x8000 16` |
| `:profile <expr>` | Profile expression | `:profile fib(10)` |
| `:reset` | Reset emulator | `:reset` |
| `:save <file>` | Save session | `:save session.minz` |
| `:load <file>` | Load file | `:load game.minz` |
| `:reg` | Show registers | `:reg` |
| `:break <addr>` | Set breakpoint | `:break 0x8100` |
| `:step` | Single step | `:step` |

## 🚀 Advanced Features

### 1. Live Code Patching
```minz
minz> fun counter() -> u8 { 
    static count: u8 = 0; 
    count = count + 1; 
    return count; 
}
minz> counter()
1
minz> :patch counter.count 10
minz> counter()
11
```

### 2. SMC Visualization
```minz
minz> :smc on
SMC tracking enabled
minz> let f = make_counter(5)
[SMC] Patched immediate at 0x8042: 0x00 -> 0x05
minz> f()
6
[SMC] Modified at 0x8042: 0x05 -> 0x06
```

### 3. Import System
```minz
minz> import std.mem
[Loaded: std.mem - 5 functions]
minz> std.mem.fill(0x9000, 32, 0xFF)
[Filled 32 bytes at 0x9000]
```

### 4. Graphics Mode (ZX Spectrum)
```minz
minz> :mode graphics
[Switched to graphics mode - 256x192]
minz> plot(128, 96, 7)  // White pixel at center
minz> line(0, 0, 255, 191, 2)  // Red diagonal
```

## 🔧 Technical Requirements

### Dependencies
1. **Z80 Emulator Library**
   - Options: z80ex, libz80, custom implementation
   - Needs: Cycle accuracy, memory callbacks, debugging hooks

2. **Terminal UI**
   - Go: `github.com/charmbracelet/bubbletea`
   - Features: Syntax highlighting, auto-complete

3. **MinZ Compiler Integration**
   - Incremental compilation support
   - Symbol table preservation
   - Error recovery

### Memory Layout
```
0x0000-0x3FFF: ROM (optional, for BASIC compatibility)
0x4000-0x5AFF: Screen memory (graphics mode)
0x5B00-0x7FFF: REPL workspace
0x8000-0xFFFF: User code and data
```

## 📊 Performance Targets

- **Compilation**: < 10ms for single expression
- **Execution**: Real-time for simple code
- **Memory**: < 10MB host memory usage
- **Startup**: < 100ms to interactive prompt

## 🎯 Use Cases

### 1. Learning Z80 Assembly
```minz
minz> :tutorial basics
Welcome to Z80 basics!
Let's start with registers...
```

### 2. Algorithm Development
```minz
minz> fun qsort(arr: *u8, len: u8) -> void { ... }
minz> let data = [5, 2, 8, 1, 9]
minz> qsort(&data, 5)
minz> data
[1, 2, 5, 8, 9]
```

### 3. Hardware Simulation
```minz
minz> :device speaker 0xFE
[Attached speaker to port 0xFE]
minz> out(0xFE, 0x10)
[BEEP]
```

### 4. Game Development
```minz
minz> :sprite load "player.spr"
minz> sprite_draw(100, 50, player)
minz> :keys
[Arrow keys mapped to QAOP]
```

## 🗺️ Implementation Roadmap

### Week 1: Foundation
- [ ] Basic REPL loop
- [ ] Simple expression evaluation
- [ ] Variable storage

### Week 2: Emulation
- [ ] Integrate Z80 emulator
- [ ] Memory management
- [ ] Basic I/O

### Week 3: Compiler Integration
- [ ] Incremental compilation
- [ ] Function definitions
- [ ] Import system

### Week 4: Polish
- [ ] Command system
- [ ] Debugging features
- [ ] Documentation

### Week 5: Advanced Features
- [ ] Graphics support
- [ ] SMC visualization
- [ ] Profiling tools

### Week 6: Testing & Release
- [ ] Comprehensive testing
- [ ] Performance optimization
- [ ] Package for distribution

## 💡 Innovation Opportunities

1. **AI Assistant Integration**
   - Code suggestions
   - Error explanations
   - Optimization hints

2. **Collaborative Mode**
   - Share REPL sessions
   - Real-time collaboration
   - Code broadcasting

3. **Time-Travel Debugging**
   - Record all state changes
   - Rewind execution
   - Replay with modifications

4. **Visual Programming**
   - Block-based mode for beginners
   - Flow diagram generation
   - Live data visualization

## 🎉 Example Session

```
$ minz repl
MinZ REPL v1.0.0 - Interactive Z80 Development
Type :help for commands, Ctrl-D to exit

minz> @print("Hello, Z80!\n")
Hello, Z80!

minz> fun fib(n: u8) -> u16 {
    if n <= 1 { return n; }
    return fib(n-1) + fib(n-2);
}
[Function 'fib' compiled - 42 bytes]

minz> fib(10)
55

minz> :profile fib(10)
Result: 55
T-states: 12,847
Calls: 177
Stack depth: 10

minz> :asm fib | head -5
fib:
    LD A, (IX+4)    ; Load n
    CP 2            ; Compare with 2
    JP NC, .recurse ; Jump if n >= 2
    RET            ; Return n

minz> let msg = "MinZ rocks!"
minz> @print("{}\n", msg)
MinZ rocks!

minz> :save my_session.minz
[Session saved to my_session.minz]

minz> :quit
Goodbye! Happy coding!
```

## 🚀 Conclusion

The MinZ REPL will revolutionize Z80 development by providing:
- **Instant feedback** for learning and experimentation
- **Professional debugging** tools in an interactive environment
- **Modern developer experience** for vintage hardware
- **Zero-cost abstraction** verification in real-time

This tool will make MinZ the most accessible and powerful way to program Z80 systems, bridging the gap between modern development practices and vintage computing constraints.