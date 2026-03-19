# Report 097 — Zero-Cost Partial Application for Nanz

**Date:** 2026-03-19
**Type:** Feature Proposal
**Related:** ADR-0035 (algebraic types & exhaustive matching)

---

## Elevator Pitch

Partial application (`add(5, _)`) on Z80 costs **5 bytes** as a trampoline,
**0 bytes** with SMC, and **0 bytes** when iterator fusion inlines it. This is
cheaper than a function pointer CALL (3 bytes). We can give Nanz Elm/Haskell-style
partial application without closures, heap allocation, or garbage collection.

---

## How Other Languages Do It

### Haskell — All Functions Are Curried

```haskell
add :: Int -> Int -> Int
add x y = x + y

add5 = add 5          -- partial application: returns a function
map (add 5) [1,2,3]   -- [6, 7, 8]
```

Every function takes one argument and returns a function. `add 5` allocates a
**closure on the heap** — a pointer to `add` plus the captured value `5`. The
garbage collector eventually frees it.

**Cost:** ~24 bytes heap per closure + GC pressure. Beautiful semantics, heavy runtime.

### Elm — Same as Haskell (GC'd closures)

```elm
add : Int -> Int -> Int
add x y = x + y

add5 = add 5
List.map (add 5) [1, 2, 3]   -- [6, 7, 8]
```

Compiles to JavaScript closures: `function(y) { return x + y; }`.
Runtime cost depends on JS engine optimization.

### Scala — Placeholder Syntax `_`

```scala
def add(x: Int, y: Int): Int = x + y

val add5 = add(5, _)           // (y: Int) => add(5, y)
List(1, 2, 3).map(add(5, _))   // List(6, 7, 8)

// Multiple placeholders:
val swap = subtract(_, _)       // (x, y) => subtract(x, y)
```

`_` in argument position creates a lambda. This is **pure syntax sugar** —
the compiler generates a closure object. JVM allocates it on heap.

### Kotlin — Explicit lambdas, no placeholder

```kotlin
fun add(x: Int, y: Int): Int = x + y

val add5 = { y: Int -> add(5, y) }   // manual lambda
listOf(1, 2, 3).map { add(5, it) }   // "it" is implicit single param
```

No placeholder syntax, but `it` in lambda gives similar feel.

### Rust — No currying, explicit closures

```rust
fn add(x: u8, y: u8) -> u8 { x + y }

let add5 = |y| add(5, y);            // closure, may be stack-allocated
vec![1, 2, 3].iter().map(|x| add(5, *x))
```

Rust closures are **stack-allocated** when possible (no heap, no GC). The
compiler monomorphizes and often inlines them entirely. Zero-cost in practice.

### C — No partial application at all

```c
uint8_t add(uint8_t x, uint8_t y) { return x + y; }

// Want add(5, _)? Write a wrapper manually:
uint8_t add5(uint8_t y) { return add(5, y); }

// Or use a macro (fragile):
#define CURRY_ADD(x) ({ uint8_t _add(uint8_t y) { return add(x, y); }; _add; })
```

No language support. Manual wrappers or GCC nested functions (non-portable).

### Summary: How Languages Handle It

| Language | Syntax | Implementation | Cost |
|----------|--------|----------------|------|
| Haskell | `add 5` (implicit) | Heap closure + GC | ~24B + GC |
| Elm | `add 5` (implicit) | JS closure | JS engine dependent |
| Scala | `add(5, _)` | JVM closure object | ~32B heap |
| Kotlin | `{ y -> add(5, y) }` | JVM closure/inline | 0-32B (inline possible) |
| Rust | `\|y\| add(5, y)` | Stack closure, monomorphized | 0B (inlined) |
| C | manual wrapper | N/A | manual |
| **Nanz (proposed)** | **`add(5, _)`** | **Trampoline / SMC / fusion** | **0-5B** |

---

## The Nanz Approach: Three Tiers

### Tier 1: Trampoline (default, always safe)

```nanz
let add5 = add(5, _)
```

Generated Z80:
```z80
add5:
    LD A, 5         ; 2 bytes — load fixed argument
    JP add          ; 3 bytes — jump to original
                    ; total: 5 bytes, fully reentrant
```

For u16:
```z80
scale_42:
    LD HL, 42       ; 3 bytes
    JP scale        ; 3 bytes — total: 6 bytes
```

