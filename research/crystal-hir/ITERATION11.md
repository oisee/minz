# Итерация 11: общий ring buffer и два borrowed сегмента

`ByteRing { data:^u8, head:u16, len:u16, cap:u16 }` добавлен в настоящий
`stdlib/mem/bytes.nanz`. Это обычный addressable HIR struct, 8 bytes на Z80.
Данные остаются в памяти вызывающей стороны. API: init, try_push, try_pop,
clear, segments. Native Crystal фасад `MinzBytes::Ring` связывается с теми же
Nanz/HIR функциями через проверяемый контракт byte-library v1.

## Wrap без division

Индекс записи обычно равен `(head+len) % cap`, но сумма может переполниться
раньше modulo. Реализация сначала проверяет full, затем использует
`remaining=cap-head`: если len < remaining, index=head+len; иначе index=len-remaining.
Оба результата находятся в диапазоне 0..cap-1 при корректном descriptor.
При pop head увеличивается и сбрасывается при достижении cap. Cap=0 допускает
пустой null descriptor и отвергает push/pop до обращения к памяти.

## Заимствование и iteration

Wrapped queue представляется двумя snapshot views, в FIFO-порядке. Например,
при storage `[3,1,2]`, head=1, len=3 получаются first=`[1,2]` и second=`[3]`.
Обход не копирует данные. Есть обычный Slice#each и общий generator/pull по
этим двум сегментам. Early stop на границе первого сегмента не должен читать
второй; отдельный byte counter делает это наблюдаемым.

Descriptor copies сохраняют pointer/length, но bytes общие. Clear не стирает
память и не меняет ранее выданные views; будущие writes могут заменить видимые
им значения. Storage должен оставаться живым, cap — истинным. `segments`
требует два разных output descriptors, не перекрывающих self/backing bytes.
Ring работает последовательно; это ещё не channel runtime, scheduler или
синхронизация с IRQ. Нет owning allocation, GC или borrow checker.

## Проверки

`TestByteRingImportedFIFO` импортирует реальную библиотеку. Независимый host
oracle — FIFO slice на Go; для отдельного backing image используется modulo,
тогда как target пользуется subtraction/branch. Capacities 0/1/3/255/256,
seeds 0/255: 10 трасс, 3224 операции и 2578 наблюдаемых queue bytes на engine. Каждый шаг проверяет status, output preservation,
len/head/cap/data. Снимки segments проверяются byte-by-byte в FIFO-порядке,
после трассы сравниваются все 258 backing/guard bytes. Пустой oracle pool
паддится техническим байтом, который не считается наблюдаемым значением.

`python3 research/crystal-hir/verify_ring.py`: native Crystal запускается раньше
длинного target-прогона. Корпус содержит 525 наборов: 256 ring_trace(cap=3)
и 256 ring_early, 8 cap=1/254, 4 cap=256 и 1 empty null. Проверяются четыре
engine и шесть отрицательных wire controls. 525 наборов и шесть отрицательных контролей прошли до добавления symlink gate.
При регрессии обнаружен path-identity bug: /tmp и /private/tmp были одним
worktree, но строки не совпадали с whitelist фасада. Exporter теперь берёт
File.real_path для source и facade. Новый verifier сравнивает wire при запуске
через symlink byte-for-byte, выполняет 7 прежних buffer-сценариев на четырёх
engine и включает независимый Go FIFO test. Финальный прогон тоже прошёл: **525 наборов на native Crystal, обоих MIR2 и
Z80, 6 отказов, symlink gate, 7 прежних buffer-наборов на четырёх engine,
10 host-model трасс**. Receipt:
[evidence/iteration11-results.json](evidence/iteration11-results.json).
Отдельный регрессионный прогон нового exporter/importer прошёл ещё 14 buffers,
16 views и 16 generators на raw/optimized MIR2 и Z80. Native fallback/target
Ring после первого зелёного прогона не менялся. `go vet ./pkg/nanz
./cmd/crystal-record-hir` прошёл.

Library commit: `afe7e685b`. Ring descriptor 8 B не следует путать с frame
двухсегментного generator: текущий консервативный frame занимает 53 B.

За пределами проверенного профиля остаются generic element rings, автоматические
lifetime checks и конкурентное использование. Физическая queue capacity 65535
на Z80 этим не проверяется; endpoint arithmetic не выдаётся за размещение
целой 64K очереди рядом с исполняемым кодом.
