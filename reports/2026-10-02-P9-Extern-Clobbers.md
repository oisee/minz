# P9: explicit external-call clobber contracts

Baseline: `origin/main` at `6b8fc3964ca690a14a9ef4b42c22af63e56cf4e1`.

## Resolution investigation

Nanz `@extern` declarations are lowered to bodyless MIR2 functions. Z80
`genCall` emits a symbolic CALL/JP, or CALL/RST to an explicit address.
`peek` and `poke` are not special names in Nanz, HIR, MIR2, or this emitter.
Their self-hosting examples run on MZV, whose implementations are host callbacks
in `minzc/cmd/mzv/file_host.go` (heap read/write), not emitted Z80 instructions.
No Z80 preservation contract follows from those callbacks. Thus the three
self-hosting examples remain unannotated, and inline-intrinsic write derivation
and a self_lanz_parser byte-count reduction do not apply to these calls.

The actual inline cases in `z80codegen_call.go` are `@mir.io.print.*`,
`@mir.io.console.log/err`, and the error-control operations `@error`, `@check`,
and `@propagate`. These are separate compiler operations; extern declarations
cannot narrow them. Their emission is unchanged by this change.

The active Nanz example externs include MZV file/heap, SQLite, canvas, TUI and
selection-screen host APIs; `net_read/write` and `ctl_read/write`; and test
externs such as `process` and `add_to_acc`. They all remain real external symbols
in Z80 emission. There is no name-based intrinsic substitution.

## Contracts and correctness

`@extern(clobbers: "A, HL, F")` and
`@extern(0x1234, clobbers: "A, F")` carry a nullable physical register list
through HIR and the MIR2 callee contract. Nil means unknown/all registers;
non-nil empty means preservation except return registers. Unknown/unsupported
registers fail parsing. Register-pair overlap uses `pkg/z80spec.Overlaps`.
Normal functions, indirect calls and unresolved symbols retain conservative
clobbers. Invalid contracts supplied directly by another frontend fall back to
all registers.

HIR previously returned immediately upon seeing an extern, dropping both its
parameter and return contracts. Those contracts now survive lowering. Caller
argument destination writes are counted separately, and complex shuffles keep
all-register saves. Result pickup avoids unsaved live scratch registers.

The assembly judges exercise an exact declared write set, default preservation,
an implementation that deliberately violates its contract (and visibly corrupts
a value), live values through real peek/poke assembly stubs, result pickup, and
unresolved callees. A declaration that omits actual writes is a user contract
violation and can cause silent wrong code. No optional runtime canary mode was
added.

## ROM/BDOS annotations

None. `pkg/disasm/analysis/abi.go` profiles provide syscall arguments and
no-return information, not authoritative clobber/preservation sets.
`regtrack.go` conservatively produces A/F/BC/DE/HL after CALL/RST; its ABI-aware
logic describes *consumption*, not preservation. `stdlib/cpm/bdos.minz` uses
assembly wrappers around CALL 0x0005, rather than Nanz extern declarations.
ZX console helpers likewise have assembly bodies. No narrower ROM/BDOS contract
was inferred from these sources.

## Corpus comparison

`/tmp/p9_corpus.go` was run separately from the branch and a `git archive
origin/main` checkout in `/tmp/p9-main`. It walks active `.nanz` and `.minz`
examples, excludes `_archive`, `aspirational`, `experimental`, `invalid` and
`working`, and uses the Nanz frontend and the default PBQP pipeline with asserts
disabled during compilation. Successfully assembled binaries are measured at
ORG 0x8000. Each top-level assertion is then judged separately on its requested
backend(s). Sandbox assertion prefixes retain shared-state semantics.
`assert_matrix.py` is absent from this baseline. Raw results are
`/tmp/p9-{main,branch}-corpus.json` (the final line is JSON; compiler diagnostics
may precede it), with logs beside them.

| Measurement | origin/main | branch |
|---|---:|---:|
| Active source files | 206 | 206 |
| Successfully assembled files | 94 | 94 |
| Total assembled bytes | 50,757 | 51,409 |
| Nanz-only assembled bytes | 48,311 | 48,963 |
| Passing assertion/backend checks | 522 | 522 |
| Failing assertion/backend checks | 10 | 10 |
| self_lanz_parser bytes | 2,092 | 2,154 |

