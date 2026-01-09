# MinZ Vision: Universal Retro-Computing Platform

## The Big Picture

MinZ is evolving from a "Z80 compiler" into a **universal retro-computing platform** that bridges:
- **Vintage hardware** (Z80, 6502, eZ80)
- **Modern development** (VS Code, DAP debugging, hot reload)
- **Virtual execution** (MIR VM with pluggable platforms)

```
┌─────────────────────────────────────────────────────────────────┐
│                     MinZ Ecosystem                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   MinZ Source (.minz)                                           │
│        │                                                        │
│        ↓                                                        │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │                    MIR (Portable IR)                    │   │
│   └───────────────────────────┬─────────────────────────────┘   │
│                               │                                 │
│         ┌─────────────────────┼─────────────────────┐           │
│         ↓                     ↓                     ↓           │
│   ┌──────────┐         ┌──────────┐          ┌──────────┐       │
│   │  Native  │         │  MIR VM  │          │  Modern  │       │
│   │ Backends │         │ Runtime  │          │ Backends │       │
│   ├──────────┤         ├──────────┤          ├──────────┤       │
│   │ Z80      │         │ Spectrum │          │ WASM     │       │
│   │ eZ80     │         │ Agon     │          │ Crystal  │       │
│   │ 6502     │         │ C64      │          │ LLVM     │       │
│   │ 68000    │         │ Headless │          │ C        │       │
│   └──────────┘         └──────────┘          └──────────┘       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Core Vision Components

### 1. MZV: The Universal MIR Virtual Machine

MZV executes MIR code on any host platform with pluggable virtual hardware:

```bash
# Run with ZX Spectrum emulation
mzv game.mir --platform spectrum

# Run with Agon Light VDP
mzv game.mir --platform agon

# Run headless for testing
mzv game.mir --platform headless

# Run with VS Code debugging
mzv game.mir --platform spectrum --dap
```

#### Platform Abstraction

```go
type Platform interface {
    // I/O (Z80-style ports)
    PortIn(port uint16) byte
    PortOut(port uint16, value byte)

    // Display
    HasDisplay() bool
    Display() Display

    // Terminal
    ReadChar() (byte, bool)
    WriteChar(b byte)

    // System
    Exit(code int)
    Tick(cycles int)
}
```

#### Predefined Platforms

| Platform | CPU Mode | Display | Audio | Use Case |
|----------|----------|---------|-------|----------|
| `headless` | Generic | None | None | CI testing, scripting |
| `terminal` | Generic | 80x25 text | Beep | CLI apps |
| `spectrum` | Z80 | 256x192 attr | Beeper | ZX games |
| `spectrum-next` | Z80N | 320x256 | AY-3-8912 | Enhanced ZX |
| `agon` | eZ80 | 640x480 VDP | PSG | Modern retro |
| `c64` | 6502 | 320x200 | SID | Commodore |
| `nes` | 6502 | 256x240 | APU | Nintendo |

### 2. VDP: Video Display Processor

Inspired by the Agon Light's VDP, MinZ supports command-based graphics:

```minz
// VDU commands (BBC Micro / Agon style)
fun draw_line(x1: i16, y1: i16, x2: i16, y2: i16) -> void {
    // VDU 25,5,x1;y1;  (move to)
    // VDU 25,5,x2;y2;  (draw to)
    vdu_plot(5, x1, y1);  // Move
    vdu_plot(5, x2, y2);  // Draw
}

// Sprite system
fun setup_sprite(id: u8, bitmap: &[u8]) -> void {
    vdu_select_sprite(id);
    vdu_define_sprite(16, 16, bitmap);
    vdu_show_sprite(id);
}

