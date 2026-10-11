# ADR-0047: MIR-native candidates; VIR as offline research

Status: Accepted boundary; incremental implementation
Date: 2026-10-11

Alice explicitly allows redesigning unfinished VIR and requires ordinary
compilation to have no external Z3 dependency. Z3 remains available for
research and offline generation/verification. This restates ADR-0043 and
supersedes the runtime Z3 fallback and unqualified optimality claims in
ADR-0040; historical measurements in those documents retain their context.

The product path remains HIR → MIR2 → Z80 with PFCCO/PBQP. VIR is a source
of machine descriptions, checked assignments and research candidates, not
a second runtime compiler that must be brought to parity first.

## Keep and extract incrementally

- Canonical z80spec physical overlap/width/encoding facts and complete ABI,
  clobber, flag and effect constraints.
- Checked local assignment/candidate descriptors and offline shape caches.
  Current pure-leaf admission includes actual return ABI and moves/saves.
- Useful lowering candidates only after independent VM/assembled Z80 checks
  and measured complete emitted-code costs, including boundary adapters.
- Offline Z3/GPU enumeration as independent research evidence. An assignment
  is optimal only in its explicitly modelled schedule/cost/constraint domain.

## Runtime candidate interface

A future MIR-native region candidate must bind the original MIR/CFG and
source identity, region instruction membership, live-ins/live-outs and
placements at every edge. It must state scratch, physical bank state, flags,
effects, call ABI, stack/spills, code template and full cost. The validator
checks these before mutation. A miss, unsupported shape or invalid candidate
retains the ordinary compiler result. No runtime process invokes Z3.

Start with the current checked-leaf consumer; extract shared pieces only
when a second concrete consumer requires them. Do not rewrite all VIR types
or resurrect LIR for an abstract architecture.

## Islands and WFC

An island is a bounded region chosen at low-liveness boundaries. It is not
an alternative-register bank. Physical bank use is a separate constrained
choice. A correct join is a parallel copy with pair/half aliases, live flags,
stack effects and bank restoration; its cost belongs to the candidate.
The existing research codegenFuncIslands currently solves its pieces
independently and omits boundary moves. It is not an admitted product path.

An in-process WFC constraint propagator may reduce domains over instruction
patterns, operand locations, live flag predicates and loop/edge contracts.
It must preserve every legal candidate within its declared domain. Collapse
is a heuristic unless independently proved otherwise. Compare bounded
solutions with independent enumeration/Z3 offline and measure compile cost
plus real native T/bytes before replacing production PBQP or enabling it.

## Current evidence and limits

Ordinary table application calls PrepareLocalTableProblem/Emit; it never
runs the solver. CLI --generate-checked-local-table is an explicit offline
operation. Checked hit with nonexistent Z3 path passes native execution.
Weighted full CALL workload retains 238→142T,44→20B joint matrix benefit;
K11 plasma and effect-safe byte fusion give independent native benefits.
These examples do not prove arbitrary CFG/island capability.

Live-F scalar writer correction is integrated; u32 comparison and extension
counterexamples are being repaired independently. Correctness of the base
compiler remains a prerequisite for evaluating additional allocation methods.
