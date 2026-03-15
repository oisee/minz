# Ideas: mzd + Superoptimizer Pipeline

**Date:** 2026-03-15
**Status:** Ideas / brainstorm
**Participants:** Alice, neighbor (MIR2/tetris), Claude

---

## What We Have

| Tool | What it does |
|------|-------------|
| **mzd --regs** | Per-function IN/OUT/CLOBBER with provenance tracking |
| **mzd --verify-abi** | Compare detected vs declared ABI (finds codegen bugs) |
| **z80-optimizer** | 602K brute-force proven sequence equivalences (CUDA) |
| **superopt_peephole.go** | Compiler integration: hash-lookup replacement |
| **mzc** | Emits `; fun name(p: type = REG) ; clobbers: REG` |

---

## Idea 1: mzd as Sample Harvester for Superoptimizer

**Problem:** z80-optimizer searches ALL 2-instruction combos. But real-world code has patterns the brute-force never sees (3+ instructions, context-dependent).

**Idea:** Extract real instruction sequences from disassembled binaries as search targets.

```
binary → mzd → extract all N-instruction windows → deduplicate → feed to z80opt stoke
```

### Implementation
1. `mzd --extract-sequences N` — emit all N-instruction sliding windows from code sections
2. Normalize to z80-optimizer format (hex immediates, canonical mnemonics)
3. Deduplicate, count frequency (hot patterns = high priority targets)
4. Feed to `z80opt stoke` (stochastic search for longer sequences)
5. Verified replacements → new superopt rules → compiler peephole

### Value
- Finds optimizations for REAL code patterns, not synthetic combos
- 3-4 instruction sequences that brute-force can't reach
- Frequency-weighted: optimize what matters most

---

## Idea 2: Superoptimizer Verifies mzd Findings

**Problem:** mzd's register analysis is heuristic (linear scan, no CFG). Could have false positives.

**Idea:** Use z80-optimizer's exhaustive executor to VERIFY mzd's IN/OUT/CLOBBER claims.

```
mzd detects: fun foo() IN: A, HL  OUT: A  CLOBBER: BC
z80opt verify: execute foo() with all possible (A, HL) inputs
              → confirm OUT=A for all, confirm BC clobbered, confirm DE preserved
```

### Implementation
1. Extract function bytes from binary
2. For each function, run z80-optimizer's executor with randomized inputs
3. Compare: which registers changed? which were read before written?
4. Cross-check against mzd's static analysis
5. Disagreement = either mzd bug or interesting edge case (conditional paths)

### Value
- Ground truth for register analysis (dynamic > static)
- Catches control-flow dependent register usage mzd misses
- Could run as CI check

---

## Idea 3: ABI-Aware Superoptimization

**Problem:** Current superopt replaces sequences purely by instruction equivalence. Doesn't know which registers are dead after a sequence.

**Idea:** Use mzd's register liveness info to unlock MORE optimizations.

```
; mzd knows: after this point, BC is dead (not read before next write)
  LD A, B        ; reads B
  ADD A, C       ; reads C
  ; BC dead here

; With dead-BC knowledge, z80opt can try replacements that clobber BC:
  ADD A, B : ... → simpler sequence that happens to trash B
```

### Implementation
1. mzd computes per-instruction liveness (which regs are dead after each instruction)
2. Export "dead flags" annotation: `ADD A, C  ; dead: BC, DE`
3. z80-optimizer gains `--dead-regs=BC,DE` mode for targeted search
4. Rules tagged with "requires BC dead" — only apply when safe
5. Compiler's peephole pass checks liveness before applying tagged rules

### Value
- Unlocks optimizations invisible to context-free search
- Especially powerful after CALL (A,F,BC,DE,HL all dead until next use)
- Could find 10-30% more optimizations in typical code

---

## Idea 4: Stack Balance Checker

**Problem:** Register allocator bugs sometimes unbalance the stack (PUSH without POP).

**Idea:** Count PUSH/POP on every path to RET. Mismatch = definite bug.

