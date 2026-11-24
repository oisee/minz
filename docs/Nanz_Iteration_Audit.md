# Nanz Iteration Audit (G0)

Date: 2026-10-20. Base: origin/main `dde023f4` (worktree `test/G0-iterator-contract`).
Scope: every iteration form the Nanz frontend recognises today, plus the two
CLAUDE.md claims about legacy `enumerate()`/`reduce()`. No new syntax — `gen`/`yield`
is G1 (see [ADR 0048](adr/0048-nanz-stackless-generators.md),
[0049](adr/0049-nanz-iteration-protocol.md), [0050](adr/0050-nanz-generator-fusion.md)).

Method: the ordinary range/ptr loops, memory/range chains, pipe/trans
expansion and manual cursor contract scenarios are pinned on raw MIR2 VM
(`hir.LowerModule` + VM), optimised MIR2 VM (`CompileHIRSteps` + VM), and
assembled Z80 in the three test files under `minzc/pkg/nanz`:
`iteration_audit_e2e_test.go`, `iterator_claims_legacy_test.go`, and
`generator_contract_e2e_test.go`. `TestG0IterChainAsmCallback` and legacy
execution tests are **Z80-only**. G0-L1 currently pins invalid
assembly/rejection (G0-5 is resolved, see below); clean acceptance must execute
and check semantics. The chain methods `skip`/`take`/`zip`/`count`/`sum` are
pinned on all three tiers in `iter_adaptors_e2e_test.go`.
G0-7 is fixed (P17): the test now parses and evaluates on all three tiers. Skipped findings stop at the first
observed failure, so a skip does not establish results on later tiers.
The callback `break` panic and exact `$F018` index address below are
inspection observations, not executable pins. The historical 0/11 corpus
claim is an audit observation; G0-L1 pins one representative program.

## Summary table

| Form | Element order | Empty (n=0) | n=1 | n=255 | n=256 | Early exit | Tiers |
|---|---|---|---|---|---|---|---|
| `for i in lo..hi` | forward `lo..hi-1` (var width follows `hi`) | 0 iters | `[lo]` | full 255 | 256 iters (u16 bound) | `break`, `continue`, `return` | VM=VM=Z80 |
| `for x in ptr[lo..hi]` | forward, loads `ptr[lo+i]` | 0 iters | `[ptr[lo]]` | full 255 | 256 iters (u16 len) | `break`, `continue`, `return` | VM=VM=Z80 |
| `range(lo..hi)` source | **forward**: `lo, lo+1, …, hi-1` (like `for`); `range(hi..0)` yields `hi-1, …, 0` | 0 iters | `[1]` | full 255 | u16 range type (G0-9 resolved) | none | VM=VM=Z80 |
| chain `map/filter → forEach` (memory src) | forward, stages innermost-first | no effects | 1 elem | full 255 | via u16 len | **not expressible** | VM=VM=Z80 |
| chain `mapInPlace` | forward, stores back | no writes | 1 | full 255 | via u16 len | not expressible | VM=VM=Z80 |
| chain `fold`/`reduce` (memory) | forward | returns `init` | `cb(init, ptr[0])` | full 255 | via u16 len | not expressible | VM=VM=Z80 |
| chainable `range(lo..hi).fold/.forEach` | forward (see range) | `init` / nothing | `[1]` | full 255 | as range | not expressible | VM=VM=Z80 |
| `pipe`/`trans` + `.apply()` | expands to nested `map`/`filter` calls — identical semantics per source | as chain | as chain | as chain | as chain | not expressible | VM=VM=Z80 |
| chain `skip(k)`/`take(k)` stages (memory or range src) | forward; `skip` drops the first k elements reaching it, `take` passes the first k reaching it (counts survivors after a `filter`) | no effects / `init` | as stage | full 255 | via u16 len | **`take`: loop stops** right after the k-th accepted element; on a memory source the next element is not even loaded (ADR 0050) | VM=VM=Z80 |
| chain `src.zip(other)` (memory only) | forward; source element i pairs with `other[i]` by **source position**, also after a `filter`; with an implicit count (sized global arrays) the shorter declared length | no effects / `init` | 1 pair | full 255 | via u16 len | via `take` | VM=VM=Z80 |
| terminals `count(n)` / `sum(n)` (`count()`/`sum()` on range) | forward; u16 result, `sum` zero-extends and wraps mod 65536 | 0 | 1 / `x` | full 255 | via u16 len | via `take` | VM=VM=Z80 |
| `enumerate`/`iter`/`collect`/`min`/`max`/`takeWhile`… | **rejected**: compile error `unknown method .name(...)` with source line (G0-5 resolved) | — | — | — | — | — | error |

