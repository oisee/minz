# Корпус: поведение native Z80 и каталог оракулов

Сборка в ASM/BIN не доказывает правильность исполнения. На базе `cb587fe4`
добавлены независимые behavioral проверки четырёх реальных файлов и каталог
всех **193** точных путей корпуса. Число уже отличается от прежних 192: каталог
сканирует текущие examples и reports/showcase-src, включая вспомогательные файлы
библиотек; одинаковые basename не объединяются.

## Новый найденный miscompile

`examples/pascal/bubble_sort.pas` на VM сортировал десять исходных значений в
`10 20 30 40 50 60 70 80 90 100`. Native Z80 возвращал после одного прохода:
`30 50 10 60 40 80 20 70 90 100`. До исправления лог сохранён в bubble-before.txt.

Причина в общем MIR2 `OpNot`, а не в Pascal repeat semantics: backend выдавал
одиночный `CPL`, не читая allocated Src и не записывая Dst. В конкретной программе
`SWAPPED` был загружен в C, complement применялся к другому A, а ветвление читало
старый D. Даже для значения в A обычный complement байта не соответствует
one-bit `bool`: `~1` должен дать 0 после усечения к ширине, а CPL даёт 254.

Новый `genNot` явно читает и пишет allocated homes. Bool использует XOR 1,
целые — complement каждого byte с сохранением unrelated A/AF. Wide operands
проходят через существующий pair/spill staging и обе register banks. Два прямых
регрессионных теста проверяют **744** VM/native случая: bool/u8 с main, IX/IY half
registers, spill и 16/24/32-битные main/shadow pairs/spill. Формула — независимое
`(~input)&((1<<width)-1)`, native SP проверен.

Операторная/типовая основа: Nanz Language Book v7 §§3.1–3.4 (ширины типов,
включая bool и u32 через EXX); общий frontend/backend §21. Pascal frontend
lowerer сохраняет NOT как typed complement; спецификация Pascal repeat..until
и boolean NOT соответствует этой программе. Никакая семантика языка не менялась.

## Реально исполненные новые нагрузки

| Файл | Entry / workload | Независимый оракул |
| --- | --- | --- |
| examples/pascal/bubble_sort.pas | INITARRAY → SORT | Независимая сортировка всех 10 байт RAM |
| examples/nanz/08_arena_allocator.nanz | 6 source tests + shared sandbox | Базовые return checks и итоговые ptr/end четырёх арен |
| examples/plm/showcase.plm | 24 scalar calls | modulo256 arithmetic, iterative Fibonacci, unsigned min |
| examples/pascal/records.pas | 16 PointSum/Manhattan calls | Явно заданные поля двух RAM records, сумма и расстояние modulo256 |

Arena: после последовательности независимая модель даёт `arena=c00c/d000`,
`perm=c008/c100`, `level=c1c8/c900`, `frame=c910/cd00`. См. Language Book v7 Ch15
и comments/API actual source. Source MIR-only asserts promoted to VM+Z80 для
проверки native поведения, включая shared sandbox. Модель final state нужна
дополнительно к этим встроенным expected returns.

PL/M: независимые boundary calls включают MULBY3, MINBYTE и Fibonacci,
чьи source asserts всё ещё закомментированы с историческими bug comments.
Старая annotation не считается текущим доказательством ошибки: новые проверки
проходят. Records использует оригинальные POINTSUM/MANHATTAN, только workload
adapter задаёт поля и вызывает их; вычисление результата не переписывается.

Функциональные алгоритмы проверены на VM и реально assembled Z80 BIN.
Ожидаемый сортированный массив и state computed outside compiler. Native return
обязан достигнуть исходного PC=7000, SP=fff2. CP/M PrintArray/main вывод этих
Pascal примеров отдельно не проверяется: выбран workload без BDOS/console.
Это ограничение входа теста отражено в каталоге.

## Каталог — что ещё не доказано

`docs/grace-showcase-native-oracles/catalog.json` содержит на каждый path:
source SHA256, frontend, HIR target, имена runnable function candidates,
externs, actual inline ASM target/ret/clobber/code, source imports и число source
assertions. Эти поля описывают frontend metadata, не pretend execution PASS.
Для каждого файла записаны workload/model либо конкретная следующая работа.

| Категория каталога | Точных файлов |
| --- | ---: |
| Bounded independent VM + Z80 coverage, привязанное здесь | 7 |
| Ещё выбрать независимый bounded oracle | 125 |
| Inline ASM: проверить target/runtime для выбранного entry | 33 |
| Extern runtime: связать reachable implementations/ABI | 22 |
| Agon eZ80/MOS/VDP target | 5 |
| ZVM host memory model, story 128 КиБ | 1 |

Семь покрытых rows — четыре новых плюс три Lizp ZX screen орaкула, повторно
проверенные здесь (6912 bytes each). Это **не исчерпывающий реестр всех уже
существующих repo tests**: другие paths остаются pending в этом каталоге, даже
если исторический тест где-то запускал похожую функцию. Не заявляется, что все
193 programs исполнены native. `entries` также может включать helper/library
functions; caller/workload и reachable environment требуется выбрать явно.
Наличие ASM/extern само по себе не запрещает проверить pure subroutine.

ZVM story[131072] не помещается в ordinary Z80 16-битную память: native backend
явно отказывает по capacity. Host I/O и memory target надо определить прежде,
чем требовать native interpreter execution. Agon потребует настоящий eZ80/MOS
контракт: Z80 emulator не исполняет RST.LIL/ADL как proof целевого hardware.

## Проверки и артефакты

Четыре новых real-source workloads PASS, прямые NOT regressions PASS,
полный `pkg/mir2` PASS. Catalog metadata все 193 paths PASS.
Три существующих Lizp full-screen VM/native cases повторно PASS.

Репрезентативные ASM/BIN arena и bubble-sort, JSON catalog, логи до/после и
SHA256 manifest сохранены в docs/grace-showcase-native-oracles/.
Повтор: из minzc `go test ./pkg/pipeline -run '^TestShowcase.*Native'` и
`go test ./pkg/mir2 -run '^TestZ80Not'`.
Каталог экспортируется `MINZ_SHOWCASE_ORACLE_CATALOG=/absolute/file.json go test
./pkg/nanz -run TestShowcaseExecutableOracleCatalog`.
