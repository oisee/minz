# Grace PGO: исходный hashmap и точная PC-атрибуция

Неизменённый полный `examples/nanz/21_hashmap_test.nanz` прошёл независимую
проверку 36 последовательных операций с общим состоянием: **138 440 T-states,
1013 байт**. Сохранённый Z80-бинарник имеет SHA256
`aefde17ab12e824e4d790cc7147f0974329faf5dded51f032643eb316d56ba86`.
Это исходный baseline без выбора оптимизации. Текущий parent `1d4d3d10`
повторил focused test за 0.304 с; source, ASM и binary побайтово совпали с
историческим baseline `ef270333`. Новое изменение добавляет только тест: broad
suite заново не запускался, ранее проверенный production gate сохраняется
отдельно.

Host-оракул независимо реализует точную семантику исходника: hash
`(key xor ((key + 37) mod 65536)) & 15`, максимум 16 probes, переход
`(slot + 1) & 15`, little-endian keys/values, обновление count только при вставке
нового ключа. `map_clear` очищает count и occupancy, сохраняя старые keys/values.
Проверены результаты, полные 82 байта глобального состояния и чтения/записи
каждого global byte на original, precompile, postcompile raw VM и actual Z80.

Workload включает empty miss, collisions с переходом через slot 15 к 0,
overwrite, заполнение 16 slots, отказ для семнадцатого ключа, overwrite полной
таблицы, miss в полной таблице, ключи/значения 0 и 65535, clear/refill. Сохранена
особенность исходного `map_has`: существующий ключ со значением 65535 даёт 0.
Она покрыта трижды и не означает обычную независимую проверку присутствия.

Инструментированный replay считает PC начала инструкции и её реальные cycles,
исключая call bootstrap. До принятия атрибуции он сверяет точные ASM и бинарник,
полные invocations/state/global traffic, bootstrap identities/costs и executed
steps с исходным evidence. Таблица показывает исключительные затраты физических
диапазонов функций: стоимость callees вынесена отдельно и не удваивается в callers.

| Физическая функция | T-states | Доля |
|---|---:|---:|
| map_put | 74 633 | 53.91% |
| rd16 | 33 390 | 24.12% |
| map_get | 14 426 | 10.42% |
| wr16 | 12 464 | 9.00% |
| map_clear | 3054 | 2.21% |
| map_has | 473 | 0.34% |

Standalone `hash16` имеет 0 T-states: вызовы встроены в put/get; отдельный машинный
код функции остаётся в бинарнике. `rd16` исполняется 159 раз по 210 T-states;
`wr16` — 38 раз по 328 T-states. Полные физические block costs сохранены отдельно.

Следующий узкий эксперимент — повторное использование вычисленного адреса в
точном `rd16`, сохраняя два явных u8 reads, сначала low, затем high, и последующее
объединение в u16. Нужны отдельное доказательство modular address/first trap и
actual whole-caller measurement. 24.12% — наблюдаемая стоимость функции, из неё
не следует выигрыш преобразования. Этап не меняет исходник, workload или production
lowering и не заявляет profitability.

Границы проверки конкретны: исходный state нулевой, первый вызов clear; наблюдались
начальные hash slots только 5, 11 и 15, count в диапазоне 0..16. Нет seeded
ненулевого count/occupancy, carry 255→256 или 65535→0, всех 16 start slots,
poisoned occupancy, отдельных rd16/wr16/hash16 entry calls или out-of-bounds
случаев. Восемь исходных assertion functions сохранены, но этот workload вызывает
только clear/put/get/has при AssertMode:none. Счётчики доступа агрегированы по
байтам и не доказывают порядок native low/high events или traps. Stack, code fetch,
bootstrap, initialization и snapshot reads исключены из declared-global traffic.

[Артефакты](grace-pgo-hashmap-artifacts/README.md) ·
[Независимый baseline review](grace-pgo-hashmap-artifacts/minz-hashmap-baseline-review.md) ·
[PC report](grace-pgo-hashmap-artifacts/pc/report.md)