Zero newly failing assertions and zero newly failing compile/assembly outcomes.
The 652-byte increase is the correctness cost of restoring extern ABI setup and
preserving live values at unresolved calls. Real externs without declarations
still clobber everything. Narrow contracts remove saves in the dedicated
assembly judges; no unsupported contracts were added to the existing corpus.

The existing failing assertion checks are:

| File | Check | Baseline diagnostic |
|---|---|---|
| `examples/nanz/hello_cpm_fib.nanz` | `top/1/z80` | line 47: assert "assert gcd(12, 8) == 4" [z80]: got 8, want 4 |
| `examples/tests/asm_caller_callee.nanz` | `top/2/z80` | line 28: assert "assert test_abc() == 60 via z80" [z80]: got 70, want 60 |
| `examples/tests/asm_caller_callee.nanz` | `top/7/z80` | line 67: assert "assert test_multi1() == 6 via z80" [z80]: got 7, want 6 |
| `examples/tests/fib_parallel_copy.nanz` | `top/3/z80` | line 17: assert "assert fib(3) == 2 via z80" [z80]: got 1, want 2 |
| `examples/tests/fib_parallel_copy.nanz` | `top/4/z80` | line 18: assert "assert fib(4) == 3 via z80" [z80]: got 1, want 3 |
| `examples/tests/fib_parallel_copy.nanz` | `top/5/z80` | line 19: assert "assert fib(5) == 5 via z80" [z80]: got 1, want 5 |
| `examples/tests/fib_parallel_copy.nanz` | `top/6/z80` | line 20: assert "assert fib(6) == 8 via z80" [z80]: got 1, want 8 |
| `examples/tests/fib_parallel_copy.nanz` | `top/7/z80` | line 21: assert "assert fib(7) == 13 via z80" [z80]: got 1, want 13 |
| `examples/zx/plasma.nanz` | `top/4/z80` | line 73: assert "assert color_to_attr_s(200, 200, 200) == 7" [z80]: got 71, want 7 |
| `examples/zx/plasma.nanz` | `top/5/z80` | line 74: assert "assert color_to_attr_s(0, 0, 0) == 0" [z80]: got 64, want 0 |

## Validation

Required gates use `set -o pipefail`, `GOCACHE=/tmp/minz-go-cache`, and
`GOFLAGS=-buildvcs=false`, sequentially:

- `go build ./pkg/... ./cmd/...`: exit 0.
- `go test ./pkg/hir ./pkg/mir2 ./pkg/nanz -count=1 -skip '^TestShowcaseCompileAssemble$'`: exit 0.
- `go test -short ./pkg/pipeline/... ./pkg/c89/... -count=1`: exit 0.

Both corpus runners exited 0. A mutation audit separately disables declared
clobbers, argument-write accounting, pair overlap, safe pickup scratch selection,
unresolved-call saves, and extern ABI lowering. Each mutation must make its
specific regression test fail with exit 1, then the original source is restored.

## FIX1: ambiguous symbols and diagnostics

MIR2 calls carry a symbol name (`Inst.Sym`), not an exact function identity.
Code generation now resolves an ABI only for a unique name. Duplicate names
fall back to conservative saves; an extern with MIR2 blocks cannot narrow
clobbers either. Nanz rejects extern/body name collisions in either declaration
order and conflicting extern redeclarations, including overloaded parameter
types, register ABI, return types, address, and clobber contracts. Existing
ordinary operator overloads do not establish overload semantics for unmangled
extern symbols. Identical extern redeclarations remain accepted for imports;
parameter names do not affect signature equality.

Malformed extern attributes now show both supported clobber syntax forms.
An extra comma in a register list reports an empty clobber register entry.
The language book explains untracked shadow registers, I and R, and why AF'
is rejected. This report now follows the date-prefixed reports convention.

New regressions cover both critic scenarios, declaration order, additional
contract/ABI conflicts, ambiguous MIR2 callees with executable clobber stubs,
externs with blocks, malformed syntax, and empty register entries. Restoring
pre-fix implementation files made each of four regression groups fail with
exit 1; the mutation audit exited 0. Logs: `/tmp/p9-fix1-mutations.log`.

