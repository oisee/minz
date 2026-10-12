# LIR retirement: current migration status

**Updated 2026-10-12.** [ADR-0044](adr/0044-lir-retirement-and-rule-migration.md) remains the decision record. Live `minzc/pkg/lir` and its emission/convergence entry points are removed from the research branch. Production uses HIR → MIR2 → allocation/codegen; VIR remains an optional research path. This page replaces the October 10 queued-work snapshot, not its immutable evidence.

| ID | Current status | Evidence and exact boundary |
|---|---|---|
| LIR-M0 | Inventory and bounded transfer audit completed | [Inventory](grace-lir-removal-inventory/README.md), [code-based final review](../reports/2026-10-11-lir-transfer-final-bounded.md). No specific functioning production feature or expected-value fixture was found lost in the reviewed domains; complete relation equivalence with the research solver is not claimed. |
| SEM-PTR | Target-width wrap implemented and tested | [ADR-0045](adr/0045-raw-pointer-semantics.md), [gap closure](grace-gap-closure.md). VM heap bounds and raw target addresses are separate contracts; generic volatile/MMIO assumptions are not inferred from wrap. |
| LIR-M1 | Useful typed identities and constant multiplication present | [Typed identity proof](../reports/2026-10-11-lir-typed-zero-identities.md), [word ×10/12 proof](grace-mul16-10-12/REPORT.md). Byte coefficients already existed. Guards preserve operand width/sign/effects; not every parsed Combine declaration was a functioning optimization. |
| LIR-M2 | Bounded effect-safe byte/word fusion transferred | [FAT12 evidence](grace-fat-word-fusion.md), `mir2/byte_word_fusion.go` and independent audit tests. Ordered byte origins, alias/address/fault-prefix guards remain; generic memory fusion is not admitted. |
| LIR-M3 | Useful native patterns retained or already present | [Final pattern mapping](../reports/2026-10-11-lir-transfer-final-bounded.md), MIR2 term/codegen and multiply tests. ISA/flags/physical preservation evidence governs admission; no requirement to retain the old emitter layout/counts. |
| LIR-M4 | Concrete constraints mapped; research solver equivalence open | Canonical overlaps, PBQP constraints, parallel moves/spills/call preservation and checked-table admission tests mapped in the [final review](../reports/2026-10-11-lir-transfer-final-bounded.md). WFC remains research history; arbitrary CFG/island composition is not established by local table hits. |
| LIR-M5 | Safe useful behavior retained; unsafe textual rules rejected | Current native peepholes and liveness-aware moves have their own tests. Old ADD1→INC/OR0 removal/PUSH-POP deletion lacked necessary carry/memory proofs and are not restored literally. [Audit](../reports/2026-10-11-lir-transfer-final-bounded.md). |
| LIR-M6 | Fixtures transferred and additional coverage implemented | [Exact test transfer](../reports/2026-10-11-lir-test-transfer-completion.md). New [composition VM/native matrix](../reports/composition-vm-native/README.md): four unchanged sources, 162 domains, 10,616,832 calls per layer, three modes, independent mathematical results and exact native CALL return. This is not the entire 3/4-argument Cartesian or full memory/flag oracle. |
| LIR-M7 | Live package and consumers removed; research archived | [Removal evidence](grace-lir-final-removal/README.md): exact 53-file archive retained outside build graph; current integrity recheck passed. Shared sexpr/datalog/ISLE, canonical z80spec and VIR remain. Retired CLI flags reject explicitly; legacy profile configuration is not silently reused. |

## What remains useful to investigate

A new feature needs a concrete production gap. Arbitrary shape recombination and CFG/island allocation need an explicit separator/ABI boundary contract, complete move costs and a whole-assignment native oracle. Current VIR island stitching has placeholder boundary moves; checked production admission rejects unsupported calls/CFG/indirect entries. Keeping that refusal is part of correctness.

The latest composition matrix strengthens a specific pre-existing coverage gap. It does not prove optimal allocation, universal WFC constraint equivalence, or closure of the wider compiler goal. [Current corpus](grace-integration-338/catalog.json), full pipeline gate and pending language/runtime decisions remain separate requirements.

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

