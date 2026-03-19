# TUI Sprint: Phase 4-6 — From Selection Screen to Window Manager

**Date:** 2026-03-19
**Goal:** Transform the TUI framework from a selection screen into a real application platform — screen navigation, reactive fields, tiling layout, widgets.

---

## Architecture: Elm-Style Model-View-Update

```
State → View(State) → Event → Update(State, Event) → new State → ...
```

Every screen is a pure function of state. No side effects in rendering. Events transform state, state drives rendering. Dirty tracking ensures only changed cells redraw.

This is ABAP's PBO/PAI formalized, React's reconciliation simplified, SwiftUI's `body` without closures — and it fits Z80 perfectly (no GC, no closures, no heap).

---

## Phase 4: Screen Stack & Navigation (THIS SPRINT)

### Goal
Multi-screen applications. `call_screen()` pushes, `leave_screen()` pops. Like ABAP `CALL SCREEN` or Turbo Vision dialog stack.

### Deliverables

- [ ] **Screen stack** (max 8 deep) — global array of Screen pointers
  - `screen_push(scr)` — push current screen, switch to new
  - `screen_pop()` — return to previous screen
  - `screen_depth() → u8` — current stack depth

- [ ] **Event loop** — centralized, drives all screens
  - `app_run(initial_screen)` — main loop: render → wait_key → dispatch
  - Renders top-of-stack screen
  - Routes events to focused screen
  - Handles global keys (F3=back always pops)

- [ ] **Navigation between screens**
  - From `@screen` metafunction: `button "Details" key F5 goto detail_screen`
  - From code: `screen_push(&detail_scr)` inside event handler

- [ ] **Example: Mini Norton Commander**
  - Screen 1: File list (hardcoded)
  - Screen 2: File viewer (shows selected file info)
  - F5 = view, F3 = back, F10 = quit

### Implementation

**New file: `stdlib/tui/app.nanz`**
```nanz
global _screen_stack: [^Screen; 8]
global _screen_depth: u8

fun screen_push(scr: ^Screen) -> void {
    _screen_stack[_screen_depth] = scr
    _screen_depth = _screen_depth + 1
}

fun screen_pop() -> void {
    if _screen_depth > 0 {
        _screen_depth = _screen_depth - 1
    }
}

fun app_run(initial: ^Screen) -> void {
    screen_push(initial)
    while _screen_depth > 0 {
        var scr: ^Screen = _screen_stack[_screen_depth - 1]
        scr.render()
        var key: u8 = tui_read_key()
        if key == KEY_F3 { screen_pop() }
        // ... dispatch to screen's handler
    }
}
```

---

## Phase 5: Reactive Fields & Validation (NEXT)

### Goal
Fields auto-update when bound variables change. Validation before submit.

### Deliverables

- [ ] `@bind(variable)` — two-way binding between field and global
- [ ] `on_change` callback — called when field value changes
- [ ] Field validation: `required`, `min`/`max` for integers, `pattern` for strings
- [ ] Visual feedback: invalid fields highlighted in red
- [ ] `SY-SUBRC` set after validation (ABAP compat)

### Design

```nanz
// @bind links a field to a variable — changes propagate both ways
global customer_id: u16 = 0

scr.add_int_bound(c"Customer", &customer_id)
// After show(): customer_id already has the user's input
// During render: field shows current value of customer_id
```

---

## Phase 6: Layout & Widgets (LATER)

### Goal
Tiling window manager with reusable widgets. Norton Commander / Midnight Commander style.

### Deliverables

- [ ] **Tiling layout**
  ```nanz
  var layout: Split = hsplit(
      file_list(),    // left panel (50%)
      file_view(),    // right panel (50%)
      50              // ratio
  )
  ```

- [ ] **Widgets**
  - `TextBox` — single-line editable text
  - `ListView` — scrollable list with cursor
  - `ProgressBar` — visual progress indicator
  - `StatusBar` — bottom line with key hints
  - `MenuBar` — top line with dropdown menus
  - `Dialog` — modal popup over current screen

- [ ] **Focus management**
  - TAB cycles through focusable widgets
  - Arrow keys within focused widget
  - Focus ring stored as linked list or array

- [ ] **Dirty tracking**
  ```nanz
  global dirty: [u32; 24]   // 32-bit bitmask per row
  // Only redraw cells where dirty bit is set
  // Mark dirty on: field edit, focus change, scroll
  ```

- [ ] **Differential rendering**
  - Compare new vs old screen state
  - Emit minimal ANSI sequences (cursor jumps over unchanged regions)
  - On ZX Spectrum: only update changed attribute cells

---

## Inspiration & References

| Source | What to steal |
|--------|--------------|
| **Turbo Vision** (Borland) | Event routing, modal dialogs, clip regions, owner-draw |
| **Midnight Commander** | Dual-panel layout, built-in viewer/editor |
| **React** | Component model, props/state separation, keys for list diff |
| **SwiftUI** | Declarative body, @State/@Binding, environment values |
| **Elm** | Model-View-Update, Cmd for side effects, Sub for events |
| **Scorpion profROM** | Resident WM, hotkey switching, memory bank management |
| **bubbletea (Go)** | Model interface, Update/View methods, tea.Cmd |
| **ncurses/cdk** | Windows, panels, forms library, field types |

---

## Success Criteria

### Phase 4 (this sprint)
- [ ] Mini Norton Commander runs on MZV
- [ ] Same binary runs on CP/M (VT100 terminal)
- [ ] Screen stack push/pop works (F5=view, F3=back)
- [ ] `@screen` metafunction supports `goto` for navigation
- [ ] All existing TUI tests still pass

### Phase 5
- [ ] ABAP PARAMETERS with validation (required field, integer range)
- [ ] `@bind` auto-updates field display when variable changes
- [ ] Red highlight on invalid fields

### Phase 6
- [ ] Dual-panel file manager (Norton Commander clone)
- [ ] Scrollable list with 100+ items
- [ ] < 3KB Z80 binary for the full WM
- [ ] 50fps on ZX Spectrum (dirty tracking, < 10% screen redrawn per frame)

---

## Files to Create/Modify

### Phase 4 (this sprint)
| File | Action | Purpose |
|------|--------|---------|
| `stdlib/tui/app.nanz` | CREATE | Screen stack, app_run, navigation |
| `stdlib/tui/screen.nanz` | MODIFY | Add event handler callback |
| `minzc/cmd/mzv/tui_host.go` | MODIFY | Support screen stack in host |
| `examples/nanz/tui_commander.nanz` | CREATE | Mini Norton Commander demo |
| `examples/nanz/tui_multiscreen.nanz` | CREATE | Simple two-screen navigation |

---

*The screen is data. The app is a state machine. The renderer is a function of state.*
