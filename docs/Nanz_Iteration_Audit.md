# Nanz Iteration Audit (G0)

Date: 2026-10-20. Base: origin/main `dde023f4` (worktree `test/G0-iterator-contract`).
Scope: every iteration form the Nanz frontend recognises today, plus the two
CLAUDE.md claims about legacy `enumerate()`/`reduce()`. No new syntax — `gen`/`yield`
is G1 (see [ADR 0048](adr/0048-nanz-stackless-generators.md),
[0049](adr/0049-nanz-iteration-protocol.md), [0050](adr/0050-nanz-generator-fusion.md)).

Method: every statement below is pinned by an executable test in
`minzc/pkg/nanz/iteration_audit_e2e_test.go` (Nanz forms),
`minzc/pkg/nanz/iterator_claims_legacy_test.go` (legacy claims) and
`minzc/pkg/nanz/generator_contract_e2e_test.go` (generator contract scaffold).
Each test runs the same module on **three tiers**: raw MIR2 VM
(`hir.LowerModule` + `mir2.NewVM`), optimised MIR2 VM (`CompileHIRSteps` +
VM), and native Z80 (assembled program on the MZE emulator core). All pinned
behaviours agree across the three tiers unless a finding says otherwise.

## Summary table

| Form | Element order | Empty (n=0) | n=1 | n=255 | n=256 | Early exit | Tiers |
|---|---|---|---|---|---|---|---|
| `for i in lo..hi` | forward `lo..hi-1` (var width follows `hi`) | 0 iters | `[lo]` | full 255 | 256 iters (u16 bound) | `break`, `continue`, `return` | VM=VM=Z80 |
| `for x in ptr[lo..hi]` | forward, loads `ptr[lo+i]` | 0 iters | `[ptr[lo]]` | full 255 | 256 iters (u16 len) | `break`, `continue`, `return` | VM=VM=Z80 |
| `range(lo..hi)` source | **countdown**: `count, count-1, …, 1` where `count = hi-lo` (or `lo` for `hi..0`); element = counter, **not** `lo+i` | 0 iters | `[1]` | full 255 | wraps to full u8 cycle (256 iters, multiset `{0..255}`) | none | VM=VM=Z80 |
| chain `map/filter → forEach` (memory src) | forward, stages innermost-first | no effects | 1 elem | full 255 | via u16 len | **not expressible** | VM=VM=Z80 |
| chain `mapInPlace` | forward, stores back | no writes | 1 | full 255 | via u16 len | not expressible | VM=VM=Z80 |
| chain `fold`/`reduce` (memory) | forward | returns `init` | `cb(init, ptr[0])` | full 255 | via u16 len | not expressible | VM=VM=Z80 |
| chainable `range(lo..hi).fold/.forEach` | countdown (see range) | `init` / nothing | `[1]` | full 255 | wraps as range | not expressible | VM=VM=Z80 |
| `pipe`/`trans` + `.apply()` | expands to nested `map`/`filter` calls — identical semantics per source | as chain | as chain | as chain | as chain | not expressible | VM=VM=Z80 |
| `take`/`skip`/`enumerate`/`iter`/`collect`… | **not recognised in Nanz** (legacy `.minz` only) — silently emit invalid asm (G0-5) | — | — | — | — | — | broken |

"Early exit not expressible": iterator-chain callbacks are separate functions — a
`break` in a callback body is a compile-time panic (`BreakStmt outside loop`), and
`return` in a callback means "end this element's body", never "stop consuming"
(pinned by `TestG0IterChainCallbackReturn`). There is no `take`-style early stop
in any Nanz chain — the direct motivation for ADR 0048's pull protocol.

## Semantics notes

- **`range(0..n)` is reverse, `for i in 0..n` is forward.** `range(0..5)` yields
  5,4,3,2,1 (DJNZ countdown); `for i in 0..5` yields 0,1,2,3,4. A sum agrees,
  an order checksum does not — generator tests must be sequence-sensitive
  (already required by ADR 0050).
