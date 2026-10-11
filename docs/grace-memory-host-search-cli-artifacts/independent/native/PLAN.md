# Explicit eight-candidate search CLI: plan only

Read-only proposal against production44739f77 search API and existing fixedpolicyCLI714f9e36. No implementation, CPU, default/CLI/core changes until parent gates and explicit later release. Preserve ordinary compiler/default, Legacy CLI --memory-host-arena-append, fixedpolicy CLI --memory-host-arena-append-policy and existing serializedschemas exactly.

## Single explicit user workflow

New mode **--memory-host-arena-append-search DIR**, shared mandatory **--memory-host-workload FILE**. Optional two named inputs **--memory-host-search-legacy-association FILE** and **--memory-host-search-preserve-association FILE**, either both explicitly nonempty or neither; never collect missingpolicy silently. No --memory-host-compare-policy in searchmode: it measures0and1, fixedstable Legacy-first policy/mask order. Existing -Osize selects actual search GoalSize over all8 measuredbinarysizes; no compilerOptSize inferred or backendpolicy disabled. Default goalactualSpeed. -o optional exactselectednativeASM copy, not ordinarysecondcompile.

Detect requested mode by values OR Changed for all3newflags, including empty values; reject incomplete/orphanflags and emptyChanged pair. Reject any Changed oldLegacy directory/profile, fixedpolicydirectory/comparepolicy/association flags (evenempty) innewmode; existingmodes likewise reject any searchflagChanged. Workload intentionallyshared. No barev1/sourceprofile conversion, no scalarpolicy shortcut, no policycache winner withoutcompleteeight search.

## Admission and owned source

Before any native call or outputwrite: validate allmode/backend/target/options/assert/import restrictions as existingclosedCLI. Only unchanged exactSelfLanz one sourceunit/sourcebytesSHA accepted by searchAPI. Read bounded8MiB source once, retainbuffer; use cachedbytes nanz.Parse with exact suppliedlogicfilename inallfactories, TargetCPM andonecompleteSourceUnitexpectedSHA; searchAPI independentlyparseownedknownbytes/fullASTmetadata and fulloriginmaps. No arbitrary transformedfactory, dependency-read/mtime/hash substitution, CSE hook or nativepermission change. Inputs knownASCII JSON workload; strictowningdecoder as existingAPI. Existing ordinary/multifile compiler stays separate, not silently constrained globally.

Associationinputs individually32MiB,total64MiB, owned bytebuffers beforecallbacks. For suppliedpair, pass exactownedbytes to searchInputs{Source,SourceName,Legacy,PreserveSpeed}; APIpreflightsbothschemas/policies/source/workload/currentexe before48factorycallbacks. A malformedsecondinput must not trigger expensivefreshnativecollection offirst. For collectmode, independently collect publicwholeassociationv2 onceforeach0/1 using sameownedsource/workload/preservedCLIexe, serialize prettyJSONexactonce, then searchperforms independentfreshreplay. Failedsample/hostdenial/gas/infrastructure/provenancedrift returnsfailure; no incompletewinner, no substitutionLegacy.

## Inventory and exact file binding

Prospective outputs:
- source.nanz, workload.json, input-legacy-association-v2.json, input-preserve-association-v2.json
- search-v1.json (mandatory complete policy-search/v1), decisions-search-v1.json, selected.a80, selected.bin
- policy-legacy/selection-v2.json and policy-preserve/selection-v2.json
- each policy subdir original/arena/append/both: baseline-v2.json,target.a80,target.bin,final.mir2,allocation.json.

Total50reservedartifactfiles, plusoptional-o. Allinputfiles(source/workload/bothassoc) andalloutputs pairwisecanonical-parent/EvalSymlinks/os.SameFile checkedincludingfuturepaths beforeCPU; directorynotinputfile; aliasesamonginputs also refused. Repeatvalidationimmediatelybeforewrites. Includehardlink/symlink cases at selected.bin AND nested policy-preserve/both/baseline-v2.json inventoryendpoint to avoidforgettingsecondpolicytree. PreserveinputbyteSHA andexactsubmittedassociationbytes, eveniftypedAllocationRawMessage compactedbyloader; never rewrite suppliedenvelopebytes undersamehash. Inputpair cannotaliasoutputassociationfilesinthe targetdir.

New decision schema **minz.memory-host-arena-append-search-cli-decision/v1**: requested Goal, selected{policy,mask}, exactchoices, sourceSHA/sourceName, SearchSHA256 of exactsavedsearch-v1.json bytes, AssociationSHA256{legacy,preserve_speed} of exactlysavedsubmittedinputbytes (also matchsearch.AssociationSHA256 orderedslots), BinarySHA256, actualTStates,Bytes. Full nestedsearch remainsauthority, no independentduplicatecost/proof override. SelecttargetONLY search.Selections[selected.Policy].Candidates[selected.Mask].Evidence.Baseline.Target; asserttuplebounds/policy/choices thenwriteexistingASM/BIN verbatim; no regenerate code, profiledecision fallback or linkdifferentcontextbinary.

Allsemanticadmission/execution/refusal beforeartifactmkdir/write. BuildallJSONpayloadbytes firstincludingdecision beforewrites toavoid serializationerrorpartialclaim. IO errors canleavepartialfiles andmuststateit; no false atomic-success promise. -o writefailure fatalwithhonestpartialartifacts. Successfulsummarynames policy+mask notambiguousflatmask; logs actualSpeed/Size separatefromderived comparisons.

## Meaningful focused tests before fullCLI

- default/oldLegacy/fixedpolicy dispatch byte/behaviorunchanged; newflagsalone/emptyChanged, both/one/neitherassoc, fixedpolicytext/scalarflags incompatible.
- duplicate/unknown/null/casealias/schema/policyswap/staleexe/source/workload/failedsamples; malformedsecond suppliedpair factory0 andnoartifactdir.
- sameexactsource factorybuffer ownership, retainedSHA alteredAST rejectedbyindependentAPIgate; importsrefused withnooutputs.
- exhaustive50fileinventory aliasessource/workload/eachassoc/-o; sourcehardlink, worksymlink, secondpolicyartifactsymlink, output/outputhardlink.
- shortbothsitesobserved collectthenpairedsameexe replay, all8completebaselinev2/proof/policyrefs, exactsearch/assoc/decisionSHA, selectedASM/BIN and-o identical; inputpairbytes unchanged.
- actualGoalSize shortordering acrossbothpolicies, strictprimarytieunitcontrolearlierLegacy; failedsecondpolicy cannotpublishfirstpolicypartialwinner.
- no unboundedopenmap/list/policyflags, no derivedSizeclaim asactual.

After frozen4?cmdsourceinventory/reviews, preservego-build CLI bytes inpermanentcheckpoint, runonefull30ActualSpeed andonefull30ActualSize (each8candidatesplus2collectionbaselines), plusboundedshortpairedcollect/replay/-o. Saved-onlyauditors checkcurrentexe/source/allobservations/events/counters/48lifecycle/proofs/nativehashes/decisionexactbytes. ActualfullSize mayselectLegacy0/mask2, whileSpeed1/mask3: reportcosts/bytes independentlynotuniversalwinner. OldfixedCLI regression remainsrequired. Parent fullpackage/integrationgate/publication separate.
