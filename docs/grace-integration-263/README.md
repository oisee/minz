# Проверка имён ASM peephole tests и сохранение source delta

Повтор с точными именами всех 13 тестов asmpeephole_test.go и двух rotate policy тестов: PASS за 0,011 с. Предыдущий root regexp проверял rotate и byte-cycle, но TestAsmPeephole не совпадал со старыми именами. Оба результата сохранены; полный MIR2 идёт на source5f8db72d.

Сохранён source delta 1d512cf0 → 7a066613 (5 изменённых compiler файлов) с SHA256 на /mnt/safe/minz/grace/2026-10-12/compiler-source-delta-1d512cf0-7a066613. Before/after отчёты не заменялись. Дополнительный native packet в reports/rotate-sled-policy-followup сохраняет те же BIN/такты после исправления только comments в агентском build; комментарии не входят в root production gate.
