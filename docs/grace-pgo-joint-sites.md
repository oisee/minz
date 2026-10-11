# Grace PGO: совместный выбор pointer-walk и scalar hoist

`MeasureJointSourceSiteVariantSets` измеряет constant-if off/on вместе с независимыми pointer-walk и скалярными hoist по source sites. Общий cap — четыре исходных цикла, максимум32кандидата. Pointer opportunities берутся из replay-verified guard observations; scalar legality — из исходного proof u16 additions. Пересечение SiteID отклоняется до подготовки кандидатов: композиция двух преобразований одного цикла пока не доказана. Текущие распознаватели структурно различны (точная u8-сумма против u16 addition).

Все политики устанавливаются до attribution seeding и первого Grace. Проверяется точное применение каждого scalar запроса, отсутствие оставшихся eligible additions, а для pointer — ровно по одной fast/fallback lineage. Незатребованные преобразования запрещены. Проверки повторяются после компиляции вместе с raw MIR поведением. Свежие фабрики, полная source/HIR/workload compatibility и refusing replay профиля сохранены; никакие observations не разрешают обход proof или runtime address guard. Неполностью применённые запросы вызывают отказ всей операции.

Evidence schema `minz.joint-source-site-variant-sets/v3` содержит отдельные eligible invariant/pointer IDs, explicit site/rule policies, decisions, до/после HIR/lineage/raw runs, actual binary SHA и измерения всех вызовов. Сначала constant-if off, затем on; scalar bits предшествуют pointer bits в каждой группе. Strict minimum T или module bytes выбирает exact retained Steps/Binary. Равенство сохраняет ранний вариант. Старые API/schema сохраняются.

В combined fixture функции weighted checksum сохранены; новый обычный caller вычисляет также byte sum и записывает сумму результатов. Это новый исходник для совместной проверки, а не заявление об ускорении прежнего неизменного benchmark. На30ordered calls с длинами0/1/5/16/64/255 и wraparound проверены все8комбинаций, независимые арифметические результаты и память после каждого вызова. Exact access oracle требует два чтения каждого используемого входного байта, одно чтение/запись persistent counter и запись обоих байтов результата; лишних чтений на zero trip нет.

| Выбор | Такты caller workload | Размер модуля |
|---|---:|---:|
| original |1417060|249 B|
| pointer-only |1336220|305 B|
| scalar-only |726480|220 B|
| pointer + scalar |645640|276 B|

Speed выбирает оба:−54.44%тактов относительно original, ещё−11.13%относительно scalar-only. Size выбирает scalar-only220B. Constant-if здесь даёт равные затраты и проигрывает раннему кандидату. Затраты измерены на полном caller, не сложены из изолированных моделей. Retained winner повторно исполняется.

Boundary тест смешивает2pointer+2scalar sites:32кандидата/33fresh modules. Пять суммарных sites, shared HIR и forged counts отклоняются. На zero-trip нельзя предполагать baseline fastest; тест сверяет реальный минимум и порядок равенств.

CLI: `--hir-profile-joint --hir-profile-invariant-sites --hir-profile-joint-sites --hir-profile-auto DIR --hir-profile-auto-workload FILE`. Новый флаг требует invariant-sites; `--hir-profile-site-rules` и module-wide independent-rules несовместимы с этим режимом.

[Артефакты и воспроизведение](grace-pgo-joint-site-artifacts/README.md). Directed gate5.328s и профильныйCLI17.615s прошли. Общий pipeline прошёл (кроме отдельно проверенного budget regression); изменения backend для этого этапа не потребовались. Следующий шаг — расширять проверенные program workloads и proof для более содержательных циклов, измеряя выгоду на полном исполняемом исходнике.
