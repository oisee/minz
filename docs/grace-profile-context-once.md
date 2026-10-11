# Reuse adjacent compatibility work without weakening identity checks

Memory-host identity checks previously built automatic context, computed full
HIR compatibility to validate it, then immediately computed the same key again.
Now the private combined automatic helper returns the key already calculated;
`PrepareMemoryHostWorkloadCompatibility` exposes that same synchronous admission
result to pipeline callers. Existing public context/prepare APIs keep their
signatures and behavior as wrappers.

Only adjacent prepare→compatibility pairs changed: baseline, native target,
policy search, association replay, loop-profile and arena/append selectors.
No factory callback occurs between the removed duplicated work. Every previous
identity observation still performs full source/workload admission, freshly
opens/hashes the executable and fingerprints current HIR. No cache crosses
calls, callbacks or mutations; no executable hash cache was added.

The key owns its dependency map independently of returned context/caller input.
Memory-host/loop semantics suffixes change only `key.Context.SemanticsID`, after
the existing full validation. BuildProfileCompatibility verifies source manifests,
origins and all semantic fields independent of that nonempty suffix, while its
Context stores it for equality. It never reuses a stale key after a source change.

## Validation and bounded measurement

Full HIR **PASS25.375s**; exact key/context/workload ownership and malformed
ABI/manifest/origin/host/target equivalence **PASS0.079s**. Existing late factory
mutation, changed original metadata/provenance, preflight and observation refusal
checks **PASS22.609s**. Source mutation across consecutive calls changes the key.
The same real bounded frozen-parser stage diagnostic was run once after change,
with CPU profiling; fullCLI was not repeated.

| Stage | Previous wall seconds | Current wall seconds |
|---|---:|---:|
| Full diagnostic | 21.338 | 17.797 |
| Fixed-policy selector | 18.389 | 15.828 |
| Closed association | 2.412 | 1.456 |
| Fresh compilation to ASM | 0.0690 | 0.0705 |

The diagnostic finished16.6% sooner and selector13.9% sooner in these single
runs. Machine load and separate test builds differ; this is bounded evidence,
not a guaranteed fullCLI speedup or controlled repeated benchmark. Nested stage
rows are not additive. Compiler identity changes across binaries normally.

Cumulative CPU `BuildProfileCompatibility`:6.92→4.11s; fingerprintValue5.89→4.27s.
Executable read/hash remains5.58→5.82s: its costly full checks were retained.
CPU stacks overlap and percentages must not be summed. Further improvements
must preserve every mutation observation boundary.

[Manifest](grace-profile-context-once/manifest.json) covers full HIR/factory
logs, equivalence test, stage timings and CPU profile. Previous audit evidence
remains immutable in `grace-search-worker-audit`. No backend/language semantics
or worker protocol changed.
