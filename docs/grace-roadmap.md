Обновление: [совместные варианты и caller workload](grace-pgo-joint.md) — до16 наборов, actual VM/Z80 results/global/resource checks, −11.82% на двухфункциональном caller. Shared RAM и codegen constant-alias исправление подтверждены tests.

Обновление: [воспроизводимый CLI PGO](grace-pgo-cli.md) (`92b581e3`) — automatic measured model, 1022 VM/Z80 scenarios, collection/selection и saved evidence. Exact одиночный u8 sum slice завершён; general corpus/multi-function/MIR lineage впереди. Старые обновления ниже — история этапов.

# План развития Grace в основном пути MinZ

Обновление: [первый слой PGO sites](grace-pgo-sites.md) — исходные structured loops Nanz с координатами и SHA256; карта до Grace/splitting. Collector, lineage и loader ещё не реализованы.

Обновление: [iterator stages](grace-iterator-stages.md) — range/filter/map/forEach, корректные DJNZ-рёбра и сохранение живого A; 48 сквозных вариантов VM/Z80.


Обновление: [captures и callable contracts](grace-capture-contracts.md) — явные лексические зависимости и проверяемые сигнатуры известных calls/aliases. PGO всё ещё не реализован; сохранение всех внешних артефактов локально включено.


Обновление: [fold correctness](grace-fold-correctness.md) — сохранены эффекты callback и стадии map/filter; исправлена Z80-запись с живым A. Countdown range соответствует книге языка, прежняя оценка его суммы как бага исправлена.


Обновление: добавлен [HIR-верификатор](grace-hir-verifier.md) и исправлены типы fold/reduce непосредственно в return. PGO остаётся следующим отдельным этапом.

**Update:** [Scalar pointer types from source through VM/Z80](grace-typed-pointers.md) are now implemented and checked. The earlier status below is retained as a snapshot.

Текущий результат: [закрытие пробелов](grace-gap-closure.md), корпус **52/52**. Арифметика указателей VM и граничные доступы Z80 согласованы для проверенных случаев; усилен MIR-верификатор. Следующие задачи — элементные типы указателей во фронтенде, проверка HIR и повторный замер pointer-walk перед снятием guard. LLM-less PGO описан в [ADR-0046](adr/0046-optimisation-policy-and-certificates.md), сборщик профиля пока не реализован.

Дата: 10 октября 2026. Ветка: `feat/grace-hir-vm`.

Цель: общий движок правил для HIR и MIR, с отдельными адаптерами и фактами.
Каждое преобразование должно давать дополнительную пользу относительно
обычных проходов и сохранять наблюдаемое поведение на MIR2 VM.

## Порядок работы

После ревью приоритет изменён: **контракт адресов → исправление Z80 ForEach
→ MIR constant branches → Z80-замеры → профиль аллокаций VM**.
Инвентарь LIR и независимый корпус регрессий ведутся с самого начала.
[Задачи, блокеры и проверенные уточнения](grace-review-follow-up.md).
Расширение pointer-walk и работа с captured bindings ждут первых двух задач.
Ниже сохранены направления долгосрочного плана, а не текущая очередь исполнения.

1. **Диапазоны и безопасный pointer-walk — текущий этап.**
   Начать с реального `examples/nanz/01_sum_array.nanz`: вывести диапазон
   счётчика и условие, при котором вычисление адреса не переполняется.
   Для неизвестной базы использовать проверяемую во время исполнения границу:
   быстрый последовательный обход и исходный индексный цикл как fallback.
   Сохранить нулевые итерации, сумму, итоговый индекс, память и ошибки.
   Использовать существующий HIR ForEach/lowering вместо второго механизма
   построения pointer-loop. Измерить длины 0, 1, 5, 16, 64, 255.
2. **Вызовы и захваченные переменные.**
   Разделить обычный вызов и встраиваемый iterator-callback; получить факты
   изменения захваченных bindings. Расширить invariant-base на `print_region`,
   сохраняя порядок I/O. Не трактовать IsIO=false как доказательство чистоты.
3. **Более широкие структурные правила HIR.**
   Использовать доказанные диапазоны/неизменность для выноса вычислений,
   упрощения условий и специализации постоянных аргументов. Выбирать правила
   по корпусу и профилям, учитывать стоимость guards и рост кода.
4. **Миграция полезных LIR-правил в основной MIR-путь.**
   Следовать `lir-retirement-backlog.md`: identities, load/store-комбинации,
   затем целевые шаблоны и ограничения распределения регистров. Для каждого
   сравнивать с уже действующими проходами. LIR остаётся deprecated reference.
5. **Стоимость анализа и инфраструктура измерений.**
   Сначала профиль стоимости; затем пересчёт затронутых функций и зависимых
   summaries вместо повторного анализа всего модуля. Сохранять ограничения
   бюджета и явное обновление фактов после изменений. Собирать корпус реальных
   функций и отслеживать отрицательные результаты вместе с положительными.
