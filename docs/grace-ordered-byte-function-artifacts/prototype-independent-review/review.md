# Independent ordered-byte helper contract review

Read-only review of /tmp/minz-rd16-contract-plan.md, ffdf877d isolated manual prototype /tmp/minz-hashmap-rd16-prototype, and its saved evidence. No parent/compiler production edits. Independent scripts/logs in /tmp/minz-rd16-independent-evidence.

## Semantics

The exact u16 multiplication idx*2 produces an even logical offset in0..65534. Consequently logical base+1 cannot overflow, and ((arr+base) mod65536)+1 mod65536 equals original arr+(base+1) mod65536. Exact typed source remains necessary: do not generalize to widened index arithmetic, different stride, mutable base or mixed pointer widths. The even-offset exclusion also avoids relying on differences between raw IndexExpr overflow and pointer-add semantics.

Both original and rewritten HIR evaluate low byte then high byte. Rewritten PtrAdd is pure, singleinitialpointerbinding, two LoadExpr TyU8, no wide load. Raw131612 oracle covers every index at arr0/65535 plus tiny heaps5/6/16 first/second traps; observer events before trap and complete memory are checked. Gas equality is not claimed: rewrite changes pure expression count, and plan must preserve semantic memory traps under sufficientgas rather than claim arbitrary equal MaxSteps exhaustion.

## Independent actual evidence

Python reimplementation of unchanged source hash/probe/clear/sentinel semantics audited all36 persistent candidatecall results, complete82globalbytes, eachglobalbyte reads/writes for originalraw/precompile/postcompile/actualZ80, and candidate BIN SHA/cost sums. PASS. Original133994T984B SHA0390aa22f7b028216e55bca731f3ca1bb4e91ab06e70ddead41e756235034860; candidate126680T980B SHA31e686b05d328346cd092ee65a310c111de010c53c775d3c3cd8b92ccd016f11, savedpre-byteconstant backend context. Laterparentcost must remeasure rather than reuse arithmetic.

Existing targetboundarytest verifies expectedaddress counters/no stores but does not independently forbid a thirdbyte read or check order. Independent standalone target.go fills this gap:72actualcalls(36pairs×2variants), per-instruction profiler delta across allnoncode/nonstack RAM is exactlysinglelowread followedbysinglehighread, nootherdatabyte reads, fullRAM bytes unchanged except explicitstack. Bothwrap directions and u16idx boundaries exercised. Code/stackdata overlap intentionally excluded. EmulatorhaswrapRAM, so targettrapbehavior is notclaimed; partialtraps belongrawVMproof/domain.

The fullcaller initiallyglobalszero; clear doesn'tzero keys/values. Saved calls cover startslots5/11/15, count0..16, fullinsertfailure, fulloverwrite, sentinelhas quirks; no independentlyseedednonzeroglobal/countcarry/directallhashstarts claim. This boundedcaller evidence is sufficientmeasuredworkload, not all-program performance.

## Required production-proof checks

Prototype is an exactnamedfunction manual experiment, no function-site/sourcefreshness/postproof/Gracepolicy attribution. It retains originalbaseDecl but replaces Return withoutSourceLocations transfer; that is an acknowledgedprototype provenance gap, not acceptableproductionproof. Productionmust independently bind originalreturnpath to transformedreturn, preserveallnonloop origins/SourceUnits and ordinaryloop markers, rejectdetachedactualmapkeys/sharedStmt identities/orphanpolicykeys, and normalizeunrelatedpostsplit funcs without expectedAST recompilation. Functionmetadata exact includingRetTys, ParamsSMC/IsIO/captures/customclasses/etc and immutable generatednames remain essential.

Plan's separate functionID basedcomplete sourceSHA+ABI+sourcebody avoids invented loop IDs. Enumerating alllegalhelpers inclunobserved and limitingTOTAL4dimensions meansatmost16measuredcandidates; observedcountcannotshrinkcap. Unknownfalse IDs reject aswelltrue; disabledhelpers stillproofbound; tiesfavororiginal/smallerchangedcount. LoopProfiledoesnotprovefunctionentrycounts. Sharedfactory/mutatedsource/workload/binary/proofs/policy and helpermetadata forgeries must failbeforecompile. Pure instruction/gas difference should be explicit, no claimed equal resourceexhaustion.

No concrete semantic counterexample found for current exactmanualtransformation. Meaningfulnew72targetorderedreadaudit is PASS. Productioncontract implementation and finalparentbackend measurements remain independent pending work; this report is not approvaloffutureAPI.
