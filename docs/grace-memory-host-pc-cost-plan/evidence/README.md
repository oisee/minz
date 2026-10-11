# Actual native PC cost diagnostic

Exactly two final full original SelfLanz 30-call measurements ran through the unchanged closed native runner: original 1221494 T /127538 steps and both 1147025 T /119303 steps. BIN, ASM, all per-call T/steps, complete observations and sidecar/frame/allocation/MIR hashes match prior authoritative CLI evidence. The diagnostic test process has its own executable context, distinct from CLI 1bd95516.

The isolated hook is not proposed for production. It observes owned pre-step bytes and records cost only after successful native instruction and permission/frame checks. No additional memory authority, source-loop lineage or emulator-independent timing proof is claimed.

The assembler listing omits standalone function labels. Therefore the original raw PC ledgers correctly contain all observed PCs/costs but owner fields remain `unowned`. They are preserved untouched. `physical-owner-projection.json` is a separate offline derivation from the same embedded assembly declarations, assembled symbols and exact instruction byte table. It includes host adapters explicitly and records `node_tag EQU peek` as an alias without its own physical interval. Physical spans run to the next emitting entry, final span to the last listed instruction extent; compiler helpers in a span share its physical owner. These are exclusive physical costs, not inclusive call-chain attribution.

| Physical owner | Original T | Both T |
| --- | ---: | ---: |
| intern_region | 334605 | 263235 |
| cur | 239391 | 239391 |
| parse_sexpr | 236919 | 236919 |
| make_node | 141414 | 141414 |
| is_atom_char | 120219 | 120219 |
| region_match | 51575 | 48476 |
| adapter:peek | 35292 | 35292 |
| adapter:poke | 15997 | 15997 |

All remaining owners are preserved in the projection; none are unowned after mapping. The entire74469T difference is71370T in intern_region and3099T in region_match. `cur` and parser scanning are worthwhile next review targets, but no optimisation has been implemented from this diagnostic.

Two earlier original-only attempts refused a reference MIR hash because the existing test fixture used a different relative source name. BIN/cost/state/other hashes already matched. The failure logs and payload are preserved; the final factory uses exact logical source-unit name from CLI and compares hashes directly. No hash normalization was used.

Deterministic gzip preserves large raw payloads byte for byte. The separate offline projector is `../tools/project_owners.py`; portable audits do not run the CPU. The built diagnostic.test saved in the permanent checkpoint directory was built separately and has a different hash from the go-test execution image; native context hash in each payload is the authoritative process identity for that run.
