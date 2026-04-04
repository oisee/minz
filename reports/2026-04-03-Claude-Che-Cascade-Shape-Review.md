# Solver-Friendly Reshaping: che_cascade Draw Loops

**Date:** 2026-04-03
**Scope:** Read-only review of `fun/che_cascade.nanz` draw loop families
**Goal:** 3-5 source-level reshapes to reduce VIR/PBQP fallback pressure

---

## Functions Reviewed

| Function | Vregs (est.) | Calls inner | Pressure source |
|----------|-------------|-------------|-----------------|
| `xor_pixel` | 10+ | 0 (leaf) | address arithmetic: 5 divisions, 3 multiplications, all u16 |
| `xor_blk` | 3+args | 1 (`xor_pixel`) per pixel | call-heavy: blk×blk calls, 3 args live across each |
| `fill_buf` | 5+global | 1 (`lfsr16`) per iter | u16 global + u16 idx + u8 mask + pointer: wide+byte mixing |
| `apply_buf` | 5+args | 1 (`xor_blk`) per cell | address arithmetic + call with 3 args + conditional |

`xor_blk_row` / `apply_buf_row` do not exist as separate functions — the row
iteration is inline in `xor_blk` and `apply_buf`.

---

## Reshape Proposals (ranked by payoff / invasiveness)

### 1. Split `xor_pixel` address calc from XOR operation

**Problem:** `xor_pixel` has ~10 live vregs: `px`, `py`, `third`, `prow`, `crow`,
`xbyte`, `xbit`, `addr`, `bit_mask`, `s`, plus the pointer. The address formula
(`0x4000 + third*2048 + prow*256 + crow*32 + xbyte`) creates 5 intermediate u16
values all live simultaneously. This is the single highest-pressure function.

**Reshape:** Split into `screen_addr(px, py) -> u16` and `xor_at(addr, px)`.
The address function has ~6 vregs (borderline table-eligible). The XOR function
has ~4 (bit shift loop + pointer). Neither exceeds the solver comfort zone.

**Helps:** Repeated address arithmetic (5 div/mod + 3 mul all in one scope).
Wide+byte mixing (u16 addr terms mixed with u8 bit operations).

**Payoff:** High. This is the hottest function (called per-pixel) and the most
likely to PBQP-fallback due to vreg pressure.
**Invasiveness:** Low. Pure function split, no semantic change.

### 2. Replace bit-shift loop with lookup or direct mask

**Problem:** Lines 56-58 compute `bit_mask` via a while-loop (`s < xbit`), creating
loop-carried dependency with `bit_mask` and `s` both live across iterations. The
loop body is trivial but the loop structure forces the solver to handle back-edges.

**Reshape:** Replace with `bit_mask = 1 << (7 - (px % 8))`. If shift-left isn't
available, use a 8-byte lookup table:
```nanz
let masks: ^u8 = MASK_TABLE_ADDR  // [128,64,32,16,8,4,2,1]
let bit_mask: u8 = (masks + (px % 8))^
```

**Helps:** Eliminates a loop entirely from the hottest function. Removes 2 loop-
carried vregs (`bit_mask`, `s`). The lookup version is 1 load + 1 add.

**Payoff:** High. Eliminates loop codegen complexity in the inner function.
**Invasiveness:** Low. 3 lines → 2 lines.

### 3. Hoist buffer pointer to row-start in `apply_buf`

**Problem:** Line 84 recomputes `0xC000 + by * 32 + bx` on every inner iteration.
`by * 32` is invariant within the inner loop but the compiler sees the full
expression each time, creating repeated u16 address arithmetic mixed with u8
loop vars.

**Reshape:** Compute row base pointer once per outer iteration:
```nanz
fun apply_buf(ox: u8, oy: u8, blk: u8) -> void {
    var row_ptr: u16 = 0xC000
    var by: u8 = 0
    while by < 24 {
        var bx: u8 = 0
        while bx < 32 {
            let p: ^u8 = row_ptr + bx
            if p^ != 0 {
                xor_blk(ox + bx * blk, oy + by * blk, blk)
            }
            bx = bx + 1
        }
        row_ptr = row_ptr + 32
        by = by + 1
    }
}
```

