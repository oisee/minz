# Z80: корректные byte constant operands

Эксперимент с u8 slot в hashmap выявил ошибку backend: folded constant имел
назначенный физический регистр A, но его значение фактически не загружалось.
Ранняя ALU-ветка принимала текущий accumulator за RHS constant. Вместо
`(slot + 1) & 15` получался `AND C`, где C содержал новый slot; переход через 15
ломал slot wrapping и последующую вставку.

Общий repair исключает не материализованные constants из ранней RHS-in-A
ветки (`!rhsIsConst || !deadConsts`) и исключает constants из commutative cache
ветки (`!rhsIsConst`). Ранее загруженная constant продолжает использовать
безопасную прежнюю ветку; folded constant обрабатывается immediate path. Физическая allocation сама по себе не
доказывает, что folded constant материализован в регистре. Изменение касается
byte ALU operands, а не специального lowering hashmap.

450 actual machine cases проверяют ADD, SUB, AND, OR и XOR: два расположения
constant operand × девять boundary inputs × пять constants, после вычисления
`x + 1` с живым accumulator. Независимое ожидание учитывает u8 wrap. Before log
сохраняет ошибки; финальный focused gate прошёл за 0.291 с. Финальная проверка двух 36-call
workloads прошла за 0.648 с. Полный MIR2 7.126 с относился к промежуточным
conservative guards и сохранён только как история. Финальные проверки прошли:
pipeline — 271.656 с, HIR/MIR2/Nanz/C89 — 18.482/4.664/1.591/5.254 с,
CLI — 20.054 с. В showcase остались прежние 31 падение, новых нет.

Manual HIR slot prototype после repair проходит все 36 actual Z80 вызовов с
независимыми results, полными 82 байтами global state и каждым побайтовым access
counter: **141 354 T / 1047 B**. Это отрицательный результат относительно
исходного ранее проверенного **133 994 T / 984 B**. Narrowing не принят как
оптимизация. До repair actual target останавливался на третьем `map_put(13,200)`:
`map_keys[0]` оставался 0 вместо 13. Ошибка, ASM, бинарник, expected state и null
failed target evidence сохранены; oracle не изменён под ошибочное исполнение.

Исходник hashmap и workload не переписаны. Прототип содержит два manual HIR/test
файла, не является production Grace rule и не зарегистрирован в PGO API. Он
проверял точную исходную форму masked slot и заменял slot reads на cast u8→u16;
его source-origin clone bookkeeping пока не является независимо подтверждённым
production proof. Raw table domain и successful actual caller не заменяют
такого доказательства.

Финальный corrected backend сохраняет исходный hashmap **133 994 T / 984 B**:
source, workload, ASM и binary побайтово совпадают с предыдущим word-shift8
этапом. Negative slot prototype хуже на **7360 T-states и 63 байта**. Это
измеренный отказ от оптимизации, а не выигрыш general repair. Промежуточный
conservative repair давал исходный 134 827 T / 986 B и slot 142 187 T / 1049 B;
после измеренной регрессии guard уточнён безопасно для materialized constants.
Эта промежуточная версия, её patch и незавершённые gates сохранены только как
история. Финальные source snapshots и focused результаты зафиксированы.

Traffic evidence агрегировано по declared global bytes и не доказывает native
ordered events или trap ordering. Четыре budget-варианта заново скомпилированы
за 251.813 с: исходник, ASM и BIN побайтово совпали с предыдущим этапом, размеры
344/344/232/344 байта. Поэтому прежняя независимая машинная проверка применима
к тем же бинарникам. Новый полный машинный прогон budget не заявляется.

[Артефакты](grace-byte-constant-artifacts/README.md) ·
[Независимая проверка repaired slot workload](grace-byte-constant-artifacts/independent-slot-verification.txt)

Финальный CLI заново проверил все 4 сочетания copy/region на 30 вызовах tokenizer для обоих режимов. Стоимости и все4бинарника остались прежними: 2257650 T / 4317 B, 2201516 T / 4703 B, 2247229 T / 4446 B, 2191095 T / 4832 B. Saved-artifact host/typedSHA проверки подтвердили результаты, полное состояние, побайтовые обращения ресурсов и выбор speed=3 / size=0.
