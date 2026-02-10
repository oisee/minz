# Complete I/O Implementation Plan

## Overview

Comprehensive plan to implement all I/O systems for MinZ: File I/O, Sound, Keyboard, and Networking.

## 🎯 Implementation Priority

### Phase 1: Foundation (Week 1-2)
✅ **DONE**: Design all I/O systems
🚧 **IN PROGRESS**: MZE port interceptor framework
⏳ **TODO**: Module import system in compiler

### Phase 2: File I/O (Week 3-4)
- [ ] Implement `@platform` directive
- [ ] Create zx.io module (tape/disk)
- [ ] Create cpm.io module (BDOS)
- [ ] Wire up MZE file interceptors
- [ ] Test with real programs

### Phase 3: Sound (Week 5-6)
- [ ] Integrate Ayumi emulator in MZE
- [ ] Create zx.ay module
- [ ] Create msx.psg module
- [ ] Implement port interceptors
- [ ] Build sound effects library

### Phase 4: Keyboard (Week 7)
- [ ] Enhanced keyboard buffer in MZE
- [ ] Create keyboard modules
- [ ] Terminal integration
- [ ] Game input support

### Phase 5: Networking (Week 8-9)
- [ ] TCP/IP interceptor in MZE
- [ ] Create net.tcp module
- [ ] HTTP client library
- [ ] Test with real servers

## 📦 Module Architecture

```
stdlib/
├── platform/
│   ├── zx/
│   │   ├── io.minz      # File I/O (tape/disk)
│   │   ├── ay.minz      # AY-3-8912 sound
│   │   ├── keyboard.minz # Keyboard matrix
│   │   └── screen.minz   # Screen operations
│   ├── cpm/
│   │   ├── io.minz      # BDOS file operations
│   │   └── console.minz  # Console I/O
│   ├── msx/
│   │   ├── io.minz      # -> ../cpm/io.minz (alias)
│   │   ├── psg.minz     # PSG sound (AY compatible)
│   │   └── vdp.minz     # Video display processor
│   └── generic/
│       ├── keyboard.minz # MZE enhanced keyboard
│       └── network.minz  # TCP/IP networking
├── media/
│   ├── sfx.minz         # Sound effects library
│   └── music.minz       # Music player
└── net/
    ├── tcp.minz         # TCP client/server
    ├── http.minz        # HTTP client
    └── ws.minz          # WebSocket (future)
```

## 🔧 Compiler Changes Required

### 1. Platform Directive Parser

```go
// pkg/parser/platform.go
type PlatformDirective struct {
    Platforms []string
    Body      ast.Node
}

func parsePlatformDirective(p *Parser) *PlatformDirective {
    // @platform("zxspectrum", "zx128") { ... }
}
```

### 2. Module Import Resolution

```go
// pkg/resolver/imports.go
type ImportResolver struct {
    searchPaths []string
    platform    string
    cache       map[string]*Module
}

func (r *ImportResolver) Resolve(import string) *Module {
    // Search in stdlib/platform/{platform}/
    // Then stdlib/
    // Then user paths
}
```

### 3. Inline Assembly for I/O

```go
// pkg/codegen/asm.go
func generateInlineAsm(asm *ast.InlineAsm) []byte {
    // Parse assembly
    // Resolve labels
    // Generate bytes
}
```

## 🖥️ MZE Emulator Integration

### Core Interceptor System

```go
// pkg/emulator/interceptor.go
type IOInterceptor interface {
    HandleIN(port uint16) byte
    HandleOUT(port uint16, value byte)
    ShouldIntercept(port uint16) bool
}

type InterceptorChain struct {
    interceptors []IOInterceptor
}

func (e *Emulator) AddInterceptor(i IOInterceptor) {
    e.interceptors = append(e.interceptors, i)
}

// In CPU loop
func (cpu *Z80) IN(port uint16) byte {
    for _, i := range cpu.interceptors {
        if i.ShouldIntercept(port) {
            return i.HandleIN(port)
        }
    }
    return 0xFF // Default
}
```

