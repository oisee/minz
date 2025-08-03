# 128: TUI Multi-Window REPL with Interactive ZX Spectrum Screen

## 🎯 Vision

A proper TUI (Terminal User Interface) REPL with multiple windows:
- **REPL Window** - Command input and MinZ code
- **ZX Spectrum Screen** - Live 32x24 display with input capability
- **Registers Window** - Real-time register display
- **Memory Window** - Memory viewer/editor

## 🏗️ Architecture

### Window Layout
```
┌─────────────────────────────────────────────────────────────────┐
│ MinZ REPL v1.0 - [REPL] [SCREEN*] [REGISTERS] [MEMORY]         │
├─────────────────────────────────────────────────────────────────┤
│ ┌─── ZX Spectrum Screen (32x24) ──┐ ┌─── Registers ───────┐   │
│ │>HELLO FROM ZX SPECTRUM!          │ │ AF =1234 BC =5678   │   │
│ │>Ready                            │ │ DE =9ABC HL =DEF0   │   │
│ │>_                                │ │ IX =0000 IY =0000   │   │
│ │                                  │ │ SP =FFFF PC =8000   │   │
│ │                                  │ ├────────────────────┤   │
│ │                                  │ │ Flags: SZ-H-P-C    │   │
│ │                                  │ └────────────────────┘   │
│ │                                  │ ┌─── Memory ─────────┐   │
│ │                                  │ │ 8000: 3E 41 D7 10  │   │
│ │                                  │ │ 8004: 3E 42 D7 10  │   │
│ │                                  │ │ 8008: C9 00 00 00  │   │
│ └──────────────────────────────────┘ └────────────────────┘   │
├─────────────────────────────────────────────────────────────────┤
│ [REPL Input]                                                    │
│ minz> _                                                         │
└─────────────────────────────────────────────────────────────────┘

SHIFT+TAB: Switch windows | F1: Help | F5: Run | F10: Quit
```

## 🎮 Keyboard Controls

### Window Navigation
- `SHIFT+TAB` - Cycle through windows
- `TAB` - Next window
- `F1-F4` - Jump to specific window (F1=REPL, F2=Screen, F3=Regs, F4=Mem)

### ZX Spectrum Screen Mode
When focused (indicated by highlight):
- **Type normally** - Characters sent via RST 18 (keyboard input)
- `ENTER` - Send CR (0x0D)
- `BACKSPACE` - Send BS (0x08)
- `ESC` - Return to REPL window

### REPL Mode
- Normal REPL commands
- `/tui` - Toggle TUI mode on/off
- `/focus screen` - Switch focus to ZX screen

## 🔧 Implementation Plan

### Phase 1: Basic TUI Framework
```go
type TUIWindow interface {
    Draw(x, y, width, height int)
    HandleKey(key rune) bool
    SetFocus(focused bool)
    Update()
}

type REPLTUI struct {
    screen     tcell.Screen
    windows    []TUIWindow
    activeWin  int
    
    // Windows
    replWin    *REPLWindow
    spectrumWin *SpectrumWindow
    registersWin *RegistersWindow
    memoryWin   *MemoryWindow
}
```

### Phase 2: ZX Spectrum Input Hook
```go
// When ZX Screen window has focus
func (s *SpectrumWindow) HandleKey(key rune) bool {
    if s.focused {
        // Send to Z80 via RST 18 hook
        s.emulator.SendKeypress(byte(key))
        return true
    }
    return false
}

// Hook for RST 18 (collect character)
z80.Hooks.OnRST18 = func() byte {
    if s.inputBuffer.Len() > 0 {
        return s.inputBuffer.ReadByte()
    }
    return 0 // No input available
}
```

### Phase 3: Bidirectional Communication
```minz
// MinZ code can now do input!
@abi("register: A=char")
fun getchar() -> u8 {
    asm {
        RST 0x18   ; Collect character from keyboard
    }
}

fun input_line(buffer: *u8, max_len: u8) -> u8 {
    let i: u8 = 0;
    while i < max_len {
        let ch = getchar();
        if ch == 0x0D {  // Enter pressed
            buffer[i] = 0;
            return i;
        }
        if ch == 0x08 && i > 0 {  // Backspace
            i = i - 1;
            putchar(0x08);
            putchar(' ');
            putchar(0x08);
        } else if ch >= 32 {  // Printable
            buffer[i] = ch;
            putchar(ch);  // Echo
            i = i + 1;
        }
    }
    return i;
}
```

