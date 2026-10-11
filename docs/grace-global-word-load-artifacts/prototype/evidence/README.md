# Direct global word-load experiment

Two production codegen files extend the existing exact direct global u16 load from HL to DE/BC. Both matcher and emitter require an actual known global, width16, and a single function-wide address use for DE/BC. Numeric/offset/spill/index-register and shared/asm/successor addresses keep previous paths. HL's emitted text remains unchanged. Existing native ED read recipes and permissions are unchanged.

| Full original SelfLanz thirty-call workload | Previous T / bytes | Current T / bytes | Whole-caller change |
| --- | ---: | ---: | --- |
| Original HIR | 1221494 /2091 | 1135370 /2000 | -86124 T (-7.05%), -91 B |
| Existing arena+append HIR | 1147025 /2197 | 1079636 /2099 | -67389 T (-5.88%), -98 B |

These are actual independent whole-caller measurements, not summed instruction estimates. Source/workload and HIR/function origins stay exact. The new test executable identity is embedded in current evidence and differs from historical CLI1bd; no old profile was supplied or relabeled. This prototype measures a public closed baseline only; it does not contain a newly collected selector profile or per-PC global traffic trace.

Focused tests executed DE/BC loads across six word values, full A/F/HL/other-pair state and RAM preservation, exact opcode/data counts, shared/numeric/successor/asm-address refusal, destination-byte cache/trunc/store/return behavior, and eight positive/malformed closed ED recipes. MIR2 .027s and pipeline .014s passed; bounded original/both full30 passed2.960s. The ordered byte recipe validates low/high permissions; independent source model verifies all ten globals, complete declared regions and3017 ordered host events.

All saved BIN/ASM, final MIR diagnostic, allocation and native access/frame hashes are audited. Both historical and current artifacts are retained separately, with deterministic gzip preserving exact payload bytes. `comparison.json` includes binary hashes and distinct process contexts. No runtime permission widening or general pointer rewrite occurred.

Use `verify_host.py`, `verify_typed_payload.go` and `independent-oracle-review/audit_actual.py` to audit saved raw/gzip files without CPU execution. Production integration and broad compiler/CLI/corpus gates remain the parent task.