fun move_sprite(id: u8, x: i16, y: i16) -> void {
    vdu_select_sprite(id);
    vdu_move_sprite(x, y);
}
```

#### VDU Command Reference

| Command | Description | Parameters |
|---------|-------------|------------|
| VDU 22,n | Set screen mode | n = mode (0-7) |
| VDU 25,k,x;y; | PLOT command | k=type, x/y=coords |
| VDU 23,0,n,... | System commands | Sprites, cursors |
| VDU 23,27,n,... | Sprite commands | Define, move, show |
| VDU 29,x;y; | Set graphics origin | x/y = origin |

### 3. DAP: Debug Adapter Protocol

Full VS Code integration for debugging MIR code:

```json
// .vscode/launch.json
{
    "version": "0.2.0",
    "configurations": [{
        "type": "minz",
        "request": "launch",
        "name": "Debug MIR",
        "program": "${workspaceFolder}/game.mir",
        "platform": "spectrum",
        "stopOnEntry": true
    }]
}
```

#### DAP Features

- **Breakpoints** - Source and address level
- **Stepping** - Step in, over, out
- **Variables** - Register inspection
- **Memory** - Memory view/edit
- **Disassembly** - MIR and native
- **Watch** - Expression evaluation
- **Call Stack** - Full trace
- **SMC Events** - Self-modifying code tracking

### 4. 24-bit Types for eZ80

Native support for eZ80's 24-bit addressing:

```minz
// 16MB address space
let ptr: u24 = 0x100000;  // 1MB mark
let data: [u8; 1000000];  // Million-byte array

// Works seamlessly
data[ptr] = 42;
```

#### Backend Support

| Target | Implementation |
|--------|----------------|
| eZ80 (ADL) | Native 24-bit ops |
| Z80 | Synthesized 16+8 |
| 68000 | Masked 32-bit |
| MIR VM | Native 64-bit |

### 5. Cross-Platform Development Workflow

```
┌───────────────────────────────────────────────────────────┐
│                  Development Cycle                         │
├───────────────────────────────────────────────────────────┤
│                                                           │
│   1. Write code in VS Code                                │
│      └── MinZ extension: syntax, snippets, hover         │
│                                                           │
│   2. Compile to MIR                                       │
│      └── mz game.minz --emit-mir -o game.mir             │
│                                                           │
│   3. Test in MZV with DAP debugging                       │
│      └── F5 in VS Code → spectrum platform               │
│      └── Set breakpoints, inspect variables               │
│                                                           │
│   4. Profile and optimize                                 │
│      └── Cycle-accurate execution                         │
│      └── Hot path identification                          │
│                                                           │
│   5. Deploy to target                                     │
│      └── mz game.minz -b z80 -o game.tap                 │
│      └── mz game.minz -b ez80 -o game.bin                │
│                                                           │
└───────────────────────────────────────────────────────────┘
```

## Future Vision

### Phase 1: Foundation (Current)
- [x] MZV platform abstraction
- [x] 24-bit type design
- [x] I/O port opcodes
- [ ] VDP command processor
- [ ] DAP integration

### Phase 2: Platforms (Q1 2026)
- [ ] Full Agon Light support
- [ ] ZX Spectrum Next (Z80N)
- [ ] C64/VIC-20 platform
- [ ] NES platform

### Phase 3: Advanced Features (Q2 2026)
- [ ] Time-travel debugging
- [ ] Hot code reload
- [ ] JIT compilation for MIR
- [ ] Network play support

### Phase 4: Ecosystem (Q3 2026)
- [ ] Package manager for .mir
- [ ] Asset pipeline (graphics, sound)
- [ ] Game templates
- [ ] Community platform library

## Why This Matters

### For Retro Developers
- Modern tooling (VS Code, debugging)
- Test instantly on host machine
- Deploy to real hardware
- One language, all platforms

### For Educators
- Teach computer architecture
- Visual execution tracing
- Safe sandbox environment
- Step-by-step debugging

### For Preservationists
- Accurate emulation
- Platform documentation
- Code archaeology tools
- Migration utilities

## Technical Foundation

### Already Implemented
- MIR intermediate representation
- Platform interface abstraction
- GenericDisplay framebuffer
- I/O port system
- Syscall mechanism
- 24-bit type definitions

### In Progress
- VDP command processor
- DAP server for MZV
- Agon-specific platform

### Planned
- DZRP for MZV
- Time-travel debugger
- JIT compiler

---

*MinZ: Where vintage meets modern, and creativity knows no bounds.*