### Implementation
1. Walk CFG (not linear scan — need to handle branches)
2. At each RET, stack delta must be 0
3. At each CALL, assume callee is balanced
4. Report: `fun board_set: stack imbalance +1 on path 0x1234→0x1240→RET`

### Value
- Catches a class of bugs that register analysis can't see
- 100% precision: stack imbalance is always a bug
- Cheap to implement (extend existing instruction walker)

---

## Idea 5: Tail Call Detection

**Problem:** `CALL foo; RET` wastes 17T + 1 byte vs `JP foo`.

**Idea:** mzd detects the pattern, reports as optimization hint.

```
; mzd: tail call candidate (saves 1B, 10T)
0123: CD 00 01    CALL foo
0126: C9          RET
; → could be: JP foo
```

### Implementation
1. Find RET instructions
2. Check if previous instruction is CALL (unconditional)
3. Verify no code between them (no fall-through from elsewhere)
4. Report as hint
5. Bonus: feed to compiler as missed optimization

### Value
- Easy wins, common pattern
- Especially in recursive functions (factorial, fibonacci)

---

## Idea 6: Dead Store / Redundant Load Detection

**Problem:** Codegen sometimes writes a register then overwrites it without reading.

```
  LD A, 5      ; dead store — A overwritten next
  LD A, (HL)   ; this is the real load
```

### Implementation
1. Extend provenance tracking: when a register with non-nil provenance is overwritten without being consumed → dead store
2. For redundant loads: track what was last loaded into each register, flag if same load repeated with unchanged source

### Value
- Direct indicator of codegen inefficiency
- Combined with superopt: "these 2 instructions are both dead → delete them" is better than "replace with shorter equivalent"

---

## Idea 7: Corpus-Based Rule Prioritization

**Problem:** 602K rules, but compiler only ever hits ~50 of them. Loading 100MB for 50 matches is wasteful.

**Idea:** Use mzd to analyze ALL compiled binaries, extract which patterns actually appear, build a "hot rules" subset.

```
all .a80 files → mzd --extract-sequences 2 → frequency count
                                                  ↓
602K rules → filter by frequency → top-1000 rules → fast_rules.json (50KB)
```

### Implementation
1. `mzd --extract-sequences 2` on all examples/tests
2. Match against superopt rules
3. Rank by frequency × savings
4. Export top-N as compact rule file
5. Default compiler load: fast_rules.json (instant) instead of full 100MB

### Value
- 1000x faster rule loading
- Same optimization coverage for real code
- Can ship with compiler (50KB vs 100MB)

---

## Idea 8: Inter-Procedural Register Analysis

**Problem:** `main()` shows `IN: BC, DE, A, F` because CALL to internal functions conservatively consumes everything.

**Idea:** Analyze callees first, use their actual IN sets.

### Implementation
1. Build call graph from xrefs
2. Topological sort (callees before callers)
3. When processing CALL to known function, consume only callee's IN set
4. Iterate until stable (handles mutual recursion)

### Value
- Eliminates false inputs in caller functions
- Makes --verify-abi more precise
- Enables better dead-register analysis

---

## Priority Matrix

| Idea | Impact | Effort | Dependencies |
|------|--------|--------|-------------|
| 4. Stack balance | HIGH (finds real bugs) | LOW | None |
| 5. Tail call | MEDIUM (easy wins) | LOW | None |
| 8. Inter-proc IN | HIGH (precision) | MEDIUM | None |
| 1. Sample harvester | HIGH (new rules) | MEDIUM | z80opt stoke |
| 7. Corpus prioritization | MEDIUM (perf) | LOW | All .a80 files |
| 6. Dead store | MEDIUM (codegen quality) | MEDIUM | Provenance |
| 3. ABI-aware superopt | HIGH (more rules) | HIGH | mzd liveness + z80opt changes |
| 2. Dynamic verification | MEDIUM (confidence) | HIGH | z80opt executor |

**Recommended next:** Stack balance (4) → Inter-proc IN (8) → Sample harvester (1)
