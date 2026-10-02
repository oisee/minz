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
passing in general, not only declared-clobber calls. The critic's single extern
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
