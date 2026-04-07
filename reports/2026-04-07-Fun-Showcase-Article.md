# MinZ Fun Showcase: Modern Languages, Vintage Hardware

**27 programs. 3 languages. 1 Z80 backend. Zero-cost abstractions on 3.5 MHz.**

*April 2026*

---

MinZ compiles modern language constructs — algebraic data types, pattern matching, pipe operators, zero-cost interfaces, iterator fusion, tail recursion — into Z80 assembly that a human would be proud to have written by hand.

This article walks through the `fun/` showcase directory, explaining what each example demonstrates, why it matters, and what the compiler does to make it fast.

All ASM shown here is real compiler output, verified against the MIR2 VM.

---

## 1. The Headline: ADT Option Type

**`adt_option.nanz`** — 5 asserts, all pass

The Option type is the poster child of type-safe programming: no null pointer exceptions, no sentinel values, no undefined behavior. On Z80? That sounds expensive. Let's see.

```nanz
enum Option { None, Some(u8) }

fun unwrap_or(opt: Option, def: u8) -> u8 {
    return match opt {
        Some(val) => val,    // destructure: extract payload
        None      => def,    // fallback to default
    }
}

fun safe_div(a: u8, b: u8) -> Option {
    if b == 0 { return None }
    return Some(a / b)
}
```

This is real pattern matching with payload destructuring. `Some(val) => val` binds the inner value. Now the punchline — what happens when you compose these?

```nanz
assert unwrap_or(safe_div(10, 3), 255) == 3
```

The compiler traces through the entire chain at compile time:

1. `safe_div(10, 3)` → `b != 0` → `Some(10 / 3)` → `Some(3)`
2. `unwrap_or(Some(3), 255)` → match Some → `3`
3. The assert becomes `3 == 3` → true

**Z80 output:**

```z80
test_safe_div_ok:    LD A, 3   ; the entire chain collapsed
                     RET

test_unwrap_some:    LD A, 42  ; unwrap_or(Some(42), 0)
                     RET

test_is_none:        LD A, 0   ; is_some(None)
                     RET
```

Two instructions per function. The ADT, the match, the destructuring, the division — all gone. This is what "zero-cost abstraction" actually means: you write safe code, and the compiler erases the safety scaffolding because it proved it was unnecessary.

The non-constant path (`unwrap_or` called at runtime) is also efficient:

```z80
unwrap_or:
    LD A, L / CP 1         ; check tag: 1 = Some
    JR NZ, .else           ; not Some → use default
    LD A, C / RET           ; Some(val) → return payload from C
.else:
    ...                     ; None → return def
```

Tag check → branch → payload extraction. No heap allocation, no boxing, no vtable lookup. The ADT is a (tag, payload) pair in registers.

**Why this matters:** Rust and Swift have Option. Haskell has Maybe. Now a Z80 at 3.5 MHz has it too, and it compiles to the same code you'd write by hand if you didn't have the abstraction.

---

## 2. Pipe Operator: Functional Composition

**`pipes.frl`** — Frill language, 9 asserts, all pass

Frill is MinZ's ML-style frontend. It has the pipe operator `|>` and function composition `>>`:

```frill
let double (x : u8) : u8 = x + x
let inc    (x : u8) : u8 = x + 1
let sq     (x : u8) : u8 = x * x

let pipe_dbl_inc (x : u8) : u8 = x |> double |> inc
let dbl_then_inc = double >> inc
let inc_then_dbl = inc >> double
```

Three different ways to say "double, then increment." All three compile identically:

```z80
pipe_dbl_inc:       dbl_then_inc:
    ADD A, A            ADD A, A        ; double: x + x
    INC A               INC A           ; inc: x + 1
    RET                 RET

inc_then_dbl:
    INC A               ; inc first
    ADD A, A            ; then double
    RET
```

Three instructions. Both `double` and `inc` fully inlined. The pipe operator and composition operator are completely erased — they're syntactic sugar that costs nothing.

