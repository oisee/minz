# Полный токенизатор и границы сохранения A

Исходный self_tokenizer main без изменений повторно прошёл после интеграции за 1,366 с. Независимая модель: 19 tokens, 91 interned byte, 146 байтов stdout; все globals/RAM/guards/SP/code сверены. Native порт23: 199102 T / 5466 B; явный CP/M adapter: 226357 T / 5554 B. Это эмуляция BDOS2, не physical OS. Коммит 078cca62.

Byte packing защищён от неявных MIR потребителей флагов (81e0d8b6). Дополнительные два пути сохранения A на exhausted scratch исправлены в 0041d9e4. Перед исправлением native DE=0x1234 становился 0x2534; после него сохраняются и DE, и A=37. Полный MIR2 PASS6,865с и production conditional/FAT12/call matrix PASS5,704с.

Код запушен. Пакеты docs/grace-byte-pack-state, docs/grace-original-tokenizer-main и docs/grace-accumulator-exhaustion скопированы SHA256-проверенной копией на /mnt/safe/minz/grace/2026-10-11/compiler-docs/. Большой pipeline-cc502-full выполняется на старом исходном checkpoint; новый PASS не заявляется.
