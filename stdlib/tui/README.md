# stdlib/tui — Universal TUI Framework

Interactive screen/form framework that works across **all backends**:
CP/M (BDOS), MZV (MIR2 VM), ZX Spectrum, Agon Light, and native (QBE).

The screen is **data, not code**. The same program runs everywhere —
only the renderer changes.

## Quick Start

### Level 1: Simple Selection Screen (works everywhere)

```nanz
@extern fun sel_register_str(name: ^u8, length: u8, defval: ^u8, bufptr: ^u8) -> void
@extern fun sel_register_int(name: ^u8, defval: u16) -> void
@extern fun sel_show() -> u8
@extern fun sel_get_int(idx: u8) -> u16

global name_buf: ^u8

fun main() -> void {
    sel_register_str(c"NAME", 20, c"World", @name_buf)
    sel_register_int(c"COUNT", 3)
    var rc: u8 = sel_show()
    var count: u16 = sel_get_int(1)
}
```

### Level 2: OOP Screen API (UFCS methods)

```nanz
import stdlib.tui.screen

global scr: Screen
global name_buf: [22]u8

fun main() -> void {
    scr.init(c"Customer Master")
    scr.add_field(c"Customer", 10, &name_buf)
    scr.add_int(c"Count", 5)
    scr.add_button(c"Execute", KEY_F8)
    scr.show()

    var count: u16 = scr.get_int(1)
}
```

### Level 3: Declarative DSL (compile-time metafunction)

Define a `fun @screen` metafunction — it runs at compile time on the MIR2 VM,
receives the block as data, and emits Level 2 Nanz code:

```nanz
// Step 1: Define the metafunction (runs at compile time)
fun @screen(title: ^u8) -> void {
    var q: ^u8 = str_chr(34)  // quote character

    emit(c"fun _generated_screen() -> void {")
    emit(c"    tui_clear()")
    emit(c"    tui_color(7, 4, 1)")
    emit(c"    tui_goto(0, 0)")

    // Emit title: tui_puts(c"  Material Report")
    var line: ^u8 = str_concat(c"    tui_puts(c", q)
    line = str_concat(line, c"  ")
    line = str_concat(line, title)
    line = str_concat(line, q)
    emit(str_concat(line, c")"))
    emit(c"    tui_reset()")

    // Iterate block nodes
    var n: u8 = block_len()
    var i: u8 = 0
    while i < n {
        var kw: ^u8 = node_keyword(i)
        var label: ^u8 = node_arg_str(i, 0)
        var row: ^u8 = str_from_int(u16(i) + 2)

        if str_eq(kw, c"field") == 1 {
            emit(str_concat(c"    tui_goto(2, ", str_concat(row, c")")))
            emit(c"    tui_color(6, 0, 0)")
            var s: ^u8 = str_concat(c"    tui_puts(c", q)
            s = str_concat(s, label)
            s = str_concat(s, c"    ")
            s = str_concat(s, q)
            emit(str_concat(s, c")"))
            emit(c"    tui_color(7, 0, 0)")
            s = str_concat(c"    tui_puts(c", q)
            s = str_concat(s, c"[__________]")
            s = str_concat(s, q)
            emit(str_concat(s, c")"))
            emit(c"    tui_reset()")
        }
        if str_eq(kw, c"button") == 1 {
            emit(str_concat(c"    tui_goto(2, ", str_concat(row, c")")))
            emit(c"    tui_color(0, 7, 1)")
            var s: ^u8 = str_concat(c"    tui_puts(c", q)
            s = str_concat(s, c"[")
            s = str_concat(s, label)
            s = str_concat(s, c"]")
            s = str_concat(s, q)
            emit(str_concat(s, c")"))
            emit(c"    tui_reset()")
        }
        i = i + 1
    }

    emit(c"    var key: u8 = tui_read_key()")
    emit(c"    tui_clear()")
    emit(c"    tui_reset()")
    emit(c"}")
}

// Step 2: Use it — the compiler generates _generated_screen() for you
@screen("Material Report") {
    field "Material"
    field "Plant"
    field "Count"
    button "Execute"
}

fun main() -> void {
    _generated_screen()
}
```

Run it:

```bash
$ echo "" | mzv -H examples/nanz/meta_screen.nanz
```

Output (with ANSI colors — white-on-blue title, cyan labels, inverted button):

```
  Material Report
Material    [__________]
Plant       [__________]
Count       [__________]
[Execute]
```

**How it works:** The compiler sees `@screen("Material Report") { ... }`,
finds `fun @screen`, compiles it to MIR2, executes it on the VM. The
metafunction calls `block_len()` (returns 4), iterates each node with
`node_keyword(i)` and `node_arg_str(i, 0)`, builds `tui_*` call strings
via `str_concat`/`str_chr`, and `emit()`s them. The emitted Nanz source
is parsed and merged into the program. Zero runtime overhead — all code
generation happens at compile time.
```

## Running on Different Backends

### MZV (MIR2 VM) — development & testing

```bash
# Interactive (prompts for input)
mzv examples/nanz/tui_screen.nanz

# Headless with piped input
printf 'Z80\n\n' | mzv -H examples/abap/hello_input.abap

