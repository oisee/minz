Обновление 11 октября: [живой HL при global store](grace-global-hl.md) — сохраняются HL/H/L перед прямой записью константы byte.12 source и4 machine случая прошли; полный pipeline/CLI и бюджетная сверка зелёные. Hashmap прежний123708 T /965 B; tokenizer speed2174363 T /4820 B, size2240918 T /4305 B. Исходный SelfLanz30 сохраняет baseline1221494 T /2091 B. Следующий этап — отдельный production memory-host baseline contract, затем host-aware PGO certificate.

Обновление 11 октября: [bool extension в Z80](grace-bool-extension.md) — исправлены bool→u8/u16, pending predicates и повторные использования.608 machine cases,14 source assertions, полный pipeline/CLI и бюджетная сверка прошли; новых showcase падений нет. Все16 PGO кандидатов реальных программ перепроверены, выбранные бинарники прежние. Следом — сохранение живого HL при direct global store; затем явный memory-host contract и arena-first certificate для исходного SelfLanz PGO.

Обновление 11 октября: [локальные integer bindings и SelfLanz](grace-local-bindings-and-self-lanz.md) — исправлено приведение initializer/assignment/fold к объявленной ширине и shadowing LetIn. Неизменённый рекурсивный parser прошёл 30 вызовов VM/Z80 с test-only peek/poke ABI адаптерами: 1221494 T / 2091 B, полное дерево, строки, globals и порядок byte events. Tokenizer CLI speed2174342 T / 4818 B, size2240897 T / 4303 B; все новые кандидаты перепроверены. Полный pipeline прошёл, бюджетная сверка прошла. Далее — общее расширение bool в Z80 и отдельный production host contract для parser PGO.

Обновление 11 октября: [совместный PGO ordered-byte read/write](grace-ordered-byte-rw-functions.md) — typed Kind / Enabled, до четырёх общих функций и 16 фактических вариантов. Исходный hashmap: оба helper дают 123708 T / 965 B вместо 133994 T / 984 B, выигрыш 7,68% и 19 байт. Native CLI выбирает оба в speed и size; память и обращения подтверждены независимо. Общие pipeline/budget проверки прошли, новых падений showcase нет. Следующий реальный baseline — рекурсивный self_lanz_parser.nanz с явными тестовыми peek/poke адаптерами.

Обновление 11 октября: [PGO для ordered-byte функций](grace-ordered-byte-functions.md) — отдельные source-bound FunctionSiteID, до четырёх helper / 16 измеренных вариантов и opt-in CLI. На исходном hashmap `rd16` даёт 126680 T / 980 B вместо 133994 T / 984 B; оба режима выбирают его. Порядок чтений проверен raw и actual Z80. Общий pipeline прошёл, новых падений showcase нет. Следующий измеренный прототип — аналогичное повторное использование адреса `wr16`.

Обновление 11 октября: [Z80 folded constant operands](grace-byte-constant-operands.md) — исправлено использование A для константы, загрузку которой убрал codegen. 450 машинных случаев проходят; исходные бинарники hashmap и tokenizer сохранились. Сужение slot оказалось дороже и остаётся экспериментом. Следующий кандидат — повторное использование адреса в rd16: прототип ускоряет весь hashmap на 5,46% и экономит 4 байта; контракт production PGO ещё разрабатывается.

Обновление11октября: [Z80 word byte extraction](grace-word-shift8.md) — unsignedu16 >>8 и /256 через перенос старшего байта. На исходном hashmap133994T984B вместо138440T1013B:−3,21%T/−29B. TokenizerfullCLIоба2191095T4832B,все4сочетания заново измерены. Следом — sourceproofmaskedslot induction в горячих probingциклах.

Обновление11октября: [оригинальный hashmap и PC-профиль](grace-pgo-hashmap.md) — 36последовательных операций полного исходного Nanz,138440T1013B, independent82Bglobals и все байтовые read/write counts. PC-профиль без двойного счёта: put54%, rd16 24%, get10%, wr16 9%. Следом — измерить общие byte-shift lowering и maskedslot narrowing на неизменённом исходнике. Ускорение для hashmap пока не заявлено.

Обновление11октября: [совместный copy + region PGO](grace-pgo-copy-region.md) — все4сочетания реально измерены на30вызовах полного tokenizer. Speed2203731T4861B против original2270286T4346B: −2,93%T/+515B; size выбираетoriginal. Отдельный общий source/postsplit proof сохраняет остальные функции/origins/lineage; standaloneконтракты строги. Следующий шаг — расширить корпус реальных программ с независимым oracle.

Обновление11октября: [PGO region comparison](grace-pgo-region-compare.md) — явное Grace-правило и CLI для точного сравнения байтовых областей. Полный tokenizer30calls: speed2214152T4732B против original2270286T4346B, −2,47%T/+386B; size сохраняет original. Независимые source/pre/post-split proofs и state/resource-traffic oracle. Следующий шаг — измерить совместные copy/region choices и расширить корпус реальных программ.

Обновление11октября: [seeded u16 byte sum](grace-pgo-seeded-sum.md) — source proof и guard для точной 16-битной суммы байтов с seed, полный VM/address/Z80/caller контроль. Измеренное преобразование дороже:148452T62B→241596T164B; PGO сохраняетoriginal, ускорение не заявлено. Следующий шаг — уменьшить backend стоимость fast loop и заново проверить фактическую выгоду.

