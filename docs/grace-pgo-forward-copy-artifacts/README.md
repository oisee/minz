# Forward byte copy: saved evidence

`selector/` contains current evidence for the exact typed `copy_forward` source and its `append_token` caller. The workload performs 30 persistent calls over five byte patterns for lengths 0/1/5/16/64/255, including forward overlap. Global counters wrap and every call snapshots all declared resources. This source is a typed allocation/copy slice; it does not establish compilation or profiling of the original extern tokenizer.

Both current candidates pass the independent sequential host oracle and raw VM / postcompile raw VM / assembled Z80 comparisons. Speed selects the transformed candidate: 388885 → 283030 T-states. Size selects the original: 86 bytes instead of 221. These costs include complete caller invocations; bootstrap execution is recorded separately in target evidence.

`audit.py` independently reconstructs all 30 calls from workload bytes with sequential aliasing, including the caller's initial length store, per-call global updates, complete memory snapshots, and byte-level read/write counts. It checks 720 candidate call results across both selector modes and both built-CLI records and three execution forms, binary hashes, cost sums, proof enabled states, minimum-cost winner, and exact selected binary. `audit-report.json` records its successful result. The script checks original and postcompile raw evidence independently instead of using agreement between compiled variants as its expected answer.

`audit-evidence.go` uses the production Go evidence type and encoding/json to verify the self-excluding evidence SHA256 for both saved modes and both built-CLI records. `typed-evidence-audit.txt` records those hashes. Python is not used to reserialize the Go evidence hash.

Reproduce the saved-artifact audit from the repository root:

```sh
python3 docs/grace-pgo-forward-copy-artifacts/audit.py
cd minzc
go run ../docs/grace-pgo-forward-copy-artifacts/audit-evidence.go ../docs/grace-pgo-forward-copy-artifacts
```

Large JSON files are losslessly gzip-compressed with deterministic gzip timestamps. Their embedded binary and byte snapshots remain available in full. The selected binaries, source, workload, ASM, MIR, standalone target evidence, and host oracle are preserved alongside them.

`history/` is the untouched earlier offline pending evidence copied from `/home/alice/dev/minz/artifacts/grace-reports/2026-10-10-resume/pending-forward-copy-evidence`. Its top-level original baseline fails at invocation 1 (`got 12, want 1`): a byte store through DE overwrote the live loop index in A. Its original 84-byte baseline and 219-byte transformed artifact precede the backend fix; they are diagnostic history, not the current speed comparison. `history/latest-caller/` retains the separate pending snapshot under its original name. The current authoritative 86/221-byte candidates live in `selector/`.

Focused logs cover source legality, 8192 raw alias/address cases with first-trap and partial-memory expectations, assembled boundary cases, independent structural/factory refusals, 4-site/16-candidate boundary, unobserved sites, and old API regressions. `store-final.txt` verifies 38 machine cases for live accumulator stores and nested AF/HL restoration. Final HIR/MIR2/Nanz package gate passed7.766/3.887/1.633s; profile CLI17.723s. Built CLI preserves both actualselected binaries and its separate copy decisions. Showcase31existingfailures0new relative to the preceding weighted stage. The initial full pipeline failed solely because the old scalar budget fixture now matched the weighted pointer proof. The fixture now orders its two pure modular additions byte-first, keeping scalar budget competition outside that proof with the same independent arithmetic/access oracle. pipeline-before-budget-fixture-update.txt preserves that failure; the authoritative full gate including the long budget test passed722.080s, saved in pipeline-final.txt.

`manifest.json` lists SHA256 and sizes for every other file in this directory. Regenerate it after appending new gates or CLI evidence.
