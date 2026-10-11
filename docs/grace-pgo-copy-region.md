# Grace PGO: измеренная композиция copy и region compare

Полный typed tokenizer с неизменёнными 30 последовательными входами теперь
проверен для всех четырёх комбинаций двух оптимизаций. Выбор основан на запуске
каждого собранного Z80-бинарника на всём workload; стоимость комбинации не
выводится из суммы отдельных выигрышей.

| Вариант | T-states всего workload | Байты всего модуля |
|---|---:|---:|
| Исходный | 2 270 286 | 4346 |
| Region compare | 2 214 152 | 4732 |
| Forward copy | 2 259 865 | 4475 |
| Оба | 2 203 731 | 4861 |

Speed выбирает оба преобразования: экономия 66 555 T-states относительно
исходного исполнения, около 2.93%, при росте на 515 байт. Size выбирает исходный
вариант. Бинарник выбранного speed-варианта имеет SHA256
`51abe0f5a37e37f5bd19f9c63b964f4f97cec1db8695c6a899941e9c7ad233d6`.
Это измерение данного workload и текущего backend, а не универсальная модель
стоимости или гарантия выигрыша на другом входе.

Отдельный API `MeasureMixedCopyRegionSiteVariantSets` принимает свежую фабрику
HIR и совместимые наблюдения, повторяет исходный controlled workload и сравнивает
не более четырёх наблюдавшихся source sites: максимум 16 комбинаций. Copy и
region используют разные типизированные карты запросов. Неисполненные legal
sites остаются исходными и получают disabled proofs. Неизвестные ключи, включая
false и ключ другого вида, несовместимый source/workload, повторно использованные
AST и прежние sticky policies отклоняются. Все остальные HIR Grace rules
выключены.

Типизированное evidence использует отдельную схему
`minz.mixed-copy-region-site-variant-sets/v1`. Каждая комбинация сохраняет обе
карты запросов, исходные и postcompile proofs, raw и postcompile raw execution,
actual Z80 execution, полный HIR, собранный бинарник, SHA256, фактические bytes и
T-states. Строгое улучшение определяет победителя; исходный вариант выигрывает
при равной стоимости.

Новый aggregate validator независимо проверяет обе формы kernels, точные policy
карты, неизменённые остальные функции и metadata, source origins и lineage всех
достижимых loops. Standalone copy/region contracts сохраняют свои границы. После
обычного HIR splitting validator независимо восстанавливает логическое тело из
терминального вызова и точного suffix helper: проверяет lexical inputs, порядок и
типы arguments, helper metadata, возврат результата и отсутствие лишних helpers.
Он не повторяет splitter, чтобы построить ожидаемое преобразование.

Независимый host lexer проверил все global values, полные resource snapshots и
каждое побайтовое чтение/запись ресурсов на всех 30 вызовах каждого из восьми
измерений: четыре кандидата для speed и четыре для size. Raw VM, postcompile raw
VM и actual Z80 совпали; global access counts проверены по VM/Z80 agreement.
Сохранённые Python и Go verifier повторно проверили артефакты, типизированные
SHA256 evidence и бинарников. ASM сохранён gzip без изменения исходных байтов.

Сквозные focused tests прошли за 20.204 с: четыре композиции, 16-кандидатная граница,
ранний отказ для пяти наблюдавшихся sites, неисполненный disabled site, свежесть
фабрики/observations и неправильные kind keys. Семь атак на compiled evidence
отклонены: изменение arguments, suffix, helper metadata, lineage rule, source
origin, requested policy и постороннего тела `intern`.

Финальные проверки прошли: pipeline271.889с, HIR19.876с/MIR4.153с/Nanz1.790с/C895.472с, CLI23.838с; showcase31прежнее падение, новых нет. Pipeline исключает только отдельно проверяемый TestIndependentGraceRulesBudgetCompetition. Отдельный финальный compile/assembler-аудит подтвердил точное равенство прежнего исходника, всех4ASM и бинарников:344/344/232/344байта. Прежний722.080-секундный machine gate сохранён для этих неизменных бинарников; нового исполнения долгого workload не заявляем.

[Артефакты](grace-pgo-copy-region-artifacts/README.md) ·
[Независимый host audit](grace-pgo-copy-region-artifacts/independent-host-verification.json) ·
[Типизированный SHA audit](grace-pgo-copy-region-artifacts/typed-evidence-verification.json)

Final built CLI confirms all four rows above for both objectives on the unchanged 30-call tokenizer. Saved-artifact host audit passes all results, full global/resource values and all resource reads/writes; global traffic is VM/target agreement only. Both typed evidence SHA hashes are independently recomputed. CLI speed selects both: binary SHA256 `51abe0f5a37e37f5bd19f9c63b964f4f97cec1db8695c6a899941e9c7ad233d6`; size selects original `048b95267b15d26a4a71d0a7051a16dd12f5f66c36e9ce1c922b45d109babc33`.
