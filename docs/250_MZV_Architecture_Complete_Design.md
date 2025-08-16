# MZV Architecture: Modern Virtual Machine for MinZ Development

*Date: 2025-08-16*  
*Status: Research & Architecture Document*

## Executive Summary

MZV (MinZ Virtual machine) represents a **paradigm shift** from targeting vintage hardware to creating a modern, purpose-built VM that eliminates Z80's limitations while maintaining MinZ's philosophy of simplicity and performance. This is our "Escape Velocity" strategy - break free from 1970s constraints!

## 🎯 Why MZV Changes Everything

### Freedom from Hardware Limitations
- **No 64KB limit** - Full 32-bit addressing (4GB!)
- **No register pressure** - Unlimited virtual registers
- **No bank switching** - Linear memory model
- **No cycle counting** - Predictable performance

### Modern Development Experience
- **Built-in debugging** - Breakpoints, watches, stepping
- **Native networking** - TCP/IP, WebSockets, HTTP
- **Graphics/Audio** - SDL2 backend, 60fps guaranteed
- **Cross-platform** - Same bytecode everywhere

## 🏗️ Core Architecture

### Virtual Machine Design
```
┌─────────────────────────────────────┐
│           MZV Runtime               │
├─────────────────────────────────────┤
│  Graphics │ Network │ Audio │ Input │ <- Native Subsystems
├─────────────────────────────────────┤
│         Bytecode Interpreter        │ <- Stack-based VM
├─────────────────────────────────────┤
│    Memory Manager (32-bit flat)     │ <- No banking!
├─────────────────────────────────────┤
│      Host OS (Linux/Mac/Win)        │
└─────────────────────────────────────┘
```

### Instruction Set Philosophy
```minz
// MZV bytecode is semantic, not hardware-specific
CALL_METHOD    // Not CALL $8000
LOAD_FIELD     // Not LD A, (HL)
STRING_CONCAT  // Native string ops!
AWAIT_ASYNC    // Modern concurrency!
```

## 📺 Video/Graphics Architecture

### Display Model
```minz
module mzv.graphics {
    // Multiple display modes
    enum DisplayMode {
        TEXT_80x25,      // Classic terminal
        TEXT_132x50,     // Extended terminal
        BITMAP_320x240,  // Retro graphics
        BITMAP_640x480,  // High-res
        BITMAP_1920x1080 // Full HD!
    }
    
    // Hardware-accelerated operations
    struct Surface {
        width: u32,
        height: u32,
        pixels: &[Color32],
        
        fun blit(src: Surface, x: i32, y: i32) -> void;
        fun fill_rect(x: i32, y: i32, w: u32, h: u32, color: Color32) -> void;
        fun draw_text(x: i32, y: i32, text: str, font: Font) -> void;
    }
}
```

### Terminal Emulation
```minz
module mzv.terminal {
    // Full VT100/ANSI support
    class Terminal {
        cols: u32,
        rows: u32,
        cursor: Point,
        attributes: TextAttributes,
        
        fun write(text: str) -> void {
            // Handles ANSI escape sequences
            // Supports UTF-8
            // Hardware scrolling
        }
        
        fun set_color(fg: Color, bg: Color) -> void;
        fun move_cursor(x: u32, y: u32) -> void;
        fun clear_screen() -> void;
    }
}
```

## 🌐 I/O Architecture

### Port-Based I/O (Compatibility Layer)
```minz
// For porting Z80 code that uses IN/OUT
@port(0x00)
struct LegacyPort {
    data: u8,
    control: u8
}

// But also modern I/O
let socket = TcpSocket::connect("example.com:80")?;
socket.write(b"GET / HTTP/1.1\r\n")?;
```

### Memory-Mapped I/O (Modern Approach)
```minz
// Graphics framebuffer
@mmio(0x10000000)
struct Framebuffer {
    pixels: [Color32; 1920 * 1080],
    vsync: bool,
    
    fun wait_vsync() -> void {
        while !self.vsync { }
    }
}

// Audio buffer
@mmio(0x20000000)
struct AudioBuffer {
    samples: [i16; 4096],
    position: u32,
    
    fun play() -> void;
}
```

## 🔌 Networking Architecture

