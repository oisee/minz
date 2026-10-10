# Первое профильное решение Grace: настоящий sum_array на Z80

10 октября 2026. Input: неизменённый `examples/nanz/01_sum_array.nanz`. Все новые API opt-in; production default не меняется.

## Исполненная цепочка

Nanz source manifest → controlled workload → unoptimised MIR2 VM observations → strict compatibility loader → target cost decision → Grace legality predicates/runtime address guard → lowered MIR2 → production PBQP/Z80 → assembled binary → Z80 emulator.

Workload: array bytes 1..255, calls с длинами 0/1/5/16/64/255. Профиль: 6 entries / 341 body entries, all-fast address eligibility. Guard observations пока считаются для успешных прямых calls точного sum-idiom: исходник не исполняет guard, collector оценивает тот же unsigned predicate `ptr <= 65535-n`. Называть это выполненными branch counters нельзя. Internal/recursive calls не получают такого счётчика; loader проверяет суммы coverage, policy сохраняет original при неполном покрытии или guard misses. Failed snapshots отклоняются.

## Решение и проверенные цены

Модель привязана к exact typed-HIR SHA256, target ID и function name. EvidenceID указывает этот набор assembled measurements. Для all-fast пути: original = 69 + 141*n T-states; guarded pointer = 189 + 89*n. Каждая точка ниже реально исполнена, линейная модель сверена на этих шести длинах. Экстраполяция на весь u8 domain этим отчётом не доказана. Стоимость includes function entry→RET, excludes bootstrap/CALL/DI/HALT. Размер — весь модуль, включая main и storage, не только функция.

| n | original T-states | pointer T-states | result |
|---|---:|---:|---:|
| 0 | 69 | 189 | 0 |
| 1 | 210 | 278 | 1 |
| 5 | 774 | 634 | 15 |
| 16 | 2325 | 1613 | 136 |
| 64 | 9093 | 5885 | 32 |
| 255 | 36024 | 22884 | 128 |

Полная workload: **48495 → 31483 T-states (−35.1%)**, module size **128 → 177 bytes (+49)**. Speed policy выбирает pointer-walk. На короткой workload 0/1 original=279, pointer=467: выбирается original. Size policy оставляет original даже на полной workload. invariant-base не срабатывает на данном исходнике; size-Grace совпадает с original.

Каждый original/pointer/selected вариант проверен raw MIR2 VM, assembled Z80 execution и независимой формулой `n*(n+1)/2 mod 256`; input memory проверена после исполнения. Z80 ABI bootstrap берёт реальные allocation/contracts. Исходники, workload/profile/decisions/measurements, HIR/MIR, ASM и binaries сохранены в [artifacts](grace-pgo-decision-artifacts/decision.json).

## Attribution

`SeedLoopLineage` привязывает input loops к site IDs. Pointer rewrite сохраняет fast/fallback роли с исходным ID; invariant-base переносит lineage при замене while. `CurrentLoopLineage` показывает только reachable current nodes. Lowering переносит origins на raw MIR body labels; range synthetic while наследует identity в частной lowerer map. MIR optimisation/splitting/fusion не имеют общего lineage контракта: raw labels могут исчезнуть, это не final instruction source map.

## Границы

Cost model supplied externally и не является proof; профиль не подтверждает истинность произвольных пользовательских counts. Loader отвергает unknown/ineligible guard sites, inconsistent coverage и missing fields. Политика uses big integers для totals, не допускает overflow. Адресный guard остаётся в emitted code. Legality проверяется обычными predicates независимо от profitability. Caller-selected models пока не зарегистрированы автоматическим target runner.

Следующая работа для общей цели: полный u8/fallback target cost domain, автоматическая регистрация measured model, lineage через MIR passes, расширение на корпус реальных функций, затем интеграция selection в CLI/pipeline. Ночной sum_array срез исполнен; общая цель оптимизации реальных программ остаётся активной.