Required gates ran sequentially with `set -o pipefail`,
`GOCACHE=/tmp/minz-go-cache`, and `GOFLAGS=-buildvcs=false`:

- `go build ./pkg/... ./cmd/...`: exit 0.
- `go test ./pkg/hir ./pkg/mir2 ./pkg/nanz -count=1 -skip '^TestShowcaseCompileAssemble$'`: exit 0.
- `go test -short ./pkg/pipeline/... ./pkg/c89/... -count=1`: exit 0.

Critic probes reran using a compiler rebuilt from this worktree, original
sources and stubs, and the supplied assembler/emulator. p1 compiles/assembles
with exit 0 and returns A=10; p2 and unannotated control p2c compile/assemble
with exit 0 and return A=55. Emulator process exits equal those return values.
p3 rejects overloaded externs with compile exit 1. p4 and its unannotated
control p4m reject extern/body collisions with compile exit 1. The probe audit
exited 0; artifacts and full command logs: `/tmp/p9-fix1-critic/`.

## FIX2: retain extern ABIs and validate emitted symbols

Identical bodyless extern redeclarations now produce exactly one `Module.Funcs`
entry after local declarations, imports, and generated helpers are merged.
Parameter names do not affect equivalence. Clobbers compare as sets, ignoring
order and repeated register entries, while nil (unknown writes) remains distinct
from an explicitly empty contract. Conflicting declarations still fail.

Z80 symbol validation uses the emitter's `sanitizeIdent`, including register-name
prefixing (`f` becomes `v_f`) and generated-name punctuation (`x$split_1` becomes
`x_split_1`). Both pipeline entry points report collisions before emission.
Function/function and function/global collisions fail rather than reaching label
deduplication. Direct codegen also rejects invalid modules. Call resolution uses
emitted symbols and fails on ambiguity; it never substitutes a nil callee and
loses argument setup, result pickup, or a fixed address. A preservation decision
cannot discard the rest of a callee's ABI.

`hir/lower.go` now gives externs a **Params contract**, fixing extern argument
passing for small arities, not only declared-clobber calls. **Known gap:** each
extern parameter class maps straight to one fixed register, so signatures with
several parameters of the same class collide (e.g. `(u16, u16, u16)` -> HL, DE,
DE; four `u8` -> A, C, B, C; `canvas_line` in `examples/frill/graphics.frl`) and
the call site silently loses an argument. origin/main set up no extern arguments
at all, so this is not a regression; distinct-register assignment for externs is
a follow-up. The critic's single extern
`ps1` returns 3 on origin/main and 6 on this branch with the same `ADD A,C; RET`
stub. This changes the extern ABI for other frontends using HIR lowering: callers
now copy arguments into the contract registers and pick up the declared return.
External implementations must obey those parameter/return contracts as well as
any declared clobbers.

The optional removal of empty labels for bodyless externs without an address is
**deferred for corpus compatibility**. `self_lanz_parser.nanz` still calls the
unprovided Z80 host symbols `peek` and `poke`; its current output assembles with
exit 0, but deleting those two labels makes the same assembler fail with exit 1
and undefined-symbol diagnostics. The pipeline's `emitExternStubs` can also
reintroduce missing labels with RET stubs, so removing codegen labels alone would
not establish a reliable unresolved-extern error. Existing empty labels may still
fall through if no external implementation is supplied; they are not valid
implementations. Changing this behavior requires an explicit linking/stub policy
and corpus migration. No preservation guarantee is inferred from those labels.
Evidence: `/tmp/p9-fix2-critic/{with,no}-extern-labels.log`.

Regression tests execute repeated declarations (`ps`), fixed-address tail calls
(`pt`), and two imported declarations (`pe`), expecting A=6. Address tests also
require `JP 0x1234` and reject `JP ext`. Further tests reject the `f`/`v_f`
collision (`pv`), generated split-name and global collisions, and ambiguous
MIR2 callees. Disabling deduplication, clobber-set comparison, emitted-symbol
validation, and ambiguous-call rejection separately makes the corresponding
regression fail (exit 1 for each); audit exit 0. Restored-source focused tests
exit 0. Log: `/tmp/p9-fix2-mutations.log`.