- **`range(lo..hi)` yields the countdown counter, not `lo+i`:** `range(2..7)`
  yields 5,4,3,2,1 (count = hi−lo). Documented in the Nanz book, chapter 6,
  and in `lowerRangeLoop`.
- **`range(hi..0)` (literal 0 upper bound) is the "Rev" spelling** of the same
  countdown: `range(5..0)` ≡ `range(0..5)`.
- **Side-effect order (chains):** downstream callback effects happen once per
  surviving element, in source order, after that element's map/filter stages —
  pinned by `order_trace` in `TestG0IterChainMemSemantics`.
- **`fold` on empty input returns the initial accumulator** without calling the
  callback (pinned: `fold_empty == 42`).
- **Callbacks are inlined when their body is a simple expression or a statement
  list without asm/non-trailing returns**; otherwise the chain emits a real CALL
  (semantics-preserving fallback, see G0-1/G0-2 below).

## Findings

Fixed in this audit (local, test-pinned; see `minzc/pkg/hir/lower.go`
`iterCallbackNeedsCall`):

- **G0-1 (fixed)** — a `return` nested in control flow inside an inlined
  forEach callback returned from the *enclosing consumer function* instead of
  ending that element's body (top-level returns were dropped, nested ones were
  lowered as consumer returns). Repro: `forEach(src, cb, 5)` with
  `fun cb(x: u8) -> void { if x == 3 { return } acc = acc + x }` — consumer
  aborted at x==3 with no return value. Fix: callbacks containing a
  non-trailing `return` (at any nesting depth) are invoked with a real CALL.
  Test: `TestG0IterChainCallbackReturn` (fails without the fix).
- **G0-2 (fixed)** — an inlined forEach callback whose body is an inline `asm`
  block kept the standalone-function ABI (parameter in A) that the fused loop
  does not provide; the element lives wherever the loop allocated it (observed
  in C), so `OUT ($23), A` printed stale A. Repro:
  `forEach(&nums, emit, 5)` with `fun emit(x: u8) -> void { asm { OUT ($23), A } }`
  printed `00 00 00 00 00` instead of `01 02 03 04 05` on Z80. Fix: callbacks
  containing any asm block get a real CALL. Test: `TestG0IterChainAsmCallback`.

Recorded, not fixed here:

- **G0-3** — `for x in ptr[3..1]` (hi < lo): the u8 length `end-start`
  underflows to 254; the loop runs 254 iterations reading past the source.
  Correct behaviour (0 iterations or a diagnostic) needs a lowering/parse
  decision. Test: `TestG0Finding_ReversedSliceIsEmpty` (skip).
- **G0-4** — `range(5..3)` (lo > hi, hi ≠ 0, not the Rev spelling): count
  `hi-lo` underflows u8 to 254 and the loop yields 254..1. Same class as G0-3.
  Test: `TestG0Finding_CrossedRangeBounds` (skip).
- **G0-5** — unsupported iterator methods (`take`, `skip`, `enumerate`, `iter`,
  …) are accepted by the Nanz parser and lowered to placeholder code: the
  module compiles with **no error**, but Z80 validation reports
  `LD A, ?` invalid instructions and undefined labels — one such call anywhere
  breaks the whole module's assembly. Correct behaviour: a source-level
  diagnostic. Test: `TestG0Finding_UnsupportedMethodRejected` (skip).
- **G0-6** — a function whose result exceeds its declared `u16` return width
  returns an incorrect value on the raw MIR2 VM (`300*300` → 24464 in the
  current regression run) while Z80 returns the truncated 33792. Out-of-contract input, but the VM
  should mirror target truncation. Test: `TestG0Finding_RangeOverflowReturnWidth`
  (skip).
- **G0-7** — parser: `return (a * 3) + x` is rejected ("unexpected token +")
  while `return a * 3 + x` parses; a parenthesised multiplication operand
  before `+` breaks expression parsing (assignments too). Test:
  `TestG0Finding_ParenMulPlusParses` (skip).

