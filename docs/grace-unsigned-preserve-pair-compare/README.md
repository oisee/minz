# Isolated unsigned preserve-pair compare

Base2908f18f. PrivateExperimentalUnsignedPreservePair falsebydefault, threecodegenfiles/newmachine test only, no pipeline/defaultactivation. Beforegenericmaterialization exactU16pair Ult/Uge flagF, ordinarydistinctHL/DE/BC, Adead inclpendingvalues, constantsexcluded. OnlywhenoldSBC writesfuturelivepair. OldSBCphysicalwritesHLalways andDEwhenrhsHL/lhsnotHL staging. BC/DEnolivelHLfallback; readonlyreviewpending.

160actualmachinecompare+branch/typedwordreturns pass: fourHLpairorientations×2predicates×10boundpairs×2incomingflags,20/23T highdiff/equal,C/Zlogical, allpair/SPvaluespreserved. LiveA/signed/two-test/deadpairfallback byteidentical; directguardtestsconstant/pendingcondition/pendingacc refuse. FixturesreturntypedU16HL, liveAU8A, deadpredboolA; forceallocationlegalatcmp testsegment only. No promiseunrelatedFbits. Sourcecorefrozenreviewbeforefull30.

Дополнительный прямой тест cache state сначала отказал из-за неполной ручной инициализации codegen: отсутствовал ComputeLiveness(f), а pending-регистры имели нулевые значения вместо NoReg. Исторический failed log сохранён в focused-cache-fixture-failure.txt. Исправлена только тестовая инициализация; emitter остаётся byte-exact 8084758c. Тест отказов теперь сначала подтверждает допустимый baseline, затем проверяет константу и живые pending-регистры. Финальный focused gate: PASS 0.016s.
