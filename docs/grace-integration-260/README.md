# Byte-cycle: интеграция и независимые native проверки

Общий emitter сохраняет свободную от всех операндов пару BC/DE/HL и использует её старший байт для закрытого цикла пересылок. PUSH/POP сохраняет внешние живые значения; проверка перекрытий включает identity, уже выполненные пересылки и обе половины пары. Основные flags и shadow AF не меняются. При отсутствии подходящей пары сохраняется прежний fallback.

После интеграции: exhaustive byte-cycle native PASS за 2,653 с; original tokenizer, guarded tokenizer/self-Lanz, packet и прежний prospective heldout PASS за 4,806 с. До интеграции полный MIR2 donor прошёл за 87,192 с. Старый полный pipeline на executable 1f2365be продолжает выполняться и этого emitter изменения не содержит.

Точный обмен A/C: 95 → 33 такта, 14 → 5 байт. Tokenizer PBQP: 5683 → 5593 байта; init 17488 → 16868 тактов. Greedy неизменён и всё ещё лучше. Все before/after исходники, ASM/BIN, состояния и измерения сохранены в reports/byte-cycle-shuffle. Независимый audit.py проверяет SHA и результаты без импорта компилятора.
