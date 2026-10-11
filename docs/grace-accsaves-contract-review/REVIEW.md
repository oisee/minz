# InsertAccSaves bounded independent review

Read-only currentd2682636 source. No production changes orCPU were performed. Root's observedlegacyCompileHIRWithOptions invalidassembly is real; this document separates source-confirmed omissions from unmeasured counterexamples.

## Priority1: synthetic allocation identity

pickAccSaveScratch callsf.AllocReg thenassignscombined.Locs[synth]. The function's nextReg is not raised to the maximumexistingmodule allocation key. ModuleRegs were madegloballyunique earlier, but a syntheticreg mayfall into a laterfunction's existingrange. Thisoverwrites thatfunction's physicalmapping; ordinarySteps doesnotrunthispass, explainingroutescope. ReservefreshIDs acrosscombinedallocation andallfunctiondefs/uses; preserveeveryexistingmapping. Regression must create at leasttwofunctionswithadjacentreg ranges and forceanAsaveinfirst, thenchecksecondfunction's completeallocandassembledsource.

## Priority2: scratchphysical lifetime

pickScratch onlymarksusesinthecurrentandnexttwoinstructions, andcomparesphysicalnamesexactly. A futureliveDE value doesnotreserveD/E; a futureliveHL doesnotreserveH/L. Renameextends scratch lifetime throughlateruses/terminator, so a selectedscratch mayclobber anotherlivevalue oritselfbeoverwritten beforeitsrenameduse. Thisissource-confirmedalgorithmicpremisegap, notyetnativecounterexample. Meaningfulforcedallocation fixture: liveA acrossdestructivecall, liveDE usedafterthreeunrelatedinsts/return, ensureD/E avoided orproperlypreserved. Checkallhalves/pairs andinstructiondefs/clobbers overtheactualsavedinterval, notonlynearbyuses.

## Priority3: canonical rewrite coverage and stopping

isUsedAfterIdx knowsa redefinition stop, butrenameUses rewritesallfutureSrc/Args throughdefinitionsandtheterminator. Inst.Uses includesAsm.Ins while renameUses ignoresit. renameTermReg lacksTermDJNZ (counter/body/exitargs) andTermBrIf2 (lhs/rhs/eq/lt/gtargs). ExtraRetsarealso definitions. Regression should showusedoldA beforethenafterredefinition: onlyoldvalue'susesrename, thenstop; assemblerinput oldA andDJNZ/BrIf2 edgeargs mustnotretainclobberedregister. Prefercanonicaluses/defs walkers orboundedrefusal ofunsupportedforms, notpartialmetadatarewrite.

## Other limits to state honestly

accOwner isinitializedfromfunctioncontractparams in everyblock, notonlyentry, andownerisnotclearedafterclobber whennodestinationisallocatedA. Thiscanselectstaleowners/redundantfuturemoves; requiresconcretetestbeforeclaimingwrongcode. instClobbersA is approximate (notallasm/load/conditionalcalls), andisUsedAfterIdx isblocklocal: dominatingAuseddirectlyinsuccessorwithoutedgeargsisnotcounted. Do notclaimcompletefunctionwidecallpreservationfromthislegacyheuristic. Broadpassremovalorcanonicalglobalrepair isrootimplementationchoice, notauthorizedbythisreadonlyreview.

Initialboundedproductionpatch shouldtargetobservedrouteinvalidassembly andfreshIDsafeallocation, withmeaningfullegacyroute regression. Ifscratch/rewriteconditionsarechanged, testtheir actualvalue semantics separately. Keep source/raw/optimized/allocated/nativeexpectations distinct; no broadCPU untilfinalcode+two reviews. PreservefirstinvalidASM andcomparecombinedallocationbefore/after.
