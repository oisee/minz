# 127: ZX Spectrum Screen Emulation in REPL

## 🎯 Overview

We've integrated a ZX Spectrum text screen emulator into the MinZ REPL, allowing you to see the output as it would appear on a real ZX Spectrum!

## 🏗️ Architecture

### Screen Emulation
- **32x24 character display** - Standard ZX Spectrum text mode
- **RST 16 (RST $10) hook** - Intercepts ROM print character calls
- **System variables** - Mirrors ZX Spectrum system vars (0x5C00-0x5CFF)
- **I/O ports** - Debug output on port 0x42, screen control on port 0x21

### Hook System
```go
// When RST 16 is executed:
RST 0x10 → Hook → Screen.PrintChar(A register) → Buffer update

// When OUT instruction is executed:
OUT 0x42, A → Hook → Screen.PrintChar(A) → Direct output
```

## 📝 REPL Commands

### Screen Commands
- `/screen` - Show current ZX Spectrum screen
- `/screens` - Toggle auto-show screen after execution
- `/cls` - Clear the ZX Spectrum screen

### Example Session
```
minz> /cls
ZX Spectrum screen cleared

minz> fun hello() -> void {
    @asm("LD A, 'H'")
    @asm("RST 0x10")
    @asm("LD A, 'i'")
    @asm("RST 0x10")
}
Function 'hello' defined

minz> hello()

minz> /screen
--- ZX Spectrum Screen (32x24) ---
╔════════════════════════════════╗
║Hi                              ║
║                                ║
║                                ║
...
╚════════════════════════════════╝
```

## 🎮 ZX Spectrum Compatibility

### System Variables Supported
```
0x5C88 (S_POSN)  - Screen column position
0x5C89 (S_POSNL) - Screen line position
0x5C8D (ATTR_P)  - Permanent attributes (ink/paper/bright/flash)
```

### Special Characters
```
0x0D - Carriage return (new line)
0x08 - Backspace
0x09 - Tab
0x0C - Clear screen (form feed)
```

### I/O Ports
```
Port 0x42 - Debug output (prints directly)
Port 0x21 - Screen control:
  Bits 0-2: Command (0=CLS, 1=SetX, 2=SetY, 3=Ink, 4=Paper)
  Bits 3-7: Parameter
```

## 💻 MinZ Code Examples

### Using RST 16 Directly
```minz
@abi("register: A=char")
fun putchar(c: u8) -> void {
    asm {
        RST 0x10   ; Call ROM print routine
    }
}

fun main() -> void {
    putchar('A');
    putchar('B');
    putchar('C');
}
```

### Using OUT Port
```minz
@abi("register: A=value")
fun debug_print(ch: u8) -> void {
    asm {
        OUT (0x42), A  ; Debug output port
    }
}
```

### Clear Screen
```minz
fun cls() -> void {
    asm {
        LD A, 0x0C     ; Form feed character
        RST 0x10       ; Print it
    }
}
```

## 🚀 Benefits

1. **Visual Debugging** - See what your code would display on real hardware
2. **ROM Compatibility** - Uses standard ZX Spectrum ROM routines
3. **Text Mode** - Perfect for REPL output and debugging
4. **No Graphics Needed** - Simple text display for terminal

## 🔧 Implementation Details

### Screen Buffer
```go
type ZXScreen struct {
    buffer      [24][32]byte  // Screen characters
    cursorX     int           // Column (0-31)
    cursorY     int           // Line (0-23)
    outputBuffer strings.Builder // Captured output
}
```

### RST Hook Integration
```go
z80.Hooks.OnRST10 = func(a byte) {
    screen.PrintChar(a)  // Print to screen buffer
}
```

### Display Modes
1. **Full Screen** - Shows complete 32x24 grid with borders
2. **Compact** - Shows only non-empty lines
3. **Output Only** - Returns captured text without formatting

## 📊 Performance

- **Zero overhead** when screen display is off
- **Minimal impact** on execution (just buffer updates)
- **Instant display** of screen state
- **No graphics emulation** needed (text only)

## 🎯 Future Enhancements

### Planned Features
- [ ] Color attributes (INK, PAPER, BRIGHT, FLASH)
- [ ] Extended character set (block graphics)
- [ ] Screen memory mapping (0x4000-0x5AFF)
- [ ] Border color support
- [ ] Input hooks (RST 18 for keyboard input)

### Possible Extensions
- ASCII art mode for block graphics
- ANSI color codes for attributes
- Screen recording/playback
- Integration with real ZX Spectrum fonts

## 📚 References

- ZX Spectrum ROM Disassembly
- System Variables at 0x5C00-0x5CFF
- RST routines in ROM
- Standard I/O ports

---

*"See your code run on a virtual ZX Spectrum screen!"* - The MinZ Way