# Final source admission review: closed combined CLI

Reviewed frozen9d29c7bc95369dea12a52752236204b83f75462d at permanent
memory-host-arena-append-cli; parent baselineb1e0fa66. Threefile CLI inventory:
main.go four added lines, new memory_host_cli.go, new memory_host_cli_test.go.
This reviewer performs source review and saves artifacts only: no compiler
edits or duplicate native workload execution. Owner focused tests/build and
final builtin/nativeCLI payload audit remain separate required evidence.

Result: no remaining concrete input/output admission hole found in bounded
synchronous CLI contract. The draft findings below were resolved beforefreeze.

## Resolved draft findings

1. ParseWithOpts could reread imports and output fixed artifacts could replace an
importedsource. Final factory uses nanz.Parse and requires complete exactlyone
SourceUnits entry equal literal sourceFile and actualcachedsourceSHA. Imported
units are conservatively outside firststageCLI; no genericdependencyauthority.
2. Go typed JSON decode permits single case aliases such as Schema. New local
workload keytree check is exact recursively for typedstructrecords while named
maps retain user identifiers. Existing duplicatechecker already casefolds keys;
Schema/schema combined keys were already duplicate refused. This report corrects
that draft distinction. Null/nonobject workload also rejected.
3. Initial new decoder read workload twice, decoding A but casechecking B. Final
all duplicate/typed/tree checks use one bounded owned bytebuffer, preventing
read-version laundering. Old ordinaryCLI decoder is unchanged.
4. Initial decision digest referred compact JSON while selectionfile waspretty.
Final selection is MarshalIndent once; exact same encodedbytes are saved and
hashed by decision.SelectionSHA256. Portableaudit can hash savedfile bytes.

## Admission and source ownership

Only explicit --memory-host-arena-append directory entersmode; workloadrequired,
profileoptional. Dependent flags withoutmode reject. Compile dispatch branches
before ordinaryHIR assertionfiltering/extraoptimization. Defaultabsence remains
same ordinaryroute. Validation requires .nanz, z80, targetcpm, codeoutput empty
orcode. OrdinaryHIR/legacyPGO, extraHIRGrace/Grace/LIR/solver, optimizationdisable,
GPU/emit/TAS/debugexecution/MIR/AST/annotation/alternateassert flags rejected.
Osize chooses measuredsize objective only, without changedcandidatecompileropts.

Source8MiB, workload4MiB, profile8MiB bounded. Source bytes readonce and every
factorycallback creates fresh originalHIR from exactlysamecachedbytes/path, CPM
target and singleactualsourceSHA. Workload singlebuffer strict duplicates,
unknownfields, case-exact typedkeys and objectshape. Optionalprofile is readbounded
and passed to unchangedstrictselectorloader/freshwholeprofile replay. Omitted
profile is actually collected in same executable/source/workload before replay;
no fabricatedcounts or automaticrefresh ofexplicitstalepayload.

## Files and output identity

Allfixed main/subcandidatewritepaths are preflighted against inputsource/workload/
profile/-o and eachother. Canonical resolver follows existing symlinks then
reconstructs nonexisting suffixes from resolvedparents; SameFile checks existing
hardlink aliases. Directorymustbedirectory. Validation repeated after execution
immediatelybeforewrites. No input/import authority is writable; imports refused.
This is not a concurrent hostile filesystem security guarantee between preflight
and WriteFile; ordinary synchronous filesystem model remains explicit.

Export selectedTarget.Assembly/Binary directly, without ordinaryrecompile or
legacyProfileDecision. Fourbaselines/allocation/MIR/ASM/BIN saved, source/workload/
profile complete and typedselection/dedicateddecision included. Decision binds
exactselectionfileSHA, selectedmask/choices/binarySHA/T-states/bytes. Optional -o
copies same selectedASM. The nativeproduct includes closedadapter/fixedlayout;
report it as bounded-workload artifact, not unrestricted applicationABI.

## Narrow gate status and interpretation

Owner source tests cover unsupportedmode/source/outputformat, partialflags,
ordinaryprofiling/HIRGrace conflicts, hardlink/symlink/-o and resolvedparent
aliases, nested casealiaskeys/duplicates/null and validJSON. Nativebinary build
completed separately. Final original30 bothgoals and smalloptional-sameexeprofile
path may proceed once at thisfreeze; independent savedoutput host/typedSHA audit
must still be retained before parentintegration. No nativecosts asserted here.

Structural/sourceproof/GC/lifecycle authority remains the reviewed finite
selector and controlledbaseline APIs; CLI adds no arbitraryASM, transformed
factory, usercallback, effectpurity or numericalprotectedstorage grant. No
postprepareHIR certificate or inheritedprofilecounter is claimed.
