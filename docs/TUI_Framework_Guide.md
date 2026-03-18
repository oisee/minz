# TUI Framework Guide — Universal Screens for Nanz

## Overview

The TUI framework provides interactive input screens that work across **all backends**: CP/M (BDOS line prompts), MZV (ANSI terminal), ZX Spectrum (`--zx` pixel grid), Agon Light (VDP), and native (QBE/ncurses).

The key insight: **the screen is data, not code**. The same Nanz program renders correctly whether it runs as Z80 binary on CP/M or as MIR2 bytecode on the VM.

Three API levels, each built on the previous:

```
Level 3:  @screen("title") { field ... }     ← declarative DSL (metafunction)
              ↓ compile-time code generation
Level 2:  scr.add_field(...); scr.show()     ← OOP UFCS methods
              ↓ function calls
Level 1:  sel_register_str(...); sel_show()  ← flat host API
              ↓ host dispatch
Backend:  MZV (ANSI) | CP/M (BDOS) | ZX (pixel) | Native (ncurses)
```

---

## Quick Start

### Simplest example (Level 1)

```nanz
// tui_demo.nanz — piped input or defaults
@extern fun sel_register_int(name: ^u8, defval: u16) -> void
@extern fun sel_show() -> u8
@extern fun sel_get_int(idx: u8) -> u16

fun main() -> void {
    sel_register_int(c"COUNT", 42)
    var rc: u8 = sel_show()
    var count: u16 = sel_get_int(0)
    @print_u8(u8(count))
}
```

```bash
$ printf '7\n' | mzv -H tui_demo.nanz
7

$ echo "" | mzv -H tui_demo.nanz    # uses default
42
```

### Full TUI screen (Level 2)

```nanz
// tui_screen.nanz — colored ANSI screen with fields and buttons
@extern fun tui_goto(x: u8, y: u8) -> void
@extern fun tui_color(fg: u8, bg: u8, bright: u8) -> void
@extern fun tui_reset() -> void
@extern fun tui_clear() -> void
@extern fun tui_puts(str: ^u8) -> void
@extern fun tui_read_key() -> u8

fun main() -> void {
    tui_clear()

    // Title bar: white on blue (fg=7, bg=4, bright=1)
    tui_color(7, 4, 1)
    tui_goto(0, 0)
    tui_puts(c"  Material Report")
    tui_reset()

    // Field: cyan label + white input
    tui_goto(2, 2)
    tui_color(6, 0, 0)        // cyan on black
    tui_puts(c"Material    ")
    tui_color(7, 0, 0)        // white on black
    tui_puts(c"[*                 ]")
    tui_reset()

    // Button: inverted
    tui_goto(2, 6)
    tui_color(0, 7, 1)        // black on white, bright
    tui_puts(c"[F8=Execute]")
    tui_reset()

    var key: u8 = tui_read_key()
    tui_clear()
}
```

```bash
$ echo "" | mzv -H tui_screen.nanz
```

Output (with ANSI colors):
```
  Material Report                    ← white on blue title bar
Material    [*                 ]     ← cyan label, white input
Plant       [    ]
Count       [10]
[F8=Execute]  [F3=Back]              ← inverted buttons
  TAB=Next  Enter=Edit  F8=Execute   ← blue status bar
```

### Declarative DSL (Level 3 — metafunction)

```nanz
// Define a reusable BlockNode struct for block iteration
struct BlockNode {
    keyword: ^u8
    label:   ^u8
    value:   ^u8
    length:  u8
    fkey:    u8
}

// @screen metafunction: runs at compile time on MIR2 VM
fun @screen(title: ^u8) -> void {
    emit(c"fun _generated_screen() -> void {")
    emit(c"    tui_clear()")

    emit_tui_color(7, 4, 1)
    emit_tui_goto(0, 0)
    emit_tui_puts(str_concat(c"  ", title))
    emit(c"    tui_reset()")

    // Iterate block as typed struct pointers — native Nanz!
    var nodes: ^BlockNode = block_nodes()
    var row: u8 = 2
    for node: ^BlockNode in nodes[0..block_len()] {
        if str_eq(node.keyword, c"field") == 1 {
            emit_tui_goto(2, row)
            emit_tui_color(6, 0, 0)
            emit_tui_puts(str_concat(node.label, c"    "))
            emit_tui_color(7, 0, 0)
            emit_tui_puts(c"[__________]")
            emit(c"    tui_reset()")
        }
        if str_eq(node.keyword, c"button") == 1 {
            emit_tui_goto(2, row)
            emit_tui_color(0, 7, 1)
            emit_tui_puts(str_concat(c"[", str_concat(node.label, c"]")))
            emit(c"    tui_reset()")
        }
        row = row + 1
    }

    emit(c"    var key: u8 = tui_read_key()")
    emit(c"    tui_clear()")
    emit(c"}")
}

// Use it — 5 lines instead of 50
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

```bash
$ echo "" | mzv -H meta_screen.nanz

  Material Report
