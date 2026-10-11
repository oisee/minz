# Проверенный выбор arena-first сравнения по бинарникам

Реализован ограниченный selector для одного точного helper
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
прошёл (0.454s) и construction/proof binding прошёл (0.178s). На интегрированном компиляторе HIR20.398 s, MIR5.706 s, Nanz1.745 s, C895.611 s,
CLI20.866 s проходят. Showcase31 прежнее падение,0 новых. Все16 свежих
CLI-кандидатов hashmap/tokenizer сохранили прежние ASM/BIN и прошли независимые
host/typedpayload аудиты. Четырёхвариантный budget audit прошёл за241.611 s: свежие ASM/BIN
совпали с предыдущим этапом (344/344/232/344 B). Полный pipeline прошёл за397.044 s.

Проверка optional call-frame emission: `PUSH IX` перенесён лишь после построения
локального массива snapshot-пар. Этот расчёт не эмитит инструкций; при nil sink
`beginNativeCallFrame` возвращается сразу, `endNativeCallFrame` лишь очищает nil
state. Машинная последовательность default path должна остаться прежней.
Контроль actual ASM/BIN выполняется вместе со свежими CLI-артефактами16 вариантов
и четырьмя бюджетными компиляциями со свежим assembler.

VM domain:33 924 случая/67 848 запусков,11 837 460 точных ordered byte reads,
длины0..255 и каждая позиция mismatch, same/±1 alias, modular arithmetic и четыре
префикса host-denial. Target domain:14 успешных +4 failed workloads, оба
actualбинарника:36 наблюдений, точные globals/resources/events/failure.
Final selector E2E прошёл21.898 s; independent host/typedpayload/proof audits
прошли для всех четырёх кандидатов обоих goals, включая gzip-копии.

Новый stack capability поддерживает только фактически проверенную форму
A-byte→caller-saved byte через IX snapshot. Другие result ABI отклоняются.
Фиксированная последовательность сохраняет flags, проверяет slot→source byte,
initialized stack, SP/IX anchor и границы, исключая return frames. Optional
markers не добавляют машинных инструкций; authority не выводится из numeric
адреса или user-provided ASM.

[Артефакты](grace-memory-host-arena-artifacts/manifest.json). Следующая цель —
append-copy внутриintern_region, где254254 T составляют20,82% исходного workload;
Изолированная проба выноса двух баз адресов уже дала1150124 T /2076 B
против1221494 T /2091 B: −71370 T (5,84%) и −15 B. Полный30-call
workload прошёл независимые аудиты состояния и ordered host events.
Независимый pre/post proof и граничный domain ещё проверяются; этот выигрыш
не входит в текущий arena selector.
