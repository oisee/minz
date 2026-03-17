# Report #090 — Universal TUI Framework: Three Levels of Screen Abstraction

**Date:** 2026-03-17
**Status:** Phase 1-2 ✅ DONE, Phase 3 🚧 WIP (metafunctions)

---

## Summary

Built a universal screen/TUI framework for Nanz that works across **all backends** — CP/M (BDOS), MZV (VM), and future QBE/Web. The screen is **data, not code**: the same ABAP or Nanz program renders correctly whether it runs as Z80 binary on CP/M or as MIR2 bytecode on the VM.

Three levels of API, each built on top of the previous:

| Level | API Style | Example | Status |
|-------|-----------|---------|--------|
| **L1** | Procedural | `sel_register_str(...); sel_show()` | ✅ Done |
| **L2** | OOP (UFCS) | `scr.add_field(...); scr.show()` | ✅ Done |
| **L3** | Declarative DSL | `@screen("title") { field "X" }` | 🚧 WIP |

---

## Level 1: sel_register / sel_show (Procedural)

### Architecture

```
sel_register_str("P_NAME", 20, "World", &buf)
sel_register_int("P_COUNT", 3)
rc = sel_show()    // returns: 1=MZV handled, 0=Z80 fallback
```

**Dual-path dispatch:** `sel_show()` returns 1 on MZV (host function reads stdin, writes to VM heap) or 0 on Z80 (fallback to inline BDOS prompts). Same binary, both backends.

### CP/M Output (Z80 → mze)

```
$ printf 'Z80\n\n' | mze hello_input.com -t cpm
P_NAME [World]:
P_COUNT [3]:

Hello, Z80!
```

### MZV Output (MIR2 VM)

```
$ printf 'Z80\n\n' | mzv -H hello_input.abap
Hello, Z80!

$ mzv -H hello_input.abap  # no input → uses defaults
Hello, World!
```

### Key Insight: Anti-Inlining

Z80 stubs for `sel_show`/`sel_register_*` use inline asm (`NOP`/`XOR A`) to prevent `InlineTrivial` from eliminating host-overridable calls. Without this, the MIR2 optimizer folds the constant-returning stub and the host function never executes.

---

## Level 2: Screen.add_field / Screen.show (OOP UFCS)

### stdlib/tui/ Module Structure

```
stdlib/tui/
├── widget.nanz    — Rect, ScreenField, Screen structs, Key/Color constants
├── render.nanz    — @extern tui_* primitives (goto, color, putch, read_key)
└── screen.nanz    — UFCS Screen API (init, add_field, add_button, show)
```

### API Example

```nanz
global scr: Screen

fun main() -> void {
    scr.init(c"Customer Master")
    scr.add_field(c"Customer", 10, &name_buf)
    scr.add_int(c"Count", 5)
    scr.add_button(c"Execute", KEY_F8)
    scr.show()
    var count: u16 = scr.get_int(1)
}
```

### TUI Rendering (tui_screen.nanz demo)

```
  Material Report                                    ← white on blue title bar
Material    [*                 ]                      ← cyan label + white input
Plant       [    ]
Count       [10]
[F8=Execute]  [F3=Back]                              ← inverted button
  TAB=Next  Enter=Edit  F8=Execute  F3=Back          ← blue status bar
```

Rendered via ANSI escape sequences to stderr. Each `tui_*` call maps to:

| Host Function | ANSI Output |
|---------------|-------------|
| `tui_goto(x,y)` | `ESC[{y+1};{x+1}H` |
| `tui_color(fg,bg,bright)` | `ESC[{30+fg};{40+bg}m` |
| `tui_putch(1)` | `┌` (box-drawing translation) |
| `tui_clear()` | `ESC[2J ESC[H` |

---

## Level 3: @screen Metafunction (Declarative DSL) — WIP

### The Vision

```nanz
fun @screen(title: ^u8) -> void {
    emit(c"fun _generated_screen() -> void {")
    var n: u8 = block_len()
    var i: u8 = 0
    while i < n {
        var kw: ^u8 = node_keyword(i)
        var label: ^u8 = node_arg_str(i, 0)
        // ... emit TUI code for each field/button
        i = i + 1
    }
    emit(c"}")
}

@screen("Material Report") {
    field "Material"
    field "Plant"
    field "Count"
    button "Execute"
}
```

### Architecture

**Metafunctions run on the MIR2 VM at compile time:**

```
@screen("title") { block }
    ↓ Go parser (parse.go)
captures raw source + parses block → []metaBlockNode
    ↓ Nanz → HIR → MIR2
metafun compiled to VM bytecode
    ↓ VM.Call("screen", args)
metafun iterates block via host functions, calls emit()
    ↓ emitted text
valid Nanz source code
    ↓ Parse → HIR
merged into caller module
```

### Host Functions for Metafunctions

| Function | Purpose |
|----------|---------|
| `emit(str)` | Append line to output buffer |
| `block_len()` | Number of nodes in DSL block |
| `node_keyword(i)` | Keyword of i-th node ("field", "button") |
| `node_arg_str(i, j)` | j-th string argument |
| `node_kwarg(i, key)` | Named keyword argument |
| `str_concat(a, b)` | String concatenation |
| `str_from_int(n)` | Integer → string |
| `str_chr(code)` | ASCII code → single-char string |
| `str_eq(a, b)` | String comparison |