### Interceptor Registry

```go
// cmd/mze/main.go
func setupInterceptors(emu *Emulator, flags Flags) {
    if flags.EnableIO {
        emu.AddInterceptor(NewFileInterceptor(flags.IODirs))
    }
    if flags.EnableSound {
        emu.AddInterceptor(NewAYInterceptor())
    }
    if flags.EnableNetwork {
        emu.AddInterceptor(NewNetworkInterceptor())
    }
    if flags.EnableKeyboard {
        emu.AddInterceptor(NewKeyboardInterceptor())
    }
}
```

## 📊 Success Metrics

### File I/O
- [ ] Save/load files on host filesystem
- [ ] TAP/TZX file generation for ZX
- [ ] CP/M file operations work
- [ ] Directory listing functional

### Sound
- [ ] AY register writes produce sound
- [ ] Multiple channels work
- [ ] Sound effects play correctly
- [ ] Music playback functional

### Keyboard
- [ ] Matrix scanning works
- [ ] Enhanced buffer mode works
- [ ] Special keys handled
- [ ] No input lag

### Networking
- [ ] TCP connections establish
- [ ] Data transfer works
- [ ] HTTP requests succeed
- [ ] No port conflicts

## 🧪 Test Programs

### 1. I/O Test Suite
```minz
// tests/io_comprehensive.minz
fun test_all_io() -> void {
    test_file_io();
    test_sound();
    test_keyboard();
    test_network();
    print("ALL I/O TESTS PASSED!\n");
}
```

### 2. Demo Applications

#### Text Editor with Save/Load
```minz
import zx.io;
import zx.keyboard;

fun editor() -> void {
    let buffer: [16384]u8;
    let filename: [8]u8;
    
    // Edit loop with keyboard
    // Save with zx.io.tape_save()
    // Load with zx.io.tape_load()
}
```

#### Network Chat
```minz
import net.tcp;
import mze.keyboard;

fun chat() -> void {
    let server = tcp.connect([192,168,1,100], 6667);
    // Chat loop
}
```

#### Sound Demo
```minz
import zx.ay;
import sfx;

fun sound_demo() -> void {
    // Play scale
    for note in 60..72 {
        ay.play_note(Channel.A, note, 4, 15);
        wait_frames(10);
    }
    
    // Sound effects
    sfx.explosion();
    wait_frames(60);
    sfx.laser();
}
```

## 🚀 Development Workflow

1. **Week 1-2**: Complete MZE interceptor framework
2. **Week 3**: Implement platform directives in compiler
3. **Week 4**: Create file I/O modules and test
4. **Week 5**: Integrate Ayumi and test sound
5. **Week 6**: Complete keyboard system
6. **Week 7**: Implement TCP/IP basics
7. **Week 8**: HTTP client and testing
8. **Week 9**: Polish and documentation

## 📈 Expected Impact

When complete, MinZ will have:
- **Full I/O capabilities** matching commercial 8-bit development
- **Modern networking** on vintage hardware
- **Authentic sound** via cycle-perfect emulation
- **Cross-platform** file operations
- **Type-safe** I/O with zero overhead

## 🎯 Deliverables

### For v0.13.0 Release
1. ✅ File I/O on all platforms
2. ✅ AY-3-8912 sound support
3. ✅ Enhanced keyboard input
4. ✅ Basic TCP/IP networking
5. ✅ Example programs demonstrating all features
6. ✅ Complete documentation
7. ✅ Test coverage >80%

## 💡 Innovation Highlights

1. **First 8-bit language** with built-in TCP/IP support
2. **ROM/BDOS interception** for transparent file I/O
3. **Cycle-perfect sound** emulation in development environment
4. **Type-safe networking** with zero-cost abstractions
5. **Modern development** for vintage platforms

---

**Status**: Ready for implementation! All designs complete, architecture solid, just needs coding! 🚀