Required gates ran sequentially with `set -o pipefail`,
`GOCACHE=/tmp/minz-go-cache`, and `GOFLAGS=-buildvcs=false`:

- `go build ./pkg/... ./cmd/...`: exit 0.
- `go test ./pkg/hir ./pkg/mir2 ./pkg/nanz -count=1 -skip '^TestShowcaseCompileAssemble$'`: exit 0.
- `go test -short ./pkg/pipeline/... ./pkg/c89/... -count=1`: exit 0.

Rebuilt compiler and original critic sources/assembler/emulator:

| Suite | Repro | Result |
|---|---|---|
| critic-P9 | p1 | A=10 |
| critic-P9 | p2, p2c | A=55 each |
| critic-P9 | p3 | compile exit 1, conflicting externs |
| critic-P9 | p4, p4m | compile exit 1, extern/body collisions |
| critic-P9b | ps, ps1 | A=6 each |
| critic-P9b | pt, pe | A=6 each, fixed address retained |
| critic-P9b | pv | compile exit 1, ambiguous emitted symbol |

Successful repros compile and assemble with exit 0; emulator exit codes equal A.
For pt/pe the harness redirects literal `0x1234` to a supplied assembly stub,
after first verifying the original assembly's literal address and absence of
`JP ext`. Probe audit exit 0, artifacts: `/tmp/p9-fix2-critic/`.

The critic-style sweep covers all 64 top-level `examples/nanz/*.nanz` files with
the supplied origin/main compiler and the rebuilt branch compiler: 63 compile
successfully on each, zero changed compile statuses, runner exit 0. The shared
failure is `hello_cpm_fib.nanz`'s known Z80 gcd assertion. Logs and JSON:
`/tmp/p9-fix2-sweep/`, `/tmp/p9-fix2-sweep.log`.

## FIX3: Frill registration, reserved helpers, and codegen errors

Frill transfers an adopted curried/composed helper out of `autoFuncs` into its
single top-level declaration. Imports keep the first function binding, including
its arity and IO status. This preserves main's first-definition behavior for
shared library names: `functional_demo.frl` imports both functional and math
libraries, which repeat `max`, `min`, `clamp`, `is_even`, and `is_odd`. In
particular, the two `clamp` bodies differ syntactically. Removing duplicate
frontend registrations allows `functional_demo.frl`, `pipe.frl`, `showcase.frl`,
and `stdlib_demo.frl` to compile and assemble with strict symbol validation.
Their MIR2 assertions pass. Local conflicting definitions still reach the strict
codegen validation; no assembly-label deduplication was restored.

`ValidateZ80Symbols` reserves `__mul8`, `__mul16`, `__call_ix`, all eight
`__rotate_N` entries, and the `_mir2_str_`, `_spill_`, and `_tsmc_` generated
namespaces. It also reserves actual struct-field EQU aliases and named SMC slot
patchers/aliases using their emission spelling. Functions and globals cannot
claim these names, even when a helper is unused. Both critic multiply repros,
`pm.nanz` and the unannotated `pm0.nanz`, are rejected. A preservation contract
cannot redefine the compiler's multiplication implementation.

`Z80Codegen` now returns `(string, error)` and no assembly on validation errors.
`genCall` returns an ambiguity error; the emitter propagates its first call error.
Both pipeline entry points and VIR's PBQP fallback consume these errors. The
VIR regression forces the fallback with a missing solver executable and checks
that colliding emitted symbols produce failed `FuncResult`s instead of a panic.
Success-only test callers unwrap results through package-local test helpers.

### Cross-frontend extern ABI effects

The critic's original cross-frontend sweep records **27 changed generated
assembly files** from the extern Params/return contract work. This extends beyond
clobber narrowing: Nanz, ABAP, PL/M, and Frill callers now obey HIR extern
parameter and return contracts. After stripping comments, 25 of those outputs
still differ; `02_sum_array_idiomatic.nanz` and `tui_commander_l3.nanz` have only
trace/comment changes in that snapshot. None change compile/assemble status.
The original comparison is in critic-P9c's `sw-main/` and `sw-branch/`.

