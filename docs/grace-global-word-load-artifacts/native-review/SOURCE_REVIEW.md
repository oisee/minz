# Readonly source review a149a48f

Два production файла просмотрены независимо. DE/BC matcher требует OpLoad16, same-block OpAddrOf известного global, local use1 и countRegUses всей функции ==1; actual global ByteWidth>=2. countRegUses использует Inst.Uses и termUses всех блоков, поэтому direct successor use не подавляется. При повторных номерах регистров guard консервативно отказывает. Прежний HL matcher не расширен и emitted HL строка восстановлена точно.

Emitter допускает те же HL/DE/BC; DE/BC использует одну typed semantic annotation width2 по тому же Inst/root. invalidate(dst) удаляет destination pair/byte aliases. LD direct не меняет AF/HL; unrelated caches сохраняются. Numeric/offset/multiuse/other-destination paths не добавлены. Optional sidecar/runtime/codegen helpers не изменены.

Существующие 12 машинных значений DE/BC проверяют actual opcode ED5B/4B, 20T, значение, AF/HL/other-pair/SP и отсутствие записей; shared/numeric/direct-successor отказ подтверждён focused-current.txt PASS0.018. Это owner's execution, не мой повтор CPU. Счётчики этих тестов подтверждают оба байта, но не временной порядок; порядок независимо подтверждён emulator ld16rrnn и runtime byte recipe. Дополнительный subsequent destination/cache/store и ED malformed sidecar test ожидается от владельца до финальной фиксации regression scope.

На текущем production snapshot конкретных remaining correctness holes не найдено. Полный caller эксперимент выполняет владелец после freeze; данный документ не заявляет новых T/byte результатов.
