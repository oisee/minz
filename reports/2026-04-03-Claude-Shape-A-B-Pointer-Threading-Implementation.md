# Shape A/B Pointer-Threading Implementation

**Date:** 2026-04-03
**Track:** Bounded implementation, /home/alice/dev/minz only
**Predecessor:** Multi-Block Pointer-Threading Design Note (same date)

---

## What Was Implemented

### Grace Rule: `pointer-walk-threading` (priority 22)

A new Grace rule that matches Shape A counted loops and replaces indexed
addressing with pointer-walk threading through block arguments.

**Pattern matched:**
```
header(%idx): cmp idx < limit; br_if @body(%idx), @exit()
body(%bidx):  addr = add(const_base, bidx); load/store addr; idx_next = add(bidx, stride); jmp @header(idx_next)
```

**Transform applied:**
1. Add pointer block param to header (ClassPointer, TyU16)
2. Add corresponding param to body (if body has params)
3. Remove address computation (`add base, idx`) from body
4. Replace all uses of the address register with the pointer param
5. Add pointer increment (`add ptr, stride`, ClassPointer) in body
6. Thread pointer through back-edge jump args
7. Compute initial pointer (`base + idx0`) in pre-header blocks

### New Predicate: `has-pointer-walk-opportunity`

Detects the pattern by checking:
- Header has ≥1 block param (the index)
- Body has `OpAdd(const_base, idx_param)` where result feeds `OpLoad`/`OpStore`
- Body's back-edge jump increments the index param by a constant stride (1-256)
- Both operand orderings handled (`add base,idx` and `add idx,base`)
- Constants found in both head and body blocks

### New Action: `threadPointerWalk`

Performs the actual transform. Conservative:
- Only fires on exact Shape A (2-block: header + body with direct back-edge)
- Only matches affine addressing (`add const, idx` → load/store)
- Does not match multiply-based or multi-level addressing
- Allocates fresh registers via `f.AllocReg()`
- Updates ALL edges to header (not just the back-edge)

### Helper Functions Added

- `findPointerWalkCandidate(f, head, body)` — pattern detection
- `replaceRegInBlock(blk, old, new)` — register substitution
- `removeInstFromBlock(blk, idx)` — instruction removal
- `addInitialPtrArg(f, blk, headerLabel, baseImm, idxParamIdx)` — pre-header pointer init

## Does It Cover fill_buf-Class Loops?

**Yes.** The test `TestGraceRunner_PointerWalkShapeA` models `fill_buf` directly:

```
while idx < 768 { let p = 0xC000 + idx; p^ = 1; idx = idx + 1 }
```

After the transform:
- **Entry**: computes `ptr0 = 0xC000 + 0`, passes to header
- **Header**: receives `(%idx, %ptr)` — 2 params
- **Body**: uses `%ptr` directly for store (no address recomputation),
  increments `%ptr` by 1, passes back to header
- The `0xC000` constant is completely removed from the loop body

This eliminates one `OpAdd` (the address computation) per iteration and replaces
it with a simple pointer increment — exactly the `INC HL` vs `ADD HL,idx` win
from the design note.

## What Regressed or Did Not Help

- **Shape B (3-block)**: Correctly does NOT fire. The Grace edge pattern requires
  body→header direct back-edge. Shape B (body→latch→header) is a separate rule.
  This is by design — Shape B needs its own rule to handle the latch block.

- **Priority interaction**: Initially had priority 7, but `cond-ret-sink` (priority 20)
  transformed the header's `TermBrIf` into `TermCondRet` before pointer-walk could
  match. Fixed by raising to priority 22. This is safe: pointer-walk threading
  doesn't conflict with cond-ret-sink (they operate on different block pairs).

- **Dead constant**: After removing the address computation, the `0xC000` constant
  in body becomes dead. The existing `dead-store-elim` rule (priority 50) cleans
  it up automatically in a subsequent pass.

## Tests

3 new tests, all passing:

| Test | Type | Verifies |
|------|------|----------|
| `TestGraceRunner_PointerWalkShapeA` | Positive | fill_buf-class loop: rule fires, addr comp removed, pointer threaded, initial ptr computed in entry |
| `TestGraceRunner_PointerWalk_Negative` | Negative | Multiply-based addressing (not add-with-const): rule does NOT fire |
| `TestGraceRunner_PointerWalkShapeB` | Boundary | 3-block loop (body→latch→header): rule does NOT fire (correct, Shape A only) |

All 5 existing Grace tests continue to pass unchanged.

```
$ go test ./pkg/mir2/ -run 'TestGraceRunner' -v -count=1
--- PASS: TestGraceRunner_DSE
--- PASS: TestGraceRunner_CondRetSink
--- PASS: TestGraceRunner_SplitJoinRet
--- PASS: TestGraceRunner_DeadBlockArg
--- PASS: TestGraceRunner_Stats
--- PASS: TestGraceRunner_PointerWalkShapeA
--- PASS: TestGraceRunner_PointerWalk_Negative
--- PASS: TestGraceRunner_PointerWalkShapeB
PASS

$ go build ./pkg/mir2/ ./pkg/vir/ ./pkg/rewrite/... ./cmd/...
(clean)
```

## Files Changed

| File | Change |
|------|--------|
| `minzc/pkg/mir2/grace_runner.go` | +gracePointerWalkRule, +predicate, +action, +4 helper functions (~280 LOC) |
| `minzc/pkg/mir2/grace_runner_test.go` | +3 tests, +1 helper (~250 LOC) |

## Commits

- `23acfb45` — B6 pre-split prototype (previous)
- *(this commit)* — Shape A pointer-walk threading via Grace

## Recommendation

Shape A is solid and covers the `fill_buf` class. Shape B (3-block with latch)
would require a separate rule matching `(edge ?body ?latch "succ") (edge ?latch ?head "succ")`
with address computation in body and stride in latch. The infrastructure
(`findPointerWalkCandidate`, `replaceRegInBlock`, etc.) is reusable — Shape B
would mainly need a new Grace rule string and an updated predicate to look across
the latch block for the stride. Estimated effort: ~50 LOC on top of Shape A.