And `sq` (square)?

```z80
sq:
    LD B, A          ; copy x to B (multiplier)
    JP __mul8        ; tail call: A = A * B
```

Two instructions + tail call to the shared multiply routine. `JP` instead of `CALL`+`RET` — the peephole optimizer saw that `sq` ends with a call and return, and fused them.

**Why this matters:** The pipe operator is beloved in Elixir, F#, and shell scripting. On Z80, it's free. You write `x |> f |> g` for readability, and the compiler generates `f` then `g` inline. No function call overhead, no intermediate values, no pipeline object.

---

## 3. Iterator Fusion: Three Operations, One Loop

**`iterator_fusion.nanz`**

This is the showpiece of MinZ's iterator chain optimization. Three operations — map, filter, forEach — fused into a single loop:

```nanz
fun filter_map_explicit(buf: u16, n: u8, threshold: u8) -> void {
    for x: u8 in buf[0..n] {
        let doubled: u8 = x * 2
        if doubled > threshold {
            process(doubled)
        }
    }
}
```

The equivalent fluent style also works:

```nanz
buf.map(|x| x*2).filter(|x| x > threshold).forEach(|x| process(x), n)
```

Both produce the same Z80:

```z80
filter_map_explicit:
    LD B, C                    ; B = loop counter (n)
    LD C, D                    ; C = threshold
.loop:
    LD A, B / AND A / RET Z    ; n == 0? done
    LD D, (HL)                 ; load buf[i]
    LD A, D / ADD A, A         ; doubled = x * 2
    LD D, A
    LD A, C / CP D             ; compare: threshold vs doubled
    CALL C, process            ; CONDITIONAL CALL — only when doubled > threshold
    INC HL                     ; advance pointer
    DEC B / JRS .loop          ; next element
```

Notice `CALL C, process` — this is a Z80 conditional call instruction. The processor evaluates the carry flag from the preceding `CP` and either executes or skips the CALL in a single instruction. No branch, no jump-over, no wasted cycles when the filter doesn't pass.

MinZ detects this pattern at three levels:

1. **MIR2 level:** `FoldConditionalCalls` finds a `BrIf → single-call-block → join` CFG pattern and rewrites it to `OpCallCond`
2. **Peephole level:** `foldCondCall` catches `JR cc, +N / CALL target / label:` patterns in the assembly text
3. **Grace level:** A declarative CFG rule matches the same pattern for the Grace optimization path

The lambda `|x| x*2` is completely inlined — no function pointer, no closure object, no call overhead. The iterator chain `.map().filter().forEach()` produces the same code as the explicit `for` loop. Zero-cost abstractions.

**Why this matters:** Java 8 streams, Rust iterators, Kotlin sequences — they all fuse. But they all run on CPUs with branch predictors, speculative execution, and gigabytes of memory. MinZ fuses iterator chains on a CPU from 1976 with 64KB of RAM and no branch prediction. And it generates a conditional CALL that many assembly programmers don't even know exists.

---

## 4. Zero-Cost Interfaces: OOP Without Vtables

**`oop_shapes.nanz`** — traits + impl blocks

```nanz
struct Rect { kind: u8, w: u8, h: u8 }
struct Circle { kind: u8, radius: u8 }
trait Shape { area, perimeter }

impl Shape for Rect {
    fun area(self) -> u8 { return self.w * self.h }
    fun perimeter(self) -> u8 { return (self.w + self.h) * 2 }
}

impl Shape for Circle {
    fun area(self) -> u8 { return 3 * self.radius * self.radius }
    fun perimeter(self) -> u8 { return 6 * self.radius }
}
```

MinZ uses CTIE (Compile-Time Interface Execution) to resolve `rect.area()` to `Rect_area(rect)` at compile time. No vtable, no indirect call, no dynamic dispatch:

