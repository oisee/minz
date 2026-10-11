# Grace PGO: scalar invariant и буферная программа

Правило hir-invariant-base теперь выносит обычное неизменное u16 сложение параметров/литералов, включая scalar a+b. Выражение сохраняет точный тип и место внутри окружающего блока, не перемещает loads/calls/division и не переассоциирует операции. Мутации, shadowing, escaping parameters, loop calls и nested loops остаются запретами.

Ревью обнаружило битовую запись a.0=1, которую прежний guard не считал мутацией. Исправлены классификация записи в invariance proof и перенос изменяемой переменной через loop block parameters в lowering. Первая регрессия до исправления возвращала 1 вместо 2; repeated-bit loop теперь получает текущее значение. Boundary tests проверяют нулевые итерации, 1/5/16/64/255, wrap, direct/bit writes, shadowing, calls и допустимые pre-loop writes.

Новая программа grace_weighted_checksum.nanz вычисляет seeded checksum буфера: каждый байт прибавляет собственное значение и weight a+b, modulo65536. Обычный caller вызывает loop-функцию и record-функцию, которая сохраняет последний результат и увеличивает counter. 30 вызовов охватывают длины 0/1/5/16/64/255, zero/ff/ramp/alternating/изменённые буферы, wrap весов/seed/counter. Независимый host oracle проверяет каждый результат и полный globals/resources snapshot. exact_data_access проверяет число каждого выполненного чтения/записи; guard-байт после буфера не читается.

| Программа/нагрузка | Исходный | Hoisted | Выбор |
|---|---:|---:|---|
| checksum: 30 direct/caller calls |1166563 T /168 B|475239 T /116 B|hoisted|
| scalar accumulate: 36 calls с wrap и globals |647844 T /77 B|171768 T /36 B|hoisted для speed/size|

Checksum выигрывает 59,3% тактов и 31,0% размера полного модуля. Это реальные entry→RET такты собранного Z80; bootstrap отдельно. Каждый кандидат сначала проходит raw VM и target oracle, затем выбирается строгое улучшение. HIR/lineage подтверждают фактическое срабатывание правила. Эксперимент подтверждает эту нагрузку; эквивалентность для всего пространства входов им не доказана.

Программа выявила ещё три backend пробела: parallel copies между парой и её половинкой не видели физическое перекрытие; byte load использовал A как scratch, теряя live index/bound; копии byte spill тоже теряли A. Mixed pair/half copies теперь разрешаются на уровне байтов; существующие безопасные load/copy helpers сохраняют scratch. Полный workload с 255 итерациями сохранён, данные для прохождения теста не сужались.

Артефакты: [артефакты и воспроизведение](grace-pgo-scalar-artifacts/README.md), исходник [grace_weighted_checksum.nanz](../examples/nanz/grace_weighted_checksum.nanz). Исторические отказ/isolated diagnostics сохраняются отдельно от текущей успешной evidence. Общая цель остаётся активной: дальнейшие формы циклов и program-level rule policy требуют своей проверки.

Nested bit assignment без адресуемой scalar-базы теперь отклоняется HIR verifier до lowering вместо panic. Reviewer не воспроизвёл других hoist-расхождений в 792 focused VM comparisons; это дополнительное тестовое свидетельство, не формальное доказательство. Новые machine-code регрессии проверяют 230 комбинаций copies/loads/spills.

После backend preservation текущий sum-array модельный fast-path стоит 212+91n, equality добавляет7, fallback231+143n; original69+141n. Full workload для длин0/1/5/16/64/255 теперь 48495→32303 T, модуль128→185 B. Старые коэффициенты остаются историей; профильные evidence заново измеряются и связываются с compiler SHA.

Итоговые проверки: полный pipeline прошёл; HIR/MIR2/Lanz и Nanz unit tests прошли. Полный Nanz/showcase содержит 31 прежнее падение, новых нет; asm_caller_callee исправлен относительно базы с 32 падениями. Профильный CLI и полный u8 cost domain прошли. [Логи проверки](grace-pgo-scalar-artifacts/validation.md).
