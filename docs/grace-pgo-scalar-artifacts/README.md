# Verified scalar invariant program regression

Added `examples/nanz/grace_weighted_checksum.nanz` and `minzc/pkg/pipeline/profile_scalar_program_test.go`; artifact output stays in `docs/grace-pgo-scalar-artifacts`. No production code or existing tests were edited by this task; concurrent parent changes were visible in this shared clone. No commits were made.

The example accumulates a seed plus each buffer byte and a per-byte invariant weight `a+b`, with wrapping u16 arithmetic. An ordinary caller invokes the accumulator and a recording callee updates persistent u8/u16 globals. Calls surround the loop because the parent proof excludes loop calls. Zero-trip direct calls preserve the seed, globals, and resources without reading any resource; zero-trip caller calls still record the seed.

Thirty calls cover lengths 0/1/5/16/64/255 across zero, ff, ramp, alternating, and mismatched ramp data. Mismatches occur at offsets 0/15/254; offset 255 remains an unread guard. Parameters cover carry, a weight wrapping to zero, accumulator wrap, and a wrapping global call counter. Six invocations call the core directly and 24 invoke the ordinary caller.

The independent host oracle computes using wide Go integers and explicit truncation. It checks each return and every declared global/resource byte at every invocation boundary. `exact_data_access: true` checks access equivalence. Both raw structural candidates pass all 30 host return/memory checks and exactly match the original HIR accesses. Both target binaries pass all 30 calls. The selected exact binary is replayed and compared with the measured evidence.

Actual enabled HIR contains `var __grace_base_0: u16 = a+b` before the while. Only the enabled candidate has `hir-invariant-base` lineage on the surviving accumulator loop. No pointer variant sites are eligible. The regression uses `MeasureGraceVariantSets` and requires the strict minimum whole-workload T-state candidate, with baseline winning ties; it verifies both retained binary identities against independently reproduced compilation/execution.

| Candidate | Invocation T-states | Module bytes |
| --- | ---: | ---: |
| baseline | 1,166,563 | 168 |
| invariant hoisted | 475,239 | 116 |

Selected candidate: 1. Savings: 691,324 T-states (59.26%) and 52 bytes (30.95%). Costs sum entry-through-RET invocation execution, as required by the measurement API, rather than including bootstrap execution in the comparison.

The first parent state failed baseline n=5 (65501 vs 65505) and hoisted n=0 (220 vs 65500). Isolated diagnostics executed every original call, including n=255, with original entry memory. Those historical failures are retained under `initial-failure/` and explicitly labelled as belonging to earlier binaries. Concurrent parent fixes resolved them; the final test passes without shrinking the workload. The test retains its diagnostic branch so future target failures export both binaries, ASM/MIR/HIR/lineage and per-call repros before failing.

Retained artifacts include source and manifest, workload, profile/profile run, independent oracle, decisions, complete measured evidence, results, costs, both exact `.bin` files with SHA-256 hashes, ASM, MIR, HIR, symbols, lineage, raw results, target results, and selected binary/ASM/MIR. `test-output.txt` records the successful run. `artifact-sha256.json` binds all artifacts except itself. `costs.json` hashes match both binary files.

Reproduce using `./docs/grace-pgo-scalar-artifacts/reproduce.sh`. The final targeted Go test passed with the requested Go 1.26.8 toolchain and GOCACHE. Scope formatting/diff checks pass.
