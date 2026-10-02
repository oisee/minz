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
