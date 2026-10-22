# O7: proven byte loop idioms

The compiler recognizes unsigned ascending byte scans, fills and ordered copies from MIR2 facts and emits CPIR or guarded seeded LDIR. No function or benchmark name participates in admission. Four separate implementation commits cover shared analysis, scan, fill and copy; follow-up adapter fixes retain conditional guards and expose the actual register demands.

Baseline: `c5b0065eb8c5fd7101e81c6b0e6a42c7593c13a7` in `/mnt/ssd-510/minz/tmp/o7-before`. Measured compiler commit: `fd23c91f153d861857f26dc5cbe7c025eac30842`. Full artifacts, sources, assembly, raw/final MIR, binaries, workload hashes and measurements: `/mnt/ssd-510/minz/bench/o7-before` and `/mnt/ssd-510/minz/bench/o7-after`.

## Legality and representation

* Dominance-based natural-loop discovery is extracted from invariant-address hoisting. Facts include entering/latch edges, affine modular recurrences, invariants, escaping SSA values (including exit-edge arguments), loads, stores and opaque effects. Hoisting retains its unconditional-preheader requirement. DJNZ facts honor its implicit first argument. Forwarded parameters are invariant only when edge and parameter types match; narrowing conversions remain observable. Analysis is refreshed after every rewrite, including when a later loop uses an earlier loop exit as its bound.
* Scans admit a zero-start u16 index or unit pointer recurrence, one byte load/equality-to-zero test, no writes or opaque effects and supported live-outs. CPIR starts with BC=0; JP NZ repeats a complete address-space miss. A hit returns HL-1 or `65535-BC`, equal to HL-1-base modulo 65536. No terminator or object-size assumption is inferred from fixtures. Signed/narrow scan indices remain scalar.
* Fill/copy counts are u8 or u16; strict unsigned bounds and unit induction prove no premature index wrap. Both guard zero. Fill stores the first byte once, tests the residual count, and runs LDIR only when it is nonzero. The byte-count adapter can test INC C/DEC C while preserving the fill byte in A, including C=255.
* Fill values captured before the loop are invariant SSA values. Reloaded value/bound cells require distinct ordinary nonvolatile, non-fixed globals and a destination object containing every possible byte-count store. Fixed-address aliases, insufficient object extents and aliased reloads are rejected. Verification rechecks symbolic-load disjointness. Native seeded fill uses ordinary RAM; volatile/port/SMC effects are excluded.
* Copy performs each increasing source read immediately before its destination write. LDIR preserves forward overlap propagation; no memmove host is used. The VM commits bytes sequentially and preserves successful access prefixes on permission/bounds failures.
* Typed intrinsic calls carry explicit register inputs and retained original byte instructions for VM access hooks. Liveness and DSE use ordinary call semantics. Codegen adapts fixed operands with parallel copies and preserves live registers, halves and predicates. Conditional-call folding leaves these inline operations and their metadata intact. Closed workload consumers recognize validated memory operations without granting host authority.
* Recognition runs in both compiler drivers, after simplification and before contracts/allocation. The direct driver now uses the existing identity simplifier for C promotions. eZ80 and application runtime entry modes remain outside this lowering's admission. Unsupported CFGs, arithmetic widths and escaping values stay scalar.

LDIR is used for both size and speed profiles. Scalar unrolling would exceed the strict byte-growth budget on the Nanz fill routine; no larger code variant is enabled merely for its better long-loop core timing.

## Failing-first and correctness evidence

`TestNaturalLoopFactsIndependentOfLayout` fails on origin/main because AnalyzeNaturalLoops is absent. `TestByteScanZeroCPIRNative`, `TestByteFillLDIRNative` and `TestByteCopyOrderedLDIRNative` fail there on missing CPIR/LDIR. The implementation diff was also reversed in an isolated worktree with tests retained: all three opcode-shape suites fail (`o7-reversed-red.log`); the primary worktree was untouched. The same source-level tests pass after implementation, using independent buffer oracles, raw MIR, optimized MIR and the remogatto Z80 emulator. Default-mode tests check the direct compiler API too.

Coverage includes C/Nanz indexed and pointer forms, global fills (including C/AF symbol names), byte-count fills, returning/void foreach copies; counts/lengths 0,1,2,255,256,2047 where representable; values 0,129,255; overlap deltas -17,-1,0,1,17; full buffer and immutable input checks; PC/SP checks. MIR tests additionally cover modular addresses near 65535, a 65535-byte fill, invalid-memory zero counts, no-match scan gas, permission failures and exact access prefixes. Nine native canary cases permute input homes and preserve A/F/BC/DE/HL/IX/IY outside the designated result across operations. Qualified and register-like global labels assemble and execute correctly.

Additional failing-first tests caught a narrowed invariant parameter (`o7-narrowed-red.log`) and global-label spelling (`o7-symbol-red.log`), both green after correction (`o7-adapter-green.log`, `o7-symbol-green.log`). Sequential-loop exit bounds are covered too. Negative tests cover volatile accesses, calls, signed/inclusive bounds, nonunit steps, narrow scan indices, oversized bounds, varying stores, additional loads/live-outs, fixed or overlapping globals, insufficient object size, and malformed intrinsic metadata. Related pointer, foreach, hardware LDIR, invariant-address and real intern-program tests pass. The intern workload retains its independent exact native byte-traffic oracle. A retained parser copy helper changes the parser binary golden; direct, factory and instrumented routes agree on its refreshed hash.

