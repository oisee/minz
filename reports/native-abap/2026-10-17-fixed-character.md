# Owned ABAP character data, with complete-report reference checks

Fixed-length `TYPE c` now owns its bytes. Assignment copies content, truncates
on the right and fills the remaining destination with spaces. CLEAR restores
spaces. Comparisons examine content with space padding instead of comparing
addresses. This closes a semantic gap in the [native ABAP objective](objective.md),
not the complete string/type/runtime goal.

## Representation and native path

Report globals are byte arrays with their complete space-padded initializers.
Local DATA owns automatic array storage. Generic FORM TYPE c receives the
actual object's data address and its u16 length. USING and CHANGING can alias
one object; forwarding preserves that actual-object contract. Character
literals may be passed to USING only after the existing transitive no-write
check. No pointer-cell assignment stands in for character copying.

For example, this unmodified ABAP operation copies five source characters
into a three-character object:

```abap
DATA x TYPE c LENGTH 5 VALUE 'ABCDE'.
DATA y TYPE c LENGTH 3 VALUE 'xyz'.
FORM copy USING source TYPE c CHANGING result TYPE c.
  result = source.
ENDFORM.
PERFORM copy USING x CHANGING y.
```

The destination becomes `ABC`. Copying it back to x produces `ABC  `.
The [helpers](../../minzc/pkg/abap/character.go) are typed Nanz loops through
ordinary HIR/MIR2/Z80, not a separate string interpreter or handwritten ASM.
Local initialization expands to multiple statements without hiding the DATA
name in an extra lexical block.

## Full reports, not harness-only FORM calls

Three complete reports are frozen in
[character-reference](../../minzc/pkg/abap/testdata/character-reference):

| Report | Evidence |
|---|---|
| [copy-report](../../minzc/pkg/abap/testdata/character-reference/copy-report.abap) | Truncation, right padding, content comparison and CLEAR |
| [alias-report](../../minzc/pkg/abap/testdata/character-reference/alias-report.abap) | Local array, nested PERFORM, source/destination alias and read-only literal USING |
| [ordering-report](../../minzc/pkg/abap/testdata/character-reference/ordering-report.abap) | Equality/inequality, both ordering directions, padded equality, blank ordering and exact WRITE output |

Each neighboring JSON records the source SHA256, complete-report entry,
provider version, final globals and console transcript. The independent
`@abaplint/transpiler`/runtime 2.14.8 executes the unchanged report itself.
There is no harness FORM invocation or input mutation.

Raw and final MIR VM execute main and compare complete character buffers and
the four-byte integer marker. VM console hosting intercepts only putchar.
Native machines run assembled Z80 in both ordinary bare-metal mode (HALT with
balanced SP) and the existing called-main codegen contract (RET to F100,
SP F000). The latter re-emits the same final MIR/allocation with CalledMain;
no source or assembly patch is used. Native BDOS2 interception records the
actual character output and returns to the caller. Stack canaries and the
arithmetic-error object are checked. Ordering-report's exact transcript is
`A  A    `, including all fixed-width spaces.

[Directed tests](2026-10-17-character-artifacts/directed.txt) pass all three
reports plus explicit refusals for non-ASCII text, numeric/text conversions,
unsupported mixed string comparisons and writes through literal arguments.
Single-quote character versus backtick string literal identity is preserved
for subsequent string hardening.

## Backend bug exposed by complete execution

Bare-metal void main used a tail JP for its final CALL. The callee's RET then
consumed a nonexistent program return address and skipped main's final HALT.
Native output happened correctly before execution escaped. Tail calls are now
excluded at that halting entry boundary; called-main and CP/M entry contracts
retain their normal return behavior.

An independent native MIR regression covers bare, called and CP/M modes,
including observable writes and PC/SP. The old external-ABI harness CALLs main
and checks return/SP, so it now explicitly requests CalledMain instead of
relying on the former tail-call escape. Argument checks are unchanged.
[Entry/ABI log](2026-10-17-character-artifacts/main-tail.txt).

## Verification scope and remaining work

The first full core run passed HIR, ABAP and the embedded parser; it exposed
that external-ABI harness mismatch in MIR2. The initial log is retained as
[core-first](2026-10-17-character-artifacts/core-first.txt), not relabeled PASS.
The corrected complete MIR2 package rerun passed; its duration is recorded in
[mir2-recheck](2026-10-17-character-artifacts/mir2-recheck.txt).
Selected existing tokenizer, implementation main and Agon entry pipeline
checks passed in 2.751 seconds.
[Selected pipeline log](2026-10-17-character-artifacts/selected-pipeline.txt).

All three sources compile through the real CLI with PATH=/nonexistent,
without Node/npm or external tools.
[CLI invocations and exits](2026-10-17-character-artifacts/cli-compilation.json).
The binary, sources, references and emitted ASM are kept locally under
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-17/native-abap/fixed-character`.
From minzc, build with `go build -o /tmp/minzc ./cmd/minzc`, then compile e.g.
`/tmp/minzc pkg/abap/testdata/character-reference/copy-report.abap -o /tmp/copy-report.a80`.

This checkpoint admits the byte-character ASCII path for fixed c DATA and
FORMs. Owned dynamic string storage/copying, Unicode, substring overlap and
numeric conversions are not established. String FORM ABI and character
method/attribute ABIs are refused until implemented. Selection-screen field
transport has a preliminary copy into owned c storage but is not certified
by these reports. Guarded CP/M runtime currently rejects the legacy sel_show
ASM body even for these no-input reports; that runtime integration remains
open. Ordinary Z80 and called-main tests above do not claim guarded CP/M
execution of the ABAP reports.

Full-width input, remaining executable/system-field admission, dynamic-string
reference evidence and main integration remain required. The full goal stays
active.
