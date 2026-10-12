# Parser reference после удаления trampoline

Полный retained pipeline dd100284 сообщил factory/association drift. Третий target путь отдельно воспроизведён. Все3tests проходят независимый30-case oracle до fixed-cost assertion; FAIL записи и исходные exact source/ASM/BIN/MIR/allocation/target сохранены до изменения ожидания.

Исторический6d80:1296385T/2380B. Текущийa08a:1296226T/2377B. Diff показывает один удалённый trampoline parse_sexpr: branch сразу ведёт на loop_exit21 вместо промежуточного JRS. Бинарная разница−3байта; per-call разницы тактов суммируются в159. Удалённые комментарии adapter peek/poke байты не меняют. 30состояний/results/globals/regions/events/error совпадают с historical, кроме диагностического steps. Новая association также проверяет exact равенство всего native TargetEvidence с отдельным baseline.

Обновлён только fixed test reference. Все3пути PASS4,335с. Старый полный pipeline остаётся работающим на своём неизменном executable и будет иметь исходный FAIL. Он не перезапускается и не объявляется PASS после targeted repair.

before содержит pre-assert artifacts, historical-6d80 — прежний baseline; asm.diff и audit.json сохраняют объяснение. Независимая корректность предшествует изменению T/SHA; workload и ожидаемые parser эффекты не ослаблены.