Logs: `o7-origin-main-red.log`, `o7-origin-analysis-red.log`, `o7-unit-final.log`, `o7-final-focused.log`, `o7-parser-routes.log`, `o7-judges-direct.log`, `o7-judges-folded.log` under `/mnt/ssd-510/minz/bench`. The fast pipeline inventory is sorted and includes all three added tests.

Required package tests: PASS. Selected pipeline tests: PASS. Direct-call seeds 0–499: PASS 500/500. Folded seeds 0–99: PASS 100/100. No whole-pipeline run, heldout suite, larger fuzz campaign or assert_matrix was run.

## Main suite

Cells show total program bytes / T states, before → after. All 36 MinZ cells pass and none grows in bytes or T.

| Case | Default B / T | Size B / T | Grace B / T |
|---|---:|---:|---:|
| abs_diff | 27 / 220,904 → 27 / 220,904 | 27 / 220,904 → 27 / 220,904 | 27 / 220,904 → 27 / 220,904 |
| bitwise8reg | 101 / 872,448 → 101 / 872,448 | 101 / 872,448 → 101 / 872,448 | 101 / 872,448 → 101 / 872,448 |
| copy_bytes | 182 / 301,600 → 172 / 109,332 | 182 / 301,600 → 172 / 109,332 | 182 / 301,600 → 172 / 109,332 |
| crc8 | 340 / 165,616,162 → 340 / 165,616,162 | 340 / 165,616,162 → 340 / 165,616,162 | 326 / 152,877,602 → 326 / 152,877,602 |
| fat12_decode | 167 / 30,336 → 167 / 30,336 | 167 / 30,336 → 167 / 30,336 | 167 / 30,336 → 167 / 30,336 |
| gcd | 57 / 4,953,966 → 57 / 4,953,966 | 57 / 4,953,966 → 57 / 4,953,966 | 51 / 3,067,013 → 51 / 3,067,013 |
| memset_fill | 299 / 18,403,840 → 297 / 3,588,390 | 299 / 18,403,840 → 297 / 3,588,390 | 299 / 18,403,840 → 297 / 3,588,390 |
| mul8x8 | 47 / 1,167,872 → 47 / 1,167,872 | 47 / 1,167,872 → 47 / 1,167,872 | 47 / 1,167,872 → 47 / 1,167,872 |
| reverse_loop11 | 561 / 23,644,160 → 561 / 23,644,160 | 561 / 23,644,160 → 561 / 23,644,160 | 561 / 23,644,160 → 561 / 23,644,160 |
| strlen_bytes | 293 / 19,468,800 → 283 / 3,574,400 | 293 / 19,468,800 → 283 / 3,574,400 | 293 / 19,468,800 → 283 / 3,574,400 |
| struct_roundtrip | 71 / 612,352 → 71 / 612,352 | 71 / 612,352 → 71 / 612,352 | 71 / 612,352 → 71 / 612,352 |
| sum_array | 313 / 7,581,952 → 313 / 7,581,952 | 313 / 7,581,952 → 313 / 7,581,952 | 313 / 7,581,952 → 313 / 7,581,952 |

## Idiomatic Nanz targets

The same frozen sources and measurement harness run in all three profiles. All nine cells pass with no byte/T regression.

| Case | Default B / T | Size B / T | Grace B / T |
|---|---:|---:|---:|
| copy_bytes | 194 / 616,990 → 171 / 108,630 | 194 / 616,990 → 171 / 108,630 | 194 / 616,990 → 171 / 108,630 |
| memset_fill | 303 / 8,855,680 → 303 / 3,638,295 | 303 / 8,855,680 → 303 / 3,638,295 | 303 / 8,855,680 → 303 / 3,638,295 |
| strlen_bytes | 297 / 17,536,000 → 287 / 3,611,520 | 297 / 17,536,000 → 287 / 3,611,520 | 297 / 17,536,000 → 287 / 3,611,520 |

## Files

* `minzc/pkg/hir/memory_host_workload.go`
* `minzc/pkg/hir/profile_workload_admission.go`
* `minzc/pkg/mir2/byte_loop.go`
* `minzc/pkg/mir2/byte_loop_test.go`
* `minzc/pkg/mir2/byte_loop_vm.go`
* `minzc/pkg/mir2/condcall.go`
* `minzc/pkg/mir2/contracts.go`
* `minzc/pkg/mir2/dump.go`
* `minzc/pkg/mir2/inst.go`
* `minzc/pkg/mir2/invariant_address.go`
* `minzc/pkg/mir2/natural_loop.go`
* `minzc/pkg/mir2/natural_loop_test.go`
* `minzc/pkg/mir2/regs.go`
* `minzc/pkg/mir2/verify.go`
* `minzc/pkg/mir2/vm.go`
* `minzc/pkg/mir2/z80codegen_byte_loop.go`
* `minzc/pkg/mir2/z80codegen_byte_loop_test.go`
* `minzc/pkg/mir2/z80codegen_call.go`
* `minzc/pkg/mir2/z80cost.go`
* `minzc/pkg/pipeline/byte_loop_test.go`
* `minzc/pkg/pipeline/memory_host_baseline_test.go`
* `minzc/pkg/pipeline/pipeline.go`
* `minzc/pkg/pipeline/profile_workload.go`
* `scripts/ci/pipeline_profiles.json`
* `reports/2026-10-22-o7-loop-idioms.md`

EXIT 0
