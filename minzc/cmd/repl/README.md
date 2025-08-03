# MinZ REPL - Interactive Z80 Development

The MinZ REPL provides an interactive environment for Z80 development with ZX Spectrum screen emulation.

## Installation

From the project root:
```bash
cd minzc
make repl           # Build REPL only
make all            # Build both compiler and REPL
make install        # Install to /usr/local/bin (requires sudo)

# Or use the convenience script:
./install.sh        # Installs to ~/.local/bin (no sudo needed)
```

## Running the REPL

```bash
minz                # If installed
./minz              # From minzc directory
make run-repl       # Build and run
```

## Features

- **Interactive MinZ compilation** - Type and run MinZ code instantly
- **ZX Spectrum screen emulation** - 32x24 text display with RST 16 support
- **Keyboard input** - RST 18 hooks for interactive programs
- **Register inspection** - View all Z80 registers including shadows
- **Memory viewer** - Inspect memory contents
- **Function management** - Define and call functions

## Quick Command Reference

| Command | Shortcut | Description |
|---------|----------|-------------|
| `/help` | `/h` `/？` | Show help |
| `/quit` | `/q` | Exit REPL |
| `/reg` | `/r` | Show registers |
| `/regc` | `/rc` | Compact register view |
| `/screen` | `/s` | Show ZX Spectrum screen |
| `/screens` | `/ss` | Toggle auto-show screen |
| `/cls` | | Clear screen |
| `/vars` | `/v` | Show variables |
| `/funcs` | `/f` | Show functions |
| `/mem` | `/m` | Show memory |

## Example Session

```
╔══════════════════════════════════════════════════════════════╗
║           MinZ REPL v1.0 - Interactive Z80 Development      ║
║                  With ZX Spectrum Screen Emulation          ║
╚══════════════════════════════════════════════════════════════╝
Type /h for help, /q to quit, or enter MinZ code

minz> let x: u8 = 42
Variable 'x' defined

minz> fun double(n: u8) -> u8 { return n * 2; }
Function 'double' defined at 0x8000

minz> double(x)
84

minz> /r
╔══════════════════════════════════════════════════════════════╗
║                    Z80 Register State                        ║
╠══════════════════════════════════════════════════════════════╣
║ AF =0054  BC =0000  DE =0000  HL =0054                      ║
║ AF'=0000  BC'=0000  DE'=0000  HL'=0000                      ║
║ IX =0000  IY =0000  SP =FFFF  PC =8010                      ║
║ I  =00    R  =00    IFF1=false  IFF2=false  IM=0            ║
╠══════════════════════════════════════════════════════════════╣
║ Flags: S=0 Z=0 H=0 P/V=0 N=0 C=0                            ║
╚══════════════════════════════════════════════════════════════╝

minz> /q
Goodbye! Happy coding!
```

## ZX Spectrum Screen Output

The REPL emulates a ZX Spectrum text screen. Any code that uses RST 16 (RST $10) will display on the virtual screen:

```minz
@abi("register: A=char")
fun putchar(c: u8) -> void {
    asm { RST 0x10 }
}

fun main() -> void {
    putchar('H');
    putchar('i');
    putchar('!');
}
```

View the screen with `/s` or enable auto-display with `/ss`.

## Keyboard Input

Programs can read keyboard input via RST 18:

```minz
@abi("register: A=char")
fun getchar() -> u8 {
    asm { RST 0x18 }
}
```

Future versions will support interactive typing directly into the ZX Spectrum screen window.

## Current Limitations

- Assembly generation is placeholder (needs z80asm integration)
- No actual code execution yet (compiler integration in progress)
- Input hooks ready but not connected to keyboard
- No persistent session state

## Architecture

The REPL consists of:
- **Main loop** (`main.go`) - Command processing and UI
- **Compiler integration** (`compiler.go`) - MinZ compilation pipeline
- **Z80 emulator** - Cycle-accurate Z80 emulation
- **ZX Screen** - Text mode screen emulation with I/O hooks

## Future Features

- [ ] Full compiler integration with z80asm
- [ ] TUI mode with multiple windows
- [ ] Interactive debugging with breakpoints
- [ ] Session recording and playback
- [ ] Network collaboration mode

---

*Turn your terminal into a ZX Spectrum!*