# ABAP event boundaries and PR gate corrections

The final audit caught a report-control error: event bodies were concatenated
into main, so RETURN in INITIALIZATION or AT SELECTION-SCREEN ended the entire
report. The runtime now gives regular event blocks their own internal procedure
boundaries. START-OF-SELECTION retains its specified special behavior: RETURN
stops further reporting events.

The rules come from the ABAP 7.52 keyword documentation:

- [RETURN](https://help.sap.com/doc/abapdocu_752_index_htm/7.52/en-US/abapreturn.htm): normal processing-block exit, with the START-OF-SELECTION exception.
- [START-OF-SELECTION](https://help.sap.com/doc/abapdocu_752_index_htm/7.52/en-US/abapstart-of-selection.htm): implicit executable prefix precedes explicit START blocks.
- [Event blocks](https://help.sap.com/doc/abapdocu_752_index_htm/7.52/en-US/abenevent_blocks.htm): repeated regular blocks execute in source order; AT SELECTION-SCREEN is a procedure and can have local DATA.

The semantic adapter preserves separate block boundaries and an aggregate
event view. Lowering checks their agreement so an edited aggregate body cannot
be silently ignored. Repeated START blocks remain a single reporting sequence;
regular blocks get separate calls, including repeated INITIALIZATION and END.
AT SELECTION-SCREEN local DATA now uses the existing procedure-local storage
and shadowing rules. Other report-event declarations retain report storage.

Nine event/order/RETURN cases and one selection-local shadowing case pass raw
VM, final VM and assembled Z80 with global bytes, loop-index state, PC/SP and
canary checks. A fixed selection-input adapter is explicitly test-owned; it
does not claim native TUI integration. The independent 2.14.8 transpiler reports
`Initialization, transpiler todo`, so these event tests use specification
oracles, not a fabricated independent reference. The previous unchanged
whole-report reference gates remain separate and pass.

## First PR run

[PR #190](https://github.com/oisee/minz/pull/190), run
[38997728626](https://github.com/oisee/minz/actions/runs/38997728626), stopped
because three new pipeline tests were missing from the exhaustive profile
catalog. All three are now classified fast; no test is omitted to repair CI.

The subsequent local catalog run selected all 438 fast entries from 809 total
in three process shards. It exposed four native harnesses that placed a return
address on the stack but assembled bare-main HALT termination: sprite-chain,
Pascal loader observation, PL/M module entry and whole-main SQLite. The
harnesses now use the existing explicit `CalledMain` codegen contract on the
same allocated MIR. Outputs, protocol events, memory and PC/SP expectations
remain intact. Production CP/M runtime assembly is not re-emitted by this
test-only helper. All four focused regressions pass.

The advisory output sweep compared 844 sources and recorded 73 output changes,
with seven changed exit codes, all in historical ABAP examples: two invalid
indented-star comment sources, three unsupported INTO TABLE prototypes, one
unsupported interface and one reserved SQLite-runtime name. They are explicit
parser/admission refusals. Other emitted-code changes are not by themselves
proof of either regression or equivalence; required execution gates cover the
checked paths. The advisory failure is retained, not relabelled as PASS.

Before the final metadata guard, the complete ABAP/parser/MZV suites passed
35.967 / 0.314 / 1.275 seconds. Focused event/scope/metadata checks passed after
the guard. The complete local fast-pipeline rerun passed all 438 selected
entries in three shards; the ordinary ABAP CP/M CLI gate also passed (1.864
seconds). Fresh required PR CI remains the merge gate; completion of the ABAP
goal still requires main integration.

## Reviewed catalog follow-up

Run [38999430421](https://github.com/oisee/minz/actions/runs/38999430421)
passed the complete stable compiler/pipeline gate, upstream system C and
embedded demos. Nanz then refused stale reviewed SHA256 entries for the
changed SQLite/Pascal test harnesses. Four affected catalog rows retain their
original source and observation scope; current harness/helper hashes and a
[refresh receipt](../../docs/grace-integration-338/called-main-refresh.txt)
record the checked contract change. Historical manifests are not rewritten.
The full existing Nanz language gate passed after this refresh (14.627 s),
metafunction CLI checks passed (10.377 s), and both Z3 smoke scripts passed.
Fresh required PR CI remains the merge requirement.