**Helps:** Repeated address arithmetic — eliminates `by * 32` multiply from inner
loop. Pointer-walk friendliness — `row_ptr + bx` is a simple u16+u8 add vs
a full multiply-accumulate. Reduces u16 intermediates in the inner scope.

**Payoff:** Medium. Saves one u16 multiply per inner iteration (32×24 = 768 times).
**Invasiveness:** Low. Same structure, just hoisted computation.

### 4. Pointer-walk `fill_buf` instead of indexed addressing

**Problem:** Line 34 computes `0xC000 + idx` each iteration, with `idx` as u16.
The inner loop also calls `lfsr16(g_state)` — a non-leaf call with `idx` and
`and_mask` live across it. This is the wide+byte mixing problem: u16 `idx` and
u16 `g_state` coexist with u8 `lo` and u8 `and_mask`.

**Reshape:** Replace index with pointer increment:
```nanz
fun fill_buf(and_mask: u8) -> void {
    var p: ^u8 = 0xC000
    var remaining: u16 = 768
    while remaining > 0 {
        g_state = lfsr16(g_state)
        let lo: u8 = g_state % 256
        if (lo % (and_mask + 1)) == and_mask {
            p^ = 1
        } else {
            p^ = 0
        }
        p = p + 1
        remaining = remaining - 1
    }
}
```

**Helps:** Pointer-walk friendliness — `p = p + 1` is INC HL (6T) vs recomputing
`0xC000 + idx` each time. Reduces one u16 add from the inner loop. Still has
the `lfsr16` call with live-across pressure, but one fewer u16 vreg helps.

**Payoff:** Medium. One fewer u16 vreg in the hot loop.
**Invasiveness:** Very low. Mechanical transformation.

### 5. Extract `xor_blk` row body as `xor_row`

**Problem:** `xor_blk` has a nested loop calling `xor_pixel(px+dx, py+dy)` —
3 args (`px+dx`, `py+dy`, implicit from `xor_pixel`) with `dy`, `dx`, `px`,
`py`, `blk` all live in the outer scope. The call to `xor_pixel` keeps `dy`,
`blk`, `px`, `py` live across it.

**Reshape:** Extract inner loop:
```nanz
fun xor_row(px: u8, y: u8, blk: u8) -> void {
    var dx: u8 = 0
    while dx < blk {
        xor_pixel(px + dx, y)
        dx = dx + 1
    }
}

fun xor_blk(px: u8, py: u8, blk: u8) -> void {
    var dy: u8 = 0
    while dy < blk { xor_row(px, py + dy, blk)  dy = dy + 1 }
}
```

**Helps:** Call-heavy inner loops — `xor_row` has only 3 vregs + call (`dx`, `px`,
`blk`, plus `y` consumed before call). `xor_blk` drops from 5 live-across to 3
(`px`, `py+dy` computed, `blk`).

**Payoff:** Low-medium. Adds one CALL/RET per row but reduces solver pressure in
both functions. Net win depends on block size (blk=4: 4 extra CALLs, blk=8: 8).
**Invasiveness:** Low. Mechanical extraction.

---

## Summary Matrix

| # | Reshape | Addresses | Payoff | Invasiveness |
|---|---------|-----------|--------|-------------|
| 1 | Split xor_pixel → addr+xor | address arith, wide+byte | **High** | Low |
| 2 | Bit-mask lookup replaces loop | call-free inner loop | **High** | Low |
| 3 | Hoist row_ptr in apply_buf | address arith, pointer-walk | Medium | Low |
| 4 | Pointer-walk fill_buf | pointer-walk, wide+byte | Medium | Very low |
| 5 | Extract xor_row from xor_blk | call-heavy inner loops | Low-med | Low |

**Recommended order:** 1 → 2 → 3 → 4. Reshape 5 is optional — only if `xor_blk`
still hits PBQP after 1+2.

All reshapes are local, mechanical, and composable. None require EXX, new
language features, or architectural changes.
