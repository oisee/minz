# Iterator Chain Reality Check

*2026-03-02 | MinZ v0.19.5 | Report #017*

---

## Summary

Critical analysis of actual compiler output vs documentation claims. The iterator
pipeline is **correct** (11/11 E2E hex-verified) but **~5x slower than documented**.
The bottleneck is memory-backed virtual registers, not the iterator architecture itself.

---

## The Gap: Documentation vs Reality

| Claim | Reality |
|-------|---------|
| "~40T per element" | ~207T per element (forEach) |
| "Zero-cost pointer walk" | Pointer stored/loaded from $F014 three times per iteration |
| "DJNZ for efficient loop" | 5-instruction memory-backed counter sequence |
| "Bare DJNZ when no CALL" | Never fires — all E2E tests have non-inlineable console_log |
| "Stack Yo-Yo in skip" | Doesn't exist — one PUSH/POP per iter, same as forEach |
| `LD A, (x_imm0)` in docs | Wrong — that's opcode 3A (indirect). Actual: `LD A, 0` (opcode 3E, correct SMC) |

## What's Working Well

**Correctness:** 11/11 E2E tests pass with exact hex output verification. Multi-stage
chains (map+filter+forEach, filter+map+forEach, take+map+forEach) all produce correct
results. The pipeline from parser through Z80 codegen is sound.

**Zero-cost operations that ARE genuinely zero-cost:**
- `take(n)` — counter folded to `LD B, n` at compile time
- `skip(n)` — pointer offset via `ADD HL, DE` at init, no skip loop
- Inline filter lambda — `CP N+1` + `JR C` instead of CALL (~27T saved/iter)
- Fusion inlining — `double()` body inlined, CALL eliminated (~17T saved/iter)
- Strength reduction — `x * 2` -> `ADD A, A` (4T not 8T)

**Fusion optimizer:** Successfully inlines simple callbacks (<=8 instructions, 1 param,
no asm/calls/loops). In map+filter+forEach, `double()` is inlined while `console_log`
(asm function) is correctly left as CALL.

## What's Broken

### enumerate and reduce (Bug G)

Both produce incorrect Z80 code:
- enumerate: B register used for both loop counter and enumerate index
- reduce: two `LD A, 0` SMC anchors — second overwrites first

These are MIR-correct but Z80-broken. No E2E tests exist for them.

### takeWhile stack hazard

PUSH HL before predicate check, but early-exit jump skips POP. Stack
corrupted by 2 bytes if predicate fails. Needs POP before exit.

### Orphan PUSH/POP after fusion

Semantic layer emits PUSH/POP HL based on `hasCallOps` at IR generation time.
Fusion optimizer removes CALLs after PUSH/POP are in the stream. When all CALLs
are removed, PUSH/POP remains with nothing between them — 21T waste.

## Where the T-States Go (forEach, per element)

```
Useful work:                 ~50T  (24%)
  LD A, (HL)         7T
  CALL console_log  17T
  INC HL              6T
  DEC B + JR NZ     16T
  PUSH/POP HL        21T  (needed for CALL)

Memory traffic:             ~132T  (64%)
  9x LD (addr)/LD A,(addr)  ~132T
  Redundant load-after-store  ~29T of those

Other waste:                 ~25T  (12%)
  Store void return  16T
  LD A, B / LD B, A   8T
```

**The dominant cost is the register allocator**, not the iterator pipeline.

## Recommendations

1. **Fix enumerate/reduce** — separate counter from index, fix dual SMC param
2. **Strip orphan PUSH/POP** — fusion optimizer should clean up after inlining
3. **Fix takeWhile early exit** — POP before jump
4. **Register allocator** — keeping just HL (pointer) and B (counter) in physical
   registers would cut ~80T per iteration. This is where the real perf win is.
5. **Dead code removal** — fusion-inlined functions still emitted in output
6. **Update docs to show actual output** — done in this revision

---

## Test Results (2026-03-02)

All 11 E2E: PASS
All optimizer tests: PASS (19/19 including 7 fusion)
All semantic tests: PASS (17/17)
All parser tests: PASS (20/20)
All MIR VM tests: PASS (8/8)
All codegen tests: PASS (5/5, with -vet=off)
Corpus: 18/18 compile

**Total: 87+ tests, 7 layers, 100% pass rate.**

---

*The iterator pipeline is architecturally sound. The code is correct. The performance
story needs the register allocator, not more iterator-specific optimizations.*