### Native TCP/IP Stack
```minz
module mzv.net {
    class TcpSocket {
        fun connect(addr: str) -> Result<TcpSocket, Error>;
        fun listen(port: u16) -> Result<TcpListener, Error>;
        fun read(buf: &mut [u8]) -> Result<usize, Error>;
        fun write(data: &[u8]) -> Result<usize, Error>;
    }
    
    class HttpClient {
        fun get(url: str) -> Result<Response, Error>;
        fun post(url: str, body: &[u8]) -> Result<Response, Error>;
    }
    
    // WebSocket support!
    class WebSocket {
        fun connect(url: str) -> Result<WebSocket, Error>;
        fun send_text(msg: str) -> Result<(), Error>;
        fun send_binary(data: &[u8]) -> Result<(), Error>;
        fun receive() -> Result<Message, Error>;
    }
}
```

### BBS/Terminal Server
```minz
// Run a modern BBS on MZV!
let server = TcpListener::bind("0.0.0.0:23")?;
loop {
    let client = server.accept()?;
    spawn {
        let term = Terminal::new(client);
        term.write("\x1b[2J\x1b[H");  // Clear screen
        term.write("Welcome to MZV BBS!\r\n");
        bbs_main_menu(term);
    };
}
```

## ⌨️ Input Architecture

### Unified Input System
```minz
module mzv.input {
    enum Event {
        KeyDown { key: Key, mods: Modifiers },
        KeyUp { key: Key, mods: Modifiers },
        TextInput { text: str },  // UTF-8!
        MouseMove { x: i32, y: i32 },
        MouseButton { button: u8, pressed: bool },
        GamepadButton { id: u8, button: u8, pressed: bool },
        GamepadAxis { id: u8, axis: u8, value: f32 }
    }
    
    fun poll_event() -> Option<Event>;
    fun wait_event() -> Event;
}
```

### Keyboard Handling
```minz
// Raw scancode access
let scancode = mzv.input.get_scancode();

// Cooked input with modifiers
match mzv.input.wait_event() {
    KeyDown { key: Key::A, mods: Modifiers::CTRL } => {
        // Ctrl+A pressed
    },
    TextInput { text } => {
        // Handle Unicode text input
        buffer.append(text);
    }
}
```

## 🎮 Game Development Features

### Sprite System
```minz
module mzv.sprites {
    struct Sprite {
        texture: Texture,
        x: f32,
        y: f32,
        scale: f32,
        rotation: f32,
        
        fun draw(surface: Surface) -> void;
        fun collides_with(other: Sprite) -> bool;
    }
    
    struct TileMap {
        tiles: [[u16; 256]; 256],
        tileset: Texture,
        
        fun draw(surface: Surface, camera: Camera) -> void;
    }
}
```

### Audio Engine
```minz
module mzv.audio {
    class Sound {
        fun load(path: str) -> Result<Sound, Error>;
        fun play() -> void;
        fun play_looped() -> void;
        fun set_volume(vol: f32) -> void;
    }
    
    class Music {
        fun load(path: str) -> Result<Music, Error>;
        fun play() -> void;
        fun pause() -> void;
        fun set_position(seconds: f32) -> void;
    }
    
    // Chip emulation for retro feel!
    class PSG {  // Programmable Sound Generator
        fun tone(channel: u8, freq: u16) -> void;
        fun noise(type: NoiseType) -> void;
        fun envelope(attack: u8, decay: u8, sustain: u8, release: u8) -> void;
    }
}
```

## 🚀 Revolutionary Features

### 1. Time Travel Debugging
```minz
// MZV can record and replay execution!
mzv.debug.start_recording();
game.play();
mzv.debug.stop_recording();

// Replay with different RNG seed
mzv.debug.replay(seed: 12345);
```

### 2. Hot Code Reload
```minz
// Change code while running!
@hot_reload
fun update_player(player: &Player) -> void {
    // Edit this function and see changes instantly
    player.x += player.velocity_x;
}
```

### 3. Built-in Profiler
```minz
mzv.profiler.start();
expensive_function();
let stats = mzv.profiler.stop();
print("Time: {stats.elapsed_ms}ms");
print("Allocations: {stats.allocations}");
```

### 4. Native Serialization
```minz
// Save/load entire game state
let state = mzv.serialize(game_world);
mzv.save_file("save.dat", state);

// Load later
let state = mzv.load_file("save.dat")?;
let game_world = mzv.deserialize<GameWorld>(state)?;
```

