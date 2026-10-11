# Full-source FAT controlled measured modes: prospective result

With closed dead-extern admission `d2e24c78` and operation-scoped HIR referent
retention `c0a7e49f`, the unchanged original `stdlib/fs/fat12.nanz` completes the
public `MeasureCompilerModeVariants` pipeline. The pre-frozen resource workloads
contain 18 training calls and 36 heldout calls, with four original packed FAT12/
FAT16 read/write entries and independent full-buffer/nibble preservation oracles.
The prospective test now requires positive execution and no longer treats the
historical blanket disk-extern refusal as a passing result.

| Candidate | ContractOpt | MIR Grace | Training T | Complete BIN bytes |
|---|---|---|---:|---:|
| 0 | off | off | 5,572 | 9,543 |
| 1 | on | off | 5,660 | 9,689 |
| 2 | off | on | 5,572 | 9,738 |
| 3 | on | on | 5,660 | 9,882 |

Both strict Speed and Size objectives choose candidate 0. Speed's equality uses
existing deterministic earlier-candidate tie breaking. The selected unchanged
BIN passes all 36 heldout calls at 11,199 T. Its SHA256 is
`60803503e50440190c40715fc3aa2c89ad23ba8ac83a301dd6beb91c64612b6c`.
This is a null optimization result: no speed or size improvement on these workloads.
ContractOpt is slower by 88 training T (+1.58%), while MIR Grace matches T and
adds 195 bytes. Complete BIN size includes retained unrelated module functions;
timings cover only the selected four computational entries. This does not prove
all FAT functions, disk I/O, mounting or CP/M filesystem integration.

`TestFATPackedProspectiveMeasuredModes` PASS 24.343 s (both objectives). All four
native candidates are checked on training; the selected exact ASM/BIN is replayed
against independently checked frozen heldout raw VM state. Full source provenance,
factory/HIR/ABI/source-context mutation gates, retained candidate executable
binding and all-candidate failure cancellation remain active.

The preceding standalone admission commit passed full HIR (33.803 s), all existing
Z80 ProfileWorkload tests and the new negative matrix, plus ordinary/Grace full
source raw VM/native probes. The separate referent-retention fix prevents false
ownership collisions after legitimate compiler AST replacement without loosening
shared-input rejection. It is a prerequisite donor, not duplicated in this patch.

Exact source, workloads, candidate ASM/BIN/final MIR, full evidence, selected
ASM/BIN, heldout runs and logs are saved in `grace-profile-dead-extern-selected`
with SHA256 manifest. Historical refusal evidence is preserved by the earlier
admission packet and original reports. No full CLI or slow full-pipeline rerun.
