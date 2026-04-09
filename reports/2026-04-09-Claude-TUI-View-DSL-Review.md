# Review: TUI View DSL Proposal

**Reviewer:** Claude (minz session)
**Document:** `reports/2026-04-09-TUI-View-DSL-Proposal-RU.md`
**Scope:** Bounded critique per codex request — syntax, IR, Z80 risk, widget set.

---

## 1. Syntax Ergonomics for IRC-like Apps

**The `@screen` block syntax is good.** The target example:

```nanz
@screen("MinZ IRC") {
    status top bind status_text
    hsplit {
        log main bind chat_lines, chat_count
        list side width 18 bind nick_items, nick_count
    }
    input bottom prompt "> " bind input_buf, input_len
}
```

This reads naturally — a human can look at it and see the screen layout. Four observations:

**A. `bind` keyword overloaded.** `status` binds one buffer. `log` binds array + count. `list` binds array + count. `input` binds buffer + length. The bind arity is implicit per widget type. This is fine for 4 widgets, but consider making it explicit:

```nanz
log main bind(data: chat_lines, count: chat_count)
input bottom bind(buf: input_buf, len: input_len) prompt "> "
```

Named bindings prevent mix-ups when widgets have similar signatures.

**B. `main` and `side` are position hints, not names.** In the `hsplit`, `log main` means "log takes the main (larger) area" and `list side` means "list takes the side panel". This works for two-panel split. For three panels, what? Consider `weight` instead:

```nanz
hsplit {
    log weight 3 bind ...
    list weight 1 bind ...
}
```

Or keep `main`/`side` as sugar for `weight 3`/`weight 1`.

**C. Missing: key bindings.** IRC needs: Enter → send, Up/Down → scroll log, Tab → switch panels. The DSL doesn't show where key dispatch goes. Suggestion: `on_key` clause per widget or per screen:

```nanz
input bottom bind ... {
    on_enter: send_message()
    on_up: scroll_log(-1)
}
```

Or keep key handling in pure Nanz code outside the DSL (simpler, more flexible).

**D. Missing: color scheme.** `log` lines have `kind` and `color` fields in `ChatLine`, but there's no DSL syntax for "render nick in color X, text in white, joins in green". This is a rendering policy. Two options: (a) hardcode in widget runtime, (b) add `style` clause. Recommend (a) for now — keep DSL about layout, not styling.

---

## 2. Missing IR/Runtime Pieces

**The IR sketch is too thin.** `screen_new`, `screen_add_log`, `screen_emit` — this is a builder API, not an IR. An IR should be inspectable and transformable. What's missing:

**A. Geometry resolution.** Who computes `x, y, w, h` for each widget? The DSL says `hsplit` but doesn't say how to split. Options:
- Compile-time: `@screen` metafunction resolves geometry at compile time for fixed terminal size (80x24). Simple, deterministic, no runtime layout engine.
- Runtime: widgets query `tui_width()`/`tui_height()` and divide. Flexible, but needs a layout algorithm on Z80.

**Recommendation: compile-time for v1.** IRC client targets 80x24. Hardcode geometry in the metafunction. Add runtime layout later.

**B. Redraw strategy.** The proposal says "rendering in tiny runtime widgets" but doesn't specify WHEN to redraw. Options:
- Every main loop tick: simple but wasteful (redraw 80x24 = 1920 chars per tick).
- Dirty flags: each `append_chat_line()` sets `log_dirty = true`, redraw only dirty widgets. Efficient.
- Double buffer: render to memory, diff with previous, emit only changes. Complex.

**Recommendation: dirty flags for v1.** One byte per widget. Check in main loop: `if log_dirty { render_log(); log_dirty = 0 }`.

**C. Ring buffer for log.** `chat_lines: [128]ChatLine` with `chat_count: u8` — when count reaches 128, what? Proposal mentions "append-only or ring semantics" but doesn't decide. IRC needs ring buffer (oldest lines dropped). This affects the renderer: it must handle wrap-around index.

**Decision needed:** ring buffer with `head` + `count`, or simple array with shift-up? Ring is O(1) append, shift is O(n). On Z80 with 128 lines × ~112 bytes = 14KB — shift is too slow. **Ring buffer required.**

---

## 3. Z80 Lowering Risks

**A. `ChatLine` is 114 bytes.** `kind(1) + color(1) + nick(16) + text(96) = 114`. Array of 128 = 14,592 bytes. Plus `NickItem(18) × 64 = 1,152`. Plus status(64) + input(128). Total: **~16KB for UI data alone.** On 64KB Z80, this is 25% of RAM. Manageable but tight.

**Mitigation:** reduce chat_lines to 64 (7KB) or text to 64 chars (save 4KB). Or use SoA layout internally (struct of arrays — nick array separate from text array).

**B. `render_log` must iterate visible window.** If terminal is 20 lines for log area, renderer reads 20 × 114 bytes = 2,280 bytes per frame. At 3.5MHz with LDIR (~21T per byte to read), that's ~48,000 T-states just for data access. Plus VT100 escape sequences for each line. Total: ~200,000T per full log redraw = ~57ms.

**At 20 FPS this is fine.** But redrawing every tick would eat 100% CPU. Dirty flags essential.

**C. `hsplit` geometry is compile-time trivial.** `log` gets columns 0-61, `list` gets 62-79 (width 18). No runtime division needed. This is a strength — the DSL compiles to constants.

**D. `render_list_item` for nick list.** 18-char wide column, iterate `nick_count` items, print each `nick[16]` left-aligned. Simple LDIR loop. No risk.

---

## 4. Is status/log/list/input the Right Minimal Set?

**Yes, with one addition: `separator`.** The hsplit between log and nick list needs a visible vertical bar. Without it, the two panels bleed together. A `separator` widget (draws `│` at fixed column) costs 24 bytes of output per frame and makes the UI readable.

Proposed minimal set:

| Widget | Purpose | IRC use |
|--------|---------|---------|
| `status` | Fixed text bar (1 line) | Server name, channel, mode |
| `log` | Scrolling text area (ring buffer) | Chat messages |
| `list` | Static/scrollable item list | Nick list |
| `input` | Editable text line + cursor | Message input |
| `separator` | Vertical or horizontal line | Panel divider |

**What's NOT needed for v1:** tabs, menu, scrollbar, table, panel with border. These can come later without changing the core DSL.

---

## 5. Summary

| Aspect | Verdict |
|--------|---------|
| Syntax | Good. Add named bind args, consider `weight` for splits |
| Widget set | Right. Add `separator`. Skip tabs/menu for v1 |
| IR | Too thin — needs geometry resolution strategy and redraw policy |
| Z80 risk | 16KB data is tight but OK. Dirty flags essential. Ring buffer required for log |
| Migration path | Correct order. Phase 1 (runtime) before Phase 2 (widgets) before Phase 3 (DSL) |

**The proposal is architecturally sound.** The gap is in the runtime contract details (geometry, redraw, ring buffer), not in the DSL syntax or widget choice. Fix the runtime contract, then the DSL can be implemented incrementally.

---

*Reviewed by Claude (minz session), 2026-04-09.*
