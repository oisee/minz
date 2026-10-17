# Native ABAP: private runtime helpers and guarded CP/M

Two unchanged numeric ABAP reports now compile and execute as guarded CP/M
COM images. The native results match their independent full-report reference:
`unbounded` produces x=123, y=123, seen=0; `limit` produces x=1, y=0, seen=11.
Both use signed four-byte ABAP integers and an explicit 1024-byte runtime stack.

## What changed

HIR functions can now identify compiler/library helpers as internal. Such
helpers remain reachable through real calls and explicit assertion entries,
but are no longer implicit exported pruning roots. User-facing functions keep
their existing visibility. Internal status grants no permission for assembly,
foreign memory access or host services.

ABAP runtime helpers carry this metadata. Declarations that collide with the
reserved runtime namespace are explicitly rejected rather than replacing a
helper or being intercepted as a host service. Tests cover global, local,
FORM and PARAMETERS collisions. This is a runtime namespace boundary, not a
claim that every possible identifier form has been audited.

MIR pruning now follows conditional calls and calls whose names begin with
`@` when a concrete function exists. Unbound intrinsics do not create phantom
functions. Reachable opaque assembly still retains the conservative pruning
rule; this change does not silently grant authority to assembly helpers.

Terminal `@mir.io.halt` is emitted directly as DI / HALT / JP back to the
terminal halt. Its interface must be void with no arguments or results.
The ABAP fault path uses that intrinsic rather than a concrete opaque helper.
A native test explicitly resumes the halted emulator, executes more cycles
and checks that execution halts again without changing SP or enabling INT.
This is terminal failure, not exception unwinding or ordinary return.

## Evidence and limits

The [guarded CP/M test](../../minzc/pkg/abap/cpm_runtime_test.go) assembles the
actual generated runtime image and enters it at 0100 through a monitor CALL.
For both reports it checks all three four-byte globals, zero ABAP/runtime
status, final PC=7003 and SP=F000, the saved outer return word, and every byte
outside the loaded image except that permitted return word. Foreign instruction
fetches are rejected. These numeric reports perform no console or database I/O.

The tests also verify that unused `sel_show` is absent. Before this change,
it was retained as an exported helper and prevented guarded runtime admission.
Character reports now reach a narrower, legitimate refusal: stores through
computed pointers are not yet admitted by the guarded CP/M memory contract.
Character/string, CHANGING, input/output, native SQL linkage and Nanz TUI
require their own reachable runtime checks. This checkpoint does not establish
all ABAP reports or general SAP dynpro support.

All HIR, MIR2, ABAP, embedded Go-parser and MZV package tests pass. Focused
pipeline tests cover internal visibility, explicit entries and terminal halt;
MIR tests cover bound intrinsic and conditional-call reachability. The work is
on `feat/native-abap-hardening`; main integration remains required.

## Reproduce

From `minzc`:

```sh
MINZ_ABAP_CPM_ARTIFACTS=/tmp/minz-abap-cpm-artifacts go test ./pkg/abap -run '^TestABAPGuardedCPMFullReports$' -count=1 -v
go test ./pkg/pipeline -run '^(TestInternalFunctionsAndExplicitEntries|TestTerminalHaltIntrinsic)' -count=1 -v
go test ./pkg/mir2 -run '^TestPruneUnreachableFuncs' -count=1 -v
go test ./pkg/hir ./pkg/mir2 ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```

The equivalent compiler runtime options are `-t cpm --runtime=cpm --runtime-stack=1024`.
The test retains original ABAP, independent reference JSON, generated ASM and
COM when the artifact environment variable is set. Node is not required by
these compiler or test paths.
