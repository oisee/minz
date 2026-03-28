# Session 13 Wisdom — O(1) Register Allocation Architecture

**Date:** 2026-03-28
**Duration:** ~4 hours
**Commits:** 7 (4 on remote after rebase dedup)

---

## Hard-Won Lessons

### 1. Callee vreg namespace collision (P0)
`translateCall` used `cp.Reg` (callee's vreg) as arg-setup destination. Callee vreg IDs overlap with caller's → `computeLiveness` sees the LAST defAt, destroying the caller's liveness for that vreg. Fix: offset fresh vregs (+8000). Same pattern already existed in `translateRuntimeCall` (+7000/+7100) — should have been applied from the start.

### 2. DstHint ≠ specific register (P0)
`regClassToLocSet(cp.Class)` returns the whole register CLASS (e.g., any GPR), not the specific PFCCO-assigned register. The solver picks whichever is cheapest within the class — wrong if callee was compiled with a specific register. Fix: `fixCallArgHints()` post-pass narrows DstHint to the exact `FuncParamLocs[callee][param]` singleton.

### 3. Peephole rules must never assume LD before CALL is dead (P0)
"LD A, X / CALL func → dead store" seems logical (CALL overwrites A with return value). BUT the LD may be loading the callee's ARGUMENT into A. The solver only emits moves that are needed — if it emits LD A, X before CALL, it's the arg setup. Rule permanently disabled.

### 4. OpMove src/dst can share a register (P0)
Standard interference says "two live vregs at the same instruction can't share a register." But OpMove's src and dst CAN — `LD A, A` is a valid nop. Without move coalescing in the interference set, the solver forces unnecessary round-trips (save to B, restore from B) when the value is already in the right place.

### 5. Cross-block params are invisible to per-block liveness (P1)
`computeLiveness` is per-block. If param `b` is used in block 2 but not block 0, it has NO Z3 variable in block 0. Edge move constraints can't fire → no `LD A, B` emitted → wrong register read. Fix: inject function params at ALL instructions of the entry block when they're used in later blocks. This creates Z3 variables + move cost terms + correct interference.

### 6. findMovePattern needs pair→half fallback (P1)
Z3 assigns 8-bit vregs to 16-bit pair locations (IY=12). `findMovePattern(IY, A)` fails — no `LD A, IY` pattern. Fix: `findMovePatternRemap` maps IY→IYL, IX→IXL, BC→C, DE→E, HL→L and retries. Returns remapped phys index for correct emission.

### 7. Island label emission: position-based skip is wrong (P2)
`emitPIRWithLabels` skipped labels when `br.start == startPC`. For entry blocks with 0 ops, the next block (loop_head) starts at the same offset → label lost. Fix: check for "entry" label string instead of position. Also: shared `emittedLabels` map across multi-island emission to prevent duplicates.

### 8. O(1) regalloc: signature = (shape_hash, op_bag_hash)
The enriched table key is NOT the full constraint description. It's two hashes:
- **InterferenceShape**: canonical graph (renumbered vregs + sorted edges) → SHA256
- **OpBag**: multiset of abstract VIR ops {Add:2, Sub:1, Cmp:3} → SHA256
- Abstract ops (not Z80 patterns!) break the circular dependency: op_bag → lookup → assignment → THEN pattern selection

### 9. Graph decomposition covers 91% without Z3
- Cut vertex detection (Tarjan, O(V+E)): 33% of functions have articulation points → free split
- ≤6v direct lookup: 79% of corpus (648/820 functions)
- 7v+ decomposable to ≤6v via cuts: +102 functions → 91% total
- Bipartite graphs (EXX feasible): 70% of functions
- GPU partition for 7-14v: <1ms per function

### 10. Cross-session collaboration via dedelulu
Three sessions (minz-vir, z80-optimizer, main minz) coordinated in real-time:
- z80-optimizer provided enriched tables, GPU partition kernel, corpus evaluation
- main minz provided testing (Tetris, Hello Frill), priority coordination
- This session provided VIR fixes, enriched integration, graph analysis
- `ddll send session:main "message"` — instant coordination, no context switching

---

## Architecture Decisions

### Enriched table pipeline wiring
The existing `regalloc_table.go:Lookup()` tries three signature formats in order:
1. Legacy signature (original format)
2. GPU exhaustive signature (56 entries)
3. Enriched signature (shapeHash:opBagHash — O(1) from 37.6M entries)

Auto-loads from well-known paths + `VIR_ENRICHED_TABLE` env var.

### 5-level regalloc pipeline
```
L0: cut vertices → free decomposition (87% of graphs)
L1: enriched ≤6v → O(1) hash lookup (37.6M entries)
L2: EXX bipartite → dual-bank 7-12v (70% feasible)
L3: GPU min-cut partition → <1ms for ≤14v
L4: Z3 fallback → 30s (<1% functions)
```

### Corpus stats (820 functions)
- 235 unique (shape, opBag) signatures
- Operation mix: move 34%, const 21%, call 13%, add 10%, cmp 9%
- move=34% confirms regalloc directly impacts 1/3 of all instructions
- mul=0% — nobody multiplies on Z80 (shifts/adds instead)

---

## Files Changed

| File | Changes |
|------|---------|
| `bridge.go` | translateCall fresh vregs (+8000), DstHint from class→specific |
| `pipeline.go` | fixCallArgHints(), disabled LD-before-CALL peephole, label emission fix, batch dump |
| `solver.go` | OpMove coalescing, findMovePatternRemap (pair→half) |
| `cfgsolver.go` | Param injection ALL entry-block insts, move coalescing, edge move remap |
| `regalloc_table.go` | OpBag, InterferenceShape, EnrichedSignature, LoadEnriched, Tarjan, IsBipartite, DecomposeAtCutVertices |
| `reports/` | Session 12-13 report |
| `CLAUDE.md` | VIR status update, 3 new Design Wisdom (#9-11), 3 resolved issues |
