# Первое профильное решение Grace: настоящий sum_array на Z80

10 октября 2026. Input: неизменённый `examples/nanz/01_sum_array.nanz`. Все новые API opt-in; production default не меняется.

## Исполненная цепочка

Nanz source manifest → controlled workload → unoptimised MIR2 VM observations → strict compatibility loader → target cost decision → Grace legality predicates/runtime address guard → lowered MIR2 → production PBQP/Z80 → assembled binary → Z80 emulator.

Workload: array bytes 1..255, calls с длинами 0/1/5/16/64/255. Профиль: 6 entries / 341 body entries, all-fast address eligibility. Entry hooks точного sum-idiom записывают eligibility `ptr <= 65535-n`, в том числе при внутренних вызовах. Исходник не исполняет transformed guard; это не branch counters. Loader проверяет полноту coverage; policy сохраняет original при неполном покрытии или отсутствии измеренной fallback стоимости. Failed snapshots отклоняются.

## Решение и проверенные цены

Модель привязана к exact typed-HIR SHA256, target ID и function name. EvidenceID указывает этот набор assembled measurements. Для all-fast пути: original = 69 + 141*n T-states; guarded pointer = 189 + 89*n. Каждая точка ниже реально исполнена. Модель дополнительно сверена на 1022 сценариях полного u8 domain, включая equality, fallback и wrapping ranges (детали ниже). Стоимость includes function entry→RET, excludes bootstrap/CALL/DI/HALT. Размер — весь модуль, включая main и storage, не только функция.

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

## Независимое ревью и расширенный домен

После двух CLI reviews проверен полный u8 domain: **1022 сценария**, original и pointer в MIR2 VM и assembled Z80. 256 обычных fast bases, 256 exact guard boundaries, 255 first-fallback bases и 255 wrapping ranges. Проверяются input memory, отсутствие overlap с code/stack и фактически посещённые fast/fallback branch labels.

Выявлено уточнение: exact equality `ptr == 65535-n` добавляет **7 T-states** к fast setup (вторая условная ветка). Fallback = **208 + 141*n**, +139 к original. Schema профиля теперь v4: entry hook записывает actual loop input values, включая внутренние ordinary calls; equal entries отдельно. Это наблюдение eligibility исходного цикла, а branch labels проверяются независимо на Z80. Mixed fast/fallback policy использует обе измеренные цены.

Решения original теперь реально применяются/исполняются, включая n=2/3 вокруг crossover. Sticky module policy предотвращает отмену решения повторным pipeline Grace. Saved decisions несут полный compatibility key и сверяются снова при применении. Models связаны с executable/semantics; pipeline input дополнительно проверяет compilation fingerprint всех options + assembler mode, пересчитывает workload context и сохраняет решения в Steps. Обе pipeline entrypoints принимают ProfileSelection; CLI collection/application и automatic measured runner реализованы: [пошаговый запуск](grace-pgo-cli.md). Поддерживаемый assembler mode пока default; speed mode явно отвергается.

Независимое ревью также воспроизвело существующий foreach-u16 wrong code: 256→0 и 300→44. Подагент исправил width counter/params/constants/decrement, проверил VM/Z80 для 0/1/255/256/300, continue и break после 270. U8 sum cost остаётся прежней. Callback-inline lineage исправлен в `ebb530c7`: две встроенные копии callback сохраняют его SiteID в raw MIR, входная metadata map не изменяется. Это attribution исходного site; общего контракта provenance через все MIR passes ещё нет.

## Автоматическое измерение и CLI

`MeasureLoopCostModel` выводит коэффициенты из actual VM/Z80 execution, проверяет 1022 сценария и хеширует evidence. `--hir-profile-auto` делает collection → measure → selection → compilation одной командой. Поддержка пока точного одиночного u8 sum site; вся общая цель остаётся активной.
