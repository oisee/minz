# Production Z80 assembly repair (P8)

Baseline: `6b8fc396` (the worktree's origin/main base). Measurements use the
production PBQP pipeline, `--asserts none`, then the built-in MZA assembler.
The seeded generator is `.local/fuzz2.py`: seeds 0–1099, 1,100 generated
programs, no generation skips. No `scripts/fuzz_diff.py` or
`scripts/assert_matrix.py` existed at the baseline.

All 522 tracked source examples were attempted, including archived examples
and Objective-C sources. 215 fail compilation on both versions; those are
reported separately from assembly failures. Both parser self-host examples
already assemble at this baseline, despite the older task observations.

## Before and after

| Measurement | Main | Repaired |
|---|---:|---:|
| Seeded programs failing assembly | 683 / 1,100 | 0 / 1,100 |
| Tracked sources failing assembly | 65 / 522 | 61 / 522 |
| Tracked sources failing compilation | 215 | 215 |
| Corpus files with validator diagnostics | 37 | 25 |
| Corpus validator instruction markers | 1,331 | 408 |
| Successfully assembled corpus binary bytes | 141,594 | 157,697 |
| Binary bytes for the same successfully assembled files | 141,594 | 141,630 |
| Emitted corpus assembly text bytes | 3,872,997 | 3,917,272 |

The larger binary total includes four recovered programs; the common corpus
increases by 36 bytes (0.03%). FIX1 reduces the previous branch total by
1,060 bytes (158,757 → 157,697). No previously assembling corpus file stops
assembling.

Four tracked sources newly assemble:

- `examples/glsl_sphere_demo.minz`
- `examples/nanz/nc.nanz`
- `examples/nanz/rotozoomer.nanz`
- `examples/nanz/test_irc_minimal.nanz`

## Root causes

Counts below are unique assembler diagnostics with an emitted source-line
mapping; identical repeated pass diagnostics are counted once. Validator
markers are collected separately. A single file can contain several classes.
The assembler often reports an undefined symbol as “invalid operands.”

| Class | Fuzzer before | Corpus before | Corpus after | Emitting path |
|---|---:|---:|---:|---|
| Spilled byte extension / zero high byte | 631 | 8 | 0 | `mir2/z80codegen_move.go`: `genExt`, `genSext` |
| Spill labels introduced after data emission | 449 | 31 | 0 | `mir2/z80codegen.go`: `genFunc`; `z80codegen_move.go`: `fixOrphanedTSMCStores` |
| Wide/word spill operands in register-only forms | 1 | 472 | 0 | `z80codegen_inst.go`: constants; `z80codegen_move.go`: copies; `z80codegen_alu.go`: shifts/arithmetic |
| Missing split-function definitions | 105 | 5 | 0 | `hir/split.go`: dependency walker, `ApplySplit`, `splitRecursive` |
| Byte ALU with spill or pair operand and A as rhs | 17 | 8 | 0 | `z80codegen_alu.go`: `genBinOp` |
| SBC with promoted index-register byte | 2 | 3 | 0 | `z80codegen_alu.go`: `emitSBCHL` |
| Conditional intrinsic call to a non-existent/raw label | 0 | 1 | 0 | `z80codegen_inst.go`: `OpCallCond` |
| Unresolved external/import/inline-assembly symbols | 0 | 214 | 214 | Frontend/import/stdlib symbol lowering; `genCall` and inline assembly |
| Address operands without backing local/global storage | 0 | 51 | 51 | HIR address lowering → `z80codegen_inst.go`: `OpAddrOf` |
| Missing MIR operands (`?`) | 0 | 38 | 38 | HIR lowering → `z80codegen_call.go`: `emitCallArgs` |
| Other invalid inline/legacy forms | 0 | 58 | 56 | Frontend inline assembly → `OpAsm` |
| Other assembler diagnostics | 0 | 3 | 3 | Unresolved EQU symbols and a duplicate inline label |

Representative failures include `LD _spill_v_f_r29, 0`,
`LD (_spill_v_f_r48), A` without a definition, `PUSH _spill_fp_div_r14`,
`SRA _spill_fp_mul_r7`, `JP f_split_7`, `ADD A, _spill_v_f_r20`,
`SBC HL, IY`, and `CALL Z, @mir.io.print.nl`.

Spill high bytes now address `slot+1`. Byte operations use the existing legal
memory/register helpers. Word shifts stage through a preserved register;
wide spill operations stage main and shadow words through preserved pairs,
and wide copies/returns carry both words. Spill storage uses three/four bytes only for 24/32-bit integer values;
struct and pointer values use two-byte address slots. Orphan TSMC stores are rewritten before
spill references are collected.

Split dependencies include single returns and nested expressions/statements.
Split callees preserve return types and callers forward their return values.
A returned split call is protected against repeated splitting and duplicate
function definitions. Conditional calls use regular call lowering, which
already shares `sanitizeIdent` with function definitions and handles inline
intrinsics and fixed addresses.

The remaining failures require frontend/runtime/import repairs or changes to
legacy inline assembly, rather than legalizing spill operands. Examples:
`CALL disk_read`, `CALL sql__sqlite__sqlite_query`, `LD HL, val`, `LD C, ?`,
inline references to `zx_console_con_attr`, and prose emitted as assembly.
They are retained as failures, not stubbed or silently accepted. The Zork VM
source also exceeds the assembler scanner line limit; this parse-level error
has no emitted instruction-line diagnostic and is retained separately in the
raw assembler log.

## Correctness guard and regressions

This correctness evidence covers **fuzz2-shaped programs only**. Assembly
success does not establish general compiler correctness.

All 683 newly assembling seeds produce the same result on Z80 and the MIR2
VM: **683 newly assemble + VM-correct; 0 newly assemble + VM-wrong**.
679 also agree with the generator's independent Python source oracle.
Four source-oracle/compiler disagreements remain and are reported as found
semantic discrepancies rather than counted as source-oracle passes:

| Seed | Python expected | MIR2 result | Z80 result |
|---|---:|---:|---:|
| 56 | 15,271 | 15,015 | 15,015 |
| 784 | 50,813 | 50,809 | 50,809 |
| 812 | 55,334 | 55,590 | 55,590 |
| 967 | 56,519 | 56,775 | 56,775 |

The isolated cause is literal-only expressions being typed u8 even in a u16
context: `let y: u16 = 7 + 255` produces 6 instead of 262. The Python oracle
is right; MIR2 and Z80 share the frontend error. Seed 56 contains `255 + 3`.
These are not Z80-versus-VM mismatches.

FIX1 checks all 1,100 seeds, including previously assembling ones: zero
assembly failures, 1,100 VM/Z80 agreements, 1,095 source-oracle passes.
The fifth shared oracle failure is seed 399 (expected 712, VM/Z80 456), which
was outside the earlier set of 683 newly assembling seeds.

Regression files:

- `minzc/pkg/mir2/p8_invalid_asm_test.go`: assembling and executing byte/word
  spill extensions, accumulator-rhs arithmetic, index-byte subtraction,
  preserved spilled-constant staging, widening to a spill, wide arithmetic and
  shifts, an in-place wide spill, conditional intrinsic/sanitized calls, and
  word spill shifts.
- `minzc/pkg/mir2/p8_spill_labels_test.go`: orphan-store data definition.
- `minzc/pkg/mir2/p8_fix1_test.go`: the critic u16 constant block-parameter
  regression (`0xff34` versus `0x1234`), every stored u24/u32 byte checked
  after poisoning, and two-byte struct/pointer slots without shadow staging,
  for both explicit and rescued spills.
- `minzc/pkg/mir2/p8_fix1_internal_test.go`: exhausted staging pairs cause an
  explicit codegen failure, rather than falling through to invalid assembly.
- `minzc/pkg/hir/p8_split_test.go`: return forwarding on VM and Z80, nested
  conditional dependencies, and prevention of repeated returned-call splits.
- `minzc/pkg/hir/exhaustive_judge_test.go`: replaces the old known-u32-
  assembly-failure skip with 65,636 executed Z80 sums, also checked in MIR2.
  It writes callee spill slots directly and does not judge the real memory
  parameter calling convention.

Reverting the production changes gives exit 1 for the P8 regressions;
restoring them gives exit 0. The additional conditional-dependency,
returned-split, and in-place-wide regressions were also individually checked
red with their fixes reverted and green after restoration.

The per-assert matrix reparses each source, forces Z80, and isolates each
module assert. Sandbox checks retain preceding assertions because their
shared state is a dependency. It covers imported assertions as well as the
source file's own assertions; it uses Nanz, Pascal, and Frill frontends.
The baseline has 4,016 checks: 3,070 pass and 946 fail. The repair has zero
newly failing checks and two newly passing checks.

## Gates and retained evidence

All required gates were run sequentially with `set -o pipefail`,
`GOCACHE=/tmp/minz-go-cache`, and `GOFLAGS=-buildvcs=false`:

| Command (from `minzc`) | Final exit code |
|---|---:|
| `go build ./pkg/... ./cmd/...` | 0 |
| `go test ./pkg/hir ./pkg/mir2 -count=1` | 0 |
| `go test -short ./pkg/pipeline/... ./pkg/c89/... -count=1` | 0 |
| `go test ./pkg/nanz -skip '^TestShowcaseCompileAssemble$' -count=1` | 0 |

The first HIR gate exited 1 because its deliberate old-u32-limitation marker
required replacing the skip when assembly began succeeding. The replacement
Z80 judge passes; the final gate exits 0.

Raw generated sources, assembly, binary sizes, compiler/assembler logs,
validator lines, classified offending lines with codegen paths, oracle
results, assert matrices, and gate logs are retained under `.local/`.
Collection helpers are `.local/measure.py`, `.local/report_errors.py`,
`.local/check_values.py`, and `.local/assert_matrix.go`. That directory is
intentionally not committed.

## FIX1 correctness limits and remaining bugs

The critic reported gen3 programs with globals and u8 temporaries: 150/150
assembled on the P8 branch but 0/150 were correct; main assembled 35/150
and had 0 correct. These observations show pre-existing wrong-value bugs
surfacing once assembly errors are removed. They are separate from the
fuzz2 correctness evidence above. The retained gen3 script has several modes;
rerun results below identify the mode rather than assuming equal populations.

Isolated pre-existing defects retained for follow-up:

- Memory-parameter ABI mismatch: a caller uses `physName` to emit a store
  such as `LD ($F072), DE`, but the callee reads `_spill_v_f_rN`. The add32
  judge bypasses this mismatch by poking callee slots directly; its comment
  now describes that limitation.
- Byte XOR of truncated u16 values computes zero in the isolated repro.
- Tail-call parallel moves clobber B when setting up small constant arguments.
- Literal-only expressions use u8 arithmetic in a u16 context, including
  `let y: u16 = 7 + 255` producing 6. The shared fuzz2 oracle failures are
  compiler errors, not evidence against the source oracle. This also explains
  the critic's four P8 oracle seeds plus three in its separate fuzz2 run.
- Remaining invalid assembly includes shifts on IX/IY half registers and
  u32 spill-to-global stores.
- `emitMov32` still performs four-byte moves for u24 values; a neighbouring
  three-byte spill can be overwritten. Wide move/truncation behavior also
  remains outside the constant-store regression's coverage.

P8 also fixed conditional-call argument setup (`OpCallCond` now uses normal
call lowering) and `highByte(spill)` addressing the low byte twice instead
of `slot+1`. FIX1 limits wide staging and spill sizes to 24/32-bit integers,
stores every byte of spilled constant block parameters, and caches register
information once per function. Insufficient staging pairs now abort codegen
with a diagnostic containing the function, operation, and pair counts
(the string-returning codegen API reports this as a panic).

Binary size changes from the prior P8 commit:

| Source | Before FIX1 | After FIX1 | Delta |
|---|---:|---:|---:|
| `examples/c89/fatfs_lowlevel.c` | 1,605 | 1,545 | -60 |
| `examples/glsl_sphere_demo.minz` | 3,775 | 2,878 | -897 |
| `examples/nanz/canvas_house.nanz` | 931 | 889 | -42 |
| `examples/nanz/typed_print.nanz` | 892 | 826 | -66 |
| `examples/nanz/rotozoomer.nanz` | 2,641 | 2,646 | +5 |
| All successfully assembled corpus files | 158,757 | 157,697 | -1,060 |

Every FIX1 regression was observed failing with its repair removed (exit 1),
then passing after restoration (exit 0). The exhaustion regression removes
only the diagnostic guard so it tests a silent fallback, not a build failure.
Raw FIX1 evidence is retained in `.local/fix1-*`; `.local/` is not committed.

FIX1 rerun of the retained `gen3.py`, seeds 0–149, default `all` mode:
150 compile, 118 assemble, 32 fail assembly (IX/IY-half shifts such as
`SRL IYL`), and 0 pass Z80 source-oracle assertions. Of the 118 assembling
programs, all 118 fail correctness; MIR2 passes 94/150 source assertions.
This population includes shifts and u32 operations and differs from the
critic's quoted 150/150-assembling population.

FIX1 per-assert comparison against unchanged `origin/main` (`6b8fc396`):
4,016 checks, 3,070 → 3,072 passes, zero newly failing, two newly passing.
Corpus remains at 215 compilation failures and 61 assembly failures out of
522 tracked sources, with no new failures. The four Go gates all exit 0.

A second gen3 run with the retained script's `noshift` mode (same seeds,
including u32 and conditional calls) compiles and assembles 150/150:
1 Z80 source-oracle pass and 149 wrong values; MIR2 passes 92/150. This
result does not establish correctness beyond the one passing program.