"Early exit not expressible": iterator-chain callbacks are separate functions — a
`break` in a callback body is a compile-time panic (`BreakStmt outside loop`), and
`return` in a callback means "end this element's body", never "stop consuming"
(pinned by `TestG0IterChainCallbackReturn`). The only early stop in a chain is
the `take(k)` stage (added 2026-11, `iter_adaptors_e2e_test.go`); predicate
stops (`takeWhile`, `find`) remain inexpressible — the direct motivation for
ADR 0048's pull protocol.

## Semantics notes

- **`range(lo..hi)` is forward, like `for i in lo..hi`.** `range(0..5)` yields
  0,1,2,3,4 and `range(2..7)` yields 2,3,4,5,6 (lo..hi-1; empty when lo == hi).
  Order-sensitive tests are required (a sum cannot tell directions apart; see
  ADR 0050). Before 2026-11 `range` yielded the DJNZ countdown (5,4,3,2,1 for
  both `range(0..5)` and `range(2..7)`); that was changed by owner decision.
- **Lowering:** when the element is read by a stage or callback, the chain is
  lowered through the `for i in lo..hi` path (`lowerForRange`) so it costs the
  same. When the element is unused, the counter-only DJNZ loop is kept.
- **`range(hi..0)` (literal 0 upper bound) is the descending spelling:**
  `range(5..0)` yields 4,3,2,1,0, the reverse of `range(0..5)` (element =
  DJNZ counter − 1).
- **Side-effect order (chains):** downstream callback effects happen once per
  surviving element, in source order, after that element's map/filter stages —
  pinned by `order_trace` in `TestG0IterChainMemSemantics`.
- **`fold` on empty input returns the initial accumulator** without calling the
  callback (pinned: `fold_empty == 42`).
- **Callbacks are inlined when their body is a simple expression or a statement
  list without asm/non-trailing returns**; otherwise the chain emits a real CALL
  for named/noncapturing callbacks (G0-1/G0-2). Capturing lambdas with
  early returns/asm cannot use that fallback: G0-8 remains unresolved.

## Findings

Fixed in this audit (local, test-pinned; see `minzc/pkg/hir/lower.go`
`iterCallbackNeedsCall`):

- **G0-1 (fixed)** — a `return` nested in control flow inside an inlined
  forEach callback returned from the *enclosing consumer function* instead of
  ending that element's body (top-level returns were dropped, nested ones were
  lowered as consumer returns). Repro: `forEach(src, cb, 5)` with
  `fun cb(x: u8) -> void { if x == 3 { return } acc = acc + x }` — consumer
  aborted at x==3 with no return value. Fix: callbacks containing a
  non-trailing `return` (at any nesting depth) are invoked with a real CALL
  when noncapturing; capturing callbacks remain broken (G0-8).
  Test: `TestG0IterChainCallbackReturn` (fails without the fix).
- **G0-2 (fixed)** — an inlined forEach callback whose body is an inline `asm`
  block kept the standalone-function ABI (parameter in A) that the fused loop
  does not provide; the element lives wherever the loop allocated it (observed
  in C), so `OUT ($23), A` printed stale A. Repro:
  `forEach(&nums, emit, 5)` with `fun emit(x: u8) -> void { asm { OUT ($23), A } }`
  printed `00 00 00 00 00` instead of `01 02 03 04 05` on Z80. Fix: callbacks
  containing any asm block get a real CALL (capturing lambdas: G0-8).
  Test: `TestG0IterChainAsmCallback`.