```z80
Rect_area:
    EX DE, HL              ; save self pointer
    LD BC, 2 / ADD HL, BC  ; HL = &self.w
    LD B, (HL)             ; B = width
    LD H, D / LD L, E      ; restore self pointer
    LD BC, 3 / ADD HL, BC  ; HL = &self.h
    LD E, (HL)             ; E = height
    LD A, B / LD B, E
    JP __mul8              ; tail call: A = w * h

Rect_perimeter:
    ...
    LD A, (HL) / RLA       ; w * 2 (rotate left = shift left 1)
    LD C, A
    ...
    LD A, (HL) / RLA       ; h * 2
    ADD A, C               ; (w * 2) + (h * 2) = perimeter
    RET
```

Three things to notice:

1. **`JP __mul8`** instead of `CALL __mul8 / RET` — tail call optimization. The multiply routine's `RET` returns directly to `Rect_area`'s caller. Saves 17 T-states (one CALL instruction).

2. **`RLA`** for `*2` — the compiler chooses the single-byte rotate-left-through-carry instruction. Same speed as `ADD A,A` (4T) but chains better for `*4`, `*8`.

3. **`__mul8` is shared** — emitted once for the entire module. Every `*` operation on u8 calls this 80T routine. GPU-proven optimal.

**Why this matters:** C++ invented zero-cost abstractions. Rust refined them. MinZ proves they work on hardware where every byte and every T-state counts. A `rect.area()` call in MinZ compiles to `CALL Rect_area` — the exact same instruction you'd write in hand-coded Z80 assembly. The trait declaration, the impl block, the method dispatch — all erased.

---

## 5. Tail Recursion: Infinite Depth on 256 Bytes

**`tail_recursion.nanz`** — asserts verified

```nanz
fun fib_tail(n: u8, a: u8, b: u8) -> u8 {
    if n == 0 { return a }
    return fib_tail(n - 1, b, a + b)
}

assert fib_tail(10, 0, 1) == 55
```

The Grace tail-recursion-elimination rule detects that `fib_tail` calls itself in tail position and replaces `CALL fib_tail / RET` with `JP fib_tail`:

```z80
fib_tail:
    AND A                  ; test n
    JRS NZ, .continue      ; n != 0 → keep going
    LD A, B / RET           ; n == 0 → return a (in B)
.continue:
    DEC A                  ; n - 1
    LD A, B / ADD A, C     ; a + b
    LD B, A                ; new accumulator
    LD E, C / LD C, B / LD B, E  ; swap: b→a, a+b→b
    JP fib_tail            ; loop back, no CALL
```

`JP fib_tail` — a jump, not a call. No stack frame pushed, no return address saved. `fib_tail(255, 0, 1)` runs on 0 bytes of stack. On a Z80 with a 256-byte stack, this is the difference between "works" and "crashes."

The test module has four tail-recursive functions — Fibonacci, factorial, power-of-2, and sum — all converted to loops.

**Why this matters:** Scheme mandates tail call optimization. Haskell gets it by laziness. Most C compilers do it as an optimization. MinZ does it at the Grace level — a declarative CFG pattern rule that matches "block ends with self-call + ret" and rewrites it. The rule is 6 lines of Grace DSL.

---

## 6. State Machine: Enum + Match

**`state_machine.nanz`** — 13 asserts, all pass

A game entity state machine: Idle → Walking → Jumping → Dead, with transitions driven by input:

```nanz
enum State { Idle, Walking, Jumping, Dead }

fun next_state(s: State, input: u8) -> u8 {
    if (s == State.Idle) {
        return match input {
            1 => State.Walking,
            2 => State.Jumping,
            3 => State.Dead,
            _ => State.Idle,
        }
    }
    ...
}

fun is_alive(s: State) -> u8 {
    return match s {
        Dead => 0,
        _    => 1,
    }
}
```

The `is_alive` function compiles to a gem:

