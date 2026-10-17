# Native ABAP: readiness audit and main integration

The ordinary ABAP frontend embeds the pinned Go parser from abapiti and
lowers a checked subset through HIR → MIR2 → assembled Z80. The work was
developed on `feat/native-abap-hardening` and is integrated into main by
[PR #190](https://github.com/oisee/minz/pull/190) at merge commit
`3e26dbe27a1737e8eb8657d495fb149d5cc43639`, with a passing post-merge audit.

## Original criteria and evidence

| Requirement | Authoritative evidence | Status |
| --- | --- | --- |
| Ordinary ABAP compilation without Node/npm | `pkg/abap/native.go` directly calls the embedded parser, with no fallback; empty-PATH CLI checks and `TestABAPCPMOrdinaryCLI` use the ordinary entry | Implemented and checked |
| Complete tree, positions and diagnostics against the same parser version | `TestPinnedTypeScriptParserParity` compares complete typed results to ten SHA-verified TypeScript fixtures, including malformed-source diagnostics, at core 2.120.56 | Checked within the pinned fixture scope |
| CLEAR and explicit refusal of unsupported executable syntax | Actual-object CLEAR tests with explicit invocation; executable, expression, declaration, class, report/FORM and SELECT admission probes | Implemented for the documented subset |
| Precise numbers, strings and CHANGING | Checked i32 arithmetic/input/output; owned c/string storage; real reference FORM ABI, alias/forwarding tests and conservative mode refusals | Checked within the documented admission rules |
| Two unchanged whole programs: independent reference, raw/final VM and assembled Z80 with memory and PC/SP | `TestCharacterFullReportReferenceVMAndZ80` and `TestStringFullReportReferenceVMAndZ80`; fixture SHA and `entry: full report` are checked before execution | Checked; more than two whole reports covered |
| Code, report and launch commands in main | [PR #190](https://github.com/oisee/minz/pull/190), merge commit `3e26dbe27a1737e8eb8657d495fb149d5cc43639` | Integrated (PR #190, 3e26dbe27); post-merge audit PASS |

The two representative whole-report sources are
[fixed-character copy](../../minzc/pkg/abap/testdata/character-reference/copy-report.abap)
and [string alias/forwarding](../../minzc/pkg/abap/testdata/string-reference/alias-report.abap).
Their adjacent JSON files record independently executed ABAP transpiler/runtime
2.14.8 results and the exact source SHA256. Tests inspect actual global bytes,
string lengths/capacity, output and error status in both VMs. Native tests
execute both ordinary bare termination and the called-main contract, with
PC/SP, canary and return-word checks. They do not patch the source or use MinZ
to generate the expected ABAP result.

Function-boundary arithmetic/method tests, the SQL row model and the two
guarded numeric CP/M COMs provide additional evidence with narrower scopes;
they are not substituted for the whole-report criterion.

## What the subset means

- TYPE i is signed 32-bit with checked overflow and specified division rules.
  Approximate p/f/n/d/t mappings are refused.
- TYPE c is fixed owned ASCII storage; string is a bounded owned object,
  without a general heap. Copies and writes preserve the tested byte semantics.
- Fully typed FORM USING/CHANGING uses actual object addresses. Writable USING
  is a reference; a literal is accepted only with a conservative no-write proof.
  Untyped, VALUE and LIKE formals remain explicit refusals.
- IF/ELSEIF, short-circuit expressions, DO/WHILE and early exits have native
  evidence. Full-width SY-INDEX and SELECT cleanup survive nested RETURN.
- WRITE supports counted stream output and default templates. General SAP
  list formatting, exception unwinding, full OO dispatch and classic dynpros
  are not claimed.
- SELECT scalar transport has actual SQLite VM tests and native row-model
  tests. EXEC SQL/seed requests use checked typed calls; errors stop with
  fault 6. General Open SQL, native database-server linkage, host bindings,
  INTO TABLE/ALV and automatic TUI dispatch remain outside certified support.
- CP/M guarded execution is proven for two numeric fixtures with 1024 bytes
  of stack. Unsupported reachable pointer/runtime paths still refuse.

The current [ABAP guide](../../examples/abap/README.md) supplies build/run/test
commands. [Parser provenance](../../minzc/pkg/abap/internal/abapparse/PROVENANCE.md)
includes both licenses, pinned hashes and a reproduction installer. The full
generated closure costs roughly 34 MB of Go source; parity here is evidence
for ten complete fixtures, not a proof for every ABAP program.

The [PBO/PAI–Nanz TUI plan](2026-10-16-pbo-pai-nanz-tui.md) is a subsequent
extension. It does not replace this goal's semantic or main-integration gates.

## Integration gate

The integration core passed HIR (29.199 s), MIR2 (110.527 s), ABAP (36.416 s),
embedded parser (0.304 s) and MZV (1.234 s). The initial Nanz run failed the
Agon MOS boundary harness: it called main as a subroutine while assembling
bare-main HALT behavior. The harness now explicitly generates the existing
`CalledMain` contract; it retains the same output and PC/SP expectations.
The Nanz package then passed (14.821 s). Later WHERE arithmetic/text admission
refinements passed focused checks; no failing test was skipped.

The rebuilt ordinary CLI, with `PATH=/nonexistent`, compiled the template
report to ASM and produced its exact independent output in MZV. A real
seed/EXEC SQL/SELECT report printed `65537` through actual SQLite in MZV.
This is a host-SQL check, not execution of a native database server.

The final local integration run and real CLI receipt are saved with the
SQL-execution artifacts. The normal gate includes `pkg/abap/...` and
`cmd/mzv/...` so parser parity,
admission, whole-report and actual SQLite checks remain maintained.
Historical checkpoint reports are retained as history; this matrix and the
current guide describe the final admission boundary.

The [event-boundary/PR-gate follow-up](2026-10-17-event-blocks-and-pr-gate.md)
records a later semantic correction, the first CI failure and the explicit
called-main harness repairs. The earlier Nanz language run excluded the legacy
compile-only showcase inventory, matching the existing PR gate; it did not
establish a complete showcase PASS. Final CI
[run 39000547135](https://github.com/oisee/minz/actions/runs/39000547135)
passed all required checks; the advisory output sweep failed with the previously
explained output/exit-code changes, and advisory Go lint was cancelled with no
result. Post-merge result on clean main: `Main audit PASS: 3e26dbe27a1737e8eb8657d495fb149d5cc43639`.

The post-merge audit used a clean detached checkout of main at `3e26dbe27`, with
Node absent (PATH=/nonexistent for the compile/run steps).
Checks, all exit 0:
- 756d589e4 (PR head) is an ancestor of main; checkout clean; no minzc/ or stdlib/ diff between PR head and main
- builds of minzc (mz) and mzv
- go test ./pkg/abap/... ./cmd/mzv/... : pkg/abap 42.665s, internal/abapparse 0.215s, cmd/mzv 1.686s, all ok
- go test ./cmd/minzc -run '^TestABAPCPMOrdinaryCLI$' : ok 1.374s
- Node-free compile of pkg/abap/testdata/write-reference/templates.abap to CP/M ASM, and mzv --headless output
  equal to the independent reference output in templates.json

The audit receipt is kept with the machine-local evidence (receipt SHA256
`29847c5afc467b0a694e6a7227825039a16d9124ec0de614a97589ce27d984a4`).
