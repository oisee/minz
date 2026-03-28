# Session 14 Seed — Wire Enriched Pipeline + Loop Codegen

**Date:** 2026-03-29
**Repo:** minz-vir (VIR backend)
**Branch:** master

---

## What Was Done (Session 13)

### 7 commits, 3 bugfixes + enriched architecture
1. **P0 CALL arg setup** (4 bugs): vreg collision, DstHint, peephole, coalescing
2. **P1 cross-block params**: param injection + pair→half move fallback
3. **P2 loop_head labels**: position-based skip → label-based + dedup
4. **OpBag + InterferenceShape**: enriched table signature computation
5. **Enriched pipeline wiring**: LoadEnriched + 3-format Lookup
6. **Tarjan + bipartite**: cut vertex decomposition + EXX feasibility
7. **Session report + CLAUDE.md**: updated with 5-level pipeline

### Corpus delivered
- 820 functions (546 Nanz + 274 C89), `/tmp/corpus_combined.jsonl`
- 235 unique signatures, 79% ≤6v, 91% O(1) with cut decomposition
- z80-optimizer ran batch evaluation, confirmed all findings

### z80-optimizer collaboration
- 37.6M enriched entries shipped (bb62fb5)
- GPU partition kernel working (<1ms for 10v) (2a7b4bb)
- Go reader (`pkg/regalloc/enriched.go`) promised for next session
- `exx_2colorable` flag in next enrichment batch

---

## Priority 1: Wire Full Decomposition Pipeline

Currently: `table.Lookup()` tries enriched signature but only works for single-island functions.

Need: for 7v+ functions, decompose → per-partition enriched lookup → compose assignments.

```go
// In CodegenFunc, before Z3:
shape := ComputeInterferenceShape(allOps, desc)
if shape.NVregs <= 6 {
    // Direct enriched lookup (already wired)
} else {
    components := shape.DecomposeAtCutVertices()
    if allComponentsLE6(components) {
        // Per-component enriched lookup + compose
    } else if bip, coloring := shape.IsBipartite(); bip {
        // EXX dual-bank scheduling
    } else {
        // GPU partition or Z3 fallback
    }
}
```

Files: `pipeline.go` (CodegenFunc), `regalloc_table.go` (compose logic)

---

## Priority 2: While-Loop Codegen

`fact()` has `CP A` (compares A with itself) instead of proper loop condition.
The VIR island solver flattens all blocks → loses which vreg holds what.
Separate from label fix (P2 done).

Test: `fun fact(n: u8) -> u8 { var r=1; var i=1; while i<=n { r=r*i; i=i+1 } return r }`

---

## Priority 3: Enriched Reader Integration

z80-optimizer writing `pkg/regalloc/enriched.go`:
- `LoadEnriched(path) → EnrichedTable`
- `table.Lookup(shapeHash, opBagHash) → {assignment, cost, flags, patterns}`
- Binary .enr format (16-byte header + per-entry records)

Our `LoadEnriched` in `regalloc_table.go` currently reads JSON/JSONL.
May need to add binary .enr reader when it arrives.

---

## Priority 4: MIR2 VM Bug (from main session)

Cross-function host peek/poke: `read()` wrapper loses return value.
Root cause: Nanz lowering sees `peek` as void call (no `@extern` declaration).
Fix: `@extern fun peek(addr: u16) -> u8` in Nanz scope.
Main session handling this.

---

## Key Files

| File | Purpose |
|------|---------|
| `pkg/vir/regalloc_table.go` | OpBag, InterferenceShape, Tarjan, bipartite, LoadEnriched |
| `pkg/vir/pipeline.go` | CodegenFunc (table lookup before Z3), fixCallArgHints |
| `pkg/vir/cfgsolver.go` | Param injection, edge moves, move coalescing |
| `pkg/vir/solver.go` | findMovePatternRemap, OpMove coalescing |
| `pkg/vir/bridge.go` | translateCall fresh vregs |

---

## Test Commands

```bash
# VIR assert tests (all should pass except GCD)
cd minzc && go test ./pkg/vir/ -run 'TestVIR_Assert' -timeout 120s

# Corpus dump
VIR_DUMP_GPU_BATCH=1 go run ./cmd/minzc examples/nanz/assert_test.nanz 2>/dev/null

# Enriched table (when available)
VIR_ENRICHED_TABLE=/path/to/enriched.jsonl go run ./cmd/minzc program.nanz

# Graph analysis
go run /tmp/test_graph.go  # (script from session 13, uses corpus_combined.jsonl)
```

---

## Active Sessions

- **minz-vir** (this): VIR backend, enriched integration
- **z80-optimizer** (um2dy4ex): GPU kernels, enriched tables, partition optimizer
- **main minz** (ju6yy047): production codegen, testing, self-hosting tokenizer
