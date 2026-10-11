# Исходные циклы и native memory-host baseline

Отдельный RunZ80MemoryHostLoopProfile связывает реальные счётчики исходного HIR
с проверенным бинарником и закрытым byte-host workload. Неизменённый SelfLanz
проходит30 persistent calls:11 исходных циклов, все cumulative и per-call
счётчики совпадают с независимой Python-моделью; три недостижимых цикла имеют0/0.
region_match даёт49 входов и59 входов в тело. Native baseline остался
1221494 T /2091 B, BIN SHA256fed98ba78becf9431a5301f31330363cc3005f1f6692fd1f9e9183d80b9d0a51.

Счётчики получены отдельным private VM, до Grace/splitting, настоящими entry/body
событиями; рекурсия накапливается в том же source site. Инструментация потребляет
VM gas, поэтому её Steps не являются стоимостью исходника или Z80. Native
бинарник не содержит counter hooks. Контекст профиля имеет отдельную semantics
identity и schema; старые no-host loaders его не принимают. Runtime counts
не доказывают legality или source-to-target loop lineage.

Workload копируется до первого factory вызова. Пять независимо выделенных HIR
копий проверяются по полному typed/source/origin identity; изменяться может
только privately compiled candidate. Последний sentinel ловит позднюю мутацию
сохранённых source nodes. Counter hosts только внутренние, без user callbacks;
имена исключают globals/functions/calls, и instrumented VM heap extent заново
проверяется против RAM регионов. Все исходные loop origins обязательны.

Семантический отказ встроенного byte-host сохраняет точный Kind/Address/Write,
состояние и порядок уже выполненных обращений, а также префикс счётчиков. Gas
и infrastructure ошибки не дают baseline association evidence. Goto исходники
явно отклоняются: текущие счётчики определены для структурированных входов.
Обычный низкоуровневый profiler сохраняет прежний API и guard-mode.

Независимый review HIR и factory wrapper не нашёл конкретных дефектов в этом
ограниченном контракте. Проверены zero-trip, рекурсия, отказ в condition/body,
reset/persistent state, inward goto, namespace collisions, detached origins,
instrumented heap overlap, gas и поздняя мутация factory source.

На интегрированном компиляторе HIR20.290 s, MIR6.230 s, Nanz1.811 s, C895.955 s,
CLI20.874 s проходят; showcase31 прежнее падение,0 новых. Полный pipeline
прошёл296.585 s. Четыре бюджетных варианта с двумя Grace-проходами прошли
247.920 s; свежий assembler подтвердил точные прежние ASM/BIN344/344/232/344 B.
Прежний машинный gate722.080 s сохранён по идентичности бинарников.

[Артефакты](grace-memory-host-loop-artifacts/manifest.json).
Следующая проба — host-bound first-byte peeling arena-first сравнения. Её
проверка потребовала отдельного контракта compiler-owned IX snapshot; он
остаётся в изолированной ветке, не входит в этот профильный этап.