**When:** General use. Function pointers, callbacks, stored in variables.

### Tier 2: SMC Patch (TSMC-enabled, non-reentrant)

```nanz
@smc fun add(a: u8, b: u8) -> u8 { return a + b }
let add5 = add(5, _)   // patches the immediate in add's body
```

Generated Z80:
```z80
add:
    LD A, 0         ; ← this byte gets patched to 5
    ADD A, C
    RET

; "Currying" = one memory write:
    LD A, 5
    LD (add+1), A   ; done. add IS add5 now.
```

**0 bytes** for the curried version. The original function becomes the curried one.
Already supported via `@smc` infrastructure.

**When:** Iterator callbacks, single-threaded hot loops, non-recursive contexts.

### Tier 3: Fusion (iterator chains, zero overhead)

```nanz
data |> map(multiply(2, _)) |> filter(greater_than(_, 10))
```

The iterator fusion optimizer:
1. Sees `multiply(2, _)` → generates `lambda_7: LD A,2 / JP multiply`
2. Sees the trampoline is trivial → **inlines it** into the DJNZ loop body
3. Result: `LD A, 2` appears directly before `CALL multiply` in the loop

```z80
.loop_body:
    LD C, (HL)          ; load array element
    LD A, 2             ; ← was trampoline, now inlined
    CALL multiply       ; multiply(2, element)
    CP 10               ; ← from filter(greater_than(_, 10))
    JR C, .skip         ; skip if < 10
    ; ... store result ...
.skip:
    INC HL
    DJNZ .loop_body
```

**0 bytes trampoline overhead.** The partial application evaporated at compile time.

**When:** `map`, `filter`, `fold` iterator chains — the common case.

---

## Real-World Use Cases

### 1. Iterator Chains (most common)

```nanz
// Current (verbose):
scores |> map(|x| x * 2) |> filter(|x| x > threshold)

// Proposed (cleaner):
scores |> map(multiply(2, _)) |> filter(greater_than(_, threshold))
```

No lambda boilerplate. Reader immediately sees "multiply by 2" and "greater than threshold".

### 2. Callback Registration (TUI/event handling)

```nanz
// Current:
fun on_key_up() -> void { move_cursor(0, -1) }
fun on_key_down() -> void { move_cursor(0, 1) }
register_handler(KEY_UP, on_key_up)
register_handler(KEY_DOWN, on_key_down)

// Proposed:
register_handler(KEY_UP, move_cursor(0, -1, _))     // wait... doesn't fit
// Better: actual callback that does nothing with the key arg:
register_handler(KEY_UP, |_| move_cursor(0, -1))    // lambda still better here
```

Partial application shines when **some args are data, some come later**.

### 3. Array/Table Operations

```nanz
// Fill array with offsets from base address
fun add_offset(base: u16, index: u8) -> u16 { return base + u16(index) }

// Generate offset table for VRAM at 0x4000
offsets |> map(add_offset(0x4000, _))

// Apply color to all sprites
sprites |> forEach(set_color(_, COLOR_RED))
```

### 4. Configuration Patterns

```nanz
fun draw_char(x: u8, y: u8, ch: u8, color: u8) -> void { ... }

// "Configure" a drawing function for a specific color
let draw_red = draw_char(_, _, _, COLOR_RED)    // 3 free args, 1 fixed
let draw_blue = draw_char(_, _, _, COLOR_BLUE)

// Use in rendering:
draw_red(10, 5, 65)    // draw 'A' at (10,5) in red
```

### 5. FAT Filesystem Operations

```nanz
fun file_read(drive: u8, cluster: u16, size: u16, buf: ^u8, max: u16) -> u16

// Bind to drive 0:
let read_disk0 = file_read(0, _, _, _, _)

// Now all reads go to drive 0 without repeating it:
let n = read_disk0(cluster, size, &buffer, 512)
```

---

## Parser Implementation Sketch

In `parse.go`, at the call-argument parsing site (lines 2893-2903):

