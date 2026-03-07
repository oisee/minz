# MIR2 Development Roadmap

`pkg/mir2` — clean-room IR + register allocator + Z80 codegen,
developed in parallel with the production compiler.

---

## Phase 1 — Core pipeline (DONE)

- [x] SSA-like IR with block arguments
- [x] Interference graph + greedy register allocator
- [x] Z80CostTable with 14 register classes
- [x] Z80 codegen: 8/16-bit arithmetic, branches, loops, calls
- [x] Parallel-copy resolution (HL↔DE cycles, trampolines)
- [x] Peepholes: INC/DEC for ±1, CP imm8, NEG+ADD for rhs=A
- [x] Shadow register guard (InfCost for standard classes — ADR-0011)
- [x] E2E verified: fib, clamp, abs_diff, sum_range, mul8

## Phase 2 — Codegen quality (CURRENT)

- [ ] **DSE pass**: suppress `OpConst` emission when peephole made it dead
- [ ] **More examples**: gcd, max3, popcount, rol8, ... → inspect assembly quality
- [ ] **CmpGt/CmpLe**: proper multi-condition jump (currently only handles Z flag)
- [ ] **Multi-word ops**: 16-bit add/sub with carry, u16 comparisons
- [ ] **Call convention**: callee-save PUSH/POP scaffold for functions with locals
- [ ] **liveness across calls**: spill/reload around OpCall

## Phase 3 — Feature coverage (NEXT QUARTER)

Goal: cover enough of the old IR to run real MinZ programs through MIR2.

- [ ] Struct field access (OpField, OpFieldStore)
- [ ] Array indexing (OpIndex)
- [ ] Global variables (OpGlobal)
- [ ] String literals + @print
- [ ] Multi-function modules with cross-call convention
- [ ] u8/u16 mixed-width arithmetic (zero-extend, sign-extend)

## Phase 4 — minz → MIR2 lowering

Gating condition: Phase 3 complete + MIR2 passes all old-IR Z80 tests.

- [ ] Semantic lowering pass: MinZ AST → MIR2 instead of `pkg/ir`
- [ ] Iterator chain → DJNZ loop in MIR2 (replaces current special-case in IR)
- [ ] EXX region detection (ADR-0012) — use B'/C'/D'/E' for loop scalars
- [ ] Deprecate old `pkg/ir` → `pkg/codegen/z80.go` pipeline

## Phase 5 — Optimisation

- [ ] Dead-code elimination at MIR2 level (unreachable blocks)
- [ ] Constant propagation across blocks
- [ ] Coalescing: eliminate moves by pre-colouring interfering vregs
- [ ] PBQP allocator (if greedy proves insufficient for complex programs)
- [ ] EXX region allocation (ADR-0012)

---

## Example coverage tracker

| Example     | Verified | T-states | Notes |
|-------------|----------|----------|-------|
| fibonacci   | yes      | ~300 (n=8) | recursive |
| clamp       | yes      | ~40    | 3-arg, 2 branches |
| abs_diff    | yes      | ~20    | NEG trick |
| sum_range   | yes      | ~140 (n=10) | loop + accumulate |
| mul8        | yes      | ~220 (6×7) | shift-add loop |
| gcd         | —        | —      | Phase 2 |
| max3        | —        | —      | Phase 2 |
| popcount    | —        | —      | Phase 2 |
| rol8        | —        | —      | Phase 2 |

---

## When to write minz → MIR2?

When Phase 3 is complete and MIR2 produces correct Z80 for:
- at least 20 distinct example programs
- struct + array access
- multi-function modules

Rough estimate: after Phase 2 (~2 weeks) + Phase 3 (~4-6 weeks).
Until then, the old pipeline (`pkg/ir` + `pkg/codegen/z80.go`) stays production.
MIR2 is the sandbox where we prove out better allocation strategies first.
