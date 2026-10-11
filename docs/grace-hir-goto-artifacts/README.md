# Goto and C89 evidence preparation

`export.go` extracts the sole literal `const src` inside
`TestGotoC89SourceVMZ80`, using the Go AST. A `src` in another test cannot replace
it. It independently parses that test's `ProfileWorkload.Calls` literal and
preserves all ten calls in their exact order. Each known function has an explicit
mathematical result oracle. Additional functions or invocation arguments require
an explicit exporter/oracle update.

The exporter checks raw HIR/MIR execution against independent result and static
storage/access expectations. It assembles the compiler's emitted assembly and
runs the actual Z80 binary, with Grace disabled and enabled. Every invocation's
complete declared memory snapshot and per-byte declared data access matches the
raw execution. The binary SHA256 equals the target harness digest. Instruction
fetches, stack traffic, initialization, and snapshot reads are excluded from the
declared data-access contract. ASM and HIR/MIR dumps are gzip compressed without
altering their exact source bytes; gzip output has no timestamp or file name.

Source identity currently comes from the test/exporter wrapper: it explicitly
supplies the SHA256 of the exact extracted C source. **This does not demonstrate
C frontend source-unit or source-line collector support.** The C frontend does
not currently populate those fields. The explicit fixture wrapper maps the unique exact `for(int i=0;i<3;i++)`
source token to the sole structured loop in `loop_entry`. The byte offset, line,
byte column, and full source SHA are computed from the extracted bytes. It checks
that there is exactly one such token and one structured loop across the fixture;
additional loops require explicit mapping updates. This is controlled fixture
provenance, not general C frontend collector support. The integrated parent compiler exporter and independent retained-evidence
verifier passed. All ten calls return `1/7/9/7/11/7/9/7/2/2`. Grace disabled and
enabled produce the same 195-byte assembled binary and 434 total T-states; SHA256
`a6f8eb582ccc1762232455df6a15f8cc4d076b6a6e218e3522f4df6837c0fa93`.
This is a correctness repair, with no performance-improvement claim.

From the integrated compiler's `minzc` directory:

```sh
GOCACHE=/tmp/minz-grace-gocache go run ../docs/grace-hir-goto-artifacts/export.go -out ../docs/grace-hir-goto-artifacts
python3 ../docs/grace-hir-goto-artifacts/verify.py --dir ../docs/grace-hir-goto-artifacts
```

`verify.py` independently checks retained source/binary digests, results, complete
static state, declared per-byte traffic, invocation costs, and readable gzip
dumps. It does not rerun the compiler or emulator; `export.go` does those runs.
`history/` preserves earlier failing scope/C89 inspections and bounded independent
review probes. History observations are not final compiler gate evidence.

The `cli/` subdirectory retains actual `/tmp/minzc-hir-goto-final` speed/size runs of the
30-call typed tokenizer workload. The independent Python host model checks all
global values, complete resources, and every per-byte resource read/write for
raw, postcompile raw, and actual Z80 candidate executions. Global access counts
are checked by VM/Z80 agreement. A separate Go decoder recomputes both typed
evidence SHA256 values and all four candidate binary SHA256 values. Speed chooses
candidate 1: 2,259,865 T-states and 4,475 bytes. Size chooses candidate 0:
2,270,286 T-states and 4,346 bytes. Both selected binaries exactly match the prior
TSMC-stage retained bytes; no old compiler or emulator was rerun. `commands.txt`
records the actual final executable. Both final CLI runs completed after the
mutable function-pointer verifier repair, and both independent audits passed.
Earlier CLI executions are retained in `history/pre-final-verifier/`. The C89
ten-call exporter was also rerun after the final verifier change, with unchanged
results, complete state/access checks, costs and binary SHA256.

The mutable raw function-pointer repair is verified by focused HIR/MIR tests and
six existing C89 MIR assertion annotations. These observations establish MIR
behavior only: full function-pointer workload emission to actual Z80 remains
unsupported, and this stage makes no closure ABI claim. Exact old-HEAD overlay
files and its failing CLI observations are retained under `history/`, alongside
current C89 HIR shape comparison and focused passing checks. The overlay failed
because the earlier verifier rejected reassignment of the C raw pointer `fp` as
an immutable callable alias; C HIR source shape was unchanged. No source frontend
rewrite is attributed to the pointer repair.

Historical .go files are preserved as exact gzip bytes after packing. Extract a requested historical file before using its retained Go overlay mapping; absolute /tmp paths identify the original capture environment. Final production source/tests are linked by the report source mirror.

Final package checks pass: HIR 8.703s, MIR2 4.333s, Nanz 1.575s (known showcase separately compared), C89 5.380s; CLI 20.074s. Final pipeline PASS250.312s; final four budget compilations and exact source/ASM/binary comparison PASS against the prior TSMC stage,344/344/232/344 bytes; prior722.080s machine gate retained without a fresh workload rerun. Prior intermediate failing CLI/package logs remain labeled history and are not final gate results.

The21 directed hand-built HIR cases use synthetic unit metadata solely to exercise the profile harness; this unit fixture metadata is not source provenance evidence. Actual C89 source identity and sole loop mapping are separately derived from the preserved source bytes.