6. **Декларативные факты и самооптимизация.**
   Отделить монотонное вычисление фактов от ограниченных переписываний.
   После стабилизации semantics/контрактов: компиляция правил, Grace над Grace,
   проверка бутстрапа. Это дальнейшие задачи, сейчас не реализовано.

## Условия завершения текущего этапа

- План, диапазонные факты и защитное условие доступны в коде/документации.
- Правило реально срабатывает на `sum_array` без изменения исходника.
- Есть fallback для непокрытых адресов, без изменения семантики VM.
- VM differential tests сравнивают возврат, память и причины ошибок; отдельный
  oracle проверяет сумму и итоговый индекс. Проверяются переполнение адреса,
  малая память, все u8-длины и оба режима обычной MIR / MIR Grace.
- Есть замеры инструкций и времени, исходники и дампы HIR/MIR. Показаны также
  цена guard, случаи без выигрыша и стоимость компиляции/размер IR.
- Новые проверки проходят; старые падения отделены от новых регрессий.


## Результат первого этапа

Реализован `hir-guarded-pointer-walk`, факты index-range и address guard.
На 255 байтах: 1788→1285 VM-инструкций, время вызова −20,0%; на 0–1 байте
есть регрессия из-за setup/guard. Правило остаётся opt-in. Улучшен общий
ForEach lowering и исправлена коллизия labels в обоих split-join-ret путях.
Новые VM-проверки и source-level MIR/Z80 assertions проходят; старое native
ForEach ограничение воспроизведено отдельно на чистой базе.
Подробности: [сквозной отчёт](grace-pointer-walk-e2e.md).
Следующий этап после ревью — контракт адресов и исправление Z80 ForEach;
эффекты обычных вызовов против captured bindings callbacks отложены до снятия
этих блокеров. Реализованный этап не означает готовности включать правило
по умолчанию или применять его к реальным Z80-программам.


## Аудит и первые целевые замеры

[Результат аудита HIR/MIR](grace-hir-mir-audit.md): исправлены ошибки loop control,
constant propagation, DSE traps, pointer CSE, bounds checks VM и Z80 scratch HL.
Автоматический MIR pointer-threading отключён до доказательства границ.
Константные ветвления теперь сворачивает обычный MIR. Получены проверенные
Z80 T-states/bytes для sum_array и region_match. Контракт указателей остаётся
предложением ADR-0045; его миграция и удаление HIR guard ещё не выполнены.

## Цели оптимизации и сертификаты

[ADR-0046](adr/0046-optimisation-policy-and-certificates.md): legality отдельно
от profitability; профиль выбирает ставку, guard/proof разрешает преобразование.
LLM предлагает проверяемое доказательство или guard, но не выдаёт разрешение
самим фактом своего участия. Source maps обеспечивают идентичность и lineage,
а не доказательство эквивалентности. Сбор/загрузчик сертификатов ещё не реализован.
Первый шаг политики уже работает: `--hir-grace --Osize` исключает versioned
pointer-walk, оставляя invariant-base. На sum_array выбирает 38 вместо 85 байт.

## Input-HIR profiling foundation (10 октября)

[Карта исходных мест](grace-pgo-sites.md) и [отдельный VM collector](grace-pgo-collector.md) сохраняют source identity, entries/body entries и признак failed calls. Production pipeline не принимает эти наблюдения как профиль оптимизаций. Следом: compatibility fingerprint/refusing loader, затем lineage и cost policy; legality остаётся отдельной проверкой.

[Ключ совместимости и refusing loader](grace-pgo-compatibility.md) теперь сверяют typed HIR, origins и объявленный harness context; malformed/partial/stale наблюдения отклоняются. Следующий срез — автоматическая identity/dependency manifest и lineage перед cost policy.

[Автоматический context builder](grace-pgo-context.md) использует executable SHA256 и Nanz source manifest, включая loop-free imports. Следующая очередь: controlled execution harness и lineage/guard events до cost policy.

[Контролируемый VM harness](grace-pgo-harness.md) выполняет ordered calls, initial globals/resources и limits из хешируемого workload. Следующая очередь: site lineage и guard events, затем cost policy.

[Первое профильное решение на реальном sum_array](grace-pgo-decision.md): full workload 48495→31483 T-states, +49 module bytes; short/size выбирают original. VM + assembled Z80 + oracle проходят для 0/1/5/16/64/255. Общая PGO инфраструктура opt-in; следующие этапы — fallback/full-domain model и CLI/pipeline registration, corpus и MIR lineage.

## Скалярные циклы и проверенный checksum (11 октября)

[Измеренный scalar этап](grace-pgo-scalar.md): invariant-base выносит точное неизменное u16 сложение; полный checksum workload с обычным caller, persistent globals и буфером проходит независимые VM/Z80 oracles и exact access counts. 30 вызовов: 1166563→475239 T, 168→116 B. Текущие sum-array коэффициенты после исправлений backend: original69+141n, fast212+91n, equality+7, fallback231+143n; прежние числа выше относятся к истории. Общая цель — проверяемая оптимизация реальных программ до бинарного Z80; следующие формы циклов и составная rule policy ещё требуют реализации и измерения.
