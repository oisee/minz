# Compiler checks: PR and deep audits

Policy requested by Alice on 2026-10-13: exhaustive domains and expensive
measurements run on release tags, nightly and manual audits rather than every PR.

The pipeline inventory is partitioned explicitly by `pipeline_profiles.json`.
Fast and deep profiles cover the complete compiled test inventory exactly once;
unknown, deleted or duplicate entries fail validation. Updating tests requires
reviewing their profile. The full profile remains available for local integration
verification and does not omit any test.

PR gate builds tools, runs stable core/frontend packages and the fast pipeline
profile, verifies Nanz regressions, assembles Fibonacci and runs the small
Z-machine VM fixtures. Positive exhaustive hardware matrices are deep; invalid
hardware descriptors and both actual Spectrum ROM/IRQ demos remain fast.
The assertion judge uses the small tuple-return fixture with both Z80 and MIR2
and negative controls. PR fuzz count is 300; full/nightly count remains 10000.
Fuzz workflow success is subject to its existing failure policy, not a claim that
all cases agree with the oracle. Direct-call fuzz uses `--fail-on all`: any
finding or tool error fails the job.

Deep pipeline tests run in 12 independent jobs on `v*` tags, nightly and manual
invocation. Release/manual audits also run the full assertion matrix and fuzz;
the existing nightly compiler audit supplies that matrix on scheduled runs.
Per-process timeout remains 30 minutes. Full assertion runs retain inventories,
completed-unit JSONL and an atomic partial snapshot. An incomplete run is never
reported as a complete PASS.

For the current broad compiler integration, the already started local full
583-entry run continues. Moving tests to a deep profile does not establish that
that integration has passed; its logs and outstanding results remain explicit.

```sh
cd minzc
python3 ../scripts/ci/run_stable_pipeline.py --profile fast --output /tmp/minz-fast
python3 ../scripts/ci/run_stable_pipeline.py --profile full --shards 12 --workers 3 --output /tmp/minz-full
```

Every exported inventory records the complete profile assignment, selected
entries, process groups, compiler-test binary SHA and commit.