```go
// After collecting args:
placeholders := []int{}
for i, arg := range args {
    if vr, ok := arg.(*hir.VarRefExpr); ok && vr.Name == "_" {
        placeholders = append(placeholders, i)
    }
}

if len(placeholders) > 0 {
    // Generate synthetic lambda
    lambdaName := fmt.Sprintf("_partial_%d", p.lambdaCounter)
    p.lambdaCounter++

    // Build param list from placeholder positions
    params := []hir.Param{}
    for j, pos := range placeholders {
        paramName := fmt.Sprintf("_p%d", j)
        paramTy := p.inferArgType(name, pos) // from funcSigs
        params = append(params, hir.Param{Name: paramName, Ty: paramTy})
        args[pos] = &hir.VarRefExpr{Name: paramName, Ty: paramTy}
    }

    // Emit: fun _partial_N(params...) -> retTy { return orig(args...) }
    body := &hir.ReturnStmt{Vals: []hir.Expr{
        &hir.CallExpr{Fn: name, Args: args, Ty: callTy},
    }}
    lambdaFunc := &hir.Func{Name: lambdaName, Params: params, ...}
    p.module.Funcs = append(p.module.Funcs, lambdaFunc)

    // Replace call expr with reference to the lambda
    base = &hir.VarRefExpr{Name: lambdaName, Ty: mir2.TyPtr}
}
```

**Estimated: ~50 lines in parser, ~20 lines for type inference helpers.**

The MIR2 codegen and iterator fusion already handle the rest — they see a small
function that loads a constant and calls another function, and optimize accordingly.

---

## Cost Analysis: Nanz vs. Other Languages

Partial application of `add(5, _)`, called 100 times in a loop:

| Language | Per-curry cost | Per-call cost | Total (100 calls) | Notes |
|----------|---------------|---------------|--------------------| ------|
| Haskell | 24B heap + GC | indirect call | ~2400B + GC pauses | Closure allocated per curry |
| JavaScript | ~64B heap + GC | indirect call | ~6400B + GC | V8 closure objects |
| Scala/JVM | 32B heap + GC | virtual call | ~3200B + GC | Invokedynamic |
| Rust | 0B (inlined) | direct call | 0B overhead | Monomorphized at compile time |
| **Nanz trampoline** | **5B code** | **JP + CALL** | **5B total** | One trampoline, reused |
| **Nanz SMC** | **0B** | **direct CALL** | **0B** | Function patched in place |
| **Nanz fusion** | **0B** | **inline** | **0B** | Trampoline inlined into loop |

On Z80 at 3.5MHz, the trampoline JP adds **10 T-states** per call (~2.8us). With
fusion, it's 0 T-states.

---

## Interaction with ADR-0035 Features

### Tagged unions + partial application

```nanz
type Shape = Circle(u8) | Rect(u8, u8)

fun area(s: Shape) -> u16 {
    match s {
        Circle(r) -> multiply(r, r)     // u8 * u8
        Rect(w, h) -> multiply(w, h)
    }
}

// Partial application on a constructor:
let unit_circle = Circle       // no args → constructor as function
let small_rect = Rect(2, _)    // fix width=2, height free

let r: Shape = small_rect(5)   // Rect(2, 5)
```

Constructors are functions. Partial application works on them too.

### Exhaustive matching + curried callbacks

```nanz
type Event = KeyPress(u8) | MouseClick(u8, u8) | Quit

fun handle(e: Event, screen: ^Screen) -> void {
    match e {
        KeyPress(k) -> process_key(k, screen)
        MouseClick(x, y) -> click_at(x, y, screen)
        Quit -> shutdown(screen)
    }
}

// Bind screen, leave event free:
let handler = handle(_, &my_screen)

// Event loop:
while running != 0 {
    let ev: Event = poll_event()
    handler(ev)
}
```

---

## Summary

| What | Effort | Value |
|------|--------|-------|
| `_` placeholder parsing | ~50 LOC | Elm-style syntax in Nanz |
| Trampoline codegen | ~100 LOC (or 0 — lambda infra does it) | 5B per curry, reentrant |
| SMC curry detection | ~50 LOC (reuse @smc) | 0B per curry, hot paths |
| Fusion inlining | Already exists | 0B, 0T overhead |
| **Total** | **~200 LOC** | **Zero-cost partial application on Z80** |

The `_` placeholder desugars to a lambda at parse time. Everything downstream
(HIR lowering, MIR2 codegen, iterator fusion, TSMC) already works. We're adding
syntax sugar on top of a foundation that's already there.

**Recommendation:** Implement `_` placeholder (Phase 3a in ADR-0035). It's the
highest value-to-effort ratio feature in the proposal — 200 LOC for Elm-grade
partial application on an 8-bit CPU from 1976.
