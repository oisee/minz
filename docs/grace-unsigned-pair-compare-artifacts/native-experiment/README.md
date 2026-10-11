# Private preserved-pair comparison experiment

Source remains the exact original SelfLanz input. Three full30 native callers are measured with diagnostic executable context dfdf728: original, private backend comparison, and isolated one-load reuse plus private comparison. No public selector, default compiler flag or memory/frame permission change is introduced.

| Variant | T-states | Binary bytes | Steps |
|---|---:|---:|---:|
| Original | 1135370 | 2000 | 109829 |
| Compare | 1132449 | 2001 | 109966 |
| Reuse + compare | 1096139 | 1998 | 108955 |

Original matches current dba production native ASM/BIN, per-call costs/state and every typed allocation/MIR/access/frame hash exactly. Compare saves2921T and adds1byte; combined saves39231T (~3.46%) and2bytes. All90 measured native observations and each raw-source oracle match full results/globals/regions/orderedhostevents. Costs are wholecaller actual measurements, not additive estimates or a new PC ledger.

Compare-only leaves cur's original reload and comparison unchanged; the finite backend option activates at another eligible live-pair comparison. Combined cur holds pos across the boundary check and now compares high/low bytes using scratch A, preserving both word pairs; no PUSH/POP or repeated pos load remains. Eligibility remains the independently reviewed restricted unsigned U16 Ult/Uge, physicalpairs, live-pair/Adead gate.

The first original-only attempt used stale requested design base d1fc (old HL-only global load backend); its authentic fed98/1221494T payload failed strict current reference and is preserved separately in history-stale-base. Neither candidate ran there. Exact production two backend files and six tests were then copied from2908; the original reference was not weakened. Corrected three-case test passed3.593s.

Both normal emission and internal annotated regeneration receive the same private option; baseline binary equivalence and runtime recipe checks stay strict. A private raw MIR hook performs the reviewed exact1curLoad→Move only in the combined case, before optimization/contracts/allocation. Evidence baseline containers are internal diagnostics, not public source-to-MIR certificates.

Payloads/source/ASM/BIN/MIR/allocation/optioncaptures/sourceinventory/commands/logs and stale history are retained. Deterministic gzip preserves exact payload bytes with SHA records. Portable source-host and typed hashes pass; Independent final reviews c417beb1 (host/source) and03d2ddf6 (native/provenance) pass and are copied beside the evidence. The exact executed compiler hash is recorded in every payload. The temporary go-test executable was removed after execution and has not been recovered; no reconstructed binary is represented as the executed one.
