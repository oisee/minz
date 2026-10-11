The corrected private prototype passes independent saved-only audit on all90 original/compare/reuse+compare observations from the unchanged original30-call SelfLanz workload. Results, ten globals, complete persistent resource snapshots, ordered peek/poke events and typed failure fields match the independent fixed-source model. Binary/assembly, compact allocation, final MIR, sidecar and call-frame payload SHA bindings pass. Original exactly matches current dba production baseline; the experiment executable context dfdf728... is new and shared consistently across three candidates. No new profiles or PC counts are claimed.

| Candidate | Actual T | Bytes | Executed steps | Delta from original |
|---|---:|---:|---:|---|
| original | 1135370 | 2000 | 109829 | — |
| compare | 1132449 | 2001 | 109966 | -2921T, +1B |
| reuse+compare | 1096139 | 1998 | 108955 | -39231T, -2B |

Combined whole-workload saving is about3.46%; it is measured for this fixed workload, not a universal optimization result. Compare-only preserves the entire final MIR/allocation and changes the intern_region search comparison HL versusBC. It does not change cur, whose original pair staging needs no live-pair save. Reuse+compare additionally keeps pos inDE across cur's comparison, applies bytelex there, removes the second pos load and needs no PUSH/POP DE. Its raw MIR capture differs at exactly one instruction (r57 load r56→move r49); all HIR/source/raw/function identities remain equal.

The first run was based on stale source missing current direct DE/BC global word loads and stopped at the original-reference gate. Its oldfed98 payload/log remains history; the reference was not relaxed. Corrected source imports exact current implementation. Final160 native selected consumer tests and focused pending/cache/fallback tests passed. The independent boundary arithmetic model covers786513 paths. No native CPU was executed by this audit, and the prototype remains private/default-off with no permission or public failure-capture changes.

Reproduce saved audit: `python3 audit_actual.py /path/to/actual /path/to/current-production/original/factory-baseline.json actual-audit.json`. JSON/gz input support is portable. The source model admits only known original SHA0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526.
