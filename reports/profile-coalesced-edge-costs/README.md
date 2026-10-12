# Коэффициенты профиля после coalesced-edge backend

Три старых profile-теста падали из-за ожидаемых коэффициентов до коммита 897e7646, который убрал переходы при уже совмещённых block-edge регистрах. Расхождения воспроизведены на baseline 299559a8 до PGO/table интеграции. Сохранённый бинарник 94a41120 прошёл старые ожидания. Production в этом изменении не меняется.

Свежие 1022 измерения raw VM/native Z80 подтверждают прежние результаты и следующие коэффициенты:

| Путь | До | После | Причина |
| --- | --- | --- | --- |
| fast, n>0 | 260 + 37n | 241 + 37n | conditional jump/trampoline заменены RET Z: нет taken jump 12T и trampoline 12T, есть untaken RET Z 5T; −19T |
| fast, n=0 | 262 | 232 | untaken branch 7T + два taken jumps по 12T + RET 10T заменены taken RET Z 11T; −30T |
| fallback | 281 + 193n | 258 + 191n | untaken branch 7T заменён untaken RET NC 5T на итерации; завершение 12+12+10 заменено 11T; −23T |
| whole module pointer | 262B | 251B | удалены пять 2-byte relative jumps и один RET |

В ASM-delta видны RET Z, RET NC, удаление трёх trampolines и отдельного loop-exit RET. FastIteration=37, original=69+191n и equality extra=7 не изменились. Long_sum(40) стал 1436 вместо 1455T по той же −19T дельте. На фиксированных шести длинах sum_array original=65545T, selected=14054T (раньше14179T).

Ожидания обновлены только после сравнения ASM и повторного независимого измерения всех 1022 случаев, включая equality/fallback/wrap, результатов, памяти, ветвей и PC/SP. Три прежде падавших теста PASS 21,204 с, включая отказы stale/missing/config profile. Synthetic multi-function selector model остаётся тестовой моделью; его коэффициенты не объявлены production measurement.

Исторические и новые источники, ASM/BIN, workload, rows/model/evidence и execution binaries сохранены на шаре в compiler-reports/profile-coalesced-edge-costs/executed. Все старые FAIL логи сохранены в profile-checked-table-replay. Полный retained pipeline ещё работает; свежая полная проверка потребуется отдельно.
