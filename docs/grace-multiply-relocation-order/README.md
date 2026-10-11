# Multiply: живой аккумулятор и snapshot scratch

Полный backend gate 714dff60: HIR FAIL за 25,823 с, MIR2 PASS за 9,085 с, Nanz PASS за 13,314 с, VIR PASS за 2,101 с, canonical ISA/rewrite PASS. HIR m18 возвращает 99 вместо 102 для 3,5,7,11; 256 из 257 случаев не совпали.

Причина: genMul оборачивает byte sequence в saveWrittenScratch до переноса живого A. physOverride позже указывает на E, но внешний restore DE возвращает старый E. Следующее чтение живого x видит неверное значение. Snapshot перемещён после saveAccOperandIfLive; __mul8 по-прежнему сохраняет BC/DE перед ABI copies, результат picked up до restore. Constant coefficient и регистровые ограничения не ослаблены.

Существующие независимые production accumulator judge m10/m12/m17/m18 с concrete+256-byte sweep проходят после исправления, 0,019 с. Production-код исправлен; новый дублирующий тест не нужен. Полные HIR/MIR2 запущены отдельно и пока не объявляются PASS. Измеренного ускорения этого correctness fix не заявляется.

Полные пакеты после исправления: HIR PASS за 27,235 с, MIR2 PASS за 10,551 с. Эти пакеты построены после изменения порядка scratch snapshot; новая CP/M recursion и typed-print/indirect fixes ещё не входят в них.