## 📊 Performance Characteristics

### Bytecode Efficiency
| Operation | Z80 Cycles | MZV Bytecode | Speedup |
|-----------|------------|--------------|---------|
| Function call | 17-44 | 1 instruction | 20x |
| String concat | 100+ | 1 instruction | 100x |
| Array bounds check | 20+ | Built-in | Safe + Fast |
| Hash table lookup | 200+ | 1 instruction | 200x |

### Memory Model
```
0x00000000 - 0x0FFFFFFF: Program code (256MB)
0x10000000 - 0x1FFFFFFF: Graphics memory (256MB)
0x20000000 - 0x2FFFFFFF: Audio buffers (256MB)
0x30000000 - 0xFFFFFFFF: Heap (3.25GB!)
```

## 🎯 Implementation Roadmap

### Phase 1: Core VM (2 weeks)
- [ ] Stack-based bytecode interpreter
- [ ] Basic instruction set (100 ops)
- [ ] Memory management
- [ ] Simple I/O

### Phase 2: Graphics & Terminal (1 month)
- [ ] SDL2 integration
- [ ] Terminal emulator
- [ ] Basic sprite rendering
- [ ] Font rendering

### Phase 3: Networking (2 weeks)
- [ ] TCP/IP sockets
- [ ] HTTP client
- [ ] WebSocket support
- [ ] async/await

### Phase 4: Advanced Features (1 month)
- [ ] Time-travel debugging
- [ ] Hot reload
- [ ] Profiler
- [ ] JIT compilation

## 💡 Killer Apps for MZV

### 1. Cloud IDE
```minz
// MinZ development environment running IN MZV!
let editor = CodeEditor::new();
let compiler = MinzCompiler::new();
let terminal = Terminal::new();

// Edit, compile, run - all in MZV
editor.on_save(|code| {
    let result = compiler.compile(code);
    terminal.run(result.bytecode);
});
```

### 2. Multiplayer Game Server
```minz
// MMO server in MinZ!
let server = GameServer::new();
server.listen(":8080");

server.on_connect(|player| {
    world.spawn_player(player);
    broadcast("${player.name} joined!");
});
```

### 3. Modern BBS System
```minz
// Telnet + Web accessible!
let bbs = BBS::new();
bbs.add_door_game("tradewars.mzv");
bbs.add_message_board("general");
bbs.listen_telnet(":23");
bbs.listen_http(":8080");
```

## 🌟 Why MZV Will Win

### For Developers
- **No hardware quirks** - Just write code
- **Modern tooling** - Debugger, profiler, hot reload
- **Unlimited resources** - No 64KB limit!
- **Cross-platform** - Write once, run everywhere

### For Users
- **Fast** - JIT-compiled performance
- **Safe** - Memory protection, bounds checking
- **Connected** - Native networking
- **Beautiful** - Hardware-accelerated graphics

### For MinZ
- **Escape velocity** - Break free from Z80 limitations
- **Showcase platform** - Demonstrate MinZ's full potential
- **Development speed** - Iterate without hardware constraints
- **Future-proof** - Modern VM can evolve with needs

## 📚 Technical Specifications

### Bytecode Format
```
Header (16 bytes):
  Magic: "MZV\0" (4 bytes)
  Version: u32
  Entry point: u32
  Flags: u32

Code section:
  Size: u32
  Instructions: [u8]

Data section:
  Size: u32
  Constants: [bytes]

Debug section (optional):
  Source maps
  Symbol table
  Type information
```

### Instruction Encoding
```
1-byte opcodes (256 instructions)
Variable-length operands
Stack-based with register cache
```

## 🚀 Call to Action

MZV transforms MinZ from "interesting Z80 language" to "the modern platform for retro-style development." It's our moon shot - escape the gravity well of 1970s hardware!

### Immediate Actions
1. **Week 1**: Define bytecode instruction set
2. **Week 2**: Implement basic interpreter
3. **Week 3**: Add SDL2 graphics
4. **Week 4**: Demo game running!

The future isn't in emulating the past - it's in building something better while keeping what made the past great: **simplicity, performance, and fun**.

---

*"MZV: Where vintage aesthetics meet modern performance"* 🚀✨