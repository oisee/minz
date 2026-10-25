# Итерация 9: общий byte buffer/view API — библиотечный слой

Коммит реализации: `0a0eb3d2c`. Импортируемый модуль
[`stdlib/mem/bytes.nanz`](../../stdlib/mem/bytes.nanz) содержит ByteView и
ByteBuffer: 4 и 6 байт на текущем Z80. Представление ByteView проверяется на
точное совпадение с общим `hir.BorrowedViewType`.

API: init, try_get, try_set, subview, try_push, view, clear. Память предоставляет
вызывающая сторона; descriptor не выделяет/не освобождает память. Срез копирует
pointer/length и разделяет bytes. Clear обнуляет длину buffer, но не длину ранее
сохранённого view и не backing storage. Проверки границ не являются проверками
lifetime. Владелец обязан сохранять storage живым и сообщать верный размер.

## Доказательства

`python3 research/crystal-hir/verify_buffers.py` воспроизводит:

- 20 наборов = capacities 0/1/3/255/256 × seeds 0/1/127/255;
- каждый проверяет все 258 исходных backing/guard bytes, полную ёмкость,
  отказ лишнего push/get/set, сохранение output при отказе, пустой end view,
  subview с output=input, alias writes, clear и сохранённый view;
- ещё 2 сценария: диапазон с переполнением u16 и пустые null descriptors;
- 22 target assertions на raw MIR2, optimized MIR2 и собранном Z80;
- 22 соответствующих native Crystal reference сценария на stock Slice.

Тест импортирует настоящий модуль, не встроенную копию. Проверяет отдельные
байты и поля, а не checksum. `go vet ./pkg/nanz` прошёл.
Receipt с hashes исходников: [evidence/iteration9-library-results.json](evidence/iteration9-library-results.json).

## Что ещё не завершено

Crystal reference — независимый oracle поведения, **не вызов Nanz-библиотеки из
Crystal**. Следующий шаг — фасад Crystal, который связывается с теми же HIR
функциями этой библиотеки, с отрицательными проверками сигнатур и сквозными
native/target случаями. Не добавлены generic buffers, ring buffers, владение,
автоматическая проверка lifetime или runtime каналов. Существующие BufWriter
и BufStream не заменены.
