# Literal call cycle rematerialization

База08e3ae7f, freeze5897e1de до codegen changes. Production chain f1b1e289/a3a0e4b3/fb5771c2: только `z80codegen_call.go`, pipeline/контракты/interference не изменяются. Новые2 training и4 heldout SHA заморожены до замеров.

Прямой CALL получает реальный allocated callee home (или canonical фиксированныйABI). Только byte exact-storage closed cycle допускает замену ребра на immediate assignment. Source должен иметь единственную OpConst def **в том же блоке перед этим exactCALL**, одинаковый scalar integertype, без function/blockparam aliases и не liveaftercall по CFG. Не indirect/flags/spill/wide/functionaddress. Nonliteral sources копируются целиком прежде immediates через общий emitter; identity сохраняется. Producer LD намеренно остаётся: измеренный выигрыш включает его стоимость. Word cycle не меняется — дешёвый EX DE,HL нельзя заменить более дорогими literal loads. Runtime guards/caller saves остаются обычными.

Первый вариант «все dead literals» семантически прошёл, но greedy стал5477B/15841initT вместо5465/15790. Это отрицательное evidence `uncosted-negative`: LD r,n7T иногда хужеLD r,r4T. Итоговый вариант ограничен cycles и не меняет greedy image/cost.

| Whole unchanged tokenizer PBQP | Before | After |
|---|---:|---:|
| ordinaryBIN |5593B|**5533B**|
| adapted nativeBIN ORG0400 |5597B|**5537B**|
| init_keywords |16868T|**16273T**|
| historical two-input sequence |327286T|**326691T**|
| new frozen training sequence |412898T|**412303T**|

Оба independent lexer/interner/fullstate/stdout/code/SP oracle PASS; scan/render unchanged. Greedy5465B/15790initT unchanged; newtraining411820T. **PBQP всё ещё dominated bygreedy**. Это полезное уменьшение реального PBQP productionoutput, не новаяпобедаPBQP selector и не завершениеPFCCOцели.

Speed/Size выбор наnewtraining строгоdeterministic выбираетgreedy; exactsame savedASM/BIN повторяется на4heldout inputs безnewsearch:885117T/5465B, fulloraclePASS. SelectedBIN сравнивается между savedtraining/heldout payloads независимымaudit. Это prospective повтор compilerstrategy selection в test-owned wholeprogram nativeharness, не publicgeneralPGOAPI. Beforepatch current strategies измерены отдельно на тех же frozeninputs; before/afterheldoutwinner identical. No sourceedit/runtimeZ3/productionprofilehook.

Native literal-cycle test: typedbytecycle иwordnochange, live/nonlive,256literal×256other values×4=262144actualCALL/RETexecutions; registervalues/F(all256)/IX/IY/SP checked. Word other values here are permutationx*257, **не all65536word literals**. ReducedcalleeRET preserves incomingargs, validates boundaryshuffle notcalleealgorithm. Mixedbyte/wordhalf+identity256cases, noncanonicaldefinitions6negativecontrols, existingpairhalf tests bothcall/edge productionentrypoints. Originaltokenizer executes actualcalleebodies/globals/clobber/caller-save semantics. No genericallcallsproof fromRETfixture.

ScopedfinalnativePASS0.229s. FullMIR2PASS87.127s atf1b1e289; later narrowingwordprofit/currentcallpresence checkedfocusednative and finalfrozenpipeline actualtokenizer/runtime/packet/prospective allPASS. Finalsourcefrozenexecutable builtfb5771c2 identity inbuild.json. Fullsuite wasnotrepeated aftersafe narrowing; current exactfinal output+guards proven. GuardedoriginalSelfLanz135stdoutbytes419878T/3987B andtokenizer146bytes411987T/7391B SP/entirestate/stackguardsPASS. Statefulpacket8857→7888T52Bunchanged. PhysicalhardwareCPM notclaimed.

Reproducefrozenexe cwdminzc/pkg/pipeline with `-test.v -test.run '^(TestCallLiteralTokenizerProspective|TestPFCCOOriginalTokenizerRenderingConsumer|TestCPMRuntimeGuardedTokenizerMain|TestCPMRuntimeOriginalSelfLanzMain|TestPFCCOMeasuredNativePacket)$'`; MINZ_CALL_LITERAL_TOKENIZER_EXPORT absolutefreshoutputdir; PFCCO_TOKENIZER_EXPORT absolutehistoricaloutputdir. Fullsources/options/MIR/allocation/ASM/BIN/observations/negativeapproach retained. Run `python3 reports/call-literal-rematerialization/audit.py`. Share `/mnt/safe/minz/grace/2026-10-12/call-literal-rematerialization/`.
