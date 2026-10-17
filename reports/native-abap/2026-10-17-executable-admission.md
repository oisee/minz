# ABAP names, exceptions and selection-screen event admission

The adapter now rejects undeclared reads and assignments before returning an
executable HIR module. Previously an assignment created a local implicitly,
and a missing read received a fallback u16 type. ABAP requires declaration;
another FORM's local DATA does not satisfy that requirement.

The exception gate now names the missing capability: TRY/CATCH requires
catchable exceptions, which this runtime does not implement. The tested
exception sources already failed before this change, but only because walking
their wrapper encountered an unsupported ClassName. TRY and CATCH are no
longer treated as generic wrappers to flatten.

Selection-screen additions had a separate semantic loss: OUTPUT, ON a field
and ON EXIT-COMMAND all became the same AT-SELECTION-SCREEN event. They now
produce explicit admission errors. The unmodified validation event remains
admitted. This is a prerequisite for the [Nanz TUI adapter design](2026-10-16-pbo-pai-nanz-tui.md):
OUTPUT must run before rendering, whereas validation must run after field
transport. This checkpoint does not implement the dialog loop or connect TUI.

## Verification

Eight complete-source name probes cover assignment, read, WRITE and IF in
report and FORM scope. Two TRY/CATCH sources check the explicit refusal.
Three selection-screen additions are rejected, and the ordinary validation
header is accepted. The existing sibling-FORM isolation regression now checks
the earlier frontend error instead of waiting for HIR verification.

The full ABAP package and embedded Go parser tests pass, including existing
unchanged-report raw/final VM and assembled Z80 reference checks. These
negative admission probes need no new runtime oracle: their contract is that
no executable module is returned. No backend or runtime algorithm changed.

Reproduce from minzc:

```sh
go test ./pkg/abap -run '^(TestExecutableAdmission|TestProcedureLocalsDoNotLeak)' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse -count=1
```

Full-width input, system-field and remaining operator admission, guarded CP/M
runtime closure and main integration remain required by the overall goal.