# Headless with defaults (empty input)
echo "" | mzv -H examples/abap/hello_input.abap

# Trace mode (shows sel_register/sel_show calls)
printf 'Z80\n' | mzv -t -H examples/abap/hello_input.abap
```

Output on MZV:
```
$ printf 'Z80\n\n' | mzv -H examples/abap/hello_input.abap
Hello, Z80!

$ echo "" | mzv -H examples/abap/hello_input.abap
Hello, World!
```

### CP/M (Z80 binary via mze) — retro target

```bash
# Compile → assemble → run on CP/M emulator
minzc examples/abap/hello_input.abap -o hello.a80
mza hello.a80 -o hello.com
printf 'Z80\n\n' | mze hello.com -t cpm
```

Output on CP/M:
```
P_NAME [World]:
P_COUNT [3]:

Hello, Z80!
```

On CP/M, `sel_show()` returns 0 (no host handler), so the fallback BDOS
path runs: each field gets a text prompt via BDOS function 0x0A (buffered
line input). Empty input keeps the default value.

### ZX Spectrum (pixel screen via mzv --zx)

```bash
# ZX Spectrum mode: 32x24 attribute grid rendered to stdout
mzv --zx examples/zx_demos/tetris.nanz

# Headless ZX capture (for testing/screenshots)
mzv -H --zx --max-frames 100 tetris.nanz
```

The `--zx` flag enables ZX Spectrum screen rendering. Without it, stdout
is clean program output. ZX mode and TUI mode are mutually exclusive.

### Native (QBE backend) — future

TUI functions compile to printf/ncurses calls via the QBE backend.
Same Nanz source, native binary.

## TUI Rendering Primitives

The `tui_*` functions are declared as `@extern` in `render.nanz` and
implemented as host functions per backend:

| Function | MZV | CP/M | ZX Spectrum |
|----------|-----|------|-------------|
| `tui_goto(x, y)` | ANSI `ESC[y;xH` | VT100 escape | screen addr calc |
| `tui_color(fg, bg, bright)` | ANSI `ESC[fg;bgm` | VT100 escape | attribute write |
| `tui_putch(ch)` | stdout + box-draw | BDOS 0x02 | pixel font blit |
| `tui_puts(str)` | stdout loop | BDOS 0x09 | print string |
| `tui_clear()` | ANSI `ESC[2J` | form feed | LDIR zero fill |
| `tui_read_key()` | stdin raw | BDOS 0x01 | IN (0xFE) matrix |
| `tui_read_line(buf, max)` | stdin line | BDOS 0x0A | line editor |
| `tui_width()` | 80 | 80 | 32 |
| `tui_height()` | 24 | 24 | 24 |

Box-drawing characters are sent as codes 1-6 and translated by the host:

| Code | Character | Name |
|------|-----------|------|
| 1 | `┌` | BOX_TL |
| 2 | `┐` | BOX_TR |
| 3 | `└` | BOX_BL |
| 4 | `┘` | BOX_BR |
| 5 | `─` | BOX_H |
| 6 | `│` | BOX_V |

## Level 2 TUI Screen Example

When `tui_screen.nanz` runs, it renders a SAP-style selection screen
with ANSI colors:

```
  Material Report                    ← white on blue title bar
Material    [*                 ]     ← cyan label + white input
Plant       [    ]
Count       [10]
[F8=Execute]  [F3=Back]              ← inverted buttons
  TAB=Next  Enter=Edit  F8=Execute   ← blue status bar
```

## Architecture

```
Level 3:  @screen("title") { field ... }     ← declarative DSL
              ↓ compile-time VM execution
Level 2:  scr.add_field(...); scr.show()     ← OOP UFCS methods
              ↓ function calls
Level 1:  sel_register_str(...); sel_show()  ← flat host API
              ↓ host dispatch
Backend:  MZV (ANSI) | CP/M (BDOS) | ZX (pixel) | Native (ncurses)
```

Each level is built on the one below. Level 3 metafunctions run at
compile time and emit Level 2 calls. Level 2 methods call Level 1
host functions. Level 1 dispatches to the appropriate backend.

## Files

| File | Purpose |
|------|---------|
| `widget.nanz` | Base types: `Rect`, `ScreenField`, `Screen`, key/color constants |
| `render.nanz` | `@extern tui_*` rendering primitives + `draw_box`, `clear_rect` |
| `screen.nanz` | UFCS `Screen` API + Phase 1 `sel_*` backward compatibility |

## Metafunction Host Functions

Available inside `fun @name(...)` compile-time functions:

| Function | Purpose |
|----------|---------|
| `emit(str)` | Append line to output buffer |
| `block_len()` | Number of nodes in DSL block |
| `node_keyword(i)` | Keyword of i-th node (`"field"`, `"button"`) |
| `node_arg_str(i, j)` | j-th positional string argument |
| `node_kwarg(i, key)` | Named keyword argument value |
| `str_concat(a, b)` | String concatenation |
| `str_from_int(n)` | Integer to decimal string |
| `str_chr(code)` | ASCII code to single-char string |
| `str_eq(a, b)` | String equality (returns 0 or 1) |
