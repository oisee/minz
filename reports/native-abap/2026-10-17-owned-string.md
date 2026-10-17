# Owned ABAP strings with dynamic lengths and checked capacity

`TYPE string` now owns storage and copies values instead of aliasing an
interned C-string pointer. CLEAR makes its logical length zero. Globals and
local DATA have their own buffers; reference-mode FORM USING/CHANGING carries
the actual object's descriptor. This is progress toward the
[native ABAP objective](objective.md), not completion of its input,
remaining admission or main-integration requirements.

## Layout and admitted operations

The descriptor is four bytes: u16 length, u16 capacity, then the owned payload.
Lengths can change at runtime. A conservative compile-time bound takes the
largest character literal/fixed field in the compilation unit for the admitted
scalar-copy path. Copies among strings cannot grow beyond those sources;
concatenation and unknown dynamic producers are refused. No heap or GC is
introduced. The current bound is shared across initial allocations, not an
optimal per-object bound. Objects must fit the 16-bit layout.

FORM writes check the actual destination descriptor's capacity. If a copy
cannot fit, `_abap_error_code` becomes 3 and execution stops before payload or
length writes. This is a terminal capacity error; it does not unwind the
active caller stack or implement catchable SAP exceptions. Valid internal
copy-only reports do not hit it because their inferred storage covers all
admitted sources. A foreign caller must supply a valid owned descriptor.

Character-to-string assignment trims trailing spaces; string literals written
with backticks retain them. String-to-c copying truncates/pads as before.
Fixed c comparison uses padding. A comparison involving string converts the
fixed operand by trimming trailing spaces, then uses length-sensitive lexical
comparison. WRITE prints the actual active bytes. Numeric/text conversions,
Unicode, concatenation, string selection fields and character object-method
ABIs remain explicit refusals. TYPE string FORM currently requires actual
owned variables; literal reference arguments are refused.

[Storage, capacity analysis and helpers](../../minzc/pkg/abap/string.go) use
ordinary HIR and typed Nanz. Obsolete pointer-cell string startup assignments
have been removed. The destination length is committed only after copying.

## Independent complete-report evidence

The two [unchanged reports](../../minzc/pkg/abap/testdata/string-reference)
execute their own PERFORM calls and report body. The independent
`@abaplint/transpiler`/runtime 2.14.8 records full-source SHA256, final globals
and MemoryConsole output. There is no harness-added FORM call or input
mutation in this reference gate.

| Report | Observations |
|---|---|
| [copy-report](../../minzc/pkg/abap/testdata/string-reference/copy-report.abap) | Source changes after copying, initializer trimming, retained string blanks, c/string conversion and CLEAR; output `Alon`; i32 marker 65551 |
| [alias-report](../../minzc/pkg/abap/testdata/string-reference/alias-report.abap) | Self-alias, writable USING, local string, nested reference forwarding, mixed comparison and trailing-blank ordering; output `ABCABC `; i32 marker 65567 |

Both reports match raw/final VM and assembled Z80. Tests compare descriptor
length/capacity, active bytes, fixed-character memory, four-byte integer
markers and actual character output. Native execution checks ordinary HALT
and the existing called-main return contract separately, with stack canaries
and returned PC F100 / SP F000 for the called mode. Re-emission uses the same
final MIR/allocation and the existing CalledMain option; no source or ASM
patch is used. Backend escaping names the global c as v_c; the test checks
that actual symbol rather than reading address zero from a missing lookup.

Two additional fault probes give the destination a valid smaller declared
capacity (zero/one) and attempt a two-byte copy. Raw/final VM and Z80 stop
with code 3, leave the entire destination unchanged, do not execute the next
assignment and retain the outer return address. These are capacity-contract
probes, not claims that the independent SAP-style runtime has this allocation
limit. [Directed log](2026-10-17-string-artifacts/directed.txt).

The full ABAP package passed in 24.909 seconds, including previous numeric,
reference-parameter and fixed-character regressions. The generated parser
package passed uncached in 0.198 seconds.
[Core log](2026-10-17-string-artifacts/core.txt).
No backend algorithm changed in this checkpoint.

## Reproduction and status

From minzc, build `go build -o /tmp/minzc ./cmd/minzc`, then run e.g.
`/tmp/minzc pkg/abap/testdata/string-reference/copy-report.abap -o /tmp/string-copy.a80`.
Both complete sources compiled using the real CLI with PATH=/nonexistent.
[Recorded commands and exits](2026-10-17-string-artifacts/cli-compilation.json).
Sources, references, emitted ASM and compiler binary are saved under
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-17/native-abap/owned-string`.
Node is required only to regenerate independent references.

Full-width selection input, remaining executable/system-field admission,
guarded CP/M runtime closure and main integration remain open. The goal
stays active. These copy-only string reports do not establish full SAP string
operations, arbitrary growth or the selection-screen TUI runtime.
