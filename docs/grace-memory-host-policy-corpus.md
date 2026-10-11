# Четыре workload с явной compare policy

Те же четыре неизменённых Self-Lanz workload измерены через текущий CLI393af1
с фиксированной `preserve-speed` policy. Восемь actual Speed/Size запусков
прошли; оба независимых аудита подтвердили сохранение поведения. Все четыре
набора выбрали **append-only, mask2** для обеих целей.

| Workload | Calls | Legacy append T | Preserve-speed append T | Разница T |
| --- | ---: | ---: | ---: | ---: |
| Short reuse | 12 | 327008 | 326090 | −918 |
| Long atoms | 9 | 3968942 | 3968554 | −388 |
| Nested mixed | 9 | 1281762 | 1270168 | −11594 |
| Fresh dominant | 20 | 699845 | 696909 | −2936 |

Новый выбранный размер1979 B против сохранённого Legacy1978 B. Победитель
не изменился. Для каждого workload все четыре same-mask сравнения дали ту
же измеренную разницу T и +1 B; универсальный коэффициент выигрыша не заявляем.
Исторические Legacy результаты взяты из отдельной сохранённой сборки dba34,
а новые профили собраны и fresh replay проверены в executable393af1. Старые
контексты не переподписаны и не служили входом нового selector.

## Проверка

32 candidate v2 envelopes и восемь исходных association baselines: 500 native
наблюдений. Два fresh full30 integration запуска production CLI добавили
восемь кандидатов и два baseline,300 наблюдений. Всего800 observations
прошли независимый original-source model: результаты,10 globals,полные
persistent resource snapshots, ordered host events и11исходных site counts.
Aggregate proofs, origins/function maps, policy/context, native SHA и SHA
точных сохранённых association/selection bytes также проверены.

Результаты свежего full30 production CLI совпали с ранее проверенной
изолированной сборкой795f по ASM/BIN/состоянию/T/steps; context393af1 сохранён
отдельно. Speed выбрал both1076715T2100B; Size append1076874T1979B.

Независимый native review нашёл одну изменённую unsigned HL/BC compare
последовательность в intern_region на каждый машинный image. Это static
emission evidence; число dynamic executions и распределение ветвей из
разницы T не выводятся. Новый PC ledger и target-loop-lineage не заявляются.
Все машинные BIN отличаются от Legacy; одинакового output случая в этом
корпусе не встретилось.

[Пакет](grace-memory-host-policy-corpus-artifacts/README.md) ·
[SHA manifest](grace-memory-host-policy-corpus-artifacts/manifest.json) ·
[Независимый аудит](grace-memory-host-policy-corpus-artifacts/independent-oracle/FINAL-AUDIT.md).
684 payloads сохранены и tracked: точный executable, 32 кандидатных и восемь
выбранных BIN, whole profiles/proofs/decisions, ASM/MIR/allocation, logs и два
аудита. Relocated packaged audit повторно PASS800 без CPU. Это artifact-only
этап; production compiler/runtime source не менялись. Полный pipeline1241.045s
и CLI69.418s относятся к предшествующему переносу, новый общий прогон здесь
не нужен. Следующий этап — явный measured выбор среди двух backend policies
и четырёх mask с отдельным whole contract; существующий fixed-policy API
остаётся прежним. Hardware ещё впереди.
