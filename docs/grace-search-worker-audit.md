# Search worker audit: compilation is not the current bottleneck

The bounded frozen SelfLanz two-invocation workload passed its measured diagnostic
at snapshot `f918ccd9`: **21.338s**, with CPU profiling enabled. This is a small
workload, not the full270-second CLI test and not a compiler speed benchmark.
No default or production execution path changed.

| Measured stage | Wall seconds |
|---|---:|
| Fresh source parse | 0.00028 |
| Raw VM including admission | 0.08339 |
| Fresh ordinary compilation to ASM | 0.06904 |
| Native run, annotations, admission | 0.36250 |
| Closed association including baseline | 2.41183 |
| Fixed-policy four-candidate selector and guards | 18.38912 |

Rows overlap conceptually: the closed association and selector each include
baseline/compilation work. They must not be summed as exclusive phase costs.
The diagnostic runs the real private native runner and fixed-policy selector;
it does not bypass identities, guards, source ownership or evidence binding.

Cumulative CPU profile (overlapping call stacks, not additive percentages):
`PrepareMemoryHostWorkload`13.54CPU seconds/51.1%; automatic profile context
9.60/36.2%; compatibility6.92/26.1%; fingerprintValue5.89/22.2%; executable
file reads/hash (`io.Copy`)5.58/21.1%. BuildAutomaticProfileContext repeatedly
opens and hashes the complete running executable on each source identity check.
The profile recorded26.49CPU seconds across21.32wall due to runtime concurrency.

Therefore worker isolation of only CompileHIRSteps would address very little
of this fixture. The useful next investigation is operation-scoped reuse of
compiler executable identity while preserving mutation refusal, followed by
reducing repeated semantic fingerprint work under equivalent ownership guards.
No global mtime-only hash cache is safe: size/mtime alone cannot establish equal
compiler bytes. Any scoped reuse must explicitly define its mutation contract.

## Actual source/factory ownership boundary

Policy search reparses owned frozen source bytes independently and validates
both associations before invoking the factory. It keeps cross-candidate source
graphs alive, checks their complete identity repeatedly and requires a fixed
48-factory-call lifecycle:24per policy. Within each selector, baseline/variant
runner owns fresh original, private compilation candidate and sentinel, with
cross-retained mutation checks. Parallel goroutines around the current factory
would change ordering and share mutable guards; dumping MIR loses types,
contracts and provenance. Neither is an acceptable worker implementation.

A future source-only process prototype could isolate one whole fixed-policy
selection per worker, with same executable, exact owned source/workload/input
association bytes,24local lifecycle checks and parent deterministic policy0/1
merge. Parent must privately spawn workers, bind response to immutable input
SHA/options/ABI, reject any process failure or candidate failure and validate
full nested evidence before ranking. Existing strict ties retain earlier fixed
order. Arbitrary caller-supplied factory graphs cannot be serialized this way.
That would be a separate source-only API, not a transparent drop-in change.

No worker protocol is implemented here: measured compilation0.069s does not
justify changing the source/mutation admission contract. FullCLI was not rerun.

Repeat diagnostic from `minzc`:

```sh
MINZ_SEARCH_STAGE_TIMINGS=/absolute/new-directory go test ./pkg/pipeline \
  -run '^TestSearchBoundedStageTiming$' -count=1 -v \
  -cpuprofile=/absolute/profile.pprof
```

[Manifest](grace-search-worker-audit/manifest.json) preserves timings, terminal
PASSlog, cumulative profile text and original pprof. Test is explicitly gated by
artifact environment; it records timings, never asserts a fragile speed threshold.
