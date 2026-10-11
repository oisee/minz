# Композиция малых исторических форм: реальные кандидаты и ограничения

Аудит всех 72 `examples/nanz/*.nanz` на root b6058e46: 981 скомпилированная функция, 32 подходят текущему `PrepareLocalTableProblem`. В каталоге75 файлов суммарно, из них72 Nanz источника; function rows представлены всеми72 файлами; exact snapshots всех72 сохранены. Только две подготовленные функции имеют >5 переменных: weighted13 и StrRef hash3 9. Weighted уже покрыта исторической2v quotient без изменения операций. Остальных реальных кандидатов, где только размер задачи мешает полезной композиции, этот корпус не дал.

Для hash3 реальный ABI A/C/D (VIR location ids0/2/3), а historical byte-singletons только A/C. Само разбиение не устраняет точный D. Старый bounded D↔C rename сохраняет один непредставимый singleton и отказывается. Test-only probe явно снимает singleton D **только с недоверенного proposal query**, получает готовое3v quotient назначение и затем проверяет полные исходные Ops/точный A/C/D ABI через `AddChecked`. Оно допущено; direct production importer не менялся.

**Результат равный:** оригинальный hash3 baseline/candidate6B30T, полныйmodule45B. Native+VM 65 536 тройных byte samples на каждый путь (`a=low(x),b=high(x),c=low(53*x+17)`), независимое `a+2*b+3*c mod256`, SP/return PC/image сохранены. Это не все2^24 тройки и не выигрыш. Probe не является доказательством exact historical shape hit или полезности composition.

## Реальный граф и безопасный план

Для hash3 vregs14..22. Точные edges:14–15,14–16,15–16,15–17,16–17,16–18,16–19,16–20. Vertex16 (`c`) — separator: после удаления core14/15/17, одиночные18/19/20 и disconnected final21/return22. Малые компоненты структурно есть; сохраняется операция, соединяющая операнды из разных components, например `h2=h1+c`. Interference graph edge decomposition не равен dependency DAG partition.

`tryCutVertexDecompose` (`pkg/vir/pipeline.go:1294`) фильтрует subOps только если все refs внутриcomponent, вызывает LookupChecked с пустымиopts и проверяет лишь shared-vreg consistency. Cross-component operations могут исчезать, полное покрытие и ABI не доказаны. `codegenFuncIslands` около1752–1779 вычисляетboundary, но оставляет `_ = boundary`, не выполняя нужныхcopies. Их нельзя переносить в production как готовое доказательство.

Безопасная будущая композиция — **единое предложение назначения** для всей функции:

1. Полный исходный список Ops, current ABI/hints и exact simultaneous-live graph остаются неизменны.
2. Separator locations перечисляются в конечном допустимом physical domain; связанные halves/pairs учитываются через реальные aliases.
3. Каждая component subproblem получает separator как fixed boundary input и все cross-boundary constraints. Не требуется удалять Ops или отдельно emit islands: локальные решения — только assignments.
4. Результаты объединяются только при одинаковых shared locations, полном покрытии каждого исходного vreg и сохранении всех interference/alias constraints.
5. Полный `AddChecked(originalOps, wholeAssignment, originalOptions)` проверяет реальную ISA/scratch/flags/moves/cost; fullBIN selector/nativeoracle решают полезность.

Числа3+4=7 или4+2=6 верны только при непересекающихся sets; при shared separator размерunion=3+4−|separator|. Раздельные оптимальные components не гарантируют глобальный оптимум из-за cost/constraints boundary. Precomputed assignments могут служить candidate bank; молчаливое ослаблениеABI или сложение claimed costs неприемлемо.

## Почему implementation не добавлена

Единственный remaining large realleaf hash3 уже имеет6B30T baseline; импортированное назначение равно ему. Доказанный measured gain от новой composition отсутствует. Самый маленький следующий полезный шаг — поиск реального arithmetic leaf после осторожного расширения поддержанных native opcodes либо отдельного доказанного block boundary domain. Исследовательскую composition нельзя объявлять production ради количества случаев.

`composition-inventory.json` содержит prepared/rejected причины981functions и exact large problems. `hash3-graph.json` — independent live graph. Все72sources, historical proposal/currentcertificate, fullmoduleASM, native log, sourceSHA manifest сохранены в `reports/artifacts/historical-composition-evidence.tar.xz`. Тесты opt-in `COMPOSITION_EXPORT`; generation новогоsolver не используется. FullVIR отдельная проверка; fullpipelinePASS не заявляется.
