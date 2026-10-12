# Повтор восьми offline Z3 проверок таблиц

Снимок compiler `604547db`. В полном pipeline `350149ad` эти восемь тестов были SKIP из-за отсутствия VIR_TEST_Z3. Отдельный повтор с `/home/alice/dev/minz-artifacts/grace-tools/z3-python/bin/z3`: все 8 PASS, 14.719 s. Производственный путь не запускает Z3; исследовательский генератор создаёт кандидаты для текущего MIR/ABI и checked admission.

Проверены реальные weighted и уже оптимальная leaf-функция, function pointer leaf, word multiplication, hash16 generation, комбинации off/greedy/PBQP × Grace × table, StrRef и перенос с повторным допуском после изменения константы/ABI. Вся область входов указана в логе; ограниченные StrRef tuples не означают весь домен 2^24.

Матрица weighted: полный модуль 44 → 20 B; сумма 256 caller-вызовов 60928 → 36352 T, то есть 238 → 142 T на вызов. Пересылки и сохранение живого x через CALL включены. Дополнительного выигрыша Grace/PFCCO на этом примере нет. Равные baseline кандидаты остаются равными; устаревшие назначения при изменённом ABI отклоняются.

Команда: `VIR_TEST_Z3=/home/alice/dev/minz-artifacts/grace-tools/z3-python/bin/z3 GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^(TestCheckedRealWeightedSolverAndNativeCallers|TestCheckedRealAlreadyOptimalLeafKeepsBaseline|TestCheckedExistingFunctionPointerLeaf|TestCheckedActualWordMultiplication|TestCheckedExistingHash16GenerationEvidence|TestCombinedContractTableGraceNative|TestCheckedStrRefEqualNative|TestCheckedExactBindingMissAndRevalidatedTransfer)$' -count=1 -timeout=30m -v` из minzc.

Этот повтор дополняет полный gate, но не меняет его исходные SKIP и FAIL. Остальные opt-in/export проверки ещё требуют отдельного аудита.
