# LIR retirement: migration backlog

**2026-10-10.** Decision: [ADR-0044](adr/0044-lir-retirement-and-rule-migration.md).
Native LIR is deprecated. Production stays HIR → MIR2 → PBQP/codegen.
Transfers below are queued, not implemented.

| ID | Priority | Work | Destination | Completion evidence |
|---|---|---|---|---|
| LIR-M0 | P0 | Inventory transformations, patterns, constraints, diagnostics and tests; identify production duplicates | Inventory in this backlog | Every component classified migrate / already present / research only / obsolete, with source pointer |
| SEM-PTR | P0 | Specify Nanz pointer arithmetic, wrap/trap/UB, dereference bounds and one-past pointers | Language contract + MIR2 VM + Z80 conformance tests | Explicit decision and boundary tests; blocks removal of pointer-walk guard and M2 address guards |
| LIR-M1 | P1 | Useful identity/constant-multiply rules from `pkg/lir/combine.go` | Grace MIR rules; HIR where structurally useful | Width/sign/overflow guards; exhaustive byte VM cases; cost vs existing folding/strength reduction |
| LIR-M2 | P1 | `load16_le` combining from `pkg/lir/combine.go`; audit analogous stores in `pkg/vir/isle.go` | MIR memory rules + production backend lowering | Order, endian, alias/address wrap guards; reject volatile/MMIO/unknown effects; memory VM checks + target tests; size/time delta |
| LIR-M3 | P1 | Sound instruction patterns from `pkg/lir/z80.go` and selection tables | Production backend and cost model | Valid opcodes, flags, ABI/register-pair constraints; assembled/emulated differential tests; no ISLE requirement |
| LIR-M4 | P2 | Audit Wave Function Collapse (WFC) selection/allocation constraints, register restrictions, clobber/call rules | Production allocation constraints / PBQP inputs | Identify missing constraints first; loop/call/spill regressions; correctness/compile-cost measurements |
| LIR-M5 | P2 | Audit peepholes, layout, setup moves, spill/half-register handling | Production codegen/peepholes | Flag/liveness conditions; bytes/T-states on real cases; exclude fixes for LIR-only invalid instructions |
| LIR-M6 | P0, continuous | Useful examples and independent expected-value tests | Production regression corpus | Tests execute main path; VM and machine checks where needed; LIR output is not the sole oracle |
| LIR-M7 | P3 | Consolidate duplicate ISLE implementations/declarations; retire unused native-emission entry points after migration | Research/archive cleanup | Call-site audit; useful tests retained; CLI compatibility or separate deprecation; preserve research consumers |

Inventory must also cover multi-block flow, calling conventions, location
descriptors (IX/IY/shadow/SMC), convergence diagnostics and RISC/6502 research
support. Do not infer implementation or value from feature comments alone.
Preserve research-specific capabilities independently where they have a consumer.

## Existing evidence

Re-run on 2026-10-10: `TestCombine_Load16LE` reports **8 → 2 MIROps**;
`TestCombine_IdentityRules` reports **3 → 1**. The combined load returns the
expected `0xBEEF` on RISC32 and RISC8 LIR VMs. These are local rule measurements,
not an ISLE on/off production benchmark. Full-backend comparisons conflate
combining, selection and allocation.

`pkg/vir/isle.go` calls hand-written Go transforms inside `ISLECombine`; its
S-expression rules are not the executing matcher. `pkg/rewrite/isle` has no
production call sites in this checkout. The real LIR matcher is `pkg/lir/isle`,
called by `pkg/lir/combine.go`.

## Gate for each transfer

1. Demonstrate a production gap on a real source case; skip duplicates.
2. Specify type, effect, control-flow and target applicability.
3. Compare behavior using independent expected values and VM checks.
4. Keep passing assertions passing and failing assertions failing.
5. Measure Z80 code bytes and emulator T-states for M1–M5, plus compiler overhead including zero-match programs. VM instruction counts are supporting evidence, not target performance.
6. Start behind a flag; enable only after correctness/cost checks.

Grace can provide a common authoring surface with graph/term matching and facts
underneath. Recursion, rewrite fuel and tail-call optimization do not replace
effect proofs, instruction-selection cost models or register allocation.

## Review follow-up and order

Order: M0 and continuous M6 → SEM-PTR → M1 with constant MIR branch folding
→ M3/M2 with Z80 measurements → M4/M5 → M7. Track the existing backend failures
in [Grace blockers](grace-review-follow-up.md); compare them with the baseline
before attributing failures to a transfer.

The same gate applies to existing HIR rules. Literal constant-if has only a
demo source case, so it is an experiment, not a candidate for default enablement.
Audit `FoldConstants`/`PropagateConstants` before adding MIR folding of constant
`br_if` and `cond_ret`; preserve chosen edge arguments and return values.
The 8→2 load example above does not establish Z80 bytes or T-states savings.
WFC is Wave Function Collapse, confirmed by `pkg/lir/wfc.go`: location sets
propagate constraints and then collapse to a choice for selection/allocation.