### Legacy `.minz` pipeline (claims evidence)

The production CLI routes `.minz` through the Nanz parser
(`cmd/minzc/main.go`), so the legacy frontend (`pkg/parser` → `pkg/semantic` →
`pkg/codegen`, MIR1) is reachable only in-process. Evidence tests:
`minzc/pkg/nanz/iterator_claims_legacy_test.go`.

- **G0-L1** — the documented "11/11 E2E hex-verified" iterator suite
  (`examples/e2e_iterators/run_e2e.sh`, v0.19.5) is **0/11 on current main**:
  the same files now compile via the Nanz parser, which emits `CALL forEach` /
  `CALL iter` with undefined labels (and `LD A, ?` for argument positions).
  Test: `TestG0Claims_ProductionPathDropsLegacySyntax` (skip).
- **CLAUDE.md claim "enumerate broken on Z80 (B counter+index conflict)":
  mechanism refuted, outcome confirmed.** The emitted loop no longer uses B
  for both counter and index — the index is memory-backed (`$F018`), the loop
  terminates, and with a parameterless asm callback reading the pushed
  (element, index) pair the observed output is exactly
  `00 01 01 02 02 03 03 04 04 05` for input [1..5] — index and element both
  correct (`TestG0Claims_EnumerateLoopTerminates`, PASS). However, with a
  normal typed callback the loop emits `CALL <unmangled name>` (the definition
  is mangled, e.g. `log_u8`), which fails to assemble; additionally the pair
  is stack-passed while the callee expects register params (G0-L2, skip test).
- **CLAUDE.md claim "reduce broken (A overwritten by 2nd SMC param)":
  mechanism refuted, outcome confirmed.** The emitted callee reads register
  params (A/E); the historical double `LD A` SMC pattern is gone. The current
  defect is the same unmangled `CALL sum2` (undefined label) plus stack-passed
  arguments against a register ABI (G0-L3, skip test).
- **G0-L4** — legacy local array literals (`let nums: [u8; 5] = [1,2,3,4,5]`)
  never materialise: every element reads as 0. The 11/11 suite depended on
  this working. `TestG0Claims_LegacyArrayLiteralUninitialised` (skip).

CLAUDE.md's "Iterator Chain Fusion" section (11/11 E2E, enumerate/reduce
bugs) describes the retired pipeline and should be marked historical; the
Nanz-path status is this document.

## Generator contract scaffold (G1 oracle)

`minzc/pkg/nanz/generator_contract_e2e_test.go` implements the ADR 0048/0049
pull protocol `next(frame, out) -> bool` twice — as a Go reference model and
as hand-written Nanz "manual cursor" state machines — and requires both to
agree on all three tiers:

- **byte cursor** (`ByteCursor{pos, left, done}`) and **RLE decoder**
  (`RleCursor{src, pairs, left, val, done}` over `[count,value]` pairs,
  nested refill loop, zero-length runs yield nothing);
- properties exercised: creation issues no requests; forward decode for
  n ∈ {8, 255}; empty input (DONE on first request, out unchanged at sentinel
  0xAA); single element then sticky DONE; two further requests after
  exhaustion stay DONE with no effects and out preserved; two independent
  instances over the same source pulled alternately; partial consumption
  (pull 3, abandon, resume the same frame); RLE example
  `[3,65, 0,90, 2,66] → 65,65,65,66,66`; RLE total of exactly 255;
  **ADR 0050 demand example** `filter(x>0).take(2)` over `[0,0,7,8,9]` →
  outputs `[7,8]` with exactly 4 source requests (9 never requested), and
  `take(3)` exhausting the source at 5 requests.

Expected values are computed by the Go reference at test time (never
hand-baked). When G1 lowers `gen`/`yield`, its generated frames must match
these scenarios bit-for-bit on the same three tiers; extending this file is
the acceptance gate for G1/G2.
