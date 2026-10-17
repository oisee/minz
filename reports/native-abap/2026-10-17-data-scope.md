# ABAP DATA: object lifetime and source visibility

A DATA declaration inside an ABAP loop must not reset the object on each
pass. Local numeric, fixed-character and owned-string objects now receive
storage and start values once per procedure invocation. Their declarations
are collected into the HIR entry block; subsequent passes retain mutations.
A later invocation creates fresh local state.

DATA inside report events or their control-flow blocks now belongs to report
global storage. A FORM can observe that same object rather than an accidental
main-local. Duplicate report objects and duplicate local/parameter objects
produce diagnostics. Invalid start-value expressions remain refused; named
constant declarations are not implemented by this admission.

Moving storage creation must not move source visibility. Local objects use
separate physical HIR names, and the source binding changes at the declaration.
An earlier read of an identically named global therefore keeps reading the
global. Numeric PERFORM arguments and character/string reference bindings use
the physical local address after declaration.

## Evidence

Four unchanged complete reports match independent
`@abaplint/transpiler`/runtime 2.14.8, raw VM, final VM and assembled Z80:

| Report | Exercise | x | y | seen |
|---|---|---:|---:|---:|
| loop | Numeric local retained for three passes, then fresh on second call | 4144 | 13 | 2 |
| conditional | Declaration inside an IF in the loop, visible afterwards | 4144 | 13 | 2 |
| characters | Mutated c/string retained across passes, fresh on second call | 40 | 2 | 2 |
| report-global | DATA inside event loop observed by a FORM | 148 | 13 | 1 |

The native calls verify observable four-byte globals, restored return PC/SP,
a canary and the original return word. Source SHA256 and independent outputs
are frozen beside the
[unchanged sources](../../minzc/pkg/abap/testdata/data-scope-reference).
The independent runner is
[loop-reference.mjs](../../research/native-abap/loop-reference.mjs).

Separate numeric and character shadow probes verify early global reads,
subsequent local reads and unchanged global storage on both VMs and Z80.
These are specification-backed tests. The numeric probe fails to execute in
provider 2.14.8 because its generated JavaScript uses a lexical local before
initialization (TDZ). It is not counted as provider parity.

The rules come from ABAP documentation for
[DATA](https://eduardocopat.github.io/abap-docs/7.54/abapdata/) (visibility from
its position),
[VALUE](https://eduardocopat.github.io/abap-docs/7.54/abapdata_options/)
(literal/predefined-constant start values) and
[local data](https://eduardocopat.github.io/abap-docs/7.54/abenlocal_data_glosry/)
(procedure context).

## Reproduce and remaining work

From minzc:

```sh
go test ./pkg/abap -run '^TestDataScope' -count=1 -v
go test ./pkg/abap ./pkg/abap/internal/abapparse ./cmd/mzv -count=1
```

Set MINZ_ABAP_LOOP_ARTIFACTS to save actual source/ASM/BIN for these probes.
The compiler's admission still needs its final audit, including scalar
method RETURN, remaining executable/SQL additions and guarded CP/M runtime
closure. This checkpoint does not establish recursion or general SAP runtime
compatibility. Main integration remains part of the active goal.
