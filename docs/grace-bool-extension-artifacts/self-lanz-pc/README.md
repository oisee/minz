# Original Self-Lanz address profile

Replay uses the unchanged 2091-byte adapter binary, SHA256 `fed98ba78becf9431a5301f31330363cc3005f1f6692fd1f9e9183d80b9d0a51`, the same 30 calls, and the same persistent string arena. Total is exactly 1221494 T-states. Every call matches saved root, globals, full node/string arenas and ordered peek/poke events. Profiling samples PC before each actual Step and assigns the cycle delta to that instruction's function address range; call instructions belong to the caller and callee instructions to the callee. Recursive time is not counted twice, and host setup/bootstrap is excluded.

| Function | Exclusive T-states | Share |
| --- | ---: | ---: |
| intern_region | 334605 | 27.39% |
| cur | 239391 | 19.60% |
| parse_sexpr | 236919 | 19.40% |
| make_node | 141414 | 11.58% |
| is_atom_char | 120219 | 9.84% |
| region_match | 51575 | 4.22% |
| peek | 35292 | 2.89% |
| skip_ws | 21166 | 1.73% |
| poke | 15997 | 1.31% |
| is_space | 13290 | 1.09% |
| set_d1 | 8700 | 0.71% |
| is_digit | 2926 | 0.24% |

`cur` contains the hottest individual instructions (0x0560:23504T, 0x0574:23104T, 0x0565:22035T), mostly global word loads and the position/source bound path; peek adapter at 0x04F6 costs20760T for2076 loads. Detailed exact PC costs and assembler symbols are saved. These instruction counts are observed target counts, separate from loop-profile metadata.

Current source eligibility queries report zero mixed copy/region sites and zero ordered read/write helpers. This is expected: original source uses u16 absolute addresses and extern CallExpr peek/poke, while published rules require direct typed LoadExpr/StoreStmt and exact byte-pointer shapes. `region_match` reads arena first, then source; changing the order would violate its current ordered external-event contract. `node_d0`/`node_d1` read offset1/2 and3/4 via peek and do not match arr[idx*2] word helpers. `make_node` and set_d1 issue ordered writes via poke. Node constructors are not an existing word-accessor optimization opportunity. This 30-call parse workload does not execute node_d0/node_d1/printing.

The strongest next PGO kernel candidate is the 334605T intern_region scan plus its 51575T region_match callee. A new bounded original-source certificate could recognize the exact arena scan and arena-first equality loop, retain early mismatch and precise byte-read order, and measure original vs pointer-walk on the same whole caller. Prerequisite: a separately opt-in, independently validated memory-effect/ABI contract for the exact peek/poke adapter pair; generic extern calls cannot be treated as pure memory operations. Keep source unchanged and reject unsealed extern semantics. Existing typed tokenizers are not evidence of this original source's eligibility.

A separate general backend opportunity is repeated global-word load/preservation cost in cur (239391T): investigate instruction/liveness improvements independently before a global-cache source rewrite. Caching pos/src_len/src_buf across extern calls requires a proved no-global-write contract and is not currently authorized by extern declarations. Scan and cur costs are measured exclusive costs; their possible gains are not additive estimates.

`profile.go` is isolated and reads saved adapter assembly/binary; it never recompiles or mutates parent code. Original invocation: from parent minzc, go run /tmp/minz-self-lanz-pc-evidence/profile.go. Optional arguments override raw baseline input directory and output directory. `audit.py` independently checks digest, all PC/function/call totals and range attribution. Preserved source/ASM/workload/target files use deterministic gzip0mtime where applicable. Baseline adapters remain test-only; this report does not authorize production extern semantics, claim default PGO support or claim a typed source port.
