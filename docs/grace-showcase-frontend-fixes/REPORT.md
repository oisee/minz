# Showcase: типы и адреса frontend

База: `4f556d23`. HIR verifier, knownFailures и corpus skip не изменены.

Исправления:

- Pascal array store теперь передаёт адрес индексируемого элемента, а не его байтовое значение.
- Pascal record имеет реальный StructTy с общим объектом типа для аргументов, globals и полей. Поля сохраняют объявленную ширину и packed byte offset; `Y` больше не читает `X`.
- Pascal WriteU8 разделяет метки и инструкции inline ASM на отдельные строки. Иначе метка `ones` терялась при обработке assembly.
- PL/M объявленный scalar WORD `PTR(I)` означает байтовое чтение по хранимому адресу. Array store использует address-of; объявленные globals сохраняют BYTE/WORD типы.
- PL/M `.CELL` и `.ARR(index)` сохраняются как address expression вместо чтения CELL и потери индекса. Ранее молча отбрасывавшееся поле `.record.field` теперь явно отклоняется, пока frontend не поддерживает record address.
- Lizp `peek`/`poke` превращаются в byte load/store, poke явно усекает word до byte. В return-контексте `cond`/`if` с `progn` сохраняют побочные эффекты префикса и возвращают последнее выражение.

## Проверка

Три regression tests выполняют по три независимых ожидания через MIR2 VM и собранный Z80: всего девять пар проверок. Lizp проверяет 300 → byte 44 и итог 44 + 256 = 300 после side effect; Pascal проверяет X/Y offsets, вызов с record и записи Arr[0..2]; PL/M проверяет .ARR(1), byte indexing и сложение modulo 256. Адреса в ожиданиях берутся из source address-of, не из предположения о расположении VM heap.

Полные `pkg/lizp`, `pkg/pascal`, `pkg/plm`, `pkg/lanz`: PASS.

Из пяти исходных showcase HIR failures все устранены. Четыре исходных showcase собираются в BIN: bubble_sort.pas 489 bytes, records.pas 207 bytes, hello.plm 21 bytes, sum_array.plm 32 bytes. self_tokenizer.lizp теперь доходит до Z80 assertion assembly, где открывает отсутствующий runtime `_print_u8`/`_print_nl`: этот downstream дефект не скрыт и остаётся отдельной задачей.

## Границы доказательства

Showcase проверяет сборку; полное bubble sort исполнение, CP/M console output, полное tokenizer исполнение и hello.plm main не проверялись в этой работе. Девять regression expectations проверяют исполнение реальных BIN. Pascal record ABI по значению и var-record semantics шире этих конкретных чтений не доказывались. Обычный Lizp progn в произвольном expression-контексте вне return требует отдельной поддержки; текущий фикс покрывает обнаруженный return/cond путь. Библиотека language book Nanz не определяет всю семантику Pascal/PLM/Lisp; ожидания здесь ссылаются на объявленные source types, существующее описание `PTR(I)` в self source и стандартный ordered progn return.
