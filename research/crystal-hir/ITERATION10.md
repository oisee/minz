# Итерация 10: Crystal вызывает общую byte-библиотеку

Коммит фасада и bindings: `802c61573`; исправление backend: `f0b316b84`.

`bytes.cr` — native Crystal facade с обычными struct `MinzBytes::Buffer` и
`MinzBytes::View`, методами init/get/set/subview/push/view/clear и getters.
`buffer_showcase.cr` использует этот API без marker-функций в прикладном коде.

## Связывание

Exporter признаёт bindings только для точного файла `bytes.cr`, owner и
whitelist методов. Host fallback body нужен native Crystal, но не экспортируется
как вторая target-реализация. JSON содержит явный `library` у соответствующих
функций. Конструкторы связываются с `ByteBuffer_init` / `ByteView_init`.

Go importer читает настоящий `stdlib/mem/bytes.nanz`, разбирает Nanz в HIR и
добавляет функции библиотеки к Crystal-модулю. Короткие wrappers проверяют
return, arity, scalar types и каждое имя/тип поля record; только затем разрешён
pointer cast к ABI библиотеки. Проверка не ограничивается размером структуры.
В layout receipt сохраняется SHA256 реально прочитанной библиотеки.

Nanz пока стирает `^Struct` params до byte pointers в публичном HIR. Поэтому
bridge явно восстанавливает роли self/output по контракту byte API v1; это не
универсальный linker произвольных Nanz/Crystal signatures. Для устранения этого
шва нужен отдельный проход сохранения semantic pointee metadata в Nanz.

По умолчанию stdlib ищется от cwd; `MINZ_STDLIB` задаёт абсолютный корень.
Verifier устанавливает его сам. Методы из пользовательского исходника с теми
же именами не становятся библиотечными bindings автоматически.

## Память и lifetime

View занимает 4 target bytes, Buffer — 6. Field offsets вычисляются по target
HIR, host layout не переносится. Copy/snapshot хранит собственную длину,
разделяя backing bytes. Clear сбрасывает длину buffer; сохранённый view остаётся
прежнего размера и видит новые записи. Null допустим при пустом descriptor.
Caller предоставляет storage и гарантирует время жизни/истинную ёмкость.
Автоматического borrow checker, owning allocations и GC этим не добавлено.

Поддержаны pointers на value-record как параметры и локальный `pointerof` в
верхнеуровневой функции. Pointer/aggregate escape по return остаётся за
границами admitted top-level profile. Явное wrapping `UInt16#to_u8!` понижается
в truncating cast; checked `to_u8` не подменяется wrapping-конверсией.

## Проверка

Команда: `python3 research/crystal-hir/verify_buffer_bridge.py`.
Верификатор проверяет настоящий shared library hash, все восемь bindings,
backing bytes, canaries, bounds failures, output preservation, in-place subview,
clear/snapshot, capacity 256, empty null и overflowing range.
Отдельные аргументы с эффектами обязаны выполниться слева направо даже при
отказе операции. Отрицательные wire controls: unknown binding, body+binding,
wrong return/argument, signed length, reordered fields.

Результат: **538 наборов на каждом из четырёх engine**, шесть отрицательных
контролей. 256 seeds × buffer(cap=3) и effects = 512, ещё 12 наборов capacities
0/1/254, четыре capacity=256, восемь foreach/pull и два boundary scenarios.
Receipt: [evidence/iteration10-results.json](evidence/iteration10-results.json).
Все восемь bindings присутствуют; hash библиотеки совпал с сохранённым receipt
итерации 9. Новая реализация target-операций не дублирует код библиотеки.

Прежние views и generators проверены новым exporter/importer: по 16 выбранных
случаев на raw/optimized MIR2 и Z80. Общие HIR descriptor/completion/derived-frame
и Nanz byte-buffer tests прошли. Backend regression comparisons/fields прошёл.
`go vet ./cmd/crystal-record-hir` и `go vet -stdmethods=false ./pkg/mir2` прошли;
обычный mir2 vet сообщает прежние ReadByte/WriteByte naming warnings в
flag_writer_physical_test.go, этот файл не менялся.

## Общее исправление backend

Новая связка buffer → view → pull обнаружила Z80-only mismatch: seed=1 давал
0 вместо 1, при зелёных обоих MIR2. Перед записью comparison bool в frame
адресация поля через ADD HL меняла carry. Коммит **f0b316b84** материализует
pending predicate перед flag-clobbering field/pointer-bump адресацией; короткие
16-bit INC сохраняют flags и не требуют этого шага. Native regression:
6 offsets × 6 conditions × 6 operand pairs = 216 проверок. До исправления
1 < 2 превращалось в false; после оно проходит вместе с исходной связкой.
Сохранены before logs и отрицательные wire controls.

Старые scalar/meta/arena пути не заменены. Этот facade не является полным
Crystal stdlib или generic slice API. Ring buffers, owning allocations и
автоматическая проверка lifetime остаются отдельными шагами.