For example, `tui_demo.nanz` now emits `LD E,C; LD D,0` before
`CALL sel_register_int`, placing the default value into the extern's DE parameter.
`examples/abap/hello_input.abap` gains argument setup and saves around extern
calls; `examples/plm/hello.plm` likewise changes its external-call ABI code.
External implementations must follow these Params/return contracts, as well as
any declared clobbers.

**Known pre-existing C issue:** C prototypes are not HIR `@extern` declarations.
The critic's `pc.c` declares `ext(a,b)` and calls `ext(x,x)` through `twice`.
Both origin/main and this branch emit `JP ext` without setting up `b`. Retaining
HIR extern contracts does not repair this separate C prototype lowering issue.
This is not a FIX3 regression.

### Full corpus status comparison

Baseline remains origin/main `6b8fc396`; fetch/rebase reported the branch current.
The rebuilt branch compiler and the critic's main compiler were run with
`--asserts none`, followed by the same assembler. The sweep recursively includes
all 510 `.nanz`, `.c`, `.abap`, `.pas`, `.plm`, `.frl`, `.lanz`, `.lizp`, and
`.minz` sources under `examples`, including archived/invalid examples. Both C
directories and root MinZ examples are covered; Lanz has one source outside a
dedicated `examples/lanz` directory.

| Measurement | origin/main | FIX3 branch |
|---|---:|---:|
| Sources | 510 | 510 |
| Compile successes | 296 | 296 |
| Assemble successes | 233 | 233 |
| Total assembled bytes | 133,714 | 134,283 |

Zero compile/assemble status changes and zero regressions; sweep exit 0.
The +569-byte aggregate includes the extern ABI repairs and removal of duplicate
Frill helper bodies. Logs, per-file assembly/binaries, and JSON:
`/tmp/p9-fix3-sweep/`, `/tmp/p9-fix3-sweep.log`; runner:
`/tmp/p9-fix3-sweep.py`.

The isolated assertion harness inspects all 82 Nanz/Frill sources in those two
example directories, resolves stdlib imports, disables assertions during normal
compilation, then runs each top-level assert separately with Via cleared on Z80
(the `--asserts-force z80` semantics). Sandbox checks retain each shared-state
prefix and also clear Via. Both versions run **3,737 checks: 2,801 pass and 936
fail**, with **zero newly failing checks**. This includes exhaustive Frill
properties originally intended for MIR2; forcing Z80 exposes existing failures.
`examples/frill/hello.frl` has the same pre-existing parse failure on both and
cannot run its assertions. Both runner exits and the comparison exit are 0.
Data/logs: `/tmp/p9-fix3-{main,branch}-asserts.{json,log}`; runner:
`/tmp/p9-fix3-asserts.go`.

Seven independently disabled fixes each make their regression test fail with
exit 1: helper ownership, import merging, reserved runtime namespaces, generated
aliases, codegen error returns, call error returns, and VIR error propagation.
The source is restored after each mutation. Audit exit 0; log and runner:
`/tmp/p9-fix3-mutations.log`, `/tmp/p9-fix3-mutations.py`.

Final gates ran one at a time with `set -o pipefail`,
`GOCACHE=/tmp/minz-go-cache`, and `GOFLAGS=-buildvcs=false`:

- `go build ./pkg/... ./cmd/...`: exit 0.
- `go test ./pkg/hir ./pkg/mir2 ./pkg/nanz ./pkg/frill -count=1 -skip '^TestShowcaseCompileAssemble$'`: exit 0.
- `go test -short ./pkg/pipeline/... ./pkg/c89/... -count=1`: exit 0.
- Restored-source VIR fallback regression: exit 0.

Logs: `/tmp/p9-fix3-{build,core,pipeline,vir}-final.log`.
The original critic `pm` and `pm0` probes both reject the reserved helper with
compile exit 1. `pc.c` compiles with exit 0 on both compilers and retains the
same missing second-argument setup. Probe audit exit 0; artifacts and log:
`/tmp/p9-fix3-probes/`, `/tmp/p9-fix3-probes.log`.
