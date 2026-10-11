# Closed combined CLI input/output refinement

Read-only review of current owner plan6cd1b6da at baseb1e0fa66. This document
refines earlier ed219e23 review; production CLI is not yet available. It modifies
no source/tests and repeats no native execution.

## Minimal explicit flags and route

`--memory-host-arena-append DIR` is the sole mode opt-in.
`--memory-host-workload FILE` is required; `--memory-host-profile FILE` optional.
No mode flag means ordinary CLI remains unchanged. Dependent flags without mode
must reject. Require original .nanz, z80 backend, explicit targetcpm aligned with
closed cpm-z80-v1 layout; do not silently label a CPM result as default Spectrum.
Default speed, --Osize selects size comparison only. Candidate compiler options
stay identical/default; goal never reapplies --Osize outside selector.

Dedicated dispatch must happen before ordinary compileViaHIR assertion filtering,
GPU path, profile opts or extra optimization hooks. Reject all ordinary HIR
profile collect/apply/auto/joint/rule modes, legacyPGO, alternate backend/emit,
GPU, assertion selection/force/run modes, unsupporteddebug/optimizationdisable,
HIRGrace/LIR or extra compile flags which would otherwise be silently ignored.
Validation uses existing global state consistently, and tests must restore it.
Only -o selectedASM copy is allowed besides fixed artifact directory outputs.

## Original source, workload and profile ownership

Read original source once with an explicit bounded limit. Capture immutable bytes,
exact sourcefilename/BaseDir/Stdlib context and CPMtarget. Every public factory
callback parses a NEW original module from those same bytes/context. Do not copy
already parsed/transformed AST or normalize unitname differently after profiling.
Imports require the existing source-unit complete identity checks; a fresh imported
file drift must fail compatibility/function-origin replay. No automatic assertion
filtering or invented origins for the private transform.

Strict workload loader uses existing4MiB bound, recursiveduplicate and unknown
field rejection, exact schema/hosts and finite regions. The controlled pipeline
then owns it before callbacks. Optional profile8MiB cap before decoding/callbacks;
strict current closed loader verifies complete typed fields/UseNumber/sourcecontext
and full fresh replay. Never accept arbitrary nativeASM, precomputed costs,
transformedfactory, externalhost implementation or fabricated expected results.

If optionalprofile omitted, use real closed RunMemoryHostLoopProfile with SAME
source/workload/executable context, serialize unchanged, then selector's fresh
association replays it. This avoids cross-executable profilecontext mistakes while
retaining actualcounter provenance. Save generatedoriginalprofile so output can
be reviewed; do not silently refresh a stale explicitly supplied profile.

## Path safety and writing

Validate source, workload and optionalprofile against -o and ALL artifact names,
including candidate subdirectory payloads before execution/writing. Abs equality
and SameFile catch existing symlink/hardlink aliases. Resolved existing parents
matter for nonexisting output paths; plain string paths cannot catch an aliasing
symlink directory. Artifact directory must actually be a directory, not source or
inputfile. No output may replace original input/importedunit while execution still
uses cachedbytes or rereads imports. If dependency paths cannot be safely enumerated,
conservatively reject output layout capable of overwriting source/importtree.

Preflight all destinationpaths, not just selectedASM. Fixed files include copied
source/workload/profile, fullselection, dedicateddecisions and selectedASM/BIN;
all4 candidatebaseline JSON/ASM/BIN/allocation/MIR diagnostics and audits. Do not
allow a user-supplied profile named selected.bin to be overwritten by export.
Also reject -o colliding with fixed artifacts (includinginode aliases) to preserve
one unambiguous payload per path. Apply same checks at writes where appropriate;
no partial export should turn a failed run into an apparently complete proof.

## Native output and record binding

Selector returns four actual baselines with selectedmask. Export selected
Evidence.Target.Assembly and Binary EXACTLY; do not recompile/reassemble through
ordinary codegen, apply generic ProfileDecision or inherit unbound ordinaryCLI
options. Include complete selection schema, chosenchoices, proof and evidencedigest
in a separate dedicated decision record, not old profile decisions format.

These binaries include the closed workload adapter and fixed code/stack/return
layout. Describe them as bounded-workload native artifacts; no unrestricted
standalone runtime/externABI promise. Save full source/workload/originalprofile,
all4 candidates and typedproofsource/origins so chosen evidence remains reviewable.
SelectedASM bytes and selectedBIN SHA must match selected candidate evidence;
typedallocation/finalMIR/sidecar hashes and actualT-state sum also remain bound.

## Required narrow validation and final audit

- No mode default route unchanged; dependentflags/conflicts reject beforefactory,
  CPU or writes. CPM/z80/sourceformat/goal restrictions explicit.
- Workload/profile size, recursiveduplicate/unknown/stale fields, source/import
  drift and explicitcross-executable profile reject; omittedprofile truly collected.
- Existingfile/symlink/hardlink/parentdiraliases for source/workload/profile/-o/
  candidatepayloads refuse; outputs cannot overwrite any input.
- Selectednative bytes/schema/mask/proof/evidence consistency; all4 candidates
  saved including losing results. No ordinary compile after selection.
- BuiltCLI original30 speed+size once by owner afterfreeze; reviewer audits saved
  source hostoracle plus typednativepayload bytes without duplicateCPU.

There is no production approval from this design-only report; final source and
builtpayload evidence remain required. Parent compiler/worktree untouched.
