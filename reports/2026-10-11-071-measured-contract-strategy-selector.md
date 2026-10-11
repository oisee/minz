# Измеренный выбор off / greedy / PBQP на реальном arena

Добавлен отдельный API `MeasureContractStrategies(factory, workload, goal, fixedOptions)` для закрытого `hir.ProfileWorkload` native domain. Он сравнивает **off, greedy, PBQP** в фиксированном порядке, по всей длине assembled BIN для Size или фактическим Z80 T для Speed. При равенстве сохраняется более ранний вариант. Возвращаются именно скомпилированные и проверенные Steps/ASM/BIN победителя, без повторной сборки с другими опциями.

Новая схема `minz.measured-contract-strategies/v1`, candidate set `off-greedy-pbqp/v1`. Старые mode4, их schema и replay не менялись. CLI нового поиска/loader/replay для новой схемы в этом шаге нет: JSON evidence диагностический, его нельзя подавать старому replay как доверенный выбор.

Реальный consumer — неизменённый `examples/nanz/08_arena_allocator.nanz`, база `c30d333e`, фиксированные нейтральные опции `AssertMode=none, OptSize=true`. Training: исходная test_split. Heldout, заранее объявленный и не участвующий в выборе: test_alloc_sequence, test_oom, test_small_allocs, test_typed_alloc. Никакого retuning после heldout. Выбор consumer уже основан на предыдущем исследовании; это frozen independent validation workload, не претензия на статистическую репрезентативность.

| Кандидат | Полный BIN | Training T |
|---|---:|---:|
| off | 1184 B | 3808 |
| greedy | 1193 B | 3896 |
| PBQP | 1184 B | 3808 |

**Обе цели выбирают off**: он равен PBQP и стоит раньше. PBQP лучше greedy, но не выигрывает у выключенной контрактной оптимизации на этом примере. Heldout выбранного артефакта — **7165 T**, все четыре исходных ожидаемых результата 0 совпали с VM/native. В report069 сравнивались только greedy/PBQP; новый отрицательный off-control сохраняет более полный вывод.

API владеет копией workload до первого factory callback; требует свежие независимые HIR, сравнивает полные input metadata плюс семантическую compatibility и site map. Структурные HIR/Grace варианты и внешние tables/PGO/runtime/config sinks исключены из этого ограниченного candidate set. Preoptimization raw MIR должен совпадать между кандидатами. Сохранены exact options с нынешним ProfileCompilationID, input metadata, raw baseline MIR, final MIR, ASM/BIN, native per-call snapshots/T и общий hash evidence. После измерений sentinel повторно проверяет источник, postcompile HIR/MIR/type metadata, allocation, ASM и configuration ID.

Поведение кандидатов проверяет закрытый `RunZ80ProfileWorkload` против privately-created `hir.RunLoopProfile` oracle. Любой compile/native/identity/factory сбой возвращает nil результат — частичного победителя нет. Resource/globals/pointer layout остаются границами существующего verifier; числовые pointer tables, extern memory hosts, ports/output/ASM не становятся допустимыми. Self_tokenizer с peek/poke/render этим API явно отвергается: исключать печать ради выгодного числа нельзя. Для него нужен отдельный memory/output-domain proof, report070 speed tradeoff остаётся действительным.

Отрицательные controls: общий HIR объект, изменённый source, изменённый assert metadata, поздняя мутация ABI, ошибка sentinel после всех measurements, неизвестная функция workload, unsupported config и memory/output inputs. Caller меняет исходный аргумент workload в factory — результат сохраняет owned исходное значение. Positive control проверяет fixed tie order и exact selected artifact; source arena training/heldout дополнительно сверяются с независимыми source expectations.

Focused selector tests PASS 4.050s; финальный повтор вместе со старыми mode4, их поздней type mutation проверкой, arena native и ContractStrategy admission/source controls **PASS 6.120s**. Полный pipeline в этой ветке не запускали; module-wide compile gate не утверждается.

Артефакты `reports/artifacts/contract-selector/`: exact source, frozen train/heldout workloads, evidence обоих goals, native heldout captures, каждый candidate ASM/BIN и выбранные ASM/BIN, regression log и SHA256 manifest. BIN relocation ORG 8000 — ровно native ProfileWorkload configuration, без guarded runtime улучшений.

Пример вызова из Go:

```go
result, err := pipeline.MeasureContractStrategies(factory, train, "speed",
    pipeline.Options{AssertMode: "none", OptSize: true})
// result.Steps.Assembly и result.Binary — измеренный выбранный продукт.
```
