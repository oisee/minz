# Grace PGO: независимый выбор структурных правил

Новый opt-in API `MeasureGraceRuleVariantSets` измеряет четыре module-wide политики: ни одного правила, только constant-if, только invariant-base, оба. Каждая политика пересекается с комбинациями наблюдавшихся eligible pointer-walk sites. Максимум четыре pointer sites дают 64 кандидата. Прежние `MeasureGraceVariantSets` (bundled) и `MeasureProfileVariantSets` (pointer-only) сохраняют прежние варианты и schema.

CLI: `--hir-profile-independent-rules` требует `--hir-profile-joint`, `--hir-profile-auto DIR` и `--hir-profile-auto-workload FILE`. Дополнительный флаг сохраняет совместимость существующих joint команд. Функциональное ограничение в joint режиме запрещено.

Новый evidence schema `minz.joint-profile-rule-variant-sets/v2` содержит явный `rule_policy` каждого кандидата, actual HIR/lineage, профиль с совместимостью, SHA бинарника и измеренные вызовы. `common_grace_rules` означает включение обоих правил; v2 потребитель должен читать `rule_policy`. Evidence SHA256 связывает политики с остальными данными. Порядок политик: 00, constant-if only, invariant-base only, 11; внутри каждой группы прежний pointer-mask порядок. При равенстве затрат сохраняется более ранний кандидат, all-disabled baseline идёт первым. Это выбор политики всего модуля, не per-loop legality certificate и не новый proof engine.

Каждый кандидат получает свежий совместимый input HIR; shared/mutated factory и поддельные наблюдения отклоняются. Raw VM сравнивает возвраты, память и opt-in exact access counts; assembler и Z80 harness проверяют всю ordered workload. Победитель возвращает именно измеренные Steps и binary, без повторной компиляции. Предварительные HIR/lineage сохраняются отдельно от final_grace_hir (после второго Grace, перед function splitting) и postcompile_lineage/raw_run (после компиляции и split). Финальный raw VM снова сравнивается с oracle, pointer realization проверяется повторно.

На неизменном weighted checksum, 30 вызовов с длинами 0/1/5/16/64/255 и persistent caller state:

| constant-if | invariant-base | Такты вызовов | Размер модуля |
|---|---|---:|---:|
| off | off |1166563|168 B|
| on | off |1166563|168 B|
| off | on |475239|116 B|
| on | on |475239|116 B|

Speed и size выбирают только invariant-base. Результат равен прежнему bundled победителю: дополнительного ускорения на этом исходнике нет. Независимый host oracle пересчитывает результат и каждый globals/resources snapshot из параметров и байтов; retained binary повторно исполняется. Caller с двумя pointer loops проверяет все 16 комбинаций и persistent counter. Тесты также проверяют cap, forged counts, shared HIR и SHA evidence.

[Артефакты и воспроизведение](grace-pgo-rule-artifacts/README.md). Следующая очередь: workload, где комбинация правил даёт иной измеренный выигрыш, затем более точный выбор по месту с отдельной проверкой происхождения и legality.

## Регрессия конкуренции бюджета

Специальный testdata/grace_rule_budget.nanz содержит два checksum цикла и 200 literal-false ветвлений в невызванной функции. Это намеренный тест бюджета, не новый реальный corpus benchmark. Каждый из двух Grace запусков имеет 100 переписываний; constant-if расходует их раньше hoist. Independent policy может включить invariant-base отдельно. Тест требует strict improvement относительно обеих bundled политик, две фактические hoist lineage после компиляции и независимые byte-access/return/memory oracles для всех 30 вызовов, отдельно speed и size. Speed-прогон: bundled off/on 2103327 T /344 B, invariant-base-only 858267 T /232 B. Финальный speed/size прогон прошёл534.103s; каждый режим выбирает invariant-base-only. [Бюджетные артефакты](grace-pgo-budget-artifacts/README.md) содержат lossless compressed evidence, исходник, oracle и бинарники.

Бюджетный тест намеренно дорогой: каждое срабатывание пересобирает facts и проверяет lowered MIR. Для его отдельного запуска используйте `go test ./pkg/pipeline -run ^TestIndependentGraceRulesBudgetCompetition$ -count=1 -timeout=30m`; workload и критерии не ослабляются ради скорости теста.
