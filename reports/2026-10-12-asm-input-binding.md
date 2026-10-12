# HIR ASM inputs: value names before register aliases

During strictCPMTUIadmission prototype, canonical matcher refused actual `(in x,y)` because Asm.Ins was[x,x], despite Contractparams[x,y]. Exactbeforemetadata retained fromcurrentTetris compile. In lower.go namex/y mapped unknown physicalregisterclass0, thenmatchedfirst unannotatedParam.RegClass0 beforevariablefallback. Source AsmOperand documented as inscopevariable; parse(in) readsidentifier names; verifier acceptsvariables orvalidphysicalregister.

355c467c resolves inscopevariable first; fallbackphysicalalias onlynonzero class. Unit tests x/y, variableAoverridesphysicalA, explicitA/C aliases ofpinnedparams; unknownidentifier explicitlyrefusedbyhir.Verify (direct LowerModule remains legacyunchecked API, not publicadmission). This corrects use metadata; currentTetris actualnativevalue ABI alreadyA/C, so no claim defaultnativewrongvalue. Matcher remainsstrict. PublicCPMTetris stillblockedbydynamicboard_set storeboundproof; runtimeadmissionprototype notintegratedwiththisfix.

FocusedtestsPASS, frozenmetadata/artifact SHA attached. NoTUI source/scenario edits. No broadgate required here; root integrationgate separate.