```z80
is_alive:
    CP 3                   ; compare s with Dead (= 3)
    JR Z, .dead            ; equal → dead
    LD A, 1 / RET           ; alive
.dead:
    LD A, 0 / RET           ; dead
```

Five instructions. The enum is an integer. The match is a compare-and-branch. The wildcard `_` is a fallthrough. No jump tables, no lookup arrays — just the minimal compare chain that a human would write.

`state_speed` shows a multi-case match:

```z80
state_speed:                ; match s { Idle=>0, Walking=>2, Jumping=>4, Dead=>0 }
    CP 1 / JR NZ, .not1    ; Walking?
    LD A, 2 / RET            ; speed = 2
.not1:
    CP 2 / JR NZ, .not2    ; Jumping?
    LD A, 4 / RET            ; speed = 4
.not2:
    LD A, 0 / RET            ; Idle or Dead → speed = 0
```

The compiler noticed that Idle (0) and Dead (3) both return 0, so it merged them into the default case. Less code, same behavior.

---

## 7. Widemath: Operator Overloading for Z80

**`widemath.nanz`** — 26 functions, 31 asserts, all pass

```nanz
fun *(a: u8, b: u8) -> u16 { ... }   // widening multiply: u8 × u8 → u16

assert sat_add(200, 100) == 255       // saturating add (clamps at 255)
assert abs_diff(200, 50) == 150       // |a - b| without underflow
assert brightness_blend(100, 200, 128) == 150  // linear interpolation
```

Scalar operator overloading: when you write `a * b` and both are `u8`, MinZ dispatches to the widening multiply that returns `u16`. No template instantiation, no generic bloat — just multi-dispatch by operand type.

The library includes saturating arithmetic (`sat_add`, `sat_sub`), absolute value, sign, min/max, clamp, and pixel blending — everything a demoscene programmer needs, verified against the VM.

---

## 8. Bit Intent: Native Bit Manipulation

**`bit_intent.nanz`**

```nanz
fun set_ptr_bit() -> u8 {
    var p: ptr = &flags_g
    p^.4 = 1              // set bit 4 through a pointer
    return p^.4            // read bit 4 back
}
```

The `.N` syntax works on both scalars (`x.4`) and dereferenced pointers (`p^.4`). It compiles directly to Z80's native BIT/SET/RES instructions:

- `p^.4 = 1` → `SET 4, (HL)` — a single instruction that sets bit 4 of the byte at address HL
- `p^.4` → `BIT 4, (HL)` — test bit 4, result in zero flag

No mask calculation, no AND/OR dance, no temporary variables.

---

## 9. Tuple Returns: Multiple Values Without Structs

**`tuple_return.nanz`** / **`triple_return_skip.nanz`**

```nanz
fun minmax(a: u16, b: u16) -> (u16, u16) {
    if a <= b { return (a, b) }
    return (b, a)
}

let (lo, hi) = minmax(x, y)
let (first, _, last) = stats3(x, y, z)  // skip middle value with _
```

Multiple return values via register pairs. The blank identifier `_` tells the compiler to skip a value at zero cost — no register allocated, no move emitted.

---

## 10. Generative Art: LFSR Cascade

**`che_cascade.nanz`** / **`che_intro.nanz`** / **`che_nanz.nanz`**

The Che Guevara portrait series uses LFSR-16 pseudo-random sequences to place pixels. 64 layers, each with a different seed, XOR their pixels together. The final image emerges from the interference:

```nanz
fun xor_pixel(x: u8, y: u8) -> void {
    let addr: u16 = 0x4000 + y7*2048 + y2_0*256 + y5_3*32 + xbyte
    let p: ^u8 = addr
    p^ = p^ xor mask    // flip pixel via pointer XOR
}
```

The ZX Spectrum's screen memory layout requires splitting Y into three bit fields (y7, y2_0, y5_3) and combining them with shifts. MinZ handles this naturally through pointer arithmetic — `p^ = p^ xor mask` is a read-modify-write through a computed address.