Material    [__________]
Plant       [__________]
Count       [__________]
[Execute]
```

**How it works:** The compiler sees `@screen(...){}`, finds `fun @screen`,
compiles it to MIR2, runs it on the VM. The metafunction iterates the block
as a `^BlockNode` struct array, calls `emit_tui_*()` helpers, and the emitted
Nanz source gets parsed and merged into the program. Zero runtime overhead.

---

## Running on Different Backends

### MZV (development & testing)

```bash
mzv program.nanz              # interactive (raw terminal)
mzv -H program.nanz           # headless (auto-execute, stdout only)
printf 'input\n' | mzv -H p.nanz  # piped input
mzv -t -H program.nanz        # trace (shows sel_register/sel_show calls)
```

### CP/M (Z80 binary)

```bash
minzc program.abap -o out.a80   # compile ABAP → Z80 assembly
mza out.a80 -o out.com          # assemble → CP/M binary
mze out.com -t cpm              # run on CP/M emulator

# With piped input:
printf 'Z80\n\n' | mze out.com -t cpm
```

On CP/M, `sel_show()` returns 0 (no host handler). The fallback path
prompts for each field via BDOS 0x0A (buffered line input):

```
P_NAME [World]:
P_COUNT [3]:

Hello, Z80!
```

### ZX Spectrum (pixel screen)

```bash
mzv --zx program.nanz          # ZX Spectrum 32×24 attribute grid
mzv -H --zx --max-frames 100   # headless ZX with frame limit
```

The `--zx` flag is **required** for ZX Spectrum rendering. Without it,
stdout is clean program output.

---

## API Reference

### Level 1: sel_register / sel_show

| Function | Description |
|----------|-------------|
| `sel_register_str(name, len, default, buf)` | Register text field |
| `sel_register_int(name, default)` | Register integer field |
| `sel_show() → u8` | Show screen (1=host handled, 0=fallback) |
| `sel_get_int(idx) → u16` | Get integer value after show |

### Level 2: tui_* rendering primitives

| Function | Description | ANSI Output |
|----------|-------------|-------------|
| `tui_goto(x, y)` | Position cursor | `ESC[y+1;x+1H` |
| `tui_color(fg, bg, bright)` | Set colors (0-7) | `ESC[fg+30;bg+40m` |
| `tui_reset()` | Reset to default | `ESC[0m` |
| `tui_clear()` | Clear screen | `ESC[2J ESC[H` |
| `tui_putch(ch)` | Print character | char or box-drawing |
| `tui_puts(str)` | Print string | string bytes |
| `tui_read_key() → u8` | Read key (blocking) | stdin |
| `tui_read_line(buf, max) → u8` | Read line into buffer | stdin line |
| `tui_width() → u8` | Terminal width | 80 |
| `tui_height() → u8` | Terminal height | 24 |

**Colors** (ANSI 3-bit): 0=black, 1=red, 2=green, 3=yellow, 4=blue, 5=magenta, 6=cyan, 7=white

**Box-drawing characters:** tui_putch codes 1-6 → `┌ ┐ └ ┘ ─ │`

**Key codes:** 8=BS, 9=TAB, 13=ENTER, 27=ESC, 128-131=arrows, 140-147=F1-F8

### Level 3: Metafunction host functions

Available inside `fun @name(...)` compile-time functions:

| Function | Description |
|----------|-------------|
| `emit(str)` | Append line to output buffer |
| `block_len() → u8` | Number of nodes in block |
| `block_nodes() → ^u8` | Pointer to BlockNode array on heap |
| `node_keyword(i) → ^u8` | Keyword of i-th node |
| `node_arg_str(i, j) → ^u8` | j-th string argument |
| `node_kwarg(i, key) → ^u8` | Named keyword argument value |
| `str_concat(a, b) → ^u8` | String concatenation |
| `str_from_int(n) → ^u8` | Integer to string |
| `str_chr(code) → ^u8` | ASCII code to character |
| `str_eq(a, b) → u8` | String equality (0 or 1) |
| `emit_tui_puts(str)` | Emit `tui_puts(c"str")` |
| `emit_tui_goto(x, y)` | Emit `tui_goto(x, y)` |
| `emit_tui_color(fg, bg, br)` | Emit `tui_color(fg, bg, br)` |

### Typed struct pointer iteration

```nanz
struct BlockNode {
    keyword: ^u8    // offset 0, 2 bytes
    label:   ^u8    // offset 2, 2 bytes
    value:   ^u8    // offset 4, 2 bytes
    length:  u8     // offset 6, 1 byte
    fkey:    u8     // offset 7, 1 byte
}

// Iterate with automatic stride = sizeof(BlockNode) = 8
for node: ^BlockNode in nodes[0..n] {
    node.keyword   // Load(ptr + 0, TyPtr)
    node.label     // Load(ptr + 2, TyPtr)
    node.length    // Load(ptr + 6, TyU8)
}
```

The `for node: ^Struct in ptr[0..n]` syntax computes `stride = sizeof(Struct)`
at parse time. The loop variable `node` is a pointer that advances by stride
each iteration. Field access uses computed byte offsets — zero overhead.

---

## ABAP Integration

ABAP PARAMETERS automatically generate sel_register calls:

```abap
PARAMETERS: p_name  TYPE c LENGTH 20 DEFAULT 'World',
            p_count TYPE i DEFAULT 3.

START-OF-SELECTION.
  WRITE p_name.
```

The ABAP lowerer emits:
1. `sel_register_str("P_NAME", 20, "World", &buf)` — register field
2. `sel_register_int("P_COUNT", 3)` — register integer field
3. `sel_show()` → 1 on MZV (host reads stdin), 0 on Z80 (BDOS fallback)
4. On Z80: inline BDOS prompts for each field

Same ABAP program runs on both backends:

```bash
# CP/M
printf 'Z80\n\n' | mze hello_input.com -t cpm
P_NAME [World]:
P_COUNT [3]:
Hello, Z80!

# MZV
printf 'Z80\n\n' | mzv -H hello_input.abap
Hello, Z80!
```

---

## Architecture Notes

### Metafunction Pipeline

```
@screen("title") { block }
    ↓ parser (parse.go)
1. Capture metafunction source (raw text extraction)
2. Parse block → []metaBlockNode (keyword + args + kwargs)
    ↓ compilation
3. Metafun source + extern preamble + struct defs → Parse → HIR → MIR2
    ↓ VM execution
4. Serialize block as BlockNode struct array on VM heap
5. VM.Call("screen", [title_ptr]) — metafun runs
6. emit() calls append Nanz source to buffer
    ↓ splice
7. Parse emitted text → HIR
8. Remap string indices (offset by parent pool size)
9. Merge funcs/globals/structs into caller module
```

### Anti-inlining

Z80 stubs for sel_show/sel_register_* use inline asm (`NOP`/`XOR A`)
to prevent the MIR2 `InlineTrivial` pass from folding them. This ensures
MZV can override them via the host function table.

### Display modes

MZV has two mutually exclusive display modes:

- **Default**: stdout is program output (TUI, @print, ABAP WRITE)
- **`--zx`**: stdout is ZX Spectrum 32×24 pixel frame (for Tetris etc.)

No auto-detection — explicit `--zx` flag required.

---

## Files

| File | Purpose |
|------|---------|
| `stdlib/tui/widget.nanz` | Rect, ScreenField, Screen structs, key/color constants |
| `stdlib/tui/render.nanz` | @extern tui_* rendering primitives + draw_box, clear_rect |
| `stdlib/tui/screen.nanz` | UFCS Screen API + Phase 1 sel_* backward compat |
| `minzc/cmd/mzv/screen_host.go` | MZV sel_* host functions (stdin reader) |
| `minzc/cmd/mzv/tui_host.go` | MZV tui_* host functions (ANSI renderer) |
| `minzc/pkg/nanz/meta.go` | Metafunction runtime (VM execution, block parsing) |
| `examples/nanz/tui_demo.nanz` | Level 1 demo |
| `examples/nanz/tui_screen.nanz` | Level 2 demo |
| `examples/nanz/meta_screen.nanz` | Level 3 demo |