## 📝 Example Interactive Session

```
┌─── ZX Spectrum Screen ────────────┐
│ MinZ BASIC v1.0                   │
│ Ready                             │
│ >10 PRINT "HELLO"                 │  <- User types here
│ >20 GOTO 10                       │
│ >RUN                              │
│ HELLO                             │
│ HELLO                             │
│ HELLO                             │
│ Break at line 20                  │
│ >_                                │  <- Cursor here
└───────────────────────────────────┘

[User presses SHIFT+TAB to switch back to REPL]

minz> /reg
[Registers update in real-time in the register window]
```

## 🚀 Advanced Features

### Live Updates
- Registers update as code executes
- Screen updates character by character
- Memory view follows PC or custom address

### Breakpoints
- Click on memory address to set breakpoint
- Visual indicators in memory window
- Step through code instruction by instruction

### Screen Recording
```go
type ScreenRecording struct {
    frames []ScreenFrame
    timestamps []time.Duration
}

func (s *SpectrumWindow) StartRecording() {
    // Record all screen changes with timing
}

func (s *SpectrumWindow) PlaybackRecording() {
    // Replay screen activity
}
```

## 🎨 Visual Enhancements

### Color Support (if terminal supports)
```go
// ZX Spectrum colors
const (
    BLACK   = 0
    BLUE    = 1
    RED     = 2
    MAGENTA = 3
    GREEN   = 4
    CYAN    = 5
    YELLOW  = 6
    WHITE   = 7
)

// With BRIGHT flag
BRIGHT_BLUE = BLUE | 0x08
```

### Block Graphics
```
┌─── ZX Spectrum Screen ────────────┐
│ ▀▄ ▄▀ Block graphics support      │
│ ▄▀▀▀▄ Using Unicode box chars     │
│ █░░░█ For authentic look          │
└───────────────────────────────────┘
```

## 🔌 Integration Points

### Input Sources
1. **Direct typing** in ZX Screen window
2. **REPL commands** that generate input
3. **Script playback** from files
4. **Network input** (for multiplayer?)

### Output Destinations
1. **Screen buffer** (visual display)
2. **Output capture** (for testing)
3. **Recording** (for playback)
4. **Streaming** (for sharing)

## 💡 Use Cases

### Interactive Programs
```minz
fun main() -> void {
    cls();
    print_string("What is your name? ");
    let name: [32]u8;
    input_line(&name[0], 31);
    print_string("\nHello, ");
    print_string(&name[0]);
    print_string("!\n");
}
```

### Games
```minz
fun game_loop() -> void {
    while true {
        let key = getchar();
        switch key {
            case 'w': move_up();
            case 's': move_down();
            case 'a': move_left();
            case 'd': move_right();
            case 'q': return;
        }
        draw_screen();
    }
}
```

### BASIC Interpreter
```minz
fun basic_repl() -> void {
    print_string("MinZ BASIC v1.0\nReady\n");
    while true {
        print_string(">");
        let line: [80]u8;
        let len = input_line(&line[0], 79);
        if len > 0 {
            execute_basic(&line[0]);
        }
    }
}
```

## 🛠️ Required Libraries

### For Full TUI
- `github.com/gdamore/tcell/v2` - Terminal control
- `github.com/rivo/tview` - TUI framework (alternative)

### For Simple Mode
- ANSI escape codes only
- No external dependencies

## 📊 Benefits

1. **Authentic Experience** - Like using a real ZX Spectrum
2. **Interactive Debugging** - See everything in real-time
3. **Educational** - Perfect for learning Z80 and systems programming
4. **Fun** - Write and play games in the REPL!

---

*"Turn your terminal into a ZX Spectrum!"* - The MinZ TUI Way