Обновление11октября: [совместные pointer/scalar source-site choices](grace-pgo-joint-sites.md) — до32полных измерений на4суммарных циклах, ordinary checked_packet caller30calls, speed645640T/276B и size726480T/220B относительно original1417060T/249B. Новый combined fixture проверяет арифметику, persistent state и exact byte accesses; существующие checksum functions сохранены. Следующая очередь: содержательные buffer/program loops и расширение исходного proof с самостоятельными граничными проверками.

Обновление11октября: [constant-if вместе с per-source-loop scalar hoist](grace-pgo-site-rules.md) — до32 измеренных комбинаций, источник→HIR/MIR→assembler→Z80, полный checksum30calls с independent return/snapshot/exact byte-access oracle. Collector/refusing compatibility loader, persistent caller workloads и измеренный выбор уже реализованы; прежние статусы ниже сохранены как история. Следующий шаг — совместный bounded поиск pointer/scalar choices с проверкой непересекающихся source sites, затем расширение proof и поиск выигрыша на более широком program corpus.

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


[Byte countdown и точный zero-trip model](grace-pgo-djnz.md): ForEach/DJNZ исправлен до actual Z80, seeded caller148452→139581T; sum_array48495→13879T. Новая модель fast n0=212, n>0=210+37n, module128/183B; fallback231+143n, equality+7. Profile observations/v5 требуют fast_zero_entries и measured zero cost. Следующая очередь — новые реальные формы циклов и проверка выигрыша на корпусе; hardware остаётся дальнейшим этапом.


[Weighted composition v4](grace-pgo-weighted-composition.md): exactscalarpreheader плюс guardedpointerbyteiteration даёт weightedcaller475239→347888T; mixedcaller553520→345795T. НовыйexplicitAPI/CLI проверяетsourceproof и actualpre/postshape; genericoverlapнеразрешён. Следующиеэтапы — дополнительныеформы/эффекты реальныхциклов и снижениеcodegrowth; hardwareещёпредстоит.

[Forward byte copy](grace-pgo-forward-copy.md): отдельный source-site selector измеряет sequential copy с alias, exact state/access oracle и сохранёнными бинарниками; typed allocation caller ускорился388885→283030T, size сохраняет86B baseline. Следующий этап — дополнительные реальные callers и контроль роста кода.

[Typed interning mixed PGO](grace-pgo-mixed-intern.md): целый caller поиска/переиспользования/размещения строк и checksum,8измеренных вариантов; speed1635072→962137T, size626→539B. Следующая очередь — расширение tokenizer program coverage и контроль proof/target ABI.

[Полный typed tokenizer](grace-pgo-tokenizer.md): весь scanner/intern/token-record caller проходит независимый state/resource-traffic oracle и assembled Z80;30 persistent calls. Copy PGO 2270286→2259865T (−0,459%), 4346→4475B; size оставляет original. Исправлена lexical interface зависимость HIR splitter. Следующий этап — standalone Block shadowing в HIR lowerer, затем доминирующие scanner/search loops; original extern ABI и hardware ещё впереди.

[Lexical HIR lowering](grace-hir-scope.md): восстановление scalar/type/alias/array bindings, break/continue edge values и range/ForEach binders проверяетсяVM/Z80. ExactPCaudit tokenizer: intern23,12%,scan20,19%,peek_char13,26%,region_eq9,87%. Следующая очередь — backwardgoto lexicaledges, затем отдельный проверенный Pascal search PGO; hardwareвпереди.

[Word constant TSMC repair](grace-tsmc-const.md): backend сохраняет живые A/flags при16-bit immediate patching. Regioncompare prototype raw6144/full30hostZ80 проходитпослеfix, но всеизмеренныеформы медленнеебазовогоtokenizer; productionruleнеподключён. Следующиезадачи — goto lexicalSSA и снижение guard/setupcost поactualPC;hardwareвпереди.

[Goto lexical SSA и C scope](grace-hir-goto.md): переходы сохраняют внешние значения и области, пропущенная используемая инициализация отклоняется;21VM/Z80вариант и10настоящихCвызовов с independentstate/traffic. MutableCfunctionpointers отделены от immutablealiases,6MIRassertions; actualZ80indirectABI покаограничен. Следующая очередь — снижениеguard/setupcost доминирующихtokenizerloops и измерениецелогоcaller; hardwareвпереди.


[Закрытый memory-host baseline](grace-memory-host-baseline.md): неизменённый Self-Lanz выполняет30 persistent calls на verified input MIR VM и actual Z80, с проверкой памяти и порядка ресурсных byte events.1221494 T /2091 B. Host inputs замкнуты на два встроенных byte-действия; глобальные/stack/spill обращения проверяет внутренний instruction sidecar. Полный pipeline289.268 s PASS, showcase31 прежнее падение,0 новых. Это основание для следующего host-aware arena-first string comparison сертификата и source-bound профиля; оптимизация parser пока не заявлена.


[Исходные циклы и native host-baseline](grace-memory-host-loop-baseline.md): отдельный закрытый collector измеряет11 source sites SelfLanz30; cumulative/per-call counters совпадают с независимой моделью, region_match49/59. Native baseline1221494 T /2091 B unchanged; fullpipeline296.585 s PASS, showcase31same0new. Счётчики privateVM не являются legality или targetlooplineage. Далее source-bound first-byte arena comparison proof/domain и измеренный выбор вариантов.