The `fun/fun/` subdirectory contains the animation player variant that reads frame data from binary files and plays back the cascade in real time.

---

## 11. Raymarcher: 3D Rendering on Z80

**`raymarcher.nanz`** — 2816 lines of Z80 output

The largest program in the showcase: a signed distance field raymarcher that renders a sphere-minus-box CSG scene on the ZX Spectrum screen.

```nanz
struct Vec3 { x: i16, y: i16, z: i16 }

fun scene(p: Vec3) -> i16 {
    let sphere: i16 = sdf_sphere(p, 150)
    let box: i16 = sdf_box(p, Vec3{x: 100, y: 100, z: 100})
    return sdf_subtract(sphere, box)
}
```

Fixed-point 8.8 arithmetic, SDF primitives (sphere, box, subtraction), surface normal estimation via central differences, basic shading. All in ~200 lines of Nanz that compile to ~2800 lines of Z80 assembly.

---

## 12. Frill Graphics: Functional Pixel Art

**`frill_graphics.frl`** — ML-style pattern generators

```frill
let sierpinski (x : u8) (y : u8) = if (x & y) == 0 then 1 else 0
let xor_texture (x : u8) (y : u8) = x ^ y
let checker (x : u8) (y : u8) = ((x / 8) + (y / 8)) & 1
```

Each pattern generator is a pure function from (x, y) → pixel. Sierpinski triangle in one line. XOR texture in one expression. The beauty is that these are fully typed, fully checked Frill functions that compile to minimal Z80 — no function call overhead since they're inlined at usage sites.

---

## The Numbers

| Program | Language | Functions | Asserts | ASM Lines |
|---------|----------|-----------|---------|-----------|
| adt_option | Nanz | 8 | 5/5 ✅ | 131 |
| pipes | Frill | 10 | 9/9 ✅ | 108 |
| iterator_fusion | Nanz | 3 | -- | 81 |
| oop_shapes | Nanz | 8 | -- | 213 |
| tail_recursion | Nanz | 8 | ✅ | 165 |
| state_machine | Nanz | 3 | 13/13 ✅ | 115 |
| widemath | Nanz | 26 | 31/31 ✅ | 566 |
| raymarcher | Nanz | ~40 | -- | 2816 |
| sha256 | Nanz | 15 | -- | 271 |
| vectors | Nanz | 16 | -- | 836 |
| bit_intent | Nanz | 5 | -- | 92 |
| tuple_return | Nanz | 4 | ✅ | 89 |
| frill_showcase | Frill | 39 | -- | 634 |
| frill_graphics | Frill | ~20 | -- | 431 |
| che_cascade | Nanz | 12 | -- | 554 |
| *...11 more* | | | | |
| **Total** | **3 langs** | **~200** | **58+ ✅** | **~8000** |

**27 out of 27 programs compile.** All assert programs verified. Zero failures.

---

## What Makes MinZ Different

Most Z80 compilers (SDCC, z88dk, HiSoft C) translate C to Z80. They handle the language C gives them: no ADTs, no pattern matching, no closures, no pipe operators, no iterator fusion.

MinZ starts from the other end: what abstractions do modern programmers want? Then it figures out how to make them free on Z80.

- **ADTs** compile to tag + payload in registers — no heap allocation
- **Pattern matching** compiles to compare chains — no jump tables when cases are sparse
- **Pipe operators** inline completely — zero overhead
- **Iterator chains** fuse into a single loop — no intermediate arrays
- **Interfaces** dispatch at compile time — no vtables
- **Tail recursion** becomes a jump — no stack growth
- **Conditional calls** use Z80's native `CALL cc` — no branch overhead

The compiler doesn't just translate. It *understands* — and erases what it understands is unnecessary.

---

*MinZ: Modern programming abstractions with zero-cost performance on vintage Z80 hardware.*
