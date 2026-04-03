# Context Seed: Grace Solver-Friendly Shapes

## Current position

Recent work already established:

- direct bit-intent from Nanz through MIR2 and VIR,
- `BIT/SET/RES` lowering for memory-backed and register-backed cases,
- IX/IY halves treated as first-class 8-bit registers,
- more idiomatic and more solver-friendly rewrites of `che_intro.nanz` and `che_cascade.nanz`.

The next frontier is not another isolated peephole. It is semi-automatic reshaping toward solver-friendly forms.

## Core wisdom

- IX/IY-half-aware allocation tables help, but they do not rescue bad function shape by themselves.
- Large or branchy functions often need structural simplification more than another local instruction rule.
- Grace is the best candidate for structural rewrites.
- Datalog-like facts are best used as analysis/profitability input for Grace, not as the only rewrite engine.
- ISLE-style rules are still useful for local cleanup after Grace rewrites.
- EXX is promising, but current architecture still has clear wins left before making EXX a central mechanism.

## Most useful transforms

- indexed loop -> pointer walk
- nested row/column worker -> row helper extraction
- hoist repeated address terms
- separate wide setup from byte loop
- move loop counters into colder regs or IX/IY halves when that frees hotter regs

## Register-allocation guidance

- Do not treat loop-counter placement as a purely local instruction-cost choice.
- `DJNZ` is great, but freeing `B` can be worth more than using `DJNZ`.
- IXH/IXL/IYH/IYL should be considered normal candidate 8-bit regs for counters/walkers when profitable.
- For pointer walks, compare `(HL)/(DE) + INC` against repeated `(IX+d)/(IY+d)` instead of defaulting to indexed addressing.

## Osize / Ospeed

- `Osize`: runtime calculation often beats LUTs unless LUTs compress awkward formulas.
- `Ospeed`: prefer precomputed row-base/LUT forms in hot regions.

## Recommended next implementation steps

1. Add fact extraction for indexed inner loops, repeated address terms, invariant loads, counters, and walkers.
2. Add Grace transform for affine indexed loops into pointer walks.
3. Add Grace transform for row-helper extraction.
4. Add loop-aware counter placement policy.
5. Keep EXX as a later region-only experiment after the above is measured.