### Status

Block parsing and metafunction execution pipeline work end-to-end. The `@screen` metafunction successfully generates Nanz code for all fields and buttons. Runtime string pool wiring needs one more fix for the generated code to execute cleanly.

**Verified working:** block parsing (4/4 nodes), metafunction VM execution, code emission, HIR merge.
**WIP:** emitted string references in the spliced module.

---

## Architecture Decisions

### Screen as Data

The screen descriptor is a **data table**, not code. A Nanz struct, an ABAP PARAMETERS declaration, or a Lanz S-expression all produce the **same field array**. The renderer reads this array and does the platform-appropriate thing.

### SwiftUI-Inspired Model

| SwiftUI | ABAP | Nanz (Phase 2+) |
|---------|------|-----------------|
| `@State var` | `DATA` + PBO | `@state global` (future) |
| `$binding` | `FIELD ... MODULE` | `@bind(var)` (future) |
| `body: some View` | PBO module | `Screen.render()` |
| `Button { action }` | PAI + SY-UCOMM | `add_button(label, KEY_F8)` |

### Tiling WM Potential

The `tui_*` primitives (goto, color, putch, clear_rect) are sufficient for a **tiling window manager** on Z80:

```
┌─ Editor ──────────────────┬─ Files ─┐
│ fun main() -> void {      │ main.nz │
│     tui_clear()           │ lib.nz  │
│     ...                   │ test.nz │
├─ Output ──────────────────┤         │
│ Compiled OK (3 functions) │         │
└───────────────────────────┴─────────┘
```

Dirty-region tracking with 24×32-bit bitmask. HALT-synced 50fps. ~2-3KB Z80 code.

---

## Files Created/Modified

### New Files (8)

| File | LOC | Purpose |
|------|-----|---------|
| `stdlib/tui/widget.nanz` | 75 | Base types: Rect, ScreenField, Screen, constants |
| `stdlib/tui/render.nanz` | 95 | @extern tui_* rendering primitives |
| `stdlib/tui/screen.nanz` | 200 | UFCS Screen API + Phase 1 backward compat |
| `minzc/cmd/mzv/screen_host.go` | 195 | MZV Phase 1 host (stdin reader) |
| `minzc/cmd/mzv/tui_host.go` | 210 | MZV Phase 2 host (ANSI renderer) |
| `minzc/pkg/nanz/meta.go` | 330 | Metafunction runtime (VM execution, block parsing) |
| `examples/nanz/tui_screen.nanz` | 85 | Level 2 demo |
| `examples/nanz/meta_screen.nanz` | 93 | Level 3 demo |

### Modified Files (4)

| File | Changes | Purpose |
|------|---------|---------|
| `minzc/pkg/nanz/parse.go` | +210 | `fun @name` + `@name() {}` + lexer save/restore |
| `minzc/pkg/abap/lower.go` | +140 | sel_register + internStr dedup fix |
| `minzc/cmd/mzv/main.go` | +6 | Register TUI + screen hosts |
| `minzc/pkg/mir2/vm.go` | +8 | WriteHeap → WriteHeapBytes consolidation |

---

## Verification Matrix

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| ABAP defaults on MZV | `mzv -H hello_input.abap` | `Hello, World!` | ✅ |
| ABAP piped on MZV | `printf 'Z80\n' \| mzv -H hello_input.abap` | `Hello, Z80!` | ✅ |
| ABAP on CP/M | `printf 'Z80\n' \| mze hello_input.com -t cpm` | `Hello, Z80!` | ✅ |
| Nanz tui_demo defaults | `printf '\n' \| mzv -H tui_demo.nanz` | `42` | ✅ |
| Nanz tui_demo piped | `printf '7\n' \| mzv -H tui_demo.nanz` | `7` | ✅ |
| Nanz tui_screen | `mzv -H tui_screen.nanz` | TUI with colors | ✅ |
| Metafunc block parse | `@test() { field "X" \n field "Y" }` | 2 nodes | ✅ |
| Metafunc code emit | `fun @hello() { emit(...) }` | spliced func | ✅ |
| ABAP unit tests | `go test ./pkg/abap/` | 3/3 pass | ✅ |
| Nanz showcase | `go test ./pkg/nanz/ -run Showcase` | tui_* pass | ✅ |

---

## Next Steps

1. **Fix Level 3 runtime** — string pool wiring for emitted code modules
2. **`@quote { }` syntax** — template blocks with `#{}` interpolation
3. **`@Block` + `@match`** — pattern matching on DSL block nodes
4. **Phase 2 TUI** — TAB navigation, cursor-in-field editing, field validation
5. **Phase 3 TUI** — multiple screens, CALL SCREEN, PBO/PAI callbacks
6. **CP/M VT100 renderer** — tui_* implemented as BDOS + escape sequences
7. **Tiling WM** — TextBox + ListView + StatusBar + split layout

---

*The screen is data, not code. One program, all platforms.*
