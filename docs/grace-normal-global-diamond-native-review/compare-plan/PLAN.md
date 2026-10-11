# Preserve-pair unsigned comparison: bounded design only

Negativeglobal-loadreuse remains1137339T1998B vsbaseline1135370T2000B. No defaultoptimizeractivation justified. First alternative isretainoriginalreload forspeed; tiny2Bsizegain onlyif explicitboundedmeasuredchoice. Moreambitiousbackendhypothesis below must beisolated/measuredseparately. No source/CPUchange inthisplan.

ExistinggenCmp16 usesdestructive SBC HL,rr, withEXstaging forlhsDE/rhsHL. saveWrittenScratch buffers selectedsequence, computes actualphysicalwrites andpreservesunrelatedlivepair aftercomparison. WithreusedposDEfuturelive, EX+SBC overwritesDE andwrapperaddsPUSHDE/POPDE21T. Thisissafetycorrect, notbug todelete. genCmp generic materializePendingFlag/materializePendingAcc/saveAccForCondRet canmove liveA beforewordcmp; their semantics mustremainunchanged unlessseparatelyreviewed.

Firstcandidate: exactunsignedword operands alreadyin distinct ordinarypairs HL/DE/BC, canonicalTyU16, ordinaryCmpEq/Ne/Ult/Ule/Ugt/Uge only; no signedordering, syntheticcarry, bytepromotion, IX/IY, spill, constantsrematerialization or unknownwidth. Require A dead and nofuturependingAcc/flag condition needingpreservation; otherwiseexistingSBCfallback. Preferrestrictfirstprototype Ult/Uge branchuse only toavoid bool/two-jumpstate complexity. Match actualflagresult expectation independently and preservecmpSwapped/cmpNeedsTwo orientation, notinheritoldstaging toggles.

Sequence with original operands:
LD A, lhs.high
CP rhs.high
JR NZ, done
LD A, lhs.low
CP rhs.low
done:

C equalsunsignedlhs<rhs; Z equalswholewordequality. Ifhighdiff, finalCP high retains C/Z; ifhighequal, finalCP low supplies C/Z. Bothpairsunchanged, A scratch. Noinitialcarryclear required because CP ignoresincomingcarry. Cannotuse S/PV forsignedword: highunsignedsign/overflow semantics differfrom16bitSBC, sofirstcandidateexplicitlyunsigned. ForUle/Ugt codeconsumerneedscorrecttwo-test CLE/CGT state; forUlt/Uge C/NCsimple. Boolmaterialization mustreadlogicalpredicate sameasoriginal, notassumeallbool=C; repeatedpredicatefutureALU/branchtest requiredifthoseconditionslaterallowed.

Costusualregisterbytes: highdiff4+4+12=20T; highequal4+4+7+4+4=23T. Sequence6B (LD1,CP1,JR2,LD1,CP1), SBCstagingEX+OR+ED+EX5B27T; savedliveDE adds2B21T so48T7B. Thislocalarithmeticdoesnotprovewholegain. Currentcur inputspos/src_lencommonlysamehighbyte, costs23T; mayavoidallstackpreserve. Existingno-livepairSBC27T isnearer, butsubsetshould initiallyonlyselectwhenSBCwouldpreservepairandAdead. DirectHL/rhsDE fallbackwithoutEX19T canoutperformbytelex; do notreplaceallwordcompares.

Caches: invalidateA only, maintainpairsunchanged; pendingAccReg ifdeadclear/consistent. setpendingFlagReg currentDst, logicalconditiontrackedcorrectly. saveWrittenScratch parserrecognizesCP/JR/LD; bytelex shouldavoidg.invalidateHL/DE orcmpSwapped afteroldbranch. IfcontextgenericcallalreadyrelocatesA unnecessarily, do notaddglobalautorestore: gate/delegateexistinghelper carefully. No memorynativepermissiondescriptor neededforregisteronlysequence; controlflownewlocalJR mustfit2bytejump (body2bytes). Global/stacksnapshotcontracts unchanged.

Meaningfulactualmachine tests: bothoperandordersHL/DE,HL/BC,DE/BC forallowedconditions anddeadA; highless/equal/greater, lows0/255, 0/65535, signedbitboundary32767/32768 unsigned; incomingC/Z combinations; operandslivefutureadd/store. ActualflagslogicalC/Z checkedbutdon'tpromiseunrelatedFbitsasABI. Boolresult0/1 materialization+branchbothoutcomes, repeatedpredicateafterALU ifallowed. LiveA andsigned/synthetic/spilled/IXcase musttakeoldfallback (actualmathcorrect), nofakeforcedallocoverlapclaim.

Firstisolatedmanualoptin codegenchoice, nodefaultparentmerge. Source/rawsemantics unchanged; existingnegativeMIRhelperonlyprivatecandidate. Measureoriginalandcandidatefull30 throughclosedrunner onceafterfinalreview, exactbaselineBIN/ASM/allnativeSHA referencebeforeaccept; noperPCcountsmanufactured. FinerPCprofilecanfollowonlyifactualprofitability/side effectsneedresolve; do notrerunredundantcorpusbeforecodefreeze.

Largergoal: generic safewordcomparison selection basedactualoperandloc/liveness/condition and measuredcosts, enablingglobal-value reuse withoutstackpenalty. Still fullrealparser/source/hoststateprefix correctness; notassemblytextpeephole orfunction-namecur specialization.
