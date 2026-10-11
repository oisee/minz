# Измеренный выбор append-copy base hoist

Новый ограниченный API принимает фабрику неизменённого HIR, закрытый
memory-host workload и сериализованный source-loop профиль. Выбор относится
только к исходному append-циклу intern_region/body/s5, не к соседнему циклу
поиска. После строгой загрузки весь профиль повторно исполняется; caller
не передаёт transformed factory, assembly или готовые стоимости.

Приватное преобразование после исходного header poke сохраняет две u16 базы:
src_buf+start и str_bump+1. Оно оставляет j:u8, исходный while, forward порядок
peek→poke, bump и return. Сложения остаются модульными. Поэтому пересекающиеся
буферы наблюдают предыдущие записи так же, как исходный цикл; snapshot-copy
не используется. Число внутренних чтений globals уменьшается; контракт
сохраняет ordered host-resource events и итоговое состояние, не внутренний gas.

Независимый pre/post validator проверяет точную функцию, весь остальной модуль,
метаданные, reserved bindings и реальные source-origin. Он самостоятельно
исключает пересечение ресурсов с VM module storage: одного Prepare недостаточно.
Native runner независимо контролирует свою размещённую память, globals, spills,
stack и code. Новых разрешений доступа и изменений backend для hoist нет.

Typed precompile proof связывает исходный source site с хвостом body/s7,
полные original/actual FunctionSiteMap и natural baseline compatibility.
Он относится к HIR до compiler preparation; у преобразованного цикла нет
унаследованных счётчиков или заявленного сертификата final MIR. Фабрики
проверяются до/после callback, прежние AST/backing allocations удерживаются
через GC, поздние изменения источника отклоняются.

Изолированный public selector прошёл оба objectives: original1221494 T /2091 B,
hoisted1150124 T /2076 B. Speed и size выбирают hoisted: −71370 T (5,84%),
−15 B. Это измерение полного workload из30 вызовов неизменённого SelfLanz.
Нулевой append-вызов выбирает original731 T против895 T; равная основная
стоимость также сохраняет original. Goal меняет выбор, не compiler options.

Raw domain:1286 случаев/2572 VM invocations/655388 ordered events, длины0..255,
alias offsets−1/0/+1/+2, header propagation, modular source sums и6 typed
partial denials. Actual domain:16 successful+5 denial workloads,42 native
invocations, independent sequential oracle для globals/resources/events.
Проверки прошли; massive domain не повторяется без изменения преобразования.

Final independent selector review и GC/cap/proof-binding gates прошли.
Семь новых файлов интегрированы; существующий backend не изменён. HIR21.144 s,
MIR6.295 s, C896.057 s и CLI21.199 s PASS; showcase31 прежнее падение,0 новых.
Первый общий Nanz run включал известный showcase и потому завершился FAIL;
отдельный Nanz без showcase прошёл за2.148 s. Полный pipeline ещё выполняется.

[Артефакты](grace-memory-host-append-artifacts/manifest.json).
