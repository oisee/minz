# ZSQL: исходный диалог на собранном Z80

Исходный main проходит CREATE TABLE, INSERT hello/42, SELECT, `.quit` и 28 пустых вводов. В текущем исходнике `.quit` печатает подсказку, а main выполняет ровно 32 prompt-вызова. Это поведение сохранено, новый выход не добавлен.

Три режима и два BDOS clobber-контракта прошли: 388 точных stdout bytes, строки hello/42, порядок SQL команд, все записи column_text, final db/stmt/row-count и полный RAM с точными исключениями для spill/стека. BDOS10 принимает только замороженные строки в точный input buffer; хвост последнего длинного ввода проверяется независимо. PC=7003/SP=F000 восстановлены. Все режимы дают **1545 байт / 42 787 тактов** без времени SQLite/BDOS callbacks. Совместный повтор SQLite/SAP/ZSQL **PASS 0,510 с**.

Первая проба нашла настоящий assembler bug: `.help` внутри DB string заменялась при раскрытии local label references. Исправление8243e20b сохраняет quoted bytes; negative сохранён отдельно. Полный z80asm **PASS0,050с**. Валидатор по-прежнему предупреждает на допустимый `LD HL, _inbuf+2`; ассемблирование и выполнение проходят, предупреждение не скрыто.

Это native test-owned SQLite/BDOS runtime, не VM ASM emulation или физический CP/M/provider. Trace портов сохраняется; оракул проверяет результаты/порядок типов команд, не каждый байт trace. [Лог](root.txt) · [Freeze](frozen/freeze.json) · [Контрпример](string-before.txt).
