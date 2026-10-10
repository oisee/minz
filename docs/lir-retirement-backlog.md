# LIR retirement: migration backlog

**2026-10-10.** Decision: [ADR-0044](adr/0044-lir-retirement-and-rule-migration.md).
Native LIR is deprecated. Production stays HIR → MIR2 → PBQP/codegen.
Transfers below are queued, not implemented.

| ID | Priority | Work | Destination | Completion evidence |
|---|---|---|---|---|
| LIR-M0 | P0 | Inventory transformations, patterns, constraints, diagnostics and tests; identify production duplicates | Inventory in this backlog | Every component classified migrate / already present / research only / obsolete, with source pointer |
| LIR-M1 | P1 | Useful identity/constant-multiply rules from `pkg/lir/combine.go` | Grace MIR rules; HIR where structurally useful | Width/sign/overflow guards; exhaustive byte VM cases; cost vs existing folding/strength reduction |
| LIR-M2 | P1 | `load16_le` combining from `pkg/lir/combine.go`; audit analogous stores in `pkg/vir/isle.go` | MIR memory rules + production backend lowering | Order, endian, alias/address wrap guards; reject volatile/MMIO/unknown effects; memory VM checks + target tests; size/time delta |
| LIR-M3 | P1 | Sound instruction patterns from `pkg/lir/z80.go` and selection tables | Production backend and cost model | Valid opcodes, flags, ABI/register-pair constraints; assembled/emulated differential tests; no ISLE requirement |
| LIR-M4 | P2 | Audit WFC constraints, register restrictions, clobber/call rules | Production allocation constraints / PBQP inputs | Identify missing constraints first; loop/call/spill regressions; correctness/compile-cost measurements |
| LIR-M5 | P2 | Audit peepholes, layout, setup moves, spill/half-register handling | Production codegen/peepholes | Flag/liveness conditions; bytes/T-states on real cases; exclude fixes for LIR-only invalid instructions |
| LIR-M6 | P2 | Useful examples and independent expected-value tests | Production regression corpus | Tests execute main path; VM and machine checks where needed; LIR output is not the sole oracle |
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
5. Measure benefit and compiler overhead, including zero-match programs.
6. Start behind a flag; enable only after correctness/cost checks.

Grace can provide a common authoring surface with graph/term matching and facts
underneath. Recursion, rewrite fuel and tail-call optimization do not replace
effect proofs, instruction-selection cost models or register allocation.
