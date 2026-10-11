# Unchanged self-Lanz parser: prospective held-out selection

Exact new inputs were committed and pushed in `c533cf10` before collection or measurement. Three training forms describe counting/folding/telemetry functions. Six disjoint held-out forms exercise recursion, records, common-prefix identifiers, repeated interned symbols, comments, whitespace and unsigned decimal wrap. The parser source is unchanged, SHA256 `0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526`.

The public APIs `RunZ80MemoryHostLoopProfileWithOptions` and `MeasureMemoryHostArenaAppendWithOptions` collect only training inputs and choose among four existing proved transformations. Policy is fixed Legacy, objective pure speed. Current MemoryHostBaselineOptions only supports pair-comparison policy; internally it explicitly constructs Options with ContractOpt omitted. After the approved boolean fix, contract optimization is therefore false in every candidate. This experiment records that choice rather than changing API behavior or historical measurements.

| Training mask | Actual Z80 T-states | Whole target bytes |
|---|---:|---:|
| original | 1 016 414 | 2364 |
| arena-first | 1 017 340 | 2485 |
| append-copy | **992 049** | **2344** |
| both | 992 975 | 2465 |

The actual selector picks append-copy alone. Arena peeling loses 926 T and adds 121 bytes on these inputs; it is not forced. Append-copy saves 24 365 T (2.40%) and 20 bytes on training.

After selection completed, the selected choices were reconstructed and compiled without any held-out selector call. The native harness refused any byte difference from the training candidate. It then executed precisely the same adapted BIN on the six frozen held-out forms:

| Held-out execution | Actual Z80 T-states | Whole target bytes |
|---|---:|---:|
| original BIN | 2 141 572 | 2364 |
| selected BIN | **2 093 170** | **2344** |

Held-out benefit: **48 402 T (2.26%), 20 bytes**. Selected BIN SHA256 `24f6b56e2960ff965275cba73bb842b12651c82c1548a898032e07d6979b2a0a`. Input ordering retains string interning across calls while resetting position and node bump; both splits start from separate clean memory states. Source, node and string regions have explicit limits; no failed or partial candidate is accepted.

Independent Go parser logic checks the root address, full node/string/source regions, position and bump globals, and every ordered host memory read/write (address and value). It is separate from HIR/Grace. The native harness validates the same complete observations. Portable `audit.py`, with no compiler imports, independently parses all saved inputs and checks all four training candidates, both held-out binaries, selector choice, and exact events. It passes. This verifies S-expression parsing and state, not whole Lanz-to-machine-code compilation or a CP/M terminal user interface.

The label-audit prints an undefined `node_tag` warning, but the emitted module defines `node_tag EQU peek`; assembly and execution succeed. It is a warning caused by the audit's alias recognition, not a discarded compile failure. Tests retain this warning.

Frozen test executable: `/home/alice/dev/minz-artifacts/grace-checkpoints/parser-prospective-heldout.test`, SHA256 `6f6f8ae3ae836ff3e05561313873c3a9455715d9552607914dcced588f8cffee`. Run from `minzc/pkg/pipeline` with `-test.run '^TestParserProspectiveHeldout$' -test.v`. The frozen executable passed in 27.37 s. No whole slow pipeline suite was rerun. Artifacts include both workloads before selection, training profile/association, all four candidate ASM/BIN/MIR/allocations/observations, both held-out observations and manifest hashes.

This is a prospective result for this unchanged parser and these frozen inputs. Together with the distinct unchanged HashMap workload, it supplies multi-program held-out evidence; it does not establish that every Grace rule helps every program, nor that backend correctness is complete.
