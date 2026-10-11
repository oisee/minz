# First native gate saved diagnosis

Saved-only inspection of15 exportedcases; noCPUrerun. The failed test's first termination assertion masked two other issues.

HALT wrapper hasDI atdone andHALT atdone+1. Remogatto CPU retainsPC atHALT, so savedPC263 equalsHALT0x107; expected done+2=0x108 was an off-by-one harness assertion. The CPU completed withSP0xF000; no terminationwrongcodeclaim.

NewRemogattoZ80'sMemorydefaultsromEnd0x4000. FixtureORG0x100 placesglobalsbelowROM boundary; CPUstores toanswer/first are silentlyignored. GetMemoryreads the sameMemory.data usedbyCPU, so there is no shadow/stale-state access. DE4661 aftershow andA2 afterstorage-s demonstratecomputedvalue butnotstorecommit. TestfixturemustselectCPMallRAMSetROMEnd0; preservecurrentfailedpayloads.

There is also realoutputwrongcode. SavedMIR Sloopcallargumentr12 isclassacc, butallocationplacesr11 andr12 inH. Byte-loademitterpreservescountA viaEXAF,loadsdataintoA,copiesitH,thenrestorescountA. genCall'sprint.char specialcase bypassesnormalargumentshuffle andemitsOUTA withoutmaterializing itsallocatedargumentH. It outputs2,1 forAB ratherthan65,66. S255 emits255..1;Lloops similarlyoutputstaleAzeros ratherthanpayload. CStringAB output414221 iscorrectbecauseitsintrinsicexplicitlyloads(HL).

This is an intrinsicallocated-argument boundary bug, distinctfromS/Lformatlowering andthetwofixtureerrors. A narrowcodegenfix musthonor actualprint.charArg location andpreserveotherliveA/pendingflagvalues. A simplisticLDA,H woulddestroySloopcountA, so callclobber/preservation mustbeproved/tested. Do notweakenoracle oradddummyregistersourceconstraints tohideit. Originalformat/storagefixsource remainsisolated; noPASSclaim.