Recorded, not fixed here:

- **G0-3** — `for x in ptr[3..1]` (hi < lo): the u8 length `end-start`
  underflows to 254; the loop runs 254 iterations reading past the source.
  Correct behaviour (0 iterations or a diagnostic) needs a lowering/parse
  decision. Test: `TestG0Finding_ReversedSliceIsEmpty` (skip).
- **G0-4 (resolved 2026-11)** — `range(5..3)` (lo > hi, hi ≠ 0, not the Rev
  spelling) is empty, like `for i in 5..3`. The unused-element loop used to
  count `hi-lo` in a byte, which underflowed to 254 iterations; it now counts
  in B only when the count is a byte that cannot underflow (`range(0..hi)` or
  `range(n..0)` over a u8 domain, or literal bounds lo ≤ hi) and takes the
  `for` lowering otherwise. Test: `TestG0Finding_CrossedRangeBounds`,
  `TestRangeChainCountWidthAndBounds`.
- **G0-5 (resolved 2026-11)** — unsupported iterator methods used to be
  accepted by the Nanz parser and lowered to placeholder code (`LD A, ?`,
  CALLs to undefined labels). Now `skip`, `take`, `zip`, `count` and `sum`
  are chain methods (fused, three-tier tests in `iter_adaptors_e2e_test.go`),
  and `hir.CheckIterChains` (run first in `pipeline.CompileHIRSteps`) turns
  everything else into a compile error with the source line:
  - `x.name(args)` where no function `name` is declared or imported and
    `name` is not a chain method (`enumerate`, `iter`, `collect`, `min`,
    `max`, `takeWhile`, …) → `unknown method .name(...)`. Only method-call
    syntax is checked; a plain call `name(args)` may still name an assembly
    or runtime label.
  - a chain the recogniser refuses (wrong arity, a non-callable callback,
    `zip` anywhere but directly on a memory source or twice, a callback
    whose element arity does not match the paired/single state, `sum` on
    pairs, `mapInPlace` with `zip` or a non-head `skip`/`take`, a memory
    terminal without its count — unless the source is a sized global array,
    whose declared length is then used) → `unsupported iterator chain: …`;
  - a chain method outside a lowered position (a stage without terminal, a
    value terminal inside a larger expression) → an error naming the rule.
  Further rules of the new chain path (review round 1): `k` and `n` must be
  u8 or u16 (or a non-negative literal) — signed and bool counts are
  rejected; element types must be u8, i8, bool, u16 or i16, and must match
  the byte width the source (or `other`) declares; the chain's arguments are
  evaluated once in source order (source, `zip` argument, stage arguments,
  terminal arguments); a value chain may be the whole right-hand side of an
  assignment to any target, including `p^ = …`; a callback inside another
  callback may not use the outer callback's parameter (bind it to a local).
  An original `fold` assigned to a global or memory target used to drop the
  store; it now goes through the same temporary.
  Review round 2: `skip`/`take`/`zip`/`count`/`sum` are chain methods only in
  method syntax (`x.sum(n)`, marked on the HIR call as `CallExpr.Method`); a
  plain `sum(buf, 4)` is a call, as on origin/main (it may name an asm
  label), and `CheckIterChains` checks only method-syntax calls, so the
  functional spellings (`forEach(src, cb, n)`, `fold(...)`) and every other
  frontend behave as before. An untyped fold accumulator takes the init's
  type (an untyped literal init keeps origin/main's hint: a pointer source's
  pointer type, a map's result type; u16 where main had none). `mapInPlace`
  gets the element-type hint and must store the type it loads. Struct
  elements are rejected in the original chains too (every such shape loaded
  wrong bytes or emitted invalid assembly on origin/main); a scalar parameter
  walking a struct or nested array as bytes/words, and an unread array
  parameter, keep working. A global initialiser `global s: u16 = a.sum()`
  gives 0 silently, like any call in a global initialiser on origin/main.
  A user function named like a chain method (`fun sum(...)`) keeps plain
  UFCS meaning. Test: `TestG0Finding_UnsupportedMethodRejected` (no longer
  skips: each spelling must be a source-line error or run correctly) and
  `TestIterChainDiagnostics`. Ten `examples/e2e_iterators/*.minz` files that
  used to "compile" into undefined labels now get the diagnostic.
- **G0-6 (withdrawn):** u16 overflow truncates consistently to 24464 on all tiers.
- **G0-7 (fixed by P17)** — a scalar `return` stopped after its first
  parenthesised operand (`return (a * 3) + x`, `return (b+b)-(a+a)`), so the
  rest became an unreachable statement or a parse error; the parser now reads
  the complete expression. Test: `TestG0Finding_ParenMulPlusParses` (passes).
- **G0-8** — capturing `forEach` lambdas containing a non-trailing return
  (or asm) require CALL, but `LowerModule` omits capturing lambdas as
  standalone functions. Minimal repro:
  `var sum: u8 = 0; src8.forEach(|x: u8| { if x == 3 { return } sum = sum + x }, 5)`
  with src8=[1,2,3,4,5] must yield sum=12. Raw VM reports
  `unknown function lambda_0`; emitted Z80 references an undefined lambda.
  Test: `TestG0Finding_CapturingCallbackReturn` (skip on observed raw-VM
  unknown-lambda error; correct sum asserted through the three-tier helper).
  No lowering change: CALL fallback has no error-return channel; adding a
  panic would not be a clean compile diagnostic. Closure support or proper
  frontend rejection is deferred.
- **G0-9 (resolved 2026-11)** — a range whose count does not fit a byte
  (`range(0..256)`, `range(0..300)`, a u16 bound, `range(n..0)` with a u16
  `n`) with an unused element used to truncate the DJNZ count to u8
  (`range(0..300)` ran 44 times). Such ranges now take the `for` lowering and
  its u16 range type; the descending spelling counts an index up and derives
  the element `n-1-idx`. Tests: `TestG0Finding_RangeCounterWidth`,
  `TestIterRangeCountWide`, `TestRangeChainCountWidthAndBounds`.
- **G0-10 (resolved 2026-11)** — a *named* function inlined as a chain
  callback resolved its free names in the enclosing function, so a caller
  local named like a global replaced the global (`let g = 100` in the caller,
  `fun addg(x) { return x + g }` as the callback: 621 instead of 51). A named
  callback whose body mentions a name bound in the enclosing function is now
  called instead of inlined; a lambda still captures. Test:
  `TestNamedChainCallbackSeesGlobals`.

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
  arguments against a register ABI (G0-L3, skip test). Both typed-callback
  tests assert actual console values after linking succeeds: the complete
  enumerate (index, element) log and reduce result 15, respectively.
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
- properties exercised: creation initializes frame state without changing
  source bytes or read probes, with the first pull observing source mutations
  made after creation; forward decode for
  n ∈ {8, 255}; empty input (DONE on first request, out unchanged at sentinel
  0xAA); single element then sticky DONE; two further requests after
  exhaustion stay DONE with out preserved even after resetting the byte
  count/source or RLE run/pair state; two independent
  instances over the same source pulled alternately; partial consumption
  (pull 3, abandon, resume the same frame); RLE example
  `[3,65, 0,90, 2,66] → 65,65,65,66,66`; RLE total of exactly 255;
  **ADR 0050 demand example** `filter(x>0).take(2)` over `[0,0,7,8,9]` →
  outputs `[7,8]` with exactly 4 source requests (9 never requested), and
  `take(3)` accepting [7,8,9] at 5 requests (without requesting DONE).

Expected values are computed by the Go reference at test time, with additional
independent literal pins requiring ADR 0050 pulls=4 and outputs=[7,8].
When G1 lowers `gen`/`yield`, its generated frames must match
these scenarios bit-for-bit on the same three tiers; extending this file is
the acceptance gate for G1/G2.
