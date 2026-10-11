# Измеримый selector для arena-peel — подготовлено к интеграции

В отдельной ветке реализован ограниченный selector для одного точного helper
`region_match` оригинального SelfLanz. Он принимает исходную фабрику, закрытый
memory-host workload и сериализованный source-loop профиль v1. Проверяет строгую
JSON-форму, свежий source/workload/context, затем повторяет наблюдение профиля.
Согласованные поддельные счётчики не становятся доказательством: selector требует
совпадения с реально повторённым закрытым запуском.

В объединённой изолированной ветке с проверкой call-frame полный workload из
30 вызовов прошёл на исходной и peeled версиях: original 1221494 T / 2091 B,
peeled 1218395 T / 2212 B. Speed выбирает peeled (−3099 T, примерно −0.254%),
size выбирает original. Goal выбирает по фактическим расходам; настройки
компилятора остаются настройками baseline. При равной основной стоимости
сохраняется original. Эти результаты — scoped измерение данного workload,
не обещание универсального ускорения.

Peel сначала проверяется независимым HIR-validator: точная исходная функция,
порядок двух byte-read, zero/one-length возвраты, i=1 для хвоста, неизменность
остального модуля и перенос истинных source-origin на хвост. Typed proof сохраняет
исходные и преобразованные compatibility, полные FunctionSiteMap и точный SiteID.
После измерения proof связывается с идентичностями обоих реально исполненных
baseline. Это proof до compiler preparation. Splitting и final MIR не получают
унаследованный post-HIR сертификат; final MIR/allocations/native sidecar остаются
диагностикой и контролируемым evidence исполнения.

Фабрика возвращает только оригинальные исходники. Selector сам создаёт вариант,
удерживает все прежние AST-узлы через GC и проверяет сохранённые источники после
поздних callback. Исходник, workload, profile и proof не могут быть заменены
пользовательской assembly, callback или произвольной transformed factory.

Изолированные focused проверки загрузчика/GC/cap прошли; независимый HIR proof
прошёл (0.454s) и construction/proof binding прошёл (0.178s). Полные production
HIR/MIR/pipeline/CLI/showcase gates и свежий четырёхвариантный budget audit должны
пройти после окончательной интеграции замороженных исходников. Parent ещё не
изменён; этап пока не объявлен production-complete.

Проверка optional call-frame emission: `PUSH IX` перенесён лишь после построения
локального массива snapshot-пар. Этот расчёт не эмитит инструкций; при nil sink
`beginNativeCallFrame` возвращается сразу, `endNativeCallFrame` лишь очищает nil
state. Машинная последовательность default path должна остаться прежней.
Контроль actual ASM/BIN обязателен, потому что затронут codegen: одного reasoning
о комментариях недостаточно. После freeze нужны все пакеты, fresh CLI evidence
для 16 вариантов, showcase и четыре budget-компиляции со свежим assembler.
